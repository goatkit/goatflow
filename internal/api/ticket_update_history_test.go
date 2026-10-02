package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/lookups"
)

// PUT /api/tickets/:id once answered 200 without writing anything, and
// GET /api/tickets/:id/history returned two hard-coded entries.

// historyByActor returns history type name -> entry name of the ticket's
// history rows created by actor.
func historyByActor(t *testing.T, f *tbaFixture, ticketID int) map[string]string {
	t.Helper()
	rows, err := f.db.Query(database.ConvertPlaceholders(`
		SELECT tht.name, th.name FROM ticket_history th
		JOIN ticket_history_type tht ON tht.id = th.history_type_id
		WHERE th.ticket_id = ? AND th.create_by = ?`), ticketID, f.agent)
	require.NoError(t, err)
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var typ, name string
		require.NoError(t, rows.Scan(&typ, &name))
		out[typ] = name
	}
	require.NoError(t, rows.Err())
	return out
}

func (f *tbaFixture) ticketString(t *testing.T, column string, ticketID int) string {
	t.Helper()
	var v string
	require.NoError(t, f.db.QueryRow(database.ConvertPlaceholders(
		"SELECT "+column+" FROM ticket WHERE id = ?"), ticketID).Scan(&v))
	return v
}

func TestAPITicketUpdate_PersistsWithHistory(t *testing.T) {
	f := newTBAFixture(t)
	ctx := context.Background()
	high, err := lookups.ID(ctx, f.db, lookups.PriorityTable, "4 high")
	require.NoError(t, err)
	open, err := lookups.ID(ctx, f.db, lookups.StateLookup, lookups.StateOpen)
	require.NoError(t, err)

	t.Run("writes the fields and their history", func(t *testing.T) {
		code, resp := f.send(t, http.MethodPut, fmt.Sprintf("/api/tickets/%d", f.ticketRW), map[string]any{
			"title": "Renamed via API", "priority_id": high, "state_id": open, "queue_id": f.queueMove,
		}, nil)
		require.Equal(t, http.StatusOK, code, resp)
		data, _ := resp["data"].(map[string]any)
		assert.Equal(t, "Renamed via API", data["title"], resp)
		assert.Equal(t, float64(high), data["priority_id"], resp)
		assert.Equal(t, float64(open), data["state_id"], resp)
		assert.Equal(t, float64(f.queueMove), data["queue_id"], resp)

		assert.Equal(t, "Renamed via API", f.ticketString(t, "title", f.ticketRW))
		assert.Equal(t, high, f.ticketInt(t, "ticket_priority_id", f.ticketRW))
		assert.Equal(t, open, f.ticketInt(t, "ticket_state_id", f.ticketRW))
		assert.Equal(t, f.queueMove, f.ticketInt(t, "queue_id", f.ticketRW))
		assert.Equal(t, f.agent, f.ticketInt(t, "change_by", f.ticketRW))

		h := historyByActor(t, f, f.ticketRW)
		assert.Len(t, h, 4, h)
		assert.Contains(t, h["TitleUpdate"], "Renamed via API")
		assert.Contains(t, h["PriorityUpdate"], "4 high")
		assert.Contains(t, h["StateUpdate"], lookups.StateOpen)
		assert.Contains(t, h["Move"], "tba_move_")
	})

	t.Run("read-only ticket is refused", func(t *testing.T) {
		code, resp := f.send(t, http.MethodPut, fmt.Sprintf("/api/tickets/%d", f.ticketRO),
			map[string]any{"title": "hijacked"}, nil)
		assert.Equal(t, http.StatusForbidden, code, resp)
		assert.Equal(t, "tba ticket", f.ticketString(t, "title", f.ticketRO))
		assert.Empty(t, historyByActor(t, f, f.ticketRO))
	})

	t.Run("invalid values are rejected without writing", func(t *testing.T) {
		for _, body := range []map[string]any{
			{"user_id": 999999999},
			{"responsible_user_id": nil},
			{"queue_id": "not a number"},
			{"ticket_lock_id": 999},
			{"unknown_field": 1},
		} {
			code, resp := f.send(t, http.MethodPut, fmt.Sprintf("/api/tickets/%d", f.ticketRW2), body, nil)
			assert.Equal(t, http.StatusBadRequest, code, "%v -> %v", body, resp)
		}
		assert.Equal(t, 1, f.ticketInt(t, "user_id", f.ticketRW2))
		assert.Equal(t, 1, f.ticketInt(t, "responsible_user_id", f.ticketRW2))
		assert.Equal(t, f.queueRW, f.ticketInt(t, "queue_id", f.ticketRW2))
		assert.Equal(t, 1, f.ticketInt(t, "change_by", f.ticketRW2))
		assert.Empty(t, historyByActor(t, f, f.ticketRW2))
	})

	t.Run("owner and responsible changes are recorded", func(t *testing.T) {
		code, resp := f.send(t, http.MethodPut, fmt.Sprintf("/api/tickets/%d", f.ticketRW2),
			map[string]any{"user_id": f.agent, "responsible_user_id": f.agent, "ticket_lock_id": 2}, nil)
		require.Equal(t, http.StatusOK, code, resp)
		assert.Equal(t, f.agent, f.ticketInt(t, "user_id", f.ticketRW2))
		assert.Equal(t, f.agent, f.ticketInt(t, "responsible_user_id", f.ticketRW2))
		assert.Equal(t, 2, f.ticketInt(t, "ticket_lock_id", f.ticketRW2))
		h := historyByActor(t, f, f.ticketRW2)
		assert.Len(t, h, 3, h)
		assert.Contains(t, h, "OwnerUpdate")
		assert.Contains(t, h, "ResponsibleUpdate")
		assert.Contains(t, h, "Lock")
	})
}

func TestAPITicketHistory_ReadsTicketHistory(t *testing.T) {
	f := newTBAFixture(t)
	var login string
	require.NoError(t, f.db.QueryRow(database.ConvertPlaceholders(
		"SELECT login FROM users WHERE id = ?"), f.agent).Scan(&login))

	code, resp := f.send(t, http.MethodPut, fmt.Sprintf("/api/tickets/%d", f.ticketRW),
		map[string]any{"title": "History probe"}, nil)
	require.Equal(t, http.StatusOK, code, resp)

	code, resp = f.send(t, http.MethodGet, fmt.Sprintf("/api/tickets/%d/history", f.ticketRW), nil, nil)
	require.Equal(t, http.StatusOK, code, resp)
	assert.Equal(t, float64(f.ticketRW), resp["ticket_id"], resp)
	entries, _ := resp["history"].([]any)
	require.Len(t, entries, 1, resp)
	e := entries[0].(map[string]any)
	assert.Equal(t, "TitleUpdate", e["history_type"], e)
	assert.Contains(t, e["name"], "History probe", e)
	assert.Contains(t, e["queue"], "tba_rw_", e)
	by, _ := e["created_by"].(map[string]any)
	assert.Equal(t, login, by["login"], e)
	assert.Equal(t, "Body Authz", by["name"], e)

	// A ticket without history has an empty history (not invented rows).
	code, resp = f.send(t, http.MethodGet, fmt.Sprintf("/api/tickets/%d/history", f.ticketRO), nil, nil)
	require.Equal(t, http.StatusOK, code, resp)
	assert.Equal(t, []any{}, resp["history"], resp)

	code, resp = f.send(t, http.MethodGet, fmt.Sprintf("/api/tickets/%d/history", f.ticketNone), nil, nil)
	assert.Equal(t, http.StatusForbidden, code, resp)
	code, resp = f.send(t, http.MethodGet, fmt.Sprintf("/api/tickets/%d/history?limit=0", f.ticketRW), nil, nil)
	assert.Equal(t, http.StatusBadRequest, code, resp)
}
