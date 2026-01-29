package api

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/config"
	"github.com/gotrs-io/gotrs-ce/internal/constants"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/history"
	"github.com/gotrs-io/gotrs-ce/internal/mailqueue"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/notifications"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/service"
	"github.com/gotrs-io/gotrs-ce/internal/utils"
)

// This fixes the 500 error when users try to create tickets with attachments.
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
		Body           string `json:"body" form:"body"`
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

	if strings.TrimSpace(req.Body) == "" {
		if desc := strings.TrimSpace(c.PostForm("description")); desc != "" {
			req.Body = desc
		}
	}

	if strings.TrimSpace(req.Body) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Body is required"})
		return
	}

	// Fallback: if customer_email missing but customer_user_id present, try to look it up or treat as email
	if strings.TrimSpace(req.CustomerEmail) == "" {
		candidate := strings.TrimSpace(req.CustomerUserID)
		if candidate == "" {
			candidate = strings.TrimSpace(c.PostForm("customer_user_id"))
		}
		if candidate != "" {
			if strings.Contains(candidate, "@") {
				// If it contains @, treat it as an email address
				req.CustomerEmail = candidate
				log.Printf("CreateTicketWithAttachments: filled CustomerEmail from customer_user_id='%s'", candidate)
			} else {
				// Try to look up email from customer_user table
				// Match on login OR email since customer_user_id could contain either
				db, err := database.GetDB()
				if err == nil {
					var foundEmail sql.NullString
					err := db.QueryRow(database.ConvertPlaceholders(`SELECT email FROM customer_user WHERE (login = ? OR email = ?) AND valid_id = 1`), candidate, candidate).Scan(&foundEmail)
					if err == nil && foundEmail.Valid && foundEmail.String != "" {
						req.CustomerEmail = foundEmail.String
						log.Printf("CreateTicketWithAttachments: found customer email '%s' for user '%s'", req.CustomerEmail, candidate)
					}
				}
			}
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

	createdBy := GetUserIDFromCtxUint(c, 1)
	log.Printf("CreateTicketWithAttachments: createdBy user_id=%d", createdBy)

	db, err := database.GetDB()
	if err != nil {
		log.Printf("ERROR: Database connection failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	ticketRepo := repository.NewTicketRepository(db)
	articleRepo := repository.NewArticleRepository(db)
	ticketSvc := service.NewTicketService(ticketRepo, service.WithArticleRepository(articleRepo))

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
	priorityID := getPriorityID(req.Priority)
	visible := true
	// Agent-created tickets (phone/email) have sender type "agent" - the agent is recording/sending
	// This matches OTRS behavior where AgentTicketPhone/AgentTicketEmail articles are from the agent
	createInput := service.CreateTicketInput{
		Title:                         ticketTitle,
		QueueID:                       int(queueID),
		PriorityID:                    priorityID,
		StateID:                       stateID,
		UserID:                        int(createdBy),
		Body:                          req.Body,
		ArticleSubject:                ticketTitle,
		ArticleSenderTypeID:           constants.ArticleSenderAgent,
		ArticleTypeID:                 constants.ArticleTypeEmailExternal,
		ArticleIsVisibleForCustomer:   &visible,
		ArticleCommunicationChannelID: 1,
		CustomerUserID:                customerEmail,
		PendingUntil:                  pendingUnix,
	}
	if typeID > 0 {
		createInput.TypeID = int(typeID)
	}

	ticket, err := ticketSvc.Create(c.Request.Context(), createInput)
	if err != nil {
		log.Printf("ERROR: Failed to create ticket via service: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create ticket: " + err.Error()})
		return
	}
	log.Printf("Successfully created ticket ID: %d (with attachment support)", ticket.ID)

	// Process dynamic fields from form submission
	if !ticketSideEffectsDisabled() && c.Request.PostForm != nil {
		if dfErr := ProcessDynamicFieldsFromForm(c.Request.PostForm, ticket.ID, DFObjectTicket, "AgentTicketPhone"); dfErr != nil {
			log.Printf("WARNING: Failed to process dynamic fields for ticket %d: %v", ticket.ID, dfErr)
			// Non-fatal - continue with ticket creation
		}
	}

	var article *models.Article
	if !ticketSideEffectsDisabled() {
		if fetched, ferr := articleRepo.GetLatestCustomerArticleForTicket(uint(ticket.ID)); ferr != nil {
			log.Printf("WARNING: failed to load initial customer article for ticket %d: %v", ticket.ID, ferr)
		} else {
			article = fetched
		}
		if article == nil {
			if fallback, ferr := articleRepo.GetLatestArticleForTicket(uint(ticket.ID)); ferr == nil {
				article = fallback
			} else if ferr != nil {
				log.Printf("WARNING: fallback article lookup failed for ticket %d: %v", ticket.ID, ferr)
			}
		}
	} else {
		log.Printf("DEBUG: skipping article lookup for ticket %d due to side effect flag", ticket.ID)
	}

	// Process file attachments if present
	attachmentInfo := []map[string]interface{}{}

	if ticketSideEffectsDisabled() {
		log.Printf("DEBUG: skipping attachment persistence for ticket %d due to side effect flag", ticket.ID)
	} else if c.Request.MultipartForm != nil && c.Request.MultipartForm.File != nil {
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
				log.Printf("ERROR: File %s too large (%d bytes)", fileHeader.Filename, fileHeader.Size)
				c.JSON(http.StatusBadRequest, gin.H{"error": "file too large"})
				return
			}

			// Validate file type (basic security check)
			ext := filepath.Ext(fileHeader.Filename)
			blockedExtensions := map[string]bool{
				".exe": true, ".bat": true, ".sh": true, ".cmd": true,
				".com": true, ".scr": true, ".vbs": true, ".js": true,
			}

			if blockedExtensions[ext] {
				log.Printf("ERROR: File type %s not allowed for %s", ext, fileHeader.Filename)
				c.JSON(http.StatusBadRequest, gin.H{"error": "file type not allowed: " + ext})
				return
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
					if n, _ := file.Read(buf); n > 0 { //nolint:errcheck // Best effort content type detection
						contentType = detectContentType(fileHeader.Filename, buf[:n])
					}
					_, _ = file.Seek(0, 0) //nolint:errcheck // Best effort seek back
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
				uploaderID := GetUserIDFromCtx(c, int(createdBy))

				// Use unified storage service; ensure we have an article
				if article != nil && article.ID > 0 {
					storageSvc := GetStorageService()
					storagePath := service.GenerateOTRSStoragePath(ticket.ID, article.ID, fileHeader.Filename)
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
									) VALUES (?,?,?,?,?,?,?,?,?,?)`),
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
					log.Printf("Successfully saved attachment: %s (%d bytes) for ticket %d",
						fileHeader.Filename, fileHeader.Size, ticket.ID)
				} else {
					log.Printf("WARNING: No article available for attachments on ticket %d", ticket.ID)
				}
			}()
		}
	}

	// Prepare response
	response := gin.H{
		"success":       true,
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
	if minutes > 0 && !ticketSideEffectsDisabled() {
		log.Printf("CreateTicketWithAttachments: attempting to save time entry ticket_id=%d minutes=%d", ticket.ID, minutes)
		if err := saveTimeEntry(db, ticket.ID, nil, minutes, int(createdBy)); err != nil {
			// Non-fatal: log and proceed with success response
			log.Printf("WARNING: Failed to save initial time entry for ticket %d: %v", ticket.ID, err)
		} else {
			log.Printf("Saved initial time entry for ticket %d: %d minutes", ticket.ID, minutes)
		}
	}

	actorID := GetUserIDFromCtx(c, int(createdBy))

	// Record ticket creation history entry
	if ticketRepo != nil && !ticketSideEffectsDisabled() {
		recorder := history.NewRecorder(ticketRepo)

		var historyTicket = ticket
		if snapshot, err := ticketRepo.GetByID(uint(ticket.ID)); err == nil {
			historyTicket = snapshot
		} else {
			log.Printf("history snapshot (ticket create) failed: %v", err)
		}
		if historyTicket != nil {
			if historyTicket.ChangeTime.IsZero() {
				historyTicket.ChangeTime = time.Now()
			}
			var articleIDPtr *int
			if article != nil && article.ID > 0 {
				aid := article.ID
				articleIDPtr = &aid
			}
			message := fmt.Sprintf("Ticket created (%s)", historyTicket.TicketNumber)
			if err := recorder.Record(
				c.Request.Context(), nil, historyTicket, articleIDPtr, history.TypeNewTicket, message, actorID); err != nil {
				log.Printf("history record (ticket create) failed: %v", err)
			}
		}
	}

	// Queue email notification for ticket creation
	if ticketSideEffectsDisabled() {
		log.Printf("DEBUG: skipping ticket notification queue for ticket %d due to side effect flag", ticket.ID)
	} else {
		log.Printf("DEBUG: Queuing email for customerEmail='%s', ticketNumber='%s'", customerEmail, ticket.TicketNumber)
		go func() {
			subject := fmt.Sprintf("Ticket Created: %s", ticket.TicketNumber)
			body := fmt.Sprintf(
				"Your ticket has been created successfully.\n\nTicket Number: %s\nTitle: %s\n\n"+
					"Message:\n%s\n\nYou can view your ticket at: /tickets/%d\n\nBest regards,\nGOTRS Support Team",
				ticket.TicketNumber, ticket.Title, req.Body, ticket.ID)

			// Queue the email for processing by EmailQueueTask
			queueRepo := mailqueue.NewMailQueueRepository(db)
			var emailCfg *config.EmailConfig
			if cfg := config.Get(); cfg != nil {
				emailCfg = &cfg.Email
			}
			renderCtx := notifications.BuildRenderContext(context.Background(), db, req.CustomerUserID, actorID)
			branding, brandErr := notifications.PrepareQueueEmail(
				context.Background(),
				db,
				ticket.QueueID,
				body,
				utils.IsHTML(body),
				emailCfg,
				renderCtx,
			)
			if brandErr != nil {
				log.Printf("Queue identity lookup failed for ticket %d: %v", ticket.ID, brandErr)
			}
			senderEmail := branding.EnvelopeFrom
			var articleID64 *int64
			if article != nil && article.ID > 0 {
				id := int64(article.ID)
				articleID64 = &id
			}
			queueItem := &mailqueue.MailQueueItem{
				ArticleID: articleID64,
				Sender:    &senderEmail,
				Recipient: customerEmail,
				RawMessage: mailqueue.BuildEmailMessageWithThreading(
					branding.HeaderFrom, customerEmail, subject, branding.Body, branding.Domain, "", ""),
				Attempts:   0,
				CreateTime: time.Now(),
			}

			if err := queueRepo.Insert(context.Background(), queueItem); err != nil {
				log.Printf("Failed to queue email for %s: %v", customerEmail, err)
			} else {
				log.Printf("Queued email for %s (ticket %s) for processing", customerEmail, ticket.TicketNumber)
				messageID := mailqueue.ExtractMessageIDFromRawMessage(queueItem.RawMessage)
				if messageID != "" && articleID64 != nil {
					if _, err := db.Exec(database.ConvertPlaceholders(`
						UPDATE article_data_mime
						SET a_message_id = ?, a_in_reply_to = ?, a_references = ?,
						    change_time = CURRENT_TIMESTAMP, change_by = ?
						WHERE article_id = ?
					`), messageID, "", "", actorID, *articleID64); err != nil {
						log.Printf("Failed to store threading headers for article %d: %v", *articleID64, err)
					}
				}
			}
		}()
	}

	// For HTMX, set the redirect header to the ticket detail page
	c.Header("HX-Redirect", fmt.Sprintf("/tickets/%d", ticket.ID))
	c.JSON(http.StatusCreated, response)
}

func ticketSideEffectsDisabled() bool {
	val := strings.ToLower(strings.TrimSpace(os.Getenv("SKIP_TICKET_SIDE_EFFECTS")))
	switch val {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
