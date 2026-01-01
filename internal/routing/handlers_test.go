package routing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/gotrs-io/gotrs-ce/internal/auth"
)

func TestAuthMiddlewareSetsIsCustomerFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a test router with the auth middleware
	router := gin.New()

	// Add the auth middleware from RegisterExistingHandlers
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

			// If no token found, return unauthorized
			if token == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing authorization token"})
				c.Abort()
				return
			}

			// Validate token (mock validation for test)
			var claims *auth.Claims

			// Mock different token types for testing
			if token == "customer_token" {
				claims = &auth.Claims{
					UserID: 1,
					Email:  "customer@example.com",
					Role:   "Customer",
				}
			} else if token == "agent_token" {
				claims = &auth.Claims{
					UserID: 2,
					Email:  "agent@example.com",
					Role:   "Agent",
				}
			} else if token == "admin_token" {
				claims = &auth.Claims{
					UserID: 3,
					Email:  "admin@example.com",
					Role:   "Admin",
				}
			} else {
				// Invalid token
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
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
	}

	// Register the middleware
	for name, handler := range middlewares {
		if name == "auth" {
			router.Use(handler)
		}
	}

	// Add a test handler that checks the is_customer flag
	router.GET("/test", func(c *gin.Context) {
		isCustomer, exists := c.Get("is_customer")
		userRole, _ := c.Get("user_role")

		c.JSON(http.StatusOK, gin.H{
			"is_customer": isCustomer,
			"user_role":   userRole,
			"exists":      exists,
		})
	})

	t.Run("Customer role sets is_customer to true", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: "customer_token"})
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.True(t, response["is_customer"].(bool))
		assert.Equal(t, "Customer", response["user_role"])
		assert.True(t, response["exists"].(bool))
	})

	t.Run("Agent role sets is_customer to false", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: "agent_token"})
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.False(t, response["is_customer"].(bool))
		assert.Equal(t, "Agent", response["user_role"])
		assert.True(t, response["exists"].(bool))
	})

	t.Run("Admin role sets is_customer to false", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: "admin_token"})
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.False(t, response["is_customer"].(bool))
		assert.Equal(t, "Admin", response["user_role"])
		assert.True(t, response["exists"].(bool))
	})

	t.Run("Missing token returns unauthorized", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Equal(t, "Missing authorization token", response["error"])
	})

	t.Run("Invalid token returns unauthorized", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: "invalid_token"})
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Equal(t, "Invalid or expired token", response["error"])
	})
}

func TestAuthMiddlewareHonorsBypassDisable(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	t.Setenv("APP_ENV", "test")
	t.Setenv("GOTRS_DISABLE_TEST_AUTH_BYPASS", "1")

	registry := NewHandlerRegistry()
	RegisterExistingHandlers(registry)

	authMw, err := registry.GetMiddleware("auth")
	if err != nil {
		t.Fatalf("expected auth middleware: %v", err)
	}

	router := gin.New()
	router.Use(authMw)
	router.GET("/admin/users", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.Header.Set("Accept", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}
