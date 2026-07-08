package auth

import (
	"testing"

	"github.com/goatkit/goatflow/internal/platform/models"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

// TestPKCEGeneration verifies that PKCE code verifier and challenge are generated correctly
func TestPKCEGeneration(t *testing.T) {
	t.Parallel()
	// Generate code verifier
	codeVerifier := oauth2.GenerateVerifier()
	assert.NotEmpty(t, codeVerifier, "code verifier should not be empty")
	assert.GreaterOrEqual(t, len(codeVerifier), 43, "code verifier should be at least 43 characters")

	// Generate S256 challenge from verifier
	codeChallenge := oauth2.S256ChallengeFromVerifier(codeVerifier)
	assert.NotEmpty(t, codeChallenge, "code challenge should not be empty")
	assert.Len(t, codeChallenge, 43, "S256 challenge should be exactly 43 characters (base64url-encoded SHA256)")
 }

// TestPKCEChallengeConsistency verifies that generating the same verifier produces the same challenge
func TestPKCEChallengeConsistency(t *testing.T) {
	t.Parallel()
	codeVerifier := oauth2.GenerateVerifier()

	// Generate challenge twice
	challenge1 := oauth2.S256ChallengeFromVerifier(codeVerifier)
	challenge2 := oauth2.S256ChallengeFromVerifier(codeVerifier)

	assert.Equal(t, challenge1, challenge2, "same verifier should produce same challenge")
 }

// TestPKCEMultipleVerifiers verifies that different verifiers produce different challenges
func TestPKCEMultipleVerifiers(t *testing.T) {
	t.Parallel()
	verifier1 := oauth2.GenerateVerifier()
	verifier2 := oauth2.GenerateVerifier()

	challenge1 := oauth2.S256ChallengeFromVerifier(verifier1)
	challenge2 := oauth2.S256ChallengeFromVerifier(verifier2)

	assert.NotEqual(t, verifier1, verifier2, "different calls should produce different verifiers")
	assert.NotEqual(t, challenge1, challenge2, "different verifiers should produce different challenges")
 }

// TestOidcProvider_Name verifies the provider name
func TestOidcProvider_Name(t *testing.T) {
	t.Parallel()
	stateStore := NewMemoryStateStore()
	provider := NewOidcProvider(&OidcConfig{
		DiscoveryURL: "https://example.com/.well-known/openid-configuration",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scopes:       "openid email profile",
	}, ProviderDependencies{
		StateStore: stateStore,
		UserRepo:   &mockUserRepo{users: make(map[string]*models.User)},
	})

	assert.Equal(t, "oidc", provider.Name())
 }

// TestOidcProvider_Priority verifies the provider priority
func TestOidcProvider_Priority(t *testing.T) {
	t.Parallel()
	stateStore := NewMemoryStateStore()
	provider := NewOidcProvider(&OidcConfig{
		DiscoveryURL: "https://example.com/.well-known/openid-configuration",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scopes:       "openid email profile",
	}, ProviderDependencies{
		StateStore: stateStore,
		UserRepo:   &mockUserRepo{users: make(map[string]*models.User)},
	})

	assert.Equal(t, 3, provider.Priority())
 }

// TestOidcProvider_AuthenticateReturnsError verifies that password authentication returns error
func TestOidcProvider_AuthenticateReturnsError(t *testing.T) {
	t.Parallel()
	stateStore := NewMemoryStateStore()
	provider := NewOidcProvider(&OidcConfig{
		DiscoveryURL: "https://example.com/.well-known/openid-configuration",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scopes:       "openid email profile",
	}, ProviderDependencies{
		StateStore: stateStore,
		UserRepo:   &mockUserRepo{users: make(map[string]*models.User)},
	})

	_, err := provider.Authenticate(nil, "user", "pass")
	assert.Error(t, err)
	assert.Equal(t, ErrAuthBackendFailed, err)
 }
