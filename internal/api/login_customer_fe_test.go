package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestHandleLoginPageRedirectsOnCustomerFE ensures a customer-only deployment
// never serves the agent login page (which renders the version string) at
// /login; it must redirect to the customer login instead. Every spelling the
// server accepts for CUSTOMER_FE_ONLY (cmd/goats mounts CustomerOnlyGuard for
// all of them) must trigger the redirect, not only "true".
func TestHandleLoginPageRedirectsOnCustomerFE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, val := range []string{"true", "1", "yes", " On ", "TRUE"} {
		t.Run(val, func(t *testing.T) {
			t.Setenv("CUSTOMER_FE_ONLY", val)
			r := gin.New()
			r.GET("/login", handleLoginPage)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/login", nil)
			r.ServeHTTP(w, req)

			if w.Code != http.StatusFound {
				t.Fatalf("CUSTOMER_FE_ONLY=%q: expected HTTP 302 redirect, got %d", val, w.Code)
			}
			if loc := w.Header().Get("Location"); loc != "/customer/login" {
				t.Fatalf("CUSTOMER_FE_ONLY=%q: expected redirect to /customer/login, got %q", val, loc)
			}
		})
	}
}
