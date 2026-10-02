// Package lookupstest provides test helpers for lookup numbering.
package lookupstest

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/lookups"
)

// StateNumbering assigns ticket_state_type and ticket_state ids by name.
type StateNumbering struct {
	Name       string
	StateTypes map[string]int
	States     map[string]int
}

// GoatFlowStateNumbering is the numbering of a fresh GoatFlow install
// (000002 seed plus the 000029 OTRS defaults).
var GoatFlowStateNumbering = StateNumbering{
	Name: "goatflow",
	StateTypes: map[string]int{
		"new": 1, "open": 2, "pending reminder": 3, "pending auto": 4, "closed": 5, "removed": 6, "merged": 7,
	},
	States: map[string]int{
		"new": 1, "open": 2, "pending reminder": 3, "closed successful": 4, "closed unsuccessful": 5,
		"pending auto close+": 6, "pending auto close-": 7, "removed": 8, "merged": 9,
	},
}

// OTRSStateNumbering is the numbering of a database imported from OTRS/Znuny.
var OTRSStateNumbering = StateNumbering{
	Name: "otrs",
	StateTypes: map[string]int{
		"new": 1, "open": 2, "closed": 3, "pending reminder": 4, "pending auto": 5, "removed": 6, "merged": 7,
	},
	States: map[string]int{
		"new": 1, "closed successful": 2, "closed unsuccessful": 3, "open": 4, "removed": 5,
		"pending reminder": 6, "pending auto close+": 7, "pending auto close-": 8, "merged": 9,
	},
}

// StateNumberings lists both numberings so tests can run once per numbering.
var StateNumberings = []StateNumbering{GoatFlowStateNumbering, OTRSStateNumbering}

type renumberTable struct {
	table    string
	cols     string // copied columns besides id and name
	children [][2]string
}

var (
	stateTypeTable = renumberTable{
		table:    "ticket_state_type",
		cols:     "comments, create_time, create_by, change_time, change_by",
		children: [][2]string{{"ticket_state", "type_id"}},
	}
	stateTable = renumberTable{
		table: "ticket_state",
		cols:  "comments, type_id, valid_id, create_time, create_by, change_time, change_by, color",
		children: [][2]string{
			{"ticket", "ticket_state_id"},
			{"ticket_history", "state_id"},
		},
	}
)

// UseStateNumbering renumbers the ticket_state_type and ticket_state rows of
// db to n (rows are moved, references follow, foreign keys stay enforced) and
// restores the original ids when the test ends. Tests using it must not run
// in parallel with other tests of the same database.
func UseStateNumbering(t *testing.T, db *sql.DB, n StateNumbering) {
	t.Helper()
	origTypes := currentIDs(t, db, stateTypeTable.table, n.StateTypes)
	origStates := currentIDs(t, db, stateTable.table, n.States)
	t.Cleanup(func() {
		// States first: their type_id references must stay valid while types move.
		renumber(t, db, stateTable, origStates)
		renumber(t, db, stateTypeTable, origTypes)
		lookups.Invalidate(db)
	})
	renumber(t, db, stateTable, n.States)
	renumber(t, db, stateTypeTable, n.StateTypes)
	lookups.Invalidate(db)
}

func currentIDs(t *testing.T, db *sql.DB, table string, names map[string]int) map[string]int {
	t.Helper()
	out := make(map[string]int, len(names))
	for name := range names {
		var id int
		err := db.QueryRow(database.ConvertPlaceholders("SELECT id FROM "+table+" WHERE name = ?"), name).Scan(&id)
		if err != nil {
			t.Fatalf("UseStateNumbering: %s %q: %v", table, name, err)
		}
		out[name] = id
	}
	return out
}

func renumber(t *testing.T, db *sql.DB, tb renumberTable, target map[string]int) {
	t.Helper()
	const tmpOffset = 1000
	const tmpSuffix = " #renumber"
	current := currentIDs(t, db, tb.table, target)
	// Target ids must be free or held by a row that is being moved.
	for name, id := range target {
		var holder string
		err := db.QueryRow(database.ConvertPlaceholders("SELECT name FROM "+tb.table+" WHERE id = ?"), id).Scan(&holder)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			t.Fatalf("UseStateNumbering: %s id %d: %v", tb.table, id, err)
		}
		if _, moving := target[holder]; !moving {
			t.Fatalf("UseStateNumbering: %s id %d (wanted for %q) is held by unmanaged row %q", tb.table, id, name, holder)
		}
	}
	moved := map[string]int{}
	for name, id := range target {
		if current[name] == id {
			continue
		}
		move(t, db, tb, current[name], id+tmpOffset, name+tmpSuffix)
		moved[name] = id
	}
	for name, id := range moved {
		move(t, db, tb, id+tmpOffset, id, name)
	}
}

func move(t *testing.T, db *sql.DB, tb renumberTable, from, to int, name string) {
	t.Helper()
	stmts := []struct {
		q    string
		args []any
	}{
		{"INSERT INTO " + tb.table + " (id, name, " + tb.cols + ") SELECT ?, ?, " + tb.cols + " FROM " + tb.table + " WHERE id = ?", []any{to, name, from}},
	}
	for _, c := range tb.children {
		stmts = append(stmts, struct {
			q    string
			args []any
		}{"UPDATE " + c[0] + " SET " + c[1] + " = ? WHERE " + c[1] + " = ?", []any{to, from}})
	}
	stmts = append(stmts, struct {
		q    string
		args []any
	}{"DELETE FROM " + tb.table + " WHERE id = ?", []any{from}})
	for _, s := range stmts {
		if _, err := db.Exec(database.ConvertPlaceholders(s.q), s.args...); err != nil {
			t.Fatalf("UseStateNumbering: %s: %v", s.q, fmt.Errorf("move %d->%d: %w", from, to, err))
		}
	}
}
