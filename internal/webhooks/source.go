package webhooks

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/webhook"
)

// batchSize bounds the rows read from one source per poll.
const batchSize = 200

// Event sources. Every ticket and article write path inserts a row into
// ticket or article, and every ticket attribute change writes an OTRS
// ticket_history row, so reading those tables by id catches changes made by
// any handler, the email importer, escalation checks or external tools.
const (
	sourceTicket  = "ticket"
	sourceArticle = "article"
	sourceHistory = "ticket_history"
)

// sourceEvent is one change read from a source table.
type sourceEvent struct {
	id         int64
	event      string
	occurredAt time.Time
	ticketID   int64
	articleID  int64
	change     map[string]interface{}
}

// Service turns ticket, article and history rows into webhook deliveries and
// sends due deliveries. Run it periodically from a single goroutine per
// process (the runner task does); several processes may run it concurrently.
type Service struct {
	db         *sql.DB
	repo       *webhook.Repository
	dispatcher *webhook.Dispatcher

	mu sync.Mutex
	// horizon holds, per source, the highest id seen on the previous poll.
	// Rows are only turned into events once they are at or below the previous
	// poll's horizon: ids are allocated before commit, so a row with a lower
	// id can become visible after a higher one. Waiting one poll interval lets
	// such transactions commit before the cursor moves past them.
	horizon map[string]int64
}

// NewService returns a service on db.
func NewService(db *sql.DB) *Service {
	repo := webhook.NewRepository(db)
	return &Service{
		db:         db,
		repo:       repo,
		dispatcher: webhook.NewDispatcher(repo),
		horizon:    map[string]int64{},
	}
}

// RunOnce publishes new events and then sends due deliveries until none are
// left or ctx expires. It returns how many events were queued and how many
// delivery attempts were made.
func (s *Service) RunOnce(ctx context.Context) (queued, attempted int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	queued, err = s.publish(ctx)
	if err != nil {
		return queued, 0, err
	}
	const sendBatch = 50
	for ctx.Err() == nil {
		n, err := s.dispatcher.ProcessDue(ctx, sendBatch)
		attempted += n
		if err != nil {
			return queued, attempted, err
		}
		if n < sendBatch {
			break
		}
	}
	return queued, attempted, nil
}

func (s *Service) publish(ctx context.Context) (int, error) {
	hooks, err := s.repo.List(ctx, nil)
	if err != nil {
		return 0, err
	}
	wanted := map[string]bool{}
	for _, w := range hooks {
		if w.IsActive {
			for _, e := range w.Events {
				wanted[e] = true
			}
		}
	}
	total := 0
	for _, src := range []string{sourceTicket, sourceArticle, sourceHistory} {
		n, err := s.publishSource(ctx, src, hooks, wanted)
		total += n
		if err != nil {
			return total, fmt.Errorf("webhook source %s: %w", src, err)
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

// publishSource moves one source's cursor over the rows that settled since the
// last poll, queueing deliveries for subscribed webhooks in the same
// transaction as each cursor move.
func (s *Service) publishSource(ctx context.Context, src string, hooks []*webhook.Webhook, wanted map[string]bool) (int, error) {
	last, err := s.repo.Cursor(ctx, src, func(ctx context.Context) (int64, error) { return s.maxID(ctx, src) })
	if err != nil {
		return 0, err
	}
	current, err := s.maxID(ctx, src)
	if err != nil {
		return 0, err
	}
	if current < last {
		// The rows at the top were deleted, or ids were reset (restore,
		// sequence reset). Every published row still present has an id
		// <= current, so lowering the cursor re-publishes nothing and keeps
		// reused ids from being skipped.
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return 0, err
		}
		moved, err := s.repo.AdvanceCursor(ctx, tx, src, last, current)
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
		n, moved, err := s.publishBatch(ctx, src, last, events, hooks, wanted)
		queued += n
		if err != nil || !moved || len(events) < batchSize {
			return queued, err
		}
		last = events[len(events)-1].id
	}
	return queued, ctx.Err()
}

// publishBatch advances the cursor from last to the final event's id and
// queues the deliveries in one transaction. moved is false when another
// process advanced the cursor first; nothing is queued then.
func (s *Service) publishBatch(ctx context.Context, src string, last int64, events []sourceEvent,
	hooks []*webhook.Webhook, wanted map[string]bool) (queued int, moved bool, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, err
	}
	defer func() { _ = tx.Rollback() }()
	moved, err = s.repo.AdvanceCursor(ctx, tx, src, last, events[len(events)-1].id)
	if err != nil || !moved {
		return 0, false, err
	}
	for _, ev := range events {
		if ev.event == "" || !wanted[ev.event] {
			continue
		}
		data, err := s.eventData(ctx, ev)
		if err != nil {
			return 0, false, err
		}
		n, err := s.dispatcher.Enqueue(ctx, tx, hooks, ev.event, ev.occurredAt, data)
		if err != nil {
			return 0, false, err
		}
		queued += n
	}
	if err := tx.Commit(); err != nil {
		return 0, false, err
	}
	return queued, true, nil
}

// read returns the rows of src with last < id <= upper, oldest first, at most
// batchSize of them.
func (s *Service) read(ctx context.Context, src string, last, upper int64) ([]sourceEvent, error) {
	switch src {
	case sourceTicket:
		return s.readTickets(ctx, last, upper)
	case sourceArticle:
		return s.readArticles(ctx, last, upper)
	default:
		return s.readHistory(ctx, last, upper)
	}
}

func (s *Service) readTickets(ctx context.Context, last, upper int64) ([]sourceEvent, error) {
	rows, err := s.db.QueryContext(ctx, database.ConvertPlaceholders(`
		SELECT id, create_time FROM ticket WHERE id > ? AND id <= ? ORDER BY id LIMIT ?`),
		last, upper, batchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []sourceEvent
	for rows.Next() {
		ev := sourceEvent{event: EventTicketCreated}
		if err := rows.Scan(&ev.id, &ev.occurredAt); err != nil {
			return nil, err
		}
		ev.ticketID = ev.id
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (s *Service) readArticles(ctx context.Context, last, upper int64) ([]sourceEvent, error) {
	rows, err := s.db.QueryContext(ctx, database.ConvertPlaceholders(`
		SELECT id, ticket_id, create_time FROM article WHERE id > ? AND id <= ? ORDER BY id LIMIT ?`),
		last, upper, batchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []sourceEvent
	for rows.Next() {
		ev := sourceEvent{event: EventArticleCreated}
		if err := rows.Scan(&ev.id, &ev.ticketID, &ev.occurredAt); err != nil {
			return nil, err
		}
		ev.articleID = ev.id
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (s *Service) readHistory(ctx context.Context, last, upper int64) ([]sourceEvent, error) {
	rows, err := s.db.QueryContext(ctx, database.ConvertPlaceholders(`
		SELECT th.id, th.ticket_id, th.name, tht.name, st.name, th.create_by, th.create_time
		FROM ticket_history th
		JOIN ticket_history_type tht ON tht.id = th.history_type_id
		LEFT JOIN ticket_state s ON s.id = th.state_id
		LEFT JOIN ticket_state_type st ON st.id = s.type_id
		WHERE th.id > ? AND th.id <= ?
		ORDER BY th.id LIMIT ?`), last, upper, batchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []sourceEvent
	for rows.Next() {
		var (
			ev          sourceEvent
			message     sql.NullString
			historyType string
			stateType   sql.NullString
			userID      int64
		)
		if err := rows.Scan(&ev.id, &ev.ticketID, &message, &historyType, &stateType, &userID, &ev.occurredAt); err != nil {
			return nil, err
		}
		// Unmapped history types still advance the cursor (empty event).
		ev.event, _ = historyEvent(historyType, stateType.String)
		ev.change = map[string]interface{}{
			"history_id":   ev.id,
			"history_type": historyType,
			"message":      message.String,
			"user_id":      userID,
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

// eventData builds the "data" object of the payload.
func (s *Service) eventData(ctx context.Context, ev sourceEvent) (map[string]interface{}, error) {
	ticket, err := s.ticketSnapshot(ctx, ev.ticketID)
	if err != nil {
		return nil, err
	}
	data := map[string]interface{}{"ticket": ticket}
	if ev.articleID != 0 {
		article, err := s.articleSnapshot(ctx, ev.articleID)
		if err != nil {
			return nil, err
		}
		data["article"] = article
	}
	if ev.change != nil {
		data["change"] = ev.change
	}
	return data, nil
}

func nullString(v sql.NullString) interface{} {
	if v.Valid {
		return v.String
	}
	return nil
}

func nullInt(v sql.NullInt64) interface{} {
	if v.Valid {
		return v.Int64
	}
	return nil
}

// ticketSnapshot returns the ticket as it is now. A ticket deleted before the
// event was published is reported by id only.
func (s *Service) ticketSnapshot(ctx context.Context, id int64) (map[string]interface{}, error) {
	var (
		tn, title                             string
		queueID, stateID, priorityID, ownerID int64
		responsibleID                         sql.NullInt64
		queue, state, stateType, priority     sql.NullString
		customerID, customerUserID            sql.NullString
		created, changed                      time.Time
	)
	err := s.db.QueryRowContext(ctx, database.ConvertPlaceholders(`
		SELECT t.tn, t.title, t.queue_id, q.name, t.ticket_state_id, s.name, st.name,
			t.ticket_priority_id, p.name, t.user_id, t.responsible_user_id,
			t.customer_id, t.customer_user_id, t.create_time, t.change_time
		FROM ticket t
		LEFT JOIN queue q ON q.id = t.queue_id
		LEFT JOIN ticket_state s ON s.id = t.ticket_state_id
		LEFT JOIN ticket_state_type st ON st.id = s.type_id
		LEFT JOIN ticket_priority p ON p.id = t.ticket_priority_id
		WHERE t.id = ?`), id).Scan(&tn, &title, &queueID, &queue, &stateID, &state, &stateType,
		&priorityID, &priority, &ownerID, &responsibleID, &customerID, &customerUserID, &created, &changed)
	if errors.Is(err, sql.ErrNoRows) {
		return map[string]interface{}{"id": id, "deleted": true}, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":                  id,
		"ticket_number":       tn,
		"title":               title,
		"queue_id":            queueID,
		"queue":               nullString(queue),
		"state_id":            stateID,
		"state":               nullString(state),
		"state_type":          nullString(stateType),
		"priority_id":         priorityID,
		"priority":            nullString(priority),
		"owner_id":            ownerID,
		"responsible_user_id": nullInt(responsibleID),
		"customer_id":         nullString(customerID),
		"customer_user_id":    nullString(customerUserID),
		"created_at":          created.UTC(),
		"updated_at":          changed.UTC(),
	}, nil
}

// articleSnapshot returns article metadata (not the body: endpoints that
// need it can fetch it through the REST API).
func (s *Service) articleSnapshot(ctx context.Context, id int64) (map[string]interface{}, error) {
	var (
		ticketID, createdBy int64
		senderType          sql.NullString
		visible             int
		from, subject       sql.NullString
		created             time.Time
	)
	err := s.db.QueryRowContext(ctx, database.ConvertPlaceholders(`
		SELECT a.ticket_id, ast.name, a.is_visible_for_customer, a.create_by, a.create_time,
			m.a_from, m.a_subject
		FROM article a
		LEFT JOIN article_sender_type ast ON ast.id = a.article_sender_type_id
		LEFT JOIN article_data_mime m ON m.article_id = a.id
		WHERE a.id = ?`), id).Scan(&ticketID, &senderType, &visible, &createdBy, &created, &from, &subject)
	if errors.Is(err, sql.ErrNoRows) {
		return map[string]interface{}{"id": id, "deleted": true}, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":                      id,
		"ticket_id":               ticketID,
		"sender_type":             nullString(senderType),
		"is_visible_for_customer": visible == 1,
		"from":                    nullString(from),
		"subject":                 nullString(subject),
		"created_by":              createdBy,
		"created_at":              created.UTC(),
	}, nil
}
