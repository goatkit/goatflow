package api

import (
	"bytes"
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

	"github.com/goatkit/goatflow/internal/platform/customfields"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/routing"
)

// tbaFixture is a non-admin agent with explicit queue permissions and tickets
// in queues it may write (rw), only read (ro) and not see at all (none). The
// route middleware checks only the ticket in the path or "some queue"; these
// tests cover the tickets and queues named in the request body.
type tbaFixture struct {
	db     *sql.DB
	router *gin.Engine
	token  string
	agent  int

	queueRW, queueRO, queueNone, queueMove int

	ticketRW, ticketRW2, ticketRO, ticketNone int
	tnRW, tnRO, tnNone                        string
	articleRO, articleNone                    int64
}

func newTBAFixture(t *testing.T) *tbaFixture {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)

	f := &tbaFixture{db: db}
	sfx := fmt.Sprintf("%d", time.Now().UnixNano())
	var groupIDs, queueIDs []int
	t.Cleanup(func() {
		in := func(ids []int) (string, []any) {
			args := make([]any, len(ids))
			for i, id := range ids {
				args[i] = id
			}
			return strings.TrimSuffix(strings.Repeat("?,", len(ids)), ","), args
		}
		qp, qargs := in(queueIDs)
		gp, gargs := in(groupIDs)
		for _, q := range []struct {
			sql  string
			args []any
		}{
			{"DELETE FROM article_data_mime WHERE article_id IN (SELECT id FROM article WHERE ticket_id IN (SELECT id FROM ticket WHERE queue_id IN (" + qp + ")))", qargs},
			{"DELETE FROM ticket_history WHERE ticket_id IN (SELECT id FROM ticket WHERE queue_id IN (" + qp + "))", qargs},
			{"DELETE FROM article WHERE ticket_id IN (SELECT id FROM ticket WHERE queue_id IN (" + qp + "))", qargs},
			{"DELETE FROM ticket WHERE queue_id IN (" + qp + ")", qargs},
			{"DELETE FROM group_user WHERE user_id = ? OR group_id IN (" + gp + ")", append([]any{f.agent}, gargs...)},
			// Rows other code adds for every group (e.g. customer company
			// group grants) must go before the groups.
			{"DELETE FROM group_customer WHERE group_id IN (" + gp + ")", gargs},
			{"DELETE FROM group_customer_user WHERE group_id IN (" + gp + ")", gargs},
			{"DELETE FROM group_role WHERE group_id IN (" + gp + ")", gargs},
			{"DELETE FROM queue WHERE id IN (" + qp + ")", qargs},
			{"DELETE FROM `groups` WHERE id IN (" + gp + ")", gargs},
			{"DELETE FROM users WHERE id = ?", []any{f.agent}},
		} {
			if _, err := db.Exec(database.ConvertPlaceholders(q.sql), q.args...); err != nil {
				t.Errorf("cleanup %q: %v", q.sql, err)
			}
		}
	})

	mustID := func(id int64, err error) int {
		t.Helper()
		require.NoError(t, err)
		return int(id)
	}
	newQueue := func(name string) (queueID, groupID int) {
		groupID = mustID(database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(
			"INSERT INTO `groups` (name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (?, 'tba test', 1, NOW(), 1, NOW(), 1) RETURNING id"),
			"tba_"+name+"_"+sfx))
		groupIDs = append(groupIDs, groupID)
		queueID = mustID(database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
			INSERT INTO queue (name, group_id, system_address_id, salutation_id, signature_id,
				follow_up_id, follow_up_lock, comments, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 1, 1, 1, 1, 0, 'tba test', 1, NOW(), 1, NOW(), 1) RETURNING id`),
			"tba_"+name+"_"+sfx, groupID))
		queueIDs = append(queueIDs, queueID)
		return queueID, groupID
	}
	var groupRW, groupRO, groupMove int
	f.queueRW, groupRW = newQueue("rw")
	f.queueRO, groupRO = newQueue("ro")
	f.queueNone, _ = newQueue("none")
	f.queueMove, groupMove = newQueue("move")

	login := "tba_agent_" + sfx
	f.agent = mustID(database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 'x', 'Body', 'Authz', 1, NOW(), 1, NOW(), 1) RETURNING id`), login))
	for _, g := range []struct {
		group int
		key   string
	}{{groupRW, "rw"}, {groupRO, "ro"}, {groupMove, "move_into"}} {
		_, err := db.Exec(database.ConvertPlaceholders(`
			INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, NOW(), 1, NOW(), 1)`), f.agent, g.group, g.key)
		require.NoError(t, err)
	}

	newTicket := func(queueID int) (int, string) {
		tn := fmt.Sprintf("TBA%d%d", queueID, time.Now().UnixNano())
		id := mustID(database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
			INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, type_id, user_id, responsible_user_id,
				ticket_priority_id, ticket_state_id, customer_id, customer_user_id, timeout, until_time,
				escalation_time, escalation_update_time, escalation_response_time, escalation_solution_time,
				archive_flag, create_time, create_by, change_time, change_by)
			VALUES (?, 'tba ticket', ?, 1, 1, 1, 1, 3, 1, 'tba-co', 'tba@example.com', 0, 0, 0, 0, 0, 0, 0,
				NOW(), 1, NOW(), 1) RETURNING id`), tn, queueID))
		return id, tn
	}
	newArticle := func(ticketID int) int64 {
		id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
			INSERT INTO article (ticket_id, article_sender_type_id, communication_channel_id, is_visible_for_customer,
				search_index_needs_rebuild, create_time, create_by, change_time, change_by)
			VALUES (?, 1, 3, 0, 1, NOW(), 1, NOW(), 1) RETURNING id`), ticketID)
		require.NoError(t, err)
		return id
	}
	f.ticketRW, f.tnRW = newTicket(f.queueRW)
	f.ticketRW2, _ = newTicket(f.queueRW)
	f.ticketRO, f.tnRO = newTicket(f.queueRO)
	f.ticketNone, f.tnNone = newTicket(f.queueNone)
	f.articleRO = newArticle(f.ticketRO)
	f.articleNone = newArticle(f.ticketNone)

	f.token = testSessionToken(t, uint(f.agent), login, login, "Agent", false, 0)

	gin.SetMode(gin.TestMode)
	f.router = gin.New()
	require.NoError(t, routing.LoadYAMLRoutesForTesting(f.router))
	return f
}

func (f *tbaFixture) send(t *testing.T, method, path string, jsonBody any, form url.Values) (int, map[string]any) {
	t.Helper()
	var req *http.Request
	switch {
	case jsonBody != nil:
		b, err := json.Marshal(jsonBody)
		require.NoError(t, err)
		req = httptest.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
	case form != nil:
		req = httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	default:
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Authorization", "Bearer "+f.token)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return w.Code, resp
}

func (f *tbaFixture) ticketInt(t *testing.T, column string, ticketID int) int {
	t.Helper()
	var v int
	require.NoError(t, f.db.QueryRow(database.ConvertPlaceholders(
		"SELECT "+column+" FROM ticket WHERE id = ?"), ticketID).Scan(&v))
	return v
}

func (f *tbaFixture) articleTicket(t *testing.T, articleID int64) int {
	t.Helper()
	var v int
	require.NoError(t, f.db.QueryRow(database.ConvertPlaceholders(
		"SELECT ticket_id FROM article WHERE id = ?"), articleID).Scan(&v))
	return v
}

// bulkErrors asserts a bulk result: which tickets succeeded and the per-ticket
// errors for the refused ones.
func bulkErrors(t *testing.T, code int, resp map[string]any, succeeded int, errs ...string) {
	t.Helper()
	require.Equal(t, http.StatusOK, code, resp)
	assert.Equal(t, float64(succeeded), resp["succeeded"], resp)
	assert.Equal(t, float64(len(errs)), resp["failed"], resp)
	got := []string{}
	if list, ok := resp["errors"].([]any); ok {
		for _, e := range list {
			got = append(got, e.(string))
		}
	}
	assert.ElementsMatch(t, errs, got)
}

func TestBulkTicketActions_CheckEachTicket(t *testing.T) {
	f := newTBAFixture(t)
	denied := func(id int) string { return fmt.Sprintf("Ticket %d: permission denied", id) }
	missing := func(id int) string { return fmt.Sprintf("Ticket %d: not found", id) }
	ids := []int{f.ticketRW, f.ticketRO, f.ticketNone}

	t.Run("status needs rw on each ticket", func(t *testing.T) {
		var closed int
		require.NoError(t, f.db.QueryRow(database.ConvertPlaceholders(
			`SELECT id FROM ticket_state WHERE name = 'closed successful'`)).Scan(&closed))
		code, resp := f.send(t, http.MethodPost, "/agent/tickets/bulk/status",
			gin.H{"ticket_ids": ids, "status_id": closed}, nil)
		bulkErrors(t, code, resp, 1, denied(f.ticketRO), missing(f.ticketNone))
		assert.Equal(t, closed, f.ticketInt(t, "ticket_state_id", f.ticketRW))
		assert.Equal(t, 1, f.ticketInt(t, "ticket_state_id", f.ticketRO))
		assert.Equal(t, 1, f.ticketInt(t, "ticket_state_id", f.ticketNone))
	})

	t.Run("priority needs priority on each ticket", func(t *testing.T) {
		code, resp := f.send(t, http.MethodPost, "/agent/tickets/bulk/priority",
			gin.H{"ticket_ids": ids, "priority_id": 5}, nil)
		bulkErrors(t, code, resp, 1, denied(f.ticketRO), missing(f.ticketNone))
		assert.Equal(t, 5, f.ticketInt(t, "ticket_priority_id", f.ticketRW))
		assert.Equal(t, 3, f.ticketInt(t, "ticket_priority_id", f.ticketRO))
		assert.Equal(t, 3, f.ticketInt(t, "ticket_priority_id", f.ticketNone))
	})

	t.Run("assign needs owner on each ticket", func(t *testing.T) {
		code, resp := f.send(t, http.MethodPost, "/agent/tickets/bulk/assign",
			gin.H{"ticket_ids": ids, "user_id": f.agent}, nil)
		bulkErrors(t, code, resp, 1, denied(f.ticketRO), missing(f.ticketNone))
		assert.Equal(t, f.agent, f.ticketInt(t, "user_id", f.ticketRW))
		assert.Equal(t, 1, f.ticketInt(t, "user_id", f.ticketRO))
		assert.Equal(t, 1, f.ticketInt(t, "user_id", f.ticketNone))
	})

	t.Run("lock needs rw on each ticket", func(t *testing.T) {
		code, resp := f.send(t, http.MethodPost, "/agent/tickets/bulk/lock",
			gin.H{"ticket_ids": ids, "lock": true}, nil)
		bulkErrors(t, code, resp, 1, denied(f.ticketRO), missing(f.ticketNone))
		assert.Equal(t, 2, f.ticketInt(t, "ticket_lock_id", f.ticketRW))
		assert.Equal(t, 1, f.ticketInt(t, "ticket_lock_id", f.ticketRO))
		assert.Equal(t, 1, f.ticketInt(t, "ticket_lock_id", f.ticketNone))
	})

	t.Run("queue move needs move_into on the target queue", func(t *testing.T) {
		for _, target := range []int{f.queueNone, f.queueRO} {
			code, resp := f.send(t, http.MethodPost, "/agent/tickets/bulk/queue",
				gin.H{"ticket_ids": []int{f.ticketRW2}, "queue_id": target}, nil)
			assert.Equal(t, http.StatusForbidden, code, resp)
			assert.Equal(t, f.queueRW, f.ticketInt(t, "queue_id", f.ticketRW2))
		}
	})

	t.Run("queue move needs move_into on each ticket", func(t *testing.T) {
		code, resp := f.send(t, http.MethodPost, "/agent/tickets/bulk/queue",
			gin.H{"ticket_ids": []int{f.ticketRW2, f.ticketRO, f.ticketNone}, "queue_id": f.queueMove}, nil)
		bulkErrors(t, code, resp, 1, denied(f.ticketRO), missing(f.ticketNone))
		assert.Equal(t, f.queueMove, f.ticketInt(t, "queue_id", f.ticketRW2))
		assert.Equal(t, f.queueRO, f.ticketInt(t, "queue_id", f.ticketRO))
		assert.Equal(t, f.queueNone, f.ticketInt(t, "queue_id", f.ticketNone))
	})
}

func TestBulkTicketMerge_ChecksTargetAndSources(t *testing.T) {
	f := newTBAFixture(t)

	t.Run("target the agent cannot see is not found", func(t *testing.T) {
		code, resp := f.send(t, http.MethodPost, "/agent/tickets/bulk/merge",
			gin.H{"ticket_ids": []int{f.ticketRW}, "target_ticket_id": f.ticketNone}, nil)
		assert.Equal(t, http.StatusNotFound, code, resp)
		assert.Equal(t, 1, f.ticketInt(t, "ticket_state_id", f.ticketRW))
	})

	t.Run("read-only target is forbidden", func(t *testing.T) {
		code, resp := f.send(t, http.MethodPost, "/agent/tickets/bulk/merge",
			gin.H{"ticket_ids": []int{f.ticketRW}, "target_ticket_id": f.ticketRO}, nil)
		assert.Equal(t, http.StatusForbidden, code, resp)
		assert.Equal(t, 1, f.ticketInt(t, "ticket_state_id", f.ticketRW))
	})

	t.Run("sources need rw", func(t *testing.T) {
		code, resp := f.send(t, http.MethodPost, "/agent/tickets/bulk/merge",
			gin.H{"ticket_ids": []int{f.ticketRO, f.ticketNone}, "target_ticket_id": f.ticketRW}, nil)
		require.Equal(t, http.StatusOK, code, resp)
		assert.Equal(t, float64(0), resp["succeeded"], resp)
		assert.Equal(t, float64(2), resp["failed"], resp)
		assert.Equal(t, f.ticketRO, f.articleTicket(t, f.articleRO))
		assert.Equal(t, f.ticketNone, f.articleTicket(t, f.articleNone))
		assert.Equal(t, 1, f.ticketInt(t, "ticket_state_id", f.ticketRO))
		assert.Equal(t, 1, f.ticketInt(t, "ticket_state_id", f.ticketNone))
	})
}

func TestAgentTicketMerge_ChecksTarget(t *testing.T) {
	f := newTBAFixture(t)
	// Source is writable; the target from the form must be writable too.
	path := fmt.Sprintf("/agent/tickets/%d/merge", f.ticketRW)

	code, resp := f.send(t, http.MethodPost, path, nil, url.Values{"target_tn": {f.tnNone}})
	assert.Equal(t, http.StatusNotFound, code, resp)
	code, resp = f.send(t, http.MethodPost, path, nil, url.Values{"target_tn": {f.tnRO}})
	assert.Equal(t, http.StatusForbidden, code, resp)
	assert.Equal(t, 1, f.ticketInt(t, "ticket_state_id", f.ticketRW))
}

func TestTicketQueueMove_ChecksTargetQueue(t *testing.T) {
	for _, path := range []string{"/agent/tickets/%d/queue", "/api/tickets/%d/queue"} {
		t.Run(path, func(t *testing.T) {
			f := newTBAFixture(t)
			p := fmt.Sprintf(path, f.ticketRW)
			for _, target := range []int{f.queueNone, f.queueRO} {
				code, resp := f.send(t, http.MethodPost, p, nil, url.Values{"queue_id": {fmt.Sprint(target)}})
				assert.Equal(t, http.StatusForbidden, code, resp)
				assert.Equal(t, f.queueRW, f.ticketInt(t, "queue_id", f.ticketRW))
			}
			code, resp := f.send(t, http.MethodPost, p, nil, url.Values{"queue_id": {fmt.Sprint(f.queueMove)}})
			assert.Equal(t, http.StatusOK, code, resp)
			assert.Equal(t, f.queueMove, f.ticketInt(t, "queue_id", f.ticketRW))
		})
	}
}

func TestCustomFieldValues_FollowTicketAccess(t *testing.T) {
	f := newTBAFixture(t)
	repo := customfields.NewRepositoryWithDB(f.db)
	field := fmt.Sprintf("tba_cf_%d", time.Now().UnixNano())
	defID, err := repo.CreateDef(&customfields.FieldDef{
		Name: field, Label: "TBA", EntityType: customfields.EntityTicket,
		FieldType: customfields.FieldText, OwnerType: customfields.OwnerAdmin, Section: "custom", ValidID: 1,
	}, 1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = repo.HardDeleteDef(defID) })
	for _, id := range []int{f.ticketRW, f.ticketRO, f.ticketNone} {
		require.NoError(t, repo.SetValues(customfields.EntityTicket, int64(id), map[string]any{field: "orig"}))
	}
	values := func(id int) string { return fmt.Sprintf("/api/v1/custom-fields/values/ticket/%d", id) }
	stored := func(id int) any {
		v, err := repo.GetValues(customfields.EntityTicket, int64(id), []string{field})
		require.NoError(t, err)
		return v[field]
	}

	code, resp := f.send(t, http.MethodGet, values(f.ticketNone), nil, nil)
	assert.Equal(t, http.StatusNotFound, code, resp)
	assert.Nil(t, resp["values"])
	code, resp = f.send(t, http.MethodGet, values(f.ticketRO), nil, nil)
	require.Equal(t, http.StatusOK, code, resp)
	assert.Equal(t, "orig", resp["values"].(map[string]any)[field])
	code, resp = f.send(t, http.MethodGet, fmt.Sprintf("/api/v1/custom-fields/values/article/%d", f.articleNone), nil, nil)
	assert.Equal(t, http.StatusNotFound, code, resp)

	set := gin.H{"values": gin.H{field: "changed"}}
	code, resp = f.send(t, http.MethodPut, values(f.ticketNone), set, nil)
	assert.Equal(t, http.StatusNotFound, code, resp)
	code, resp = f.send(t, http.MethodPut, values(f.ticketRO), set, nil)
	assert.Equal(t, http.StatusForbidden, code, resp)
	code, resp = f.send(t, http.MethodPut, values(f.ticketRW), set, nil)
	assert.Equal(t, http.StatusOK, code, resp)
	assert.Equal(t, "orig", stored(f.ticketNone))
	assert.Equal(t, "orig", stored(f.ticketRO))
	assert.Equal(t, "changed", stored(f.ticketRW))

	code, resp = f.send(t, http.MethodPost, "/api/v1/custom-fields/query", gin.H{
		"entity_type": "ticket",
		"filters":     []gin.H{{"field": field, "operator": "eq", "value": "orig"}},
	}, nil)
	require.Equal(t, http.StatusOK, code, resp)
	assert.ElementsMatch(t, []any{float64(f.ticketRO)}, resp["object_ids"])

	// Queue values are admin data: readable with ro on the queue, never
	// writable by a non-admin.
	code, resp = f.send(t, http.MethodPut, fmt.Sprintf("/api/v1/custom-fields/values/queue/%d", f.queueRW), gin.H{"values": gin.H{}}, nil)
	assert.Equal(t, http.StatusForbidden, code, resp)
	code, resp = f.send(t, http.MethodGet, fmt.Sprintf("/api/v1/custom-fields/values/queue/%d", f.queueNone), nil, nil)
	assert.Equal(t, http.StatusNotFound, code, resp)
}

// The create middleware once read queue_id from the query string while the
// handler created in the queue_id from the body. A request naming an allowed
// queue in the query and a forbidden one in the body must create nothing
// (the middleware refuses conflicting ids; the handler checks its own queue).
func TestTicketCreate_ChecksQueueFromBody(t *testing.T) {
	f := newTBAFixture(t)
	count := func(queueID int) int {
		var n int
		require.NoError(t, f.db.QueryRow(database.ConvertPlaceholders(
			`SELECT COUNT(*) FROM ticket WHERE queue_id = ?`), queueID).Scan(&n))
		return n
	}
	for _, path := range []string{"/api/tickets", "/tickets"} {
		t.Run(path, func(t *testing.T) {
			before := count(f.queueNone)
			code, resp := f.send(t, http.MethodPost, fmt.Sprintf("%s?queue_id=%d", path, f.queueRW), nil, url.Values{
				"queue_id": {fmt.Sprint(f.queueNone)}, "subject": {"redirected"}, "body": {"b"},
				"customer_email": {"tba@example.com"},
			})
			assert.Contains(t, []int{http.StatusBadRequest, http.StatusForbidden}, code, resp)
			assert.Equal(t, before, count(f.queueNone))
		})
	}
}
