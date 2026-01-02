package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/notifications"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/routing"

	"github.com/xeonx/timeago"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func init() {
	routing.RegisterHandler("handleDashboard", handleDashboard)
	routing.RegisterHandler("handleDashboardStats", handleDashboardStats)
	routing.RegisterHandler("handleRecentTickets", handleRecentTickets)
	routing.RegisterHandler("dashboard_queue_status", dashboard_queue_status)
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
			"Title":         "Dashboard - GOTRS",
			"Stats":         gin.H{"openTickets": 0, "pendingTickets": 0, "closedToday": 0},
			"RecentTickets": []gin.H{},
			"User":          getUserMapForTemplate(c),
			"ActivePage":    "dashboard",
		})
		return
	}

	// Use repository for database operations
	ticketRepo := repository.NewTicketRepository(db)

	// Get ticket statistics using repository methods
	var openTickets, pendingTickets, closedToday int

	openTickets, err = ticketRepo.CountByStateID(2) // state_id = 2 for open
	if err != nil {
		openTickets = 0
	}

	pendingTickets, err = ticketRepo.CountByStateID(5) // state_id = 5 for pending
	if err != nil {
		pendingTickets = 0
	}

	closedToday, err = ticketRepo.CountClosedToday()
	if err != nil {
		closedToday = 0
	}

	stats := gin.H{
		"openTickets":     openTickets,
		"pendingTickets":  pendingTickets,
		"closedToday":     closedToday,
		"avgResponseTime": "N/A", // Would require more complex calculation
	}

	// Get recent tickets from database
	// ticketRepo already created above
	listReq := &models.TicketListRequest{
		Page:      1,
		PerPage:   5,
		SortBy:    "create_time",
		SortOrder: "desc",
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
			query := database.ConvertPlaceholders("SELECT name FROM ticket_state WHERE id = $1")
			err = db.QueryRow(query, ticket.TicketStateID).Scan(&statusRow.Name)
			if err == nil {
				statusLabel = statusRow.Name
			}

			// Get priority label from database
			priorityLabel := "normal"
			var priorityRow struct {
				Name string
			}
			query = database.ConvertPlaceholders("SELECT name FROM ticket_priority WHERE id = $1")
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

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/dashboard.pongo2", pongo2.Context{
		"Stats":         stats,
		"RecentTickets": recentTickets,
		"User":          getUserMapForTemplate(c),
		"ActivePage":    "dashboard",
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

	var openTickets, pendingTickets, closedToday int

	// Get actual ticket state IDs from database instead of hardcoded values
	var openStateID, pendingStateID, closedStateID int
	_ = db.QueryRow("SELECT id FROM ticket_state WHERE name = 'open'").Scan(&openStateID)       //nolint:errcheck // Defaults to 0
	_ = db.QueryRow("SELECT id FROM ticket_state WHERE name = 'pending'").Scan(&pendingStateID) //nolint:errcheck // Defaults to 0
	_ = db.QueryRow("SELECT id FROM ticket_state WHERE name = 'closed'").Scan(&closedStateID)   //nolint:errcheck // Defaults to 0

	// Count open tickets
	if openStateID > 0 {
		_ = db.QueryRow("SELECT COUNT(*) FROM ticket WHERE ticket_state_id = ?", openStateID).Scan(&openTickets) //nolint:errcheck // Defaults to 0
	}

	// Count pending tickets
	if pendingStateID > 0 {
		_ = db.QueryRow("SELECT COUNT(*) FROM ticket WHERE ticket_state_id = ?", pendingStateID).Scan(&pendingTickets) //nolint:errcheck // Defaults to 0
	}

	// Count tickets closed today
	if closedStateID > 0 {
		row := db.QueryRow(database.ConvertPlaceholders(`
			SELECT COUNT(*) FROM ticket
			WHERE ticket_state_id = ?
			AND DATE(change_time) = CURDATE()
		`), closedStateID)
		_ = row.Scan(&closedToday) //nolint:errcheck // Defaults to 0
	}

	// Return HTML for HTMX
	c.Header("Content-Type", "text/html")
	html := fmt.Sprintf(`
        <div class="overflow-hidden rounded-lg bg-white dark:bg-gray-800 px-4 py-5 shadow sm:p-6">
            <dt class="truncate text-sm font-medium text-gray-500 dark:text-gray-400">Open Tickets</dt>
            <dd class="mt-1 text-3xl font-semibold tracking-tight text-gray-900 dark:text-white">%d</dd>
        </div>
        <div class="overflow-hidden rounded-lg bg-white dark:bg-gray-800 px-4 py-5 shadow sm:p-6">
            <dt class="truncate text-sm font-medium text-gray-500 dark:text-gray-400">New Today</dt>
            <dd class="mt-1 text-3xl font-semibold tracking-tight text-gray-900 dark:text-white">%d</dd>
        </div>
        <div class="overflow-hidden rounded-lg bg-white dark:bg-gray-800 px-4 py-5 shadow sm:p-6">
            <dt class="truncate text-sm font-medium text-gray-500 dark:text-gray-400">Pending</dt>
            <dd class="mt-1 text-3xl font-semibold tracking-tight text-gray-900 dark:text-white">%d</dd>
        </div>
        <div class="overflow-hidden rounded-lg bg-white dark:bg-gray-800 px-4 py-5 shadow sm:p-6">
            <dt class="truncate text-sm font-medium text-gray-500 dark:text-gray-400">Overdue</dt>
            <dd class="mt-1 text-3xl font-semibold tracking-tight text-gray-900 dark:text-white">%d</dd>
        </div>`, openTickets, closedToday, pendingTickets, 0) // Note: Overdue calculation not implemented yet

	c.String(http.StatusOK, html)
}

// handleRecentTickets returns recent tickets for dashboard.
func handleRecentTickets(c *gin.Context) {
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
	ticketResponse, err := ticketRepo.List(listReq)
	tickets := []models.Ticket{}
	if err == nil && ticketResponse != nil {
		tickets = ticketResponse.Tickets
	}

	// Build HTML response
	var html strings.Builder
	html.WriteString(`<ul role="list" class="-my-5 divide-y divide-gray-200 dark:divide-gray-700">`)

	if len(tickets) == 0 {
		html.WriteString(`
                        <li class="py-4">
                            <div class="flex items-center space-x-4">
                                <div class="min-w-0 flex-1">
                                    <p class="truncate text-sm font-medium text-gray-900 dark:text-white">No recent tickets</p>
                                    <p class="truncate text-sm text-gray-500 dark:text-gray-400">No tickets found in the system</p>
                                </div>
                            </div>
                        </li>`)
	} else {
		for _, ticket := range tickets {
			// Get status label from database
			statusLabel := "unknown"
			var statusRow struct {
				Name string
			}
			query := database.ConvertPlaceholders("SELECT name FROM ticket_state WHERE id = $1")
			err = db.QueryRow(query, ticket.TicketStateID).Scan(&statusRow.Name)
			if err == nil {
				statusLabel = statusRow.Name
			}

			// Get priority name and determine CSS class
			priorityName := "normal"
			var priorityRow struct {
				Name string
			}
			query = database.ConvertPlaceholders("SELECT name FROM ticket_priority WHERE id = $1")
			err = db.QueryRow(query, ticket.TicketPriorityID).Scan(&priorityRow.Name)
			if err == nil {
				priorityName = priorityRow.Name
			}

			priorityClass := "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300"
			switch strings.ToLower(priorityName) {
			case "1 very low", "2 low":
				priorityClass = "bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-300"
			case "3 normal":
				priorityClass = "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300"
			case "4 high", "5 very high":
				priorityClass = "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300"
			case "critical":
				priorityClass = "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300"
			}

			statusClass := "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300"
			switch strings.ToLower(statusLabel) {
			case "new":
				statusClass = "bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-300"
			case "open":
				statusClass = "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300"
			case "pending":
				statusClass = "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300"
			case "closed":
				statusClass = "bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-300"
			}

			const custBadge = "px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 " +
				"text-gray-800 dark:bg-gray-900 dark:text-gray-300"
			html.WriteString(fmt.Sprintf(`
			<li class="py-4">
				<div class="flex items-start space-x-4">
					<div class="min-w-0 flex-1">
						<a href="/tickets/%s" class="text-sm font-medium text-gray-900 dark:text-white">
							%s: %s
						</a>
						<div class="mt-2 flex flex-wrap gap-1">
							<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium %s">
								%s
							</span>
							<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium %s">
								%s
							</span>
							<span class="`+custBadge+`">%s</span>
						</div>
					</div>
				</div>
			</li>`,
				ticket.TicketNumber,
				ticket.TicketNumber,
				ticket.Title,
				priorityClass,
				priorityName,
				statusClass,
				statusLabel,
				func() string {
					if ticket.CustomerUserID != nil {
						return fmt.Sprintf("Customer: %s", *ticket.CustomerUserID)
					}
					return "Customer: Unknown"
				}()))
		}
	}

	html.WriteString(`</ul>`)

	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, html.String())
}

// dashboard_queue_status returns queue status for dashboard.
func dashboard_queue_status(c *gin.Context) {
	if htmxHandlerSkipDB() {
		renderDashboardQueueStatusFallback(c)
		return
	}
	db, err := database.GetDB()
	if err != nil || db == nil {
		renderDashboardQueueStatusFallback(c)
		return
	}

	// Query queues with ticket counts by state
	rows, err := db.Query(`
		SELECT q.id, q.name,
		       SUM(CASE WHEN t.ticket_state_id = 1 THEN 1 ELSE 0 END) as new_count,
		       SUM(CASE WHEN t.ticket_state_id = 2 THEN 1 ELSE 0 END) as open_count,
		       SUM(CASE WHEN t.ticket_state_id = 3 THEN 1 ELSE 0 END) as pending_count,
		       SUM(CASE WHEN t.ticket_state_id = 4 THEN 1 ELSE 0 END) as closed_count
		FROM queue q
		LEFT JOIN ticket t ON t.queue_id = q.id
		WHERE q.valid_id = 1
		GROUP BY q.id, q.name
		ORDER BY q.name
		LIMIT 10`)

	if err != nil {
		// Return JSON error on query failure
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to load queue status",
		})
		return
	}
	defer rows.Close()

	// Build HTML response with table format
	const thClass = "px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase"
	var html strings.Builder
	html.WriteString(`<div class="mt-6">
	<div class="overflow-x-auto">
		<table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
			<thead class="bg-gray-50 dark:bg-gray-700">
				<tr>
					<th scope="col" class="` + thClass + `">Queue</th>
					<th scope="col" class="` + thClass + `">New</th>
					<th scope="col" class="` + thClass + `">Open</th>
					<th scope="col" class="` + thClass + `">Pending</th>
					<th scope="col" class="` + thClass + `">Closed</th>
					<th scope="col" class="` + thClass + `">Total</th>
				</tr>
			</thead>
			<tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">`)

	queueCount := 0
	for rows.Next() {
		var queueID int
		var queueName string
		var newCount, openCount, pendingCount, closedCount int
		if err := rows.Scan(&queueID, &queueName, &newCount, &openCount, &pendingCount, &closedCount); err != nil {
			continue
		}

		totalCount := newCount + openCount + pendingCount + closedCount

		const tdClass = "px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-white"
		const badgeClass = "inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium"
		html.WriteString(fmt.Sprintf(`
		<tr class="hover:bg-gray-50 dark:hover:bg-gray-700">
			<td class="px-6 py-4 whitespace-nowrap">
				<a href="/queues/%d" class="text-sm font-medium text-gray-900 dark:text-white">%s</a>
			</td>
			<td class="`+tdClass+`">
				<span class="`+badgeClass+` bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-300">%d</span>
			</td>
			<td class="`+tdClass+`">
				<span class="`+badgeClass+` bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300">%d</span>
			</td>
			<td class="`+tdClass+`">
				<span class="`+badgeClass+` bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-300">%d</span>
			</td>
			<td class="`+tdClass+`">
				<span class="`+badgeClass+` bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-300">%d</span>
			</td>
			<td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-white">%d</td>
		</tr>`, queueID, queueName, newCount, openCount, pendingCount, closedCount, totalCount))
		queueCount++
	}
	if err := rows.Err(); err != nil {
		log.Printf("error iterating queue rows: %v", err)
	}

	// If no queues found, show a message
	if queueCount == 0 {
		html.WriteString(`
                    <tr>
                        <td colspan="6" class="px-6 py-4 text-center text-sm text-gray-500 dark:text-gray-400">
                            No queues found
                        </td>
                    </tr>`)
	}

	html.WriteString(`
                </tbody>
            </table>
        </div>
    </div>`)

	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, html.String())
}

func renderDashboardQueueStatusFallback(c *gin.Context) {
	// Provide deterministic HTML so link checks have stable content without DB access
	const thClass = "px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase"
	const tdClass = "px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-300"
	const tdName = "px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900 dark:text-white"
	const stub = `
<div class="mt-6">
	<div class="overflow-x-auto">
		<table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
			<thead class="bg-gray-50 dark:bg-gray-700">
				<tr>
					<th scope="col" class="` + thClass + `">Queue</th>
					<th scope="col" class="` + thClass + `">New</th>
					<th scope="col" class="` + thClass + `">Open</th>
					<th scope="col" class="` + thClass + `">Pending</th>
					<th scope="col" class="` + thClass + `">Closed</th>
				</tr>
			</thead>
			<tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
				<tr>
					<td class="` + tdName + `">Raw</td>
					<td class="` + tdClass + `">2</td>
					<td class="` + tdClass + `">4</td>
					<td class="` + tdClass + `">1</td>
					<td class="` + tdClass + `">0</td>
				</tr>
				<tr>
					<td class="` + tdName + `">Support</td>
					<td class="` + tdClass + `">0</td>
					<td class="` + tdClass + `">3</td>
					<td class="` + tdClass + `">1</td>
					<td class="` + tdClass + `">5</td>
				</tr>
			</tbody>
		</table>
	</div>
</div>`
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, stub)
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
	switch v := value.(type) {
	case uint:
		return int(v)
	case uint32:
		return int(v)
	case uint64:
		if v > uint64(math.MaxInt) {
			return 0
		}
		return int(v)
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		if v > int64(math.MaxInt) || v < int64(math.MinInt) {
			return 0
		}
		return int(v)
	case float64:
		if v > float64(math.MaxInt) || v < float64(math.MinInt) {
			return 0
		}
		return int(v)
	case string:
		if id, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return id
		}
	case fmt.Stringer:
		if id, err := strconv.Atoi(strings.TrimSpace(v.String())); err == nil {
			return id
		}
	}
	return 0
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
