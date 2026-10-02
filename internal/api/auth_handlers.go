package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/constants"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/httpcookie"
	"github.com/goatkit/goatflow/internal/platform/service"
	"github.com/goatkit/goatflow/internal/platform/shared"
)

// HandleAuthLogin handles the login form submission.
var HandleAuthLogin = func(c *gin.Context) {
	contentType := c.GetHeader("Content-Type")
	var username, password string
	if strings.Contains(contentType, "application/json") {
		var payload struct {
			Login    string `json:"login"`
			Username string `json:"username"`
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&payload); err == nil {
			username = payload.Login
			if username == "" {
				username = payload.Username
			}
			if username == "" {
				username = payload.Email
			}
			password = payload.Password
		}
	} else {
		username = c.PostForm("username")
		password = c.PostForm("password")
		if username == "" {
			username = c.PostForm("login")
		}
		if username == "" {
			username = c.PostForm("email")
		}
		if username == "" {
			username = c.PostForm("user")
		}
	}
	provider := c.PostForm("provider")
	if provider == "" {
		provider = c.Query("provider")
	}
	provider = strings.ToLower(provider)

	if username == "" || password == "" {
		if strings.Contains(contentType, "application/json") {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "username and password required"})
		} else {
			getPongo2Renderer().HTML(c, http.StatusBadRequest, "components/error.pongo2",
				pongo2.Context{"error": "Username and password are required"})
		}
		return
	}

	// Fail2ban-style throttle shared with /api/v1/auth/login and customer login.
	clientIP := c.ClientIP()
	if blocked, remaining := auth.DefaultLoginRateLimiter.IsBlocked(clientIP, username); blocked {
		switch {
		case strings.Contains(contentType, "application/json"):
			tooManyAttemptsJSON(c, remaining)
		case c.GetHeader("HX-Request") == "true":
			html := `<div class="rounded-md bg-red-50 dark:bg-red-900/20 p-4 mt-4">` +
				`<div class="text-sm text-red-800 dark:text-red-200">` +
				`Too many failed attempts. Try again later.</div></div>`
			c.Data(http.StatusTooManyRequests, "text/html; charset=utf-8", []byte(html))
		default:
			c.Redirect(http.StatusSeeOther, "/login?error=Too+many+failed+attempts.+Try+again+later")
		}
		return
	}

	// Get auth service
	authService := GetAuthService()
	if authService == nil {
		if strings.Contains(contentType, "application/json") {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "authentication unavailable"})
			return
		}
		if c.GetHeader("HX-Request") == "true" {
			html := `<div class="rounded-md bg-yellow-50 dark:bg-yellow-900/20 p-4 mt-4">` +
				`<div class="text-sm text-yellow-800 dark:text-yellow-100">` +
				`Authentication temporarily unavailable</div></div>`
			c.Data(http.StatusUnauthorized, "text/html; charset=utf-8", []byte(html))
			return
		}
		c.Redirect(http.StatusSeeOther, "/login?error=Authentication+temporarily+unavailable")
		return
	}

	// Authenticate user
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use the real auth service for production-grade authentication
	// NOTE: provider ordering currently controlled via config Auth::Providers.
	// Explicit provider field is advisory; future: route to single-provider auth path.
	user, err := authService.Login(ctx, username, password)
	if err != nil {
		auth.DefaultLoginRateLimiter.RecordFailure(clientIP, username)
		if strings.Contains(contentType, "application/json") {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "invalid credentials"})
			return
		}
		if c.GetHeader("HX-Request") == "true" {
			html := `<div class="rounded-md bg-red-50 dark:bg-red-900/20 p-4 mt-4">` +
				`<div class="text-sm text-red-800 dark:text-red-200">` +
				`Invalid username or password</div></div>`
			c.Data(http.StatusUnauthorized, "text/html; charset=utf-8", []byte(html))
		} else {
			c.Redirect(http.StatusSeeOther, "/login?error=Invalid+username+or+password")
		}
		return
	}
	auth.DefaultLoginRateLimiter.RecordSuccess(clientIP, username)

	// Check if 2FA is enabled for this user. A failed lookup aborts the
	// login: skipping the second factor on an error would bypass it.
	mfaDB, mfaErr := database.GetDB()
	var mfa mfaStatus
	if mfaErr == nil {
		mfa, mfaErr = agentMFAStatus(mfaDB, int(user.ID))
	}
	if mfaErr != nil {
		log.Printf("login: second-factor status for user %d unavailable: %v", user.ID, mfaErr)
		if strings.Contains(contentType, "application/json") {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Login temporarily unavailable"})
		} else {
			c.Redirect(http.StatusSeeOther, "/login?error=server_error")
		}
		return
	}
	if mfa.Enabled() {
		// 2FA is enabled - don't complete login yet
		sessionMgr := auth.GetTOTPSessionManager()
		token, err := sessionMgr.CreateAgentSession(int(user.ID), username, c.ClientIP(), c.Request.UserAgent())
		if err != nil {
			if strings.Contains(contentType, "application/json") {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create 2FA session"})
			} else {
				c.Redirect(http.StatusSeeOther, "/login?error=2FA+session+error")
			}
			return
		}

		// Store token in cookie - user data is server-side
		httpcookie.SetAuth(c, "2fa_pending", token, 300) // 5 min expiry

		if c.GetHeader("HX-Request") == "true" {
			// HTMX boosted form - use 302 redirect (hx-boost follows standard redirects)
			c.Redirect(http.StatusFound, "/login/2fa")
		} else if strings.Contains(contentType, "application/json") {
			c.JSON(http.StatusOK, gin.H{
				"success":      true,
				"requires_2fa": true,
				"redirect":     "/login/2fa",
			})
		} else {
			c.Redirect(http.StatusFound, "/login/2fa")
		}
		return
	}

	// Get user's preferred session timeout and regenerate JWT with that duration.
	sessionTimeout := shared.GetSystemSessionMaxTime()
	if db, err := database.GetDB(); err == nil && db != nil {
		prefService := service.NewUserPreferencesService(db)
		if userTimeout := prefService.GetSessionTimeout(int(user.ID)); userTimeout > 0 {
			sessionTimeout = shared.ResolveSessionTimeout(userTimeout)
		}

		// Persist pre-login language selection to user preferences
		if preLoginLang, err := c.Cookie("goatflow_lang"); err == nil && preLoginLang != "" {
			if setErr := prefService.SetLanguage(int(user.ID), preLoginLang); setErr != nil {
				log.Printf("Failed to save language preference: %v", setErr)
			}
		}
	}
	if sessionTimeout <= 0 {
		sessionTimeout = constants.DefaultSessionTimeout
	}

	// The session row is what makes the tokens revocable (admin "kill
	// session", logout); the JWT expires with the cookie, at the user's
	// session duration rather than the system default.
	sessionID, accessToken, refreshToken, err := authService.IssueTokens(user, c.ClientIP(), c.Request.UserAgent(),
		time.Duration(sessionTimeout)*time.Second)
	if err != nil {
		log.Printf("login: issue tokens for %s: %v", username, err)
		if strings.Contains(contentType, "application/json") {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Login temporarily unavailable"})
		} else {
			c.Redirect(http.StatusSeeOther, "/login?error=server_error")
		}
		return
	}

	// Set cookies for tokens - set both names for compatibility across middlewares.
	httpcookie.SetAuth(c, "auth_token", accessToken, sessionTimeout)
	httpcookie.SetAuth(c, "access_token", accessToken, sessionTimeout)
	httpcookie.SetAuth(c, "refresh_token", refreshToken, constants.RefreshTokenTimeout)
	// Session ID cookie for logout cleanup
	httpcookie.SetAuth(c, "session_id", sessionID, sessionTimeout)

	// Store user in session (use "user_id" to match middleware)
	c.Set("user", user)
	c.Set("user_id", user.ID)
	if provider != "" {
		c.Set("auth_provider", provider)
	}

	// Set a non-httpOnly indicator so JavaScript can detect authentication
	// (auth tokens are httpOnly for security, but JS needs to know user is logged in)
	httpcookie.SetAuthState(c, "goatflow_logged_in", "1", sessionTimeout)

	redirectTarget := "/dashboard"
	if strings.EqualFold(user.Role, "customer") {
		redirectTarget = "/customer"
	}

	// Check if a plugin has set a custom landing page
	if lp := shared.GetLandingPage(); lp != "" && redirectTarget == "/dashboard" {
		redirectTarget = lp
	}

	if strings.Contains(contentType, "application/json") {
		c.JSON(http.StatusOK, gin.H{
			"success": true, "access_token": accessToken, "refresh_token": refreshToken,
			"token_type": "Bearer", "redirect": redirectTarget,
		})
		return
	}
	if c.GetHeader("HX-Request") == "true" {
		c.Header("HX-Redirect", redirectTarget)
		c.String(http.StatusOK, "Login successful, redirecting...")
		return
	}
	c.Redirect(http.StatusSeeOther, redirectTarget)
}

// getEnvDefault returns environment variable value or fallback default.
func getEnvDefault(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

// newLoginSession creates the sessions row for a login that completed outside
// AuthService.IssueTokens (second factor, passkey, SSO, customer portal) and
// returns its id for the access token's sid claim. Access tokens are only valid
// while their session row exists, so a failure here must fail the login.
func newLoginSession(c *gin.Context, userID int, login, userType string) (string, error) {
	sessionSvc := shared.GetSessionService()
	if sessionSvc == nil {
		return "", errors.New("session service unavailable")
	}
	return sessionSvc.CreateSession(userID, login, userType, c.ClientIP(), c.Request.UserAgent())
}
