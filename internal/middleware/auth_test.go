package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/auth"
)

func TestAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "production")

	// Create JWT manager for testing
	jwtManager := auth.NewJWTManager("test-secret", 1*time.Hour)
	authMiddleware := NewAuthMiddleware(jwtManager)

	t.Run("RequireAuth blocks unauthenticated requests", func(t *testing.T) {
		router := gin.New()
		router.Use(authMiddleware.RequireAuth())
		router.GET("/protected", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "success"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/protected", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Missing authorization token")
	})

	t.Run("RequireAuth allows authenticated requests", func(t *testing.T) {
		router := gin.New()
		router.Use(authMiddleware.RequireAuth())
		router.GET("/protected", func(c *gin.Context) {
			userID, _ := c.Get("user_id")
			c.JSON(200, gin.H{"user_id": userID})
		})

		// Generate valid token
		token, err := jwtManager.GenerateToken(123, "test@example.com", "Admin", 1)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "123")
	})

	t.Run("RequireAuth rejects invalid token", func(t *testing.T) {
		router := gin.New()
		router.Use(authMiddleware.RequireAuth())
		router.GET("/protected", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "success"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer invalid.token.here")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid or expired token")
	})

	t.Run("RequireRole blocks unauthorized roles", func(t *testing.T) {
		router := gin.New()

		// First apply auth middleware
		router.Use(authMiddleware.RequireAuth())
		// Then apply role middleware
		router.Use(authMiddleware.RequireRole("Admin"))

		router.GET("/admin", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "admin access"})
		})

		// Create token with Agent role
		token, err := jwtManager.GenerateToken(1, "agent@example.com", "Agent", 1)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/admin", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "Insufficient permissions")
	})

	t.Run("RequireRole allows authorized roles", func(t *testing.T) {
		router := gin.New()
		router.Use(authMiddleware.RequireAuth())
		router.Use(authMiddleware.RequireRole("Admin", "Agent"))
		router.GET("/resource", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "success"})
		})

		// Create token with Agent role
		token, err := jwtManager.GenerateToken(1, "agent@example.com", "Agent", 1)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/resource", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("RequirePermission checks permissions", func(t *testing.T) {
		router := gin.New()
		router.Use(authMiddleware.RequireAuth())
		router.Use(authMiddleware.RequirePermission(auth.PermissionTicketCreate))
		router.GET("/tickets", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "can create tickets"})
		})

		// Admin should have ticket create permission
		adminToken, err := jwtManager.GenerateToken(1, "admin@example.com", "Admin", 1)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/tickets", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Customer should not have ticket create permission
		customerToken, err := jwtManager.GenerateToken(2, "customer@example.com", "Customer", 1)
		require.NoError(t, err)

		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest("GET", "/tickets", nil)
		req2.Header.Set("Authorization", "Bearer "+customerToken)
		router.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusForbidden, w2.Code)
	})

	t.Run("OptionalAuth works without token", func(t *testing.T) {
		router := gin.New()
		router.Use(authMiddleware.OptionalAuth())
		router.GET("/public", func(c *gin.Context) {
			authenticated, exists := c.Get("authenticated")
			if exists && authenticated.(bool) {
				userID, _ := c.Get("user_id")
				c.JSON(200, gin.H{"authenticated": true, "user_id": userID})
			} else {
				c.JSON(200, gin.H{"authenticated": false})
			}
		})

		// Request without token
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/public", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"authenticated":false`)
	})

	t.Run("OptionalAuth works with valid token", func(t *testing.T) {
		router := gin.New()
		router.Use(authMiddleware.OptionalAuth())
		router.GET("/public", func(c *gin.Context) {
			authenticated, exists := c.Get("authenticated")
			if exists && authenticated.(bool) {
				userID, _ := c.Get("user_id")
				c.JSON(200, gin.H{"authenticated": true, "user_id": userID})
			} else {
				c.JSON(200, gin.H{"authenticated": false})
			}
		})

		// Generate valid token
		token, err := jwtManager.GenerateToken(456, "test@example.com", "User", 1)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/public", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"authenticated":true`)
		assert.Contains(t, w.Body.String(), "456")
	})

	t.Run("extractToken from Authorization header", func(t *testing.T) {
		router := gin.New()
		var extractedToken string

		router.GET("/test", func(c *gin.Context) {
			extractedToken = authMiddleware.extractToken(c)
			c.JSON(200, gin.H{"token": extractedToken})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer mytoken123")
		router.ServeHTTP(w, req)

		assert.Equal(t, "mytoken123", extractedToken)
	})

	t.Run("extractToken from query parameter", func(t *testing.T) {
		router := gin.New()
		var extractedToken string

		router.GET("/test", func(c *gin.Context) {
			extractedToken = authMiddleware.extractToken(c)
			c.JSON(200, gin.H{"token": extractedToken})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test?token=querytoken456", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, "querytoken456", extractedToken)
	})

	t.Run("extractToken from cookie", func(t *testing.T) {
		router := gin.New()
		var extractedToken string

		router.GET("/test", func(c *gin.Context) {
			extractedToken = authMiddleware.extractToken(c)
			c.JSON(200, gin.H{"token": extractedToken})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: "cookietoken789"})
		router.ServeHTTP(w, req)

		assert.Equal(t, "cookietoken789", extractedToken)
	})

	t.Run("IsAuthenticated checks authentication", func(t *testing.T) {
		router := gin.New()
		router.Use(authMiddleware.OptionalAuth())

		router.GET("/check", func(c *gin.Context) {
			isAuth := authMiddleware.IsAuthenticated(c)
			c.JSON(200, gin.H{"authenticated": isAuth})
		})

		// Without token
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest("GET", "/check", nil)
		router.ServeHTTP(w1, req1)
		assert.Contains(t, w1.Body.String(), `"authenticated":false`)

		// With token
		token, _ := jwtManager.GenerateToken(1, "test@example.com", "User", 1)
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest("GET", "/check", nil)
		req2.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w2, req2)
		assert.Contains(t, w2.Body.String(), `"authenticated":true`)
	})

	t.Run("GetUserID retrieves user ID", func(t *testing.T) {
		router := gin.New()
		router.Use(authMiddleware.RequireAuth())

		router.GET("/userid", func(c *gin.Context) {
			userID, exists := authMiddleware.GetUserID(c)
			c.JSON(200, gin.H{"user_id": userID, "exists": exists})
		})

		token, _ := jwtManager.GenerateToken(999, "test@example.com", "User", 1)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/userid", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		assert.Contains(t, w.Body.String(), "999")
		assert.Contains(t, w.Body.String(), `"exists":true`)
	})

	t.Run("GetUserRole retrieves user role", func(t *testing.T) {
		router := gin.New()
		router.Use(authMiddleware.RequireAuth())

		router.GET("/role", func(c *gin.Context) {
			role, exists := authMiddleware.GetUserRole(c)
			c.JSON(200, gin.H{"role": role, "exists": exists})
		})

		token, _ := jwtManager.GenerateToken(1, "test@example.com", "Agent", 1)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/role", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		assert.Contains(t, w.Body.String(), "Agent")
		assert.Contains(t, w.Body.String(), `"exists":true`)
	})

	t.Run("CanAccessTicket checks ticket access", func(t *testing.T) {
		router := gin.New()
		router.Use(authMiddleware.RequireAuth())

		router.GET("/ticket/:id", func(c *gin.Context) {
			// Simulate ticket owner ID (in real app, would query from DB)
			ticketOwnerID := uint(100)
			canAccess := authMiddleware.CanAccessTicket(c, ticketOwnerID)
			c.JSON(200, gin.H{"can_access": canAccess})
		})

		// Test with Admin (should have access)
		adminToken, _ := jwtManager.GenerateToken(1, "admin@example.com", "Admin", 1)
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest("GET", "/ticket/1", nil)
		req1.Header.Set("Authorization", "Bearer "+adminToken)
		router.ServeHTTP(w1, req1)
		assert.Contains(t, w1.Body.String(), `"can_access":true`)

		// Test with Customer who owns the ticket
		customerToken, _ := jwtManager.GenerateToken(100, "customer@example.com", "Customer", 1)
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest("GET", "/ticket/1", nil)
		req2.Header.Set("Authorization", "Bearer "+customerToken)
		router.ServeHTTP(w2, req2)
		assert.Contains(t, w2.Body.String(), `"can_access":true`)

		// Test with Customer who doesn't own the ticket
		otherCustomerToken, _ := jwtManager.GenerateToken(200, "other@example.com", "Customer", 1)
		w3 := httptest.NewRecorder()
		req3, _ := http.NewRequest("GET", "/ticket/1", nil)
		req3.Header.Set("Authorization", "Bearer "+otherCustomerToken)
		router.ServeHTTP(w3, req3)
		assert.Contains(t, w3.Body.String(), `"can_access":false`)
	})

	t.Run("RequireAuth without JWT manager respects bypass flag", func(t *testing.T) {
		t.Setenv("APP_ENV", "test")
		t.Setenv("GOTRS_DISABLE_TEST_AUTH_BYPASS", "1")

		router := gin.New()
		router.Use(NewAuthMiddleware(nil).RequireAuth())
		router.GET("/protected", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/protected", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("RequireAuth without JWT manager allows bypass when enabled", func(t *testing.T) {
		t.Setenv("APP_ENV", "test")
		t.Setenv("GOTRS_DISABLE_TEST_AUTH_BYPASS", "0")

		router := gin.New()
		router.Use(NewAuthMiddleware(nil).RequireAuth())
		router.GET("/protected", func(c *gin.Context) {
			role, _ := c.Get("user_role")
			email, _ := c.Get("user_email")
			c.JSON(http.StatusOK, gin.H{"role": role, "email": email})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/protected", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"role":"Admin"`)
		assert.Contains(t, w.Body.String(), "test@gotrs.local")
	})

	t.Run("unauthorizedResponse returns JSON for Accept: application/json", func(t *testing.T) {
		router := gin.New()
		router.Use(authMiddleware.RequireAuth())
		router.GET("/api/protected", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "success"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/protected", nil)
		req.Header.Set("Accept", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
		assert.Contains(t, w.Body.String(), "Missing authorization token")
	})

	t.Run("unauthorizedResponse redirects for Accept: text/html", func(t *testing.T) {
		router := gin.New()
		router.Use(authMiddleware.RequireAuth())
		router.GET("/protected", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "success"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/protected", nil)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "/login", w.Header().Get("Location"))
	})

	t.Run("unauthorizedResponse returns JSON when Accept header is missing", func(t *testing.T) {
		router := gin.New()
		router.Use(authMiddleware.RequireAuth())
		router.GET("/api/endpoint", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "success"})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/endpoint", nil)
		// No Accept header set
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	})
}
