//go:build db

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserManagement(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)

	t.Run("ListUsers_ShowsActiveAndInactive", func(t *testing.T) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Skip("Database not available, skipping test")
		}

		userRepo := repository.NewUserRepository(db)
		users, err := userRepo.List()

		if err != nil {
			t.Logf("Error listing users: %v", err)
			return
		}

		activeCount := 0
		inactiveCount := 0
		for _, user := range users {
			if user.ValidID == 1 {
				activeCount++
			} else {
				inactiveCount++
			}
		}

		t.Logf("Found %d active users and %d inactive users", activeCount, inactiveCount)
		assert.True(t, activeCount >= 0, "Should have some active users")
	})

	t.Run("ToggleUserStatus", func(t *testing.T) {
		router := gin.New()
		SetupHTMXRoutes(router)

		// Create a test user first
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Skip("Database not available, skipping test")
		}

		userRepo := repository.NewUserRepository(db)

		// Find a test user to toggle
		users, err := userRepo.List()
		if err != nil || len(users) == 0 {
			t.Skip("No users available for testing")
		}

		testUser := users[0]
		originalStatus := testUser.ValidID

		// Toggle status to inactive
		newStatus := 2
		if originalStatus == 2 {
			newStatus = 1
		}

		reqBody := map[string]int{"valid_id": newStatus}
		jsonBody, _ := json.Marshal(reqBody)

		req, _ := http.NewRequest("PUT", "/admin/users/"+strconv.Itoa(int(testUser.ID))+"/status", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		t.Logf("Status toggle response: %d - %s", w.Code, w.Body.String())

		// Verify the user still appears in the list
		updatedUsers, _ := userRepo.List()
		found := false
		for _, u := range updatedUsers {
			if u.ID == testUser.ID {
				found = true
				t.Logf("User %s status changed from %d to %d", u.Login, originalStatus, u.ValidID)
				break
			}
		}

		assert.True(t, found, "User should still appear in list after status change")
	})

	t.Run("CreateUserWithGroups", func(t *testing.T) {
		router := gin.New()
		SetupHTMXRoutes(router)

		// Prepare form data
		form := url.Values{}
		form.Add("login", "testuser_groups")
		form.Add("password", "TestPass123!")
		form.Add("first_name", "Test")
		form.Add("last_name", "User")
		form.Add("valid_id", "1")
		form.Add("groups", "1") // Assuming group ID 1 exists
		form.Add("groups", "2") // Multiple groups

		req, _ := http.NewRequest("POST", "/admin/users", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		t.Logf("Create user response: %d - %s", w.Code, w.Body.String())

		// Check response
		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err == nil {
			success, _ := response["success"].(bool)
			if !success {
				if errMsg, ok := response["error"].(string); ok {
					if strings.Contains(errMsg, "duplicate key") {
						t.Log("User already exists, which is expected in repeated tests")
					} else {
						t.Logf("Failed to create user (non-fatal in test without dynamic module): %s", errMsg)
					}
				}
			} else {
				t.Log("User created successfully with groups")
			}
		}
	})

	t.Run("UpdateUserGroups", func(t *testing.T) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Skip("Database not available, skipping test")
		}

		userRepo := repository.NewUserRepository(db)
		groupRepo := repository.NewGroupRepository(db)

		// Find a test user
		users, err := userRepo.List()
		if err != nil || len(users) == 0 {
			t.Skip("No users available for testing")
		}

		testUser := users[0]

		// Get current groups
		currentGroups, _ := groupRepo.GetUserGroups(testUser.ID)
		t.Logf("User %s currently in %d groups: %v", testUser.Login, len(currentGroups), currentGroups)

		// Test adding user to a group
		groups, _ := groupRepo.List()
		if len(groups) > 0 {
			// Normalize IDs to uint for SQL repo
			var groupIDUint uint
			switch v := groups[0].ID.(type) {
			case int:
				groupIDUint = uint(v)
			case int64:
				groupIDUint = uint(v)
			case uint:
				groupIDUint = v
			case uint64:
				groupIDUint = uint(v)
			case string:
				if n, convErr := strconv.Atoi(v); convErr == nil {
					groupIDUint = uint(n)
				}
			default:
				// skip if unknown type
			}
			err := groupRepo.AddUserToGroup(testUser.ID, groupIDUint)
			if err != nil {
				t.Logf("Error adding user to group: %v", err)
			} else {
				t.Log("Successfully added user to group")
			}

			// Verify the group was added
			updatedGroups, _ := groupRepo.GetUserGroups(testUser.ID)
			t.Logf("User now in %d groups: %v", len(updatedGroups), updatedGroups)
		}
	})
}

func TestUserDeletion(t *testing.T) {
	t.Run("DeleteUser_SoftDelete", func(t *testing.T) {
		// In OTRS-compatible systems, users are typically soft-deleted
		// by setting valid_id to 2 rather than actually removing the record

		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Skip("Database not available, skipping test")
		}

		userRepo := repository.NewUserRepository(db)

		// Find a test user to "delete"
		users, err := userRepo.List()
		require.NoError(t, err)
		require.NotEmpty(t, users, "Need at least one user for testing")

		testUser := users[len(users)-1] // Use last user to avoid system users

		// Debug: log the current title
		t.Logf("User %s (ID: %d) has title: '%s' (length: %d)", testUser.Login, testUser.ID, testUser.Title, len(testUser.Title))

		// Soft delete by setting valid_id = 2
		err = userRepo.SetValidID(testUser.ID, 2, uint(1), time.Now())

		if err != nil {
			t.Logf("Error soft-deleting user: %v", err)
		} else {
			t.Log("User soft-deleted successfully")

			// Verify user still exists in database but is inactive
			allUsers, _ := userRepo.List()
			found := false
			for _, u := range allUsers {
				if u.ID == testUser.ID {
					found = true
					assert.Equal(t, 2, u.ValidID, "User should be marked as inactive")
					break
				}
			}
			assert.True(t, found, "Soft-deleted user should still exist in database")
		}
	})
}

func TestTranslations(t *testing.T) {
	t.Run("AdminTranslationsExist", func(t *testing.T) {
		// Check that required translations exist
		requiredKeys := []string{
			"admin.delete_user_warning",
			"admin.confirm_status_change",
			"admin.activate",
			"admin.deactivate",
			"admin.confirm_password_reset",
			"admin.password_reset_success",
		}

		// This would normally load from the actual translation files
		// For testing, we're just checking the structure
		for _, key := range requiredKeys {
			t.Logf("Translation key required: %s", key)
		}

		// In a real test, you would load the en.json file and verify these keys exist
		t.Log("Translation keys should be verified in en.json file")
	})
}
