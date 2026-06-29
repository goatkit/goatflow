package oauth2

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubClientRepo struct {
	clients []*Client
}

func (r *stubClientRepo) Create(client *Client) error { return nil }

func (r *stubClientRepo) GetByID(id string) (*Client, error) {
	for _, client := range r.clients {
		if client.ID == id {
			return client, nil
		}
	}
	return nil, errors.New("client not found")
}

func (r *stubClientRepo) GetByCredentials(id, secret string) (*Client, error) {
	client, err := r.GetByID(id)
	if err != nil {
		return nil, err
	}
	if client.Secret != secret {
		return nil, errors.New("invalid client secret")
	}
	return client, nil
}

func (r *stubClientRepo) List() ([]*Client, error) { return r.clients, nil }

func (r *stubClientRepo) Update(client *Client) error { return nil }

func (r *stubClientRepo) Delete(id string) error { return nil }

func TestSetupOAuth2RoutesFailsClosedWithoutSecurity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	provider := NewProvider(&stubClientRepo{}, nil, nil, nil, "https://issuer.example")
	router := gin.New()

	err := provider.SetupOAuth2Routes(router)
	require.ErrorIs(t, err, ErrOAuth2RouteSecurityRequired)

	for _, path := range []string{"/oauth2/jwks", "/admin/oauth2/clients"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	}
}

func TestSetupOAuth2RoutesWithSecurityProtectsAdminRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	provider := NewProvider(&stubClientRepo{
		clients: []*Client{{ID: "client-1", Name: "Test Client", IsActive: true}},
	}, nil, nil, nil, "https://issuer.example")
	router := gin.New()

	authMiddleware := func(c *gin.Context) {
		c.Set("user_id", uint(42))
		c.Set("user_email", "admin@example.com")
		c.Set("user_role", "Admin")
		c.Next()
	}
	adminMiddleware := func(c *gin.Context) {
		if c.GetHeader("X-Test-Admin") != "1" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin required"})
			return
		}
		c.Next()
	}

	err := provider.SetupOAuth2RoutesWithSecurity(router, RouteSecurity{
		AuthMiddleware:  authMiddleware,
		AdminMiddleware: adminMiddleware,
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/oauth2/clients", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/admin/oauth2/clients", nil)
	req.Header.Set("X-Test-Admin", "1")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "client-1")
}

func TestGetCurrentUserReadsAuthenticatedContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	provider := NewProvider(nil, nil, nil, nil, "https://issuer.example")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	_, _, _, authenticated := provider.getCurrentUser(c)
	assert.False(t, authenticated)

	c.Set("user_id", uint(42))
	c.Set("user_email", "agent@example.com")
	c.Set("user_role", "Agent")

	userID, email, role, authenticated := provider.getCurrentUser(c)
	assert.True(t, authenticated)
	assert.Equal(t, uint(42), userID)
	assert.Equal(t, "agent@example.com", email)
	assert.Equal(t, "Agent", role)
}

func TestValidatePKCE(t *testing.T) {
	provider := NewProvider(nil, nil, nil, nil, "https://issuer.example")
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	assert.True(t, provider.validatePKCE(verifier, "plain", verifier))
	assert.True(t, provider.validatePKCE(challenge, "S256", verifier))
	assert.False(t, provider.validatePKCE(challenge, "S256", "wrong-verifier"))
	assert.False(t, provider.validatePKCE(challenge, "unsupported", verifier))
	assert.False(t, provider.validatePKCE("", "S256", verifier))
}
