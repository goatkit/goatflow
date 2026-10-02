package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/routing"
)

// eppFixture: a queue in its own group, an agent holding rw on that group
// only through a role (role_user -> group_role), an agent in the admin group
// with no grant on the queue's group, an agent with no permission at all, and
// a ticket in the queue.
type eppFixture struct {
	db                         *sql.DB
	router                     *gin.Engine
	queue, ticket              int
	roleTok, adminTok, noneTok string
}

func newEPPFixture(t *testing.T) *eppFixture {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, db)
	f := &eppFixture{db: db}
	sfx := fmt.Sprintf("%d", time.Now().UnixNano())
	adapter := database.GetAdapter()
	mustID := func(id int64, err error) int {
		t.Helper()
		require.NoError(t, err)
		return int(id)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		_, err := db.Exec(database.ConvertPlaceholders(q), args...)
		require.NoError(t, err)
	}

	var groupID, roleID int
	var agents []int
	t.Cleanup(func() {
		for _, q := range []struct {
			sql  string
			args []any
		}{
			{"DELETE FROM article_data_mime WHERE article_id IN (SELECT id FROM article WHERE ticket_id IN (SELECT id FROM ticket WHERE queue_id = ?))", []any{f.queue}},
			{"DELETE FROM ticket_history WHERE ticket_id IN (SELECT id FROM ticket WHERE queue_id = ?)", []any{f.queue}},
			{"DELETE FROM article WHERE ticket_id IN (SELECT id FROM ticket WHERE queue_id = ?)", []any{f.queue}},
			{"DELETE FROM ticket WHERE queue_id = ?", []any{f.queue}},
			{"DELETE FROM role_user WHERE role_id = ?", []any{roleID}},
			{"DELETE FROM group_role WHERE role_id = ? OR group_id = ?", []any{roleID, groupID}},
			{"DELETE FROM roles WHERE id = ?", []any{roleID}},
			{"DELETE FROM group_customer WHERE group_id = ?", []any{groupID}},
			{"DELETE FROM group_customer_user WHERE group_id = ?", []any{groupID}},
			{"DELETE FROM group_user WHERE group_id = ?", []any{groupID}},
			{"DELETE FROM queue WHERE id = ?", []any{f.queue}},
			{"DELETE FROM `groups` WHERE id = ?", []any{groupID}},
		} {
			if _, err := db.Exec(database.ConvertPlaceholders(q.sql), q.args...); err != nil {
				t.Errorf("cleanup %q: %v", q.sql, err)
			}
		}
		for _, a := range agents {
			for _, q := range []string{
				"DELETE FROM group_user WHERE user_id = ?",
				"DELETE FROM role_user WHERE user_id = ?",
				"DELETE FROM user_preferences WHERE user_id = ?",
				"DELETE FROM users WHERE id = ?",
			} {
				if _, err := db.Exec(database.ConvertPlaceholders(q), a); err != nil {
					t.Errorf("cleanup %q: %v", q, err)
				}
			}
		}
	})

	groupID = mustID(adapter.InsertWithReturning(db, database.ConvertPlaceholders(
		"INSERT INTO `groups` (name, comments, valid_id, create_time, create_by, change_time, change_by) VALUES (?, 'epp test', 1, NOW(), 1, NOW(), 1) RETURNING id"),
		"epp_"+sfx))
	f.queue = mustID(adapter.InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO queue (name, group_id, system_address_id, salutation_id, signature_id,
			follow_up_id, follow_up_lock, comments, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 1, 1, 1, 1, 0, 'epp test', 1, NOW(), 1, NOW(), 1) RETURNING id`), "epp_"+sfx, groupID))
	roleID = mustID(adapter.InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO roles (name, comments, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 'epp test', 1, NOW(), 1, NOW(), 1) RETURNING id`), "epp_role_"+sfx))
	exec(`INSERT INTO group_role (role_id, group_id, permission_key, permission_value, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'rw', 1, NOW(), 1, NOW(), 1)`, roleID, groupID)

	newAgent := func(name string) (int, string) {
		login := "epp_" + name + "_" + sfx
		id := mustID(adapter.InsertWithReturning(db, database.ConvertPlaceholders(`
			INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, 'x', 'Effective', 'Perm', 1, NOW(), 1, NOW(), 1) RETURNING id`), login))
		agents = append(agents, id)
		return id, login
	}
	token := func(id int, login, role string, isAdmin bool) string {
		return testSessionToken(t, uint(id), login, login, role, isAdmin, 0)
	}

	roleAgent, roleLogin := newAgent("role")
	exec(`INSERT INTO role_user (user_id, role_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, NOW(), 1, NOW(), 1)`, roleAgent, roleID)
	f.roleTok = token(roleAgent, roleLogin, "Agent", false)

	adminAgent, adminLogin := newAgent("admin")
	var adminGroup int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		"SELECT id FROM `groups` WHERE name = ?"), "admin").Scan(&adminGroup))
	exec(`INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'rw', NOW(), 1, NOW(), 1)`, adminAgent, adminGroup)
	f.adminTok = token(adminAgent, adminLogin, "Admin", true)

	noneAgent, noneLogin := newAgent("none")
	f.noneTok = token(noneAgent, noneLogin, "Agent", false)

	f.ticket = mustID(adapter.InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO ticket (tn, title, queue_id, ticket_lock_id, type_id, user_id, responsible_user_id,
			ticket_priority_id, ticket_state_id, customer_id, customer_user_id, timeout, until_time,
			escalation_time, escalation_update_time, escalation_response_time, escalation_solution_time,
			archive_flag, create_time, create_by, change_time, change_by)
		VALUES (?, 'epp ticket', ?, 1, 1, 1, 1, 3, 1, 'epp-co', 'epp@example.com', 0, 0, 0, 0, 0, 0, 0,
			NOW(), 1, NOW(), 1) RETURNING id`), "EPP"+sfx, f.queue))

	gin.SetMode(gin.TestMode)
	f.router = gin.New()
	require.NoError(t, routing.LoadYAMLRoutesForTesting(f.router))
	return f
}

func (f *eppFixture) send(t *testing.T, token, method, path string, body any) (int, string) {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	return w.Code, w.Body.String()
}

func (f *eppFixture) ticketsInQueue(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, f.db.QueryRow(database.ConvertPlaceholders(
		`SELECT COUNT(*) FROM ticket WHERE queue_id = ?`), f.queue).Scan(&n))
	return n
}

func (f *eppFixture) title(t *testing.T) string {
	t.Helper()
	var title string
	require.NoError(t, f.db.QueryRow(database.ConvertPlaceholders(
		`SELECT title FROM ticket WHERE id = ?`), f.ticket).Scan(&title))
	return title
}

// The ticket create/update APIs must honour the same effective permissions
// as the queue access middleware: rw granted through a role, and admin-group
// membership, allow the write; no permission is a 403 that writes nothing.
func TestTicketWriteAPIs_UseEffectivePermissions(t *testing.T) {
	f := newEPPFixture(t)

	for _, tc := range []struct {
		name  string
		token string
	}{
		{"rw via role", f.roleTok},
		{"admin group", f.adminTok},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := f.ticketsInQueue(t)
			code, body := f.send(t, tc.token, http.MethodPost, "/api/v1/tickets", map[string]any{
				"title": "epp " + tc.name, "queue_id": f.queue, "body": "created", "customer_email": "epp@example.com",
			})
			assert.Equal(t, http.StatusCreated, code, body)
			assert.Equal(t, before+1, f.ticketsInQueue(t))

			newTitle := "epp updated by " + tc.name
			code, body = f.send(t, tc.token, http.MethodPut, fmt.Sprintf("/api/v1/tickets/%d", f.ticket), map[string]any{"title": newTitle})
			assert.Equal(t, http.StatusOK, code, body)
			assert.Equal(t, newTitle, f.title(t))
		})
	}

	t.Run("no permission", func(t *testing.T) {
		before, title := f.ticketsInQueue(t), f.title(t)
		code, body := f.send(t, f.noneTok, http.MethodPost, "/api/v1/tickets", map[string]any{
			"title": "epp denied", "queue_id": f.queue, "body": "denied", "customer_email": "epp@example.com",
		})
		assert.Equal(t, http.StatusForbidden, code, body)
		assert.Equal(t, before, f.ticketsInQueue(t))

		code, body = f.send(t, f.noneTok, http.MethodPut, fmt.Sprintf("/api/v1/tickets/%d", f.ticket), map[string]any{"title": "epp denied"})
		assert.Equal(t, http.StatusForbidden, code, body)
		assert.Equal(t, title, f.title(t))
	})
}
