package pluginui

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
)

// mockCaller implements PluginCaller for testing.
type mockCaller struct {
	responses map[string]json.RawMessage
	calls     []mockCall
	enabled   map[string]bool
}

func (m *mockCaller) IsEnabled(name string) bool {
	return m.enabled[name]
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

// stubAuth stands in for SessionOrJWTAuth: it authenticates every request as
// the given principal.
func stubAuth(userID int, role string) UIAuth {
	return UIAuth{Authenticate: func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("user_role", role)
		c.Set("is_customer", role == "Customer")
		c.Next()
	}}
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
	err := registerOneUI(eng, ui, nil, caller, renderer, stubAuth(5, "Customer"), logger)
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
		if navItems[1]["badge_count"] != int64(5) {
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
			registerOneUI(eng, ui, nil, caller, renderer, stubAuth(2, "Agent"), logger)

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
	registerOneUI(eng, ui, nil, caller, renderer, stubAuth(2, "Agent"), logger)

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
	registerOneUI(eng, ui, nil, caller, renderer, stubAuth(2, "Agent"), logger)

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

// TestUIRoutesForwardIdentity ensures buildUIHandler forwards the authenticated
// user's identity (user_id, login, is_admin, role, org) into the plugin args on
// UI page calls, mirroring what the API buildPluginArgs does. Regression for the
// slice where UI routes received no session middleware and plugins could only
// guess who was calling.
func TestUIRoutesForwardIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	caller := &mockCaller{
		responses: map[string]json.RawMessage{
			"testplugin.ui_home": json.RawMessage(`{"html":"<h1>Home</h1>"}`),
		},
	}
	renderer := &mockRenderer{}
	logger := slog.Default()

	cfgJSON, _ := json.Marshal(UIConfig{
		Routes: []UIRouteConfig{{Path: "/", Handler: "ui_home"}},
		Auth:   &UIAuthConfig{Method: AuthSession},
	})
	cfgRaw := json.RawMessage(cfgJSON)
	ui := PluginUI{
		ID:         2,
		PluginName: "testplugin",
		UIID:       "app",
		FullID:     "testplugin_app",
		Name:       "Test App",
		UIType:     TypeAdminPage,
		Shell:      ShellStandard,
		Config:     &cfgRaw,
		Enabled:    true,
		ValidID:    1,
	}

	// sessionAuth stands in for SessionOrJWTAuth: it populates the gin context
	// keys that buildUIHandler reads and forwards to the plugin.
	sessionAuth := func(c *gin.Context) {
		c.Set("user_id", 42)
		c.Set("user_email", "coach@example.com")
		c.Set("user_login", "coach42")
		c.Set("isInAdminGroup", true)
		c.Set("user_role", "Agent")
		c.Set("org_id", 7)
		c.Next()
	}

	eng := gin.New()
	if err := registerOneUI(eng, ui, nil, caller, renderer, UIAuth{Authenticate: sessionAuth}, logger); err != nil {
		t.Fatalf("registerOneUI: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ui/testplugin_app/", nil)
	eng.ServeHTTP(w, req)

	if len(caller.calls) != 1 {
		t.Fatalf("expected 1 plugin call, got %d", len(caller.calls))
	}
	var args map[string]any
	if err := json.Unmarshal(caller.calls[0].Args, &args); err != nil {
		t.Fatalf("unmarshal args: %v", err)
	}
	if args["_user_id"] != float64(42) {
		t.Errorf("_user_id = %v, want 42", args["_user_id"])
	}
	if args["_user_login"] != "coach42" {
		t.Errorf("_user_login = %v, want coach42", args["_user_login"])
	}
	if args["_is_admin"] != true {
		t.Errorf("_is_admin = %v, want true", args["_is_admin"])
	}
	if args["_user_role"] != "Agent" {
		t.Errorf("_user_role = %v, want Agent", args["_user_role"])
	}
	if args["_org_id"] != float64(7) {
		t.Errorf("_org_id = %v, want 7", args["_org_id"])
	}
}

// fakeUIInfo is a UIInfoLookup for nav-gating tests.
type fakeUIInfo struct {
	byFullID map[string]*PluginUI
}

func (f *fakeUIInfo) GetByFullID(fullID string) (*PluginUI, error) {
	u, ok := f.byFullID[fullID]
	if !ok {
		return nil, fmt.Errorf("not found: %s", fullID)
	}
	return u, nil
}

func TestBuildNavItemsCrossPluginUI(t *testing.T) {
	kanban := &PluginUI{PluginName: "goat-kanban", UIID: "board", FullID: "goat-kanban_board", Enabled: true, ValidID: 1}
	kanbanOff := &PluginUI{PluginName: "goat-kanban", UIID: "board", FullID: "goat-kanban_board", Enabled: false, ValidID: 1}

	repo := &fakeUIInfo{byFullID: map[string]*PluginUI{
		"goat-kanban_board": kanban,
	}}

	caller := &mockCaller{enabled: map[string]bool{"goat-kanban": true}}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request, _ = http.NewRequest("GET", "/ui/coach_main/", nil)

	ui := PluginUI{PluginName: "goatcoach", UIID: "main", FullID: "goatcoach_main"}

	t.Run("relative items keep ui-scoped href", func(t *testing.T) {
		cfg := &UIConfig{Nav: &UINavConfig{Position: "side", Items: []UINavItemConfig{
			{Label: "Dashboard", Icon: "fa-house", Path: "/dashboard"},
		}}}
		items := buildNavItems(c, ui, cfg, caller, "/dashboard", repo)
		if len(items) != 1 || items[0]["href"] != "/ui/goatcoach_main/dashboard" {
			t.Fatalf("items = %v", items)
		}
		if items[0]["active"] != true {
			t.Error("expected active")
		}
	})

	t.Run("absolute item shown when target ui+plugin enabled", func(t *testing.T) {
		cfg := &UIConfig{Nav: &UINavConfig{Position: "side", Items: []UINavItemConfig{
			{Label: "Boards", Icon: "fa-table-cells-large", Path: "/ui/goat-kanban_board/"},
		}}}
		items := buildNavItems(c, ui, cfg, caller, "/", repo)
		if len(items) != 1 || items[0]["href"] != "/ui/goat-kanban_board/" {
			t.Fatalf("items = %v", items)
		}
	})

	t.Run("absolute item hidden when target ui disabled", func(t *testing.T) {
		repo.byFullID["goat-kanban_board"] = kanbanOff
		defer func() { repo.byFullID["goat-kanban_board"] = kanban }()
		cfg := &UIConfig{Nav: &UINavConfig{Items: []UINavItemConfig{
			{Label: "Boards", Path: "/ui/goat-kanban_board/"},
		}}}
		if items := buildNavItems(c, ui, cfg, caller, "/", repo); len(items) != 0 {
			t.Fatalf("expected no items, got %v", items)
		}
	})

	t.Run("absolute item hidden when target plugin disabled", func(t *testing.T) {
		disabled := &mockCaller{enabled: map[string]bool{"goat-kanban": false}}
		cfg := &UIConfig{Nav: &UINavConfig{Items: []UINavItemConfig{
			{Label: "Boards", Path: "/ui/goat-kanban_board/"},
		}}}
		if items := buildNavItems(c, ui, cfg, disabled, "/", repo); len(items) != 0 {
			t.Fatalf("expected no items, got %v", items)
		}
	})

	t.Run("absolute item hidden for unknown ui", func(t *testing.T) {
		empty := &fakeUIInfo{byFullID: map[string]*PluginUI{}}
		cfg := &UIConfig{Nav: &UINavConfig{Items: []UINavItemConfig{
			{Label: "Boards", Path: "/ui/goat-kanban_board/"},
		}}}
		if items := buildNavItems(c, ui, cfg, caller, "/", empty); len(items) != 0 {
			t.Fatalf("expected no items, got %v", items)
		}
	})
}

// headerAuth authenticates from the X-Test-Principal header ("agent:<id>" or
// "customer:<id>") and answers 401 without it, like SessionOrJWTAuth. Its
// RequireGroup admits callers whose X-Test-Groups header lists the group.
func headerAuth() UIAuth {
	return UIAuth{
		Authenticate: func(c *gin.Context) {
			kind, idText, _ := strings.Cut(c.GetHeader("X-Test-Principal"), ":")
			id, err := strconv.Atoi(idText)
			if err != nil || (kind != "agent" && kind != "customer") {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
				return
			}
			c.Set("user_id", id)
			if kind == "customer" {
				c.Set("user_role", "Customer")
				c.Set("is_customer", true)
			} else {
				c.Set("user_role", "Agent")
				c.Set("is_customer", false)
			}
			c.Next()
		},
		RequireGroup: func(name string) gin.HandlerFunc {
			return func(c *gin.Context) {
				for _, g := range strings.Split(c.GetHeader("X-Test-Groups"), ",") {
					if g == name {
						c.Next()
						return
					}
				}
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "access denied: requires group " + name})
			}
		},
	}
}

func gatedUI(uiType string, auth *UIAuthConfig) PluginUI {
	cfgJSON, _ := json.Marshal(UIConfig{Routes: []UIRouteConfig{{Path: "/", Handler: "home"}}, Auth: auth})
	cfgRaw := json.RawMessage(cfgJSON)
	return PluginUI{
		PluginName: "gate", UIID: "ui", FullID: "gate_ui", Name: "Gate",
		UIType: uiType, Shell: ShellNone, Config: &cfgRaw, Enabled: true, ValidID: 1,
	}
}

// TestUIAuthGates: every UI auth method either gates the routes or the UI is
// not registered. token and pin used to add no middleware at all, auth.groups
// were ignored, and agent/admin UIs admitted customers (whose customer_user id
// can equal an agent's users id).
func TestUIAuthGates(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type req struct {
		principal, groups string
		want              int
	}
	tests := []struct {
		name   string
		uiType string
		auth   *UIAuthConfig
		reqs   []req
	}{
		{"agent_app default session", TypeAgentApp, nil, []req{
			{"", "", http.StatusUnauthorized},
			{"customer:2", "", http.StatusForbidden},
			{"agent:2", "", http.StatusOK},
		}},
		{"admin_page refuses customers", TypeAdminPage, nil, []req{
			{"customer:1", "", http.StatusForbidden},
			{"agent:1", "", http.StatusOK},
		}},
		{"customer_app refuses agents", TypeCustomerApp, nil, []req{
			{"", "", http.StatusUnauthorized},
			{"agent:3", "", http.StatusForbidden},
			{"customer:3", "", http.StatusOK},
		}},
		{"token method authenticates", TypeAgentApp, &UIAuthConfig{Method: AuthToken}, []req{
			{"", "", http.StatusUnauthorized},
			{"customer:2", "", http.StatusForbidden},
			{"agent:2", "", http.StatusOK},
		}},
		{"auth groups enforced", TypeAgentApp, &UIAuthConfig{Method: AuthSession, Groups: []string{"coach"}}, []req{
			{"agent:2", "users", http.StatusForbidden},
			{"agent:2", "users,coach", http.StatusOK},
		}},
		{"public_page stays public", TypePublicPage, nil, []req{
			{"", "", http.StatusOK},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eng := gin.New()
			if err := registerOneUI(eng, gatedUI(tt.uiType, tt.auth), nil, &mockCaller{}, &mockRenderer{}, headerAuth(), slog.Default()); err != nil {
				t.Fatalf("registerOneUI: %v", err)
			}
			for _, r := range tt.reqs {
				w := httptest.NewRecorder()
				hr, _ := http.NewRequest("GET", "/ui/gate_ui/", nil)
				if r.principal != "" {
					hr.Header.Set("X-Test-Principal", r.principal)
				}
				hr.Header.Set("X-Test-Groups", r.groups)
				eng.ServeHTTP(w, hr)
				if w.Code != r.want {
					t.Errorf("principal %q groups %q: status %d, want %d (body %s)", r.principal, r.groups, w.Code, r.want, w.Body.String())
				}
				if r.want == http.StatusOK && w.Body.String() != "<p>Hello from plugin</p>" {
					t.Errorf("principal %q: body %q, want plugin HTML", r.principal, w.Body.String())
				}
			}
		})
	}
}

// TestUIAuthUnenforceableConfigNotRegistered: an auth setting nothing can
// enforce leaves the UI unregistered (404) instead of serving it openly.
func TestUIAuthUnenforceableConfigNotRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name   string
		uiType string
		auth   *UIAuthConfig
		uiAuth UIAuth
	}{
		{"pin (no platform PIN flow)", TypeKiosk, &UIAuthConfig{Method: "pin"}, headerAuth()},
		{"unknown method", TypeAgentApp, &UIAuthConfig{Method: "sesion"}, headerAuth()},
		{"session without authenticator", TypeAgentApp, nil, UIAuth{}},
		{"groups on a public UI", TypePublicPage, &UIAuthConfig{Groups: []string{"coach"}}, headerAuth()},
		{"groups on a customer UI", TypeCustomerApp, &UIAuthConfig{Groups: []string{"coach"}}, headerAuth()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eng := gin.New()
			caller := &mockCaller{}
			if err := registerOneUI(eng, gatedUI(tt.uiType, tt.auth), nil, caller, &mockRenderer{}, tt.uiAuth, slog.Default()); err == nil {
				t.Error("registerOneUI succeeded, want an error")
			}
			w := httptest.NewRecorder()
			hr, _ := http.NewRequest("GET", "/ui/gate_ui/", nil)
			eng.ServeHTTP(w, hr)
			if w.Code != http.StatusNotFound {
				t.Errorf("status %d, want 404", w.Code)
			}
			if len(caller.calls) != 0 {
				t.Errorf("plugin was called %d times", len(caller.calls))
			}
		})
	}
}

// TestUIArgsAdminFlagNotSpoofable: _is_admin is always sent, so a plugin that
// overlays the host's identity keys on the client form body (goatcoach) never
// keeps a client-supplied "_is_admin": true for a non-admin agent.
func TestUIArgsAdminFlagNotSpoofable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfgJSON, _ := json.Marshal(UIConfig{Routes: []UIRouteConfig{{Path: "/save", Method: "POST", Handler: "save"}}})
	cfgRaw := json.RawMessage(cfgJSON)
	ui := PluginUI{PluginName: "gate", UIID: "ui", FullID: "gate_ui", Name: "Gate", UIType: TypeAgentApp, Shell: ShellNone, Config: &cfgRaw, Enabled: true, ValidID: 1}
	caller := &mockCaller{}
	eng := gin.New()
	if err := registerOneUI(eng, ui, nil, caller, &mockRenderer{}, headerAuth(), slog.Default()); err != nil {
		t.Fatalf("registerOneUI: %v", err)
	}
	w := httptest.NewRecorder()
	hr, _ := http.NewRequest("POST", "/ui/gate_ui/save", strings.NewReader(`{"_is_admin":true}`))
	hr.Header.Set("X-Test-Principal", "agent:9")
	eng.ServeHTTP(w, hr)
	if len(caller.calls) != 1 {
		t.Fatalf("expected 1 plugin call, got %d (status %d)", len(caller.calls), w.Code)
	}
	var args map[string]any
	if err := json.Unmarshal(caller.calls[0].Args, &args); err != nil {
		t.Fatalf("unmarshal args: %v", err)
	}
	if v, ok := args["_is_admin"]; !ok || v != false {
		t.Errorf("_is_admin = %v (present %v), want false", v, ok)
	}
	if args["_user_id"] != float64(9) {
		t.Errorf("_user_id = %v, want 9", args["_user_id"])
	}
}
