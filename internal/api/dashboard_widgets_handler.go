package api

import (
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/plugin"
	"github.com/goatkit/goatflow/internal/routing"
	"github.com/goatkit/goatflow/internal/service"
)

func init() {
	routing.RegisterHandler("handleDashboardWidgetsConfig", handleDashboardWidgetsConfig)
	routing.RegisterHandler("handleDashboardWidgetsUpdate", handleDashboardWidgetsUpdate)
	routing.RegisterHandler("handleDashboardWidgetsList", handleDashboardWidgetsList)
}

// WidgetInfo describes a widget for the configuration UI.
type WidgetInfo struct {
	ID         string `json:"id"`          // "plugin_name:widget_id"
	PluginName string `json:"plugin_name"`
	WidgetID   string `json:"widget_id"`
	Title      string `json:"title"`
	Enabled    bool   `json:"enabled"`
	Position   int    `json:"position"`
}

// handleDashboardWidgetsList returns all available widgets with their current config.
// GET /api/dashboard/widgets
func handleDashboardWidgetsList(c *gin.Context) {
	userID := getDashboardUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database unavailable"})
		return
	}

	// Get all available widgets metadata (don't render, just list)
	// Use AllWidgets which triggers lazy loading of discovered plugins
	var allWidgetsMeta []plugin.PluginWidget
	if pluginManager != nil {
		allWidgetsMeta = pluginManager.AllWidgets("dashboard")
	}

	// Get user's config
	prefService := service.NewUserPreferencesService(db)
	userConfig, _ := prefService.GetDashboardWidgets(userID)

	// Build config map for quick lookup
	configMap := make(map[string]service.DashboardWidgetConfig)
	for _, cfg := range userConfig {
		configMap[cfg.WidgetID] = cfg
	}

	// Build response with all widgets and their config
	widgets := make([]WidgetInfo, 0, len(allWidgetsMeta))
	for i, w := range allWidgetsMeta {
		fullID := w.PluginName + ":" + w.ID
		enabled := true
		position := i

		if cfg, ok := configMap[fullID]; ok {
			enabled = cfg.Enabled
			position = cfg.Position
		}

		widgets = append(widgets, WidgetInfo{
			ID:         fullID,
			PluginName: w.PluginName,
			WidgetID:   w.ID,
			Title:      w.Title,
			Enabled:    enabled,
			Position:   position,
		})
	}
	
	// If no widgets found (plugin manager not initialized), return empty
	if widgets == nil {
		widgets = []WidgetInfo{}
	}

	// Sort by position
	sort.Slice(widgets, func(i, j int) bool {
		return widgets[i].Position < widgets[j].Position
	})

	c.JSON(http.StatusOK, gin.H{"success": true, "widgets": widgets})
}

// handleDashboardWidgetsConfig returns user's current widget config.
// GET /api/dashboard/widgets/config
func handleDashboardWidgetsConfig(c *gin.Context) {
	userID := getDashboardUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database unavailable"})
		return
	}

	prefService := service.NewUserPreferencesService(db)
	config, err := prefService.GetDashboardWidgets(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to get config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "config": config})
}

// handleDashboardWidgetsUpdate saves user's widget config.
// POST /api/dashboard/widgets/config
// Body: {"widgets": [{"widget_id": "stats:stats_overview", "enabled": true, "position": 0}, ...]}
func handleDashboardWidgetsUpdate(c *gin.Context) {
	userID := getDashboardUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
		return
	}

	var req struct {
		Widgets []service.DashboardWidgetConfig `json:"widgets"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database unavailable"})
		return
	}

	prefService := service.NewUserPreferencesService(db)
	if err := prefService.SetDashboardWidgets(userID, req.Widgets); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to save config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Dashboard widgets updated"})
}

// getDashboardUserID extracts user ID from context for dashboard widgets.
func getDashboardUserID(c *gin.Context) int {
	if val, ok := c.Get("user_id"); ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case uint:
			return int(v)
		case uint64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return 0
}
