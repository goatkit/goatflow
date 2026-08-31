package plugin

import (
	"context"

	pkgplugin "github.com/goatkit/goatflow/pkg/plugin"

	"github.com/goatkit/goatflow/internal/platform/ticketstate"
)

// ChangeTicketStatus changes a ticket's state with core semantics (pending
// due-time rule). Delegates to the shared ticketstate package so plugins and
// core share one implementation.
func (h *ProdHostAPI) ChangeTicketStatus(ctx context.Context, ticketID, stateID, userID int64, untilTime int64) error {
	db, err := h.getDB("")
	if err != nil {
		return err
	}
	return ticketstate.ChangeTicketStatus(ctx, db, ticketID, stateID, userID, untilTime)
}

// ListTicketStates returns all valid ticket states with type info.
func (h *ProdHostAPI) ListTicketStates(ctx context.Context) ([]pkgplugin.TicketStateInfo, error) {
	db, err := h.getDB("")
	if err != nil {
		return nil, err
	}
	return ticketstate.ListTicketStates(ctx, db)
}
