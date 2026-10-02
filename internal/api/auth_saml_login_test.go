package api

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/xml"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/crewjam/saml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/models"
)

// testSAMLIdP is a SAML identity provider built on crewjam/saml with a key
// generated at runtime. It trusts the SP metadata GoatFlow serves and signs
// responses for one user.
type testSAMLIdP struct {
	idp   *saml.IdentityProvider
	sp    *saml.EntityDescriptor
	email string
}

func newTestSAMLIdP(t *testing.T, email string) *testSAMLIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "idp.example.test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	f := &testSAMLIdP{email: email}
	f.idp = &saml.IdentityProvider{
		Key:                     key,
		Certificate:             cert,
		MetadataURL:             url.URL{Scheme: "https", Host: "idp.example.test", Path: "/metadata"},
		SSOURL:                  url.URL{Scheme: "https", Host: "idp.example.test", Path: "/sso"},
		ServiceProviderProvider: f,
	}
	return f
}

func (f *testSAMLIdP) GetServiceProvider(_ *http.Request, id string) (*saml.EntityDescriptor, error) {
	if f.sp == nil || f.sp.EntityID != id {
		return nil, os.ErrNotExist
	}
	return f.sp, nil
}

func (f *testSAMLIdP) metadataXML(t *testing.T) string {
	t.Helper()
	b, err := xml.Marshal(f.idp.Metadata())
	require.NoError(t, err)
	return string(b)
}

// trustSP configures the IdP with the metadata GoatFlow serves for providerID,
// as an administrator would.
func (f *testSAMLIdP) trustSP(t *testing.T, providerID uint) {
	t.Helper()
	c, w := ssoContext(http.MethodGet, "/auth/"+strconv.FormatUint(uint64(providerID), 10)+"/metadata", nil, providerID)
	handleSAMLMetadata(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var ed saml.EntityDescriptor
	require.NoError(t, xml.Unmarshal(w.Body.Bytes(), &ed))
	f.sp = &ed
}

func (f *testSAMLIdP) session() *saml.Session {
	now := time.Now()
	return &saml.Session{
		ID: "idp-session", CreateTime: now, ExpireTime: now.Add(time.Hour), Index: "1",
		NameID: f.email, UserEmail: f.email,
	}
}

// login answers the AuthnRequest in authURL (the SP's redirect) the way a
// real IdP does: it validates the request and POSTs back a signed response
// in reply to it, echoing the RelayState.
func (f *testSAMLIdP) login(t *testing.T, authURL string) url.Values {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, authURL, nil)
	req, err := saml.NewIdpAuthnRequest(f.idp, r)
	require.NoError(t, err)
	require.NoError(t, req.Validate(), "the IdP must accept GoatFlow's AuthnRequest")
	require.NotEmpty(t, req.Request.ID)
	return f.respond(t, req)
}

// unsolicited builds an IdP-initiated response, or one claiming to answer a
// request GoatFlow never issued when inResponseTo is set.
func (f *testSAMLIdP) unsolicited(t *testing.T, inResponseTo, relayState string) url.Values {
	t.Helper()
	req := &saml.IdpAuthnRequest{
		IDP:                     f.idp,
		HTTPRequest:             httptest.NewRequest(http.MethodGet, f.idp.SSOURL.String(), nil),
		RelayState:              relayState,
		Now:                     saml.TimeNow(),
		ServiceProviderMetadata: f.sp,
		SPSSODescriptor:         &f.sp.SPSSODescriptors[0],
		ACSEndpoint:             &f.sp.SPSSODescriptors[0].AssertionConsumerServices[0],
	}
	req.Request.ID = inResponseTo
	return f.respond(t, req)
}

func (f *testSAMLIdP) respond(t *testing.T, req *saml.IdpAuthnRequest) url.Values {
	t.Helper()
	require.NoError(t, saml.DefaultAssertionMaker{}.MakeAssertion(req, f.session()))
	form, err := req.PostBinding()
	require.NoError(t, err)
	return url.Values{"SAMLResponse": {form.SAMLResponse}, "RelayState": {form.RelayState}}
}

// SP-initiated SAML login works end to end: the response the IdP sends in
// reply to the AuthnRequest issued to this browser logs the user in. Every
// other response is refused: one answering a different request, one that
// was already used, one posted from another browser, and unsolicited
// (IdP-initiated) responses, which GoatFlow does not accept.
func TestSAMLSPInitiatedLogin(t *testing.T) {
	db := getTestDB(t)
	setDB(db)
	defer restoreDB()
	auth.SetStateStore(auth.NewMemoryStateStore())

	for _, tc := range []struct {
		name     string
		entityID string
	}{
		{"explicit entity ID", "https://sp.example.test"},
		{"default entity ID", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			email := fmt.Sprintf("sso-saml-%d@example.test", time.Now().UnixNano())
			idp := newTestSAMLIdP(t, email)
			providerID := ssoTestProvider(t, &models.IdentityProvider{
				Name: email, ProviderType: "saml2", EntityID: tc.entityID,
				IdPMetadataXML: idp.metadataXML(t), UserClaimEmail: "mail", AutoProvision: true,
			})
			idp.trustSP(t, providerID)

			start := func(t *testing.T) (string, *http.Cookie) {
				t.Helper()
				c, w := ssoContext(http.MethodGet, "/auth/"+strconv.FormatUint(uint64(providerID), 10)+"/saml", nil, providerID)
				handleSAMLRedirect(c)
				c.Writer.WriteHeaderNow()
				require.Equal(t, http.StatusFound, w.Code)
				require.Contains(t, location(w), idp.idp.SSOURL.String(), "redirect to the IdP")
				ck := responseCookie(w, ssoBrowserCookie)
				require.NotNil(t, ck)
				return location(w), ck
			}
			acs := func(t *testing.T, form url.Values, cookie *http.Cookie) *httptest.ResponseRecorder {
				t.Helper()
				c, w := ssoContext(http.MethodPost, "/auth/"+strconv.FormatUint(uint64(providerID), 10)+"/acs", form, providerID)
				if cookie != nil {
					c.Request.AddCookie(cookie)
				}
				handleSAMLCallback(c)
				c.Writer.WriteHeaderNow()
				require.Equal(t, http.StatusFound, w.Code)
				if sid := responseCookie(w, "session_id"); sid != nil {
					_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM sessions WHERE session_id = ?`), sid.Value)
				}
				return w
			}
			loggedIn := func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				assert.Equal(t, "/dashboard", location(w))
				assert.NotNil(t, responseCookie(w, "auth_token"))
			}
			refused := func(t *testing.T, w *httptest.ResponseRecorder, reason string) {
				t.Helper()
				assert.Contains(t, location(w), "error="+reason, "got %s", location(w))
				assert.Nil(t, responseCookie(w, "auth_token"))
			}

			authURL, ck := start(t)
			form := idp.login(t, authURL)

			t.Run("response to this browser's request logs in", func(t *testing.T) {
				loggedIn(t, acs(t, form, ck))
			})
			t.Run("replayed response is refused", func(t *testing.T) {
				refused(t, acs(t, form, ck), "invalid_state")
			})
			t.Run("response to another request is refused", func(t *testing.T) {
				// The used response, presented with a live RelayState and the
				// matching cookie, answers a different AuthnRequest.
				authURL3, ck3 := start(t)
				stolen := url.Values{"SAMLResponse": form["SAMLResponse"], "RelayState": idp.login(t, authURL3)["RelayState"]}
				refused(t, acs(t, stolen, ck3), "auth_failed")
			})
			t.Run("response posted from another browser is refused", func(t *testing.T) {
				authURL4, _ := start(t)
				_, other := start(t)
				refused(t, acs(t, idp.login(t, authURL4), other), "invalid_state")
			})
			t.Run("unsolicited response is refused", func(t *testing.T) {
				refused(t, acs(t, idp.unsolicited(t, "", ""), nil), "missing_state")
				authURL5, ck5 := start(t)
				relay := idp.login(t, authURL5)["RelayState"][0]
				refused(t, acs(t, idp.unsolicited(t, "", relay), ck5), "auth_failed")
			})
			t.Run("response to an unknown request ID is refused", func(t *testing.T) {
				authURL6, ck6 := start(t)
				relay := idp.login(t, authURL6)["RelayState"][0]
				refused(t, acs(t, idp.unsolicited(t, "id-not-issued-by-goatflow", relay), ck6), "auth_failed")
			})
		})
	}
}
