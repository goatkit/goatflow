package pluginui

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
)

// mockCaller implements PluginCaller for testing.
type mockCaller struct {
	responses map[string]json.RawMessage
	calls     []mockCall
}

type mockCall struct {
	PluginName string
	Fn         string
	Args       json.RawMessage
}

func (m *mockCaller) Call(_ context.Context, pluginName, fn string, args []byte) ([]byte, error) {
	m.calls = append(m.calls, mockCall{PluginName: pluginName, Fn: fn, Args: args})
	key := pluginName + "." + fn
	if resp, ok := m.responses[key]; ok {
		return resp, nil
	}
	return json.RawMessage(`{"html":"<p>Hello from plugin</p>"}`), nil
}

// mockRenderer implements TemplateRenderer for testing.
type mockRenderer struct {
	lastTemplate string
	lastData     map[string]any
}

func (m *mockRenderer) HTML(c *gin.Context, code int, name string, data interface{}) {
	m.lastTemplate = name
	m.lastData = make(map[string]any)
	switch values := data.(type) {
	case pongo2.Context:
		for k, v := range values {
			m.lastData[k] = v
		}
	case map[string]any:
		for k, v := range values {
			m.lastData[k] = v
		}
	}
	if html, ok := m.lastData["PluginHTML"].(string); ok {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(code, html)
		return
	}
	c.String(code, "rendered: "+name)
}

func TestRegisterUIRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	caller := &mockCaller{
		responses: map[string]json.RawMessage{
			"testplugin.ui_home":     json.RawMessage(`{"html":"<h1>Home</h1>","title":"Home"}`),
			"testplugin.ui_items":    json.RawMessage(`{"html":"<h1>Items</h1>"}`),
			"testplugin.ui_detail":   json.RawMessage(`{"html":"<h1>Detail</h1>"}`),
			"testplugin.badge_count": json.RawMessage(`{"count":5}`),
		},
	}
	renderer := &mockRenderer{}
	logger := slog.Default()

	cfg := UIConfig{
		Routes: []UIRouteConfig{
			{Path: "/", Handler: "ui_home"},
			{Path: "/items", Handler: "ui_items"},
			{Path: "/items/:id", Handler: "ui_detail", Method: "GET"},
		},
		Nav: &UINavConfig{
			Position: "bottom",
			Items: []UINavItemConfig{
				{Label: "Home", Icon: "fa-house", Path: "/"},
				{Label: "Items", Icon: "fa-box", Path: "/items", Badge: "badge_count"},
			},
		},
		Branding: &UIBrandingConfig{
			AppName: "Test App",
			Color:   "#ff0000",
		},
		PWA: &UIPWAConfig{
			Enabled: true,
			Display: "standalone",
		},
	}
	cfgJSON, _ := json.Marshal(cfg)
	cfgRaw := json.RawMessage(cfgJSON)

	ui := PluginUI{
		ID:         1,
		PluginName: "testplugin",
		UIID:       "app",
		FullID:     "testplugin_app",
		Name:       "Test App",
		UIType:     TypeCustomerApp,
		Shell:      ShellMinimal,
		Config:     &cfgRaw,
		Enabled:    true,
		ValidID:    1,
	}

	eng := gin.New()
	err := registerOneUI(eng, ui, caller, renderer, logger)
	if err != nil {
		t.Fatalf("registerOneUI: %v", err)
	}

	t.Run("home route", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ui/testplugin_app/", nil)
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}
		if w.Body.String() != "<h1>Home</h1>" {
			t.Errorf("body = %q", w.Body.String())
		}
		if renderer.lastTemplate != "layouts/ui_minimal.pongo2" {
			t.Errorf("template = %q, want ui_minimal", renderer.lastTemplate)
		}
	})

	t.Run("items route", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ui/testplugin_app/items", nil)
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d", w.Code)
		}
	})

	t.Run("detail route with param", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ui/testplugin_app/items/42", nil)
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d", w.Code)
		}

		// Verify plugin received the params.
		found := false
		for _, call := range caller.calls {
			if call.Fn == "ui_detail" {
				var args map[string]any
				json.Unmarshal(call.Args, &args)
				params, _ := args["params"].(map[string]any)
				if params["id"] == "42" {
					found = true
				}
			}
		}
		if !found {
			t.Error("plugin did not receive id=42 param")
		}
	})

	t.Run("PWA manifest", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ui/testplugin_app/manifest.json", nil)
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d", w.Code)
		}

		var manifest map[string]any
		json.Unmarshal(w.Body.Bytes(), &manifest)
		if manifest["name"] != "Test App" {
			t.Errorf("name = %v", manifest["name"])
		}
		if manifest["display"] != "standalone" {
			t.Errorf("display = %v", manifest["display"])
		}
		if manifest["theme_color"] != "#ff0000" {
			t.Errorf("theme_color = %v", manifest["theme_color"])
		}
		if manifest["start_url"] != "/ui/testplugin_app/" {
			t.Errorf("start_url = %v", manifest["start_url"])
		}
	})

	t.Run("non-existent route returns 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ui/testplugin_app/nonexistent", nil)
		eng.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", w.Code)
		}
	})

	t.Run("nav items include badge counts", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ui/testplugin_app/", nil)
		eng.ServeHTTP(w, req)

		// Check renderer received nav items.
		if renderer.lastData == nil {
			t.Fatal("no template data")
		}
		navItems, ok := renderer.lastData["ui_nav_items"].([]map[string]any)
		if !ok {
			t.Fatal("no nav items in template data")
		}
		if len(navItems) != 2 {
			t.Fatalf("expected 2 nav items, got %d", len(navItems))
		}
		// First item (Home) should be active on /
		if navItems[0]["active"] != true {
			t.Error("Home should be active on /")
		}
		// Second item (Items) should have a badge count.
		if navItems[1]["badge_count"] != float64(5) {
			t.Errorf("badge_count = %v, want 5", navItems[1]["badge_count"])
		}
		// ui_nav must be a map whose "position" the shell templates can resolve
		// (pongo2 reads Go field names, not JSON tags, so the *UINavConfig struct
		// would not expose `.position`). Regression: standard/minimal shell nav
		// conditions were silently false before this.
		uiNav, ok := renderer.lastData["ui_nav"].(map[string]any)
		if !ok {
			t.Fatal("ui_nav is not a map in template data")
		}
		if uiNav["position"] != "bottom" {
			t.Errorf("ui_nav.position = %v, want bottom", uiNav["position"])
		}
	})
}

func TestRegisterUIRoutes_Shells(t *testing.T) {
	gin.SetMode(gin.TestMode)
	caller := &mockCaller{}
	logger := slog.Default()

	tests := []struct {
		shell   string
		wantTpl string
	}{
		{ShellStandard, "layouts/ui_standard.pongo2"},
		{ShellMinimal, "layouts/ui_minimal.pongo2"},
		{ShellNone, "layouts/ui_none.pongo2"},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			renderer := &mockRenderer{}
			cfg := UIConfig{Routes: []UIRouteConfig{{Path: "/", Handler: "home"}}}
			cfgJSON, _ := json.Marshal(cfg)
			cfgRaw := json.RawMessage(cfgJSON)

			ui := PluginUI{
				PluginName: "test", UIID: tt.shell, FullID: fmt.Sprintf("test_%s", tt.shell),
				Name: "Test", UIType: TypeAgentApp, Shell: tt.shell,
				Config: &cfgRaw, Enabled: true, ValidID: 1,
			}

			eng := gin.New()
			registerOneUI(eng, ui, caller, renderer, logger)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", fmt.Sprintf("/ui/test_%s/", tt.shell), nil)
			eng.ServeHTTP(w, req)

			if renderer.lastTemplate != tt.wantTpl {
				t.Errorf("shell %q used template %q, want %q", tt.shell, renderer.lastTemplate, tt.wantTpl)
			}
		})
	}
}

func TestRegisterUIRoutes_JSONResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	caller := &mockCaller{
		responses: map[string]json.RawMessage{
			"test.api_handler": json.RawMessage(`{"status":"ok","data":[1,2,3]}`),
		},
	}
	renderer := &mockRenderer{}
	logger := slog.Default()

	cfg := UIConfig{Routes: []UIRouteConfig{{Path: "/api/data", Handler: "api_handler"}}}
	cfgJSON, _ := json.Marshal(cfg)
	cfgRaw := json.RawMessage(cfgJSON)

	ui := PluginUI{
		PluginName: "test", UIID: "api", FullID: "test_api",
		Name: "API", UIType: TypeAgentApp, Shell: ShellStandard,
		Config: &cfgRaw, Enabled: true, ValidID: 1,
	}

	eng := gin.New()
	registerOneUI(eng, ui, caller, renderer, logger)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ui/test_api/api/data", nil)
	eng.ServeHTTP(w, req)

	// No "html" key in response — should return raw JSON.
	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("response = %v", resp)
	}
}

func TestRegisterUIRoutes_POSTRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	caller := &mockCaller{
		responses: map[string]json.RawMessage{
			"test.handle_submit": json.RawMessage(`{"html":"<p>Submitted</p>"}`),
		},
	}
	renderer := &mockRenderer{}
	logger := slog.Default()

	cfg := UIConfig{Routes: []UIRouteConfig{{Path: "/submit", Method: "POST", Handler: "handle_submit"}}}
	cfgJSON, _ := json.Marshal(cfg)
	cfgRaw := json.RawMessage(cfgJSON)

	ui := PluginUI{
		PluginName: "test", UIID: "form", FullID: "test_form",
		Name: "Form", UIType: TypeAgentApp, Shell: ShellMinimal,
		Config: &cfgRaw, Enabled: true, ValidID: 1,
	}

	eng := gin.New()
	registerOneUI(eng, ui, caller, renderer, logger)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/ui/test_form/submit", nil)
	eng.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
}

func TestExtractParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	eng := gin.New()

	var captured map[string]string
	eng.GET("/test/:id/:action", func(c *gin.Context) {
		captured = extractParams(c)
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test/42/edit", nil)
	eng.ServeHTTP(w, req)

	if captured["id"] != "42" || captured["action"] != "edit" {
		t.Errorf("params = %v", captured)
	}
}
