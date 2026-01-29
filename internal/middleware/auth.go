package middleware

import (
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/convert"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/service"
)

// Session service singleton for middleware (avoids import cycle with shared package)
var (
	middlewareSessionService *service.SessionService
	middlewareSessionOnce    sync.Once
)

func getMiddlewareSessionService() *service.SessionService {
	middlewareSessionOnce.Do(func() {
		db, err := database.GetDB()
		if err != nil {
			return
		}
		repo := repository.NewSessionRepository(db)
		middlewareSessionService = service.NewSessionService(repo)
	})
	return middlewareSessionService
}

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
			m.unauthorizedResponse(c, "Missing authorization token")
			return
		}

		if m.jwtManager == nil {
			m.unauthorizedResponse(c, "Authentication is not configured")
			return
		}

		claims, err := m.jwtManager.ValidateToken(token)
		if err != nil {
			m.unauthorizedResponse(c, "Invalid or expired token")
			return
		}

		// Validate session exists in database (session was not killed)
		// Check for customer-specific session cookie first for /customer paths
		isCustomerPath := strings.HasPrefix(c.Request.URL.Path, "/customer")
		sessionID, cookieErr := c.Cookie("session_id")
		if isCustomerPath {
			if custSessionID, err := c.Cookie("customer_session_id"); err == nil && custSessionID != "" {
				sessionID = custSessionID
				cookieErr = nil
			}
		}
		log.Printf("DEBUG: auth middleware - session_id cookie: '%s', err: %v", sessionID, cookieErr)
		if cookieErr == nil && sessionID != "" {
			sessionSvc := getMiddlewareSessionService()
			log.Printf("DEBUG: auth middleware - sessionSvc nil? %v", sessionSvc == nil)
			if sessionSvc != nil {
				session, err := sessionSvc.GetSession(sessionID)
				log.Printf("DEBUG: auth middleware - GetSession result: session=%v, err=%v", session != nil, err)
				if err != nil || session == nil {
					// Session was killed or doesn't exist - clear cookies and reject
					log.Printf("DEBUG: auth middleware - session terminated, rejecting request")
					c.SetCookie("auth_token", "", -1, "/", "", false, true)
					c.SetCookie("access_token", "", -1, "/", "", false, true)
					c.SetCookie("session_id", "", -1, "/", "", false, true)
					// Also clear customer-specific cookies
					c.SetCookie("customer_auth_token", "", -1, "/", "", false, true)
					c.SetCookie("customer_access_token", "", -1, "/", "", false, true)
					c.SetCookie("customer_session_id", "", -1, "/", "", false, true)
					m.unauthorizedResponse(c, "Session has been terminated")
					return
				}
				// Update last request time for session activity tracking
				_ = sessionSvc.TouchSession(sessionID)
			}
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

	// For customer portal paths, check customer-specific cookies first
	// This prevents agent/customer session conflicts in the same browser
	if strings.HasPrefix(c.Request.URL.Path, "/customer") {
		if cookie, err := c.Cookie("customer_auth_token"); err == nil && cookie != "" {
			return cookie
		}
		if cookie, err := c.Cookie("customer_access_token"); err == nil && cookie != "" {
			return cookie
		}
	}

	// Check standard cookies (used by agent portal)
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

func (m *AuthMiddleware) IsAuthenticated(c *gin.Context) bool {
	_, exists := c.Get("user_id")
	return exists
}

func (m *AuthMiddleware) GetUserID(c *gin.Context) (uint, bool) {
	if _, exists := c.Get("user_id"); !exists {
		return 0, false
	}
	return getUserIDFromCtxUint(c, 0), true
}

// getUserIDFromCtxUint extracts the authenticated user's ID from gin context as uint.
func getUserIDFromCtxUint(c *gin.Context, fallback uint) uint {
	v, ok := c.Get("user_id")
	if !ok {
		return fallback
	}
	return convert.ToUint(v, fallback)
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
