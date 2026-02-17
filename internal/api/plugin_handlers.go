package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/middleware"
	"github.com/goatkit/goatflow/internal/plugin"
	"github.com/goatkit/goatflow/internal/plugin/packaging"
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

// SetPluginManager sets the global plugin manager.
func SetPluginManager(mgr *plugin.Manager) {
	pluginManager = mgr
}

// GetPluginManager returns the global plugin manager.
func GetPluginManager() *plugin.Manager {
	return pluginManager
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
func buildPluginArgs(c *gin.Context) json.RawMessage {
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
	// user_login is set by session auth; for JWT, fall back to email
	if login, exists := c.Get("user_login"); exists {
		args["_user_login"] = login
	} else if email, exists := c.Get("user_email"); exists {
		args["_user_login"] = email
	}
	if role, exists := c.Get("user_role"); exists {
		args["_user_role"] = role
	}
	if isAdmin, exists := c.Get("isInAdminGroup"); exists {
		args["_is_admin"] = isAdmin
	}

	result, _ := json.Marshal(args)
	return result
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
		// If session middleware already authenticated the user, continue
		if _, exists := c.Get("user_id"); exists {
			c.Next()
			return
		}
		// Fall back to JWT auth
		JWTAuthMiddleware()(c)
	}
}

func RegisterPluginAPIRoutes(r *gin.RouterGroup) {
	// Plugin list and call - require authentication (session or JWT)
	plugins := r.Group("/plugins")
	plugins.Use(SessionOrJWTAuth())
	{
		plugins.GET("", HandlePluginList)
		plugins.POST("/:name/call/:fn", HandlePluginCall)
		plugins.GET("/widgets", HandlePluginWidgetList)
		plugins.GET("/:name/widgets/:id", HandlePluginWidget)
	}

	// Plugin management - require admin (session or JWT)
	pluginAdmin := r.Group("/plugins")
	pluginAdmin.Use(SessionOrJWTAuth(), RequireAdmin())
	{
		pluginAdmin.POST("/:name/enable", HandlePluginEnable)
		pluginAdmin.POST("/:name/disable", HandlePluginDisable)
		pluginAdmin.POST("/upload", HandlePluginUpload)
		pluginAdmin.GET("/logs", HandlePluginLogs)
		pluginAdmin.DELETE("/logs", HandleClearPluginLogs)
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

// pluginDir is the directory where plugins are stored.
// Set via SetPluginDir during app initialization.
var pluginDir string

// pluginReloader is called after a plugin is uploaded to trigger a load/reload.
var pluginReloader func(ctx context.Context, name string) error

// SetPluginDir sets the plugin directory for uploads.
func SetPluginDir(dir string) {
	pluginDir = dir
}

// SetPluginReloader sets the callback used to load/reload a plugin after upload.
func SetPluginReloader(fn func(ctx context.Context, name string) error) {
	pluginReloader = fn
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
		// Extract ZIP package
		pkg, err := packaging.ExtractPlugin(tempPath, pluginDir)
		os.Remove(tempPath) // Clean up temp file
		if err != nil {
			plugin.GetLogBuffer().Log("system", "error", fmt.Sprintf("Plugin upload failed: invalid package: %s", err.Error()), nil)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid plugin package: " + err.Error()})
			return
		}
		pluginName = pkg.Manifest.Name
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
