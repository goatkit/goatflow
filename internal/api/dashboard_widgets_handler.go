package api

import (
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/platform/service"
)

func init() {
	routing.RegisterHandler("handleDashboardWidgetsConfig", handleDashboardWidgetsConfig)
	routing.RegisterHandler("handleDashboardWidgetsUpdate", handleDashboardWidgetsUpdate)
	routing.RegisterHandler("handleDashboardWidgetsList", handleDashboardWidgetsList)
}

// WidgetInfo describes a widget for the configuration UI.
type WidgetInfo struct {
	ID         string `json:"id"` // "plugin_name:widget_id"
	PluginName string `json:"plugin_name"`
	WidgetID   string `json:"widget_id"`
	Title      string `json:"title"`
	Size       string `json:"size"`
	Enabled    bool   `json:"enabled"`
	Position   int    `json:"position"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
	W          int    `json:"w"`
	H          int    `json:"h"`
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

		// Default grid dimensions: use curated layout if available,
		// otherwise fall back to generic size-based defaults.
		var gx, gy, gw, gh int
		if dx, dy, dw, dh, ok := defaultWidgetLayout(fullID); ok {
			gx, gy, gw, gh = dx, dy, dw, dh
		} else {
			gw, gh = sizeToGrid(w.Size)
		}

		if cfg, ok := configMap[fullID]; ok {
			enabled = cfg.Enabled
			position = cfg.Position
			if cfg.W > 0 {
				gw = cfg.W
			}
			if cfg.H > 0 {
				gh = cfg.H
			}
			gx = cfg.X
			gy = cfg.Y
		}

		widgets = append(widgets, WidgetInfo{
			ID:         fullID,
			PluginName: w.PluginName,
			WidgetID:   w.ID,
			Title:      w.Title,
			Size:       w.Size,
			Enabled:    enabled,
			Position:   position,
			X:          gx,
			Y:          gy,
			W:          gw,
			H:          gh,
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

// sizeToGrid converts a widget size hint to default grid dimensions (w, h)
// on a 12-column grid.
func sizeToGrid(size string) (int, int) {
	switch size {
	case "small":
		return 6, 2
	case "large":
		return 12, 4
	case "full":
		return 12, 2
	default: // "medium" or unset
		return 6, 3
	}
}

// defaultWidgetLayout returns sensible default (x, y, w, h) grid values for
// known widgets so the dashboard looks good out of the box. Unknown widgets
// return nil and fall back to sizeToGrid defaults at (0, 0).
//
// Default layout (12-column grid, cellHeight=120):
//
//	Row 0:  [Ticket Overview   6x3] [Recent Tickets     6x6]
//	Row 3:  [Tickets by Status 6x3]                      |
//	Row 6:  [Queue Status      6x4] [Ticket Chart       6x5]
//
// Design rationale — agent workflow priority:
//  1. Overview numbers (top-left, first thing scanned)
//  2. Recent tickets (top-right, tall — most actionable, shows more items)
//  3. Status breakdown (below overview, completes the workload picture)
//  4. Queue pressure (mid-left, which queues need attention)
//  5. Trend chart (mid-right, contextual not urgent)
//  6. Plugin widgets (bottom, supplementary information)
func defaultWidgetLayout(fullID string) (x, y, w, h int, ok bool) {
	type layout struct{ x, y, w, h int }
	defaults := map[string]layout{
		"stats:stats_overview":          {0, 0, 6, 3},
		"dashboard-core:recent_tickets": {6, 0, 6, 6},
		"stats:stats_by_status":         {0, 3, 6, 3},
		"dashboard-core:queue_status":   {0, 6, 6, 4},
		"stats:stats_chart":             {6, 6, 6, 5},
	}
	if l, found := defaults[fullID]; found {
		return l.x, l.y, l.w, l.h, true
	}
	return 0, 0, 0, 0, false
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
