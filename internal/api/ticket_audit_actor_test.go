package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// ticketActorRouter mounts the ticket and article write handlers behind a
// middleware that authenticates as actor, storing the id with the Go type the
// given auth path uses (int for API tokens/JWT API, uint for the agent UI
// session). actor 0 means "authenticated flag without a user id".
func ticketActorRouter(actor int, asUint bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("is_authenticated", true)
		if actor > 0 {
			if asUint {
				c.Set("user_id", uint(actor))
			} else {
				c.Set("user_id", actor)
			}
		}
		c.Next()
	})
	r.PUT("/tickets/:id", HandleUpdateTicketAPI)
	r.POST("/tickets/:id/close", HandleCloseTicketAPI)
	r.POST("/tickets/:id/priority", handleUpdateTicketPriority)
	r.POST("/tickets/:id/notes", handleAddTicketNote)
	r.POST("/tickets/:id/time", handleAddTicketTime)
	r.POST("/tickets/:id/articles", HandleCreateArticleAPI)
	r.POST("/tickets/:id/internal-notes", HandleCreateInternalNote)
	return r
}

type ticketActorCase struct {
	name     string
	method   string
	path     string // %d = ticket id
	json     any
	form     url.Values
	stamped  string // query returning the audit column written by the request; arg = ticket id
	wantCode int
}

func ticketActorCases() []ticketActorCase {
	return []ticketActorCase{
		{name: "update ticket", method: http.MethodPut, path: "/tickets/%d", json: map[string]any{"title": "audit actor"},
			stamped: `SELECT change_by FROM ticket WHERE id = ?`, wantCode: http.StatusOK},
		{name: "close ticket API", method: http.MethodPost, path: "/tickets/%d/close", json: map[string]any{"resolution": "resolved"},
			stamped: `SELECT change_by FROM ticket WHERE id = ?`, wantCode: http.StatusOK},
		{name: "set priority", method: http.MethodPost, path: "/tickets/%d/priority", form: url.Values{"priority": {"4"}},
			stamped: `SELECT change_by FROM ticket WHERE id = ?`, wantCode: http.StatusOK},
		{name: "add note", method: http.MethodPost, path: "/tickets/%d/notes", json: map[string]any{"content": "audit note"},
			stamped: `SELECT create_by FROM article WHERE ticket_id = ? ORDER BY id DESC LIMIT 1`, wantCode: http.StatusCreated},
		{name: "add time", method: http.MethodPost, path: "/tickets/%d/time", json: map[string]any{"time_units": 7},
			stamped: `SELECT create_by FROM time_accounting WHERE ticket_id = ? ORDER BY id DESC LIMIT 1`, wantCode: http.StatusOK},
		{name: "create article", method: http.MethodPost, path: "/tickets/%d/articles", json: map[string]any{"subject": "s", "body": "audit article"},
			stamped: `SELECT create_by FROM article WHERE ticket_id = ? ORDER BY id DESC LIMIT 1`, wantCode: http.StatusCreated},
		{name: "create internal note", method: http.MethodPost, path: "/tickets/%d/internal-notes", json: map[string]any{"content": "audit internal"},
			stamped: `SELECT create_by FROM article WHERE ticket_id = ? ORDER BY id DESC LIMIT 1`, wantCode: http.StatusCreated},
	}
}

func ticketActorDo(t *testing.T, r *gin.Engine, tc ticketActorCase, ticketID int64) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	path := fmt.Sprintf(tc.path, ticketID)
	if tc.form != nil {
		req = httptest.NewRequest(tc.method, path, strings.NewReader(tc.form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		var buf bytes.Buffer
		require.NoError(t, json.NewEncoder(&buf).Encode(tc.json))
		req = httptest.NewRequest(tc.method, path, &buf)
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// grantQueueOneRW gives agent rw on the group of queue 1 (removed with the
// agent's group_user rows at cleanup).
func grantQueueOneRW(t *testing.T, agentID int) {
	t.Helper()
	db := isolatedDB(t)
	_, err := db.Exec(database.ConvertPlaceholders(`
		INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
		SELECT ?, group_id, 'rw', CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1 FROM queue WHERE id = 1`), agentID)
	require.NoError(t, err)
}

// Ticket and article writes stamp create_by/change_by with the acting agent,
// whatever Go type the auth middleware stored the id as, never with user 1.
func TestTicketWritesStampActingAgent(t *testing.T) {
	if !dbAvailable() {
		t.Skip("DB not available")
	}
	db := isolatedDB(t)

	for _, idType := range []struct {
		name   string
		asUint bool
	}{{"int user_id", false}, {"uint user_id", true}} {
		for _, tc := range ticketActorCases() {
			t.Run(idType.name+"/"+tc.name, func(t *testing.T) {
				agentID, _ := createIsolatedAgent(t, "audit_actor")
				require.NotEqual(t, 1, agentID)
				grantQueueOneRW(t, agentID)
				ticketID := createArticleTestTicket(t, db, "audit-actor@example.test")

				w := ticketActorDo(t, ticketActorRouter(agentID, idType.asUint), tc, ticketID)
				require.Equal(t, tc.wantCode, w.Code, w.Body.String())

				var stamped int
				require.NoError(t, db.QueryRow(database.ConvertPlaceholders(tc.stamped), ticketID).Scan(&stamped))
				assert.Equal(t, agentID, stamped, "audit column must be the acting agent")
			})
		}
	}
}

// Without an authenticated user id the writes are refused with 401 and
// nothing is attributed to user 1.
func TestTicketWritesWithoutUserIDAreUnauthorized(t *testing.T) {
	if !dbAvailable() {
		t.Skip("DB not available")
	}
	db := isolatedDB(t)
	r := ticketActorRouter(0, false)

	for _, tc := range ticketActorCases() {
		t.Run(tc.name, func(t *testing.T) {
			ticketID := createArticleTestTicket(t, db, "audit-anon@example.test")
			var beforeTitle string
			var beforeState, beforePriority int
			require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
				`SELECT title, ticket_state_id, ticket_priority_id FROM ticket WHERE id = ?`), ticketID).
				Scan(&beforeTitle, &beforeState, &beforePriority))

			w := ticketActorDo(t, r, tc, ticketID)
			assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())

			var title string
			var state, priority int
			require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
				`SELECT title, ticket_state_id, ticket_priority_id FROM ticket WHERE id = ?`), ticketID).
				Scan(&title, &state, &priority))
			assert.Equal(t, []any{beforeTitle, beforeState, beforePriority}, []any{title, state, priority})
			assert.Zero(t, articleCountRows(t, db, `SELECT COUNT(*) FROM article WHERE ticket_id = ?`, ticketID))
			assert.Zero(t, articleCountRows(t, db, `SELECT COUNT(*) FROM time_accounting WHERE ticket_id = ?`, ticketID))
		})
	}
}
