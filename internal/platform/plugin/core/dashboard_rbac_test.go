package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/plugin"
)

// fakeHost implements plugin.HostAPI backed by the live test database for
// DBQuery; every other method is a no-op stub.
type fakeHost struct{}

func (f *fakeHost) DBQuery(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	// DBQuery converts placeholders itself for portability.
	q := database.ConvertPlaceholders(query)
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	var out []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		_ = rows.Scan(ptrs...)
		row := make(map[string]any)
		for i, c := range cols {
			switch v := vals[i].(type) {
			case []byte:
				row[c] = string(v)
			case nil:
				row[c] = nil
			default:
				row[c] = v
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
func (f *fakeHost) DBExec(ctx context.Context, query string, args ...any) (int64, error) {
	db, err := database.GetDB()
	if err != nil {
		return 0, err
	}
	res, err := db.ExecContext(ctx, database.ConvertPlaceholders(query), args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
func (f *fakeHost) CacheGet(ctx context.Context, key string) ([]byte, bool, error) {
	return nil, false, nil
}
func (f *fakeHost) CacheSet(ctx context.Context, key string, value []byte, ttlSeconds int) error {
	return nil
}
func (f *fakeHost) CacheDelete(ctx context.Context, key string) error { return nil }
func (f *fakeHost) HTTPRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) (int, []byte, error) {
	return 200, nil, nil
}
func (f *fakeHost) SendEmail(ctx context.Context, to, subject, body string, html bool) error {
	return nil
}
func (f *fakeHost) Log(ctx context.Context, level, message string, fields map[string]any) {}
func (f *fakeHost) ConfigGet(ctx context.Context, key string) (string, error)             { return "", nil }
func (f *fakeHost) Translate(ctx context.Context, key string, args ...any) string         { return "" }
func (f *fakeHost) CallPlugin(ctx context.Context, pluginName, function string, args json.RawMessage) (json.RawMessage, error) {
	return nil, nil
}
func (f *fakeHost) PublishEvent(ctx context.Context, channel string, eventType string, data string) error {
	return nil
}
func (f *fakeHost) EntitySoftDelete(ctx context.Context, entityType string, entityID int64, reason string) error {
	return nil
}
func (f *fakeHost) EntityRestore(ctx context.Context, entityType string, entityID int64) error {
	return nil
}
func (f *fakeHost) EntityHardDelete(ctx context.Context, entityType string, entityID int64, reason string) error {
	return nil
}
func (f *fakeHost) RecycleBinList(ctx context.Context, entityType string) (json.RawMessage, error) {
	return nil, nil
}
func (f *fakeHost) SecureConfigGet(ctx context.Context, key string) (string, error) { return "", nil }
func (f *fakeHost) SecureConfigSet(ctx context.Context, key string, value string) error {
	return nil
}
func (f *fakeHost) OrgID(ctx context.Context) int64 { return 0 }
func (f *fakeHost) CustomFieldsGet(ctx context.Context, entityType string, objectID int64, fields []string) (map[string]any, error) {
	return nil, nil
}
func (f *fakeHost) CustomFieldsSet(ctx context.Context, entityType string, objectID int64, values map[string]any) error {
	return nil
}
func (f *fakeHost) CustomFieldsQuery(ctx context.Context, entityType string, filters []plugin.CustomFieldFilter) ([]int64, error) {
	return nil, nil
}
func (f *fakeHost) StoreFile(ctx context.Context, key string, data []byte, metadata map[string]string) error {
	return nil
}
func (f *fakeHost) GetFile(ctx context.Context, key string) ([]byte, map[string]string, error) {
	return nil, nil, nil
}
func (f *fakeHost) DeleteFile(ctx context.Context, key string) error { return nil }
func (f *fakeHost) ListFiles(ctx context.Context, prefix string) ([]plugin.FileInfo, error) {
	return nil, nil
}
func (f *fakeHost) GenerateThumbnail(_ context.Context, _ []byte, _ string, _, _ int) ([]byte, string, error) {
	return nil, "", fmt.Errorf("not implemented")
}

// setupRBACTestDB seeds two groups, two queues (one per group), and a user
// belonging to only the first group. Returns the restricted user id and the
// first (accessible) queue name.
// seedQueues creates two queues owned by two fresh groups and returns their
// names plus their owning group IDs, registering cleanup.
func seedQueues(t *testing.T) (qa, qb string, g1id, g2id int) {
	db, err := database.GetDB()
	require.NoError(t, err)
	sfx := fmt.Sprintf("%d", time.Now().UnixNano())
	g1 := "rbac_g1_" + sfx
	g2 := "rbac_g2_" + sfx
	_, err = db.Exec(`
		INSERT INTO `+"`groups`"+` (name, comments, valid_id, create_time, change_time, create_by, change_by)
		VALUES (?, 'test', 1, NOW(), NOW(), 1, 1),
		       (?, 'test', 1, NOW(), NOW(), 1, 1)`, g1, g2)
	require.NoError(t, err)
	require.NoError(t, db.QueryRow(`SELECT id FROM `+"`groups`"+` WHERE name = ?`, g1).Scan(&g1id))
	require.NoError(t, db.QueryRow(`SELECT id FROM `+"`groups`"+` WHERE name = ?`, g2).Scan(&g2id))
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM `groups` WHERE id IN (?, ?)", g1id, g2id)
	})

	qa = "rbac_qa_" + sfx
	qb = "rbac_qb_" + sfx
	// Reuse an existing queue's follow_up_id (a valid FK) so this test does not
	// depend on a specific seed row in follow_up_possible being present.
	var followUpID int
	if err := db.QueryRow("SELECT follow_up_id FROM queue ORDER BY id LIMIT 1").Scan(&followUpID); err != nil {
		followUpID = 1
	}
	_, err = db.Exec(`
		INSERT INTO queue (name, group_id, system_address_id, salutation_id, signature_id,
			follow_up_id, follow_up_lock, valid_id, create_time, change_time, create_by, change_by)
		VALUES (?, ?, 1, 1, 1, ?, 0, 1, NOW(), NOW(), 1, 1),
		       (?, ?, 1, 1, 1, ?, 0, 1, NOW(), NOW(), 1, 1)`, qa, g1id, followUpID, qb, g2id, followUpID)
	require.NoError(t, err)
	var qaID, qbID int
	require.NoError(t, db.QueryRow("SELECT id FROM queue WHERE name = ?", qa).Scan(&qaID))
	require.NoError(t, db.QueryRow("SELECT id FROM queue WHERE name = ?", qb).Scan(&qbID))
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM queue WHERE id IN (?, ?)", qaID, qbID)
	})
	return qa, qb, g1id, g2id
}

// createUserInGroup creates an active user with rw membership in one group.
func createUserInGroup(t *testing.T, login string, groupID int) int {
	db, err := database.GetDB()
	require.NoError(t, err)
	_, err = db.Exec(`
		INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 'x', 'RBAC', 'User', 1, NOW(), 1, NOW(), 1)`, login)
	require.NoError(t, err)
	// Resolve the id by login rather than trusting LastInsertID: the shared
	// test DB can race other tests, so we re-query to bind group_user to the
	// actual committed row.
	var uid int
	require.NoError(t, db.QueryRow(`SELECT id FROM users WHERE login = ?`, login).Scan(&uid))
	_, err = db.Exec(`
		INSERT INTO group_user (user_id, group_id, permission_key, create_time, change_time, create_by, change_by)
		VALUES (?, ?, 'rw', NOW(), NOW(), 1, 1)`, uid, groupID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM group_user WHERE user_id = ?", uid)
		_, _ = db.Exec("DELETE FROM users WHERE id = ?", uid)
	})
	return int(uid)
}

func setupRBACTestDB(t *testing.T) (restrictedUser int, accessibleQueue, hiddenQueue string) {
	qa, qb, g1id, _ := seedQueues(t)
	login := fmt.Sprintf("rbac_u_%d", time.Now().UnixNano())
	uid := createUserInGroup(t, login, g1id)
	return uid, qa, qb
}

// TestQueueStatus_RBAC verifies the queue status widget only lists queues the
// user's groups own, and that admin sees all queues.
func TestQueueStatus_RBAC(t *testing.T) {
	if err := database.InitTestDB(); err != nil {
		t.Skipf("test database not available: %v", err)
	}
	ctx := context.Background()
	p := &DashboardPlugin{host: &fakeHost{}}

	t.Run("restricted user sees only own group's queue", func(t *testing.T) {
		uid, accessibleQueue, hiddenQueue := setupRBACTestDB(t)
		args, _ := json.Marshal(map[string]any{"_user_id": uid})
		res, err := p.callQueueStatus(ctx, args)
		require.NoError(t, err)
		assert.Contains(t, res, accessibleQueue)
		assert.NotContains(t, res, hiddenQueue)
	})

	t.Run("admin sees all queues", func(t *testing.T) {
		db, err := database.GetDB()
		require.NoError(t, err)
		// Seed fresh queues so the dashboard has data to show.
		qa, qb, _, _ := seedQueues(t)

		// Admin user: member of the real 'admin' group (bypasses RBAC).
		login := fmt.Sprintf("rbac_admin_%d", time.Now().UnixNano())
		_, err = db.Exec(`
			INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, 'x', 'RBAC', 'Admin', 1, NOW(), 1, NOW(), 1)`, login)
		require.NoError(t, err)
		var uid int
		require.NoError(t, db.QueryRow(`SELECT id FROM users WHERE login = ?`, login).Scan(&uid))
		var adminGrp int
		err = db.QueryRow("SELECT id FROM `groups` WHERE name = 'admin' AND valid_id = 1 LIMIT 1").Scan(&adminGrp)
		require.NoError(t, err)
		_, err = db.Exec(`
			INSERT INTO group_user (user_id, group_id, permission_key, create_time, change_time, create_by, change_by)
			VALUES (?, ?, 'rw', NOW(), NOW(), 1, 1)`, uid, adminGrp)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = db.Exec("DELETE FROM group_user WHERE user_id = ?", uid)
			_, _ = db.Exec("DELETE FROM users WHERE id = ?", uid)
		})

		args, _ := json.Marshal(map[string]any{"_user_id": uid})
		res, err := p.callQueueStatus(ctx, args)
		require.NoError(t, err)
		// Admin bypasses RBAC and sees both seeded queues.
		assert.Contains(t, res, "<table")
		assert.Contains(t, res, qa)
		assert.Contains(t, res, qb)
	})

	t.Run("widgetArgsUserID parsing", func(t *testing.T) {
		args, _ := json.Marshal(map[string]any{"_user_id": float64(42)})
		assert.Equal(t, 42, widgetArgsUserID(args))
		assert.Equal(t, 0, widgetArgsUserID(nil))
		assert.Equal(t, 0, widgetArgsUserID([]byte(`{"_user_id":"nope"}`)))
		argsS, _ := json.Marshal(map[string]any{"_user_id": "7"})
		assert.Equal(t, 7, widgetArgsUserID(argsS))
	})
}

// callQueueStatus invokes handleQueueStatus and returns the rendered HTML or
// the raw error message for assertion.
func (p *DashboardPlugin) callQueueStatus(ctx context.Context, args json.RawMessage) (string, error) {
	res, err := p.handleQueueStatus(ctx, args)
	if err != nil {
		return "", err
	}
	var data struct {
		HTML string `json:"html"`
	}
	if err := json.Unmarshal(res, &data); err != nil {
		return "", err
	}
	return data.HTML, nil
}

var _ = strings.Contains // keep import used across builds
