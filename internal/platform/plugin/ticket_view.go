package plugin

import (
	"context"
)

// ListTicketViews returns the ticket-view URL templates declared by enabled
// plugins' enabled UIs.
func (h *ProdHostAPI) ListTicketViews(ctx context.Context) ([]TicketViewInfo, error) {
	if h.PluginManager == nil {
		return nil, nil
	}
	return h.PluginManager.ListTicketViews(ctx)
}
