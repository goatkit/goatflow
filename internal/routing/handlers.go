package routing

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/shared"
)

// RegisterExistingHandlers registers existing handlers with the registry
func RegisterExistingHandlers(registry *HandlerRegistry) {
	// Register middleware only - all route handlers are now in YAML
	middlewares := map[string]gin.HandlerFunc{
		"auth": func(c *gin.Context) {
			// Check for token in cookie (auth_token) or Authorization header
			token, err := c.Cookie("auth_token")
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

            // If no token found, redirect for HTML requests, JSON for APIs
            if token == "" {
                accept := strings.ToLower(c.GetHeader("Accept"))
                if strings.Contains(accept, "text/html") || accept == "" {
                    // Browser navigation -> redirect to login
                    c.Redirect(http.StatusSeeOther, "/login")
                } else {
                    c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing authorization token"})
                }
                c.Abort()
                return
            }

			// Validate token
			jwtManager := shared.GetJWTManager()
			claims, err := jwtManager.ValidateToken(token)
            if err != nil {
                // Clear invalid cookie
                c.SetCookie("auth_token", "", -1, "/", "", false, true)
                accept := strings.ToLower(c.GetHeader("Accept"))
                if strings.Contains(accept, "text/html") || accept == "" {
                    c.Redirect(http.StatusSeeOther, "/login")
                } else {
                    c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
                }
                c.Abort()
                return
            }

			// Store user info in context
			c.Set("user_id", claims.UserID)
			c.Set("user_email", claims.Email)
			c.Set("user_role", claims.Role)
			c.Set("user_name", claims.Email)

			// Set is_customer based on role (for customer middleware compatibility)
			if claims.Role == "Customer" {
				c.Set("is_customer", true)
			} else {
				c.Set("is_customer", false)
			}

			c.Next()
		},

		"auth-optional": func(c *gin.Context) {
			c.Next()
		},

		"admin": func(c *gin.Context) {
			role, exists := c.Get("user_role")
			if !exists || role != "Admin" {
				c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
				c.Abort()
				return
			}
			c.Next()
		},

		"agent": func(c *gin.Context) {
			role, exists := c.Get("user_role")
			if !exists || (role != "Agent" && role != "Admin") {
				c.JSON(http.StatusForbidden, gin.H{"error": "Agent access required"})
				c.Abort()
				return
			}
			c.Next()
		},

		"customer": func(c *gin.Context) {
			isCustomer, exists := c.Get("is_customer")
			if !exists || !isCustomer.(bool) {
				c.JSON(http.StatusForbidden, gin.H{"error": "Customer access required"})
				c.Abort()
				return
			}
			c.Next()
		},

		"audit": func(c *gin.Context) {
			c.Next()
		},
	}

	// Register all middleware
	for name, handler := range middlewares {
		registry.RegisterMiddleware(name, handler)
	}
}

// RegisterAPIHandlers registers API handlers with the registry
func RegisterAPIHandlers(registry *HandlerRegistry, apiHandlers map[string]gin.HandlerFunc) {
	// Override existing handlers with API handlers
	registry.OverrideBatch(apiHandlers)
}
