package api

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/config"
	"github.com/goatkit/goatflow/internal/platform/constants"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/httpcookie"
	"github.com/goatkit/goatflow/internal/platform/service"
	"github.com/goatkit/goatflow/internal/platform/shared"
)

// resolveUserRole determines the role and admin status for a user by checking
// their admin group membership. Used by all login paths (direct, 2FA)
// to ensure consistent JWT claims. A failed lookup is an error: a guessed
// role would silently demote administrators for the token's lifetime.
func resolveUserRole(userID uint) (role string, isAdmin bool, err error) {
	db, err := database.GetDB()
	if err == nil && db == nil {
		err = errors.New("database connection is nil")
	}
	if err != nil {
		return "", false, fmt.Errorf("resolve role for user %d: %w", userID, err)
	}
	var cnt int
	if err := db.QueryRow(database.ConvertPlaceholders(
		`SELECT COUNT(*) FROM group_user gu JOIN `+"`groups`"+` g ON gu.group_id = g.id WHERE gu.user_id = ? AND LOWER(g.name) = 'admin'`), userID).Scan(&cnt); err != nil {
		return "", false, fmt.Errorf("resolve role for user %d: %w", userID, err)
	}
	if cnt > 0 {
		return "Admin", true, nil
	}
	return "Agent", false, nil
}

// handleLoginPage shows the login page.
func handleLoginPage(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/customer") {
		c.Redirect(http.StatusFound, "/customer/login")
		return
	}

	if config.CustomerFEOnly() {
		c.Redirect(http.StatusFound, "/customer/login")
		return
	}

	if cookie, err := c.Cookie("access_token"); err == nil && cookie != "" {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}

	cfg := config.Get()
	errorMsg := ""
	if rawErr := c.Query("error"); rawErr != "" {
		messages := map[string]string{
			"invalid_provider":   "Invalid login provider.",
			"server_error":       "A server error occurred. Please try again.",
			"provider_not_found": "The selected login provider could not be found.",
			"provider_disabled":  "This login provider has been disabled.",
			"provider_mismatch":  "Authentication provider mismatch. Please try again.",
			"auth_failed":        "Authentication failed. Please check your provider configuration or try again.",
			"saml_config_error":  "SAML provider configuration error. Contact your administrator.",
			"missing_params":     "Missing authentication parameters. Please try again.",
			"missing_state":      "Authentication state missing. Please try again.",
			"invalid_state":      "Authentication session expired. Please try again.",
			"session_error":      "Failed to create a session. Please try again.",
		}
		if msg, ok := messages[rawErr]; ok {
			errorMsg = msg
		} else {
			errorMsg = rawErr
		}
	}

	allowLostPassword := cfg != nil && cfg.Features.LostPassword

	// Get available OIDC providers for IdP buttons
	var idpButtons []map[string]string
	if db, err := database.GetDB(); err == nil && db != nil {
		query := database.ConvertPlaceholders(
			`SELECT id, name, provider_type FROM gk_identity_provider
			WHERE (org_id IS NULL OR org_id = ?) AND enabled = 1
			ORDER BY provider_type, name`,
		)
		rows, err := db.Query(query, activeOrgCookie(c))
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id uint
				var name, ptype string
				if err := rows.Scan(&id, &name, &ptype); err == nil {
					idpButtons = append(idpButtons, map[string]string{
						"id":   fmt.Sprintf("%d", id),
						"name": name,
						"type": ptype,
						"slug": fmt.Sprintf("%d", id),
					})
				}
			}
		}
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/login.pongo2", pongo2.Context{
		"error":             errorMsg,
		"AllowLostPassword": allowLostPassword,
		"IdPButtons":        idpButtons,
	})
}

func handleCustomerLoginPage(c *gin.Context) {
	jwtManager := shared.GetJWTManager()
	// A customer who is already signed in goes straight to their landing page.
	if cookie, err := c.Cookie("customer_access_token"); err == nil && cookie != "" && jwtManager != nil {
		if claims, err := jwtManager.ValidateToken(cookie); err == nil && claims.Role == "Customer" {
			c.Redirect(http.StatusFound, customerLandingRedirect(claims.Login))
			return
		}
	}
	// An agent session must not linger next to a customer sign-in.
	if cookie, err := c.Cookie("access_token"); err == nil && cookie != "" {
		httpcookie.SetAuth(c, "access_token", "", -1)
		httpcookie.SetAuth(c, "auth_token", "", -1)
	}

	errorMsg := c.Query("error")
	cfg := config.Get()

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/customer/login.pongo2", pongo2.Context{
		"error":             errorMsg,
		"AllowLostPassword": cfg != nil && cfg.Features.LostPassword,
		"AllowRegistration": cfg != nil && cfg.Features.Registration,
	})
}

// handleLogout handles logout requests.
func handleLogout(c *gin.Context) {
	// Delete session record from database (check both agent and customer session cookies)
	if sessionID, err := c.Cookie("session_id"); err == nil && sessionID != "" {
		if sessionSvc := shared.GetSessionService(); sessionSvc != nil {
			if err := sessionSvc.KillSession(sessionID); err != nil {
				log.Printf("Failed to delete session record: %v", err)
			}
		}
	}
	if sessionID, err := c.Cookie("customer_session_id"); err == nil && sessionID != "" {
		if sessionSvc := shared.GetSessionService(); sessionSvc != nil {
			if err := sessionSvc.KillSession(sessionID); err != nil {
				log.Printf("Failed to delete customer session record: %v", err)
			}
		}
	}

	// Clear all agent auth cookies
	httpcookie.SetAuth(c, "access_token", "", -1)
	httpcookie.SetAuth(c, "auth_token", "", -1)
	httpcookie.SetAuth(c, "token", "", -1)
	httpcookie.SetAuth(c, "refresh_token", "", -1)
	httpcookie.SetAuth(c, "session_id", "", -1)
	httpcookie.SetAuthState(c, "goatflow_logged_in", "", -1)

	// Clear all customer-specific auth cookies
	httpcookie.SetAuth(c, "customer_access_token", "", -1)
	httpcookie.SetAuth(c, "customer_auth_token", "", -1)
	httpcookie.SetAuth(c, "customer_session_id", "", -1)
	httpcookie.SetAuthState(c, "goatflow_customer_logged_in", "", -1)

	c.Redirect(http.StatusFound, loginRedirectPath(c))
}

func loginRedirectPath(c *gin.Context) string {
	path := c.Request.URL.Path
	if strings.Contains(path, "/customer") {
		return "/customer/login"
	}

	if ref := c.Request.Referer(); strings.Contains(ref, "/customer/") || strings.HasSuffix(ref, "/customer") {
		return "/customer/login"
	}

	if full := c.FullPath(); full != "" && strings.HasPrefix(full, "/customer") {
		return "/customer/login"
	}

	if role, ok := c.Get("user_role"); ok {
		if strings.EqualFold(fmt.Sprintf("%v", role), "customer") {
			return "/customer/login"
		}
	}

	if isCustomer, ok := c.Get("is_customer"); ok {
		if val, ok := isCustomer.(bool); ok && val {
			return "/customer/login"
		}
	}

	if strings.HasPrefix(c.Request.URL.Path, "/customer") {
		return "/customer/login"
	}

	if config.CustomerFEOnly() {
		return "/customer/login"
	}

	return "/login"
}

// handle2FAPage shows the 2FA verification page.
func handle2FAPage(c *gin.Context) {
	token, err := c.Cookie("2fa_pending")
	if err != nil || token == "" {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	session := auth.GetTOTPSessionManager().ValidateAndGetSession(token, c.ClientIP(), c.Request.UserAgent())
	if session == nil || session.IsCustomer {
		httpcookie.SetAuth(c, "2fa_pending", "", -1)
		c.Redirect(http.StatusFound, "/login")
		return
	}

	db, err := database.GetDB()
	var status mfaStatus
	if err == nil {
		status, err = agentMFAStatus(db, session.UserID)
	}
	if err != nil {
		log.Printf("2FA page: second-factor status for user %d unavailable: %v", session.UserID, err)
		c.String(http.StatusInternalServerError, "Login temporarily unavailable")
		return
	}
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/login_2fa.pongo2", mfaLoginPageContext(status))
}

// handle2FAVerify processes the 2FA verification during login.
func handle2FAVerify(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get pending 2FA token from cookie
		pendingToken, err := c.Cookie("2fa_pending")
		if err != nil || pendingToken == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "No pending 2FA session - please login again",
			})
			return
		}

		// SECURITY: Get user data from server-side session manager, NOT cookies
		sessionMgr := auth.GetTOTPSessionManager()
		session := sessionMgr.ValidateAndGetSession(pendingToken, c.ClientIP(), c.Request.UserAgent())
		if session == nil {
			httpcookie.SetAuth(c, "2fa_pending", "", -1)
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Invalid or expired session - please login again",
			})
			return
		}

		userID := session.UserID
		username := session.Username
		limiterKey := twoFactorLimiterKey(false, username)
		if rejectIfLoginBlocked(c, limiterKey) {
			return
		}

		// Get the TOTP code from request
		code := c.PostForm("code")
		if code == "" {
			var req struct {
				Code string `json:"code"`
			}
			if err := c.ShouldBindJSON(&req); err == nil {
				code = req.Code
			}
		}

		if code == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Verification code is required",
			})
			return
		}

		// Verify the TOTP code
		db, err := database.GetDB()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Database unavailable",
			})
			return
		}

		totpService := service.NewTOTPService(db, "GoatFlow")
		valid, err := totpService.ValidateCode(userID, code)
		if err != nil || !valid {
			auth.DefaultLoginRateLimiter.RecordFailure(c.ClientIP(), limiterKey)
			remaining := sessionMgr.RecordFailedAttempt(pendingToken)
			if remaining <= 0 {
				sessionMgr.InvalidateSession(pendingToken)
				httpcookie.SetAuth(c, "2fa_pending", "", -1)
				c.JSON(http.StatusUnauthorized, gin.H{
					"success": false,
					"error":   "Too many failed attempts - please login again",
				})
				return
			}
			c.JSON(http.StatusUnauthorized, gin.H{
				"success":            false,
				"error":              "Invalid verification code",
				"attempts_remaining": remaining,
			})
			return
		}

		auth.DefaultLoginRateLimiter.RecordSuccess(c.ClientIP(), limiterKey)
		// 2FA verified - clear session and cookie
		sessionMgr.InvalidateSession(pendingToken)
		httpcookie.SetAuth(c, "2fa_pending", "", -1)

		// Complete the login - generate token and set cookies
		if jwtManager == nil {
			log.Printf("2FA login: JWT manager unavailable for user %d", userID)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Authentication unavailable"})
			return
		}
		role, isAdmin, err := resolveUserRole(uint(userID))
		if err != nil {
			log.Printf("2FA login: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to generate token"})
			return
		}
		token, err := jwtManager.GenerateTokenWithLogin(uint(userID), username, username, role, isAdmin, 1)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to generate token",
			})
			return
		}

		sessionTimeout := constants.DefaultSessionTimeout
		var userTheme, userThemeMode string
		prefService := service.NewUserPreferencesService(db)
		if userTimeout := prefService.GetSessionTimeout(userID); userTimeout > 0 {
			sessionTimeout = userTimeout
		}
		userTheme = prefService.GetTheme(userID)
		userThemeMode = prefService.GetThemeMode(userID)

		// SECURITY: wipe customer session cookies on agent login (second code
		// path — 2FA-completed login). Keeps parity with the primary agent
		// login so ExtractToken can't mix an old customer session with a new
		// agent one.
		httpcookie.SetAuth(c, "customer_access_token", "", -1)
		httpcookie.SetAuth(c, "customer_auth_token", "", -1)
		httpcookie.SetAuth(c, "customer_session_id", "", -1)
		httpcookie.SetAuthState(c, "goatflow_customer_logged_in", "", -1)
		httpcookie.SetAuth(c, "access_token", token, sessionTimeout)
		httpcookie.SetAuth(c, "auth_token", token, sessionTimeout)
		httpcookie.SetAuthState(c, "goatflow_logged_in", "1", sessionTimeout)

		if userTheme != "" {
			c.SetCookie("goatflow_theme", userTheme, sessionTimeout, "/", "", false, false)
		}
		if userThemeMode != "" {
			c.SetCookie("goatflow_mode", userThemeMode, sessionTimeout, "/", "", false, false)
		}

		// Create session record
		if sessionSvc := shared.GetSessionService(); sessionSvc != nil {
			sessionID, err := sessionSvc.CreateSession(
				userID,
				username,
				"User",
				c.ClientIP(),
				c.Request.UserAgent(),
			)
			if err != nil {
				log.Printf("Failed to create session record: %v", err)
			} else {
				httpcookie.SetAuth(c, "session_id", sessionID, sessionTimeout)
			}
		}

		// Respond based on request type
		contentType := c.GetHeader("Content-Type")
		if c.GetHeader("HX-Request") == "true" {
			c.Header("HX-Redirect", "/dashboard")
			c.JSON(http.StatusOK, gin.H{
				"success":  true,
				"redirect": "/dashboard",
			})
			return
		} else if strings.Contains(contentType, "application/json") {
			// JSON fetch request (from login_2fa.pongo2 form)
			c.JSON(http.StatusOK, gin.H{
				"success":  true,
				"redirect": "/dashboard",
			})
			return
		}

		c.Redirect(http.StatusFound, "/dashboard")
	}
}
