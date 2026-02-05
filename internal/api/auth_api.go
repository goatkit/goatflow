package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/service"
	"github.com/gotrs-io/gotrs-ce/internal/shared"
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
	authService := service.NewAuthService(db, getJWTManager())

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

	// Return success with tokens
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"user": gin.H{
			"id":         user.ID,
			"login":      user.Login,
			"email":      user.Email,
			"first_name": user.FirstName,
			"last_name":  user.LastName,
			"role":       user.Role,
		},
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    900, // 15 minutes in seconds
	})
}

// HandleRefreshTokenAPI refreshes an expired JWT token.
//
//	@Summary		Refresh token
//	@Description	Exchange a refresh token for a new access token
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Param			token	body		object	true	"Refresh token"
//	@Success		200		{object}	map[string]interface{}	"New access token"
//	@Failure		400		{object}	map[string]interface{}	"Invalid request"
//	@Failure		401		{object}	map[string]interface{}	"Invalid or expired refresh token"
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
	authService := service.NewAuthService(db, getJWTManager())

	// Refresh the token
	newAccessToken, err := authService.RefreshToken(refreshRequest.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Invalid or expired refresh token",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"access_token": newAccessToken,
		"token_type":   "Bearer",
		"expires_in":   900, // 15 minutes in seconds
	})
}

// HandleLogoutAPI logs out a user (client-side token removal).
//
//	@Summary		Logout
//	@Description	Logout user (invalidates session, client should discard tokens)
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"Logout successful"
//	@Security		BearerAuth
//	@Router			/auth/logout [post]
func HandleLogoutAPI(c *gin.Context) {
	// In a JWT-based system, logout is typically handled client-side
	// We could implement token blacklisting here if needed

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Successfully logged out",
	})
}

// HandleRegisterAPI registers a new user (if enabled).
//
//	@Summary		Register user
//	@Description	Register a new user (if self-registration is enabled)
//	@Tags			Authentication
//	@Accept			json
//	@Produce		json
//	@Param			user	body		object	true	"User registration data"
//	@Success		201		{object}	map[string]interface{}	"Registered user"
//	@Failure		400		{object}	map[string]interface{}	"Invalid request or registration disabled"
//	@Router			/auth/register [post]
func HandleRegisterAPI(c *gin.Context) {
	// Registration is typically disabled in OTRS-style systems
	// Users are created by administrators
	c.JSON(http.StatusNotImplemented, gin.H{
		"success": false,
		"error":   "User registration is disabled. Please contact an administrator.",
	})
}

// ExtractToken extracts the JWT token from the Authorization header or cookies.
func ExtractToken(c *gin.Context) string {
	// Check Authorization header first
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		// Check for Bearer token
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1]
		}
	}

	// Check cookies (used by admin pages)
	if cookie, err := c.Cookie("auth_token"); err == nil && cookie != "" {
		return cookie
	}
	if cookie, err := c.Cookie("access_token"); err == nil && cookie != "" {
		return cookie
	}
	if cookie, err := c.Cookie("token"); err == nil && cookie != "" {
		return cookie
	}

	return ""
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
