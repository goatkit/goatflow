package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/pluginui"
)

type pluginUIAdminResponse struct {
	ID           int64                      `json:"id"`
	PluginName   string                     `json:"plugin_name"`
	UIID         string                     `json:"ui_id"`
	FullID       string                     `json:"full_id"`
	Name         string                     `json:"name"`
	Description  *string                    `json:"description,omitempty"`
	UIType       string                     `json:"ui_type"`
	Shell        string                     `json:"shell"`
	Icon         *string                    `json:"icon,omitempty"`
	Enabled      bool                       `json:"enabled"`
	Active       bool                       `json:"active"`
	CustomDomain *string                    `json:"custom_domain,omitempty"`
	Branding     *pluginui.UIBrandingConfig `json:"branding,omitempty"`
	PWAEnabled   bool                       `json:"pwa_enabled"`
	BasePath     string                     `json:"base_path"`
	ManifestPath string                     `json:"manifest_path,omitempty"`
	ChangeTime   string                     `json:"change_time"`
}

type pluginUIUpdateRequest struct {
	CustomDomain *string                    `json:"custom_domain"`
	Branding     *pluginui.UIBrandingConfig `json:"branding"`
}

type pluginUIToggleRequest struct {
	Enabled bool `json:"enabled"`
}

// HandleAdminPluginUIs renders the plugin UI management page.
func HandleAdminPluginUIs(c *gin.Context) {
	renderAdminPage(c, "pages/admin/plugin_uis.pongo2")
}

// HandlePluginUIAdminList returns all registered plugin UIs for administrators.
func HandlePluginUIAdminList(c *gin.Context) {
	repo, ok := pluginUIAdminRepo(c)
	if !ok {
		return
	}
	uis, err := repo.List(c.Query("plugin"), c.Query("type"), false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list plugin UIs"})
		return
	}

	resp := make([]pluginUIAdminResponse, 0, len(uis))
	for i := range uis {
		resp = append(resp, buildPluginUIAdminResponse(&uis[i]))
	}
	c.JSON(http.StatusOK, gin.H{"uis": resp})
}

// HandlePluginUIAdminUpdate updates admin-managed UI settings.
func HandlePluginUIAdminUpdate(c *gin.Context) {
	repo, ok := pluginUIAdminRepo(c)
	if !ok {
		return
	}
	id, ok := pluginUIIDParam(c)
	if !ok {
		return
	}

	var req pluginUIUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	updated, err := repo.UpdateAdminOverrides(id, req.CustomDomain, req.Branding, GetUserIDFromCtx(c, 1))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update plugin UI"})
		return
	}
	if updated == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "plugin UI not found"})
		return
	}

	RebuildDynamicEngine()
	c.JSON(http.StatusOK, gin.H{"ui": buildPluginUIAdminResponse(updated)})
}

// HandlePluginUIAdminToggle enables or disables a plugin UI.
func HandlePluginUIAdminToggle(c *gin.Context) {
	repo, ok := pluginUIAdminRepo(c)
	if !ok {
		return
	}
	id, ok := pluginUIIDParam(c)
	if !ok {
		return
	}

	var req pluginUIToggleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := repo.SetEnabled(id, req.Enabled, GetUserIDFromCtx(c, 1)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update plugin UI status"})
		return
	}
	updated, err := repo.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load plugin UI"})
		return
	}
	if updated == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "plugin UI not found"})
		return
	}

	RebuildDynamicEngine()
	c.JSON(http.StatusOK, gin.H{"ui": buildPluginUIAdminResponse(updated)})
}

func pluginUIAdminRepo(c *gin.Context) (*pluginui.Repository, bool) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
		return nil, false
	}
	return pluginui.NewRepositoryWithDB(db), true
}

func pluginUIIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plugin UI id"})
		return 0, false
	}
	return id, true
}

func buildPluginUIAdminResponse(ui *pluginui.PluginUI) pluginUIAdminResponse {
	cfg, _ := ui.ParsedConfig()
	var branding *pluginui.UIBrandingConfig
	pwaEnabled := false
	if cfg != nil {
		branding = cfg.Branding
		pwaEnabled = cfg.PWA != nil && cfg.PWA.Enabled
	}
	manifestPath := ""
	if pwaEnabled {
		manifestPath = ui.BasePath() + "manifest.json"
	}

	return pluginUIAdminResponse{
		ID:           ui.ID,
		PluginName:   ui.PluginName,
		UIID:         ui.UIID,
		FullID:       ui.FullID,
		Name:         ui.Name,
		Description:  ui.Description,
		UIType:       ui.UIType,
		Shell:        ui.Shell,
		Icon:         ui.Icon,
		Enabled:      ui.Enabled,
		Active:       ui.IsActive(),
		CustomDomain: ui.CustomDomain,
		Branding:     branding,
		PWAEnabled:   pwaEnabled,
		BasePath:     ui.BasePath(),
		ManifestPath: manifestPath,
		ChangeTime:   ui.ChangeTime.Format("2006-01-02 15:04"),
	}
}
