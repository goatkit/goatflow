package plugin_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gotrs-io/gotrs-ce/internal/plugin"
	"github.com/gotrs-io/gotrs-ce/internal/plugin/example"
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

func (m *mockHostAPI) Log(ctx context.Context, level, message string, fields map[string]any) {
	m.logs = append(m.logs, logEntry{level, message, fields})
}

func (m *mockHostAPI) ConfigGet(ctx context.Context, key string) (string, error) {
	return "", nil
}

func (m *mockHostAPI) Translate(ctx context.Context, key string, args ...any) string {
	return ""
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
