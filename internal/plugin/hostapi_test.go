package plugin

import (
	"context"
	"encoding/json"
	"testing"
)

func TestHostAPIInterface(t *testing.T) {
	// Test that HostAPI interface is correctly defined
	var _ HostAPI = (*testHostAPI)(nil)
}

func TestDefaultHostAPI(t *testing.T) {
	ctx := context.Background()
	host := NewDefaultHostAPI()

	t.Run("DBQuery returns empty", func(t *testing.T) {
		rows, err := host.DBQuery(ctx, "SELECT 1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if rows != nil {
			t.Errorf("expected nil, got %v", rows)
		}
	})

	t.Run("DBExec returns 0", func(t *testing.T) {
		affected, err := host.DBExec(ctx, "UPDATE test SET x = 1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if affected != 0 {
			t.Errorf("expected 0, got %d", affected)
		}
	})

	t.Run("CacheGet returns not found", func(t *testing.T) {
		val, found, err := host.CacheGet(ctx, "key")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if found {
			t.Error("expected not found")
		}
		if val != nil {
			t.Errorf("expected nil, got %v", val)
		}
	})

	t.Run("CacheSet does nothing", func(t *testing.T) {
		err := host.CacheSet(ctx, "key", []byte("value"), 60)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("CacheDelete does nothing", func(t *testing.T) {
		err := host.CacheDelete(ctx, "key")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("HTTPRequest returns 200", func(t *testing.T) {
		status, body, err := host.HTTPRequest(ctx, "GET", "http://example.com", nil, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if status != 200 {
			t.Errorf("expected 200 status, got %d", status)
		}
		if body != nil {
			t.Errorf("expected nil body, got %v", body)
		}
	})

	t.Run("SendEmail does nothing", func(t *testing.T) {
		err := host.SendEmail(ctx, "to@test.com", "subject", "body", false)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Log does nothing", func(t *testing.T) {
		// Should not panic
		host.Log(ctx, "info", "test message", nil)
	})

	t.Run("ConfigGet returns empty", func(t *testing.T) {
		val, err := host.ConfigGet(ctx, "key")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if val != "" {
			t.Errorf("expected empty, got %s", val)
		}
	})

	t.Run("Translate returns empty", func(t *testing.T) {
		result := host.Translate(ctx, "hello.world")
		if result != "" {
			t.Errorf("expected empty, got %s", result)
		}
	})

	t.Run("CallPlugin returns error", func(t *testing.T) {
		_, err := host.CallPlugin(ctx, "plugin", "func", nil)
		if err == nil {
			t.Error("expected error from default CallPlugin")
		}
	})
}

// testHostAPI is a complete implementation for testing
type testHostAPI struct {
	queries     []string
	execs       []string
	cacheStore  map[string][]byte
	logs        []string
	configStore map[string]string
}

func newTestHostAPI() *testHostAPI {
	return &testHostAPI{
		cacheStore:  make(map[string][]byte),
		configStore: make(map[string]string),
	}
}

func (h *testHostAPI) DBQuery(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	h.queries = append(h.queries, query)
	return []map[string]any{{"id": 1, "name": "test"}}, nil
}

func (h *testHostAPI) DBExec(ctx context.Context, query string, args ...any) (int64, error) {
	h.execs = append(h.execs, query)
	return 1, nil
}

func (h *testHostAPI) CacheGet(ctx context.Context, key string) ([]byte, bool, error) {
	v, ok := h.cacheStore[key]
	return v, ok, nil
}

func (h *testHostAPI) CacheSet(ctx context.Context, key string, value []byte, ttlSeconds int) error {
	h.cacheStore[key] = value
	return nil
}

func (h *testHostAPI) CacheDelete(ctx context.Context, key string) error {
	delete(h.cacheStore, key)
	return nil
}

func (h *testHostAPI) HTTPRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) (int, []byte, error) {
	return 200, []byte(`{"status": "ok"}`), nil
}

func (h *testHostAPI) SendEmail(ctx context.Context, to, subject, body string, html bool) error {
	return nil
}

func (h *testHostAPI) Log(ctx context.Context, level, message string, fields map[string]any) {
	h.logs = append(h.logs, message)
}

func (h *testHostAPI) ConfigGet(ctx context.Context, key string) (string, error) {
	return h.configStore[key], nil
}

func (h *testHostAPI) Translate(ctx context.Context, key string, args ...any) string {
	return key
}

func (h *testHostAPI) CallPlugin(ctx context.Context, pluginName, function string, args json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(map[string]string{"called": pluginName + "." + function})
}

func (h *testHostAPI) PublishEvent(ctx context.Context, eventType string, data string) error {
	return nil
}

func TestHostAPIUsage(t *testing.T) {
	ctx := context.Background()
	host := newTestHostAPI()

	t.Run("DBQuery", func(t *testing.T) {
		results, err := host.DBQuery(ctx, "SELECT * FROM test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(results) == 0 {
			t.Error("expected results")
		}
		if len(host.queries) != 1 {
			t.Errorf("expected 1 query, got %d", len(host.queries))
		}
	})

	t.Run("DBExec", func(t *testing.T) {
		affected, err := host.DBExec(ctx, "UPDATE test SET name = ?", "new")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if affected != 1 {
			t.Errorf("expected 1 affected, got %d", affected)
		}
	})

	t.Run("Cache", func(t *testing.T) {
		// Set
		err := host.CacheSet(ctx, "key1", []byte("value1"), 60)
		if err != nil {
			t.Errorf("CacheSet failed: %v", err)
		}

		// Get
		val, found, err := host.CacheGet(ctx, "key1")
		if err != nil {
			t.Errorf("CacheGet failed: %v", err)
		}
		if !found {
			t.Error("expected to find cached value")
		}
		if string(val) != "value1" {
			t.Errorf("expected value1, got %s", string(val))
		}

		// Delete
		err = host.CacheDelete(ctx, "key1")
		if err != nil {
			t.Errorf("CacheDelete failed: %v", err)
		}

		// Verify deleted
		_, found, _ = host.CacheGet(ctx, "key1")
		if found {
			t.Error("expected value to be deleted")
		}
	})

	t.Run("HTTPRequest", func(t *testing.T) {
		status, body, err := host.HTTPRequest(ctx, "GET", "https://example.com", nil, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if status != 200 {
			t.Errorf("expected 200, got %d", status)
		}
		if len(body) == 0 {
			t.Error("expected body")
		}
	})

	t.Run("Log", func(t *testing.T) {
		host.Log(ctx, "info", "test message", nil)
		if len(host.logs) != 1 {
			t.Errorf("expected 1 log, got %d", len(host.logs))
		}
	})

	t.Run("CallPlugin", func(t *testing.T) {
		result, err := host.CallPlugin(ctx, "other", "func", nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result == nil {
			t.Error("expected result")
		}
	})
}

func TestPluginContextKey(t *testing.T) {
	// Test that context keys are defined and usable
	ctx := context.Background()
	
	// Set and retrieve caller
	ctx = context.WithValue(ctx, PluginCallerKey, "test-plugin")
	if v := ctx.Value(PluginCallerKey); v != "test-plugin" {
		t.Error("PluginCallerKey context should work")
	}
	
	// Set and retrieve language
	ctx = context.WithValue(ctx, PluginLanguageKey, "en")
	if v := ctx.Value(PluginLanguageKey); v != "en" {
		t.Error("PluginLanguageKey context should work")
	}
}

func TestGKRegistration(t *testing.T) {
	reg := GKRegistration{
		Name:        "test-plugin",
		Version:     "1.0.0",
		Description: "Test plugin",
		Author:      "Test Author",
		License:     "MIT",
		Routes: []RouteSpec{
			{Method: "GET", Path: "/api/test", Handler: "test"},
		},
		MenuItems: []MenuItemSpec{
			{ID: "test-menu", Label: "Test", Location: "admin"},
		},
		Widgets: []WidgetSpec{
			{ID: "test-widget", Title: "Test Widget", Location: "admin_home"},
		},
		Jobs: []JobSpec{
			{ID: "test-job", Schedule: "0 * * * *", Handler: "job"},
		},
	}

	if reg.Name != "test-plugin" {
		t.Errorf("expected test-plugin, got %s", reg.Name)
	}
	if len(reg.Routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(reg.Routes))
	}
	if len(reg.MenuItems) != 1 {
		t.Errorf("expected 1 menu item, got %d", len(reg.MenuItems))
	}
	if len(reg.Widgets) != 1 {
		t.Errorf("expected 1 widget, got %d", len(reg.Widgets))
	}
	if len(reg.Jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(reg.Jobs))
	}
}

func TestRouteSpec(t *testing.T) {
	route := RouteSpec{
		Method:      "POST",
		Path:        "/api/v1/test",
		Handler:     "handle_test",
		Middleware:  []string{"auth", "admin"},
		Description: "Test route",
	}

	if route.Method != "POST" {
		t.Error("method mismatch")
	}
	if len(route.Middleware) != 2 {
		t.Error("middleware count mismatch")
	}
}

func TestWidgetSpec(t *testing.T) {
	widget := WidgetSpec{
		ID:          "stats-widget",
		Title:       "Statistics",
		Description: "Shows stats",
		Handler:     "render_stats",
		Location:    "agent_home",
		Size:        "medium",
	}

	if widget.ID != "stats-widget" {
		t.Error("id mismatch")
	}
	if widget.Size != "medium" {
		t.Error("size mismatch")
	}
}

func TestJobSpecStruct(t *testing.T) {
	job := JobSpec{
		ID:          "cleanup",
		Schedule:    "0 0 * * *",
		Handler:     "run_cleanup",
		Description: "Daily cleanup",
		Enabled:     true,
		Timeout:     "5m",
	}

	if job.ID != "cleanup" {
		t.Error("id mismatch")
	}
	if !job.Enabled {
		t.Error("should be enabled")
	}
}
