package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/organisation"
	"github.com/goatkit/goatflow/internal/repository"
)

// handleAPIListOrgPluginAccess returns the plugin-access bindings for an
// organisation: which plugins are enabled and which group gates access
// for each. Used by the org admin page to populate the "Plugins" card.
func handleAPIListOrgPluginAccess(c *gin.Context) {
	orgID, ok := parseOrgIDParam(c)
	if !ok {
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
		return
	}
	repo := repository.NewPluginAccessRepository(db)
	rows, err := repo.ListForOrg(orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type item struct {
		PluginName string `json:"plugin_name"`
		GroupID    int    `json:"group_id"`
		GroupName  string `json:"group_name"`
	}
	out := make([]item, 0, len(rows))
	for _, r := range rows {
		out = append(out, item{PluginName: r.PluginName, GroupID: r.GroupID, GroupName: r.GroupName})
	}
	c.JSON(http.StatusOK, gin.H{"plugin_access": out})
}

// handleAPISetOrgPluginAccess replaces the binding for (org, plugin_name)
// with the supplied group_id. group_id = 0 disables the plugin for the
// org (removes the binding without inserting a new one). Enforces the
// "one group per (org, plugin)" UI rule via repository.ReplaceForOrgPlugin.
//
// Body: {"plugin_name": "goatfictus", "group_id": 42}
func handleAPISetOrgPluginAccess(c *gin.Context) {
	orgID, ok := parseOrgIDParam(c)
	if !ok {
		return
	}

	var req struct {
		PluginName string `json:"plugin_name"`
		GroupID    int    `json:"group_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.PluginName = strings.TrimSpace(req.PluginName)
	if req.PluginName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "plugin_name is required"})
		return
	}
	if req.GroupID < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "group_id must be >= 0"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
		return
	}

	// Record the acting admin for the audit trail on create_by. Falls
	// back to 1 (system) if the context key hasn't been populated — that
	// only happens for machine-invoked flows, which don't normally hit
	// this endpoint.
	createdBy := 1
	if v, ok := c.Get("user_id"); ok {
		switch n := v.(type) {
		case int:
			createdBy = n
		case uint:
			createdBy = int(n)
		case int64:
			createdBy = int(n)
		}
	}

	repo := repository.NewPluginAccessRepository(db)
	if err := repo.ReplaceForOrgPlugin(orgID, req.PluginName, req.GroupID, createdBy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "plugin_name": req.PluginName, "group_id": req.GroupID})
}

// handleAPISetCaptivePlugin writes gk_organisation.captive_plugin for an
// org after validating that the named plugin is actually enabled for
// the org via gk_org_plugin_access. Body:
//
//	{"captive_plugin": "goatfictus"}  // enable capture
//	{"captive_plugin": ""}             // disable capture (null also ok)
//
// The UI enforces the "can only capture into an enabled plugin" rule;
// this handler re-enforces it server-side so a direct API call can't
// bypass the UX check. An empty string clears the setting.
func handleAPISetCaptivePlugin(c *gin.Context) {
	orgID, ok := parseOrgIDParam(c)
	if !ok {
		return
	}

	var req struct {
		CaptivePlugin string `json:"captive_plugin"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.CaptivePlugin = strings.TrimSpace(req.CaptivePlugin)

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
		return
	}

	// Enforce: target plugin must be enabled for this org. An empty
	// string clears the setting, which is always allowed.
	if req.CaptivePlugin != "" {
		bindings, err := repository.NewPluginAccessRepository(db).ListForOrg(orgID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		enabled := false
		for _, b := range bindings {
			if b.PluginName == req.CaptivePlugin {
				enabled = true
				break
			}
		}
		if !enabled {
			c.JSON(http.StatusBadRequest, gin.H{"error": "plugin is not enabled for this organisation — enable it first via the Plugins settings"})
			return
		}
	}

	orgR, err := organisation.NewRepository()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
		return
	}
	userID := 1
	if v, ok := c.Get("user_id"); ok {
		switch n := v.(type) {
		case int:
			userID = n
		case uint:
			userID = int(n)
		case int64:
			userID = int(n)
		}
	}
	var ptr *string
	if req.CaptivePlugin != "" {
		ptr = &req.CaptivePlugin
	}
	if err := orgR.SetCaptivePlugin(orgID, ptr, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "captive_plugin": req.CaptivePlugin})
}

// handleAPIDeleteOrgPluginAccess disables a plugin for the org by removing
// every binding with (org_id, plugin_name). Equivalent to the POST
// endpoint with group_id=0, kept as a distinct route so the UI can use a
// DELETE verb for clarity.
func handleAPIDeleteOrgPluginAccess(c *gin.Context) {
	orgID, ok := parseOrgIDParam(c)
	if !ok {
		return
	}
	pluginName := strings.TrimSpace(c.Param("plugin"))
	if pluginName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "plugin is required"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
		return
	}
	createdBy := 1
	if v, ok := c.Get("user_id"); ok {
		switch n := v.(type) {
		case int:
			createdBy = n
		case uint:
			createdBy = int(n)
		case int64:
			createdBy = int(n)
		}
	}
	repo := repository.NewPluginAccessRepository(db)
	if err := repo.ReplaceForOrgPlugin(orgID, pluginName, 0, createdBy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "disabled", "plugin_name": pluginName})
}

// parseOrgIDParam extracts and validates the :id path parameter shared by
// every org-scoped admin route.
func parseOrgIDParam(c *gin.Context) (int64, bool) {
	raw := c.Param("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organisation id"})
		return 0, false
	}
	return id, true
}
