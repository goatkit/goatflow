package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/utils"
)

// RegisterCustomerRoutes registers all customer portal routes
func RegisterCustomerRoutes(r *gin.RouterGroup, db *sql.DB) {
	// Note: Routes are now handled via YAML configuration files
	// See routes/customer/*.yaml for route definitions

	// Commented out - now handled by YAML routes
	// // Dashboard
	// r.GET("/dashboard", handleCustomerDashboard(db))
	//
	// // Ticket management
	// r.GET("/tickets", handleCustomerTickets(db))
	// r.GET("/tickets/new", handleCustomerNewTicket(db))
	// r.POST("/tickets/new", handleCustomerCreateTicket(db))
	// r.GET("/tickets/:id", handleCustomerTicketView(db))
	// r.POST("/tickets/:id/reply", handleCustomerTicketReply(db))
	// r.POST("/tickets/:id/close", handleCustomerCloseTicket(db))
	//
	// // Profile management
	// r.GET("/profile", handleCustomerProfile(db))
	// r.POST("/profile", handleCustomerUpdateProfile(db))
	// r.GET("/profile/password", handleCustomerPasswordForm(db))
	// r.POST("/profile/password", handleCustomerChangePassword(db))
	//
	// // Knowledge base
	// r.GET("/kb", handleCustomerKnowledgeBase(db))
	// r.GET("/kb/search", handleCustomerKBSearch(db))
	// r.GET("/kb/article/:id", handleCustomerKBArticle(db))
	//
	// // Company info (if customer belongs to company)
	// r.GET("/company", handleCustomerCompanyInfo(db))
	// r.GET("/company/users", handleCustomerCompanyUsers(db))
}

// handleCustomerDashboard shows the customer's main dashboard
func handleCustomerDashboard(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt("userID")
		username := c.GetString("username")

		// Get customer's statistics
		stats := struct {
			OpenTickets     int
			ClosedTickets   int
			TotalTickets    int
			LastTicketDate  *time.Time
			AvgResponseTime string
		}{}

		// Count open tickets for this customer
		db.QueryRow(database.ConvertPlaceholders(`
			SELECT COUNT(*) FROM ticket 
			WHERE customer_user_id = $1 
			AND ticket_state_id IN (SELECT id FROM ticket_state WHERE type_id IN (1, 2))
		`), username).Scan(&stats.OpenTickets)

		// Count closed tickets for this customer
		db.QueryRow(database.ConvertPlaceholders(`
			SELECT COUNT(*) FROM ticket 
			WHERE customer_user_id = $1 
			AND ticket_state_id IN (SELECT id FROM ticket_state WHERE type_id = 3)
		`), username).Scan(&stats.ClosedTickets)

		stats.TotalTickets = stats.OpenTickets + stats.ClosedTickets

		// Get last ticket date
		var lastDate sql.NullTime
		db.QueryRow(database.ConvertPlaceholders(`
			SELECT MAX(create_time) FROM ticket 
			WHERE customer_user_id = $1
		`), username).Scan(&lastDate)
		if lastDate.Valid {
			stats.LastTicketDate = &lastDate.Time
		}

		// Get recent tickets
		rows, _ := db.Query(database.ConvertPlaceholders(`
			SELECT t.id, t.tn, t.title, 
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
				   t.create_time,
				   t.change_time,
				   (SELECT COUNT(*) FROM article WHERE ticket_id = t.id) as article_count,
				   (SELECT COUNT(*) FROM article WHERE ticket_id = t.id AND create_by != $2) as unread_count
			FROM ticket t
			LEFT JOIN ticket_state ts ON t.ticket_state_id = ts.id
			LEFT JOIN ticket_priority tp ON t.ticket_priority_id = tp.id
			WHERE t.customer_user_id = $1
			ORDER BY t.create_time DESC
			LIMIT 10
		`), username, userID)
		defer rows.Close()

		recentTickets := []map[string]interface{}{}
		for rows.Next() {
			var ticket struct {
				ID            int
				TN            string
				Title         string
				State         string
				Priority      string
				PriorityColor sql.NullString
				CreateTime    time.Time
				ChangeTime    time.Time
				ArticleCount  int
				UnreadCount   int
			}
			rows.Scan(&ticket.ID, &ticket.TN, &ticket.Title, &ticket.State,
				&ticket.Priority, &ticket.PriorityColor, &ticket.CreateTime,
				&ticket.ChangeTime, &ticket.ArticleCount, &ticket.UnreadCount)

			recentTickets = append(recentTickets, map[string]interface{}{
				"id":             ticket.ID,
				"tn":             ticket.TN,
				"title":          ticket.Title,
				"state":          ticket.State,
				"priority":       ticket.Priority,
				"priority_color": ticket.PriorityColor.String,
				"age":            formatAge(ticket.CreateTime),
				"created_at_iso": ticket.CreateTime.UTC().Format(time.RFC3339),
				"last_changed":   formatAge(ticket.ChangeTime),
				"updated_at_iso": ticket.ChangeTime.UTC().Format(time.RFC3339),
				"article_count":  ticket.ArticleCount,
				"unread_count":   ticket.UnreadCount,
			})
		}

		// Get customer info
		var customerInfo struct {
			FirstName sql.NullString
			LastName  sql.NullString
			Email     string
			Company   sql.NullString
		}
		db.QueryRow(database.ConvertPlaceholders(`
			SELECT cu.first_name, cu.last_name, cu.email, cc.name as company
			FROM customer_user cu
			LEFT JOIN customer_company cc ON cu.customer_id = cc.customer_id
			WHERE cu.login = $1
		`), username).Scan(&customerInfo.FirstName, &customerInfo.LastName,
			&customerInfo.Email, &customerInfo.Company)

		// Get announcements/news (if any)
		announcements := []map[string]interface{}{}
		// TODO: Add announcements table/feature

		pongo2Renderer.HTML(c, http.StatusOK, "pages/customer/dashboard.pongo2", pongo2.Context{
			"Title":         "Customer Portal",
			"ActivePage":    "customer",
			"Stats":         stats,
			"RecentTickets": recentTickets,
			"CustomerInfo":  customerInfo,
			"Announcements": announcements,
		})
	}
}

// handleCustomerTickets shows the customer's ticket list
func handleCustomerTickets(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.GetString("username")
		userID := c.GetInt("userID")

		// Get filter parameters
		status := c.DefaultQuery("status", "all")
		search := c.Query("search")

		// Build query
		query := `
			SELECT t.id, t.tn, t.title,
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
				   s.name as service,
				   t.create_time,
				   t.change_time,
				   (SELECT COUNT(*) FROM article WHERE ticket_id = t.id) as article_count,
				   (SELECT COUNT(*) FROM article WHERE ticket_id = t.id AND create_by != $1) as unread_count
			FROM ticket t
			LEFT JOIN ticket_state ts ON t.ticket_state_id = ts.id
			LEFT JOIN ticket_priority tp ON t.ticket_priority_id = tp.id
			LEFT JOIN service s ON t.service_id = s.id
			WHERE t.customer_user_id = $2
		`

		args := []interface{}{userID, username}
		argCount := 2

		// Apply status filter
		if status == "open" {
			query += " AND t.ticket_state_id IN (SELECT id FROM ticket_state WHERE type_id IN (1, 2))"
		} else if status == "closed" {
			query += " AND t.ticket_state_id IN (SELECT id FROM ticket_state WHERE type_id = 3)"
		}

		// Apply search
		if search != "" {
			argCount++
			query += fmt.Sprintf(" AND (t.tn ILIKE $%d OR t.title ILIKE $%d)",
				argCount, argCount)
			args = append(args, "%"+search+"%")
		}

		// Add ordering
		sortBy := c.DefaultQuery("sort", "create_time")
		sortOrder := c.DefaultQuery("order", "desc")
		query += fmt.Sprintf(" ORDER BY t.%s %s", sanitizeSortColumn(sortBy), sortOrder)

		// Execute query
		rows, err := db.Query(query, args...)
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
				State         string
				Priority      string
				PriorityColor sql.NullString
				Service       sql.NullString
				CreateTime    time.Time
				ChangeTime    time.Time
				ArticleCount  int
				UnreadCount   int
			}

			err := rows.Scan(&ticket.ID, &ticket.TN, &ticket.Title, &ticket.State,
				&ticket.Priority, &ticket.PriorityColor, &ticket.Service,
				&ticket.CreateTime, &ticket.ChangeTime, &ticket.ArticleCount,
				&ticket.UnreadCount)

			if err != nil {
				continue
			}

			tickets = append(tickets, map[string]interface{}{
				"id":             ticket.ID,
				"tn":             ticket.TN,
				"title":          ticket.Title,
				"state":          ticket.State,
				"priority":       ticket.Priority,
				"priority_color": ticket.PriorityColor.String,
				"service":        ticket.Service.String,
				"age":            formatAge(ticket.CreateTime),
				"created_at_iso": ticket.CreateTime.UTC().Format(time.RFC3339),
				"last_changed":   formatAge(ticket.ChangeTime),
				"updated_at_iso": ticket.ChangeTime.UTC().Format(time.RFC3339),
				"article_count":  ticket.ArticleCount,
				"unread_count":   ticket.UnreadCount,
			})
		}

		pongo2Renderer.HTML(c, http.StatusOK, "pages/customer/tickets.pongo2", pongo2.Context{
			"Title":      "My Tickets",
			"ActivePage": "customer",
			"Tickets":    tickets,
			"CurrentFilters": map[string]string{
				"status": status,
				"search": search,
				"sort":   sortBy,
				"order":  sortOrder,
			},
		})
	}
}

// handleCustomerNewTicket shows the new ticket form
func handleCustomerNewTicket(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get available services for customer
		rows, _ := db.Query(database.ConvertPlaceholders(`
			SELECT id, name FROM service 
			WHERE valid_id = 1
			ORDER BY name
		`))
		defer rows.Close()

		services := []map[string]interface{}{}
		for rows.Next() {
			var service struct {
				ID   int
				Name string
			}
			rows.Scan(&service.ID, &service.Name)
			services = append(services, map[string]interface{}{
				"id":   service.ID,
				"name": service.Name,
			})
		}

		// Get priorities customer can select
		prRows, _ := db.Query(database.ConvertPlaceholders(`
			SELECT id, name FROM ticket_priority 
			WHERE valid_id = 1 AND name NOT IN ('1 very low', '5 very high')
			ORDER BY id
		`))
		defer prRows.Close()

		priorities := []map[string]interface{}{}
		for prRows.Next() {
			var priority struct {
				ID   int
				Name string
			}
			prRows.Scan(&priority.ID, &priority.Name)
			priorities = append(priorities, map[string]interface{}{
				"id":   priority.ID,
				"name": priority.Name,
			})
		}

		pongo2Renderer.HTML(c, http.StatusOK, "pages/customer/new_ticket.pongo2", pongo2.Context{
			"Title":      "Create New Ticket",
			"ActivePage": "customer",
			"Services":   services,
			"Priorities": priorities,
		})
	}
}

// handleCustomerCreateTicket creates a new ticket
func handleCustomerCreateTicket(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.GetString("username")
		userID := c.GetInt("userID")

		// Get form data
		title := c.PostForm("title")
		message := c.PostForm("message")
		serviceID := c.PostForm("service_id")
		priorityID := c.PostForm("priority_id")

		if title == "" || message == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Title and message are required"})
			return
		}

		// Generate ticket number
		tn := fmt.Sprintf("%d%02d%02d%02d%02d%02d",
			time.Now().Year(),
			time.Now().Month(),
			time.Now().Day(),
			time.Now().Hour(),
			time.Now().Minute(),
			time.Now().Second())

		// Get customer's company
		var customerID sql.NullString
		db.QueryRow(database.ConvertPlaceholders(`
			SELECT customer_id FROM customer_user WHERE login = $1
		`), username).Scan(&customerID)

		// Set defaults
		if priorityID == "" {
			priorityID = "3" // Normal priority
		}

		// Create ticket
		var ticketID int
		typeColumn := database.TicketTypeColumn()
		err := db.QueryRow(database.ConvertPlaceholders(fmt.Sprintf(`
			INSERT INTO ticket (
				tn, title, queue_id, %s, service_id,
				ticket_state_id, ticket_priority_id,
				customer_id, customer_user_id,
				create_time, create_by, change_time, change_by
			) VALUES (
				$1, $2, 1, 1, NULLIF($3, '')::integer,
				1, $4,
				$5, $6,
				NOW(), $7, NOW(), $7
			) RETURNING id
		`, typeColumn)), tn, title, serviceID, priorityID, customerID, username, userID).Scan(&ticketID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create ticket"})
			return
		}

		// Detect content type - check for HTML first, then markdown patterns
		contentType := "text/plain"
		if utils.IsHTML(message) {
			sanitizer := utils.NewHTMLSanitizer()
			message = sanitizer.Sanitize(message)
			contentType = "text/html"
		} else if utils.IsMarkdown(message) {
			// Convert markdown back to HTML for rich text preservation
			message = utils.MarkdownToHTML(message)
			sanitizer := utils.NewHTMLSanitizer()
			message = sanitizer.Sanitize(message)
			contentType = "text/html"
		}

		// Start transaction for article creation
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
			return
		}
		defer tx.Rollback()

		// Create first article
		var articleID int64
		err = tx.QueryRow(database.ConvertPlaceholders(`
			INSERT INTO article (
				ticket_id, article_sender_type_id, communication_channel_id,
				is_visible_for_customer, subject,
				create_time, create_by, change_time, change_by
			) VALUES (
				$1, 3, 1,
				1, $2,
				NOW(), $3, NOW(), $3
			) RETURNING id
		`), ticketID, title, userID).Scan(&articleID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create article"})
			return
		}

		// Insert article data in mime table
		_, err = tx.Exec(database.ConvertPlaceholders(`
			INSERT INTO article_data_mime (article_id, a_from, a_subject, a_body, a_content_type, incoming_time, create_time, create_by, change_time, change_by)
			VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP, $7, CURRENT_TIMESTAMP, $7)
		`), articleID, username, title, message, contentType, time.Now().Unix(), userID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create article data"})
			return
		}

		// Commit transaction
		if err = tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
			return
		}

		// Redirect to ticket view
		c.Redirect(http.StatusSeeOther, fmt.Sprintf("/customer/ticket/%d", ticketID))
	}
}

// handleCustomerTicketView displays a specific ticket with all its articles
func handleCustomerTicketView(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID := c.Param("id")
		username := c.GetString("username")

		// Get ticket details - ensure customer owns this ticket
		var ticket struct {
			ID            int
			TN            string
			Title         string
			State         string
			StateID       int
			Priority      string
			PriorityColor sql.NullString
			Service       sql.NullString
			Queue         string
			Owner         sql.NullString
			Responsible   sql.NullString
			CreateTime    time.Time
			ChangeTime    time.Time
		}

		err := db.QueryRow(database.ConvertPlaceholders(`
			SELECT t.id, t.tn, t.title,
			       ts.name as state, ts.id as state_id,
			       tp.name as priority, 
				   CASE 
				       WHEN tp.name LIKE '%very low%' THEN '#03c4f0'
				       WHEN tp.name LIKE '%low%' THEN '#83bfc8'
				       WHEN tp.name LIKE '%normal%' THEN '#cdcdcd'
				       WHEN tp.name LIKE '%high%' THEN '#ffaaaa'
				       WHEN tp.name LIKE '%very high%' THEN '#ff505e'
				       ELSE '#666666'
				   END as priority_color,
			       s.name as service,
			       q.name as queue,
			       ou.login as owner,
			       ru.login as responsible,
			       t.create_time, t.change_time
			FROM ticket t
			LEFT JOIN ticket_state ts ON t.ticket_state_id = ts.id
			LEFT JOIN ticket_priority tp ON t.ticket_priority_id = tp.id
			LEFT JOIN service s ON t.service_id = s.id
			LEFT JOIN queue q ON t.queue_id = q.id
			LEFT JOIN users ou ON t.user_id = ou.id
			LEFT JOIN users ru ON t.responsible_user_id = ru.id
			WHERE t.id = $1 AND t.customer_user_id = $2
		`), ticketID, username).Scan(
			&ticket.ID, &ticket.TN, &ticket.Title,
			&ticket.State, &ticket.StateID,
			&ticket.Priority, &ticket.PriorityColor,
			&ticket.Service, &ticket.Queue,
			&ticket.Owner, &ticket.Responsible,
			&ticket.CreateTime, &ticket.ChangeTime)

		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found or access denied"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}

		// Get articles for this ticket (only customer-visible ones)
		rows, _ := db.Query(database.ConvertPlaceholders(`
			SELECT a.id, adm.a_subject, adm.a_body,
			       ast.name as sender_type,
			       u.login as author,
			       cu.login as customer_author,
			       a.create_time
			FROM article a
			LEFT JOIN article_data_mime adm ON a.id = adm.article_id
			LEFT JOIN article_sender_type ast ON a.article_sender_type_id = ast.id
			LEFT JOIN users u ON a.create_by = u.id
			LEFT JOIN customer_user cu ON a.create_by = cu.id AND ast.id = 3
			WHERE a.ticket_id = $1 
			  AND a.is_visible_for_customer = 1
			ORDER BY a.create_time ASC
		`), ticket.ID)
		defer rows.Close()

		articles := []map[string]interface{}{}
		for rows.Next() {
			var article struct {
				ID             int
				Subject        string
				Body           string
				SenderType     string
				Author         sql.NullString
				CustomerAuthor sql.NullString
				CreateTime     time.Time
			}

			rows.Scan(&article.ID, &article.Subject, &article.Body,
				&article.SenderType, &article.Author, &article.CustomerAuthor,
				&article.CreateTime)

			// Determine display author
			author := "System"
			isCustomer := false
			if article.SenderType == "customer" && article.CustomerAuthor.Valid {
				author = article.CustomerAuthor.String
				isCustomer = true
			} else if article.Author.Valid {
				author = article.Author.String
			}

			articles = append(articles, map[string]interface{}{
				"id":          article.ID,
				"subject":     article.Subject,
				"body":        article.Body,
				"author":      author,
				"is_customer": isCustomer,
				"created":     formatAge(article.CreateTime),
				"create_time": article.CreateTime.Format("Jan 2, 2006 15:04"),
			})
		}

		// Check if ticket can be closed by customer
		canClose := ticket.StateID != 3 // Not already closed

		pongo2Renderer.HTML(c, http.StatusOK, "pages/customer/ticket_view.pongo2", pongo2.Context{
			"Title":      fmt.Sprintf("Ticket #%s", ticket.TN),
			"ActivePage": "customer",
			"Ticket": map[string]interface{}{
				"id":             ticket.ID,
				"tn":             ticket.TN,
				"title":          ticket.Title,
				"state":          ticket.State,
				"state_id":       ticket.StateID,
				"priority":       ticket.Priority,
				"priority_color": ticket.PriorityColor.String,
				"service":        ticket.Service.String,
				"queue":          ticket.Queue,
				"owner":          ticket.Owner.String,
				"responsible":    ticket.Responsible.String,
				"age":            formatAge(ticket.CreateTime),
				"created_at_iso": ticket.CreateTime.UTC().Format(time.RFC3339),
				"last_changed":   formatAge(ticket.ChangeTime),
				"updated_at_iso": ticket.ChangeTime.UTC().Format(time.RFC3339),
				"can_close":      canClose,
			},
			"Articles": articles,
		})
	}
}

func handleCustomerTicketReply(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID := c.Param("id")
		username := c.GetString("username")
		userID := c.GetInt("userID")

		// Verify customer owns this ticket
		var exists bool
		err := db.QueryRow(database.ConvertPlaceholders(`
			SELECT EXISTS(
				SELECT 1 FROM ticket 
				WHERE id = $1 AND customer_user_id = $2
			)
		`), ticketID, username).Scan(&exists)

		if err != nil || !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}

		// Get reply content
		message := c.PostForm("message")
		if message == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Message is required"})
			return
		}

		// Get ticket title for article subject
		var ticketTitle string
		db.QueryRow(database.ConvertPlaceholders("SELECT title FROM ticket WHERE id = $1"), ticketID).Scan(&ticketTitle)

		// Create article
		_, err = db.Exec(database.ConvertPlaceholders(`
			INSERT INTO article (
				ticket_id, article_sender_type_id, communication_channel_id,
				is_visible_for_customer, subject, body,
				create_time, create_by, change_time, change_by
			) VALUES (
				$1, 3, 1,
				1, $2, $3,
				NOW(), $4, NOW(), $4
			)
		`), ticketID, "Re: "+ticketTitle, message, userID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add reply"})
			return
		}

		// Update ticket state to open if it was pending
		db.Exec(database.ConvertPlaceholders(`
			UPDATE ticket 
			SET ticket_state_id = 4, change_time = NOW(), change_by = $2
			WHERE id = $1 AND ticket_state_id IN (6, 7)
		`), ticketID, userID)

		// Redirect back to ticket view
		c.Redirect(http.StatusSeeOther, fmt.Sprintf("/customer/tickets/%s", ticketID))
	}
}

func handleCustomerCloseTicket(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID := c.Param("id")
		username := c.GetString("username")
		userID := c.GetInt("userID")

		// Verify customer owns this ticket and it's not already closed
		var stateID int
		err := db.QueryRow(database.ConvertPlaceholders(`
			SELECT ticket_state_id FROM ticket 
			WHERE id = $1 AND customer_user_id = $2
		`), ticketID, username).Scan(&stateID)

		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}

		if stateID == 2 || stateID == 3 { // Already closed
			c.JSON(http.StatusBadRequest, gin.H{"error": "Ticket is already closed"})
			return
		}

		// Close the ticket
		_, err = db.Exec(database.ConvertPlaceholders(`
			UPDATE ticket 
			SET ticket_state_id = 2, change_time = NOW(), change_by = $2
			WHERE id = $1
		`), ticketID, userID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to close ticket"})
			return
		}

		// Add a note about closure
		db.Exec(database.ConvertPlaceholders(`
			INSERT INTO article (
				ticket_id, article_sender_type_id, communication_channel_id,
				is_visible_for_customer, subject, body,
				create_time, create_by, change_time, change_by
			) VALUES (
				$1, 3, 1,
				1, 'Ticket closed by customer', 'Customer closed this ticket.',
				NOW(), $2, NOW(), $2
			)
		`), ticketID, userID)

		// Redirect to tickets list
		c.Redirect(http.StatusSeeOther, "/customer/tickets")
	}
}

func handleCustomerProfile(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "TODO: Customer profile"})
	}
}

func handleCustomerUpdateProfile(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "TODO: Update profile"})
	}
}

func handleCustomerPasswordForm(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "TODO: Password form"})
	}
}

func handleCustomerChangePassword(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "TODO: Change password"})
	}
}

func handleCustomerKnowledgeBase(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "TODO: Knowledge base"})
	}
}

func handleCustomerKBSearch(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "TODO: KB search"})
	}
}

func handleCustomerKBArticle(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "TODO: KB article"})
	}
}

func handleCustomerCompanyInfo(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "TODO: Company info"})
	}
}

func handleCustomerCompanyUsers(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "TODO: Company users"})
	}
}
