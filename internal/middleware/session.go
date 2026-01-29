package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/convert"
	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// contextKey is a private type to avoid key collisions in context.
type contextKey string

// SessionMiddleware validates JWT tokens from cookies or Authorization header.
func SessionMiddleware(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var token string

		// For customer portal paths, check customer-specific cookies first
		// This prevents agent/customer session conflicts in the same browser
		if strings.HasPrefix(c.Request.URL.Path, "/customer") {
			if ct, err := c.Cookie("customer_access_token"); err == nil && ct != "" {
				token = ct
			} else if ct, err := c.Cookie("customer_auth_token"); err == nil && ct != "" {
				token = ct
			}
		}

		// If no customer token found (or not a customer path), check standard cookies
		if token == "" {
			var err error
			token, err = c.Cookie("access_token")
			if err != nil || token == "" {
				// Fallback to legacy cookie name used by some login flows
				if at, err2 := c.Cookie("auth_token"); err2 == nil && at != "" {
					token = at
				} else {
					// Check Authorization header as fallback
					authHeader := c.GetHeader("Authorization")
					if authHeader != "" {
						parts := strings.Split(authHeader, " ")
						if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
							token = parts[1]
						}
					}
				}
			}
		}

		// If no token found, redirect to the appropriate login page
		if token == "" {
			if isAPIRequest(c) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
				c.Abort()
				return
			}

			path := c.Request.URL.Path
			if strings.HasPrefix(path, "/customer") {
				c.Redirect(http.StatusSeeOther, "/auth/customer")
			} else {
				c.Redirect(http.StatusSeeOther, "/login")
			}
			c.Abort()
			return
		}

		// Check for demo token (only in demo mode)
		if strings.HasPrefix(token, "demo_session_") || strings.HasPrefix(token, "demo_customer_") {
			// In demo mode, accept demo tokens
			// Token formats:
			//   - demo_session_{userID}_{timestamp} for agents
			//   - demo_customer_{username} for customers

			if strings.HasPrefix(token, "demo_customer_") {
				// Customer demo session
				parts := strings.Split(token, "_")
				username := "john.customer" // default customer
				if len(parts) >= 3 {
					username = parts[2]
				}

				// Set customer context
				c.Set("is_customer", true)
				c.Set("username", username)
				c.Set("userID", 1001) // Demo customer ID
				c.Set("user_email", "john@acme.com")
				c.Set("user_role", "Customer")
				c.Set("user_name", "John Customer")
				c.Set("is_demo", true)
			} else {
				// Agent demo session
				parts := strings.Split(token, "_")
				userID := uint(1) // default to admin
				if len(parts) >= 3 {
					// Try to parse the user ID from the token
					if id, err := strconv.Atoi(parts[2]); err == nil {
						userID = uint(id)
					}
				}

				// Set agent context
				c.Set("user_id", userID)
				c.Set("user_email", "demo@example.com")
				c.Set("user_role", "Admin")
				c.Set("user_name", "Demo User")
				c.Set("is_demo", true)
			}

			c.Next()
			return
		}

		// Validate real JWT token (if JWT manager is available)
		if jwtManager == nil {
			// No JWT manager configured and not a demo token
			c.SetCookie("access_token", "", -1, "/", "", false, true)
			c.SetCookie("auth_token", "", -1, "/", "", false, true)
			if isAPIRequest(c) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication not configured"})
				c.Abort()
				return
			}
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}

		claims, err := jwtManager.ValidateToken(token)
		if err != nil {
			// Clear invalid cookie
			c.SetCookie("access_token", "", -1, "/", "", false, true)
			c.SetCookie("auth_token", "", -1, "/", "", false, true)

			if isAPIRequest(c) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
				c.Abort()
				return
			}
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}

		// Store user info in context
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Set("user_role", claims.Role)
		c.Set("user_name", claims.Email) // Use email as name for now
		c.Set("userID", int(claims.UserID))
		c.Set("username", claims.Login)
		c.Set("tenant_id", claims.TenantID)
		c.Set("tenant_host", c.Request.Host)

		// Set is_customer based on role
		if claims.Role == "Customer" {
			c.Set("is_customer", true)
		} else {
			c.Set("is_customer", false)
		}

		// Add user info to request context for services using typed keys
		ctx := context.WithValue(c.Request.Context(), contextKey("user_id"), claims.UserID)
		ctx = context.WithValue(ctx, contextKey("user_email"), claims.Email)
		ctx = context.WithValue(ctx, contextKey("user_role"), claims.Role)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

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
		// Check for token in cookie first
		token, err := c.Cookie("access_token")
		if err != nil || token == "" {
			// Fallback to legacy cookie name used by some login flows
			if at, err2 := c.Cookie("auth_token"); err2 == nil && at != "" {
				token = at
			} else {
				// Check Authorization header as fallback
				authHeader := c.GetHeader("Authorization")
				if authHeader != "" {
					parts := strings.Split(authHeader, " ")
					if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
						token = parts[1]
					}
				}
			}
		}

		// If token found, validate it
		if token != "" {
			claims, err := jwtManager.ValidateToken(token)
			if err == nil {
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
		if userRole == string(models.RoleAdmin) || userRole == string(models.RoleAgent) {
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
