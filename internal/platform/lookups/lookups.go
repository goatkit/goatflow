// Name-based resolution of OTRS/Znuny lookup rows (state types, history types,
// sender types, lock types, communication channels, link types/states) by
// name.
//
// Lookup ids differ between installations: a fresh GoatFlow install seeds
// ticket_state_type as 1=new 2=open 3=pending reminder 4=pending auto
// 5=closed, while a database imported from OTRS/Znuny has 1=new 2=open
// 3=closed 4=pending reminder 5=pending auto 6=removed 7=merged. Code must
// therefore never hard-code these ids; it names the row (as OTRS does) and
// resolves the id here, or matches by name inside SQL with the fragments
// below.

package lookups

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// Table is a lookup table whose rows are resolved by name. Only these tables
// are accepted, so the table name can safely be part of the query text.
type Table string

// Lookup tables with fixed, OTRS-defined row names.
const (
	StateType       Table = "ticket_state_type"
	HistoryType     Table = "ticket_history_type"
	SenderType      Table = "article_sender_type"
	LockType        Table = "ticket_lock_type"
	Channel         Table = "communication_channel"
	LinkType        Table = "link_type"
	LinkState       Table = "link_state"
	LinkObject      Table = "link_object"
	AutoResponse    Table = "auto_response_type"
	FollowUp        Table = "follow_up_possible"
	ValidLookup     Table = "valid"
	StateLookup     Table = "ticket_state"
	PriorityTable   Table = "ticket_priority"
	TicketTypeTable Table = "ticket_type"
)

var queries = map[Table]string{
	StateType:       "SELECT id, name FROM ticket_state_type",
	HistoryType:     "SELECT id, name FROM ticket_history_type",
	SenderType:      "SELECT id, name FROM article_sender_type",
	LockType:        "SELECT id, name FROM ticket_lock_type",
	Channel:         "SELECT id, name FROM communication_channel",
	LinkType:        "SELECT id, name FROM link_type",
	LinkState:       "SELECT id, name FROM link_state",
	LinkObject:      "SELECT id, name FROM link_object",
	AutoResponse:    "SELECT id, name FROM auto_response_type",
	FollowUp:        "SELECT id, name FROM follow_up_possible",
	ValidLookup:     "SELECT id, name FROM valid",
	StateLookup:     "SELECT id, name FROM ticket_state",
	PriorityTable:   "SELECT id, name FROM ticket_priority",
	TicketTypeTable: "SELECT id, name FROM ticket_type",
}

// cachedTables are never renamed in practice (OTRS treats their names as
// API), so their rows are cached per database handle. ticket_state,
// ticket_priority and ticket_type are admin-editable and are always read fresh.
var cachedTables = map[Table]bool{
	StateType: true, HistoryType: true, SenderType: true, LockType: true, Channel: true,
	LinkType: true, LinkState: true, LinkObject: true, AutoResponse: true, FollowUp: true, ValidLookup: true,
}

// OTRS ticket state type names.
const (
	StateTypeNew             = "new"
	StateTypeOpen            = "open"
	StateTypeClosed          = "closed"
	StateTypePendingReminder = "pending reminder"
	StateTypePendingAuto     = "pending auto"
	StateTypeRemoved         = "removed"
	StateTypeMerged          = "merged"
)

// OTRS default ticket state names.
const (
	StateNew                = "new"
	StateOpen               = "open"
	StateClosedSuccessful   = "closed successful"
	StateClosedUnsuccessful = "closed unsuccessful"
	StatePendingReminder    = "pending reminder"
	StatePendingAutoPlus    = "pending auto close+"
	StatePendingAutoMinus   = "pending auto close-"
	StateRemoved            = "removed"
	StateMerged             = "merged"
)

// OTRS article sender type names.
const (
	SenderAgent    = "agent"
	SenderSystem   = "system"
	SenderCustomer = "customer"
)

// OTRS ticket lock type names.
const (
	LockUnlock  = "unlock"
	LockLock    = "lock"
	LockTmpLock = "tmp_lock"
)

// OTRS communication channel names.
const (
	ChannelEmail    = "Email"
	ChannelPhone    = "Phone"
	ChannelInternal = "Internal"
	ChannelChat     = "Chat"
)

// SQL fragments selecting ticket_state ids by state type name. Use them
// inside `ticket_state_id IN (...)` / `NOT IN (...)`.
const (
	stateIDsOfTypes = "SELECT lk_s.id FROM ticket_state lk_s JOIN ticket_state_type lk_st ON lk_st.id = lk_s.type_id WHERE lk_st.name IN "

	// NewOpenStateIDsSQL: states of type new or open.
	NewOpenStateIDsSQL = stateIDsOfTypes + "('new', 'open')"
	// PendingStateIDsSQL: states of type pending reminder or pending auto.
	PendingStateIDsSQL = stateIDsOfTypes + "('pending reminder', 'pending auto')"
	// PendingReminderStateIDsSQL: states of type pending reminder.
	PendingReminderStateIDsSQL = stateIDsOfTypes + "('pending reminder')"
	// ClosedStateIDsSQL: states of type closed.
	ClosedStateIDsSQL = stateIDsOfTypes + "('closed')"
	// ViewableStateIDsSQL: OTRS Ticket::ViewableStateType default
	// (new, open, pending reminder, pending auto).
	ViewableStateIDsSQL = stateIDsOfTypes + "('new', 'open', 'pending reminder', 'pending auto')"
	// FinishedStateIDsSQL: states of type closed, removed or merged.
	FinishedStateIDsSQL = stateIDsOfTypes + "('closed', 'removed', 'merged')"
)

// IsPendingStateType reports whether a state type name is a pending type.
func IsPendingStateType(typeName string) bool {
	n := strings.ToLower(strings.TrimSpace(typeName))
	return n == StateTypePendingReminder || n == StateTypePendingAuto
}

// IsClosedStateType reports whether a state type name ends the ticket's
// life (closed, removed or merged).
func IsClosedStateType(typeName string) bool {
	n := strings.ToLower(strings.TrimSpace(typeName))
	return n == StateTypeClosed || n == StateTypeRemoved || n == StateTypeMerged
}

type table struct {
	byName map[string]int
	byID   map[int]string
}

var (
	mu    sync.RWMutex
	cache = map[*sql.DB]map[Table]*table{}
)

func load(ctx context.Context, db *sql.DB, t Table) (*table, error) {
	q, ok := queries[t]
	if !ok {
		return nil, fmt.Errorf("lookups: unknown table %q", t)
	}
	rows, err := db.QueryContext(ctx, database.ConvertPlaceholders(q))
	if err != nil {
		return nil, fmt.Errorf("lookups: load %s: %w", t, err)
	}
	defer rows.Close()
	tb := &table{byName: map[string]int{}, byID: map[int]string{}}
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("lookups: scan %s: %w", t, err)
		}
		tb.byName[strings.ToLower(name)] = id
		tb.byID[id] = name
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lookups: load %s: %w", t, err)
	}
	if cachedTables[t] {
		mu.Lock()
		if cache[db] == nil {
			cache[db] = map[Table]*table{}
		}
		cache[db][t] = tb
		mu.Unlock()
	}
	return tb, nil
}

func get(ctx context.Context, db *sql.DB, t Table, fresh bool) (*table, error) {
	if db == nil {
		return nil, fmt.Errorf("lookups: nil database")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if !fresh && cachedTables[t] {
		mu.RLock()
		tb := cache[db][t]
		mu.RUnlock()
		if tb != nil {
			return tb, nil
		}
	}
	return load(ctx, db, t)
}

// ID returns the id of the row named name (case-insensitive) in table t. A
// cache miss reloads the table once, so rows added after the first load are
// found. Missing rows return an error wrapping sql.ErrNoRows.
func ID(ctx context.Context, db *sql.DB, t Table, name string) (int, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	tb, err := get(ctx, db, t, false)
	if err != nil {
		return 0, err
	}
	if id, ok := tb.byName[key]; ok {
		return id, nil
	}
	if !cachedTables[t] {
		return 0, fmt.Errorf("lookups: %s %q: %w", t, name, sql.ErrNoRows)
	}
	if tb, err = get(ctx, db, t, true); err != nil {
		return 0, err
	}
	if id, ok := tb.byName[key]; ok {
		return id, nil
	}
	return 0, fmt.Errorf("lookups: %s %q: %w", t, name, sql.ErrNoRows)
}

// IDs resolves several names of one table; names that do not exist are
// skipped (an OTRS database may lack optional rows).
func IDs(ctx context.Context, db *sql.DB, t Table, names ...string) ([]int, error) {
	out := make([]int, 0, len(names))
	for _, n := range names {
		id, err := ID(ctx, db, t, n)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

// Name returns the name of row id in table t ("" with sql.ErrNoRows when
// absent). A cache miss reloads the table once.
func Name(ctx context.Context, db *sql.DB, t Table, id int) (string, error) {
	tb, err := get(ctx, db, t, false)
	if err != nil {
		return "", err
	}
	if n, ok := tb.byID[id]; ok {
		return n, nil
	}
	if cachedTables[t] {
		if tb, err = get(ctx, db, t, true); err != nil {
			return "", err
		}
		if n, ok := tb.byID[id]; ok {
			return n, nil
		}
	}
	return "", fmt.Errorf("lookups: %s id %d: %w", t, id, sql.ErrNoRows)
}

// StateTypeNameOfState returns the state type name of ticket_state id.
func StateTypeNameOfState(ctx context.Context, db *sql.DB, stateID int) (string, error) {
	if db == nil {
		return "", fmt.Errorf("lookups: nil database")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	var name string
	err := db.QueryRowContext(ctx, database.ConvertPlaceholders(
		`SELECT st.name FROM ticket_state s JOIN ticket_state_type st ON st.id = s.type_id WHERE s.id = ?`),
		stateID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf("lookups: state type of state %d: %w", stateID, err)
	}
	return name, nil
}

// Invalidate drops cached rows for db (all tables).
func Invalidate(db *sql.DB) {
	mu.Lock()
	delete(cache, db)
	mu.Unlock()
}
