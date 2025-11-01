package history

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

const (
	// History type names aligned with OTRS/Znuny expectations.
	TypeNewTicket         = "NewTicket"
	TypeAddNote           = "AddNote"
	TypeTimeAccounting    = "TimeAccounting"
	TypePriorityUpdate    = "PriorityUpdate"
	TypeQueueMove         = "Move"
	TypeStateUpdate       = "StateUpdate"
	TypeSetPendingTime    = "SetPendingTime"
	TypeOwnerUpdate       = "OwnerUpdate"
	TypeResponsibleUpdate = "ResponsibleUpdate"
	TypeMerged            = "Merged"
)

const maxHistoryNameLength = 200

// Recorder coordinates writing ticket history entries while keeping updates DRY.
type Recorder struct {
	repo *repository.TicketRepository
}

// NewRecorder constructs a recorder bound to the provided repository instance.
func NewRecorder(repo *repository.TicketRepository) *Recorder {
	return &Recorder{repo: repo}
}

// ChangeMessage generates a consistent message for value transitions.
func ChangeMessage(field, from, to string) string {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)

	switch {
	case from == "" && to == "":
		return ""
	case from == "":
		return fmt.Sprintf("%s set to %s", field, to)
	case to == "":
		return fmt.Sprintf("%s cleared (was %s)", field, from)
	case strings.EqualFold(from, to):
		return fmt.Sprintf("%s remains %s", field, to)
	default:
		return fmt.Sprintf("%s changed from %s to %s", field, from, to)
	}
}

// SetMessage produces a simple "field set to" entry.
func SetMessage(field, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Sprintf("%s cleared", field)
	}
	return fmt.Sprintf("%s set to %s", field, value)
}

// Excerpt returns a shortened snippet suitable for history payloads.
func Excerpt(body string, limit int) string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return ""
	}
	if limit <= 0 {
		limit = maxHistoryNameLength
	}
	if utf8.RuneCountInString(trimmed) <= limit {
		return trimmed
	}
	runes := []rune(trimmed)
	if limit >= len(runes) {
		return trimmed
	}
	excerpt := strings.TrimSpace(string(runes[:limit]))
	if excerpt == "" {
		return trimmed
	}
	return excerpt + "…"
}

// Record persists a ticket_history entry using the provided ticket snapshot.
func (r *Recorder) Record(ctx context.Context, exec repository.ExecContext, ticket *models.Ticket, articleID *int, historyType, message string, actorID int) error {
	if r == nil || r.repo == nil {
		return errors.New("history recorder not initialized")
	}
	if ticket == nil {
		return errors.New("ticket snapshot required")
	}
	historyType = strings.TrimSpace(historyType)
	if historyType == "" {
		return errors.New("history type required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	entry := models.TicketHistoryInsert{
		TicketID:    ticket.ID,
		ArticleID:   articleID,
		TypeID:      valueFromPtrOrDefault(ticket.TypeID, 1), // Default to Unclassified (1) if NULL
		QueueID:     ticket.QueueID,
		OwnerID:     valueFromPtr(ticket.UserID),
		PriorityID:  ticket.TicketPriorityID,
		StateID:     ticket.TicketStateID,
		CreatedBy:   actorID,
		HistoryType: historyType,
		Name:        truncate(message, maxHistoryNameLength),
		CreatedAt:   ticket.ChangeTime,
	}

	return r.repo.AddTicketHistoryEntry(ctx, exec, entry)
}

// RecordByTicketID loads a ticket snapshot and records the history entry in one step.
func (r *Recorder) RecordByTicketID(ctx context.Context, exec repository.ExecContext, ticketID int, articleID *int, historyType, message string, actorID int) error {
	if r == nil || r.repo == nil {
		return errors.New("history recorder not initialized")
	}
	if ticketID <= 0 {
		return errors.New("ticket id required")
	}
	ticket, err := r.repo.GetByID(uint(ticketID))
	if err != nil {
		return err
	}
	return r.Record(ctx, exec, ticket, articleID, historyType, message, actorID)
}

func truncate(input string, max int) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" || max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(trimmed) <= max {
		return trimmed
	}
	runes := []rune(trimmed)
	if max > len(runes) {
		max = len(runes)
	}
	return strings.TrimSpace(string(runes[:max-1])) + "…"
}

func valueFromPtr(ptr *int) int {
	if ptr == nil {
		return 0
	}
	return *ptr
}

func valueFromPtrOrDefault(ptr *int, defaultValue int) int {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}
