package example

import (
	"context"
	"encoding/json"
	"testing"
)

// mockHostAPI for testing
type mockHostAPI struct {
	logs []string
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
func (m *mockHostAPI) CacheDelete(ctx context.Context, key string) error { return nil }
func (m *mockHostAPI) HTTPRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) (int, []byte, error) {
	return 200, nil, nil
}
func (m *mockHostAPI) SendEmail(ctx context.Context, to, subject, body string, html bool) error {
	return nil
}
func (m *mockHostAPI) Log(ctx context.Context, level, message string, fields map[string]any) {
	m.logs = append(m.logs, message)
}
func (m *mockHostAPI) ConfigGet(ctx context.Context, key string) (string, error) { return "", nil }
func (m *mockHostAPI) Translate(ctx context.Context, key string, args ...any) string {
	return ""
}
func (m *mockHostAPI) CallPlugin(ctx context.Context, pluginName, function string, args json.RawMessage) (json.RawMessage, error) {
	return nil, nil
}

func (m *mockHostAPI) PublishEvent(ctx context.Context, eventType string, data string) error {
	return nil
}

func TestHelloPlugin(t *testing.T) {
	ctx := context.Background()
	host := &mockHostAPI{}

	p := NewHelloPlugin()

	t.Run("GKRegister", func(t *testing.T) {
		reg := p.GKRegister()

		if reg.Name != "hello" {
			t.Errorf("expected name 'hello', got %s", reg.Name)
		}
		if reg.Version != "1.0.0" {
			t.Errorf("expected version '1.0.0', got %s", reg.Version)
		}
		if len(reg.Routes) == 0 {
			t.Error("expected at least one route")
		}
	})

	t.Run("Init", func(t *testing.T) {
		err := p.Init(ctx, host)
		if err != nil {
			t.Errorf("Init failed: %v", err)
		}
	})

	t.Run("Call_hello", func(t *testing.T) {
		args, _ := json.Marshal(map[string]string{"name": "TestUser"})
		result, err := p.Call(ctx, "hello", args)
		if err != nil {
			t.Fatalf("Call failed: %v", err)
		}

		var response map[string]any
		if err := json.Unmarshal(result, &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		msg, ok := response["message"].(string)
		if !ok {
			t.Fatal("response should have 'message'")
		}
		if msg == "" {
			t.Error("message should not be empty")
		}
	})

	t.Run("Call_hello_default_name", func(t *testing.T) {
		// Call without name arg
		result, err := p.Call(ctx, "hello", nil)
		if err != nil {
			t.Fatalf("Call failed: %v", err)
		}

		var response map[string]any
		json.Unmarshal(result, &response)

		msg := response["message"].(string)
		// Should use default name
		if msg == "" {
			t.Error("message should not be empty")
		}
	})

	t.Run("Call_stats", func(t *testing.T) {
		result, err := p.Call(ctx, "stats", nil)
		if err != nil {
			t.Fatalf("Call failed: %v", err)
		}

		var response map[string]any
		if err := json.Unmarshal(result, &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// Check for expected stats fields
		if _, ok := response["call_count"]; !ok {
			t.Error("expected 'call_count' in stats")
		}
	})

	t.Run("Call_widget", func(t *testing.T) {
		result, err := p.Call(ctx, "widget", nil)
		if err != nil {
			t.Fatalf("Call failed: %v", err)
		}

		var response map[string]string
		if err := json.Unmarshal(result, &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		html, ok := response["html"]
		if !ok {
			t.Error("expected 'html' in widget response")
		}
		if html == "" {
			t.Error("html should not be empty")
		}
	})

	t.Run("Call_scheduled_hello", func(t *testing.T) {
		result, err := p.Call(ctx, "scheduled_hello", nil)
		if err != nil {
			t.Fatalf("Call failed: %v", err)
		}

		var response map[string]bool
		if err := json.Unmarshal(result, &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if !response["ok"] {
			t.Error("expected ok=true in response")
		}
	})

	t.Run("Call_unknown_function", func(t *testing.T) {
		_, err := p.Call(ctx, "nonexistent", nil)
		if err == nil {
			t.Error("expected error for unknown function")
		}
	})

	t.Run("Shutdown", func(t *testing.T) {
		err := p.Shutdown(ctx)
		if err != nil {
			t.Errorf("Shutdown failed: %v", err)
		}
	})
}

func TestHelloPluginRoutes(t *testing.T) {
	p := NewHelloPlugin()
	reg := p.GKRegister()

	// Check routes are defined correctly
	routePaths := make(map[string]bool)
	for _, r := range reg.Routes {
		routePaths[r.Path] = true
	}

	if !routePaths["/api/plugins/hello"] {
		t.Error("expected /api/plugins/hello route")
	}
	if !routePaths["/api/plugins/hello/stats"] {
		t.Error("expected /api/plugins/hello/stats route")
	}
}

func TestHelloPluginMenuItems(t *testing.T) {
	p := NewHelloPlugin()
	reg := p.GKRegister()

	if len(reg.MenuItems) == 0 {
		t.Error("expected at least one menu item")
	}

	found := false
	for _, mi := range reg.MenuItems {
		if mi.ID == "hello-menu" {
			found = true
			if mi.Location != "admin" {
				t.Errorf("expected admin location, got %s", mi.Location)
			}
		}
	}
	if !found {
		t.Error("expected hello-menu menu item")
	}
}
