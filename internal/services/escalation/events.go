package escalation

import (
	"context"
	"database/sql"
	"log"
)

// EventHandler handles ticket events that require escalation index updates.
// This matches OTRS Kernel::System::Ticket::Event::TicketEscalationIndex.
type EventHandler struct {
	service *Service
	logger  *log.Logger
}

// NewEventHandler creates a new escalation event handler.
func NewEventHandler(db *sql.DB, logger *log.Logger) *EventHandler {
	if logger == nil {
		logger = log.Default()
	}
	return &EventHandler{
		service: NewService(db, logger),
		logger:  logger,
	}
}

// Initialize loads calendars and prepares the handler.
func (h *EventHandler) Initialize(ctx context.Context) error {
	return h.service.Initialize(ctx)
}

// OnTicketCreate handles ticket creation events.
// Should be called after a new ticket is created.
func (h *EventHandler) OnTicketCreate(ctx context.Context, ticketID, userID int) {
	if err := h.service.TicketEscalationIndexBuild(ctx, ticketID, userID); err != nil {
		h.logger.Printf("escalation: failed to build index for new ticket %d: %v", ticketID, err)
	}
}

// OnTicketStateUpdate handles ticket state change events.
// Should be called when ticket state changes (may clear escalation for closed tickets).
func (h *EventHandler) OnTicketStateUpdate(ctx context.Context, ticketID, userID int) {
	if err := h.service.TicketEscalationIndexBuild(ctx, ticketID, userID); err != nil {
		h.logger.Printf("escalation: failed to rebuild index after state change for ticket %d: %v", ticketID, err)
	}
}

// OnTicketQueueUpdate handles queue change events.
// Should be called when ticket is moved to different queue (escalation settings may differ).
func (h *EventHandler) OnTicketQueueUpdate(ctx context.Context, ticketID, userID int) {
	if err := h.service.TicketEscalationIndexBuild(ctx, ticketID, userID); err != nil {
		h.logger.Printf("escalation: failed to rebuild index after queue change for ticket %d: %v", ticketID, err)
	}
}

// OnTicketSLAUpdate handles SLA change events.
// Should be called when ticket SLA is changed.
func (h *EventHandler) OnTicketSLAUpdate(ctx context.Context, ticketID, userID int) {
	if err := h.service.TicketEscalationIndexBuild(ctx, ticketID, userID); err != nil {
		h.logger.Printf("escalation: failed to rebuild index after SLA change for ticket %d: %v", ticketID, err)
	}
}

// OnArticleCreate handles new article events.
// Should be called when an article is added (may affect update escalation).
func (h *EventHandler) OnArticleCreate(ctx context.Context, ticketID, userID int) {
	if err := h.service.TicketEscalationIndexBuild(ctx, ticketID, userID); err != nil {
		h.logger.Printf("escalation: failed to rebuild index after article create for ticket %d: %v", ticketID, err)
	}
}

// OnTicketServiceUpdate handles service change events.
func (h *EventHandler) OnTicketServiceUpdate(ctx context.Context, ticketID, userID int) {
	if err := h.service.TicketEscalationIndexBuild(ctx, ticketID, userID); err != nil {
		h.logger.Printf("escalation: failed to rebuild index after service change for ticket %d: %v", ticketID, err)
	}
}

// OnTicketMerge handles ticket merge events.
// The merged ticket should have escalation cleared.
func (h *EventHandler) OnTicketMerge(ctx context.Context, ticketID, userID int) {
	if err := h.service.TicketEscalationIndexBuild(ctx, ticketID, userID); err != nil {
		h.logger.Printf("escalation: failed to rebuild index after merge for ticket %d: %v", ticketID, err)
	}
}

// OnTicketUpdate is a generic handler for any ticket update.
// Use this when you're not sure which specific event occurred.
func (h *EventHandler) OnTicketUpdate(ctx context.Context, ticketID, userID int) {
	if err := h.service.TicketEscalationIndexBuild(ctx, ticketID, userID); err != nil {
		h.logger.Printf("escalation: failed to rebuild index for ticket %d: %v", ticketID, err)
	}
}

// Global singleton for easy access from handlers
var globalEventHandler *EventHandler

// InitGlobalEventHandler initializes the global event handler.
// Should be called during application startup.
func InitGlobalEventHandler(db *sql.DB, logger *log.Logger) error {
	globalEventHandler = NewEventHandler(db, logger)
	return globalEventHandler.Initialize(context.Background())
}

// GetEventHandler returns the global event handler.
// Returns nil if not initialized.
func GetEventHandler() *EventHandler {
	return globalEventHandler
}

// TriggerTicketCreate triggers escalation index build for a new ticket.
// Safe to call even if handler not initialized (will be a no-op).
func TriggerTicketCreate(ctx context.Context, ticketID, userID int) {
	if h := GetEventHandler(); h != nil {
		h.OnTicketCreate(ctx, ticketID, userID)
	}
}

// TriggerTicketUpdate triggers escalation index rebuild for an updated ticket.
// Safe to call even if handler not initialized (will be a no-op).
func TriggerTicketUpdate(ctx context.Context, ticketID, userID int) {
	if h := GetEventHandler(); h != nil {
		h.OnTicketUpdate(ctx, ticketID, userID)
	}
}

// TriggerArticleCreate triggers escalation index rebuild after article creation.
// Safe to call even if handler not initialized (will be a no-op).
func TriggerArticleCreate(ctx context.Context, ticketID, userID int) {
	if h := GetEventHandler(); h != nil {
		h.OnArticleCreate(ctx, ticketID, userID)
	}
}
