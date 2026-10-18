package plugin

import (
	"strings"
	"testing"
)

func addAppModePlugin(m *Manager, name string, spec *AppModeSpec, enabled, switchOn bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plugins[name] = &registeredPlugin{
		plugin:   &fakePlugin{name: name},
		manifest: GKRegistration{Name: name, AppMode: spec},
		enabled:  enabled,
		appMode:  switchOn,
		health:   healthInitForRegister(),
	}
}

func TestAppModeForMatchesAudienceAndNeverHidesAdmin(t *testing.T) {
	m := newTestManager()
	addAppModePlugin(m, "coach", &AppModeSpec{
		Groups:          []string{"coach", "coach_admin"},
		LandingPage:     "/ui/coach_app/",
		HideMenuItems:   []string{"dashboard", "tickets", "admin"},
		HidePluginMenus: true,
		Branding:        &UIBrandingSpec{AppName: "Coach"},
	}, true, true)

	if !m.HasAppMode() {
		t.Fatal("HasAppMode() = false with an enabled plugin switched on")
	}
	if got := m.AppModeFor([]string{"users", "stats"}); got != nil {
		t.Fatalf("agent outside the groups got app mode %+v", got)
	}
	got := m.AppModeFor([]string{"users", "coach_admin"})
	if got == nil {
		t.Fatal("coach_admin member got no app mode")
	}
	if got.PluginName != "coach" || got.LandingPage != "/ui/coach_app/" || !got.HidePluginMenus || got.Branding.AppName != "Coach" {
		t.Fatalf("resolved app mode = %+v", got)
	}
	if strings.Join(got.HiddenMenuItems, ",") != "dashboard,tickets" {
		t.Fatalf("hidden items = %v, want admin dropped", got.HiddenMenuItems)
	}
}

func TestAppModeForEmptyGroupsMeansEveryAgent(t *testing.T) {
	m := newTestManager()
	addAppModePlugin(m, "kiosk", &AppModeSpec{LandingPage: "/kiosk"}, true, true)

	if got := m.AppModeFor(nil); got == nil || got.LandingPage != "/kiosk" {
		t.Fatalf("agent with no groups got %+v, want the kiosk app", got)
	}
}

func TestAppModeForSkipsDisabledPluginsAndSwitchedOffAppModes(t *testing.T) {
	m := newTestManager()
	addAppModePlugin(m, "disabled", &AppModeSpec{LandingPage: "/disabled"}, false, true)
	addAppModePlugin(m, "off", &AppModeSpec{LandingPage: "/off"}, true, false)
	addAppModePlugin(m, "plain", nil, true, true)

	if m.HasAppMode() {
		t.Fatal("HasAppMode() = true with nothing usable")
	}
	if got := m.AppModeFor([]string{"coach"}); got != nil {
		t.Fatalf("got app mode %+v from a disabled plugin or a switched-off app mode", got)
	}
}

func TestAppModeForPrefersTheFirstPluginByName(t *testing.T) {
	m := newTestManager()
	addAppModePlugin(m, "zeta", &AppModeSpec{Groups: []string{"coach"}, LandingPage: "/zeta"}, true, true)
	addAppModePlugin(m, "alpha", &AppModeSpec{Groups: []string{"coach"}, LandingPage: "/alpha"}, true, true)

	for i := 0; i < 20; i++ {
		if got := m.AppModeFor([]string{"coach"}); got == nil || got.PluginName != "alpha" {
			t.Fatalf("iteration %d: got %+v, want alpha", i, got)
		}
	}
}

func TestAppModeStatusAndSwitch(t *testing.T) {
	m := newTestManager()
	addAppModePlugin(m, "coach", &AppModeSpec{Groups: []string{"coach"}, HideMenuItems: []string{"tickets", "admin"}}, true, true)
	addAppModePlugin(m, "plain", nil, true, false)

	if st := m.AppModeStatus("plain"); st != nil {
		t.Fatalf("plugin without app mode has status %+v", st)
	}
	if st := m.AppModeStatus("missing"); st != nil {
		t.Fatalf("unknown plugin has status %+v", st)
	}
	st := m.AppModeStatus("coach")
	if st == nil || !st.Enabled || strings.Join(st.HideMenuItems, ",") != "tickets" || strings.Join(st.Groups, ",") != "coach" {
		t.Fatalf("status = %+v", st)
	}

	if err := m.SetAppModeEnabled("plain", true); err == nil {
		t.Fatal("switching app mode on a plugin that declares none succeeded")
	}
	if err := m.SetAppModeEnabled("missing", true); err == nil {
		t.Fatal("switching app mode on an unknown plugin succeeded")
	}
	if err := m.SetAppModeEnabled("coach", false); err != nil {
		t.Fatalf("switch off: %v", err)
	}
	if m.AppModeStatus("coach").Enabled || m.AppModeFor([]string{"coach"}) != nil || m.HasAppMode() {
		t.Fatal("app mode still active after switching it off")
	}
	if err := m.SetAppModeEnabled("coach", true); err != nil {
		t.Fatalf("switch on: %v", err)
	}
	if m.AppModeFor([]string{"coach"}) == nil {
		t.Fatal("app mode inactive after switching it back on")
	}
}
