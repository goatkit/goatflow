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

// The YAML "admin" middleware must admit both session admins (user_role
// "Admin") and callers the auth layer marked as admin-group members
// (isInAdminGroup=true, set for '*' / 'admin:*' scoped API tokens and for JWTs
// with the admin claim), and reject everyone else with 403.
func TestAdminMiddlewareAcceptsAdminGroupFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := NewHandlerRegistry()
	RegisterExistingHandlers(registry)
	adminMw, err := registry.GetMiddleware("admin")
	require.NoError(t, err)

	cases := []struct {
		name string
		ctx  map[string]any
		want int
	}{
		{"admin-scoped API token as plain user", map[string]any{"isInAdminGroup": true, "user_role": "User"}, http.StatusOK},
		{"admin group member with agent role", map[string]any{"isInAdminGroup": true, "user_role": "Agent"}, http.StatusOK},
		{"session admin", map[string]any{"user_role": "Admin"}, http.StatusOK},
		{"plain user outside admin group", map[string]any{"isInAdminGroup": false, "user_role": "User"}, http.StatusForbidden},
		{"admin flag of wrong type", map[string]any{"isInAdminGroup": "true", "user_role": "Agent"}, http.StatusForbidden},
		{"no auth context", map[string]any{}, http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				for k, v := range tc.ctx {
					c.Set(k, v)
				}
				c.Next()
			})
			router.Use(adminMw)
			router.GET("/api/v1/webhooks", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"success": true})
			})

			req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks", nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			assert.Equal(t, tc.want, resp.Code)
			var body map[string]any
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
			if tc.want == http.StatusOK {
				assert.Equal(t, true, body["success"])
			} else {
				assert.Equal(t, "Admin access required", body["error"])
			}
		})
	}
}
