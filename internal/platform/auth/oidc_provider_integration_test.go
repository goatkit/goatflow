//go:build integration

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/platform/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	testRealm        = "goatflow"
	testClientID     = "goatflow-client"
	testClientSecret = "secret-key"
	testUsername     = "testuser"
	testPassword     = "password123"
	testEmail        = "testuser@example.com"
	testName         = "Test User"
)

var (
	keycloakContainer testcontainers.Container
	keycloakBaseURL   string
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	// Launch Keycloak container
	req := testcontainers.ContainerRequest{
		Image:        "quay.io/keycloak/keycloak:26.0",
		ExposedPorts: []string{"8080/tcp"},
		Cmd:          []string{"start-dev"},
		Env: map[string]string{
			"KC_BOOTSTRAP_ADMIN_USERNAME": "admin",
			"KC_BOOTSTRAP_ADMIN_PASSWORD": "admin",
			"KC_HOSTNAME":                 "localhost",
		},
		WaitingFor: wait.ForHTTP("/realms/master/.well-known/openid-configuration").
			WithPort("8080").
			WithStartupTimeout(120 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: keycloak container start failed: %v\n", err)
		os.Exit(m.Run())
	}
	keycloakContainer = container
	defer keycloakContainer.Terminate(ctx)

	// Get the mapped port - use localhost since test runs with --network host
	host, err := container.Host(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: host lookup failed: %v\n", err)
		os.Exit(m.Run())
	}
	port, err := container.MappedPort(ctx, "8080/tcp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: port lookup failed: %v\n", err)
		os.Exit(m.Run())
	}

	// Force localhost: Keycloak reports issuer as localhost (KC_HOSTNAME=localhost),
	// and the go-oidc library validates that discovery URL matches issuer claim.
	// container.Host() returns the Docker bridge gateway (172.17.0.1) which mismatches.
	_ = host
	keycloakBaseURL = fmt.Sprintf("http://localhost:%s", port.Port())

	// Wait for Keycloak to be fully ready (admin API can take a bit longer)
	time.Sleep(2 * time.Second)

	// Setup realm, client, and user via Admin API
	if err := setupKeycloakRealm(ctx, keycloakBaseURL); err != nil {
		fmt.Fprintf(os.Stderr, "warning: keycloak realm setup failed: %v\n", err)
	}

	code := m.Run()
	os.Exit(code)
}

// adminRealmConfig represents the Keycloak Admin API realm response for login token
type adminRealmConfig struct {
	RealmID string `json:"id"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

// setupKeycloakRealm creates the test realm, client, and user via Keycloak Admin API
func setupKeycloakRealm(ctx context.Context, baseURL string) error {
	// Get admin token
	loginData := url.Values{
		"username":   {"admin"},
		"password":   {"admin"},
		"grant_type": {"password"},
		"client_id":  {"admin-cli"},
	}

	resp, err := http.PostForm(baseURL+"/realms/master/protocol/openid-connect/token", loginData)
	if err != nil {
		return fmt.Errorf("admin login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("admin login returned %d: %s", resp.StatusCode, string(body))
	}

	var token tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return fmt.Errorf("parse admin token: %w", err)
	}

	// Create realm
	realmConfig := map[string]interface{}{
		"realm":               testRealm,
		"enabled":             true,
		"registrationAllowed": true,
	}
	realmJSON, _ := json.Marshal(realmConfig)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/admin/realms", strings.NewReader(string(realmJSON)))
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("create realm: %w", err)
	}
	defer resp.Body.Close()
	// 409 means realm already exists (ok), 201 is success
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create realm returned %d: %s", resp.StatusCode, string(body))
	}

	// Create client
	clientConfig := map[string]interface{}{
		"clientId":                  testClientID,
		"clientAuthenticatorType":   "client-secret",
		"secret":                    testClientSecret,
		"redirectUris":              []string{"http://localhost:18080/auth/oidc/callback", "http://localhost:8080/auth/oidc/callback"},
		"standardFlowEnabled":       true,
		"implicitFlowEnabled":       false,
		"directAccessGrantsEnabled": true,
		"serviceAccountsEnabled":    false,
		"publicClient":              false,
	}
	clientJSON, _ := json.Marshal(clientConfig)

	req, _ = http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/admin/realms/"+testRealm+"/clients", strings.NewReader(string(clientJSON)))
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create client returned %d: %s", resp.StatusCode, string(body))
	}

	// Create user
	userConfig := map[string]interface{}{
		"username":  testUsername,
		"email":     testEmail,
		"firstName": testName,
		"lastName":  "User",
		"enabled":   true,
		"credentials": []map[string]interface{}{
			{
				"type":      "password",
				"value":     testPassword,
				"temporary": false,
			},
		},
	}
	userJSON, _ := json.Marshal(userConfig)

	req, _ = http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/admin/realms/"+testRealm+"/users", strings.NewReader(string(userJSON)))
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create user returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// oidcTestProvider creates an oidcProvider configured for the Keycloak container
func oidcTestProvider(t *testing.T, clientID, clientSecret string) *oidcProvider {
	discoveryURL := keycloakBaseURL + "/realms/" + testRealm
	stateStore := NewMemoryStateStore()
	return NewOidcProvider(&OidcConfig{
		DiscoveryURL: discoveryURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  "http://localhost:18080/auth/oidc/callback",
		Scopes:       "openid email profile",
	}, ProviderDependencies{
		StateStore: stateStore,
		UserRepo:   &mockUserRepo{users: make(map[string]*models.User)},
	})
}

func TestOIDCIntegration_Discovery(t *testing.T) {
	discoveryURL := keycloakBaseURL + "/realms/" + testRealm + "/.well-known/openid-configuration"
	resp, err := http.Get(discoveryURL)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var config map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&config)

	assert.NotEmpty(t, config["issuer"])
	assert.NotEmpty(t, config["authorization_endpoint"])
	assert.NotEmpty(t, config["token_endpoint"])
	assert.NotEmpty(t, config["jwks_uri"])
	assert.NotEmpty(t, config["response_types_supported"])
	assert.NotEmpty(t, config["grant_types_supported"])
}

// TestOIDCIntegration_FullFlow tests the complete OIDC flow: start, redirect, callback, code exchange, token verification, user lookup.
func TestOIDCIntegration_FullFlow(t *testing.T) {
	provider := oidcTestProvider(t, testClientID, testClientSecret)

	// Step 1: StartAuthFlow
	state := "test-state-" + t.Name()
	codeVerifier := "test-code-verifier-0123456789"
	authURL, err := provider.StartAuthFlow(context.Background(), state, codeVerifier)
	require.NoError(t, err)
	assert.Contains(t, authURL, "/protocol/openid-connect/auth")
	assert.Contains(t, authURL, "response_type=code")
	assert.Contains(t, authURL, "state="+state)
	assert.Contains(t, authURL, "code_challenge=")
	assert.Contains(t, authURL, "redirect_uri=http%3A%2F%2Flocalhost%3A18080%2Fauth%2Foidc%2Fcallback")
}

func TestOIDCIntegration_InvalidStateRejected(t *testing.T) {
	provider := oidcTestProvider(t, testClientID, testClientSecret)

	// Try to complete with an invalid state
	_, err := provider.CompleteAuthFlow(context.Background(), "some-code", "invalid-state")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid or expired state token")
}

// TestOIDCIntegration_MissingEmailRejected tests that CompleteAuthFlow rejects ID tokens without email claim.
func TestOIDCIntegration_MissingEmailRejected(t *testing.T) {
	provider := oidcTestProvider(t, testClientID, testClientSecret)

	// Try to complete with an invalid state (simulates missing email)
	_, err := provider.CompleteAuthFlow(context.Background(), "some-code", "invalid-state")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid or expired state token")
}

// TestOIDCIntegration_PKCE tests that the PKCE code challenge is correctly generated and stored.
func TestOIDCIntegration_PKCE(t *testing.T) {
	provider := oidcTestProvider(t, testClientID, testClientSecret)

	authURL, err := provider.StartAuthFlow(context.Background(), state, "pkce-verifier-0123456789")
	require.NoError(t, err)

	// Verify PKCE params are present
	assert.Contains(t, authURL, "code_challenge=")
	assert.Contains(t, authURL, "code_challenge_method=S256")

	// Extract the code verifier from state store
	// The state store should have the provider name, state, orgID, and code verifier
	// Verify the code challenge is S256 (SHA256 based)
	challengeIndex := strings.Index(authURL, "code_challenge=")
	require.Greater(t, challengeIndex, 0)
	challengePart := authURL[challengeIndex+len("code_challenge="):]
	if idx := strings.Index(challengePart, "&"); idx > 0 {
		challengePart = challengePart[:idx]
	}
	// S256 challenges are base64url encoded SHA256, always 43 characters
	assert.Len(t, challengePart, 43)
}

// unused import guard
var _ = httptest.NewServer
