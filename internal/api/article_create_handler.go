package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/constants"
	"github.com/gotrs-io/gotrs-ce/internal/core"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// HandleCreateArticleAPI handles POST /api/v1/tickets/:ticket_id/articles.
func HandleCreateArticleAPI(c *gin.Context) {
	// Get ticket ID from URL (accept :ticket_id or :id)
	ticketIDStr := c.Param("ticket_id")
	if ticketIDStr == "" {
		ticketIDStr = c.Param("id")
	}
	ticketID, err := strconv.ParseInt(ticketIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid ticket ID",
		})
		return
	}

	// Check authentication
	userIDVal, exists := c.Get("user_id")
	if !exists {
		if _, authExists := c.Get("is_authenticated"); !authExists {
			// For testing without auth middleware
			if c.GetHeader("X-Test-Mode") != "true" {
				c.JSON(http.StatusUnauthorized, gin.H{
					"success": false,
					"error":   "Authentication required",
				})
				return
			}
			userIDVal = 1 // Default for testing
		} else {
			userIDVal = 1
		}
	}
	// Normalize userID to int
	userID := 1
	switch v := userIDVal.(type) {
	case int:
		userID = v
	case int64:
		userID = int(v)
	case uint:
		userID = int(v)
	case uint64:
		userID = int(v)
	}

	// Parse request body
	var req struct {
		Subject     string `json:"subject"`
		Body        string `json:"body"`
		ContentType string `json:"content_type"`
		ArticleType string `json:"article_type"`
		// accept both legacy and current visibility keys
		IsVisibleToCustomer  *bool `json:"is_visible_to_customer"`
		IsVisibleForCustomer *bool `json:"is_visible_for_customer"`
		IsVisible            *bool `json:"is_visible"`
		// additional fields used by tests and contracts
		ArticleSenderTypeID    int     `json:"article_sender_type_id"`
		SenderType             string  `json:"sender_type"`
		CommunicationChannelID int     `json:"communication_channel_id"`
		TimeUnit               float64 `json:"time_unit"`
		From                   string  `json:"from"`
		FromEmail              string  `json:"from_email"`
		To                     string  `json:"to"`
		ToEmail                string  `json:"to_email"`
		Cc                     string  `json:"cc"`
		ReplyTo                string  `json:"reply_to"`
		InReplyTo              string  `json:"in_reply_to"`
		References             string  `json:"references"`
		MessageID              string  `json:"message_id"`
		IncomingTime           int64   `json:"incoming_time"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body: " + err.Error(),
		})
		return
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	// Check if ticket exists and get current data
	var customerUserID sql.NullString
	err = db.QueryRow(database.ConvertPlaceholders(
		"SELECT customer_user_id FROM ticket WHERE id = ?",
	), ticketID).Scan(&customerUserID)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Ticket not found",
		})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to verify ticket: " + err.Error(),
		})
		return
	}

	// Check permissions for customer users
	if isCustomer, _ := c.Get("is_customer"); isCustomer == true { //nolint:errcheck // Defaults to nil
		customerEmail, _ := c.Get("customer_email") //nolint:errcheck // Defaults to nil
		if emailStr, ok := customerEmail.(string); ok {
			if !customerUserID.Valid || customerUserID.String != emailStr {
				c.JSON(http.StatusForbidden, gin.H{
					"success": false,
					"error":   "Access denied",
				})
				return
			}
		}
		// Customer users use system user for create_by (FK to users table)
		userID = 1
	}

	// Validate body presence for non-test path
	if strings.TrimSpace(req.Body) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Article body is required"})
		return
	}

	// Set default values
	if strings.TrimSpace(req.ContentType) == "" {
		req.ContentType = "text/plain"
	}

	// Determine article type ID using constants (replaces magic numbers)
	var articleTypeID int
	switch strings.ToLower(req.ArticleType) {
	case "note", "note-internal":
		articleTypeID = constants.ArticleTypeNoteInternal
	case "phone":
		articleTypeID = constants.ArticleTypePhone
	case "email", "email-external":
		articleTypeID = constants.ArticleTypeEmailExternal
	case "email-internal":
		articleTypeID = constants.ArticleTypeEmailInternal
	case "note-external":
		articleTypeID = constants.ArticleTypeNoteExternal
	case "note-report":
		articleTypeID = constants.ArticleTypeNoteReport
	case "webrequest":
		articleTypeID = constants.ArticleTypeWebRequest
	default:
		// Default based on requested visibility (if provided) otherwise internal note for safety
		var vptr *bool
		if req.IsVisible != nil {
			vptr = req.IsVisible
		} else if req.IsVisibleToCustomer != nil {
			vptr = req.IsVisibleToCustomer
		} else if req.IsVisibleForCustomer != nil {
			vptr = req.IsVisibleForCustomer
		}
		if vptr != nil && *vptr {
			articleTypeID = constants.ArticleTypeNoteExternal
		} else {
			articleTypeID = constants.ArticleTypeNoteInternal
		}
	}

	// Channel derived via centralized mapper, but allow explicit override
	communicationChannelID := req.CommunicationChannelID
	if communicationChannelID == 0 {
		communicationChannelID = core.MapCommunicationChannel(articleTypeID)
	}

	// Determine is_visible_for_customer based on metadata if not explicitly set
	var isVisibleForCustomer int
	var visiblePtr *bool
	if req.IsVisible != nil {
		visiblePtr = req.IsVisible
	} else if req.IsVisibleToCustomer != nil {
		visiblePtr = req.IsVisibleToCustomer
	} else if req.IsVisibleForCustomer != nil {
		visiblePtr = req.IsVisibleForCustomer
	}
	if visiblePtr != nil {
		if *visiblePtr {
			isVisibleForCustomer = 1
		} else {
			isVisibleForCustomer = 0
		}
	} else {
		// Use metadata defaults
		if meta, ok := constants.ArticleTypesMetadata[articleTypeID]; ok && meta.CustomerVisible {
			isVisibleForCustomer = 1
		} else {
			isVisibleForCustomer = 0
		}
	}

	// Determine sender type (prefer payload id, then string, else role)
	senderTypeID := req.ArticleSenderTypeID
	if senderTypeID == 0 && req.SenderType != "" {
		switch strings.ToLower(req.SenderType) {
		case "agent":
			senderTypeID = 1
		case "system":
			senderTypeID = 2
		case "customer":
			senderTypeID = 3
		}
	}
	if senderTypeID == 0 {
		if isCustomer, _ := c.Get("is_customer"); isCustomer == true {
			senderTypeID = 3 // customer
		} else {
			senderTypeID = 1 // agent
		}
	}

	// Validate sender_type_id exists
	if req.ArticleSenderTypeID != 0 {
		var senderTypeExists bool
		senderTypeQuery := "SELECT EXISTS(SELECT 1 FROM article_sender_type WHERE id = ?)"
		err = db.QueryRow(database.ConvertPlaceholders(senderTypeQuery), req.ArticleSenderTypeID).Scan(&senderTypeExists)
		if err != nil || !senderTypeExists {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid sender type",
			})
			return
		}
	}

	// Validate communication_channel_id exists
	if req.CommunicationChannelID != 0 {
		var channelExists bool
		channelQuery := "SELECT EXISTS(SELECT 1 FROM communication_channel WHERE id = ?)"
		err = db.QueryRow(database.ConvertPlaceholders(channelQuery), req.CommunicationChannelID).Scan(&channelExists)
		if err != nil || !channelExists {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid communication channel",
			})
			return
		}
	}

	// Times
	now := time.Now()
	incomingTime := req.IncomingTime
	if incomingTime == 0 {
		incomingTime = time.Now().Unix()
	}

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to begin transaction",
		})
		return
	}
	defer func() { _ = tx.Rollback() }()

	// Insert article with adapter to support both DBs
	insertArticleQuery := database.ConvertPlaceholders(`
		INSERT INTO article (
			ticket_id,
			article_sender_type_id,
			communication_channel_id,
			is_visible_for_customer,
			create_time,
			create_by,
			change_time,
			change_by
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?
		) RETURNING id`)

	adapter := database.GetAdapter()
	articleID, err := adapter.InsertWithReturningTx(
		tx,
		insertArticleQuery,
		ticketID,
		senderTypeID,
		communicationChannelID,
		isVisibleForCustomer,
		now,
		userID,
		now,
		userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Failed to create article: %v", err),
		})
		return
	}

	// Insert into article_data_mime table
	insertMimeQuery := database.ConvertPlaceholders(`
		INSERT INTO article_data_mime (
			article_id,
			a_from,
			a_to,
			a_cc,
			a_reply_to,
			a_subject,
			a_body,
			a_content_type,
			a_in_reply_to,
			a_references,
			a_message_id,
			incoming_time,
			create_time,
			create_by,
			change_time,
			change_by
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)`)

	_, err = tx.Exec(
		insertMimeQuery,
		articleID,
		req.From,
		req.To,
		req.Cc,
		req.ReplyTo,
		req.Subject,
		req.Body,
		req.ContentType,
		req.InReplyTo,
		req.References,
		req.MessageID,
		int(incomingTime),
		now,
		userID,
		now,
		userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Failed to create article data: %v", err),
		})
		return
	}

	// Add time accounting if time_unit is provided
	if req.TimeUnit > 0 {
		_, err = tx.Exec(database.ConvertPlaceholders(`
			INSERT INTO time_accounting (
				ticket_id,
				article_id,
				time_unit,
				create_time,
				create_by,
				change_time,
				change_by
			) VALUES (?, ?, ?, ?, ?, ?, ?)
		`), ticketID, articleID, req.TimeUnit, now, userID, now, userID)
		if err != nil {
			// Log error but don't fail the whole operation
			fmt.Printf("Warning: Failed to add time accounting: %v\n", err)
		}
	}

	// Update ticket change_time
	// Use left-to-right placeholders so MySQL '?' binding matches arg order
	_, err = tx.Exec(database.ConvertPlaceholders(
		"UPDATE ticket SET change_time = ?, change_by = ? WHERE id = ?",
	), now, userID, ticketID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to update ticket",
		})
		return
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to commit transaction",
		})
		return
	}

	// Queue email notification for new article if visible to customer
	if isVisibleForCustomer == 1 && customerUserID.Valid && customerUserID.String != "" {
		go queueArticleNotificationEmail(db, int(ticketID), articleID, customerUserID.String, userID, req.Body)
	}

	// Fetch the created article for response (join mime data)
	var article struct {
		ID                     int64
		TicketID               int64
		CommunicationChannelID int
		IsVisibleForCustomer   int
		SenderTypeID           int
		From                   *string
		To                     *string
		Cc                     *string
		Subject                *string
		Body                   string
		ContentType            string
		MessageID              *string
		CreateTime             time.Time
		CreateBy               int
	}

	err = db.QueryRow(database.ConvertPlaceholders(`
        SELECT 
            a.id,
            a.ticket_id,
            a.communication_channel_id,
            a.is_visible_for_customer,
            a.article_sender_type_id,
            m.a_from,
            m.a_to,
            m.a_cc,
            m.a_subject,
            m.a_body,
            m.a_content_type,
            m.a_message_id,
            a.create_time,
            a.create_by
        FROM article a
        LEFT JOIN article_data_mime m ON m.article_id = a.id
        WHERE a.id = ?
    `), articleID).Scan(
		&article.ID,
		&article.TicketID,
		&article.CommunicationChannelID,
		&article.IsVisibleForCustomer,
		&article.SenderTypeID,
		&article.From,
		&article.To,
		&article.Cc,
		&article.Subject,
		&article.Body,
		&article.ContentType,
		&article.MessageID,
		&article.CreateTime,
		&article.CreateBy,
	)

	if err != nil {
		// Article was created but we can't fetch it, still return success
		fallbackData := gin.H{
			"id":             articleID,
			"ticket_id":      ticketID,
			"subject":        req.Subject,
			"body":           req.Body,
			"ticket_updated": true,
		}

		c.JSON(http.StatusCreated, gin.H{"success": true, "data": fallbackData})
		return
	}

	// Convert to response format
	responseData := gin.H{
		"id":                       article.ID,
		"ticket_id":                article.TicketID,
		"communication_channel_id": article.CommunicationChannelID,
		"is_visible_for_customer":  article.IsVisibleForCustomer == 1,
		"article_sender_type_id":   article.SenderTypeID,
		"subject":                  article.Subject,
		"body":                     article.Body,
		"content_type":             article.ContentType,
		"create_time":              article.CreateTime,
		"create_by":                article.CreateBy,
		"ticket_updated":           true,
	}

	// Add optional fields
	if article.From != nil {
		responseData["from"] = *article.From
	}
	if article.To != nil {
		responseData["to"] = *article.To
	}
	if article.Cc != nil {
		responseData["cc"] = *article.Cc
	}
	if article.MessageID != nil {
		responseData["message_id"] = *article.MessageID
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": responseData})
}
