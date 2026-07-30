package api

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/marketplace"
	"github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/goatkit/goatflow/internal/platform/routing"
)

func init() {
	routing.RegisterHandler("HandleAdminMarketplace", HandleAdminMarketplace)
}

// HandleAdminMarketplace renders the marketplace browser page.
func HandleAdminMarketplace(c *gin.Context) {
	renderAdminPage(c, "pages/admin/marketplace.pongo2")
}

// HandleMarketplaceIndex returns the full marketplace index as JSON.
// GET /api/v1/plugins/marketplace
func HandleMarketplaceIndex(c *gin.Context) {
	client := marketplace.NewClient(pluginDir)
	index, err := client.FetchIndex()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch marketplace index: " + err.Error()})
		return
	}

	installed, _ := client.ListInstalled()
	installedMap := make(map[string]string, len(installed))
	for _, inst := range installed {
		installedMap[inst.Name] = inst.Version
	}

	type pluginWithStatus struct {
		marketplace.PluginEntry
		InstalledVersion string `json:"installed_version"`
		UpdateAvailable  bool   `json:"update_available"`
	}

	plugins := make([]pluginWithStatus, 0, len(index.Plugins))
	for _, p := range index.Plugins {
		status := pluginWithStatus{PluginEntry: p}
		if v, ok := installedMap[p.Name]; ok {
			status.InstalledVersion = v
			if marketplace.CompareVersions(p.LatestVersion, v) > 0 {
				status.UpdateAvailable = true
			}
		}
		plugins = append(plugins, status)
	}

	c.JSON(http.StatusOK, gin.H{
		"version":    index.Version,
		"updated_at": index.UpdatedAt,
		"plugins":    plugins,
	})
}

// HandleMarketplaceSearch searches the marketplace index.
// GET /api/v1/plugins/marketplace/search?q=knowledge
func HandleMarketplaceSearch(c *gin.Context) {
	query := c.Query("q")
	category := c.Query("category")

	client := marketplace.NewClient(pluginDir)
	index, err := client.FetchIndex()
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch marketplace index: " + err.Error()})
		return
	}

	results := make([]marketplace.PluginEntry, 0, len(index.Plugins))
	for _, p := range index.Plugins {
		if category != "" && p.Category != category {
			continue
		}
		if query != "" && !marketplace.MatchesQuery(p, query) {
			continue
		}
		results = append(results, p)
	}

	c.JSON(http.StatusOK, gin.H{"plugins": results, "total": len(results)})
}

// HandleMarketplaceInstall downloads, verifies, and installs a plugin from the marketplace.
// POST /api/v1/plugins/marketplace/install  {"name": "goat-kb"}
func HandleMarketplaceInstall(c *gin.Context) {
	if pluginDir == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Plugin directory not configured"})
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Plugin name required"})
		return
	}

	client := marketplace.NewClient(pluginDir)

	entry, err := client.FindPlugin(req.Name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Plugin %q not found in marketplace: %v", req.Name, err)})
		return
	}

	// Check if already installed at same version
	installed, _ := client.ListInstalled()
	for _, inst := range installed {
		if inst.Name == req.Name && marketplace.CompareVersions(inst.Version, entry.LatestVersion) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"message": fmt.Sprintf("Plugin %s v%s already installed", req.Name, entry.LatestVersion),
				"name":    req.Name,
				"version": entry.LatestVersion,
				"action":  "noop",
			})
			return
		}
	}

	// Determine if this is an update or new install
	isUpdate := false
	for _, inst := range installed {
		if inst.Name == req.Name {
			isUpdate = true
			break
		}
	}

	if isUpdate {
		if err := client.Update(entry); err != nil {
			plugin.GetLogBuffer().Log(req.Name, "error", fmt.Sprintf("Marketplace update failed: %s", err.Error()), nil)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Update failed: %v", err)}) //nolint:gk-sql-sprintf // hardcoded column fragments; user values bound via ?
			return
		}
	} else {
		if err := client.Install(entry); err != nil {
			plugin.GetLogBuffer().Log(req.Name, "error", fmt.Sprintf("Marketplace install failed: %s", err.Error()), nil)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Install failed: %v", err)})
			return
		}
	}

	log.Printf("📦 Plugin %s v%s installed from marketplace", req.Name, entry.LatestVersion)
	plugin.GetLogBuffer().Log(req.Name, "info", fmt.Sprintf("Installed from marketplace: v%s", entry.LatestVersion), nil)

	// Trigger hot-reload
	if pluginReloader != nil {
		go func() {
			if err := pluginReloader(context.Background(), req.Name); err != nil {
				log.Printf("⚠️  Plugin reload failed for %s: %v", req.Name, err)
				plugin.GetLogBuffer().Log(req.Name, "error", fmt.Sprintf("Reload failed: %v", err), nil)
			} else {
				log.Printf("✅ Plugin %s loaded/reloaded after marketplace install", req.Name)
				plugin.GetLogBuffer().Log(req.Name, "info", "Plugin loaded/reloaded after marketplace install", nil)
				RebuildDynamicEngine()
			}
		}()
	}

	action := "installed"
	if isUpdate {
		action = "updated"
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Plugin %s v%s %s successfully", req.Name, entry.LatestVersion, action),
		"name":    req.Name,
		"version": entry.LatestVersion,
		"action":  action,
	})
}
