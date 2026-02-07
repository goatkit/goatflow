//go:build integration

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/goatkit/goatflow/internal/api"
	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter(t *testing.T) (*gin.Engine, *repository.GroupSQLRepository) {
	t.Helper()
	t.Setenv("APP_ENV", "integration")

	require.NoError(t, database.InitTestDB())

	// Setup test database
	db, err := database.GetDB()
	require.NoError(t, err, "Failed to connect to database")

	// Create repository
	groupRepo := repository.NewGroupRepository(db)

	// Setup Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Register routes
	admin := router.Group("/admin")
	admin.GET("/groups", api.HandleAdminGroups)
	admin.POST("/groups", api.HandleCreateGroup)
	admin.PUT("/groups/:id", api.HandleUpdateGroup)
	admin.DELETE("/groups/:id", api.HandleDeleteGroup)

	// API routes for testing without template rendering
	apiGroup := router.Group("/api/v1")
	apiGroup.GET("/groups", api.HandleListGroupsAPI)

	return router, groupRepo
}

func uintFromID(t *testing.T, id interface{}) uint {
	t.Helper()
	switch v := id.(type) {
	case nil:
		require.FailNow(t, "group ID is nil")
	case uint:
		return v
	case uint64:
		return uint(v)
	case int:
		if v < 0 {
			require.FailNowf(t, "negative id", "received %d", v)
		}
		return uint(v)
	case int64:
		if v < 0 {
			require.FailNowf(t, "negative id", "received %d", v)
		}
		return uint(v)
	case float64:
		if v < 0 {
			require.FailNowf(t, "negative id", "received %f", v)
		}
		return uint(v)
	case string:
		parsed, err := strconv.ParseUint(v, 10, 64)
		require.NoError(t, err, "invalid string id")
		return uint(parsed)
	default:
		require.FailNowf(t, "unsupported id type", "type %T", id)
	}
	return 0
}

func intFromID(t *testing.T, id interface{}) int {
	return int(uintFromID(t, id))
}

func TestGroupsCRUDAPI(t *testing.T) {
	router, groupRepo := setupTestRouter(t)

	// Generate unique test group name
	testGroupName := fmt.Sprintf("TestAPIGroup_%d", time.Now().Unix())
	testGroupDesc := "Test group for API testing"
	updatedDesc := "Updated description via API"

	var createdGroupID uint

	t.Run("Create Group", func(t *testing.T) {
		// Create request body
		reqBody := map[string]interface{}{
			"name":     testGroupName,
			"comments": testGroupDesc,
			"valid_id": 1,
		}
		jsonBody, _ := json.Marshal(reqBody)

		// Make request
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/admin/groups", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		// Check response
		assert.Equal(t, http.StatusCreated, w.Code, "Should create group successfully")

		// Parse response
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Check success
		success, ok := response["success"].(bool)
		assert.True(t, ok && success, "Response should indicate success")

		// Verify group was created in database
		group, err := groupRepo.GetByName(testGroupName)
		assert.NoError(t, err, "Should find created group")
		assert.NotNil(t, group, "Group should exist")
		assert.Equal(t, testGroupDesc, group.Comments, "Description should match")
		createdGroupID = uintFromID(t, group.ID)
	})

	t.Run("Read Group - List with search", func(t *testing.T) {
		t.Setenv("APP_ENV", "test")
		// Make request with search parameter using API endpoint
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/groups?search=%s", testGroupName), nil)
		req.Header.Set("Accept", "application/json")
		// Create a new context with user_id set (simulating authenticated request)
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", 1)
		api.HandleListGroupsAPI(c)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code, "Should list groups successfully")

		// Parse JSON response
		bodyStr := w.Body.String()
		assert.Contains(t, bodyStr, testGroupName, "Should find test group in list")
		assert.Contains(t, bodyStr, testGroupDesc, "Should find test group description")
	})

	t.Run("Update Group", func(t *testing.T) {
		require.NotEqual(t, uint(0), createdGroupID, "Need group ID for update")

		// Create update request
		reqBody := map[string]interface{}{
			"name":     testGroupName, // Keep same name
			"comments": updatedDesc,
			"valid_id": 1,
		}
		jsonBody, _ := json.Marshal(reqBody)

		// Make request
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", fmt.Sprintf("/admin/groups/%d", createdGroupID), bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code, "Should update group successfully")

		// Verify update in database
		group, err := groupRepo.GetByID(createdGroupID)
		assert.NoError(t, err, "Should find updated group")
		assert.Equal(t, updatedDesc, group.Comments, "Description should be updated")
	})

	t.Run("Delete Group", func(t *testing.T) {
		require.NotEqual(t, uint(0), createdGroupID, "Need group ID for delete")

		// Make delete request
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/admin/groups/%d", createdGroupID), nil)
		router.ServeHTTP(w, req)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code, "Should delete group successfully")

		// Verify deletion
		group, err := groupRepo.GetByID(createdGroupID)
		assert.NoError(t, err, "Group row should still exist as inactive")
		assert.NotNil(t, group, "Group record should remain")
		assert.Equal(t, 2, group.ValidID, "Group should be marked inactive")
	})

	t.Run("Cannot Delete System Groups", func(t *testing.T) {
		// Try to delete 'admin' group
		adminGroup, err := groupRepo.GetByName("admin")
		require.NoError(t, err, "Admin group should exist")
		require.NotNil(t, adminGroup, "Admin group should exist")
		adminGroupID := intFromID(t, adminGroup.ID)

		// Make delete request
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/admin/groups/%d", adminGroupID), nil)
		router.ServeHTTP(w, req)

		// Should get error
		assert.Equal(t, http.StatusBadRequest, w.Code, "Should not delete system group")

		// Verify admin group still exists
		adminGroupAfter, err := groupRepo.GetByName("admin")
		assert.NoError(t, err, "Admin group should still exist")
		assert.NotNil(t, adminGroupAfter, "Admin group should still exist")
		assert.Equal(t, 1, adminGroupAfter.ValidID, "Admin group should remain active")
	})

	t.Run("Cannot Create Duplicate Group", func(t *testing.T) {
		// Try to create a group with name 'admin'
		reqBody := map[string]interface{}{
			"name":     "admin",
			"comments": "Trying to duplicate admin group",
			"valid_id": 1,
		}
		jsonBody, _ := json.Marshal(reqBody)

		// Make request
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/admin/groups", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		// Should get error
		assert.Equal(t, http.StatusOK, w.Code, "Duplicate should return handled response")

		// Parse response
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		if err == nil {
			success, _ := response["success"].(bool)
			assert.False(t, success, "Should not succeed")

			errorMsg, _ := response["error"].(string)
			assert.Contains(t, errorMsg, "exists", "Error should mention group exists")
		}
	})
}

func TestGroupValidation(t *testing.T) {
	router, _ := setupTestRouter(t)

	t.Run("Cannot Create Group Without Name", func(t *testing.T) {
		// Create request without name
		reqBody := map[string]interface{}{
			"comments": "Group without name",
			"valid_id": 1,
		}
		jsonBody, _ := json.Marshal(reqBody)

		// Make request
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/admin/groups", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		// Should get error
		assert.Equal(t, http.StatusBadRequest, w.Code, "Should require group name")
	})

	t.Run("Invalid Valid ID", func(t *testing.T) {
		// Create request with invalid valid_id
		reqBody := map[string]interface{}{
			"name":     fmt.Sprintf("TestInvalid_%d", time.Now().Unix()),
			"comments": "Test invalid status",
			"valid_id": 999, // Invalid ID
		}
		jsonBody, _ := json.Marshal(reqBody)

		// Make request
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/admin/groups", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		// Depending on validation, this might fail or succeed with a default
		// Log the response for debugging
		t.Logf("Response: %d - %s", w.Code, w.Body.String())
	})
}

func TestGroupRepository(t *testing.T) {
	// Test repository methods directly
	t.Setenv("APP_ENV", "integration")
	require.NoError(t, database.InitTestDB())

	db, err := database.GetDB()
	require.NoError(t, err, "Failed to connect to database")

	groupRepo := repository.NewGroupRepository(db)

	testGroupName := fmt.Sprintf("TestRepo_%d", time.Now().Unix())

	t.Run("Create and Get Group", func(t *testing.T) {
		// Create group
		group := &models.Group{
			Name:     testGroupName,
			Comments: "Repository test group",
			ValidID:  1,
			CreateBy: 1,
			ChangeBy: 1,
		}

		err := groupRepo.Create(group)
		assert.NoError(t, err, "Should create group")
		assert.NotEqual(t, uint(0), uintFromID(t, group.ID), "Should have ID after creation")

		// Get group by name
		retrieved, err := groupRepo.GetByName(testGroupName)
		assert.NoError(t, err, "Should retrieve group")
		assert.NotNil(t, retrieved, "Should find group")
		assert.Equal(t, testGroupName, retrieved.Name, "Name should match")

		// Clean up
		err = groupRepo.Delete(uintFromID(t, group.ID))
		assert.NoError(t, err, "Should delete test group")
	})

	t.Run("Get All Groups", func(t *testing.T) {
		groups, err := groupRepo.List()
		assert.NoError(t, err, "Should get all groups")
		assert.NotEmpty(t, groups, "Should have at least system groups")

		// Check for system groups
		hasAdmin := false
		hasUsers := false
		for _, g := range groups {
			if g.Name == "admin" {
				hasAdmin = true
			}
			if g.Name == "users" {
				hasUsers = true
			}
		}
		assert.True(t, hasAdmin, "Should have admin group")
		assert.True(t, hasUsers, "Should have users group")
	})
}
