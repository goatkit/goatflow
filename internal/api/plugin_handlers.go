package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/auth"
	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/middleware"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/organisation"
	"github.com/goatkit/goatflow/internal/plugin"
	"github.com/goatkit/goatflow/internal/plugin/packaging"
	"github.com/goatkit/goatflow/internal/repository"
)

// pluginContextWithLanguage adds the request language to the context for i18n support.
func pluginContextWithLanguage(c *gin.Context) context.Context {
	ctx := c.Request.Context()
	if lang, exists := c.Get(middleware.LanguageContextKey); exists {
		if langStr, ok := lang.(string); ok {
			ctx = context.WithValue(ctx, plugin.PluginLanguageKey, langStr)
		}
	}
	return ctx
}

// pluginManager is the global plugin manager instance.
// Set via SetPluginManager during app initialization.
var pluginManager *plugin.Manager

// pluginSSEBroker is the global SSE broker for plugin events.
var pluginSSEBroker *plugin.SSEBroker

// SetPluginManager sets the global plugin manager and wires up the lazy-load callback
// so MCP tools and dynamic routes are refreshed when a plugin is loaded on demand.
func SetPluginManager(mgr *plugin.Manager) {
	pluginManager = mgr
	if mgr != nil {
		mgr.OnPluginLoaded = func() {
			RebuildDynamicEngine()
			RefreshPluginMCPTools()
		}
	}
}

// GetPluginManager returns the global plugin manager.
func GetPluginManager() *plugin.Manager {
	return pluginManager
}

// SetPluginSSEBroker sets the global SSE broker for plugin channel endpoints.
func SetPluginSSEBroker(b *plugin.SSEBroker) {
	pluginSSEBroker = b
}

// HandlePluginList returns all registered plugins.
// GET /api/v1/plugins
func HandlePluginList(c *gin.Context) {
	if pluginManager == nil {
		c.JSON(http.StatusOK, gin.H{"plugins": []any{}})
		return
	}

	manifests := pluginManager.List()

	// Build response with enabled status
	loadedNames := make(map[string]bool)
	plugins := make([]map[string]any, 0, len(manifests))
	for _, m := range manifests {
		loadedNames[m.Name] = true
		plugins = append(plugins, map[string]any{
			"name":        m.Name,
			"version":     m.Version,
			"description": m.Description,
			"author":      m.Author,
			"license":     m.License,
			"routes":      m.Routes,
			"widgets":     m.Widgets,
			"jobs":        m.Jobs,
			"menuItems":   m.MenuItems,
			"enabled":     pluginManager.IsEnabled(m.Name),
			"loaded":      true,
		})
	}

	// Add discovered but not loaded plugins (lazy loading)
	for _, name := range pluginManager.Discovered() {
		if loadedNames[name] {
			continue
		}
		plugins = append(plugins, map[string]any{
			"name":        name,
			"version":     "",
			"description": "Not loaded (lazy loading enabled)",
			"loaded":      false,
			"enabled":     false,
		})
	}

	c.JSON(http.StatusOK, gin.H{"plugins": plugins})
}

// HandlePluginCall invokes a plugin function.
// POST /api/v1/plugins/:name/call/:fn
func HandlePluginCall(c *gin.Context) {
	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Plugin system not initialized"})
		return
	}

	pluginName := c.Param("name")
	fnName := c.Param("fn")

	// Read request body as args
	var args json.RawMessage
	if err := c.ShouldBindJSON(&args); err != nil {
		args = nil
	}

	// Inject org context into params unless the plugin opts out.
	if !pluginManager.SkipsOrgInjection(pluginName) {
		if orgID := orgIDFromContext(c); orgID != 0 {
			args = injectOrgID(args, orgID)
		}
	}

	result, err := pluginManager.Call(c.Request.Context(), pluginName, fnName, args)
	if err != nil {
		// Return 404 for plugin not found errors
		var notFoundErr *plugin.PluginNotFoundError
		if errors.As(err, &notFoundErr) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		// Return 403 for disabled plugin errors
		var disabledErr *plugin.PluginDisabledError
		if errors.As(err, &disabledErr) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Return raw JSON result
	c.Data(http.StatusOK, "application/json", result)
}

// HandlePluginHealth returns the live health snapshot for every loaded
// plugin. Used by the admin UI's plugin-health widget to show which
// plugins are healthy, in restart backoff, or abandoned. Includes the
// optional rich payload that plugins return from their __health_ping__
// handler.
// GET /api/v1/plugins/health
func HandlePluginHealth(c *gin.Context) {
	if pluginManager == nil {
		c.JSON(http.StatusOK, gin.H{"plugins": map[string]any{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"plugins": pluginManager.AllHealthStatuses()})
}

// HandlePluginResetCrashLoop clears the crash-loop-abandoned flag for a
// plugin so auto-recovery can resume. Called by the admin UI after the
// operator has fixed the underlying problem (replaced a broken binary,
// patched a config error, etc.).
// POST /api/v1/plugins/:name/reset-crashloop
func HandlePluginResetCrashLoop(c *gin.Context) {
	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Plugin system not initialized"})
		return
	}
	name := c.Param("name")
	if !pluginManager.ResetCrashLoop(name) {
		c.JSON(http.StatusNotFound, gin.H{"error": "plugin not found"})
		return
	}
	plugin.GetLogBuffer().Log(name, "info", "Crash-loop guard reset by admin", nil)
	c.JSON(http.StatusOK, gin.H{"status": "reset"})
}

// HandlePluginEnable enables a plugin.
// POST /api/v1/plugins/:name/enable
func HandlePluginEnable(c *gin.Context) {
	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Plugin system not initialized"})
		return
	}

	name := c.Param("name")
	if err := pluginManager.Enable(name); err != nil {
		plugin.GetLogBuffer().Log(name, "error", fmt.Sprintf("Failed to enable plugin: %s", err.Error()), nil)
		// Return 404 for plugin not found errors
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "not registered") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	plugin.GetLogBuffer().Log(name, "info", fmt.Sprintf("Plugin enabled: %s", name), nil)
	RefreshPluginMCPTools()
	c.JSON(http.StatusOK, gin.H{"status": "enabled"})
}

// HandlePluginDisable disables a plugin.
// POST /api/v1/plugins/:name/disable
func HandlePluginDisable(c *gin.Context) {
	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Plugin system not initialized"})
		return
	}

	name := c.Param("name")
	if err := pluginManager.Disable(name); err != nil {
		plugin.GetLogBuffer().Log(name, "error", fmt.Sprintf("Failed to disable plugin: %s", err.Error()), nil)
		// Return 404 for plugin not found errors
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "not registered") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	plugin.GetLogBuffer().Log(name, "info", fmt.Sprintf("Plugin disabled: %s", name), nil)
	RefreshPluginMCPTools()
	c.JSON(http.StatusOK, gin.H{"status": "disabled"})
}

// HandlePluginWidgetList returns available widgets for a location (triggers lazy load).
// GET /api/v1/plugins/widgets?location=dashboard
func HandlePluginWidgetList(c *gin.Context) {
	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Plugin system not initialized"})
		return
	}

	location := c.DefaultQuery("location", "dashboard")

	// This triggers lazy loading for all discovered plugins
	widgets := pluginManager.AllWidgets(location)

	type widgetInfo struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		PluginName  string `json:"plugin_name"`
		Size        string `json:"size"`
		Refreshable bool   `json:"refreshable"`
		RefreshSec  int    `json:"refresh_sec,omitempty"`
	}

	result := make([]widgetInfo, 0, len(widgets))
	for _, w := range widgets {
		result = append(result, widgetInfo{
			ID:          w.ID,
			Title:       w.Title,
			PluginName:  w.PluginName,
			Size:        w.Size,
			Refreshable: w.Refreshable,
			RefreshSec:  w.RefreshSec,
		})
	}

	c.JSON(http.StatusOK, gin.H{"widgets": result})
}

// HandlePluginWidget returns a specific widget's HTML.
// GET /api/v1/plugins/:name/widgets/:id
// This triggers lazy loading if needed, making it HTMX-friendly.
func HandlePluginWidget(c *gin.Context) {
	if pluginManager == nil {
		c.String(http.StatusServiceUnavailable, "Plugin system not initialized")
		return
	}

	pluginName := c.Param("name")
	widgetID := c.Param("id")

	// Get plugin (triggers lazy load via Call if needed)
	p, ok := pluginManager.Get(pluginName)
	if !ok {
		c.String(http.StatusNotFound, "Plugin not found: %s", pluginName)
		return
	}

	// Get manifest and find the widget spec
	manifest := p.GKRegister()
	var widgetHandler string
	var widgetTitle string
	for _, w := range manifest.Widgets {
		if w.ID == widgetID {
			widgetHandler = w.Handler
			widgetTitle = w.Title
			break
		}
	}
	if widgetHandler == "" {
		c.String(http.StatusNotFound, "Widget not found: %s/%s", pluginName, widgetID)
		return
	}

	// Call the widget handler (pass empty JSON object, not nil)
	ctx := pluginContextWithLanguage(c)
	result, err := pluginManager.Call(ctx, pluginName, widgetHandler, []byte("{}"))
	if err != nil {
		c.String(http.StatusInternalServerError, "Widget error: %v", err)
		return
	}

	var data struct {
		HTML string `json:"html"`
	}
	if err := json.Unmarshal(result, &data); err != nil {
		c.String(http.StatusInternalServerError, "Invalid widget response")
		return
	}

	// Return HTML with optional wrapper for HTMX
	if c.Query("wrap") == "true" {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, `<div class="gk-card-header"><h3 class="gk-card-title">%s</h3></div><div class="gk-card-body">%s</div>`, widgetTitle, data.HTML)
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, data.HTML)
}

// GetPluginWidgets returns rendered widgets for a dashboard location.
// Used by dashboard handlers to include plugin widgets.
// Pass a gin.Context to enable i18n and RBAC support in widgets.
func GetPluginWidgets(ctx context.Context, location string, ginCtx ...*gin.Context) []PluginWidgetData {
	if pluginManager == nil {
		log.Printf("🔌 GetPluginWidgets: pluginManager is nil!")
		return nil
	}

	// Build widget args with RBAC context if gin context is available
	widgetArgs := []byte("{}")
	if len(ginCtx) > 0 && ginCtx[0] != nil {
		c := ginCtx[0]
		argsMap := map[string]any{}

		if val, exists := c.Get("is_queue_admin"); exists {
			argsMap["is_queue_admin"] = val
		}
		if val, exists := c.Get("accessible_queue_ids"); exists {
			argsMap["accessible_queue_ids"] = val
		}

		if len(argsMap) > 0 {
			if data, err := json.Marshal(argsMap); err == nil {
				widgetArgs = data
			}
		}
	}

	// Use AllWidgets to trigger lazy loading of discovered plugins
	widgets := pluginManager.AllWidgets(location)
	log.Printf("🔌 GetPluginWidgets(%s): found %d widgets from manager", location, len(widgets))
	results := make([]PluginWidgetData, 0, len(widgets))

	for _, w := range widgets {
		// Call the widget handler to get HTML (ctx should already have language if from gin)
		result, err := pluginManager.Call(ctx, w.PluginName, w.Handler, widgetArgs)
		if err != nil {
			log.Printf("🔌 Widget %s:%s call failed: %v", w.PluginName, w.Handler, err)
			continue
		}

		var data struct {
			HTML string `json:"html"`
		}
		if err := json.Unmarshal(result, &data); err != nil {
			log.Printf("🔌 Widget %s:%s unmarshal failed: %v (raw: %s)", w.PluginName, w.Handler, err, string(result))
			continue
		}

		results = append(results, PluginWidgetData{
			ID:          w.ID,
			Title:       w.Title,
			PluginName:  w.PluginName,
			HTML:        data.HTML,
			Size:        w.Size,
			Refreshable: w.Refreshable,
			RefreshSec:  w.RefreshSec,
		})
	}

	return results
}

// PluginWidgetData is the rendered widget data for templates.
type PluginWidgetData struct {
	ID          string
	Title       string
	PluginName  string
	HTML        string
	Size        string
	Refreshable bool
	RefreshSec  int
	GridX       int
	GridY       int
	GridW       int
	GridH       int
}

// GetPluginMenuItems returns menu items for a location.
func GetPluginMenuItems(location string) []plugin.PluginMenuItem {
	if pluginManager == nil {
		return nil
	}
	return pluginManager.MenuItems(location)
}

// RegisterPluginRoutes is a no-op kept for backwards compatibility.
// Plugin routes are now handled by the unified dynamic engine (MountDynamicEngine).
// Deprecated: Use MountDynamicEngine instead.
func RegisterPluginRoutes(r *gin.Engine) int {
	return 0
}

// buildPluginArgs extracts request data into JSON args for the plugin.
func buildPluginArgs(c *gin.Context, pluginName ...string) json.RawMessage {
	args := make(map[string]any)

	// URL parameters
	for _, param := range c.Params {
		args[param.Key] = param.Value
	}

	// Query parameters
	for key, values := range c.Request.URL.Query() {
		if len(values) == 1 {
			args[key] = values[0]
		} else {
			args[key] = values
		}
	}

	// Request body (if present)
	if c.Request.Body != nil && c.Request.ContentLength > 0 {
		var body map[string]any
		if err := c.ShouldBindJSON(&body); err == nil {
			for k, v := range body {
				args[k] = v
			}
		}
	}

	// Include request metadata
	args["_method"] = c.Request.Method
	args["_path"] = c.Request.URL.Path

	// Include authenticated user context
	if userID, exists := c.Get("user_id"); exists {
		args["_user_id"] = userID
	}
	if email, exists := c.Get("user_email"); exists {
		args["_user_email"] = email
	}
	// _user_login is the canonical login. Different middlewares store it
	// under different keys (`user_login`, `username`); fall through to
	// the email if neither is set so the plugin always has something.
	if login, exists := c.Get("user_login"); exists {
		args["_user_login"] = login
	} else if username, exists := c.Get("username"); exists {
		args["_user_login"] = username
	} else if email, exists := c.Get("user_email"); exists {
		args["_user_login"] = email
	}
	if customerLogin, exists := c.Get("customer_login"); exists {
		args["_customer_login"] = customerLogin
	}
	if role, exists := c.Get("user_role"); exists {
		args["_user_role"] = role
	}
	if isAdmin, exists := c.Get("isInAdminGroup"); exists {
		args["_is_admin"] = isAdmin
	}

	// Inject org context from the authenticated session unless the plugin opts out.
	// Use the cookie-aware helper rather than OrgIDFromContext(ctx) because
	// no middleware currently calls WithOrgID on the request context for plugin
	// routes. _org_id keeps the legacy key; org_id matches the JSON field name
	// plugins already unmarshal, so plain-request-struct handlers get it for free.
	skipOrg := len(pluginName) > 0 && pluginManager != nil && pluginManager.SkipsOrgInjection(pluginName[0])
	if !skipOrg {
		if orgID := orgIDFromContext(c); orgID != 0 {
			args["_org_id"] = orgID
			args["org_id"] = orgID
		}
	}

	result, _ := json.Marshal(args)
	return result
}

// injectOrgID merges org_id / _org_id into a JSON args object. If args is nil
// or not a JSON object, it creates a new object with those keys.
func injectOrgID(args json.RawMessage, orgID int64) json.RawMessage {
	m := make(map[string]json.RawMessage)
	if len(args) > 0 {
		_ = json.Unmarshal(args, &m)
	}
	idBytes, _ := json.Marshal(orgID)
	m["_org_id"] = idBytes
	m["org_id"] = idBytes
	out, _ := json.Marshal(m)
	return out
}

// RegisterPluginAPIRoutes registers the plugin management API endpoints.
// GET  /api/v1/plugins                    - List all plugins (authenticated)
// POST /api/v1/plugins/:name/call/:fn     - Call a plugin function (authenticated)
// GET  /api/v1/plugins/:name/widgets/:id  - Get widget HTML (authenticated, HTMX-friendly)
// POST /api/v1/plugins/:name/enable       - Enable a plugin (admin only)
// POST /api/v1/plugins/:name/disable      - Disable a plugin (admin only)
// SessionOrJWTAuth middleware accepts either session-based auth (cookie) or JWT token auth.
// Session auth is checked first (user_id already set by session middleware), then falls back to JWT.
func SessionOrJWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// A prior middleware may have already authenticated (e.g. the
		// YAML session middleware). In that case user_id is set but the
		// customer-branch keys RequirePluginAccess needs
		// (customer_login, username, is_customer) may not be — those
		// were added for this refactor and older middlewares don't know
		// about them. Extract the JWT separately and backfill the keys
		// before continuing, so plugin routes never 403 a real customer
		// with "identity missing".
		if _, exists := c.Get("user_id"); exists {
			enrichContextFromToken(c)
			c.Next()
			return
		}

		// Extract token from cookies or Authorization header and validate.
		token := ExtractToken(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Authentication required"})
			c.Abort()
			return
		}

		jwtMgr := getJWTManager()
		if jwtMgr == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Auth service unavailable"})
			c.Abort()
			return
		}

		claims, err := jwtMgr.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Invalid or expired token"})
			c.Abort()
			return
		}

		setAuthContextFromClaims(c, claims)
		c.Next()
	}
}

// setAuthContextFromClaims sets the full set of auth-related context keys
// used across the platform. Mirrored by enrichContextFromToken for the
// "prior middleware already authenticated" path.
func setAuthContextFromClaims(c *gin.Context, claims *auth.Claims) {
	c.Set("user_id", int(claims.UserID))
	c.Set("user_email", claims.Email)
	c.Set("user_role", claims.Role)
	c.Set("claims", claims)
	c.Set("isInAdminGroup", claims.IsAdmin)
	c.Set("username", claims.Login)
	c.Set("is_customer", claims.Role == "Customer")
	if claims.Role == "Customer" {
		c.Set("customer_login", claims.Login)
	}
}

// enrichContextFromToken backfills customer_login / username / is_customer
// when an earlier middleware authenticated the request but didn't set
// them. Safe to call repeatedly — every c.Set is a no-op overwrite.
func enrichContextFromToken(c *gin.Context) {
	// Prefer claims already on the context. Fall back to extracting the
	// token fresh if the earlier middleware didn't stash them.
	var claims *auth.Claims
	if v, ok := c.Get("claims"); ok {
		if cl, ok := v.(*auth.Claims); ok {
			claims = cl
		}
	}
	if claims == nil {
		if token := ExtractToken(c); token != "" {
			if mgr := getJWTManager(); mgr != nil {
				if cl, err := mgr.ValidateToken(token); err == nil {
					claims = cl
				}
			}
		}
	}
	if claims == nil {
		return
	}
	// Only fill in keys that are missing so we don't clobber a prior
	// middleware's (possibly richer) setting.
	if _, ok := c.Get("username"); !ok {
		c.Set("username", claims.Login)
	}
	if _, ok := c.Get("is_customer"); !ok {
		c.Set("is_customer", claims.Role == "Customer")
	}
	if claims.Role == "Customer" {
		if _, ok := c.Get("customer_login"); !ok {
			c.Set("customer_login", claims.Login)
		}
	}
}

// HandlePluginSSEChannel serves an SSE stream scoped to a specific plugin and channel.
// GET /api/v1/plugins/:name/events/:channel
// Auth-scoped: requires a valid session or JWT token.
func HandlePluginSSEChannel(c *gin.Context) {
	if pluginSSEBroker == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "SSE not available"})
		return
	}
	pluginName := c.Param("name")
	channel := c.Param("channel")

	// Validate plugin exists
	if pluginManager != nil {
		if _, ok := pluginManager.Get(pluginName); !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "plugin not found"})
			return
		}
	}

	pluginSSEBroker.ServeChannel(c.Writer, c.Request, pluginName, channel)
}

func RegisterPluginAPIRoutes(r *gin.RouterGroup) {
	// Plugin list and call - require authentication (session or JWT)
	plugins := r.Group("/plugins")
	plugins.Use(SessionOrJWTAuth())
	{
		plugins.GET("", HandlePluginList)
		plugins.GET("/health", HandlePluginHealth)
		plugins.POST("/:name/call/:fn", HandlePluginCall)
		plugins.GET("/widgets", HandlePluginWidgetList)
		plugins.GET("/:name/widgets/:id", HandlePluginWidget)
		// Per-plugin SSE channel endpoint (auth-scoped)
		plugins.GET("/:name/events/:channel", HandlePluginSSEChannel)
	}

	// Plugin management - require admin (session or JWT)
	pluginAdmin := r.Group("/plugins")
	pluginAdmin.Use(SessionOrJWTAuth(), RequireAdmin())
	{
		pluginAdmin.POST("/:name/enable", HandlePluginEnable)
		pluginAdmin.POST("/:name/disable", HandlePluginDisable)
		pluginAdmin.POST("/:name/reset-crashloop", HandlePluginResetCrashLoop)
		pluginAdmin.POST("/upload", HandlePluginUpload)
		pluginAdmin.GET("/logs", HandlePluginLogs)
		pluginAdmin.DELETE("/logs", HandleClearPluginLogs)
	}

	pluginUIAdmin := r.Group("/plugin-uis")
	pluginUIAdmin.Use(SessionOrJWTAuth(), RequireAdmin())
	{
		pluginUIAdmin.GET("", HandlePluginUIAdminList)
		pluginUIAdmin.PUT("/:id", HandlePluginUIAdminUpdate)
		pluginUIAdmin.POST("/:id/toggle", HandlePluginUIAdminToggle)
	}
}

// RequireAdmin middleware checks if the user is an admin.
// Supports both session-based auth (user_role) and JWT auth (isInAdminGroup).
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check session-based auth (user_role set by YAML route middleware)
		if role, exists := c.Get("user_role"); exists && role == "Admin" {
			c.Next()
			return
		}
		// Check JWT-based auth (isInAdminGroup set by JWT/API token middleware)
		if isAdmin, exists := c.Get("isInAdminGroup"); exists && isAdmin == true {
			c.Next()
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		c.Abort()
	}
}

// RequireGroup checks that the authenticated user belongs to a specific group.
// Admin users (role=Admin or isInAdminGroup) bypass group checks.
// Used by plugins via "group:<name>" middleware, e.g. "group:myplugin-users".
func RequireGroup(groupName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Admin users bypass group checks.
		if role, exists := c.Get("user_role"); exists && role == "Admin" {
			c.Next()
			return
		}
		if isAdmin, exists := c.Get("isInAdminGroup"); exists && isAdmin == true {
			c.Next()
			return
		}

		userIDRaw, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "authentication required"})
			c.Abort()
			return
		}

		// Convert user_id to uint (may be stored as int, uint, float64, or string).
		var userID uint
		switch v := userIDRaw.(type) {
		case uint:
			userID = v
		case int:
			userID = uint(v)
		case int64:
			userID = uint(v)
		case float64:
			userID = uint(v)
		default:
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid user identity"})
			c.Abort()
			return
		}

		db, err := database.GetDB()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database unavailable"})
			c.Abort()
			return
		}

		groupRepo := repository.NewGroupRepository(db)
		groups, err := groupRepo.GetUserGroups(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check group membership"})
			c.Abort()
			return
		}

		for _, g := range groups {
			if g == groupName {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "access denied: requires group " + groupName})
		c.Abort()
	}
}

// RequirePluginAccess gates a plugin route. Admins bypass; customers
// pass when the active org has a gk_org_plugin_access binding for
// pluginName and the customer is in one of those groups; agents pass
// when they're a member of agentGroup.
func RequirePluginAccess(pluginName, agentGroup string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Platform admin bypass.
		if role, exists := c.Get("user_role"); exists && role == "Admin" {
			c.Next()
			return
		}
		if isAdmin, exists := c.Get("isInAdminGroup"); exists && isAdmin == true {
			c.Next()
			return
		}

		db, err := database.GetDB()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database unavailable"})
			c.Abort()
			return
		}

		role, _ := c.Get("user_role")

		// Customer branch.
		if role == "Customer" {
			var login string
			if v, ok := c.Get("customer_login"); ok {
				login, _ = v.(string)
			}
			if login == "" {
				if v, ok := c.Get("username"); ok {
					login, _ = v.(string)
				}
			}
			if login == "" {
				c.JSON(http.StatusForbidden, gin.H{"error": "customer identity missing"})
				c.Abort()
				return
			}
			orgID := orgIDFromContext(c)
			if orgID == 0 {
				orgID = primaryOrgForCustomer(db, login)
			}
			if orgID == 0 {
				c.JSON(http.StatusForbidden, gin.H{"error": "no organisation membership for this customer"})
				c.Abort()
				return
			}
			accessRepo := repository.NewPluginAccessRepository(db)
			ok, err := accessRepo.HasCustomerAccess(orgID, pluginName, login)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check plugin access"})
				c.Abort()
				return
			}
			if !ok {
				c.JSON(http.StatusForbidden, gin.H{"error": pluginName + " is not enabled for your organisation"})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		// Agent branch — require membership in the plugin's agent group.
		userIDRaw, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "authentication required"})
			c.Abort()
			return
		}
		var userID uint
		switch v := userIDRaw.(type) {
		case uint:
			userID = v
		case int:
			userID = uint(v)
		case int64:
			userID = uint(v)
		case float64:
			userID = uint(v)
		default:
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid user identity"})
			c.Abort()
			return
		}
		groupRepo := repository.NewGroupRepository(db)
		groups, err := groupRepo.GetUserGroups(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check group membership"})
			c.Abort()
			return
		}
		for _, g := range groups {
			if g == agentGroup {
				c.Next()
				return
			}
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied: requires group " + agentGroup})
		c.Abort()
	}
}

// HasPluginAccess returns true iff the caller is entitled to use
// pluginName. Read-only equivalent of RequirePluginAccess, safe to call
// from template rendering.
func HasPluginAccess(c *gin.Context, pluginName string) bool {
	if role, exists := c.Get("user_role"); exists && role == "Admin" {
		return true
	}
	if isAdmin, exists := c.Get("isInAdminGroup"); exists && isAdmin == true {
		return true
	}

	db, err := database.GetDB()
	if err != nil {
		return false
	}

	role, _ := c.Get("user_role")
	if role == "Customer" {
		var login string
		if v, ok := c.Get("customer_login"); ok {
			login, _ = v.(string)
		}
		if login == "" {
			if v, ok := c.Get("username"); ok {
				login, _ = v.(string)
			}
		}
		if login == "" {
			return false
		}
		orgID := orgIDFromContext(c)
		if orgID == 0 {
			orgID = primaryOrgForCustomer(db, login)
		}
		if orgID == 0 {
			return false
		}
		ok, err := repository.NewPluginAccessRepository(db).HasCustomerAccess(orgID, pluginName, login)
		return err == nil && ok
	}

	// Agents default to allow — the route itself enforces group
	// membership on click, and menu items don't carry the agent-group
	// metadata needed to filter them here yet. Revisit once menu items
	// carry the gate group alongside PluginName.
	return true
}

func orgIDFromContext(c *gin.Context) int64 {
	for _, k := range []string{"active_org_id", "org_id", "orgID"} {
		if v, ok := c.Get(k); ok {
			switch n := v.(type) {
			case int64:
				return n
			case int:
				return int64(n)
			case uint:
				return int64(n)
			case float64:
				return int64(n)
			}
		}
	}
	if orgID := organisation.OrgIDFromContext(c.Request.Context()); orgID != 0 {
		return orgID
	}
	if cookie, err := c.Cookie("active_org_id"); err == nil && cookie != "" {
		var n int64
		_, _ = fmt.Sscan(cookie, &n)
		return n
	}
	return 0
}

// maxWebhookBodySize is the maximum request body size for webhook endpoints (1MB).
const maxWebhookBodySize = 1 << 20

// webhookRequestsPerHour is the default rate limit for webhook endpoints per source IP per plugin.
const webhookRequestsPerHour = 500

// WebhookRateLimit applies IP-based rate limiting scoped to the plugin name.
// This runs BEFORE signature verification to reject floods cheaply.
func WebhookRateLimit(pluginName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := "webhook:" + pluginName + ":" + c.ClientIP()
		if !middleware.GlobalRateLimiter().Allow(key, webhookRequestsPerHour) {
			log.Printf("⚠️  Webhook %s: rate limited %s", pluginName, c.ClientIP())
			c.Header("Retry-After", "60")
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// WebhookAuth provides authentication for webhook endpoints.
// No session/JWT required — instead, verifies HMAC-SHA256 signature from the
// request header. The signing secret is stored in the plugin's secure config
// under the key "<plugin>_webhook_secret".
//
// If no webhook secret is configured, requests are REJECTED by default.
// Set GOATFLOW_WEBHOOK_ALLOW_UNSIGNED=true to allow unsigned webhooks
// during development (NOT recommended for production).
//
// Signature verification supports:
//   - Standard HMAC: X-Signature-256, X-Hub-Signature-256, X-Webhook-Signature
//     (format: "sha256=<hex>" or plain "<hex>")
//   - Stripe: Stripe-Signature header (format: "t=<timestamp>,v1=<signature>")
//     Stripe signatures are computed as HMAC-SHA256(secret, "<timestamp>.<body>")
func WebhookAuth(pluginName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("🔔 Webhook: %s %s from %s (plugin: %s)",
			c.Request.Method, c.Request.URL.Path, c.ClientIP(), pluginName)

		// Read the webhook secret from secure config.
		secretKey := pluginName + "_webhook_secret"
		secret := ""
		if pluginManager != nil {
			if host := pluginManager.Host(); host != nil {
				secret, _ = host.SecureConfigGet(c.Request.Context(), secretKey)
			}
		}

		if secret == "" {
			// No secret configured — reject unless explicitly allowed.
			if os.Getenv("GOATFLOW_WEBHOOK_ALLOW_UNSIGNED") == "true" {
				log.Printf("⚠️  Webhook %s: no signing secret — allowing (GOATFLOW_WEBHOOK_ALLOW_UNSIGNED=true)",
					pluginName)
				c.Next()
				return
			}
			log.Printf("❌ Webhook %s: no signing secret configured (%s) — rejecting", pluginName, secretKey)
			c.JSON(http.StatusForbidden, gin.H{"error": "webhook not configured"})
			c.Abort()
			return
		}

		// Check for signature header.
		sigHeader := ""
		sigValue := ""
		for _, hdr := range []string{"Stripe-Signature", "X-Signature-256", "X-Hub-Signature-256", "X-Webhook-Signature"} {
			if v := c.GetHeader(hdr); v != "" {
				sigHeader = hdr
				sigValue = v
				break
			}
		}

		if sigValue == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "webhook signature required"})
			c.Abort()
			return
		}

		// Read body with size limit to prevent OOM.
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxWebhookBodySize+1))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
			c.Abort()
			return
		}
		if int64(len(body)) > maxWebhookBodySize {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "webhook body too large"})
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		// Verify signature based on header type.
		var verified bool
		if sigHeader == "Stripe-Signature" {
			verified = verifyStripeSignature(body, sigValue, secret)
		} else {
			verified = verifyHMACSignature(body, sigValue, secret)
		}

		if !verified {
			log.Printf("❌ Webhook %s: signature mismatch from %s", pluginName, c.ClientIP())
			c.JSON(http.StatusForbidden, gin.H{"error": "webhook signature required"})
			c.Abort()
			return
		}

		log.Printf("✅ Webhook %s: verified from %s", pluginName, c.ClientIP())
		c.Next()
	}
}

// verifyHMACSignature verifies a standard HMAC-SHA256 signature.
// Accepts formats: "sha256=<hex>", "<hex>".
func verifyHMACSignature(body []byte, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	// Strip "sha256=" or similar prefix.
	cleanSig := signature
	if idx := strings.Index(cleanSig, "="); idx > 0 && idx < 10 {
		cleanSig = cleanSig[idx+1:]
	}

	return hmac.Equal([]byte(expected), []byte(strings.ToLower(cleanSig)))
}

// verifyStripeSignature verifies Stripe's webhook signature format.
// Header format: "t=<timestamp>,v1=<signature>"
// Signed payload: "<timestamp>.<body>"
// Rejects signatures older than 5 minutes to prevent replay attacks.
func verifyStripeSignature(body []byte, header, secret string) bool {
	var timestamp, sig string
	for _, part := range strings.Split(header, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			timestamp = kv[1]
		case "v1":
			sig = kv[1]
		}
	}

	if timestamp == "" || sig == "" {
		return false
	}

	// Replay protection: reject signatures older than 5 minutes.
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix()-ts > 300 {
		log.Printf("⚠️  Stripe webhook: signature too old (%ds)", time.Now().Unix()-ts)
		return false
	}

	// Stripe signs: "<timestamp>.<body>"
	signedPayload := timestamp + "." + string(body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(strings.ToLower(sig)))
}

// EnsurePluginGroups auto-creates groups declared by plugins in their GKRegistration.
// Called after plugin loading to ensure the groups exist in the GoatFlow groups table.
// Idempotent — skips groups that already exist.
func EnsurePluginGroups() {
	if pluginManager == nil {
		return
	}
	db, err := database.GetDB()
	if err != nil {
		log.Printf("⚠️  Cannot ensure plugin groups: database unavailable")
		return
	}
	groupRepo := repository.NewGroupRepository(db)

	for _, manifest := range pluginManager.List() {
		for _, gs := range manifest.Groups {
			if gs.Name == "" {
				continue
			}
			existing, _ := groupRepo.GetByName(gs.Name)
			if existing != nil {
				continue // already exists
			}
			group := &models.Group{
				Name:     gs.Name,
				Comments: gs.Description,
				ValidID:  1, // active
				CreateBy: 1, // system user
				ChangeBy: 1,
			}
			if err := groupRepo.Create(group); err != nil {
				log.Printf("⚠️  Failed to create plugin group %q: %v", gs.Name, err)
			} else {
				log.Printf("✅ Auto-created plugin group %q (%s)", gs.Name, gs.Description)
			}
		}
	}
}

// pluginDir is the directory where plugins are stored.
// Set via SetPluginDir during app initialization.
var pluginDir string

// pluginReloader is called after a plugin is uploaded to trigger a load/reload.
var pluginReloader func(ctx context.Context, name string) error

// pluginUnloader is called before overwriting a gRPC plugin binary to stop the running process.
var pluginUnloader func(ctx context.Context, name string) error

// SetPluginDir sets the plugin directory for uploads.
func SetPluginDir(dir string) {
	pluginDir = dir
}

// SetPluginReloader sets the callback used to load/reload a plugin after upload.
func SetPluginReloader(fn func(ctx context.Context, name string) error) {
	pluginReloader = fn
}

// SetPluginUnloader sets the callback used to stop a running plugin before binary replacement.
func SetPluginUnloader(fn func(ctx context.Context, name string) error) {
	pluginUnloader = fn
}

// HandlePluginUpload handles uploading a new WASM plugin.
// POST /api/v1/plugins/upload
func HandlePluginUpload(c *gin.Context) {
	if pluginDir == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Plugin directory not configured"})
		return
	}

	// Get uploaded file
	file, header, err := c.Request.FormFile("plugin")
	if err != nil {
		plugin.GetLogBuffer().Log("system", "error", fmt.Sprintf("Plugin upload failed: %s", err.Error()), nil)
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}
	defer file.Close()

	// Validate file extension
	lowerName := strings.ToLower(header.Filename)
	isWasm := strings.HasSuffix(lowerName, ".wasm")
	isZip := strings.HasSuffix(lowerName, ".zip")

	if !isWasm && !isZip {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only .wasm and .zip files are allowed"})
		return
	}

	// Sanitize filename
	filename := filepath.Base(header.Filename)
	if filename == "" || filename == "." || filename == ".." {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid filename"})
		return
	}

	// Ensure plugin directory exists
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create plugin directory"})
		return
	}

	// Save uploaded file to temp location first
	tempPath := filepath.Join(pluginDir, ".upload_"+filename)
	dest, err := os.Create(tempPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp file"})
		return
	}

	// Copy file content
	if _, err := io.Copy(dest, file); err != nil {
		dest.Close()
		os.Remove(tempPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save plugin file"})
		return
	}
	dest.Close()

	var pluginName string
	var destPath string

	if isZip {
		// Validate the package first without extracting to the live directory.
		manifest, err := packaging.ValidatePackage(tempPath)
		if err != nil {
			os.Remove(tempPath)
			plugin.GetLogBuffer().Log("system", "error", fmt.Sprintf("Plugin upload failed: invalid package: %s", err.Error()), nil)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid plugin package: " + err.Error()})
			return
		}
		pluginName = manifest.Name

		// For gRPC plugins, the binary may be running. Unload it first to
		// release the file handle so extraction can overwrite it.
		if pluginUnloader != nil && strings.ToLower(manifest.Runtime) == "grpc" {
			log.Printf("🔌 Unloading running plugin %s before binary replacement...", pluginName)
			if err := pluginUnloader(context.Background(), pluginName); err != nil {
				log.Printf("⚠️  Pre-upload unload of %s failed (may be first install): %v", pluginName, err)
			} else {
				log.Printf("🔌 Plugin %s unloaded, waiting for process exit...", pluginName)
			}
			// Wait for the OS to release the file handle after process exit.
			time.Sleep(2 * time.Second)
		}

		// Now extract — the binary file should no longer be locked.
		pkg, err := packaging.ExtractPlugin(tempPath, pluginDir)
		os.Remove(tempPath)
		if err != nil {
			plugin.GetLogBuffer().Log("system", "error", fmt.Sprintf("Plugin upload failed: %s", err.Error()), nil)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid plugin package: " + err.Error()})
			return
		}
		destPath = pkg.BinaryPath
		runtimeType := pkg.RuntimeType
		log.Printf("🔌 Plugin package extracted: %s v%s (runtime: %s)", pluginName, pkg.Manifest.Version, runtimeType)
		plugin.GetLogBuffer().Log(pluginName, "info", fmt.Sprintf("Plugin uploaded: %s (runtime: %s, size: %d bytes)", pluginName, runtimeType, header.Size), nil)

		// Trigger load/reload of the uploaded plugin
		if pluginReloader != nil {
			go func() {
				if err := pluginReloader(context.Background(), pluginName); err != nil {
					log.Printf("⚠️  Plugin reload failed for %s: %v", pluginName, err)
					plugin.GetLogBuffer().Log(pluginName, "error", fmt.Sprintf("Reload failed: %v", err), nil)
				} else {
					log.Printf("✅ Plugin %s loaded/reloaded after upload", pluginName)
					plugin.GetLogBuffer().Log(pluginName, "info", "Plugin loaded/reloaded after upload", nil)
					// Rebuild dynamic engine to pick up new/changed routes
					RebuildDynamicEngine()
				}
			}()
		}

		RefreshPluginMCPTools()
		c.JSON(http.StatusOK, gin.H{
			"message": "Plugin uploaded successfully",
			"name":    pluginName,
			"path":    destPath,
			"runtime": runtimeType,
		})
		return
	} else {
		// Direct WASM upload
		destPath = filepath.Join(pluginDir, filename)
		if err := os.Rename(tempPath, destPath); err != nil {
			os.Remove(tempPath)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save plugin file"})
			return
		}
		pluginName = strings.TrimSuffix(filename, ".wasm")
		log.Printf("🔌 Plugin uploaded: %s", pluginName)
		plugin.GetLogBuffer().Log(pluginName, "info", fmt.Sprintf("Plugin uploaded: %s (runtime: wasm, size: %d bytes)", pluginName, header.Size), nil)
	}

	RefreshPluginMCPTools()
	c.JSON(http.StatusOK, gin.H{
		"message": "Plugin uploaded successfully",
		"name":    pluginName,
		"path":    destPath,
	})
}

// HandlePluginLogs returns plugin log entries.
// GET /api/v1/plugins/logs?plugin=name&level=info&limit=100
func HandlePluginLogs(c *gin.Context) {
	logBuffer := plugin.GetLogBuffer()

	pluginName := c.Query("plugin")
	level := c.Query("level")
	limitStr := c.DefaultQuery("limit", "100")

	limit := 100
	if n, err := parseInt(limitStr); err == nil && n > 0 {
		limit = n
	}

	// Start with all entries, then filter
	var entries []plugin.LogEntry

	if pluginName != "" {
		entries = logBuffer.GetByPlugin(pluginName)
	} else {
		entries = logBuffer.GetRecent(limit)
	}

	// Apply level filter if specified
	if level != "" {
		filtered := make([]plugin.LogEntry, 0, len(entries))
		for _, e := range entries {
			if e.Level == level {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// Apply limit
	if len(entries) > limit {
		entries = entries[:limit]
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":  entries,
		"count": len(entries),
		"total": logBuffer.Count(),
	})
}

// HandleClearPluginLogs clears the plugin log buffer.
// DELETE /api/v1/plugins/logs
func HandleClearPluginLogs(c *gin.Context) {
	plugin.GetLogBuffer().Clear()
	c.JSON(http.StatusOK, gin.H{"message": "Plugin logs cleared"})
}

func parseInt(s string) (int, error) {
	var n int
	err := json.Unmarshal([]byte(s), &n)
	return n, err
}
