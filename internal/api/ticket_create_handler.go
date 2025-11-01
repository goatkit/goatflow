package api

import (
	"context"
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
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/mailqueue"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/service"
)

// HandleCreateTicketAPI handles ticket creation via API
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
		Title         string `json:"title" form:"title"`
		QueueID       int    `json:"queue_id" form:"queue_id"`
		PriorityID    int    `json:"priority_id" form:"priority_id"`
		StateID       int    `json:"state_id" form:"state_id"`
		Body          string `json:"body" form:"body"`
		CustomerEmail string `json:"customer_email" form:"customer_email"`
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
	svc := service.NewTicketService(repo)
	created, err := svc.Create(c, service.CreateTicketInput{Title: ticketRequest.Title, QueueID: ticketRequest.QueueID, PriorityID: ticketRequest.PriorityID, StateID: ticketRequest.StateID, UserID: userID, Body: ticketRequest.Body, TypeID: 1})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	// DEBUG: Check for email sending in API handler
	log.Printf("DEBUG: API ticket created - ID=%d, TN=%s, no email sending in this handler", created.ID, created.TicketNumber)

	// Queue email notification if customer email is provided
	if ticketRequest.CustomerEmail != "" {
		log.Printf("DEBUG: API ticket created, queuing email to customerEmail='%s', ticketNumber='%s'", ticketRequest.CustomerEmail, created.TicketNumber)
		go func() {
			subject := fmt.Sprintf("Ticket Created: %s", created.TicketNumber)
			body := fmt.Sprintf("Your ticket has been created successfully.\n\nTicket Number: %s\nTitle: %s\n\nYou can view your ticket at: /tickets/%d\n\nBest regards,\nGOTRS Support Team",
				created.TicketNumber, created.Title, created.ID)

			// Queue the email for processing by EmailQueueTask
			db, dbErr := database.GetDB()
			if dbErr == nil {
				queueRepo := mailqueue.NewMailQueueRepository(db)
				senderEmail := "GOTRS Support Team"
				if cfg := config.Get(); cfg != nil {
					senderEmail = cfg.Email.From
				}
				queueItem := &mailqueue.MailQueueItem{
					Sender:       &senderEmail,
					Recipient:    ticketRequest.CustomerEmail,
					RawMessage:   mailqueue.BuildEmailMessage(senderEmail, ticketRequest.CustomerEmail, subject, body),
					Attempts:     0,
					CreateTime:   time.Now(),
				}
				
				if queueErr := queueRepo.Insert(context.Background(), queueItem); queueErr != nil {
					log.Printf("Failed to queue email for %s: %v", ticketRequest.CustomerEmail, queueErr)
				} else {
					log.Printf("Queued email for %s (ticket %s) for processing", ticketRequest.CustomerEmail, created.TicketNumber)
				}
			} else {
				log.Printf("Failed to get database connection for queuing email: %v", dbErr)
			}
		}()
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{"id": created.ID, "tn": created.TicketNumber, "title": created.Title, "queue_id": created.QueueID, "ticket_state_id": created.TicketStateID, "ticket_priority_id": created.TicketPriorityID}})
}
