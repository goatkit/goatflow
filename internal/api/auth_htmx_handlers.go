package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/auth"
	"github.com/goatkit/goatflow/internal/config"
	"github.com/goatkit/goatflow/internal/constants"
	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/service"
	"github.com/goatkit/goatflow/internal/shared"
)

// handleLoginPage shows the login page.
func handleLoginPage(c *gin.Context) {
	if cookie, err := c.Cookie("access_token"); err == nil && cookie != "" {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}

	cfg := config.Get()
	errorMsg := c.Query("error")

	// Default to false if config is not initialized (e.g., in tests)
	allowRegistration := false
	allowLostPassword := false
	if cfg != nil {
		allowRegistration = cfg.Features.Registration
		allowLostPassword = cfg.Features.LostPassword
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/login.pongo2", pongo2.Context{
		"error":             errorMsg,
		"AllowRegistration": allowRegistration,
		"AllowLostPassword": allowLostPassword,
	})
}

func handleCustomerLoginPage(c *gin.Context) {
	if cookie, err := c.Cookie("access_token"); err == nil && cookie != "" {
		jwtManager := shared.GetJWTManager()
		if claims, err := jwtManager.ValidateToken(cookie); err == nil && claims.Role == "Customer" {
			c.Redirect(http.StatusFound, "/customer")
			return
		}
		c.SetCookie("access_token", "", -1, "/", "", false, true)
		c.SetCookie("auth_token", "", -1, "/", "", false, true)
	}

	errorMsg := c.Query("error")

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/customer/login.pongo2", pongo2.Context{
		"error": errorMsg,
	})
}

// handleLogin processes login requests.
func handleLogin(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.PostForm("username")
		password := c.PostForm("password")

		clientIP := c.ClientIP()
		if blocked, remaining := auth.DefaultLoginRateLimiter.IsBlocked(clientIP, username); blocked {
			if c.GetHeader("HX-Request") == "true" {
				c.JSON(http.StatusTooManyRequests, gin.H{
					"success":         false,
					"error":           fmt.Sprintf("too many failed attempts, try again in %d seconds", int(remaining.Seconds())),
					"retry_after_sec": int(remaining.Seconds()),
				})
			} else {
				getPongo2Renderer().HTML(c, http.StatusTooManyRequests, "pages/login.pongo2", pongo2.Context{
					"Error": fmt.Sprintf("Too many failed attempts. Please try again in %d seconds.", int(remaining.Seconds())),
				})
			}
			return
		}

		validLogin := false
		userID := uint(1)

		db, err := database.GetDB()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid credentials",
			})
			return
		}

		var dbUserID int
		var dbPassword string
		var validID int

		query := database.ConvertPlaceholders(`
			SELECT id, pw, valid_id
			FROM users
			WHERE login = ?
			AND valid_id = 1`)
		err = db.QueryRow(query, username).Scan(&dbUserID, &dbPassword, &validID)
		if err != nil {
			// User not found or other database error
		} else if validID == 1 {
			if verifyPassword(password, dbPassword) {
				validLogin = true
				userID = uint(dbUserID)
			}
		} else {
			query2 := database.ConvertPlaceholders(`
				SELECT id, pw, valid_id
				FROM users
				WHERE login = ?
				AND pw = ?
				AND valid_id = 1`)
			err = db.QueryRow(query2, username, password).Scan(&dbUserID, &dbPassword, &validID)

			if err == nil && validID == 1 {
				validLogin = true
				userID = uint(dbUserID)

				salt := generateSalt()
				combined := password + salt
				hash := sha256.Sum256([]byte(combined))
				hashedPassword := fmt.Sprintf("sha256$%s$%s", salt, hex.EncodeToString(hash[:]))

				updateQuery := database.ConvertPlaceholders(`
					UPDATE users
					SET pw = ?,
					    change_time = CURRENT_TIMESTAMP
					WHERE id = ?`)
				_, _ = db.Exec(updateQuery, hashedPassword, dbUserID) //nolint:errcheck // Best-effort password rehash
			}
		}

		if !validLogin {
			auth.DefaultLoginRateLimiter.RecordFailure(clientIP, username)
			isHXRequest := c.GetHeader("HX-Request") == "true"
			isJSONRequest := strings.Contains(c.GetHeader("Accept"), "application/json")
			renderMissing := getPongo2Renderer() == nil || getPongo2Renderer().TemplateSet() == nil
			if isHXRequest || isJSONRequest || renderMissing {
				c.JSON(http.StatusUnauthorized, gin.H{
					"success": false,
					"error":   "Invalid credentials",
				})
				return
			}
			getPongo2Renderer().HTML(c, http.StatusUnauthorized, "pages/login.pongo2", pongo2.Context{
				"Error": "Invalid username or password",
			})
			return
		}

		auth.DefaultLoginRateLimiter.RecordSuccess(clientIP, username)

		// Check if 2FA is enabled for this user
		totpService := service.NewTOTPService(db, "GoatFlow")
		if totpService.IsEnabled(int(userID)) {
			// 2FA is enabled - don't complete login yet
			// SECURITY FIX (V3/V4/V5/V7): Use session manager instead of raw cookies
			sessionMgr := auth.GetTOTPSessionManager()
			token, err := sessionMgr.CreateAgentSession(int(userID), username, c.ClientIP(), c.Request.UserAgent())
			if err != nil {
				sendErrorResponse(c, http.StatusInternalServerError, "Failed to create 2FA session")
				return
			}
			
			// Only store the token in cookie - user data is server-side
			c.SetCookie("2fa_pending", token, 300, "/", "", false, true) // 5 min expiry
			
			if c.GetHeader("HX-Request") == "true" {
				c.Header("HX-Redirect", "/login/2fa")
				c.JSON(http.StatusOK, gin.H{
					"success":      true,
					"requires_2fa": true,
					"redirect":     "/login/2fa",
				})
				return
			}
			
			c.Redirect(http.StatusFound, "/login/2fa")
			return
		}

		var token string
		if jwtManager != nil {
			// Determine role and admin status from group membership
			role := "Agent"
			isAdmin := false
			if db != nil {
				var cnt int
				_ = db.QueryRowContext(c.Request.Context(), database.ConvertPlaceholders(
					`SELECT COUNT(*) FROM group_user gu JOIN `+"`groups`"+` g ON gu.group_id = g.id WHERE gu.user_id = ? AND LOWER(g.name) = 'admin'`), userID).Scan(&cnt)
				if cnt > 0 {
					role = "Admin"
					isAdmin = true
				}
			}
			tokenStr, err := jwtManager.GenerateTokenWithLogin(userID, username, username, role, isAdmin, 1)
			if err != nil {
				sendErrorResponse(c, http.StatusInternalServerError, "Failed to generate token")
				return
			}
			token = tokenStr
		} else {
			token = fmt.Sprintf("demo_session_%d_%d", userID, time.Now().Unix())
		}

		sessionTimeout := constants.DefaultSessionTimeout
		var userTheme, userThemeMode string
		if db != nil {
			prefService := service.NewUserPreferencesService(db)
			if userTimeout := prefService.GetSessionTimeout(int(userID)); userTimeout > 0 {
				sessionTimeout = userTimeout
			}
			// Load user's saved theme preferences from database
			userTheme = prefService.GetTheme(int(userID))
			userThemeMode = prefService.GetThemeMode(int(userID))
		}

		c.SetCookie("access_token", token, sessionTimeout, "/", "", false, true)
		c.SetCookie("auth_token", token, sessionTimeout, "/", "", false, true)
		// Set a non-httpOnly indicator so JavaScript can detect authentication
		// (auth tokens are httpOnly for security, but JS needs to know user is logged in)
		c.SetCookie("goatflow_logged_in", "1", sessionTimeout, "/", "", false, false)

		// Set theme cookies from database preferences (if user has saved preferences)
		// These will override any login-page localStorage values in the browser
		if userTheme != "" {
			c.SetCookie("goatflow_theme", userTheme, sessionTimeout, "/", "", false, false)
		}
		if userThemeMode != "" {
			c.SetCookie("goatflow_mode", userThemeMode, sessionTimeout, "/", "", false, false)
		}

		// Create session record in database for admin session management
		if sessionSvc := shared.GetSessionService(); sessionSvc != nil {
			sessionID, err := sessionSvc.CreateSession(
				int(userID),
				username,
				"User",
				c.ClientIP(),
				c.Request.UserAgent(),
			)
			if err != nil {
				// Log error but don't fail login - session tracking is non-critical
				log.Printf("Failed to create session record: %v", err)
			} else {
				// Store session ID in a cookie for logout cleanup
				c.SetCookie("session_id", sessionID, sessionTimeout, "/", "", false, true)
			}
		}

		if c.GetHeader("HX-Request") == "true" {
			c.Header("HX-Redirect", "/dashboard")
			c.JSON(http.StatusOK, gin.H{
				"success":  true,
				"redirect": "/dashboard",
			})
			return
		}

		c.Redirect(http.StatusFound, "/dashboard")
	}
}

// handleHTMXLogin handles HTMX login requests.
func handleHTMXLogin(c *gin.Context) {
	demoEmail := os.Getenv("DEMO_LOGIN_EMAIL")
	demoPassword := os.Getenv("DEMO_LOGIN_PASSWORD")

	var payload struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	_ = c.ShouldBindJSON(&payload) //nolint:errcheck // Defaults to empty

	if strings.TrimSpace(payload.Email) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "email required"})
		return
	}

	if demoEmail != "" || demoPassword != "" {
		if payload.Email != demoEmail || payload.Password != demoPassword {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Invalid credentials"})
			return
		}

		token, err := getJWTManager().GenerateToken(1, demoEmail, "Agent", 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to generate token"})
			return
		}

		c.Header("HX-Redirect", "/dashboard")
		c.JSON(http.StatusOK, gin.H{
			"success":      true,
			"access_token": token,
			"token_type":   "Bearer",
			"user": gin.H{
				"login":      demoEmail,
				"email":      demoEmail,
				"first_name": "Test",
				"last_name":  "User",
				"role":       "Agent",
			},
		})
		return
	}

	testEmail := os.Getenv("TEST_AUTH_EMAIL")
	testPass := os.Getenv("TEST_AUTH_PASSWORD")
	if testEmail != "" && testPass != "" && payload.Email == testEmail && payload.Password == testPass {
		token := "test-token"
		c.Header("HX-Redirect", "/dashboard")
		c.JSON(http.StatusOK, gin.H{
			"success":      true,
			"access_token": token,
			"token_type":   "Bearer",
			"user": gin.H{
				"login":      payload.Email,
				"email":      payload.Email,
				"first_name": "Admin",
				"last_name":  "User",
				"role":       "Agent",
			},
		})
		return
	}

	c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Invalid credentials"})
}

// handleDemoCustomerLogin creates a demo customer token for testing.
func handleDemoCustomerLogin(c *gin.Context) {
	token := fmt.Sprintf("demo_customer_%s_%d", "john.customer", time.Now().Unix())
	c.SetCookie("access_token", token, 86400, "/", "", false, true)
	c.Redirect(http.StatusFound, "/customer/")
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
	c.SetCookie("access_token", "", -1, "/", "", false, true)
	c.SetCookie("auth_token", "", -1, "/", "", false, true)
	c.SetCookie("token", "", -1, "/", "", false, true)
	c.SetCookie("session_id", "", -1, "/", "", false, true)
	c.SetCookie("goatflow_logged_in", "", -1, "/", "", false, false)

	// Clear all customer-specific auth cookies
	c.SetCookie("customer_access_token", "", -1, "/", "", false, true)
	c.SetCookie("customer_auth_token", "", -1, "/", "", false, true)
	c.SetCookie("customer_session_id", "", -1, "/", "", false, true)
	c.SetCookie("goatflow_customer_logged_in", "", -1, "/", "", false, false)

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

	switch strings.ToLower(strings.TrimSpace(os.Getenv("CUSTOMER_FE_ONLY"))) {
	case "1", "true":
		return "/customer/login"
	}

	return "/login"
}

// handle2FAPage shows the 2FA verification page.
func handle2FAPage(c *gin.Context) {
	if _, err := c.Cookie("2fa_pending"); err != nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/login_2fa.pongo2", pongo2.Context{})
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
			c.SetCookie("2fa_pending", "", -1, "/", "", false, true)
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Invalid or expired session - please login again",
			})
			return
		}

		userID := session.UserID
		username := session.Username

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
			// Record failed attempt
			remaining := sessionMgr.RecordFailedAttempt(pendingToken)
			if remaining <= 0 {
				sessionMgr.InvalidateSession(pendingToken)
				c.SetCookie("2fa_pending", "", -1, "/", "", false, true)
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

		// 2FA verified - clear session and cookie
		sessionMgr.InvalidateSession(pendingToken)
		c.SetCookie("2fa_pending", "", -1, "/", "", false, true)

		// Complete the login - generate token and set cookies
		var token string
		if jwtManager != nil {
			tokenStr, err := jwtManager.GenerateToken(uint(userID), username, "user", 1)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error":   "Failed to generate token",
				})
				return
			}
			token = tokenStr
		} else {
			token = fmt.Sprintf("demo_session_%d_%d", userID, time.Now().Unix())
		}

		sessionTimeout := constants.DefaultSessionTimeout
		var userTheme, userThemeMode string
		prefService := service.NewUserPreferencesService(db)
		if userTimeout := prefService.GetSessionTimeout(userID); userTimeout > 0 {
			sessionTimeout = userTimeout
		}
		userTheme = prefService.GetTheme(userID)
		userThemeMode = prefService.GetThemeMode(userID)

		c.SetCookie("access_token", token, sessionTimeout, "/", "", false, true)
		c.SetCookie("auth_token", token, sessionTimeout, "/", "", false, true)
		c.SetCookie("goatflow_logged_in", "1", sessionTimeout, "/", "", false, false)

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
				c.SetCookie("session_id", sessionID, sessionTimeout, "/", "", false, true)
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
