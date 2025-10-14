package api

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
    "path/filepath"
    "time"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/constants"
	"github.com/gotrs-io/gotrs-ce/internal/core"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
    "github.com/gotrs-io/gotrs-ce/internal/service"
    "github.com/gotrs-io/gotrs-ce/internal/config"
)

// HandleAgentCreateTicket creates a new ticket from the agent interface
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
		stateID := c.PostForm("state_id")
		customerUserID := c.PostForm("customer_user_id")
		customerEmail := c.PostForm("customer_email")
		// customerName := c.PostForm("customer_name")
		customerID := c.PostForm("customer_id")
		// Optional time accounting (minutes)
		timeUnitsStr := strings.TrimSpace(c.PostForm("time_units"))
		if timeUnitsStr == "" { timeUnitsStr = strings.TrimSpace(c.PostForm("timeUnits")) }
		timeUnits := 0
		if timeUnitsStr != "" { if n, err := strconv.Atoi(timeUnitsStr); err == nil && n > 0 { timeUnits = n } }

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
			err := db.QueryRow(database.ConvertPlaceholders(`
				SELECT customer_id
				FROM customer_user
				WHERE login = $1 AND valid_id = 1
			`), customerUserID).Scan(&foundCustomerID)

			if err == nil {
				customerUserIDValue = sql.NullString{String: customerUserID, Valid: true}
				if foundCustomerID.Valid {
					customerIDValue = foundCustomerID
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
				WHERE email = $1 AND valid_id = 1
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
		if typeIDInt != 0 { typePtr = &typeIDInt }
		var custIDPtr *string
		if customerIDValue.Valid { v := customerIDValue.String; custIDPtr = &v }
		var custUserPtr *string
		if customerUserIDValue.Valid { v := customerUserIDValue.String; custUserPtr = &v }
		var userIDInt = int(userID)
		ticketModel := &models.Ticket{
			Title:            title,
			QueueID:          queueIDInt,
			TicketLockID:     1,
			TypeID:           typePtr,
			UserID:           &userIDInt,
			ResponsibleUserID:&userIDInt,
			TicketPriorityID: priorityIDInt,
			TicketStateID:    stateIDInt,
			CustomerID:       custIDPtr,
			CustomerUserID:   custUserPtr,
			CreateBy:         userIDInt,
			ChangeBy:         userIDInt,
		}
		tRepo := repository.NewTicketRepository(db)
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
			if verr := core.ValidateArticleCombination(resolved.ArticleTypeID, resolved.ArticleSenderTypeID, resolved.CustomerVisible); verr != nil {
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
		if c.Request.MultipartForm != nil && c.Request.MultipartForm.File != nil && articleModel != nil && articleModel.ID > 0 {
			files := c.Request.MultipartForm.File["attachments"]
			if files == nil { files = c.Request.MultipartForm.File["attachment"] }
			if files == nil { files = c.Request.MultipartForm.File["file"] }

			// Limits and type allowlist from config
			var maxSize int64 = 10 * 1024 * 1024
			allowed := map[string]struct{}{}
			if cfg := config.Get(); cfg != nil {
				if cfg.Storage.Attachments.MaxSize > 0 { maxSize = cfg.Storage.Attachments.MaxSize }
				for _, t := range cfg.Storage.Attachments.AllowedTypes { allowed[strings.ToLower(t)] = struct{}{} }
			}

			// Blocked extensions
			blocked := map[string]bool{ ".exe":true, ".bat":true, ".cmd":true, ".sh":true, ".vbs":true, ".js":true, ".com":true, ".scr":true }

			storageSvc := GetStorageService()
			for _, fh := range files {
				if fh == nil { continue }
				if fh.Size > maxSize { log.Printf("attachment too large: %s", fh.Filename); continue }
				if blocked[strings.ToLower(filepath.Ext(fh.Filename))] { log.Printf("blocked file type: %s", fh.Filename); continue }
				f, err := fh.Open(); if err != nil { log.Printf("open attachment failed: %v", err); continue }
				func() {
					defer f.Close()
					// Determine content type with fallback
					contentType := fh.Header.Get("Content-Type")
					if contentType == "" || contentType == "application/octet-stream" {
						buf := make([]byte, 512)
						if n, _ := f.Read(buf); n > 0 { contentType = detectContentType(fh.Filename, buf[:n]) }
						f.Seek(0, 0)
					}
					if len(allowed) > 0 && contentType != "" && contentType != "application/octet-stream" {
						if _, ok := allowed[strings.ToLower(contentType)]; !ok { log.Printf("type not allowed: %s %s", fh.Filename, contentType); return }
					}

					// Store via service; attach article_id and user_id in context
					ctx := c.Request.Context()
					ctx = service.WithUserID(ctx, int(userID))
					ctx = service.WithArticleID(ctx, articleModel.ID)
					storagePath := service.GenerateOTRSStoragePath(ticketModel.ID, articleModel.ID, fh.Filename)
					if _, err := storageSvc.Store(ctx, f, fh, storagePath); err != nil {
						log.Printf("storage Store failed: %v", err)
						return
					}
					// For local FS backend, also insert attachment row so listing/download works
					if _, isDB := storageSvc.(*service.DatabaseStorageService); !isDB {
						if f2, e2 := fh.Open(); e2 == nil {
							defer f2.Close()
							bytes, rerr := io.ReadAll(f2); if rerr == nil {
								ct := contentType; if ct == "" { ct = "application/octet-stream" }
								_, ierr := db.Exec(database.ConvertPlaceholders(`
									INSERT INTO article_data_mime_attachment (
										article_id, filename, content_type, content_size, content,
										disposition, create_time, create_by, change_time, change_by
									) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`),
									articleModel.ID,
									fh.Filename,
									ct,
									int64(len(bytes)),
									bytes,
									"attachment",
									time.Now(), int(userID), time.Now(), int(userID),
								)
								if ierr != nil { log.Printf("attachment metadata insert failed: %v", ierr) }
							}
						}
					}
				}()
			}
			c.Header("HX-Trigger", "attachments-updated")
		}

		// Persist initial time accounting if provided
		if timeUnits > 0 {
			articleID := int(articleModel.ID)
			if err := saveTimeEntry(db, ticketID, &articleID, timeUnits, int(userID)); err != nil {
				log.Printf("WARNING: Failed to save initial time entry for ticket %d: %v", ticketID, err)
			} else {
				log.Printf("Saved initial time entry for ticket %d: %d minutes", ticketID, timeUnits)
			}
		}

		// Redirect to ticket view using repository-assigned ticket number
		c.Redirect(http.StatusSeeOther, fmt.Sprintf("/tickets/%s", ticketModel.TicketNumber))
	}
}

// detectTicketContentType determines the MIME type based on content analysis
func detectTicketContentType(content string) string {
	// Check for HTML tags
	if strings.Contains(content, "<") && strings.Contains(content, ">") {
		// Look for common HTML tags
		htmlTags := []string{"<p>", "<br", "<div>", "<span>", "<strong>", "<em>", "<b>", "<i>", "<h1>", "<h2>", "<h3>", "<ul>", "<ol>", "<li>"}
		for _, tag := range htmlTags {
			if strings.Contains(content, tag) {
				return "text/html"
			}
		}
	}
	
	// Check for markdown syntax
	if strings.Contains(content, "#") || strings.Contains(content, "**") || strings.Contains(content, "*") || strings.Contains(content, "`") {
		return "text/markdown"
	}
	
	// Default to plain text
	return "text/plain"
}
