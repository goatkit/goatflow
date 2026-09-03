package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/pluginui"
	"github.com/goatkit/goatflow/internal/platform/swconfig"
)

// HandleRedirect handles redirect routes.
func HandleRedirect(c *gin.Context) {
	routeConfig, exists := c.Get("route_config")
	if !exists {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	config := routeConfig.(map[string]interface{}) //nolint:errcheck // Type asserted from context
	redirectTo := "/login"                         // default
	if to, ok := config["redirect_to"].(string); ok {
		redirectTo = to
	}

	redirectCode := http.StatusFound // Default 302
	if code, ok := config["redirect_code"].(float64); ok {
		redirectCode = int(code)
	}

	c.Redirect(redirectCode, redirectTo)
}

// HandleTemplate handles template rendering routes.
func HandleTemplate(c *gin.Context) {
	routeConfig, exists := c.Get("route_config")
	if !exists {
		c.String(http.StatusInternalServerError, "No route config")
		return
	}

	config := routeConfig.(map[string]interface{}) //nolint:errcheck // Type asserted from context
	template := "pages/login.pongo2"               // default
	if tmpl, ok := config["template"].(string); ok {
		template = tmpl
	}

	data := gin.H{}
	if configData, ok := config["data"].(map[string]interface{}); ok {
		for k, v := range configData {
			data[k] = v
		}
	}

	// Add user context if authenticated
	if user, exists := c.Get("user"); exists {
		data["User"] = user
	}

	// Use the pongo2 renderer
	getPongo2Renderer().HTML(c, http.StatusOK, template, data)
}

// HandleStaticFiles serves static files from the static directory.
func HandleStaticFiles(c *gin.Context) {
	// Get the full path from the request
	requestPath := c.Request.URL.Path

	// Map the request path to the file system path
	var filePath string

	if requestPath == "/favicon.ico" {
		filePath = "./static/favicon.ico"
	} else if requestPath == "/favicon.svg" {
		filePath = "./static/favicon.svg"
	} else if requestPath == "/manifest.json" {
		filePath = "./static/manifest.json"
	} else if requestPath == "/sw.js" {
		c.Header("Service-Worker-Allowed", "/")
		filePath = "./static/sw.js"
	} else if strings.HasPrefix(requestPath, "/static/") {
		// Extract the static file path
		filePath = "." + requestPath
	} else {
		c.Status(http.StatusNotFound)
		return
	}

	// App assets ship without content hashes; revalidate so browsers
	// and service workers never serve a frozen stale copy across deploys.
	if strings.HasPrefix(requestPath, "/static/") {
		c.Header("Cache-Control", "no-cache")
	}
	c.File(filePath)
}

// HandleServiceWorkerConfig returns the runtime cache configuration consumed by /sw.js.
func HandleServiceWorkerConfig(c *gin.Context) {
	var uis []pluginui.PluginUI
	db, err := database.GetDB()
	if err == nil && db != nil {
		repo := pluginui.NewRepositoryWithDB(db)
		if active, listErr := repo.ListActive(); listErr == nil {
			uis = active
		}
	}

	c.Header("Cache-Control", "no-cache")
	c.JSON(http.StatusOK, swconfig.Build(db, uis))
}
