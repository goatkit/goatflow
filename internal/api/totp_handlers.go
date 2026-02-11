package api

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"fmt"
	"image/png"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"

	"github.com/goatkit/goatflow/internal/auth"
	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/notifications"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/routing"
	"github.com/goatkit/goatflow/internal/service"
	"github.com/goatkit/goatflow/internal/shared"
)

func init() {
	// Agent 2FA handlers
	routing.RegisterHandler("handleTOTPStatus", handleTOTPStatus)
	routing.RegisterHandler("handleTOTPSetup", handleTOTPSetup)
	routing.RegisterHandler("handleTOTPConfirm", handleTOTPConfirm)
	routing.RegisterHandler("handleTOTPDisable", handleTOTPDisable)
	routing.RegisterHandler("handleTOTPVerify", handleTOTPVerify)

	// Customer 2FA handlers
	routing.RegisterHandler("handleCustomerTOTPStatus", handleCustomerTOTPStatus)
	routing.RegisterHandler("handleCustomerTOTPSetup", handleCustomerTOTPSetup)
	routing.RegisterHandler("handleCustomerTOTPConfirm", handleCustomerTOTPConfirm)
	routing.RegisterHandler("handleCustomerTOTPDisable", handleCustomerTOTPDisable)
	routing.RegisterHandler("handleCustomer2FAPage", handleCustomer2FAPage)
	routing.RegisterHandler("handleCustomer2FAVerify", handleCustomer2FAVerify)

	// Admin 2FA override handlers
	routing.RegisterHandler("handleAdmin2FAOverride", handleAdmin2FAOverride)
	routing.RegisterHandler("handleAdminCustomer2FAOverride", handleAdminCustomer2FAOverride)
}

// handleTOTPStatus returns whether 2FA is enabled for the current user.
func handleTOTPStatus(c *gin.Context) {
	userID := getTOTPUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "database unavailable"})
		return
	}

	totpService := service.NewTOTPService(db, "GoatFlow")
	enabled := totpService.IsEnabled(userID)
	remaining := totpService.GetRemainingRecoveryCodes(userID)

	c.JSON(http.StatusOK, gin.H{
		"success":                  true,
		"enabled":                  enabled,
		"recovery_codes_remaining": remaining,
	})
}

// handleTOTPSetup initiates 2FA setup - returns secret and QR code.
func handleTOTPSetup(c *gin.Context) {
	userID := getTOTPUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}

	// Require password to initiate 2FA setup
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "password is required"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "database unavailable"})
		return
	}

	// Verify password before allowing 2FA setup
	userRepo := repository.NewUserRepository(db)
	user, err := userRepo.GetByID(uint(userID))
	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "user not found"})
		return
	}
	if !auth.NewPasswordHasher().VerifyPassword(req.Password, user.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "incorrect password"})
		return
	}

	userEmail := getTOTPUserEmail(c)
	if userEmail == "" {
		userEmail = "user@goatflow.local"
	}

	totpService := service.NewTOTPService(db, "GoatFlow")

	// Check if already enabled
	if totpService.IsEnabled(userID) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "2FA is already enabled"})
		return
	}

	// Generate setup data
	setup, err := totpService.GenerateSetup(userID, userEmail)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to generate 2FA setup"})
		return
	}

	// Generate QR code image
	key, err := otp.NewKeyFromURL(setup.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to generate QR code"})
		return
	}

	img, err := key.Image(200, 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to generate QR image"})
		return
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to encode QR image"})
		return
	}

	qrBase64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"secret":         setup.Secret,
		"qr_code":        "data:image/png;base64," + qrBase64,
		"recovery_codes": setup.RecoveryCodes,
		"message":        "Scan the QR code with your authenticator app, then enter a code to confirm setup.",
	})
}

// handleTOTPConfirm confirms 2FA setup with a verification code.
// V9: Requires password re-verification to prevent session hijacking attacks.
func handleTOTPConfirm(c *gin.Context) {
	userID := getTOTPUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}

	var req struct {
		Code     string `json:"code" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "code and password are required"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "database unavailable"})
		return
	}

	// V9: Verify password before allowing 2FA setup
	userRepo := repository.NewUserRepository(db)
	user, err := userRepo.GetByID(uint(userID))
	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "user not found"})
		return
	}
	if !auth.NewPasswordHasher().VerifyPassword(req.Password, user.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "incorrect password"})
		return
	}

	totpService := service.NewTOTPService(db, "GoatFlow")

	if err := totpService.ConfirmSetup(userID, req.Code); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	// Send security notification email
	go send2FAEnabledNotification(db, uint(userID), c.ClientIP())

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Two-factor authentication has been enabled.",
	})
}

// handleTOTPDisable disables 2FA for the current user.
// V9: Requires password re-verification to prevent session hijacking attacks.
func handleTOTPDisable(c *gin.Context) {
	userID := getTOTPUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}

	var req struct {
		Code     string `json:"code" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "code and password are required to disable 2FA"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "database unavailable"})
		return
	}

	// V9: Verify password before allowing 2FA disable
	userRepo := repository.NewUserRepository(db)
	user, err := userRepo.GetByID(uint(userID))
	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "user not found"})
		return
	}
	if !auth.NewPasswordHasher().VerifyPassword(req.Password, user.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "incorrect password"})
		return
	}

	totpService := service.NewTOTPService(db, "GoatFlow")

	if err := totpService.Disable(userID, req.Code); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	// Send security notification email
	go send2FADisabledNotification(db, uint(userID), c.ClientIP())

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Two-factor authentication has been disabled.",
	})
}

// send2FADisabledNotification sends an email notification when 2FA is disabled for an agent.
func send2FADisabledNotification(db *sql.DB, userID uint, clientIP string) {
	userRepo := repository.NewUserRepository(db)
	user, err := userRepo.GetByID(userID)
	if err != nil || user == nil || user.Email == "" {
		log.Printf("[SECURITY] 2FA disabled for user %d but could not send notification: %v", userID, err)
		return
	}

	subject := "Security Alert: Two-Factor Authentication Disabled"
	body := fmt.Sprintf(`Hello %s,

Two-factor authentication has been disabled on your GoatFlow account.

Details:
- Time: %s
- IP Address: %s

If you did not make this change, please contact your administrator immediately and consider changing your password.

This is an automated security notification from GoatFlow.
`, user.FirstName, time.Now().Format("2006-01-02 15:04:05 MST"), clientIP)

	if err := notifications.SendEmail(user.Email, subject, body); err != nil {
		log.Printf("[SECURITY] Failed to send 2FA disabled notification to %s: %v", user.Email, err)
	} else {
		log.Printf("[SECURITY] 2FA disabled notification sent to %s", user.Email)
	}
}

// send2FAEnabledNotification sends an email notification when 2FA is enabled for an agent.
func send2FAEnabledNotification(db *sql.DB, userID uint, clientIP string) {
	userRepo := repository.NewUserRepository(db)
	user, err := userRepo.GetByID(userID)
	if err != nil || user == nil || user.Email == "" {
		log.Printf("[SECURITY] 2FA enabled for user %d but could not send notification: %v", userID, err)
		return
	}

	subject := "Security Alert: Two-Factor Authentication Enabled"
	body := fmt.Sprintf(`Hello %s,

Two-factor authentication has been enabled on your GoatFlow account.

Details:
- Time: %s
- IP Address: %s

Your account is now more secure. You will need to enter a verification code from your authenticator app each time you log in.

If you did not make this change, please contact your administrator immediately.

This is an automated security notification from GoatFlow.
`, user.FirstName, time.Now().Format("2006-01-02 15:04:05 MST"), clientIP)

	if err := notifications.SendEmail(user.Email, subject, body); err != nil {
		log.Printf("[SECURITY] Failed to send 2FA enabled notification to %s: %v", user.Email, err)
	} else {
		log.Printf("[SECURITY] 2FA enabled notification sent to %s", user.Email)
	}
}

// send2FADisabledByAdminNotification sends an email when an admin disables a user's 2FA.
func send2FADisabledByAdminNotification(username, email string) {
	if email == "" {
		log.Printf("[SECURITY] Admin disabled 2FA for %s but no email available", username)
		return
	}

	subject := "Security Alert: Two-Factor Authentication Disabled by Administrator"
	body := fmt.Sprintf(`Hello %s,

An administrator has disabled two-factor authentication on your account.

Details:
- Time: %s

If you did not request this change, please contact your administrator immediately.

This is an automated security notification from GoatFlow.
`, username, time.Now().Format("2006-01-02 15:04:05 MST"))

	if err := notifications.SendEmail(email, subject, body); err != nil {
		log.Printf("[SECURITY] Failed to send admin 2FA override notification to %s: %v", email, err)
	} else {
		log.Printf("[SECURITY] Admin 2FA override notification sent to %s", email)
	}
}

// handleTOTPVerify verifies a TOTP code during login (called after password verification).
// SECURITY: User data is retrieved from server-side session manager, NOT from cookies.
func handleTOTPVerify(c *gin.Context) {
	// Get token from cookie
	token, err := c.Cookie("2fa_pending")
	if err != nil || token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "no pending 2FA session"})
		return
	}

	// SECURITY FIX (V3/V5/V7): Validate session and get user data from server-side session manager
	sessionMgr := auth.GetTOTPSessionManager()
	session := sessionMgr.ValidateAndGetSession(token, c.ClientIP(), c.Request.UserAgent())
	if session == nil {
		c.SetCookie("2fa_pending", "", -1, "/", "", false, true)
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "invalid or expired 2FA session"})
		return
	}

	// Get code from request body
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "verification code is required"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "database unavailable"})
		return
	}

	totpService := service.NewTOTPService(db, "GoatFlow")
	valid, err := totpService.ValidateCode(session.UserID, req.Code)

	if err != nil || !valid {
		// SECURITY FIX (V3/V7): Record failed attempt and check if session should be invalidated
		remaining := sessionMgr.RecordFailedAttempt(token)
		if remaining <= 0 {
			c.SetCookie("2fa_pending", "", -1, "/", "", false, true)
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "too many failed attempts, please login again",
			})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{
			"success":            false,
			"error":              "invalid code",
			"attempts_remaining": remaining,
		})
		return
	}

	// Success! Invalidate the pending session
	sessionMgr.InvalidateSession(token)
	c.SetCookie("2fa_pending", "", -1, "/", "", false, true)

	// Complete login - issue JWT token
	jwtManager := shared.GetJWTManager()
	if jwtManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "authentication not configured"})
		return
	}

	// Determine role and admin status from group membership
	totpRole := "Agent"
	totpIsAdmin := false
	if db != nil {
		var cnt int
		_ = db.QueryRowContext(c.Request.Context(), database.ConvertPlaceholders(
			`SELECT COUNT(*) FROM group_user gu JOIN `+"`groups`"+` g ON gu.group_id = g.id WHERE gu.user_id = ? AND LOWER(g.name) = 'admin'`), session.UserID).Scan(&cnt)
		if cnt > 0 {
			totpRole = "Admin"
			totpIsAdmin = true
		}
	}
	jwtToken, err := jwtManager.GenerateTokenWithLogin(uint(session.UserID), session.Username, session.Username, totpRole, totpIsAdmin, 1)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to generate token"})
		return
	}

	// Set auth cookies
	sessionTimeout := 86400 // 24 hours
	c.SetCookie("access_token", jwtToken, sessionTimeout, "/", "", false, true)
	c.SetCookie("auth_token", jwtToken, sessionTimeout, "/", "", false, true)
	c.SetCookie("goatflow_logged_in", "1", sessionTimeout, "/", "", false, false)

	c.Header("HX-Redirect", "/dashboard")
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"redirect": "/dashboard",
		"message":  "Code verified successfully.",
	})
}

// Helper to get user ID from context for TOTP handlers
func getTOTPUserID(c *gin.Context) int {
	if val, exists := c.Get("user_id"); exists {
		return shared.ToInt(val, 0)
	}
	return 0
}

// Helper to get user email from context for TOTP handlers
func getTOTPUserEmail(c *gin.Context) string {
	if val, exists := c.Get("user_login"); exists {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// Helper to get customer login from context
func getCustomerLogin(c *gin.Context) string {
	// Check if user is a customer
	isCustomer, _ := c.Get("is_customer")
	if isCustomer != true {
		// Also check for customer_login set by API token middleware
		if val, exists := c.Get("customer_login"); exists {
			if s, ok := val.(string); ok && s != "" {
				return s
			}
		}
		return ""
	}

	// For authenticated customers, login is stored in "username"
	if val, exists := c.Get("username"); exists {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// ============================================================================
// Customer 2FA Handlers - reuse TOTPService with ForCustomer methods
// ============================================================================

// handleCustomerTOTPStatus returns whether 2FA is enabled for the current customer.
func handleCustomerTOTPStatus(c *gin.Context) {
	customerLogin := getCustomerLogin(c)
	if customerLogin == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "database unavailable"})
		return
	}

	totpService := service.NewTOTPService(db, "GoatFlow")
	enabled := totpService.IsEnabledForCustomer(customerLogin)
	remaining := totpService.GetRemainingRecoveryCodesForCustomer(customerLogin)

	c.JSON(http.StatusOK, gin.H{
		"success":                  true,
		"enabled":                  enabled,
		"recovery_codes_remaining": remaining,
	})
}

// handleCustomerTOTPSetup initiates 2FA setup for a customer.
func handleCustomerTOTPSetup(c *gin.Context) {
	customerLogin := getCustomerLogin(c)
	if customerLogin == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}

	// Require password to initiate 2FA setup
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "password is required"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "database unavailable"})
		return
	}

	// Verify password before allowing 2FA setup
	if !verifyCustomerPassword(db, customerLogin, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "incorrect password"})
		return
	}

	totpService := service.NewTOTPService(db, "GoatFlow")

	// Check if already enabled
	if totpService.IsEnabledForCustomer(customerLogin) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "2FA is already enabled"})
		return
	}

	// Generate setup data
	setup, err := totpService.GenerateSetupForCustomer(customerLogin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to generate 2FA setup"})
		return
	}

	// Generate QR code image
	key, err := otp.NewKeyFromURL(setup.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to generate QR code"})
		return
	}

	img, err := key.Image(200, 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to generate QR image"})
		return
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to encode QR image"})
		return
	}

	qrBase64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"secret":         setup.Secret,
		"qr_code":        "data:image/png;base64," + qrBase64,
		"recovery_codes": setup.RecoveryCodes,
		"message":        "Scan the QR code with your authenticator app, then enter a code to confirm setup.",
	})
}

// verifyCustomerPassword checks if the given password is correct for a customer.
func verifyCustomerPassword(db *sql.DB, customerLogin, password string) bool {
	var pw string
	query := `SELECT pw FROM customer_user WHERE login = ? OR email = ?`
	err := db.QueryRow(database.ConvertPlaceholders(query), customerLogin, customerLogin).Scan(&pw)
	if err != nil {
		return false
	}
	hasher := auth.NewPasswordHasher()
	return hasher.VerifyPassword(password, pw)
}

// handleCustomerTOTPConfirm confirms 2FA setup for a customer.
// V9: Requires password re-verification to prevent session hijacking attacks.
func handleCustomerTOTPConfirm(c *gin.Context) {
	customerLogin := getCustomerLogin(c)
	if customerLogin == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}

	var req struct {
		Code     string `json:"code" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "code and password are required"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "database unavailable"})
		return
	}

	// V9: Verify password before allowing 2FA setup
	if !verifyCustomerPassword(db, customerLogin, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "incorrect password"})
		return
	}

	totpService := service.NewTOTPService(db, "GoatFlow")

	if err := totpService.ConfirmSetupForCustomer(customerLogin, req.Code); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	// Send security notification email
	go sendCustomer2FAEnabledNotification(db, customerLogin, c.ClientIP())

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Two-factor authentication has been enabled.",
	})
}

// handleCustomerTOTPDisable disables 2FA for a customer.
// V9: Requires password re-verification to prevent session hijacking attacks.
func handleCustomerTOTPDisable(c *gin.Context) {
	customerLogin := getCustomerLogin(c)
	if customerLogin == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}

	var req struct {
		Code     string `json:"code" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "code and password are required to disable 2FA"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "database unavailable"})
		return
	}

	// V9: Verify password before allowing 2FA disable
	if !verifyCustomerPassword(db, customerLogin, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "incorrect password"})
		return
	}

	totpService := service.NewTOTPService(db, "GoatFlow")

	if err := totpService.DisableForCustomer(customerLogin, req.Code); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	// Send security notification email
	go sendCustomer2FADisabledNotification(db, customerLogin, c.ClientIP())

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Two-factor authentication has been disabled.",
	})
}

// sendCustomer2FADisabledNotification sends an email notification when 2FA is disabled for a customer.
func sendCustomer2FADisabledNotification(db *sql.DB, customerLogin, clientIP string) {
	var email, firstName string
	err := db.QueryRow(database.ConvertPlaceholders(
		`SELECT email, first_name FROM customer_user WHERE (login = ? OR email = ?) AND valid_id = 1`,
	), customerLogin, customerLogin).Scan(&email, &firstName)

	if err != nil || email == "" {
		log.Printf("[SECURITY] 2FA disabled for customer %s but could not send notification: %v", customerLogin, err)
		return
	}

	subject := "Security Alert: Two-Factor Authentication Disabled"
	body := fmt.Sprintf(`Hello %s,

Two-factor authentication has been disabled on your account.

Details:
- Time: %s
- IP Address: %s

If you did not make this change, please contact support immediately and consider changing your password.

This is an automated security notification.
`, firstName, time.Now().Format("2006-01-02 15:04:05 MST"), clientIP)

	if err := notifications.SendEmail(email, subject, body); err != nil {
		log.Printf("[SECURITY] Failed to send 2FA disabled notification to customer %s: %v", email, err)
	} else {
		log.Printf("[SECURITY] 2FA disabled notification sent to customer %s", email)
	}
}

// sendCustomer2FAEnabledNotification sends an email notification when 2FA is enabled for a customer.
func sendCustomer2FAEnabledNotification(db *sql.DB, customerLogin, clientIP string) {
	var email, firstName string
	err := db.QueryRow(database.ConvertPlaceholders(
		`SELECT email, first_name FROM customer_user WHERE (login = ? OR email = ?) AND valid_id = 1`,
	), customerLogin, customerLogin).Scan(&email, &firstName)

	if err != nil || email == "" {
		log.Printf("[SECURITY] 2FA enabled for customer %s but could not send notification: %v", customerLogin, err)
		return
	}

	subject := "Security Alert: Two-Factor Authentication Enabled"
	body := fmt.Sprintf(`Hello %s,

Two-factor authentication has been enabled on your account.

Details:
- Time: %s
- IP Address: %s

Your account is now more secure. You will need to enter a verification code from your authenticator app each time you log in.

If you did not make this change, please contact support immediately.

This is an automated security notification.
`, firstName, time.Now().Format("2006-01-02 15:04:05 MST"), clientIP)

	if err := notifications.SendEmail(email, subject, body); err != nil {
		log.Printf("[SECURITY] Failed to send 2FA enabled notification to customer %s: %v", email, err)
	} else {
		log.Printf("[SECURITY] 2FA enabled notification sent to customer %s", email)
	}
}

// handleCustomer2FAPage renders the 2FA verification page during customer login.
func handleCustomer2FAPage(c *gin.Context) {
	// Check for pending 2FA cookie
	if _, err := c.Cookie("customer_2fa_pending"); err != nil {
		c.Redirect(http.StatusFound, "/customer/login")
		return
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/customer/login_2fa.pongo2", nil)
}

// handleCustomer2FAVerify verifies the 2FA code and completes customer login.
// SECURITY: Customer login is retrieved from server-side session, NOT from cookies (V4 fix).
func handleCustomer2FAVerify(c *gin.Context) {
	// Get token from cookie
	token, err := c.Cookie("customer_2fa_pending")
	if err != nil || token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "no pending 2FA session"})
		return
	}

	// SECURITY FIX (V3/V4/V5/V7): Get customer login from server-side session manager
	sessionMgr := auth.GetTOTPSessionManager()
	session := sessionMgr.ValidateAndGetSession(token, c.ClientIP(), c.Request.UserAgent())
	if session == nil || !session.IsCustomer {
		c.SetCookie("customer_2fa_pending", "", -1, "/", "", false, true)
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "invalid or expired 2FA session"})
		return
	}

	// Get the code from request
	var code string
	if c.GetHeader("Content-Type") == "application/json" {
		var req struct {
			Code string `json:"code"`
		}
		if err := c.ShouldBindJSON(&req); err == nil {
			code = req.Code
		}
	} else {
		code = c.PostForm("code")
	}

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "verification code is required"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "database unavailable"})
		return
	}

	// Validate the 2FA code using customer login from server-side session
	totpService := service.NewTOTPService(db, "GoatFlow")
	valid, err := totpService.ValidateCodeForCustomer(session.UserLogin, code)

	if err != nil || !valid {
		// SECURITY FIX (V3/V7): Record failed attempt
		remaining := sessionMgr.RecordFailedAttempt(token)
		if remaining <= 0 {
			c.SetCookie("customer_2fa_pending", "", -1, "/", "", false, true)
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "too many failed attempts, please login again",
			})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{
			"success":            false,
			"error":              "invalid verification code",
			"attempts_remaining": remaining,
		})
		return
	}

	// Success! Invalidate the pending session
	sessionMgr.InvalidateSession(token)
	c.SetCookie("customer_2fa_pending", "", -1, "/", "", false, true)

	// Complete the login - issue JWT token
	jwtManager := shared.GetJWTManager()
	if jwtManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "authentication not configured"})
		return
	}

	// Look up user to get full details
	var userID uint
	var email, firstName, lastName string
	query := "SELECT id, email, first_name, last_name FROM customer_user WHERE login = ?"
	if err := db.QueryRow(query, session.UserLogin).Scan(&userID, &email, &firstName, &lastName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to load user"})
		return
	}

	// Generate token
	jwtToken, err := jwtManager.GenerateTokenWithLogin(userID, session.UserLogin, email, "Customer", false, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to generate token"})
		return
	}

	// Set auth cookies
	sessionTimeout := 86400 // 24 hours
	c.SetCookie("customer_access_token", jwtToken, sessionTimeout, "/", "", false, true)
	c.SetCookie("customer_auth_token", jwtToken, sessionTimeout, "/", "", false, true)
	c.SetCookie("goatflow_customer_logged_in", "1", sessionTimeout, "/", "", false, false)

	// Redirect to customer dashboard
	c.Header("HX-Redirect", "/customer")
	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"access_token": jwtToken,
		"redirect":     "/customer",
	})
}

// =============================================================================
// Admin 2FA Override Handlers
// =============================================================================

// handleAdmin2FAOverride allows admins to disable 2FA for an agent/admin user.
// This action is logged to the admin_action_log table.
func handleAdmin2FAOverride(c *gin.Context) {
	// Get admin user from context
	adminID := getAdminUserID(c)
	if adminID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "admin access required"})
		return
	}

	// Get target user ID from URL param
	targetUserID, _ := strconv.Atoi(c.Param("id"))
	if targetUserID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid user ID"})
		return
	}

	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "reason is required"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "database unavailable"})
		return
	}

	// Get target user info
	userRepo := repository.NewUserRepository(db)
	targetUser, err := userRepo.GetByID(uint(targetUserID))
	if err != nil || targetUser == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "user not found"})
		return
	}

	// Check if 2FA is enabled
	totpService := service.NewTOTPService(db, "GoatFlow")
	if !totpService.IsEnabled(targetUserID) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "2FA is not enabled for this user"})
		return
	}

	// Disable 2FA (bypassing code verification)
	if err := totpService.ForceDisable(targetUserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to disable 2FA"})
		return
	}

	// Log the action
	if err := logAdminAction(db, "2FADisable", "user", targetUserID, targetUser.Login, adminID, req.Reason, nil); err != nil {
		log.Printf("Failed to log admin action: %v", err)
		// Don't fail the request, just log the error
	}

	// Send notification to affected user
	go send2FADisabledByAdminNotification(targetUser.Login, targetUser.Email)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "2FA disabled for user"})
}

// handleAdminCustomer2FAOverride allows admins to disable 2FA for a customer.
func handleAdminCustomer2FAOverride(c *gin.Context) {
	adminID := getAdminUserID(c)
	if adminID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "admin access required"})
		return
	}

	customerLogin := c.Param("login")
	if customerLogin == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid customer login"})
		return
	}

	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "reason is required"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "database unavailable"})
		return
	}

	// Get customer info
	var customerEmail string
	query := database.ConvertPlaceholders("SELECT email FROM customer_user WHERE login = ?")
	if err := db.QueryRow(query, customerLogin).Scan(&customerEmail); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "customer not found"})
		return
	}

	// Check if 2FA is enabled
	totpService := service.NewTOTPService(db, "GoatFlow")
	if !totpService.IsEnabledForCustomer(customerLogin) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "2FA is not enabled for this customer"})
		return
	}

	// Disable 2FA
	if err := totpService.ForceDisableForCustomer(customerLogin); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to disable 2FA"})
		return
	}

	// Log the action
	if err := logAdminAction(db, "2FADisable", "customer", 0, customerLogin, adminID, req.Reason, nil); err != nil {
		log.Printf("Failed to log admin action: %v", err)
	}

	// Send notification
	go send2FADisabledByAdminNotification(customerLogin, customerEmail)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "2FA disabled for customer"})
}

// getAdminUserID extracts admin user ID from context (requires Admin role).
func getAdminUserID(c *gin.Context) int {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0
	}

	role, _ := c.Get("user_role")
	if role != "Admin" {
		return 0
	}

	switch id := userID.(type) {
	case float64:
		return int(id)
	case int:
		return id
	case uint:
		return int(id)
	case int64:
		return int(id)
	case uint64:
		return int(id)
	}
	return 0
}

// logAdminAction records an admin action to the audit log.
func logAdminAction(db *sql.DB, actionName, targetType string, targetID int, targetIdentifier string, adminID int, reason string, details map[string]interface{}) error {
	// Get action type ID
	var actionTypeID int
	query := database.ConvertPlaceholders("SELECT id FROM admin_action_type WHERE name = ?")
	if err := db.QueryRow(query, actionName).Scan(&actionTypeID); err != nil {
		return fmt.Errorf("unknown action type: %s", actionName)
	}

	// Insert log entry
	var detailsJSON interface{}
	if details != nil {
		// Convert to JSON string if needed
		detailsJSON = details
	}

	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO admin_action_log 
		(action_type_id, target_type, target_id, target_identifier, reason, details, create_time, create_by)
		VALUES (?, ?, ?, ?, ?, ?, NOW(), ?)
	`)

	_, err := db.Exec(insertQuery, actionTypeID, targetType, targetID, targetIdentifier, reason, detailsJSON, adminID)
	return err
}
