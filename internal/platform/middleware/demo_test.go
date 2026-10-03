package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// The auth middleware marks admins with isInAdminGroup=true (membership of the
// admin group in group_user) and user_role="Admin"; DemoGuard must honour both.
func TestDemoGuardIsAdminUsesAuthContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name string
		keys map[string]any
		want bool
	}{
		{"admin group member", map[string]any{"isInAdminGroup": true, "user_role": "Agent"}, true},
		{"Admin role from JWT claims", map[string]any{"user_role": "Admin"}, true},
		{"agent outside admin group", map[string]any{"isInAdminGroup": false, "user_role": "Agent"}, false},
		{"customer", map[string]any{"user_role": "Customer"}, false},
		{"no auth context", map[string]any{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			for k, v := range tc.keys {
				c.Set(k, v)
			}
			assert.Equal(t, tc.want, isAdmin(c))
		})
	}
}

// A blocked request answers by what the client accepts. Short Accept values
// such as curl's "*/*" used to panic (accept[:16] on a 3-byte string), and a
// page opened without a Referer redirected to itself.
func TestBlockDemoChange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name         string
		method       string
		headers      map[string]string
		wantStatus   int
		wantLocation string
	}{
		{"curl default accept", http.MethodGet, map[string]string{"Accept": "*/*"}, http.StatusSeeOther, "/"},
		{"browser page with referer", http.MethodGet, map[string]string{
			"Accept":  "text/html,application/xhtml+xml",
			"Referer": "https://demo.example/agent/dashboard",
		}, http.StatusSeeOther, "https://demo.example/agent/dashboard"},
		{"no accept header", http.MethodGet, nil, http.StatusSeeOther, "/"},
		{"json accept with charset", http.MethodPost, map[string]string{"Accept": "application/json; charset=utf-8"}, http.StatusForbidden, ""},
		{"xhr", http.MethodPost, map[string]string{"X-Requested-With": "XMLHttpRequest"}, http.StatusForbidden, ""},
		{"json body", http.MethodPost, map[string]string{"Content-Type": "application/json"}, http.StatusForbidden, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(tc.method, "/agent/password", nil)
			for k, v := range tc.headers {
				c.Request.Header.Set(k, v)
			}

			blockDemoChange(c)

			assert.True(t, c.IsAborted())
			assert.Equal(t, tc.wantStatus, rec.Code)
			assert.Equal(t, tc.wantLocation, rec.Header().Get("Location"))
		})
	}
}
