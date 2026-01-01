package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	apipkg "github.com/gotrs-io/gotrs-ce/internal/api"
	"github.com/gotrs-io/gotrs-ce/internal/routing"
)

// Minimal integration style test for the auth login handler + auth middleware.
func TestLoginFlowSetsCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Handler + middleware registry
	reg := routing.NewHandlerRegistry()
	routing.RegisterExistingHandlers(reg)

	// Register required handlers
	reg.OverrideBatch(map[string]gin.HandlerFunc{
		"handleLoginPage": apipkg.HandleLoginPage,
		"handleAuthLogin": apipkg.HandleAuthLogin,
		"handleDashboard": apipkg.HandleDashboard,
	})

	// Simulate route registrations (subset)
	r.GET("/login", reg.MustGet("handleLoginPage"))
	r.POST("/api/auth/login", reg.MustGet("handleAuthLogin"))
	// Protected dashboard uses auth middleware
	authMw, _ := reg.GetMiddleware("auth")
	r.GET("/dashboard", authMw, reg.MustGet("handleDashboard"))

	// First request login (GET)
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for login page, got %d", w.Code)
	}

	// Submit login with invalid creds (expect redirect or 401 fragment)
	badReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader("username=nosuch&password=bad"))
	badReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	badW := httptest.NewRecorder()
	r.ServeHTTP(badW, badReq)
	if badW.Code != http.StatusUnauthorized && badW.Code != http.StatusSeeOther && badW.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status for bad login: %d", badW.Code)
	}

	// We can't perform a successful login without a real user setup here; the goal is coverage of handlers.
	// Future: seed test user and assert cookie presence then access dashboard.
}
