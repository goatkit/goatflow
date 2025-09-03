package api

import (
	"database/sql"
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// TestGroupAssignmentWorkflow tests the complete workflow that the user reported as broken
func TestGroupAssignmentWorkflow(t *testing.T) {
	// Initialize database connection
    db, err := database.GetDB()
    if err != nil || db == nil {
        t.Skip("Database not available, skipping integration test")
    }

	// Setup test user and groups
	testUser := setupGroupAssignmentTestUser(t, db)
	defer cleanupGroupAssignmentTestUser(t, db, testUser.ID)

	// Verify test groups exist
	groups := verifyTestGroups(t, db)
	require.GreaterOrEqual(t, len(groups), 2, "Need at least 2 groups for testing")

	t.Run("RED: User group assignment via API should persist to database", func(t *testing.T) {
		// Setup Gin router
		gin.SetMode(gin.TestMode)
		router := gin.New()
		
		// Register the actual handler
		router.PUT("/admin/users/:id", HandleAdminUserUpdate)

		// Prepare form data to assign user to groups (mimicking UI behavior)
		formData := url.Values{}
		formData.Set("login", testUser.Login)
		formData.Set("first_name", testUser.FirstName)
		formData.Set("last_name", testUser.LastName)
		formData.Set("valid_id", "1")
		
		// Add multiple groups (like the UI would)
		for _, group := range groups[:2] { // Assign to first 2 groups
			formData.Add("groups", group.Name)
		}

		// Create request
		req, err := http.NewRequest("PUT", "/admin/users/"+strconv.Itoa(testUser.ID), 
			strings.NewReader(formData.Encode()))
		require.NoError(t, err)
		
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		
		// Execute request
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Check API response
		assert.Equal(t, http.StatusOK, w.Code, "API should return 200 OK")
		
		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err, "Response should be valid JSON")
		
		assert.True(t, response["success"].(bool), "API should report success")

		// THE CRITICAL TEST: Check if groups were actually saved to database
		actualGroups := getUserGroupsFromDB(t, db, testUser.ID)
		
		// This should pass if the system works correctly
		expectedGroups := []string{groups[0].Name, groups[1].Name}
		assert.ElementsMatch(t, expectedGroups, actualGroups, 
			"Database should contain the groups assigned via API")
	})

	t.Run("GREEN: Fix the group assignment persistence issue", func(t *testing.T) {
		// This test will initially fail, then pass after we fix the bug
		t.Skip("Implement after identifying the root cause")
	})

	t.Run("REFACTOR: Ensure UI feedback matches database reality", func(t *testing.T) {
		// Test that UI retrieval shows what's actually in the database  
		t.Skip("Implement after fixing the core issue")
	})
}

func TestGroupAssignmentEdgeCases(t *testing.T) {
	// Initialize database connection
    db, err := database.GetDB()
    if err != nil || db == nil {
        t.Skip("Database not available, skipping integration test")
    }

	testUser := setupGroupAssignmentTestUser(t, db)
	defer cleanupGroupAssignmentTestUser(t, db, testUser.ID)

	t.Run("Empty groups array should remove all group memberships", func(t *testing.T) {
		// First assign some groups
		assignUserToGroups(t, db, testUser.ID, []string{"admin"})
		
		// Verify groups were assigned
		groups := getUserGroupsFromDB(t, db, testUser.ID)
		require.Greater(t, len(groups), 0, "User should have groups")

		// Now update with empty groups via API
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.PUT("/admin/users/:id", HandleAdminUserUpdate)

		formData := url.Values{}
		formData.Set("login", testUser.Login)
		formData.Set("first_name", testUser.FirstName) 
		formData.Set("last_name", testUser.LastName)
		formData.Set("valid_id", "1")
		// No groups added = should clear all memberships

		req, err := http.NewRequest("PUT", "/admin/users/"+strconv.Itoa(testUser.ID),
			strings.NewReader(formData.Encode()))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Check database - should have no groups
		actualGroups := getUserGroupsFromDB(t, db, testUser.ID)
		assert.Empty(t, actualGroups, "User should have no groups after empty update")
	})

	t.Run("Invalid group names should be ignored", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.PUT("/admin/users/:id", HandleAdminUserUpdate)

		formData := url.Values{}
		formData.Set("login", testUser.Login)
		formData.Set("first_name", testUser.FirstName)
		formData.Set("last_name", testUser.LastName)
		formData.Set("valid_id", "1")
		formData.Add("groups", "admin") // Valid group
		formData.Add("groups", "nonexistent_group") // Invalid group

		req, err := http.NewRequest("PUT", "/admin/users/"+strconv.Itoa(testUser.ID),
			strings.NewReader(formData.Encode()))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Check database - should only have valid group
		actualGroups := getUserGroupsFromDB(t, db, testUser.ID)
		assert.Contains(t, actualGroups, "admin", "Should have valid group")
		assert.NotContains(t, actualGroups, "nonexistent_group", "Should not have invalid group")
	})
}

// Helper functions
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
	// Create test user
	login := "test_group_user_" + randomString(8)
	var userID int
    query := database.ConvertPlaceholders(`
        INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
        VALUES ($1, '', $2, $3, 1, NOW(), 1, NOW(), 1)
        RETURNING id`)
    err := db.QueryRow(query, login, "Test", "User").Scan(&userID)
	require.NoError(t, err, "Failed to create test user")

	return TestUser{
		ID:        userID,
		Login:     login,
		FirstName: "Test",
		LastName:  "User",
	}
}

func cleanupGroupAssignmentTestUser(t *testing.T, db *sql.DB, userID int) {
	// Clean up group memberships
    _, err := db.Exec(database.ConvertPlaceholders("DELETE FROM group_user WHERE user_id = $1"), userID)
	if err != nil {
		t.Logf("Warning: Failed to cleanup group memberships: %v", err)
	}
	
	// Clean up user
    _, err = db.Exec(database.ConvertPlaceholders("DELETE FROM users WHERE id = $1"), userID)
	if err != nil {
		t.Logf("Warning: Failed to cleanup test user: %v", err)
	}
}

func verifyTestGroups(t *testing.T, db *sql.DB) []TestGroup {
	rows, err := db.Query("SELECT id, name FROM groups WHERE valid_id = 1 ORDER BY name LIMIT 5")
	require.NoError(t, err, "Failed to query groups")
	defer rows.Close()

	var groups []TestGroup
	for rows.Next() {
		var group TestGroup
		err := rows.Scan(&group.ID, &group.Name)
		require.NoError(t, err)
		groups = append(groups, group)
	}

	return groups
}

func getUserGroupsFromDB(t *testing.T, db *sql.DB, userID int) []string {
    sqlQuery := database.ConvertPlaceholders(`
        SELECT g.name 
        FROM groups g
        JOIN group_user gu ON g.id = gu.group_id
        WHERE gu.user_id = $1 AND g.valid_id = 1
        ORDER BY g.name`)
    rows, err := db.Query(sqlQuery, userID)
	require.NoError(t, err, "Failed to query user groups")
	defer rows.Close()

	var groups []string
	for rows.Next() {
		var groupName string
		err := rows.Scan(&groupName)
		require.NoError(t, err)
		groups = append(groups, groupName)
	}

	return groups
}

func assignUserToGroups(t *testing.T, db *sql.DB, userID int, groupNames []string) {
	for _, groupName := range groupNames {
		var groupID int
		err := db.QueryRow("SELECT id FROM groups WHERE name = $1 AND valid_id = 1", groupName).Scan(&groupID)
		require.NoError(t, err, "Group %s should exist", groupName)

        _, err = db.Exec(database.ConvertPlaceholders(`
			INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
			VALUES ($1, $2, 'rw', NOW(), 1, NOW(), 1)`),
            userID, groupID)
		require.NoError(t, err, "Failed to assign user to group %s", groupName)
	}
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}