package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/auth"
	platformmodels "github.com/goatkit/goatflow/internal/platform/models"
)

// fakeSessions is a SessionChecker over an in-memory set of live session ids.
// lastRequest optionally sets a session's last-request time; touches counts
// TouchSession calls per id.
type fakeSessions struct {
	live        map[string]bool
	lastRequest map[string]time.Time
	touches     map[string]int
}

func (f *fakeSessions) GetSession(id string) (*platformmodels.Session, error) {
	if f.live[id] {
		return &platformmodels.Session{SessionID: id, LastRequest: f.lastRequest[id]}, nil
	}
	return nil, errors.New("session not found")
}

func (f *fakeSessions) TouchSession(id string) error {
	f.touches[id]++
	return nil
}

// useFakeSessions installs an in-memory session checker for the test; the
// returned set is what VerifySession consults.
func useFakeSessions(t *testing.T, live ...string) *fakeSessions {
	t.Helper()
	f := &fakeSessions{live: map[string]bool{}, lastRequest: map[string]time.Time{}, touches: map[string]int{}}
	for _, id := range live {
		f.live[id] = true
	}
	prev := middlewareSessionService
	middlewareSessionOnce.Do(func() {})
	middlewareSessionService = f
	t.Cleanup(func() { middlewareSessionService = prev })
	return f
}

// sessionToken mints an access token bound to session "live".
func sessionToken(t *testing.T, m *auth.JWTManager, userID uint, email, role string) string {
	t.Helper()
	token, err := m.GenerateTokenWithLogin("live", userID, email, email, role, false, 1)
	require.NoError(t, err)
	return token
}

func TestAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "production")

	// Production rejects short/placeholder secrets, so use a real-length key.
	jwtManager := auth.NewJWTManager("middleware-auth-suite-signing-key-0123456789", 1*time.Hour)
	authMiddleware := NewAuthMiddleware(jwtManager)
	sessions := useFakeSessions(t, "live")

	t.Run("RequireAuth rejects a token whose session was killed", func(t *testing.T) {
		router := gin.New()
		router.Use(authMiddleware.RequireAuth())
		router.GET("/protected", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

		killed, err := jwtManager.GenerateTokenWithLogin("killed", 123, "test@example.com", "test@example.com", "Admin", false, 1)
		require.NoError(t, err)
		for _, send := range map[string]func(*http.Request){
			"bearer": func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+killed) },
			"cookie": func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "auth_token", Value: killed}) },
		} {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/protected", nil)
			send(req)
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Contains(t, w.Body.String(), "Session has been terminated")
		}

		// Same token once the session exists again: accepted, and a later
		// kill is seen on the next request (nothing is cached across requests).
		sessions.live["killed"] = true
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+killed)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		delete(sessions.live, "killed")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("OptionalAuth ignores a token whose session was killed", func(t *testing.T) {
		router := gin.New()
		router.Use(authMiddleware.OptionalAuth())
		router.GET("/public", func(c *gin.Context) { c.JSON(200, gin.H{"authenticated": authMiddleware.IsAuthenticated(c)}) })

		killed, err := jwtManager.GenerateTokenWithLogin("killed", 123, "test@example.com", "test@example.com", "Admin", false, 1)
		require.NoError(t, err)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/public", nil)
		req.Header.Set("Authorization", "Bearer "+killed)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"authenticated":false`)
	})

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
		token := sessionToken(t, jwtManager, 123, "test@example.com", "Admin")

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
		token := sessionToken(t, jwtManager, 1, "agent@example.com", "Agent")

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
		token := sessionToken(t, jwtManager, 1, "agent@example.com", "Agent")

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
		adminToken := sessionToken(t, jwtManager, 1, "admin@example.com", "Admin")

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/tickets", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Customer should not have ticket create permission
		customerToken := sessionToken(t, jwtManager, 2, "customer@example.com", "Customer")

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
		token := sessionToken(t, jwtManager, 456, "test@example.com", "User")

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

	t.Run("extractToken ignores query parameter on normal route", func(t *testing.T) {
		router := gin.New()
		var extractedToken string

		router.GET("/*path", func(c *gin.Context) {
			extractedToken = authMiddleware.extractToken(c)
			c.JSON(200, gin.H{"token": extractedToken})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test?token=querytoken456", nil)
		router.ServeHTTP(w, req)

		assert.Empty(t, extractedToken)
	})

	t.Run("extractToken query parameter cannot override cookie on normal route", func(t *testing.T) {
		router := gin.New()
		var extractedToken string

		router.GET("/*path", func(c *gin.Context) {
			extractedToken = authMiddleware.extractToken(c)
			c.JSON(200, gin.H{"token": extractedToken})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test?token=querytoken456", nil)
		req.AddCookie(&http.Cookie{Name: "auth_token", Value: "cookietoken789"})
		router.ServeHTTP(w, req)

		assert.Equal(t, "cookietoken789", extractedToken)
	})

	t.Run("extractToken accepts query parameter on SSE route", func(t *testing.T) {
		router := gin.New()
		var extractedToken string

		router.GET("/*path", func(c *gin.Context) {
			extractedToken = authMiddleware.extractToken(c)
			c.JSON(200, gin.H{"token": extractedToken})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/mcp/sse?token=querytoken456", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, "querytoken456", extractedToken)
	})

	t.Run("extractToken accepts query parameter on plugin SSE route", func(t *testing.T) {
		router := gin.New()
		var extractedToken string

		router.GET("/*path", func(c *gin.Context) {
			extractedToken = authMiddleware.extractToken(c)
			c.JSON(200, gin.H{"token": extractedToken})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/plugins/demo/events/progress?token=querytoken456", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, "querytoken456", extractedToken)
	})

	t.Run("extractToken accepts query parameter on WebSocket upgrade", func(t *testing.T) {
		router := gin.New()
		var extractedToken string

		router.GET("/*path", func(c *gin.Context) {
			extractedToken = authMiddleware.extractToken(c)
			c.JSON(200, gin.H{"token": extractedToken})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ws?token=querytoken456", nil)
		req.Header.Set("Connection", "keep-alive, Upgrade")
		req.Header.Set("Upgrade", "websocket")
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
		token := sessionToken(t, jwtManager, 1, "test@example.com", "User")
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

		token := sessionToken(t, jwtManager, 999, "test@example.com", "User")
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

		token := sessionToken(t, jwtManager, 1, "test@example.com", "Agent")
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
		adminToken := sessionToken(t, jwtManager, 1, "admin@example.com", "Admin")
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest("GET", "/ticket/1", nil)
		req1.Header.Set("Authorization", "Bearer "+adminToken)
		router.ServeHTTP(w1, req1)
		assert.Contains(t, w1.Body.String(), `"can_access":true`)

		// Test with Customer who owns the ticket
		customerToken := sessionToken(t, jwtManager, 100, "customer@example.com", "Customer")
		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest("GET", "/ticket/1", nil)
		req2.Header.Set("Authorization", "Bearer "+customerToken)
		router.ServeHTTP(w2, req2)
		assert.Contains(t, w2.Body.String(), `"can_access":true`)

		// Test with Customer who doesn't own the ticket
		otherCustomerToken := sessionToken(t, jwtManager, 200, "other@example.com", "Customer")
		w3 := httptest.NewRecorder()
		req3, _ := http.NewRequest("GET", "/ticket/1", nil)
		req3.Header.Set("Authorization", "Bearer "+otherCustomerToken)
		router.ServeHTTP(w3, req3)
		assert.Contains(t, w3.Body.String(), `"can_access":false`)
	})

	t.Run("RequireAuth without JWT manager returns 401", func(t *testing.T) {
		router := gin.New()
		router.Use(NewAuthMiddleware(nil).RequireAuth())
		router.GET("/protected", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/protected", nil)
		req.Header.Set("Accept", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
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

// TestVerifySessionThrottlesTouch: every request checks the session exists
// (so a kill revokes at once), but the last-request write happens only once
// the stored time is older than sessionTouchInterval.
func TestVerifySessionThrottlesTouch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sessions := useFakeSessions(t, "fresh", "stale")
	sessions.lastRequest["fresh"] = time.Now().Add(-sessionTouchInterval / 2)
	sessions.lastRequest["stale"] = time.Now().Add(-2 * sessionTouchInterval)

	verify := func(sid string) bool {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		return VerifySession(c, &auth.Claims{SessionID: sid})
	}

	for range 3 {
		assert.True(t, verify("fresh"))
		assert.True(t, verify("stale"))
	}
	assert.Zero(t, sessions.touches["fresh"], "a recently touched session is not rewritten")
	assert.Equal(t, 3, sessions.touches["stale"], "a session past the interval is touched")

	delete(sessions.live, "fresh")
	assert.False(t, verify("fresh"), "a killed session is refused on the very next request")
}
