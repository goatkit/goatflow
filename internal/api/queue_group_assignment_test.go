package api

import (
	"bytes"
	"context"
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
)

// queueGroupFixture holds two fresh teams and removes every queue/team the test
// created.
type queueGroupFixture struct {
	t      *testing.T
	sfx    string
	teamA  int
	teamB  int
	queues []int
}

func newQueueGroupFixture(t *testing.T) *queueGroupFixture {
	t.Helper()
	svc, _ := setupSvcTestDB(t)
	gin.SetMode(gin.TestMode)
	f := &queueGroupFixture{t: t, sfx: fmt.Sprintf("_%d", time.Now().UnixNano()%1000000000)}
	var err error
	f.teamA, err = svc.CreateGroup(context.Background(), "QGTeamA"+f.sfx, "queue group test", 1)
	require.NoError(t, err)
	f.teamB, err = svc.CreateGroup(context.Background(), "QGTeamB"+f.sfx, "queue group test", 1)
	require.NoError(t, err)
	t.Cleanup(f.cleanup)
	return f
}

func (f *queueGroupFixture) cleanup() {
	db, err := database.GetDB()
	if err != nil {
		return
	}
	for _, id := range f.queues {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM queue WHERE id = ?`), id)
	}
	for _, gid := range []int{f.teamA, f.teamB} {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM group_user WHERE group_id = ?`), gid)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM groups WHERE id = ?`), gid)
	}
}

// track registers a queue created by name so cleanup removes it.
func (f *queueGroupFixture) track(name string) int {
	f.t.Helper()
	db, err := database.GetDB()
	require.NoError(f.t, err)
	var id int
	require.NoError(f.t, db.QueryRow(database.ConvertPlaceholders(`SELECT id FROM queue WHERE name = ?`), name).Scan(&id))
	f.queues = append(f.queues, id)
	return id
}

// seedQueue inserts a queue owned by groupID directly, so tests of the
// update/assign/delete handlers do not depend on the create handler.
func (f *queueGroupFixture) seedQueue(name string, groupID int) int {
	f.t.Helper()
	db, err := database.GetDB()
	require.NoError(f.t, err)
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO queue (
			name, group_id, system_address_id, salutation_id, signature_id,
			unlock_timeout, follow_up_id, follow_up_lock, valid_id,
			create_time, create_by, change_time, change_by
		) VALUES (?, ?, 1, 1, 1, 0, 1, 0, 1, NOW(), 1, NOW(), 1) RETURNING id`), name, groupID)
	require.NoError(f.t, err)
	f.queues = append(f.queues, int(id))
	return int(id)
}

func (f *queueGroupFixture) queueGroup(queueID int) int {
	f.t.Helper()
	db, err := database.GetDB()
	require.NoError(f.t, err)
	var gid int
	require.NoError(f.t, db.QueryRow(database.ConvertPlaceholders(`SELECT group_id FROM queue WHERE id = ?`), queueID).Scan(&gid))
	return gid
}

func (f *queueGroupFixture) queueCount(name string) int {
	f.t.Helper()
	db, err := database.GetDB()
	require.NoError(f.t, err)
	var n int
	require.NoError(f.t, db.QueryRow(database.ConvertPlaceholders(`SELECT COUNT(*) FROM queue WHERE name = ?`), name).Scan(&n))
	return n
}

// queueAdminRouter mounts the real queue handlers behind an authenticated admin.
func queueAdminRouter() *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Set("user_role", "Admin")
		c.Next()
	})
	r.POST("/api/v1/queues", HandleCreateQueueAPI)
	r.GET("/api/v1/queues", HandleListQueuesAPI)
	r.GET("/api/v1/queues/:id", HandleGetQueueAPI)
	r.PUT("/api/v1/queues/:id", HandleUpdateQueueAPI)
	r.DELETE("/api/v1/queues/:id", HandleDeleteQueueAPI)
	r.POST("/api/v1/queues/:id/groups", HandleAssignQueueGroupAPI)
	r.DELETE("/api/v1/queues/:id/groups/:group_id", HandleRemoveQueueGroupAPI)
	return r
}

func queueDoJSON(t *testing.T, r *gin.Engine, method, path string, body interface{}) (int, map[string]interface{}) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	out := map[string]interface{}{}
	if w.Body.Len() > 0 {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out), "body: %s", w.Body.String())
	}
	return w.Code, out
}

// singleGroup asserts that payload["groups"] is exactly [{id: groupID, name: name}].
func singleGroup(t *testing.T, payload map[string]interface{}, groupID int, name string) {
	t.Helper()
	groups, ok := payload["groups"].([]interface{})
	require.True(t, ok, "groups must be a list: %v", payload["groups"])
	require.Len(t, groups, 1)
	g := groups[0].(map[string]interface{})
	assert.EqualValues(t, groupID, g["id"])
	assert.Equal(t, name, g["name"])
}

func TestQueueGroup_CreateWithGroup(t *testing.T) {
	f := newQueueGroupFixture(t)
	r := queueAdminRouter()
	name := "QGCreate" + f.sfx

	code, resp := queueDoJSON(t, r, http.MethodPost, "/api/v1/queues", gin.H{"name": name, "group_id": f.teamA})
	require.Equal(t, http.StatusCreated, code, "%v", resp)
	id := f.track(name)
	data := resp["data"].(map[string]interface{})
	assert.EqualValues(t, id, data["id"])
	assert.EqualValues(t, f.teamA, data["group_id"])
	singleGroup(t, data, f.teamA, "QGTeamA"+f.sfx)
	assert.Equal(t, f.teamA, f.queueGroup(id))

	// GET returns the queue's one group.
	code, resp = queueDoJSON(t, r, http.MethodGet, fmt.Sprintf("/api/v1/queues/%d", id), nil)
	require.Equal(t, http.StatusOK, code, "%v", resp)
	singleGroup(t, resp["data"].(map[string]interface{}), f.teamA, "QGTeamA"+f.sfx)
}

func TestQueueGroup_CreateRequiresExistingGroup(t *testing.T) {
	f := newQueueGroupFixture(t)
	r := queueAdminRouter()

	name := "QGNoGroup" + f.sfx
	code, resp := queueDoJSON(t, r, http.MethodPost, "/api/v1/queues", gin.H{"name": name})
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, resp["error"], "group_id is required")
	assert.Equal(t, 0, f.queueCount(name))

	code, resp = queueDoJSON(t, r, http.MethodPost, "/api/v1/queues", gin.H{"name": name, "group_id": 99999999})
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, resp["error"], "not found")
	assert.Equal(t, 0, f.queueCount(name))
}

func TestQueueGroup_UpdateChangesGroup(t *testing.T) {
	f := newQueueGroupFixture(t)
	r := queueAdminRouter()
	name := "QGUpdate" + f.sfx
	id := f.seedQueue(name, f.teamA)
	path := fmt.Sprintf("/api/v1/queues/%d", id)

	code, resp := queueDoJSON(t, r, http.MethodPut, path, gin.H{"group_id": f.teamB})
	require.Equal(t, http.StatusOK, code, "%v", resp)
	singleGroup(t, resp["data"].(map[string]interface{}), f.teamB, "QGTeamB"+f.sfx)
	assert.Equal(t, f.teamB, f.queueGroup(id))

	// An unknown group is rejected and the queue keeps its group.
	code, _ = queueDoJSON(t, r, http.MethodPut, path, gin.H{"group_id": 99999999})
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Equal(t, f.teamB, f.queueGroup(id))

	// The list shows the queue with its one group to a member of that group.
	db, err := database.GetDB()
	require.NoError(t, err)
	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
		VALUES (1, ?, 'rw', NOW(), 1, NOW(), 1)`), f.teamB)
	require.NoError(t, err)
	code, resp = queueDoJSON(t, r, http.MethodGet, "/api/v1/queues", nil)
	require.Equal(t, http.StatusOK, code, "%v", resp)
	var found map[string]interface{}
	for _, q := range resp["data"].([]interface{}) {
		if m := q.(map[string]interface{}); m["name"] == name {
			found = m
		}
	}
	require.NotNil(t, found, "queue %s missing from list", name)
	singleGroup(t, found, f.teamB, "QGTeamB"+f.sfx)
}

func TestQueueGroup_AssignAndRejectRemove(t *testing.T) {
	f := newQueueGroupFixture(t)
	r := queueAdminRouter()
	id := f.seedQueue("QGAssign"+f.sfx, f.teamA)
	groupsPath := fmt.Sprintf("/api/v1/queues/%d/groups", id)

	// Assigning sets queue.group_id.
	code, resp := queueDoJSON(t, r, http.MethodPost, groupsPath, gin.H{"group_id": f.teamB})
	require.Equal(t, http.StatusOK, code, "%v", resp)
	singleGroup(t, resp["data"].(map[string]interface{}), f.teamB, "QGTeamB"+f.sfx)
	assert.Equal(t, f.teamB, f.queueGroup(id))

	// Re-assigning the same group is idempotent.
	code, _ = queueDoJSON(t, r, http.MethodPost, groupsPath, gin.H{"group_id": f.teamB})
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, f.teamB, f.queueGroup(id))

	// Unknown group, unknown queue, missing group_id.
	code, _ = queueDoJSON(t, r, http.MethodPost, groupsPath, gin.H{"group_id": 99999999})
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = queueDoJSON(t, r, http.MethodPost, "/api/v1/queues/99999999/groups", gin.H{"group_id": f.teamA})
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = queueDoJSON(t, r, http.MethodPost, groupsPath, gin.H{})
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Equal(t, f.teamB, f.queueGroup(id))

	// Removing the queue's only group is rejected; the queue keeps it.
	code, resp = queueDoJSON(t, r, http.MethodDelete, fmt.Sprintf("%s/%d", groupsPath, f.teamB), nil)
	assert.Equal(t, http.StatusConflict, code)
	assert.Contains(t, resp["error"], "must have a group")
	assert.Equal(t, f.teamB, f.queueGroup(id))

	// A group the queue does not have is not found.
	code, _ = queueDoJSON(t, r, http.MethodDelete, fmt.Sprintf("%s/%d", groupsPath, f.teamA), nil)
	assert.Equal(t, http.StatusNotFound, code)
	assert.Equal(t, f.teamB, f.queueGroup(id))
}

func TestQueueGroup_DeleteQueue(t *testing.T) {
	f := newQueueGroupFixture(t)
	r := queueAdminRouter()
	id := f.seedQueue("QGDelete"+f.sfx, f.teamA)

	code, resp := queueDoJSON(t, r, http.MethodDelete, fmt.Sprintf("/api/v1/queues/%d", id), nil)
	require.Equal(t, http.StatusOK, code, "%v", resp)
	db, err := database.GetDB()
	require.NoError(t, err)
	var validID, groupID int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT valid_id, group_id FROM queue WHERE id = ?`), id).Scan(&validID, &groupID))
	assert.Equal(t, 2, validID)
	assert.Equal(t, f.teamA, groupID)

	code, _ = queueDoJSON(t, r, http.MethodDelete, "/api/v1/queues/99999999", nil)
	assert.Equal(t, http.StatusNotFound, code)
}

// TestQueueGroup_SetupTasks drives the setup assistant's create_queue and
// assign_queue_group tasks through the real task handler.
func TestQueueGroup_SetupTasks(t *testing.T) {
	f := newQueueGroupFixture(t)
	setupTemplateRenderer(t)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", 1); c.Next() })
	r.GET("/admin/setup/task/:plugin/:task_id", handleAdminSetupTask)
	r.POST("/admin/setup/task/:plugin/:task_id", handleAdminSetupTask)

	post := func(task string, form url.Values) map[string]interface{} {
		req := httptest.NewRequest(http.MethodPost, "/admin/setup/task/setup-assistant/"+task, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		out := map[string]interface{}{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
		return out
	}

	// The create_queue form asks for exactly one team.
	req := httptest.NewRequest(http.MethodGet, "/admin/setup/task/setup-assistant/create_queue", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	page := w.Body.String()
	assert.Contains(t, page, `<select name="group_id" required`)
	assert.Contains(t, page, fmt.Sprintf(`<option value="%d">QGTeamA%s</option>`, f.teamA, f.sfx))
	assert.NotContains(t, page, `name="group_ids"`)

	name := "QGSetup" + f.sfx
	res := post("create_queue", url.Values{"name": {name}, "group_id": {fmt.Sprint(f.teamA)}})
	require.Equal(t, true, res["success"], "%v", res)
	id := f.track(name)
	assert.Equal(t, f.teamA, f.queueGroup(id))

	res = post("create_queue", url.Values{"name": {"QGSetupNoTeam" + f.sfx}})
	assert.Equal(t, false, res["success"])
	assert.Equal(t, 0, f.queueCount("QGSetupNoTeam"+f.sfx))

	res = post("assign_queue_group", url.Values{"queue_id": {fmt.Sprint(id)}, "group_id": {fmt.Sprint(f.teamB)}})
	require.Equal(t, true, res["success"], "%v", res)
	assert.Equal(t, f.teamB, f.queueGroup(id))
}
