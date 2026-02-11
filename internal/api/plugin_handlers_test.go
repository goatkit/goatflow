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

var testJWTToken string

func setupPluginTestRouter(t *testing.T) (*gin.Engine, *plugin.Manager) {
	t.Helper()
	// Use real test DB via ProdHostAPI — no mocking DB access
	db := getTestDB(t)
	host := plugin.NewProdHostAPI(plugin.WithDB("default", db))
	mgr := plugin.NewManager(host)
	SetPluginManager(mgr)

	// Register test plugin — seedDefaultDisabled will insert a sysconfig_default
	// entry marking it disabled. Explicitly enable it for tests.
	hello := example.NewHelloPlugin()
	mgr.Register(context.Background(), hello)
	mgr.Enable("hello")

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
	r, _ := setupPluginTestRouter(t)

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

	p := plugins[0].(map[string]any)
	if p["name"] != "hello" {
		t.Errorf("expected hello plugin, got %v", p["name"])
	}
}

func TestHandlePluginCall(t *testing.T) {
	r, _ := setupPluginTestRouter(t)

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
	r, _ := setupPluginTestRouter(t)

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
	r, mgr := setupPluginTestRouter(t)

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
	r, mgr := setupPluginTestRouter(t)

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
	r, _ := setupPluginTestRouter(t)

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
	r, _ := setupPluginTestRouter(t)

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
	r, _ := setupPluginTestRouter(t)

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
	tmpDir := t.TempDir()
	SetPluginDir(tmpDir)

	r, _ := setupPluginTestRouter(t)

	wasmContent := []byte("fake wasm content")

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

	if _, err := os.Stat(filepath.Join(tmpDir, "test.wasm")); os.IsNotExist(err) {
		t.Error("WASM file not saved")
	}
}

func TestHandlePluginUploadInvalidType(t *testing.T) {
	tmpDir := t.TempDir()
	SetPluginDir(tmpDir)

	r, _ := setupPluginTestRouter(t)

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

	r, _ := setupPluginTestRouter(t)

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

	manifestYAML := "name: test-plugin\nversion: \"1.0.0\"\nruntime: wasm\n"
	mw, _ := zw.Create("plugin.yaml")
	mw.Write([]byte(manifestYAML))

	ww, _ := zw.Create("plugin.wasm")
	ww.Write([]byte("fake wasm"))

	zw.Close()
}

func TestHandlePluginCallWithBody(t *testing.T) {
	r, _ := setupPluginTestRouter(t)

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
	r, _ := setupPluginTestRouter(t)

	req := httptest.NewRequest("POST", "/api/v1/plugins/hello/call/hello", nil)
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlePluginEnableAlreadyEnabled(t *testing.T) {
	r, mgr := setupPluginTestRouter(t)

	if !mgr.IsEnabled("hello") {
		t.Fatal("plugin should start enabled")
	}

	req := httptest.NewRequest("POST", "/api/v1/plugins/hello/enable", nil)
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if !mgr.IsEnabled("hello") {
		t.Error("plugin should still be enabled after idempotent enable")
	}
}

func TestHandlePluginEnableNotFound(t *testing.T) {
	r, _ := setupPluginTestRouter(t)

	req := httptest.NewRequest("POST", "/api/v1/plugins/nonexistent/enable", nil)
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandlePluginDisableNotFound(t *testing.T) {
	r, _ := setupPluginTestRouter(t)

	req := httptest.NewRequest("POST", "/api/v1/plugins/nonexistent/disable", nil)
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandlePluginCallDisabled(t *testing.T) {
	r, mgr := setupPluginTestRouter(t)

	if err := mgr.Disable("hello"); err != nil {
		t.Fatalf("failed to disable: %v", err)
	}
	defer mgr.Enable("hello")

	req := httptest.NewRequest("POST", "/api/v1/plugins/hello/call/hello", nil)
	req.Header.Set("Content-Type", "application/json")
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 when calling disabled plugin, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDisableThenEnableRestoresFunction(t *testing.T) {
	r, mgr := setupPluginTestRouter(t)

	// Verify works initially
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

	// Verify broken
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

	// Verify works again
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
	r, _ := setupPluginTestRouter(t)

	// Clear after setup to eliminate noise from plugin registration/enable audit logs
	buf := plugin.GetLogBuffer()
	buf.Clear()
	buf.Log("test-plugin", "info", "info log", nil)
	buf.Log("test-plugin", "error", "error log", nil)
	buf.Log("other-plugin", "info", "other log", nil)

	req := httptest.NewRequest("GET", "/api/v1/plugins/logs?plugin=test-plugin&level=error&limit=50", nil)
	addAuthHeader(req)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)

	count := int(result["count"].(float64))
	if count != 1 {
		t.Errorf("expected 1 filtered log (test-plugin + error), got %d", count)
	}
}

func TestHandlePluginUploadNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	SetPluginDir(tmpDir)

	r, _ := setupPluginTestRouter(t)

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

// setupSessionAuthRouter creates a router with session auth for testing.
func setupSessionAuthRouter(t *testing.T, sessionMW gin.HandlerFunc) *gin.Engine {
	t.Helper()
	db := getTestDB(t)
	host := plugin.NewProdHostAPI(plugin.WithDB("default", db))
	mgr := plugin.NewManager(host)
	SetPluginManager(mgr)
	SetPluginDir(os.TempDir())

	hello := example.NewHelloPlugin()
	mgr.Register(context.Background(), hello)
	mgr.Enable("hello")

	r := gin.New()
	if sessionMW != nil {
		r.Use(sessionMW)
	}
	api := r.Group("/api/v1")
	RegisterPluginAPIRoutes(api)

	return r
}

// pluginAdminRoutes returns the admin-only plugin routes.
func pluginAdminRoutes() []gin.RouteInfo {
	r := gin.New()
	api := r.Group("/api/v1")
	RegisterPluginAPIRoutes(api)

	var adminRoutes []gin.RouteInfo
	for _, route := range r.Routes() {
		path := route.Path
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
	r := setupSessionAuthRouter(t, sessionAuthMiddleware(1, "Admin"))
	adminRoutes := pluginAdminRoutes()

	if len(adminRoutes) == 0 {
		t.Fatal("no admin routes discovered — RegisterPluginAPIRoutes may have changed")
	}

	for _, route := range adminRoutes {
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			path := strings.ReplaceAll(route.Path, ":name", "hello")
			var req *http.Request
			if route.Method == "POST" && strings.Contains(path, "/upload") {
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
	r := setupSessionAuthRouter(t, sessionAuthMiddleware(2, "Agent"))
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
	r := setupSessionAuthRouter(t, nil)

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
	r := setupSessionAuthRouter(t, sessionAuthMiddleware(2, "Agent"))
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
	SetPluginDir("")

	r, _ := setupPluginTestRouter(t)

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
