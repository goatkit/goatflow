package api

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/auth"
	"github.com/goatkit/goatflow/internal/constants"
	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/middleware"
	"github.com/goatkit/goatflow/internal/service"
	"github.com/goatkit/goatflow/internal/shared"
)

// HandleCustomerLogin is the exported handler for customer login POST requests.
var HandleCustomerLogin = func(c *gin.Context) {
	handleCustomerLogin(shared.GetJWTManager())(c)
}

func handleCustomerLogin(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var login, password string

		contentType := c.GetHeader("Content-Type")

		if strings.Contains(contentType, "application/json") {
			var payload struct {
				Login    string `json:"login"`
				Password string `json:"password"`
			}
			if err := c.ShouldBindJSON(&payload); err == nil {
				login = payload.Login
				password = payload.Password
			}
		} else {
			// Form data
			login = c.PostForm("login")
			if login == "" {
				login = c.PostForm("username")
			}
			password = c.PostForm("password")
		}

		login = strings.TrimSpace(login)
		password = strings.TrimSpace(password)
		if login == "" || password == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "login and password required"})
			return
		}

		// Server-side rate limiting (fail2ban style)
		clientIP := c.ClientIP()
		if blocked, remaining := auth.DefaultLoginRateLimiter.IsBlocked(clientIP, login); blocked {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success":         false,
				"error":           fmt.Sprintf("too many failed attempts, try again in %d seconds", int(remaining.Seconds())),
				"retry_after_sec": int(remaining.Seconds()),
			})
			return
		}

		if jwtManager == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "authentication not configured"})
			return
		}

		db, err := database.GetDB()
		if err != nil || db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "database unavailable"})
			return
		}

		provider, err := auth.CreateProvider("database", auth.ProviderDependencies{DB: db})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "auth provider unavailable"})
			return
		}

		authenticator := auth.NewAuthenticator(provider)
		user, err := authenticator.Authenticate(c.Request.Context(), login, password)
		if err != nil || user == nil || strings.ToLower(user.Role) != "customer" {
			auth.DefaultLoginRateLimiter.RecordFailure(clientIP, login)
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "invalid credentials"})
			return
		}

		// Clear rate limit on successful login
		auth.DefaultLoginRateLimiter.RecordSuccess(clientIP, login)

		// Check if 2FA is enabled for this customer
		totpService := service.NewTOTPService(db, "GoatFlow")
		if totpService.IsEnabledForCustomer(user.Login) {
			// SECURITY FIX (V3/V4/V5/V7): Use session manager - customer login stored server-side
			sessionMgr := auth.GetTOTPSessionManager()
			token, err := sessionMgr.CreateCustomerSession(user.Login, c.ClientIP(), c.Request.UserAgent())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to create 2FA session"})
				return
			}
			// V4 FIX: Only store token in cookie, NOT the customer's login
			c.SetCookie("customer_2fa_pending", token, 300, "/", "", false, true) // 5 min expiry

			// Handle HTMX vs regular request
			if c.GetHeader("HX-Request") == "true" {
				c.Header("HX-Redirect", "/customer/login/2fa")
				c.JSON(http.StatusOK, gin.H{
					"success":      false,
					"requires_2fa": true,
					"redirect":     "/customer/login/2fa",
				})
				return
			}
			c.Redirect(http.StatusFound, "/customer/login/2fa")
			return
		}

		tenantID := middleware.ResolveTenantFromHost(c.Request.Host)
		token, err := jwtManager.GenerateTokenWithLogin(user.ID, user.Login, user.Email, "Customer", false, tenantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to generate token"})
			return
		}

		sessionTimeout := constants.DefaultSessionTimeout
		// Use customer-specific cookie names to avoid conflicts with agent sessions
		// This allows agent and customer to be logged in simultaneously in the same browser
		c.SetCookie("customer_access_token", token, sessionTimeout, "/", "", false, true)
		c.SetCookie("customer_auth_token", token, sessionTimeout, "/", "", false, true)
		// Set a non-httpOnly indicator so JavaScript can detect authentication
		// (auth tokens are httpOnly for security, but JS needs to know user is logged in)
		c.SetCookie("goatflow_customer_logged_in", "1", sessionTimeout, "/", "", false, false)

		// Use CustomerPreferencesService - keyed by login, not numeric ID
		prefService := service.NewCustomerPreferencesService(db)

		// Persist pre-login language selection to customer preferences
		if preLoginLang, err := c.Cookie("goatflow_lang"); err == nil && preLoginLang != "" {
			if setErr := prefService.SetLanguage(user.Login, preLoginLang); setErr != nil {
				log.Printf("Failed to save customer language preference: %v", setErr)
			}
		}

		// Load customer's saved theme preferences from database and set cookies
		if userTheme := prefService.GetTheme(user.Login); userTheme != "" {
			c.SetCookie("goatflow_theme", userTheme, sessionTimeout, "/", "", false, false)
		}
		if userThemeMode := prefService.GetThemeMode(user.Login); userThemeMode != "" {
			c.SetCookie("goatflow_mode", userThemeMode, sessionTimeout, "/", "", false, false)
		}

		// Create session record in database for admin session management
		if sessionSvc := shared.GetSessionService(); sessionSvc != nil {
			sessionID, err := sessionSvc.CreateSession(
				int(user.ID),
				user.Login,
				"Customer",
				c.ClientIP(),
				c.Request.UserAgent(),
			)
			if err != nil {
				// Log error but don't fail login - session tracking is non-critical
				log.Printf("Failed to create customer session record: %v", err)
			} else {
				// Store session ID in a customer-specific cookie for logout cleanup
				c.SetCookie("customer_session_id", sessionID, sessionTimeout, "/", "", false, true)
			}
		}

		// Always redirect to customer dashboard after login
		c.Header("HX-Redirect", "/customer")
		c.JSON(http.StatusOK, gin.H{
			"success":      true,
			"access_token": token,
			"token_type":   "Bearer",
			"user": gin.H{
				"id":         user.ID,
				"login":      user.Login,
				"email":      user.Email,
				"first_name": user.FirstName,
				"last_name":  user.LastName,
				"role":       "Customer",
			},
		})
	}
}
