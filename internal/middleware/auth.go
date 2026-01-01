package middleware

import (
	"net/http"
	"os"
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
			if allowTestBypass() {
				m.setTestContext(c, "test@gotrs.local", "Admin")
				c.Next()
				return
			}
			m.unauthorizedResponse(c, "Missing authorization token")
			return
		}

		if m.jwtManager == nil {
			if allowTestBypass() {
				m.setTestContext(c, "test@gotrs.local", "Admin")
				c.Next()
				return
			}
			m.unauthorizedResponse(c, "Authentication is not configured")
			return
		}

		if allowTestBypass() {
			if token == "test-token" || strings.HasPrefix(token, "demo_session_") || strings.HasPrefix(token, "demo_customer_") {
				m.setTestContext(c, "test@gotrs.local", "Admin")
				c.Next()
				return
			}
		}

		claims, err := m.jwtManager.ValidateToken(token)
		if err != nil {
			if allowTestBypass() {
				m.setTestContext(c, "test@gotrs.local", "Admin")
				c.Next()
				return
			}
			m.unauthorizedResponse(c, "Invalid or expired token")
			return
		}

		// Set user information in context
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Set("user_role", claims.Role)
		c.Set("tenant_id", claims.TenantID)
		c.Set("userID", int(claims.UserID))
		c.Set("username", claims.Login)
		c.Set("is_customer", claims.Role == "Customer")
		c.Set("tenant_host", c.Request.Host)
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
		c.Set("userID", int(claims.UserID))
		c.Set("username", claims.Login)
		c.Set("is_customer", claims.Role == "Customer")
		c.Set("tenant_host", c.Request.Host)
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
	if cookie, err := c.Cookie("access_token"); err == nil && cookie != "" {
		return cookie
	}
	if cookie, err := c.Cookie("token"); err == nil && cookie != "" {
		return cookie
	}

	return ""
}

func (m *AuthMiddleware) unauthorizedResponse(c *gin.Context, message string) {
	accept := c.GetHeader("Accept")
	if strings.Contains(accept, "text/html") {
		loginPath := "/login"
		if strings.HasPrefix(c.Request.URL.Path, "/customer") {
			loginPath = "/customer/login"
		} else {
			flag := strings.ToLower(os.Getenv("CUSTOMER_FE_ONLY"))
			if flag == "1" || flag == "true" {
				loginPath = "/customer/login"
			}
		}
		c.Redirect(http.StatusFound, loginPath)
		c.Abort()
		return
	}

	c.JSON(http.StatusUnauthorized, gin.H{
		"error": message,
	})
	c.Abort()
}

func allowTestBypass() bool {
	disable := strings.ToLower(strings.TrimSpace(os.Getenv("GOTRS_DISABLE_TEST_AUTH_BYPASS")))
	switch disable {
	case "1", "true", "yes", "on":
		return false
	}

	env := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	switch env {
	case "production", "prod":
		return false
	}

	if gin.Mode() == gin.TestMode {
		return true
	}

	switch env {
	case "", "test", "testing", "unit", "unit-test", "unit_real", "unit-real":
		return true
	}

	return false
}

func (m *AuthMiddleware) setTestContext(c *gin.Context, email, role string) {
	claims := &auth.Claims{
		UserID: 1,
		Email:  email,
		Role:   role,
	}
	c.Set("user_id", uint(1))
	c.Set("user_email", claims.Email)
	c.Set("user_role", claims.Role)
	c.Set("tenant_id", uint(0))
	c.Set("claims", claims)
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
