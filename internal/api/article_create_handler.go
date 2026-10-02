package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/core"
	"github.com/goatkit/goatflow/internal/platform/constants"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/services"
	"github.com/goatkit/goatflow/internal/storage"
)

// HandleCreateArticleAPI handles POST /api/v1/tickets/:ticket_id/articles.
//
//	@Summary		Create article
//	@Description	Add a new article (reply/note) to a ticket
//	@Tags			Articles
//	@Accept			json
//	@Produce		json
//	@Param			ticket_id	path		int		true	"Ticket ID"
//	@Param			article		body		object	true	"Article data (body, subject, article_type, sender_type, etc.)"
//	@Success		201			{object}	map[string]interface{}	"Created article"
//	@Failure		400			{object}	map[string]interface{}	"Invalid request"
//	@Failure		401			{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404			{object}	map[string]interface{}	"Ticket not found"
//	@Security		BearerAuth
//	@Router			/tickets/{ticket_id}/articles [post]
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
	userID, ok := auditUserID(c)
	if !ok {
		return
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
	// Check both is_customer flag and user_role (API tokens set user_role, not is_customer)
	isCustomer := false
	if ic, _ := c.Get("is_customer"); ic == true {
		isCustomer = true
	} else if role, _ := c.Get("user_role"); role == "Customer" {
		isCustomer = true
	}

	if isCustomer {
		customerEmail, _ := c.Get("customer_email") //nolint:errcheck // Defaults to nil
		customerLogin, _ := c.Get("customer_login") //nolint:errcheck // Defaults to nil
		// Check email or login matches
		emailStr, _ := customerEmail.(string)
		loginStr, _ := customerLogin.(string)
		if emailStr != "" {
			if !customerUserID.Valid || customerUserID.String != emailStr {
				c.JSON(http.StatusForbidden, gin.H{
					"success": false,
					"error":   "Access denied",
				})
				return
			}
		} else if loginStr != "" {
			if !customerUserID.Valid || customerUserID.String != loginStr {
				c.JSON(http.StatusForbidden, gin.H{
					"success": false,
					"error":   "Access denied",
				})
				return
			}
		}
		// Customer users use system user for create_by (FK to users table)
		userID = 1
	} else {
		// Agent users need 'note' or 'rw' permission on the ticket's queue
		permSvc := services.NewPermissionService(db)
		canNote, err := permSvc.CanAddNote(userID, ticketID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to check permissions"})
			return
		}
		// Security: return 404 to avoid revealing ticket existence
		if !canNote {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Ticket not found"})
			return
		}
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

	if req.From == "" {
		req.From = req.FromEmail
	}
	if req.To == "" {
		req.To = req.ToEmail
	}

	// Sender type: customers always write as customer; agents may pick one by
	// id or name and default to agent.
	senderTypeID := req.ArticleSenderTypeID
	if senderTypeID == 0 && req.SenderType != "" {
		switch strings.ToLower(req.SenderType) {
		case "agent":
			senderTypeID = constants.ArticleSenderAgent
		case "system":
			senderTypeID = constants.ArticleSenderSystem
		case "customer":
			senderTypeID = constants.ArticleSenderCustomer
		}
	}
	if isCustomer {
		senderTypeID = constants.ArticleSenderCustomer
	} else if senderTypeID == 0 {
		senderTypeID = constants.ArticleSenderAgent
	}

	if req.ArticleSenderTypeID != 0 && !isCustomer {
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

	// Article type → channel + customer visibility (internal/core/channel_mapping.go).
	// Without an explicit type, a requested visibility picks external/internal note.
	var visiblePtr *bool
	if req.IsVisible != nil {
		visiblePtr = req.IsVisible
	} else if req.IsVisibleToCustomer != nil {
		visiblePtr = req.IsVisibleToCustomer
	} else if req.IsVisibleForCustomer != nil {
		visiblePtr = req.IsVisibleForCustomer
	}
	intent := core.ArticleIntent{SenderTypeID: senderTypeID, ForceVisible: visiblePtr}
	if strings.TrimSpace(req.ArticleType) != "" {
		typeID, ok := core.ArticleTypeByName(req.ArticleType)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid article type"})
			return
		}
		intent.ExplicitArticleTypeID = typeID
	} else if visiblePtr != nil {
		intent.ExplicitArticleTypeID = constants.ArticleTypeNoteInternal
		if *visiblePtr {
			intent.ExplicitArticleTypeID = constants.ArticleTypeNoteExternal
		}
	}
	resolved, err := core.DetermineArticleType(intent)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Article type not allowed for this sender"})
		return
	}
	isVisibleForCustomer := 0
	if resolved.CustomerVisible {
		isVisibleForCustomer = 1
	}

	// Channel derived from the article type unless explicitly overridden.
	communicationChannelID := req.CommunicationChannelID
	if communicationChannelID == 0 {
		communicationChannelID = core.MapCommunicationChannel(resolved.ArticleTypeID)
	} else {
		var channelExists bool
		channelQuery := "SELECT EXISTS(SELECT 1 FROM communication_channel WHERE id = ?)"
		err = db.QueryRow(database.ConvertPlaceholders(channelQuery), communicationChannelID).Scan(&channelExists)
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
			content_path,
			incoming_time,
			create_time,
			create_by,
			change_time,
			change_by
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
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
		storage.ContentPath(now),
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
			"id":                       articleID,
			"ticket_id":                ticketID,
			"subject":                  req.Subject,
			"body":                     req.Body,
			"content_type":             req.ContentType,
			"article_sender_type_id":   senderTypeID,
			"communication_channel_id": communicationChannelID,
			"is_visible_for_customer":  isVisibleForCustomer == 1,
			"article_type":             core.ArticleTypeName(core.ArticleTypeFromStorage(communicationChannelID, isVisibleForCustomer == 1)),
			"create_by":                userID,
			"ticket_updated":           true,
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
		"article_type":             core.ArticleTypeName(core.ArticleTypeFromStorage(article.CommunicationChannelID, article.IsVisibleForCustomer == 1)),
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
