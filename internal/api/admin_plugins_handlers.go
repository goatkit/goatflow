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
	user := getUserMapForTemplate(c)
	isInAdminGroup := false
	if v, ok := user["IsInAdminGroup"].(bool); ok {
		isInAdminGroup = v
	}

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/plugin_logs.pongo2", pongo2.Context{
		"ActivePage":     "admin",
		"User":           user,
		"IsInAdminGroup": isInAdminGroup,
	})
}

// HandleAdminPlugins renders the admin plugin management page.
// GET /admin/plugins
func HandleAdminPlugins(c *gin.Context) {
	user := getUserMapForTemplate(c)
	isInAdminGroup := false
	if v, ok := user["IsInAdminGroup"].(bool); ok {
		isInAdminGroup = v
	}

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

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/plugins.pongo2", pongo2.Context{
		"ActivePage":     "admin",
		"User":           user,
		"IsInAdminGroup": isInAdminGroup,
		"Plugins":        plugins,
		"PluginsJSON":    string(pluginsJSON),
		"EnabledCount":   enabledCount,
		"DisabledCount":  disabledCount,
	})
}
