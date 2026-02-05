package middleware

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/apierrors"
	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// debugLog logs only when LOG_LEVEL=debug
func debugLog(format string, v ...interface{}) {
	if os.Getenv("LOG_LEVEL") == "debug" {
		log.Printf(format, v...)
	}
}

// APITokenVerifier is the interface for verifying API tokens.
// This breaks the import cycle between api and middleware packages.
type APITokenVerifier interface {
	VerifyToken(ctx context.Context, rawToken string) (*models.APIToken, error)
	UpdateLastUsed(ctx context.Context, tokenID int64, ip string) error
}

// Global token verifier - set by api package during init
var tokenVerifier APITokenVerifier

// SetAPITokenVerifier sets the global token verifier
func SetAPITokenVerifier(v APITokenVerifier) {
	tokenVerifier = v
}

// IsAPIToken checks if a token string is a GoatKit API token (gf_ prefix)
func IsAPIToken(token string) bool {
	return strings.HasPrefix(token, models.TokenPrefix)
}

// APITokenAuthMiddleware authenticates requests using GoatKit API tokens (gf_*).
// Sets user context similar to JWT auth for compatibility with existing handlers.
func APITokenAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			apierrors.Error(c, apierrors.CodeUnauthorized)
			c.Abort()
			return
		}

		// Only handle gf_ tokens
		if !IsAPIToken(token) {
			apierrors.ErrorWithMessage(c, apierrors.CodeInvalidToken, "Expected API token (gf_*)")
			c.Abort()
			return
		}

		if tokenVerifier == nil {
			apierrors.Error(c, apierrors.CodeServiceUnavailable)
			c.Abort()
			return
		}

		// Verify the token
		apiToken, err := tokenVerifier.VerifyToken(c.Request.Context(), token)
		if err != nil {
			errMsg := err.Error()
			switch {
			case strings.Contains(errMsg, "expired"):
				apierrors.Error(c, apierrors.CodeTokenExpired)
			case strings.Contains(errMsg, "revoked"):
				apierrors.Error(c, apierrors.CodeTokenRevoked)
			default:
				apierrors.Error(c, apierrors.CodeInvalidToken)
			}
			c.Abort()
			return
		}

		// Update last used asynchronously
		go func() {
			_ = tokenVerifier.UpdateLastUsed(c.Request.Context(), apiToken.ID, c.ClientIP())
		}()

		// Set user context
		c.Set("user_id", apiToken.UserID)
		c.Set("api_token", apiToken)
		c.Set("api_token_id", apiToken.ID)
		c.Set("api_token_scopes", apiToken.Scopes)

		if apiToken.UserType == models.APITokenUserAgent {
			c.Set("user_role", "User")
		} else {
			c.Set("user_role", "Customer")
			c.Set("customer_user_id", apiToken.UserID)
		}

		c.Next()
	}
}

// UnifiedAuthMiddleware handles both JWT tokens and API tokens (gf_*).
func UnifiedAuthMiddleware(jwtManager interface{ ValidateToken(string) (*auth.Claims, error) }) gin.HandlerFunc {
	debugLog("DEBUG: UnifiedAuthMiddleware created")
	return func(c *gin.Context) {
		debugLog("DEBUG unified_auth: processing request %s %s", c.Request.Method, c.Request.URL.Path)
		token := extractToken(c)
		if token == "" {
			debugLog("DEBUG unified_auth: no token found")
			apierrors.Error(c, apierrors.CodeUnauthorized)
			c.Abort()
			return
		}

		debugLog("DEBUG unified_auth: token found, isAPIToken=%v", IsAPIToken(token))
		if IsAPIToken(token) {
			authenticateAPIToken(c, token)
		} else {
			authenticateJWT(c, token, jwtManager)
		}
	}
}

// authenticateAPIToken handles gf_* token authentication
func authenticateAPIToken(c *gin.Context, token string) {
	debugLog("DEBUG api_token: authenticating gf_* token (prefix: %s...)", token[:min(15, len(token))])

	if tokenVerifier == nil {
		debugLog("DEBUG api_token: tokenVerifier is nil!")
		apierrors.Error(c, apierrors.CodeServiceUnavailable)
		c.Abort()
		return
	}

	apiToken, err := tokenVerifier.VerifyToken(c.Request.Context(), token)
	if err != nil {
		debugLog("DEBUG api_token: verification failed: %v", err)
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, "expired"):
			apierrors.Error(c, apierrors.CodeTokenExpired)
		case strings.Contains(errMsg, "revoked"):
			apierrors.Error(c, apierrors.CodeTokenRevoked)
		default:
			apierrors.Error(c, apierrors.CodeInvalidToken)
		}
		c.Abort()
		return
	}
	debugLog("DEBUG api_token: verified token id=%d user_id=%d", apiToken.ID, apiToken.UserID)

	go func() {
		_ = tokenVerifier.UpdateLastUsed(c.Request.Context(), apiToken.ID, c.ClientIP())
	}()

	c.Set("user_id", apiToken.UserID)
	c.Set("api_token", apiToken)
	c.Set("api_token_id", apiToken.ID)
	c.Set("api_token_scopes", apiToken.Scopes)

	if apiToken.UserType == models.APITokenUserAgent {
		c.Set("user_role", "User")
	} else {
		c.Set("user_role", "Customer")
		c.Set("customer_user_id", apiToken.UserID)
		if apiToken.CustomerLogin != "" {
			c.Set("customer_login", apiToken.CustomerLogin)
		}
	}

	c.Next()
}

// authenticateJWT handles standard JWT token authentication
func authenticateJWT(c *gin.Context, token string, jwtManager interface{ ValidateToken(string) (*auth.Claims, error) }) {
	claims, err := jwtManager.ValidateToken(token)
	if err != nil {
		apierrors.Error(c, apierrors.CodeInvalidToken)
		c.Abort()
		return
	}

	c.Set("user_id", int(claims.UserID))
	c.Set("user_email", claims.Email)
	c.Set("user_role", claims.Role)
	c.Set("claims", claims)
	c.Set("isInAdminGroup", claims.IsAdmin)

	c.Next()
}

// RequireScope middleware checks that the API token has the required scope.
// It also enforces AgentOnly and RequireRole restrictions from the scope definition.
func RequireScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, exists := c.Get("api_token"); !exists {
			c.Next()
			return
		}

		apiToken, _ := c.Get("api_token")
		token, ok := apiToken.(*models.APIToken)
		if !ok {
			apierrors.Error(c, apierrors.CodeInternalError)
			c.Abort()
			return
		}

		// Check if token has the scope
		if !token.HasScope(scope) {
			apierrors.ErrorWithMessage(c, apierrors.CodeForbidden, "Token missing required scope: "+scope)
			c.Abort()
			return
		}

		// Check scope restrictions (AgentOnly, RequireRole)
		userRole, _ := c.Get("user_role")
		roleStr, _ := userRole.(string)
		isCustomer := token.UserType == models.APITokenUserCustomer

		if !models.IsScopeAllowed(scope, roleStr, isCustomer) {
			if isCustomer {
				apierrors.ErrorWithMessage(c, apierrors.CodeForbidden, "This endpoint is not available to customers")
			} else {
				apierrors.ErrorWithMessage(c, apierrors.CodeForbidden, "Insufficient role for scope: "+scope)
			}
			c.Abort()
			return
		}

		c.Next()
	}
}

// extractToken extracts token from Authorization header or cookies
func extractToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1]
		}
		// Also accept raw API tokens without "Bearer " prefix (convenience for Swagger UI)
		if len(parts) == 1 && IsAPIToken(parts[0]) {
			return parts[0]
		}
	}

	if cookie, err := c.Cookie("auth_token"); err == nil && cookie != "" {
		return cookie
	}
	if cookie, err := c.Cookie("access_token"); err == nil && cookie != "" {
		return cookie
	}

	return ""
}
