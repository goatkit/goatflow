package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/lookups/lookupstest"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/service"
)

// createStateTicket inserts a ticket in the named state for customerLogin.
func createStateTicket(t *testing.T, db *sql.DB, stateName, customerLogin, title string, untilTime int64) int64 {
	t.Helper()
	tn := fmt.Sprintf("NUM-%d", time.Now().UnixNano())
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket (tn, title, queue_id, type_id, ticket_state_id, ticket_priority_id,
		                    ticket_lock_id, user_id, responsible_user_id, customer_user_id,
		                    timeout, until_time, escalation_time, escalation_update_time,
		                    escalation_response_time, escalation_solution_time,
		                    create_time, create_by, change_time, change_by)
		VALUES (?, ?, 1, 1, (SELECT id FROM ticket_state WHERE name = ?), 3, 1, 1, 1, ?,
		        0, ?, 0, 0, 0, 0, NOW(), 1, NOW(), 1)
		RETURNING id`), tn, title, stateName, customerLogin, untilTime)
	require.NoError(t, err)
	t.Cleanup(func() { deleteNumberingTicket(db, id) })
	return id
}

func deleteNumberingTicket(db *sql.DB, id int64) {
	for _, q := range []string{
		`DELETE FROM article_data_mime WHERE article_id IN (SELECT id FROM article WHERE ticket_id = ?)`,
		`DELETE FROM time_accounting WHERE ticket_id = ?`,
		`DELETE FROM ticket_history WHERE ticket_id = ?`,
		`DELETE FROM article WHERE ticket_id = ?`,
		`DELETE FROM ticket WHERE id = ?`,
	} {
		_, _ = db.Exec(database.ConvertPlaceholders(q), id)
	}
}

func ticketStateOf(t *testing.T, db *sql.DB, ticketID int64) (state, stateType string) {
	t.Helper()
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`
		SELECT s.name, st.name FROM ticket t
		JOIN ticket_state s ON s.id = t.ticket_state_id
		JOIN ticket_state_type st ON st.id = s.type_id
		WHERE t.id = ?`), ticketID).Scan(&state, &stateType))
	return state, stateType
}

func historyTypesOf(t *testing.T, db *sql.DB, ticketID int64) []string {
	t.Helper()
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT tht.name FROM ticket_history th
		JOIN ticket_history_type tht ON tht.id = th.history_type_id
		WHERE th.ticket_id = ? ORDER BY th.id`), ticketID)
	require.NoError(t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		require.NoError(t, rows.Scan(&n))
		out = append(out, n)
	}
	require.NoError(t, rows.Err())
	return out
}

func serveNumberingJSON(t *testing.T, r http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// State logic must follow state type NAMES: the same scenario has to behave
// identically on fresh-install numbering and on OTRS-imported numbering.
func TestTicketStateLogicIndependentOfLookupNumbering(t *testing.T) {
	db, err := database.GetDB()
	require.NoError(t, err)
	setupTemplateRenderer(t)
	gin.SetMode(gin.TestMode)

	for _, n := range lookupstest.StateNumberings {
		t.Run(n.Name, func(t *testing.T) {
			lookupstest.UseStateNumbering(t, db, n)
			ctx := context.Background()
			login := fmt.Sprintf("num-%s-%d", n.Name, time.Now().UnixNano())
			tag := fmt.Sprintf("NUMTAG%d", time.Now().UnixNano())

			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set("user_id", uint(1))
				c.Set("username", login)
				c.Set("user_role", "Customer")
				c.Next()
			})
			r.POST("/api/v1/tickets/:id/close", HandleCloseTicketAPI)
			r.POST("/api/v1/tickets/:id/reopen", HandleReopenTicketAPI)
			r.POST("/tickets/:id/close", handleCloseTicket)
			r.POST("/tickets/:id/notes", handleAddTicketNote)
			r.POST("/agent/tickets/:id/merge", handleAgentTicketMerge(db))
			r.POST("/customer/tickets/:id/close", handleCustomerCloseTicket(db))
			r.GET("/customer/tickets", handleCustomerTickets(db))

			// Fresh ticket through the service: NewTicket history, state 'new'.
			svc := service.NewTicketService(repository.NewTicketRepository(db))
			created, err := svc.Create(ctx, service.CreateTicketInput{
				Title: tag + " created", QueueID: 1, PriorityID: 3, UserID: 1, CustomerUserID: login,
			})
			require.NoError(t, err)
			createdID := int64(created.ID)
			t.Cleanup(func() { deleteNumberingTicket(db, createdID) })
			state, _ := ticketStateOf(t, db, createdID)
			assert.Equal(t, "new", state)
			assert.Contains(t, historyTypesOf(t, db, createdID), "NewTicket")

			// Note: AddNote history.
			w := serveNumberingJSON(t, r, http.MethodPost, fmt.Sprintf("/tickets/%d/notes", createdID),
				map[string]any{"content": "note " + tag, "internal": true, "time_units": 1})
			require.Less(t, w.Code, 300, w.Body.String())
			assert.Contains(t, historyTypesOf(t, db, createdID), "AddNote")

			// Agent close without a state: 'closed successful' + StateUpdate history.
			w = serveNumberingJSON(t, r, http.MethodPost, fmt.Sprintf("/tickets/%d/close", createdID),
				map[string]any{"notes": "done " + tag})
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			state, typ := ticketStateOf(t, db, createdID)
			assert.Equal(t, "closed successful", state)
			assert.Equal(t, "closed", typ)
			assert.Contains(t, historyTypesOf(t, db, createdID), "StateUpdate")

			// API close/reopen: already-closed detection and the reopen target.
			w = serveNumberingJSON(t, r, http.MethodPost, fmt.Sprintf("/api/v1/tickets/%d/close", createdID),
				map[string]any{"resolution": "fixed"})
			assert.Equal(t, http.StatusBadRequest, w.Code, "closed ticket must be reported as already closed: %s", w.Body.String())
			w = serveNumberingJSON(t, r, http.MethodPost, fmt.Sprintf("/api/v1/tickets/%d/reopen", createdID),
				map[string]any{"reason": "again"})
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			state, _ = ticketStateOf(t, db, createdID)
			assert.Equal(t, "open", state)
			assert.Contains(t, w.Body.String(), fmt.Sprintf(`"state_id":%d`, n.States["open"]))

			pendingID := createStateTicket(t, db, "pending reminder", login, tag+" pending", time.Now().Add(-time.Hour).Unix())
			w = serveNumberingJSON(t, r, http.MethodPost, fmt.Sprintf("/api/v1/tickets/%d/close", pendingID),
				map[string]any{"resolution": "wontfix"})
			require.Equal(t, http.StatusOK, w.Code, "pending ticket is not closed: %s", w.Body.String())
			state, _ = ticketStateOf(t, db, pendingID)
			assert.Equal(t, "closed unsuccessful", state)

			// Pending reminder job picks pending-reminder tickets only.
			reminderID := createStateTicket(t, db, "pending reminder", login, tag+" reminder", time.Now().Add(-time.Hour).Unix())
			autoID := createStateTicket(t, db, "pending auto close+", login, tag+" auto", time.Now().Add(-time.Hour).Unix())
			due, err := repository.NewTicketRepository(db).FindDuePendingReminders(ctx, time.Now(), 1000)
			require.NoError(t, err)
			dueIDs := map[int64]bool{}
			for _, d := range due {
				dueIDs[int64(d.TicketID)] = true
			}
			assert.True(t, dueIDs[reminderID], "pending reminder ticket not due")
			assert.False(t, dueIDs[autoID], "pending auto ticket picked by reminder job")

			// Customer portal: closed filter lists closed tickets only; customer close.
			openID := createStateTicket(t, db, "open", login, tag+" custopen", 0)
			req := httptest.NewRequest(http.MethodGet, "/customer/tickets?status=closed", nil)
			w = httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code)
			page := w.Body.String()
			assert.True(t, strings.Contains(page, tag+" pending"), "closed ticket missing from closed filter")
			assert.False(t, strings.Contains(page, tag+" custopen"), "open ticket listed as closed")
			assert.False(t, strings.Contains(page, tag+" reminder"), "pending ticket listed as closed")

			req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/customer/tickets/%d/close", openID), strings.NewReader(url.Values{}.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w = httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Less(t, w.Code, 400, w.Body.String())
			_, typ = ticketStateOf(t, db, openID)
			assert.Equal(t, "closed", typ)

			// Merge: the source ends in the OTRS 'merged' state, with history.
			targetID := createStateTicket(t, db, "open", login, tag+" target", 0)
			var targetTN string
			require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT tn FROM ticket WHERE id = ?`), targetID).Scan(&targetTN))
			form := url.Values{"target_tn": {targetTN}}
			req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/agent/tickets/%d/merge", reminderID), strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w = httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			state, typ = ticketStateOf(t, db, reminderID)
			assert.Equal(t, "merged", state)
			assert.Equal(t, "merged", typ)
			assert.Contains(t, historyTypesOf(t, db, targetID), "Merged")
		})
	}
}
