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

// ResolveTickets returns every ticket the identifier can name: the ticket
// whose tn equals it (first) and, for a numeric identifier, the ticket with
// that id. Handlers read the path value either way, so the access check must
// cover both when they differ.
func (r *TicketQueueResolverImpl) ResolveTickets(db *sql.DB, ticketIDStr string) ([]middleware.ResolvedTicket, error) {
	var out []middleware.ResolvedTicket
	var t middleware.ResolvedTicket
	err := db.QueryRow(database.ConvertPlaceholders("SELECT id, queue_id FROM ticket WHERE tn = ?"), ticketIDStr).Scan(&t.ID, &t.QueueID)
	switch {
	case err == nil:
		out = append(out, t)
	case err != sql.ErrNoRows:
		return nil, err
	}

	if numericID, parseErr := strconv.ParseUint(ticketIDStr, 10, 64); parseErr == nil && (len(out) == 0 || out[0].ID != numericID) {
		t = middleware.ResolvedTicket{ID: numericID}
		err = db.QueryRow(database.ConvertPlaceholders("SELECT queue_id FROM ticket WHERE id = ?"), numericID).Scan(&t.QueueID)
		switch {
		case err == nil:
			out = append(out, t)
		case err != sql.ErrNoRows:
			return nil, err
		}
	}

	if len(out) == 0 {
		return nil, sql.ErrNoRows
	}
	return out, nil
}

var _ middleware.TicketQueueResolver = (*TicketQueueResolverImpl)(nil)
