package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// The "Clone Permissions" modal on /admin/permissions posts multipart form
// data to /admin/permissions/clone. These tests run through the real router.
func TestAdminPermissionsCloneRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := getTestDB(t)
	router := NewSimpleRouterWithDB(db)
	adminToken := GetTestAuthToken(t)

	grant := func(t *testing.T, userID, groupID int, keys ...string) {
		t.Helper()
		for _, key := range keys {
			_, err := db.Exec(database.ConvertPlaceholders(`
				INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
				VALUES (?, ?, ?, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)`), userID, groupID, key)
			require.NoError(t, err)
		}
	}
	permsOf := func(t *testing.T, userID int) []string {
		t.Helper()
		rows, err := db.Query(database.ConvertPlaceholders(
			`SELECT group_id, permission_key FROM group_user WHERE user_id = ?`), userID)
		require.NoError(t, err)
		defer rows.Close()
		var out []string
		for rows.Next() {
			var gid int
			var key string
			require.NoError(t, rows.Scan(&gid, &key))
			out = append(out, fmt.Sprintf("%d:%s", gid, key))
		}
		require.NoError(t, rows.Err())
		sort.Strings(out)
		return out
	}
	post := func(t *testing.T, token string, source, target any) *httptest.ResponseRecorder {
		t.Helper()
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		require.NoError(t, mw.WriteField("source_user_id", fmt.Sprint(source)))
		require.NoError(t, mw.WriteField("target_user_id", fmt.Sprint(target)))
		require.NoError(t, mw.Close())
		req := httptest.NewRequest(http.MethodPost, "/admin/permissions/clone", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	decode := func(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
		t.Helper()
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
		return body
	}

	source, sourceLogin := createIsolatedAgent(t, "clone_src")
	target, _ := createIsolatedAgent(t, "clone_dst")
	groupA, _ := createIsolatedGroup(t, "clone_a")
	groupB, _ := createIsolatedGroup(t, "clone_b")

	grant(t, source, groupA, "ro", "note")
	grant(t, source, groupB, "rw")
	grant(t, target, groupB, "owner") // must disappear: clone replaces

	t.Run("agent outside the admin group is rejected", func(t *testing.T) {
		agentToken := testSessionToken(t, uint(source), sourceLogin, sourceLogin, "Agent", false, 0)
		w := post(t, agentToken, source, target)
		require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
		assert.Equal(t, []string{fmt.Sprintf("%d:owner", groupB)}, permsOf(t, target))
	})

	t.Run("same source and target is a bad request", func(t *testing.T) {
		w := post(t, adminToken, source, source)
		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Equal(t, false, decode(t, w)["success"])
	})

	t.Run("non-numeric id is a bad request", func(t *testing.T) {
		w := post(t, adminToken, "abc", target)
		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	})

	t.Run("unknown target user is not found and changes nothing", func(t *testing.T) {
		var missing int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT COALESCE(MAX(id), 0) + 1000 FROM users`)).Scan(&missing))
		w := post(t, adminToken, source, missing)
		require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
		assert.Equal(t, false, decode(t, w)["success"])
		var n int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			`SELECT COUNT(*) FROM group_user WHERE user_id = ?`), missing).Scan(&n))
		assert.Zero(t, n)
	})

	t.Run("clone replaces the target's permissions with the source's", func(t *testing.T) {
		before := permsOf(t, source)
		w := post(t, adminToken, source, target)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, true, decode(t, w)["success"])

		want := []string{
			fmt.Sprintf("%d:note", groupA),
			fmt.Sprintf("%d:ro", groupA),
			fmt.Sprintf("%d:rw", groupB),
		}
		sort.Strings(want)
		assert.Equal(t, want, permsOf(t, target))
		assert.Equal(t, before, permsOf(t, source), "source must be untouched")
	})

	t.Run("cloning from a user without permissions empties the target", func(t *testing.T) {
		empty, _ := createIsolatedAgent(t, "clone_empty")
		w := post(t, adminToken, empty, target)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Empty(t, permsOf(t, target))
	})
}
