package api

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/constants"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/service"
	"github.com/gotrs-io/gotrs-ce/internal/shared"
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
	user, accessToken, refreshToken, err := authService.Login(ctx, username, password)
	if err != nil {
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

	// Get user's preferred session timeout
	sessionTimeout := shared.GetSystemSessionMaxTime()
	if db, err := database.GetDB(); err == nil && db != nil {
		prefService := service.NewUserPreferencesService(db)
		if userTimeout := prefService.GetSessionTimeout(int(user.ID)); userTimeout > 0 {
			sessionTimeout = shared.ResolveSessionTimeout(userTimeout)
		}
	}
	if sessionTimeout <= 0 {
		sessionTimeout = constants.DefaultSessionTimeout
	}

	// Set cookies for tokens - set both names for compatibility across middlewares
	c.SetCookie(
		"auth_token", // AuthMiddleware looks for this name
		accessToken,
		sessionTimeout,
		"/",
		"",
		false, // Not HTTPS in dev
		true,  // HttpOnly
	)

	// Also set access_token for components expecting this name
	c.SetCookie(
		"access_token",
		accessToken,
		sessionTimeout,
		"/",
		"",
		false,
		true,
	)

	c.SetCookie(
		"refresh_token",
		refreshToken,
		constants.RefreshTokenTimeout, // 7 days
		"/",
		"",
		false,
		true,
	)

	// Store user in session (use "user_id" to match middleware)
	c.Set("user", user)
	c.Set("user_id", user.ID)
	if provider != "" {
		c.Set("auth_provider", provider)
	}

	// Create session record in database for admin session management
	log.Printf("DEBUG: HandleAuthLogin - attempting to create session for user %s (ID: %d)", username, user.ID)
	sessionSvc := shared.GetSessionService()
	if sessionSvc == nil {
		log.Printf("DEBUG: HandleAuthLogin - session service is nil!")
	} else {
		sessionID, err := sessionSvc.CreateSession(
			int(user.ID),
			username,
			user.Role,
			c.ClientIP(),
			c.Request.UserAgent(),
		)
		if err != nil {
			// Log error but don't fail login - session tracking is non-critical
			log.Printf("Failed to create session record: %v", err)
		} else {
			log.Printf("DEBUG: HandleAuthLogin - session created: %s", sessionID)
			// Store session ID in a cookie for logout cleanup
			c.SetCookie("session_id", sessionID, sessionTimeout, "/", "", false, true)
		}
	}

	redirectTarget := "/dashboard"
	if strings.EqualFold(user.Role, "customer") {
		redirectTarget = "/customer/tickets"
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

// HandleAuthLogout handles user logout.
var HandleAuthLogout = func(c *gin.Context) {
	// Delete session record from database
	if sessionID, err := c.Cookie("session_id"); err == nil && sessionID != "" {
		if sessionSvc := shared.GetSessionService(); sessionSvc != nil {
			if err := sessionSvc.KillSession(sessionID); err != nil {
				log.Printf("Failed to delete session record: %v", err)
			}
		}
	}

	// Clear cookies
	c.SetCookie("auth_token", "", -1, "/", "", false, true)
	c.SetCookie("access_token", "", -1, "/", "", false, true)
	c.SetCookie("refresh_token", "", -1, "/", "", false, true)
	c.SetCookie("session_id", "", -1, "/", "", false, true)

	// Redirect to login
	c.Redirect(http.StatusSeeOther, "/login")
}

// HandleAuthCheck checks if user is authenticated.
var HandleAuthCheck = func(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"authenticated": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"authenticated": true,
		"userID":        userID,
	})
}

func handleAuthRefresh(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"success": false,
		"error":   "token refresh not implemented",
	})
}

func handleAuthRegister(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"success": false,
		"error":   "self-service registration disabled",
	})
}

// getEnvDefault returns environment variable value or fallback default.
func getEnvDefault(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}
