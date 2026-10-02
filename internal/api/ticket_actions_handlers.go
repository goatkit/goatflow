package api

// Ticket action handlers (assign, close, reopen, reply, update, delete).
// Split from ticket_htmx_handlers.go for maintainability.

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/history"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/constants"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/lookups"
	"github.com/goatkit/goatflow/internal/platform/routing"
	"github.com/goatkit/goatflow/internal/repository"
)

func init() {
	routing.RegisterHandler("handleAssignTicket", handleAssignTicket)
	routing.RegisterHandler("handleCloseTicket", handleCloseTicket)
	routing.RegisterHandler("handleReopenTicket", handleReopenTicket)
	routing.RegisterHandler("handleTicketReply", handleTicketReply)
	routing.RegisterHandler("handleDeleteTicket", handleDeleteTicket)
	routing.RegisterHandler("handleUpdateTicketPriority", handleUpdateTicketPriority)
	routing.RegisterHandler("handleUpdateTicketQueue", handleUpdateTicketQueue)
	routing.RegisterHandler("handleUpdateTicketStatus", handleUpdateTicketStatus)
	routing.RegisterHandler("handleGetAvailableAgents", handleGetAvailableAgents)
}

// handleAssignTicket assigns a ticket to an agent.
func handleAssignTicket(c *gin.Context) {
	ticketID := c.Param("id")

	// Get agent ID from form data
	userID := c.PostForm("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No agent selected"})
		return
	}

	// Convert userID to int
	agentID, err := strconv.Atoi(userID)
	if err != nil || agentID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	// Convert ticket ID to int
	ticketIDInt, err := strconv.Atoi(ticketID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("handleAssignTicket: database unavailable: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database unavailable"})
		return
	}

	changeByUserID, ok := auditUserID(c)
	if !ok {
		return
	}

	var exists int
	if err := db.QueryRow(database.ConvertPlaceholders(`SELECT 1 FROM ticket WHERE id = ?`), ticketIDInt).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
			return
		}
		log.Printf("handleAssignTicket: lookup ticket %d failed: %v", ticketIDInt, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign ticket"})
		return
	}

	repoPtr := repository.NewTicketRepository(db)
	ticketRepo = repoPtr
	_, updateErr := db.Exec(database.ConvertPlaceholders(`
	            UPDATE ticket
	            SET user_id = ?,
	                responsible_user_id = ?,
	                change_time = NOW(),
	                change_by = ?
	            WHERE id = ?
	        `), agentID, agentID, changeByUserID, ticketIDInt)
	if updateErr != nil {
		log.Printf("handleAssignTicket: update ticket %d failed: %v", ticketIDInt, updateErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign ticket"})
		return
	}

	// Get the agent's name for the response
	var agentName string
	if nameErr := db.QueryRow(database.ConvertPlaceholders(`
            SELECT CONCAT(first_name, ' ', last_name)
            FROM users
            WHERE id = ?
	        `), agentID).Scan(&agentName); nameErr != nil {
		agentName = fmt.Sprintf("Agent %d", agentID)
	}

	if ticket, terr := repoPtr.GetByID(uint(ticketIDInt)); terr == nil {
		recorder := history.NewRecorder(repoPtr)
		msg := fmt.Sprintf("Assigned to %s", agentName)
		if err := recorder.Record(c.Request.Context(), nil, ticket, nil, history.TypeOwnerUpdate, msg, changeByUserID); err != nil {
			log.Printf("history record (assign) failed: %v", err)
		}
	} else {
		log.Printf("history snapshot (assign) failed: %v", terr)
	}

	// HTMX trigger header expected by tests (include showMessage and success)
	c.Header("HX-Trigger", `{"showMessage":{"type":"success","text":"Assigned"},"success":true}`)
	c.JSON(http.StatusOK, gin.H{
		"message":   fmt.Sprintf("Ticket %s assigned to %s", ticketID, agentName),
		"agent_id":  agentID,
		"ticket_id": ticketID,
		"time":      time.Now().Format("2006-01-02 15:04"),
	})
}

// handleCloseTicket closes a ticket.
func handleCloseTicket(c *gin.Context) {
	ticketID := c.Param("id")

	// Parse request body
	var closeData struct {
		StateID              int                    `json:"state_id"`
		Resolution           string                 `json:"resolution"`
		Notes                string                 `json:"notes" binding:"required"`
		TimeUnits            int                    `json:"time_units"`
		NotifyCustomer       bool                   `json:"notify_customer"`
		DynamicFields        map[string]interface{} `json:"dynamic_fields"`
		ArticleDynamicFields map[string]interface{} `json:"article_dynamic_fields"`
	}

	if err := c.ShouldBindJSON(&closeData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}
	ctx := c.Request.Context()
	if closeData.StateID == 0 {
		// Default to closed successful if not specified
		closeData.StateID, err = lookups.ID(ctx, db, lookups.StateLookup, lookups.StateClosedSuccessful)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resolve close state"})
			return
		}
	} else if typeName, terr := lookups.StateTypeNameOfState(ctx, db, closeData.StateID); terr != nil || typeName != lookups.StateTypeClosed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "state_id must be a closed state"})
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

	prevTicket, prevErr := ticketRepo.GetByID(uint(ticketIDInt))
	if prevErr != nil {
		log.Printf("history snapshot (close before) failed: %v", prevErr)
	}

	userID, ok := auditUserID(c)
	if !ok {
		return
	}

	// Update ticket state. Not wrapped in a transaction with the close
	// article: the article repository updates the same ticket row on its own
	// connection, which would wait forever on this transaction's row lock.
	_, err = db.Exec(database.ConvertPlaceholders(`
		UPDATE ticket
		SET ticket_state_id = ?, change_time = NOW(), change_by = ?
		WHERE id = ?
	`), closeData.StateID, userID, ticketIDInt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to close ticket"})
		return
	}

	// Create close article
	var closeArticleID int
	if strings.TrimSpace(closeData.Notes) != "" {
		articleRepo := repository.NewArticleRepository(db)
		closeArticle := &models.Article{
			TicketID:               ticketIDInt,
			Subject:                "Ticket Closed",
			Body:                   closeData.Notes,
			SenderTypeID:           constants.ArticleSenderAgent,
			CommunicationChannelID: constants.CommunicationChannelInternal,
			IsVisibleForCustomer:   0, // Internal by default
			CreateBy:               userID,
			ChangeBy:               userID,
		}
		if closeData.NotifyCustomer {
			closeArticle.IsVisibleForCustomer = 1
		}
		if aerr := articleRepo.Create(closeArticle); aerr != nil {
			log.Printf("WARNING: Failed to create close article: %v", aerr)
		} else {
			closeArticleID = closeArticle.ID
		}
	}

	// Persist time accounting for close operation if provided
	if closeData.TimeUnits > 0 {
		articleIDPtr := &closeArticleID
		if closeArticleID == 0 {
			articleIDPtr = nil
		}
		_ = saveTimeEntry(db, ticketIDInt, articleIDPtr, closeData.TimeUnits, userID) //nolint:errcheck // Best-effort time entry
	}

	// Process dynamic fields from close form (after successful commit)
	if len(closeData.DynamicFields) > 0 {
		// Convert map[string]interface{} to url.Values for ProcessDynamicFieldsFromForm
		formValues := make(map[string][]string)
		for k, v := range closeData.DynamicFields {
			switch val := v.(type) {
			case string:
				formValues[k] = []string{val}
			case []interface{}:
				strVals := make([]string, 0, len(val))
				for _, item := range val {
					if s, ok := item.(string); ok {
						strVals = append(strVals, s)
					}
				}
				formValues[k] = strVals
			case []string:
				formValues[k] = val
			}
		}
		if dfErr := ProcessDynamicFieldsFromForm(formValues, ticketIDInt, DFObjectTicket, "AgentTicketClose"); dfErr != nil {
			log.Printf("WARNING: Failed to process dynamic fields for ticket %d on close: %v", ticketIDInt, dfErr)
		}
	}

	// Process Article dynamic fields for the close article
	if closeArticleID > 0 && len(closeData.ArticleDynamicFields) > 0 {
		articleFormValues := make(map[string][]string)
		for k, v := range closeData.ArticleDynamicFields {
			switch val := v.(type) {
			case string:
				articleFormValues[k] = []string{val}
			case []interface{}:
				strVals := make([]string, 0, len(val))
				for _, item := range val {
					if s, ok := item.(string); ok {
						strVals = append(strVals, s)
					}
				}
				articleFormValues[k] = strVals
			case []string:
				articleFormValues[k] = val
			}
		}
		if dfErr := ProcessArticleDynamicFieldsFromForm(articleFormValues, closeArticleID, "AgentArticleClose"); dfErr != nil {
			log.Printf("WARNING: Failed to process article dynamic fields for article %d on close: %v", closeArticleID, dfErr)
		}
	}

	if updatedTicket, terr := ticketRepo.GetByID(uint(ticketIDInt)); terr == nil {
		recorder := history.NewRecorder(ticketRepo)
		prevStateName := ""
		if prevTicket != nil {
			if st, serr := loadTicketState(ticketRepo, prevTicket.TicketStateID); serr == nil && st != nil {
				prevStateName = st.Name
			} else if serr != nil {
				log.Printf("history close previous state lookup failed: %v", serr)
			} else {
				prevStateName = fmt.Sprintf("state %d", prevTicket.TicketStateID)
			}
		}
		newStateName := fmt.Sprintf("state %d", closeData.StateID)
		if st, serr := loadTicketState(ticketRepo, closeData.StateID); serr == nil && st != nil {
			newStateName = st.Name
		} else if serr != nil {
			log.Printf("history close new state lookup failed: %v", serr)
		}
		stateMsg := history.ChangeMessage("State", prevStateName, newStateName)
		if strings.TrimSpace(stateMsg) == "" {
			stateMsg = fmt.Sprintf("State set to %s", newStateName)
		}
		if err := recorder.Record(c.Request.Context(), nil, updatedTicket, nil, history.TypeStateUpdate, stateMsg, userID); err != nil {
			log.Printf("history record (close state) failed: %v", err)
		}
	} else if terr != nil {
		log.Printf("history snapshot (close after) failed: %v", terr)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"ticketId": ticketIDInt,
		"status":   "closed",
		"stateId":  closeData.StateID,
		"closedAt": time.Now().Format("2006-01-02 15:04"),
	})
}

// handleReopenTicket reopens a ticket.
func handleReopenTicket(c *gin.Context) {
	ticketID := c.Param("id")

	// Parse the request body for additional reopen data
	var reopenData struct {
		StateID        int    `json:"state_id"`
		Reason         string `json:"reason" binding:"required"`
		Notes          string `json:"notes"`
		NotifyCustomer bool   `json:"notify_customer"`
	}

	if err := c.ShouldBindJSON(&reopenData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reopen request: " + err.Error()})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Get ticket to verify it exists
	ticketRepo := repository.NewTicketRepository(db)
	ticketIDInt, err := strconv.Atoi(ticketID)
	if err != nil {
		// Try to get by ticket number
		ticket, err := ticketRepo.GetByTicketNumber(ticketID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
			return
		}
		ticketIDInt = ticket.ID
	}

	prevTicket, prevErr := ticketRepo.GetByID(uint(ticketIDInt))
	if prevErr != nil {
		log.Printf("history snapshot (reopen before) failed: %v", prevErr)
	}

	// Reopen target must be a state of type new or open; default to the 'open' state.
	ctx := c.Request.Context()
	targetStateID := reopenData.StateID
	targetTypeName := lookups.StateTypeOpen
	if targetStateID == 0 {
		targetStateID, err = lookups.ID(ctx, db, lookups.StateLookup, lookups.StateOpen)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resolve open state"})
			return
		}
	} else {
		typeName, terr := lookups.StateTypeNameOfState(ctx, db, targetStateID)
		if terr != nil || (typeName != lookups.StateTypeNew && typeName != lookups.StateTypeOpen) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "state_id must be a new or open state"})
			return
		}
		targetTypeName = typeName
	}

	userID, ok := auditUserID(c)
	if !ok {
		return
	}

	// Update ticket state
	_, err = db.Exec(database.ConvertPlaceholders(`
		UPDATE ticket
		SET ticket_state_id = ?, change_time = NOW(), change_by = ?
		WHERE id = ?
	`), targetStateID, userID, ticketIDInt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reopen ticket"})
		return
	}

	// Add a reopen note/history entry
	reopenNote := fmt.Sprintf("Ticket reopened\nReason: %s", reopenData.Reason)
	if reopenData.Notes != "" {
		reopenNote += fmt.Sprintf("\nAdditional notes: %s", reopenData.Notes)
	}

	// Insert article for reopen note (internal note, channel 3)
	// First insert article record
	articleID, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id,
			is_visible_for_customer, search_index_needs_rebuild, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, 0, 1, NOW(), ?, NOW(), ?) RETURNING id
	`), ticketIDInt, constants.ArticleSenderAgent, constants.CommunicationChannelInternal, userID, userID)

	if err != nil {
		// Log the error but don't fail the reopen operation
		fmt.Printf("Warning: Failed to add reopen article: %v\n", err)
	} else {
		// Insert article_data_mime with the actual content
		if articleID > 0 {
			_, mimeErr := db.Exec(database.ConvertPlaceholders(`
				INSERT INTO article_data_mime (article_id, a_from, a_subject, a_body,
					a_content_type, incoming_time, create_time, create_by, change_time, change_by)
				VALUES (?, 'System', ?, ?, 'text/plain', 0, NOW(), ?, NOW(), ?)
			`), articleID, "Ticket Reopened", reopenNote, userID, userID)
			if mimeErr != nil {
				fmt.Printf("Warning: Failed to add reopen article mime data: %v\n", mimeErr)
			}
		}
	}

	if updatedTicket, terr := ticketRepo.GetByID(uint(ticketIDInt)); terr == nil {
		recorder := history.NewRecorder(ticketRepo)
		prevStateName := ""
		if prevTicket != nil {
			if st, serr := loadTicketState(ticketRepo, prevTicket.TicketStateID); serr == nil && st != nil {
				prevStateName = st.Name
			} else if serr != nil {
				log.Printf("history reopen previous state lookup failed: %v", serr)
			} else {
				prevStateName = fmt.Sprintf("state %d", prevTicket.TicketStateID)
			}
		}
		newStateName := fmt.Sprintf("state %d", targetStateID)
		if st, serr := loadTicketState(ticketRepo, targetStateID); serr == nil && st != nil {
			newStateName = st.Name
		} else if serr != nil {
			log.Printf("history reopen new state lookup failed: %v", serr)
		}
		stateMsg := history.ChangeMessage("State", prevStateName, newStateName)
		if strings.TrimSpace(stateMsg) == "" {
			stateMsg = fmt.Sprintf("State set to %s", newStateName)
		}
		if err := recorder.Record(c.Request.Context(), nil, updatedTicket, nil, history.TypeStateUpdate, stateMsg, userID); err != nil {
			log.Printf("history record (reopen state) failed: %v", err)
		}

		noteExcerpt := history.Excerpt(reopenNote, 160)
		if noteExcerpt != "" {
			msg := fmt.Sprintf("Reopen note — %s", noteExcerpt)
			if err := recorder.Record(c.Request.Context(), nil, updatedTicket, nil, history.TypeAddNote, msg, userID); err != nil {
				log.Printf("history record (reopen note) failed: %v", err)
			}
		}
	} else if terr != nil {
		log.Printf("history snapshot (reopen after) failed: %v", terr)
	}

	// TODO: Implement customer notification if reopenData.NotifyCustomer is true

	statusText := targetTypeName

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"ticketId":   ticketIDInt,
		"status":     statusText,
		"reason":     reopenData.Reason,
		"reopenedAt": time.Now().Format("2006-01-02 15:04"),
	})
}

// handleTicketReply creates a reply or internal note on a ticket and returns HTML.
func handleTicketReply(c *gin.Context) {
	ticketID := c.Param("id")
	replyText := c.PostForm("reply")
	isInternal := c.PostForm("internal") == "true" || c.PostForm("internal") == "1"
	timeUnitsStr := strings.TrimSpace(c.PostForm("time_units"))
	timeUnits := 0
	if timeUnitsStr != "" {
		if v, err := strconv.Atoi(timeUnitsStr); err == nil && v >= 0 {
			timeUnits = v
		}
	}

	if strings.TrimSpace(replyText) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reply text is required"})
		return
	}

	ticketIDInt, err := strconv.Atoi(ticketID)
	if err != nil || ticketIDInt <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("handleTicketReply: database unavailable: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database unavailable"})
		return
	}

	var ticketTitle string
	if err := db.QueryRow(database.ConvertPlaceholders(`SELECT title FROM ticket WHERE id = ?`), ticketIDInt).Scan(&ticketTitle); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
			return
		}
		log.Printf("handleTicketReply: lookup ticket %d failed: %v", ticketIDInt, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load ticket"})
		return
	}

	userID, ok := auditUserID(c)
	if !ok {
		return
	}
	channelID := constants.CommunicationChannelEmail
	visible := 1
	subject := "Re: " + ticketTitle
	if isInternal {
		channelID = constants.CommunicationChannelInternal
		visible = 0
		subject = defaultNoteSubject(channelID)
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("handleTicketReply: begin tx failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save reply"})
		return
	}
	defer func() { _ = tx.Rollback() }()

	articleID, err := insertArticle(tx, ArticleInsertParams{
		TicketID:             int64(ticketIDInt),
		CommunicationChannel: channelID,
		IsVisibleForCustomer: visible,
		CreateBy:             int64(userID),
	})
	if err != nil {
		log.Printf("handleTicketReply: insert article failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save reply"})
		return
	}
	if err := insertArticleMimeData(tx, ArticleMimeParams{
		ArticleID:    articleID,
		From:         "Agent",
		Subject:      subject,
		Body:         replyText,
		ContentType:  "text/plain; charset=utf-8",
		IncomingTime: time.Now().Unix(),
		CreateBy:     int64(userID),
	}); err != nil {
		log.Printf("handleTicketReply: insert article_data_mime failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save reply"})
		return
	}
	if _, err := tx.Exec(database.ConvertPlaceholders(`
		UPDATE ticket SET change_time = CURRENT_TIMESTAMP, change_by = ? WHERE id = ?
	`), userID, ticketIDInt); err != nil {
		log.Printf("handleTicketReply: touch ticket failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save reply"})
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("handleTicketReply: commit failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save reply"})
		return
	}

	badge := ""
	if isInternal {
		badge = `<span class="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium ` +
			`bg-red-100 text-red-800 dark:bg-red-900/20 dark:text-red-200 ml-2">Internal</span>`
	}

	if timeUnits > 0 {
		aid := int(articleID)
		if err := saveTimeEntry(db, ticketIDInt, &aid, timeUnits, userID); err != nil {
			log.Printf("handleTicketReply: save time entry failed: %v", err)
			c.Header("X-Guru-Error", "Failed to save time entry (reply)")
		}
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	// Basic HTML escape for reply content
	safe := strings.ReplaceAll(replyText, "&", "&amp;")
	safe = strings.ReplaceAll(safe, "<", "&lt;")
	safe = strings.ReplaceAll(safe, ">", "&gt;")
	c.String(http.StatusOK, fmt.Sprintf(`
<div class="p-3 border rounded">
  <div class="flex items-center justify-between">
    <div class="font-medium">Reply on Ticket #%s %s</div>
    <div class="text-xs text-gray-500">%s</div>
  </div>
  <div class="mt-2 text-sm">%s</div>
</div>`,
		ticketID,
		badge,
		time.Now().Format("2006-01-02 15:04"),
		safe,
	))
}

// handleDeleteTicket deletes a ticket (soft delete).
func handleDeleteTicket(c *gin.Context) {
	ticketIDStr := c.Param("id")

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// First get the ticket by number to get its ID
	ticketRepo := repository.NewTicketRepository(db)
	ticket, err := ticketRepo.GetByTicketNumber(ticketIDStr)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "Ticket not found",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to retrieve ticket",
			})
		}
		return
	}

	// Soft delete the ticket
	err = ticketRepo.Delete(uint(ticket.ID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to delete ticket",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Ticket %s deleted", ticketIDStr),
	})
}

// handleUpdateTicketPriority updates a ticket priority (HTMX/API helper).
func handleUpdateTicketPriority(c *gin.Context) {
	ticketID := c.Param("id")
	priorityInput := strings.TrimSpace(c.PostForm("priority"))
	if priorityInput == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "priority is required"})
		return
	}

	priorityFields := strings.Fields(priorityInput)
	pid, err := strconv.Atoi(priorityFields[0])
	if err != nil || pid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid priority"})
		return
	}

	actorID, ok := auditUserID(c)
	if !ok {
		return
	}
	userID := uint(actorID)

	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("handleUpdateTicketPriority: database unavailable: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	repo := repository.NewTicketRepository(db)
	tid, _ := strconv.Atoi(ticketID) //nolint:errcheck // Validated above
	if err := repo.UpdatePriority(uint(tid), uint(pid), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update priority"})
		return
	}

	resultPriority := priorityInput
	if len(priorityFields) == 1 {
		resultPriority = strconv.Itoa(pid)
	}

	if ticket, terr := repo.GetByID(uint(tid)); terr == nil {
		recorder := history.NewRecorder(repo)
		msg := fmt.Sprintf("Priority set to %s", strings.TrimSpace(resultPriority))
		if err := recorder.Record(c.Request.Context(), nil, ticket, nil, history.TypePriorityUpdate, msg, int(userID)); err != nil {
			log.Printf("history record (priority) failed: %v", err)
		}
	} else if terr != nil {
		log.Printf("history snapshot (priority) failed: %v", terr)
	}
	c.JSON(http.StatusOK, gin.H{
		"message":     fmt.Sprintf("Ticket %s priority updated", ticketID),
		"priority":    resultPriority,
		"priority_id": pid,
	})
}

// handleUpdateTicketQueue moves a ticket to another queue (HTMX/API helper).
func handleUpdateTicketQueue(c *gin.Context) {
	ticketID := c.Param("id")
	queueIDStr := c.PostForm("queue_id")
	if strings.TrimSpace(queueIDStr) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "queue_id is required"})
		return
	}

	qid, err := strconv.Atoi(queueIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid queue_id"})
		return
	}

	actorID, ok := auditUserID(c)
	if !ok {
		return
	}
	userID := uint(actorID)

	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("handleUpdateTicketQueue: database unavailable: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	// The middleware checked the ticket's current queue; the target queue
	// needs move_into too.
	authz := agentTicketAuthzOrAbort(c, db)
	if authz == nil || !authz.moveTargetOrAbort(c, qid) {
		return
	}

	repo := repository.NewTicketRepository(db)
	tid, _ := strconv.Atoi(ticketID) //nolint:errcheck // Validated above
	if err := repo.UpdateQueue(uint(tid), uint(qid), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to move queue"})
		return
	}

	if ticket, terr := repo.GetByID(uint(tid)); terr == nil {
		recorder := history.NewRecorder(repo)
		msg := fmt.Sprintf("Moved to queue %d", qid)
		if err := recorder.Record(c.Request.Context(), nil, ticket, nil, history.TypeQueueMove, msg, int(userID)); err != nil {
			log.Printf("history record (queue move) failed: %v", err)
		}
	} else if terr != nil {
		log.Printf("history snapshot (queue move) failed: %v", terr)
	}

	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Ticket %s moved to queue %d", ticketID, qid), "queue_id": qid})
}

// handleUpdateTicketStatus updates ticket state (supports pending_until).
func handleUpdateTicketStatus(c *gin.Context) {
	ticketID := c.Param("id")
	status := strings.TrimSpace(c.PostForm("status"))
	pendingUntil := strings.TrimSpace(c.PostForm("pending_until"))
	if status == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status is required"})
		return
	}
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	repo := repository.NewTicketRepository(db)

	resolvedStateID := 0
	var resolvedState *models.TicketState
	if id, st, rerr := resolveTicketState(repo, status, 0); rerr != nil {
		log.Printf("handleUpdateTicketStatus: state resolution error: %v", rerr)
		if id > 0 {
			resolvedStateID = id
			resolvedState = st
		}
	} else if id > 0 {
		resolvedStateID = id
		resolvedState = st
	}
	if resolvedStateID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown status"})
		return
	}
	if resolvedState == nil {
		st, lerr := loadTicketState(repo, resolvedStateID)
		if lerr != nil {
			log.Printf("handleUpdateTicketStatus: load state %d failed: %v", resolvedStateID, lerr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load state"})
			return
		}
		if st == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown status"})
			return
		}
		resolvedState = st
	}

	pendingUnix := 0
	if pendingUntil != "" {
		pendingUnix = parsePendingUntil(pendingUntil)
		if pendingUnix <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid pending time format"})
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

	tid, err := strconv.Atoi(ticketID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ticket id"})
		return
	}
	actorID, ok := auditUserID(c)
	if !ok {
		return
	}
	userID := uint(actorID)

	var previousTicket *models.Ticket
	if prev, perr := repo.GetByID(uint(tid)); perr == nil {
		previousTicket = prev
	} else if perr != nil {
		log.Printf("history snapshot (status before) failed: %v", perr)
	}

	query := database.ConvertPlaceholders(`
		UPDATE ticket
		SET ticket_state_id = ?,
			until_time = ?,
			change_time = CURRENT_TIMESTAMP,
			change_by = ?
		WHERE id = ?`)
	if _, err := db.Exec(query, resolvedStateID, pendingUnix, int(userID), tid); err != nil {
		log.Printf("handleUpdateTicketStatus: failed to update ticket %d: %v", tid, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update status"})
		return
	}

	response := gin.H{
		"message": fmt.Sprintf("Ticket %s status updated", ticketID),
		"status":  resolvedStateID,
	}
	if pendingUnix > 0 {
		response["pending_until"] = pendingUntil
	}

	if updatedTicket, terr := repo.GetByID(uint(tid)); terr == nil {
		recorder := history.NewRecorder(repo)
		prevStateName := ""
		if previousTicket != nil {
			if st, serr := loadTicketState(repo, previousTicket.TicketStateID); serr == nil && st != nil {
				prevStateName = st.Name
			} else if serr != nil {
				log.Printf("history status previous state lookup failed: %v", serr)
			} else {
				prevStateName = fmt.Sprintf("state %d", previousTicket.TicketStateID)
			}
		}
		newStateName := resolvedState.Name
		stateMsg := history.ChangeMessage("State", prevStateName, newStateName)
		if strings.TrimSpace(stateMsg) == "" {
			stateMsg = fmt.Sprintf("State set to %s", newStateName)
		}
		if err := recorder.Record(
			c.Request.Context(), nil, updatedTicket, nil, history.TypeStateUpdate, stateMsg, int(userID)); err != nil {
			log.Printf("history record (state update) failed: %v", err)
		}

		pendingMsg := ""
		if pendingUnix > 0 {
			pendingTime := time.Unix(int64(pendingUnix), 0).In(time.Local).Format("02 Jan 2006 15:04")
			pendingMsg = fmt.Sprintf("Pending until %s", pendingTime)
		} else if previousTicket != nil && previousTicket.UntilTime > 0 {
			pendingMsg = "Pending time cleared"
		}
		if strings.TrimSpace(pendingMsg) != "" {
			if err := recorder.Record(
				c.Request.Context(), nil, updatedTicket, nil, history.TypeSetPendingTime, pendingMsg, int(userID)); err != nil {
				log.Printf("history record (pending time) failed: %v", err)
			}
		}
	} else if terr != nil {
		log.Printf("history snapshot (status after) failed: %v", terr)
	}

	c.JSON(http.StatusOK, response)
}

// handleGetAvailableAgents returns agents who have permissions for the ticket's queue.
func handleGetAvailableAgents(c *gin.Context) {
	ticketID := c.Param("id")

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Convert ticket ID to int
	ticketIDInt, err := strconv.Atoi(ticketID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	// Query to get agents who have permissions for the ticket's queue
	// This joins ticket -> queue -> groups -> group_user -> users
	query := `
		SELECT DISTINCT u.id, u.login, u.first_name, u.last_name
		FROM users u
		INNER JOIN group_user ug ON u.id = ug.user_id
		INNER JOIN queue q ON q.group_id = ug.group_id
		INNER JOIN ticket t ON t.queue_id = q.id
		WHERE t.id = ?
		  AND u.valid_id = 1
		  AND ug.permission_key IN ('rw', 'move_into', 'create', 'owner')
		ORDER BY u.id
	`

	rows, err := db.Query(database.ConvertPlaceholders(query), ticketIDInt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch agents"})
		return
	}
	defer rows.Close()

	agents := []gin.H{}
	for rows.Next() {
		var id int
		var login, firstName, lastName sql.NullString
		err := rows.Scan(&id, &login, &firstName, &lastName)
		if err != nil {
			continue
		}

		agents = append(agents, gin.H{
			"id":    id,
			"name":  fmt.Sprintf("%s %s", firstName.String, lastName.String),
			"login": login,
		})
	}
	if err := rows.Err(); err != nil {
		log.Printf("error iterating agents: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"agents":  agents,
	})
}
