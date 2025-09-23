package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/auth"
)

type AuthMiddleware struct {
	jwtManager *auth.JWTManager
	rbac       *auth.RBAC
}

func NewAuthMiddleware(jwtManager *auth.JWTManager) *AuthMiddleware {
	return &AuthMiddleware{
		jwtManager: jwtManager,
		rbac:       auth.NewRBAC(),
	}
}

func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := m.extractToken(c)
		if token == "" {
			// Check if this is a web page request (accepts HTML) vs API request
			accept := c.GetHeader("Accept")
			if strings.Contains(accept, "text/html") {
				// Web page request - redirect to login
				c.Redirect(http.StatusFound, "/login")
				c.Abort()
				return
			}
			// API request - return JSON error
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Missing authorization token",
			})
			c.Abort()
			return
		}

		claims, err := m.jwtManager.ValidateToken(token)
		if err != nil {
			// Check if this is a web page request (accepts HTML) vs API request
			accept := c.GetHeader("Accept")
			if strings.Contains(accept, "text/html") {
				// Web page request - redirect to login
				c.Redirect(http.StatusFound, "/login")
				c.Abort()
				return
			}
			// API request - return JSON error
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
			})
			c.Abort()
			return
		}

		// Set user information in context
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Set("user_role", claims.Role)
		c.Set("tenant_id", claims.TenantID)
		c.Set("claims", claims)

		c.Next()
	}
}

func (m *AuthMiddleware) RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// First ensure user is authenticated
		userRole, exists := c.Get("user_role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User not authenticated",
			})
			c.Abort()
			return
		}

		// Check if user has required role
		roleStr := userRole.(string)
		for _, role := range roles {
			if roleStr == role {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"error": "Insufficient permissions",
		})
		c.Abort()
	}
}

func (m *AuthMiddleware) RequirePermission(permission auth.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		// First ensure user is authenticated
		userRole, exists := c.Get("user_role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User not authenticated",
			})
			c.Abort()
			return
		}

		// Check if user has required permission
		if !m.rbac.HasPermission(userRole.(string), permission) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Insufficient permissions",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func (m *AuthMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := m.extractToken(c)
		if token == "" {
			// No token provided, continue without authentication
			c.Next()
			return
		}

		claims, err := m.jwtManager.ValidateToken(token)
		if err != nil {
			// Invalid token, continue without authentication
			c.Next()
			return
		}

		// Set user information in context
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Set("user_role", claims.Role)
		c.Set("tenant_id", claims.TenantID)
		c.Set("claims", claims)
		c.Set("authenticated", true)

		c.Next()
	}
}

func (m *AuthMiddleware) extractToken(c *gin.Context) string {
	// Check Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		// Bearer token format: "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1]
		}
	}

	// Check query parameter (useful for WebSocket connections)
	if token := c.Query("token"); token != "" {
		return token
	}

	// Check cookie
	if cookie, err := c.Cookie("auth_token"); err == nil && cookie != "" {
		return cookie
	}

	return ""
}

func (m *AuthMiddleware) IsAuthenticated(c *gin.Context) bool {
	_, exists := c.Get("user_id")
	return exists
}

func (m *AuthMiddleware) GetUserID(c *gin.Context) (uint, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	return userID.(uint), true
}

func (m *AuthMiddleware) GetUserRole(c *gin.Context) (string, bool) {
	role, exists := c.Get("user_role")
	if !exists {
		return "", false
	}
	return role.(string), true
}

func (m *AuthMiddleware) CanAccessTicket(c *gin.Context, ticketOwnerID uint) bool {
	role, roleExists := m.GetUserRole(c)
	userID, userExists := m.GetUserID(c)
	
	if !roleExists || !userExists {
		return false
	}
	
	return m.rbac.CanAccessTicket(role, ticketOwnerID, userID)
}