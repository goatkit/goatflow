package pluginui

import (
	"encoding/json"
	"testing"
)

func TestValidUITypes(t *testing.T) {
	types := ValidUITypes()
	if len(types) != 5 {
		t.Errorf("expected 5 UI types, got %d", len(types))
	}
	for _, tt := range types {
		if !IsValidUIType(tt) {
			t.Errorf("%q should be valid", tt)
		}
	}
	if IsValidUIType("bogus") {
		t.Error("bogus should be invalid")
	}
}

func TestValidShells(t *testing.T) {
	shells := ValidShells()
	if len(shells) != 3 {
		t.Errorf("expected 3 shells, got %d", len(shells))
	}
	for _, s := range shells {
		if !IsValidShell(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	if IsValidShell("bogus") {
		t.Error("bogus should be invalid")
	}
}

func TestDefaultShell(t *testing.T) {
	tests := []struct {
		uiType string
		want   string
	}{
		{TypeAdminPage, ShellStandard},
		{TypeAgentApp, ShellStandard},
		{TypeCustomerApp, ShellMinimal},
		{TypePublicPage, ShellMinimal},
		{TypeKiosk, ShellNone},
		{"unknown", ShellStandard},
	}
	for _, tt := range tests {
		t.Run(tt.uiType, func(t *testing.T) {
			if got := DefaultShell(tt.uiType); got != tt.want {
				t.Errorf("DefaultShell(%q) = %q, want %q", tt.uiType, got, tt.want)
			}
		})
	}
}

func TestDefaultAuthMethod(t *testing.T) {
	tests := []struct {
		uiType string
		want   string
	}{
		{TypeAdminPage, AuthSession},
		{TypeAgentApp, AuthSession},
		{TypeCustomerApp, AuthSession},
		{TypePublicPage, AuthNone},
		{TypeKiosk, AuthNone},
		{"unknown", AuthSession},
	}
	for _, tt := range tests {
		t.Run(tt.uiType, func(t *testing.T) {
			if got := DefaultAuthMethod(tt.uiType); got != tt.want {
				t.Errorf("DefaultAuthMethod(%q) = %q, want %q", tt.uiType, got, tt.want)
			}
		})
	}
}

func TestPluginUI_IsActive(t *testing.T) {
	tests := []struct {
		name    string
		validID int
		enabled bool
		want    bool
	}{
		{"valid and enabled", 1, true, true},
		{"valid but disabled", 1, false, false},
		{"invalid but enabled", 2, true, false},
		{"invalid and disabled", 2, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &PluginUI{ValidID: tt.validID, Enabled: tt.enabled}
			if got := u.IsActive(); got != tt.want {
				t.Errorf("IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPluginUI_BasePath(t *testing.T) {
	u := &PluginUI{FullID: "inventory_app"}
	if got := u.BasePath(); got != "/ui/inventory_app/" {
		t.Errorf("BasePath() = %q, want %q", got, "/ui/inventory_app/")
	}
}

func TestPluginUI_ParsedConfig(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		u := &PluginUI{}
		cfg, err := u.ParsedConfig()
		if err != nil {
			t.Fatal(err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil config")
		}
	})

	t.Run("valid config", func(t *testing.T) {
		raw := json.RawMessage(`{"routes":[{"path":"/","handler":"home"}],"data_scope":"org"}`)
		u := &PluginUI{Config: &raw}
		cfg, err := u.ParsedConfig()
		if err != nil {
			t.Fatal(err)
		}
		if len(cfg.Routes) != 1 {
			t.Errorf("expected 1 route, got %d", len(cfg.Routes))
		}
		if cfg.DataScope != "org" {
			t.Errorf("data_scope = %q", cfg.DataScope)
		}
	})

	t.Run("config with nav", func(t *testing.T) {
		raw := json.RawMessage(`{"nav":{"position":"bottom","items":[{"label":"Home","icon":"fa-house","path":"/"}]}}`)
		u := &PluginUI{Config: &raw}
		cfg, err := u.ParsedConfig()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Nav == nil {
			t.Fatal("expected nav")
		}
		if cfg.Nav.Position != "bottom" {
			t.Errorf("nav position = %q", cfg.Nav.Position)
		}
		if len(cfg.Nav.Items) != 1 {
			t.Errorf("expected 1 nav item, got %d", len(cfg.Nav.Items))
		}
	})

	t.Run("config with branding", func(t *testing.T) {
		raw := json.RawMessage(`{"branding":{"app_name":"My App","color":"#ff0000"}}`)
		u := &PluginUI{Config: &raw}
		cfg, err := u.ParsedConfig()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Branding == nil || cfg.Branding.AppName != "My App" {
			t.Error("expected branding with app_name 'My App'")
		}
		if cfg.Branding.Color != "#ff0000" {
			t.Errorf("color = %q", cfg.Branding.Color)
		}
	})

	t.Run("config with PWA", func(t *testing.T) {
		raw := json.RawMessage(`{"pwa":{"enabled":true,"display":"fullscreen"}}`)
		u := &PluginUI{Config: &raw}
		cfg, err := u.ParsedConfig()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.PWA == nil || !cfg.PWA.Enabled {
			t.Error("expected PWA enabled")
		}
		if cfg.PWA.Display != "fullscreen" {
			t.Errorf("display = %q", cfg.PWA.Display)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		raw := json.RawMessage(`{bad}`)
		u := &PluginUI{Config: &raw}
		_, err := u.ParsedConfig()
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestPluginUI_SetBranding(t *testing.T) {
	t.Run("merges override and preserves config", func(t *testing.T) {
		raw, err := UISpecToConfig(UIConfig{
			Routes: []UIRouteConfig{{Path: "/", Handler: "home"}},
			PWA:    &UIPWAConfig{Enabled: true, CacheRoutes: []string{"/"}},
		})
		if err != nil {
			t.Fatal(err)
		}

		u := &PluginUI{Config: raw}
		err = u.SetBranding(&UIBrandingConfig{
			AppName: "  Ops App  ",
			Color:   " #123456 ",
		})
		if err != nil {
			t.Fatal(err)
		}

		cfg, err := u.ParsedConfig()
		if err != nil {
			t.Fatal(err)
		}
		if len(cfg.Routes) != 1 || cfg.Routes[0].Handler != "home" {
			t.Fatalf("routes were not preserved: %+v", cfg.Routes)
		}
		if cfg.PWA == nil || !cfg.PWA.Enabled || len(cfg.PWA.CacheRoutes) != 1 {
			t.Fatalf("pwa config was not preserved: %+v", cfg.PWA)
		}
		if cfg.Branding == nil {
			t.Fatal("expected branding override")
		}
		if cfg.Branding.AppName != "Ops App" {
			t.Errorf("AppName = %q", cfg.Branding.AppName)
		}
		if cfg.Branding.Color != "#123456" {
			t.Errorf("Color = %q", cfg.Branding.Color)
		}
	})

	t.Run("clears empty override", func(t *testing.T) {
		raw, err := UISpecToConfig(UIConfig{
			Branding: &UIBrandingConfig{AppName: "Existing"},
		})
		if err != nil {
			t.Fatal(err)
		}

		u := &PluginUI{Config: raw}
		if err := u.SetBranding(&UIBrandingConfig{AppName: "   "}); err != nil {
			t.Fatal(err)
		}

		cfg, err := u.ParsedConfig()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Branding != nil {
			t.Fatalf("expected branding to be cleared, got %+v", cfg.Branding)
		}
	})
}

func TestBuildFullID(t *testing.T) {
	if got := BuildFullID("inventory", "app"); got != "inventory_app" {
		t.Errorf("got %q, want %q", got, "inventory_app")
	}
}

func TestShellTemplateName(t *testing.T) {
	tests := []struct {
		shell string
		want  string
	}{
		{ShellNone, "layouts/ui_none.pongo2"},
		{ShellMinimal, "layouts/ui_minimal.pongo2"},
		{ShellStandard, "layouts/ui_standard.pongo2"},
		{"unknown", "layouts/ui_standard.pongo2"},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			if got := shellTemplateName(tt.shell); got != tt.want {
				t.Errorf("shellTemplateName(%q) = %q, want %q", tt.shell, got, tt.want)
			}
		})
	}
}

func TestGenerateMenuItems(t *testing.T) {
	icon := "fa-box"
	uis := []PluginUI{
		{FullID: "inv_app", Name: "Inventory", UIType: TypeCustomerApp, Icon: &icon, ValidID: 1, Enabled: true},
		{FullID: "inv_admin", Name: "Inv Admin", UIType: TypeAdminPage, ValidID: 1, Enabled: true},
		{FullID: "inv_agent", Name: "Inv Agent", UIType: TypeAgentApp, ValidID: 1, Enabled: true},
		{FullID: "inv_public", Name: "Status", UIType: TypePublicPage, ValidID: 1, Enabled: true},
		{FullID: "inv_kiosk", Name: "Kiosk", UIType: TypeKiosk, ValidID: 1, Enabled: true},
		{FullID: "disabled", Name: "Disabled", UIType: TypeAgentApp, ValidID: 1, Enabled: false}, // disabled
	}

	items := GenerateMenuItems(uis)

	if len(items["customer"]) != 1 {
		t.Errorf("expected 1 customer item, got %d", len(items["customer"]))
	}
	if len(items["admin"]) != 1 {
		t.Errorf("expected 1 admin item, got %d", len(items["admin"]))
	}
	if len(items["agent"]) != 1 {
		t.Errorf("expected 1 agent item (disabled excluded), got %d", len(items["agent"]))
	}

	// Check customer item details.
	if items["customer"][0]["label"] != "Inventory" {
		t.Errorf("label = %v", items["customer"][0]["label"])
	}
	if items["customer"][0]["path"] != "/ui/inv_app/" {
		t.Errorf("path = %v", items["customer"][0]["path"])
	}
	if items["customer"][0]["icon"] != "fa-box" {
		t.Errorf("icon = %v", items["customer"][0]["icon"])
	}
}

func TestValidAuthMethods(t *testing.T) {
	methods := ValidAuthMethods()
	if len(methods) != 4 {
		t.Errorf("expected 4 auth methods, got %d", len(methods))
	}
}

func TestValidDataScopes(t *testing.T) {
	scopes := ValidDataScopes()
	if len(scopes) != 3 {
		t.Errorf("expected 3 data scopes, got %d", len(scopes))
	}
}

func TestValidNavPositions(t *testing.T) {
	positions := ValidNavPositions()
	if len(positions) != 3 {
		t.Errorf("expected 3 nav positions, got %d", len(positions))
	}
}

func TestUISpecToConfig(t *testing.T) {
	spec := map[string]any{
		"routes": []map[string]string{{"path": "/", "handler": "home"}},
	}
	result, err := UISpecToConfig(spec)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	var parsed map[string]any
	json.Unmarshal(*result, &parsed)
	routes, ok := parsed["routes"].([]any)
	if !ok || len(routes) != 1 {
		t.Error("expected 1 route in config")
	}
}
