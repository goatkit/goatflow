package routing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Unauthenticated calls to /api/** must get 401 JSON whatever the Accept
// header says (curl and many HTTP clients send none or */*); only browser
// pages are redirected to the login page.
func TestAuthMiddlewareAPIPathsAnswer401(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := NewHandlerRegistry()
	RegisterExistingHandlers(registry)
	authMw, err := registry.GetMiddleware("auth")
	require.NoError(t, err)

	router := gin.New()
	router.Use(authMw)
	ok := func(c *gin.Context) { c.String(http.StatusOK, "reached") }
	router.GET("/api/v1/tickets", ok)
	router.GET("/dashboard", ok)
	router.GET("/customer/tickets", ok)

	cases := []struct {
		name, path, accept, cookie string
		wantCode                   int
		wantError                  string // JSON error for 401
		wantLocation               string // redirect target for 303
	}{
		{"api without Accept", "/api/v1/tickets", "", "", http.StatusUnauthorized, "Missing authorization token", ""},
		{"api with browser Accept", "/api/v1/tickets", "text/html,application/xhtml+xml,*/*;q=0.8", "", http.StatusUnauthorized, "Missing authorization token", ""},
		{"api with */*", "/api/v1/tickets", "*/*", "", http.StatusUnauthorized, "Missing authorization token", ""},
		{"api with JSON Accept", "/api/v1/tickets", "application/json", "", http.StatusUnauthorized, "Missing authorization token", ""},
		{"api with invalid token, no Accept", "/api/v1/tickets", "", "access_token=not-a-jwt", http.StatusUnauthorized, "Invalid or expired token", ""},
		{"page without Accept", "/dashboard", "", "", http.StatusSeeOther, "", "/login"},
		{"page with browser Accept", "/dashboard", "text/html", "", http.StatusSeeOther, "", "/login"},
		{"customer page", "/customer/tickets", "text/html", "", http.StatusSeeOther, "", "/customer/login"},
		{"page with invalid token", "/dashboard", "text/html", "access_token=not-a-jwt", http.StatusSeeOther, "", "/login"},
		{"page asking for JSON", "/dashboard", "application/json", "", http.StatusUnauthorized, "Missing authorization token", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if tc.accept != "" {
				req.Header.Set("Accept", tc.accept)
			}
			if tc.cookie != "" {
				req.Header.Set("Cookie", tc.cookie)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, tc.wantCode, w.Code, w.Body.String())
			if tc.wantCode == http.StatusSeeOther {
				assert.Equal(t, tc.wantLocation, w.Header().Get("Location"))
				return
			}
			assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
			var body map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, tc.wantError, body["error"])
		})
	}
}
