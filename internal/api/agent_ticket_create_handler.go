package api

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/config"
	"github.com/gotrs-io/gotrs-ce/internal/constants"
	"github.com/gotrs-io/gotrs-ce/internal/core"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/mailqueue"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/notifications"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/utils"
)

// HandleAgentCreateTicket creates a new ticket from the agent interface.
func HandleAgentCreateTicket(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Ensure multipart form is parsed (for attachments)
		if err := c.Request.ParseMultipartForm(10 << 20); err != nil && err != http.ErrNotMultipart {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form"})
			return
		}
		// Get agent user info from context
		userID := c.GetUint("user_id")
		if userID == 0 {
			// Fallback for now if auth middleware not applied (development/testing)
			userID = 1
		}
		// username := c.GetString("username")

		// Get form data
		title := c.PostForm("subject") // Agent form uses 'subject' field name
		message := c.PostForm("body")
		queueID := c.PostForm("queue_id")
		priorityID := c.PostForm("priority")
		typeID := c.PostForm("type_id")
		serviceID := c.PostForm("service_id")
		stateID := strings.TrimSpace(c.PostForm("next_state_id"))
		nextStateName := strings.TrimSpace(c.PostForm("next_state"))
		if stateID == "" {
			stateID = strings.TrimSpace(c.PostForm("state_id"))
		}
		pendingUntil := strings.TrimSpace(c.PostForm("pending_until"))
		customerUserID := c.PostForm("customer_user_id")
		customerEmail := c.PostForm("customer_email")
		// customerName := c.PostForm("customer_name")
		customerID := c.PostForm("customer_id")
		// Optional time accounting (minutes)
		timeUnitsStr := strings.TrimSpace(c.PostForm("time_units"))
		if timeUnitsStr == "" {
			timeUnitsStr = strings.TrimSpace(c.PostForm("timeUnits"))
		}
		timeUnits := 0
		if timeUnitsStr != "" {
			if n, err := strconv.Atoi(timeUnitsStr); err == nil && n > 0 {
				timeUnits = n
			}
		}

		// Validate required fields
		if title == "" || message == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Subject and message are required"})
			return
		}
		// Customer selection is optional: allow creating tickets without a customer user/email/id
		// If all customer fields are empty, we'll insert NULLs for customer_id and customer_user_id

		// Set defaults for agent-created tickets
		if queueID == "" {
			queueID = "1" // Default queue
		}
		if priorityID == "" {
			priorityID = "3" // Normal priority
		}
		if typeID == "" {
			typeID = "1" // Default type
		}
		if stateID == "" {
			stateID = "1" // New state
		}

		// Map textual priority codes (form values) to numeric IDs
		switch priorityID {
		case "very_low":
			priorityID = "1"
		case "low":
			priorityID = "2"
		case "normal":
			priorityID = "3"
		case "high":
			priorityID = "4"
		case "very_high":
			priorityID = "5"
		}

		// Manual TN generation removed; repository handles ticket number based on configured generator

		// Get database connection
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database connection failed"})
			return
		}

		// Parse numeric IDs
		queueIDInt, err := strconv.Atoi(queueID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid queue ID"})
			return
		}
		priorityIDInt, err := strconv.Atoi(priorityID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid priority ID"})
			return
		}
		typeIDInt, err := strconv.Atoi(typeID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid type ID"})
			return
		}
		stateIDInt, err := strconv.Atoi(stateID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid state ID"})
			return
		}

		tRepo := repository.NewTicketRepository(db)

		resolvedStateID := stateIDInt
		var resolvedState *models.TicketState
		if id, st, rerr := resolveTicketState(tRepo, nextStateName, stateIDInt); rerr != nil {
			log.Printf("HandleAgentCreateTicket: state resolution failed: %v", rerr)
			if id > 0 {
				resolvedStateID = id
				resolvedState = st
			}
		} else if id > 0 {
			resolvedStateID = id
			resolvedState = st
		}
		if resolvedState == nil && resolvedStateID > 0 {
			st, lerr := loadTicketState(tRepo, resolvedStateID)
			if lerr != nil {
				log.Printf("HandleAgentCreateTicket: load state %d failed: %v", resolvedStateID, lerr)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load ticket state"})
				return
			}
			if st == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid state selection"})
				return
			}
			resolvedState = st
		}
		stateIDInt = resolvedStateID

		pendingUnix := 0
		if pendingUntil != "" {
			pendingUnix = parsePendingUntil(pendingUntil)
			if pendingUnix <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid pending until value"})
				return
			}
		}
		if isPendingState(resolvedState) {
			if pendingUnix <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "pending_until is required for pending states"})
				return
			}
		} else {
			pendingUnix = 0
		}

		// Handle customer assignment
		var customerIDValue sql.NullString
		var customerUserIDValue sql.NullString

		if customerID != "" {
			customerIDValue = sql.NullString{String: customerID, Valid: true}
		}
		// Handle customer user selection
		if customerUserID != "" {
			// Get customer info from the selected customer user
			var foundCustomerID sql.NullString
			var foundEmail sql.NullString
			err := db.QueryRow(database.ConvertPlaceholders(`
				SELECT customer_id, email
				FROM customer_user
				WHERE login = ? AND valid_id = 1
			`), customerUserID).Scan(&foundCustomerID, &foundEmail)

			if err == nil {
				customerUserIDValue = sql.NullString{String: customerUserID, Valid: true}
				if foundCustomerID.Valid {
					customerIDValue = foundCustomerID
				}
				// Use the email from the customer user record for notifications
				if foundEmail.Valid && foundEmail.String != "" {
					customerEmail = foundEmail.String
					log.Printf("HandleAgentCreateTicket: found customer email '%s' for user '%s'", customerEmail, customerUserID)
				}
			} else {
				log.Printf("Error finding customer user %s: %v", customerUserID, err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer user selected"})
				return
			}
		}
		// Fallback to email lookup if customer_user_id is not provided (for backward compatibility)
		if customerEmail != "" && !customerUserIDValue.Valid {
			// Try to find existing customer user by email
			var foundCustomerID, foundCustomerUserID sql.NullString
			err := db.QueryRow(database.ConvertPlaceholders(`
				SELECT customer_id, login
				FROM customer_user
				WHERE email = ? AND valid_id = 1
			`), customerEmail).Scan(&foundCustomerID, &foundCustomerUserID)

			if err == nil && foundCustomerUserID.Valid {
				// Found existing customer user
				customerUserIDValue = foundCustomerUserID
				customerIDValue = foundCustomerID
			} else {
				// Create new customer user - use email as login for now
				customerUserIDValue = sql.NullString{String: customerEmail, Valid: true}
			}
		}

		// Build ticket model and use repository (central generator + logging)
		var typePtr *int
		if typeIDInt != 0 {
			typePtr = &typeIDInt
		}
		var custIDPtr *string
		if customerIDValue.Valid {
			v := customerIDValue.String
			custIDPtr = &v
		}
		var custUserPtr *string
		if customerUserIDValue.Valid {
			v := customerUserIDValue.String
			custUserPtr = &v
		}
		var serviceIDPtr *int
		if serviceID != "" {
			if sid, serr := strconv.Atoi(serviceID); serr == nil && sid > 0 {
				serviceIDPtr = &sid
			}
		}
		var userIDInt = int(userID)
		ticketModel := &models.Ticket{
			Title:             title,
			QueueID:           queueIDInt,
			TicketLockID:      1,
			TypeID:            typePtr,
			ServiceID:         serviceIDPtr,
			UserID:            &userIDInt,
			ResponsibleUserID: &userIDInt,
			TicketPriorityID:  priorityIDInt,
			TicketStateID:     stateIDInt,
			CustomerID:        custIDPtr,
			CustomerUserID:    custUserPtr,
			CreateBy:          userIDInt,
			ChangeBy:          userIDInt,
		}
		if pendingUnix > 0 {
			ticketModel.UntilTime = pendingUnix
		}
		if err := tRepo.Create(ticketModel); err != nil {
			log.Printf("Error creating ticket via repository: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create ticket"})
			return
		}
		ticketID := ticketModel.ID

		// Determine interaction / article type
		interaction := c.PostForm("interaction_type")
		// Resolve article type + visibility
		var articleModel *models.Article
		{
			articleRepo := repository.NewArticleRepository(db)
			intent := core.ArticleIntent{Interaction: constants.InteractionType(interaction), SenderTypeID: constants.ArticleSenderAgent}
			resolved, derr := core.DetermineArticleType(intent)
			if derr != nil {
				log.Printf("Article type resolution failed: %v", derr)
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid interaction type"})
				return
			}
			verr := core.ValidateArticleCombination(
				resolved.ArticleTypeID, resolved.ArticleSenderTypeID, resolved.CustomerVisible)
			if verr != nil {
				log.Printf("Article combination invalid: %v", verr)
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid article combination"})
				return
			}
			visibility := 0
			if resolved.CustomerVisible {
				visibility = 1
			}
			articleModel = &models.Article{
				TicketID:               ticketID,
				ArticleTypeID:          resolved.ArticleTypeID,
				SenderTypeID:           resolved.ArticleSenderTypeID,
				CommunicationChannelID: core.MapCommunicationChannel(resolved.ArticleTypeID),
				IsVisibleForCustomer:   visibility,
				Subject:                title,
				Body:                   message,
				MimeType:               detectTicketContentType(message),
				Charset:                "utf-8",
				CreateBy:               int(userID),
				ChangeBy:               int(userID),
			}
			if err := articleRepo.Create(articleModel); err != nil {
				log.Printf("Error creating initial article via repository: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create initial article"})
				return
			}
		}

		// Process attachments from the new ticket form using unified storage
		if c.Request.MultipartForm != nil && articleModel != nil && articleModel.ID > 0 {
			files := getFormFiles(c.Request.MultipartForm)
			if len(files) > 0 {
				processFormAttachments(files, attachmentProcessParams{
					ctx:       c.Request.Context(),
					db:        db,
					ticketID:  ticketModel.ID,
					articleID: articleModel.ID,
					userID:    int(userID),
				})
				c.Header("HX-Trigger", "attachments-updated")
			}
		}

		// Persist initial time accounting if provided
		if timeUnits > 0 {
			articleID := articleModel.ID
			if err := saveTimeEntry(db, ticketID, &articleID, timeUnits, int(userID)); err != nil {
				log.Printf("WARNING: Failed to save initial time entry for ticket %d: %v", ticketID, err)
			} else {
				log.Printf("Saved initial time entry for ticket %d: %d minutes", ticketID, timeUnits)
			}
		}

		// Redirect to ticket view using repository-assigned ticket number
		c.Redirect(http.StatusSeeOther, fmt.Sprintf("/tickets/%s", ticketModel.TicketNumber))

		// Queue email notification if customer email is available
		if customerEmail != "" {
			log.Printf("DEBUG: Agent ticket created, queuing email to customer='%s', ticket='%s'",
				customerEmail, ticketModel.TicketNumber)
			go func() {
				subject := fmt.Sprintf("Ticket Created: %s", ticketModel.TicketNumber)
				body := fmt.Sprintf("Your ticket has been created successfully.\n\n"+
					"Ticket Number: %s\nTitle: %s\n\nMessage:\n%s\n\n"+
					"You can view your ticket at: /tickets/%s\n\nBest regards,\nGOTRS Support Team",
					ticketModel.TicketNumber, ticketModel.Title, message, ticketModel.TicketNumber)

				// Queue the email for processing by EmailQueueTask
				db, dbErr := database.GetDB()
				if dbErr != nil {
					log.Printf("Failed to get database connection for queuing email: %v", dbErr)
					return
				}

				queueRepo := mailqueue.NewMailQueueRepository(db)
				var emailCfg *config.EmailConfig
				if cfg := config.Get(); cfg != nil {
					emailCfg = &cfg.Email
				}
				renderCtx := notifications.BuildRenderContext(context.Background(), db, customerUserIDValue.String, int(userID))
				branding, brandErr := notifications.PrepareQueueEmail(
					context.Background(),
					db,
					ticketModel.QueueID,
					body,
					utils.IsHTML(body),
					emailCfg,
					renderCtx,
				)
				if brandErr != nil {
					log.Printf("Queue identity lookup failed for ticket %d: %v", ticketModel.ID, brandErr)
				}
				senderEmail := branding.EnvelopeFrom
				queueItem := &mailqueue.MailQueueItem{
					Sender:     &senderEmail,
					Recipient:  customerEmail,
					RawMessage: mailqueue.BuildEmailMessage(branding.HeaderFrom, customerEmail, subject, branding.Body),
					Attempts:   0,
					CreateTime: time.Now(),
				}

				if queueErr := queueRepo.Insert(context.Background(), queueItem); queueErr != nil {
					log.Printf("Failed to queue email for %s: %v", customerEmail, queueErr)
				} else {
					log.Printf("Queued email for %s (ticket %s) for processing", customerEmail, ticketModel.TicketNumber)
				}
			}()
		}
	}
}

// detectTicketContentType determines the MIME type based on content analysis.
func detectTicketContentType(content string) string {
	// Check for HTML tags
	if strings.Contains(content, "<") && strings.Contains(content, ">") {
		// Look for common HTML tags
		htmlTags := []string{
			"<p>", "<br", "<div>", "<span>", "<strong>", "<em>",
			"<b>", "<i>", "<h1>", "<h2>", "<h3>", "<ul>", "<ol>", "<li>",
		}
		for _, tag := range htmlTags {
			if strings.Contains(content, tag) {
				return "text/html"
			}
		}
	}

	// Check for markdown syntax
	hasMarkdown := strings.Contains(content, "#") || strings.Contains(content, "**") ||
		strings.Contains(content, "*") || strings.Contains(content, "`")
	if hasMarkdown {
		return "text/markdown"
	}

	// Default to plain text
	return "text/plain"
}
