package api

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// Fixtures owned by a single test. Tests that update, deactivate or change the
// memberships of an agent, group or queue use these instead of the seeded rows
// (users.id 1, groups 1-5, queues 1-6), and every row is deleted again, in
// foreign-key order, when the test ends.

var (
	agentCleanup = []string{
		`DELETE FROM group_user WHERE user_id = ?`,
		`DELETE FROM role_user WHERE user_id = ?`,
		`DELETE FROM user_preferences WHERE user_id = ?`,
		// Handlers stamp change_by with the acting agent; a self-reference
		// blocks the delete on MySQL (FK_users_change_by_id).
		`UPDATE users SET create_by = 1, change_by = 1 WHERE id = ?`,
		`DELETE FROM users WHERE id = ?`,
	}
	groupCleanup = []string{
		`DELETE FROM group_user WHERE group_id = ?`,
		`DELETE FROM group_role WHERE group_id = ?`,
		`DELETE FROM group_customer_user WHERE group_id = ?`,
		`DELETE FROM group_customer WHERE group_id = ?`,
		"DELETE FROM `groups` WHERE id = ?",
	}
	queueCleanup = []string{
		`DELETE FROM personal_queues WHERE queue_id = ?`,
		`DELETE FROM queue_auto_response WHERE queue_id = ?`,
		`DELETE FROM queue_preferences WHERE queue_id = ?`,
		`DELETE FROM queue_standard_template WHERE queue_id = ?`,
		`DELETE FROM queue WHERE id = ?`,
	}
)

// createIsolatedAgent inserts an agent owned by the calling test.
func createIsolatedAgent(t *testing.T, prefix string) (int, string) {
	t.Helper()
	db := isolatedDB(t)
	login := fmt.Sprintf("%s_%d@example.test", prefix, time.Now().UnixNano())
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO users (login, pw, title, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 'x', '', 'Isolated', 'Agent', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
		RETURNING id`), login)
	require.NoError(t, err)
	t.Cleanup(func() { deleteIsolatedRows(t, int(id), agentCleanup) })
	return int(id), login
}

// createIsolatedGroup inserts a group owned by the calling test.
func createIsolatedGroup(t *testing.T, prefix string) (int, string) {
	t.Helper()
	db := isolatedDB(t)
	name := fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO `+"`groups`"+` (name, comments, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 'isolated test group', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
		RETURNING id`), name)
	require.NoError(t, err)
	t.Cleanup(func() { deleteIsolatedRows(t, int(id), groupCleanup) })
	return int(id), name
}

// createIsolatedQueue inserts a queue (in the seeded users group) owned by the
// calling test.
func createIsolatedQueue(t *testing.T, prefix string) (int, string) {
	t.Helper()
	db := isolatedDB(t)
	name := fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO queue (name, group_id, system_address_id, salutation_id, signature_id, unlock_timeout,
			follow_up_id, follow_up_lock, comments, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 1, 1, 1, 1, 0, 1, 0, 'isolated test queue', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
		RETURNING id`), name)
	require.NoError(t, err)
	t.Cleanup(func() { deleteIsolatedRows(t, int(id), queueCleanup) })
	return int(id), name
}

// cleanupAgentByLogin deletes an agent created through a handler under test.
func cleanupAgentByLogin(t *testing.T, login string) {
	t.Helper()
	cleanupByName(t, `SELECT id FROM users WHERE login = ?`, login, agentCleanup)
}

// cleanupGroupByNameAtEnd deletes a group created through a handler under test.
func cleanupGroupByNameAtEnd(t *testing.T, name string) {
	t.Helper()
	cleanupByName(t, "SELECT id FROM `groups` WHERE name = ?", name, groupCleanup)
}

// cleanupQueueByNameAtEnd deletes a queue created through a handler under test.
func cleanupQueueByNameAtEnd(t *testing.T, name string) {
	t.Helper()
	cleanupByName(t, `SELECT id FROM queue WHERE name = ?`, name, queueCleanup)
}

func isolatedDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	return db
}

// cleanupByName resolves the id when the test ends, so it also covers rows the
// handler under test created; a missing row means the handler created nothing.
func cleanupByName(t *testing.T, lookup, name string, cleanup []string) {
	t.Helper()
	t.Cleanup(func() {
		db, err := database.GetDB()
		if err != nil {
			t.Errorf("cleanup %q: %v", name, err)
			return
		}
		var id int
		err = db.QueryRow(database.ConvertPlaceholders(lookup), name).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return
		}
		if err != nil {
			t.Errorf("cleanup %q: %v", name, err)
			return
		}
		deleteIsolatedRows(t, id, cleanup)
	})
}

func deleteIsolatedRows(t *testing.T, id int, cleanup []string) {
	t.Helper()
	db, err := database.GetDB()
	if err != nil {
		t.Errorf("cleanup %d: %v", id, err)
		return
	}
	for _, q := range cleanup {
		if _, err := db.Exec(database.ConvertPlaceholders(q), id); err != nil {
			t.Errorf("cleanup %d: %s: %v", id, q, err)
		}
	}
}
