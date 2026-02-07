package api

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/auth"
	"github.com/goatkit/goatflow/internal/config"
	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/i18n"
	"github.com/goatkit/goatflow/internal/service"
	"github.com/goatkit/goatflow/internal/sysconfig"
	"github.com/goatkit/goatflow/internal/utils"
)

// RegisterCustomerRoutes registers all customer portal routes.
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

func customerPortalConfigFromContext(c *gin.Context, db *sql.DB) sysconfig.CustomerPortalConfig {
	if cfg, ok := c.Get("customer_portal_config"); ok {
		if typed, ok := cfg.(sysconfig.CustomerPortalConfig); ok {
			return typed
		}
	}
	if cfg, err := sysconfig.LoadCustomerPortalConfig(db); err == nil {
		return cfg
	}
	return sysconfig.DefaultCustomerPortalConfig()
}

// requireCustomerAuth checks if the customer is authenticated and redirects to login if not.
// Returns true if authenticated, false if redirected (caller should return early).
func requireCustomerAuth(c *gin.Context) bool {
	username := c.GetString("username")
	role, _ := c.Get("user_role")
	if username == "" || role != "Customer" {
		accept := c.GetHeader("Accept")
		if accept == "" || strings.Contains(accept, "text/html") || strings.Contains(accept, "*/*") {
			c.Redirect(http.StatusFound, "/customer/login")
			c.Abort()
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
		}
		return false
	}
	return true
}

func withPortalContext(ctx pongo2.Context, cfg sysconfig.CustomerPortalConfig) pongo2.Context {
	if ctx == nil {
		ctx = pongo2.Context{}
	}
	ctx["Portal"] = cfg
	ctx["PortalConfig"] = cfg
	if title, ok := ctx["Title"].(string); !ok || strings.TrimSpace(title) == "" {
		ctx["Title"] = cfg.Title
	}
	return ctx
}

// getCustomerInfo fetches customer info for the given username and returns it as a simple map.
// Returns both CamelCase and snake_case keys for template compatibility.
func getCustomerInfo(db *sql.DB, username string) map[string]string {
	var custInfo struct {
		FirstName sql.NullString
		LastName  sql.NullString
		Email     string
		Company   sql.NullString
	}
	row := db.QueryRow(database.ConvertPlaceholders(`
		SELECT cu.first_name, cu.last_name, cu.email, cc.name as company
		FROM customer_user cu
		LEFT JOIN customer_company cc ON cu.customer_id = cc.customer_id
		WHERE cu.login = ?
	`), username)
	_ = row.Scan(&custInfo.FirstName, &custInfo.LastName, //nolint:errcheck
		&custInfo.Email, &custInfo.Company)

	initials := getCustomerInitials(custInfo.FirstName.String, custInfo.LastName.String)

	return map[string]string{
		// CamelCase for CustomerInfo access
		"FirstName": custInfo.FirstName.String,
		"LastName":  custInfo.LastName.String,
		"Email":     custInfo.Email,
		"Company":   custInfo.Company.String,
		"Initials":  initials,
		// snake_case for Customer access
		"first_name": custInfo.FirstName.String,
		"last_name":  custInfo.LastName.String,
		"email":      custInfo.Email,
		"company":    custInfo.Company.String,
		"initials":   initials,
	}
}

// withPortalContextAndCustomer adds portal context and customer info to the template context.
func withPortalContextAndCustomer(ctx pongo2.Context, cfg sysconfig.CustomerPortalConfig, db *sql.DB, username string) pongo2.Context {
	ctx = withPortalContext(ctx, cfg)
	customerInfo := getCustomerInfo(db, username)
	ctx["CustomerInfo"] = customerInfo
	// Only set Customer if not already set by the handler
	if _, exists := ctx["Customer"]; !exists {
		ctx["Customer"] = customerInfo
	}
	return ctx
}

// handleCustomerDashboard shows the customer's main dashboard.
func handleCustomerDashboard(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCustomerAuth(c) {
			return
		}
		username := c.GetString("username")
		cfg := customerPortalConfigFromContext(c, db)

		// Get customer's statistics
		stats := struct {
			OpenTickets     int
			ClosedTickets   int
			TotalTickets    int
			LastTicketDate  time.Time
			HasLastTicket   bool
			AvgResponseTime string
		}{}

		// Count open tickets for this customer
		row := db.QueryRow(database.ConvertPlaceholders(`
			SELECT COUNT(*) FROM ticket
			WHERE customer_user_id = ?
			AND ticket_state_id IN (SELECT id FROM ticket_state WHERE type_id IN (1, 2))
		`), username)
		_ = row.Scan(&stats.OpenTickets) //nolint:errcheck // Count defaults to 0

		// Count closed tickets for this customer
		row = db.QueryRow(database.ConvertPlaceholders(`
			SELECT COUNT(*) FROM ticket
			WHERE customer_user_id = ?
			AND ticket_state_id IN (SELECT id FROM ticket_state WHERE type_id = 3)
		`), username)
		_ = row.Scan(&stats.ClosedTickets) //nolint:errcheck // Count defaults to 0

		stats.TotalTickets = stats.OpenTickets + stats.ClosedTickets

		// Get last ticket date
		var lastDate sql.NullTime
		row = db.QueryRow(database.ConvertPlaceholders(`
			SELECT MAX(create_time) FROM ticket
			WHERE customer_user_id = ?
		`), username)
		_ = row.Scan(&lastDate) //nolint:errcheck // Defaults to null
		if lastDate.Valid {
			stats.LastTicketDate = lastDate.Time
			stats.HasLastTicket = true
		}

		// Get recent tickets
		// Note: For MySQL compatibility, placeholder order must match order of appearance in query
		// ? = userID (for unread_count subquery), ? = username (for WHERE clause)
		rows, err := db.Query(database.ConvertPlaceholders(`
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
				   (SELECT COUNT(*) FROM article WHERE ticket_id = t.id AND is_visible_for_customer = 1) as article_count,
				   0 as unread_count
			FROM ticket t
			LEFT JOIN ticket_state ts ON t.ticket_state_id = ts.id
			LEFT JOIN ticket_priority tp ON t.ticket_priority_id = tp.id
			WHERE t.customer_user_id = ?
			ORDER BY t.create_time DESC
			LIMIT 10
		`), username)
		if err != nil {
			log.Printf("handleCustomerDashboard: query error: %v", err)
		}
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
			if err := rows.Scan(&ticket.ID, &ticket.TN, &ticket.Title, &ticket.State,
				&ticket.Priority, &ticket.PriorityColor, &ticket.CreateTime,
				&ticket.ChangeTime, &ticket.ArticleCount, &ticket.UnreadCount); err != nil {
				continue
			}

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
		_ = rows.Err() //nolint:errcheck // Iteration errors don't affect UI

		// Get announcements/news (if any)
		announcements := []map[string]interface{}{}
		// TODO: Add announcements table/feature

		getPongo2Renderer().HTML(c, http.StatusOK, "pages/customer/dashboard.pongo2", withPortalContextAndCustomer(pongo2.Context{
			"Title":         cfg.Title,
			"ActivePage":    "customer",
			"Stats":         stats,
			"RecentTickets": recentTickets,
			"Announcements": announcements,
		}, cfg, db, username))
	}
}

// handleCustomerTickets shows the customer's ticket list.
func handleCustomerTickets(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCustomerAuth(c) {
			return
		}
		username := c.GetString("username")
		cfg := customerPortalConfigFromContext(c, db)

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
				   (SELECT COUNT(*) FROM article WHERE ticket_id = t.id AND is_visible_for_customer = 1) as article_count,
				   0 as unread_count
			FROM ticket t
			LEFT JOIN ticket_state ts ON t.ticket_state_id = ts.id
			LEFT JOIN ticket_priority tp ON t.ticket_priority_id = tp.id
			LEFT JOIN service s ON t.service_id = s.id
			WHERE t.customer_user_id = ?
		`

		args := []interface{}{username}

		// Apply status filter
		if status == "open" {
			query += " AND t.ticket_state_id IN (SELECT id FROM ticket_state WHERE type_id IN (1, 2))"
		} else if status == "closed" {
			query += " AND t.ticket_state_id IN (SELECT id FROM ticket_state WHERE type_id = 3)"
		}

		// Apply search
		if search != "" {
			query += " AND (LOWER(t.tn) LIKE LOWER(?) OR LOWER(t.title) LIKE LOWER(?))"
			args = append(args, "%"+search+"%", "%"+search+"%")
		}

		// Add ordering
		sortBy := c.DefaultQuery("sort", "create_time")
		sortOrder := c.DefaultQuery("order", "desc")
		query += fmt.Sprintf(" ORDER BY t.%s %s", sanitizeSortColumn(sortBy), sortOrder)

		// Execute query - adapter handles placeholder conversion and arg remapping
		rows, err := database.GetAdapter().Query(db, query, args...)
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
		_ = rows.Err() //nolint:errcheck // Iteration errors don't affect UI

		getPongo2Renderer().HTML(c, http.StatusOK, "pages/customer/tickets.pongo2", withPortalContextAndCustomer(pongo2.Context{
			"Title":      fmt.Sprintf("%s - My Tickets", cfg.Title),
			"ActivePage": "customer",
			"Tickets":    tickets,
			"CurrentFilters": map[string]string{
				"status": status,
				"search": search,
				"sort":   sortBy,
				"order":  sortOrder,
			},
		}, cfg, db, username))
	}
}

// handleCustomerNewTicket shows the new ticket form.
func handleCustomerNewTicket(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCustomerAuth(c) {
			return
		}
		username := c.GetString("username")
		cfg := customerPortalConfigFromContext(c, db)

		// Get services assigned to this customer user via service_customer_user table
		query := database.ConvertPlaceholders(`
			SELECT s.id, s.name FROM service s
			INNER JOIN service_customer_user scu ON s.id = scu.service_id
			WHERE s.valid_id = 1 AND scu.customer_user_login = ?
			ORDER BY s.name
		`)
		rows, err := db.Query(query, username)
		services := []map[string]interface{}{}
		if err == nil && rows != nil {
			defer rows.Close()
			for rows.Next() {
				var service struct {
					ID   int
					Name string
				}
				if err := rows.Scan(&service.ID, &service.Name); err != nil {
					continue
				}
				services = append(services, map[string]interface{}{
					"id":   service.ID,
					"name": service.Name,
				})
			}
			_ = rows.Err() //nolint:errcheck // Iteration errors don't affect UI
		}

		// Fall back to <DEFAULT> services if no explicit assignments and config allows it
		if len(services) == 0 {
			// Check config for Ticket::Service::Default::UnknownCustomer equivalent
			useDefaults := true // Default to true for OTRS compatibility
			if appCfg := config.Get(); appCfg != nil {
				useDefaults = appCfg.Ticket.Service.DefaultUnknownCustomer
			}

			if useDefaults {
				defaultQuery := database.ConvertPlaceholders(`
					SELECT s.id, s.name FROM service s
					INNER JOIN service_customer_user scu ON s.id = scu.service_id
					WHERE s.valid_id = 1 AND scu.customer_user_login = '<DEFAULT>'
					ORDER BY s.name
				`)
				defaultRows, err := db.Query(defaultQuery)
				if err == nil && defaultRows != nil {
					defer defaultRows.Close()

					for defaultRows.Next() {
						var service struct {
							ID   int
							Name string
						}
						if err := defaultRows.Scan(&service.ID, &service.Name); err != nil {
							continue
						}
						services = append(services, map[string]interface{}{
							"id":   service.ID,
							"name": service.Name,
						})
					}
					_ = defaultRows.Err() //nolint:errcheck // Iteration errors don't affect UI
				}
			}
		}

		// Get priorities customer can select
		prRows, err := db.Query(database.ConvertPlaceholders(`
			SELECT id, name FROM ticket_priority
			WHERE valid_id = 1 AND name NOT IN ('1 very low', '5 very high')
			ORDER BY id
		`))

		priorities := []map[string]interface{}{}
		if err == nil && prRows != nil {
			defer prRows.Close()
			for prRows.Next() {
				var priority struct {
					ID   int
					Name string
				}
				if err := prRows.Scan(&priority.ID, &priority.Name); err != nil {
					continue
				}
				priorities = append(priorities, map[string]interface{}{
					"id":   priority.ID,
					"name": priority.Name,
				})
			}
			_ = prRows.Err() //nolint:errcheck // Iteration errors don't affect UI
		}

		// Get dynamic fields for customer ticket creation
		var customerDynamicFields []FieldWithScreenConfig
		dfFields, dfErr := GetFieldsForScreenWithConfig("CustomerTicketMessage", DFObjectTicket)
		if dfErr != nil {
			log.Printf("Error getting customer ticket dynamic fields: %v", dfErr)
		} else {
			customerDynamicFields = dfFields
		}

		getPongo2Renderer().HTML(c, http.StatusOK, "pages/customer/new_ticket.pongo2", withPortalContextAndCustomer(pongo2.Context{
			"Title":                 fmt.Sprintf("%s - Create New Ticket", cfg.Title),
			"ActivePage":            "customer",
			"Services":              services,
			"Priorities":            priorities,
			"CustomerDynamicFields": customerDynamicFields,
		}, cfg, db, username))
	}
}

// handleCustomerCreateTicket creates a new ticket.
func handleCustomerCreateTicket(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCustomerAuth(c) {
			return
		}
		username := c.GetString("username")
		// For create_by/change_by we need a valid users.id; customer_user.id is not in users table
		// Use system user (id=1) for customer-created tickets
		systemUserID := 1

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
		row := db.QueryRow(database.ConvertPlaceholders(`
			SELECT customer_id FROM customer_user WHERE login = ?
		`), username)
		_ = row.Scan(&customerID) //nolint:errcheck // Defaults to empty

		// Set defaults
		if priorityID == "" {
			priorityID = "3" // Normal priority
		}

		// Create ticket
		var ticketID int64
		typeColumn := database.TicketTypeColumn()

		// Handle empty serviceID
		var serviceIDVal interface{}
		if serviceID == "" {
			serviceIDVal = nil
		} else {
			serviceIDVal = serviceID
		}

		result, err := db.Exec(database.ConvertPlaceholders(fmt.Sprintf(`
			INSERT INTO ticket (
				tn, title, queue_id, %s, service_id,
				ticket_state_id, ticket_priority_id, ticket_lock_id,
				user_id, responsible_user_id,
				customer_id, customer_user_id,
				timeout, until_time,
				escalation_time, escalation_update_time, escalation_response_time, escalation_solution_time,
				create_time, create_by, change_time, change_by
			) VALUES (
				?, ?, 1, 1, ?,
				1, ?, 1,
				?, ?,
				?, ?,
				0, 0,
				0, 0, 0, 0,
				NOW(), ?, NOW(), ?
			)
		`, typeColumn)), tn, title, serviceIDVal, priorityID, systemUserID, systemUserID, customerID, username, systemUserID, systemUserID)

		if err != nil {
			log.Printf("Customer create ticket error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create ticket"})
			return
		}

		ticketID, err = result.LastInsertId()
		if err != nil {
			log.Printf("Customer create ticket error (get ID): %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get ticket ID"})
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
		defer func() { _ = tx.Rollback() }()

		// Create first article (OTRS schema: subject/body are in article_data_mime)
		var articleID int64
		articleResult, err := tx.Exec(database.ConvertPlaceholders(`
			INSERT INTO article (
				ticket_id, article_sender_type_id, communication_channel_id,
				is_visible_for_customer, search_index_needs_rebuild,
				create_time, create_by, change_time, change_by
			) VALUES (
				?, 3, 1,
				1, 1,
				NOW(), ?, NOW(), ?
			)
		`), ticketID, systemUserID, systemUserID)

		if err != nil {
			log.Printf("Customer create article error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create article"})
			return
		}

		articleID, err = articleResult.LastInsertId()
		if err != nil {
			log.Printf("Customer create article error (get ID): %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get article ID"})
			return
		}

		// Insert article data in mime table
		_, err = tx.Exec(database.ConvertPlaceholders(`
			INSERT INTO article_data_mime (
				article_id, a_from, a_subject, a_body, a_content_type, incoming_time,
				create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, ?)
		`), articleID, username, title, message, contentType, time.Now().Unix(), systemUserID, systemUserID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create article data"})
			return
		}

		// Commit transaction
		if err = tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
			return
		}

		// Process dynamic fields from customer create form
		if c.Request.PostForm != nil {
			if dfErr := ProcessDynamicFieldsFromForm(
				c.Request.PostForm, int(ticketID), DFObjectTicket, "CustomerTicketMessage"); dfErr != nil {
				log.Printf("WARNING: Failed to process dynamic fields for customer ticket %d: %v",
					ticketID, dfErr)
			}
		}

		// Process attachments from form
		if err := c.Request.ParseMultipartForm(10 << 20); err == nil && c.Request.MultipartForm != nil {
			files := getFormFiles(c.Request.MultipartForm)
			if len(files) > 0 {
				processFormAttachments(files, attachmentProcessParams{
					ctx:       context.Background(),
					db:        db,
					ticketID:  int(ticketID),
					articleID: int(articleID),
					userID:    systemUserID,
				})
			}
		}

		// Redirect to ticket view
		c.Redirect(http.StatusSeeOther, fmt.Sprintf("/customer/tickets/%d", ticketID))
	}
}

// handleCustomerTicketView displays a specific ticket with all its articles.
func handleCustomerTicketView(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCustomerAuth(c) {
			return
		}
		ticketID := c.Param("id")
		username := c.GetString("username")
		cfg := customerPortalConfigFromContext(c, db)

		// Get ticket details - ensure customer owns this ticket
		var ticket struct {
			ID            int
			TN            string
			Title         string
			State         string
			StateID       int
			StateTypeID   int
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
			       ts.name as state, ts.id as state_id, ts.type_id as state_type_id,
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
			WHERE t.id = ? AND t.customer_user_id = ?
		`), ticketID, username).Scan(
			&ticket.ID, &ticket.TN, &ticket.Title,
			&ticket.State, &ticket.StateID, &ticket.StateTypeID,
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
		rows, err := db.Query(database.ConvertPlaceholders(`
			SELECT a.id, adm.a_subject, adm.a_body,
			       ast.name as sender_type,
			       u.login as author,
			       adm.a_from as customer_from,
			       a.create_time
			FROM article a
			LEFT JOIN article_data_mime adm ON a.id = adm.article_id
			LEFT JOIN article_sender_type ast ON a.article_sender_type_id = ast.id
			LEFT JOIN users u ON a.create_by = u.id
			WHERE a.ticket_id = ?
			  AND a.is_visible_for_customer = 1
			ORDER BY a.create_time ASC
		`), ticket.ID)

		articles := []map[string]interface{}{}
		if err == nil && rows != nil {
			defer rows.Close()
			for rows.Next() {
				var article struct {
					ID           int
					Subject      sql.NullString
					Body         sql.NullString
					SenderType   string
					Author       sql.NullString
					CustomerFrom sql.NullString
					CreateTime   time.Time
				}

				if err := rows.Scan(&article.ID, &article.Subject, &article.Body,
					&article.SenderType, &article.Author, &article.CustomerFrom,
					&article.CreateTime); err != nil {
					continue
				}

				// Determine display author
				author := "System"
				isCustomer := false
				if article.SenderType == "customer" {
					isCustomer = true
					if article.CustomerFrom.Valid && article.CustomerFrom.String != "" {
						author = article.CustomerFrom.String
					}
				} else if article.Author.Valid {
					author = article.Author.String
				}

				articles = append(articles, map[string]interface{}{
					"id":          article.ID,
					"subject":     article.Subject.String,
					"body":        article.Body.String,
					"author":      author,
					"is_customer": isCustomer,
					"created":     formatAge(article.CreateTime),
					"create_time": article.CreateTime.Format("Jan 2, 2006 15:04"),
				})
			}
			_ = rows.Err() //nolint:errcheck // Iteration errors don't affect UI
		}

		// Note: OTRS doesn't track article-level "Seen" status for customers.
		// The article_flag table has a FK constraint to users.id, not customer_user.id.
		// Customer "new message" tracking would require schema changes.

		// Check if ticket can be closed by customer (type_id 3 = closed states)
		canClose := ticket.StateTypeID != 3

		// Get dynamic field values for display on customer ticket view
		var dynamicFieldsDisplay []DynamicFieldDisplay
		dfDisplay, dfErr := GetDynamicFieldValuesForDisplay(ticket.ID, DFObjectTicket, "CustomerTicketZoom")
		if dfErr != nil {
			log.Printf("Error getting dynamic field values for customer ticket %d: %v", ticket.ID, dfErr)
		} else {
			dynamicFieldsDisplay = dfDisplay
		}

		// Get article dynamic fields for customer reply form
		var replyArticleDynamicFields []FieldWithScreenConfig
		replyDFs, replyDFErr := GetFieldsForScreenWithConfig("CustomerArticleReply", DFObjectArticle)
		if replyDFErr != nil {
			log.Printf("Error getting article dynamic fields for customer reply: %v", replyDFErr)
		} else {
			replyArticleDynamicFields = replyDFs
		}

		getPongo2Renderer().HTML(c, http.StatusOK, "pages/customer/ticket_view.pongo2", withPortalContextAndCustomer(pongo2.Context{
			"Title":      fmt.Sprintf("%s - Ticket #%s", cfg.Title, ticket.TN),
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
			"Articles":                  articles,
			"DynamicFields":             dynamicFieldsDisplay,
			"ReplyArticleDynamicFields": replyArticleDynamicFields,
		}, cfg, db, username))
	}
}

func handleCustomerTicketReply(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCustomerAuth(c) {
			return
		}
		ticketID := c.Param("id")
		username := c.GetString("username")
		// For create_by/change_by we need a valid users.id
		systemUserID := 1

		// Verify customer owns this ticket
		var exists bool
		err := db.QueryRow(database.ConvertPlaceholders(`
			SELECT EXISTS(
				SELECT 1 FROM ticket
				WHERE id = ? AND customer_user_id = ?
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

		// Get ticket title and customer email for article
		var ticketTitle, customerEmail string
		_ = db.QueryRow(database.ConvertPlaceholders("SELECT title FROM ticket WHERE id = ?"), ticketID).Scan(&ticketTitle)             //nolint:errcheck // Defaults to empty
		_ = db.QueryRow(database.ConvertPlaceholders("SELECT email FROM customer_user WHERE login = ?"), username).Scan(&customerEmail) //nolint:errcheck // Defaults to empty

		// Create article (OTRS schema: article + article_data_mime)
		// article_sender_type_id: 3 = customer
		// communication_channel_id: 1 = email (Internal is typically 5, but we use 1 for customer web replies)
		result, err := db.Exec(database.ConvertPlaceholders(`
			INSERT INTO article (
				ticket_id, article_sender_type_id, communication_channel_id,
				is_visible_for_customer, search_index_needs_rebuild,
				create_time, create_by, change_time, change_by
			) VALUES (
				?, 3, 1,
				1, 1,
				NOW(), ?, NOW(), ?
			)
		`), ticketID, systemUserID, systemUserID)

		if err != nil {
			log.Printf("Customer reply error (article insert): %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add reply"})
			return
		}

		articleID, err := result.LastInsertId()
		if err != nil {
			log.Printf("Customer reply error (get article ID): %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get article ID"})
			return
		}

		// Insert article_data_mime with subject and body
		_, err = db.Exec(database.ConvertPlaceholders(`
			INSERT INTO article_data_mime (
				article_id, a_from, a_subject, a_body, a_content_type,
				incoming_time, create_time, create_by, change_time, change_by
			) VALUES (
				?, ?, ?, ?, 'text/plain; charset=utf-8',
				UNIX_TIMESTAMP(), NOW(), ?, NOW(), ?
			)
		`), articleID, customerEmail, "Re: "+ticketTitle, message, systemUserID, systemUserID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add reply content"})
			return
		}

		// Save article dynamic fields for customer reply
		_ = c.Request.ParseForm() //nolint:errcheck // Form already parsed by handler
		if dfErr := ProcessArticleDynamicFieldsFromForm(c.Request.PostForm, int(articleID), "CustomerArticleReply"); dfErr != nil {
			log.Printf("Error saving article dynamic fields for customer reply: %v", dfErr)
		}

		// Process attachments from reply form
		if err := c.Request.ParseMultipartForm(10 << 20); err == nil && c.Request.MultipartForm != nil {
			files := getFormFiles(c.Request.MultipartForm)
			if len(files) > 0 {
				// Convert ticketID string to int for attachment processing
				var ticketIDInt int
				fmt.Sscanf(ticketID, "%d", &ticketIDInt)
				processFormAttachments(files, attachmentProcessParams{
					ctx:       context.Background(),
					db:        db,
					ticketID:  ticketIDInt,
					articleID: int(articleID),
					userID:    systemUserID,
				})
			}
		}

		// Update ticket state to open if it was pending
		//nolint:errcheck // Best-effort state update
		_, _ = db.Exec(database.ConvertPlaceholders(`
			UPDATE ticket
			SET ticket_state_id = 4, change_time = NOW(), change_by = ?
			WHERE id = ? AND ticket_state_id IN (6, 7)
		`), ticketID, systemUserID)

		// Redirect back to ticket view
		c.Redirect(http.StatusSeeOther, fmt.Sprintf("/customer/tickets/%s", ticketID))
	}
}

func handleCustomerCloseTicket(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCustomerAuth(c) {
			return
		}
		ticketID := c.Param("id")
		username := c.GetString("username")
		// For create_by/change_by we need a valid users.id
		systemUserID := 1

		// Verify customer owns this ticket and it's not already closed
		var stateID int
		err := db.QueryRow(database.ConvertPlaceholders(`
			SELECT ticket_state_id FROM ticket
			WHERE id = ? AND customer_user_id = ?
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

		// Close the ticket (args must match SQL text order: change_by first, then id)
		_, err = db.Exec(database.ConvertPlaceholders(`
			UPDATE ticket
			SET ticket_state_id = 2, change_time = NOW(), change_by = ?
			WHERE id = ?
		`), systemUserID, ticketID)

		if err != nil {
			log.Printf("Failed to close ticket %s: %v", ticketID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to close ticket: " + err.Error()})
			return
		}

		// Add a note about closure - insert into article table first
		result, err := db.Exec(database.ConvertPlaceholders(`
			INSERT INTO article (
				ticket_id, article_sender_type_id, communication_channel_id,
				is_visible_for_customer,
				create_time, create_by, change_time, change_by
			) VALUES (
				?, 3, 1,
				1,
				NOW(), ?, NOW(), ?
			)
		`), ticketID, systemUserID, systemUserID)

		if err == nil {
			if articleID, err := result.LastInsertId(); err == nil {
				// Insert article content into article_data_mime (incoming_time is required)
				//nolint:errcheck // Best-effort article data insert
				_, _ = db.Exec(database.ConvertPlaceholders(`
					INSERT INTO article_data_mime (
						article_id, a_from, a_to, a_subject, a_body, a_content_type,
						incoming_time, create_time, create_by, change_time, change_by
					) VALUES (
						?, 'Customer', '', 'Ticket closed by customer', 'Customer closed this ticket.', 'text/plain',
						UNIX_TIMESTAMP(), NOW(), ?, NOW(), ?
					)
				`), articleID, systemUserID, systemUserID)
			}
		}

		// Redirect to tickets list
		c.Redirect(http.StatusSeeOther, "/customer/tickets")
	}
}

func handleCustomerProfile(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCustomerAuth(c) {
			return
		}
		username := c.GetString("username")
		cfg := customerPortalConfigFromContext(c, db)

		// Get customer user details
		var customer struct {
			ID        int
			Login     string
			Email     string
			Title     sql.NullString
			FirstName string
			LastName  string
			Phone     sql.NullString
			Mobile    sql.NullString
		}

		err := db.QueryRow(database.ConvertPlaceholders(`
			SELECT id, login, email, title, first_name, last_name, phone, mobile
			FROM customer_user
			WHERE login = ?
		`), username).Scan(
			&customer.ID, &customer.Login, &customer.Email,
			&customer.Title, &customer.FirstName, &customer.LastName,
			&customer.Phone, &customer.Mobile)

		if err != nil {
			log.Printf("Error loading customer profile for %s: %v", username, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load profile"})
			return
		}

		// Generate initials for avatar
		initials := getCustomerInitials(customer.FirstName, customer.LastName)

		getPongo2Renderer().HTML(c, http.StatusOK, "pages/customer/profile.pongo2", withPortalContextAndCustomer(pongo2.Context{
			"Title":      fmt.Sprintf("%s - %s", cfg.Title, "Profile"),
			"ActivePage": "profile",
			"Customer": map[string]interface{}{
				"id":         customer.ID,
				"login":      customer.Login,
				"email":      customer.Email,
				"title":      customer.Title.String,
				"first_name": customer.FirstName,
				"last_name":  customer.LastName,
				"phone":      customer.Phone.String,
				"mobile":     customer.Mobile.String,
				"initials":   initials,
			},
		}, cfg, db, username))
	}
}

// getCustomerInitials generates initials from first and last name
func getCustomerInitials(firstName, lastName string) string {
	if firstName == "" && lastName == "" {
		return "?"
	}
	if firstName == "" {
		return strings.ToUpper(lastName[:1])
	}
	if lastName == "" {
		return strings.ToUpper(firstName[:1])
	}
	return strings.ToUpper(firstName[:1] + lastName[:1])
}

func handleCustomerUpdateProfile(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCustomerAuth(c) {
			return
		}
		username := c.GetString("username")

		var request struct {
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Title     string `json:"title"`
			Phone     string `json:"phone"`
			Mobile    string `json:"mobile"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid request format",
			})
			return
		}

		// Validate required fields
		if request.FirstName == "" || request.LastName == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "First name and last name are required",
			})
			return
		}

		// Update customer profile
		_, err := db.Exec(database.ConvertPlaceholders(`
			UPDATE customer_user
			SET first_name = ?, last_name = ?, title = ?, phone = ?, mobile = ?, change_time = NOW()
			WHERE login = ?
		`), request.FirstName, request.LastName, request.Title, request.Phone, request.Mobile, username)

		if err != nil {
			log.Printf("Error updating customer profile for %s: %v", username, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to update profile",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Profile updated successfully",
		})
	}
}

func handleCustomerGetLanguage(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCustomerAuth(c) {
			return
		}
		username := c.GetString("username")

		prefService := service.NewCustomerPreferencesService(db)
		lang := prefService.GetLanguage(username)

		// Build list of available languages with display names
		availableLanguages := i18n.GetInstance().GetSupportedLanguages()
		languageList := make([]gin.H, 0, len(availableLanguages))
		for _, code := range availableLanguages {
			if config, exists := i18n.GetLanguageConfig(code); exists {
				languageList = append(languageList, gin.H{
					"code":        code,
					"name":        config.Name,
					"native_name": config.NativeName,
				})
			} else {
				languageList = append(languageList, gin.H{
					"code":        code,
					"name":        code,
					"native_name": code,
				})
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"value":     lang,
			"available": languageList,
		})
	}
}

func handleCustomerSetLanguage(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCustomerAuth(c) {
			return
		}
		username := c.GetString("username")

		var request struct {
			Value string `json:"value"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid request format",
			})
			return
		}

		// Validate language is supported (empty is allowed - means system default)
		if request.Value != "" {
			instance := i18n.GetInstance()
			supported := false
			for _, lang := range instance.GetSupportedLanguages() {
				if lang == request.Value {
					supported = true
					break
				}
			}
			if !supported {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error":   "Unsupported language: " + request.Value,
				})
				return
			}
		}

		prefService := service.NewCustomerPreferencesService(db)
		if err := prefService.SetLanguage(username, request.Value); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to save preference",
			})
			return
		}

		// Set/clear cookie to reflect preference immediately
		if request.Value != "" {
			c.SetCookie("lang", request.Value, 86400*30, "/", "", false, true)
		} else {
			c.SetCookie("lang", "", -1, "/", "", false, true)
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Language preference saved successfully",
		})
	}
}

func handleCustomerGetSessionTimeout(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCustomerAuth(c) {
			return
		}
		username := c.GetString("username")

		prefService := service.NewCustomerPreferencesService(db)
		timeout := prefService.GetSessionTimeout(username)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"value":   timeout,
		})
	}
}

func handleCustomerSetSessionTimeout(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCustomerAuth(c) {
			return
		}
		username := c.GetString("username")

		var request struct {
			Value int `json:"value"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid request format",
			})
			return
		}

		prefService := service.NewCustomerPreferencesService(db)
		if err := prefService.SetSessionTimeout(username, request.Value); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to save preference",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Session timeout preference saved successfully",
		})
	}
}

func handleCustomerPasswordForm(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCustomerAuth(c) {
			return
		}
		username := c.GetString("username")
		cfg := customerPortalConfigFromContext(c, db)

		// Load password policy from sysconfig
		policy, err := sysconfig.LoadCustomerPasswordPolicy(db)
		if err != nil {
			log.Printf("Error loading customer password policy: %v", err)
			// Continue with default policy (all disabled)
			policy = sysconfig.DefaultCustomerPasswordPolicy()
		}

		getPongo2Renderer().HTML(c, http.StatusOK, "pages/customer/password_form.pongo2", withPortalContextAndCustomer(pongo2.Context{
			"Title":      fmt.Sprintf("%s - %s", cfg.Title, "Change Password"),
			"ActivePage": "profile",
			"Policy":     policy,
		}, cfg, db, username))
	}
}

func handleCustomerChangePassword(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requireCustomerAuth(c) {
			return
		}
		username := c.GetString("username")

		var request struct {
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
			ConfirmPassword string `json:"confirm_password"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid request format",
			})
			return
		}

		// Validate required fields
		if request.CurrentPassword == "" || request.NewPassword == "" || request.ConfirmPassword == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "All password fields are required",
			})
			return
		}

		// Verify passwords match
		if request.NewPassword != request.ConfirmPassword {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Passwords do not match",
			})
			return
		}

		// Get current password hash from database
		var currentHash string
		err := db.QueryRow(database.ConvertPlaceholders(`
			SELECT pw FROM customer_user WHERE login = ?
		`), username).Scan(&currentHash)

		if err != nil {
			log.Printf("Error getting customer password for %s: %v", username, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to verify current password",
			})
			return
		}

		// Verify current password using auth package
		hasher := auth.NewPasswordHasher()
		if !hasher.VerifyPassword(request.CurrentPassword, currentHash) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Current password is incorrect",
			})
			return
		}

		// Check that new password is different from current
		if request.NewPassword == request.CurrentPassword {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "New password must be different from current password",
			})
			return
		}

		// Load and validate against password policy
		policy, err := sysconfig.LoadCustomerPasswordPolicy(db)
		if err != nil {
			log.Printf("Error loading customer password policy: %v", err)
			// Continue with default policy
			policy = sysconfig.DefaultCustomerPasswordPolicy()
		}

		if validationErr := policy.ValidatePassword(request.NewPassword); validationErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   getPasswordPolicyErrorMessage(validationErr.Code),
			})
			return
		}

		// Hash the new password
		newHash, err := hasher.HashPassword(request.NewPassword)
		if err != nil {
			log.Printf("Error hashing new password for customer %s: %v", username, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to process new password",
			})
			return
		}

		// Update password in database
		_, err = db.Exec(database.ConvertPlaceholders(`
			UPDATE customer_user
			SET pw = ?, change_time = NOW()
			WHERE login = ?
		`), newHash, username)

		if err != nil {
			log.Printf("Error updating password for customer %s: %v", username, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to update password",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Password changed successfully",
		})
	}
}

// getPasswordPolicyErrorMessage returns a user-friendly error message for password policy validation errors.
func getPasswordPolicyErrorMessage(code string) string {
	switch code {
	case "regexp_mismatch":
		return "Password does not meet the required pattern"
	case "min_size":
		return "Password is too short"
	case "min_2_lower_2_upper":
		return "Password must contain at least 2 uppercase and 2 lowercase letters"
	case "need_digit":
		return "Password must contain at least 1 number"
	case "min_2_characters":
		return "Password must contain at least 2 letters"
	default:
		return "Password does not meet the security requirements"
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
