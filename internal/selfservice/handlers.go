package selfservice

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/mailqueue"
)

// HandleForgotPassword renders the forgot password form.
// Title is set via i18n in the template using t("self_service.forgot_password.title").
func HandleForgotPassword(c *gin.Context) {
	c.HTML(http.StatusOK, "pages/forgot_password.pongo2", gin.H{})
}

// HandleForgotPasswordSubmit processes the forgot password form.
// Generates a reset token and sends an email with the reset link.
func HandleForgotPasswordSubmit(captchaCfg *CAPTCHAConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		email := c.PostForm("email")
		if email == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email is required"})
			return
		}

		// Verify CAPTCHA if configured.
		if err := VerifyCAPTCHA(captchaCfg, c.PostForm("captcha_token")); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "CAPTCHA verification failed"})
			return
		}

		repo, err := NewRepository()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Service unavailable"})
			return
		}

		// Generate token regardless of whether email exists (prevent enumeration).
		token, err := GenerateToken()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Service unavailable"})
			return
		}

		// Check if email belongs to a customer.
		db, _ := database.GetDB()
		var customerLogin string
		if db != nil {
			db.QueryRow(database.ConvertPlaceholders(
				"SELECT login FROM customer_user WHERE email = ? AND valid_id = 1"), email).Scan(&customerLogin)
		}

		if customerLogin != "" {
			authToken := &AuthToken{
				Token:         token,
				TokenType:     TokenPasswordReset,
				UserType:      UserCustomer,
				CustomerLogin: &customerLogin,
				Email:         email,
				ExpiresAt:     time.Now().Add(DefaultTokenExpiry),
				CreatedAt:     time.Now(),
			}
			repo.CreateToken(authToken)

			// Send reset email.
			sendPasswordResetEmail(db, email, token)
		}

		// Always return success (anti-enumeration).
		c.JSON(http.StatusOK, gin.H{
			"message": "If an account exists with that email, a password reset link has been sent.",
		})
	}
}

// HandleResetPassword processes the password reset form.
func HandleResetPassword(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		token = c.Param("token")
	}

	repo, err := NewRepository()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Service unavailable"})
		return
	}

	authToken, err := repo.GetToken(token)
	if err != nil || authToken == nil || !authToken.IsValid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired reset link"})
		return
	}

	// GET: show reset form.
	if c.Request.Method == "GET" {
		c.HTML(http.StatusOK, "pages/reset_password.pongo2", gin.H{
			"Token": token,
		})
		return
	}

	// POST: process password change.
	password := c.PostForm("password")
	confirmPassword := c.PostForm("confirm_password")
	if password == "" || len(password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 8 characters"})
		return
	}
	if password != confirmPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Passwords do not match"})
		return
	}

	// Hash password.
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	// Update password in database.
	db, _ := database.GetDB()
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database unavailable"})
		return
	}

	if authToken.UserType == UserCustomer && authToken.CustomerLogin != nil {
		_, err = db.Exec(database.ConvertPlaceholders(
			"UPDATE customer_user SET pw = ?, change_time = NOW(), change_by = 1 WHERE login = ?"),
			string(hashed), *authToken.CustomerLogin)
	} else if authToken.UserType == UserAgent && authToken.UserID != nil {
		_, err = db.Exec(database.ConvertPlaceholders(
			"UPDATE users SET pw = ?, change_time = NOW(), change_by = 1 WHERE id = ?"),
			string(hashed), *authToken.UserID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	// Consume the token.
	repo.ConsumeToken(token)

	c.JSON(http.StatusOK, gin.H{"message": "Password has been reset successfully"})
}

// HandleCustomerRegister processes customer self-registration.
func HandleCustomerRegister(captchaCfg *CAPTCHAConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Verify CAPTCHA.
		if err := VerifyCAPTCHA(captchaCfg, c.PostForm("captcha_token")); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "CAPTCHA verification failed"})
			return
		}

		email := c.PostForm("email")
		firstName := c.PostForm("first_name")
		lastName := c.PostForm("last_name")

		if email == "" || firstName == "" || lastName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email, first name, and last name are required"})
			return
		}

		repo, err := NewRepository()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Service unavailable"})
			return
		}

		// Generate approval token.
		approvalToken, _ := GenerateToken()

		req := &RegistrationRequest{
			Email:         email,
			FirstName:     firstName,
			LastName:      lastName,
			Status:        StatusPending,
			ApprovalToken: &approvalToken,
			CreatedAt:     time.Now(),
		}

		_, err = repo.CreateRegistration(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit registration"})
			return
		}

		// Send verification email to the user.
		db, _ := database.GetDB()
		sendVerificationEmail(db, email, approvalToken)

		c.JSON(http.StatusOK, gin.H{
			"message": "Registration submitted. Please check your email to verify your address.",
		})
	}
}

// HandleVerifyEmail processes the email verification link.
func HandleVerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		token = c.Param("token")
	}

	repo, err := NewRepository()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Service unavailable"})
		return
	}

	authToken, err := repo.GetToken(token)
	if err != nil || authToken == nil || !authToken.IsValid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired verification link"})
		return
	}

	if authToken.TokenType != TokenEmailVerify {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid token type"})
		return
	}

	// Consume the token.
	repo.ConsumeToken(token)

	c.JSON(http.StatusOK, gin.H{"message": "Email verified successfully. An admin will review your registration."})
}

// HandleAdminApproveRegistration approves a pending registration.
func HandleAdminApproveRegistration(c *gin.Context) {
	repo, err := NewRepository()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Service unavailable"})
		return
	}

	var req struct {
		ID int64 `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Registration ID is required"})
		return
	}

	userID := 1
	if id, exists := c.Get("user_id"); exists {
		if uid, ok := id.(int); ok {
			userID = uid
		}
	}

	if err := repo.ApproveRegistration(req.ID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "approved"})
}

// HandleAdminRejectRegistration rejects a pending registration.
func HandleAdminRejectRegistration(c *gin.Context) {
	repo, err := NewRepository()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Service unavailable"})
		return
	}

	var req struct {
		ID     int64  `json:"id"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Registration ID is required"})
		return
	}

	userID := 1
	if id, exists := c.Get("user_id"); exists {
		if uid, ok := id.(int); ok {
			userID = uid
		}
	}

	if err := repo.RejectRegistration(req.ID, req.Reason, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "rejected"})
}

// HandleAdminListPendingRegistrations lists pending registrations.
func HandleAdminListPendingRegistrations(c *gin.Context) {
	repo, err := NewRepository()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Service unavailable"})
		return
	}

	reqs, err := repo.ListPendingRegistrations()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"registrations": reqs})
}

// --- Email helpers ---

func sendPasswordResetEmail(db interface{}, email, token string) {
	resetURL := fmt.Sprintf("/reset-password?token=%s", token)
	body := fmt.Sprintf(
		"You requested a password reset.\n\nClick the link below to reset your password:\n%s\n\nThis link expires in 1 hour.\n\nIf you did not request this, ignore this email.",
		resetURL,
	)

	slog.Info("password reset email queued", "email", email, "reset_url", resetURL)
	queueEmail(email, "Password Reset", body)
}

func sendVerificationEmail(db interface{}, email, token string) {
	verifyURL := fmt.Sprintf("/verify-email?token=%s", token)
	body := fmt.Sprintf(
		"Please verify your email address by clicking the link below:\n%s\n\nThis link expires in 24 hours.",
		verifyURL,
	)

	slog.Info("verification email queued", "email", email)
	queueEmail(email, "Verify Your Email", body)
}

func queueEmail(to, subject, body string) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		slog.Warn("cannot queue email — no database", "to", to, "subject", subject)
		return
	}

	repo := mailqueue.NewMailQueueRepository(db)
	msg := mailqueue.BuildEmailMessage("noreply@goatflow.local", to, subject, body)

	item := &mailqueue.MailQueueItem{
		ArticleID:  nil,
		Recipient:  to,
		RawMessage: msg,
	}
	ctx := context.Background()
	if err := repo.Insert(ctx, item); err != nil {
		slog.Warn("failed to queue email", "to", to, "error", err)
	}
}
