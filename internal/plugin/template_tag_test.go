package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/flosch/pongo2/v6"
)

func TestSetTemplatePluginManager(t *testing.T) {
	// Initially nil
	original := templatePluginManager
	defer func() { templatePluginManager = original }()

	mgr := NewManager(nil)
	SetTemplatePluginManager(mgr)

	if templatePluginManager != mgr {
		t.Error("plugin manager not set")
	}

	// Can set to nil
	SetTemplatePluginManager(nil)
	if templatePluginManager != nil {
		t.Error("should be nil")
	}
}

// mockTagPlugin is a simple plugin for template tag tests
type mockTagPlugin struct{}

func (m *mockTagPlugin) GKRegister() GKRegistration {
	return GKRegistration{
		Name:        "tagmock",
		Version:     "1.0.0",
		Description: "Mock plugin for tests",
		Widgets: []WidgetSpec{
			{ID: "test-widget", Handler: "render_widget"},
		},
	}
}

func (m *mockTagPlugin) Init(ctx context.Context, host HostAPI) error {
	return nil
}

func (m *mockTagPlugin) Call(ctx context.Context, fn string, args json.RawMessage) (json.RawMessage, error) {
	switch fn {
	case "hello":
		return json.Marshal(map[string]string{"message": "Hello from mock!"})
	case "render_widget":
		return json.Marshal(map[string]string{"html": "<div>Widget HTML</div>"})
	default:
		return nil, fmt.Errorf("unknown function: %s", fn)
	}
}

func (m *mockTagPlugin) Shutdown(ctx context.Context) error {
	return nil
}

func TestPluginCallerCall(t *testing.T) {
	ctx := context.Background()
	host := &mockHostAPIForTag{}
	mgr := NewManager(host)

	mockPlugin := &mockTagPlugin{}
	mgr.Register(ctx, mockPlugin)

	caller := &PluginCaller{
		Manager:    mgr,
		PluginName: "tagmock",
		Ctx:        ctx,
	}

	t.Run("successful call", func(t *testing.T) {
		result := caller.Call("hello", map[string]string{"name": "Test"})

		// Should return a parsed result
		if result == nil {
			t.Error("expected result")
		}

		// Check it's a map
		if m, ok := result.(map[string]interface{}); ok {
			if _, hasMsg := m["message"]; !hasMsg {
				t.Error("expected message in result")
			}
		}
	})

	t.Run("call without args", func(t *testing.T) {
		result := caller.Call("hello")
		if result == nil {
			t.Error("expected result")
		}
	})

	t.Run("call unknown function", func(t *testing.T) {
		result := caller.Call("nonexistent")

		// Should return error map
		if m, ok := result.(map[string]string); ok {
			if _, hasErr := m["error"]; !hasErr {
				t.Error("expected error in result")
			}
		}
	})

	t.Run("call with nil manager", func(t *testing.T) {
		nilCaller := &PluginCaller{
			Manager:    nil,
			PluginName: "tagmock",
		}
		result := nilCaller.Call("hello")

		if m, ok := result.(map[string]string); ok {
			if m["error"] != "plugin manager not available" {
				t.Errorf("expected 'plugin manager not available', got %v", m["error"])
			}
		} else {
			t.Error("expected error map")
		}
	})

	t.Run("call with nil context uses background", func(t *testing.T) {
		callerNoCtx := &PluginCaller{
			Manager:    mgr,
			PluginName: "tagmock",
			Ctx:        nil,
		}
		result := callerNoCtx.Call("hello")
		if result == nil {
			t.Error("expected result even without context")
		}
	})
}

func TestPluginCallerWidget(t *testing.T) {
	ctx := context.Background()
	host := &mockHostAPIForTag{}
	mgr := NewManager(host)

	mockPlugin := &mockTagPlugin{}
	mgr.Register(ctx, mockPlugin)

	caller := &PluginCaller{
		Manager:    mgr,
		PluginName: "tagmock",
		Ctx:        ctx,
	}

	t.Run("render widget", func(t *testing.T) {
		result := caller.Widget("test-widget")
		if result != "<div>Widget HTML</div>" {
			t.Errorf("expected widget HTML, got: %s", result)
		}
	})

	t.Run("widget not found", func(t *testing.T) {
		result := caller.Widget("nonexistent")
		if result == "" {
			t.Error("expected comment for missing widget")
		}
		// Should contain "not found"
		if !containsSubstr(result, "not found") {
			t.Errorf("expected 'not found' in result, got: %s", result)
		}
	})

	t.Run("nil manager", func(t *testing.T) {
		nilCaller := &PluginCaller{
			Manager:    nil,
			PluginName: "tagmock",
		}
		result := nilCaller.Widget("any")
		if !containsSubstr(result, "not available") {
			t.Errorf("expected 'not available', got: %s", result)
		}
	})

	t.Run("plugin not found", func(t *testing.T) {
		wrongCaller := &PluginCaller{
			Manager:    mgr,
			PluginName: "nonexistent",
			Ctx:        ctx,
		}
		result := wrongCaller.Widget("any")
		if !containsSubstr(result, "not found") {
			t.Errorf("expected 'not found', got: %s", result)
		}
	})
}

func TestPluginCallerTranslate(t *testing.T) {
	mgr := NewManager(nil)
	caller := &PluginCaller{
		Manager:    mgr,
		PluginName: "tagmock",
	}

	t.Run("returns key as fallback", func(t *testing.T) {
		result := caller.Translate("some.key")
		if result != "some.key" {
			t.Errorf("expected 'some.key', got %s", result)
		}
	})

	t.Run("nil manager returns key", func(t *testing.T) {
		nilCaller := &PluginCaller{
			Manager:    nil,
			PluginName: "tagmock",
		}
		result := nilCaller.Translate("another.key")
		if result != "another.key" {
			t.Errorf("expected 'another.key', got %s", result)
		}
	})
}

func TestUseTag(t *testing.T) {
	// Testing the use tag requires pongo2 template parsing
	// which is more of an integration test
	tag := &useTag{pluginName: "test-plugin"}
	if tag.pluginName != "test-plugin" {
		t.Error("plugin name not set")
	}
}

func TestTagUseParserIntegration(t *testing.T) {
	ctx := context.Background()
	host := &mockHostAPIForTag{}
	mgr := NewManager(host)

	mockPlugin := &mockTagPlugin{}
	mgr.Register(ctx, mockPlugin)
	SetTemplatePluginManager(mgr)
	defer SetTemplatePluginManager(nil)

	t.Run("valid use tag", func(t *testing.T) {
		// Call returns a map, access the message field
		tpl, err := pongo2.FromString(`{% use "tagmock" %}{% with result=tagmock.Call("hello") %}{{ result.message }}{% endwith %}`)
		if err != nil {
			t.Fatalf("template parse error: %v", err)
		}

		out, err := tpl.Execute(pongo2.Context{})
		if err != nil {
			t.Fatalf("execute error: %v", err)
		}

		if !containsSubstr(out, "Hello from mock") {
			t.Errorf("expected hello message, got: %s", out)
		}
	})

	t.Run("use tag missing plugin name", func(t *testing.T) {
		_, err := pongo2.FromString(`{% use %}`)
		if err == nil {
			t.Error("expected parse error for missing plugin name")
		}
	})

	t.Run("use tag plugin not found", func(t *testing.T) {
		tpl, err := pongo2.FromString(`{% use "nonexistent" %}test`)
		if err != nil {
			t.Fatalf("template parse error: %v", err)
		}

		_, err = tpl.Execute(pongo2.Context{})
		if err == nil {
			t.Error("expected execute error for missing plugin")
		}
	})

	t.Run("use tag without manager", func(t *testing.T) {
		SetTemplatePluginManager(nil)

		tpl, err := pongo2.FromString(`{% use "tagmock" %}test`)
		if err != nil {
			t.Fatalf("template parse error: %v", err)
		}

		_, err = tpl.Execute(pongo2.Context{})
		if err == nil {
			t.Error("expected execute error without manager")
		}

		// Restore
		SetTemplatePluginManager(mgr)
	})

	t.Run("use tag with widget call", func(t *testing.T) {
		tpl, err := pongo2.FromString(`{% use "tagmock" %}{{ tagmock.Widget("test-widget") }}`)
		if err != nil {
			t.Fatalf("template parse error: %v", err)
		}

		out, err := tpl.Execute(pongo2.Context{})
		if err != nil {
			t.Fatalf("execute error: %v", err)
		}

		if !containsSubstr(out, "Widget HTML") {
			t.Errorf("expected widget HTML, got: %s", out)
		}
	})

	t.Run("plugin metadata accessible", func(t *testing.T) {
		tpl, err := pongo2.FromString(`{% use "tagmock" %}{{ tagmock_meta.version }}`)
		if err != nil {
			t.Fatalf("template parse error: %v", err)
		}

		out, err := tpl.Execute(pongo2.Context{})
		if err != nil {
			t.Fatalf("execute error: %v", err)
		}

		if out != "1.0.0" {
			t.Errorf("expected version 1.0.0, got: %s", out)
		}
	})
}

// Helper
func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// mockHostAPIForTag is a minimal mock for template testing
type mockHostAPIForTag struct{}

func (m *mockHostAPIForTag) DBQuery(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	return nil, nil
}
func (m *mockHostAPIForTag) DBExec(ctx context.Context, query string, args ...any) (int64, error) {
	return 0, nil
}
func (m *mockHostAPIForTag) CacheGet(ctx context.Context, key string) ([]byte, bool, error) {
	return nil, false, nil
}
func (m *mockHostAPIForTag) CacheSet(ctx context.Context, key string, value []byte, ttlSeconds int) error {
	return nil
}
func (m *mockHostAPIForTag) CacheDelete(ctx context.Context, key string) error { return nil }
func (m *mockHostAPIForTag) HTTPRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) (int, []byte, error) {
	return 200, nil, nil
}
func (m *mockHostAPIForTag) SendEmail(ctx context.Context, to, subject, body string, html bool) error {
	return nil
}
func (m *mockHostAPIForTag) Log(ctx context.Context, level, message string, fields map[string]any) {}
func (m *mockHostAPIForTag) ConfigGet(ctx context.Context, key string) (string, error) {
	return "", nil
}
func (m *mockHostAPIForTag) Translate(ctx context.Context, key string, args ...any) string {
	return key
}
func (m *mockHostAPIForTag) CallPlugin(ctx context.Context, pluginName, function string, args json.RawMessage) (json.RawMessage, error) {
	return nil, nil
}
