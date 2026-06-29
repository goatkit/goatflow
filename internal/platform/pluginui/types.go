// Package pluginui implements the GoatKit PaaS Plugin UI System.
//
// Plugins declare independent UIs (agent apps, customer portals, public pages,
// kiosks) via UISpec in GKRegistration. The platform handles routing, shells,
// auth, navigation, and PWA manifest generation.
package pluginui

import (
	"encoding/json"
	"strings"
	"time"
)

// UI types.
const (
	TypeAdminPage   = "admin_page"
	TypeAgentApp    = "agent_app"
	TypeCustomerApp = "customer_app"
	TypePublicPage  = "public_page"
	TypeKiosk       = "kiosk"
)

// Shell types.
const (
	ShellNone     = "none"
	ShellMinimal  = "minimal"
	ShellStandard = "standard"
)

// Auth methods.
const (
	AuthSession = "session"
	AuthPIN     = "pin"
	AuthToken   = "token"
	AuthNone    = "none"
)

// Data scopes.
const (
	ScopeSelf = "self"
	ScopeOrg  = "org"
	ScopeAll  = "all"
)

// Nav positions.
const (
	NavBottom = "bottom"
	NavTop    = "top"
	NavSide   = "side"
)

// ValidUITypes returns all supported UI types.
func ValidUITypes() []string {
	return []string{TypeAdminPage, TypeAgentApp, TypeCustomerApp, TypePublicPage, TypeKiosk}
}

// ValidShells returns all supported shell types.
func ValidShells() []string {
	return []string{ShellNone, ShellMinimal, ShellStandard}
}

// ValidAuthMethods returns all supported auth methods.
func ValidAuthMethods() []string {
	return []string{AuthSession, AuthPIN, AuthToken, AuthNone}
}

// ValidDataScopes returns all supported data scopes.
func ValidDataScopes() []string {
	return []string{ScopeSelf, ScopeOrg, ScopeAll}
}

// ValidNavPositions returns all supported nav positions.
func ValidNavPositions() []string {
	return []string{NavBottom, NavTop, NavSide}
}

// IsValidUIType checks whether the given UI type is supported.
func IsValidUIType(t string) bool {
	for _, v := range ValidUITypes() {
		if t == v {
			return true
		}
	}
	return false
}

// IsValidShell checks whether the given shell type is supported.
func IsValidShell(s string) bool {
	for _, v := range ValidShells() {
		if s == v {
			return true
		}
	}
	return false
}

// DefaultShell returns the default shell for a UI type.
func DefaultShell(uiType string) string {
	switch uiType {
	case TypeAdminPage, TypeAgentApp:
		return ShellStandard
	case TypeCustomerApp, TypePublicPage:
		return ShellMinimal
	case TypeKiosk:
		return ShellNone
	default:
		return ShellStandard
	}
}

// DefaultAuthMethod returns the default auth method for a UI type.
func DefaultAuthMethod(uiType string) string {
	switch uiType {
	case TypeAdminPage, TypeAgentApp:
		return AuthSession
	case TypeCustomerApp:
		return AuthSession
	case TypePublicPage:
		return AuthNone
	case TypeKiosk:
		return AuthNone
	default:
		return AuthSession
	}
}

// PluginUI represents a stored plugin UI from gk_plugin_ui.
type PluginUI struct {
	ID           int64            `json:"id" db:"id"`
	PluginName   string           `json:"plugin_name" db:"plugin_name"`
	UIID         string           `json:"ui_id" db:"ui_id"`
	FullID       string           `json:"full_id" db:"full_id"`
	Name         string           `json:"name" db:"name"`
	Description  *string          `json:"description,omitempty" db:"description"`
	UIType       string           `json:"ui_type" db:"ui_type"`
	Shell        string           `json:"shell" db:"shell"`
	Icon         *string          `json:"icon,omitempty" db:"icon"`
	Config       *json.RawMessage `json:"config,omitempty" db:"config"`
	Enabled      bool             `json:"enabled" db:"enabled"`
	CustomDomain *string          `json:"custom_domain,omitempty" db:"custom_domain"`
	ValidID      int              `json:"valid_id" db:"valid_id"`
	CreateTime   time.Time        `json:"create_time" db:"create_time"`
	CreateBy     int              `json:"create_by" db:"create_by"`
	ChangeTime   time.Time        `json:"change_time" db:"change_time"`
	ChangeBy     int              `json:"change_by" db:"change_by"`
}

// IsActive returns true if this UI is valid and enabled.
func (u *PluginUI) IsActive() bool {
	return u.ValidID == 1 && u.Enabled
}

// ParsedConfig returns the full UISpec config from the JSON column.
func (u *PluginUI) ParsedConfig() (*UIConfig, error) {
	if u.Config == nil {
		return &UIConfig{}, nil
	}
	var cfg UIConfig
	if err := json.Unmarshal(*u.Config, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SetBranding merges administrator-managed branding overrides into the stored UI config.
func (u *PluginUI) SetBranding(branding *UIBrandingConfig) error {
	cfg, err := u.ParsedConfig()
	if err != nil {
		return err
	}
	if branding == nil || branding.IsZero() {
		cfg.Branding = nil
	} else {
		cfg.Branding = branding.Clean()
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	jrm := json.RawMessage(raw)
	u.Config = &jrm
	return nil
}

// BasePath returns the URL path prefix for this UI: /ui/{full_id}/
func (u *PluginUI) BasePath() string {
	return "/ui/" + u.FullID + "/"
}

// UIConfig stores the full UISpec configuration in the config JSON column.
type UIConfig struct {
	Routes    []UIRouteConfig   `json:"routes,omitempty"`
	Nav       *UINavConfig      `json:"nav,omitempty"`
	Branding  *UIBrandingConfig `json:"branding,omitempty"`
	Auth      *UIAuthConfig     `json:"auth,omitempty"`
	PWA       *UIPWAConfig      `json:"pwa,omitempty"`
	DataScope string            `json:"data_scope,omitempty"`
	RateLimit int               `json:"rate_limit,omitempty"`
}

// UIRouteConfig is a route within a plugin UI.
type UIRouteConfig struct {
	Path    string `json:"path"`
	Method  string `json:"method,omitempty"`
	Handler string `json:"handler"`
}

// UINavConfig defines the navigation for a plugin UI.
type UINavConfig struct {
	Position string            `json:"position"` // bottom, top, side
	Items    []UINavItemConfig `json:"items"`
}

// UINavItemConfig is a navigation item.
type UINavItemConfig struct {
	Label string `json:"label"`
	Icon  string `json:"icon"`
	Path  string `json:"path"`
	Badge string `json:"badge,omitempty"` // plugin function name for badge count
	Order int    `json:"order,omitempty"`
}

// UIBrandingConfig holds per-UI branding overrides.
type UIBrandingConfig struct {
	AppName string `json:"app_name,omitempty"`
	Logo    string `json:"logo,omitempty"`
	Favicon string `json:"favicon,omitempty"`
	Color   string `json:"color,omitempty"`
}

// Clean trims string fields and returns a normalized branding config.
func (b *UIBrandingConfig) Clean() *UIBrandingConfig {
	if b == nil {
		return nil
	}
	return &UIBrandingConfig{
		AppName: strings.TrimSpace(b.AppName),
		Logo:    strings.TrimSpace(b.Logo),
		Favicon: strings.TrimSpace(b.Favicon),
		Color:   strings.TrimSpace(b.Color),
	}
}

// IsZero reports whether no branding override is set.
func (b *UIBrandingConfig) IsZero() bool {
	if b == nil {
		return true
	}
	clean := b.Clean()
	return clean.AppName == "" && clean.Logo == "" && clean.Favicon == "" && clean.Color == ""
}

// UIAuthConfig holds per-UI auth configuration.
type UIAuthConfig struct {
	Method string   `json:"method,omitempty"` // session, pin, token, none
	Groups []string `json:"groups,omitempty"`
}

// UIPWAConfig holds PWA manifest configuration.
type UIPWAConfig struct {
	Enabled     bool     `json:"enabled"`
	StartURL    string   `json:"start_url,omitempty"`
	Display     string   `json:"display,omitempty"` // standalone, fullscreen, minimal-ui
	ThemeColor  string   `json:"theme_color,omitempty"`
	CacheRoutes []string `json:"cache_routes,omitempty"`
}

func cleanOptionalString(s *string) *string {
	if s == nil {
		return nil
	}
	cleaned := strings.TrimSpace(*s)
	if cleaned == "" {
		return nil
	}
	return &cleaned
}
