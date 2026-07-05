package repository

import (
	"database/sql"
	"strconv"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/middleware"
)

// TicketQueueResolverImpl resolves ticket identifiers to queue IDs.
// Implements middleware.TicketQueueResolver.
type TicketQueueResolverImpl struct{}

func (r *TicketQueueResolverImpl) ResolveQueueID(db *sql.DB, ticketIDStr string) (queueID uint, ticketID uint64, err error) {
	query := database.ConvertPlaceholders("SELECT id, queue_id FROM ticket WHERE tn = ?")
	err = db.QueryRow(query, ticketIDStr).Scan(&ticketID, &queueID)

	if err != nil {
		numericID, parseErr := strconv.ParseUint(ticketIDStr, 10, 64)
		if parseErr == nil {
			ticketID = numericID
			query = database.ConvertPlaceholders("SELECT queue_id FROM ticket WHERE id = ?")
			err = db.QueryRow(query, ticketID).Scan(&queueID)
		}
	}
	return
}

var _ middleware.TicketQueueResolver = (*TicketQueueResolverImpl)(nil)
