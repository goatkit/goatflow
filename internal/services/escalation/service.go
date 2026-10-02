package escalation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/lookups"
	"github.com/goatkit/goatflow/internal/repository"
)

// Escalation kinds, in the order OTRS evaluates them.
const (
	FirstResponse = "FirstResponse"
	Update        = "Update"
	Solution      = "Solution"
)

// Kinds lists the three escalation kinds.
var Kinds = []string{FirstResponse, Update, Solution}

// History types (ticket_history_type, seeded by migration 000029) of the OTRS
// escalation events.
var (
	startEvent = map[string]string{
		FirstResponse: "EscalationResponseTimeStart",
		Update:        "EscalationUpdateTimeStart",
		Solution:      "EscalationSolutionTimeStart",
	}
	notifyBeforeEvent = map[string]string{
		FirstResponse: "EscalationResponseTimeNotifyBefore",
		Update:        "EscalationUpdateTimeNotifyBefore",
		Solution:      "EscalationSolutionTimeNotifyBefore",
	}
	stopEvent = map[string]string{
		FirstResponse: "EscalationResponseTimeStop",
		Update:        "EscalationUpdateTimeStop",
		Solution:      "EscalationSolutionTimeStop",
	}
)

// Service computes the OTRS escalation index of tickets
// (Kernel::System::Ticket::TicketEscalationIndexBuild) and raises the
// escalation events (Maint::Ticket::EscalationCheck and
// Ticket::Event::TriggerEscalationStopEvents).
type Service struct {
	db       *sql.DB
	location *time.Location
	now      func() time.Time
	logger   *log.Logger
	history  *repository.TicketRepository

	mu        sync.RWMutex
	calendars *Calendars
}

// Option configures a Service.
type Option func(*Service)

// WithLocation sets the time zone of the default calendar (and of numbered
// calendars without TimeZone::CalendarN). Default UTC.
func WithLocation(loc *time.Location) Option {
	return func(s *Service) {
		if loc != nil {
			s.location = loc
		}
	}
}

// WithClock replaces time.Now.
func WithClock(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}

// WithLogger sets the logger.
func WithLogger(l *log.Logger) Option {
	return func(s *Service) {
		if l != nil {
			s.logger = l
		}
	}
}

// NewService returns a service on db. Calendars are loaded on first use and
// by LoadCalendars.
func NewService(db *sql.DB, opts ...Option) *Service {
	s := &Service{
		db:       db,
		location: time.UTC,
		now:      time.Now,
		logger:   log.Default(),
		history:  repository.NewTicketRepository(db),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// LoadCalendars (re)reads the calendar settings.
func (s *Service) LoadCalendars(ctx context.Context) error {
	cs, err := LoadCalendars(ctx, s.db, s.location)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.calendars = cs
	s.mu.Unlock()
	return nil
}

func (s *Service) calendar(ctx context.Context, name string) (*Calendar, error) {
	s.mu.RLock()
	cs := s.calendars
	s.mu.RUnlock()
	if cs == nil {
		if err := s.LoadCalendars(ctx); err != nil {
			return nil, err
		}
		s.mu.RLock()
		cs = s.calendars
		s.mu.RUnlock()
	}
	return cs.Get(name), nil
}

// Preferences are the escalation settings of a ticket: its SLA's when it has
// one, else its queue's (OTRS TicketEscalationPreferences). Times are in
// minutes, notify values in percent.
type Preferences struct {
	FirstResponseTime   int
	FirstResponseNotify int
	UpdateTime          int
	UpdateNotify        int
	SolutionTime        int
	SolutionNotify      int
	Calendar            string
}

func (p Preferences) timeOf(kind string) (minutes, notify int) {
	switch kind {
	case FirstResponse:
		return p.FirstResponseTime, p.FirstResponseNotify
	case Update:
		return p.UpdateTime, p.UpdateNotify
	default:
		return p.SolutionTime, p.SolutionNotify
	}
}

func (p Preferences) any() bool {
	return p.FirstResponseTime > 0 || p.UpdateTime > 0 || p.SolutionTime > 0
}

// Index is a ticket's escalation index: OTRS destination times in unix
// seconds, 0 where the kind does not escalate.
type Index struct {
	Escalation int64 // earliest of the three
	Response   int64
	Update     int64
	Solution   int64
}

// Of returns the destination time of one kind.
func (ix Index) Of(kind string) int64 {
	switch kind {
	case FirstResponse:
		return ix.Response
	case Update:
		return ix.Update
	default:
		return ix.Solution
	}
}

// ticket is the ticket data the escalation code reads.
type ticket struct {
	id         int
	queueID    int
	slaID      sql.NullInt64
	stateType  string
	created    time.Time
	index      Index
	typeID     sql.NullInt64
	ownerID    int
	priorityID int
	stateID    int
}

func (s *Service) loadTicket(ctx context.Context, q queryer, ticketID int) (*ticket, error) {
	var t ticket
	err := q.QueryRowContext(ctx, database.ConvertPlaceholders(`
		SELECT t.id, t.queue_id, t.sla_id, tst.name, t.create_time,
		       t.escalation_time, t.escalation_response_time, t.escalation_update_time, t.escalation_solution_time,
		       t.type_id, t.user_id, t.ticket_priority_id, t.ticket_state_id
		FROM ticket t
		JOIN ticket_state ts ON ts.id = t.ticket_state_id
		JOIN ticket_state_type tst ON tst.id = ts.type_id
		WHERE t.id = ?`), ticketID).Scan(
		&t.id, &t.queueID, &t.slaID, &t.stateType, &t.created,
		&t.index.Escalation, &t.index.Response, &t.index.Update, &t.index.Solution,
		&t.typeID, &t.ownerID, &t.priorityID, &t.stateID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load ticket %d: %w", ticketID, err)
	}
	t.stateType = strings.ToLower(t.stateType)
	return &t, nil
}

type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// finished reports whether the state type ends escalation (OTRS: merge|close|remove).
func finished(stateType string) bool {
	return strings.HasPrefix(stateType, "merge") || strings.HasPrefix(stateType, "close") ||
		strings.HasPrefix(stateType, "remove")
}

func (s *Service) preferences(ctx context.Context, t *ticket) (Preferences, error) {
	var p Preferences
	var err error
	if t.slaID.Valid && t.slaID.Int64 > 0 {
		err = s.db.QueryRowContext(ctx, database.ConvertPlaceholders(`
			SELECT first_response_time, COALESCE(first_response_notify, 0), update_time, COALESCE(update_notify, 0),
			       solution_time, COALESCE(solution_notify, 0), COALESCE(calendar_name, '')
			FROM sla WHERE id = ?`), t.slaID.Int64).Scan(
			&p.FirstResponseTime, &p.FirstResponseNotify, &p.UpdateTime, &p.UpdateNotify,
			&p.SolutionTime, &p.SolutionNotify, &p.Calendar)
	} else {
		err = s.db.QueryRowContext(ctx, database.ConvertPlaceholders(`
			SELECT COALESCE(first_response_time, 0), COALESCE(first_response_notify, 0),
			       COALESCE(update_time, 0), COALESCE(update_notify, 0),
			       COALESCE(solution_time, 0), COALESCE(solution_notify, 0), COALESCE(calendar_name, '')
			FROM queue WHERE id = ?`), t.queueID).Scan(
			&p.FirstResponseTime, &p.FirstResponseNotify, &p.UpdateTime, &p.UpdateNotify,
			&p.SolutionTime, &p.SolutionNotify, &p.Calendar)
	}
	if errors.Is(err, sql.ErrNoRows) {
		// A deleted SLA or queue: no escalation settings, as in OTRS.
		return Preferences{}, nil
	}
	if err != nil {
		return Preferences{}, fmt.Errorf("escalation preferences of ticket %d: %w", t.id, err)
	}
	return p, nil
}

// computeIndex is OTRS TicketEscalationIndexBuild without the writes.
func (s *Service) computeIndex(ctx context.Context, t *ticket) (Index, error) {
	var ix Index
	if finished(t.stateType) {
		return ix, nil
	}
	prefs, err := s.preferences(ctx, t)
	if err != nil {
		return ix, err
	}
	if !prefs.any() {
		return ix, nil
	}
	cal, err := s.calendar(ctx, prefs.Calendar)
	if err != nil {
		return ix, err
	}

	if prefs.FirstResponseTime > 0 {
		responded, err := s.exists(ctx, `
			SELECT 1 FROM article a
			JOIN article_sender_type ast ON ast.id = a.article_sender_type_id
			WHERE a.ticket_id = ? AND ast.name = 'agent' AND a.is_visible_for_customer = 1`, t.id)
		if err != nil {
			return ix, fmt.Errorf("first response of ticket %d: %w", t.id, err)
		}
		if !responded {
			if ix.Response, err = destination(cal, t.created, prefs.FirstResponseTime); err != nil {
				return ix, err
			}
		}
	}

	if prefs.UpdateTime > 0 && !strings.HasPrefix(t.stateType, "pending") {
		since, err := s.updateEscalationStart(ctx, t.id)
		if err != nil {
			return ix, err
		}
		if !since.IsZero() {
			if ix.Update, err = destination(cal, since, prefs.UpdateTime); err != nil {
				return ix, err
			}
		}
	}

	if prefs.SolutionTime > 0 {
		solved, err := s.exists(ctx, `
			SELECT 1 FROM ticket_history th
			JOIN ticket_history_type tht ON tht.id = th.history_type_id
			WHERE th.ticket_id = ? AND tht.name IN ('StateUpdate', 'NewTicket')
			  AND th.state_id IN (`+lookups.ClosedStateIDsSQL+`)`, t.id)
		if err != nil {
			return ix, fmt.Errorf("solution of ticket %d: %w", t.id, err)
		}
		if !solved {
			if ix.Solution, err = destination(cal, t.created, prefs.SolutionTime); err != nil {
				return ix, err
			}
		}
	}

	for _, v := range []int64{ix.Response, ix.Update, ix.Solution} {
		if v > 0 && (ix.Escalation == 0 || v < ix.Escalation) {
			ix.Escalation = v
		}
	}
	return ix, nil
}

func destination(cal *Calendar, start time.Time, minutes int) (int64, error) {
	t, err := cal.AddWorkingTime(start, int64(minutes)*60)
	if err != nil {
		return 0, err
	}
	return t.Unix(), nil
}

func (s *Service) exists(ctx context.Context, query string, args ...any) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, database.ConvertPlaceholders(query+" LIMIT 1"), args...).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// updateEscalationStart returns when the update escalation clock started
// (OTRS: the latest agent article, or the oldest of the customer articles
// after it; internal articles and other senders are skipped, but the newest
// article of any kind is the fallback). Zero when the ticket has no article.
func (s *Service) updateEscalationStart(ctx context.Context, ticketID int) (time.Time, error) {
	rows, err := s.db.QueryContext(ctx, database.ConvertPlaceholders(`
		SELECT ast.name, a.is_visible_for_customer, a.create_time
		FROM article a
		JOIN article_sender_type ast ON ast.id = a.article_sender_type_id
		WHERE a.ticket_id = ?
		ORDER BY a.create_time DESC, a.id DESC`), ticketID)
	if err != nil {
		return time.Time{}, fmt.Errorf("articles of ticket %d: %w", ticketID, err)
	}
	defer rows.Close()

	var since time.Time
	lastSender := ""
	for rows.Next() {
		var sender string
		var visible int
		var created time.Time
		if err := rows.Scan(&sender, &visible, &created); err != nil {
			return time.Time{}, err
		}
		if since.IsZero() {
			since = created
		}
		if visible == 0 || (sender != "agent" && sender != "customer") {
			continue
		}
		if sender == "agent" && lastSender == "customer" {
			break
		}
		since = created
		if sender == "customer" {
			lastSender = "customer"
			continue
		}
		break // latest agent article
	}
	return since, rows.Err()
}

// TicketEscalationIndexBuild recomputes a ticket's escalation index and
// stores it in the ticket's escalation_* columns. When the change ends an
// escalation that had started, it records the OTRS Escalation*TimeStop
// history event on behalf of userID. Concurrent rebuilds of the same ticket
// are safe: only the one that moves the stored index records stop events.
func (s *Service) TicketEscalationIndexBuild(ctx context.Context, ticketID, userID int) error {
	return s.build(ctx, ticketID, Change{UserID: userID, At: s.now()})
}

// Change describes the ticket change a rebuild follows: who made it and when.
// An escalation counts as stopped by the change when it had started by then
// and no longer has.
type Change struct {
	UserID int
	At     time.Time
}

func (s *Service) build(ctx context.Context, ticketID int, change Change) error {
	t, err := s.loadTicket(ctx, s.db, ticketID)
	if err != nil || t == nil {
		return err
	}
	ix, err := s.computeIndex(ctx, t)
	if err != nil {
		return err
	}
	if ix == t.index {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, database.ConvertPlaceholders(`
		UPDATE ticket SET escalation_time = ?, escalation_response_time = ?,
		       escalation_update_time = ?, escalation_solution_time = ?
		WHERE id = ? AND escalation_time = ? AND escalation_response_time = ?
		  AND escalation_update_time = ? AND escalation_solution_time = ?`),
		ix.Escalation, ix.Response, ix.Update, ix.Solution,
		t.id, t.index.Escalation, t.index.Response, t.index.Update, t.index.Solution)
	if err != nil {
		return fmt.Errorf("store escalation index of ticket %d: %w", t.id, err)
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		// Another rebuild stored its result first; it also owns the events.
		return err
	}

	at := change.At.Unix()
	for _, kind := range Kinds {
		before, after := t.index.Of(kind), ix.Of(kind)
		if before > 0 && before <= at && !(after > 0 && after <= at) {
			if err := s.addHistory(ctx, tx, t, stopEvent[kind], change.UserID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *Service) addHistory(ctx context.Context, exec interface{}, t *ticket, event string, userID int) error {
	if userID <= 0 {
		userID = 1
	}
	entry := models.TicketHistoryInsert{
		TicketID:    t.id,
		QueueID:     t.queueID,
		OwnerID:     t.ownerID,
		PriorityID:  t.priorityID,
		StateID:     t.stateID,
		CreatedBy:   userID,
		HistoryType: event,
		Name:        "%%" + event + "%%triggered",
		CreatedAt:   s.now().UTC(),
	}
	if t.typeID.Valid {
		entry.TypeID = int(t.typeID.Int64)
	}
	if err := s.history.AddTicketHistoryEntry(ctx, exec, entry); err != nil {
		return fmt.Errorf("record %s for ticket %d: %w", event, t.id, err)
	}
	return nil
}

// RebuildAll rebuilds the escalation index of every ticket that can escalate
// or still has an index (OTRS Maint::Ticket::EscalationIndexRebuild). It
// returns how many tickets it rebuilt; a failing ticket is logged and skipped.
func (s *Service) RebuildAll(ctx context.Context, userID int) (int, error) {
	rows, err := s.db.QueryContext(ctx, database.ConvertPlaceholders(`
		SELECT id FROM ticket
		WHERE ticket_state_id NOT IN (`+lookups.FinishedStateIDsSQL+`)
		   OR escalation_time <> 0 OR escalation_response_time <> 0
		   OR escalation_update_time <> 0 OR escalation_solution_time <> 0
		ORDER BY id`))
	if err != nil {
		return 0, fmt.Errorf("list tickets for escalation rebuild: %w", err)
	}
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	change := Change{UserID: userID, At: s.now()}
	return s.rebuild(ctx, ids, func(int) Change { return change })
}

func (s *Service) rebuild(ctx context.Context, ids []int, changeOf func(int) Change) (int, error) {
	done := 0
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return done, err
		}
		if err := s.build(ctx, id, changeOf(id)); err != nil {
			s.logger.Printf("escalation: rebuild ticket %d: %v", id, err)
			continue
		}
		done++
	}
	return done, nil
}
