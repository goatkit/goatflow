package middleware

import (
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
