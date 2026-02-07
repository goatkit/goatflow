// Package history provides ticket history recording and formatting utilities.
package history

import (
	"context"
	"time"

	"github.com/goatkit/goatflow/internal/models"
)

// HistoryType constants for common ticket history events.
const (
	TypeNewTicket       = "NewTicket"
	TypeOwnerUpdate     = "OwnerUpdate"
	TypeStateUpdate     = "StateUpdate"
	TypeAddNote         = "AddNote"
	TypePriorityUpdate  = "PriorityUpdate"
	TypeQueueMove       = "Move"
	TypeSetPendingTime  = "SetPendingTime"
	TypeMerged          = "Merged"
	TypeTimeAccounting  = "TimeAccounting"
)

// HistoryInserter is an interface for inserting ticket history entries.
type HistoryInserter interface {
	AddTicketHistoryEntry(ctx context.Context, exec interface{}, entry models.TicketHistoryInsert) error
}

// TicketGetter is an interface for getting ticket data needed for history snapshots.
type TicketGetter interface {
	GetByID(id uint) (*models.Ticket, error)
}

// Recorder records ticket history entries.
type Recorder struct {
	repo interface{}
}

// NewRecorder creates a new history recorder with the given repository.
// The repository should implement AddTicketHistoryEntry method.
// Returns nil if repo is nil.
func NewRecorder(repo interface{}) *Recorder {
	if repo == nil {
		return nil
	}
	return &Recorder{repo: repo}
}

// Record records a history entry for a ticket.
// The tx parameter can be nil to use the default database connection.
// ticket can be either a *models.Ticket or an int (ticket ID).
// articleID is optional and can be nil (accepts *int or *uint).
func (r *Recorder) Record(ctx context.Context, tx interface{}, ticket interface{}, articleID interface{}, historyType string, message string, userID int) error {
	if r == nil || r.repo == nil {
		return nil
	}

	inserter, ok := r.repo.(HistoryInserter)
	if !ok {
		return nil
	}

	var ticketData *models.Ticket

	switch t := ticket.(type) {
	case *models.Ticket:
		ticketData = t
	case int:
		// Try to get ticket data from repo if it implements TicketGetter
		if getter, ok := r.repo.(TicketGetter); ok {
			var err error
			ticketData, err = getter.GetByID(uint(t))
			if err != nil {
				// Create minimal ticket data if we can't fetch it
				ticketData = &models.Ticket{ID: t}
			}
		} else {
			ticketData = &models.Ticket{ID: t}
		}
	default:
		return nil
	}

	if ticketData == nil || ticketData.ID <= 0 {
		return nil
	}

	entry := models.TicketHistoryInsert{
		TicketID:    ticketData.ID,
		TypeID:      0,
		QueueID:     ticketData.QueueID,
		OwnerID:     0,
		PriorityID:  ticketData.TicketPriorityID,
		StateID:     ticketData.TicketStateID,
		CreatedBy:   userID,
		HistoryType: historyType,
		Name:        message,
		CreatedAt:   time.Now().UTC(),
	}

	if ticketData.UserID != nil {
		entry.OwnerID = *ticketData.UserID
	}

	if ticketData.TypeID != nil {
		entry.TypeID = *ticketData.TypeID
	}

	// Handle articleID which can be *int or *uint
	switch aid := articleID.(type) {
	case *int:
		if aid != nil && *aid > 0 {
			entry.ArticleID = aid
		}
	case *uint:
		if aid != nil && *aid > 0 {
			v := int(*aid)
			entry.ArticleID = &v
		}
	}

	return inserter.AddTicketHistoryEntry(ctx, tx, entry)
}

// RecordByTicketID records a history entry using a ticket ID directly.
// This is a convenience method when you only have a ticket ID.
func (r *Recorder) RecordByTicketID(ctx context.Context, tx interface{}, ticketID int, articleID interface{}, historyType string, message string, userID int) error {
	return r.Record(ctx, tx, ticketID, articleID, historyType, message, userID)
}

// Excerpt returns a truncated version of the input string, suitable for history messages.
func Excerpt(s string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 50
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ChangeMessage generates a history message for a field change.
func ChangeMessage(field, oldVal, newVal string) string {
	if oldVal == "" {
		return field + " set to " + newVal
	}
	if newVal == "" {
		return field + " cleared (was: " + oldVal + ")"
	}
	return field + " changed from " + oldVal + " to " + newVal
}
