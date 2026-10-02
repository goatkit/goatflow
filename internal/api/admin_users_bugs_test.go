package api

import (
	"bytes"
	"encoding/json"
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

// Regression tests for the agent edit dialog. Every test works on an agent it
// creates itself, so the seeded agents (1, 15 testuser) keep their groups.

func putAdminUserJSON(t *testing.T, router *gin.Engine, id int, body map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()
	jsonData, err := json.Marshal(body)
	require.NoError(t, err)
	req, _ := http.NewRequest("PUT", "/admin/users/"+strconv.Itoa(id), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// TestGroupAssignmentNotPersisting tests Bug #1: Group assignment not persisting to database.
func TestGroupAssignmentNotPersisting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := seedAdminRouter()
	router.PUT("/admin/users/:id", HandleAdminUserUpdate)

	t.Run("Group assignment should persist to database", func(t *testing.T) {
		id, login := createIsolatedAgent(t, "bug_persist")

		w := putAdminUserJSON(t, router, id, map[string]interface{}{
			"login":      login,
			"first_name": "Test",
			"last_name":  "Agent",
			"valid_id":   1,
			"groups":     []string{"support"},
		})

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.True(t, response["success"].(bool), "Response should indicate success")
		assert.Equal(t, []string{"support"}, adminTestUserGroups(t, id))
	})
}

// TestXlatsNotWorking tests Bug #2: the user payload carries translated fields.
func TestXlatsNotWorking(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := seedAdminRouter()
	router.GET("/admin/users/:id", HandleAdminUserGet)

	t.Run("User data should include translated xlats", func(t *testing.T) {
		id, _ := createIsolatedAgent(t, "bug_xlats")

		req, _ := http.NewRequest("GET", "/admin/users/"+strconv.Itoa(id), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.True(t, response["success"].(bool))

		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "Response should contain data object")
		assert.Equal(t, float64(id), data["id"])
		assert.Equal(t, map[string]interface{}{"valid_id": "valid"}, data["xlats"])
		assert.Equal(t, "valid", data["valid_id_xlat"])
	})
}

// TestNoWayToRemoveUserFromAllGroups tests Bug #3: an empty group selection removes all memberships.
func TestNoWayToRemoveUserFromAllGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := seedAdminRouter()
	router.PUT("/admin/users/:id", HandleAdminUserUpdate)

	t.Run("Should support removing user from all groups", func(t *testing.T) {
		db, err := database.GetDB()
		require.NoError(t, err)
		id, login := createIsolatedAgent(t, "bug_remove_all")
		_, err = db.Exec(database.ConvertPlaceholders(`
			INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
			SELECT ?, g.id, 'rw', CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1
			FROM `+"`groups`"+` g WHERE g.name = 'support'`), id)
		require.NoError(t, err)
		require.Equal(t, []string{"support"}, adminTestUserGroups(t, id))

		w := putAdminUserJSON(t, router, id, map[string]interface{}{
			"login":      login,
			"first_name": "Test",
			"last_name":  "Agent",
			"valid_id":   1,
			"groups":     []string{}, // Empty array should remove all groups
		})

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.True(t, response["success"].(bool), "Response should indicate success")

		var finalGroupCount int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			"SELECT COUNT(*) FROM group_user WHERE user_id = ?"), id).Scan(&finalGroupCount))
		assert.Equal(t, 0, finalGroupCount, "User should have 0 groups after sending empty groups array")
	})
}

// TestUserWorkflowEndToEnd tests the complete user edit workflow: load, add a group, remove it.
func TestUserWorkflowEndToEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := seedAdminRouter()
	router.GET("/admin/users/:id", HandleAdminUserGet)
	router.PUT("/admin/users/:id", HandleAdminUserUpdate)

	t.Run("Complete edit workflow should persist group changes", func(t *testing.T) {
		db, err := database.GetDB()
		require.NoError(t, err)
		id, _ := createIsolatedAgent(t, "bug_workflow")
		_, err = db.Exec(database.ConvertPlaceholders(`
			INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
			SELECT ?, g.id, 'rw', CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1
			FROM `+"`groups`"+` g WHERE g.name = 'users'`), id)
		require.NoError(t, err)

		// Load the edit dialog.
		req1, _ := http.NewRequest("GET", "/admin/users/"+strconv.Itoa(id), nil)
		w1 := httptest.NewRecorder()
		router.ServeHTTP(w1, req1)
		require.Equal(t, http.StatusOK, w1.Code)

		var getUserResponse map[string]interface{}
		require.NoError(t, json.Unmarshal(w1.Body.Bytes(), &getUserResponse))
		userData := getUserResponse["data"].(map[string]interface{})
		currentGroups := userData["groups"].([]interface{})
		require.Equal(t, []interface{}{"users"}, currentGroups)

		update := func(groups []string) {
			t.Helper()
			w := putAdminUserJSON(t, router, id, map[string]interface{}{
				"login":      userData["login"],
				"first_name": userData["first_name"],
				"last_name":  userData["last_name"],
				"valid_id":   userData["valid_id"],
				"groups":     groups,
			})
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		}

		// Tick "support" and save.
		update([]string{"users", "support"})
		assert.Equal(t, []string{"support", "users"}, adminTestUserGroups(t, id))

		// Untick it again and save.
		update([]string{"users"})
		groups := adminTestUserGroups(t, id)
		assert.Equal(t, []string{"users"}, groups)
		assert.NotContains(t, strings.Join(groups, ","), "support")
	})
}

// TestFormDataBindingIssues tests form-encoded group data (how HTMX sends it).
func TestFormDataBindingIssues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := seedAdminRouter()
	router.PUT("/admin/users/:id", HandleAdminUserUpdate)

	t.Run("Should handle form-encoded group data", func(t *testing.T) {
		id, login := createIsolatedAgent(t, "bug_form")
		formData := "login=" + login + "&first_name=Test&last_name=Agent&valid_id=1&groups=admin&groups=support"

		req, _ := http.NewRequest("PUT", "/admin/users/"+strconv.Itoa(id), strings.NewReader(formData))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.True(t, response["success"].(bool), "Form data should be processed successfully")
		assert.Equal(t, []string{"admin", "support"}, adminTestUserGroups(t, id))
	})
}
