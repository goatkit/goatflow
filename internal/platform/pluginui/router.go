package pluginui

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
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

// RegisterUIRoutes registers all active plugin UI routes on the given gin engine.
// Call this during dynamic engine rebuild, after YAML routes and before plugin routes.
func RegisterUIRoutes(eng *gin.Engine, repo *Repository, caller PluginCaller, renderer TemplateRenderer, sessionAuth gin.HandlerFunc, logger *slog.Logger) error {
	uis, err := repo.ListActive()
	if err != nil {
		return fmt.Errorf("load active plugin UIs: %w", err)
	}

	for _, ui := range uis {
		if err := registerOneUI(eng, ui, caller, renderer, sessionAuth, logger); err != nil {
			logger.Warn("failed to register plugin UI routes", "ui", ui.FullID, "error", err)
		}
	}

	if len(uis) > 0 {
		logger.Info("registered plugin UI routes", "count", len(uis))
	}

	return nil
}

func registerOneUI(eng *gin.Engine, ui PluginUI, caller PluginCaller, renderer TemplateRenderer, sessionAuth gin.HandlerFunc, logger *slog.Logger) error {
	cfg, err := ui.ParsedConfig()
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	basePath := "/ui/" + ui.FullID
	group := eng.Group(basePath)

	// Apply auth middleware based on UI type.
	applyAuthMiddleware(group, ui, cfg, sessionAuth)

	// Register each route.
	for _, route := range cfg.Routes {
		method := route.Method
		if method == "" {
			method = "GET"
		}

		handler := buildUIHandler(ui, cfg, route, caller, renderer)

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

// buildUIHandler creates a gin handler that calls the plugin and wraps the response in the correct shell.
func buildUIHandler(ui PluginUI, cfg *UIConfig, route UIRouteConfig, caller PluginCaller, renderer TemplateRenderer) gin.HandlerFunc {
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

		// Forward the authenticated user's identity so plugins can resolve the
		// acting user on UI page calls (same keys as the API buildPluginArgs).
		// Without this, UI page handlers can only guess who is calling.
		if userID, exists := c.Get("user_id"); exists {
			args["_user_id"] = userID
		}
		if email, exists := c.Get("user_email"); exists {
			args["_user_email"] = email
		}
		if login, exists := c.Get("user_login"); exists {
			args["_user_login"] = login
		} else if username, exists := c.Get("username"); exists {
			args["_user_login"] = username
		} else if email, exists := c.Get("user_email"); exists {
			args["_user_login"] = email
		}
		if isAdmin, exists := c.Get("isInAdminGroup"); exists {
			args["_is_admin"] = isAdmin
		}
		if role, exists := c.Get("user_role"); exists {
			args["_user_role"] = role
		}
		if orgID, exists := c.Get("org_id"); exists {
			args["_org_id"] = orgID
			args["org_id"] = orgID
		}

		argsJSON, _ := json.Marshal(args)
		ctx := c.Request.Context()

		result, err := caller.Call(ctx, ui.PluginName, route.Handler, argsJSON)
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

		// Raw asset passthrough (e.g. a plugin-embedded JS/CSS bundle served
		// from a UI route): a response with content_type + content is served
		// verbatim instead of being wrapped in the shell. Content is a string,
		// so arbitrary bytes travel as JSON and arrive base64-free.
		if contentType, ok := response["content_type"].(string); ok {
			if content, ok := response["content"].(string); ok {
				c.Data(http.StatusOK, contentType, []byte(content))
				return
			}
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
		navItems := buildNavItems(c, ui, cfg, caller, currentPath)

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

// buildNavItems resolves nav items with active state and badge counts.
func buildNavItems(c *gin.Context, ui PluginUI, cfg *UIConfig, caller PluginCaller, currentPath string) []map[string]any {
	if cfg.Nav == nil || len(cfg.Nav.Items) == 0 {
		return nil
	}

	items := make([]map[string]any, 0, len(cfg.Nav.Items))
	for _, item := range cfg.Nav.Items {
		active := currentPath == item.Path || (item.Path != "/" && strings.HasPrefix(currentPath, item.Path))

		navItem := map[string]any{
			"label":  item.Label,
			"icon":   item.Icon,
			"path":   item.Path,
			"active": active,
		}

		// Resolve badge count if badge function is specified.
		if item.Badge != "" && caller != nil {
			badgeArgs, _ := json.Marshal(map[string]any{"ui_id": ui.FullID})
			if result, err := caller.Call(c.Request.Context(), ui.PluginName, item.Badge, badgeArgs); err == nil {
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

// applyAuthMiddleware adds the correct auth middleware to a route group based on UI type.
func applyAuthMiddleware(group *gin.RouterGroup, ui PluginUI, cfg *UIConfig, sessionAuth gin.HandlerFunc) {
	authMethod := DefaultAuthMethod(ui.UIType)
	if cfg.Auth != nil && cfg.Auth.Method != "" {
		authMethod = cfg.Auth.Method
	}

	switch authMethod {
	case AuthSession:
		// Session auth: authenticate the request (populating user_id /
		// isInAdminGroup / user_email / username in the gin context) so
		// buildUIHandler can forward the acting identity to the plugin. The
		// middleware is supplied by the API layer (SessionOrJWTAuth) to avoid
		// an import cycle. Previously UI routes were unauthenticated and the
		// plugin could only guess who was calling.
		if sessionAuth != nil {
			group.Use(sessionAuth)
		}
	case AuthPIN:
		// PIN auth handled by the plugin — platform provides the PIN entry route.
	case AuthToken:
		// Token auth — accept Bearer tokens.
	case AuthNone:
		// No auth required.
	}
	// Note: actual middleware functions are wired in by the dynamic router,
	// which has access to the auth middleware implementations.
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
