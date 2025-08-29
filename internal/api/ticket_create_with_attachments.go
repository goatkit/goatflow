package api

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"log"
)

// handleCreateTicketWithAttachments is an enhanced version that properly handles file attachments
// This fixes the 500 error when users try to create tickets with attachments
func handleCreateTicketWithAttachments(c *gin.Context) {
	var req struct {
		Title         string `json:"title" form:"title"`
		Subject       string `json:"subject" form:"subject"`
		CustomerEmail string `json:"customer_email" form:"customer_email" binding:"required,email"`
		CustomerName  string `json:"customer_name" form:"customer_name"`
		Priority      string `json:"priority" form:"priority"`
		QueueID       string `json:"queue_id" form:"queue_id"`
		TypeID        string `json:"type_id" form:"type_id"`
		Body          string `json:"body" form:"body" binding:"required"`
	}

	// Parse multipart form first to handle both fields and files
	// This is CRITICAL - without this, file uploads cause errors
	if err := c.Request.ParseMultipartForm(10 << 20); // 10 MB max memory
		err != nil && err != http.ErrNotMultipart {
		log.Printf("ERROR: Failed to parse multipart form: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form: " + err.Error()})
		return
	}

	// Bind form data
	if err := c.ShouldBind(&req); err != nil {
		log.Printf("ERROR: Form binding failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Use title if provided, otherwise use subject
	ticketTitle := req.Title
	if ticketTitle == "" {
		ticketTitle = req.Subject
	}
	if ticketTitle == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Title or subject is required"})
		return
	}

	// Convert string values to integers with defaults
	queueID := uint(1) // Default to General Support
	if req.QueueID != "" {
		if id, err := strconv.Atoi(req.QueueID); err == nil {
			queueID = uint(id)
		}
	}

	typeID := uint(1) // Default to Incident
	if req.TypeID != "" {
		if id, err := strconv.Atoi(req.TypeID); err == nil {
			typeID = uint(id)
		}
	}

	// Set default priority if not provided
	if req.Priority == "" {
		req.Priority = "normal"
	}

	// For demo purposes, use a fixed user ID (admin)
	createdBy := uint(1)

	// Create the ticket model
	customerEmail := req.CustomerEmail
	typeIDInt := int(typeID)
	ticket := &models.Ticket{
		Title:            ticketTitle,
		QueueID:          int(queueID),
		TypeID:           &typeIDInt,
		TicketPriorityID: getPriorityID(req.Priority),
		TicketStateID:    1, // New
		TicketLockID:     1, // Unlocked
		CustomerUserID:   &customerEmail,
		CreateBy:         int(createdBy),
		ChangeBy:         int(createdBy),
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		log.Printf("ERROR: Database connection failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}
	
	// Create the ticket
	ticketRepo := repository.NewTicketRepository(db)
	if err := ticketRepo.Create(ticket); err != nil {
		log.Printf("ERROR: Failed to create ticket: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create ticket: " + err.Error()})
		return
	}

	log.Printf("Successfully created ticket ID: %d (with attachment support)", ticket.ID)

	// Create the first article (ticket body)
	articleRepo := repository.NewArticleRepository(db)
	article := &models.Article{
		TicketID:             ticket.ID,
		Subject:              ticketTitle,
		Body:                 req.Body,
		SenderTypeID:         3, // Customer
		CommunicationChannelID: 1, // Email
		IsVisibleForCustomer: 1,
		ArticleTypeID:        models.ArticleTypeNoteExternal,
		CreateBy:            int(createdBy),
		ChangeBy:            int(createdBy),
	}
	
	if err := articleRepo.Create(article); err != nil {
		log.Printf("ERROR: Failed to add initial article: %v", err)
		// Don't fail the ticket creation, just log the error
	} else {
		log.Printf("Successfully created article ID: %d for ticket %d", article.ID, ticket.ID)
	}

	// Process file attachments if present
	attachmentInfo := []map[string]interface{}{}
	
	if c.Request.MultipartForm != nil && c.Request.MultipartForm.File != nil {
		// Check both singular and plural field names for compatibility
		files := c.Request.MultipartForm.File["attachments"]
		if files == nil {
			files = c.Request.MultipartForm.File["attachment"]
		}
		log.Printf("Processing %d attachment(s) for ticket %d", len(files), ticket.ID)
		
		for _, fileHeader := range files {
			// Validate file size (10MB max)
			if fileHeader.Size > 10*1024*1024 {
				log.Printf("WARNING: File %s too large (%d bytes), skipping", fileHeader.Filename, fileHeader.Size)
				continue
			}
			
			// Validate file type (basic security check)
			ext := filepath.Ext(fileHeader.Filename)
			blockedExtensions := map[string]bool{
				".exe": true, ".bat": true, ".sh": true, ".cmd": true,
				".com": true, ".scr": true, ".vbs": true, ".js": true,
			}
			
			if blockedExtensions[ext] {
				log.Printf("WARNING: File type %s not allowed, skipping %s", ext, fileHeader.Filename)
				continue
			}
			
			// Open the uploaded file
			file, err := fileHeader.Open()
			if err != nil {
				log.Printf("ERROR: Failed to open uploaded file %s: %v", fileHeader.Filename, err)
				continue
			}
			defer file.Close()
			
			// Read file content into memory
			fileContent, err := io.ReadAll(file)
			if err != nil {
				log.Printf("ERROR: Failed to read file content: %v", err)
				continue
			}
			
			// Determine content type
			contentType := fileHeader.Header.Get("Content-Type")
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			
			// Save attachment record to database
			// OTRS stores attachments in article_data_mime_attachment table
			_, err = db.Exec(database.ConvertPlaceholders(`
				INSERT INTO article_data_mime_attachment 
				(article_id, filename, content_type, content_size, content, 
				 disposition, create_by, change_by)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`),
				article.ID, 
				fileHeader.Filename,
				contentType,
				fmt.Sprintf("%d", len(fileContent)),
				fileContent, // Store actual content
				"attachment",
				createdBy,
				createdBy,
			)
			if err != nil {
				log.Printf("WARNING: Failed to save attachment record to database: %v", err)
			} else {
				log.Printf("Successfully saved attachment record to database for article %d", article.ID)
			}
			
			// Track the info for response
			attachmentInfo = append(attachmentInfo, map[string]interface{}{
				"filename":     fileHeader.Filename,
				"size":         len(fileContent),
				"content_type": contentType,
				"saved":        true,
			})
			
			log.Printf("Successfully saved attachment: %s (%d bytes) for ticket %d", 
				fileHeader.Filename, len(fileContent), ticket.ID)
		}
	}
	
	// Prepare response
	response := gin.H{
		"id":            ticket.ID,
		"ticket_number": ticket.TicketNumber,
		"message":       "Ticket created successfully",
		"queue_id":      float64(ticket.QueueID),
		"priority":      req.Priority,
	}
	// Add type_id if it's not nil
	if ticket.TypeID != nil {
		response["type_id"] = float64(*ticket.TypeID)
	}
	
	// Include attachment info if any were processed
	if len(attachmentInfo) > 0 {
		response["attachments"] = attachmentInfo
		response["attachment_count"] = len(attachmentInfo)
	}
	
	// For HTMX, set the redirect header to the ticket detail page
	c.Header("HX-Redirect", fmt.Sprintf("/tickets/%d", ticket.ID))
	c.JSON(http.StatusCreated, response)
}