package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDemoTokensNeverAuthenticate pins that no production middleware grants an
// identity for the old "demo_session_*" / "demo_customer_*" literal tokens.
// A removed SessionMiddleware used to accept any such value as Admin or as a
// customer without a demo-mode check; it must not be re-wired.
func TestDemoTokensNeverAuthenticate(t *testing.T) {
	router := newProductionRouter(t)

	tokens := []string{"demo_session_admin", "demo_session_1_1755839704", "demo_customer_john.customer"}
	carriers := map[string]func(*http.Request, string){
		"bearer":              func(r *http.Request, tok string) { r.Header.Set("Authorization", "Bearer "+tok) },
		"cookie access_token": func(r *http.Request, tok string) { r.AddCookie(&http.Cookie{Name: "access_token", Value: tok}) },
		"cookie auth_token":   func(r *http.Request, tok string) { r.AddCookie(&http.Cookie{Name: "auth_token", Value: tok}) },
	}
	paths := []string{"/api/v1/users/me", "/dashboard", "/admin/users", "/customer/tickets", "/api/v1/tickets"}

	for _, tok := range tokens {
		for name, apply := range carriers {
			for _, path := range paths {
				req := httptest.NewRequest(http.MethodGet, path, nil)
				req.Header.Set("Accept", "application/json")
				apply(req, tok)
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				require.Truef(t, denied(w), "%s %q via %s: got %d (body %q), want 401/403 or login redirect",
					path, tok, name, w.Code, w.Body.String())
			}
		}
	}
}
