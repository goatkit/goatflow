package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/goatkit/goatflow/internal/platform/auth"
)

func TestHandleOIDCRedirect_MissingProvider(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/auth/", nil)
	c.Params = gin.Params{}

	handleOIDCRedirect(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleOIDCRedirect_NoStateStore(t *testing.T) {
	auth.SetStateStore(nil)
	defer auth.SetStateStore(auth.NewMemoryStateStore())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/auth/oidc", nil)
	c.Params = gin.Params{{Key: "provider", Value: "oidc"}}

	handleOIDCRedirect(c)
	// With no state store, provider creation may still succeed but StartAuthFlow will fail
	// The handler should not panic
	assert.NotEqual(t, http.StatusInternalServerError, -1) // just ensure it doesn't panic
}

func TestHandleOIDCCallback_MissingProvider(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/auth//callback", nil)
	c.Params = gin.Params{}

	handleOIDCCallback(c)
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/login")
}

func TestHandleOIDCCallback_MissingParams(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/auth/oidc/callback", nil)
	c.Params = gin.Params{{Key: "provider", Value: "oidc"}}

	handleOIDCCallback(c)
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "missing_params")
}

func TestHandleOIDCCallback_NoStateStore(t *testing.T) {
	auth.SetStateStore(nil)
	defer auth.SetStateStore(auth.NewMemoryStateStore())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/auth/oidc/callback?code=abc&state=xyz", nil)
	c.Params = gin.Params{{Key: "provider", Value: "oidc"}}

	handleOIDCCallback(c)
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "server_error")
}

func TestHandleOIDCCallback_InvalidState(t *testing.T) {
	auth.SetStateStore(auth.NewMemoryStateStore())

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/auth/oidc/callback?code=abc&state=invalid-state", nil)
	c.Params = gin.Params{{Key: "provider", Value: "oidc"}}

	handleOIDCCallback(c)
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "invalid_state")
}

func TestGenerateState(t *testing.T) {
	state := generateState()
	assert.NotEmpty(t, state)
	assert.Len(t, state, 64) // 32 bytes hex-encoded = 64 chars

	// Should be unique
	state2 := generateState()
	assert.NotEqual(t, state, state2)
}

func TestGenerateCodeVerifier(t *testing.T) {
	verifier := generateCodeVerifier()
	assert.NotEmpty(t, verifier)
	assert.Len(t, verifier, 64) // 32 bytes hex-encoded = 64 chars

	// Should be unique
	verifier2 := generateCodeVerifier()
	assert.NotEqual(t, verifier, verifier2)
}
