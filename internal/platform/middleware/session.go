package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/convert"
	platformmodels "github.com/goatkit/goatflow/internal/platform/models"
)

// RequireRole checks if the user has the required role.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("user_role")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			c.Abort()
			return
		}

		roleStr, ok := userRole.(string)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid role"})
			c.Abort()
			return
		}

		// Check if user has one of the required roles
		for _, role := range roles {
			if roleStr == role {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		c.Abort()
	}
}

// GetCurrentUser retrieves the current user from context.
func GetCurrentUser(c *gin.Context) (uint, string, string, bool) {
	email, emailExists := c.Get("user_email")
	role, roleExists := c.Get("user_role")

	if !emailExists || !roleExists {
		return 0, "", "", false
	}

	id := getSessionUserIDFromCtxUint(c, 0)
	if id == 0 {
		return 0, "", "", false
	}

	emailStr, ok := email.(string)
	if !ok {
		return 0, "", "", false
	}

	roleStr, ok := role.(string)
	if !ok {
		return 0, "", "", false
	}

	return id, emailStr, roleStr, true
}

// getSessionUserIDFromCtxUint extracts the authenticated user's ID from gin context as uint.
func getSessionUserIDFromCtxUint(c *gin.Context, fallback uint) uint {
	v, ok := c.Get("user_id")
	if !ok {
		return fallback
	}
	return convert.ToUint(v, fallback)
}

// isAPIRequest checks if the request is for an API endpoint.
func isAPIRequest(c *gin.Context) bool {
	// Check for AJAX request header
	if c.GetHeader("X-Requested-With") == "XMLHttpRequest" {
		return true
	}
	// Check for JSON content type in Accept header
	if strings.Contains(c.GetHeader("Accept"), "application/json") {
		return true
	}
	// Check if path is an API endpoint
	return strings.HasPrefix(c.Request.URL.Path, "/api/")
}

// OptionalAuth is middleware that validates tokens if present but doesn't require them.
func OptionalAuth(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := ExtractToken(c)

		// If token found, validate it
		if token != "" {
			claims, err := jwtManager.ValidateToken(token)
			if err == nil && VerifySession(c, claims) {
				// Store user info in context
				c.Set("user_id", claims.UserID)
				c.Set("user_email", claims.Email)
				c.Set("user_role", claims.Role)
				c.Set("user_name", claims.Email) // Use email as name for now
				c.Set("username", claims.Login)  // Required by customer portal handlers
				c.Set("authenticated", true)
			}
		}

		c.Next()
	}
}

// RequirePermission checks if the user has the required permission.
func RequirePermission(rbac *auth.RBAC, permission auth.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("user_role")
		if !exists {
			if isAPIRequest(c) {
				c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			} else {
				c.Redirect(http.StatusSeeOther, "/login?error=access_denied")
			}
			c.Abort()
			return
		}

		roleStr, ok := userRole.(string)
		if !ok {
			if isAPIRequest(c) {
				c.JSON(http.StatusForbidden, gin.H{"error": "Invalid role"})
			} else {
				c.Redirect(http.StatusSeeOther, "/login?error=invalid_role")
			}
			c.Abort()
			return
		}

		// Check if user has the required permission
		if !rbac.HasPermission(roleStr, permission) {
			if isAPIRequest(c) {
				c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			} else {
				// For now, redirect to a simple error page or use JSON
				c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to access this resource"})
			}
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAnyPermission checks if the user has any of the required permissions.
func RequireAnyPermission(rbac *auth.RBAC, permissions ...auth.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("user_role")
		if !exists {
			if isAPIRequest(c) {
				c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			} else {
				c.Redirect(http.StatusSeeOther, "/login?error=access_denied")
			}
			c.Abort()
			return
		}

		roleStr, ok := userRole.(string)
		if !ok {
			if isAPIRequest(c) {
				c.JSON(http.StatusForbidden, gin.H{"error": "Invalid role"})
			} else {
				c.Redirect(http.StatusSeeOther, "/login?error=invalid_role")
			}
			c.Abort()
			return
		}

		// Check if user has any of the required permissions
		for _, permission := range permissions {
			if rbac.HasPermission(roleStr, permission) {
				c.Next()
				return
			}
		}

		if isAPIRequest(c) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		} else {
			// For now, use JSON response for web routes too
			c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to access this resource"})
		}
		c.Abort()
	}
}

// RequireTicketAccess checks if the user can access a specific ticket.
func RequireTicketAccess(rbac *auth.RBAC) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _, userRole, hasUser := GetCurrentUser(c)
		if !hasUser {
			if isAPIRequest(c) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			} else {
				c.Redirect(http.StatusSeeOther, "/login")
			}
			c.Abort()
			return
		}

		// Debug logging to understand what's happening
		// Get ticket ID from URL parameter
		ticketIDStr := c.Param("id")
		if ticketIDStr == "" {
			// If no ticket ID in URL, allow access (for list views, etc.)
			c.Next()
			return
		}

		// For admins and agents, allow access to any ticket
		// For customers, we'd need to check database ownership
		if userRole == string(platformmodels.RoleAdmin) || userRole == string(platformmodels.RoleAgent) {
			c.Next()
			return
		}

		// For customers, would need actual ticket lookup from database
		// For now, simplified implementation
		ticketOwnerID := userID // This needs proper DB lookup in production

		if !rbac.CanAccessTicket(userRole, ticketOwnerID, userID) {
			if isAPIRequest(c) {
				c.JSON(http.StatusForbidden, gin.H{"error": "Cannot access this ticket"})
			} else {
				// For now, use JSON response for web routes too
				c.JSON(http.StatusForbidden, gin.H{"error": "You don't have permission to access this ticket"})
			}
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAdminAccess is a convenience function for admin-only routes.
func RequireAdminAccess(rbac *auth.RBAC) gin.HandlerFunc {
	return RequirePermission(rbac, auth.PermissionAdminAccess)
}

// RequireAgentAccess allows both admins and agents.
func RequireAgentAccess(rbac *auth.RBAC) gin.HandlerFunc {
	return RequireAnyPermission(rbac,
		auth.PermissionAdminAccess,
		auth.PermissionTicketRead,
	)
}
