package routing

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLoadYAMLRoutesPreservesPrefix(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	yaml := `apiVersion: v1
kind: RouteGroup
metadata:
  name: api-notifications
  description: "Test notification routes"
  namespace: default
  enabled: true
spec:
  prefix: /api
  middleware:
    - auth
  routes:
    - path: /notifications/pending
      method: GET
      handler: handlePendingReminderFeed
`
	file := filepath.Join(dir, "api-notifications.yaml")
	if err := os.WriteFile(file, []byte(yaml), 0o644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	router := gin.New()
	registry := NewHandlerRegistry()
	if err := registry.RegisterMiddleware("auth", func(c *gin.Context) { c.Next() }); err != nil {
		t.Fatalf("register middleware failed: %v", err)
	}
	if err := registry.Register("handlePendingReminderFeed", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	}); err != nil {
		t.Fatalf("register handler failed: %v", err)
	}

	if err := LoadYAMLRoutes(router, dir, registry); err != nil {
		t.Fatalf("LoadYAMLRoutes failed: %v", err)
	}

	t.Run("prefixed route", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/notifications/pending", nil)
		router.ServeHTTP(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", w.Code)
		}
	})

	t.Run("no stray root route", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/notifications/pending", nil)
		router.ServeHTTP(w, req)
		if w.Code == http.StatusNoContent {
			t.Fatalf("unexpected handler without prefix: status %d", w.Code)
		}
	})
}

func TestLoadYAMLRoutesFromGlobalMap_SetsGlobalRegistry(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	// Reset global state
	old := globalRegistry
	globalRegistry = nil
	defer func() { globalRegistry = old }()

	// Write a minimal route file
	dir := t.TempDir()
	yaml := `apiVersion: v1
kind: RouteGroup
metadata:
  name: test-global-registry
  description: "Test"
  namespace: default
  enabled: true
spec:
  prefix: /test
  routes:
    - path: /ping
      method: GET
      handler: testPingHandler
      description: "Ping"
`
	if err := os.WriteFile(filepath.Join(dir, "test.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	// Register a handler in the global map
	GlobalHandlerMap["testPingHandler"] = func(c *gin.Context) {
		c.Status(http.StatusOK)
	}
	defer delete(GlobalHandlerMap, "testPingHandler")

	router := gin.New()
	if err := LoadYAMLRoutesFromGlobalMap(router, dir); err != nil {
		t.Fatalf("LoadYAMLRoutesFromGlobalMap failed: %v", err)
	}

	// The global registry MUST be set — this is what the MCP bridge relies on
	if GetGlobalRegistry() == nil {
		t.Fatal("GetGlobalRegistry() returned nil after LoadYAMLRoutesFromGlobalMap — MCP bridge will fail")
	}

	// Verify the registry contains handlers
	handler, err := GetGlobalRegistry().Get("testPingHandler")
	if err != nil {
		t.Fatalf("Registry should contain testPingHandler: %v", err)
	}
	if handler == nil {
		t.Fatal("Handler should not be nil")
	}
}

// A group with an empty prefix must not turn its middleware into engine-wide
// middleware for groups registered after it (routes/redirects.yaml has prefix
// "" and middleware [auth]; config order is map order, so the leak made
// unrelated API routes randomly redirect to the login page).
func TestRegisterRouteConfig_EmptyPrefixMiddlewareStaysInGroup(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	registry := NewHandlerRegistry()
	if err := registry.RegisterMiddleware("deny", func(c *gin.Context) {
		c.AbortWithStatus(http.StatusSeeOther)
	}); err != nil {
		t.Fatalf("register middleware failed: %v", err)
	}
	ok := func(c *gin.Context) { c.Status(http.StatusNoContent) }
	if err := registry.Register("rootHandler", ok); err != nil {
		t.Fatalf("register handler failed: %v", err)
	}
	if err := registry.Register("apiHandler", ok); err != nil {
		t.Fatalf("register handler failed: %v", err)
	}
	loader, err := NewRouteLoader(t.TempDir(), registry, router)
	if err != nil {
		t.Fatalf("NewRouteLoader failed: %v", err)
	}

	root := &RouteConfig{
		Metadata: RouteMetadata{Name: "root", Enabled: true},
		Spec: RouteSpec{Prefix: "", Middleware: []string{"deny"}, Routes: []RouteDefinition{
			{Path: "/guarded", Method: "GET", Handler: "rootHandler"},
		}},
	}
	api := &RouteConfig{
		Metadata: RouteMetadata{Name: "api", Enabled: true},
		Spec: RouteSpec{Prefix: "/api", Routes: []RouteDefinition{
			{Path: "/open", Method: "GET", Handler: "apiHandler"},
		}},
	}
	// Root group first: the order that leaked.
	if err := loader.registerRouteConfig(root); err != nil {
		t.Fatalf("register root: %v", err)
	}
	if err := loader.registerRouteConfig(api); err != nil {
		t.Fatalf("register api: %v", err)
	}

	for path, want := range map[string]int{"/guarded": http.StatusSeeOther, "/api/open": http.StatusNoContent} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != want {
			t.Errorf("GET %s: got %d, want %d", path, w.Code, want)
		}
	}
}
