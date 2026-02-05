package v1

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	api "github.com/gotrs-io/gotrs-ce/internal/api"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/service/ticket_number"
)

// HandleListTickets returns a paginated list of tickets (exported for tests).
func (router *APIRouter) HandleListTickets(c *gin.Context) { api.HandleListTicketsAPI(c) }

// HandleUpdateTicket updates a ticket (exported for tests).
func (router *APIRouter) HandleUpdateTicket(c *gin.Context) { api.HandleUpdateTicketAPI(c) }

// handleListTickets returns a paginated list of tickets.
func (router *APIRouter) handleListTickets(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))         //nolint:errcheck // Defaults to 0
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "25")) //nolint:errcheck // Defaults to 0
	status := c.Query("status")
	priority := c.Query("priority")
	// assignedTo := c.Query("assigned_to") // Not used currently
	queueIDStr := c.Query("queue_id")
	search := c.Query("search")

	// Parse queue ID if provided
	var queueID *uint
	if queueIDStr != "" {
		if id, err := strconv.ParseUint(queueIDStr, 10, 32); err == nil {
			queueIDVal := uint(id)
			queueID = &queueIDVal
		}
	}

	// Parse state ID based on status
	var stateID *uint
	if status != "" {
		// Map status string to state ID
		var id uint
		switch status {
		case "new":
			id = 1
		case "open":
			id = 4
		case "closed":
			id = 2
		}
		if id > 0 {
			stateID = &id
		}
	}

	// Parse priority ID based on priority
	var priorityID *uint
	if priority != "" {
		// Map priority string to priority ID
		var id uint
		switch priority {
		case "low":
			id = 1
		case "normal":
			id = 3
		case "high":
			id = 4
		case "very-high":
			id = 5
		}
		if id > 0 {
			priorityID = &id
		}
	}

	// Get tickets from service
	ticketService := api.GetTicketService()
	request := &models.TicketListRequest{
		Page:       page,
		PerPage:    perPage,
		StateID:    stateID,
		PriorityID: priorityID,
		QueueID:    queueID,
		Search:     search,
	}

	response, err := ticketService.ListTickets(request)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to retrieve tickets")
		return
	}

	// Convert to API format
	tickets := []gin.H{}
	for _, t := range response.Tickets {
		// Get queue name from database
		queueName := fmt.Sprintf("Queue %d", t.QueueID)
		var queueRow struct {
			Name string
		}
		db, err := database.GetDB()
		if err != nil {
			// Handle error or continue with default queue name
		} else {
			err := db.QueryRow(database.ConvertPlaceholders("SELECT name FROM queue WHERE id = ?"), t.QueueID).Scan(&queueRow.Name)
			if err == nil {
				queueName = queueRow.Name
			}
		}

		ticket := gin.H{
			"id":             t.ID,
			"number":         t.TicketNumber,
			"title":          t.Title,
			"status":         mapTicketState(t.TicketStateID),
			"priority":       mapTicketPriority(t.TicketPriorityID),
			"queue_id":       t.QueueID,
			"queue_name":     queueName,
			"customer_email": t.CustomerUserID,
			"created_at":     t.CreateTime,
			"updated_at":     t.ChangeTime,
		}

		if t.UserID != nil {
			ticket["assigned_to"] = *t.UserID
			// Get user name from database
			assignedName := fmt.Sprintf("User %d", *t.UserID)
			if db != nil {
				var firstName, lastName string
				err := db.QueryRow(database.ConvertPlaceholders("SELECT COALESCE(first_name, ''), COALESCE(last_name, '') FROM users WHERE id = ?"), *t.UserID).Scan(&firstName, &lastName)
				if err == nil {
					assignedName = strings.TrimSpace(firstName + " " + lastName)
					if assignedName == "" {
						assignedName = fmt.Sprintf("User %d", *t.UserID)
					}
				}
			}
			ticket["assigned_name"] = assignedName
		}

		tickets = append(tickets, ticket)
	}

	pagination := Pagination{
		Page:       response.Page,
		PerPage:    response.PerPage,
		Total:      response.Total,
		TotalPages: response.TotalPages,
		HasNext:    response.Page < response.TotalPages,
		HasPrev:    response.Page > 1,
	}

	sendPaginatedResponse(c, tickets, pagination)
}

// HandleCreateTicket creates a new ticket.
func (router *APIRouter) HandleCreateTicket(c *gin.Context) {
	var ticketRequest struct {
		Title          string                 `json:"title" binding:"required"`
		QueueID        int                    `json:"queue_id" binding:"required"`
		TypeID         int                    `json:"type_id"`
		StateID        int                    `json:"state_id"`
		PriorityID     int                    `json:"priority_id"`
		CustomerUserID string                 `json:"customer_user_id"`
		CustomerID     string                 `json:"customer_id"`
		Article        map[string]interface{} `json:"article"`
	}

	if err := c.ShouldBindJSON(&ticketRequest); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid ticket request: "+err.Error())
		return
	}

	// Validate title length
	if len(ticketRequest.Title) > 255 {
		sendError(c, http.StatusBadRequest, "Title too long (max 255 characters)")
		return
	}

	userID, _, _, exists := middleware.GetCurrentUser(c)
	if !exists {
		sendError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusServiceUnavailable, "Database connection failed")
		return
	}

	// Validate queue exists
	var queueExists bool
	queueExistsQuery := "SELECT EXISTS(SELECT 1 FROM queue WHERE id = ? AND valid_id = 1)"
	err = db.QueryRow(database.ConvertPlaceholders(queueExistsQuery), ticketRequest.QueueID).Scan(&queueExists)
	if err != nil || !queueExists {
		sendError(c, http.StatusBadRequest, "Invalid queue_id")
		return
	}

	// Create ticket number generator
	generatorConfig := map[string]interface{}{
		"type": os.Getenv("TICKET_NUMBER_GENERATOR"),
	}
	if generatorConfig["type"] == "" {
		generatorConfig["type"] = "date"
	}

	generator, err := ticket_number.NewGeneratorFromConfig(db, generatorConfig)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to initialize ticket number generator")
		return
	}

	// Generate ticket number
	ticketNumber, err := generator.Generate()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to generate ticket number")
		return
	}

	// Set defaults for missing values
	if ticketRequest.TypeID == 0 {
		ticketRequest.TypeID = 1 // Default type
	}
	if ticketRequest.StateID == 0 {
		ticketRequest.StateID = 1 // new
	}
	if ticketRequest.PriorityID == 0 {
		ticketRequest.PriorityID = 3 // normal
	}

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to start transaction")
		return
	}
	defer func() { _ = tx.Rollback() }()

	// Get database adapter
	adapter := database.GetAdapter()

	// Insert ticket
	ticketTypeColumn := database.TicketTypeColumn()
	ticketQuery := database.ConvertPlaceholders(fmt.Sprintf(`
		INSERT INTO ticket (
			tn, title, queue_id, %s, ticket_state_id, 
			ticket_priority_id, customer_user_id, customer_id,
			ticket_lock_id, user_id, responsible_user_id,
			create_time, create_by, change_time, change_by
		) VALUES (
			?, ?, ?, ?, ?, 
			?, ?, ?,
			1, ?, ?,
			NOW(), ?, NOW(), ?
		) RETURNING id
	`, ticketTypeColumn))

	ticketID, err := adapter.InsertWithReturningTx(
		tx,
		ticketQuery,
		ticketNumber, ticketRequest.Title, ticketRequest.QueueID,
		ticketRequest.TypeID, ticketRequest.StateID,
		ticketRequest.PriorityID, ticketRequest.CustomerUserID, ticketRequest.CustomerID,
		userID, userID, userID, userID,
	)

	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to create ticket: "+err.Error())
		return
	}

	// Create initial article if provided
	if ticketRequest.Article != nil {
		subject, _ := ticketRequest.Article["subject"].(string)          //nolint:errcheck // Defaults to empty
		body, _ := ticketRequest.Article["body"].(string)                //nolint:errcheck // Defaults to empty
		contentType, _ := ticketRequest.Article["content_type"].(string) //nolint:errcheck // Defaults to empty
		if contentType == "" {
			contentType = "text/plain"
		}

		articleTypeID := 1 // email-external default
		senderTypeID := 3  // customer default

		if atID, ok := ticketRequest.Article["article_type_id"].(float64); ok {
			articleTypeID = int(atID)
		}
		if stID, ok := ticketRequest.Article["sender_type_id"].(float64); ok {
			senderTypeID = int(stID)
		}

		// Insert article
		articleQuery := database.ConvertPlaceholders(`
			INSERT INTO article (
				ticket_id, article_sender_type_id, communication_channel_id,
				is_visible_for_customer, create_time, create_by, 
				change_time, change_by
			) VALUES (
				?, ?, 1, 1, NOW(), ?, NOW(), ?
			) RETURNING id
		`)

		articleID, err := adapter.InsertWithReturningTx(
			tx,
			articleQuery,
			ticketID, senderTypeID, userID, userID,
		)

		if err != nil {
			sendError(c, http.StatusInternalServerError, "Failed to create article: "+err.Error())
			return
		}

		// Insert article content in article_data_mime table
		if subject != "" || body != "" {
			contentQuery := database.ConvertPlaceholders(`
				INSERT INTO article_data_mime (
					article_id, a_subject, a_body, a_content_type,
					create_time, create_by, change_time, change_by
				) VALUES (
					?, ?, ?, ?,
					NOW(), ?, NOW(), ?
				)
			`)

			_, err = tx.Exec(contentQuery,
				articleID, subject, body, contentType,
				userID, userID,
			)

			if err != nil {
				// Log error but don't fail the whole ticket creation
				// Article metadata is saved, just content failed
				fmt.Printf("Warning: Failed to save article content: %v\n", err)
			}
		}
		_ = articleTypeID
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to commit transaction")
		return
	}

	// Fetch the created ticket for response
	var ticket struct {
		ID             int64     `json:"id"`
		TicketNumber   string    `json:"tn"`
		Title          string    `json:"title"`
		QueueID        int       `json:"queue_id"`
		TypeID         int       `json:"type_id"`
		StateID        int       `json:"ticket_state_id"`
		PriorityID     int       `json:"ticket_priority_id"`
		CustomerUserID *string   `json:"customer_user_id"`
		CustomerID     *string   `json:"customer_id"`
		CreateTime     time.Time `json:"create_time"`
	}

	// Query the created ticket
	typeSelect := fmt.Sprintf("%s AS type_id", database.TicketTypeColumn())
	query := database.ConvertPlaceholders(fmt.Sprintf(`
		SELECT id, tn, title, queue_id, %s, ticket_state_id,
		       ticket_priority_id, customer_user_id, customer_id, create_time
		FROM ticket
		WHERE id = ?
	`, typeSelect))

	row := db.QueryRow(query, ticketID)
	err = row.Scan(
		&ticket.ID, &ticket.TicketNumber, &ticket.Title,
		&ticket.QueueID, &ticket.TypeID, &ticket.StateID,
		&ticket.PriorityID, &ticket.CustomerUserID, &ticket.CustomerID,
		&ticket.CreateTime,
	)

	if err != nil {
		// Ticket was created but we can't fetch it - still return success with basic info
		c.JSON(http.StatusCreated, APIResponse{
			Success: true,
			Data: gin.H{
				"id":       ticketID,
				"tn":       ticketNumber,
				"title":    ticketRequest.Title,
				"queue_id": ticketRequest.QueueID,
				"message":  "Ticket created successfully",
			},
		})
		return
	}

	// Return full ticket data
	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data:    ticket,
	})
}

// handleGetTicket returns a specific ticket by ID.
func (router *APIRouter) handleGetTicket(c *gin.Context) {
	ticketIDStr := c.Param("id")
	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid ticket ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	query := database.ConvertQuery(`
		SELECT t.id, t.tn, t.title, t.queue_id, t.ticket_state_id, t.ticket_priority_id,
			t.user_id, t.customer_user_id, t.create_time, t.change_time,
			q.name as queue_name, ts.name as state_name, tp.name as priority_name
		FROM ticket t
		LEFT JOIN queue q ON q.id = t.queue_id
		LEFT JOIN ticket_state ts ON ts.id = t.ticket_state_id
		LEFT JOIN ticket_priority tp ON tp.id = t.ticket_priority_id
		WHERE t.id = ? AND t.archive_flag = 0
	`)

	var id int
	var tn, title, queueName, stateName, priorityName string
	var queueID, stateID, priorityID int
	var userID *int
	var customerUserID *string
	var createTime, changeTime time.Time

	err = db.QueryRow(query, ticketID).Scan(&id, &tn, &title, &queueID, &stateID, &priorityID,
		&userID, &customerUserID, &createTime, &changeTime, &queueName, &stateName, &priorityName)
	if err != nil {
		sendError(c, http.StatusNotFound, "Ticket not found")
		return
	}

	ticket := gin.H{
		"id":             id,
		"number":         tn,
		"title":          title,
		"status":         stateName,
		"status_id":      stateID,
		"priority":       priorityName,
		"priority_id":    priorityID,
		"queue_id":       queueID,
		"queue_name":     queueName,
		"customer_email": customerUserID,
		"created_at":     createTime,
		"updated_at":     changeTime,
	}

	if userID != nil {
		ticket["assigned_to"] = *userID
		// Get assigned user name
		var firstName, lastName string
		nameQuery := database.ConvertQuery(`SELECT COALESCE(first_name, ''), COALESCE(last_name, '') FROM users WHERE id = ?`)
		if db.QueryRow(nameQuery, *userID).Scan(&firstName, &lastName) == nil {
			ticket["assigned_name"] = strings.TrimSpace(firstName + " " + lastName)
		}
	}

	// Get article count
	var articleCount int
	countQuery := database.ConvertQuery(`SELECT COUNT(*) FROM article WHERE ticket_id = ?`)
	db.QueryRow(countQuery, ticketID).Scan(&articleCount)
	ticket["article_count"] = articleCount

	sendSuccess(c, ticket)
}

// handleUpdateTicket updates an existing ticket.
func (router *APIRouter) handleUpdateTicket(c *gin.Context) {
	ticketIDStr := c.Param("id")
	if ticketIDStr == "" {
		sendError(c, http.StatusBadRequest, "Ticket ID required")
		return
	}

	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid ticket ID")
		return
	}

	// Parse update request
	var updateRequest struct {
		Title             *string `json:"title"`
		QueueID           *int    `json:"queue_id"`
		TypeID            *int    `json:"type_id"`
		StateID           *int    `json:"state_id"`
		PriorityID        *int    `json:"priority_id"`
		CustomerUserID    *string `json:"customer_user_id"`
		CustomerID        *string `json:"customer_id"`
		ResponsibleUserID *int    `json:"responsible_user_id"`
		OwnerID           *int    `json:"owner_id"`
		LockID            *int    `json:"lock_id"`
	}

	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid update request: "+err.Error())
		return
	}

	// Check if any fields to update
	if updateRequest.Title == nil && updateRequest.QueueID == nil && updateRequest.TypeID == nil &&
		updateRequest.StateID == nil && updateRequest.PriorityID == nil && updateRequest.CustomerUserID == nil &&
		updateRequest.CustomerID == nil && updateRequest.ResponsibleUserID == nil && updateRequest.OwnerID == nil &&
		updateRequest.LockID == nil {
		sendError(c, http.StatusBadRequest, "No fields to update")
		return
	}

	// Get user ID
	userID := getContextUserID(c)

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusServiceUnavailable, "Database unavailable")
		return
	}

	// Check if ticket exists and get current customer_user_id for permission check
	var customerUserID string
	err = db.QueryRow(database.ConvertPlaceholders(
		"SELECT customer_user_id FROM ticket WHERE id = ?",
	), ticketID).Scan(&customerUserID)

	if err != nil {
		if err == sql.ErrNoRows {
			sendError(c, http.StatusNotFound, "Ticket not found")
		} else {
			sendError(c, http.StatusInternalServerError, "Database error")
		}
		return
	}

	// Check permissions for customer users
	if isCustomer, _ := c.Get("is_customer"); isCustomer == true {
		customerEmail, _ := c.Get("customer_email")
		if email, ok := customerEmail.(string); ok && customerUserID != email {
			sendError(c, http.StatusForbidden, "Access denied")
			return
		}
		// Customers can only update title and add articles
		if updateRequest.QueueID != nil || updateRequest.TypeID != nil || updateRequest.StateID != nil ||
			updateRequest.PriorityID != nil || updateRequest.ResponsibleUserID != nil || updateRequest.OwnerID != nil ||
			updateRequest.LockID != nil {
			sendError(c, http.StatusForbidden, "Customers can only update ticket title")
			return
		}
	}

	// Validate referenced IDs exist
	if updateRequest.QueueID != nil {
		var exists bool
		_ = db.QueryRow(database.ConvertPlaceholders("SELECT EXISTS(SELECT 1 FROM queue WHERE id = ?)"), *updateRequest.QueueID).Scan(&exists) //nolint:errcheck // Defaults to false
		if !exists {
			sendError(c, http.StatusBadRequest, "Invalid queue ID")
			return
		}
	}

	if updateRequest.StateID != nil {
		var exists bool
		_ = db.QueryRow(database.ConvertPlaceholders("SELECT EXISTS(SELECT 1 FROM ticket_state WHERE id = ?)"), *updateRequest.StateID).Scan(&exists) //nolint:errcheck // Defaults to false
		if !exists {
			sendError(c, http.StatusBadRequest, "Invalid state ID")
			return
		}
	}

	if updateRequest.PriorityID != nil {
		var exists bool
		_ = db.QueryRow(database.ConvertPlaceholders("SELECT EXISTS(SELECT 1 FROM ticket_priority WHERE id = ?)"), *updateRequest.PriorityID).Scan(&exists) //nolint:errcheck // Defaults to false
		if !exists {
			sendError(c, http.StatusBadRequest, "Invalid priority ID")
			return
		}
	}

	// Build dynamic UPDATE query
	var setClauses []string
	var args []interface{}

	if updateRequest.Title != nil {
		setClauses = append(setClauses, "title = ?")
		args = append(args, *updateRequest.Title)
	}
	if updateRequest.QueueID != nil {
		setClauses = append(setClauses, "queue_id = ?")
		args = append(args, *updateRequest.QueueID)
	}
	if updateRequest.TypeID != nil {
		setClauses = append(setClauses, fmt.Sprintf("%s = ?", database.TicketTypeColumn()))
		args = append(args, *updateRequest.TypeID)
	}
	if updateRequest.StateID != nil {
		setClauses = append(setClauses, "ticket_state_id = ?")
		args = append(args, *updateRequest.StateID)
	}
	if updateRequest.PriorityID != nil {
		setClauses = append(setClauses, "ticket_priority_id = ?")
		args = append(args, *updateRequest.PriorityID)
	}
	if updateRequest.CustomerUserID != nil {
		setClauses = append(setClauses, "customer_user_id = ?")
		args = append(args, *updateRequest.CustomerUserID)
	}
	if updateRequest.CustomerID != nil {
		setClauses = append(setClauses, "customer_id = ?")
		args = append(args, *updateRequest.CustomerID)
	}
	if updateRequest.ResponsibleUserID != nil {
		setClauses = append(setClauses, "responsible_user_id = ?")
		args = append(args, *updateRequest.ResponsibleUserID)
	}
	if updateRequest.OwnerID != nil {
		setClauses = append(setClauses, "user_id = ?")
		args = append(args, *updateRequest.OwnerID)
	}
	if updateRequest.LockID != nil {
		setClauses = append(setClauses, "ticket_lock_id = ?")
		args = append(args, *updateRequest.LockID)
	}

	// Always update change_time and change_by
	setClauses = append(setClauses, "change_time = NOW()")
	setClauses = append(setClauses, "change_by = ?")
	args = append(args, userID)

	// Add ticket ID for WHERE clause
	args = append(args, ticketID)

	updateQuery := database.ConvertPlaceholders(fmt.Sprintf(
		"UPDATE ticket SET %s WHERE id = ?",
		strings.Join(setClauses, ", "),
	))

	// Execute update
	result, err := db.Exec(updateQuery, args...)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to update ticket: "+err.Error())
		return
	}

	rowsAffected, _ := result.RowsAffected() //nolint:errcheck // Defaults to 0
	if rowsAffected == 0 {
		sendError(c, http.StatusNotFound, "Ticket not found")
		return
	}

	// Fetch updated ticket
	var ticket models.Ticket
	typeSelectUpdated := fmt.Sprintf("%s AS type_id", database.TicketTypeColumn())
	err = db.QueryRow(database.ConvertPlaceholders(fmt.Sprintf(`
		SELECT id, tn, title, queue_id, %s, ticket_state_id, 
		       ticket_priority_id, customer_user_id, customer_id,
		       ticket_lock_id, user_id, responsible_user_id,
		       create_time, change_time
		FROM ticket WHERE id = ?
	`, typeSelectUpdated)), ticketID).Scan(
		&ticket.ID, &ticket.TicketNumber, &ticket.Title, &ticket.QueueID,
		&ticket.TypeID, &ticket.TicketStateID, &ticket.TicketPriorityID,
		&ticket.CustomerUserID, &ticket.CustomerID, &ticket.TicketLockID,
		&ticket.UserID, &ticket.ResponsibleUserID,
		&ticket.CreateTime, &ticket.ChangeTime,
	)

	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to fetch updated ticket")
		return
	}

	sendSuccess(c, ticket)
}

// HandleDeleteTicket archives a ticket (OTRS doesn't hard delete tickets) - exported for YAML routing.
func (router *APIRouter) HandleDeleteTicket(c *gin.Context) {
	ticketIDStr := c.Param("id")
	if ticketIDStr == "" {
		sendError(c, http.StatusBadRequest, "Ticket ID required")
		return
	}

	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid ticket ID")
		return
	}

	// Get user ID
	userID := getContextUserID(c)

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusServiceUnavailable, "Database unavailable")
		return
	}

	// Check if ticket exists and get current state
	var currentStateID int
	var customerUserID string
	err = db.QueryRow(database.ConvertPlaceholders(
		"SELECT ticket_state_id, customer_user_id FROM ticket WHERE id = ?",
	), ticketID).Scan(&currentStateID, &customerUserID)

	if err != nil {
		if err == sql.ErrNoRows {
			sendError(c, http.StatusNotFound, "Ticket not found")
		} else {
			sendError(c, http.StatusInternalServerError, "Database error")
		}
		return
	}

	// Check permissions for customer users
	if isCustomer, _ := c.Get("is_customer"); isCustomer == true {
		// Customers cannot delete tickets
		sendError(c, http.StatusForbidden, "Customers cannot delete tickets")
		return
	}

	// Check if ticket is already archived/closed
	// States: 2 = closed successful, 3 = closed unsuccessful, 9 = merged
	if currentStateID == 2 || currentStateID == 3 || currentStateID == 9 {
		sendError(c, http.StatusBadRequest, "Ticket is already closed")
		return
	}

	// Archive the ticket by setting state to "closed successful" and archive_flag to 1
	// In OTRS, tickets are never actually deleted, just archived
	updateQuery := database.ConvertPlaceholders(`
		UPDATE ticket 
		SET ticket_state_id = 2,
		    archive_flag = 1,
		    change_time = NOW(),
		    change_by = ?
		WHERE id = ?
	`)

	result, err := db.Exec(updateQuery, userID, ticketID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to archive ticket: "+err.Error())
		return
	}

	rowsAffected, _ := result.RowsAffected() //nolint:errcheck // Defaults to 0
	if rowsAffected == 0 {
		sendError(c, http.StatusNotFound, "Ticket not found")
		return
	}

	// Add a final article noting the ticket was archived
	insertArticleQuery := database.ConvertPlaceholders(`
		INSERT INTO article (
			ticket_id,
			article_sender_type_id,
			communication_channel_id,
			is_visible_for_customer,
			search_index_needs_rebuild,
			create_time,
			create_by,
			change_time,
			change_by
		) VALUES (
			?, 1, 1, 0, 0, NOW(), ?, NOW(), ?
		)
	`)

	articleResult, err := db.Exec(insertArticleQuery, ticketID, userID, userID)
	if err == nil {
		articleID, _ := articleResult.LastInsertId() //nolint:errcheck // Best effort

		// Insert article content
		insertMimeQuery := database.ConvertPlaceholders(`
			INSERT INTO article_data_mime (
				article_id,
				a_subject,
				a_body,
				a_content_type,
				incoming_time,
				create_time,
				create_by,
				change_time,
				change_by
			) VALUES (
				?, 'Ticket Archived', 'This ticket has been archived.', 'text/plain', 
				?, NOW(), ?, NOW(), ?
			)
		`)

		_, _ = db.Exec(insertMimeQuery, articleID, time.Now().Unix(), userID, userID) //nolint:errcheck // Best effort
	}

	sendSuccess(c, gin.H{
		"id":           ticketID,
		"archived_at":  time.Now().UTC(),
		"message":      "Ticket archived successfully",
		"state_id":     2,
		"archive_flag": 1,
	})
}

// HandleAssignTicket assigns a ticket to a user.
func (router *APIRouter) HandleAssignTicket(c *gin.Context) {
	ticketIDStr := c.Param("id")
	if ticketIDStr == "" {
		sendError(c, http.StatusBadRequest, "Ticket ID required")
		return
	}

	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid ticket ID")
		return
	}

	var assignRequest struct {
		AssignedTo int    `json:"assigned_to" binding:"required"`
		Comment    string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&assignRequest); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid assign request: "+err.Error())
		return
	}

	// Get user ID from context
	userID := getContextUserID(c)

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusServiceUnavailable, "Database unavailable")
		return
	}

	// Check if ticket exists
	var currentResponsibleID sql.NullInt32
	var title string
	err = db.QueryRow(database.ConvertPlaceholders(
		"SELECT responsible_user_id, title FROM ticket WHERE id = ?",
	), ticketID).Scan(&currentResponsibleID, &title)

	if err != nil {
		if err == sql.ErrNoRows {
			sendError(c, http.StatusNotFound, "Ticket not found")
		} else {
			sendError(c, http.StatusInternalServerError, "Database error")
		}
		return
	}

	// Check if the user to assign to exists
	var assigneeLogin string
	err = db.QueryRow(database.ConvertPlaceholders(
		"SELECT login FROM users WHERE id = ? AND valid_id = 1",
	), assignRequest.AssignedTo).Scan(&assigneeLogin)

	if err != nil {
		if err == sql.ErrNoRows {
			sendError(c, http.StatusBadRequest, "User not found or inactive")
		} else {
			sendError(c, http.StatusInternalServerError, "Database error")
		}
		return
	}

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to start transaction")
		return
	}
	defer func() { _ = tx.Rollback() }()

	// Update ticket with new responsible user
	updateQuery := database.ConvertPlaceholders(`
		UPDATE ticket 
		SET responsible_user_id = ?,
		    change_time = NOW(),
		    change_by = ?
		WHERE id = ?
	`)

	_, err = tx.Exec(updateQuery, assignRequest.AssignedTo, userID, ticketID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to assign ticket")
		return
	}

	// Add article documenting the assignment
	if assignRequest.Comment != "" || true { // Always document assignment
		insertArticleQuery := database.ConvertPlaceholders(`
			INSERT INTO article (
				ticket_id,
				article_sender_type_id,
				communication_channel_id,
				is_visible_for_customer,
				search_index_needs_rebuild,
				create_time,
				create_by,
				change_time,
				change_by
			) VALUES (
				?, 1, 1, 0, 0, NOW(), ?, NOW(), ?
			)
		`)

		articleResult, err := tx.Exec(insertArticleQuery, ticketID, userID, userID)
		if err == nil {
			articleID, _ := articleResult.LastInsertId() //nolint:errcheck // Best effort

			// Build assignment message
			var previousAssignee string
			if currentResponsibleID.Valid {
				_ = db.QueryRow(database.ConvertPlaceholders( //nolint:errcheck // Defaults to empty
					"SELECT login FROM users WHERE id = ?",
				), currentResponsibleID.Int32).Scan(&previousAssignee)
			}

			var body string
			if previousAssignee != "" {
				body = fmt.Sprintf("Ticket reassigned from %s to %s.", previousAssignee, assigneeLogin)
			} else {
				body = fmt.Sprintf("Ticket assigned to %s.", assigneeLogin)
			}

			if assignRequest.Comment != "" {
				body += "\n\nComment: " + assignRequest.Comment
			}

			// Insert article content
			insertMimeQuery := database.ConvertPlaceholders(`
				INSERT INTO article_data_mime (
					article_id,
					a_subject,
					a_body,
					a_content_type,
					incoming_time,
					create_time,
					create_by,
					change_time,
					change_by
				) VALUES (
					?, 'Ticket Assignment', ?, 'text/plain', 
					?, NOW(), ?, NOW(), ?
				)
			`)

			_, _ = tx.Exec(insertMimeQuery, articleID, body, time.Now().Unix(), userID, userID) //nolint:errcheck // Best effort
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to commit transaction")
		return
	}

	// Return success response
	sendSuccess(c, gin.H{
		"id":          ticketID,
		"assigned_to": assignRequest.AssignedTo,
		"assignee":    assigneeLogin,
		"comment":     assignRequest.Comment,
		"assigned_at": time.Now().UTC(),
	})
}

// HandleCloseTicket closes a ticket.
func (router *APIRouter) HandleCloseTicket(c *gin.Context) {
	ticketIDStr := c.Param("id")
	if ticketIDStr == "" {
		sendError(c, http.StatusBadRequest, "Ticket ID required")
		return
	}

	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid ticket ID")
		return
	}

	var closeRequest struct {
		Resolution string `json:"resolution" binding:"required"`
		Comment    string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&closeRequest); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid close request: "+err.Error())
		return
	}

	// Get user ID from context
	userID := getContextUserID(c)

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusServiceUnavailable, "Database unavailable")
		return
	}

	// Check if ticket exists and get current state
	var currentStateID int
	var title string
	err = db.QueryRow(database.ConvertPlaceholders(
		"SELECT ticket_state_id, title FROM ticket WHERE id = ?",
	), ticketID).Scan(&currentStateID, &title)

	if err != nil {
		if err == sql.ErrNoRows {
			sendError(c, http.StatusNotFound, "Ticket not found")
		} else {
			sendError(c, http.StatusInternalServerError, "Database error")
		}
		return
	}

	// Check if ticket is already closed
	if currentStateID == 2 || currentStateID == 3 {
		sendError(c, http.StatusBadRequest, "Ticket is already closed")
		return
	}

	// Determine close state based on resolution
	var newStateID int
	if strings.ToLower(closeRequest.Resolution) == "successful" ||
		strings.ToLower(closeRequest.Resolution) == "resolved" ||
		strings.ToLower(closeRequest.Resolution) == "fixed" {
		newStateID = 2 // closed successful
	} else {
		newStateID = 3 // closed unsuccessful
	}

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to start transaction")
		return
	}
	defer func() { _ = tx.Rollback() }()

	// Update ticket state
	updateQuery := database.ConvertPlaceholders(`
		UPDATE ticket 
		SET ticket_state_id = ?,
		    change_time = NOW(),
		    change_by = ?
		WHERE id = ?
	`)

	_, err = tx.Exec(updateQuery, newStateID, userID, ticketID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to close ticket")
		return
	}

	// Add article documenting the closure
	if closeRequest.Comment != "" {
		insertArticleQuery := database.ConvertPlaceholders(`
			INSERT INTO article (
				ticket_id,
				article_sender_type_id,
				communication_channel_id,
				is_visible_for_customer,
				search_index_needs_rebuild,
				create_time,
				create_by,
				change_time,
				change_by
			) VALUES (
				?, 1, 1, 1, 0, NOW(), ?, NOW(), ?
			)
		`)

		articleResult, err := tx.Exec(insertArticleQuery, ticketID, userID, userID)
		if err == nil {
			articleID, _ := articleResult.LastInsertId() //nolint:errcheck // Best effort

			// Insert article content
			subject := fmt.Sprintf("Ticket Closed: %s", closeRequest.Resolution)
			body := closeRequest.Comment
			if body == "" {
				body = fmt.Sprintf("Ticket has been closed as %s.", closeRequest.Resolution)
			}

			insertMimeQuery := database.ConvertPlaceholders(`
				INSERT INTO article_data_mime (
					article_id,
					a_subject,
					a_body,
					a_content_type,
					incoming_time,
					create_time,
					create_by,
					change_time,
					change_by
				) VALUES (
					?, ?, ?, 'text/plain', 
					?, NOW(), ?, NOW(), ?
				)
			`)

			_, _ = tx.Exec(insertMimeQuery, articleID, subject, body, time.Now().Unix(), userID, userID) //nolint:errcheck // Best effort
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to commit transaction")
		return
	}

	// Return success response
	stateName := "closed successful"
	if newStateID == 3 {
		stateName = "closed unsuccessful"
	}

	sendSuccess(c, gin.H{
		"id":         ticketID,
		"state_id":   newStateID,
		"state":      stateName,
		"resolution": closeRequest.Resolution,
		"comment":    closeRequest.Comment,
		"closed_at":  time.Now().UTC(),
	})
}

// HandleReopenTicket reopens a closed ticket.
func (router *APIRouter) HandleReopenTicket(c *gin.Context) {
	ticketIDStr := c.Param("id")
	if ticketIDStr == "" {
		sendError(c, http.StatusBadRequest, "Ticket ID required")
		return
	}

	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid ticket ID")
		return
	}

	var reopenRequest struct {
		Reason string `json:"reason" binding:"required"`
	}

	if err := c.ShouldBindJSON(&reopenRequest); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid reopen request: "+err.Error())
		return
	}

	// Get user ID from context
	userID := getContextUserID(c)

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusServiceUnavailable, "Database unavailable")
		return
	}

	// Check if ticket exists and get current state
	var currentStateID int
	var title string
	err = db.QueryRow(database.ConvertPlaceholders(
		"SELECT ticket_state_id, title FROM ticket WHERE id = ?",
	), ticketID).Scan(&currentStateID, &title)

	if err != nil {
		if err == sql.ErrNoRows {
			sendError(c, http.StatusNotFound, "Ticket not found")
		} else {
			sendError(c, http.StatusInternalServerError, "Database error")
		}
		return
	}

	// Check if ticket is not closed
	if currentStateID != 2 && currentStateID != 3 {
		sendError(c, http.StatusBadRequest, "Ticket is not closed")
		return
	}

	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to start transaction")
		return
	}
	defer func() { _ = tx.Rollback() }()

	// Update ticket state to open
	updateQuery := database.ConvertPlaceholders(`
		UPDATE ticket 
		SET ticket_state_id = 4,
		    archive_flag = 0,
		    change_time = NOW(),
		    change_by = ?
		WHERE id = ?
	`)

	_, err = tx.Exec(updateQuery, userID, ticketID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to reopen ticket")
		return
	}

	// Add article documenting the reopening
	insertArticleQuery := database.ConvertPlaceholders(`
		INSERT INTO article (
			ticket_id,
			article_sender_type_id,
			communication_channel_id,
			is_visible_for_customer,
			search_index_needs_rebuild,
			create_time,
			create_by,
			change_time,
			change_by
		) VALUES (
			?, 1, 1, 1, 0, NOW(), ?, NOW(), ?
		)
	`)

	articleResult, err := tx.Exec(insertArticleQuery, ticketID, userID, userID)
	if err == nil {
		articleID, _ := articleResult.LastInsertId() //nolint:errcheck // Best effort

		// Insert article content
		insertMimeQuery := database.ConvertPlaceholders(`
			INSERT INTO article_data_mime (
				article_id,
				a_subject,
				a_body,
				a_content_type,
				incoming_time,
				create_time,
				create_by,
				change_time,
				change_by
			) VALUES (
				?, 'Ticket Reopened', ?, 'text/plain', 
				?, NOW(), ?, NOW(), ?
			)
		`)

		body := fmt.Sprintf("Ticket has been reopened. Reason: %s", reopenRequest.Reason)
		_, _ = tx.Exec(insertMimeQuery, articleID, body, time.Now().Unix(), userID, userID) //nolint:errcheck // Best effort
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to commit transaction")
		return
	}

	// Return success response
	sendSuccess(c, gin.H{
		"id":          ticketID,
		"state_id":    4,
		"state":       "open",
		"reason":      reopenRequest.Reason,
		"reopened_at": time.Now().UTC(),
	})
}

// handleUpdateTicketPriority updates ticket priority.
func (router *APIRouter) handleUpdateTicketPriority(c *gin.Context) {
	ticketIDStr := c.Param("id")
	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid ticket ID")
		return
	}

	var priorityRequest struct {
		PriorityID int    `json:"priority_id" binding:"required"`
		Comment    string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&priorityRequest); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid priority request: "+err.Error())
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	userID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = idInt
		}
	}

	now := time.Now()
	query := database.ConvertQuery(`
		UPDATE ticket SET ticket_priority_id = ?, change_time = ?, change_by = ?
		WHERE id = ? AND archive_flag = 0
	`)

	result, err := db.Exec(query, priorityRequest.PriorityID, now, userID, ticketID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to update priority")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		sendError(c, http.StatusNotFound, "Ticket not found")
		return
	}

	sendSuccess(c, gin.H{
		"id":          ticketID,
		"priority_id": priorityRequest.PriorityID,
		"updated_at":  now,
	})
}

// handleMoveTicketQueue moves ticket to a different queue.
func (router *APIRouter) handleMoveTicketQueue(c *gin.Context) {
	ticketIDStr := c.Param("id")
	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid ticket ID")
		return
	}

	var queueRequest struct {
		QueueID int    `json:"queue_id" binding:"required"`
		Comment string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&queueRequest); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid queue request: "+err.Error())
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	userID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = idInt
		}
	}

	now := time.Now()
	query := database.ConvertQuery(`
		UPDATE ticket SET queue_id = ?, change_time = ?, change_by = ?
		WHERE id = ? AND archive_flag = 0
	`)

	result, err := db.Exec(query, queueRequest.QueueID, now, userID, ticketID)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to move ticket")
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		sendError(c, http.StatusNotFound, "Ticket not found")
		return
	}

	sendSuccess(c, gin.H{
		"id":       ticketID,
		"queue_id": queueRequest.QueueID,
		"moved_at": now,
	})
}

// handleGetTicketArticles returns ticket articles/messages.
func (router *APIRouter) handleGetTicketArticles(c *gin.Context) {
	ticketIDStr := c.Param("id")
	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid ticket ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendSuccess(c, []interface{}{})
		return
	}

	query := database.ConvertQuery(`
		SELECT a.id, a.ticket_id, 
			adm.a_from, adm.a_to, adm.a_subject, adm.a_body,
			ast.name as sender_type, acc.name as channel,
			a.is_visible_for_customer, a.create_time
		FROM article a
		LEFT JOIN article_data_mime adm ON adm.article_id = a.id
		LEFT JOIN article_sender_type ast ON ast.id = a.article_sender_type_id
		LEFT JOIN communication_channel acc ON acc.id = a.communication_channel_id
		WHERE a.ticket_id = ?
		ORDER BY a.create_time ASC
	`)

	rows, err := db.Query(query, ticketID)
	if err != nil {
		sendSuccess(c, []interface{}{})
		return
	}
	defer rows.Close()

	articles := []gin.H{}
	for rows.Next() {
		var id, tktID int
		var from, to, subject, body, senderType, channel *string
		var visibleForCustomer int
		var createdAt time.Time

		if err := rows.Scan(&id, &tktID, &from, &to, &subject, &body, &senderType, &channel, &visibleForCustomer, &createdAt); err != nil {
			continue
		}

		articles = append(articles, gin.H{
			"id":          id,
			"ticket_id":   tktID,
			"from":        from,
			"to":          to,
			"subject":     subject,
			"body":        body,
			"sender_type": senderType,
			"channel":     channel,
			"visible":     visibleForCustomer == 1,
			"created_at":  createdAt,
		})
	}

	sendSuccess(c, articles)
}

// handleAddTicketArticle adds a new article to a ticket.
func (router *APIRouter) handleAddTicketArticle(c *gin.Context) { api.HandleCreateArticleAPI(c) }

// handleGetTicketArticle returns a specific article.
func (router *APIRouter) handleGetTicketArticle(c *gin.Context) {
	ticketIDStr := c.Param("id")
	articleIDStr := c.Param("article_id")

	ticketID, err := strconv.Atoi(ticketIDStr)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid ticket ID")
		return
	}

	articleID, err := strconv.Atoi(articleIDStr)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid article ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	query := database.ConvertQuery(`
		SELECT a.id, a.ticket_id, 
			adm.a_from, adm.a_to, adm.a_subject, adm.a_body,
			ast.name as sender_type, acc.name as channel,
			a.is_visible_for_customer, a.create_time
		FROM article a
		LEFT JOIN article_data_mime adm ON adm.article_id = a.id
		LEFT JOIN article_sender_type ast ON ast.id = a.article_sender_type_id
		LEFT JOIN communication_channel acc ON acc.id = a.communication_channel_id
		WHERE a.id = ? AND a.ticket_id = ?
	`)

	var id, tktID int
	var from, to, subject, body, senderType, channel *string
	var visibleForCustomer int
	var createdAt time.Time

	err = db.QueryRow(query, articleID, ticketID).Scan(&id, &tktID, &from, &to, &subject, &body, &senderType, &channel, &visibleForCustomer, &createdAt)
	if err != nil {
		sendError(c, http.StatusNotFound, "Article not found")
		return
	}

	sendSuccess(c, gin.H{
		"id":          id,
		"ticket_id":   tktID,
		"from":        from,
		"to":          to,
		"subject":     subject,
		"body":        body,
		"sender_type": senderType,
		"channel":     channel,
		"visible":     visibleForCustomer == 1,
		"created_at":  createdAt,
	})
}

// Bulk operations

// handleBulkAssignTickets assigns multiple tickets to a user.
func (router *APIRouter) handleBulkAssignTickets(c *gin.Context) {
	var bulkRequest struct {
		TicketIDs  []int  `json:"ticket_ids" binding:"required"`
		AssignedTo int    `json:"assigned_to" binding:"required"`
		Comment    string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&bulkRequest); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid bulk assign request: "+err.Error())
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	userID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = idInt
		}
	}

	now := time.Now()
	updated := 0

	for _, ticketID := range bulkRequest.TicketIDs {
		query := database.ConvertQuery(`
			UPDATE ticket SET user_id = ?, change_time = ?, change_by = ?
			WHERE id = ? AND archive_flag = 0
		`)
		result, err := db.Exec(query, bulkRequest.AssignedTo, now, userID, ticketID)
		if err == nil {
			if affected, _ := result.RowsAffected(); affected > 0 {
				updated++
			}
		}
	}

	sendSuccess(c, gin.H{
		"ticket_ids":  bulkRequest.TicketIDs,
		"assigned_to": bulkRequest.AssignedTo,
		"updated":     updated,
		"assigned_at": now,
	})
}

// handleBulkCloseTickets closes multiple tickets.
func (router *APIRouter) handleBulkCloseTickets(c *gin.Context) {
	var bulkRequest struct {
		TicketIDs  []int  `json:"ticket_ids" binding:"required"`
		Resolution string `json:"resolution" binding:"required"`
		Comment    string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&bulkRequest); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid bulk close request: "+err.Error())
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	userID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = idInt
		}
	}

	// Get closed state ID
	var closedStateID int
	stateQuery := database.ConvertQuery(`SELECT id FROM ticket_state WHERE name = 'closed successful' OR name = 'closed' LIMIT 1`)
	if err := db.QueryRow(stateQuery).Scan(&closedStateID); err != nil {
		closedStateID = 2 // Fallback
	}

	now := time.Now()
	closed := 0

	for _, ticketID := range bulkRequest.TicketIDs {
		query := database.ConvertQuery(`
			UPDATE ticket SET ticket_state_id = ?, change_time = ?, change_by = ?
			WHERE id = ? AND archive_flag = 0
		`)
		result, err := db.Exec(query, closedStateID, now, userID, ticketID)
		if err == nil {
			if affected, _ := result.RowsAffected(); affected > 0 {
				closed++
			}
		}
	}

	sendSuccess(c, gin.H{
		"ticket_ids": bulkRequest.TicketIDs,
		"resolution": bulkRequest.Resolution,
		"closed":     closed,
		"closed_at":  now,
	})
}

// handleBulkUpdatePriority updates priority for multiple tickets.
func (router *APIRouter) handleBulkUpdatePriority(c *gin.Context) {
	var bulkRequest struct {
		TicketIDs  []int `json:"ticket_ids" binding:"required"`
		PriorityID int   `json:"priority_id" binding:"required"`
		Comment    string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&bulkRequest); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid bulk priority request: "+err.Error())
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	userID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = idInt
		}
	}

	now := time.Now()
	updated := 0

	for _, ticketID := range bulkRequest.TicketIDs {
		query := database.ConvertQuery(`
			UPDATE ticket SET ticket_priority_id = ?, change_time = ?, change_by = ?
			WHERE id = ? AND archive_flag = 0
		`)
		result, err := db.Exec(query, bulkRequest.PriorityID, now, userID, ticketID)
		if err == nil {
			if affected, _ := result.RowsAffected(); affected > 0 {
				updated++
			}
		}
	}

	sendSuccess(c, gin.H{
		"ticket_ids":  bulkRequest.TicketIDs,
		"priority_id": bulkRequest.PriorityID,
		"updated":     updated,
		"updated_at":  now,
	})
}

// handleBulkMoveQueue moves multiple tickets to a queue.
func (router *APIRouter) handleBulkMoveQueue(c *gin.Context) {
	var bulkRequest struct {
		TicketIDs []int  `json:"ticket_ids" binding:"required"`
		QueueID   int    `json:"queue_id" binding:"required"`
		Comment   string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&bulkRequest); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid bulk move request: "+err.Error())
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	userID := 1
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = idInt
		}
	}

	now := time.Now()
	moved := 0

	for _, ticketID := range bulkRequest.TicketIDs {
		query := database.ConvertQuery(`
			UPDATE ticket SET queue_id = ?, change_time = ?, change_by = ?
			WHERE id = ? AND archive_flag = 0
		`)
		result, err := db.Exec(query, bulkRequest.QueueID, now, userID, ticketID)
		if err == nil {
			if affected, _ := result.RowsAffected(); affected > 0 {
				moved++
			}
		}
	}

	sendSuccess(c, gin.H{
		"ticket_ids": bulkRequest.TicketIDs,
		"queue_id":   bulkRequest.QueueID,
		"moved":      moved,
		"moved_at":   now,
	})
}

// Helper functions for mapping ticket states and priorities

func mapTicketState(stateID int) string {
	db, err := database.GetDB()
	if err != nil {
		return "unknown"
	}
	var stateRow struct {
		Name string
	}
	err = db.QueryRow(database.ConvertPlaceholders("SELECT name FROM ticket_state WHERE id = ?"), stateID).Scan(&stateRow.Name)
	if err == nil {
		return stateRow.Name
	}
	return "unknown"
}

func mapTicketPriority(priorityID int) string {
	db, err := database.GetDB()
	if err != nil {
		return "unknown"
	}
	var priorityRow struct {
		Name string
	}
	err = db.QueryRow(database.ConvertPlaceholders("SELECT name FROM ticket_priority WHERE id = ?"), priorityID).Scan(&priorityRow.Name)
	if err == nil {
		return priorityRow.Name
	}
	return "normal"
}
