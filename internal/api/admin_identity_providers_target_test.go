package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/webhook"
)

// The discovery/metadata URL of a provider is fetched by the server on every
// unauthenticated /auth/:id request, so an admin must not be able to point
// it at loopback, link-local (cloud metadata) or other internal addresses.
func TestIdentityProviderURLRefusesInternalTargets(t *testing.T) {
	setupTemplateRenderer(t)
	db := getTestDB(t)
	token := GetTestAuthToken(t)
	setDB(db)
	defer restoreDB()
	router := NewSimpleRouterWithDB(db)
	send := func(method, path string, form url.Values) (int, map[string]any) {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		var body map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		return w.Code, body
	}
	post := func(path string, form url.Values) (int, map[string]any) { return send(http.MethodPost, path, form) }
	form := func(providerType, discoveryURL string) url.Values {
		f := url.Values{
			"name":          {fmt.Sprintf("Test%s_%d", providerType, time.Now().UnixNano())},
			"provider_type": {providerType},
			"client_id":     {"client"},
			"discovery_url": {discoveryURL},
			"enabled":       {"1"},
		}
		if providerType == "saml2" {
			f.Set("signing_cert", "-----BEGIN CERTIFICATE-----\nMIIBtest\n-----END CERTIFICATE-----")
			f.Set("private_key", "-----BEGIN PRIVATE KEY-----\nMIIBtestkey\n-----END PRIVATE KEY-----")
		}
		return f
	}
	internal := []string{
		"http://127.0.0.1/.well-known/openid-configuration",
		"http://169.254.169.254/latest/meta-data/",
		"http://localhost:8080/health",
		"http://[::1]/metadata",
		"http://10.0.0.5/metadata",
	}

	t.Setenv(webhook.AllowPrivateTargetsEnv, "")
	for _, providerType := range []string{"oidc", "saml2"} {
		for _, u := range internal {
			code, body := post("/admin/identity-providers", form(providerType, u))
			assert.Equal(t, http.StatusBadRequest, code, "%s %s: %v", providerType, u, body)
			assert.Contains(t, fmt.Sprint(body["error"]), webhook.AllowPrivateTargetsEnv, "%s %s", providerType, u)
		}
	}
	for _, u := range []string{"ftp://idp.example.com/metadata", "idp.example.com/metadata", "/metadata"} {
		code, body := post("/admin/identity-providers", form("oidc", u))
		assert.Equal(t, http.StatusBadRequest, code, "%s: %v", u, body)
	}

	// A public host is accepted; updating it to an internal one is refused
	// and leaves the stored URL unchanged.
	code, body := post("/admin/identity-providers", form("oidc", "https://idp.example.com/.well-known/openid-configuration"))
	require.Equal(t, http.StatusOK, code, "%v", body)
	id := uint(body["id"].(float64))
	defer cleanupProvider(db, id)
	update := form("oidc", "http://127.0.0.1/.well-known/openid-configuration")
	code, body = send(http.MethodPut, "/admin/identity-providers/"+strconv.FormatUint(uint64(id), 10), update)
	assert.Equal(t, http.StatusBadRequest, code, "%v", body)
	var stored string
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT discovery_url FROM gk_identity_provider WHERE id = ?`), id).Scan(&stored))
	assert.Equal(t, "https://idp.example.com/.well-known/openid-configuration", stored)

	// The operator escape hatch allows internal IdPs (dev stacks).
	t.Setenv(webhook.AllowPrivateTargetsEnv, "true")
	code, body = post("/admin/identity-providers", form("oidc", "http://127.0.0.1:8081/.well-known/openid-configuration"))
	require.Equal(t, http.StatusOK, code, "%v", body)
	cleanupProvider(db, uint(body["id"].(float64)))
}
