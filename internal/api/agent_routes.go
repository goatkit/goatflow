package api

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// formatAge formats a timestamp as a human-readable relative time.
func formatAge(t time.Time) string {
	diff := time.Since(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		if minutes := int(diff.Minutes()); minutes == 1 {
			return "1 minute ago"
		} else {
			return fmt.Sprintf("%d minutes ago", minutes)
		}
	case diff < 24*time.Hour:
		if hours := int(diff.Hours()); hours == 1 {
			return "1 hour ago"
		} else {
			return fmt.Sprintf("%d hours ago", hours)
		}
	case diff < 7*24*time.Hour:
		if days := int(diff.Hours() / 24); days == 1 {
			return "1 day ago"
		} else {
			return fmt.Sprintf("%d days ago", days)
		}
	default:
		return t.Format("2006-01-02")
	}
}

// RegisterAgentRoutes registers all agent interface routes.
func RegisterAgentRoutes(r *gin.RouterGroup, db *sql.DB) {
	// Note: Routes are now handled via YAML configuration files
	// See routes/agent/*.yaml for route definitions
}

// handleAgentTickets shows the agent's ticket list.
func handleAgentTickets(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context (middleware sets "user_id" not "userID")
		userIDInterface, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
			return
		}

		userID := uint(0)
		switch v := userIDInterface.(type) {
		case uint:
			userID = v
		case int:
			userID = uint(v)
		case float64:
			userID = uint(v)
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
			return
		}

		// Get filter parameters
		status := c.DefaultQuery("status", "not_closed")
		queue := c.DefaultQuery("queue", "all")
		assignee := c.DefaultQuery("assignee", "all")
		search := c.Query("search")

		// Pagination parameters
		pageStr := c.DefaultQuery("page", "1")
		perPageStr := c.DefaultQuery("per_page", "25")
		page, _ := strconv.Atoi(pageStr)       //nolint:errcheck // Defaults to 1
		perPage, _ := strconv.Atoi(perPageStr) //nolint:errcheck // Defaults to 25
		if page < 1 {
			page = 1
		}
		if perPage < 1 || perPage > 100 {
			perPage = 25
		}
		offset := (page - 1) * perPage

		// Parse dynamic field filters from query params (df_FieldName=value)
		dfFilters := ParseDynamicFieldFiltersFromQuery(c.Request.URL.Query())

		// Build query
		query := `
			SELECT t.id, t.tn, t.title,
				   c.login as customer,
				   cc.name as company,
				   q.name as queue,
				   ts.name as state,
				   tp.name as priority,
				   CASE
				       WHEN tp.name LIKE '%very low%' THEN '#03c4f0'
				       WHEN tp.name LIKE '%low%' THEN '#83bfc8'
				       WHEN tp.name LIKE '%normal%' THEN '#cdcdcd'
				       WHEN tp.name LIKE '%high%' THEN '#ffaaaa'
				       WHEN tp.name LIKE '%very high%' THEN '#ff505e'
				       ELSE '#666666'
				   END as priority_color,
				   u.login as assigned_to,
				   t.create_time,
				   t.change_time,
				   (SELECT COUNT(*) FROM article WHERE ticket_id = t.id) as article_count
			FROM ticket t
			LEFT JOIN customer_user c ON t.customer_user_id = c.login
			LEFT JOIN customer_company cc ON t.customer_id = cc.customer_id
			LEFT JOIN queue q ON t.queue_id = q.id
			LEFT JOIN ticket_state ts ON t.ticket_state_id = ts.id
			LEFT JOIN ticket_priority tp ON t.ticket_priority_id = tp.id
			LEFT JOIN users u ON t.responsible_user_id = u.id
			WHERE 1=1
		`

		args := []interface{}{}

		// Apply status filter
		if status == "open" {
			// Include both "new" (type_id=1) and "open" (type_id=2) tickets
			query += " AND t.ticket_state_id IN (SELECT id FROM ticket_state WHERE type_id IN (1, 2))"
		} else if status == "pending" {
			// Pending states have type_id 4 and 5
			query += " AND t.ticket_state_id IN (SELECT id FROM ticket_state WHERE type_id IN (4, 5))"
		} else if status == "closed" {
			// Closed states have type_id 3
			query += " AND t.ticket_state_id IN (SELECT id FROM ticket_state WHERE type_id = 3)"
		} else if status == "not_closed" {
			// Exclude closed state types (type_id 3)
			query += " AND t.ticket_state_id NOT IN (SELECT id FROM ticket_state WHERE type_id = 3)"
		}

		// Check if user is admin
		var isAdmin bool
		adminCheckErr := db.QueryRow(database.ConvertPlaceholders(`
			SELECT EXISTS(
				SELECT 1 FROM group_user gu
				JOIN groups g ON gu.group_id = g.id
				WHERE gu.user_id = ? AND g.name = 'admin'
			)
		`), userID).Scan(&isAdmin)
		if adminCheckErr != nil {
			isAdmin = false // Fail-safe: if we can't check, assume not admin
		}

		// Apply queue filter - SECURITY: Always validate queue access for non-admin users
		if queue != "all" {
			// SECURITY CHECK: Verify user has access to the requested queue
			if !isAdmin {
				var hasAccess bool
				accessCheckErr := db.QueryRow(database.ConvertPlaceholders(`
					SELECT EXISTS(
						SELECT 1 FROM queue q
						WHERE q.id = ?
						AND q.group_id IN (
							SELECT group_id FROM group_user WHERE user_id = ?
						)
					)
				`), queue, userID).Scan(&hasAccess)

				if accessCheckErr != nil || !hasAccess {
					// User requested a queue they don't have access to - return 403
					c.JSON(http.StatusForbidden, gin.H{
						"error": "You do not have permission to access tickets in this queue",
					})
					return
				}
			}
			query += " AND t.queue_id = ?"
			args = append(args, queue)
		} else {
			// No specific queue requested - filter by user's accessible queues (non-admin only)
			if !isAdmin {
				// Regular agents see only queues they have access to through group membership
				query += ` AND t.queue_id IN (
					SELECT DISTINCT q2.id FROM queue q2
					WHERE q2.group_id IN (
						SELECT group_id FROM group_user WHERE user_id = ?
					)
				)`
				args = append(args, userID)
			}
			// Admin sees all queues - no filter needed
		}

		// Apply assignee filter
		if assignee == "me" {
			query += " AND t.responsible_user_id = ?"
			args = append(args, userID)
		} else if assignee == "unassigned" {
			query += " AND t.responsible_user_id IS NULL"
		}

		// Apply search
		if search != "" {
			pattern := "%" + search + "%"
			query += " AND (LOWER(t.tn) LIKE LOWER(?) OR LOWER(t.title) LIKE LOWER(?) OR LOWER(c.login) LIKE LOWER(?))"
			args = append(args, pattern, pattern, pattern)
		}

		// Apply dynamic field filters
		if len(dfFilters) > 0 {
			dfSQL, dfArgs, err := BuildDynamicFieldFilterSQL(dfFilters, 0)
			if err == nil && dfSQL != "" {
				query += dfSQL
				args = append(args, dfArgs...)
			}
		}

		// Build count query for pagination (before adding ORDER BY and LIMIT)
		// Use a simpler approach: just count tickets with same WHERE clause
		countQuery := `SELECT COUNT(DISTINCT t.id) FROM ticket t
			LEFT JOIN customer_user c ON t.customer_user_id = c.login
			LEFT JOIN customer_company cc ON t.customer_id = cc.customer_id
			LEFT JOIN queue q ON t.queue_id = q.id
			LEFT JOIN ticket_state ts ON t.ticket_state_id = ts.id
			LEFT JOIN ticket_priority tp ON t.ticket_priority_id = tp.id
			LEFT JOIN users u ON t.responsible_user_id = u.id
			WHERE 1=1`

		// Find WHERE 1=1 in main query and append conditions after it
		whereIdx := strings.Index(query, "WHERE 1=1")
		if whereIdx > 0 {
			// Get everything after "WHERE 1=1" (the conditions)
			afterWhere := query[whereIdx+len("WHERE 1=1"):]
			countQuery += afterWhere
		}

		var totalCount int
		countErr := db.QueryRow(database.ConvertPlaceholders(countQuery), args...).Scan(&totalCount)
		if countErr != nil {
			log.Printf("Count query error: %v", countErr)
			totalCount = 0
		}

		// Calculate pagination info
		totalPages := (totalCount + perPage - 1) / perPage
		if totalPages < 1 {
			totalPages = 1
		}

		// Calculate page number range for pagination UI (show up to 10 pages)
		pageNumbers := make([]int, 0)
		startPage := page - 4
		if startPage < 1 {
			startPage = 1
		}
		endPage := startPage + 9
		if endPage > totalPages {
			endPage = totalPages
			startPage = endPage - 9
			if startPage < 1 {
				startPage = 1
			}
		}
		for i := startPage; i <= endPage; i++ {
			pageNumbers = append(pageNumbers, i)
		}

		// Add ordering and pagination
		sortBy := c.DefaultQuery("sort", "create_time")
		sortOrder := c.DefaultQuery("order", "desc")
		query += fmt.Sprintf(" ORDER BY t.%s %s", sanitizeSortColumn(sortBy), sortOrder)
		query += " LIMIT ?"
		args = append(args, perPage)
		query += " OFFSET ?"
		args = append(args, offset)

		// Execute query
		rows, err := db.Query(database.ConvertPlaceholders(query), args...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		tickets := []map[string]interface{}{}
		for rows.Next() {
			var ticket struct {
				ID            int
				TN            string
				Title         string
				Customer      sql.NullString
				Company       sql.NullString
				Queue         string
				State         string
				Priority      string
				PriorityColor sql.NullString
				AssignedTo    sql.NullString
				CreateTime    time.Time
				ChangeTime    time.Time
				ArticleCount  int
			}

			err := rows.Scan(&ticket.ID, &ticket.TN, &ticket.Title, &ticket.Customer,
				&ticket.Company, &ticket.Queue, &ticket.State, &ticket.Priority,
				&ticket.PriorityColor, &ticket.AssignedTo, &ticket.CreateTime,
				&ticket.ChangeTime, &ticket.ArticleCount)

			if err != nil {
				continue
			}

			tickets = append(tickets, map[string]interface{}{
				"id":             ticket.ID,
				"tn":             ticket.TN,
				"title":          ticket.Title,
				"customer":       ticket.Customer.String,
				"company":        ticket.Company.String,
				"queue":          ticket.Queue,
				"state":          ticket.State,
				"priority":       ticket.Priority,
				"priority_color": ticket.PriorityColor.String,
				"assigned_to":    ticket.AssignedTo.String,
				"age":            formatAge(ticket.CreateTime),
				"created_at_iso": ticket.CreateTime.UTC().Format(time.RFC3339),
				"last_changed":   formatAge(ticket.ChangeTime),
				"updated_at_iso": ticket.ChangeTime.UTC().Format(time.RFC3339),
				"article_count":  ticket.ArticleCount,
			})
		}
		_ = rows.Err() //nolint:errcheck // Iteration errors don't affect UI

		// Get available queues for filter
		queueRows, err := db.Query(database.ConvertPlaceholders(`
			SELECT q.id, q.name
			FROM queue q
			JOIN group_user gu ON q.group_id = gu.group_id
			WHERE gu.user_id = ?
			ORDER BY q.name
		`), userID)

		availableQueues := []map[string]interface{}{}
		if err == nil && queueRows != nil {
			defer queueRows.Close()
			for queueRows.Next() {
				var q struct {
					ID   int
					Name string
				}
				if err := queueRows.Scan(&q.ID, &q.Name); err != nil {
					continue
				}
				availableQueues = append(availableQueues, map[string]interface{}{
					"id":   fmt.Sprintf("%d", q.ID),
					"name": q.Name,
				})
			}
			_ = queueRows.Err() //nolint:errcheck // Iteration errors don't affect UI
		}

		// Get user from context for navigation display
		user := getUserFromContext(c)

		// Check if user is in admin group for Dev tab
		var isInAdminGroup bool
		adminErr := db.QueryRow(database.ConvertPlaceholders(`
			SELECT EXISTS(
				SELECT 1 FROM group_user gu
				JOIN groups g ON gu.group_id = g.id
				WHERE gu.user_id = ? AND g.name = 'admin'
			)
		`), userID).Scan(&isInAdminGroup)
		if adminErr == nil && isInAdminGroup && user != nil {
			// Set a flag in context or add to user struct if it has the field
			c.Set("isInAdminGroup", true)
		}

		// Pass the isInAdminGroup flag to template
		adminGroupFlag, _ := c.Get("isInAdminGroup") //nolint:errcheck // Defaults to nil

		// Get searchable dynamic fields for the filter UI
		searchableDFs, _ := GetFieldsForSearch() //nolint:errcheck // Defaults to empty

		// Build current DF filter values for template
		currentDFFilters := make(map[string]string)
		for _, f := range dfFilters {
			key := fmt.Sprintf("df_%s", f.FieldName)
			if f.Operator != "" && f.Operator != "eq" {
				key = fmt.Sprintf("df_%s_%s", f.FieldName, f.Operator)
			}
			currentDFFilters[key] = f.Value
		}

		getPongo2Renderer().HTML(c, http.StatusOK, "pages/agent/tickets.pongo2", pongo2.Context{
			"Title":                   "Ticket Management",
			"ActivePage":              "tickets",
			"User":                    user,
			"IsInAdminGroup":          adminGroupFlag,
			"Tickets":                 tickets,
			"AvailableQueues":         availableQueues,
			"SearchableDynamicFields": searchableDFs,
			"CurrentDFFilters":        currentDFFilters,
			"CurrentFilters": map[string]string{
				"status":   status,
				"queue":    queue,
				"assignee": assignee,
				"search":   search,
				"sort":     sortBy,
				"order":    sortOrder,
			},
			"Pagination": map[string]interface{}{
				"Page":        page,
				"PerPage":     perPage,
				"TotalCount":  totalCount,
				"TotalPages":  totalPages,
				"PrevPage":    page - 1,
				"NextPage":    page + 1,
				"PageNumbers": pageNumbers,
				"HasFirst":    page > 1,
				"HasLast":     page < totalPages,
			},
		})
	}
}

// Other handler functions would continue here...

// Helper functions.
func sanitizeSortColumn(col string) string {
	allowedColumns := map[string]bool{
		"create_time": true,
		"change_time": true,
		"tn":          true,
		"title":       true,
		"priority":    true,
	}

	if allowedColumns[col] {
		return col
	}
	return "create_time"
}

func handleTicketCustomerUsers(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID := c.Param("id")

		// Get ticket's current customer info
		var currentCustomerID, currentCustomerUserID string
		err := db.QueryRow(database.ConvertPlaceholders(`
			SELECT COALESCE(customer_id, ''), COALESCE(customer_user_id, '')
			FROM ticket WHERE id = ?
		`), ticketID).Scan(&currentCustomerID, &currentCustomerUserID)

		if err != nil {
			log.Printf("Error fetching ticket customer info: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch ticket info"})
			return
		}

		type CustomerUserOption struct {
			Login       string `json:"login"`
			Email       string `json:"email"`
			FirstName   string `json:"first_name"`
			LastName    string `json:"last_name"`
			CustomerID  string `json:"customer_id"`
			CompanyName string `json:"company_name"`
			DisplayName string `json:"display_name"`
			IsCurrent   bool   `json:"is_current"`
		}

		var customerUsers []CustomerUserOption

		// Query customer users - prioritize same company
		query := database.ConvertPlaceholders(`
			SELECT
				cu.login,
				COALESCE(cu.email, cu.login) as email,
				COALESCE(cu.first_name, '') as first_name,
				COALESCE(cu.last_name, '') as last_name,
				COALESCE(cu.customer_id, '') as customer_id,
				COALESCE(cc.name, '') as company_name
			FROM customer_user cu
			LEFT JOIN customer_company cc ON cu.customer_id = cc.customer_id
			WHERE cu.valid_id = 1
				AND (? = '' OR cu.customer_id = ?)
			ORDER BY
				CASE WHEN cu.customer_id = ? THEN 0 ELSE 1 END,
				cu.first_name, cu.last_name
			LIMIT 50
		`)

		rows, err := db.Query(query, currentCustomerID, currentCustomerID, currentCustomerID)
		if err != nil {
			log.Printf("Error querying customer users: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch customer users"})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var cu CustomerUserOption
			err := rows.Scan(&cu.Login, &cu.Email, &cu.FirstName, &cu.LastName,
				&cu.CustomerID, &cu.CompanyName)
			if err != nil {
				log.Printf("Error scanning customer user: %v", err)
				continue
			}

			// Build display name
			if cu.FirstName != "" || cu.LastName != "" {
				cu.DisplayName = fmt.Sprintf("%s %s <%s>", cu.FirstName, cu.LastName, cu.Email)
			} else {
				cu.DisplayName = cu.Email
			}

			// Mark current customer
			cu.IsCurrent = (cu.Login == currentCustomerUserID || cu.Email == currentCustomerUserID)

			customerUsers = append(customerUsers, cu)
		}
		_ = rows.Err() //nolint:errcheck // Iteration errors don't affect UI

		// If no customer users found, at least return the current one
		if len(customerUsers) == 0 && currentCustomerUserID != "" {
			customerUsers = append(customerUsers, CustomerUserOption{
				Login:       currentCustomerUserID,
				Email:       currentCustomerUserID,
				DisplayName: currentCustomerUserID,
				IsCurrent:   true,
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"success":        true,
			"customer_users": customerUsers,
			"current":        currentCustomerUserID,
		})
	}
}

func handleAgentQueues(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context (middleware sets "user_id" not "userID")
		userIDInterface, exists := c.Get("user_id")
		if !exists {
			log.Printf("handleAgentQueues: user_id not found in context")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
			return
		}

		userID := uint(0)
		switch v := userIDInterface.(type) {
		case uint:
			userID = v
		case int:
			userID = uint(v)
		case float64:
			userID = uint(v)
		default:
			log.Printf("handleAgentQueues: user_id has unexpected type %T", userIDInterface)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
			return
		}

		log.Printf("handleAgentQueues: userID = %d", userID)

		// Build query to get queues the user has access to
		query := `
			SELECT q.id, q.name, q.comments, q.valid_id,
				   COUNT(t.id) as ticket_count,
			       COUNT(CASE WHEN t.ticket_state_id IN (
			           SELECT id FROM ticket_state WHERE type_id IN (1, 2)
			       ) THEN 1 END) as open_ticket_count
			FROM queue q
			LEFT JOIN ticket t ON q.id = t.queue_id
			WHERE q.group_id IN (
				SELECT group_id FROM group_user WHERE user_id = ?
			)
			GROUP BY q.id, q.name, q.comments, q.valid_id
			ORDER BY q.name
		`

		// Execute query
		rows, err := db.Query(database.ConvertPlaceholders(query), userID)
		if err != nil {
			log.Printf("handleAgentQueues: error querying queues: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		queues := []map[string]interface{}{}
		for rows.Next() {
			var queue struct {
				ID              int
				Name            string
				Comments        sql.NullString
				ValidID         int
				TicketCount     int
				OpenTicketCount int
			}

			err := rows.Scan(&queue.ID, &queue.Name, &queue.Comments, &queue.ValidID, &queue.TicketCount, &queue.OpenTicketCount)
			if err != nil {
				log.Printf("handleAgentQueues: error scanning queue: %v", err)
				continue
			}

			queues = append(queues, map[string]interface{}{
				"ID":              queue.ID,
				"Name":            queue.Name,
				"Comments":        queue.Comments.String,
				"ValidID":         queue.ValidID,
				"TicketCount":     queue.TicketCount,
				"OpenTicketCount": queue.OpenTicketCount,
			})
		}
		_ = rows.Err() //nolint:errcheck // Iteration errors don't affect UI

		// Get user from context for navigation display
		user := getUserFromContext(c)

		// Check if user is in admin group for Dev tab
		var isInAdminGroup bool
		adminErr := db.QueryRow(database.ConvertPlaceholders(`
			SELECT EXISTS(
				SELECT 1 FROM group_user gu
				JOIN groups g ON gu.group_id = g.id
				WHERE gu.user_id = ? AND g.name = 'admin'
			)
		`), userID).Scan(&isInAdminGroup)

		if adminErr == nil && isInAdminGroup && user != nil {
			// Set a flag in context or add to user struct if it has the field
			c.Set("isInAdminGroup", true)
		}

		// Pass the isInAdminGroup flag to template
		adminGroupFlag, _ := c.Get("isInAdminGroup")

		getPongo2Renderer().HTML(c, http.StatusOK, "pages/agent/queues.pongo2", pongo2.Context{
			"Title":          "Queue Management",
			"ActivePage":     "agent",
			"User":           user,
			"IsInAdminGroup": adminGroupFlag,
			"Queues":         queues,
		})
	}
}

func handleAgentQueueView(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "TODO: Queue view"})
	}
}

func handleAgentQueueLock(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "TODO: Queue lock"})
	}
}

func handleAgentQueueUnlock(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "TODO: Queue unlock"})
	}
}

func handleAgentCustomers(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "TODO: Customer list"})
	}
}

func handleAgentCustomerView(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "TODO: Customer view"})
	}
}

func handleAgentCustomerTickets(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "TODO: Customer tickets"})
	}
}

func handleAgentSearch(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "TODO: Search form"})
	}
}

func handleAgentSearchResults(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "TODO: Search results"})
	}
}
