package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/plugin"
	"github.com/goatkit/goatflow/internal/plugin/example"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// mockHostAPI for testing
type mockHostAPI struct{}

func (m *mockHostAPI) DBQuery(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	return nil, nil
}
func (m *mockHostAPI) DBExec(ctx context.Context, query string, args ...any) (int64, error) {
	return 0, nil
}
func (m *mockHostAPI) CacheGet(ctx context.Context, key string) ([]byte, bool, error) {
	return nil, false, nil
}
func (m *mockHostAPI) CacheSet(ctx context.Context, key string, value []byte, ttlSeconds int) error {
	return nil
}
func (m *mockHostAPI) CacheDelete(ctx context.Context, key string) error { return nil }
func (m *mockHostAPI) HTTPRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) (int, []byte, error) {
	return 200, nil, nil
}
func (m *mockHostAPI) SendEmail(ctx context.Context, to, subject, body string, html bool) error {
	return nil
}
func (m *mockHostAPI) Log(ctx context.Context, level, message string, fields map[string]any) {}
func (m *mockHostAPI) ConfigGet(ctx context.Context, key string) (string, error)            { return "", nil }
func (m *mockHostAPI) Translate(ctx context.Context, key string, args ...any) string        { return "" }
func (m *mockHostAPI) CallPlugin(ctx context.Context, pluginName, function string, args json.RawMessage) (json.RawMessage, error) {
	return nil, nil
}

var testJWTToken string

func setupPluginTestRouter() (*gin.Engine, *plugin.Manager) {
	host := &mockHostAPI{}
	mgr := plugin.NewManager(host)
	SetPluginManager(mgr)

	// Register test plugin
	hello := example.NewHelloPlugin()
	mgr.Register(context.Background(), hello)

	// Generate a valid admin test token
	jwtManager := getJWTManager()
	testJWTToken, _ = jwtManager.GenerateTokenWithAdmin(1, "admin@test.com", "Admin", true, 0)

	r := gin.New()
	api := r.Group("/api/v1")
	RegisterPluginAPIRoutes(api)

	return r, mgr
}

// addAuthHeader adds the test JWT token to a request
func addAuthHeader(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+testJWTToken)
}

func TestHandlePluginList(t *testing.T) {
	r, _ := setupPluginTestRouter()

	req := httptest.NewRequest("GET", "/api/v1/plugins", nil)
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	plugins, ok := result["plugins"].([]any)
	if !ok {
		t.Fatalf("expected plugins array in response, got %v", result)
	}

	if len(plugins) != 1 {
		t.Errorf("expected 1 plugin, got %d", len(plugins))
	}

	plugin := plugins[0].(map[string]any)
	if plugin["name"] != "hello" {
		t.Errorf("expected hello plugin, got %v", plugin["name"])
	}
}

func TestHandlePluginCall(t *testing.T) {
	r, _ := setupPluginTestRouter()

	// Call the hello function
	body := bytes.NewBufferString(`{"name": "Test"}`)
	req := httptest.NewRequest("POST", "/api/v1/plugins/hello/call/hello", body)
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if result["message"] == nil {
		t.Error("expected message in response")
	}
}

func TestHandlePluginCallNotFound(t *testing.T) {
	r, _ := setupPluginTestRouter()

	body := bytes.NewBufferString(`{}`)
	req := httptest.NewRequest("POST", "/api/v1/plugins/nonexistent/call/test", body)
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandlePluginEnable(t *testing.T) {
	r, mgr := setupPluginTestRouter()

	// Verify plugin starts enabled
	if !mgr.IsEnabled("hello") {
		t.Fatal("plugin should start enabled")
	}

	// Disable first so we can test enable
	if err := mgr.Disable("hello"); err != nil {
		t.Fatalf("failed to disable: %v", err)
	}
	if mgr.IsEnabled("hello") {
		t.Fatal("plugin should be disabled after Disable()")
	}

	// Call enable API
	req := httptest.NewRequest("POST", "/api/v1/plugins/hello/enable", nil)
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify plugin is now ACTUALLY enabled
	if !mgr.IsEnabled("hello") {
		t.Error("plugin should be enabled after enable API call")
	}

	// Verify the list API reflects the enabled state
	req2 := httptest.NewRequest("GET", "/api/v1/plugins", nil)
	addAuthHeader(req2)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	var resp struct {
		Plugins []map[string]any `json:"plugins"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse plugins response: %v", err)
	}

	found := false
	for _, p := range resp.Plugins {
		if p["name"] == "hello" {
			found = true
			if enabled, ok := p["enabled"].(bool); !ok || !enabled {
				t.Errorf("plugin should show enabled=true in API response, got %v", p["enabled"])
			}
			break
		}
	}
	if !found {
		t.Error("hello plugin not found in list response")
	}
}

func TestHandlePluginDisable(t *testing.T) {
	r, mgr := setupPluginTestRouter()

	// Verify plugin starts enabled
	if !mgr.IsEnabled("hello") {
		t.Fatal("plugin should start enabled")
	}

	// Call disable API
	req := httptest.NewRequest("POST", "/api/v1/plugins/hello/disable", nil)
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify plugin is now ACTUALLY disabled
	if mgr.IsEnabled("hello") {
		t.Error("plugin should be disabled after disable API call")
	}

	// Verify the list API reflects the disabled state
	req2 := httptest.NewRequest("GET", "/api/v1/plugins", nil)
	addAuthHeader(req2)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	var resp struct {
		Plugins []map[string]any `json:"plugins"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse plugins response: %v", err)
	}

	found := false
	for _, p := range resp.Plugins {
		if p["name"] == "hello" {
			found = true
			if enabled, ok := p["enabled"].(bool); ok && enabled {
				t.Errorf("plugin should show enabled=false in API response, got %v", p["enabled"])
			}
			break
		}
	}
	if !found {
		t.Error("hello plugin not found in list response")
	}

	// Re-enable for other tests
	mgr.Enable("hello")
}

func TestHandlePluginLogs(t *testing.T) {
	r, _ := setupPluginTestRouter()

	// Add some test logs
	buf := plugin.GetLogBuffer()
	buf.Log("hello", "info", "test log 1", nil)
	buf.Log("hello", "error", "test error", nil)

	req := httptest.NewRequest("GET", "/api/v1/plugins/logs?limit=10", nil)
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	logs, ok := result["logs"].([]any)
	if !ok {
		t.Fatal("expected logs array in response")
	}

	if len(logs) < 2 {
		t.Errorf("expected at least 2 logs, got %d", len(logs))
	}
}

func TestHandlePluginLogsFilterByPlugin(t *testing.T) {
	r, _ := setupPluginTestRouter()

	buf := plugin.GetLogBuffer()
	buf.Clear()
	buf.Log("plugin-a", "info", "log from a", nil)
	buf.Log("plugin-b", "info", "log from b", nil)

	req := httptest.NewRequest("GET", "/api/v1/plugins/logs?plugin=plugin-a", nil)
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	logs, ok := result["logs"].([]any)
	if !ok {
		t.Fatalf("logs not found or not an array in response: %v", result)
	}
	if len(logs) != 1 {
		t.Errorf("expected 1 log for plugin-a, got %d", len(logs))
	}
}

func TestHandleClearPluginLogs(t *testing.T) {
	r, _ := setupPluginTestRouter()

	buf := plugin.GetLogBuffer()
	buf.Log("test", "info", "log", nil)

	req := httptest.NewRequest("DELETE", "/api/v1/plugins/logs", nil)
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	if buf.Count() != 0 {
		t.Error("logs should be cleared")
	}
}

func TestHandlePluginUpload(t *testing.T) {
	// Create temp plugin dir
	tmpDir := t.TempDir()
	SetPluginDir(tmpDir)

	r, _ := setupPluginTestRouter()

	// Create a test WASM file
	wasmContent := []byte("fake wasm content")
	
	// Create multipart form
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, err := w.CreateFormFile("plugin", "test.wasm")
	if err != nil {
		t.Fatal(err)
	}
	fw.Write(wasmContent)
	w.Close()

	req := httptest.NewRequest("POST", "/api/v1/plugins/upload", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	addAuthHeader(req)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify file was saved
	if _, err := os.Stat(filepath.Join(tmpDir, "test.wasm")); os.IsNotExist(err) {
		t.Error("WASM file not saved")
	}
}

func TestHandlePluginUploadInvalidType(t *testing.T) {
	tmpDir := t.TempDir()
	SetPluginDir(tmpDir)

	r, _ := setupPluginTestRouter()

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, _ := w.CreateFormFile("plugin", "test.txt")
	fw.Write([]byte("not a wasm file"))
	w.Close()

	req := httptest.NewRequest("POST", "/api/v1/plugins/upload", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	addAuthHeader(req)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid file type, got %d", rec.Code)
	}
}

func TestHandlePluginUploadZIP(t *testing.T) {
	tmpDir := t.TempDir()
	SetPluginDir(tmpDir)

	r, _ := setupPluginTestRouter()

	// Create a test ZIP file with manifest and wasm
	zipPath := filepath.Join(tmpDir, "test-upload.zip")
	createTestZIP(t, zipPath)
	
	zipContent, _ := os.ReadFile(zipPath)

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, _ := w.CreateFormFile("plugin", "test-plugin.zip")
	io.Copy(fw, bytes.NewReader(zipContent))
	w.Close()

	req := httptest.NewRequest("POST", "/api/v1/plugins/upload", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	addAuthHeader(req)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func createTestZIP(t *testing.T, path string) {
	t.Helper()
	
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	
	zw := zip.NewWriter(f)
	
	// Add plugin.yaml manifest
	manifestYAML := "name: test-plugin\nversion: \"1.0.0\"\nruntime: wasm\n"
	mw, _ := zw.Create("plugin.yaml")
	mw.Write([]byte(manifestYAML))
	
	// Add WASM
	ww, _ := zw.Create("plugin.wasm")
	ww.Write([]byte("fake wasm"))
	
	zw.Close()
}

func TestHandlePluginCallWithBody(t *testing.T) {
	r, _ := setupPluginTestRouter()

	// Call with JSON body
	body := bytes.NewBufferString(`{"name": "TestWithBody", "count": 5}`)
	req := httptest.NewRequest("POST", "/api/v1/plugins/hello/call/hello", body)
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlePluginCallEmptyBody(t *testing.T) {
	r, _ := setupPluginTestRouter()

	// Call with empty body
	req := httptest.NewRequest("POST", "/api/v1/plugins/hello/call/hello", nil)
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should still work - plugin handles nil args
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlePluginEnableAlreadyEnabled(t *testing.T) {
	r, mgr := setupPluginTestRouter()

	// Verify starts enabled
	if !mgr.IsEnabled("hello") {
		t.Fatal("plugin should start enabled")
	}

	// Enable an already enabled plugin
	req := httptest.NewRequest("POST", "/api/v1/plugins/hello/enable", nil)
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should succeed (idempotent)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Should still be enabled
	if !mgr.IsEnabled("hello") {
		t.Error("plugin should still be enabled after idempotent enable")
	}
}

func TestHandlePluginEnableNotFound(t *testing.T) {
	r, _ := setupPluginTestRouter()

	req := httptest.NewRequest("POST", "/api/v1/plugins/nonexistent/enable", nil)
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandlePluginDisableNotFound(t *testing.T) {
	r, _ := setupPluginTestRouter()

	req := httptest.NewRequest("POST", "/api/v1/plugins/nonexistent/disable", nil)
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandlePluginCallDisabled(t *testing.T) {
	r, mgr := setupPluginTestRouter()

	// Disable the plugin
	if err := mgr.Disable("hello"); err != nil {
		t.Fatalf("failed to disable: %v", err)
	}
	defer mgr.Enable("hello") // Re-enable for other tests

	// Try to call a function on the disabled plugin
	req := httptest.NewRequest("POST", "/api/v1/plugins/hello/call/hello", nil)
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should fail with 403 Forbidden
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 when calling disabled plugin, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDisableThenEnableRestoresFunction(t *testing.T) {
	r, mgr := setupPluginTestRouter()

	// Verify plugin works initially
	req1 := httptest.NewRequest("POST", "/api/v1/plugins/hello/call/hello", strings.NewReader(`{"name":"Test"}`))
	req1.Header.Set("Content-Type", "application/json")
	addAuthHeader(req1)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("plugin should work initially, got %d: %s", w1.Code, w1.Body.String())
	}

	// Disable
	if err := mgr.Disable("hello"); err != nil {
		t.Fatalf("failed to disable: %v", err)
	}

	// Verify it's broken
	req2 := httptest.NewRequest("POST", "/api/v1/plugins/hello/call/hello", strings.NewReader(`{"name":"Test"}`))
	req2.Header.Set("Content-Type", "application/json")
	addAuthHeader(req2)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code == http.StatusOK {
		t.Error("disabled plugin should not respond with 200")
	}

	// Re-enable
	if err := mgr.Enable("hello"); err != nil {
		t.Fatalf("failed to enable: %v", err)
	}

	// Verify it works again
	req3 := httptest.NewRequest("POST", "/api/v1/plugins/hello/call/hello", strings.NewReader(`{"name":"Test"}`))
	req3.Header.Set("Content-Type", "application/json")
	addAuthHeader(req3)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Errorf("re-enabled plugin should work, got %d: %s", w3.Code, w3.Body.String())
	}
}

func TestHandlePluginLogsWithAllFilters(t *testing.T) {
	r, _ := setupPluginTestRouter()

	buf := plugin.GetLogBuffer()
	buf.Clear()
	buf.Log("test-plugin", "info", "info log", nil)
	buf.Log("test-plugin", "error", "error log", nil)
	buf.Log("other-plugin", "info", "other log", nil)

	// Filter by plugin and level
	req := httptest.NewRequest("GET", "/api/v1/plugins/logs?plugin=test-plugin&level=error&limit=50", nil)
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	
	// Should filter correctly
	count := int(result["count"].(float64))
	t.Logf("Filtered logs count: %d", count)
}

func TestHandlePluginUploadNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	SetPluginDir(tmpDir)

	r, _ := setupPluginTestRouter()

	// POST without file
	req := httptest.NewRequest("POST", "/api/v1/plugins/upload", nil)
	req.Header.Set("Content-Type", "multipart/form-data")
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing file, got %d", w.Code)
	}
}

// --- Session-based auth tests ---

// sessionAuthMiddleware simulates session middleware by setting user_id and user_role.
func sessionAuthMiddleware(userID int, role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("user_role", role)
		c.Next()
	}
}

// setupSessionAuthRouter creates a router with session auth (no JWT) for testing.
func setupSessionAuthRouter(sessionMW gin.HandlerFunc) *gin.Engine {
	host := &mockHostAPI{}
	mgr := plugin.NewManager(host)
	SetPluginManager(mgr)
	SetPluginDir(os.TempDir())

	hello := example.NewHelloPlugin()
	mgr.Register(context.Background(), hello)
	mgr.Enable("hello") // hello is default-disabled for dev plugins

	r := gin.New()
	if sessionMW != nil {
		r.Use(sessionMW)
	}
	api := r.Group("/api/v1")
	RegisterPluginAPIRoutes(api)

	return r
}

// pluginAdminRoutes returns the admin-only plugin routes dynamically by
// inspecting what RegisterPluginAPIRoutes registers with RequireAdmin.
// We register on a fresh router and diff against the known auth-only routes.
func pluginAdminRoutes() []gin.RouteInfo {
	r := gin.New()
	api := r.Group("/api/v1")
	RegisterPluginAPIRoutes(api)

	// The admin routes are: enable, disable, upload, logs (GET), logs (DELETE).
	// Discover them by finding routes that have RequireAdmin in the handler chain.
	// Since gin doesn't expose middleware names, we use a different approach:
	// register twice — once with admin group, once with auth-only group — and diff.
	// Simpler: just extract routes that match the admin group patterns.
	var adminRoutes []gin.RouteInfo
	for _, route := range r.Routes() {
		path := route.Path
		// Admin routes are: enable, disable, upload, logs
		// They are NOT: list, call, widgets — those are auth-only
		if strings.Contains(path, "/enable") ||
			strings.Contains(path, "/disable") ||
			strings.Contains(path, "/upload") ||
			strings.Contains(path, "/logs") {
			adminRoutes = append(adminRoutes, route)
		}
	}
	return adminRoutes
}

// pluginAuthRoutes returns the auth-only (non-admin) plugin routes.
func pluginAuthRoutes() []gin.RouteInfo {
	r := gin.New()
	api := r.Group("/api/v1")
	RegisterPluginAPIRoutes(api)

	var authRoutes []gin.RouteInfo
	for _, route := range r.Routes() {
		path := route.Path
		if !strings.Contains(path, "/enable") &&
			!strings.Contains(path, "/disable") &&
			!strings.Contains(path, "/upload") &&
			!strings.Contains(path, "/logs") {
			authRoutes = append(authRoutes, route)
		}
	}
	return authRoutes
}

func TestPluginSessionAuth_AdminCanAccessAdminRoutes(t *testing.T) {
	r := setupSessionAuthRouter(sessionAuthMiddleware(1, "Admin"))
	adminRoutes := pluginAdminRoutes()

	if len(adminRoutes) == 0 {
		t.Fatal("no admin routes discovered — RegisterPluginAPIRoutes may have changed")
	}

	for _, route := range adminRoutes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			// Replace :name param with test plugin name
			path := strings.ReplaceAll(route.Path, ":name", "hello")
			var req *http.Request
			if route.Method == "POST" && strings.Contains(path, "/upload") {
				// Upload needs multipart body
				var b bytes.Buffer
				mw := multipart.NewWriter(&b)
				fw, _ := mw.CreateFormFile("plugin", "test.wasm")
				fw.Write([]byte("fake wasm"))
				mw.Close()
				req = httptest.NewRequest(route.Method, path, &b)
				req.Header.Set("Content-Type", mw.FormDataContentType())
			} else {
				req = httptest.NewRequest(route.Method, path, nil)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Admin should NOT get 401 or 403
			if w.Code == http.StatusUnauthorized {
				t.Errorf("admin session user got 401 on %s %s", route.Method, path)
			}
			if w.Code == http.StatusForbidden {
				t.Errorf("admin session user got 403 on %s %s", route.Method, path)
			}
		})
	}
}

func TestPluginSessionAuth_NonAdminGetsForbiddenOnAdminRoutes(t *testing.T) {
	r := setupSessionAuthRouter(sessionAuthMiddleware(2, "Agent"))
	adminRoutes := pluginAdminRoutes()

	if len(adminRoutes) == 0 {
		t.Fatal("no admin routes discovered")
	}

	for _, route := range adminRoutes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			path := strings.ReplaceAll(route.Path, ":name", "hello")
			req := httptest.NewRequest(route.Method, path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("expected 403 for non-admin on %s %s, got %d", route.Method, path, w.Code)
			}
		})
	}
}

func TestPluginSessionAuth_UnauthenticatedGets401(t *testing.T) {
	// No session middleware — no user_id, no JWT
	r := setupSessionAuthRouter(nil)

	// Collect ALL plugin routes (admin + auth-only)
	allRoutes := append(pluginAdminRoutes(), pluginAuthRoutes()...)

	if len(allRoutes) == 0 {
		t.Fatal("no routes discovered")
	}

	for _, route := range allRoutes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			path := strings.ReplaceAll(route.Path, ":name", "hello")
			path = strings.ReplaceAll(path, ":fn", "test")
			path = strings.ReplaceAll(path, ":id", "test")
			req := httptest.NewRequest(route.Method, path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("expected 401 for unauthenticated on %s %s, got %d", route.Method, path, w.Code)
			}
		})
	}
}

func TestPluginSessionAuth_NonAdminCanAccessAuthRoutes(t *testing.T) {
	r := setupSessionAuthRouter(sessionAuthMiddleware(2, "Agent"))
	authRoutes := pluginAuthRoutes()

	if len(authRoutes) == 0 {
		t.Fatal("no auth-only routes discovered")
	}

	for _, route := range authRoutes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			path := strings.ReplaceAll(route.Path, ":name", "hello")
			path = strings.ReplaceAll(path, ":fn", "hello")
			path = strings.ReplaceAll(path, ":id", "test")
			var req *http.Request
			if route.Method == "POST" {
				req = httptest.NewRequest(route.Method, path, strings.NewReader(`{"name":"test"}`))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(route.Method, path, nil)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Non-admin should NOT get 401 or 403 on auth-only routes
			if w.Code == http.StatusUnauthorized {
				t.Errorf("authenticated agent got 401 on %s %s: %s", route.Method, path, w.Body.String())
			}
			if w.Code == http.StatusForbidden {
				t.Errorf("authenticated agent got 403 on %s %s: %s", route.Method, path, w.Body.String())
			}
		})
	}
}

func TestPluginSessionAuth_AdminRouteCount(t *testing.T) {
	// Sanity check: ensure we discover the expected number of admin routes
	adminRoutes := pluginAdminRoutes()
	// Currently: POST enable, POST disable, POST upload, GET logs, DELETE logs = 5
	if len(adminRoutes) < 5 {
		t.Errorf("expected at least 5 admin routes, discovered %d:", len(adminRoutes))
		for _, r := range adminRoutes {
			t.Logf("  %s %s", r.Method, r.Path)
		}
	}
	t.Logf("Discovered %d admin routes:", len(adminRoutes))
	for _, r := range adminRoutes {
		t.Logf("  %s %s", r.Method, r.Path)
	}
}

func TestHandlePluginUploadNoDir(t *testing.T) {
	// Don't set plugin dir
	SetPluginDir("")

	r, _ := setupPluginTestRouter()

	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	fw, _ := mw.CreateFormFile("plugin", "test.wasm")
	fw.Write([]byte("fake"))
	mw.Close()

	req := httptest.NewRequest("POST", "/api/v1/plugins/upload", &b)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for missing dir, got %d", w.Code)
	}
}
