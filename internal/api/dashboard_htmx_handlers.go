package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/i18n"
	"github.com/goatkit/goatflow/internal/middleware"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/notifications"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/routing"
	"github.com/goatkit/goatflow/internal/service"
	"github.com/goatkit/goatflow/internal/shared"

	"github.com/xeonx/timeago"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func init() {
	routing.RegisterHandler("handleDashboard", handleDashboard)
	routing.RegisterHandler("handleDashboardStats", handleDashboardStats)
	routing.RegisterHandler("handleRecentTickets", handleRecentTickets)
	routing.RegisterHandler("handleNotifications", handleNotifications)
	routing.RegisterHandler("handlePendingReminderFeed", handlePendingReminderFeed)
	routing.RegisterHandler("handleQuickActions", handleQuickActions)
	routing.RegisterHandler("handleActivity", handleActivity)
	routing.RegisterHandler("handlePerformance", handlePerformance)
	routing.RegisterHandler("handleActivityStream", handleActivityStream)
}

// handleDashboard shows the main dashboard.
func handleDashboard(c *gin.Context) {
	// If templates unavailable, return JSON error
	if getPongo2Renderer() == nil || getPongo2Renderer().TemplateSet() == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Template system unavailable",
		})
		return
	}

	// Get database connection through repository pattern (graceful fallback if unavailable)
	db, err := database.GetDB()
	if err != nil || db == nil {
		getPongo2Renderer().HTML(c, http.StatusOK, "pages/dashboard.pongo2", pongo2.Context{
			"Title":         "Dashboard - GoatFlow",
			"Stats":         gin.H{"openTickets": 0, "pendingTickets": 0, "closedToday": 0},
			"RecentTickets": []gin.H{},
			"User":          getUserMapForTemplate(c),
			"ActivePage":    "dashboard",
		})
		return
	}

	// Use repository for database operations
	ticketRepo := repository.NewTicketRepository(db)

	// RBAC: Queue permission filtering - use context values from middleware
	var queueFilter string
	var queueArgs []interface{}

	isQueueAdmin := false
	if val, exists := c.Get("is_queue_admin"); exists {
		if admin, ok := val.(bool); ok {
			isQueueAdmin = admin
		}
	}

	if !isQueueAdmin {
		if accessibleQueueIDs, exists := c.Get("accessible_queue_ids"); exists {
			if queueIDs, ok := accessibleQueueIDs.([]uint); ok && len(queueIDs) > 0 {
				placeholders := make([]string, len(queueIDs))
				for i, qid := range queueIDs {
					placeholders[i] = "?"
					queueArgs = append(queueArgs, qid)
				}
				queueFilter = " AND queue_id IN (" + strings.Join(placeholders, ",") + ")"
			}
		}
	}

	// Get ticket statistics - RBAC filtered
	var openTickets, pendingTickets, closedToday int

	// Get actual ticket state IDs from database
	var openStateID, pendingStateID, closedStateID int
	_ = db.QueryRow("SELECT id FROM ticket_state WHERE name = 'open'").Scan(&openStateID)       //nolint:errcheck
	_ = db.QueryRow("SELECT id FROM ticket_state WHERE name = 'pending'").Scan(&pendingStateID) //nolint:errcheck
	_ = db.QueryRow("SELECT id FROM ticket_state WHERE name = 'closed'").Scan(&closedStateID)   //nolint:errcheck

	// Count open tickets (with RBAC queue filter)
	if openStateID > 0 {
		query := "SELECT COUNT(*) FROM ticket WHERE ticket_state_id = ?" + queueFilter
		args := append([]interface{}{openStateID}, queueArgs...)
		_ = db.QueryRow(database.ConvertPlaceholders(query), args...).Scan(&openTickets) //nolint:errcheck
	}

	// Count pending tickets (with RBAC queue filter)
	if pendingStateID > 0 {
		query := "SELECT COUNT(*) FROM ticket WHERE ticket_state_id = ?" + queueFilter
		args := append([]interface{}{pendingStateID}, queueArgs...)
		_ = db.QueryRow(database.ConvertPlaceholders(query), args...).Scan(&pendingTickets) //nolint:errcheck
	}

	// Count tickets closed today (with RBAC queue filter)
	if closedStateID > 0 {
		query := `SELECT COUNT(*) FROM ticket WHERE ticket_state_id = ? AND DATE(change_time) = CURDATE()` + queueFilter
		args := append([]interface{}{closedStateID}, queueArgs...)
		_ = db.QueryRow(database.ConvertPlaceholders(query), args...).Scan(&closedToday) //nolint:errcheck
	}

	stats := gin.H{
		"openTickets":     openTickets,
		"pendingTickets":  pendingTickets,
		"closedToday":     closedToday,
		"avgResponseTime": "N/A", // Would require more complex calculation
	}

	// Get recent tickets from database (filtered by queue permissions)
	// ticketRepo already created above
	listReq := &models.TicketListRequest{
		Page:      1,
		PerPage:   5,
		SortBy:    "create_time",
		SortOrder: "desc",
	}

	// Queue permission filtering for recent tickets - reuse isQueueAdmin from above
	if !isQueueAdmin {
		if accessibleQueueIDs, exists := c.Get("accessible_queue_ids"); exists {
			if queueIDs, ok := accessibleQueueIDs.([]uint); ok {
				listReq.AccessibleQueueIDs = queueIDs
			}
		}
	}

	ticketResponse, err := ticketRepo.List(listReq)
	tickets := []models.Ticket{}
	if err == nil && ticketResponse != nil {
		tickets = ticketResponse.Tickets
	}

	recentTickets := []gin.H{}
	if err == nil && tickets != nil {
		for _, ticket := range tickets {
			// Get status label from database
			statusLabel := "unknown"
			var statusRow struct {
				Name string
			}
			query := database.ConvertPlaceholders("SELECT name FROM ticket_state WHERE id = ?")
			err = db.QueryRow(query, ticket.TicketStateID).Scan(&statusRow.Name)
			if err == nil {
				statusLabel = statusRow.Name
			}

			// Get priority label from database
			priorityLabel := "normal"
			var priorityRow struct {
				Name string
			}
			query = database.ConvertPlaceholders("SELECT name FROM ticket_priority WHERE id = ?")
			err = db.QueryRow(query, ticket.TicketPriorityID).Scan(&priorityRow.Name)
			if err == nil {
				priorityLabel = priorityRow.Name
			}

			// Calculate time ago
			timeAgo := timeago.English.Format(ticket.ChangeTime)

			recentTickets = append(recentTickets, gin.H{
				"id":       ticket.TicketNumber,
				"subject":  ticket.Title,
				"status":   statusLabel,
				"priority": priorityLabel,
				"customer": ticket.CustomerUserID,
				"updated":  timeAgo,
			})
		}
	}

	// Get plugin widgets for dashboard - filtered by user preferences
	allPluginWidgets := GetPluginWidgets(c.Request.Context(), "dashboard", c)
	
	// Get user's widget config to filter
	var dashboardUserID int
	if val, exists := c.Get("user_id"); exists {
		dashboardUserID = shared.ToInt(val, 0)
	}
	
	pluginWidgets := allPluginWidgets
	if dashboardUserID > 0 && db != nil {
		prefService := service.NewUserPreferencesService(db)
		widgetConfig, _ := prefService.GetDashboardWidgets(dashboardUserID)
		
		if widgetConfig != nil && len(widgetConfig) > 0 {
			// Build map of widget configs (enabled + position + grid)
			configMap := make(map[string]service.DashboardWidgetConfig)
			for _, cfg := range widgetConfig {
				configMap[cfg.WidgetID] = cfg
			}
			
			// Filter widgets based on config and apply grid coords
			filtered := make([]PluginWidgetData, 0, len(allPluginWidgets))
			for _, w := range allPluginWidgets {
				fullID := w.PluginName + ":" + w.ID
				
				// Check if widget is in config
				if cfg, inConfig := configMap[fullID]; inConfig {
					if cfg.Enabled {
						w.GridX = cfg.X
						w.GridY = cfg.Y
						w.GridW = cfg.W
						w.GridH = cfg.H
						filtered = append(filtered, w)
					}
					// If disabled, skip it
				} else {
					// Not in config = enabled by default
					filtered = append(filtered, w)
				}
			}
			
			// Apply default grid dimensions for widgets without saved config
			for i := range filtered {
				if filtered[i].GridW == 0 {
					filtered[i].GridW, filtered[i].GridH = sizeToGrid(filtered[i].Size)
				}
			}
			
			// Sort by saved position
			sort.SliceStable(filtered, func(i, j int) bool {
				idI := filtered[i].PluginName + ":" + filtered[i].ID
				idJ := filtered[j].PluginName + ":" + filtered[j].ID
				posI, posJ := len(filtered), len(filtered) // default: end
				if cfg, ok := configMap[idI]; ok {
					posI = cfg.Position
				}
				if cfg, ok := configMap[idJ]; ok {
					posJ = cfg.Position
				}
				return posI < posJ
			})
			
			pluginWidgets = filtered
		}
	}
	
	// Ensure all widgets have default grid dimensions
	for i := range pluginWidgets {
		if pluginWidgets[i].GridW == 0 {
			pluginWidgets[i].GridW, pluginWidgets[i].GridH = sizeToGrid(pluginWidgets[i].Size)
		}
	}

	fmt.Printf("🔌 Dashboard: showing %d of %d plugin widgets\n", len(pluginWidgets), len(allPluginWidgets))

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/dashboard.pongo2", pongo2.Context{
		"Stats":         stats,
		"RecentTickets": recentTickets,
		"User":          getUserMapForTemplate(c),
		"ActivePage":    "dashboard",
		"PluginWidgets": pluginWidgets,
	})
}

func buildTicketStatusOptions(db *sql.DB) ([]gin.H, bool) {
	titleCaser := cases.Title(language.English)
	options := []gin.H{}
	hasClosed := false
	appendDefaults := func() {
		options = append(options,
			gin.H{"Value": "1", "Param": "new", "Label": titleCaser.String("new")},
			gin.H{"Value": "2", "Param": "open", "Label": titleCaser.String("open")},
			gin.H{"Value": "3", "Param": "pending", "Label": titleCaser.String("pending")},
			gin.H{"Value": "4", "Param": "closed", "Label": titleCaser.String("closed")},
		)
		hasClosed = true
	}

	if db == nil {
		appendDefaults()
		return options, hasClosed
	}

	query := `
		SELECT ts.id, ts.name, tst.id AS type_id, tst.name AS type_name
		FROM ticket_state ts
		JOIN ticket_state_type tst ON ts.type_id = tst.id
		WHERE ts.valid_id = 1
		ORDER BY ts.name
	`
	rows, err := db.Query(database.ConvertPlaceholders(query))
	if err != nil {
		log.Printf("failed to load ticket states: %v", err)
		appendDefaults()
		return options, hasClosed
	}
	defer rows.Close()

	for rows.Next() {
		var (
			stateID   uint
			stateName string
			typeID    uint
			typeName  string
		)
		if scanErr := rows.Scan(&stateID, &stateName, &typeID, &typeName); scanErr != nil {
			continue
		}
		cleanName := strings.ReplaceAll(strings.TrimSpace(stateName), "_", " ")
		slug := strings.ReplaceAll(strings.ToLower(cleanName), " ", "_")
		options = append(options, gin.H{
			"Value": fmt.Sprintf("%d", stateID),
			"Param": slug,
			"Label": titleCaser.String(cleanName),
		})
		if strings.EqualFold(strings.TrimSpace(typeName), "closed") || typeID == uint(models.TicketStateClosed) {
			hasClosed = true
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("failed iterating ticket states: %v", err)
	}

	if len(options) == 1 {
		appendDefaults()
	}

	return options, hasClosed
}

// handleDashboardStats returns dashboard statistics.
func handleDashboardStats(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		// Return JSON error when database is unavailable
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database unavailable",
		})
		return
	}

	// Queue permission filtering - use context values from middleware
	var queueFilter string
	var queueArgs []interface{}

	isQueueAdmin := false
	if val, exists := c.Get("is_queue_admin"); exists {
		if admin, ok := val.(bool); ok {
			isQueueAdmin = admin
		}
	}

	if !isQueueAdmin {
		if accessibleQueueIDs, exists := c.Get("accessible_queue_ids"); exists {
			if queueIDs, ok := accessibleQueueIDs.([]uint); ok && len(queueIDs) > 0 {
				placeholders := make([]string, len(queueIDs))
				for i, qid := range queueIDs {
					placeholders[i] = "?"
					queueArgs = append(queueArgs, qid)
				}
				queueFilter = " AND queue_id IN (" + strings.Join(placeholders, ",") + ")"
			}
		}
	}

	var openTickets, pendingTickets, closedToday int

	// Get actual ticket state IDs from database instead of hardcoded values
	var openStateID, pendingStateID, closedStateID int
	_ = db.QueryRow("SELECT id FROM ticket_state WHERE name = 'open'").Scan(&openStateID)       //nolint:errcheck // Defaults to 0
	_ = db.QueryRow("SELECT id FROM ticket_state WHERE name = 'pending'").Scan(&pendingStateID) //nolint:errcheck // Defaults to 0
	_ = db.QueryRow("SELECT id FROM ticket_state WHERE name = 'closed'").Scan(&closedStateID)   //nolint:errcheck // Defaults to 0

	// Count open tickets (with queue filter)
	if openStateID > 0 {
		query := "SELECT COUNT(*) FROM ticket WHERE ticket_state_id = ?" + queueFilter
		args := append([]interface{}{openStateID}, queueArgs...)
		_ = db.QueryRow(database.ConvertPlaceholders(query), args...).Scan(&openTickets) //nolint:errcheck // Defaults to 0
	}

	// Count pending tickets (with queue filter)
	if pendingStateID > 0 {
		query := "SELECT COUNT(*) FROM ticket WHERE ticket_state_id = ?" + queueFilter
		args := append([]interface{}{pendingStateID}, queueArgs...)
		_ = db.QueryRow(database.ConvertPlaceholders(query), args...).Scan(&pendingTickets) //nolint:errcheck // Defaults to 0
	}

	// Count tickets closed today (with queue filter)
	if closedStateID > 0 {
		query := `SELECT COUNT(*) FROM ticket WHERE ticket_state_id = ? AND DATE(change_time) = CURDATE()` + queueFilter
		args := append([]interface{}{closedStateID}, queueArgs...)
		_ = db.QueryRow(database.ConvertPlaceholders(query), args...).Scan(&closedToday) //nolint:errcheck // Defaults to 0
	}

	// Get user language for i18n
	lang := "en"
	if l, exists := c.Get(middleware.LanguageContextKey); exists {
		if langStr, ok := l.(string); ok {
			lang = langStr
		}
	}
	i18nInstance := i18n.GetInstance()
	t := func(key string) string {
		return i18nInstance.T(lang, key)
	}

	// Return HTML for HTMX with Synthwave styling
	c.Header("Content-Type", "text/html")
	html := fmt.Sprintf(`
        <div class="gk-stat-card">
            <dt class="gk-stat-label">%s</dt>
            <dd class="gk-stat-value mt-1">%d</dd>
        </div>
        <div class="gk-stat-card success">
            <dt class="gk-stat-label">%s</dt>
            <dd class="gk-stat-value mt-1">%d</dd>
        </div>
        <div class="gk-stat-card warning">
            <dt class="gk-stat-label">%s</dt>
            <dd class="gk-stat-value mt-1">%d</dd>
        </div>
        <div class="gk-stat-card error">
            <dt class="gk-stat-label">%s</dt>
            <dd class="gk-stat-value mt-1">%d</dd>
        </div>`,
		t("dashboard.stats.open_tickets"), openTickets,
		t("dashboard.stats.new_today"), closedToday,
		t("dashboard.stats.pending"), pendingTickets,
		t("dashboard.stats.overdue"), 0) // Note: Overdue calculation not implemented yet

	c.String(http.StatusOK, html)
}

// handleRecentTickets returns recent tickets for dashboard.
func handleRecentTickets(c *gin.Context) {
	// Get user language for i18n
	lang := "en"
	if l, exists := c.Get(middleware.LanguageContextKey); exists {
		if langStr, ok := l.(string); ok {
			lang = langStr
		}
	}
	i18nInstance := i18n.GetInstance()
	t := func(key string) string {
		return i18nInstance.T(lang, key)
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		// Return JSON error when database is unavailable
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database unavailable",
		})
		return
	}

	ticketRepo := repository.NewTicketRepository(db)
	listReq := &models.TicketListRequest{
		Page:      1,
		PerPage:   5,
		SortBy:    "create_time",
		SortOrder: "desc",
	}

	// SECURITY: Queue permission filtering - use context values from middleware
	isQueueAdmin := false
	if val, exists := c.Get("is_queue_admin"); exists {
		if admin, ok := val.(bool); ok {
			isQueueAdmin = admin
		}
	}

	if !isQueueAdmin {
		if accessibleQueueIDs, exists := c.Get("accessible_queue_ids"); exists {
			if queueIDs, ok := accessibleQueueIDs.([]uint); ok {
				listReq.AccessibleQueueIDs = queueIDs
			}
		}
	}

	ticketResponse, err := ticketRepo.List(listReq)
	tickets := []models.Ticket{}
	if err == nil && ticketResponse != nil {
		tickets = ticketResponse.Tickets
	}

	// Build HTML response with Synthwave styling
	var html strings.Builder
	html.WriteString(`<ul role="list" class="-my-5 divide-y" style="border-color: var(--gk-border-default);">`)

	if len(tickets) == 0 {
		html.WriteString(fmt.Sprintf(`
                        <li class="py-4">
                            <div class="flex items-center space-x-4">
                                <div class="min-w-0 flex-1">
                                    <p class="truncate text-sm font-medium" style="color: var(--gk-text-primary);">%s</p>
                                    <p class="truncate text-sm" style="color: var(--gk-text-muted);">%s</p>
                                </div>
                            </div>
                        </li>`, t("dashboard.no_recent_tickets"), t("dashboard.no_tickets_in_system")))
	} else {
		for _, ticket := range tickets {
			// Get status label from database
			statusLabel := "unknown"
			var statusRow struct {
				Name string
			}
			query := database.ConvertPlaceholders("SELECT name FROM ticket_state WHERE id = ?")
			err = db.QueryRow(query, ticket.TicketStateID).Scan(&statusRow.Name)
			if err == nil {
				statusLabel = statusRow.Name
			}

			// Get priority name and determine CSS class
			priorityName := "normal"
			var priorityRow struct {
				Name string
			}
			query = database.ConvertPlaceholders("SELECT name FROM ticket_priority WHERE id = ?")
			err = db.QueryRow(query, ticket.TicketPriorityID).Scan(&priorityRow.Name)
			if err == nil {
				priorityName = priorityRow.Name
			}

			// Badge styles using theme variables
			priorityStyle := "background: var(--gk-success-subtle); color: var(--gk-success);"
			switch strings.ToLower(priorityName) {
			case "1 very low":
				priorityStyle = "background: var(--gk-info-subtle); color: var(--gk-info);"
			case "2 low":
				priorityStyle = "background: var(--gk-info-subtle); color: var(--gk-info);"
			case "3 normal":
				priorityStyle = "background: var(--gk-success-subtle); color: var(--gk-success);"
			case "4 high":
				priorityStyle = "background: var(--gk-warning-subtle); color: var(--gk-warning);"
			case "5 very high":
				priorityStyle = "background: var(--gk-error-subtle); color: var(--gk-error);"
			}

			statusStyle := "background: var(--gk-primary-subtle); color: var(--gk-primary);"
			statusLower := strings.ToLower(statusLabel)
			switch {
			case statusLower == "new":
				statusStyle = "background: var(--gk-secondary-subtle); color: var(--gk-secondary);"
			case statusLower == "open":
				statusStyle = "background: var(--gk-success-subtle); color: var(--gk-success);"
			case strings.HasPrefix(statusLower, "pending"):
				statusStyle = "background: var(--gk-warning-subtle); color: var(--gk-warning);"
			case strings.HasPrefix(statusLower, "closed"), strings.HasPrefix(statusLower, "merged"), strings.HasPrefix(statusLower, "removed"):
				statusStyle = "background: var(--gk-bg-elevated); color: var(--gk-text-secondary);"
			}

			const custBadge = "px-2.5 py-0.5 rounded-full text-xs font-medium"
			html.WriteString(fmt.Sprintf(`
			<li class="py-4 transition-all duration-200 hover:translate-x-1" style="border-color: var(--gk-border-default);">
				<div class="flex items-start space-x-4">
					<div class="min-w-0 flex-1">
						<a href="/tickets/%s" class="gk-link-neon text-sm font-medium">
							%s: %s
						</a>
						<div class="mt-2 flex flex-wrap gap-1">
							<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium" style="%s">
								%s
							</span>
							<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium" style="%s">
								%s
							</span>
							<span class="`+custBadge+`" style="background: var(--gk-bg-elevated); color: var(--gk-text-secondary);">%s</span>
						</div>
					</div>
				</div>
			</li>`,
				ticket.TicketNumber,
				ticket.TicketNumber,
				ticket.Title,
				priorityStyle,
				priorityName,
				statusStyle,
				statusLabel,
				func() string {
					if ticket.CustomerUserID != nil {
						return fmt.Sprintf("%s: %s", t("labels.customer"), *ticket.CustomerUserID)
					}
					return fmt.Sprintf("%s: %s", t("labels.customer"), t("labels.unknown"))
				}()))
		}
	}

	html.WriteString(`</ul>`)

	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, html.String())
}

// handleNotifications returns user notifications.
func handleNotifications(c *gin.Context) {
	// TODO: Implement actual notifications from database
	// For now, return empty list
	notifications := []gin.H{}
	c.JSON(http.StatusOK, gin.H{"notifications": notifications})
}

func handlePendingReminderFeed(c *gin.Context) {
	userVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}

	userID := normalizeUserID(userVal)
	if userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
		return
	}

	// Check if reminders are enabled for this user
	db, dbErr := database.GetDB()
	if dbErr == nil {
		prefService := service.NewUserPreferencesService(db)
		if !prefService.GetRemindersEnabled(userID) {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": gin.H{
					"reminders": []gin.H{},
				},
			})
			return
		}
	}

	hub := notifications.GetHub()
	items := hub.Consume(userID)
	reminders := make([]gin.H, 0, len(items))
	for _, reminder := range items {
		reminders = append(reminders, gin.H{
			"ticket_id":          reminder.TicketID,
			"ticket_number":      reminder.TicketNumber,
			"title":              reminder.Title,
			"queue_id":           reminder.QueueID,
			"queue_name":         reminder.QueueName,
			"pending_until":      reminder.PendingUntil.UTC().Format(time.RFC3339),
			"pending_until_unix": reminder.PendingUntil.Unix(),
			"state_name":         reminder.StateName,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"reminders": reminders,
		},
	})
}

func normalizeUserID(value interface{}) int {
	return shared.ToInt(value, 0)
}

// handleQuickActions returns quick action items.
func handleQuickActions(c *gin.Context) {
	actions := []gin.H{
		{"id": "new_ticket", "label": "New Ticket", "icon": "plus", "url": "/ticket/new"},
		{"id": "my_tickets", "label": "My Tickets", "icon": "list", "url": "/tickets?assigned=me"},
		{"id": "reports", "label": "Reports", "icon": "chart", "url": "/reports"},
	}
	c.JSON(http.StatusOK, gin.H{"actions": actions})
}

// handleActivity returns recent activity.
func handleActivity(c *gin.Context) {
	activities := []gin.H{
		{
			"id":     "1",
			"type":   "ticket_created",
			"user":   "John Doe",
			"action": "created ticket T-2024-001",
			"time":   "5 minutes ago",
		},
		{
			"id":     "2",
			"type":   "ticket_updated",
			"user":   "Alice Agent",
			"action": "updated ticket T-2024-002",
			"time":   "10 minutes ago",
		},
	}
	c.JSON(http.StatusOK, gin.H{"activities": activities})
}

// handlePerformance returns performance metrics.
func handlePerformance(c *gin.Context) {
	metrics := gin.H{
		"responseTime": []gin.H{
			{"time": "00:00", "value": 2.1},
			{"time": "04:00", "value": 1.8},
			{"time": "08:00", "value": 3.2},
			{"time": "12:00", "value": 2.5},
			{"time": "16:00", "value": 2.8},
			{"time": "20:00", "value": 2.0},
		},
		"ticketVolume": []gin.H{
			{"day": "Mon", "created": 45, "closed": 42},
			{"day": "Tue", "created": 52, "closed": 48},
			{"day": "Wed", "created": 38, "closed": 40},
			{"day": "Thu", "created": 61, "closed": 55},
			{"day": "Fri", "created": 43, "closed": 45},
		},
	}
	c.JSON(http.StatusOK, metrics)
}

// handleActivityStream provides real-time activity updates.
func handleActivityStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	db, err := database.GetDB()
	if err != nil || db == nil {
		// If no database, send a simple heartbeat
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				activity := gin.H{
					"type":   "system",
					"user":   "System",
					"action": "Heartbeat - Database unavailable",
					"time":   time.Now().Format("15:04:05"),
				}
				data, _ := json.Marshal(activity)                                   //nolint:errcheck // Best effort
				_, _ = fmt.Fprintf(c.Writer, "event: activity\ndata: %s\n\n", data) //nolint:errcheck // Best effort streaming
				c.Writer.Flush()
			case <-c.Request.Context().Done():
				return
			}
		}
	}

	// Send real activity updates from ticket_history
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Query recent ticket activity (last 24 hours)
			rows, err := db.Query(`
				SELECT
					th.name,
					tht.name as history_type,
					t.tn as ticket_number,
					u.login as user_name,
					th.create_time
				FROM ticket_history th
				JOIN ticket_history_type tht ON th.history_type_id = tht.id
				JOIN ticket t ON th.ticket_id = t.id
				LEFT JOIN users u ON th.create_by = u.id
				WHERE th.create_time >= DATE_SUB(NOW(), INTERVAL 24 HOUR)
				ORDER BY th.create_time DESC
				LIMIT 5
			`)

			if err == nil && rows != nil {
				activities := make([]gin.H, 0, 5) // Preallocate for expected LIMIT 5
				for rows.Next() {
					var name, historyType, ticketNumber, userName sql.NullString
					var createTime time.Time

					err := rows.Scan(&name, &historyType, &ticketNumber, &userName, &createTime)
					if err != nil {
						continue
					}

					// Format activity message
					action := "Unknown activity"
					if historyType.Valid {
						switch historyType.String {
						case "NewTicket":
							action = fmt.Sprintf("created ticket %s", ticketNumber.String)
						case "TicketStateUpdate":
							action = fmt.Sprintf("updated ticket %s", ticketNumber.String)
						case "AddNote":
							action = fmt.Sprintf("added note to ticket %s", ticketNumber.String)
						case "SendAnswer":
							action = fmt.Sprintf("replied to ticket %s", ticketNumber.String)
						case "Close":
							action = fmt.Sprintf("closed ticket %s", ticketNumber.String)
						default:
							if name.Valid && name.String != "" {
								action = fmt.Sprintf("%s on ticket %s", name.String, ticketNumber.String)
							} else {
								action = fmt.Sprintf("%s on ticket %s", historyType.String, ticketNumber.String)
							}
						}
					}

					user := "System"
					if userName.Valid && userName.String != "" {
						user = userName.String
					}

					activities = append(activities, gin.H{
						"type":   "ticket_activity",
						"user":   user,
						"action": action,
						"time":   createTime.Format("15:04:05"),
					})
				}
				rows.Close() //nolint:sqlclosecheck // Intentionally not using defer - inside infinite loop
				if err := rows.Err(); err != nil {
					log.Printf("error iterating activity rows: %v", err)
				}

				// Send the most recent activity
				if len(activities) > 0 {
					data, _ := json.Marshal(activities[0])                              //nolint:errcheck // Best effort
					_, _ = fmt.Fprintf(c.Writer, "event: activity\ndata: %s\n\n", data) //nolint:errcheck // Best effort streaming
					c.Writer.Flush()
				} else {
					// No recent activity
					activity := gin.H{
						"type":   "system",
						"user":   "System",
						"action": "No recent activity",
						"time":   time.Now().Format("15:04:05"),
					}
					data, _ := json.Marshal(activity)                                   //nolint:errcheck // Best effort
					_, _ = fmt.Fprintf(c.Writer, "event: activity\ndata: %s\n\n", data) //nolint:errcheck // Best effort streaming
					c.Writer.Flush()
				}
			}
		case <-c.Request.Context().Done():
			return
		}
	}
}
