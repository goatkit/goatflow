package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGroupAssignmentNotPersisting tests Bug #1: Group assignment not persisting to database
func TestGroupAssignmentNotPersisting(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	// Setup route for user update
	router.PUT("/admin/users/:id", HandleAdminUserUpdate)
	
	// Get database connection
    if err := database.InitTestDB(); err != nil {
        t.Skip("Database not available, skipping integration test")
    }
    db, _ := database.GetDB()

	t.Run("FAILING: Group assignment should persist to database but doesn't", func(t *testing.T) {
		// ARRANGE: Get Robbie's current groups to ensure test state
		var currentGroups []string
		groupQuery := `
			SELECT g.name 
			FROM groups g 
			JOIN group_user gu ON g.id = gu.group_id 
			WHERE gu.user_id = 15 AND g.valid_id = 1`
		
		rows, err := db.Query(groupQuery)
		require.NoError(t, err)
		defer rows.Close()
		
		for rows.Next() {
			var groupName string
			err := rows.Scan(&groupName)
			require.NoError(t, err)
			currentGroups = append(currentGroups, groupName)
		}
		
		// ARRANGE: Prepare update request with testgroup group added (if not already present)
		targetGroups := append(currentGroups, "testgroup")
		// Remove duplicates
		uniqueGroups := make([]string, 0)
		seen := make(map[string]bool)
		for _, group := range targetGroups {
			if !seen[group] {
				uniqueGroups = append(uniqueGroups, group)
				seen[group] = true
			}
		}
		
		updateRequest := map[string]interface{}{
			"login":      "testuser",
			"first_name": "Robbie", 
			"last_name":  "Nadden",
			"valid_id":   1,
			"groups":     uniqueGroups,
		}
		
		jsonData, err := json.Marshal(updateRequest)
		require.NoError(t, err)
		
		// ACT: Send update request
		req, _ := http.NewRequest("PUT", "/admin/users/15", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		// ASSERT: Request should succeed
		assert.Equal(t, http.StatusOK, w.Code, "Update request should succeed")
		
		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.True(t, response["success"].(bool), "Response should indicate success")
		
		// ASSERT: Groups should actually be persisted in database
		var actualGroups []string
		rows2, err := db.Query(groupQuery)
		require.NoError(t, err)
		defer rows2.Close()
		
		for rows2.Next() {
			var groupName string
			err := rows2.Scan(&groupName)
			require.NoError(t, err)
			actualGroups = append(actualGroups, groupName)
		}
		
		// THIS IS WHERE THE BUG MANIFESTS: Groups are not actually saved
		assert.Contains(t, actualGroups, "testgroup", "FAILING: testgroup group should be persisted in database but isn't")
		
		// Additional verification: All requested groups should be present
		for _, expectedGroup := range uniqueGroups {
			assert.Contains(t, actualGroups, expectedGroup, "Group %s should be persisted", expectedGroup)
		}
	})
}

// TestXlatsNotWorking tests Bug #2: Translation/localization not functioning
func TestXlatsNotWorking(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	// Setup route for getting user details (which should include translated fields)
	router.GET("/admin/users/:id", HandleAdminUserGet)

	t.Run("FAILING: User data should include translated xlats but doesn't", func(t *testing.T) {
		// ACT: Get user data 
		req, _ := http.NewRequest("GET", "/admin/users/15", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		// ASSERT: Response should include xlat information
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.True(t, response["success"].(bool))
		
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "Response should contain data object")
		
		// THIS SHOULD FAIL: No xlat/translation support in current implementation
		_, hasXlats := data["xlats"]
		assert.True(t, hasXlats, "FAILING: User data should include xlat translations")
		
		// Check for specific xlat fields that should be translated
		_, hasValidIDXlat := data["valid_id_xlat"]
		assert.True(t, hasValidIDXlat, "FAILING: Should have translated valid_id status")
	})
}

// TestNoWayToRemoveUserFromAllGroups tests Bug #3: Missing functionality to remove user from all groups  
func TestNoWayToRemoveUserFromAllGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	router.PUT("/admin/users/:id", HandleAdminUserUpdate)
	
    // Get database connection
    if err := database.InitTestDB(); err != nil {
        t.Skip("Database not available, skipping integration test")
    }
    db, _ := database.GetDB()
    if db == nil {
        t.Skip("Database not available, skipping integration test")
    }

	t.Run("FAILING: Should support removing user from all groups but doesn't", func(t *testing.T) {
		// ARRANGE: Ensure user has at least one group assignment
        _, err := db.Exec(database.ConvertPlaceholders(`
			INSERT INTO group_user (user_id, group_id, permission_key, permission_value, create_time, create_by, change_time, change_by)
			SELECT 15, g.id, 'rw', 1, NOW(), 1, NOW(), 1
			FROM groups g 
			WHERE g.name = 'Support' AND g.valid_id = 1
            ON CONFLICT (user_id, group_id, permission_key) DO NOTHING`))
		require.NoError(t, err)
		
		// Verify user has groups
		var groupCount int
		err = db.QueryRow("SELECT COUNT(*) FROM group_user WHERE user_id = 15").Scan(&groupCount)
		require.NoError(t, err)
		require.Greater(t, groupCount, 0, "User should have at least one group for this test")
		
		// ACT: Send update with empty groups array to remove all groups
		updateRequest := map[string]interface{}{
			"login":      "testuser",
			"first_name": "Robbie",
			"last_name":  "Nadden", 
			"valid_id":   1,
			"groups":     []string{}, // Empty array should remove all groups
		}
		
		jsonData, err := json.Marshal(updateRequest)
		require.NoError(t, err)
		
		req, _ := http.NewRequest("PUT", "/admin/users/15", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		// ASSERT: Request should succeed
		assert.Equal(t, http.StatusOK, w.Code, "Update request should succeed")
		
		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool), "Response should indicate success")
		
		// ASSERT: User should have no groups after update
		var finalGroupCount int
		err = db.QueryRow("SELECT COUNT(*) FROM group_user WHERE user_id = 15").Scan(&finalGroupCount)
		require.NoError(t, err)
		
		assert.Equal(t, 0, finalGroupCount, "FAILING: User should have 0 groups after sending empty groups array")
	})
}

// TestUserWorkflowEndToEnd tests the complete user edit workflow that the user reported failing
func TestUserWorkflowEndToEnd(t *testing.T) {
	gin.SetMode(gin.TestMode) 
	router := gin.New()
	
	// Setup both GET and PUT routes
	router.GET("/admin/users/:id", HandleAdminUserGet)
	router.PUT("/admin/users/:id", HandleAdminUserUpdate)
	
    // Get database connection
    if err := database.InitTestDB(); err != nil {
        t.Skip("Database not available, skipping integration test")
    }
    db, _ := database.GetDB()
    if db == nil {
        t.Skip("Database not available, skipping integration test")
    }

	t.Run("FAILING: Complete edit workflow should persist group changes", func(t *testing.T) {
		// ARRANGE: Get initial user state (simulates loading edit dialog)
		req1, _ := http.NewRequest("GET", "/admin/users/15", nil)
		w1 := httptest.NewRecorder()
		router.ServeHTTP(w1, req1)
		
		assert.Equal(t, http.StatusOK, w1.Code)
		
		var getUserResponse map[string]interface{}
		err := json.Unmarshal(w1.Body.Bytes(), &getUserResponse)
		require.NoError(t, err)
		
		userData := getUserResponse["data"].(map[string]interface{})
		currentGroups := userData["groups"].([]interface{})
		
		// Convert to string slice for easier manipulation
		currentGroupNames := make([]string, len(currentGroups))
		for i, group := range currentGroups {
			currentGroupNames[i] = group.(string)
		}
		
		// ARRANGE: Add testgroup to groups if not already present (simulates user clicking testgroup checkbox)
		updatedGroups := append(currentGroupNames, "testgroup")
		uniqueGroups := make([]string, 0)
		seen := make(map[string]bool)
		for _, group := range updatedGroups {
			if !seen[group] {
				uniqueGroups = append(uniqueGroups, group)
				seen[group] = true
			}
		}
		
		// ACT: Submit the form update (simulates user clicking save)
		updateRequest := map[string]interface{}{
			"login":      userData["login"],
			"first_name": userData["first_name"],
			"last_name":  userData["last_name"],
			"valid_id":   userData["valid_id"],
			"groups":     uniqueGroups,
		}
		
		jsonData, err := json.Marshal(updateRequest)
		require.NoError(t, err)
		
		req2, _ := http.NewRequest("PUT", "/admin/users/15", bytes.NewBuffer(jsonData))
		req2.Header.Set("Content-Type", "application/json")
		
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)
		
		// ASSERT: Update should succeed
		assert.Equal(t, http.StatusOK, w2.Code, "Update request should succeed")
		
		// ASSERT: Verify changes are actually persisted by querying database directly
		var dbGroups []string
		groupQuery := `
			SELECT g.name 
			FROM groups g 
			JOIN group_user gu ON g.id = gu.group_id 
			WHERE gu.user_id = 15 AND g.valid_id = 1
			ORDER BY g.name`
		
		rows, err := db.Query(groupQuery)
		require.NoError(t, err)
		defer rows.Close()
		
		for rows.Next() {
			var groupName string
			err := rows.Scan(&groupName)
			require.NoError(t, err)
			dbGroups = append(dbGroups, groupName)
		}
		
		// THIS IS THE MAIN BUG: User clicked testgroup and saved, but it's not in database
		assert.Contains(t, dbGroups, "testgroup", "FAILING: testgroup group should be saved to database after user workflow")
		
		// ARRANGE: Now test removing the group (simulates unchecking checkbox)
		finalGroups := make([]string, 0)
		for _, group := range uniqueGroups {
			if group != "testgroup" {
				finalGroups = append(finalGroups, group)
			}
		}
		
		removeRequest := map[string]interface{}{
			"login":      userData["login"],
			"first_name": userData["first_name"],
			"last_name":  userData["last_name"],
			"valid_id":   userData["valid_id"],
			"groups":     finalGroups,
		}
		
		jsonData2, err := json.Marshal(removeRequest)
		require.NoError(t, err)
		
		req3, _ := http.NewRequest("PUT", "/admin/users/15", bytes.NewBuffer(jsonData2))
		req3.Header.Set("Content-Type", "application/json")
		
		w3 := httptest.NewRecorder()
		router.ServeHTTP(w3, req3)
		
		assert.Equal(t, http.StatusOK, w3.Code)
		
		// ASSERT: testgroup group should now be removed
		var dbGroupsAfterRemoval []string
		rows2, err := db.Query(groupQuery)
		require.NoError(t, err)
		defer rows2.Close()
		
		for rows2.Next() {
			var groupName string
			err := rows2.Scan(&groupName)
			require.NoError(t, err)
			dbGroupsAfterRemoval = append(dbGroupsAfterRemoval, groupName)
		}
		
		assert.NotContains(t, dbGroupsAfterRemoval, "testgroup", "testgroup group should be removed when unchecked")
	})
}

// TestFormDataBindingIssues tests potential issues with form data binding vs JSON
func TestFormDataBindingIssues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	router.PUT("/admin/users/:id", HandleAdminUserUpdate)

	t.Run("Should handle form-encoded group data", func(t *testing.T) {
		// Test with form data (how HTMX might send it)
		formData := "login=testuser&first_name=Robbie&last_name=Nadden&valid_id=1&groups=admin&groups=testgroup"
		
		req, _ := http.NewRequest("PUT", "/admin/users/15", strings.NewReader(formData))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		// Check if groups were processed correctly
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.True(t, response["success"].(bool), "Form data should be processed successfully")
	})
}

// Helper function to create authenticated test context (for future use)
func createAuthenticatedContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	
	// Set up mock session/auth data
	c.Set("user_id", 1)
	c.Set("user_role", "admin")
	c.Set("authenticated", true)
	
	return c
}