package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/service"
)

// agentTicketAuthz checks the calling agent's queue permissions on tickets and
// queues named in a request body or form. The route middleware only checks the
// ticket in the path (ticket_access_*) or that the agent holds a permission on
// some queue (queue_*), so handlers that act on other tickets or move tickets
// into another queue check each of them here. Admins pass every check.
type agentTicketAuthz struct {
	ctx     context.Context
	db      *sql.DB
	qa      *service.QueueAccessService
	userID  uint
	isAdmin bool
}

var errNoAgent = errors.New("no authenticated agent")

func newAgentTicketAuthz(c *gin.Context, db *sql.DB) (*agentTicketAuthz, error) {
	a := &agentTicketAuthz{
		ctx:    c.Request.Context(),
		db:     db,
		qa:     service.NewQueueAccessService(db),
		userID: GetUserIDFromCtxUint(c, 0),
	}
	if a.userID == 0 {
		return nil, errNoAgent
	}
	isAdmin, err := a.qa.IsAdmin(a.ctx, a.userID)
	if err != nil {
		return nil, err
	}
	a.isAdmin = isAdmin
	return a, nil
}

// queue reports whether the agent holds perm on queueID.
func (a *agentTicketAuthz) queue(queueID uint, perm string) (bool, error) {
	if a.isAdmin {
		return true, nil
	}
	return a.qa.HasQueueAccess(a.ctx, a.userID, queueID, perm)
}

// ticket returns 0 when the agent holds perm on the queue of ticketID,
// http.StatusNotFound when the ticket does not exist or the agent may not read
// it (its existence is not revealed), and http.StatusForbidden when the agent
// may read it but lacks perm.
func (a *agentTicketAuthz) ticket(ticketID int, perm string) (int, error) {
	var queueID uint
	err := a.db.QueryRowContext(a.ctx, database.ConvertPlaceholders(
		`SELECT queue_id FROM ticket WHERE id = ?`), ticketID).Scan(&queueID)
	if err == sql.ErrNoRows {
		return http.StatusNotFound, nil
	}
	if err != nil {
		return 0, err
	}
	ok, err := a.queue(queueID, perm)
	if err != nil || ok {
		return 0, err
	}
	if perm != "ro" {
		canRead, err := a.queue(queueID, "ro")
		if err != nil {
			return 0, err
		}
		if canRead {
			return http.StatusForbidden, nil
		}
	}
	return http.StatusNotFound, nil
}

// agentTicketAuthzOrAbort is newAgentTicketAuthz for handlers: on failure it
// writes 401 (no agent in the context) or 500 and returns nil.
func agentTicketAuthzOrAbort(c *gin.Context, db *sql.DB) *agentTicketAuthz {
	a, err := newAgentTicketAuthz(c, db)
	if errors.Is(err, errNoAgent) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return nil
	}
	if err != nil {
		log.Printf("ticket permission check: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
		return nil
	}
	return a
}

// bulkDenial returns "" when the agent holds perm on ticketID, else the
// per-ticket error a bulk action reports. Tickets the agent may not read are
// reported as not found.
func (a *agentTicketAuthz) bulkDenial(ticketID int, perm string) string {
	status, err := a.ticket(ticketID, perm)
	switch {
	case err != nil:
		log.Printf("bulk permission check for ticket %d: %v", ticketID, err)
		return fmt.Sprintf("Ticket %d: permission check failed", ticketID)
	case status == http.StatusForbidden:
		return fmt.Sprintf("Ticket %d: permission denied", ticketID)
	case status == http.StatusNotFound:
		return fmt.Sprintf("Ticket %d: not found", ticketID)
	}
	return ""
}

// ticketOrAbort checks perm on ticketID and on refusal writes 404 (missing or
// unreadable), 403 (readable, perm missing) or 500 and returns false. what
// names the ticket in the error ("Target ticket").
func (a *agentTicketAuthz) ticketOrAbort(c *gin.Context, ticketID int, perm, what string) bool {
	status, err := a.ticket(ticketID, perm)
	switch {
	case err != nil:
		log.Printf("permission check for ticket %d: %v", ticketID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
	case status == http.StatusForbidden:
		c.JSON(http.StatusForbidden, gin.H{"error": what + ": permission denied"})
	case status == http.StatusNotFound:
		c.JSON(http.StatusNotFound, gin.H{"error": what + " not found"})
	default:
		return true
	}
	return false
}

// moveTargetOrAbort checks that queueID is a valid queue the agent may move
// tickets into (move_into). On refusal it writes 400 (no such valid queue),
// 403 or 500 and returns false.
func (a *agentTicketAuthz) moveTargetOrAbort(c *gin.Context, queueID int) bool {
	var valid int
	err := a.db.QueryRowContext(a.ctx, database.ConvertPlaceholders(
		`SELECT COUNT(*) FROM queue WHERE id = ? AND valid_id = 1`), queueID).Scan(&valid)
	if err != nil {
		log.Printf("move target queue %d lookup: %v", queueID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
		return false
	}
	if valid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid queue"})
		return false
	}
	ok, err := a.queue(uint(queueID), "move_into")
	if err != nil {
		log.Printf("move_into check on queue %d: %v", queueID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
		return false
	}
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to move tickets into this queue"})
		return false
	}
	return true
}

// createQueueOrAbort checks that the agent may create tickets in queueID (the
// queue the handler actually uses, which need not be the queue_id the route
// middleware read). On refusal it writes 403 or 500 and returns false.
func (a *agentTicketAuthz) createQueueOrAbort(c *gin.Context, queueID int) bool {
	ok, err := a.queue(uint(queueID), "create")
	if err != nil {
		log.Printf("create check on queue %d: %v", queueID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
		return false
	}
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "No permission to create tickets in this queue"})
		return false
	}
	return true
}
