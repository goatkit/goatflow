package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleAdminTickets displays all tickets for admin management.
func HandleAdminTickets(c *gin.Context) {
	db, err := database.GetDB()
	if err != nil {
		getPongo2Renderer().HTML(c, http.StatusInternalServerError, "error.pongo2", pongo2.Context{
			"error": "Database connection failed",
		})
		return
	}

	// Get filter parameters
	statusFilter := c.Query("status")
	queueFilter := c.Query("queue")
	searchQuery := c.Query("search")
	page := queryInt(c, "page", 1)
	limit := 20
	offset := (page - 1) * limit

	// Build the query
	query := `
		SELECT 
			t.id, t.tn, t.title, 
			t.create_time, t.change_time,
			ts.name as state,
			tp.name as priority,
			q.name as queue,
			COALESCE(cu.login, t.customer_user_id, '') as customer,
			COALESCE(u.login, 'unassigned') as assigned_to
		FROM ticket t
		JOIN ticket_state ts ON t.ticket_state_id = ts.id
		JOIN ticket_priority tp ON t.ticket_priority_id = tp.id
		JOIN queue q ON t.queue_id = q.id
		LEFT JOIN customer_user cu ON t.customer_user_id = cu.login
		LEFT JOIN users u ON t.responsible_user_id = u.id
		WHERE 1=1
	`

	var args []interface{}
	argCount := 0

	// Apply filters
	if statusFilter != "" && statusFilter != "all" {
		argCount++
		query += fmt.Sprintf(" AND ts.name = $%d", argCount)
		args = append(args, statusFilter)
	}

	if queueFilter != "" && queueFilter != "all" {
		argCount++
		query += fmt.Sprintf(" AND q.name = $%d", argCount)
		args = append(args, queueFilter)
	}

	if searchQuery != "" {
		argCount++
		query += fmt.Sprintf(" AND (t.tn ILIKE $%d OR t.title ILIKE $%d)", argCount, argCount)
		searchStr := "%" + searchQuery + "%"
		args = append(args, searchStr)
	}

	// Count total for pagination (parameterized, cross-DB)
	countQuery := `
        SELECT COUNT(*)
        FROM ticket t
        JOIN ticket_state ts ON t.ticket_state_id = ts.id
        JOIN queue q ON t.queue_id = q.id
        WHERE 1=1
    `

	var countArgs []interface{}
	countArgCount := 0
	if statusFilter != "" && statusFilter != "all" {
		countArgCount++
		countQuery += fmt.Sprintf(" AND ts.name = $%d", countArgCount)
		countArgs = append(countArgs, statusFilter)
	}
	if queueFilter != "" && queueFilter != "all" {
		countArgCount++
		countQuery += fmt.Sprintf(" AND q.name = $%d", countArgCount)
		countArgs = append(countArgs, queueFilter)
	}
	if searchQuery != "" {
		countArgCount++
		countQuery += fmt.Sprintf(" AND (t.tn ILIKE $%d OR t.title ILIKE $%d)", countArgCount, countArgCount)
		countArgs = append(countArgs, "%"+searchQuery+"%")
	}

	var totalCount int
	if err := db.QueryRow(database.ConvertPlaceholders(countQuery), countArgs...).Scan(&totalCount); err != nil {
		totalCount = 0
	}

	// Add ordering and pagination
	query += " ORDER BY t.create_time DESC LIMIT $" + strconv.Itoa(argCount+1) + " OFFSET $" + strconv.Itoa(argCount+2)
	args = append(args, limit, offset)

	// Execute query
	rows, err := db.Query(database.ConvertPlaceholders(query), args...)
	if err != nil {
		getPongo2Renderer().HTML(c, http.StatusInternalServerError, "error.pongo2", pongo2.Context{
			"error": fmt.Sprintf("Failed to fetch tickets: %v", err),
		})
		return
	}
	defer rows.Close()

	// Collect tickets
	var tickets []map[string]interface{}
	for rows.Next() {
		var ticket struct {
			ID         int
			Number     string
			Title      string
			CreateTime string
			ChangeTime string
			State      string
			Priority   string
			Queue      string
			Customer   string
			AssignedTo string
		}

		err := rows.Scan(
			&ticket.ID, &ticket.Number, &ticket.Title,
			&ticket.CreateTime, &ticket.ChangeTime,
			&ticket.State, &ticket.Priority, &ticket.Queue,
			&ticket.Customer, &ticket.AssignedTo,
		)
		if err != nil {
			continue
		}

		tickets = append(tickets, map[string]interface{}{
			"id":          ticket.ID,
			"number":      ticket.Number,
			"title":       ticket.Title,
			"create_time": ticket.CreateTime,
			"change_time": ticket.ChangeTime,
			"state":       ticket.State,
			"priority":    ticket.Priority,
			"queue":       ticket.Queue,
			"customer":    ticket.Customer,
			"assigned_to": ticket.AssignedTo,
		})
	}
	_ = rows.Err() //nolint:errcheck // Iteration errors don't affect UI

	// Get available states for filter
	stateRows, _ := db.Query(database.ConvertPlaceholders("SELECT DISTINCT name FROM ticket_state WHERE valid_id = 1 ORDER BY name")) //nolint:errcheck // Filter data defaults to empty
	var states []string
	if stateRows != nil {
		defer stateRows.Close()
		for stateRows.Next() {
			var state string
			_ = stateRows.Scan(&state) //nolint:errcheck // Scan errors skipped for optional filter
			states = append(states, state)
		}
	}

	// Get available queues for filter
	queueRows, _ := db.Query(database.ConvertPlaceholders("SELECT DISTINCT name FROM queue WHERE valid_id = 1 ORDER BY name")) //nolint:errcheck // Filter data defaults to empty
	var queues []string
	if queueRows != nil {
		defer queueRows.Close()
		for queueRows.Next() {
			var queue string
			_ = queueRows.Scan(&queue) //nolint:errcheck // Scan errors skipped for optional filter
			queues = append(queues, queue)
		}
	}

	// Calculate pagination
	totalPages := (totalCount + limit - 1) / limit

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/admin/tickets.pongo2", pongo2.Context{
		"User":         getUserFromContext(c),
		"ActivePage":   "admin",
		"Title":        "Ticket Management",
		"tickets":      tickets,
		"states":       states,
		"queues":       queues,
		"statusFilter": statusFilter,
		"queueFilter":  queueFilter,
		"searchQuery":  searchQuery,
		"currentPage":  page,
		"totalPages":   totalPages,
		"totalCount":   totalCount,
	})
}
