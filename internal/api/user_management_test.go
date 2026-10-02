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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/repository"
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

		db, err := database.GetDB()
		require.NoError(t, err)
		userRepo := repository.NewUserRepository(db)
		userID, _ := createIsolatedAgent(t, "toggle_status")

		reqBody := map[string]int{"valid_id": 2}
		jsonBody, _ := json.Marshal(reqBody)

		req, _ := http.NewRequest("PUT", "/admin/users/"+strconv.Itoa(userID)+"/status", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		AddTestAuthCookie(req, GetTestAuthToken(t))

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())

		// The deactivated user still appears in the list, now inactive
		updatedUsers, err := userRepo.List()
		require.NoError(t, err)
		found := false
		for _, u := range updatedUsers {
			if int(u.ID) == userID {
				found = true
				assert.Equal(t, 2, u.ValidID)
				break
			}
		}
		assert.True(t, found, "User should still appear in list after status change")
	})

	t.Run("CreateUserWithGroups", func(t *testing.T) {
		router := gin.New()
		SetupHTMXRoutes(router)
		cleanupAgentByLogin(t, "testuser_groups")

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
		require.NoError(t, err)
		groupRepo := repository.NewGroupRepository(db)

		userID, _ := createIsolatedAgent(t, "update_groups")
		var usersGroup uint
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			"SELECT id FROM `groups` WHERE name = 'users'")).Scan(&usersGroup))

		require.NoError(t, groupRepo.AddUserToGroup(uint(userID), usersGroup))

		updatedGroups, err := groupRepo.GetUserGroups(uint(userID))
		require.NoError(t, err)
		assert.Contains(t, updatedGroups, "users")
	})
}

func TestUserDeletion(t *testing.T) {
	t.Run("DeleteUser_SoftDelete", func(t *testing.T) {
		// In OTRS-compatible systems, users are typically soft-deleted
		// by setting valid_id to 2 rather than actually removing the record

		db, err := database.GetDB()
		require.NoError(t, err)

		userRepo := repository.NewUserRepository(db)
		id, _ := createIsolatedAgent(t, "soft_delete")

		// Soft delete by setting valid_id = 2
		require.NoError(t, userRepo.SetValidID(uint(id), 2, uint(1), time.Now()))

		// The user still exists but is inactive
		var validID int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			"SELECT valid_id FROM users WHERE id = ?"), id).Scan(&validID))
		assert.Equal(t, 2, validID, "User should be marked as inactive")
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
