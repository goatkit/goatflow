package api

import (
	"encoding/json"
	"net/http"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/routing"
)

func init() {
	routing.RegisterHandler("HandleAdminPlugins", HandleAdminPlugins)
	routing.RegisterHandler("HandleAdminPluginLogs", HandleAdminPluginLogs)
}

// HandleAdminPluginLogs renders the plugin logs viewer page.
// GET /admin/plugins/logs
func HandleAdminPluginLogs(c *gin.Context) {
	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/plugin_logs.pongo2", pongo2.Context{})
}

// HandleAdminPlugins renders the admin plugin management page.
// GET /admin/plugins
func HandleAdminPlugins(c *gin.Context) {
	// Get plugin list
	var plugins []map[string]any
	var enabledCount, disabledCount int

	if pluginManager != nil {
		manifests := pluginManager.List()
		for _, m := range manifests {
			// Check actual enabled state from plugin manager
			enabled := pluginManager.IsEnabled(m.Name)

			p := map[string]any{
				"Name":        m.Name,
				"Version":     m.Version,
				"Description": m.Description,
				"Author":      m.Author,
				"License":     m.License,
				"Routes":      m.Routes,
				"Widgets":     m.Widgets,
				"Jobs":        m.Jobs,
				"MenuItems":   m.MenuItems,
				"Enabled":     enabled,
			}
			plugins = append(plugins, p)
			if enabled {
				enabledCount++
			} else {
				disabledCount++
			}
		}
	}

	// Serialize plugins to JSON for JavaScript
	pluginsJSON, _ := json.Marshal(plugins)

	ctx := pongo2.Context{
		"Plugins":       plugins,
		"PluginsJSON":   string(pluginsJSON),
		"EnabledCount":  enabledCount,
		"DisabledCount": disabledCount,
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/plugins.pongo2", ctx)
}
