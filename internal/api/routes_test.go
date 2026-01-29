package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// Note: Uses centralized GetTestAuthToken() and AddTestAuthCookie() from test_helpers.go

func TestNewRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := NewSimpleRouter()

	assert.NotNil(t, router)
	// Simple router just creates a gin engine with HTMX routes
}

func TestSetupRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := NewSimpleRouter()
	token := GetTestAuthToken(t)

	tests := []struct {
		name       string
		method     string
		path       string
		statusCode int
		needsAuth  bool
	}{
		{
			name:       "Health check endpoint",
			method:     "GET",
			path:       "/health",
			statusCode: http.StatusOK,
			needsAuth:  false,
		},
		{
			name:       "Login page endpoint",
			method:     "GET",
			path:       "/login",
			statusCode: http.StatusOK,
			needsAuth:  false,
		},
		{
			name:       "Dashboard page endpoint",
			method:     "GET",
			path:       "/dashboard",
			statusCode: http.StatusOK,
			needsAuth:  true,
		},
		{
			name:       "HTMX login endpoint exists",
			method:     "POST",
			path:       "/api/auth/login",
			statusCode: http.StatusBadRequest, // Will fail due to no body
			needsAuth:  false,
		},
		{
			name:       "HTMX dashboard stats endpoint",
			method:     "GET",
			path:       "/api/dashboard/stats",
			statusCode: http.StatusOK,
			needsAuth:  true,
		},
		{
			name:       "Non-existent route returns 404",
			method:     "GET",
			path:       "/nonexistent",
			statusCode: http.StatusNotFound,
			needsAuth:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.needsAuth {
				AddTestAuthCookie(req, token)
			}
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.statusCode, w.Code)
		})
	}
}

func TestHTMXEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := NewSimpleRouter()
	token := GetTestAuthToken(t)

	tests := []struct {
		name       string
		method     string
		path       string
		statusCode int
	}{
		{
			name:       "Dashboard stats returns HTML fragment",
			method:     "GET",
			path:       "/api/dashboard/stats",
			statusCode: http.StatusOK,
		},
		{
			name:       "Recent tickets returns HTML fragment",
			method:     "GET",
			path:       "/api/dashboard/recent-tickets",
			statusCode: http.StatusOK,
		},
		{
			name:       "Activity feed returns HTML fragment",
			method:     "GET",
			path:       "/api/dashboard/activity",
			statusCode: http.StatusOK,
		},
		{
			name:       "Tickets API returns HTML fragment",
			method:     "GET",
			path:       "/api/tickets",
			statusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			AddTestAuthCookie(req, token)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.statusCode, w.Code)
		})
	}
}

func TestHealthEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := NewSimpleRouter()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "healthy")
}

func TestRouteGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := NewSimpleRouter()

	// Get all routes
	routes := router.Routes()

	// Check that routes are properly set up
	pageRoutes := 0
	apiRoutes := 0

	for _, route := range routes {
		if len(route.Path) > 4 && route.Path[:4] == "/api" {
			apiRoutes++
		} else if route.Path == "/login" || route.Path == "/dashboard" {
			pageRoutes++
		}
	}

	assert.Greater(t, apiRoutes, 0, "Should have API routes for HTMX")
	assert.Greater(t, pageRoutes, 0, "Should have page routes")
}

func BenchmarkRouting(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)

	router := NewSimpleRouter()
	req := httptest.NewRequest("GET", "/health", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}
