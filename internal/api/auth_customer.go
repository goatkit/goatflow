package api

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/auth"
	"github.com/goatkit/goatflow/internal/platform/constants"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/middleware"
	"github.com/goatkit/goatflow/internal/platform/httpcookie"
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
		if isCustomerMFAEnabled(db, c.Request, user.Login) {
			// SECURITY FIX (V3/V4/V5/V7): Use session manager - customer login stored server-side
			sessionMgr := auth.GetTOTPSessionManager()
			token, err := sessionMgr.CreateCustomerSession(user.Login, c.ClientIP(), c.Request.UserAgent())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to create 2FA session"})
				return
			}
			// V4 FIX: Only store token in cookie, NOT the customer's login
			httpcookie.SetAuth(c, "customer_2fa_pending", token, 300) // 5 min expiry

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
		// SECURITY: wipe any pre-existing agent session cookies on customer
		// login. Allowing both to coexist produced a privilege-escalation path
		// on plugin routes: ExtractToken falls back to agent cookies outside
		// /customer/*, so a customer who logged in after a prior agent session
		// browsed as that agent (admin button visible, /admin accepts the
		// request). One browser, one identity — revoke the other on sign-in.
		httpcookie.SetAuth(c, "access_token", "", -1)
		httpcookie.SetAuth(c, "auth_token", "", -1)
		httpcookie.SetAuth(c, "session_id", "", -1)
		httpcookie.SetAuth(c, "customer_access_token", token, sessionTimeout)
		httpcookie.SetAuth(c, "customer_auth_token", token, sessionTimeout)
		// Set a non-httpOnly indicator so JavaScript can detect authentication
		// (auth tokens are httpOnly for security, but JS needs to know user is logged in)
		httpcookie.SetAuthState(c, "goatflow_customer_logged_in", "1", sessionTimeout)

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
				httpcookie.SetAuth(c, "customer_session_id", sessionID, sessionTimeout)
			}
		}

		redirectTarget := "/customer"
		if target := resolveCustomerCaptiveRedirect(user.Login); target != "" {
			redirectTarget = target
		}

		c.Header("HX-Redirect", redirectTarget)
		c.JSON(http.StatusOK, gin.H{
			"success":      true,
			"access_token": token,
			"token_type":   "Bearer",
			"redirect":     redirectTarget,
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

// primaryOrgForCustomer returns the org that owns the caller's
// customer_company, falling back to gk_user_organisation.is_default.
func primaryOrgForCustomer(db *sql.DB, customerLogin string) int64 {
	var orgID int64
	_ = db.QueryRow(database.ConvertPlaceholders(`
		SELECT o.id FROM customer_user cu
		  JOIN gk_organisation o ON o.customer_company_id = cu.customer_id
		 WHERE cu.login = ?
		 ORDER BY o.id ASC
		 LIMIT 1`), customerLogin).Scan(&orgID)
	if orgID != 0 {
		return orgID
	}
	_ = db.QueryRow(database.ConvertPlaceholders(`
		SELECT org_id FROM gk_user_organisation
		 WHERE customer_login = ?
		 ORDER BY is_default DESC, create_time ASC
		 LIMIT 1`), customerLogin).Scan(&orgID)
	return orgID
}

func resolveCustomerCaptiveRedirect(customerLogin string) string {
	db, err := database.GetDB()
	if err != nil || db == nil {
		return ""
	}
	orgID := primaryOrgForCustomer(db, customerLogin)
	if orgID == 0 {
		return ""
	}
	var cp sql.NullString
	err = db.QueryRow(database.ConvertPlaceholders(
		`SELECT captive_plugin FROM gk_organisation WHERE id = ?`), orgID).Scan(&cp)
	if err != nil || !cp.Valid || cp.String == "" {
		return ""
	}
	var ok int
	err = db.QueryRow(database.ConvertPlaceholders(`
		SELECT 1 FROM gk_org_plugin_access opa
		  JOIN group_customer_user gcu ON gcu.group_id = opa.group_id
		 WHERE opa.org_id = ? AND opa.plugin_name = ? AND gcu.user_id = ?
		 LIMIT 1`), orgID, cp.String, customerLogin).Scan(&ok)
	if err != nil || ok != 1 {
		return ""
	}
	mgr := GetPluginManager()
	if mgr == nil {
		return ""
	}
	return mgr.LandingPageFor(cp.String)
}
