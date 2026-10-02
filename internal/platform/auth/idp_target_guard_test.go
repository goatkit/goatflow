package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/webhook"
)

const guardTestMetadata = `<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://idp.internal.test"><IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol"/></EntityDescriptor>`

// SAML metadata is fetched from an admin-supplied URL on unauthenticated
// requests: internal addresses are refused before any connection is made,
// unless the operator allows them.
func TestFetchIdPMetadataRefusesInternalTargets(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(guardTestMetadata))
	}))
	defer srv.Close()

	t.Setenv(webhook.AllowPrivateTargetsEnv, "")
	_, err := fetchIdPMetadata(srv.URL + "/metadata")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loopback")
	assert.Equal(t, int32(0), hits.Load(), "the internal endpoint must not be called")

	t.Setenv(webhook.AllowPrivateTargetsEnv, "true")
	ed, err := fetchIdPMetadata(srv.URL + "/metadata")
	require.NoError(t, err)
	assert.Equal(t, "https://idp.internal.test", ed.EntityID)
	assert.Equal(t, int32(1), hits.Load())
}

// OIDC discovery (and the token/JWKS endpoints it names) go through the same
// guard when the provider is built without an injected client.
func TestOIDCStartAuthFlowRefusesInternalDiscovery(t *testing.T) {
	var hits atomic.Int32
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 srv.URL,
			"authorization_endpoint": srv.URL + "/authorize",
			"token_endpoint":         srv.URL + "/token",
			"jwks_uri":               srv.URL + "/jwks",
		})
	}))
	defer srv.Close()

	prov := NewOidcProvider(&OidcConfig{DiscoveryURL: srv.URL, ClientID: "c", RedirectURL: "https://sp.example.test/cb"},
		ProviderDependencies{StateStore: NewMemoryStateStore()})

	t.Setenv(webhook.AllowPrivateTargetsEnv, "")
	_, err := prov.StartAuthFlow(context.Background(), "state", "verifier")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loopback")
	assert.Equal(t, int32(0), hits.Load(), "the internal endpoint must not be called")

	t.Setenv(webhook.AllowPrivateTargetsEnv, "true")
	authURL, err := prov.StartAuthFlow(context.Background(), "state", "verifier")
	require.NoError(t, err)
	assert.Contains(t, authURL, srv.URL+"/authorize?")
	assert.Equal(t, int32(1), hits.Load())
}
