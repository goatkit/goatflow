package v1

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	. "github.com/gotrs-io/gotrs-ce/internal/api"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/service/ticket_number"
)

// handleListTickets returns a paginated list of tickets
func (router *APIRouter) handleListTickets(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "25"))
	status := c.Query("status")
	priority := c.Query("priority")
	assignedTo := c.Query("assigned_to")
	queueID := c.Query("queue_id")
	search := c.Query("search")

	// Get tickets from service
	ticketService := GetTicketService()
	request := &models.TicketListRequest{
		Page:     page,
		PerPage:  perPage,
		Status:   status,
		Priority: priority,
		QueueID:  queueID,
		Search:   search,
	}
	
	response, err := ticketService.ListTickets(request)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to retrieve tickets")
		return
	}
	
	// Convert to API format
	tickets := []gin.H{}
	for _, t := range response.Tickets {
		ticket := gin.H{
			"id":             t.ID,
			"number":         t.TicketNumber,
			"title":          t.Title,
			"status":         mapTicketState(t.TicketStateID),
			"priority":       mapTicketPriority(t.TicketPriorityID),
			"queue_id":       t.QueueID,
			"queue_name":     fmt.Sprintf("Queue %d", t.QueueID), // TODO: Get actual queue name
			"customer_email": t.CustomerUserID,
			"created_at":     t.CreateTime,
			"updated_at":     t.ChangeTime,
		}
		
		if t.UserID != nil {
			ticket["assigned_to"] = *t.UserID
			ticket["assigned_name"] = fmt.Sprintf("User %d", *t.UserID) // TODO: Get actual user name
		}
		
		tickets = append(tickets, ticket)
	}
	
	pagination := Pagination{
		Page:       response.Pagination.Page,
		PerPage:    response.Pagination.PerPage,
		Total:      response.Pagination.Total,
		TotalPages: response.Pagination.TotalPages,
		HasNext:    response.Pagination.HasNext,
		HasPrev:    response.Pagination.HasPrev,
	}

	sendPaginatedResponse(c, tickets, pagination)
}

// HandleCreateTicket creates a new ticket
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
	err = db.QueryRow(database.ConvertPlaceholders("SELECT EXISTS(SELECT 1 FROM queue WHERE id = $1 AND valid_id = 1)"), ticketRequest.QueueID).Scan(&queueExists)
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
	defer tx.Rollback()

	// Get database adapter
	adapter := database.GetAdapter()
	
	// Insert ticket
	ticketQuery := database.ConvertPlaceholders(`
		INSERT INTO ticket (
			tn, title, queue_id, type_id, ticket_state_id, 
			ticket_priority_id, customer_user_id, customer_id,
			ticket_lock_id, user_id, responsible_user_id,
			create_time, create_by, change_time, change_by
		) VALUES (
			$1, $2, $3, $4, $5, 
			$6, $7, $8,
			1, $9, $10,
			NOW(), $11, NOW(), $12
		) RETURNING id
	`)

	ticketID, err := adapter.InsertWithReturningTx(
		tx, 
		ticketQuery,
		ticketNumber, ticketRequest.Title, ticketRequest.QueueID, 
		ticketRequest.TypeID, ticketRequest.StateID,
		ticketRequest.PriorityID, ticketRequest.CustomerUserID, ticketRequest.CustomerID,
		userID, userID, userID, userID,
	)

	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to create ticket: " + err.Error())
		return
	}

	// Create initial article if provided
	if ticketRequest.Article != nil {
		subject, _ := ticketRequest.Article["subject"].(string)
		body, _ := ticketRequest.Article["body"].(string)
		contentType, _ := ticketRequest.Article["content_type"].(string)
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
				$1, $2, 1, 1, NOW(), $3, NOW(), $4
			) RETURNING id
		`)
		
		articleID, err := adapter.InsertWithReturningTx(
			tx,
			articleQuery,
			ticketID, senderTypeID, userID, userID,
		)
		
		if err != nil {
			sendError(c, http.StatusInternalServerError, "Failed to create article: " + err.Error())
			return
		}
		
		// Insert article content in article_data_mime table
		if subject != "" || body != "" {
			contentQuery := database.ConvertPlaceholders(`
				INSERT INTO article_data_mime (
					article_id, a_subject, a_body, a_content_type,
					create_time, create_by, change_time, change_by
				) VALUES (
					$1, $2, $3, $4,
					NOW(), $5, NOW(), $6
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
		ID               int64      `json:"id"`
		TicketNumber     string     `json:"tn"`
		Title            string     `json:"title"`
		QueueID          int        `json:"queue_id"`
		TypeID           int        `json:"type_id"`
		StateID          int        `json:"ticket_state_id"`
		PriorityID       int        `json:"ticket_priority_id"`
		CustomerUserID   *string    `json:"customer_user_id"`
		CustomerID       *string    `json:"customer_id"`
		CreateTime       time.Time  `json:"create_time"`
	}

	// Query the created ticket
	query := database.ConvertPlaceholders(`
		SELECT id, tn, title, queue_id, type_id, ticket_state_id,
		       ticket_priority_id, customer_user_id, customer_id, create_time
		FROM ticket
		WHERE id = $1
	`)
	
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
				"id":            ticketID,
				"tn":            ticketNumber,
				"title":         ticketRequest.Title,
				"queue_id":      ticketRequest.QueueID,
				"message":       "Ticket created successfully",
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

// handleGetTicket returns a specific ticket by ID
func (router *APIRouter) handleGetTicket(c *gin.Context) {
	ticketID := c.Param("id")
	if ticketID == "" {
		sendError(c, http.StatusBadRequest, "Ticket ID required")
		return
	}

	// TODO: Implement actual ticket retrieval
	ticket := gin.H{
		"id":             ticketID,
		"number":         "T-2025-" + ticketID,
		"title":          "Sample ticket details",
		"description":    "This is a detailed description of the ticket.",
		"status":         "open",
		"priority":       "normal",
		"queue_id":       1,
		"queue_name":     "General",
		"assigned_to":    1,
		"assigned_name":  "John Doe",
		"customer_email": "customer@example.com",
		"created_at":     time.Now().Add(-2 * time.Hour).UTC(),
		"updated_at":     time.Now().Add(-30 * time.Minute).UTC(),
		"sla_due":        time.Now().Add(4 * time.Hour).UTC(),
		"tags":           []string{"urgent", "billing"},
		"article_count":  3,
		"attachment_count": 2,
	}

	sendSuccess(c, ticket)
}

// handleUpdateTicket updates an existing ticket
func (router *APIRouter) handleUpdateTicket(c *gin.Context) {
	ticketID := c.Param("id")
	if ticketID == "" {
		sendError(c, http.StatusBadRequest, "Ticket ID required")
		return
	}

	var updateRequest struct {
		Title       *string   `json:"title"`
		Description *string   `json:"description"`
		Priority    *string   `json:"priority"`
		Status      *string   `json:"status"`
		AssignedTo  *int      `json:"assigned_to"`
		Tags        *[]string `json:"tags"`
	}

	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid update request: "+err.Error())
		return
	}

	// TODO: Implement actual ticket update
	updatedTicket := gin.H{
		"id":         ticketID,
		"updated_at": time.Now().UTC(),
		"changes":    updateRequest,
	}

	sendSuccess(c, updatedTicket)
}

// handleDeleteTicket deletes a ticket (soft delete)
func (router *APIRouter) handleDeleteTicket(c *gin.Context) {
	ticketID := c.Param("id")
	if ticketID == "" {
		sendError(c, http.StatusBadRequest, "Ticket ID required")
		return
	}

	// TODO: Implement actual ticket deletion (soft delete)
	sendSuccess(c, gin.H{
		"id":         ticketID,
		"deleted_at": time.Now().UTC(),
		"message":    "Ticket deleted successfully",
	})
}

// handleAssignTicket assigns a ticket to a user
func (router *APIRouter) handleAssignTicket(c *gin.Context) {
	ticketID := c.Param("id")
	if ticketID == "" {
		sendError(c, http.StatusBadRequest, "Ticket ID required")
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

	// TODO: Implement actual ticket assignment
	sendSuccess(c, gin.H{
		"id":           ticketID,
		"assigned_to":  assignRequest.AssignedTo,
		"comment":      assignRequest.Comment,
		"assigned_at":  time.Now().UTC(),
	})
}

// handleCloseTicket closes a ticket
func (router *APIRouter) handleCloseTicket(c *gin.Context) {
	ticketID := c.Param("id")
	if ticketID == "" {
		sendError(c, http.StatusBadRequest, "Ticket ID required")
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

	// TODO: Implement actual ticket closing
	sendSuccess(c, gin.H{
		"id":         ticketID,
		"status":     "closed",
		"resolution": closeRequest.Resolution,
		"comment":    closeRequest.Comment,
		"closed_at":  time.Now().UTC(),
	})
}

// handleReopenTicket reopens a closed ticket
func (router *APIRouter) handleReopenTicket(c *gin.Context) {
	ticketID := c.Param("id")
	if ticketID == "" {
		sendError(c, http.StatusBadRequest, "Ticket ID required")
		return
	}

	var reopenRequest struct {
		Reason string `json:"reason" binding:"required"`
	}

	if err := c.ShouldBindJSON(&reopenRequest); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid reopen request: "+err.Error())
		return
	}

	// TODO: Implement actual ticket reopening
	sendSuccess(c, gin.H{
		"id":         ticketID,
		"status":     "open",
		"reason":     reopenRequest.Reason,
		"reopened_at": time.Now().UTC(),
	})
}

// handleUpdateTicketPriority updates ticket priority
func (router *APIRouter) handleUpdateTicketPriority(c *gin.Context) {
	ticketID := c.Param("id")
	if ticketID == "" {
		sendError(c, http.StatusBadRequest, "Ticket ID required")
		return
	}

	var priorityRequest struct {
		Priority string `json:"priority" binding:"required"`
		Comment  string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&priorityRequest); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid priority request: "+err.Error())
		return
	}

	// TODO: Implement actual priority update
	sendSuccess(c, gin.H{
		"id":         ticketID,
		"priority":   priorityRequest.Priority,
		"comment":    priorityRequest.Comment,
		"updated_at": time.Now().UTC(),
	})
}

// handleMoveTicketQueue moves ticket to a different queue
func (router *APIRouter) handleMoveTicketQueue(c *gin.Context) {
	ticketID := c.Param("id")
	if ticketID == "" {
		sendError(c, http.StatusBadRequest, "Ticket ID required")
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

	// TODO: Implement actual queue move
	sendSuccess(c, gin.H{
		"id":         ticketID,
		"queue_id":   queueRequest.QueueID,
		"comment":    queueRequest.Comment,
		"moved_at":   time.Now().UTC(),
	})
}

// handleGetTicketArticles returns ticket articles/messages
func (router *APIRouter) handleGetTicketArticles(c *gin.Context) {
	ticketID := c.Param("id")
	if ticketID == "" {
		sendError(c, http.StatusBadRequest, "Ticket ID required")
		return
	}

	// TODO: Implement actual article retrieval
	articles := []gin.H{
		{
			"id":         1,
			"ticket_id":  ticketID,
			"from":       "customer@example.com",
			"to":         "support@company.com",
			"subject":    "Initial inquiry",
			"body":       "This is the original ticket content.",
			"type":       "email",
			"created_at": time.Now().Add(-2 * time.Hour).UTC(),
		},
		{
			"id":         2,
			"ticket_id":  ticketID,
			"from":       "agent@company.com",
			"to":         "customer@example.com",
			"subject":    "Re: Initial inquiry",
			"body":       "Thank you for contacting us. We are investigating.",
			"type":       "email",
			"created_at": time.Now().Add(-1 * time.Hour).UTC(),
		},
	}

	sendSuccess(c, articles)
}

// handleAddTicketArticle adds a new article to a ticket
func (router *APIRouter) handleAddTicketArticle(c *gin.Context) {
	ticketID := c.Param("id")
	if ticketID == "" {
		sendError(c, http.StatusBadRequest, "Ticket ID required")
		return
	}

	var articleRequest struct {
		Subject string `json:"subject" binding:"required"`
		Body    string `json:"body" binding:"required"`
		To      string `json:"to" binding:"required,email"`
		Type    string `json:"type"` // email, note, phone
		Visible bool   `json:"visible"` // visible to customer
	}

	if err := c.ShouldBindJSON(&articleRequest); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid article request: "+err.Error())
		return
	}

	userID, email, _, exists := middleware.GetCurrentUser(c)
	if !exists {
		sendError(c, http.StatusUnauthorized, "User not authenticated")
		return
	}

	// TODO: Implement actual article creation
	article := gin.H{
		"id":         123,
		"ticket_id":  ticketID,
		"from":       email,
		"to":         articleRequest.To,
		"subject":    articleRequest.Subject,
		"body":       articleRequest.Body,
		"type":       articleRequest.Type,
		"visible":    articleRequest.Visible,
		"created_by": userID,
		"created_at": time.Now().UTC(),
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data:    article,
	})
}

// handleGetTicketArticle returns a specific article
func (router *APIRouter) handleGetTicketArticle(c *gin.Context) {
	ticketID := c.Param("id")
	articleID := c.Param("article_id")
	
	if ticketID == "" || articleID == "" {
		sendError(c, http.StatusBadRequest, "Ticket ID and Article ID required")
		return
	}

	// TODO: Implement actual article retrieval
	article := gin.H{
		"id":         articleID,
		"ticket_id":  ticketID,
		"from":       "agent@company.com",
		"to":         "customer@example.com",
		"subject":    "Ticket update",
		"body":       "Here is the detailed response to your inquiry.",
		"type":       "email",
		"visible":    true,
		"created_at": time.Now().Add(-30 * time.Minute).UTC(),
		"attachments": []gin.H{
			{
				"id":       1,
				"filename": "response.pdf",
				"size":     12345,
				"type":     "application/pdf",
			},
		},
	}

	sendSuccess(c, article)
}

// Bulk operations

// handleBulkAssignTickets assigns multiple tickets to a user
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

	// TODO: Implement actual bulk assignment
	sendSuccess(c, gin.H{
		"ticket_ids":   bulkRequest.TicketIDs,
		"assigned_to":  bulkRequest.AssignedTo,
		"comment":      bulkRequest.Comment,
		"assigned_at":  time.Now().UTC(),
		"count":        len(bulkRequest.TicketIDs),
	})
}

// handleBulkCloseTickets closes multiple tickets
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

	// TODO: Implement actual bulk closing
	sendSuccess(c, gin.H{
		"ticket_ids": bulkRequest.TicketIDs,
		"resolution": bulkRequest.Resolution,
		"comment":    bulkRequest.Comment,
		"closed_at":  time.Now().UTC(),
		"count":      len(bulkRequest.TicketIDs),
	})
}

// handleBulkUpdatePriority updates priority for multiple tickets
func (router *APIRouter) handleBulkUpdatePriority(c *gin.Context) {
	var bulkRequest struct {
		TicketIDs []int  `json:"ticket_ids" binding:"required"`
		Priority  string `json:"priority" binding:"required"`
		Comment   string `json:"comment"`
	}

	if err := c.ShouldBindJSON(&bulkRequest); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid bulk priority request: "+err.Error())
		return
	}

	// TODO: Implement actual bulk priority update
	sendSuccess(c, gin.H{
		"ticket_ids": bulkRequest.TicketIDs,
		"priority":   bulkRequest.Priority,
		"comment":    bulkRequest.Comment,
		"updated_at": time.Now().UTC(),
		"count":      len(bulkRequest.TicketIDs),
	})
}

// handleBulkMoveQueue moves multiple tickets to a queue
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

	// TODO: Implement actual bulk queue move
	sendSuccess(c, gin.H{
		"ticket_ids": bulkRequest.TicketIDs,
		"queue_id":   bulkRequest.QueueID,
		"comment":    bulkRequest.Comment,
		"moved_at":   time.Now().UTC(),
		"count":      len(bulkRequest.TicketIDs),
	})
}

// Helper functions for mapping ticket states and priorities

func mapTicketState(stateID int) string {
	switch stateID {
	case 1:
		return "new"
	case 2:
		return "open"
	case 3:
		return "pending"
	case 4:
		return "resolved"
	case 5, 6:
		return "closed"
	default:
		return "unknown"
	}
}

func mapTicketPriority(priorityID int) string {
	switch priorityID {
	case 1:
		return "low"
	case 2, 3:
		return "normal"
	case 4:
		return "high"
	case 5:
		return "urgent"
	default:
		return "normal"
	}
}