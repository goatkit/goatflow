package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/plugin"
)

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
	c.JSON(http.StatusOK, gin.H{"plugins": manifests})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "disabled"})
}

// GetPluginWidgets returns rendered widgets for a dashboard location.
// Used by dashboard handlers to include plugin widgets.
func GetPluginWidgets(ctx context.Context, location string) []PluginWidgetData {
	if pluginManager == nil {
		log.Printf("🔌 GetPluginWidgets: pluginManager is nil!")
		return nil
	}

	widgets := pluginManager.Widgets(location)
	log.Printf("🔌 GetPluginWidgets(%s): found %d widgets from manager", location, len(widgets))
	results := make([]PluginWidgetData, 0, len(widgets))

	for _, w := range widgets {
		// Call the widget handler to get HTML
		result, err := pluginManager.Call(ctx, w.PluginName, w.Handler, nil)
		if err != nil {
			continue
		}

		var data struct {
			HTML string `json:"html"`
		}
		if err := json.Unmarshal(result, &data); err != nil {
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
}

// GetPluginMenuItems returns menu items for a location.
func GetPluginMenuItems(location string) []plugin.PluginMenuItem {
	if pluginManager == nil {
		return nil
	}
	return pluginManager.MenuItems(location)
}

func init() {
	// Register plugin handlers
	registerPluginHandlers()
}

func registerPluginHandlers() {
	// These will be registered with the routing system
	// For now, they're available to be called directly
}
