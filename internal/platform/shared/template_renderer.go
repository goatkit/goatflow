package shared

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/config"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/i18n"
	"github.com/goatkit/goatflow/internal/platform/lookups"
	"github.com/goatkit/goatflow/internal/platform/middleware"
	platformmodels "github.com/goatkit/goatflow/internal/platform/models"
	"github.com/goatkit/goatflow/internal/platform/version"
)

// TemplateOverrideProvider allows plugins to override templates without import cycles.
type TemplateOverrideProvider interface {
	RenderOverride(ctx context.Context, templateName string, data map[string]any) (string, bool)
}

var globalTemplateOverrideProvider TemplateOverrideProvider

// SetTemplateOverrideProvider sets the global template override provider.
func SetTemplateOverrideProvider(p TemplateOverrideProvider) {
	globalTemplateOverrideProvider = p
}

// PluginMenuProvider returns plugin menu items for a given location (e.g. "admin", "agent").
// This avoids import cycles between shared and api/plugin packages.
type PluginMenuProvider func(location string) []map[string]any

var globalPluginMenuProvider PluginMenuProvider

// SetPluginMenuProvider sets the global provider for plugin menu items in templates.
func SetPluginMenuProvider(p PluginMenuProvider) {
	globalPluginMenuProvider = p
}

// PluginAccessChecker returns true if the current request's caller is
// entitled to use `pluginName`. Injected from the api package to avoid
// pulling database/repository into shared. Used by the template renderer
// to hide nav menu items the caller can't use — a customer without
// goatfictus entitlement shouldn't see the GoatFictus link and then
// bounce off a 403.
type PluginAccessChecker func(c *gin.Context, pluginName string) bool

var globalPluginAccessChecker PluginAccessChecker

// SetPluginAccessChecker installs the callback used by the template
// renderer to filter plugin-menu entries against per-user access.
func SetPluginAccessChecker(p PluginAccessChecker) {
	globalPluginAccessChecker = p
}

// HiddenMenuProvider returns a map of menu item IDs that should be hidden.
type HiddenMenuProvider func() map[string]bool

var globalHiddenMenuProvider HiddenMenuProvider

// SetHiddenMenuProvider sets the global provider for hidden menu items in templates.
func SetHiddenMenuProvider(p HiddenMenuProvider) {
	globalHiddenMenuProvider = p
}

// LandingPageProvider returns the plugin-defined landing page URL, or empty string.
type LandingPageProvider func() string

var globalLandingPageProvider LandingPageProvider

// SetLandingPageProvider sets the global provider for the plugin landing page.
func SetLandingPageProvider(p LandingPageProvider) {
	globalLandingPageProvider = p
}

// GetLandingPage returns the plugin-defined landing page URL, or empty string.
func GetLandingPage() string {
	if globalLandingPageProvider != nil {
		return globalLandingPageProvider()
	}
	return ""
}

// TemplateRenderer handles template rendering with pongo2.
type TemplateRenderer struct {
	templateSet *pongo2.TemplateSet
	templateDir string
}

// NewTemplateRenderer creates a new template renderer.
func NewTemplateRenderer(templateDir string) (*TemplateRenderer, error) {
	if templateDir == "" {
		return nil, fmt.Errorf("template directory is required")
	}

	// Check if directory exists
	if _, err := os.Stat(templateDir); err != nil {
		return nil, fmt.Errorf("template directory not found: %v", err)
	}

	// Normalize path
	abs, _ := filepath.Abs(templateDir)

	// Create template set
	templateSet := pongo2.NewSet("goatflow", pongo2.MustNewLocalFileSystemLoader(abs))

	return &TemplateRenderer{
		templateSet: templateSet,
		templateDir: abs,
	}, nil
}

// TemplateSet returns the underlying pongo2 template set for modules that need direct access.
func (r *TemplateRenderer) TemplateSet() *pongo2.TemplateSet {
	if r == nil {
		return nil
	}
	return r.templateSet
}

// HTML renders a template.
func (r *TemplateRenderer) HTML(c *gin.Context, code int, name string, data interface{}) {
	// Convert gin.H to pongo2.Context
	var ctx pongo2.Context
	switch v := data.(type) {
	case pongo2.Context:
		ctx = v
	case gin.H:
		ctx = pongo2.Context(v)
	case map[string]interface{}:
		ctx = pongo2.Context(v)
	default:
		ctx = pongo2.Context{"data": data}
	}

	// Language helpers injected via middleware detection
	lang := middleware.GetLanguage(c)
	i18nInst := i18n.GetInstance()
	ctx["t"] = func(key string, args ...interface{}) string {
		return translateWithFallback(i18nInst, lang, key, args...)
	}
	ctx["getLang"] = func() string { return lang }
	ctx["getDirection"] = func() string { return string(i18n.GetDirection(lang)) }
	ctx["isRTL"] = func() bool { return i18n.IsRTL(lang) }
	ctx["Countries"] = lookups.Countries()

	// Add language info directly to context
	ctx["Lang"] = lang
	ctx["Direction"] = string(i18n.GetDirection(lang))
	ctx["IsRTL"] = i18n.IsRTL(lang)

	// Add version info to context
	ctx["AppVersion"] = version.String()
	ctx["AppVersionShort"] = version.Short()
	ctx["AppVersionFull"] = version.Full()
	ctx["AppVersionInfo"] = version.GetInfo()

	// Auto-inject User and IsInAdminGroup from context for consistent nav bar
	isAdmin := false
	if flag, exists := c.Get("isInAdminGroup"); exists {
		if b, ok := flag.(bool); ok {
			isAdmin = b
		}
	}
	ctx["IsInAdminGroup"] = isAdmin

	// isCustomer gates the "Administration" nav button in base.pongo2
	// (and equivalents). If this key is missing, pongo2 treats it as false
	// and the guard `{% if not isCustomer ... %}` always passes — which is
	// how a customer session rendering a plugin page ended up seeing the
	// Administration button. Inject it from the auth middleware's
	// is_customer / user_role keys, with both lowercase and PascalCase
	// aliases so templates that use either variant stay coherent.
	isCustomer := false
	if v, exists := c.Get("is_customer"); exists {
		if b, ok := v.(bool); ok && b {
			isCustomer = true
		}
	}
	if !isCustomer {
		if role, exists := c.Get("user_role"); exists {
			if s, ok := role.(string); ok && s == "Customer" {
				isCustomer = true
			}
		}
	}
	ctx["isCustomer"] = isCustomer
	ctx["IsCustomer"] = isCustomer

	// Inject User from context if available
	if _, hasUser := ctx["User"]; !hasUser {
		if user := getUserFromContext(c, isAdmin); user != nil {
			ctx["User"] = user
		}
	}

	// Inject plugin menu items for navigation. Customer-side items are
	// filtered against per-user plugin access so an unentitled customer
	// doesn't see links they'd bounce off with a 403. Agent- and
	// admin-side menus stay global — agents are cross-cutting staff
	// whose access is gated at the route layer, not the menu.
	if globalPluginMenuProvider != nil {
		if _, hasIt := ctx["PluginAdminMenuItems"]; !hasIt {
			ctx["PluginAdminMenuItems"] = globalPluginMenuProvider("admin")
		}
		if _, hasIt := ctx["PluginAgentMenuItems"]; !hasIt {
			ctx["PluginAgentMenuItems"] = globalPluginMenuProvider("agent")
		}
		if _, hasIt := ctx["PluginCustomerMenuItems"]; !hasIt {
			items := globalPluginMenuProvider("customer")
			if isCustomer && globalPluginAccessChecker != nil {
				filtered := make([]map[string]any, 0, len(items))
				for _, it := range items {
					name, _ := it["PluginName"].(string)
					if name == "" {
						// Menu items without a declared plugin name aren't
						// access-gated — keep them visible so legacy items
						// don't silently vanish.
						filtered = append(filtered, it)
						continue
					}
					if globalPluginAccessChecker(c, name) {
						filtered = append(filtered, it)
					}
				}
				items = filtered
			}
			ctx["PluginCustomerMenuItems"] = items
		}
	}

	// Inject hidden nav items for navigation control
	if globalHiddenMenuProvider != nil {
		if _, hasIt := ctx["HiddenNavItems"]; !hasIt {
			ctx["HiddenNavItems"] = globalHiddenMenuProvider()
		}
	}

	// Check for active/upcoming maintenance and add to context
	addMaintenanceContext(ctx)

	// Check for plugin template override
	if globalTemplateOverrideProvider != nil {
		// Convert pongo2.Context to map for plugin
		dataMap := make(map[string]any, len(ctx))
		for k, v := range ctx {
			dataMap[k] = v
		}
		if html, ok := globalTemplateOverrideProvider.RenderOverride(c.Request.Context(), name, dataMap); ok {
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.String(code, html)
			return
		}
	}

	// Get the template (fallback for tests when templates missing)
	if r == nil || r.templateSet == nil {
		// Minimal safe fallback for tests: render a tiny stub
		c.String(code, "GoatFlow")
		return
	}
	tmpl, err := r.templateSet.FromFile(name)
	if err != nil {
		log.Printf("Template renderer failed to load template %q: %v", name, err)
		c.String(code, "Template not found: %s", name)
		return
	}

	// Set response headers
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(code)

	// Render template
	err = tmpl.ExecuteWriter(ctx, c.Writer)
	if err != nil {
		c.String(http.StatusInternalServerError, "Template execution error: %v", err)
	}
}

// getUserFromContext extracts the user from gin context (set by JWT middleware).
func getUserFromContext(c *gin.Context, isAdmin bool) *platformmodels.User {
	// Try direct user object first
	userInterface, exists := c.Get("user")
	if exists {
		if user, ok := userInterface.(*platformmodels.User); ok {
			user.IsInAdminGroup = isAdmin
			return user
		}
		if user, ok := userInterface.(platformmodels.User); ok {
			user.IsInAdminGroup = isAdmin
			return &user
		}
	}

	// Fall back to building user from individual context values
	if _, hasID := c.Get("user_id"); !hasID {
		return nil
	}

	userID := GetUserIDFromCtxUint(c, 0)
	if userID == 0 {
		return nil
	}

	user := &platformmodels.User{IsInAdminGroup: isAdmin, ID: userID}

	// Set role
	if role, ok := c.Get("user_role"); ok {
		if r, ok := role.(string); ok {
			user.Role = r
		}
	}

	// Set email
	if email, ok := c.Get("user_email"); ok {
		if e, ok := email.(string); ok {
			user.Email = e
			user.Login = e
		}
	}

	// Set name
	if name, ok := c.Get("user_name"); ok {
		if n, ok := name.(string); ok {
			parts := strings.SplitN(n, " ", 2)
			if len(parts) > 0 {
				user.FirstName = parts[0]
			}
			if len(parts) > 1 {
				user.LastName = parts[1]
			}
		}
	}

	return user
}

// translateWithFallback provides a fallback translation function.
func translateWithFallback(i18nInst *i18n.I18n, lang, key string, args ...interface{}) string {
	if i18nInst == nil {
		return key
	}
	return i18nInst.Translate(lang, key, args...)
}

// GetGlobalRenderer returns the global template renderer instance.
func GetGlobalRenderer() *TemplateRenderer {
	return globalTemplateRenderer
}

// SetGlobalRenderer sets the global template renderer instance.
func SetGlobalRenderer(renderer *TemplateRenderer) {
	globalTemplateRenderer = renderer
}

var globalTemplateRenderer *TemplateRenderer

// addMaintenanceContext checks for active/upcoming maintenance and adds to template context.
func addMaintenanceContext(ctx pongo2.Context) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		return
	}

	repo := middleware.NewMaintenanceChecker(db)
	if repo == nil {
		return
	}
	// Check active maintenance
	if active, err := repo.IsActive(); err == nil && active != nil {
		// Use default message from config if not set in record
		if active.NotifyMessage == nil || *active.NotifyMessage == "" {
			cfg := config.Get()
			if cfg != nil && cfg.Maintenance.DefaultNotifyMessage != "" {
				defaultMsg := cfg.Maintenance.DefaultNotifyMessage
				active.NotifyMessage = &defaultMsg
			}
		}
		ctx["MaintenanceActive"] = active
	}

	// Check upcoming - get minutes from config
	cfg := config.Get()
	upcomingMinutes := 30 // Default fallback
	if cfg != nil && cfg.Maintenance.TimeNotifyUpcomingMinutes > 0 {
		upcomingMinutes = cfg.Maintenance.TimeNotifyUpcomingMinutes
	}
	if coming, err := repo.IsComing(upcomingMinutes); err == nil && coming != nil {
		ctx["MaintenanceComing"] = coming
	}
}
