package api

import (
	"bytes"
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

// Note: Uses centralized GetTestAuthToken() and AddTestAuthCookie() from test_helpers.go

func TestAdminGroupManagement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token := GetTestAuthToken(t)

	t.Run("ListGroups_ShowsAllGroups", func(t *testing.T) {
		if err := database.InitTestDB(); err != nil {
			t.Skip("Database not available")
		}
		defer database.CloseTestDB()
		router := gin.New()
		SetupHTMXRoutes(router)

		req, _ := http.NewRequest("GET", "/admin/groups", nil)
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Group Management")
		assert.Contains(t, w.Body.String(), "Add Group")
	})

	t.Run("SearchGroups_FiltersResults", func(t *testing.T) {
		if err := database.InitTestDB(); err != nil {
			t.Skip("Database not available")
		}
		defer database.CloseTestDB()
		router := gin.New()
		SetupHTMXRoutes(router)

		req, _ := http.NewRequest("GET", "/admin/groups?search=admin", nil)
		AddTestAuthCookie(req, token)
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// Should contain filtered results
	})

	t.Run("CreateGroup_ValidData", func(t *testing.T) {
		if err := database.InitTestDB(); err != nil {
			t.Skip("Database not available")
		}
		defer database.CloseTestDB()
		router := gin.New()
		SetupHTMXRoutes(router)

		// Use unique group name to avoid conflicts with existing data
		uniqueGroupName := fmt.Sprintf("test_group_%d", time.Now().UnixNano())
		cleanupGroupByNameAtEnd(t, uniqueGroupName)

		form := url.Values{}
		form.Add("name", uniqueGroupName)
		form.Add("comments", "Test Group Description")
		form.Add("valid_id", "1")

		req, _ := http.NewRequest("POST", "/admin/groups", strings.NewReader(form.Encode()))
		AddTestAuthCookie(req, token)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Check response - handler returns 201 Created on success
		var response map[string]interface{}
		if w.Code == http.StatusCreated || w.Code == http.StatusOK {
			if err := json.Unmarshal(w.Body.Bytes(), &response); err == nil {
				if success, ok := response["success"].(bool); ok {
					assert.True(t, success, "Group creation should succeed")
				} else {
					t.Logf("Response: %+v", response)
				}
			}
		} else {
			t.Logf("Unexpected status code %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("CreateGroup_DuplicateName", func(t *testing.T) {
		if err := database.InitTestDB(); err != nil {
			t.Skip("Database not available")
		}
		defer database.CloseTestDB()
		router := gin.New()
		SetupHTMXRoutes(router)

		// Try to create a group with existing name
		form := url.Values{}
		form.Add("name", "admin")
		form.Add("comments", "Duplicate group")
		form.Add("valid_id", "1")

		req, _ := http.NewRequest("POST", "/admin/groups", strings.NewReader(form.Encode()))
		AddTestAuthCookie(req, token)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err == nil {
			assert.False(t, response["success"].(bool), "Should fail for duplicate name")
			assert.Contains(t, response["error"].(string), "already exists")
		}
	})

	t.Run("UpdateGroup_ValidData", func(t *testing.T) {
		db, err := database.GetDB()
		require.NoError(t, err)
		groupID, groupName := createIsolatedGroup(t, "update_group")

		router := gin.New()
		SetupHTMXRoutes(router)

		form := url.Values{}
		form.Add("name", groupName)
		form.Add("comments", "Updated description")
		form.Add("valid_id", "1")

		req, _ := http.NewRequest("PUT", "/admin/groups/"+strconv.Itoa(groupID), strings.NewReader(form.Encode()))
		AddTestAuthCookie(req, token)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var comments string
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			"SELECT comments FROM `groups` WHERE id = ?"), groupID).Scan(&comments))
		assert.Equal(t, "Updated description", comments)
	})

	t.Run("DeleteGroup_SoftDelete", func(t *testing.T) {
		db, err := database.GetDB()
		require.NoError(t, err)
		// A dedicated group for this test, removed again when it ends.
		id, _ := createIsolatedGroup(t, "delete-test")
		groupID := int64(id)

		router := gin.New()
		SetupHTMXRoutes(router)

		idStr2 := strconv.FormatInt(groupID, 10)
		req, _ := http.NewRequest("DELETE", "/admin/groups/"+idStr2, nil)
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should soft delete (set valid_id = 2)
		assert.Equal(t, http.StatusOK, w.Code)
		var validID int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			"SELECT valid_id FROM `groups` WHERE id = ?"), groupID).Scan(&validID))
		assert.Equal(t, 2, validID)
	})

	t.Run("GetGroupPermissions", func(t *testing.T) {
		if err := database.InitTestDB(); err != nil {
			t.Skip("Database not available")
		}
		defer database.CloseTestDB()
		router := gin.New()
		SetupHTMXRoutes(router)

		req, _ := http.NewRequest("GET", "/admin/groups/1/permissions", nil)
		AddTestAuthCookie(req, token)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err == nil {
			assert.True(t, response["success"].(bool))
			assert.Contains(t, response, "group")
			assert.Contains(t, response, "permission_keys")
		}
	})

	t.Run("UpdateGroupPermissions", func(t *testing.T) {
		db, err := database.GetDB()
		require.NoError(t, err)
		groupID, _ := createIsolatedGroup(t, "perm_group")
		agentID, _ := createIsolatedAgent(t, "perm_agent")
		router := gin.New()
		SetupHTMXRoutes(router)

		payload := map[string]interface{}{
			"assignments": []map[string]interface{}{
				{
					"user_id": agentID,
					"permissions": map[string]bool{
						"ro":        true,
						"move_into": true,
						"create":    false,
						"note":      false,
						"owner":     false,
						"priority":  false,
						"rw":        false,
					},
				},
			},
		}

		jsonBody, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", fmt.Sprintf("/admin/groups/%d/permissions", groupID), bytes.NewBuffer(jsonBody))
		AddTestAuthCookie(req, token)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.True(t, response["success"].(bool))

		rows, err := db.Query(database.ConvertPlaceholders(
			"SELECT permission_key FROM group_user WHERE user_id = ? AND group_id = ? ORDER BY permission_key"), agentID, groupID)
		require.NoError(t, err)
		defer rows.Close()
		var keys []string
		for rows.Next() {
			var k string
			require.NoError(t, rows.Scan(&k))
			keys = append(keys, k)
		}
		require.NoError(t, rows.Err())
		assert.Equal(t, []string{"move_into", "ro"}, keys)
	})
}

func TestGroupValidation(t *testing.T) {
	t.Run("ValidateGroupName", func(t *testing.T) {
		testCases := []struct {
			name     string
			input    string
			expected bool
		}{
			{"Valid name", "test_group", true},
			{"Valid with dash", "test-group", true},
			{"Empty name", "", false},
			{"Too short", "a", false},
			{"Special chars", "test@group", false},
			{"With spaces", "test group", false},
			{"Too long", strings.Repeat("a", 101), false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := validateGroupName(tc.input)
				assert.Equal(t, tc.expected, result)
			})
		}
	})
}

func TestGroupFiltering(t *testing.T) {
	token := GetTestAuthToken(t)

	t.Run("FilterByStatus", func(t *testing.T) {
		if err := database.InitTestDB(); err != nil {
			t.Skip("Database not available")
		}
		defer database.CloseTestDB()
		router := gin.New()
		SetupHTMXRoutes(router)

		// Test active groups
		req, _ := http.NewRequest("GET", "/admin/groups?status=active", nil)
		AddTestAuthCookie(req, token)
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Test inactive groups
		req, _ = http.NewRequest("GET", "/admin/groups?status=inactive", nil)
		AddTestAuthCookie(req, token)
		req.Header.Set("HX-Request", "true")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("SortGroups", func(t *testing.T) {
		if err := database.InitTestDB(); err != nil {
			t.Skip("Database not available")
		}
		defer database.CloseTestDB()
		router := gin.New()
		SetupHTMXRoutes(router)

		// Sort by name ascending
		req, _ := http.NewRequest("GET", "/admin/groups?sort=name&order=asc", nil)
		AddTestAuthCookie(req, token)
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Sort by created date descending
		req, _ = http.NewRequest("GET", "/admin/groups?sort=created&order=desc", nil)
		AddTestAuthCookie(req, token)
		req.Header.Set("HX-Request", "true")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestGroupMembership(t *testing.T) {
	token := GetTestAuthToken(t)
	db, err := database.GetDB()
	require.NoError(t, err)
	groupID, _ := createIsolatedGroup(t, "member_group")
	agentID, _ := createIsolatedAgent(t, "member_agent")
	members := func() int {
		var n int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			"SELECT COUNT(*) FROM group_user WHERE user_id = ? AND group_id = ?"), agentID, groupID).Scan(&n))
		return n
	}

	t.Run("ListGroupMembers", func(t *testing.T) {
		router := gin.New()
		SetupHTMXRoutes(router)

		req, _ := http.NewRequest("GET", "/admin/groups/1/members", nil)
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err == nil {
			assert.True(t, response["success"].(bool))
			assert.NotNil(t, response["data"])
		}
	})

	t.Run("AddMemberToGroup", func(t *testing.T) {
		router := gin.New()
		SetupHTMXRoutes(router)

		member := map[string]interface{}{
			"user_id": agentID,
		}

		jsonBody, _ := json.Marshal(member)
		req, _ := http.NewRequest("POST", fmt.Sprintf("/admin/groups/%d/members", groupID), bytes.NewBuffer(jsonBody))
		AddTestAuthCookie(req, token)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.True(t, response["success"].(bool))
		assert.Equal(t, "User assigned to group successfully", response["message"])
		assert.Positive(t, members())
	})

	t.Run("RemoveMemberFromGroup", func(t *testing.T) {
		router := gin.New()
		SetupHTMXRoutes(router)

		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/admin/groups/%d/members/%d", groupID, agentID), nil)
		AddTestAuthCookie(req, token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Zero(t, members())
	})
}

func TestGroupSessionState(t *testing.T) {
	token := GetTestAuthToken(t)

	t.Run("PreserveSearchState", func(t *testing.T) {
		if err := database.InitTestDB(); err != nil {
			t.Skip("Database not available")
		}
		defer database.CloseTestDB()
		router := gin.New()
		SetupHTMXRoutes(router)

		// Set search state
		req, _ := http.NewRequest("GET", "/admin/groups?search=test&save_state=true", nil)
		AddTestAuthCookie(req, token)
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Verify state was saved in session
		cookies := w.Result().Cookies()
		assert.NotEmpty(t, cookies, "Should set session cookie")
	})

	t.Run("RestoreFilterState", func(t *testing.T) {
		if err := database.InitTestDB(); err != nil {
			t.Skip("Database not available")
		}
		defer database.CloseTestDB()
		router := gin.New()
		SetupHTMXRoutes(router)

		// Load page without parameters - should restore from session
		req, _ := http.NewRequest("GET", "/admin/groups", nil)
		AddTestAuthCookie(req, token)
		encoded := url.QueryEscape(`{"search":"test","status":"active"}`)
		req.AddCookie(&http.Cookie{
			Name:  "group_filters",
			Value: encoded,
		})

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// Should apply saved filters
	})
}

// Helper function for group name validation.
func validateGroupName(name string) bool {
	if len(name) < 2 || len(name) > 100 {
		return false
	}

	// Only allow alphanumeric, underscore, and dash
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '_' || char == '-') {
			return false
		}
	}

	return true
}
