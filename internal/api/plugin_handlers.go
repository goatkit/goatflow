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
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/middleware"
	"github.com/goatkit/goatflow/internal/platform/organisation"
	"github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/goatkit/goatflow/internal/platform/plugin/packaging"
	"github.com/goatkit/goatflow/internal/platform/shared"
	"github.com/goatkit/goatflow/internal/repository"
)

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
			"icon":        m.Icon,
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

// HandlePluginCall invokes a plugin function directly, bypassing the
// plugin's own route middleware, so it is mounted admin-only.
// POST /api/v1/plugins/:name/call/:fn
func HandlePluginCall(c *gin.Context) {
	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Plugin system not initialized"})
		return
	}

	pluginName := c.Param("name")
	fnName := c.Param("fn")

	argsJSON, status, msg := pluginJSONBodyArgs(c, pluginName)
	if argsJSON == nil {
		c.JSON(status, gin.H{"error": msg})
		return
	}

	result, err := pluginManager.Call(c.Request.Context(), pluginName, fnName, argsJSON)
	if err != nil {
		writePluginCallError(c, err, http.StatusBadRequest)
		return
	}

	// Return raw JSON result
	c.Data(http.StatusOK, "application/json", result)
}

// pluginJSONBodyArgs builds the args of a plugin call whose request body is
// the function's args as an optional JSON object: the body's keys minus any
// client-supplied envelope key, plus the host envelope (caller identity,
// language, organisation). For an unreadable or non-object body it returns
// nil args with the HTTP status and message to answer with.
func pluginJSONBodyArgs(c *gin.Context, pluginName string) (json.RawMessage, int, string) {
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, maxPluginBodySize+1))
	if err != nil {
		return nil, http.StatusBadRequest, "failed to read request body"
	}
	if int64(len(raw)) > maxPluginBodySize {
		return nil, http.StatusRequestEntityTooLarge, "plugin request body too large"
	}
	args := make(map[string]any)
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, http.StatusBadRequest, "request body must be a JSON object"
		}
	}
	setPluginEnvelope(c, args, pluginName)
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, http.StatusBadRequest, err.Error()
	}
	return argsJSON, 0, ""
}

// writePluginCallError maps a plugin manager Call error to an HTTP response:
// unknown plugin 404, disabled plugin 403, anything else the fallback status.
func writePluginCallError(c *gin.Context, err error, fallback int) {
	var notFoundErr *plugin.PluginNotFoundError
	if errors.As(err, &notFoundErr) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	var disabledErr *plugin.PluginDisabledError
	if errors.As(err, &disabledErr) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	c.JSON(fallback, gin.H{"error": err.Error()})
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

	// Call the widget handler; the envelope carries the caller (user, org, language).
	widgetArgs := buildPluginArgs(c, pluginName)
	result, err := pluginManager.Call(c.Request.Context(), pluginName, widgetHandler, widgetArgs)
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
// Pass a gin.Context so each widget call carries the caller's envelope
// (identity, organisation, language) for i18n, RBAC and org scoping.
func GetPluginWidgets(ctx context.Context, location string, ginCtx ...*gin.Context) []PluginWidgetData {
	if pluginManager == nil {
		log.Printf("🔌 GetPluginWidgets: pluginManager is nil!")
		return nil
	}

	widgetArgs := func(pluginName string) []byte {
		if len(ginCtx) > 0 && ginCtx[0] != nil {
			return buildPluginArgs(ginCtx[0], pluginName)
		}
		return []byte("{}")
	}

	// Use AllWidgets to trigger lazy loading of discovered plugins
	widgets := pluginManager.AllWidgets(location)
	log.Printf("🔌 GetPluginWidgets(%s): found %d widgets from manager", location, len(widgets))
	results := make([]PluginWidgetData, 0, len(widgets))

	for _, w := range widgets {
		widget := PluginWidgetData{
			ID:          w.ID,
			Title:       w.Title,
			PluginName:  w.PluginName,
			Size:        w.Size,
			Refreshable: w.Refreshable,
			RefreshSec:  w.RefreshSec,
		}
		result, err := pluginManager.Call(ctx, w.PluginName, w.Handler, widgetArgs(w.PluginName))
		if err != nil {
			log.Printf("🔌 Widget %s:%s call failed: %v", w.PluginName, w.Handler, err)
			widget.Unavailable = true
			results = append(results, widget)
			continue
		}

		var data struct {
			HTML string `json:"html"`
		}
		if err := json.Unmarshal(result, &data); err != nil {
			log.Printf("🔌 Widget %s:%s unmarshal failed: %v (raw: %s)", w.PluginName, w.Handler, err, string(result))
			widget.Unavailable = true
			results = append(results, widget)
			continue
		}
		widget.HTML = data.HTML
		results = append(results, widget)
	}

	return results
}

// PluginWidgetData is the rendered widget data for templates. Unavailable
// marks a widget whose plugin call failed; it is shown with an error state
// instead of content.
type PluginWidgetData struct {
	ID          string
	Title       string
	PluginName  string
	HTML        string
	Unavailable bool
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

// buildPluginArgs extracts request data into JSON args for the plugin.
func buildPluginArgs(c *gin.Context, pluginName ...string) json.RawMessage {
	args := make(map[string]any)
	var rawBody, rawContentType string
	var hasRawBody bool

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

	// Request body (if present). Parse JSON bodies into the args map so
	// handlers that unmarshal into a struct pick fields up for free. For
	// non-JSON bodies (XML, CSV, plain text — e.g. an OTRS FAQ import),
	// capture the raw bytes and pass them through as _body / _content_type
	// so plugins can process payloads the JSON merger would otherwise drop.
	if c.Request.Body != nil && c.Request.ContentLength > 0 {
		raw, err := io.ReadAll(io.LimitReader(c.Request.Body, maxPluginBodySize+1))
		if err == nil {
			if int64(len(raw)) > maxPluginBodySize {
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "plugin request body too large"})
				c.Abort()
				return nil
			}
			// Restore the body in case a downstream reader needs it.
			c.Request.Body = io.NopCloser(bytes.NewReader(raw))

			contentType := c.GetHeader("Content-Type")
			if strings.Contains(strings.ToLower(contentType), "application/json") {
				var body map[string]any
				if err := json.Unmarshal(raw, &body); err == nil {
					for k, v := range body {
						args[k] = v
					}
				}
			} else if strings.Contains(strings.ToLower(contentType), "application/x-www-form-urlencoded") {
				// Parse form-encoded data into top-level args so plugins
				// receive {"title": "Hello"} not {"_body": "title=Hello"}.
				// ParseForm keeps the pairs decoded before a malformed one, so
				// the partial form still reaches the plugin.
				if err := c.Request.ParseForm(); err != nil {
					log.Printf("plugin call: malformed form body: %v", err)
				}
				for key, values := range c.Request.PostForm {
					if len(values) == 1 {
						args[key] = values[0]
					} else {
						args[key] = values
					}
				}
			} else {
				// Non-JSON payload: pass the raw body through verbatim.
				rawBody, rawContentType, hasRawBody = string(raw), contentType, true
			}
		}
	}

	stripPluginEnvelope(args)
	if hasRawBody {
		args["_body"] = rawBody
		args["_content_type"] = rawContentType
	}

	// Include request metadata
	args["_method"] = c.Request.Method
	args["_path"] = c.Request.URL.Path

	addPluginEnvelope(c, args, pluginName...)

	result, _ := json.Marshal(args)
	return result
}

// pluginEnvelopeKeys are the args keys the host writes for every plugin call.
// Plugins trust them (caller identity, admin flag, organisation, language,
// request metadata), so a client-supplied value under one of these names is
// dropped before the host sets its own; otherwise a body such as
// {"_is_admin": true} would reach the plugin whenever the auth middleware did
// not set the matching context key.
var pluginEnvelopeKeys = []string{
	"_user_id", "_user_email", "_user_login", "_customer_login", "_user_role", "_is_admin",
	"_org_id", "_lang", "_method", "_path", "_body", "_content_type",
}

func stripPluginEnvelope(args map[string]any) {
	for _, k := range pluginEnvelopeKeys {
		delete(args, k)
	}
}

// setPluginEnvelope replaces any client-supplied envelope key in args with the
// host envelope for the authenticated caller. Every plugin call built from
// client data goes through it (or buildPluginArgs).
func setPluginEnvelope(c *gin.Context, args map[string]any, pluginName string) {
	stripPluginEnvelope(args)
	addPluginEnvelope(c, args, pluginName)
}

// addPluginEnvelope writes the authenticated caller's identity, language and
// organisation into args. _is_admin is always present so plugins never fall
// back to a client-controlled value.
func addPluginEnvelope(c *gin.Context, args map[string]any, pluginName ...string) {
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
	role, hasRole := c.Get("user_role")
	if hasRole {
		args["_user_role"] = role
	}
	inAdminGroup, _ := c.Get("isInAdminGroup")
	args["_is_admin"] = inAdminGroup == true || role == "Admin"
	// The user's resolved UI language (i18n middleware: ?lang=, cookie, user
	// preference, Accept-Language). Plugins that ship their own translations
	// look strings up locally with it, and the Manager puts it into the call
	// context for HostAPI Translate.
	if lang, exists := c.Get(middleware.LanguageContextKey); exists {
		args["_lang"] = lang
	}

	// Inject org context from the authenticated session unless the plugin opts
	// out. orgIDFromContext checks the auth keys, the request context and the
	// membership-checked active-org cookie. The Manager puts _org_id into the
	// call context (HostAPI OrgID, sandbox org scoping), so a plugin that opts
	// out runs without an org. _org_id keeps the legacy key; org_id matches the
	// JSON field name plugins already unmarshal, so plain-request-struct
	// handlers get it for free.
	skipOrg := len(pluginName) > 0 && pluginManager != nil && pluginManager.SkipsOrgInjection(pluginName[0])
	if !skipOrg {
		if orgID := orgIDFromContext(c); orgID != 0 {
			args["_org_id"] = orgID
			args["org_id"] = orgID
		}
	}
}

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
		token := middleware.ExtractToken(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Authentication required"})
			c.Abort()
			return
		}

		jwtMgr := shared.GetJWTManager()
		if jwtMgr == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Auth service unavailable"})
			c.Abort()
			return
		}
		// If this is a gf_ API token, authenticate via the token verifier
		// instead of trying to validate it as a JWT.
		if middleware.IsAPIToken(token) {
			middleware.AuthenticateAPIToken(c, token)
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
		if token := middleware.ExtractToken(c); token != "" {
			if mgr := shared.GetJWTManager(); mgr != nil {
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
// Agents only (route group). When the plugin declares an EventAuthorizer, the
// plugin decides per caller and channel before the stream is opened.
func HandlePluginSSEChannel(c *gin.Context) {
	if pluginSSEBroker == nil || pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "SSE not available"})
		return
	}
	pluginName := c.Param("name")
	channel := c.Param("channel")

	p, ok := pluginManager.Get(pluginName)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "plugin not found"})
		return
	}
	if authorizer := p.GKRegister().EventAuthorizer; authorizer != "" {
		args := map[string]any{"channel": channel}
		addPluginEnvelope(c, args, pluginName)
		argsJSON, err := json.Marshal(args)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "subscription check failed"})
			return
		}
		result, err := pluginManager.Call(c.Request.Context(), pluginName, authorizer, argsJSON)
		if err != nil {
			log.Printf("plugin %s: event authorizer %s failed for channel %q: %v", pluginName, authorizer, channel, err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "subscription check failed"})
			return
		}
		var decision struct {
			Allow bool `json:"allow"`
		}
		if err := json.Unmarshal(result, &decision); err != nil || !decision.Allow {
			c.JSON(http.StatusForbidden, gin.H{"error": "subscription not allowed"})
			return
		}
	}

	pluginSSEBroker.ServeChannel(c.Writer, c.Request, pluginName, channel)
}

// HandlePluginUninstall removes a plugin: shuts it down, cleans up DB side-effects,
// removes the directory from disk, and clears in-memory state.
// DELETE /api/v1/plugins/:name
func HandlePluginUninstall(c *gin.Context) {
	name := c.Param("name")
	if name == "" || strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plugin name"})
		return
	}

	log.Printf("🗑️  Plugin uninstall requested: %s", name)

	if pluginManager == nil {
		log.Printf("ERROR plugin system not initialized for uninstall of %s", name)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Plugin system not initialized"})
		return
	}

	// 1. Unregister — proper shutdown, cascade teardown, in-memory cleanup.
	if err := pluginManager.Unregister(c.Request.Context(), name); err != nil {
		log.Printf("WARN Unregister for %s: %v — trying Unload fallback", name, err)
		pluginManager.Unload(name)
	}

	// 1b. Forget — remove from the loader's discovered set so the plugin
	// doesn't reappear in the plugin list before the next restart.
	pluginManager.Forget(name)

	// 2. Remove directory from disk.
	pmDir := pluginDir
	if pmDir != "" {
		pluginPath := filepath.Join(pmDir, name)
		if _, statErr := os.Stat(pluginPath); os.IsNotExist(statErr) {
			log.Printf("INFO plugin dir already gone for %s (path=%s)", name, pluginPath)
		} else if statErr == nil {
			if rmErr := os.RemoveAll(pluginPath); rmErr != nil {
				log.Printf("WARN removing plugin dir for %s: %v", name, rmErr)
			} else {
				log.Printf("INFO removed plugin dir %s for %s", pluginPath, name)
			}
		} else {
			log.Printf("WARN stat plugin dir for %s: %v", name, statErr)
		}
	} else {
		log.Printf("INFO plugin dir not configured — skipping disk cleanup for %s", name)
	}

	c.JSON(http.StatusOK, gin.H{"message": "plugin uninstalled", "name": name})
}

// RegisterPluginAPIRoutes registers the plugin management API endpoints.
// Listing, health, widgets and per-plugin SSE channels are for agents only:
// their callers are the agent dashboard and admin pages, and customers'
// customer_user ids overlap users ids. Direct function calls skip the plugin's
// own route middleware, so they are admin-only alongside the management
// endpoints.
func RegisterPluginAPIRoutes(r *gin.RouterGroup) {
	plugins := r.Group("/plugins")
	plugins.Use(SessionOrJWTAuth(), middleware.RequireAgent())
	{
		plugins.GET("", HandlePluginList)
		plugins.GET("/health", HandlePluginHealth)
		plugins.GET("/widgets", HandlePluginWidgetList)
		plugins.GET("/:name/widgets/:id", HandlePluginWidget)
		plugins.GET("/:name/events/:channel", HandlePluginSSEChannel)
	}

	// Plugin management - require admin (session or JWT)
	pluginAdmin := r.Group("/plugins")
	pluginAdmin.Use(SessionOrJWTAuth(), RequireAdmin())
	{
		pluginAdmin.POST("/:name/call/:fn", HandlePluginCall)
		pluginAdmin.POST("/:name/enable", HandlePluginEnable)
		pluginAdmin.POST("/:name/disable", HandlePluginDisable)
		pluginAdmin.POST("/:name/reset-crashloop", HandlePluginResetCrashLoop)
		pluginAdmin.DELETE("/:name", HandlePluginUninstall)
		pluginAdmin.POST("/upload", HandlePluginUpload)
		pluginAdmin.GET("/logs", HandlePluginLogs)
		pluginAdmin.DELETE("/logs", HandleClearPluginLogs)
		pluginAdmin.GET("/marketplace", HandleMarketplaceIndex)
		pluginAdmin.GET("/marketplace/search", HandleMarketplaceSearch)
		pluginAdmin.POST("/marketplace/install", HandleMarketplaceInstall)
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

// RequireGroup checks that the authenticated agent belongs to a specific group.
// Admin users (role=Admin or isInAdminGroup) bypass group checks. Customers are
// refused: groups hold agents (group_user.user_id = users.id), and a customer's
// customer_user.id would otherwise match an unrelated agent's memberships.
// Used by plugins via "group:<name>" middleware, e.g. "group:myplugin-users".
func RequireGroup(groupName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if middleware.IsCustomerPrincipal(c) {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied: requires group " + groupName})
			c.Abort()
			return
		}
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

		userID, ok := userIDUintFromValue(userIDRaw)
		if !ok {
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

		// Customer branch.
		if middleware.IsCustomerPrincipal(c) {
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
		userID, ok := userIDUintFromValue(userIDRaw)
		if !ok {
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
				if n <= math.MaxInt64 {
					return int64(n)
				}
			case float64:
				return int64(n)
			}
		}
	}
	if orgID := organisation.OrgIDFromContext(c.Request.Context()); orgID != 0 {
		return orgID
	}
	if orgID := activeOrgCookie(c); orgID != 0 && callerIsOrgMember(c, orgID) {
		return orgID
	}
	return 0
}

// activeOrgCookie returns the org id from the client-controlled active_org_id
// cookie, unvalidated. Only use it where it selects nothing private (the login
// page's identity-provider buttons); everything else goes through
// orgIDFromContext, which checks membership.
func activeOrgCookie(c *gin.Context) int64 {
	cookie, err := c.Cookie("active_org_id")
	if err != nil || cookie == "" {
		return 0
	}
	var n int64
	if _, err := fmt.Sscan(cookie, &n); err != nil || n <= 0 {
		return 0
	}
	return n
}

// callerIsOrgMember reports whether the authenticated caller belongs to
// orgID: customers through their company's org or a gk_user_organisation row,
// agents through gk_user_organisation.
func callerIsOrgMember(c *gin.Context, orgID int64) bool {
	db, err := database.GetDB()
	if err != nil || db == nil {
		return false
	}
	repo := organisation.NewRepositoryWithDB(db)
	var orgs []organisation.Organisation
	if role, _ := c.Get("user_role"); role == "Customer" { //nolint:errcheck // nil when absent
		login := c.GetString("customer_login")
		if login == "" {
			login = c.GetString("username")
		}
		if login == "" {
			return false
		}
		if primaryOrgForCustomer(db, login) == orgID {
			return true
		}
		orgs, err = repo.GetCustomerOrgs(login)
	} else {
		uid := shared.GetUserIDFromCtx(c, 0)
		if uid <= 0 {
			return false
		}
		orgs, err = repo.GetUserOrgs(uid)
	}
	if err != nil {
		return false
	}
	for _, o := range orgs {
		if o.ID == orgID {
			return true
		}
	}
	return false
}

// maxPluginBodySize is the maximum request body size passed through to a
// plugin via buildPluginArgs (4MB). Covers large imports such as an OTRS
// FAQ XML export while bounding memory per request.
const maxPluginBodySize = 4 << 20

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
	if err := os.MkdirAll(pluginDir, 0o750); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create plugin directory"})
		return
	}

	// Save uploaded file to temp location first
	tempPath := filepath.Join(pluginDir, ".upload_"+filename)
	dest, err := os.Create(tempPath) // #nosec G304 -- pluginDir is server config; filename reduced to filepath.Base and rejected if "." or ".."
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp file"})
		return
	}

	// Copy file content
	if _, err := io.Copy(dest, file); err != nil {
		_ = dest.Close()
		_ = os.Remove(tempPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save plugin file"})
		return
	}
	if err := dest.Close(); err != nil {
		_ = os.Remove(tempPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save plugin file"})
		return
	}

	var pluginName string
	var destPath string

	if isZip {
		// Validate the package first without extracting to the live directory.
		manifest, err := packaging.ValidatePackage(tempPath)
		if err != nil {
			_ = os.Remove(tempPath)
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
		_ = os.Remove(tempPath)
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
			_ = os.Remove(tempPath)
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
