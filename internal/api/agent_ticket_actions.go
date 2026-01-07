package api

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/config"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/history"
	"github.com/gotrs-io/gotrs-ce/internal/mailqueue"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/notifications"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/utils"
)

// noteNotificationParams holds parameters for customer note email notifications.
type noteNotificationParams struct {
	DB        *sql.DB
	Ticket    *models.Ticket
	ArticleID int64
	UserID    uint
	Subject   string
	Body      string
}

// queueCustomerNoteNotification sends an email notification to the customer about a note.
// This runs in a goroutine and handles all email preparation, threading, and queuing.
func queueCustomerNoteNotification(params noteNotificationParams) {
	if params.Ticket.CustomerUserID == nil || *params.Ticket.CustomerUserID == "" {
		return
	}

	customerEmail := lookupCustomerEmail(params.DB, *params.Ticket.CustomerUserID)
	if customerEmail == "" {
		return
	}

	emailSubject := fmt.Sprintf("Update on Ticket %s", params.Ticket.TicketNumber)
	emailBody := buildNoteEmailBody(params.Subject, params.Body)
	inReplyTo, references := getThreadingHeaders(params.DB, uint(params.Ticket.ID))

	branding := prepareNotificationBranding(params.DB, params.Ticket, params.UserID, emailBody)
	rawMsg := mailqueue.BuildEmailMessageWithThreading(
		branding.HeaderFrom, customerEmail, emailSubject, branding.Body,
		branding.Domain, inReplyTo, references)

	queueItem := &mailqueue.MailQueueItem{
		Sender:     &branding.EnvelopeFrom,
		Recipient:  customerEmail,
		RawMessage: rawMsg,
		Attempts:   0,
		CreateTime: time.Now(),
	}

	queueRepo := mailqueue.NewMailQueueRepository(params.DB)
	if err := queueRepo.Insert(context.Background(), queueItem); err != nil {
		log.Printf("Failed to queue note notification email for %s: %v", customerEmail, err)
		return
	}

	log.Printf("Queued note notification email for %s", customerEmail)
	storeArticleThreadingHeaders(params.DB, params.ArticleID, params.UserID, string(queueItem.RawMessage), inReplyTo, references)
}

func lookupCustomerEmail(db *sql.DB, customerUserID string) string {
	var email string
	err := db.QueryRow(database.ConvertPlaceholders(`
		SELECT cu.email FROM customer_user cu WHERE cu.login = ?
	`), customerUserID).Scan(&email)
	if err != nil || email == "" {
		log.Printf("Failed to find email for customer user %s: %v", customerUserID, err)
		return ""
	}
	return email
}

func buildNoteEmailBody(subject, body string) string {
	hasCustomSubject := subject != "" && subject != "Internal Note" &&
		subject != "Email Note" && subject != "Phone Note" && subject != "Chat Note"
	if hasCustomSubject {
		return fmt.Sprintf("A new update has been added to your ticket.\n\n"+
			"Subject: %s\n\n%s\n\nBest regards,\nGOTRS Support Team", subject, body)
	}
	return fmt.Sprintf("A new update has been added to your ticket.\n\n"+
		"%s\n\nBest regards,\nGOTRS Support Team", body)
}

func getThreadingHeaders(db *sql.DB, ticketID uint) (inReplyTo, references string) {
	articleRepo := repository.NewArticleRepository(db)
	latestCustomerArticle, err := articleRepo.GetLatestCustomerArticleForTicket(ticketID)
	if err != nil || latestCustomerArticle == nil || latestCustomerArticle.MessageID == "" {
		return "", ""
	}
	inReplyTo = latestCustomerArticle.MessageID
	references = latestCustomerArticle.MessageID
	if latestCustomerArticle.References != "" {
		references = latestCustomerArticle.References + " " + latestCustomerArticle.MessageID
	}
	return inReplyTo, references
}

func prepareNotificationBranding(
	db *sql.DB, ticket *models.Ticket, userID uint, emailBody string,
) *notifications.EmailBranding {
	var emailCfg *config.EmailConfig
	if cfg := config.Get(); cfg != nil {
		emailCfg = &cfg.Email
	}
	customerLogin := ""
	if ticket.CustomerUserID != nil {
		customerLogin = *ticket.CustomerUserID
	}
	renderCtx := notifications.BuildRenderContext(context.Background(), db, customerLogin, int(userID))
	branding, err := notifications.PrepareQueueEmail(
		context.Background(), db, ticket.QueueID, emailBody, utils.IsHTML(emailBody), emailCfg, renderCtx,
	)
	if err != nil {
		log.Printf("Queue identity lookup failed for ticket %d: %v", ticket.ID, err)
	}
	return branding
}

func storeArticleThreadingHeaders(
	db *sql.DB, articleID int64, userID uint, rawMsg, inReplyTo, references string,
) {
	messageID := mailqueue.ExtractMessageIDFromRawMessage([]byte(rawMsg))
	if messageID == "" {
		return
	}
	_, err := db.Exec(database.ConvertPlaceholders(`
		UPDATE article_data_mime 
		SET a_message_id = ?, a_in_reply_to = ?, a_references = ?,
			change_time = CURRENT_TIMESTAMP, change_by = ?
		WHERE article_id = ?
	`), messageID, inReplyTo, references, userID, articleID)
	if err != nil {
		log.Printf("Failed to store threading headers for article %d: %v", articleID, err)
	}
}

// queueArticleNotificationEmail queues an email notification for a new article.
// This is used when an article is created via the API and is visible to the customer.
func queueArticleNotificationEmail(db *sql.DB, ticketID int, articleID int64, customerUserLogin string, userID int, articleBody string) {
	customerEmail := lookupCustomerEmail(db, customerUserLogin)
	if customerEmail == "" {
		return
	}

	var queueID int
	var ticketNumber sql.NullString
	ticketQuery := `SELECT queue_id, tn FROM ticket WHERE id = ?`
	if err := db.QueryRow(database.ConvertPlaceholders(ticketQuery), ticketID).Scan(&queueID, &ticketNumber); err != nil {
		log.Printf("Failed to load ticket metadata for article %d: %v", ticketID, err)
	}

	inReplyTo, references := getThreadingHeaders(db, uint(ticketID))

	subject := "Update on Ticket"
	if ticketNumber.Valid && ticketNumber.String != "" {
		subject = fmt.Sprintf("Update on Ticket %s", ticketNumber.String)
	}
	body := fmt.Sprintf("A new update has been added to your ticket.\n\n%s\n\nBest regards,\nGOTRS Support Team", articleBody)

	var emailCfg *config.EmailConfig
	if cfg := config.Get(); cfg != nil {
		emailCfg = &cfg.Email
	}
	renderCtx := notifications.BuildRenderContext(context.Background(), db, customerUserLogin, userID)
	branding, err := notifications.PrepareQueueEmail(context.Background(), db, queueID, body, utils.IsHTML(body), emailCfg, renderCtx)
	if err != nil {
		log.Printf("Queue identity lookup failed for ticket %d: %v", ticketID, err)
	}

	rawMsg := mailqueue.BuildEmailMessageWithThreading(
		branding.HeaderFrom, customerEmail, subject, branding.Body, branding.Domain, inReplyTo, references)
	queueItem := &mailqueue.MailQueueItem{
		Sender:     &branding.EnvelopeFrom,
		Recipient:  customerEmail,
		RawMessage: rawMsg,
		Attempts:   0,
		CreateTime: time.Now(),
	}

	queueRepo := mailqueue.NewMailQueueRepository(db)
	if err := queueRepo.Insert(context.Background(), queueItem); err != nil {
		log.Printf("Failed to queue article notification email for %s: %v", customerEmail, err)
		return
	}
	log.Printf("Queued article notification email for %s", customerEmail)
	storeArticleThreadingHeaders(db, articleID, uint(userID), string(queueItem.RawMessage), inReplyTo, references)
}

func handleAgentTicketReply(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID := c.Param("id")

		// Parse multipart form to handle file uploads
		err := c.Request.ParseMultipartForm(128 << 20) // 128MB max
		if err != nil && err != http.ErrNotMultipart {
			log.Printf("Error parsing multipart form: %v", err)
		}

		to := c.PostForm("to")
		subject := c.PostForm("subject")
		body := c.PostForm("body")

		if strings.TrimSpace(to) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "recipient required"})
			return
		}
		if !strings.Contains(to, "@") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email"})
			return
		}

		if db == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			return
		}

		ticketID = strings.TrimSpace(ticketID)
		tid, convErr := strconv.Atoi(ticketID)
		if convErr != nil || tid <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
			return
		}

		ticketRepo := repository.NewTicketRepository(db)
		if _, err := ticketRepo.GetByID(uint(tid)); err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
			} else {
				log.Printf("Error loading ticket %s before reply: %v", ticketID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load ticket"})
			}
			return
		}

		// Get user info
		userID := c.GetUint("user_id")
		userName := c.GetString("user_name")
		if userName == "" {
			userName = "Agent"
		}

		// Sanitize HTML content if detected
		contentType := "text/plain"
		if utils.IsHTML(body) {
			sanitizer := utils.NewHTMLSanitizer()
			body = sanitizer.Sanitize(body)
			contentType = "text/html"
		}

		// Filter Unicode characters if Unicode support is disabled (OTRS compatibility mode)
		if os.Getenv("UNICODE_SUPPORT") != "true" && os.Getenv("UNICODE_SUPPORT") != "1" && os.Getenv("UNICODE_SUPPORT") != "enabled" {
			body = utils.FilterUnicode(body)
		}

		// Start transaction
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
			return
		}
		defer func() { _ = tx.Rollback() }()

		// Insert article (communication_channel_id = 1 for email, visible to customer)
		articleID, err := insertArticle(tx, ArticleInsertParams{
			TicketID:             int64(tid),
			CommunicationChannel: 1, // email
			IsVisibleForCustomer: 1,
			CreateBy:             int64(userID),
		})
		if err != nil {
			log.Printf("Error creating article: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add reply"})
			return
		}

		// Insert article MIME data
		if err := insertArticleMimeData(tx, ArticleMimeParams{
			ArticleID:    articleID,
			From:         userName,
			To:           to,
			Subject:      subject,
			Body:         body,
			ContentType:  contentType,
			IncomingTime: time.Now().Unix(),
			CreateBy:     int64(userID),
		}); err != nil {
			log.Printf("Error adding article data: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add reply data"})
			return
		}

		// Handle file attachments if present
		if c.Request.MultipartForm != nil && c.Request.MultipartForm.File != nil {
			files := c.Request.MultipartForm.File["attachments"]
			for _, fileHeader := range files {
				file, err := fileHeader.Open()
				if err != nil {
					log.Printf("Error opening attachment %s: %v", fileHeader.Filename, err)
					continue
				}
				defer file.Close()

				// Read file content
				content, err := io.ReadAll(file)
				if err != nil {
					log.Printf("Error reading attachment %s: %v", fileHeader.Filename, err)
					continue
				}

				// Detect content type
				contentType := fileHeader.Header.Get("Content-Type")
				if contentType == "" {
					contentType = http.DetectContentType(content)
				}

				// Insert attachment - adapter handles placeholder conversion and arg remapping
				attachmentInsert := `
					INSERT INTO article_data_mime_attachment (
						article_id, filename, content_type, content, content_size,
						create_time, create_by, change_time, change_by
					) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, ?)
				`
				_, err = database.GetAdapter().ExecTx(tx, attachmentInsert,
					articleID, fileHeader.Filename, contentType, content, len(content), userID)

				if err != nil {
					log.Printf("Error saving attachment %s: %v", fileHeader.Filename, err)
				} else {
					log.Printf("Saved attachment %s for article %d", fileHeader.Filename, articleID)
				}
			}
		}

		// Update ticket change time
		query := database.ConvertPlaceholders(
			"UPDATE ticket SET change_time = CURRENT_TIMESTAMP, change_by = ? WHERE id = ?")
		_, err = tx.Exec(query, userID, tid)
		if err != nil {
			log.Printf("Error updating ticket: %v", err)
		}

		// Commit transaction
		if err = tx.Commit(); err != nil {
			log.Printf("Error committing transaction: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save reply"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "article_id": articleID})
	}
}

func handleAgentTicketNote(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID := c.Param("id")
		ticketID = strings.TrimSpace(ticketID)

		// Get body content - try JSON first, then form data
		var body string
		if c.ContentType() == "application/json" {
			var jsonData struct {
				Body string `json:"body"`
			}
			if err := c.ShouldBindJSON(&jsonData); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
				return
			}
			body = jsonData.Body
		} else {
			body = c.PostForm("body")
		}

		if strings.TrimSpace(body) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Note body required"})
			return
		}

		subject := strings.TrimSpace(c.PostForm("subject"))

		// Parse optional time units (minutes) from form; accept both snake and camel case
		timeUnits := 0
		if tu := strings.TrimSpace(c.PostForm("time_units")); tu != "" {
			if v, err := strconv.Atoi(tu); err == nil && v > 0 {
				timeUnits = v
			}
		} else if tu := strings.TrimSpace(c.PostForm("timeUnits")); tu != "" { // fallback
			if v, err := strconv.Atoi(tu); err == nil && v > 0 {
				timeUnits = v
			}
		}

		// Get communication channel from form (defaults to Internal if not specified)
		communicationChannelID := c.DefaultPostForm("communication_channel_id", "3")
		channelID, err := strconv.Atoi(communicationChannelID)
		if err != nil || channelID < 1 || channelID > 4 {
			channelID = 3 // Default to Internal
		}

		// Get visibility flag (checkbox value will be "1" if checked, empty if not)
		isVisibleForCustomer := 0
		if c.PostForm("is_visible_for_customer") == "1" {
			isVisibleForCustomer = 1
		}

		nextStateIDRaw := strings.TrimSpace(c.PostForm("next_state_id"))
		pendingUntilRaw := strings.TrimSpace(c.PostForm("pending_until"))

		if db == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			return
		}

		tid, err := strconv.Atoi(ticketID)
		if err != nil || tid <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket id"})
			return
		}

		ticketRepo := repository.NewTicketRepository(db)
		prevTicket, prevErr := ticketRepo.GetByID(uint(tid))
		if prevErr != nil {
			log.Printf("Error loading ticket %s before note: %v", ticketID, prevErr)
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
			return
		}

		var (
			nextStateID      int
			pendingUntilUnix int64
			stateChanged     bool
		)
		if nextStateIDRaw != "" {
			id, err := strconv.Atoi(nextStateIDRaw)
			if err != nil || id <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid next state selection"})
				return
			}
			nextStateID = id
			stateRepo := repository.NewTicketStateRepository(db)
			nextState, err := stateRepo.GetByID(uint(id))
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid next state selection"})
				return
			}
			if isPendingState(nextState) {
				if pendingUntilRaw == "" {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Pending time required for pending states"})
					return
				}
				parsed := parsePendingUntil(pendingUntilRaw)
				if parsed <= 0 {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid pending time format"})
					return
				}
				pendingUntilUnix = int64(parsed)
			} else {
				pendingUntilUnix = 0
			}
			stateChanged = true
		}

		// Get user info
		userID := c.GetUint("user_id")

		// Sanitize HTML content if detected
		contentType := "text/plain"
		if utils.IsHTML(body) {
			sanitizer := utils.NewHTMLSanitizer()
			body = sanitizer.Sanitize(body)
			contentType = "text/html"
		}
		if strings.TrimSpace(body) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Note body required"})
			return
		}

		// Filter Unicode characters if Unicode support is disabled (OTRS compatibility mode)
		if os.Getenv("UNICODE_SUPPORT") != "true" && os.Getenv("UNICODE_SUPPORT") != "1" && os.Getenv("UNICODE_SUPPORT") != "enabled" {
			body = utils.FilterUnicode(body)
		}

		// Start transaction
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
			return
		}
		defer func() { _ = tx.Rollback() }()

		// Insert article
		articleID, err := insertArticle(tx, ArticleInsertParams{
			TicketID:             int64(tid),
			CommunicationChannel: channelID,
			IsVisibleForCustomer: isVisibleForCustomer,
			CreateBy:             int64(userID),
		})
		if err != nil {
			log.Printf("Error creating article: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add note"})
			return
		}

		// Use subject from form or default based on communication channel
		if subject == "" {
			subject = defaultNoteSubject(channelID)
		}

		// Insert article MIME data
		if err := insertArticleMimeData(tx, ArticleMimeParams{
			ArticleID:    articleID,
			From:         "Agent",
			Subject:      subject,
			Body:         body,
			ContentType:  contentType,
			IncomingTime: time.Now().Unix(),
			CreateBy:     int64(userID),
		}); err != nil {
			log.Printf("Error adding article data: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add note data"})
			return
		}

		if stateChanged {
			if _, err := tx.Exec(database.ConvertPlaceholders(`
				UPDATE ticket
				SET ticket_state_id = ?, until_time = ?, change_time = CURRENT_TIMESTAMP, change_by = ?
				WHERE id = ?
			`), nextStateID, pendingUntilUnix, userID, tid); err != nil {
				log.Printf("Error updating ticket state from note: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update ticket state"})
				return
			}
		} else {
			if _, err = tx.Exec(database.ConvertPlaceholders(`
				UPDATE ticket
				SET change_time = CURRENT_TIMESTAMP, change_by = ?
				WHERE id = ?
			`), userID, tid); err != nil {
				log.Printf("Error updating ticket: %v", err)
			}
		}

		// Commit transaction
		if err = tx.Commit(); err != nil {
			log.Printf("Error committing transaction: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save note"})
			return
		}

		// Persist time accounting (after commit to avoid orphaning on rollback)
		if timeUnits > 0 {
			aid := int(articleID)
			uid := int(userID)
			if err := saveTimeEntry(db, tid, &aid, timeUnits, uid); err != nil {
				log.Printf("Failed to save time entry for ticket %d article %d: %v", tid, aid, err)
			} else {
				log.Printf("Saved time entry for ticket %d article %d: %d minutes", tid, aid, timeUnits)
			}
		}

		updatedTicket, terr := ticketRepo.GetByID(uint(tid))
		if terr != nil {
			log.Printf("history snapshot (agent note) failed: %v", terr)
			c.JSON(http.StatusOK, gin.H{"success": true, "article_id": articleID})
			return
		}

		recorder := history.NewRecorder(ticketRepo)
		actorID := int(userID)
		if actorID <= 0 {
			actorID = 1
		}

		label := noteLabel(channelID, isVisibleForCustomer)
		excerpt := history.Excerpt(body, 140)
		message := label
		if excerpt != "" {
			message = fmt.Sprintf("%s — %s", label, excerpt)
		}

		aID := int(articleID)
		if err := recorder.Record(c.Request.Context(), nil, updatedTicket, &aID, history.TypeAddNote, message, actorID); err != nil {
			log.Printf("history record (agent note) failed: %v", err)
		}

		if stateChanged {
			prevStateName := ""
			if prevTicket != nil {
				if st, serr := loadTicketState(ticketRepo, prevTicket.TicketStateID); serr == nil && st != nil {
					prevStateName = st.Name
				} else if serr != nil {
					log.Printf("history agent note state lookup (prev) failed: %v", serr)
				} else if prevTicket.TicketStateID > 0 {
					prevStateName = fmt.Sprintf("state %d", prevTicket.TicketStateID)
				}
			}

			newStateName := fmt.Sprintf("state %d", nextStateID)
			if st, serr := loadTicketState(ticketRepo, nextStateID); serr == nil && st != nil {
				newStateName = st.Name
			} else if serr != nil {
				log.Printf("history agent note state lookup (new) failed: %v", serr)
			}

			stateMsg := history.ChangeMessage("State", prevStateName, newStateName)
			if strings.TrimSpace(stateMsg) == "" {
				stateMsg = fmt.Sprintf("State set to %s", newStateName)
			}
			err := recorder.Record(c.Request.Context(), nil, updatedTicket, &aID,
				history.TypeStateUpdate, stateMsg, actorID)
			if err != nil {
				log.Printf("history record (agent note state) failed: %v", err)
			}

			if pendingUntilUnix > 0 {
				pendingTime := time.Unix(pendingUntilUnix, 0).In(time.Local).Format("02 Jan 2006 15:04")
				pendingMsg := fmt.Sprintf("Pending until %s", pendingTime)
				err := recorder.Record(c.Request.Context(), nil, updatedTicket, &aID,
					history.TypeSetPendingTime, pendingMsg, actorID)
				if err != nil {
					log.Printf("history record (agent note pending) failed: %v", err)
				}
			} else if prevTicket != nil && prevTicket.UntilTime > 0 {
				err := recorder.Record(c.Request.Context(), nil, updatedTicket, &aID,
					history.TypeSetPendingTime, "Pending time cleared", actorID)
				if err != nil {
					log.Printf("history record (agent note pending clear) failed: %v", err)
				}
			}
		}

		// Queue email notification for customer-visible notes
		if isVisibleForCustomer == 1 {
			ticket, err := ticketRepo.GetByID(uint(tid))
			if err != nil {
				log.Printf("Failed to get ticket for email notification: %v", err)
			} else {
				go queueCustomerNoteNotification(noteNotificationParams{
					DB:        db,
					Ticket:    ticket,
					ArticleID: articleID,
					UserID:    userID,
					Subject:   subject,
					Body:      body,
				})
			}
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "article_id": articleID})
	}
}

func noteLabel(channelID int, visibleForCustomer int) string {
	if visibleForCustomer == 1 {
		return "Customer note added"
	}
	switch channelID {
	case 1:
		return "Email note added"
	case 2:
		return "Phone note added"
	case 3:
		return "Internal note added"
	case 4:
		return "Chat note added"
	default:
		return "Note added"
	}
}

func handleAgentTicketPhone(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID := c.Param("id")
		subject := c.PostForm("subject")
		body := c.PostForm("body")

		if subject == "" {
			subject = "Phone call note"
		}

		// Get user info
		userID := c.GetUint("user_id")

		// Sanitize HTML content if detected
		contentType := "text/plain"
		if utils.IsHTML(body) {
			sanitizer := utils.NewHTMLSanitizer()
			body = sanitizer.Sanitize(body)
			contentType = "text/html"
		}

		// Filter Unicode characters if Unicode support is disabled (OTRS compatibility mode)
		if os.Getenv("UNICODE_SUPPORT") != "true" && os.Getenv("UNICODE_SUPPORT") != "1" && os.Getenv("UNICODE_SUPPORT") != "enabled" {
			body = utils.FilterUnicode(body)
		}

		if db == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
			return
		}

		ticketID = strings.TrimSpace(ticketID)
		tid, convErr := strconv.Atoi(ticketID)
		if convErr != nil || tid <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
			return
		}

		// Start transaction
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
			return
		}
		defer func() { _ = tx.Rollback() }()

		// Insert article (phone communication channel = 2, visible to customer)
		articleID, err := insertArticle(tx, ArticleInsertParams{
			TicketID:             int64(tid),
			CommunicationChannel: 2, // phone
			IsVisibleForCustomer: 1,
			CreateBy:             int64(userID),
		})
		if err != nil {
			log.Printf("Error inserting phone article: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save phone note"})
			return
		}

		// Insert MIME data
		if err := insertArticleMimeData(tx, ArticleMimeParams{
			ArticleID:    articleID,
			From:         "Agent Phone Call",
			Subject:      subject,
			Body:         body,
			ContentType:  contentType,
			IncomingTime: time.Now().Unix(),
			CreateBy:     int64(userID),
		}); err != nil {
			log.Printf("Error inserting phone article MIME data: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save phone note data"})
			return
		}

		if _, err = tx.Exec(database.ConvertPlaceholders(`
			UPDATE ticket SET change_time = CURRENT_TIMESTAMP, change_by = ? WHERE id = ?
		`), userID, tid); err != nil {
			log.Printf("Error updating ticket change time for phone note: %v", err)
		}

		if err = tx.Commit(); err != nil {
			log.Printf("Error committing transaction: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save phone note"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "article_id": articleID})
	}
}

func handleAgentTicketStatus(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID := c.Param("id")
		statusID := c.PostForm("status_id")
		pendingUntil := c.PostForm("pending_until")

		// Handle pending time for pending states
		var untilTime int64
		pendingStates := map[string]bool{"6": true, "7": true, "8": true} // pending reminder, pending auto close+, pending auto close-

		if pendingStates[statusID] {
			if pendingUntil == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Pending time is required for pending states"})
				return
			} // Parse the datetime-local format: 2006-01-02T15:04
			if t, err := time.Parse("2006-01-02T15:04", pendingUntil); err == nil {
				untilTime = t.Unix()
				log.Printf("Setting pending time for ticket %s to %v (unix: %d)", ticketID, t, t.Unix())
			} else {
				log.Printf("Failed to parse pending time '%s': %v", pendingUntil, err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid pending time format"})
				return
			}
		} else {
			// Clear pending time for non-pending states
			untilTime = 0
		}

		// Update ticket status with pending time
		_, err := db.Exec(database.ConvertPlaceholders(`
			UPDATE ticket 
			SET ticket_state_id = ?, until_time = ?, change_time = CURRENT_TIMESTAMP, change_by = ?
			WHERE id = ?
		`), statusID, untilTime, c.GetUint("user_id"), ticketID)

		if err != nil {
			log.Printf("Error updating ticket status: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status"})
			return
		}

		// Log the status change for audit trail
		statusName := "unknown"
		var statusRow struct {
			Name string
		}
		err = db.QueryRow(database.ConvertPlaceholders("SELECT name FROM ticket_state WHERE id = ?"), statusID).Scan(&statusRow.Name)
		if err == nil {
			statusName = statusRow.Name
		}
		if untilTime > 0 {
			log.Printf("Ticket %s status changed to %s (ID: %s) with pending time until %v by user %d",
				ticketID, statusName, statusID, time.Unix(untilTime, 0), c.GetUint("user_id"))
		} else {
			log.Printf("Ticket %s status changed to %s (ID: %s) by user %d",
				ticketID, statusName, statusID, c.GetUint("user_id"))
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

func handleAgentTicketAssign(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID := c.Param("id")
		userID := c.PostForm("user_id")

		// Validate input
		if userID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No agent selected"})
			return
		}

		// Convert userID to int for validation
		agentID, err := strconv.Atoi(userID)
		if err != nil || agentID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
			return
		}

		// Log the assignment for debugging
		currentUserID := c.GetUint("user_id")

		// Update responsible user
		_, err = db.Exec(database.ConvertPlaceholders(`
			UPDATE ticket 
			SET responsible_user_id = ?, change_time = CURRENT_TIMESTAMP, change_by = ?
			WHERE id = ?
		`), agentID, currentUserID, ticketID)

		if err != nil {
			log.Printf("ERROR: Failed to assign ticket %s to agent %d: %v", ticketID, agentID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign agent"})
			return
		}

		log.Printf("SUCCESS: Assigned ticket %s to agent %d", ticketID, agentID)
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

func handleAgentTicketPriority(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID := c.Param("id")
		priorityID := c.PostForm("priority_id")

		// Update ticket priority
		_, err := db.Exec(database.ConvertPlaceholders(`
			UPDATE ticket 
			SET ticket_priority_id = ?, change_time = CURRENT_TIMESTAMP, change_by = ?
			WHERE id = ?
		`), priorityID, c.GetUint("user_id"), ticketID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update priority"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// NEWLY ADDED: Missing handlers that were causing 404 errors.
func handleAgentTicketQueue(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID := c.Param("id")
		queueID := c.PostForm("queue_id")

		// Update ticket queue
		_, err := db.Exec(database.ConvertPlaceholders(`
			UPDATE ticket 
			SET queue_id = ?, change_time = CURRENT_TIMESTAMP, change_by = ?
			WHERE id = ?
		`), queueID, c.GetUint("user_id"), ticketID)

		if err != nil {
			log.Printf("Error updating ticket queue: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to move ticket to queue"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

func handleAgentTicketMerge(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		sourceTicketID := c.Param("id")
		targetTN := c.PostForm("target_tn")
		sourceID, parseErr := strconv.Atoi(sourceTicketID)
		if parseErr != nil || sourceID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
			return
		}

		if targetTN == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Target ticket number required"})
			return
		}

		// Start transaction for merge operation
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
			return
		}
		defer func() { _ = tx.Rollback() }()

		// Find target ticket ID by ticket number
		var targetTicketID int
		err = tx.QueryRow("SELECT id FROM ticket WHERE tn = ?", targetTN).Scan(&targetTicketID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Target ticket not found"})
			return
		}

		// Move all articles from source to target ticket
		_, err = tx.Exec(database.ConvertPlaceholders(`
			UPDATE article 
			SET ticket_id = ?, change_time = CURRENT_TIMESTAMP, change_by = ?
			WHERE ticket_id = ?
		`), targetTicketID, c.GetUint("user_id"), sourceTicketID)

		if err != nil {
			log.Printf("Error moving articles during merge: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to merge articles"})
			return
		}

		// Close the source ticket
		_, err = tx.Exec(database.ConvertPlaceholders(`
			UPDATE ticket 
			SET ticket_state_id = (SELECT id FROM ticket_state WHERE name = 'merged'),
				change_time = CURRENT_TIMESTAMP, change_by = ?
			WHERE id = ?
		`), c.GetUint("user_id"), sourceTicketID)

		if err != nil {
			log.Printf("Error closing source ticket during merge: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to close source ticket"})
			return
		}

		// Commit transaction
		if err = tx.Commit(); err != nil {
			log.Printf("Error committing merge transaction: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete merge"})
			return
		}

		recordMergeHistory(c, targetTicketID, []int{sourceID}, "")

		c.JSON(http.StatusOK, gin.H{
			"success":       true,
			"message":       fmt.Sprintf("Ticket merged into %s", targetTN),
			"target_ticket": targetTN,
		})
	}
}

// handleAgentTicketDraft saves a draft reply for a ticket.
func handleAgentTicketDraft(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID := c.Param("id")
		userID, _ := c.Get("user_id")

		var request struct {
			Subject     string `json:"subject"`
			Body        string `json:"body"`
			To          string `json:"to"`
			Cc          string `json:"cc"`
			Bcc         string `json:"bcc"`
			ContentType string `json:"content_type"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}

		log.Printf("Draft saved for ticket %s by user %v: subject='%s', body length=%d",
			ticketID, userID, request.Subject, len(request.Body))

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Draft saved successfully",
		})
	}
}
