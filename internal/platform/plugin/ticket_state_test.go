package plugin

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// newTicketStateTestHost boots a ProdHostAPI over an in-memory SQLite DB with
// the minimal ticket/ticket_state/ticket_state_type tables. State 3 is named
// "Pending custom" (type "pending reminder") to prove pending detection is by
// type-name prefix, not hardcoded state ids.
func newTicketStateTestHost(t *testing.T) *ProdHostAPI {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ddl := []string{
		`CREATE TABLE ticket (
			id INTEGER PRIMARY KEY,
			ticket_state_id INTEGER NOT NULL,
			until_time INTEGER NOT NULL DEFAULT 0,
			change_time DATETIME,
			change_by INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE ticket_state_type (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL)`,
		`CREATE TABLE ticket_state (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			color TEXT NOT NULL DEFAULT '',
			type_id INTEGER NOT NULL,
			valid_id INTEGER NOT NULL DEFAULT 1)`,
	}
	for _, q := range ddl {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("exec ddl %q: %v", q, err)
		}
	}

	seeds := []string{
		`INSERT INTO ticket_state_type (id, name) VALUES (1, 'new'), (2, 'open'), (3, 'pending reminder')`,
		`INSERT INTO ticket_state (id, name, color, type_id, valid_id) VALUES
			(1, 'new', '#50B5FFFF', 1, 1),
			(2, 'open', '#3DD598FF', 2, 1),
			(3, 'Pending custom', '#FFC542FF', 3, 1),
			(4, 'removed', '#8D8D9BFF', 2, 0)`,
		`INSERT INTO ticket (id, ticket_state_id, until_time, change_by) VALUES (1, 1, 0, 7)`,
	}
	for _, q := range seeds {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
	return NewProdHostAPI(WithDB("default", db))
}

func scanTicketState(t *testing.T, h *ProdHostAPI, ticketID int64) (stateID, untilTime, changeBy int64) {
	t.Helper()
	db, _ := h.getDB("")
	if err := db.QueryRow(`SELECT ticket_state_id, until_time, change_by FROM ticket WHERE id = ?`, ticketID).
		Scan(&stateID, &untilTime, &changeBy); err != nil {
		t.Fatalf("scan ticket: %v", err)
	}
	return
}

func TestChangeTicketStatus_PendingRequiresUntil(t *testing.T) {
	h := newTicketStateTestHost(t)
	ctx := context.Background()

	err := h.ChangeTicketStatus(ctx, 1, 3, 7, 0)
	if err == nil {
		t.Fatal("expected error for pending state without until_time")
	}
	if !strings.Contains(err.Error(), "pending time is required") {
		t.Fatalf("unexpected error: %v", err)
	}
	stateID, _, _ := scanTicketState(t, h, 1)
	if stateID != 1 {
		t.Fatalf("ticket must not have moved, state_id = %d", stateID)
	}
}

func TestChangeTicketStatus_PendingSetsUntil(t *testing.T) {
	h := newTicketStateTestHost(t)
	ctx := context.Background()

	if err := h.ChangeTicketStatus(ctx, 1, 3, 7, 1893456000); err != nil {
		t.Fatalf("ChangeTicketStatus: %v", err)
	}
	stateID, untilTime, changeBy := scanTicketState(t, h, 1)
	if stateID != 3 || untilTime != 1893456000 || changeBy != 7 {
		t.Fatalf("got state=%d until=%d by=%d", stateID, untilTime, changeBy)
	}
}

func TestChangeTicketStatus_NonPendingClearsUntil(t *testing.T) {
	h := newTicketStateTestHost(t)
	ctx := context.Background()

	db, _ := h.getDB("")
	if _, err := db.Exec(`UPDATE ticket SET until_time = 1893456000 WHERE id = 1`); err != nil {
		t.Fatalf("seed until: %v", err)
	}

	if err := h.ChangeTicketStatus(ctx, 1, 2, 7, 1893456000); err != nil {
		t.Fatalf("ChangeTicketStatus: %v", err)
	}
	stateID, untilTime, changeBy := scanTicketState(t, h, 1)
	if stateID != 2 || untilTime != 0 || changeBy != 7 {
		t.Fatalf("got state=%d until=%d by=%d", stateID, untilTime, changeBy)
	}
}

func TestChangeTicketStatus_UnknownState(t *testing.T) {
	h := newTicketStateTestHost(t)
	ctx := context.Background()

	if err := h.ChangeTicketStatus(ctx, 1, 99, 7, 0); err == nil {
		t.Fatal("expected error for unknown state")
	}
	if err := h.ChangeTicketStatus(ctx, 1, 4, 7, 0); err == nil {
		t.Fatal("expected error for invalid (valid_id=0) state")
	}
	stateID, _, _ := scanTicketState(t, h, 1)
	if stateID != 1 {
		t.Fatalf("ticket must not have moved, state_id = %d", stateID)
	}
}

func TestChangeTicketStatus_MissingTicket(t *testing.T) {
	h := newTicketStateTestHost(t)
	ctx := context.Background()

	if err := h.ChangeTicketStatus(ctx, 42, 2, 7, 0); err == nil {
		t.Fatal("expected error for missing ticket")
	}
}

func TestListTicketStates(t *testing.T) {
	h := newTicketStateTestHost(t)
	ctx := context.Background()

	states, err := h.ListTicketStates(ctx)
	if err != nil {
		t.Fatalf("ListTicketStates: %v", err)
	}
	if len(states) != 3 {
		t.Fatalf("expected 3 valid states, got %d", len(states))
	}
	byID := make(map[int64]struct {
		name     string
		color    string
		typeName string
	}, len(states))
	for _, s := range states {
		byID[s.ID] = struct {
			name     string
			color    string
			typeName string
		}{s.Name, s.Color, s.TypeName}
	}
	if got := byID[3]; got.name != "Pending custom" || got.color != "#FFC542FF" || got.typeName != "pending reminder" {
		t.Fatalf("state 3 = %+v", got)
	}
	if _, ok := byID[4]; ok {
		t.Fatal("invalid state 4 must not be listed")
	}
}
