package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/history"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/lookups"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/services"
)

// HandleUpdateTicketAPI handles PUT /api/v1/tickets/:id.
//
//	@Summary		Update ticket
//	@Description	Update an existing ticket's properties
//	@Tags			Tickets
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int		true	"Ticket ID"
//	@Param			ticket	body		object	true	"Ticket update data (title, queue_id, priority_id, state_id, etc.)"
//	@Success		200		{object}	map[string]interface{}	"Updated ticket"
//	@Failure		400		{object}	map[string]interface{}	"Invalid request"
//	@Failure		401		{object}	map[string]interface{}	"Unauthorized"
//	@Failure		404		{object}	map[string]interface{}	"Ticket not found"
//	@Security		BearerAuth
//	@Router			/tickets/{id} [put]
func HandleUpdateTicketAPI(c *gin.Context) {
	// Get ticket ID from URL
	ticketIDStr := c.Param("id")
	ticketID, err := strconv.ParseInt(ticketIDStr, 10, 64)
	if err != nil || ticketID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid ticket ID",
		})
		return
	}

	// Check authentication
	userID, ok := auditUserID(c)
	if !ok {
		return
	}

	// Parse request body
	var updateRequest map[string]interface{}
	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	// Check if there are any fields to update
	if len(updateRequest) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "No fields to update",
		})
		return
	}

	// Get database connection
	db, err := database.GetDB()
	if err != nil || db == nil {
		log.Printf("HandleUpdateTicketAPI: database unavailable: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"error":   "Database connection failed",
		})
		return
	}

	// Check if ticket exists and get current data
	var currentTicket struct {
		ID             int64
		CustomerUserID *string
		UserID         int
	}

	err = db.QueryRow(database.ConvertPlaceholders(
		"SELECT id, customer_user_id, user_id FROM ticket WHERE id = ?",
	), ticketID).Scan(&currentTicket.ID, &currentTicket.CustomerUserID, &currentTicket.UserID)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Ticket not found",
		})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to fetch ticket",
		})
		return
	}

	// Check permissions for customer users
	if isCustomer, _ := c.Get("is_customer"); isCustomer == true { //nolint:errcheck // Defaults to nil
		customerEmail, _ := c.Get("customer_email") //nolint:errcheck // Defaults to nil
		if emailStr, ok := customerEmail.(string); ok {
			if currentTicket.CustomerUserID == nil ||
				*currentTicket.CustomerUserID != emailStr {
				c.JSON(http.StatusForbidden, gin.H{
					"success": false,
					"error":   "Access denied",
				})
				return
			}
		}
	} else {
		// Agent user - check group permissions
		permSvc := services.NewPermissionService(db)

		// Check basic write access (ro users can't update at all)
		canWrite, err := permSvc.CanWriteTicket(userID, ticketID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to check permissions",
			})
			return
		}
		if !canWrite {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   "Write access denied - requires 'rw' permission on queue",
			})
			return
		}

		// Check granular permissions for specific field updates
		// If changing priority, need 'priority' or 'rw' permission
		if _, hasPriority := updateRequest["priority_id"]; hasPriority {
			canPriority, err := permSvc.CanChangePriority(userID, ticketID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to check permissions"})
				return
			}
			if !canPriority {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "No permission to change priority"})
				return
			}
		}

		// If changing owner (user_id), need 'owner' or 'rw' permission
		if _, hasOwner := updateRequest["user_id"]; hasOwner {
			// Get current queue to check owner permission
			var queueID int
			if err := db.QueryRow(database.ConvertPlaceholders("SELECT queue_id FROM ticket WHERE id = ?"), ticketID).Scan(&queueID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Ticket not found"})
					return
				}
				log.Printf("HandleUpdateTicketAPI: load queue of ticket %d: %v", ticketID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to check permissions"})
				return
			}
			canOwn, err := permSvc.CanBeOwner(userID, queueID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to check permissions"})
				return
			}
			if !canOwn {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "No permission to change ticket owner"})
				return
			}
		}
	}

	// Validate fields that reference other tables
	// If changing queue, check move_into permission on the NEW queue
	if queueID, ok := updateRequest["queue_id"].(float64); ok {
		// Check move_into permission on target queue (agents only)
		if isCustomer, _ := c.Get("is_customer"); isCustomer != true {
			permSvc := services.NewPermissionService(db)
			canMove, err := permSvc.CanMoveInto(userID, int(queueID))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to check permissions"})
				return
			}
			if !canMove {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "No permission to move tickets into this queue"})
				return
			}
		}
	}

	ctx := c.Request.Context()
	upd, errMsg, err := parseTicketUpdate(ctx, db, updateRequest)
	if err != nil {
		log.Printf("HandleUpdateTicketAPI: validate: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to validate update"})
		return
	}
	if errMsg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": errMsg})
		return
	}

	repo := repository.NewTicketRepository(db)
	before, err := repo.GetByID(uint(ticketID))
	if err != nil {
		log.Printf("HandleUpdateTicketAPI: load ticket %d: %v", ticketID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch ticket"})
		return
	}

	// The row update and its history entries commit together.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("HandleUpdateTicketAPI: begin: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update ticket"})
		return
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	setClause, args := upd.setClause(userID)
	args = append(args, ticketID)
	updateQuery := fmt.Sprintf("UPDATE ticket SET %s WHERE id = ?", setClause) //nolint:gk-sql-sprintf // hardcoded column fragments; user values bound via ?
	if _, err := tx.ExecContext(ctx, database.ConvertPlaceholders(updateQuery), args...); err != nil {
		log.Printf("HandleUpdateTicketAPI: update ticket %d: %v", ticketID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update ticket"})
		return
	}
	if err := recordTicketUpdateHistory(ctx, db, tx, repo, before, upd, userID); err != nil {
		log.Printf("HandleUpdateTicketAPI: history for ticket %d: %v", ticketID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to record ticket history"})
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("HandleUpdateTicketAPI: commit ticket %d: %v", ticketID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update ticket"})
		return
	}

	after, err := repo.GetByID(uint(ticketID))
	if err != nil {
		log.Printf("HandleUpdateTicketAPI: reload ticket %d: %v", ticketID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Ticket updated but could not be reloaded"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": ticketRecordJSON(after)})
}

// ticketUpdate holds the validated fields of a ticket update request; nil
// means "not changed".
type ticketUpdate struct {
	title                              *string
	queueID, typeID, stateID, priority *int
	ownerID, responsibleID, lockID     *int
	customerID, customerUserID         *string
}

// parseTicketUpdate validates the request fields (types, lengths and that
// referenced rows exist and are valid). A non-empty message is a client error.
func parseTicketUpdate(ctx context.Context, db *sql.DB, req map[string]interface{}) (*ticketUpdate, string, error) {
	upd := &ticketUpdate{}
	intField := func(key string, v interface{}) (*int, string) {
		f, ok := v.(float64)
		if !ok || f < 1 || f != float64(int(f)) {
			return nil, key + " must be a positive integer"
		}
		n := int(f)
		return &n, ""
	}
	strField := func(key string, v interface{}, maxLen int) (*string, string) {
		s, ok := v.(string)
		if !ok {
			return nil, key + " must be a string"
		}
		if utf8.RuneCountInString(s) > maxLen {
			return nil, fmt.Sprintf("%s too long (max %d characters)", key, maxLen)
		}
		return &s, ""
	}
	for key, v := range req {
		var msg string
		switch key {
		case "title":
			if upd.title, msg = strField(key, v, 255); msg == "" && strings.TrimSpace(*upd.title) == "" {
				msg = "Title cannot be empty"
			}
		case "customer_id":
			upd.customerID, msg = strField(key, v, 150)
		case "customer_user_id":
			upd.customerUserID, msg = strField(key, v, 250)
		case "queue_id":
			upd.queueID, msg = intField(key, v)
		case "type_id":
			upd.typeID, msg = intField(key, v)
		case "state_id":
			upd.stateID, msg = intField(key, v)
		case "priority_id":
			upd.priority, msg = intField(key, v)
		case "user_id":
			upd.ownerID, msg = intField(key, v)
		case "responsible_user_id":
			upd.responsibleID, msg = intField(key, v)
		case "ticket_lock_id":
			upd.lockID, msg = intField(key, v)
		default:
			msg = "Unknown field: " + key
		}
		if msg != "" {
			return nil, msg, nil
		}
	}

	refs := []struct {
		id    *int
		query string
		msg   string
	}{
		{upd.queueID, "SELECT EXISTS(SELECT 1 FROM queue WHERE id = ? AND valid_id = 1)", "Invalid queue_id"},
		{upd.stateID, "SELECT EXISTS(SELECT 1 FROM ticket_state WHERE id = ? AND valid_id = 1)", "Invalid state_id"},
		{upd.priority, "SELECT EXISTS(SELECT 1 FROM ticket_priority WHERE id = ? AND valid_id = 1)", "Invalid priority_id"},
		{upd.typeID, "SELECT EXISTS(SELECT 1 FROM ticket_type WHERE id = ? AND valid_id = 1)", "Invalid type_id"},
		{upd.ownerID, "SELECT EXISTS(SELECT 1 FROM users WHERE id = ? AND valid_id = 1)", "Invalid user_id"},
		{upd.responsibleID, "SELECT EXISTS(SELECT 1 FROM users WHERE id = ? AND valid_id = 1)", "Invalid responsible_user_id"},
		{upd.lockID, "SELECT EXISTS(SELECT 1 FROM ticket_lock_type WHERE id = ? AND valid_id = 1)", "Invalid ticket_lock_id"},
	}
	for _, ref := range refs {
		if ref.id == nil {
			continue
		}
		var exists bool
		if err := db.QueryRowContext(ctx, database.ConvertPlaceholders(ref.query), *ref.id).Scan(&exists); err != nil {
			return nil, "", err
		}
		if !exists {
			return nil, ref.msg, nil
		}
	}
	return upd, "", nil
}

// setClause returns the SET list and its arguments, including change_time/by.
func (u *ticketUpdate) setClause(userID int) (string, []interface{}) {
	var cols []string
	var args []interface{}
	addStr := func(col string, v *string) {
		if v != nil {
			cols = append(cols, col+" = ?")
			args = append(args, *v)
		}
	}
	addInt := func(col string, v *int) {
		if v != nil {
			cols = append(cols, col+" = ?")
			args = append(args, *v)
		}
	}
	addStr("title", u.title)
	addInt("queue_id", u.queueID)
	addInt(database.TicketTypeColumn(), u.typeID)
	addInt("ticket_state_id", u.stateID)
	addInt("ticket_priority_id", u.priority)
	addStr("customer_user_id", u.customerUserID)
	addStr("customer_id", u.customerID)
	addInt("user_id", u.ownerID)
	addInt("responsible_user_id", u.responsibleID)
	addInt("ticket_lock_id", u.lockID)
	cols = append(cols, "change_time = CURRENT_TIMESTAMP", "change_by = ?")
	args = append(args, userID)
	return strings.Join(cols, ", "), args
}

// recordTicketUpdateHistory writes one OTRS history entry per changed field
// (TitleUpdate, Move, TypeUpdate, StateUpdate, PriorityUpdate, OwnerUpdate,
// ResponsibleUpdate, CustomerUpdate, Lock/Unlock) inside tx, with the
// ticket snapshot after the update.
func recordTicketUpdateHistory(ctx context.Context, db *sql.DB, tx *sql.Tx, repo *repository.TicketRepository,
	before *models.Ticket, upd *ticketUpdate, userID int) error {
	after := *before
	if upd.title != nil {
		after.Title = *upd.title
	}
	if upd.queueID != nil {
		after.QueueID = *upd.queueID
	}
	if upd.typeID != nil {
		after.TypeID = upd.typeID
	}
	if upd.stateID != nil {
		after.TicketStateID = *upd.stateID
	}
	if upd.priority != nil {
		after.TicketPriorityID = *upd.priority
	}
	if upd.ownerID != nil {
		after.UserID = upd.ownerID
	}
	if upd.responsibleID != nil {
		after.ResponsibleUserID = upd.responsibleID
	}
	if upd.lockID != nil {
		after.TicketLockID = *upd.lockID
	}
	if upd.customerID != nil {
		after.CustomerID = upd.customerID
	}
	if upd.customerUserID != nil {
		after.CustomerUserID = upd.customerUserID
	}

	queueName := func(id int) (string, error) {
		var name string
		err := db.QueryRowContext(ctx, database.ConvertPlaceholders("SELECT name FROM queue WHERE id = ?"), id).Scan(&name)
		return name, err
	}
	userLogin := func(id int) (string, error) {
		var login string
		err := db.QueryRowContext(ctx, database.ConvertPlaceholders("SELECT login FROM users WHERE id = ?"), id).Scan(&login)
		return login, err
	}
	lookupName := func(t lookups.Table) func(int) (string, error) {
		return func(id int) (string, error) { return lookups.Name(ctx, db, t, id) }
	}
	derefInt := func(p *int) int {
		if p == nil {
			return 0
		}
		return *p
	}
	derefStr := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}

	type change struct {
		histType, field string
		oldID, newID    int
		name            func(int) (string, error)
	}
	var entries [][2]string // history type, message
	if after.Title != before.Title {
		entries = append(entries, [2]string{"TitleUpdate", history.ChangeMessage("Title",
			truncateRunes(before.Title, 80), truncateRunes(after.Title, 80))})
	}
	for _, ch := range []change{
		{history.TypeQueueMove, "Queue", before.QueueID, after.QueueID, queueName},
		{"TypeUpdate", "Type", derefInt(before.TypeID), derefInt(after.TypeID), lookupName(lookups.TicketTypeTable)},
		{history.TypeStateUpdate, "State", before.TicketStateID, after.TicketStateID, lookupName(lookups.StateLookup)},
		{history.TypePriorityUpdate, "Priority", before.TicketPriorityID, after.TicketPriorityID, lookupName(lookups.PriorityTable)},
		{history.TypeOwnerUpdate, "Owner", derefInt(before.UserID), derefInt(after.UserID), userLogin},
		{"ResponsibleUpdate", "Responsible", derefInt(before.ResponsibleUserID), derefInt(after.ResponsibleUserID), userLogin},
	} {
		if ch.oldID == ch.newID {
			continue
		}
		newName, err := ch.name(ch.newID)
		if err != nil {
			return fmt.Errorf("%s name of %d: %w", ch.field, ch.newID, err)
		}
		oldName := ""
		if ch.oldID > 0 {
			// The previous row may have been removed since; keep its id.
			if oldName, err = ch.name(ch.oldID); err != nil {
				oldName = fmt.Sprintf("#%d", ch.oldID)
			}
		}
		entries = append(entries, [2]string{ch.histType, history.ChangeMessage(ch.field, oldName, newName)})
	}
	if derefStr(before.CustomerID) != derefStr(after.CustomerID) || derefStr(before.CustomerUserID) != derefStr(after.CustomerUserID) {
		entries = append(entries, [2]string{"CustomerUpdate", truncateRunes(fmt.Sprintf("Customer set to %s / %s",
			derefStr(after.CustomerID), derefStr(after.CustomerUserID)), 200)})
	}
	if after.TicketLockID != before.TicketLockID {
		lockName, err := lookups.Name(ctx, db, lookups.LockType, after.TicketLockID)
		if err != nil {
			return fmt.Errorf("lock name of %d: %w", after.TicketLockID, err)
		}
		histType := "Lock"
		if lockName == lookups.LockUnlock {
			histType = "Unlock"
		}
		entries = append(entries, [2]string{histType, "Lock set to " + lockName})
	}

	recorder := history.NewRecorder(repo)
	for _, e := range entries {
		if err := recorder.Record(ctx, tx, &after, nil, e[0], truncateRunes(e[1], 200), userID); err != nil {
			return fmt.Errorf("%s: %w", e[0], err)
		}
	}
	return nil
}

// truncateRunes shortens s to at most n runes, marking the cut with "...".
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-3]) + "..."
}

// ticketRecordJSON is the API representation of a ticket row (the shape of
// the SDKs' TicketRecord).
func ticketRecordJSON(t *models.Ticket) gin.H {
	str := func(p *string) string {
		if p == nil {
			return ""
		}
		return *p
	}
	num := func(p *int) int {
		if p == nil {
			return 0
		}
		return *p
	}
	return gin.H{
		"id":                  t.ID,
		"tn":                  t.TicketNumber,
		"title":               t.Title,
		"queue_id":            t.QueueID,
		"type_id":             num(t.TypeID),
		"state_id":            t.TicketStateID,
		"priority_id":         t.TicketPriorityID,
		"user_id":             num(t.UserID),
		"responsible_user_id": num(t.ResponsibleUserID),
		"ticket_lock_id":      t.TicketLockID,
		"customer_user_id":    str(t.CustomerUserID),
		"customer_id":         str(t.CustomerID),
		"create_time":         t.CreateTime,
		"create_by":           t.CreateBy,
		"change_time":         t.ChangeTime,
		"change_by":           t.ChangeBy,
	}
}
