package ticketstate

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	for _, q := range []string{
		`CREATE TABLE ticket_state_type (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`,
		`CREATE TABLE ticket_state (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			type_id INTEGER NOT NULL,
			valid_id INTEGER NOT NULL DEFAULT 1)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("exec ddl: %v", err)
		}
	}
	if _, err := db.Exec(`
		INSERT INTO ticket_state_type (id, name) VALUES
			(1, 'new'), (2, 'open'), (3, 'pending reminder'), (4, 'pending auto')`); err != nil {
		t.Fatalf("seed types: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO ticket_state (id, name, type_id, valid_id) VALUES
			(1, 'new', 1, 1),
			(2, 'open', 2, 1),
			(3, 'Pending custom', 3, 1),
			(4, 'Auto close+', 4, 1),
			(5, 'removed', 2, 0)`); err != nil {
		t.Fatalf("seed states: %v", err)
	}
	return db
}

func TestStateIsPending(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	cases := []struct {
		stateID int64
		want    bool
	}{
		{1, false}, // type 'new'
		{2, false}, // type 'open'
		{3, true},  // state "Pending custom", type 'pending reminder'
		{4, true},  // state "Auto close+", type 'pending auto'
	}
	for _, tc := range cases {
		got, err := StateIsPending(ctx, db, tc.stateID)
		if err != nil {
			t.Fatalf("state %d: %v", tc.stateID, err)
		}
		if got != tc.want {
			t.Errorf("state %d: got %v want %v", tc.stateID, got, tc.want)
		}
	}

	for _, missing := range []int64{99, 5} { // unknown id, invalid state
		if _, err := StateIsPending(ctx, db, missing); err == nil {
			t.Errorf("state %d: expected error", missing)
		}
	}
}
