package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// TestRealGroupAssignmentIssue checks that group memberships stored in
// group_user are reported by HandleAdminUserGet and replaced by
// HandleAdminUserUpdate. It runs on an agent created for the test, so the
// seeded agents' memberships are never touched.
func TestRealGroupAssignmentIssue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.GetDB()
	require.NoError(t, err)

	agentID, login := createIsolatedAgent(t, "group_assignment")
	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
		SELECT ?, g.id, 'rw', CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1
		FROM `+"`groups`"+` g WHERE g.name IN ('admin', 'users')`), agentID)
	require.NoError(t, err)

	t.Run("Database verification: agent has its groups", func(t *testing.T) {
		assert.Equal(t, []string{"admin", "users"}, adminTestUserGroups(t, agentID))
	})

	t.Run("API GET verification: HandleAdminUserGet returns the groups", func(t *testing.T) {
		router := seedAdminRouter()
		router.GET("/admin/users/:id", HandleAdminUserGet)

		req, _ := http.NewRequest("GET", "/admin/users/"+strconv.Itoa(agentID), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "response should contain data")
		groups, ok := data["groups"].([]interface{})
		require.True(t, ok, "data should contain a groups list")
		assert.ElementsMatch(t, []interface{}{"admin", "users"}, groups)
	})

	t.Run("Form submission: HandleAdminUserUpdate replaces the groups", func(t *testing.T) {
		router := seedAdminRouter()
		router.PUT("/admin/users/:id", HandleAdminUserUpdate)

		formData := url.Values{}
		formData.Set("login", login)
		formData.Set("first_name", "Isolated")
		formData.Set("last_name", "Agent")
		formData.Set("valid_id", "1")
		formData.Add("groups", "stats")
		formData.Add("groups", "users")

		req, _ := http.NewRequest("PUT", "/admin/users/"+strconv.Itoa(agentID),
			strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, []string{"stats", "users"}, adminTestUserGroups(t, agentID))
	})
}
