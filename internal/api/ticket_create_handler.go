package api

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/service/ticket_number"
)

// HandleCreateTicketAPI handles ticket creation via API
func HandleCreateTicketAPI(c *gin.Context) {
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
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid ticket request: " + err.Error(),
		})
		return
	}

	// Validate title length
	if len(ticketRequest.Title) > 255 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Title too long (max 255 characters)",
		})
		return
	}

	// Get user ID from context (set by auth middleware or use default for testing)
	userID := 1
	if uid, exists := c.Get("user_id"); exists {
		if id, ok := uid.(int); ok {
			userID = id
		}
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Validate queue exists
	var queueExists bool
	err = db.QueryRow(database.ConvertPlaceholders("SELECT EXISTS(SELECT 1 FROM queue WHERE id = $1 AND valid_id = 1)"), ticketRequest.QueueID).Scan(&queueExists)
	if err != nil || !queueExists {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid queue_id",
		})
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to initialize ticket number generator",
		})
		return
	}

	// Generate ticket number
	ticketNumber, err := generator.Generate()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Failed to generate ticket number: %v", err),
		})
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to start transaction",
		})
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
			timeout, until_time, escalation_time, escalation_update_time,
			escalation_response_time, escalation_solution_time,
			create_time, create_by, change_time, change_by
		) VALUES (
			$1, $2, $3, $4, $5, 
			$6, $7, $8,
			1, $9, $10,
			0, 0, 0, 0, 0, 0,
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to create ticket: " + err.Error(),
		})
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
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to create article: " + err.Error(),
			})
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to commit transaction",
		})
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
		c.JSON(http.StatusCreated, gin.H{
			"success": true,
			"data": gin.H{
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
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    ticket,
	})
}