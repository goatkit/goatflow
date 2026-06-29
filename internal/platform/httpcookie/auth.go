package httpcookie

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/config"
)

// SetAuth stores a sensitive authentication cookie.
func SetAuth(c *gin.Context, name, value string, maxAge int) {
	set(c, name, value, maxAge, true)
}

// ClearAuth removes a sensitive authentication cookie.
func ClearAuth(c *gin.Context, name string) {
	SetAuth(c, name, "", -1)
}

// SetAuthState stores a non-sensitive auth state cookie that client-side UI
// code may read, such as "logged in" hints.
func SetAuthState(c *gin.Context, name, value string, maxAge int) {
	set(c, name, value, maxAge, false)
}

// ClearAuthState removes a non-sensitive auth state cookie.
func ClearAuthState(c *gin.Context, name string) {
	SetAuthState(c, name, "", -1)
}

func set(c *gin.Context, name, value string, maxAge int, httpOnly bool) {
	if c == nil || c.Writer == nil {
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   secureCookieRequired(),
		HttpOnly: httpOnly,
		SameSite: sameSiteMode(),
	})
}

func secureCookieRequired() bool {
	if isProductionEnv(os.Getenv("APP_ENV")) || isProductionEnv(os.Getenv("GOATFLOW_APP_ENV")) {
		return true
	}
	if cfg := config.Get(); cfg != nil {
		return cfg.App.IsProduction() || cfg.Auth.Session.Secure
	}
	return truthy(os.Getenv("GOATFLOW_AUTH_SESSION_SECURE"))
}

func sameSiteMode() http.SameSite {
	mode := "lax"
	if cfg := config.Get(); cfg != nil && cfg.Auth.Session.SameSite != "" {
		mode = cfg.Auth.Session.SameSite
	}
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "default":
		return http.SameSiteDefaultMode
	default:
		return http.SameSiteLaxMode
	}
}

func isProductionEnv(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "production", "prod":
		return true
	default:
		return false
	}
}

func truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
