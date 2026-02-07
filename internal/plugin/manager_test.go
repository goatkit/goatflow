package plugin_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/goatkit/goatflow/internal/plugin"
	"github.com/goatkit/goatflow/internal/plugin/example"
)

// mockHostAPI implements plugin.HostAPI for testing.
type mockHostAPI struct {
	logs []logEntry
}

type logEntry struct {
	level   string
	message string
	fields  map[string]any
}

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

func (m *mockHostAPI) CacheDelete(ctx context.Context, key string) error {
	return nil
}

func (m *mockHostAPI) HTTPRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) (int, []byte, error) {
	return 200, nil, nil
}

func (m *mockHostAPI) SendEmail(ctx context.Context, to, subject, body string, html bool) error {
	return nil
}

// mockPlugin implements plugin.Plugin for testing
type mockPlugin struct{}

func (m *mockPlugin) GKRegister() plugin.GKRegistration {
	return plugin.GKRegistration{
		Name:    "mock-plugin",
		Version: "1.0.0",
	}
}

func (m *mockPlugin) Init(ctx context.Context, host plugin.HostAPI) error {
	return nil
}

func (m *mockPlugin) Call(ctx context.Context, fn string, args json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(map[string]string{"result": "ok"})
}

func (m *mockPlugin) Shutdown(ctx context.Context) error {
	return nil
}

func (m *mockHostAPI) Log(ctx context.Context, level, message string, fields map[string]any) {
	m.logs = append(m.logs, logEntry{level, message, fields})
}

func (m *mockHostAPI) ConfigGet(ctx context.Context, key string) (string, error) {
	return "", nil
}

func (m *mockHostAPI) Translate(ctx context.Context, key string, args ...any) string {
	return ""
}

func (m *mockHostAPI) CallPlugin(ctx context.Context, pluginName, function string, args json.RawMessage) (json.RawMessage, error) {
	return nil, nil
}

func TestPluginManager(t *testing.T) {
	ctx := context.Background()
	host := &mockHostAPI{}
	mgr := plugin.NewManager(host)

	// Create and register the hello plugin
	hello := example.NewHelloPlugin()

	t.Run("Register", func(t *testing.T) {
		err := mgr.Register(ctx, hello)
		if err != nil {
			t.Fatalf("Register failed: %v", err)
		}

		// Verify init was called (check logs)
		if len(host.logs) == 0 {
			t.Error("Expected init log entry")
		}
	})

	t.Run("List", func(t *testing.T) {
		manifests := mgr.List()
		if len(manifests) != 1 {
			t.Fatalf("Expected 1 plugin, got %d", len(manifests))
		}
		if manifests[0].Name != "hello" {
			t.Errorf("Expected plugin name 'hello', got %q", manifests[0].Name)
		}
	})

	t.Run("Get", func(t *testing.T) {
		p, ok := mgr.Get("hello")
		if !ok {
			t.Fatal("Plugin 'hello' not found")
		}
		if p.GKRegister().Version != "1.0.0" {
			t.Errorf("Expected version 1.0.0, got %s", p.GKRegister().Version)
		}
	})

	t.Run("Call", func(t *testing.T) {
		args, _ := json.Marshal(map[string]string{"name": "GoatKit"})
		result, err := mgr.Call(ctx, "hello", "hello", args)
		if err != nil {
			t.Fatalf("Call failed: %v", err)
		}

		var resp map[string]any
		if err := json.Unmarshal(result, &resp); err != nil {
			t.Fatalf("Unmarshal result failed: %v", err)
		}

		msg, ok := resp["message"].(string)
		if !ok || msg != "Hello, GoatKit!" {
			t.Errorf("Expected 'Hello, GoatKit!', got %q", msg)
		}
	})

	t.Run("Routes", func(t *testing.T) {
		routes := mgr.Routes()
		if len(routes) != 2 {
			t.Fatalf("Expected 2 routes, got %d", len(routes))
		}

		found := false
		for _, r := range routes {
			if r.RouteSpec.Path == "/api/plugins/hello" {
				found = true
				if r.PluginName != "hello" {
					t.Errorf("Expected plugin name 'hello', got %q", r.PluginName)
				}
			}
		}
		if !found {
			t.Error("Route /api/plugins/hello not found")
		}
	})

	t.Run("MenuItems", func(t *testing.T) {
		items := mgr.MenuItems("admin")
		if len(items) != 1 {
			t.Fatalf("Expected 1 menu item, got %d", len(items))
		}
		if items[0].ID != "hello-menu" {
			t.Errorf("Expected menu ID 'hello-menu', got %q", items[0].ID)
		}
	})

	t.Run("Widgets", func(t *testing.T) {
		widgets := mgr.Widgets("dashboard")
		if len(widgets) != 1 {
			t.Fatalf("Expected 1 widget, got %d", len(widgets))
		}
		if widgets[0].ID != "hello-widget" {
			t.Errorf("Expected widget ID 'hello-widget', got %q", widgets[0].ID)
		}
	})

	t.Run("Jobs", func(t *testing.T) {
		jobs := mgr.Jobs()
		if len(jobs) != 1 {
			t.Fatalf("Expected 1 job, got %d", len(jobs))
		}
		if jobs[0].Schedule != "0 * * * *" {
			t.Errorf("Expected schedule '0 * * * *', got %q", jobs[0].Schedule)
		}
	})

	t.Run("Disable", func(t *testing.T) {
		err := mgr.Disable("hello")
		if err != nil {
			t.Fatalf("Disable failed: %v", err)
		}

		_, ok := mgr.Get("hello")
		if ok {
			t.Error("Expected disabled plugin to not be returned by Get")
		}

		// Routes should be empty when disabled
		routes := mgr.Routes()
		if len(routes) != 0 {
			t.Errorf("Expected 0 routes when disabled, got %d", len(routes))
		}
	})

	t.Run("Enable", func(t *testing.T) {
		err := mgr.Enable("hello")
		if err != nil {
			t.Fatalf("Enable failed: %v", err)
		}

		_, ok := mgr.Get("hello")
		if !ok {
			t.Error("Expected enabled plugin to be returned by Get")
		}
	})

	t.Run("Unregister", func(t *testing.T) {
		err := mgr.Unregister(ctx, "hello")
		if err != nil {
			t.Fatalf("Unregister failed: %v", err)
		}

		_, ok := mgr.Get("hello")
		if ok {
			t.Error("Plugin should not exist after unregister")
		}
	})
}

func TestPluginManagerDuplicateRegister(t *testing.T) {
	ctx := context.Background()
	host := &mockHostAPI{}
	mgr := plugin.NewManager(host)

	hello1 := example.NewHelloPlugin()
	hello2 := example.NewHelloPlugin()

	if err := mgr.Register(ctx, hello1); err != nil {
		t.Fatalf("First register failed: %v", err)
	}

	err := mgr.Register(ctx, hello2)
	if err == nil {
		t.Error("Expected error registering duplicate plugin")
	}
}

func TestPluginManagerCallErrors(t *testing.T) {
	ctx := context.Background()
	host := &mockHostAPI{}
	mgr := plugin.NewManager(host)

	hello := example.NewHelloPlugin()
	mgr.Register(ctx, hello)

	t.Run("Call non-existent plugin", func(t *testing.T) {
		_, err := mgr.Call(ctx, "nonexistent", "hello", nil)
		if err == nil {
			t.Error("Expected error calling non-existent plugin")
		}
	})

	t.Run("Call disabled plugin", func(t *testing.T) {
		mgr.Disable("hello")
		_, err := mgr.Call(ctx, "hello", "hello", nil)
		if err == nil {
			t.Error("Expected error calling disabled plugin")
		}
		mgr.Enable("hello") // Re-enable
	})

	t.Run("Call with nil args", func(t *testing.T) {
		result, err := mgr.Call(ctx, "hello", "hello", nil)
		if err != nil {
			t.Errorf("Call with nil args should work: %v", err)
		}
		if result == nil {
			t.Error("Expected non-nil result")
		}
	})
}

func TestPluginManagerListAll(t *testing.T) {
	ctx := context.Background()
	host := &mockHostAPI{}
	mgr := plugin.NewManager(host)

	hello := example.NewHelloPlugin()
	mgr.Register(ctx, hello)

	t.Run("List includes all plugins", func(t *testing.T) {
		manifests := mgr.List()
		if len(manifests) != 1 {
			t.Errorf("Expected 1 plugin, got %d", len(manifests))
		}
	})

	t.Run("List includes disabled plugins", func(t *testing.T) {
		mgr.Disable("hello")
		manifests := mgr.List()
		// List() returns all plugins regardless of enabled state
		if len(manifests) != 1 {
			t.Errorf("Expected 1 plugin (disabled but still listed), got %d", len(manifests))
		}
		mgr.Enable("hello")
	})
}

func TestPluginManagerGetRoutes(t *testing.T) {
	ctx := context.Background()
	host := &mockHostAPI{}
	mgr := plugin.NewManager(host)

	hello := example.NewHelloPlugin()
	mgr.Register(ctx, hello)

	routes := mgr.Routes()
	
	// Hello plugin has routes
	found := false
	for _, r := range routes {
		if r.RouteSpec.Path == "/api/plugins/hello" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected hello route in Routes()")
	}
}

func TestPluginManagerWidgets(t *testing.T) {
	ctx := context.Background()
	host := &mockHostAPI{}
	mgr := plugin.NewManager(host)

	hello := example.NewHelloPlugin()
	mgr.Register(ctx, hello)

	// Widgets requires a location parameter
	widgets := mgr.Widgets("agent_home")
	
	// Hello plugin may or may not have widgets at this location
	t.Logf("Found %d widgets at agent_home location", len(widgets))

	// Test non-existent location (should return empty)
	noWidgets := mgr.Widgets("nonexistent-location")
	if len(noWidgets) != 0 {
		t.Errorf("expected 0 widgets for nonexistent location, got %d", len(noWidgets))
	}

	// Test with disabled plugin
	mgr.Disable("hello")
	disabledWidgets := mgr.Widgets("agent_home")
	if len(disabledWidgets) != 0 {
		t.Errorf("expected 0 widgets after disabling, got %d", len(disabledWidgets))
	}
}

func TestPluginManagerShutdownAll(t *testing.T) {
	ctx := context.Background()
	host := &mockHostAPI{}
	mgr := plugin.NewManager(host)

	hello := example.NewHelloPlugin()
	mgr.Register(ctx, hello)

	// Verify plugin exists
	_, ok := mgr.Get("hello")
	if !ok {
		t.Fatal("plugin should exist before shutdown")
	}

	// Shutdown all
	err := mgr.ShutdownAll(ctx)
	if err != nil {
		t.Errorf("ShutdownAll failed: %v", err)
	}
}

func TestPluginManagerJobs(t *testing.T) {
	ctx := context.Background()
	host := &mockHostAPI{}
	mgr := plugin.NewManager(host)

	hello := example.NewHelloPlugin()
	mgr.Register(ctx, hello)

	jobs := mgr.Jobs()
	t.Logf("Found %d jobs", len(jobs))
}

func TestPluginManagerMenuItems(t *testing.T) {
	ctx := context.Background()
	host := &mockHostAPI{}
	mgr := plugin.NewManager(host)

	hello := example.NewHelloPlugin()
	mgr.Register(ctx, hello)

	// Test different locations
	adminItems := mgr.MenuItems("admin")
	t.Logf("Found %d admin menu items", len(adminItems))

	agentItems := mgr.MenuItems("agent")
	t.Logf("Found %d agent menu items", len(agentItems))

	// Test non-existent location (should return empty)
	noItems := mgr.MenuItems("nonexistent-location")
	if len(noItems) != 0 {
		t.Errorf("expected 0 items for nonexistent location, got %d", len(noItems))
	}

	// Test with disabled plugin
	mgr.Disable("hello")
	disabledItems := mgr.MenuItems("admin")
	if len(disabledItems) != 0 {
		t.Errorf("expected 0 items after disabling, got %d", len(disabledItems))
	}
}

func TestPluginManagerSetLazyLoader(t *testing.T) {
	host := &mockHostAPI{}
	mgr := plugin.NewManager(host)

	// Setting nil should work
	mgr.SetLazyLoader(nil)
}

func TestPluginManagerRegisterEdgeCases(t *testing.T) {
	ctx := context.Background()
	host := &mockHostAPI{}
	mgr := plugin.NewManager(host)

	t.Run("register duplicate", func(t *testing.T) {
		mockP := &mockPlugin{}
		err := mgr.Register(ctx, mockP)
		if err != nil {
			t.Fatalf("first register failed: %v", err)
		}

		// Try to register again
		err = mgr.Register(ctx, mockP)
		if err == nil {
			t.Error("expected error for duplicate registration")
		}
	})

	t.Run("register with init failure", func(t *testing.T) {
		mgr2 := plugin.NewManager(host)
		failP := &mockFailingPlugin{}
		err := mgr2.Register(ctx, failP)
		if err == nil {
			t.Error("expected error when init fails")
		}
	})
}

// mockFailingPlugin fails on Init
type mockFailingPlugin struct{}

func (m *mockFailingPlugin) GKRegister() plugin.GKRegistration {
	return plugin.GKRegistration{
		Name:    "failing-plugin",
		Version: "1.0.0",
	}
}

func (m *mockFailingPlugin) Init(ctx context.Context, host plugin.HostAPI) error {
	return fmt.Errorf("init failed intentionally")
}

func (m *mockFailingPlugin) Call(ctx context.Context, fn string, args json.RawMessage) (json.RawMessage, error) {
	return nil, nil
}

func (m *mockFailingPlugin) Shutdown(ctx context.Context) error {
	return nil
}

func TestPluginManagerUnregisterEdgeCases(t *testing.T) {
	ctx := context.Background()
	host := &mockHostAPI{}
	mgr := plugin.NewManager(host)

	t.Run("unregister nonexistent", func(t *testing.T) {
		err := mgr.Unregister(ctx, "nonexistent")
		if err == nil {
			t.Error("expected error for nonexistent plugin")
		}
	})

	t.Run("unregister existing", func(t *testing.T) {
		mockP := &mockPlugin{}
		mgr.Register(ctx, mockP)

		err := mgr.Unregister(ctx, "mock-plugin")
		if err != nil {
			t.Errorf("unregister failed: %v", err)
		}

		// Verify it's gone
		_, ok := mgr.Get("mock-plugin")
		if ok {
			t.Error("plugin should be removed after unregister")
		}
	})
}

func TestPluginManagerEnableDisable(t *testing.T) {
	ctx := context.Background()
	host := &mockHostAPI{}
	mgr := plugin.NewManager(host)

	mockP := &mockPlugin{}
	mgr.Register(ctx, mockP)

	t.Run("disable existing plugin", func(t *testing.T) {
		err := mgr.Disable("mock-plugin")
		if err != nil {
			t.Errorf("disable failed: %v", err)
		}
	})

	t.Run("enable existing plugin", func(t *testing.T) {
		err := mgr.Enable("mock-plugin")
		if err != nil {
			t.Errorf("enable failed: %v", err)
		}
	})

	t.Run("disable nonexistent", func(t *testing.T) {
		err := mgr.Disable("nonexistent")
		if err == nil {
			t.Error("expected error for nonexistent plugin")
		}
	})

	t.Run("enable nonexistent", func(t *testing.T) {
		err := mgr.Enable("nonexistent")
		if err == nil {
			t.Error("expected error for nonexistent plugin")
		}
	})
}

// mockPluginWithTemplates has template overrides
type mockPluginWithTemplates struct{}

func (m *mockPluginWithTemplates) GKRegister() plugin.GKRegistration {
	return plugin.GKRegistration{
		Name:    "templates-plugin",
		Version: "1.0.0",
		Templates: []plugin.TemplateSpec{
			{
				Name:     "base/header.html",
				Path:     "templates/header.html",
				Override: true,
			},
		},
	}
}

func (m *mockPluginWithTemplates) Init(ctx context.Context, host plugin.HostAPI) error {
	return nil
}

func (m *mockPluginWithTemplates) Call(ctx context.Context, fn string, args json.RawMessage) (json.RawMessage, error) {
	return nil, nil
}

func (m *mockPluginWithTemplates) Shutdown(ctx context.Context) error {
	return nil
}

func TestPluginManagerRegisterWithTemplates(t *testing.T) {
	ctx := context.Background()
	host := &mockHostAPI{}
	mgr := plugin.NewManager(host)

	// Ensure template override registry exists
	plugin.SetTemplateOverrides(plugin.NewTemplateOverrideRegistry(mgr))

	mockP := &mockPluginWithTemplates{}
	err := mgr.Register(ctx, mockP)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Verify template was registered
	registry := plugin.GetTemplateOverrides()
	if !registry.HasOverride("base/header.html") {
		t.Error("template override should be registered")
	}
}

func TestPluginManagerCallFrom(t *testing.T) {
	ctx := context.Background()
	host := &mockHostAPI{}
	mgr := plugin.NewManager(host)

	// Register a mock plugin
	mockP := &mockPlugin{}
	mgr.Register(ctx, mockP)

	t.Run("CallFrom_success", func(t *testing.T) {
		result, err := mgr.CallFrom(ctx, "caller", "mock-plugin", "test_func", nil)
		if err != nil {
			t.Errorf("CallFrom failed: %v", err)
		}
		if result == nil {
			t.Error("expected result")
		}
	})

	t.Run("CallFrom_not_found", func(t *testing.T) {
		_, err := mgr.CallFrom(ctx, "caller", "nonexistent", "func", nil)
		if err == nil {
			t.Error("expected error for nonexistent plugin")
		}
		// Error should mention caller
		errStr := err.Error()
		if !containsString(errStr, "caller") {
			t.Errorf("error should mention caller: %s", errStr)
		}
	})

	t.Run("CallFrom_disabled", func(t *testing.T) {
		mgr.Disable("mock-plugin")
		_, err := mgr.CallFrom(ctx, "caller", "mock-plugin", "func", nil)
		if err == nil {
			t.Error("expected error for disabled plugin")
		}
		errStr := err.Error()
		if !containsString(errStr, "disabled") {
			t.Errorf("error should mention disabled: %s", errStr)
		}
		mgr.Enable("mock-plugin")
	})
}

func TestPluginNotFoundError(t *testing.T) {
	t.Run("without caller", func(t *testing.T) {
		err := &plugin.PluginNotFoundError{
			PluginName: "missing",
			Function:   "test",
		}
		errStr := err.Error()
		if !containsString(errStr, "missing") {
			t.Errorf("error should mention plugin name: %s", errStr)
		}
	})

	t.Run("with caller", func(t *testing.T) {
		err := &plugin.PluginNotFoundError{
			PluginName:   "missing",
			CallerPlugin: "caller",
			Function:     "test",
		}
		errStr := err.Error()
		if !containsString(errStr, "caller") {
			t.Errorf("error should mention caller: %s", errStr)
		}
	})
}

func TestPluginDisabledError(t *testing.T) {
	t.Run("without caller", func(t *testing.T) {
		err := &plugin.PluginDisabledError{
			PluginName: "disabled-plugin",
		}
		errStr := err.Error()
		if !containsString(errStr, "disabled") {
			t.Errorf("error should mention disabled: %s", errStr)
		}
	})

	t.Run("with caller", func(t *testing.T) {
		err := &plugin.PluginDisabledError{
			PluginName:   "disabled-plugin",
			CallerPlugin: "caller",
		}
		errStr := err.Error()
		if !containsString(errStr, "caller") {
			t.Errorf("error should mention caller: %s", errStr)
		}
	})
}

// helper
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// mockLazyLoader for testing lazy loading
type mockLazyLoader struct {
	discovered []string
	loadErr    error
	loaded     map[string]bool
}

func (m *mockLazyLoader) EnsureLoaded(ctx context.Context, name string) error {
	if m.loadErr != nil {
		return m.loadErr
	}
	if m.loaded == nil {
		m.loaded = make(map[string]bool)
	}
	m.loaded[name] = true
	return nil
}

func (m *mockLazyLoader) Discovered() []string {
	return m.discovered
}

func TestPluginManagerWithLazyLoader(t *testing.T) {
	ctx := context.Background()
	host := &mockHostAPI{}
	mgr := plugin.NewManager(host)

	t.Run("Call_triggers_lazy_load", func(t *testing.T) {
		loader := &mockLazyLoader{
			discovered: []string{"lazy-plugin"},
			loadErr:    nil,
		}
		mgr.SetLazyLoader(loader)

		// Call a plugin that doesn't exist - should try lazy loading
		_, err := mgr.Call(ctx, "lazy-plugin", "test", nil)
		
		// Will still fail because EnsureLoaded doesn't actually register
		if err == nil {
			t.Error("expected error since plugin still not registered")
		}
		
		// But verify lazy load was attempted
		if !loader.loaded["lazy-plugin"] {
			t.Error("lazy loader should have been called")
		}
	})

	t.Run("Call_lazy_load_error", func(t *testing.T) {
		loader := &mockLazyLoader{
			discovered: []string{"error-plugin"},
			loadErr:    fmt.Errorf("load failed"),
		}
		mgr.SetLazyLoader(loader)

		_, err := mgr.Call(ctx, "error-plugin", "test", nil)
		if err == nil {
			t.Error("expected error when lazy load fails")
		}
	})

	t.Run("CallFrom_triggers_lazy_load", func(t *testing.T) {
		loader := &mockLazyLoader{
			discovered: []string{"lazy-target"},
		}
		mgr.SetLazyLoader(loader)

		_, err := mgr.CallFrom(ctx, "caller", "lazy-target", "test", nil)
		if err == nil {
			t.Error("expected error since plugin not actually loaded")
		}
		
		if !loader.loaded["lazy-target"] {
			t.Error("lazy loader should have been called for target")
		}
	})
}
