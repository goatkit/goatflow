package api

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/models"
	"github.com/goatkit/goatflow/internal/platform/webhook"
	"github.com/goatkit/goatflow/internal/repository"
)

// fakeOIDCIdP is a minimal OpenID Provider: discovery, JWKS and a token
// endpoint that returns an ID token for email signed with its key.
type fakeOIDCIdP struct {
	srv      *httptest.Server
	key      *rsa.PrivateKey
	clientID string
	email    string
}

func newFakeOIDCIdP(t *testing.T, clientID, email string) *fakeOIDCIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	f := &fakeOIDCIdP{key: key, clientID: clientID, email: email}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                f.srv.URL,
			"authorization_endpoint":                f.srv.URL + "/authorize",
			"token_endpoint":                        f.srv.URL + "/token",
			"jwks_uri":                              f.srv.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "alg": "RS256", "use": "sig", "kid": "k1",
			"n": base64.RawURLEncoding.EncodeToString(f.key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(f.key.E)).Bytes()),
		}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		now := time.Now()
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"iss": f.srv.URL, "aud": f.clientID, "sub": "sso-user", "email": f.email,
			"iat": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(),
		})
		tok.Header["kid"] = "k1"
		signed, err := tok.SignedString(f.key)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "at", "token_type": "Bearer", "id_token": signed})
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// ssoTestProvider inserts an enabled provider row and removes it (and any
// user it provisioned) when the test ends.
func ssoTestProvider(t *testing.T, p *models.IdentityProvider) uint {
	t.Helper()
	db := getTestDB(t)
	now := time.Now()
	p.CreateTime, p.ChangeTime = now, now
	p.Enabled = true
	if p.UserTable == "" {
		p.UserTable = "users"
	}
	require.NoError(t, repository.NewIdentityProviderRepository(db).CreateProvider(p))
	t.Cleanup(func() {
		cleanupProvider(db, p.ID)
		if p.AutoProvision {
			_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM group_user WHERE user_id IN (SELECT id FROM users WHERE login = ?)`), p.Name)
			_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM users WHERE login = ?`), p.Name)
		}
	})
	return p.ID
}

func ssoContext(method, target string, form url.Values, providerID uint) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if form != nil {
		c.Request = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		c.Request = httptest.NewRequest(method, target, nil)
	}
	c.Params = gin.Params{{Key: "id", Value: strconv.FormatUint(uint64(providerID), 10)}}
	return c, w
}

func responseCookie(w *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, ck := range w.Result().Cookies() {
		if ck.Name == name {
			return ck
		}
	}
	return nil
}

func location(w *httptest.ResponseRecorder) string { return w.Header().Get("Location") }

// startSSOLogin runs the redirect handler and returns the state token the
// IdP will send back and the browser-binding cookie that was set.
func startSSOLogin(t *testing.T, redirect gin.HandlerFunc, providerID uint, stateParam string) (string, *http.Cookie) {
	t.Helper()
	c, w := ssoContext(http.MethodGet, "/auth/"+strconv.FormatUint(uint64(providerID), 10), nil, providerID)
	redirect(c)
	c.Writer.WriteHeaderNow()
	require.Equal(t, http.StatusFound, w.Code, "redirect to the IdP: %s", location(w))
	u, err := url.Parse(location(w))
	require.NoError(t, err)
	state := u.Query().Get(stateParam)
	require.NotEmpty(t, state, "%s in %s", stateParam, location(w))
	ck := responseCookie(w, ssoBrowserCookie)
	require.NotNil(t, ck, "the redirect must set the %s cookie", ssoBrowserCookie)
	assert.True(t, ck.HttpOnly)
	assert.Equal(t, ssoBrowserCookieMaxAge, ck.MaxAge)
	assert.Len(t, ck.Value, 64)
	return state, ck
}

// The OIDC callback is accepted only from the browser that started the
// login: the state alone (which an attacker can obtain by starting a login
// with their own account) is not enough.
func TestOIDCCallbackRequiresBrowserBinding(t *testing.T) {
	t.Setenv(webhook.AllowPrivateTargetsEnv, "true") // the fake IdP is on loopback
	db := getTestDB(t)
	setDB(db)
	defer restoreDB()
	auth.SetStateStore(auth.NewMemoryStateStore())

	login := fmt.Sprintf("sso-oidc-%d@example.test", time.Now().UnixNano())
	idp := newFakeOIDCIdP(t, "gf-client", login)
	providerID := ssoTestProvider(t, &models.IdentityProvider{
		Name: login, ProviderType: "oidc", ClientID: "gf-client", ClientSecret: "s",
		DiscoveryURL: idp.srv.URL, Scopes: "openid email", AutoProvision: true,
	})

	callback := func(t *testing.T, state string, cookie *http.Cookie) *httptest.ResponseRecorder {
		c, w := ssoContext(http.MethodGet, "/auth/"+strconv.FormatUint(uint64(providerID), 10)+"/callback?code=abc&state="+url.QueryEscape(state), nil, providerID)
		if cookie != nil {
			c.Request.AddCookie(cookie)
		}
		handleOIDCCallback(c)
		c.Writer.WriteHeaderNow()
		require.Equal(t, http.StatusFound, w.Code)
		return w
	}

	t.Run("redirect sets a Lax cookie bound to the state", func(t *testing.T) {
		_, ck := startSSOLogin(t, handleOIDCRedirect, providerID, "state")
		assert.Equal(t, http.SameSiteLaxMode, ck.SameSite)
	})
	t.Run("callback without the cookie is rejected", func(t *testing.T) {
		state, _ := startSSOLogin(t, handleOIDCRedirect, providerID, "state")
		w := callback(t, state, nil)
		assert.Contains(t, location(w), "error=invalid_state", "got %s", location(w))
		assert.Nil(t, responseCookie(w, "auth_token"))
		_, ok := auth.GetStateStore().GetState(state)
		assert.False(t, ok, "a rejected state must not be retryable")
	})
	t.Run("callback with another browser's cookie is rejected", func(t *testing.T) {
		state, _ := startSSOLogin(t, handleOIDCRedirect, providerID, "state")
		_, other := startSSOLogin(t, handleOIDCRedirect, providerID, "state")
		w := callback(t, state, other)
		assert.Contains(t, location(w), "error=invalid_state", "got %s", location(w))
		assert.Nil(t, responseCookie(w, "auth_token"))
	})
	t.Run("callback from the starting browser logs in and clears the cookie", func(t *testing.T) {
		state, ck := startSSOLogin(t, handleOIDCRedirect, providerID, "state")
		w := callback(t, state, ck)
		assert.Equal(t, "/dashboard", location(w))
		assert.NotNil(t, responseCookie(w, "auth_token"))
		cleared := responseCookie(w, ssoBrowserCookie)
		require.NotNil(t, cleared)
		assert.Less(t, cleared.MaxAge, 0, "binding cookie must be cleared after use")
		if sid := responseCookie(w, "session_id"); sid != nil {
			_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM sessions WHERE session_id = ?`), sid.Value)
		}
	})
}

const testIdPMetadataXML = `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://idp.example.test">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.test/sso"/>
  </IDPSSODescriptor>
</EntityDescriptor>`

// The SAML ACS accepts a RelayState only from the browser that started the
// login. A correct cookie gets past the state check (the garbage assertion
// then fails as auth_failed); a missing or foreign cookie does not.
func TestSAMLCallbackRequiresBrowserBinding(t *testing.T) {
	db := getTestDB(t)
	setDB(db)
	defer restoreDB()
	auth.SetStateStore(auth.NewMemoryStateStore())

	providerID := ssoTestProvider(t, &models.IdentityProvider{
		Name: fmt.Sprintf("sso-saml-%d", time.Now().UnixNano()), ProviderType: "saml2",
		EntityID: "https://sp.example.test", IdPMetadataXML: testIdPMetadataXML,
	})

	acs := func(t *testing.T, state string, cookie *http.Cookie) *httptest.ResponseRecorder {
		form := url.Values{"SAMLResponse": {"bm90IGEgc2FtbCByZXNwb25zZQ=="}, "RelayState": {state}}
		c, w := ssoContext(http.MethodPost, "/auth/"+strconv.FormatUint(uint64(providerID), 10)+"/acs", form, providerID)
		if cookie != nil {
			c.Request.AddCookie(cookie)
		}
		handleSAMLCallback(c)
		c.Writer.WriteHeaderNow()
		require.Equal(t, http.StatusFound, w.Code)
		return w
	}

	t.Run("redirect sets a cookie that survives the cross-site POST", func(t *testing.T) {
		_, ck := startSSOLogin(t, handleSAMLRedirect, providerID, "RelayState")
		if ck.Secure {
			assert.Equal(t, http.SameSiteNoneMode, ck.SameSite)
		} else {
			assert.NotContains(t, ck.Raw, "SameSite", "plain HTTP cannot use SameSite=None")
		}
	})
	t.Run("ACS without the cookie is rejected", func(t *testing.T) {
		state, _ := startSSOLogin(t, handleSAMLRedirect, providerID, "RelayState")
		w := acs(t, state, nil)
		assert.Contains(t, location(w), "error=invalid_state", "got %s", location(w))
	})
	t.Run("ACS with another browser's cookie is rejected", func(t *testing.T) {
		state, _ := startSSOLogin(t, handleSAMLRedirect, providerID, "RelayState")
		_, other := startSSOLogin(t, handleSAMLRedirect, providerID, "RelayState")
		w := acs(t, state, other)
		assert.Contains(t, location(w), "error=invalid_state", "got %s", location(w))
	})
	t.Run("ACS from the starting browser passes the state check", func(t *testing.T) {
		state, ck := startSSOLogin(t, handleSAMLRedirect, providerID, "RelayState")
		w := acs(t, state, ck)
		assert.Contains(t, location(w), "error=auth_failed", "got %s", location(w))
		cleared := responseCookie(w, ssoBrowserCookie)
		require.NotNil(t, cleared)
		assert.Less(t, cleared.MaxAge, 0, "binding cookie must be cleared after use")
	})
}
