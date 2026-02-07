package v1

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/repository"
)

// handleMergeTickets merges child tickets into a parent ticket.
func (router *APIRouter) handleMergeTickets(c *gin.Context) {
	parentID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid ticket ID")
		return
	}

	var req struct {
		ChildTicketIDs   []uint `json:"child_ticket_ids" binding:"required,min=1"`
		Reason           string `json:"reason" binding:"required"`
		Notes            string `json:"notes"`
		MergeMessages    bool   `json:"merge_messages"`
		MergeAttachments bool   `json:"merge_attachments"`
		CloseChildren    bool   `json:"close_children"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	userID := uint(1)
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = uint(idInt)
		}
	}

	ticketRepo := repository.NewTicketRepository(db)

	// Verify parent ticket exists
	parent, err := ticketRepo.GetByID(uint(parentID))
	if err != nil || parent == nil {
		sendError(c, http.StatusNotFound, "Parent ticket not found")
		return
	}

	now := time.Now()
	mergedTickets := []uint{}

	for _, childID := range req.ChildTicketIDs {
		// Skip if trying to merge ticket with itself
		if childID == uint(parentID) {
			continue
		}

		// Verify child ticket exists
		child, err := ticketRepo.GetByID(childID)
		if err != nil || child == nil {
			continue
		}

		// Create merge record in ticket_history
		historyQuery := database.ConvertQuery(`
			INSERT INTO ticket_history
				(ticket_id, article_id, name, history_type_id, create_by, create_time)
			VALUES (?, NULL, ?, (SELECT id FROM ticket_history_type WHERE name = 'Merged' LIMIT 1), ?, ?)
		`)
		historyName := "Merged ticket #" + child.TicketNumber + " into this ticket. Reason: " + req.Reason
		db.Exec(historyQuery, parentID, historyName, userID, now)

		// Also record in child ticket
		childHistoryName := "This ticket was merged into ticket #" + parent.TicketNumber
		db.Exec(historyQuery, childID, childHistoryName, userID, now)

		// Close child ticket if requested
		if req.CloseChildren {
			closeQuery := database.ConvertQuery(`
				UPDATE ticket
				SET ticket_state_id = (SELECT id FROM ticket_state WHERE name = 'merged' OR name = 'closed successfully' LIMIT 1),
					change_time = ?, change_by = ?
				WHERE id = ?
			`)
			db.Exec(closeQuery, now, userID, childID)
		}

		// Link tickets
		linkQuery := database.ConvertQuery(`
			INSERT INTO link_relation
				(source_object_id, source_key, target_object_id, target_key, type_id, state_id, create_time, create_by)
			VALUES 
				((SELECT id FROM link_object WHERE name = 'Ticket'), ?, 
				 (SELECT id FROM link_object WHERE name = 'Ticket'), ?,
				 (SELECT id FROM link_type WHERE name = 'ParentChild' LIMIT 1),
				 (SELECT id FROM link_state WHERE name = 'Valid' LIMIT 1),
				 ?, ?)
		`)
		db.Exec(linkQuery, parentID, childID, now, userID)

		mergedTickets = append(mergedTickets, childID)
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"parent_ticket_id": parentID,
			"merged_tickets":   mergedTickets,
			"merged_count":     len(mergedTickets),
			"reason":           req.Reason,
			"merged_at":        now,
			"merged_by":        userID,
		},
	})
}

// handleSplitTicket creates a new ticket from selected articles of an existing ticket.
func (router *APIRouter) handleSplitTicket(c *gin.Context) {
	sourceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid ticket ID")
		return
	}

	var req struct {
		ArticleIDs      []uint `json:"article_ids" binding:"required,min=1"`
		NewTicketTitle  string `json:"new_ticket_title" binding:"required"`
		NewTicketQueue  uint   `json:"new_ticket_queue" binding:"required"`
		CopyAttachments bool   `json:"copy_attachments"`
		LinkTickets     bool   `json:"link_tickets"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		sendError(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	db, err := database.GetDB()
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Database unavailable")
		return
	}

	userID := uint(1)
	if id, exists := c.Get("user_id"); exists {
		if idInt, ok := id.(int); ok && idInt > 0 {
			userID = uint(idInt)
		}
	}

	ticketRepo := repository.NewTicketRepository(db)

	// Verify source ticket exists
	source, err := ticketRepo.GetByID(uint(sourceID))
	if err != nil || source == nil {
		sendError(c, http.StatusNotFound, "Source ticket not found")
		return
	}

	now := time.Now()

	// Generate ticket number
	var ticketNumber string
	tnQuery := database.ConvertQuery(`SELECT COALESCE(MAX(CAST(tn AS UNSIGNED)), 0) + 1 FROM ticket WHERE tn REGEXP '^[0-9]+$'`)
	db.QueryRow(tnQuery).Scan(&ticketNumber)
	if ticketNumber == "" {
		ticketNumber = now.Format("2006010215040500001")
	}

	// Create new ticket
	createQuery := database.ConvertQuery(`
		INSERT INTO ticket
			(tn, title, queue_id, ticket_priority_id, ticket_state_id, 
			 customer_id, customer_user_id, user_id, responsible_user_id,
			 create_time, create_by, change_time, change_by, timeout, until_time,
			 escalation_time, escalation_update_time, escalation_response_time, escalation_solution_time,
			 archive_flag)
		VALUES (?, ?, ?, ?, 
			(SELECT id FROM ticket_state WHERE name = 'new' LIMIT 1),
			?, ?, ?, ?, ?, ?, ?, ?, 0, 0, 0, 0, 0, 0, 0)
	`)

	result, err := db.Exec(createQuery,
		ticketNumber, req.NewTicketTitle, req.NewTicketQueue, source.TicketPriorityID,
		source.CustomerID, source.CustomerUserID, source.UserID, source.ResponsibleUserID,
		now, userID, now, userID,
	)
	if err != nil {
		sendError(c, http.StatusInternalServerError, "Failed to create new ticket")
		return
	}

	newTicketID, _ := result.LastInsertId()

	// Move selected articles to new ticket
	movedCount := 0
	for _, articleID := range req.ArticleIDs {
		moveQuery := database.ConvertQuery(`
			UPDATE article SET ticket_id = ?, change_time = ?, change_by = ?
			WHERE id = ? AND ticket_id = ?
		`)
		res, err := db.Exec(moveQuery, newTicketID, now, userID, articleID, sourceID)
		if err == nil {
			if affected, _ := res.RowsAffected(); affected > 0 {
				movedCount++
			}
		}
	}

	// Link tickets if requested
	if req.LinkTickets {
		linkQuery := database.ConvertQuery(`
			INSERT INTO link_relation
				(source_object_id, source_key, target_object_id, target_key, type_id, state_id, create_time, create_by)
			VALUES 
				((SELECT id FROM link_object WHERE name = 'Ticket'), ?, 
				 (SELECT id FROM link_object WHERE name = 'Ticket'), ?,
				 (SELECT id FROM link_type WHERE name = 'Normal' LIMIT 1),
				 (SELECT id FROM link_state WHERE name = 'Valid' LIMIT 1),
				 ?, ?)
		`)
		db.Exec(linkQuery, sourceID, newTicketID, now, userID)
	}

	// Record in history
	historyQuery := database.ConvertQuery(`
		INSERT INTO ticket_history
			(ticket_id, article_id, name, history_type_id, create_by, create_time)
		VALUES (?, NULL, ?, (SELECT id FROM ticket_history_type WHERE name = 'Misc' LIMIT 1), ?, ?)
	`)
	db.Exec(historyQuery, sourceID, "Split: Created new ticket #"+ticketNumber+" from this ticket", userID, now)
	db.Exec(historyQuery, newTicketID, "Split: Created from ticket #"+source.TicketNumber, userID, now)

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data: gin.H{
			"source_ticket_id": sourceID,
			"new_ticket_id":    newTicketID,
			"new_ticket_tn":    ticketNumber,
			"articles_moved":   movedCount,
			"created_at":       now,
			"created_by":       userID,
		},
	})
}

// handleGetMergeHistory returns the merge history for a ticket.
func (router *APIRouter) handleGetMergeHistory(c *gin.Context) {
	ticketID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		sendError(c, http.StatusBadRequest, "Invalid ticket ID")
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data:    gin.H{"merges": []interface{}{}, "total": 0},
		})
		return
	}

	// Get merge-related history entries
	query := database.ConvertQuery(`
		SELECT th.id, th.name, th.create_time,
			CONCAT(u.first_name, ' ', u.last_name) as created_by
		FROM ticket_history th
		LEFT JOIN users u ON u.id = th.create_by
		WHERE th.ticket_id = ?
		AND th.name LIKE '%merged%' OR th.name LIKE '%split%'
		ORDER BY th.create_time DESC
	`)

	rows, err := db.Query(query, ticketID)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data:    gin.H{"merges": []interface{}{}, "total": 0},
		})
		return
	}
	defer rows.Close()

	type MergeEntry struct {
		ID        int       `json:"id"`
		Action    string    `json:"action"`
		CreatedAt time.Time `json:"created_at"`
		CreatedBy string    `json:"created_by"`
	}

	merges := []MergeEntry{}
	for rows.Next() {
		var m MergeEntry
		var createdBy *string
		if err := rows.Scan(&m.ID, &m.Action, &m.CreatedAt, &createdBy); err == nil {
			if createdBy != nil {
				m.CreatedBy = *createdBy
			}
			merges = append(merges, m)
		}
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    gin.H{"merges": merges, "total": len(merges)},
	})
}
