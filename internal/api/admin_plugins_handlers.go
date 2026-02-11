package api

import (
	"net/http"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/routing"
)

func init() {
	routing.RegisterHandler("HandleAdminPlugins", HandleAdminPlugins)
	routing.RegisterHandler("HandleAdminPluginLogs", HandleAdminPluginLogs)
}

// HandleAdminPluginLogs renders the plugin logs page.
// Data is fetched client-side from /api/v1/plugins/logs.
func HandleAdminPluginLogs(c *gin.Context) {
	renderAdminPage(c, "pages/admin/plugin_logs.pongo2")
}

// HandleAdminPlugins renders the plugin management page.
// Data is fetched client-side from /api/v1/plugins.
func HandleAdminPlugins(c *gin.Context) {
	renderAdminPage(c, "pages/admin/plugins.pongo2")
}

// renderAdminPage renders a template with standard admin context.
// Templates should self-hydrate via API calls rather than requiring
// server-side data preparation — this is the GoatKit way.
func renderAdminPage(c *gin.Context, template string) {
	user := getUserMapForTemplate(c)
	isInAdminGroup := false
	if v, ok := user["IsInAdminGroup"].(bool); ok {
		isInAdminGroup = v
	}

	getPongo2Renderer().HTML(c, http.StatusOK, template, pongo2.Context{
		"ActivePage":     "admin",
		"User":           user,
		"IsInAdminGroup": isInAdminGroup,
	})
}
