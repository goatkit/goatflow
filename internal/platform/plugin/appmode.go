package plugin

import (
	"context"
	"fmt"
	"sort"
)

// AppMode is the app mode resolved for one agent: where login lands, what
// the shell hides and how it is branded. See pkg/plugin.AppModeSpec.
type AppMode struct {
	PluginName      string
	LandingPage     string
	HiddenMenuItems []string // never contains "admin"
	HidePluginMenus bool
	Branding        *UIBrandingSpec
}

// AppModeStatus is a plugin's declared app mode plus its administrator
// switch, as shown on the plugin administration page.
type AppModeStatus struct {
	Groups          []string `json:"groups"`
	LandingPage     string   `json:"landing_page"`
	HideMenuItems   []string `json:"hide_menu_items"`
	HidePluginMenus bool     `json:"hide_plugin_menus"`
	Enabled         bool     `json:"enabled"`
}

// appModeConfigKey returns the sysconfig key of a plugin's app-mode switch.
func appModeConfigKey(name string) string {
	return "Plugin::" + name + "::AppMode"
}

// loadAppModeEnabled seeds and reads the administrator switch of a manifest
// that declares app mode. A manifest without one gets false and no DB access.
func (m *Manager) loadAppModeEnabled(ctx context.Context, manifest GKRegistration) bool {
	if manifest.AppMode == nil {
		return false
	}
	key := appModeConfigKey(manifest.Name)
	m.seedPluginSwitch(ctx, manifest.Name, key, "Plugin app mode (present GoatFlow as the plugin's application to its groups)", "1")
	return m.loadPluginSwitch(ctx, manifest.Name, key)
}

// HasAppMode reports whether any enabled plugin has app mode switched on, so
// callers can skip resolving an agent's groups when nothing would use them.
func (m *Manager) HasAppMode() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, rp := range m.plugins {
		if rp.enabled && rp.appMode && rp.manifest.AppMode != nil {
			return true
		}
	}
	return false
}

// AppModeFor returns the app mode that applies to a non-administrator agent
// holding groups, or nil. Callers exempt administrators before asking.
// Plugins are considered in name order so the answer is stable when more
// than one qualifies.
func (m *Manager) AppModeFor(groups []string) *AppMode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.plugins))
	for name, rp := range m.plugins {
		if rp.enabled && rp.appMode && rp.manifest.AppMode != nil {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	for _, name := range names {
		spec := m.plugins[name].manifest.AppMode
		if !appModeApplies(spec, groups) {
			continue
		}
		return &AppMode{
			PluginName:      name,
			LandingPage:     spec.LandingPage,
			HiddenMenuItems: appModeHiddenItems(spec.HideMenuItems),
			HidePluginMenus: spec.HidePluginMenus,
			Branding:        spec.Branding,
		}
	}
	return nil
}

// appModeApplies reports whether an agent holding groups is in the app's
// audience: any declared group, or everyone when none is declared.
func appModeApplies(spec *AppModeSpec, groups []string) bool {
	if len(spec.Groups) == 0 {
		return true
	}
	for _, want := range spec.Groups {
		for _, have := range groups {
			if have == want {
				return true
			}
		}
	}
	return false
}

// appModeHiddenItems drops "admin": app mode never hides Administration.
func appModeHiddenItems(items []string) []string {
	out := make([]string, 0, len(items))
	for _, id := range items {
		if id != "admin" {
			out = append(out, id)
		}
	}
	return out
}

// AppModeStatus returns the declared app mode and its switch for a loaded
// plugin, or nil when the plugin declares none.
func (m *Manager) AppModeStatus(name string) *AppModeStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rp, ok := m.plugins[name]
	if !ok || rp.manifest.AppMode == nil {
		return nil
	}
	spec := rp.manifest.AppMode
	return &AppModeStatus{
		Groups:          append([]string(nil), spec.Groups...),
		LandingPage:     spec.LandingPage,
		HideMenuItems:   appModeHiddenItems(spec.HideMenuItems),
		HidePluginMenus: spec.HidePluginMenus,
		Enabled:         rp.appMode,
	}
}

// SetAppModeEnabled flips the administrator switch of a plugin's app mode.
// The change is persisted before it takes effect, so a failed save changes
// nothing. Plugins that declare no app mode are refused.
func (m *Manager) SetAppModeEnabled(name string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rp, ok := m.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %q not found", name)
	}
	if rp.manifest.AppMode == nil {
		return fmt.Errorf("plugin %q declares no app mode", name)
	}
	if err := m.savePluginSwitch(context.Background(), appModeConfigKey(name), enabled); err != nil {
		return fmt.Errorf("save app mode for %q: %w", name, err)
	}
	rp.appMode = enabled
	return nil
}
