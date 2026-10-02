package notificationevents

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// batchSize bounds the rows read from one source per transaction.
const batchSize = 200

// Event sources. Every ticket and article write path inserts a row into
// ticket or article, and ticket changes write OTRS ticket_history rows, so
// reading those tables by id catches events from any handler, the email
// importer, the escalation check or external tools.
const (
	sourceTicket  = "ticket"
	sourceArticle = "article"
	sourceHistory = "ticket_history"
)

// event is one ticket event read from a source table.
type event struct {
	source    string
	id        int64 // row id in source
	names     []string
	ticketID  int64
	articleID int64 // 0 when the event has no article
	userID    int   // agent who caused the event
}

// Service evaluates notification rules for new ticket, article and history
// rows. Run it periodically from one goroutine per process (the runner task
// does); several processes may run it concurrently, the cursor update in
// each transaction makes sure every row is evaluated once.
type Service struct {
	db     *sql.DB
	logger *log.Logger

	mu sync.Mutex
	// horizon holds, per source, the highest id seen on the previous run.
	// Ids are allocated before commit, so a row with a lower id can become
	// visible after a higher one; rows are only evaluated once they are at or
	// below the previous run's horizon, which gives such transactions one
	// run interval to commit.
	horizon map[string]int64
}

// NewService returns a service on db.
func NewService(db *sql.DB, logger *log.Logger) *Service {
	if logger == nil {
		logger = log.Default()
	}
	return &Service{db: db, logger: logger, horizon: map[string]int64{}}
}

// RunOnce evaluates the rows that settled since the previous run and returns
// how many emails it queued.
func (s *Service) RunOnce(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rules, err := loadRules(ctx, s.db)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, src := range []string{sourceTicket, sourceArticle, sourceHistory} {
		n, err := s.runSource(ctx, src, rules)
		total += n
		if err != nil {
			return total, fmt.Errorf("notification events, source %s: %w", src, err)
		}
	}
	return total, nil
}

func (s *Service) maxID(ctx context.Context, src string) (int64, error) {
	var query string
	switch src {
	case sourceTicket:
		query = `SELECT COALESCE(MAX(id), 0) FROM ticket`
	case sourceArticle:
		query = `SELECT COALESCE(MAX(id), 0) FROM article`
	default:
		query = `SELECT COALESCE(MAX(id), 0) FROM ticket_history`
	}
	var id int64
	err := s.db.QueryRowContext(ctx, database.ConvertPlaceholders(query)).Scan(&id)
	return id, err
}

// cursor returns the last evaluated id of src. A source seen for the first
// time starts at its current maximum: rows written before notification
// evaluation existed are not notified.
func (s *Service) cursor(ctx context.Context, src string) (int64, error) {
	var last int64
	selectQ := database.ConvertPlaceholders(`SELECT last_id FROM gk_notification_event_cursor WHERE source = ?`)
	err := s.db.QueryRowContext(ctx, selectQ, src).Scan(&last)
	if err == nil {
		return last, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	initial, err := s.maxID(ctx, src)
	if err != nil {
		return 0, err
	}
	if _, err := s.db.ExecContext(ctx, database.ConvertPlaceholders(
		`INSERT INTO gk_notification_event_cursor (source, last_id, change_time) VALUES (?, ?, ?)`),
		src, initial, time.Now()); err != nil {
		// Another process created it concurrently; use its value.
		if rerr := s.db.QueryRowContext(ctx, selectQ, src).Scan(&last); rerr == nil {
			return last, nil
		}
		return 0, err
	}
	return initial, nil
}

// advanceCursor moves the cursor of src from "from" to "to" inside tx. It
// returns false when another process moved it first; the caller must then
// roll back. The row lock taken here serialises concurrent evaluators.
func advanceCursor(ctx context.Context, tx *sql.Tx, src string, from, to int64) (bool, error) {
	res, err := tx.ExecContext(ctx, database.ConvertPlaceholders(
		`UPDATE gk_notification_event_cursor SET last_id = ?, change_time = ? WHERE source = ? AND last_id = ?`),
		to, time.Now(), src, from)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (s *Service) runSource(ctx context.Context, src string, rules []*rule) (int, error) {
	last, err := s.cursor(ctx, src)
	if err != nil {
		return 0, err
	}
	current, err := s.maxID(ctx, src)
	if err != nil {
		return 0, err
	}
	if current < last {
		// Rows at the top were deleted or ids were reset (restore, sequence
		// reset). Every evaluated row still present has an id <= current, so
		// lowering the cursor evaluates nothing twice and keeps reused ids
		// from being skipped.
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return 0, err
		}
		moved, err := advanceCursor(ctx, tx, src, last, current)
		if err != nil || !moved {
			_ = tx.Rollback()
			return 0, err
		}
		if err := tx.Commit(); err != nil {
			return 0, err
		}
		last = current
	}
	upper, seen := s.horizon[src]
	s.horizon[src] = current
	if !seen {
		return 0, nil
	}
	queued := 0
	for last < upper && ctx.Err() == nil {
		events, err := s.read(ctx, src, last, upper)
		if err != nil || len(events) == 0 {
			return queued, err
		}
		n, moved, err := s.evaluateBatch(ctx, src, last, events, rules)
		queued += n
		if err != nil || !moved || len(events) < batchSize {
			return queued, err
		}
		last = events[len(events)-1].id
	}
	return queued, ctx.Err()
}

// evaluateBatch moves the cursor from last to the final event's id and
// queues the notifications of every event in one transaction. moved is false
// when another process moved the cursor first; nothing is queued then.
func (s *Service) evaluateBatch(ctx context.Context, src string, last int64, events []event, rules []*rule) (queued int, moved bool, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, err
	}
	defer func() { _ = tx.Rollback() }()
	moved, err = advanceCursor(ctx, tx, src, last, events[len(events)-1].id)
	if err != nil || !moved {
		return 0, false, err
	}
	b := newBatch(s, tx)
	for _, ev := range events {
		n, err := b.evaluate(ctx, ev, rules)
		if err != nil {
			return 0, false, fmt.Errorf("%s row %d (ticket %d): %w", ev.source, ev.id, ev.ticketID, err)
		}
		queued += n
	}
	if err := tx.Commit(); err != nil {
		return 0, false, err
	}
	return queued, true, nil
}

// read returns the rows of src with last < id <= upper, oldest first, at
// most batchSize of them.
func (s *Service) read(ctx context.Context, src string, last, upper int64) ([]event, error) {
	var query string
	switch src {
	case sourceTicket:
		query = `SELECT id, id, 0, create_by, '', '' FROM ticket WHERE id > ? AND id <= ? ORDER BY id LIMIT ?`
	case sourceArticle:
		query = `SELECT id, ticket_id, id, create_by, '', '' FROM article WHERE id > ? AND id <= ? ORDER BY id LIMIT ?`
	default:
		query = `
			SELECT th.id, th.ticket_id, COALESCE(th.article_id, 0), th.create_by, tht.name, COALESCE(st.name, '')
			FROM ticket_history th
			JOIN ticket_history_type tht ON tht.id = th.history_type_id
			LEFT JOIN ticket_state s ON s.id = th.state_id
			LEFT JOIN ticket_state_type st ON st.id = s.type_id
			WHERE th.id > ? AND th.id <= ?
			ORDER BY th.id LIMIT ?`
	}
	rows, err := s.db.QueryContext(ctx, database.ConvertPlaceholders(query), last, upper, batchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []event
	for rows.Next() {
		ev := event{source: src}
		var historyType, stateType string
		if err := rows.Scan(&ev.id, &ev.ticketID, &ev.articleID, &ev.userID, &historyType, &stateType); err != nil {
			return nil, err
		}
		switch src {
		case sourceTicket:
			ev.names = ticketRowEvents
		case sourceArticle:
			ev.names = articleRowEvents
		default:
			// Unmapped history types still advance the cursor (no names).
			ev.names = historyRowEvents(historyType, stateType)
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}
