package plugin

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// requireHostTestDB returns the real test database (schema from
// migrations/mysql + migrations/postgres), skipping when none is configured.
func requireHostTestDB(t *testing.T) *sql.DB {
	t.Helper()
	if err := database.InitTestDB(); err != nil {
		t.Skipf("test database not available: %v", err)
	}
	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Skipf("test database not available: %v", err)
	}
	return db
}

// insertHostTestUser inserts an agent row and deletes it on cleanup. Rows
// that reference it must be inserted afterwards so their cleanups run first.
func insertHostTestUser(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	now := time.Now()
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(
		`INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		 VALUES (?, 'x', 'Host', 'Test', 1, ?, 1, ?, 1) RETURNING id`),
		fmt.Sprintf("hostapi-test-%d", now.UnixNano()), now, now)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(database.ConvertPlaceholders(`DELETE FROM users WHERE id = ?`), id); err != nil {
			t.Errorf("cleanup user %d: %v", id, err)
		}
	})
	return id
}

// insertHostTestTicket inserts a ticket in the given state (queue 1, created
// by user 1) and on cleanup deletes it with any articles created on it.
func insertHostTestTicket(t *testing.T, db *sql.DB, stateID int64) int64 {
	t.Helper()
	now := time.Now()
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, type_id, user_id, responsible_user_id,
			ticket_priority_id, ticket_state_id, customer_user_id, timeout, until_time,
			escalation_time, escalation_update_time, escalation_response_time, escalation_solution_time,
			archive_flag, create_time, create_by, change_time, change_by)
		VALUES (?, 'HostAPI test', 1, 1, 1, 1, 1, 3, ?, 'j@x.com', 0, 0, 0, 0, 0, 0, 0, ?, 1, ?, 1)
		RETURNING id`),
		fmt.Sprintf("hostapi%d", now.UnixNano()), stateID, now, now)
	if err != nil {
		t.Fatalf("insert ticket: %v", err)
	}
	t.Cleanup(func() {
		for _, q := range []string{
			`DELETE FROM article_data_mime WHERE article_id IN (SELECT id FROM article WHERE ticket_id = ?)`,
			`DELETE FROM article WHERE ticket_id = ?`,
			`DELETE FROM ticket WHERE id = ?`,
		} {
			if _, err := db.Exec(database.ConvertPlaceholders(q), id); err != nil {
				t.Errorf("cleanup %q: %v", q, err)
			}
		}
	})
	return id
}

// insertHostTestState inserts a ticket_state of the given type and deletes it
// on cleanup.
func insertHostTestState(t *testing.T, db *sql.DB, name, color string, typeID, validID int64) int64 {
	t.Helper()
	now := time.Now()
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket_state (name, color, type_id, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, 1, ?, 1) RETURNING id`),
		name, color, typeID, validID, now, now)
	if err != nil {
		t.Fatalf("insert ticket_state %q: %v", name, err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(database.ConvertPlaceholders(`DELETE FROM ticket_state WHERE id = ?`), id); err != nil {
			t.Errorf("cleanup ticket_state %d: %v", id, err)
		}
	})
	return id
}

// ticketStateFixture is a ticket in the seeded "new" state (id 1) plus a
// custom state "Pending custom" of the seeded "pending reminder" type (id 3),
// proving pending detection is by type-name prefix, not state ids or names,
// and an invalid (valid_id 2) state.
type ticketStateFixture struct {
	h        *ProdHostAPI
	db       *sql.DB
	userID   int64
	ticketID int64
	pending  int64
	invalid  int64
	pendName string
}

const (
	seededStateNew  = 1 // ticket_state "new" (type new)
	seededStateOpen = 2 // ticket_state "open" (type open)
	seededTypeOpen  = 2 // ticket_state_type "open"
	seededTypePend  = 3 // ticket_state_type "pending reminder"
)

func newTicketStateFixture(t *testing.T) *ticketStateFixture {
	t.Helper()
	db := requireHostTestDB(t)
	suffix := time.Now().UnixNano()
	f := &ticketStateFixture{db: db, pendName: fmt.Sprintf("Pending custom %d", suffix)}
	f.userID = insertHostTestUser(t, db)
	f.pending = insertHostTestState(t, db, f.pendName, "#FFC542FF", seededTypePend, 1)
	f.invalid = insertHostTestState(t, db, fmt.Sprintf("removed %d", suffix), "#8D8D9BFF", seededTypeOpen, 2)
	f.ticketID = insertHostTestTicket(t, db, seededStateNew)
	f.h = NewProdHostAPI(WithDB("default", db))
	return f
}

// unusedID returns an id greater than every id in the ticket or ticket_state
// table.
func unusedID(t *testing.T, db *sql.DB, table string) int64 {
	t.Helper()
	q := map[string]string{
		"ticket":       `SELECT COALESCE(MAX(id), 0) FROM ticket`,
		"ticket_state": `SELECT COALESCE(MAX(id), 0) FROM ticket_state`,
	}[table]
	var maxID int64
	if err := db.QueryRow(database.ConvertPlaceholders(q)).Scan(&maxID); err != nil {
		t.Fatalf("max id %s: %v", table, err)
	}
	return maxID + 1000
}

func (f *ticketStateFixture) scan(t *testing.T) (stateID, untilTime, changeBy int64) {
	t.Helper()
	if err := f.db.QueryRow(database.ConvertPlaceholders(`SELECT ticket_state_id, until_time, change_by FROM ticket WHERE id = ?`), f.ticketID).
		Scan(&stateID, &untilTime, &changeBy); err != nil {
		t.Fatalf("scan ticket: %v", err)
	}
	return
}

func TestChangeTicketStatus_PendingRequiresUntil(t *testing.T) {
	f := newTicketStateFixture(t)
	ctx := context.Background()

	err := f.h.ChangeTicketStatus(ctx, f.ticketID, f.pending, f.userID, 0)
	if err == nil {
		t.Fatal("expected error for pending state without until_time")
	}
	if !strings.Contains(err.Error(), "pending time is required") {
		t.Fatalf("unexpected error: %v", err)
	}
	stateID, _, changeBy := f.scan(t)
	if stateID != seededStateNew || changeBy != 1 {
		t.Fatalf("ticket must not have moved, state_id = %d change_by = %d", stateID, changeBy)
	}
}

func TestChangeTicketStatus_PendingSetsUntil(t *testing.T) {
	f := newTicketStateFixture(t)
	ctx := context.Background()

	if err := f.h.ChangeTicketStatus(ctx, f.ticketID, f.pending, f.userID, 1893456000); err != nil {
		t.Fatalf("ChangeTicketStatus: %v", err)
	}
	stateID, untilTime, changeBy := f.scan(t)
	if stateID != f.pending || untilTime != 1893456000 || changeBy != f.userID {
		t.Fatalf("got state=%d until=%d by=%d, want state=%d until=1893456000 by=%d", stateID, untilTime, changeBy, f.pending, f.userID)
	}
}

func TestChangeTicketStatus_NonPendingClearsUntil(t *testing.T) {
	f := newTicketStateFixture(t)
	ctx := context.Background()

	if _, err := f.db.Exec(database.ConvertPlaceholders(`UPDATE ticket SET until_time = 1893456000 WHERE id = ?`), f.ticketID); err != nil {
		t.Fatalf("seed until: %v", err)
	}

	if err := f.h.ChangeTicketStatus(ctx, f.ticketID, seededStateOpen, f.userID, 1893456000); err != nil {
		t.Fatalf("ChangeTicketStatus: %v", err)
	}
	stateID, untilTime, changeBy := f.scan(t)
	if stateID != seededStateOpen || untilTime != 0 || changeBy != f.userID {
		t.Fatalf("got state=%d until=%d by=%d", stateID, untilTime, changeBy)
	}
}

func TestChangeTicketStatus_UnknownState(t *testing.T) {
	f := newTicketStateFixture(t)
	ctx := context.Background()

	if err := f.h.ChangeTicketStatus(ctx, f.ticketID, unusedID(t, f.db, "ticket_state"), f.userID, 0); err == nil {
		t.Fatal("expected error for unknown state")
	}
	if err := f.h.ChangeTicketStatus(ctx, f.ticketID, f.invalid, f.userID, 0); err == nil {
		t.Fatal("expected error for invalid (valid_id=2) state")
	}
	stateID, _, _ := f.scan(t)
	if stateID != seededStateNew {
		t.Fatalf("ticket must not have moved, state_id = %d", stateID)
	}
}

func TestChangeTicketStatus_MissingTicket(t *testing.T) {
	f := newTicketStateFixture(t)
	ctx := context.Background()

	if err := f.h.ChangeTicketStatus(ctx, unusedID(t, f.db, "ticket"), seededStateOpen, f.userID, 0); err == nil {
		t.Fatal("expected error for missing ticket")
	}
}

func TestListTicketStates(t *testing.T) {
	f := newTicketStateFixture(t)
	ctx := context.Background()

	states, err := f.h.ListTicketStates(ctx)
	if err != nil {
		t.Fatalf("ListTicketStates: %v", err)
	}
	var validCount int
	if err := f.db.QueryRow(database.ConvertPlaceholders(`SELECT COUNT(*) FROM ticket_state WHERE valid_id = 1`)).Scan(&validCount); err != nil {
		t.Fatalf("count valid states: %v", err)
	}
	if len(states) != validCount {
		t.Fatalf("expected %d valid states, got %d", validCount, len(states))
	}
	var found bool
	for i, s := range states {
		if i > 0 && states[i-1].ID >= s.ID {
			t.Fatalf("states not ordered by id: %d before %d", states[i-1].ID, s.ID)
		}
		if s.ID == f.invalid {
			t.Fatalf("invalid state %d must not be listed", f.invalid)
		}
		if s.ID == f.pending {
			found = true
			if s.Name != f.pendName || s.Color != "#FFC542FF" || s.TypeID != seededTypePend || s.TypeName != "pending reminder" {
				t.Fatalf("pending state = %+v", s)
			}
		}
	}
	if !found {
		t.Fatalf("custom state %d not listed", f.pending)
	}
}
