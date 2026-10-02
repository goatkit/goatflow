package api

// Ticket note, time tracking, and history handlers.
// Split from ticket_htmx_handlers.go for maintainability.

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/history"
	"github.com/goatkit/goatflow/internal/mailqueue"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/config"
	"github.com/goatkit/goatflow/internal/platform/constants"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/notifications"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/platform/utils"
	"github.com/goatkit/goatflow/internal/repository"
)

func init() {
	routing.RegisterHandler("handleAddTicketNote", handleAddTicketNote)
	routing.RegisterHandler("handleAddTicketTime", handleAddTicketTime)
	routing.RegisterHandler("HandleAddTicketTime", HandleAddTicketTime)
	routing.RegisterHandler("handleGetTicketHistory", handleGetTicketHistory)
}

// handleAddTicketNote adds a note to a ticket.
func handleAddTicketNote(c *gin.Context) {
	ticketID := c.Param("id")

	// Parse the note data
	var noteData struct {
		Content   string `json:"content" binding:"required"`
		Internal  bool   `json:"internal"`
		TimeUnits int    `json:"time_units"`
	}
	contentType := strings.ToLower(c.GetHeader("Content-Type"))
	if strings.Contains(contentType, "application/json") {
		if err := c.ShouldBindJSON(&noteData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Note content is required"})
			return
		}
	} else {
		// Accept form submissions too (agent path compatibility)
		noteData.Content = strings.TrimSpace(c.PostForm("body"))
		if noteData.Content == "" {
			noteData.Content = strings.TrimSpace(c.PostForm("content"))
		}
		noteData.Internal = c.PostForm("internal") == "true" || c.PostForm("internal") == "1"
		if tu := strings.TrimSpace(c.PostForm("time_units")); tu != "" {
			if v, err := strconv.Atoi(tu); err == nil && v >= 0 {
				noteData.TimeUnits = v
			}
		}
		if noteData.Content == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Note content is required"})
			return
		}
	}

	// Sanitise HTML content to prevent stored XSS.
	if utils.IsHTML(noteData.Content) {
		sanitizer := utils.NewHTMLSanitizer()
		noteData.Content = sanitizer.Sanitize(noteData.Content)
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	requireTimeUnits, err := isTimeUnitsRequired(db)
	if err != nil {
		log.Printf("addTicketNote: time units setting: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to load note settings"})
		return
	}
	if requireTimeUnits && noteData.TimeUnits <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Time units are required for notes"})
		return
	}

	// Get ticket to verify it exists
	ticketRepo := repository.NewTicketRepository(db)
	ticketIDInt, err := strconv.Atoi(ticketID)
	if err != nil {
		// Try to get by ticket number instead
		ticket, err := ticketRepo.GetByTicketNumber(ticketID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
			return
		}
		ticketIDInt = ticket.ID
	}
	userID, ok := auditUserID(c)
	if !ok {
		return
	}

	// Create article (note) in database
	articleRepo := repository.NewArticleRepository(db)
	article := &models.Article{
		TicketID:               ticketIDInt,
		Subject:                "Note",
		Body:                   noteData.Content,
		SenderTypeID:           constants.ArticleSenderAgent,
		CommunicationChannelID: constants.CommunicationChannelInternal,
		IsVisibleForCustomer:   0, // Internal note by default
		CreateBy:               userID,
		ChangeBy:               userID,
	}

	if !noteData.Internal {
		article.IsVisibleForCustomer = 1
	}

	err = articleRepo.Create(article)
	if err != nil {
		log.Printf("Error creating note: %v", err)
		c.Header("X-Guru-Error", "Failed to save note")
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to save note"})
		return
	}

	articleID := article.ID
	if ticket, terr := ticketRepo.GetByID(uint(ticketIDInt)); terr == nil {
		recorder := history.NewRecorder(ticketRepo)
		label := "Note added"
		if noteData.Internal {
			label = "Internal note added"
		} else if article.IsVisibleForCustomer == 1 {
			label = "Customer note added"
		}
		excerpt := history.Excerpt(noteData.Content, 140)
		message := label
		if excerpt != "" {
			message = fmt.Sprintf("%s — %s", label, excerpt)
		}
		if err := recorder.Record(c.Request.Context(), nil, ticket, &articleID, history.TypeAddNote, message, userID); err != nil {
			log.Printf("history record (note) failed: %v", err)
		}
	} else if terr != nil {
		log.Printf("history snapshot (note) failed: %v", terr)
	}

	// Persist time accounting if provided (associate with created article)
	if noteData.TimeUnits > 0 {
		if err := saveTimeEntry(db, ticketIDInt, &articleID, noteData.TimeUnits, userID); err != nil {
			c.Header("X-Guru-Error", "Failed to save time entry (note)")
		}
	}

	// Queue email notification for customer-visible notes
	if !noteData.Internal {
		// Get the full ticket to access customer info
		ticket, err := ticketRepo.GetByID(uint(ticketIDInt))
		if err != nil {
			log.Printf("Failed to get ticket for email notification: %v", err)
		} else if ticket.CustomerUserID != nil && *ticket.CustomerUserID != "" {
			go func() {
				// Look up customer's email address
				var customerEmail string
				err := db.QueryRow(database.ConvertPlaceholders(`
					SELECT cu.email
					FROM customer_user cu
					WHERE cu.login = ?
				`), *ticket.CustomerUserID).Scan(&customerEmail)

				if err != nil || customerEmail == "" {
					log.Printf("Failed to find email for customer user %s: %v", *ticket.CustomerUserID, err)
					return
				}

				subject := fmt.Sprintf("Update on Ticket %s", ticket.TicketNumber) //nolint:gk-sql-sprintf // hardcoded column fragments; user values bound via ?
				body := fmt.Sprintf(
					"A new update has been added to your ticket.\n\n%s\n\nBest regards,\nGoatFlow Support Team",
					noteData.Content)

				// Queue the email for processing by EmailQueueTask
				queueRepo := mailqueue.NewMailQueueRepository(db)
				var emailCfg *config.EmailConfig
				if cfg := config.Get(); cfg != nil {
					emailCfg = &cfg.Email
				}
				renderCtx, rcErr := notifications.BuildRenderContext(context.Background(), db, *ticket.CustomerUserID, userID)
				if rcErr != nil {
					log.Printf("Note notification for ticket %d not sent: %v", ticket.ID, rcErr)
					return
				}
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
				queueItem := &mailqueue.MailQueueItem{
					Sender:     &senderEmail,
					Recipient:  customerEmail,
					RawMessage: mailqueue.BuildEmailMessage(branding.HeaderFrom, customerEmail, subject, branding.Body),
					Attempts:   0,
					CreateTime: time.Now(),
				}

				if err := queueRepo.Insert(context.Background(), queueItem); err != nil {
					log.Printf("Failed to queue note notification email for %s: %v", customerEmail, err)
				} else {
					log.Printf("Queued note notification email for %s", customerEmail)
				}
			}()
		}
	}

	// Process dynamic fields from form submission (update ticket with values from note form)
	if c.Request.PostForm != nil {
		if dfErr := ProcessDynamicFieldsFromForm(c.Request.PostForm, ticketIDInt, DFObjectTicket, "AgentTicketNote"); dfErr != nil {
			log.Printf("WARNING: Failed to process dynamic fields for ticket %d from note: %v", ticketIDInt, dfErr)
		}
		// Process Article dynamic fields
		if dfErr := ProcessArticleDynamicFieldsFromForm(c.Request.PostForm, articleID, "AgentArticleNote"); dfErr != nil {
			log.Printf("WARNING: Failed to process article dynamic fields for article %d: %v", articleID, dfErr)
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":  true,
		"noteId":   article.ID,
		"ticketId": ticketIDInt,
		"created":  article.CreateTime.Format("2006-01-02 15:04"),
	})
}

// handleAddTicketTime adds a time accounting entry to a ticket and returns updated total minutes.
func handleAddTicketTime(c *gin.Context) {
	ticketID := c.Param("id")

	// Accept JSON or form
	var payload struct {
		TimeUnits int `json:"time_units"`
	}
	contentType := c.GetHeader("Content-Type")
	if strings.Contains(strings.ToLower(contentType), "application/json") {
		if err := c.ShouldBindJSON(&payload); err != nil {
			log.Printf("addTicketTime: JSON bind error for ticket %s: %v", ticketID, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid time payload"})
			return
		}
		log.Printf("addTicketTime: parsed JSON payload for ticket %s -> time_units=%d", ticketID, payload.TimeUnits)
	} else {
		tu := strings.TrimSpace(c.PostForm("time_units"))
		if tu != "" {
			if v, err := strconv.Atoi(tu); err == nil && v >= 0 {
				payload.TimeUnits = v
			}
		}
		log.Printf("addTicketTime: parsed FORM payload for ticket %s -> time_units=%d (raw='%s')", ticketID, payload.TimeUnits, tu)
	}

	if payload.TimeUnits <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "time_units must be > 0"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.Header("X-Guru-Error", "Database connection failed")
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	// Resolve ticket numeric ID from path (accepts id or ticket number)
	ticketRepo := repository.NewTicketRepository(db)
	ticketIDInt, convErr := strconv.Atoi(ticketID)
	if convErr != nil || ticketIDInt <= 0 {
		t, getErr := ticketRepo.GetByTicketNumber(ticketID)
		if getErr != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
			return
		}
		ticketIDInt = t.ID
	}

	userID, ok := auditUserID(c)
	if !ok {
		return
	}

	taRepo := repository.NewTimeAccountingRepository(db)
	if err := saveTimeEntry(db, ticketIDInt, nil, payload.TimeUnits, userID); err != nil {
		c.Header("X-Guru-Error", "Failed to save time entry")
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to save time entry"})
		return
	}
	log.Printf("addTicketTime: saved time entry ticket_id=%d minutes=%d by user=%d", ticketIDInt, payload.TimeUnits, userID)

	// Return updated total
	entries, _ := taRepo.ListByTicket(ticketIDInt) //nolint:errcheck // Empty on error
	total := 0
	for _, e := range entries {
		total += e.TimeUnit
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "time_total_minutes": total})
}

// HandleAddTicketTime is the exported wrapper for YAML routing in the routing package
// It delegates to handleAddTicketTime to keep the implementation in one place.
func HandleAddTicketTime(c *gin.Context) { handleAddTicketTime(c) }

// handleGetTicketHistory returns the ticket_history rows of a ticket, newest
// first, with history type, actor and the queue/state/priority snapshot.
// Optional query parameter limit (1-1000, default 100).
func handleGetTicketHistory(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || ticketID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid ticket ID"})
		return
	}
	limit := 100
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 1000 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "limit must be between 1 and 1000"})
			return
		}
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Database connection failed"})
		return
	}
	var exists bool
	if err := db.QueryRow(database.ConvertPlaceholders(
		"SELECT EXISTS(SELECT 1 FROM ticket WHERE id = ?)"), ticketID).Scan(&exists); err != nil {
		log.Printf("handleGetTicketHistory: ticket lookup failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to load history"})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Ticket not found"})
		return
	}

	entries, err := repository.NewTicketRepository(db).GetTicketHistoryEntries(uint(ticketID), limit)
	if err != nil {
		log.Printf("handleGetTicketHistory: query failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to load history"})
		return
	}

	items := make([]gin.H, 0, len(entries))
	for _, e := range entries {
		items = append(items, gin.H{
			"id":              e.ID,
			"history_type":    e.HistoryType,
			"name":            e.Name,
			"message":         history.NormalizeHistoryName(e),
			"created_at":      e.CreatedAt.UTC().Format(time.RFC3339),
			"created_by":      gin.H{"login": e.CreatorLogin, "name": e.CreatorFullName},
			"queue":           e.QueueName,
			"state":           e.StateName,
			"priority":        e.PriorityName,
			"article_subject": e.ArticleSubject,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"ticket_id": ticketID,
		"history":   items,
	})
}
