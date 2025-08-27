package api

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
)

// RegisterAgentRoutes registers all agent interface routes
func RegisterAgentRoutes(r *gin.RouterGroup, db *sql.DB) {
	// Note: Routes are now handled via YAML configuration files
	// See routes/agent/*.yaml for route definitions
	
	// Commented out - now handled by YAML routes
	// // Dashboard
	// r.GET("/dashboard", handleAgentDashboard(db))
	// 
	// // Ticket management
	// r.GET("/tickets", handleAgentTickets(db))
	// r.GET("/tickets/:id", handleAgentTicketView(db))
	// r.POST("/tickets/:id/reply", handleAgentTicketReply(db))
	// r.POST("/tickets/:id/note", handleAgentTicketNote(db))
	// r.PUT("/tickets/:id/status", handleAgentTicketStatus(db))
	// r.PUT("/tickets/:id/assign", handleAgentTicketAssign(db))
	// r.PUT("/tickets/:id/priority", handleAgentTicketPriority(db))
	// 
	// // Queue management
	// r.GET("/queues", handleAgentQueues(db))
	// r.GET("/queues/:id", handleAgentQueueView(db))
	// r.POST("/queues/:id/lock", handleAgentQueueLock(db))
	// r.POST("/queues/:id/unlock", handleAgentQueueUnlock(db))
	// 
	// // Customer interaction
	// r.GET("/customers", handleAgentCustomers(db))
	// r.GET("/customers/:id", handleAgentCustomerView(db))
	// r.GET("/customers/:id/tickets", handleAgentCustomerTickets(db))
	// 
	// // Search
	// r.GET("/search", handleAgentSearch(db))
	// r.POST("/search", handleAgentSearchResults(db))
}

// handleAgentDashboard shows the agent's main dashboard
func handleAgentDashboard(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt("userID")
		
		// Get agent's statistics
		stats := struct {
			OpenTickets     int
			PendingTickets  int
			ClosedToday     int
			NewToday        int
			MyTickets       int
			UnassignedInMyQueues int
		}{}
		
		// Count open tickets assigned to this agent
		db.QueryRow(`
			SELECT COUNT(*) FROM ticket 
			WHERE responsible_user_id = $1 
			AND ticket_state_id IN (SELECT id FROM ticket_state WHERE type_id = 1)
		`, userID).Scan(&stats.OpenTickets)
		
		// Count pending tickets assigned to this agent
		db.QueryRow(`
			SELECT COUNT(*) FROM ticket 
			WHERE responsible_user_id = $1 
			AND ticket_state_id IN (SELECT id FROM ticket_state WHERE type_id = 2)
		`, userID).Scan(&stats.PendingTickets)
		
		// Count tickets closed today by this agent
		db.QueryRow(`
			SELECT COUNT(*) FROM ticket 
			WHERE change_by = $1 
			AND ticket_state_id IN (SELECT id FROM ticket_state WHERE type_id = 3)
			AND DATE(change_time) = CURRENT_DATE
		`, userID).Scan(&stats.ClosedToday)
		
		// Count new tickets today in agent's queues
		db.QueryRow(`
			SELECT COUNT(*) FROM ticket t
			JOIN queue q ON t.queue_id = q.id
			JOIN group_user gu ON q.group_id = gu.group_id
			WHERE gu.user_id = $1
			AND DATE(t.create_time) = CURRENT_DATE
		`, userID).Scan(&stats.NewToday)
		
		// Count all tickets assigned to this agent
		stats.MyTickets = stats.OpenTickets + stats.PendingTickets
		
		// Count unassigned tickets in agent's queues
		db.QueryRow(`
			SELECT COUNT(*) FROM ticket t
			JOIN queue q ON t.queue_id = q.id
			JOIN group_user gu ON q.group_id = gu.group_id
			WHERE gu.user_id = $1
			AND t.responsible_user_id IS NULL
			AND t.ticket_state_id IN (SELECT id FROM ticket_state WHERE type_id IN (1, 2))
		`, userID).Scan(&stats.UnassignedInMyQueues)
		
		// Get recent tickets
		rows, _ := db.Query(`
			SELECT t.id, t.tn, t.title, 
				   c.login as customer,
				   q.name as queue,
				   ts.name as state,
				   tp.name as priority,
				   t.create_time
			FROM ticket t
			LEFT JOIN customer_user c ON t.customer_user_id = c.login
			LEFT JOIN queue q ON t.queue_id = q.id
			LEFT JOIN ticket_state ts ON t.ticket_state_id = ts.id
			LEFT JOIN ticket_priority tp ON t.ticket_priority_id = tp.id
			WHERE t.responsible_user_id = $1
			OR (t.responsible_user_id IS NULL AND q.id IN (
				SELECT q2.id FROM queue q2
				JOIN group_user gu ON q2.group_id = gu.group_id
				WHERE gu.user_id = $1
			))
			ORDER BY t.create_time DESC
			LIMIT 10
		`, userID)
		defer rows.Close()
		
		recentTickets := []map[string]interface{}{}
		for rows.Next() {
			var ticket struct {
				ID         int
				TN         string
				Title      string
				Customer   sql.NullString
				Queue      string
				State      string
				Priority   string
				CreateTime time.Time
			}
			rows.Scan(&ticket.ID, &ticket.TN, &ticket.Title, &ticket.Customer,
				&ticket.Queue, &ticket.State, &ticket.Priority, &ticket.CreateTime)
			
			recentTickets = append(recentTickets, map[string]interface{}{
				"id":       ticket.ID,
				"tn":       ticket.TN,
				"title":    ticket.Title,
				"customer": ticket.Customer.String,
				"queue":    ticket.Queue,
				"state":    ticket.State,
				"priority": ticket.Priority,
				"age":      formatAge(ticket.CreateTime),
			})
		}
		
		// Get agent's queues
		queueRows, _ := db.Query(`
			SELECT q.id, q.name, 
				   COUNT(t.id) as ticket_count,
				   COUNT(CASE WHEN t.responsible_user_id IS NULL THEN 1 END) as unassigned_count
			FROM queue q
			JOIN group_user gu ON q.group_id = gu.group_id
			LEFT JOIN ticket t ON q.id = t.queue_id 
				AND t.ticket_state_id IN (SELECT id FROM ticket_state WHERE type_id IN (1, 2))
			WHERE gu.user_id = $1
			GROUP BY q.id, q.name
			ORDER BY q.name
		`, userID)
		defer queueRows.Close()
		
		queues := []map[string]interface{}{}
		for queueRows.Next() {
			var queue struct {
				ID              int
				Name            string
				TicketCount     int
				UnassignedCount int
			}
			queueRows.Scan(&queue.ID, &queue.Name, &queue.TicketCount, &queue.UnassignedCount)
			
			queues = append(queues, map[string]interface{}{
				"id":               queue.ID,
				"name":             queue.Name,
				"ticket_count":     queue.TicketCount,
				"unassigned_count": queue.UnassignedCount,
			})
		}
		
		// Get user from context for navigation display
		user := getUserFromContext(c)
		
		// Check if user is in admin group for Dev tab
		var isInAdminGroup bool
		adminErr := db.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM group_user gu
				JOIN groups g ON gu.group_id = g.id
				WHERE gu.user_id = $1 AND g.name = 'admin'
			)
		`, userID).Scan(&isInAdminGroup)
		if adminErr == nil && isInAdminGroup && user != nil {
			// Set a flag in context or add to user struct if it has the field
			c.Set("isInAdminGroup", true)
		}
		
		// Pass the isInAdminGroup flag to template
		adminGroupFlag, _ := c.Get("isInAdminGroup")
		
		pongo2Renderer.HTML(c, http.StatusOK, "pages/agent/dashboard.pongo2", pongo2.Context{
			"Title":           "Agent Dashboard",
			"ActivePage":      "agent",
			"User":            user,
			"IsInAdminGroup":  adminGroupFlag,
			"Stats":           stats,
			"RecentTickets":   recentTickets,
			"Queues":          queues,
		})
	}
}

// handleAgentTickets shows the agent's ticket list
func handleAgentTickets(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context (middleware sets "user_id" not "userID")
		userIDInterface, exists := c.Get("user_id")
		if !exists {
			log.Printf("handleAgentTickets: user_id not found in context")
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
			log.Printf("handleAgentTickets: user_id has unexpected type %T", userIDInterface)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
			return
		}
		
		log.Printf("handleAgentTickets: userID = %d", userID)
		
		// Get filter parameters
		status := c.DefaultQuery("status", "open")
		queue := c.DefaultQuery("queue", "all")
		assignee := c.DefaultQuery("assignee", "all")
		search := c.Query("search")
		
		// Build query
		query := `
			SELECT t.id, t.tn, t.title,
				   c.login as customer,
				   cc.name as company,
				   q.name as queue,
				   ts.name as state,
				   tp.name as priority,
				   tp.color as priority_color,
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
		argCount := 0
		
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
		}
		
		// Apply queue filter
		if queue != "all" {
			argCount++
			query += fmt.Sprintf(" AND t.queue_id = $%d", argCount)
			args = append(args, queue)
		} else {
			// Check if user is admin
			var isAdmin bool
			adminCheckErr := db.QueryRow(`
				SELECT EXISTS(
					SELECT 1 FROM group_user gu
					JOIN groups g ON gu.group_id = g.id
					WHERE gu.user_id = $1 AND g.name = 'admin'
				)
			`, userID).Scan(&isAdmin)
			
			if adminCheckErr == nil && isAdmin {
				// Admin sees all queues - no filter needed
				log.Printf("User %d is admin, showing all queues", userID)
			} else {
				// Regular agents see only queues they have access to through group membership
				argCount++
				query += fmt.Sprintf(` AND t.queue_id IN (
					SELECT DISTINCT q2.id FROM queue q2
					WHERE q2.group_id IN (
						SELECT group_id FROM group_user WHERE user_id = $%d
					)
				)`, argCount)
				args = append(args, userID)
				log.Printf("User %d is not admin, filtering by queue access", userID)
			}
		}
		
		// Apply assignee filter
		if assignee == "me" {
			argCount++
			query += fmt.Sprintf(" AND t.responsible_user_id = $%d", argCount)
			args = append(args, userID)
		} else if assignee == "unassigned" {
			query += " AND t.responsible_user_id IS NULL"
		}
		
		// Apply search
		if search != "" {
			argCount++
			query += fmt.Sprintf(" AND (t.tn ILIKE $%d OR t.title ILIKE $%d OR c.login ILIKE $%d)", 
				argCount, argCount, argCount)
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
				"last_changed":   formatAge(ticket.ChangeTime),
				"article_count":  ticket.ArticleCount,
			})
		}
		
		// Get available queues for filter
		queueRows, _ := db.Query(`
			SELECT q.id, q.name
			FROM queue q
			JOIN group_user gu ON q.group_id = gu.group_id
			WHERE gu.user_id = $1
			ORDER BY q.name
		`, userID)
		defer queueRows.Close()
		
		availableQueues := []map[string]interface{}{}
		for queueRows.Next() {
			var q struct {
				ID   int
				Name string
			}
			queueRows.Scan(&q.ID, &q.Name)
			availableQueues = append(availableQueues, map[string]interface{}{
				"id":   fmt.Sprintf("%d", q.ID),
				"name": q.Name,
			})
		}
		
		// Get user from context for navigation display
		user := getUserFromContext(c)
		
		// Check if user is in admin group for Dev tab
		var isInAdminGroup bool
		adminErr := db.QueryRow(`
			SELECT EXISTS(
				SELECT 1 FROM group_user gu
				JOIN groups g ON gu.group_id = g.id
				WHERE gu.user_id = $1 AND g.name = 'admin'
			)
		`, userID).Scan(&isInAdminGroup)
		if adminErr == nil && isInAdminGroup && user != nil {
			// Set a flag in context or add to user struct if it has the field
			c.Set("isInAdminGroup", true)
		}
		
		// Pass the isInAdminGroup flag to template
		adminGroupFlag, _ := c.Get("isInAdminGroup")
		
		pongo2Renderer.HTML(c, http.StatusOK, "pages/agent/tickets.pongo2", pongo2.Context{
			"Title":           "Ticket Management",
			"ActivePage":      "agent",
			"User":            user,
			"IsInAdminGroup":  adminGroupFlag,
			"Tickets":         tickets,
			"AvailableQueues": availableQueues,
			"CurrentFilters": map[string]string{
				"status":   status,
				"queue":    queue,
				"assignee": assignee,
				"search":   search,
				"sort":     sortBy,
				"order":    sortOrder,
			},
		})
	}
}

// Other handler functions would continue here...

// Helper functions
func formatAge(t time.Time) string {
	duration := time.Since(t)
	
	if duration.Hours() < 1 {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	} else if duration.Hours() < 24 {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	} else if duration.Hours() < 24*7 {
		return fmt.Sprintf("%dd", int(duration.Hours()/24))
	} else if duration.Hours() < 24*30 {
		return fmt.Sprintf("%dw", int(duration.Hours()/(24*7)))
	} else if duration.Hours() < 24*365 {
		return fmt.Sprintf("%dmo", int(duration.Hours()/(24*30)))
	}
	return fmt.Sprintf("%dy", int(duration.Hours()/(24*365)))
}

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

// handleAgentTicketView shows detailed ticket view
func handleAgentTicketView(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID := c.Param("id")
		
		// Get ticket details
		var ticket struct {
			ID                int
			TN                string
			Title             string
			CustomerUserID    sql.NullString
			CustomerID        sql.NullString
			QueueID           int
			QueueName         string
			TypeID            sql.NullInt32
			TypeName          sql.NullString
			ServiceID         sql.NullInt32
			ServiceName       sql.NullString
			SLAID             sql.NullInt32
			SLAName           sql.NullString
			StateID           int
			StateName         string
			StateType         string
			PriorityID        int
			PriorityName      string
			ResponsibleUserID sql.NullInt32
			ResponsibleUser   sql.NullString
			CreateTime        time.Time
			ChangeTime        time.Time
		}
		
		err := db.QueryRow(`
			SELECT t.id, t.tn, t.title, 
				   t.customer_user_id, t.customer_id,
				   t.queue_id, q.name as queue_name,
				   t.type_id, tt.name as type_name,
				   t.service_id, s.name as service_name,
				   t.sla_id, sla.name as sla_name,
				   t.ticket_state_id, ts.name as state_name, tst.name as state_type,
				   t.ticket_priority_id, tp.name as priority_name,
				   t.responsible_user_id, u.login as responsible_user,
				   t.create_time, t.change_time
			FROM ticket t
			LEFT JOIN queue q ON t.queue_id = q.id
			LEFT JOIN ticket_type tt ON t.type_id = tt.id
			LEFT JOIN service s ON t.service_id = s.id
			LEFT JOIN sla ON t.sla_id = sla.id
			LEFT JOIN ticket_state ts ON t.ticket_state_id = ts.id
			LEFT JOIN ticket_state_type tst ON ts.type_id = tst.id
			LEFT JOIN ticket_priority tp ON t.ticket_priority_id = tp.id
			LEFT JOIN users u ON t.responsible_user_id = u.id
			WHERE t.id = $1
		`, ticketID).Scan(
			&ticket.ID, &ticket.TN, &ticket.Title,
			&ticket.CustomerUserID, &ticket.CustomerID,
			&ticket.QueueID, &ticket.QueueName,
			&ticket.TypeID, &ticket.TypeName,
			&ticket.ServiceID, &ticket.ServiceName,
			&ticket.SLAID, &ticket.SLAName,
			&ticket.StateID, &ticket.StateName, &ticket.StateType,
			&ticket.PriorityID, &ticket.PriorityName,
			&ticket.ResponsibleUserID, &ticket.ResponsibleUser,
			&ticket.CreateTime, &ticket.ChangeTime,
		)
		
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
			} else {
				log.Printf("Error fetching ticket: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch ticket"})
			}
			return
		}
		
		// Get customer details including email
		var customerName, customerCompany, customerEmail string
		if ticket.CustomerUserID.Valid {
			err := db.QueryRow(`
				SELECT COALESCE(cu.first_name || ' ' || cu.last_name, cu.login) as name, 
					   COALESCE(cc.name, '') as company,
					   COALESCE(cu.email, cu.login) as email
				FROM customer_user cu
				LEFT JOIN customer_company cc ON cu.customer_id = cc.customer_id
				WHERE cu.login = $1
			`, ticket.CustomerUserID.String).Scan(&customerName, &customerCompany, &customerEmail)
			if err != nil {
				// If no customer_user record, use the ID as email
				customerEmail = ticket.CustomerUserID.String
				customerName = ticket.CustomerUserID.String
			}
		}
		
		// Get articles - fetch from article_data_mime table
		rows, err := db.Query(`
			SELECT a.id, a.article_sender_type_id, 
				   COALESCE(ast.name, 'Unknown') as sender_type,
				   COALESCE(adm.a_from, 'System') as from_addr,
				   COALESCE(adm.a_to, '') as to_addr,
				   COALESCE(adm.a_subject, 'Note') as subject,
				   COALESCE(convert_from(adm.a_body, 'UTF8'), '') as body,
				   a.create_time,
				   a.is_visible_for_customer
			FROM article a
			LEFT JOIN article_sender_type ast ON a.article_sender_type_id = ast.id
			LEFT JOIN article_data_mime adm ON a.id = adm.article_id
			WHERE a.ticket_id = $1
			ORDER BY a.create_time DESC
		`, ticketID)
		
		articles := []map[string]interface{}{}
		if err != nil {
			log.Printf("Error fetching articles: %v", err)
		} else {
			defer rows.Close()
			for rows.Next() {
				var article struct {
					ID         int
					SenderTypeID int
					SenderType string
					From       string
					To         string
					Subject    string
					Body       string
					CreateTime time.Time
					IsVisible  bool
				}
				
				if err := rows.Scan(&article.ID, &article.SenderTypeID, &article.SenderType,
					&article.From, &article.To, &article.Subject, &article.Body, &article.CreateTime, &article.IsVisible); err != nil {
					log.Printf("Error scanning article: %v", err)
					continue
				}
				
				articles = append(articles, map[string]interface{}{
					"id":           article.ID,
					"sender_type_id": article.SenderTypeID,
					"sender_type":  article.SenderType,
					"from":         article.From,
					"to":           article.To,
					"subject":      article.Subject,
					"body":         article.Body,
					"create_time":  article.CreateTime.Format("2006-01-02 15:04"),
					"is_visible":   article.IsVisible,
				})
			}
		}
		
		log.Printf("Found %d articles for ticket %s", len(articles), ticketID)
		
		// Prepare template data
		templateData := pongo2.Context{
			"ticket": map[string]interface{}{
				"id":                ticket.ID,
				"tn":                ticket.TN,
				"title":             ticket.Title,
				"customer_user_id":  ticket.CustomerUserID.String,
				"customer_name":     customerName,
				"customer_email":    customerEmail,
				"customer_company":  customerCompany,
				"queue":             ticket.QueueName,
				"queue_id":          ticket.QueueID,
				"type":              ticket.TypeName.String,
				"service":           ticket.ServiceName.String,
				"sla":               ticket.SLAName.String,
				"state":             ticket.StateName,
				"state_type":        ticket.StateType,
				"priority":          ticket.PriorityName,
				"priority_id":       ticket.PriorityID,
				"assigned_to":       ticket.ResponsibleUser.String,
				"create_time":       ticket.CreateTime.Format("2006-01-02 15:04"),
				"change_time":       ticket.ChangeTime.Format("2006-01-02 15:04"),
			},
			"articles": articles,
			"User": map[string]interface{}{
				"id": c.GetUint("user_id"),
				"name": c.GetString("user_name"),
				"role": c.GetString("user_role"),
			},
			"ActivePage": "tickets",
		}
		
		// Get user from context for navigation
		user, _ := c.Get("user")
		templateData["User"] = user
		
		if renderer := GetPongo2Renderer(); renderer != nil {
			renderer.HTML(c, http.StatusOK, "pages/agent/ticket_view.pongo2", templateData)
		} else {
			c.JSON(http.StatusOK, templateData)
		}
	}
}

func handleAgentTicketReply(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID := c.Param("id")
		to := c.PostForm("to")
		subject := c.PostForm("subject")
		body := c.PostForm("body")
		
		// Get user info
		userID := c.GetUint("user_id")
		userName := c.GetString("user_name")
		if userName == "" {
			userName = "Agent"
		}
		
		// Start transaction
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
			return
		}
		defer tx.Rollback()
		
		// Insert article (sender_type_id = 1 for agent reply)
		var articleID int64
		err = tx.QueryRow(`
			INSERT INTO article (ticket_id, article_sender_type_id, is_visible_for_customer,
								create_time, create_by, change_time, change_by)
			VALUES ($1, 1, 1, CURRENT_TIMESTAMP, $2, CURRENT_TIMESTAMP, $2)
			RETURNING id
		`, ticketID, userID).Scan(&articleID)
		
		if err != nil {
			log.Printf("Error creating article: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add reply"})
			return
		}
		
		// Insert article data in mime table (incoming_time is unix timestamp)
		// Note: a_body is BYTEA type, so we need to convert
		_, err = tx.Exec(`
			INSERT INTO article_data_mime (article_id, a_from, a_to, a_subject, a_body, incoming_time, create_time, create_by, change_time, change_by)
			VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP, $7, CURRENT_TIMESTAMP, $7)
		`, articleID, userName, to, subject, []byte(body), time.Now().Unix(), userID)
		
		if err != nil {
			log.Printf("Error adding article data: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add reply data"})
			return
		}
		
		// Update ticket change time
		_, err = tx.Exec("UPDATE ticket SET change_time = CURRENT_TIMESTAMP, change_by = $1 WHERE id = $2", userID, ticketID)
		if err != nil {
			log.Printf("Error updating ticket: %v", err)
		}
		
		// Commit transaction
		if err = tx.Commit(); err != nil {
			log.Printf("Error committing transaction: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save reply"})
			return
		}
		
		c.JSON(http.StatusOK, gin.H{"success": true, "article_id": articleID})
	}
}

func handleAgentTicketNote(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID := c.Param("id")
		body := c.PostForm("body")
		
		// Get user info
		userID := c.GetUint("user_id")
		
		// Start transaction
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
			return
		}
		defer tx.Rollback()
		
		// Insert article (sender_type_id = 3 for agent)
		var articleID int64
		err = tx.QueryRow(`
			INSERT INTO article (ticket_id, article_sender_type_id, is_visible_for_customer,
								create_time, create_by, change_time, change_by)
			VALUES ($1, 3, 0, CURRENT_TIMESTAMP, $2, CURRENT_TIMESTAMP, $2)
			RETURNING id
		`, ticketID, userID).Scan(&articleID)
		
		if err != nil {
			log.Printf("Error creating article: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add note"})
			return
		}
		
		// Insert article data in mime table (incoming_time is unix timestamp)
		// Note: a_body is BYTEA type, so we need to convert
		_, err = tx.Exec(`
			INSERT INTO article_data_mime (article_id, a_from, a_subject, a_body, incoming_time, create_time, create_by, change_time, change_by)
			VALUES ($1, 'Agent', 'Note', $2, $3, CURRENT_TIMESTAMP, $4, CURRENT_TIMESTAMP, $4)
		`, articleID, []byte(body), time.Now().Unix(), userID)
		
		if err != nil {
			log.Printf("Error adding article data: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add note data"})
			return
		}
		
		// Update ticket change time
		_, err = tx.Exec("UPDATE ticket SET change_time = CURRENT_TIMESTAMP, change_by = $1 WHERE id = $2", userID, ticketID)
		if err != nil {
			log.Printf("Error updating ticket: %v", err)
		}
		
		// Commit transaction
		if err = tx.Commit(); err != nil {
			log.Printf("Error committing transaction: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save note"})
			return
		}
		
		c.JSON(http.StatusOK, gin.H{"success": true, "article_id": articleID})
	}
}

func handleAgentTicketPhone(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID := c.Param("id")
		subject := c.PostForm("subject")
		body := c.PostForm("body")
		
		if subject == "" {
			subject = "Phone call note"
		}
		
		// Get user info
		userID := c.GetUint("user_id")
		
		// Start transaction
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
			return
		}
		defer tx.Rollback()
		
		// Insert article (sender_type_id = 1 for agent, phone communication type)
		var articleID int64
		err = tx.QueryRow(`
			INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer,
								create_time, create_by, change_time, change_by)
			VALUES ($1, 1, 2, 1, CURRENT_TIMESTAMP, $2, CURRENT_TIMESTAMP, $2)
			RETURNING id
		`, ticketID, userID).Scan(&articleID)
		if err != nil {
			log.Printf("Error inserting phone article: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save phone note"})
			return
		}

		// Insert MIME data
		_, err = tx.Exec(`
			INSERT INTO article_data_mime (article_id, a_from, a_subject, a_body,
											incoming_time, create_time, create_by, change_time, change_by)
			VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, $5, CURRENT_TIMESTAMP, $5)
		`, articleID, "Agent Phone Call", subject, body, userID)
		if err != nil {
			log.Printf("Error inserting phone article MIME data: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save phone note data"})
			return
		}

		if err = tx.Commit(); err != nil {
			log.Printf("Error committing transaction: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save phone note"})
			return
		}
		
		c.JSON(http.StatusOK, gin.H{"success": true, "article_id": articleID})
	}
}

func handleAgentTicketStatus(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID := c.Param("id")
		statusID := c.PostForm("status_id")
		pendingUntil := c.PostForm("pending_until")
		
		// Handle pending time for pending states
		var untilTime sql.NullInt64
		pendingStates := map[string]bool{"4": true, "5": true, "6": true} // pending reminder, auto close+, auto close-
		
		if pendingStates[statusID] {
			if pendingUntil == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Pending time is required for pending states"})
				return
			}
			
			// Parse the datetime-local format: 2006-01-02T15:04
			if t, err := time.Parse("2006-01-02T15:04", pendingUntil); err == nil {
				untilTime = sql.NullInt64{Int64: t.Unix(), Valid: true}
				log.Printf("Setting pending time for ticket %s to %v (unix: %d)", ticketID, t, t.Unix())
			} else {
				log.Printf("Failed to parse pending time '%s': %v", pendingUntil, err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid pending time format"})
				return
			}
		} else {
			// Clear pending time for non-pending states
			untilTime = sql.NullInt64{Int64: 0, Valid: false}
		}
		
		// Update ticket status with pending time
		_, err := db.Exec(`
			UPDATE ticket 
			SET ticket_state_id = $1, until_time = $2, change_time = CURRENT_TIMESTAMP, change_by = $3
			WHERE id = $4
		`, statusID, untilTime, c.GetUint("user_id"), ticketID)
		
		if err != nil {
			log.Printf("Error updating ticket status: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status"})
			return
		}
		
		// Log the status change for audit trail
		statusName := getStatusName(statusID)
		if untilTime.Valid {
			log.Printf("Ticket %s status changed to %s (ID: %s) with pending time until %v by user %d", 
				ticketID, statusName, statusID, time.Unix(untilTime.Int64, 0), c.GetUint("user_id"))
		} else {
			log.Printf("Ticket %s status changed to %s (ID: %s) by user %d", 
				ticketID, statusName, statusID, c.GetUint("user_id"))
		}
		
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// Helper function to get status name from ID (for logging)
func getStatusName(statusID string) string {
	statusNames := map[string]string{
		"1": "new",
		"2": "open", 
		"4": "pending reminder",
		"5": "pending auto close+",
		"6": "pending auto close-",
		"7": "closed successful",
		"8": "closed unsuccessful",
	}
	if name, exists := statusNames[statusID]; exists {
		return name
	}
	return "unknown"
}
func handleAgentTicketAssign(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID := c.Param("id")
		userID := c.PostForm("user_id")
		
		// Update responsible user
		_, err := db.Exec(`
			UPDATE ticket 
			SET responsible_user_id = $1, change_time = CURRENT_TIMESTAMP, change_by = $2
			WHERE id = $3
		`, userID, c.GetUint("user_id"), ticketID)
		
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign agent"})
			return
		}
		
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

func handleAgentTicketPriority(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID := c.Param("id")
		priorityID := c.PostForm("priority_id")
		
		// Update ticket priority
		_, err := db.Exec(`
			UPDATE ticket 
			SET ticket_priority_id = $1, change_time = CURRENT_TIMESTAMP, change_by = $2
			WHERE id = $3
		`, priorityID, c.GetUint("user_id"), ticketID)
		
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update priority"})
			return
		}
		
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// NEWLY ADDED: Missing handlers that were causing 404 errors
func handleAgentTicketQueue(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID := c.Param("id")
		queueID := c.PostForm("queue_id")
		
		// Update ticket queue
		_, err := db.Exec(`
			UPDATE ticket 
			SET queue_id = $1, change_time = CURRENT_TIMESTAMP, change_by = $2
			WHERE id = $3
		`, queueID, c.GetUint("user_id"), ticketID)
		
		if err != nil {
			log.Printf("Error updating ticket queue: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to move ticket to queue"})
			return
		}
		
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

func handleAgentTicketMerge(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sourceTicketID := c.Param("id")
		targetTN := c.PostForm("target_tn")
		
		if targetTN == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Target ticket number required"})
			return
		}
		
		// Start transaction for merge operation
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
			return
		}
		defer tx.Rollback()
		
		// Find target ticket ID by ticket number
		var targetTicketID int
		err = tx.QueryRow("SELECT id FROM ticket WHERE tn = $1", targetTN).Scan(&targetTicketID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Target ticket not found"})
			return
		}
		
		// Move all articles from source to target ticket
		_, err = tx.Exec(`
			UPDATE article 
			SET ticket_id = $1, change_time = CURRENT_TIMESTAMP, change_by = $2
			WHERE ticket_id = $3
		`, targetTicketID, c.GetUint("user_id"), sourceTicketID)
		
		if err != nil {
			log.Printf("Error moving articles during merge: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to merge articles"})
			return
		}
		
		// Close the source ticket
		_, err = tx.Exec(`
			UPDATE ticket 
			SET ticket_state_id = (SELECT id FROM ticket_state WHERE name = 'merged'),
				change_time = CURRENT_TIMESTAMP, change_by = $1
			WHERE id = $2
		`, c.GetUint("user_id"), sourceTicketID)
		
		if err != nil {
			log.Printf("Error closing source ticket during merge: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to close source ticket"})
			return
		}
		
		// Commit transaction
		if err = tx.Commit(); err != nil {
			log.Printf("Error committing merge transaction: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete merge"})
			return
		}
		
		c.JSON(http.StatusOK, gin.H{
			"success": true, 
			"message": fmt.Sprintf("Ticket merged into %s", targetTN),
			"target_ticket": targetTN,
		})
	}
}

func handleAgentQueues(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "TODO: Queue list"})
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
