package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// TestGroupAssignmentWorkflow: groups submitted with an agent update are
// persisted to group_user.
func TestGroupAssignmentWorkflow(t *testing.T) {
	db := getTestDB(t)
	testUser := setupGroupAssignmentTestUser(t, db)

	groups := verifyTestGroups(t, db)
	require.GreaterOrEqual(t, len(groups), 2, "Need at least 2 groups for testing")

	w := putAgentGroups(t, testUser, groups[0].Name, groups[1].Name)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, true, response["success"])
	assert.ElementsMatch(t, []string{groups[0].Name, groups[1].Name}, getUserGroupsFromDB(t, db, testUser.ID))
}

func TestGroupAssignmentEdgeCases(t *testing.T) {
	db := getTestDB(t)
	testUser := setupGroupAssignmentTestUser(t, db)

	t.Run("Empty submitted groups remove all group memberships", func(t *testing.T) {
		assignUserToGroups(t, db, testUser.ID, []string{"admin"})
		require.Equal(t, []string{"admin"}, getUserGroupsFromDB(t, db, testUser.ID))

		w := putAgentGroups(t, testUser)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Empty(t, getUserGroupsFromDB(t, db, testUser.ID))
	})

	t.Run("Unknown group name is rejected with 400 and memberships stay unchanged", func(t *testing.T) {
		clearUserGroups(t, db, testUser.ID)
		assignUserToGroups(t, db, testUser.ID, []string{"users"})
		invalidGroup := fmt.Sprintf("nonexistent_group_%d", time.Now().UnixNano())

		w := putAgentGroups(t, testUser, "admin", invalidGroup)

		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.Equal(t, false, response["success"])
		assert.Equal(t, "unknown group: "+invalidGroup, response["error"])
		assert.Equal(t, []string{"users"}, getUserGroupsFromDB(t, db, testUser.ID),
			"a rejected update must not touch existing memberships")
	})

	t.Run("Valid group names and ids replace memberships", func(t *testing.T) {
		clearUserGroups(t, db, testUser.ID)
		assignUserToGroups(t, db, testUser.ID, []string{"users"})
		var adminID int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			"SELECT id FROM groups WHERE name = ? AND valid_id = 1"), "admin").Scan(&adminID))

		w := putAgentGroups(t, testUser, strconv.Itoa(adminID), "stats", "admin")

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, []string{"admin", "stats"}, getUserGroupsFromDB(t, db, testUser.ID))
	})
}

// putAgentGroups submits the agent edit form for user with the groups field
// set to the given tokens, as the seeded test admin.
func putAgentGroups(t *testing.T, user TestUser, groups ...string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/admin/users/:id", func(c *gin.Context) {
		c.Set("user_id", GetTestAuthConfig().UserID)
		HandleAdminUserUpdate(c)
	})

	formData := url.Values{}
	formData.Set("login", user.Login)
	formData.Set("first_name", user.FirstName)
	formData.Set("last_name", user.LastName)
	formData.Set("valid_id", "1")
	formData.Set("groups_submitted", "1")
	for _, g := range groups {
		formData.Add("groups", g)
	}

	req, err := http.NewRequest(http.MethodPut, "/admin/users/"+strconv.Itoa(user.ID),
		strings.NewReader(formData.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// Helper functions.
type TestUser struct {
	ID        int
	Login     string
	FirstName string
	LastName  string
}

type TestGroup struct {
	ID   int
	Name string
}

func setupGroupAssignmentTestUser(t *testing.T, db *sql.DB) TestUser {
	t.Helper()
	login := fmt.Sprintf("test_group_user_%d", time.Now().UnixNano())
	query := database.ConvertPlaceholders(`
        INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
        VALUES (?, '', ?, ?, 1, NOW(), 1, NOW(), 1)
        RETURNING id`)
	id64, err := database.GetAdapter().InsertWithReturning(db, query, login, "Test", "User")
	require.NoError(t, err, "Failed to create test user")
	userID := int(id64)
	t.Cleanup(func() { cleanupGroupAssignmentTestUser(t, db, userID) })

	return TestUser{
		ID:        userID,
		Login:     login,
		FirstName: "Test",
		LastName:  "User",
	}
}

func cleanupGroupAssignmentTestUser(t *testing.T, db *sql.DB, userID int) {
	clearUserGroups(t, db, userID)
	_, err := db.Exec(database.ConvertPlaceholders("DELETE FROM users WHERE id = ?"), userID)
	require.NoError(t, err, "Failed to cleanup test user")
}

func clearUserGroups(t *testing.T, db *sql.DB, userID int) {
	_, err := db.Exec(database.ConvertPlaceholders("DELETE FROM group_user WHERE user_id = ?"), userID)
	require.NoError(t, err, "Failed to clear group memberships")
}

func verifyTestGroups(t *testing.T, db *sql.DB) []TestGroup {
	rows, err := db.Query(database.ConvertPlaceholders("SELECT id, name FROM groups WHERE valid_id = 1 ORDER BY name LIMIT 5"))
	require.NoError(t, err, "Failed to query groups")
	defer rows.Close()

	var groups []TestGroup
	for rows.Next() {
		var group TestGroup
		require.NoError(t, rows.Scan(&group.ID, &group.Name))
		groups = append(groups, group)
	}
	require.NoError(t, rows.Err())

	return groups
}

func getUserGroupsFromDB(t *testing.T, db *sql.DB, userID int) []string {
	sqlQuery := database.ConvertPlaceholders(`
        SELECT g.name
        FROM groups g
        JOIN group_user gu ON g.id = gu.group_id
        WHERE gu.user_id = ? AND g.valid_id = 1
        ORDER BY g.name`)
	rows, err := db.Query(sqlQuery, userID)
	require.NoError(t, err, "Failed to query user groups")
	defer rows.Close()

	var groups []string
	for rows.Next() {
		var groupName string
		require.NoError(t, rows.Scan(&groupName))
		groups = append(groups, groupName)
	}
	require.NoError(t, rows.Err())

	return groups
}

func assignUserToGroups(t *testing.T, db *sql.DB, userID int, groupNames []string) {
	for _, groupName := range groupNames {
		var groupID int
		err := db.QueryRow(database.ConvertPlaceholders("SELECT id FROM groups WHERE name = ? AND valid_id = 1"), groupName).Scan(&groupID)
		require.NoError(t, err, "Group %s should exist", groupName)

		_, err = db.Exec(database.ConvertPlaceholders(`
			INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 'rw', NOW(), 1, NOW(), 1)`),
			userID, groupID)
		require.NoError(t, err, "Failed to assign user to group %s", groupName)
	}
}
