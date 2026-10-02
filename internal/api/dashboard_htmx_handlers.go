package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/i18n"
	"github.com/goatkit/goatflow/internal/platform/lookups"
	"github.com/goatkit/goatflow/internal/platform/middleware"
	"github.com/goatkit/goatflow/internal/platform/notifications"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/platform/service"
	"github.com/goatkit/goatflow/internal/platform/shared"
	"github.com/goatkit/goatflow/internal/repository"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func init() {
	routing.RegisterHandler("handleDashboard", handleDashboard)
	routing.RegisterHandler("handleRecentTickets", handleRecentTickets)
	routing.RegisterHandler("handlePendingReminderFeed", handlePendingReminderFeed)
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

	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("handleDashboard: database unavailable: %v", err)
		c.String(http.StatusInternalServerError, "Database unavailable")
		return
	}

	// Get plugin widgets for dashboard - filtered by user preferences
	allPluginWidgets := GetPluginWidgets(c.Request.Context(), "dashboard", c)

	// Get user's widget config to filter
	var dashboardUserID int
	if val, exists := c.Get("user_id"); exists {
		dashboardUserID = shared.ToInt(val, 0)
	}

	pluginWidgets := allPluginWidgets
	if dashboardUserID > 0 {
		prefService := service.NewUserPreferencesService(db)
		widgetConfig, err := prefService.GetDashboardWidgets(dashboardUserID)
		if err != nil {
			log.Printf("handleDashboard: widget config for user %d: %v", dashboardUserID, err)
			c.String(http.StatusInternalServerError, "Failed to load dashboard")
			return
		}

		if len(widgetConfig) > 0 {
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
					fID := filtered[i].PluginName + ":" + filtered[i].ID
					if dx, dy, dw, dh, ok := defaultWidgetLayout(fID); ok {
						filtered[i].GridX = dx
						filtered[i].GridY = dy
						filtered[i].GridW = dw
						filtered[i].GridH = dh
					} else {
						filtered[i].GridW, filtered[i].GridH = sizeToGrid(filtered[i].Size)
					}
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

	// Ensure all widgets have default grid dimensions and positions
	for i := range pluginWidgets {
		fullID := pluginWidgets[i].PluginName + ":" + pluginWidgets[i].ID
		if pluginWidgets[i].GridW == 0 {
			if dx, dy, dw, dh, ok := defaultWidgetLayout(fullID); ok {
				pluginWidgets[i].GridX = dx
				pluginWidgets[i].GridY = dy
				pluginWidgets[i].GridW = dw
				pluginWidgets[i].GridH = dh
			} else {
				pluginWidgets[i].GridW, pluginWidgets[i].GridH = sizeToGrid(pluginWidgets[i].Size)
			}
		}
	}

	fmt.Printf("🔌 Dashboard: showing %d of %d plugin widgets\n", len(pluginWidgets), len(allPluginWidgets))

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/dashboard.pongo2", pongo2.Context{
		"User":          getUserMapForTemplate(c),
		"ActivePage":    "dashboard",
		"PluginWidgets": pluginWidgets,
	})
}

// buildTicketStatusOptions returns the valid ticket states as filter options
// (Value = state id, Param = slug, Label = display name) and whether any of
// them is of the closed state type. The states come only from the database.
func buildTicketStatusOptions(db *sql.DB) ([]gin.H, bool, error) {
	if db == nil {
		return nil, false, errors.New("ticket states: database unavailable")
	}
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT ts.id, ts.name, tst.name AS type_name
		FROM ticket_state ts
		JOIN ticket_state_type tst ON ts.type_id = tst.id
		WHERE ts.valid_id = 1
		ORDER BY ts.name`))
	if err != nil {
		return nil, false, fmt.Errorf("ticket states: %w", err)
	}
	defer rows.Close()

	titleCaser := cases.Title(language.English)
	options := []gin.H{}
	hasClosed := false
	for rows.Next() {
		var (
			stateID   int
			stateName string
			typeName  string
		)
		if err := rows.Scan(&stateID, &stateName, &typeName); err != nil {
			return nil, false, fmt.Errorf("ticket states: %w", err)
		}
		cleanName := strings.ReplaceAll(strings.TrimSpace(stateName), "_", " ")
		options = append(options, gin.H{
			"Value": strconv.Itoa(stateID),
			"Param": strings.ReplaceAll(strings.ToLower(cleanName), " ", "_"),
			"Label": titleCaser.String(cleanName),
		})
		if strings.EqualFold(strings.TrimSpace(typeName), lookups.StateTypeClosed) {
			hasClosed = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("ticket states: %w", err)
	}
	return options, hasClosed, nil
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

	// Only tickets in queues the agent can read; this route has no queue_ro
	// middleware, so the scope is resolved here.
	scope, ok := resolveTicketReadScope(c, db, false)
	if !ok {
		return
	}
	tickets, err := scope.listTickets(ticketRepo, listReq)
	if err != nil {
		log.Printf("handleRecentTickets: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to load recent tickets"})
		return
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
			statusLabel, err := lookups.Name(c.Request.Context(), db, lookups.StateLookup, ticket.TicketStateID)
			if err != nil {
				log.Printf("handleRecentTickets: ticket %d state: %v", ticket.ID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to load recent tickets"})
				return
			}
			priorityName, err := lookups.Name(c.Request.Context(), db, lookups.PriorityTable, ticket.TicketPriorityID)
			if err != nil {
				log.Printf("handleRecentTickets: ticket %d priority: %v", ticket.ID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to load recent tickets"})
				return
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
				template.HTMLEscapeString(ticket.TicketNumber),
				template.HTMLEscapeString(ticket.TicketNumber),
				template.HTMLEscapeString(ticket.Title),
				priorityStyle,
				template.HTMLEscapeString(priorityName),
				statusStyle,
				template.HTMLEscapeString(statusLabel),
				func() string {
					if ticket.CustomerUserID != nil {
						return template.HTMLEscapeString(fmt.Sprintf("%s: %s", t("labels.customer"), *ticket.CustomerUserID))
					}
					return template.HTMLEscapeString(fmt.Sprintf("%s: %s", t("labels.customer"), t("labels.unknown")))
				}()))
		}
	}

	html.WriteString(`</ul>`)

	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, html.String())
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
	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database unavailable"})
		return
	}
	enabled, err := service.NewUserPreferencesService(db).GetRemindersEnabled(userID)
	if err != nil {
		log.Printf("handlePendingReminderFeed: user %d: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to load reminder preference"})
		return
	}
	if !enabled {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"reminders": []gin.H{},
			},
		})
		return
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

// handleActivityStream streams recent ticket activity (server-sent events):
// one event on connect, then one every 30 seconds. Only tickets in queues the
// agent can read are reported.
func handleActivityStream(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database unavailable"})
		return
	}
	scope, ok := resolveTicketReadScope(c, db, false)
	if !ok {
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	send := func() {
		activity, err := latestTicketActivity(c, db, scope)
		if err != nil {
			log.Printf("handleActivityStream: %v", err)
			return
		}
		data, _ := json.Marshal(activity)                                   //nolint:errcheck // Best effort
		_, _ = fmt.Fprintf(c.Writer, "event: activity\ndata: %s\n\n", data) //nolint:errcheck // Best effort streaming
		c.Writer.Flush()
	}

	send()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			send()
		case <-c.Request.Context().Done():
			return
		}
	}
}

// latestTicketActivity returns the most recent ticket_history entry of the last
// 24 hours on a ticket the scope can read, or a "No recent activity" event.
func latestTicketActivity(c *gin.Context, db *sql.DB, scope ticketReadScope) (gin.H, error) {
	cond, args := scope.filter("t")
	args = append([]interface{}{time.Now().Add(-24 * time.Hour)}, args...)
	var name, historyType, ticketNumber, userName sql.NullString
	var createTime time.Time
	err := db.QueryRowContext(c.Request.Context(), database.ConvertPlaceholders(`
		SELECT th.name, tht.name, t.tn, u.login, th.create_time
		FROM ticket_history th
		JOIN ticket_history_type tht ON th.history_type_id = tht.id
		JOIN ticket t ON th.ticket_id = t.id
		LEFT JOIN users u ON th.create_by = u.id
		WHERE th.create_time >= ? AND `+cond+`
		ORDER BY th.create_time DESC, th.id DESC
		LIMIT 1`), args...).Scan(&name, &historyType, &ticketNumber, &userName, &createTime)
	if err == sql.ErrNoRows {
		return gin.H{
			"type":   "system",
			"user":   "System",
			"action": "No recent activity",
			"time":   time.Now().Format("15:04:05"),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	tn := ticketNumber.String
	var action string
	switch historyType.String {
	case "NewTicket":
		action = fmt.Sprintf("created ticket %s", tn)
	case "TicketStateUpdate":
		action = fmt.Sprintf("updated ticket %s", tn)
	case "AddNote":
		action = fmt.Sprintf("added note to ticket %s", tn)
	case "SendAnswer":
		action = fmt.Sprintf("replied to ticket %s", tn)
	case "Close":
		action = fmt.Sprintf("closed ticket %s", tn)
	default:
		if name.String != "" {
			action = fmt.Sprintf("%s on ticket %s", name.String, tn)
		} else {
			action = fmt.Sprintf("%s on ticket %s", historyType.String, tn)
		}
	}
	user := "System"
	if userName.String != "" {
		user = userName.String
	}
	return gin.H{
		"type":   "ticket_activity",
		"user":   user,
		"action": action,
		"time":   createTime.Format("15:04:05"),
	}, nil
}
