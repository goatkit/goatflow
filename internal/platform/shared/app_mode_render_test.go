package shared

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// installAppModeFixtures wires menu, hidden-item and app-mode providers for
// the duration of a test: two plugins with agent menu entries, a global
// hidden item, and an app mode for user 7 only.
func installAppModeFixtures(t *testing.T) {
	t.Helper()
	prevMenu, prevHidden, prevApp := globalPluginMenuProvider, globalHiddenMenuProvider, globalAppModeProvider
	t.Cleanup(func() {
		globalPluginMenuProvider, globalHiddenMenuProvider, globalAppModeProvider = prevMenu, prevHidden, prevApp
	})
	SetPluginMenuProvider(func(location string) []map[string]any {
		if location != "agent" {
			return nil
		}
		return []map[string]any{
			{"PluginName": "coach", "ID": "coach_home", "Label": "Coach home", "Path": "/ui/coach_app/"},
			{"PluginName": "kb", "ID": "kb_home", "Label": "Knowledge base", "Path": "/ui/kb_app/"},
		}
	})
	SetHiddenMenuProvider(func() map[string]bool { return map[string]bool{"phone_ticket": true} })
	SetAppModeProvider(func(userID uint) *AppModeView {
		if userID != 7 {
			return nil
		}
		return &AppModeView{
			PluginName:      "coach",
			LandingPage:     "/ui/coach_app/",
			HiddenNavItems:  map[string]bool{"dashboard": true, "tickets": true},
			HidePluginMenus: true,
			AppName:         "GoatCoach",
			Logo:            "data:image/svg+xml;base64,PHN2Zy8+",
			Favicon:         "data:image/svg+xml;base64,PHN2Zy8+",
			Color:           "#123456",
		}
	})
}

func renderBaseAs(t *testing.T, userID uint, role string, admin bool) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	renderer, err := NewTemplateRenderer(filepath.Join("..", "..", "..", "templates"))
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/profile", nil)
	c.Set("user_id", userID)
	c.Set("user_role", role)
	c.Set("user_name", "Test Agent")
	c.Set("isInAdminGroup", admin)
	renderer.HTML(c, http.StatusOK, "layouts/base.pongo2", gin.H{"ActivePage": "profile"})
	if w.Code != http.StatusOK {
		t.Fatalf("render status %d: %s", w.Code, w.Body.String())
	}
	return w.Body.String()
}

func TestBaseLayoutInAppModeHidesHelpdeskNavAndBrandsTheShell(t *testing.T) {
	installAppModeFixtures(t)
	body := renderBaseAs(t, 7, "Agent", false)

	for _, absent := range []string{`href="/tickets"`, `href="/dashboard"`, `href="/tickets/new?type=phone"`, `/ui/kb_app/`, "goatflow-logo.svg", "/static/favicon.svg"} {
		if strings.Contains(body, absent) {
			t.Errorf("app-mode shell still contains %q", absent)
		}
	}
	for _, present := range []string{
		`<title>GoatCoach</title>`,
		`href="/ui/coach_app/" class="gk-logo-glow`,
		`alt="GoatCoach"`,
		`<link rel="icon" href="data:image/svg+xml;base64,PHN2Zy8+">`,
		`--gk-ui-brand: #123456`,
		`href="/queues"`,                           // not hidden by this app
		`href="/profile"`,                          // the user menu stays
		`href="/logout"`,                           // and so does logout
		`href="/ui/coach_app/" class="gk-nav-link`, // the app's own menu entry survives HidePluginMenus
	} {
		if !strings.Contains(body, present) {
			t.Errorf("app-mode shell lacks %q", present)
		}
	}
	if strings.Contains(body, `href="/admin"`) {
		t.Error("a non-admin agent got the Administration link")
	}
}

func TestBaseLayoutGlobalHiddenItemsStillApplyOutsideAppMode(t *testing.T) {
	installAppModeFixtures(t)
	body := renderBaseAs(t, 8, "Agent", false)

	if strings.Contains(body, `href="/tickets/new?type=phone"`) {
		t.Error("globally hidden phone_ticket item rendered for a plain agent")
	}
	for _, present := range []string{`href="/tickets"`, `href="/dashboard"`, `/ui/kb_app/`, `/ui/coach_app/`, "goatflow-logo.svg", "/static/favicon.svg"} {
		if !strings.Contains(body, present) {
			t.Errorf("plain agent shell lacks %q", present)
		}
	}
	if strings.Contains(body, `<title>GoatCoach</title>`) || strings.Contains(body, `alt="GoatCoach"`) {
		t.Error("plain agent shell carries the app branding")
	}
}

func TestBaseLayoutAppModeWithoutLogoShowsIconWordmark(t *testing.T) {
	installAppModeFixtures(t)
	prev := globalAppModeProvider
	t.Cleanup(func() { globalAppModeProvider = prev })
	SetAppModeProvider(func(uint) *AppModeView {
		return &AppModeView{PluginName: "coach", LandingPage: "/ui/coach_app/", AppName: "GoatCoach", Icon: "fa-bullseye"}
	})
	body := renderBaseAs(t, 7, "Agent", false)

	want := `<i class="fa-solid fa-bullseye mr-2"></i>GoatCoach</span>`
	if !strings.Contains(body, want) {
		t.Fatalf("wordmark missing: want %q", want)
	}
	if strings.Contains(body, "goatflow-logo.svg") || strings.Contains(body, `alt="GoatCoach"`) {
		t.Error("logo rendered although the app set none")
	}
	if !strings.Contains(body, "/static/favicon.svg") {
		t.Error("platform favicon missing when the app set none")
	}
}

func TestBaseLayoutAdministratorsAreNeverInAppMode(t *testing.T) {
	installAppModeFixtures(t)
	// User 7 has an app mode, but the admin flag exempts them before the
	// provider is even asked.
	body := renderBaseAs(t, 7, "Admin", true)

	for _, present := range []string{`href="/tickets"`, `href="/dashboard"`, `href="/admin"`, `/ui/kb_app/`, "goatflow-logo.svg"} {
		if !strings.Contains(body, present) {
			t.Errorf("admin shell lacks %q", present)
		}
	}
	if strings.Contains(body, `<title>GoatCoach</title>`) || strings.Contains(body, `alt="GoatCoach"`) {
		t.Error("admin shell carries the app branding")
	}
}
