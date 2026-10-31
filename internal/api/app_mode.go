package api

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/audit"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/shared"
	"github.com/goatkit/goatflow/internal/repository"
)

// AppModeForUser resolves the app mode of an agent from their group
// memberships (see pkg/plugin.AppModeSpec): nil when no enabled plugin has
// app mode on, when the agent is an administrator, or when none of the
// agent's groups is in a plugin's audience. Installed as the shared
// AppModeProvider; the shell and every login path go through it.
func AppModeForUser(userID uint) *shared.AppModeView {
	if pluginManager == nil || userID == 0 || !pluginManager.HasAppMode() {
		return nil
	}
	db, err := database.GetDB()
	if err != nil || db == nil {
		return nil
	}
	groups, err := repository.NewGroupRepository(db).GetUserGroups(userID)
	if err != nil {
		log.Printf("app mode: groups for user %d: %v", userID, err)
		return nil
	}
	for _, g := range groups {
		if strings.EqualFold(g, "admin") {
			return nil
		}
	}
	mode := pluginManager.AppModeFor(groups)
	if mode == nil {
		return nil
	}
	view := &shared.AppModeView{
		PluginName:      mode.PluginName,
		LandingPage:     mode.LandingPage,
		HiddenNavItems:  make(map[string]bool, len(mode.HiddenMenuItems)),
		HidePluginMenus: mode.HidePluginMenus,
	}
	for _, id := range mode.HiddenMenuItems {
		view.HiddenNavItems[id] = true
	}
	if b := mode.Branding; b != nil {
		view.AppName = strings.TrimSpace(b.AppName)
		view.Logo = strings.TrimSpace(b.Logo)
		view.Icon = strings.TrimSpace(b.Icon)
		view.Favicon = strings.TrimSpace(b.Favicon)
		view.Color = strings.TrimSpace(b.Color)
	}
	return view
}

// agentLandingRedirect is where an agent goes after logging in: their app
// mode's landing page, else the landing page a plugin set for everyone,
// else the dashboard.
func agentLandingRedirect(userID uint) string {
	if mode := shared.AppModeFor(userID); mode != nil && mode.LandingPage != "" {
		return mode.LandingPage
	}
	if lp := shared.GetLandingPage(); lp != "" {
		return lp
	}
	return "/dashboard"
}

// HandlePluginAppModeToggle switches a plugin's app mode on or off.
// POST /api/v1/plugins/:name/app-mode {"enabled": bool}
func HandlePluginAppModeToggle(c *gin.Context) {
	if pluginManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Plugin system not initialized"})
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "enabled (true/false) required"})
		return
	}
	name := c.Param("name")
	wasEnabled := false
	if st := pluginManager.AppModeStatus(name); st != nil {
		wasEnabled = st.Enabled
	}
	if err := pluginManager.SetAppModeEnabled(name, *req.Enabled); err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if strings.Contains(err.Error(), "declares no app mode") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		log.Printf("plugin %s: app mode toggle: %v", name, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save app mode"})
		return
	}
	recordPluginAudit(c, "admin.plugin.app_mode.update", name, map[string]any{
		"changes": audit.Changes(map[string]any{"app_mode_enabled": wasEnabled}, map[string]any{"app_mode_enabled": *req.Enabled}),
	})
	c.JSON(http.StatusOK, gin.H{"app_mode": pluginManager.AppModeStatus(name)})
}
