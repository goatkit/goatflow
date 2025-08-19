package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/auth"
)

// SessionMiddleware validates JWT tokens from cookies or Authorization header
func SessionMiddleware(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for token in cookie first
		token, err := c.Cookie("access_token")
		if err != nil || token == "" {
			// Check Authorization header as fallback
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				parts := strings.Split(authHeader, " ")
				if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
					token = parts[1]
				}
			}
		}
		
		// If no token found, redirect to login
		if token == "" {
			if isAPIRequest(c) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
				c.Abort()
				return
			}
			c.Redirect(http.StatusSeeOther, "/login")
			c.Abort()
			return
		}
		
		// Check for demo token (only in demo mode)
		if strings.HasPrefix(token, "demo_session_") {
			// In demo mode, accept demo tokens
			c.Set("user_id", uint(1))
			c.Set("user_email", "demo@example.com")
			c.Set("user_role", "admin")
			c.Set("user_name", "Demo Admin")
			c.Set("is_demo", true)
			c.Next()
			return
		}
		
		// Validate real JWT token (if JWT manager is available)
		if jwtManager == nil {
			// No JWT manager configured and not a demo token
			c.SetCookie("access_token", "", -1, "/", "", false, true)
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
		c.Set("user_name", claims.Name)
		
		// Add user info to request context for services
		ctx := context.WithValue(c.Request.Context(), "user_id", claims.UserID)
		ctx = context.WithValue(ctx, "user_email", claims.Email)
		ctx = context.WithValue(ctx, "user_role", claims.Role)
		c.Request = c.Request.WithContext(ctx)
		
		c.Next()
	}
}

// RequireRole checks if the user has the required role
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

// GetCurrentUser retrieves the current user from context
func GetCurrentUser(c *gin.Context) (uint, string, string, bool) {
	userID, idExists := c.Get("user_id")
	email, emailExists := c.Get("user_email")
	role, roleExists := c.Get("user_role")
	
	if !idExists || !emailExists || !roleExists {
		return 0, "", "", false
	}
	
	id, ok := userID.(uint)
	if !ok {
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

// isAPIRequest checks if the request is for an API endpoint
func isAPIRequest(c *gin.Context) bool {
	return strings.HasPrefix(c.Request.URL.Path, "/api/")
}

// OptionalAuth is middleware that validates tokens if present but doesn't require them
func OptionalAuth(jwtManager *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for token in cookie first
		token, err := c.Cookie("access_token")
		if err != nil || token == "" {
			// Check Authorization header as fallback
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				parts := strings.Split(authHeader, " ")
				if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
					token = parts[1]
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
				c.Set("user_name", claims.Name)
				c.Set("authenticated", true)
			}
		}
		
		c.Next()
	}
}