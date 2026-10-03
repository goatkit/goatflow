package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/marketplace"
	"github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/platform/version"
)

func init() {
	routing.RegisterHandler("HandleAdminMarketplace", HandleAdminMarketplace)
}

// HandleAdminMarketplace renders the marketplace browser page.
func HandleAdminMarketplace(c *gin.Context) {
	renderAdminPage(c, "pages/admin/marketplace.pongo2")
}

// marketplaceVersionStatus is one listed plugin version plus whether this
// GoatFlow can run it.
type marketplaceVersionStatus struct {
	marketplace.VersionEntry
	Compatible bool `json:"compatible"`
}

// marketplacePluginStatus is a marketplace entry annotated for this host.
// Versions shadows the embedded index field to add per-version compatibility.
type marketplacePluginStatus struct {
	marketplace.PluginEntry
	Versions         []marketplaceVersionStatus `json:"versions"`
	MinHostVersion   string                     `json:"min_host_version"` // lowest GoatFlow that runs any listed version
	Compatible       bool                       `json:"compatible"`       // some listed version runs on this GoatFlow
	InstallVersion   string                     `json:"install_version"`  // newest compatible version, "" if none
	InstalledVersion string                     `json:"installed_version"`
	UpdateAvailable  bool                       `json:"update_available"` // a compatible version newer than installed exists
}

func newMarketplacePluginStatus(p marketplace.PluginEntry, installed string) marketplacePluginStatus {
	all := p.AllVersions()
	status := marketplacePluginStatus{
		PluginEntry:      p,
		Versions:         make([]marketplaceVersionStatus, 0, len(all)),
		InstalledVersion: installed,
	}
	for i, v := range all {
		status.Versions = append(status.Versions, marketplaceVersionStatus{VersionEntry: v, Compatible: v.Compatible()})
		if i == 0 || v.MinHostVersion == "" ||
			(status.MinHostVersion != "" && marketplace.CompareVersions(v.MinHostVersion, status.MinHostVersion) < 0) {
			status.MinHostVersion = v.MinHostVersion
		}
	}
	if v, err := p.ResolveInstall(); err == nil {
		status.Compatible = true
		status.InstallVersion = v.Version
	}
	if installed != "" {
		if _, err := p.ResolveUpdate(installed); err == nil {
			status.UpdateAvailable = true
		}
	}
	return status
}

// HandleMarketplaceIndex returns the marketplace index annotated with install
// state and per-version compatibility with this GoatFlow.
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

	plugins := make([]marketplacePluginStatus, 0, len(index.Plugins))
	for _, p := range index.Plugins {
		plugins = append(plugins, newMarketplacePluginStatus(p, installedMap[p.Name]))
	}

	c.JSON(http.StatusOK, gin.H{
		"version":      index.Version,
		"updated_at":   index.UpdatedAt,
		"host_version": version.Short(),
		"plugins":      plugins,
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
// POST /api/v1/plugins/marketplace/install  {"name": "goat-kb", "version": "1.2.0"}
// version is optional: without it a new install gets the newest version this
// GoatFlow can run and an installed plugin gets its newest compatible update.
func HandleMarketplaceInstall(c *gin.Context) {
	if pluginDir == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Plugin directory not configured"})
		return
	}

	var req struct {
		Name    string `json:"name"`
		Version string `json:"version"`
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

	current := ""
	installed, _ := client.ListInstalled()
	for _, inst := range installed {
		if inst.Name == req.Name {
			current = inst.Version
			break
		}
	}
	isUpdate := current != ""

	// Resolve the target version up front so an incompatible or unlisted
	// request is refused before anything is downloaded.
	var target marketplace.VersionEntry
	switch {
	case req.Version != "":
		target, err = entry.ResolveVersion(req.Version)
	case isUpdate:
		target, err = entry.ResolveUpdate(current)
	default:
		target, err = entry.ResolveInstall()
	}
	if errors.Is(err, marketplace.ErrNoUpdate) || (err == nil && isUpdate && marketplace.CompareVersions(current, target.Version) == 0) {
		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("Plugin %s v%s already installed", req.Name, current),
			"name":    req.Name,
			"version": current,
			"action":  "noop",
		})
		return
	}
	if err != nil {
		status := http.StatusBadRequest
		var incompat *version.IncompatibleError
		if errors.As(err, &incompat) {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	if isUpdate {
		if _, err := client.Update(entry, target.Version); err != nil {
			plugin.GetLogBuffer().Log(req.Name, "error", fmt.Sprintf("Marketplace update failed: %s", err.Error()), nil)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Update failed: %v", err)}) //nolint:gk-sql-sprintf // hardcoded column fragments; user values bound via ?
			return
		}
	} else {
		if _, err := client.Install(entry, target.Version); err != nil {
			plugin.GetLogBuffer().Log(req.Name, "error", fmt.Sprintf("Marketplace install failed: %s", err.Error()), nil)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Install failed: %v", err)})
			return
		}
	}

	log.Printf("📦 Plugin %s v%s installed from marketplace", req.Name, target.Version)
	plugin.GetLogBuffer().Log(req.Name, "info", fmt.Sprintf("Installed from marketplace: v%s", target.Version), nil)

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
		"message": fmt.Sprintf("Plugin %s v%s %s successfully", req.Name, target.Version, action),
		"name":    req.Name,
		"version": target.Version,
		"action":  action,
	})
}
