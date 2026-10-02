package routing

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestFullRoutePathMatchesRegisteredPath registers routes the way the YAML
// loader does and checks FullRoutePath names the path gin actually serves.
// Route docs are generated with FullRoutePath, so a mismatch documents
// paths that 404 (e.g. /admin/admin/identity-providers).
func TestFullRoutePathMatchesRegisteredPath(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cases := []struct {
		prefix, path, want string
	}{
		{"/admin", "/identity-providers", "/admin/identity-providers"},
		{"/admin", "/admin/identity-providers", "/admin/identity-providers"},
		{"/admin", "/admin/identity-providers/:id/:action", "/admin/identity-providers/:id/:action"},
		{"/admin", "/admin", "/admin"},
		{"/admin", "/", "/admin"},
		{"/admin/", "users", "/admin/users"},
		{"", "/health", "/health"},
		{"", "/", "/"},
		{"/", "/login", "/login"},
		{"/api/v1", "/webhooks/:id/test", "/api/v1/webhooks/:id/test"},
		{"/static", "/*filepath", "/static/*filepath"},
		{"/customer", "/tickets/", "/customer/tickets/"},
	}
	for _, tc := range cases {
		got := FullRoutePath(tc.prefix, tc.path)
		if got != tc.want {
			t.Errorf("FullRoutePath(%q, %q) = %q, want %q", tc.prefix, tc.path, got, tc.want)
		}

		r := gin.New()
		loader := &RouteLoader{}
		loader.registerMethodRoute(r.Group(tc.prefix), http.MethodGet, tc.path, func(c *gin.Context) {})
		routes := r.Routes()
		if len(routes) != 1 {
			t.Fatalf("prefix %q path %q: registered %d routes", tc.prefix, tc.path, len(routes))
		}
		if routes[0].Path != got {
			t.Errorf("prefix %q path %q: gin serves %q, FullRoutePath says %q", tc.prefix, tc.path, routes[0].Path, got)
		}
	}
}
