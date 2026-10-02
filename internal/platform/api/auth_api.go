package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/middleware"
	platformmodels "github.com/goatkit/goatflow/internal/platform/models"
	"github.com/goatkit/goatflow/internal/platform/service"
	"github.com/goatkit/goatflow/internal/platform/shared"
)

var globalJWTManager *auth.JWTManager

// getJWTManager returns a singleton JWT manager.
func getJWTManager() *auth.JWTManager {
	if globalJWTManager == nil {
		globalJWTManager = shared.GetJWTManager()
	}
	return globalJWTManager
}

// HandleLoginAPI authenticates a user and returns JWT tokens.
//
//	@Summary		Login
//	@Description	Authenticate user with login/password and return JWT tokens
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Param			credentials	body		object	true	"Login credentials (login, password)"
//	@Success		200			{object}	map[string]interface{}	"JWT tokens (access_token, refresh_token)"
//	@Failure		400			{object}	map[string]interface{}	"Invalid request"
//	@Failure		401			{object}	map[string]interface{}	"Invalid credentials"
//	@Failure		403			{object}	map[string]interface{}	"Account disabled, or a second factor is enabled (use an API token)"
//	@Router			/auth/login [post]
func HandleLoginAPI(c *gin.Context) {
	var loginRequest struct {
		Login    string `json:"login" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&loginRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid login request: " + err.Error(),
		})
		return
	}

	// Server-side rate limiting (fail2ban style)
	clientIP := c.ClientIP()
	if blocked, remaining := auth.DefaultLoginRateLimiter.IsBlocked(clientIP, loginRequest.Login); blocked {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"success":         false,
			"error":           fmt.Sprintf("too many failed attempts, try again in %d seconds", int(remaining.Seconds())),
			"retry_after_sec": int(remaining.Seconds()),
		})
		return
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Database unavailable",
		})
		return
	}

	// Create auth service
	authService := service.NewAuthService(db, getJWTManager(), auth.GetOIDCClient(), auth.GetStateStore())

	// Authenticate user
	user, accessToken, refreshToken, err := authService.Login(context.Background(), loginRequest.Login, loginRequest.Password)
	if err != nil {
		auth.DefaultLoginRateLimiter.RecordFailure(clientIP, loginRequest.Login)
		if err == auth.ErrInvalidCredentials {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Invalid credentials",
			})
		} else if err == auth.ErrUserDisabled {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   "User account is disabled",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Authentication failed",
			})
		}
		return
	}

	// Clear rate limit on successful login
	auth.DefaultLoginRateLimiter.RecordSuccess(clientIP, loginRequest.Login)

	// The web login completes the second factor on /login/2fa before it
	// issues cookies; this endpoint has no second step, so a password alone
	// must not turn into tokens for an account that enrolled one. A failed
	// lookup refuses the login rather than skipping the factor.
	mfaRequired, err := secondFactorRequired(db, user)
	if err != nil {
		log.Printf("api login: second-factor status for %s unavailable: %v", loginRequest.Login, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Login temporarily unavailable",
		})
		return
	}
	if mfaRequired {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "This account requires a second factor. Sign in through the web UI, or use an API token.",
			"code":    "mfa_required",
		})
		return
	}

	c.JSON(http.StatusOK, tokenPairResponse(user, accessToken, refreshToken))
}

// secondFactorRequired reports whether the account has TOTP or a passkey
// enrolled (the same status the web login gates on). Accounts with no users
// row (ID 0, static providers) cannot enrol one.
func secondFactorRequired(db *sql.DB, user *platformmodels.User) (bool, error) {
	totp := service.NewTOTPService(db, "GoatFlow")
	if user.Role == "Customer" {
		enabled, _, err := totp.LoginStatusForCustomer(user.Login)
		if err != nil || enabled {
			return enabled, err
		}
		n, err := service.CountWebAuthnCredentials(db, service.WebAuthnUserTypeCustomer, user.Login)
		return n > 0, err
	}
	if user.ID == 0 {
		return false, nil
	}
	userID := int(user.ID) // #nosec G115 -- users.id is a positive auto-increment key
	enabled, _, err := totp.LoginStatus(userID)
	if err != nil || enabled {
		return enabled, err
	}
	n, err := service.CountWebAuthnCredentials(db, service.WebAuthnUserTypeAgent, service.AgentWebAuthnUserKey(userID))
	return n > 0, err
}

// tokenPairResponse is the body returned by login and refresh.
func tokenPairResponse(user *platformmodels.User, accessToken, refreshToken string) gin.H {
	jwtManager := getJWTManager()
	return gin.H{
		"success": true,
		"user": gin.H{
			"id":         user.ID,
			"login":      user.Login,
			"email":      user.Email,
			"first_name": user.FirstName,
			"last_name":  user.LastName,
			"role":       user.Role,
		},
		"access_token":       accessToken,
		"refresh_token":      refreshToken,
		"token_type":         "Bearer",
		"expires_in":         int(jwtManager.TokenDuration().Seconds()),
		"refresh_expires_in": int(jwtManager.RefreshTokenDuration().Seconds()),
	}
}

// HandleRefreshTokenAPI exchanges a refresh token for a new access token and a
// new (rotated) refresh token.
//
//	@Summary		Refresh token
//	@Description	Exchange a refresh token (from login or a previous refresh) for a new access token and refresh token. The account is reloaded, so role and admin flag are current; disabled or deleted accounts are rejected.
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Param			token	body		object	true	"Refresh token: {\"refresh_token\": \"...\"}"
//	@Success		200		{object}	map[string]interface{}	"Same body as login: user, access_token, refresh_token, token_type, expires_in, refresh_expires_in"
//	@Failure		400		{object}	map[string]interface{}	"Missing refresh_token"
//	@Failure		401		{object}	map[string]interface{}	"Invalid, expired or revoked-account refresh token"
//	@Router			/auth/refresh [post]
func HandleRefreshTokenAPI(c *gin.Context) {
	var refreshRequest struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&refreshRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid refresh request: " + err.Error(),
		})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Database unavailable",
		})
		return
	}

	authService := service.NewAuthService(db, getJWTManager(), auth.GetOIDCClient(), auth.GetStateStore())
	user, accessToken, refreshToken, err := authService.Refresh(c.Request.Context(), refreshRequest.RefreshToken)
	if err != nil {
		if errors.Is(err, service.ErrRefreshRejected) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Invalid or expired refresh token",
			})
			return
		}
		log.Printf("auth refresh: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Token refresh failed",
		})
		return
	}

	c.JSON(http.StatusOK, tokenPairResponse(user, accessToken, refreshToken))
}

// HandleLogoutAPI logs out a user (client-side token removal). No YAML route
// uses it.
func HandleLogoutAPI(c *gin.Context) {
	// In a JWT-based system, logout is typically handled client-side
	// We could implement token blacklisting here if needed

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Successfully logged out",
	})
}

// ExtractToken delegates to the canonical middleware.ExtractToken.
func ExtractToken(c *gin.Context) string {
	return middleware.ExtractToken(c)
}

// JWTAuthMiddleware is a middleware that requires JWT authentication.
func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := ExtractToken(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Missing authorization token",
			})
			c.Abort()
			return
		}

		// Validate the token
		claims, err := getJWTManager().ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Invalid or expired token",
			})
			c.Abort()
			return
		}

		// Set user information in context
		c.Set("user_id", int(claims.UserID))
		c.Set("user_email", claims.Email)
		c.Set("user_role", claims.Role)
		c.Set("claims", claims)
		c.Set("isInAdminGroup", claims.IsAdmin)

		c.Next()
	}
}
