package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestHandleLoginPageRedirectsOnCustomerFE ensures a customer-only deployment
// never serves the agent login page (which renders the version string) at
// /login; it must redirect to the customer login instead.
func TestHandleLoginPageRedirectsOnCustomerFE(t *testing.T) {
	t.Setenv("CUSTOMER_FE_ONLY", "true")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/login", handleLoginPage)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected HTTP 302 redirect, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/customer/login" {
		t.Fatalf("expected redirect to /customer/login, got %q", loc)
	}
}
