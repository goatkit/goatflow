package api

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/config"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/service"
)

// handleCreateTicketWithAttachments is an enhanced version that properly handles file attachments
// This fixes the 500 error when users try to create tickets with attachments
func handleCreateTicketWithAttachments(c *gin.Context) {
	var req struct {
		Title          string `json:"title" form:"title"`
		Subject        string `json:"subject" form:"subject"`
		CustomerEmail  string `json:"customer_email" form:"customer_email" binding:"omitempty,email"`
		CustomerUserID string `json:"customer_user_id" form:"customer_user_id"`
		CustomerName   string `json:"customer_name" form:"customer_name"`
		Priority       string `json:"priority" form:"priority"`
		QueueID        string `json:"queue_id" form:"queue_id"`
		TypeID         string `json:"type_id" form:"type_id"`
		Body           string `json:"body" form:"body" binding:"required"`
		NextState      string `json:"next_state" form:"next_state"`
		NextStateID    string `json:"next_state_id" form:"next_state_id"`
		PendingUntil   string `json:"pending_until" form:"pending_until"`
		// Optional time accounting minutes provided on the new ticket form
		TimeUnits string `json:"time_units" form:"time_units"`
	}

	// Parse multipart form first to handle both fields and files
	if err := c.Request.ParseMultipartForm(10 << 20); err != nil && err != http.ErrNotMultipart {
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

	// Fallback: if customer_email missing but customer_user_id present (autocomplete sets this), treat it as email login
	if strings.TrimSpace(req.CustomerEmail) == "" {
		candidate := strings.TrimSpace(req.CustomerUserID)
		if candidate == "" {
			candidate = strings.TrimSpace(c.PostForm("customer_user_id"))
		}
		if candidate != "" && strings.Contains(candidate, "@") {
			req.CustomerEmail = candidate
			log.Printf("CreateTicketWithAttachments: filled CustomerEmail from customer_user_id='%s'", candidate)
		}
	}

	// Enforce: we must have a customer email by now
	if strings.TrimSpace(req.CustomerEmail) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "customer_email or customer_user_id (email) is required"})
		return
	}

	// Debug: log raw time_units from form/json and possible camelCase field
	rawTime := strings.TrimSpace(req.TimeUnits)
	if rawTime == "" {
		// Accept camelCase "timeUnits" as a fallback
		rawTime = strings.TrimSpace(c.PostForm("timeUnits"))
		if rawTime != "" {
			req.TimeUnits = rawTime
			log.Printf("CreateTicketWithAttachments: picked timeUnits (camelCase)='%s'", rawTime)
		}
	}
	log.Printf("CreateTicketWithAttachments: raw time_units='%s'", req.TimeUnits)

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
	queueID := uint(1)
	if req.QueueID != "" {
		if id, err := strconv.Atoi(req.QueueID); err == nil {
			queueID = uint(id)
		}
	}

	typeID := uint(1)
	if req.TypeID != "" {
		if id, err := strconv.Atoi(req.TypeID); err == nil {
			typeID = uint(id)
		}
	}

	if req.Priority == "" {
		req.Priority = "normal"
	}

	minutes := 0
	if v := strings.TrimSpace(req.TimeUnits); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			minutes = n
		}
	}
	log.Printf("CreateTicketWithAttachments: parsed minutes=%d", minutes)

	createdBy := uint(1)

	db, err := database.GetDB()
	if err != nil {
		log.Printf("ERROR: Database connection failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	ticketRepo := repository.NewTicketRepository(db)

	nextStateID := 0
	if v := strings.TrimSpace(req.NextStateID); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			nextStateID = parsed
		}
	}

	stateID := getStateID("new")
	var resolvedState *models.TicketState
	if id, st, err := resolveTicketState(ticketRepo, req.NextState, nextStateID); err != nil {
		log.Printf("CreateTicketWithAttachments: state resolution failed: %v", err)
		if id > 0 {
			stateID = id
			resolvedState = st
		}
	} else if id > 0 {
		stateID = id
		resolvedState = st
	}
	if resolvedState == nil && stateID > 0 {
		if st, err := loadTicketState(ticketRepo, stateID); err != nil {
			log.Printf("CreateTicketWithAttachments: load state %d failed: %v", stateID, err)
		} else {
			resolvedState = st
		}
	}

	pendingUnix := parsePendingUntil(req.PendingUntil)
	if isPendingState(resolvedState) && pendingUnix <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pending_until is required for pending states"})
		return
	}

	customerEmail := req.CustomerEmail
	typeIDInt := int(typeID)
	ticket := &models.Ticket{
		Title:            ticketTitle,
		QueueID:          int(queueID),
		TypeID:           &typeIDInt,
		TicketPriorityID: getPriorityID(req.Priority),
		TicketStateID:    stateID,
		TicketLockID:     1,
		CustomerUserID:   &customerEmail,
		CreateBy:         int(createdBy),
		ChangeBy:         int(createdBy),
	}
	ownerID := int(createdBy)
	ticket.UserID = &ownerID
	ticket.ResponsibleUserID = &ownerID
	if pendingUnix > 0 && isPendingState(resolvedState) {
		ticket.UntilTime = pendingUnix
	}

	if err := ticketRepo.Create(ticket); err != nil {
		log.Printf("ERROR: Failed to create ticket: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create ticket: " + err.Error()})
		return
	}

	log.Printf("Successfully created ticket ID: %d (with attachment support)", ticket.ID)

	// Create the first article (ticket body)
	articleRepo := repository.NewArticleRepository(db)
	article := &models.Article{
		TicketID:               ticket.ID,
		Subject:                ticketTitle,
		Body:                   req.Body,
		SenderTypeID:           3, // Customer
		CommunicationChannelID: 1, // Email
		IsVisibleForCustomer:   1,
		ArticleTypeID:          models.ArticleTypeNoteExternal,
		CreateBy:               int(createdBy),
		ChangeBy:               int(createdBy),
	}

	if err := articleRepo.Create(article); err != nil {
		log.Printf("ERROR: Failed to add initial article: %v", err)
		// Without an article we cannot safely attach files; continue ticket creation
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
		if files == nil {
			files = c.Request.MultipartForm.File["file"]
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
			// Ensure we close after processing this iteration
			func() {
				defer file.Close()

				// Determine content type (fallback using simple detection)
				contentType := fileHeader.Header.Get("Content-Type")
				if contentType == "" || contentType == "application/octet-stream" {
					buf := make([]byte, 512)
					if n, _ := file.Read(buf); n > 0 {
						contentType = detectContentType(fileHeader.Filename, buf[:n])
					}
					file.Seek(0, 0)
				}

				// Enforce config limits/types if set
				if cfg := config.Get(); cfg != nil {
					max := cfg.Storage.Attachments.MaxSize
					if max > 0 && fileHeader.Size > max {
						log.Printf("WARNING: %s exceeds max size, skipping", fileHeader.Filename)
						return
					}
					if len(cfg.Storage.Attachments.AllowedTypes) > 0 && contentType != "" && contentType != "application/octet-stream" {
						allowed := map[string]struct{}{}
						for _, t := range cfg.Storage.Attachments.AllowedTypes {
							allowed[strings.ToLower(t)] = struct{}{}
						}
						if _, ok := allowed[strings.ToLower(contentType)]; !ok {
							log.Printf("WARNING: %s type %s not allowed, skipping", fileHeader.Filename, contentType)
							return
						}
					}
				}

				// Resolve uploader ID
				uploaderID := int(createdBy)
				if v, ok := c.Get("user_id"); ok {
					switch t := v.(type) {
					case int:
						uploaderID = t
					case int64:
						uploaderID = int(t)
					case uint:
						uploaderID = int(t)
					case uint64:
						uploaderID = int(t)
					case string:
						if n, e := strconv.Atoi(t); e == nil {
							uploaderID = n
						}
					}
				}

				// Use unified storage service; ensure we have an article
				if article != nil && article.ID > 0 {
					storageSvc := GetStorageService()
					storagePath := service.GenerateOTRSStoragePath(int(ticket.ID), article.ID, fileHeader.Filename)
					ctx := c.Request.Context()
					ctx = context.WithValue(ctx, service.CtxKeyArticleID, article.ID)
					ctx = service.WithUserID(ctx, uploaderID)

					if _, err := storageSvc.Store(ctx, file, fileHeader, storagePath); err != nil {
						log.Printf("ERROR: storage Store failed for ticket %d article %d: %v", ticket.ID, article.ID, err)
						return
					}

					// If backend is local FS, also insert DB metadata row for listing/download
					if _, isDB := storageSvc.(*service.DatabaseStorageService); !isDB {
						// Re-open to read bytes for DB row
						if f2, e2 := fileHeader.Open(); e2 == nil {
							defer f2.Close()
							b, rerr := io.ReadAll(f2)
							if rerr == nil {
								ct := contentType
								if ct == "" {
									ct = "application/octet-stream"
								}
								_, ierr := db.Exec(database.ConvertPlaceholders(`
									INSERT INTO article_data_mime_attachment (
										article_id, filename, content_type, content_size, content,
										disposition, create_time, create_by, change_time, change_by
									) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`),
									article.ID,
									fileHeader.Filename,
									ct,
									int64(len(b)),
									b,
									"attachment",
									time.Now(), uploaderID, time.Now(), uploaderID,
								)
								if ierr != nil {
									log.Printf("ERROR: attachment metadata insert failed: %v", ierr)
								}
							}
						}
					}

					// Track the info for response
					attachmentInfo = append(attachmentInfo, map[string]interface{}{
						"filename":     fileHeader.Filename,
						"size":         fileHeader.Size,
						"content_type": contentType,
						"saved":        true,
					})
					c.Header("HX-Trigger", "attachments-updated")
					log.Printf("Successfully saved attachment: %s (%d bytes) for ticket %d", fileHeader.Filename, fileHeader.Size, ticket.ID)
				} else {
					log.Printf("WARNING: No article available for attachments on ticket %d", ticket.ID)
				}
			}()
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

	// Persist time accounting entry if minutes were provided on creation
	if minutes > 0 {
		log.Printf("CreateTicketWithAttachments: attempting to save time entry ticket_id=%d minutes=%d", ticket.ID, minutes)
		if err := saveTimeEntry(db, ticket.ID, nil, minutes, int(createdBy)); err != nil {
			// Non-fatal: log and proceed with success response
			log.Printf("WARNING: Failed to save initial time entry for ticket %d: %v", ticket.ID, err)
		} else {
			log.Printf("Saved initial time entry for ticket %d: %d minutes", ticket.ID, minutes)
		}
	}

	// For HTMX, set the redirect header to the ticket detail page
	c.Header("HX-Redirect", fmt.Sprintf("/tickets/%d", ticket.ID))
	c.JSON(http.StatusCreated, response)
}
