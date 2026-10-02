package escalation

import (
	"context"
	"database/sql"
	"fmt"
	"hash"
	"hash/fnv"
	"strconv"
	"sync"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// Change sources. Every ticket write path in GoatFlow inserts a ticket,
// article or ticket_history row (OTRS writes history for every ticket event),
// so the rows added since the previous run name exactly the tickets whose
// escalation index may have moved, whichever handler, importer, plugin or
// external tool made the change.
var indexSources = []string{"ticket", "article", "ticket_history"}

// Indexer keeps the escalation index current: OTRS rebuilds it from the
// ticket event handler (Ticket::EventModulePost###9990-EscalationIndex);
// GoatFlow rebuilds it from RunOnce, which the scheduler calls every few
// seconds. The first run, and every run after an SLA, queue or calendar
// setting changed, rebuilds all tickets that can escalate.
type Indexer struct {
	svc *Service

	mu          sync.Mutex
	primed      bool
	fingerprint string
	// window holds, per source, the highest id seen two runs ago and on the
	// previous run. Each run handles the rows above the older one: ids are
	// allocated before commit, so a row with a lower id can become visible
	// after a higher one; looking back one extra run catches it.
	window map[string][2]int64
}

// NewIndexer returns an indexer that rebuilds through svc.
func NewIndexer(svc *Service) *Indexer {
	return &Indexer{svc: svc, window: map[string][2]int64{}}
}

// RunOnce rebuilds the index of the tickets changed since the previous run
// and returns how many it rebuilt. A run that starts while another is still
// going returns immediately.
func (ix *Indexer) RunOnce(ctx context.Context) (int, error) {
	if !ix.mu.TryLock() {
		return 0, nil
	}
	defer ix.mu.Unlock()

	fp, err := ix.configFingerprint(ctx)
	if err != nil {
		return 0, err
	}
	current := map[string]int64{}
	for _, src := range indexSources {
		if current[src], err = ix.maxID(ctx, src); err != nil {
			return 0, err
		}
	}

	if !ix.primed || fp != ix.fingerprint {
		if err := ix.svc.LoadCalendars(ctx); err != nil {
			return 0, err
		}
		n, err := ix.svc.RebuildAll(ctx, 1)
		if err != nil {
			return n, err
		}
		for _, src := range indexSources {
			ix.window[src] = [2]int64{current[src], current[src]}
		}
		ix.primed, ix.fingerprint = true, fp
		return n, nil
	}

	changes := map[int]Change{}
	var order []int
	for _, src := range indexSources {
		w := ix.window[src]
		from := w[0]
		if current[src] < from {
			from = current[src] // rows deleted or ids reset
		}
		if err := ix.collect(ctx, src, from, current[src], changes, &order); err != nil {
			return 0, err
		}
		ix.window[src] = [2]int64{min(w[1], current[src]), current[src]}
	}
	return ix.svc.rebuild(ctx, order, func(id int) Change { return changes[id] })
}

func (ix *Indexer) maxID(ctx context.Context, src string) (int64, error) {
	var query string
	switch src {
	case "ticket":
		query = `SELECT COALESCE(MAX(id), 0) FROM ticket`
	case "article":
		query = `SELECT COALESCE(MAX(id), 0) FROM article`
	default:
		query = `SELECT COALESCE(MAX(id), 0) FROM ticket_history`
	}
	var id int64
	if err := ix.svc.db.QueryRowContext(ctx, database.ConvertPlaceholders(query)).Scan(&id); err != nil {
		return 0, fmt.Errorf("escalation index: newest %s: %w", src, err)
	}
	return id, nil
}

// collect records, per ticket, the latest change among the rows of src with
// from < id <= to. The escalation events' own history rows are skipped.
func (ix *Indexer) collect(ctx context.Context, src string, from, to int64, changes map[int]Change, order *[]int) error {
	if to <= from {
		return nil
	}
	var query string
	switch src {
	case "ticket":
		query = `SELECT id, create_by, create_time FROM ticket WHERE id > ? AND id <= ? ORDER BY id`
	case "article":
		query = `SELECT ticket_id, create_by, create_time FROM article WHERE id > ? AND id <= ? ORDER BY id`
	default:
		query = `SELECT th.ticket_id, th.create_by, th.create_time FROM ticket_history th
			JOIN ticket_history_type tht ON tht.id = th.history_type_id
			WHERE th.id > ? AND th.id <= ? AND tht.name NOT LIKE 'Escalation%' ORDER BY th.id`
	}
	rows, err := ix.svc.db.QueryContext(ctx, database.ConvertPlaceholders(query), from, to)
	if err != nil {
		return fmt.Errorf("escalation index: changed %s rows: %w", src, err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, user int
		var at time.Time
		if err := rows.Scan(&id, &user, &at); err != nil {
			return err
		}
		prev, seen := changes[id]
		if !seen {
			*order = append(*order, id)
		}
		if !seen || at.After(prev.At) {
			changes[id] = Change{UserID: user, At: at}
		}
	}
	return rows.Err()
}

// configFingerprint changes whenever the escalation settings of an SLA or a
// queue, or a calendar setting (TimeWorkingHours*, TimeVacationDays*,
// TimeZone::*) is added, changed or removed.
func (ix *Indexer) configFingerprint(ctx context.Context) (string, error) {
	const calendarSettings = `name LIKE 'TimeWorkingHours%' OR name LIKE 'TimeVacationDays%' OR name LIKE 'TimeZone::%'`
	h := fnv.New64a()
	for _, q := range []string{
		`SELECT id, first_response_time, first_response_notify, update_time, update_notify,
			solution_time, solution_notify, calendar_name FROM sla ORDER BY id`,
		`SELECT id, first_response_time, first_response_notify, update_time, update_notify,
			solution_time, solution_notify, calendar_name FROM queue ORDER BY id`,
		`SELECT id, name, effective_value, is_valid FROM sysconfig_default WHERE ` + calendarSettings + ` ORDER BY id`,
		`SELECT id, name, effective_value, is_valid FROM sysconfig_modified WHERE ` + calendarSettings + ` ORDER BY id`,
	} {
		if err := hashRows(ctx, ix.svc.db, q, h); err != nil {
			return "", fmt.Errorf("escalation index: settings fingerprint: %w", err)
		}
		_, _ = h.Write([]byte{0})
	}
	return strconv.FormatUint(h.Sum64(), 16), nil
}

func hashRows(ctx context.Context, db *sql.DB, query string, h hash.Hash64) error {
	rows, err := db.QueryContext(ctx, database.ConvertPlaceholders(query))
	if err != nil {
		return err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	vals := make([]sql.NullString, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		for _, v := range vals {
			fmt.Fprintf(h, "%t%q|", v.Valid, v.String)
		}
	}
	return rows.Err()
}
