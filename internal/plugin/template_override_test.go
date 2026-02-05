package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestTemplateOverrideRegistry(t *testing.T) {
	t.Run("Register and HasOverride", func(t *testing.T) {
		registry := NewTemplateOverrideRegistry(nil)
		
		templates := []TemplateSpec{
			{Name: "pages/admin/dashboard.pongo2", Path: "dashboard.pongo2", Override: true},
			{Name: "pages/agent/home.pongo2", Path: "home.pongo2", Override: true},
			{Name: "components/widget.pongo2", Path: "widget.pongo2", Override: false}, // Not an override
		}
		
		registry.Register("test-plugin", templates)
		
		// Check override exists
		if !registry.HasOverride("pages/admin/dashboard.pongo2") {
			t.Error("should have override for dashboard")
		}
		if !registry.HasOverride("pages/agent/home.pongo2") {
			t.Error("should have override for home")
		}
		
		// Non-override template shouldn't be registered
		if registry.HasOverride("components/widget.pongo2") {
			t.Error("should not have override for non-override template")
		}
		
		// Non-existent template
		if registry.HasOverride("pages/nonexistent.pongo2") {
			t.Error("should not have override for non-existent template")
		}
	})
	
	t.Run("GetOverride", func(t *testing.T) {
		registry := NewTemplateOverrideRegistry(nil)
		
		templates := []TemplateSpec{
			{Name: "test/template.pongo2", Path: "template.pongo2", Override: true},
		}
		registry.Register("my-plugin", templates)
		
		override := registry.GetOverride("test/template.pongo2")
		if override == nil {
			t.Fatal("override should not be nil")
		}
		if override.PluginName != "my-plugin" {
			t.Errorf("expected my-plugin, got %s", override.PluginName)
		}
		if override.TemplateName != "test/template.pongo2" {
			t.Errorf("expected test/template.pongo2, got %s", override.TemplateName)
		}
		
		// Non-existent
		if registry.GetOverride("nonexistent") != nil {
			t.Error("should return nil for non-existent")
		}
	})
	
	t.Run("Unregister", func(t *testing.T) {
		registry := NewTemplateOverrideRegistry(nil)
		
		registry.Register("plugin-a", []TemplateSpec{
			{Name: "a/template.pongo2", Override: true},
		})
		registry.Register("plugin-b", []TemplateSpec{
			{Name: "b/template.pongo2", Override: true},
		})
		
		// Both should exist
		if !registry.HasOverride("a/template.pongo2") {
			t.Error("should have a")
		}
		if !registry.HasOverride("b/template.pongo2") {
			t.Error("should have b")
		}
		
		// Unregister plugin-a
		registry.Unregister("plugin-a")
		
		// plugin-a's override should be gone
		if registry.HasOverride("a/template.pongo2") {
			t.Error("should not have a after unregister")
		}
		// plugin-b's should still exist
		if !registry.HasOverride("b/template.pongo2") {
			t.Error("should still have b")
		}
	})
	
	t.Run("List", func(t *testing.T) {
		registry := NewTemplateOverrideRegistry(nil)
		
		registry.Register("plugin-a", []TemplateSpec{
			{Name: "a/one.pongo2", Override: true},
			{Name: "a/two.pongo2", Override: true},
		})
		registry.Register("plugin-b", []TemplateSpec{
			{Name: "b/one.pongo2", Override: true},
		})
		
		overrides := registry.List()
		if len(overrides) != 3 {
			t.Errorf("expected 3 overrides, got %d", len(overrides))
		}
	})
	
	t.Run("RenderOverride without manager", func(t *testing.T) {
		registry := NewTemplateOverrideRegistry(nil) // No manager
		
		registry.Register("test-plugin", []TemplateSpec{
			{Name: "test.pongo2", Override: true},
		})
		
		// Should return false when no manager
		html, ok := registry.RenderOverride(context.Background(), "test.pongo2", nil)
		if ok {
			t.Error("should return false without manager")
		}
		if html != "" {
			t.Error("should return empty html")
		}
	})
}

func TestSanitizeTemplateName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"pages/admin/dashboard.pongo2", "pages_admin_dashboard_pongo2"},
		{"test-template.html", "test_template_html"},
		{"a/b/c/d.pongo2", "a_b_c_d_pongo2"},
	}
	
	for _, tc := range tests {
		result := sanitizeTemplateName(tc.input)
		if result != tc.expected {
			t.Errorf("sanitizeTemplateName(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestTemplateOverrideWithMockManager(t *testing.T) {
	// Create a mock manager that returns HTML
	mockPlugin := &mockTemplatePlugin{
		response: map[string]string{"html": "<h1>Override!</h1>"},
	}
	
	mgr := &Manager{
		plugins: map[string]*registeredPlugin{
			"test-plugin": {
				plugin:  mockPlugin,
				enabled: true,
			},
		},
	}
	
	registry := NewTemplateOverrideRegistry(mgr)
	registry.Register("test-plugin", []TemplateSpec{
		{Name: "test.pongo2", Override: true},
	})
	
	html, ok := registry.RenderOverride(context.Background(), "test.pongo2", nil)
	
	if !ok {
		t.Error("expected override to succeed")
	}
	if html != "<h1>Override!</h1>" {
		t.Errorf("expected override HTML, got %s", html)
	}
}

// mockTemplatePlugin implements Plugin for template override tests
type mockTemplatePlugin struct {
	response map[string]string
}

func (m *mockTemplatePlugin) GKRegister() GKRegistration {
	return GKRegistration{Name: "mock-template"}
}

func (m *mockTemplatePlugin) Init(ctx context.Context, host HostAPI) error {
	return nil
}

func (m *mockTemplatePlugin) Call(ctx context.Context, fn string, args json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(m.response)
}

func (m *mockTemplatePlugin) Shutdown(ctx context.Context) error {
	return nil
}

func TestGlobalTemplateOverrides(t *testing.T) {
	// Test the global getter/setter functions
	original := GetTemplateOverrides()
	defer SetTemplateOverrides(original)

	// Create new registry with nil manager (for simple test)
	mgr := NewManager(nil)
	newReg := NewTemplateOverrideRegistry(mgr)

	SetTemplateOverrides(newReg)

	got := GetTemplateOverrides()
	if got != newReg {
		t.Error("GetTemplateOverrides should return the set registry")
	}
}

// mockFailingTemplatePlugin returns error on Call
type mockFailingTemplatePlugin struct{}

func (m *mockFailingTemplatePlugin) GKRegister() GKRegistration {
	return GKRegistration{Name: "failing-template"}
}

func (m *mockFailingTemplatePlugin) Init(ctx context.Context, host HostAPI) error {
	return nil
}

func (m *mockFailingTemplatePlugin) Call(ctx context.Context, fn string, args json.RawMessage) (json.RawMessage, error) {
	return nil, fmt.Errorf("plugin call failed")
}

func (m *mockFailingTemplatePlugin) Shutdown(ctx context.Context) error {
	return nil
}

func TestRenderOverrideCallError(t *testing.T) {
	ctx := context.Background()
	host := NewDefaultHostAPI()
	mgr := NewManager(host)

	// Register the failing plugin
	failP := &mockFailingTemplatePlugin{}
	mgr.Register(ctx, failP)

	registry := NewTemplateOverrideRegistry(mgr)
	registry.Register("failing-template", []TemplateSpec{
		{Name: "fail.html", Override: true},
	})

	html, ok := registry.RenderOverride(ctx, "fail.html", nil)
	if ok {
		t.Error("expected RenderOverride to return false on error")
	}
	if html != "" {
		t.Error("expected empty HTML on error")
	}
}

// mockInvalidJSONPlugin returns invalid JSON
type mockInvalidJSONPlugin struct{}

func (m *mockInvalidJSONPlugin) GKRegister() GKRegistration {
	return GKRegistration{Name: "invalid-json"}
}

func (m *mockInvalidJSONPlugin) Init(ctx context.Context, host HostAPI) error {
	return nil
}

func (m *mockInvalidJSONPlugin) Call(ctx context.Context, fn string, args json.RawMessage) (json.RawMessage, error) {
	return []byte(`not valid json`), nil
}

func (m *mockInvalidJSONPlugin) Shutdown(ctx context.Context) error {
	return nil
}

func TestRenderOverrideInvalidJSON(t *testing.T) {
	ctx := context.Background()
	host := NewDefaultHostAPI()
	mgr := NewManager(host)

	// Register the invalid JSON plugin
	invP := &mockInvalidJSONPlugin{}
	mgr.Register(ctx, invP)

	registry := NewTemplateOverrideRegistry(mgr)
	registry.Register("invalid-json", []TemplateSpec{
		{Name: "invalid.html", Override: true},
	})

	html, ok := registry.RenderOverride(ctx, "invalid.html", nil)
	if ok {
		t.Error("expected RenderOverride to return false on invalid JSON")
	}
	if html != "" {
		t.Error("expected empty HTML on invalid JSON")
	}
}
