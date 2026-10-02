package api

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// Adding an agent to a group is idempotent and removal deletes every row:
// group_user has no unique key, so a blind insert would pile up duplicates.
func TestAdminGroupMembershipAddRemove(t *testing.T) {
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available, skipping integration test")
	}
	WithCleanDB(t)
	db, err := database.GetDB()
	require.NoError(t, err)

	var groupID int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		`SELECT id FROM groups WHERE name = ?`), "support").Scan(&groupID))
	const userID = 15
	_, err = db.Exec(database.ConvertPlaceholders(
		`DELETE FROM group_user WHERE user_id = ? AND group_id = ?`), userID, groupID)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := seedAdminRouter()
	router.POST("/admin/groups/:id/users", HandleAdminGroupsAddUser)
	router.DELETE("/admin/groups/:id/users/:userId", HandleAdminGroupsRemoveUser)
	gid := strconv.Itoa(groupID)

	rows := func() map[string]int {
		r, err := db.Query(database.ConvertPlaceholders(
			`SELECT permission_key FROM group_user WHERE user_id = ? AND group_id = ?`), userID, groupID)
		require.NoError(t, err)
		defer r.Close()
		got := map[string]int{}
		for r.Next() {
			var k string
			require.NoError(t, r.Scan(&k))
			got[k]++
		}
		require.NoError(t, r.Err())
		return got
	}

	for range 2 {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/admin/groups/"+gid+"/users",
			strings.NewReader(`{"user_id":15}`))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	}
	assert.Equal(t, map[string]int{"rw": 1}, rows(), "two adds must leave exactly one rw row")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/groups/"+gid+"/users", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/admin/groups/"+gid+"/users/15", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Empty(t, rows())
}
