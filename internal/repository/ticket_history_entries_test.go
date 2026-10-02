//go:build integration

package repository

import (
	"fmt"
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// History rows written in the same second (e.g. state + owner change on one save) share create_time;
// the timeline must still come back newest-first by id, identically on every driver. History of other
// tickets is present so the planner uses the ticket_id index plus a sort (the realistic plan), where
// tie order is otherwise unspecified.
func TestGetTicketHistoryEntries_TiedCreateTimeOrderedByIDDesc(t *testing.T) {
	db, err := getTestDB()
	if err != nil {
		t.Fatalf("failed to get test db: %v", err)
	}
	var ids []int64
	t.Cleanup(func() {
		del := database.ConvertPlaceholders(`DELETE FROM ticket_history WHERE id = ?`)
		for _, id := range ids {
			_, _ = db.Exec(del, id)
		}
		db.Close()
	})

	tickets := make([]int64, 0, 2)
	rows, err := db.Query(database.ConvertPlaceholders(`SELECT id FROM ticket ORDER BY id LIMIT 2`))
	if err != nil {
		t.Fatalf("load tickets: %v", err)
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan ticket: %v", err)
		}
		tickets = append(tickets, id)
	}
	rows.Close()
	if len(tickets) != 2 {
		t.Fatalf("test DB needs two seeded tickets, found %d", len(tickets))
	}
	ticketID, otherTicketID := tickets[0], tickets[1]

	var typeID, queueID, ownerID, priorityID, stateID int64
	err = db.QueryRow(database.ConvertPlaceholders(`SELECT type_id, queue_id, user_id, ticket_priority_id, ticket_state_id
		FROM ticket WHERE id = ?`), ticketID).Scan(&typeID, &queueID, &ownerID, &priorityID, &stateID)
	if err != nil {
		t.Fatalf("load ticket %d: %v", ticketID, err)
	}
	var historyTypeID int64
	if err := db.QueryRow(database.ConvertPlaceholders(`SELECT MIN(id) FROM ticket_history_type`)).Scan(&historyTypeID); err != nil {
		t.Fatalf("load history type: %v", err)
	}

	insert := database.ConvertPlaceholders(`INSERT INTO ticket_history (
		name, history_type_id, ticket_id, type_id, queue_id, owner_id, priority_id, state_id,
		create_time, create_by, change_time, change_by
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?) RETURNING id`)
	add := func(name string, ticket int64, at time.Time) int64 {
		t.Helper()
		id, err := database.GetAdapter().InsertWithReturning(db, insert,
			name, historyTypeID, ticket, typeID, queueID, ownerID, priorityID, stateID, at, 1, at, 1)
		if err != nil {
			t.Fatalf("insert history row %q: %v", name, err)
		}
		ids = append(ids, id)
		return id
	}

	// Newer than anything for our ticket, so the time index alone would not find our rows quickly.
	noiseAt := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range 300 {
		add(fmt.Sprintf("tie-order-noise-%d", i), otherTicketID, noiseAt)
	}

	// Later than any seeded history so these rows are the newest for the ticket.
	tied := time.Date(2099, 1, 2, 3, 4, 5, 0, time.UTC)
	const n = 6
	tiedIDs := make([]int64, 0, n)
	for i := range n {
		tiedIDs = append(tiedIDs, add(fmt.Sprintf("tie-order-%d", i), ticketID, tied))
	}

	entries, err := NewTicketRepository(db).GetTicketHistoryEntries(uint(ticketID), n)
	if err != nil {
		t.Fatalf("GetTicketHistoryEntries: %v", err)
	}
	if len(entries) != n {
		t.Fatalf("got %d entries, want %d", len(entries), n)
	}
	for i, e := range entries {
		want := tiedIDs[n-1-i]
		if int64(e.ID) != want {
			got := make([]uint, len(entries))
			for j := range entries {
				got[j] = entries[j].ID
			}
			t.Fatalf("entry %d id = %d, want %d (got order %v, inserted %v)", i, e.ID, want, got, tiedIDs)
		}
		if !e.CreatedAt.Equal(tied) {
			t.Fatalf("entry %d create_time = %v, want %v", i, e.CreatedAt, tied)
		}
	}
}
