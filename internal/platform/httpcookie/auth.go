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

// SetIdPReturn stores a sensitive cookie that must come back on the request
// an identity provider sends the browser to after login, whatever
// session.same_site is configured: SameSite=Lax for a redirect (OIDC
// callback, a top-level GET). A SAML assertion arrives as a cross-site POST,
// which Lax cookies are not sent on, so post=true uses SameSite=None when the
// cookie is Secure; over plain HTTP, where browsers refuse SameSite=None, the
// attribute is left unset and the browser default applies.
func SetIdPReturn(c *gin.Context, name, value string, maxAge int, post bool) {
	if c == nil || c.Writer == nil {
		return
	}
	secure := secureCookieRequired()
	sameSite := http.SameSiteLaxMode
	if post {
		sameSite = http.SameSiteDefaultMode
		if secure {
			sameSite = http.SameSiteNoneMode
		}
	}
	// #nosec G124 -- Secure follows production/session.secure config (plain-HTTP dev must work)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   secure,
		HttpOnly: true,
		SameSite: sameSite,
	})
}

func set(c *gin.Context, name, value string, maxAge int, httpOnly bool) {
	if c == nil || c.Writer == nil {
		return
	}
	// #nosec G124 -- Secure follows production/session.secure config (plain-HTTP dev must work); HttpOnly is false only for the documented JS-readable auth state cookies
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
