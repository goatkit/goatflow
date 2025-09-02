package api

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "net/url"
    "os"
    "strconv"
    "strings"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/gotrs-io/gotrs-ce/internal/database"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// This test focuses on the EXACT scenario the user reported:
// 1. User has groups in database (confirmed via testing)
// 2. But UI/API might not be displaying or updating them correctly
func TestRealGroupAssignmentIssue(t *testing.T) {
	config := GetTestConfig()
	
	// Manual database check first - this should pass based on our earlier query
    t.Run("Database verification: Test user should have groups", func(t *testing.T) {
        db, err := database.GetDB()
        if err != nil || db == nil {
            t.Skip("Database not available")
        }

		// Query to verify user groups
		var groups string
		err = db.QueryRow(database.ConvertPlaceholders(`
			SELECT string_agg(g.name, ', ') as groups 
			FROM users u 
			LEFT JOIN group_user gu ON u.id = gu.user_id 
			LEFT JOIN groups g ON gu.group_id = g.id 
			WHERE u.login = $1 
			GROUP BY u.id, u.login, u.first_name, u.last_name`), config.UserLogin).Scan(&groups)

		if err != nil {
			t.Logf("Could not find test user or query failed: %v", err)
			t.Skip("Test user not found - create him first through UI")
		}

		assert.NotEmpty(t, groups, "Test user should have groups in database")
		// Check for expected groups from config
		for _, expectedGroup := range config.UserGroups {
			assert.Contains(t, groups, expectedGroup, "Test user should have %s group", expectedGroup)
		}
		t.Logf("SUCCESS: Database shows test user has groups: %s", groups)
	})

    t.Run("API GET verification: Does HandleAdminUserGet return the groups?", func(t *testing.T) {
        db, err := database.GetDB()
        if err != nil || db == nil {
            t.Skip("Database not available")
        }

		// Get test user's ID
		var testUserID int
    testUsername := os.Getenv("TEST_USERNAME")
		if testUsername == "" {
			testUsername = "testuser"
		}
        err = db.QueryRow(database.ConvertPlaceholders("SELECT id FROM users WHERE login = $1"), testUsername).Scan(&testUserID)
		if err != nil {
			t.Skip("Test user not found")
		}

		// Test the HandleAdminUserGet function directly
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.GET("/admin/users/:id", HandleAdminUserGet)

		// Make request
		req, _ := http.NewRequest("GET", "/admin/users/"+strconv.Itoa(testUserID), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code, "API should return 200")
		
		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Check if response contains groups
		if data, ok := response["data"].(map[string]interface{}); ok {
			if groups, hasGroups := data["groups"]; hasGroups {
				t.Logf("SUCCESS: API returns groups: %v", groups)
			} else {
				t.Error("FAILED: API does not include groups in response")
			}
		} else {
			t.Error("FAILED: API response malformed")
		}
	})

	t.Run("Form submission test: Can we update Test user's groups via HandleAdminUserUpdate?", func(t *testing.T) {
        db, err := database.GetDB()
        if err != nil || db == nil {
            t.Skip("Database not available")
        }

        // Get test user's ID and current data
        var testUserID int
        var firstName, lastName, login string
        testUsernameLocal := os.Getenv("TEST_USERNAME")
        if testUsernameLocal == "" {
            testUsernameLocal = "testuser"
        }
        err = db.QueryRow(database.ConvertPlaceholders("SELECT id, login, first_name, last_name FROM users WHERE login = $1"), testUsernameLocal).Scan(&testUserID, &login, &firstName, &lastName)
		if err != nil {
			t.Skip("Test user not found")
		}

		// Test the HandleAdminUserUpdate function
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.PUT("/admin/users/:id", HandleAdminUserUpdate)

		// Create form data that adds test groups
		formData := url.Values{}
		formData.Set("login", login)
		formData.Set("first_name", firstName)
		formData.Set("last_name", lastName)
		formData.Set("valid_id", "1")
		// Add groups from config
		for _, group := range config.UserGroups {
			formData.Add("groups", group)
		}

		// Make request
		req, _ := http.NewRequest("PUT", "/admin/users/"+strconv.Itoa(testUserID),
			strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Check response
		t.Logf("Update response status: %d", w.Code)
		t.Logf("Update response body: %s", w.Body.String())

		// Now check if the database was actually updated
		var newGroups string
		err = db.QueryRow(database.ConvertPlaceholders(`
			SELECT string_agg(g.name, ', ') as groups 
			FROM users u 
			LEFT JOIN group_user gu ON u.id = gu.user_id 
			LEFT JOIN groups g ON gu.group_id = g.id 
			WHERE u.login = $1 
			GROUP BY u.id, u.login, u.first_name, u.last_name`), config.UserLogin).Scan(&newGroups)

		if err == nil {
			t.Logf("Groups after update: %s", newGroups)
		} else {
			t.Logf("Could not query groups after update: %v", err)
		}
	})
}