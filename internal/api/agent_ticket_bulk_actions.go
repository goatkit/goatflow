package api

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/history"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/config"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/lookups"
	"github.com/goatkit/goatflow/internal/repository"
)

// BulkActionRequest represents a request for bulk ticket operations
type BulkActionRequest struct {
	TicketIDs []int `json:"ticket_ids" form:"ticket_ids"`
}

// BulkStatusRequest represents a bulk status change request
type BulkStatusRequest struct {
	BulkActionRequest
	StatusID     int   `json:"status_id" form:"status_id"`
	PendingUntil int64 `json:"pending_until" form:"pending_until"`
}

// BulkPriorityRequest represents a bulk priority change request
type BulkPriorityRequest struct {
	BulkActionRequest
	PriorityID int `json:"priority_id" form:"priority_id"`
}

// BulkQueueRequest represents a bulk queue move request
type BulkQueueRequest struct {
	BulkActionRequest
	QueueID int `json:"queue_id" form:"queue_id"`
}

// BulkAssignRequest represents a bulk assignment request
type BulkAssignRequest struct {
	BulkActionRequest
	UserID int `json:"user_id" form:"user_id"`
}

// BulkLockRequest represents a bulk lock/unlock request
type BulkLockRequest struct {
	BulkActionRequest
	Lock bool `json:"lock" form:"lock"`
}

// BulkMergeRequest represents a bulk merge request
type BulkMergeRequest struct {
	BulkActionRequest
	TargetTicketID int `json:"target_ticket_id" form:"target_ticket_id"`
}

// BulkActionResult represents the result of a bulk operation
type BulkActionResult struct {
	Success   bool     `json:"success"`
	Total     int      `json:"total"`
	Succeeded int      `json:"succeeded"`
	Failed    int      `json:"failed"`
	Errors    []string `json:"errors,omitempty"`
}

// handleBulkTicketStatus handles bulk status changes for tickets
func handleBulkTicketStatus(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint("user_id")

		var req BulkStatusRequest
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}

		if len(req.TicketIDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No tickets selected"})
			return
		}

		if req.StatusID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status"})
			return
		}

		// Check if status is a pending state
		stateRepo := repository.NewTicketStateRepository(db)
		state, err := stateRepo.GetByID(uint(req.StatusID))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status"})
			return
		}

		pendingUntil := int64(0)
		if isPendingState(state) {
			if req.PendingUntil <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Pending time required for pending states"})
				return
			}
			pendingUntil = req.PendingUntil
		}

		authz := agentTicketAuthzOrAbort(c, db)
		if authz == nil {
			return
		}
		result := BulkActionResult{Total: len(req.TicketIDs)}
		ticketRepo := repository.NewTicketRepository(db)
		recorder := history.NewRecorder(ticketRepo)

		for _, ticketID := range req.TicketIDs {
			if msg := authz.bulkDenial(ticketID, "rw"); msg != "" {
				result.Failed++
				result.Errors = append(result.Errors, msg)
				continue
			}
			// Get previous state for history
			prevTicket, err := ticketRepo.GetByID(uint(ticketID))
			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("Ticket %d: not found", ticketID))
				continue
			}

			// Update ticket status
			_, err = db.Exec(database.ConvertPlaceholders(`
				UPDATE ticket
				SET ticket_state_id = ?, until_time = ?, change_time = CURRENT_TIMESTAMP, change_by = ?
				WHERE id = ?
			`), req.StatusID, pendingUntil, userID, ticketID)

			if err != nil {
				log.Printf("Bulk status update failed for ticket %d: %v", ticketID, err)
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("Ticket %d: update failed", ticketID))
				continue
			}

			result.Succeeded++

			// Record history
			updatedTicket, _ := ticketRepo.GetByID(uint(ticketID))
			if updatedTicket != nil {
				prevStateName := ""
				if prevTicket != nil {
					if st, _ := stateRepo.GetByID(uint(prevTicket.TicketStateID)); st != nil {
						prevStateName = st.Name
					}
				}
				stateMsg := history.ChangeMessage("State", prevStateName, state.Name)
				_ = recorder.Record(c.Request.Context(), nil, updatedTicket, nil,
					history.TypeStateUpdate, stateMsg, int(userID))
			}
		}

		result.Success = result.Failed == 0
		c.JSON(http.StatusOK, result)
	}
}

// handleBulkTicketPriority handles bulk priority changes for tickets
func handleBulkTicketPriority(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint("user_id")

		var req BulkPriorityRequest
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}

		if len(req.TicketIDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No tickets selected"})
			return
		}

		if req.PriorityID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid priority"})
			return
		}

		// Validate priority exists
		priorityRepo := repository.NewTicketPriorityRepository(db)
		priority, err := priorityRepo.GetByID(uint(req.PriorityID))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid priority"})
			return
		}

		authz := agentTicketAuthzOrAbort(c, db)
		if authz == nil {
			return
		}
		result := BulkActionResult{Total: len(req.TicketIDs)}
		ticketRepo := repository.NewTicketRepository(db)
		recorder := history.NewRecorder(ticketRepo)

		for _, ticketID := range req.TicketIDs {
			if msg := authz.bulkDenial(ticketID, "priority"); msg != "" {
				result.Failed++
				result.Errors = append(result.Errors, msg)
				continue
			}
			// Get previous priority for history
			prevTicket, err := ticketRepo.GetByID(uint(ticketID))
			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("Ticket %d: not found", ticketID))
				continue
			}

			// Update ticket priority
			_, err = db.Exec(database.ConvertPlaceholders(`
				UPDATE ticket
				SET ticket_priority_id = ?, change_time = CURRENT_TIMESTAMP, change_by = ?
				WHERE id = ?
			`), req.PriorityID, userID, ticketID)

			if err != nil {
				log.Printf("Bulk priority update failed for ticket %d: %v", ticketID, err)
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("Ticket %d: update failed", ticketID))
				continue
			}

			result.Succeeded++

			// Record history
			updatedTicket, _ := ticketRepo.GetByID(uint(ticketID))
			if updatedTicket != nil {
				prevPriorityName := ""
				if prevTicket != nil {
					if p, _ := priorityRepo.GetByID(uint(prevTicket.TicketPriorityID)); p != nil {
						prevPriorityName = p.Name
					}
				}
				priorityMsg := history.ChangeMessage("Priority", prevPriorityName, priority.Name)
				_ = recorder.Record(c.Request.Context(), nil, updatedTicket, nil,
					history.TypePriorityUpdate, priorityMsg, int(userID))
			}
		}

		result.Success = result.Failed == 0
		c.JSON(http.StatusOK, result)
	}
}

// handleBulkTicketQueue handles bulk queue moves for tickets
func handleBulkTicketQueue(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint("user_id")

		var req BulkQueueRequest
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}

		if len(req.TicketIDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No tickets selected"})
			return
		}

		if req.QueueID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid queue"})
			return
		}

		// Validate queue exists
		var queueName string
		err := db.QueryRow(database.ConvertPlaceholders("SELECT name FROM queue WHERE id = ?"), req.QueueID).Scan(&queueName)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid queue"})
			return
		}

		authz := agentTicketAuthzOrAbort(c, db)
		if authz == nil || !authz.moveTargetOrAbort(c, req.QueueID) {
			return
		}
		result := BulkActionResult{Total: len(req.TicketIDs)}
		ticketRepo := repository.NewTicketRepository(db)
		recorder := history.NewRecorder(ticketRepo)

		for _, ticketID := range req.TicketIDs {
			if msg := authz.bulkDenial(ticketID, "move_into"); msg != "" {
				result.Failed++
				result.Errors = append(result.Errors, msg)
				continue
			}
			// Get previous queue for history
			prevTicket, err := ticketRepo.GetByID(uint(ticketID))
			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("Ticket %d: not found", ticketID))
				continue
			}

			// Update ticket queue
			_, err = db.Exec(database.ConvertPlaceholders(`
				UPDATE ticket
				SET queue_id = ?, change_time = CURRENT_TIMESTAMP, change_by = ?
				WHERE id = ?
			`), req.QueueID, userID, ticketID)

			if err != nil {
				log.Printf("Bulk queue move failed for ticket %d: %v", ticketID, err)
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("Ticket %d: update failed", ticketID))
				continue
			}

			result.Succeeded++

			// Record history
			updatedTicket, _ := ticketRepo.GetByID(uint(ticketID))
			if updatedTicket != nil {
				prevQueueName := ""
				if prevTicket != nil {
					var pq string
					if err := db.QueryRow(database.ConvertPlaceholders("SELECT name FROM queue WHERE id = ?"), prevTicket.QueueID).Scan(&pq); err == nil {
						prevQueueName = pq
					}
				}
				queueMsg := history.ChangeMessage("Queue", prevQueueName, queueName)
				_ = recorder.Record(c.Request.Context(), nil, updatedTicket, nil,
					history.TypeQueueMove, queueMsg, int(userID))
			}
		}

		result.Success = result.Failed == 0
		c.JSON(http.StatusOK, result)
	}
}

// handleBulkTicketAssign handles bulk assignment of tickets to an agent
func handleBulkTicketAssign(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		currentUserID := c.GetUint("user_id")

		var req BulkAssignRequest
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}

		if len(req.TicketIDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No tickets selected"})
			return
		}

		if req.UserID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent"})
			return
		}

		// Validate agent exists
		var agentName string
		err := db.QueryRow(database.ConvertPlaceholders(`
			SELECT CONCAT(first_name, ' ', last_name) FROM users WHERE id = ?
		`), req.UserID).Scan(&agentName)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent"})
			return
		}

		authz := agentTicketAuthzOrAbort(c, db)
		if authz == nil {
			return
		}
		result := BulkActionResult{Total: len(req.TicketIDs)}
		ticketRepo := repository.NewTicketRepository(db)
		recorder := history.NewRecorder(ticketRepo)

		for _, ticketID := range req.TicketIDs {
			if msg := authz.bulkDenial(ticketID, "owner"); msg != "" {
				result.Failed++
				result.Errors = append(result.Errors, msg)
				continue
			}
			// Get previous owner for history
			prevTicket, err := ticketRepo.GetByID(uint(ticketID))
			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("Ticket %d: not found", ticketID))
				continue
			}

			// Update ticket owner and responsible
			_, err = db.Exec(database.ConvertPlaceholders(`
				UPDATE ticket
				SET user_id = ?, responsible_user_id = ?, change_time = CURRENT_TIMESTAMP, change_by = ?
				WHERE id = ?
			`), req.UserID, req.UserID, currentUserID, ticketID)

			if err != nil {
				log.Printf("Bulk assign failed for ticket %d: %v", ticketID, err)
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("Ticket %d: update failed", ticketID))
				continue
			}

			result.Succeeded++

			// Record history
			updatedTicket, _ := ticketRepo.GetByID(uint(ticketID))
			if updatedTicket != nil {
				prevOwnerName := ""
				if prevTicket != nil && prevTicket.UserID != nil && *prevTicket.UserID > 0 {
					var po string
					if err := db.QueryRow(database.ConvertPlaceholders(`
						SELECT CONCAT(first_name, ' ', last_name) FROM users WHERE id = ?
					`), *prevTicket.UserID).Scan(&po); err == nil {
						prevOwnerName = po
					}
				}
				ownerMsg := history.ChangeMessage("Owner", prevOwnerName, agentName)
				_ = recorder.Record(c.Request.Context(), nil, updatedTicket, nil,
					history.TypeOwnerUpdate, ownerMsg, int(currentUserID))
			}
		}

		result.Success = result.Failed == 0
		c.JSON(http.StatusOK, result)
	}
}

// handleBulkTicketLock handles bulk lock/unlock of tickets
func handleBulkTicketLock(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint("user_id")

		var req BulkLockRequest
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}

		if len(req.TicketIDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No tickets selected"})
			return
		}

		lockID := models.TicketUnlocked
		lockAction := "Unlocked"
		if req.Lock {
			lockID = models.TicketLocked
			lockAction = "Locked"
		}

		authz := agentTicketAuthzOrAbort(c, db)
		if authz == nil {
			return
		}
		result := BulkActionResult{Total: len(req.TicketIDs)}
		ticketRepo := repository.NewTicketRepository(db)
		recorder := history.NewRecorder(ticketRepo)

		for _, ticketID := range req.TicketIDs {
			if msg := authz.bulkDenial(ticketID, "rw"); msg != "" {
				result.Failed++
				result.Errors = append(result.Errors, msg)
				continue
			}
			_, err := ticketRepo.GetByID(uint(ticketID))
			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("Ticket %d: not found", ticketID))
				continue
			}

			// Update ticket lock status
			_, err = db.Exec(database.ConvertPlaceholders(`
				UPDATE ticket
				SET ticket_lock_id = ?, change_time = CURRENT_TIMESTAMP, change_by = ?
				WHERE id = ?
			`), lockID, userID, ticketID)

			if err != nil {
				log.Printf("Bulk lock failed for ticket %d: %v", ticketID, err)
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("Ticket %d: update failed", ticketID))
				continue
			}

			result.Succeeded++

			// Record history (using StateUpdate for lock changes)
			updatedTicket, _ := ticketRepo.GetByID(uint(ticketID))
			if updatedTicket != nil {
				lockMsg := fmt.Sprintf("Lock: %s", lockAction)
				_ = recorder.Record(c.Request.Context(), nil, updatedTicket, nil,
					history.TypeStateUpdate, lockMsg, int(userID))
			}
		}

		result.Success = result.Failed == 0
		c.JSON(http.StatusOK, result)
	}
}

// handleBulkTicketMerge handles merging multiple tickets into a target ticket
func handleBulkTicketMerge(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint("user_id")

		var req BulkMergeRequest
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}

		if len(req.TicketIDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No tickets selected"})
			return
		}

		if req.TargetTicketID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid target ticket"})
			return
		}

		// The target gains the sources' articles: it needs rw like the sources.
		authz := agentTicketAuthzOrAbort(c, db)
		if authz == nil || !authz.ticketOrAbort(c, req.TargetTicketID, "rw", "Target ticket") {
			return
		}
		ticketRepo := repository.NewTicketRepository(db)
		targetTicket, err := ticketRepo.GetByID(uint(req.TargetTicketID))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Target ticket not found"})
			return
		}

		// Remove target from source list if present
		sourceIDs := make([]int, 0)
		for _, id := range req.TicketIDs {
			if id != req.TargetTicketID {
				sourceIDs = append(sourceIDs, id)
			}
		}

		if len(sourceIDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No source tickets to merge"})
			return
		}

		result := BulkActionResult{Total: len(sourceIDs)}

		// Start transaction
		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
			return
		}
		defer func() { _ = tx.Rollback() }()

		mergedTicketNumbers := make([]string, 0)

		for _, sourceID := range sourceIDs {
			if msg := authz.bulkDenial(sourceID, "rw"); msg != "" {
				result.Failed++
				result.Errors = append(result.Errors, msg)
				continue
			}
			// Get source ticket info
			sourceTicket, err := ticketRepo.GetByID(uint(sourceID))
			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("Ticket %d: not found", sourceID))
				continue
			}

			// Move all articles from source to target ticket
			_, err = tx.Exec(database.ConvertPlaceholders(`
				UPDATE article
				SET ticket_id = ?, change_time = CURRENT_TIMESTAMP, change_by = ?
				WHERE ticket_id = ?
			`), req.TargetTicketID, userID, sourceID)

			if err != nil {
				log.Printf("Bulk merge articles failed for ticket %d: %v", sourceID, err)
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("Ticket %d: article move failed", sourceID))
				continue
			}

			// Close the source ticket with merged state
			_, err = tx.Exec(database.ConvertPlaceholders(`
				UPDATE ticket
				SET ticket_state_id = (SELECT id FROM ticket_state WHERE name = 'merged'),
					change_time = CURRENT_TIMESTAMP, change_by = ?
				WHERE id = ?
			`), userID, sourceID)

			if err != nil {
				log.Printf("Bulk merge close failed for ticket %d: %v", sourceID, err)
				result.Failed++
				result.Errors = append(result.Errors, fmt.Sprintf("Ticket %d: close failed", sourceID))
				continue
			}

			result.Succeeded++
			mergedTicketNumbers = append(mergedTicketNumbers, sourceTicket.TicketNumber)
		}

		// Commit transaction
		if err = tx.Commit(); err != nil {
			log.Printf("Bulk merge commit failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete merge"})
			return
		}

		// Record merge history on target ticket
		if len(mergedTicketNumbers) > 0 {
			recordBulkMergeHistory(c, db, int(targetTicket.ID), sourceIDs, strings.Join(mergedTicketNumbers, ", "))
		}

		result.Success = result.Failed == 0
		c.JSON(http.StatusOK, gin.H{
			"success":       result.Success,
			"total":         result.Total,
			"succeeded":     result.Succeeded,
			"failed":        result.Failed,
			"errors":        result.Errors,
			"target_ticket": targetTicket.TicketNumber,
		})
	}
}

// recordBulkMergeHistory records the merge action in ticket history for bulk operations.
// It runs after the merge transaction committed, on the same connection pool the merge used;
// failures are logged because the merge itself cannot be undone at this point.
func recordBulkMergeHistory(c *gin.Context, db *sql.DB, targetTicketID int, sourceTicketIDs []int, sourceTicketNumbers string) {
	userID := c.GetUint("user_id")
	ticketRepo := repository.NewTicketRepository(db)
	recorder := history.NewRecorder(ticketRepo)

	targetTicket, err := ticketRepo.GetByID(uint(targetTicketID))
	if err != nil {
		log.Printf("Failed to get target ticket for merge history: %v", err)
		return
	}

	var message string
	if sourceTicketNumbers != "" {
		message = fmt.Sprintf("Merged tickets %s into this ticket", sourceTicketNumbers)
	} else {
		message = fmt.Sprintf("Merged %d tickets into this ticket", len(sourceTicketIDs))
	}

	if err := recorder.Record(c.Request.Context(), nil, targetTicket, nil,
		history.TypeMerged, message, int(userID)); err != nil {
		log.Printf("Failed to record bulk merge history for ticket %d: %v", targetTicketID, err)
	}
}

// handleGetFilteredTicketIds returns all ticket IDs matching the current filter
// This is used for "select all matching" bulk selection functionality
func handleGetFilteredTicketIds(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context
		if _, exists := c.Get("user_id"); !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
			return
		}

		userID := GetUserIDFromCtxUint(c, 0)
		if userID == 0 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID"})
			return
		}

		// Get filter parameters (same as handleAgentTickets)
		status := c.DefaultQuery("status", "not_closed")
		queue := c.DefaultQuery("queue", "all")
		assignee := c.DefaultQuery("assignee", "all")
		search := c.Query("search")

		// Build query - only select ticket IDs
		query := `
			SELECT DISTINCT t.id
			FROM ticket t
			LEFT JOIN customer_user c ON t.customer_user_id = c.login
			LEFT JOIN customer_company cc ON t.customer_id = cc.customer_id
			LEFT JOIN queue q ON t.queue_id = q.id
			LEFT JOIN ticket_state ts ON t.ticket_state_id = ts.id
			WHERE 1=1
		`

		args := []interface{}{}

		// Apply status filter
		if status == "open" {
			query += " AND t.ticket_state_id IN (" + lookups.NewOpenStateIDsSQL + ")"
		} else if status == "pending" {
			query += " AND t.ticket_state_id IN (" + lookups.PendingStateIDsSQL + ")"
		} else if status == "closed" {
			query += " AND t.ticket_state_id IN (" + lookups.ClosedStateIDsSQL + ")"
		} else if status == "not_closed" {
			query += " AND t.ticket_state_id NOT IN (" + lookups.ClosedStateIDsSQL + ")"
		}

		scope, ok := resolveTicketReadScope(c, db, false)
		if !ok {
			return
		}
		if queue != "all" {
			queueID, err := strconv.Atoi(queue)
			if err != nil || !scope.canReadQueue(queueID) {
				c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to access tickets in this queue"})
				return
			}
			query += " AND t.queue_id = ?"
			args = append(args, queueID)
		}
		scopeCond, scopeArgs := scope.filter("t")
		query += " AND " + scopeCond
		args = append(args, scopeArgs...)

		// Apply assignee filter
		if assignee == "me" {
			query += " AND t.responsible_user_id = ?"
			args = append(args, userID)
		} else if assignee == "unassigned" {
			query += " AND t.responsible_user_id IS NULL"
		}

		// Apply search
		if search != "" {
			pattern := "%" + search + "%"
			query += " AND (LOWER(t.tn) LIKE LOWER(?) OR LOWER(t.title) LIKE LOWER(?) OR LOWER(c.login) LIKE LOWER(?))"
			args = append(args, pattern, pattern, pattern)
		}

		// Get max select all limit from config (default 1000)
		maxSelectAll := 1000
		if cfg := config.Get(); cfg != nil && cfg.Ticket.BulkActions.MaxSelectAll > 0 {
			maxSelectAll = cfg.Ticket.BulkActions.MaxSelectAll
		}
		query += fmt.Sprintf(" LIMIT %d", maxSelectAll)

		// Execute query
		rows, err := db.Query(database.ConvertPlaceholders(query), args...)
		if err != nil {
			log.Printf("Error fetching filtered ticket IDs: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tickets"})
			return
		}
		defer rows.Close()

		ticketIDs := make([]int, 0)
		for rows.Next() {
			var id int
			if err := rows.Scan(&id); err == nil {
				ticketIDs = append(ticketIDs, id)
			}
		}

		// Get total count (without limit) for info
		countQuery := strings.Replace(query, "SELECT DISTINCT t.id", "SELECT COUNT(DISTINCT t.id)", 1)
		countQuery = strings.Split(countQuery, "LIMIT")[0] // Remove LIMIT clause

		var totalCount int
		if err := db.QueryRow(database.ConvertPlaceholders(countQuery), args...).Scan(&totalCount); err != nil {
			totalCount = len(ticketIDs)
		}

		c.JSON(http.StatusOK, gin.H{
			"ticket_ids":  ticketIDs,
			"count":       len(ticketIDs),
			"total_count": totalCount,
			"limited":     totalCount > maxSelectAll,
			"max_limit":   maxSelectAll,
		})
	}
}

// handleGetBulkActionOptions returns options for bulk action modals
func handleGetBulkActionOptions(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		actionType := c.Query("type")

		switch actionType {
		case "status":
			// Get all ticket states
			states := make([]map[string]interface{}, 0)
			rows, err := db.Query(database.ConvertPlaceholders(`
				SELECT ts.id, ts.name, tst.name as type_name
				FROM ticket_state ts
				JOIN ticket_state_type tst ON ts.type_id = tst.id
				WHERE ts.valid_id = 1
				ORDER BY ts.name
			`))
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var id int
					var name, typeName string
					if err := rows.Scan(&id, &name, &typeName); err == nil {
						states = append(states, map[string]interface{}{
							"id":        id,
							"name":      name,
							"type":      typeName,
							"isPending": strings.Contains(strings.ToLower(typeName), "pending"),
						})
					}
				}
			}
			c.JSON(http.StatusOK, gin.H{"states": states})

		case "priority":
			// Get all priorities
			priorities := make([]map[string]interface{}, 0)
			rows, err := db.Query(database.ConvertPlaceholders(`
				SELECT id, name FROM ticket_priority WHERE valid_id = 1 ORDER BY id
			`))
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var id int
					var name string
					if err := rows.Scan(&id, &name); err == nil {
						priorities = append(priorities, map[string]interface{}{
							"id":   id,
							"name": name,
						})
					}
				}
			}
			c.JSON(http.StatusOK, gin.H{"priorities": priorities})

		case "queue":
			// Get all queues
			queues := make([]map[string]interface{}, 0)
			rows, err := db.Query(database.ConvertPlaceholders(`
				SELECT id, name FROM queue WHERE valid_id = 1 ORDER BY name
			`))
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var id int
					var name string
					if err := rows.Scan(&id, &name); err == nil {
						queues = append(queues, map[string]interface{}{
							"id":   id,
							"name": name,
						})
					}
				}
			}
			c.JSON(http.StatusOK, gin.H{"queues": queues})

		case "agent":
			// Get all agents
			agents := make([]map[string]interface{}, 0)
			rows, err := db.Query(database.ConvertPlaceholders(`
				SELECT id, login, first_name, last_name
				FROM users
				WHERE valid_id = 1
				ORDER BY last_name, first_name
			`))
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var id int
					var login, firstName, lastName string
					if err := rows.Scan(&id, &login, &firstName, &lastName); err == nil {
						agents = append(agents, map[string]interface{}{
							"id":    id,
							"login": login,
							"name":  strings.TrimSpace(firstName + " " + lastName),
						})
					}
				}
			}
			c.JSON(http.StatusOK, gin.H{"agents": agents})

		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action type"})
		}
	}
}
