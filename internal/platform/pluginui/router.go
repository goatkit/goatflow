package pluginui

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/apierrors"
	"github.com/goatkit/goatflow/internal/platform/middleware"
)

// PluginCaller is an interface for calling plugin functions.
// Satisfied by the plugin.Manager.
type PluginCaller interface {
	Call(ctx context.Context, pluginName, fn string, args []byte) ([]byte, error)
}

// TemplateRenderer is the interface for rendering pongo2 templates.
type TemplateRenderer interface {
	HTML(c *gin.Context, code int, name string, data interface{})
}

// UIAuth carries the API layer's auth middlewares and call envelope into UI
// registration (this package cannot import internal/api).
type UIAuth struct {
	// Authenticate identifies the caller from a session cookie, JWT or API
	// token and rejects anonymous requests.
	Authenticate gin.HandlerFunc
	// RequireGroup returns middleware admitting admins and members of the
	// named agent group.
	RequireGroup func(name string) gin.HandlerFunc
	// Envelope replaces any client-supplied envelope key in args with the
	// host's call envelope for the request's caller (identity, admin flag,
	// organisation, language), the same envelope every API plugin call
	// carries. The plugin Manager turns it into the call context. Required.
	Envelope func(c *gin.Context, args map[string]any, pluginName string)
}

// RegisterUIRoutes registers all active plugin UI routes on the given gin engine.
// Call this during dynamic engine rebuild, after YAML routes and before plugin routes.
// A UI whose config or auth settings cannot be honoured is logged and left
// unregistered; the other UIs still register.
func RegisterUIRoutes(eng *gin.Engine, repo *Repository, caller PluginCaller, renderer TemplateRenderer, auth UIAuth, logger *slog.Logger) error {
	uis, err := repo.ListActive()
	if err != nil {
		return err
	}
	registered := 0
	for _, ui := range uis {
		if err := registerOneUI(eng, ui, repo, caller, renderer, auth, logger); err != nil {
			logger.Error("plugin UI not registered", "ui", ui.FullID, "error", err)
			continue
		}
		registered++
	}

	if registered > 0 {
		logger.Info("registered plugin UI routes", "count", registered)
	}

	return nil
}

func registerOneUI(eng *gin.Engine, ui PluginUI, repo *Repository, caller PluginCaller, renderer TemplateRenderer, auth UIAuth, logger *slog.Logger) error {
	cfg, err := ui.ParsedConfig()
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	if auth.Envelope == nil {
		return fmt.Errorf("no call envelope builder: plugin calls would carry no caller identity")
	}

	basePath := "/ui/" + ui.FullID
	group := eng.Group(basePath)

	// Apply auth middleware based on UI type; an unsupported setting leaves
	// the UI unregistered rather than open.
	if err := applyAuthMiddleware(group, ui, cfg, auth); err != nil {
		return err
	}
	if effectiveAuthMethod(ui, cfg) == AuthNone {
		group.Use(publicRateLimit(ui.FullID, cfg.RateLimit))
	}

	// Register each route.
	for _, route := range cfg.Routes {
		method := route.Method
		if method == "" {
			method = "GET"
		}

		handler := buildUIHandler(ui, cfg, route, repo, caller, renderer, auth.Envelope)

		path := route.Path
		if path == "" {
			path = "/"
		}

		switch strings.ToUpper(method) {
		case "GET":
			group.GET(path, handler)
		case "POST":
			group.POST(path, handler)
		case "PUT":
			group.PUT(path, handler)
		case "DELETE":
			group.DELETE(path, handler)
		case "PATCH":
			group.PATCH(path, handler)
		default:
			group.GET(path, handler)
		}
	}

	// PWA manifest endpoint.
	if cfg.PWA != nil && cfg.PWA.Enabled {
		group.GET("/manifest.json", buildManifestHandler(ui, cfg))
	}

	logger.Debug("registered plugin UI", "ui", ui.FullID, "type", ui.UIType, "shell", ui.Shell, "routes", len(cfg.Routes))
	return nil
}

// UIInfoLookup resolves plugin UI rows (used for cross-UI nav gating).
type UIInfoLookup interface {
	GetByFullID(fullID string) (*PluginUI, error)
}

// buildUIHandler creates a gin handler that calls the plugin and wraps the response in the correct shell.
func buildUIHandler(ui PluginUI, cfg *UIConfig, route UIRouteConfig, repo UIInfoLookup, caller PluginCaller, renderer TemplateRenderer, envelope func(*gin.Context, map[string]any, string)) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Build args for plugin call.
		args := map[string]any{
			"path":       c.Request.URL.Path,
			"method":     c.Request.Method,
			"params":     extractParams(c),
			"query":      c.Request.URL.Query(),
			"ui_id":      ui.FullID,
			"data_scope": cfg.DataScope,
		}
		if c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH" {
			body, _ := c.GetRawData()
			if len(body) > 0 {
				args["body"] = string(body)
			}
		}

		// The host envelope: the authenticated caller's identity (acting user),
		// admin flag, organisation and language, under the same keys as API
		// plugin calls. _is_admin is always set, so a plugin that merges these
		// keys over the client-supplied form body never keeps a body value.
		envelope(c, args, ui.PluginName)

		argsJSON, _ := json.Marshal(args)

		result, err := caller.Call(c.Request.Context(), ui.PluginName, route.Handler, argsJSON)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Parse plugin response.
		var response map[string]any
		if err := json.Unmarshal(result, &response); err != nil {
			c.Data(http.StatusOK, "application/json", result)
			return
		}

		html, hasHTML := response["html"].(string)
		if !hasHTML {
			c.Data(http.StatusOK, "application/json", result)
			return
		}

		// Determine current path for nav active state.
		currentPath := strings.TrimPrefix(c.Request.URL.Path, "/ui/"+ui.FullID)
		if currentPath == "" {
			currentPath = "/"
		}

		// Build nav items with active state and badge counts.
		navItems := buildNavItems(c, ui, cfg, caller, currentPath, repo, envelope)

		// Build template data.
		branding := &UIBrandingConfig{}
		if cfg.Branding != nil {
			branding = cfg.Branding
		}

		// ui_nav is exposed to templates as a map (not the struct) because pongo2
		// resolves Go struct field names (Position) rather than JSON tags, and the
		// shell templates check `ui_nav.position`. A nil Nav stays nil so the
		// templates' `{% if ui_nav … %}` guard falls through.
		var uiNavCtx any
		if cfg.Nav != nil {
			uiNavCtx = map[string]any{"position": cfg.Nav.Position, "items": cfg.Nav.Items}
		}

		tplData := pongo2.Context{
			"PluginHTML":     html,
			"ui_full_id":     ui.FullID,
			"ui_type":        ui.UIType,
			"ui_shell":       ui.Shell,
			"ui_branding":    branding,
			"ui_nav":         uiNavCtx,
			"ui_nav_items":   navItems,
			"ui_pwa_enabled": cfg.PWA != nil && cfg.PWA.Enabled,
			"ActivePage":     "plugin",
		}

		// Add title from response if present.
		if title, ok := response["title"].(string); ok {
			tplData["PluginTitle"] = title
		}

		// Render with the correct shell template.
		shellTemplate := shellTemplateName(ui.Shell)

		if renderer != nil {
			renderer.HTML(c, http.StatusOK, shellTemplate, tplData)
			return
		}

		// Fallback: raw HTML.
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, html)
	}
}

// buildManifestHandler creates a handler for the PWA manifest.json endpoint.
func buildManifestHandler(ui PluginUI, cfg *UIConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		branding := &UIBrandingConfig{}
		if cfg.Branding != nil {
			branding = cfg.Branding
		}

		appName := branding.AppName
		if appName == "" {
			appName = ui.Name
		}

		startURL := ui.BasePath()
		if cfg.PWA.StartURL != "" {
			startURL = cfg.PWA.StartURL
		}

		display := "standalone"
		if cfg.PWA.Display != "" {
			display = cfg.PWA.Display
		}

		themeColor := "#1a1a2e"
		if cfg.PWA.ThemeColor != "" {
			themeColor = cfg.PWA.ThemeColor
		} else if branding.Color != "" {
			themeColor = branding.Color
		}

		manifest := map[string]any{
			"name":             appName,
			"short_name":       appName,
			"start_url":        startURL,
			"display":          display,
			"background_color": "#1a1a2e",
			"theme_color":      themeColor,
			"icons": []map[string]string{
				{"src": "/static/images/icon-192.png", "sizes": "192x192", "type": "image/png"},
				{"src": "/static/images/icon-512.png", "sizes": "512x512", "type": "image/png"},
			},
		}

		c.JSON(http.StatusOK, manifest)
	}
}

// buildNavItems resolves nav items with active state, badge counts, and
// hrefs. An item Path that is an absolute "/ui/..." path links into another
// plugin's UI; such items are only kept when the target UI is enabled and
// its plugin is enabled.
func buildNavItems(c *gin.Context, ui PluginUI, cfg *UIConfig, caller PluginCaller, currentPath string, repo UIInfoLookup, envelope func(*gin.Context, map[string]any, string)) []map[string]any {
	if cfg.Nav == nil || len(cfg.Nav.Items) == 0 {
		return nil
	}

	items := make([]map[string]any, 0, len(cfg.Nav.Items))
	for _, item := range cfg.Nav.Items {
		href := "/ui/" + ui.FullID + item.Path
		if strings.HasPrefix(item.Path, "/ui/") {
			if !crossUIAvailable(repo, caller, item.Path) {
				continue
			}
			href = item.Path
		}
		active := currentPath == item.Path || (item.Path != "/" && strings.HasPrefix(currentPath, item.Path))

		navItem := map[string]any{
			"label":  item.Label,
			"icon":   item.Icon,
			"path":   item.Path,
			"href":   href,
			"active": active,
		}

		// Resolve badge count if badge function is specified.
		if item.Badge != "" && caller != nil {
			badgeArgs := map[string]any{"ui_id": ui.FullID}
			envelope(c, badgeArgs, ui.PluginName)
			badgeJSON, _ := json.Marshal(badgeArgs)
			if result, err := caller.Call(c.Request.Context(), ui.PluginName, item.Badge, badgeJSON); err == nil {
				var badgeResp map[string]any
				if json.Unmarshal(result, &badgeResp) == nil {
					if count, ok := badgeResp["count"]; ok {
						navItem["badge_count"] = wholeNumber(count)
					}
				}
			}
		}

		items = append(items, navItem)
	}
	return items
}

// crossUIAvailable reports whether an absolute /ui/... nav path points at a
// plugin UI that is currently usable: the target UI row is enabled and valid,
// and the target plugin is enabled.
func crossUIAvailable(repo UIInfoLookup, caller PluginCaller, path string) bool {
	rest := strings.TrimPrefix(path, "/ui/")
	fullID := rest
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		fullID = rest[:i]
	}
	if repo == nil || fullID == "" {
		return false
	}
	row, err := repo.GetByFullID(fullID)
	// Same predicate as ListActive's route registration, so a visible link
	// can never point at an unregistered path.
	if err != nil || row == nil || !row.IsActive() {
		return false
	}
	enabler, ok := caller.(interface{ IsEnabled(string) bool })
	if !ok || !enabler.IsEnabled(row.PluginName) {
		return false
	}
	return true
}

// wholeNumber coerces a JSON float that holds a whole value (e.g. the 8.0
// produced by a badge fn returning `{"count": 8}`) to an int so templates render
// "8" rather than "8.000000". Non-whole or non-numeric values pass through.
func wholeNumber(v any) any {
	f, ok := v.(float64)
	if !ok || f != math.Trunc(f) {
		return v
	}
	return int64(f)
}

// DefaultPublicRateLimit is the per-client request budget (requests per
// minute) of a public plugin UI whose rate_limit is unset.
const DefaultPublicRateLimit = 60

// publicRateLimit limits each client IP to perMinute requests per minute on a
// public (auth method none) plugin UI, using the shared rate limiter.
func publicRateLimit(fullID string, perMinute int) gin.HandlerFunc {
	if perMinute <= 0 {
		perMinute = DefaultPublicRateLimit
	}
	return func(c *gin.Context) {
		key := "pluginui:" + fullID + ":" + c.ClientIP()
		if !middleware.GlobalRateLimiter().AllowPer(key, perMinute, time.Minute) {
			c.Header("Retry-After", "60")
			apierrors.Error(c, apierrors.CodeRateLimited)
			c.Abort()
			return
		}
		c.Next()
	}
}

// effectiveAuthMethod is the UI's configured auth method, or its type's
// default when none is configured.
func effectiveAuthMethod(ui PluginUI, cfg *UIConfig) string {
	if cfg.Auth != nil && cfg.Auth.Method != "" {
		return cfg.Auth.Method
	}
	return DefaultAuthMethod(ui.UIType)
}

// applyAuthMiddleware gates a UI's route group according to its auth method
// and type. session and token both authenticate through auth.Authenticate
// (session cookie, Bearer JWT or API token), then admin_page/agent_app UIs
// admit agents only and customer_app UIs customers only; any auth.groups must
// all be held. none leaves the UI public (the default for public_page and
// kiosk). Any other method is an error: nothing would enforce it.
func applyAuthMiddleware(group *gin.RouterGroup, ui PluginUI, cfg *UIConfig, auth UIAuth) error {
	authMethod := effectiveAuthMethod(ui, cfg)
	var groups []string
	if cfg.Auth != nil {
		groups = cfg.Auth.Groups
	}

	switch authMethod {
	case AuthNone:
		if len(groups) > 0 {
			return fmt.Errorf("auth groups %v need session or token auth, not %q", groups, AuthNone)
		}
		return nil
	case AuthSession, AuthToken:
	default:
		return fmt.Errorf("unsupported auth method %q", authMethod)
	}

	if auth.Authenticate == nil {
		return fmt.Errorf("auth method %q: no authentication middleware configured", authMethod)
	}
	handlers := []gin.HandlerFunc{auth.Authenticate}
	switch ui.UIType {
	case TypeAdminPage, TypeAgentApp:
		handlers = append(handlers, middleware.RequireAgent())
	case TypeCustomerApp:
		if len(groups) > 0 {
			return fmt.Errorf("auth groups %v are agent groups and cannot gate a %s UI", groups, TypeCustomerApp)
		}
		handlers = append(handlers, middleware.RequireCustomer())
	}
	for _, g := range groups {
		if g == "" {
			return fmt.Errorf("auth groups contain an empty group name")
		}
		if auth.RequireGroup == nil {
			return fmt.Errorf("auth group %q: no group middleware configured", g)
		}
		handlers = append(handlers, auth.RequireGroup(g))
	}
	group.Use(handlers...)
	return nil
}

// shellTemplateName returns the template path for a shell type.
func shellTemplateName(shell string) string {
	switch shell {
	case ShellNone:
		return "layouts/ui_none.pongo2"
	case ShellMinimal:
		return "layouts/ui_minimal.pongo2"
	case ShellStandard:
		return "layouts/ui_standard.pongo2"
	default:
		return "layouts/ui_standard.pongo2"
	}
}

// extractParams extracts URL parameters from gin context.
func extractParams(c *gin.Context) map[string]string {
	params := make(map[string]string)
	for _, p := range c.Params {
		params[p.Key] = p.Value
	}
	return params
}

// GenerateMenuItems converts active plugin UIs into MenuItemSpecs for navigation injection.
// Returns items grouped by location: "admin", "agent", "customer".
func GenerateMenuItems(uis []PluginUI) map[string][]map[string]any {
	result := map[string][]map[string]any{
		"admin":    {},
		"agent":    {},
		"customer": {},
	}

	for _, ui := range uis {
		if !ui.IsActive() {
			continue
		}

		item := map[string]any{
			"id":    "ui_" + ui.FullID,
			"label": ui.Name,
			"path":  ui.BasePath(),
			"order": 900, // After core nav items
		}
		if ui.Icon != nil {
			item["icon"] = *ui.Icon
		}

		switch ui.UIType {
		case TypeAdminPage:
			result["admin"] = append(result["admin"], item)
		case TypeAgentApp:
			result["agent"] = append(result["agent"], item)
		case TypeCustomerApp:
			result["customer"] = append(result["customer"], item)
			// public_page and kiosk don't appear in nav
		}
	}

	return result
}
