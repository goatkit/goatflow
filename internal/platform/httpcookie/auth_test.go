package httpcookie

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetAuthCookiePolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "production")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	SetAuth(c, "auth_token", "secret", 300)

	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.Equal(t, "auth_token", cookie.Name)
	assert.Equal(t, "secret", cookie.Value)
	assert.Equal(t, "/", cookie.Path)
	assert.True(t, cookie.Secure)
	assert.True(t, cookie.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}

func TestSetAuthStateCookieIsReadableButSecureInProduction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "production")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	SetAuthState(c, "goatflow_logged_in", "1", 300)

	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.Equal(t, "goatflow_logged_in", cookie.Name)
	assert.True(t, cookie.Secure)
	assert.False(t, cookie.HttpOnly)
}

func TestSetAuthCookieAllowsInsecureLocalDevelopment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "development")
	t.Setenv("GOATFLOW_AUTH_SESSION_SECURE", "")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	SetAuth(c, "auth_token", "secret", 300)

	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.False(t, cookies[0].Secure)
}

func TestSetAuthCookieHonorsSecureEnvOutsideProduction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "development")
	t.Setenv("GOATFLOW_AUTH_SESSION_SECURE", "true")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	SetAuth(c, "auth_token", "secret", 300)

	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.True(t, cookies[0].Secure)
}
