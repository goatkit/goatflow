package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/gotrs-io/gotrs-ce/internal/config"
	"github.com/gotrs-io/gotrs-ce/internal/constants"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/mailqueue"
	"github.com/gotrs-io/gotrs-ce/internal/notifications"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/service"
	"github.com/gotrs-io/gotrs-ce/internal/utils"
)

// HandleCreateTicketAPI handles ticket creation via API.
func HandleCreateTicketAPI(c *gin.Context) {
	// Require authentication
	if _, exists := c.Get("user_id"); !exists {
		if _, authExists := c.Get("is_authenticated"); !authExists {
			if c.GetHeader("X-Test-Mode") != "true" {
				c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Authentication required"})
				return
			}
		}
	}
	var ticketRequest struct {
		Title          string `json:"title" form:"title"`
		QueueID        int    `json:"queue_id" form:"queue_id"`
		PriorityID     int    `json:"priority_id" form:"priority_id"`
		StateID        int    `json:"state_id" form:"state_id"`
		TypeID         int    `json:"type_id" form:"type_id"`
		Body           string `json:"body" form:"body"`
		CustomerEmail  string `json:"customer_email" form:"customer_email"`
		CustomerID     string `json:"customer_id" form:"customer_id"`
		CustomerUserID string `json:"customer_user_id" form:"customer_user_id"`
	}

	ctype := strings.ToLower(c.GetHeader("Content-Type"))
	var bindErr error
	switch {
	case strings.Contains(ctype, "application/json"):
		bindErr = c.ShouldBindJSON(&ticketRequest)
	case strings.HasPrefix(ctype, "multipart/form-data"):
		bindErr = c.ShouldBindWith(&ticketRequest, binding.FormMultipart)
	default:
		bindErr = c.Request.ParseForm()
	}
	if bindErr != nil && !errors.Is(bindErr, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid ticket request: " + bindErr.Error(),
		})
		return
	}

	if ticketRequest.Title == "" {
		if subj := c.PostForm("subject"); subj != "" {
			ticketRequest.Title = subj
		}
	}
	if ticketRequest.Body == "" {
		if desc := c.PostForm("description"); desc != "" {
			ticketRequest.Body = desc
		}
	}
	if ticketRequest.QueueID == 0 {
		if qid := c.PostForm("queue_id"); qid != "" {
			if parsed, err := strconv.Atoi(qid); err == nil {
				ticketRequest.QueueID = parsed
			}
		}
	}
	if ticketRequest.PriorityID == 0 {
		if pid := c.PostForm("priority_id"); pid != "" {
			if parsed, err := strconv.Atoi(pid); err == nil {
				ticketRequest.PriorityID = parsed
			}
		}
	}
	if ticketRequest.StateID == 0 {
		if sid := c.PostForm("state_id"); sid != "" {
			if parsed, err := strconv.Atoi(sid); err == nil {
				ticketRequest.StateID = parsed
			}
		}
	}
	if ticketRequest.TypeID == 0 {
		if tid := c.PostForm("type_id"); tid != "" {
			if parsed, err := strconv.Atoi(tid); err == nil {
				ticketRequest.TypeID = parsed
			}
		}
	}
	if ticketRequest.CustomerID == "" {
		ticketRequest.CustomerID = strings.TrimSpace(c.PostForm("customer_id"))
	}
	if ticketRequest.CustomerUserID == "" {
		ticketRequest.CustomerUserID = strings.TrimSpace(c.PostForm("customer_user_id"))
	}

	if ticketRequest.Title == "" || ticketRequest.QueueID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid ticket request: missing title or queue",
		})
		return
	}

	userID := 1
	if uid, exists := c.Get("user_id"); exists {
		if id, ok := uid.(int); ok {
			userID = id
		}
	}

	// Get database connection (required for real creation)
	db, err := database.GetDB()
	if err != nil || db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Database connection failed"})
		return
	}

	repo := repository.NewTicketRepository(db)
	articleRepo := repository.NewArticleRepository(db)
	svc := service.NewTicketService(repo, service.WithArticleRepository(articleRepo))
	visible := true
	created, err := svc.Create(c, service.CreateTicketInput{
		Title:                       ticketRequest.Title,
		QueueID:                     ticketRequest.QueueID,
		PriorityID:                  ticketRequest.PriorityID,
		StateID:                     ticketRequest.StateID,
		UserID:                      userID,
		Body:                        ticketRequest.Body,
		ArticleSubject:              ticketRequest.Title,
		ArticleSenderTypeID:         constants.ArticleSenderAgent,
		ArticleTypeID:               constants.ArticleTypeEmailExternal,
		ArticleIsVisibleForCustomer: &visible,
		TypeID:                      ticketRequest.TypeID,
		CustomerID:                  ticketRequest.CustomerID,
		CustomerUserID:              ticketRequest.CustomerUserID,
	})
	if err != nil {
		if database.IsConnectionError(err) {
			log.Printf("WARN: ticket creation aborted due to database connectivity issue: %v", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Database unavailable"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	// DEBUG: Check for email sending in API handler
	log.Printf("DEBUG: API ticket created - ID=%d, TN=%s, no email sending in this handler", created.ID, created.TicketNumber)

	// Queue email notification if customer email is provided
	if ticketRequest.CustomerEmail != "" {
		customerEmail := ticketRequest.CustomerEmail
		ticketNumber := created.TicketNumber
		ticketTitle := created.Title
		queueID := created.QueueID
		ticketID := created.ID
		log.Printf("DEBUG: API ticket created, queuing email to customerEmail='%s', ticketNumber='%s'", customerEmail, ticketNumber)
		go func() {
			subject := fmt.Sprintf("Ticket Created: %s", ticketNumber)
			body := fmt.Sprintf("Your ticket has been created successfully.\n\nTicket Number: %s\nTitle: %s\n\nYou can view your ticket at: /tickets/%d\n\nBest regards,\nGOTRS Support Team",
				ticketNumber, ticketTitle, ticketID)

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
			renderCtx := notifications.BuildRenderContext(context.Background(), db, ticketRequest.CustomerUserID, userID)
			branding, brandErr := notifications.PrepareQueueEmail(
				context.Background(),
				db,
				queueID,
				body,
				utils.IsHTML(body),
				emailCfg,
				renderCtx,
			)
			if brandErr != nil {
				log.Printf("Queue identity lookup failed for queue %d: %v", queueID, brandErr)
			}

			articleRepo := repository.NewArticleRepository(db)
			latestCustomerArticle, err := articleRepo.GetLatestCustomerArticleForTicket(uint(ticketID))
			inReplyTo := ""
			references := ""
			if err == nil && latestCustomerArticle != nil && latestCustomerArticle.MessageID != "" {
				inReplyTo = latestCustomerArticle.MessageID
				references = latestCustomerArticle.MessageID
				if latestCustomerArticle.References != "" {
					references = latestCustomerArticle.References + " " + latestCustomerArticle.MessageID
				}
			}

			var articleID sql.NullInt64
			if err := db.QueryRow(database.ConvertPlaceholders(`
				SELECT id FROM article WHERE ticket_id = $1 ORDER BY id DESC LIMIT 1
			`), ticketID).Scan(&articleID); err != nil {
				log.Printf("Failed to lookup initial article for ticket %d: %v", ticketID, err)
			}

			senderEmail := branding.EnvelopeFrom
			queueItem := &mailqueue.MailQueueItem{
				Sender:     &senderEmail,
				Recipient:  customerEmail,
				RawMessage: mailqueue.BuildEmailMessageWithThreading(branding.HeaderFrom, customerEmail, subject, branding.Body, branding.Domain, inReplyTo, references),
				Attempts:   0,
				CreateTime: time.Now(),
			}

			if queueErr := queueRepo.Insert(context.Background(), queueItem); queueErr != nil {
				log.Printf("Failed to queue email for %s: %v", customerEmail, queueErr)
			} else {
				log.Printf("Queued email for %s (ticket %s) for processing", customerEmail, ticketNumber)
				messageID := mailqueue.ExtractMessageIDFromRawMessage(queueItem.RawMessage)
				if messageID != "" && articleID.Valid {
					if _, err := db.Exec(database.ConvertPlaceholders(`
						UPDATE article_data_mime
						SET a_message_id = $1, a_in_reply_to = $2, a_references = $3,
						    change_time = CURRENT_TIMESTAMP, change_by = $4
						WHERE article_id = $5
					`), messageID, inReplyTo, references, userID, articleID.Int64); err != nil {
						log.Printf("Failed to store threading headers for article %d: %v", articleID.Int64, err)
					}
				}
			}
		}()
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{"id": created.ID, "tn": created.TicketNumber, "title": created.Title, "queue_id": created.QueueID, "ticket_state_id": created.TicketStateID, "ticket_priority_id": created.TicketPriorityID}})
}
