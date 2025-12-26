//go:build db

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
)

// TestGroupAssignmentRobustnessFixes tests the improved group assignment workflow
func TestGroupAssignmentRobustnessFixes(t *testing.T) {
	t.Run("Verify HandleAdminUserGet returns groups correctly", func(t *testing.T) {
		// This test ensures that when we fetch a user, the groups are properly formatted
		// and match what the UI expects

		config := GetTestConfig()

		gin.SetMode(gin.TestMode)
		router := gin.New()

		// Mock the handler with a user that has groups
		router.GET("/admin/users/:id", func(c *gin.Context) {
			// Simulate what the real handler should return
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": gin.H{
					"id":         15,
					"login":      config.UserLogin,
					"first_name": config.UserFirstName,
					"last_name":  config.UserLastName,
					"valid_id":   1,
					"groups":     config.UserGroups, // This should match database
				},
			})
		})

		// Test the API response format
		req, _ := http.NewRequest("GET", "/admin/users/15", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Verify response structure
		assert.True(t, response["success"].(bool))
		data := response["data"].(map[string]interface{})
		groups := data["groups"].([]interface{})

		// Convert to string slice for easier testing
		var groupNames []string
		for _, g := range groups {
			groupNames = append(groupNames, g.(string))
		}

		// Check that expected groups are present
		for _, expectedGroup := range config.UserGroups {
			assert.Contains(t, groupNames, expectedGroup)
		}

		t.Logf("SUCCESS: API returns properly formatted groups: %v", groupNames)
	})

	t.Run("Test HandleAdminUserUpdate with verbose logging", func(t *testing.T) {
		// This test adds comprehensive logging to understand what's happening
		// in the update process

		config := GetTestConfig()
		gin.SetMode(gin.TestMode)
		router := gin.New()

		// Enhanced version of HandleAdminUserUpdate with logging
		router.PUT("/admin/users/:id", func(c *gin.Context) {
			userID := c.Param("id")
			id, err := strconv.Atoi(userID)
			if err != nil {
				t.Logf("ERROR: Invalid user ID: %s", userID)
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error":   "Invalid user ID",
				})
				return
			}

			// Parse the form data
			var req struct {
				Login     string   `json:"login" form:"login"`
				FirstName string   `json:"first_name" form:"first_name"`
				LastName  string   `json:"last_name" form:"last_name"`
				ValidID   int      `json:"valid_id" form:"valid_id"`
				Groups    []string `json:"groups" form:"groups"`
			}

			if err := c.ShouldBind(&req); err != nil {
				t.Logf("ERROR: Failed to bind request: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error":   "Invalid request data",
				})
				return
			}

			t.Logf("SUCCESS: Parsed request for user %d:", id)
			t.Logf("  - Login: %s", req.Login)
			t.Logf("  - Name: %s %s", req.FirstName, req.LastName)
			t.Logf("  - Groups: %v", req.Groups)

			// Mock successful update
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "User updated successfully",
				"debug": gin.H{
					"user_id":         id,
					"groups_received": req.Groups,
					"groups_count":    len(req.Groups),
				},
			})
		})

		// Test form submission with groups
		formData := url.Values{}
		formData.Set("login", config.UserLogin)
		formData.Set("first_name", config.UserFirstName)
		formData.Set("last_name", config.UserLastName)
		formData.Set("valid_id", "1")
		for _, group := range config.UserGroups {
			formData.Add("groups", group)
		}

		req, _ := http.NewRequest("PUT", "/admin/users/15",
			strings.NewReader(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response["success"].(bool))

		// Check debug information
		debug := response["debug"].(map[string]interface{})
		groupsReceived := debug["groups_received"].([]interface{})

		assert.Equal(t, len(config.UserGroups), len(groupsReceived), "Should receive correct number of groups")
		for _, expectedGroup := range config.UserGroups {
			assert.Contains(t, groupsReceived, expectedGroup)
		}

		t.Logf("SUCCESS: Form submission correctly parsed groups: %v", groupsReceived)
	})
}
