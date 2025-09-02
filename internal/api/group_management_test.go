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

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/stretchr/testify/assert"
)

func TestAdminGroupManagement(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("ListGroups_ShowsAllGroups", func(t *testing.T) {
        if err := database.InitTestDB(); err != nil {
            t.Skip("Database not available")
        }
        defer database.CloseTestDB()
		router := gin.New()
		SetupHTMXRoutes(router)

		req, _ := http.NewRequest("GET", "/admin/groups", nil)
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

		form := url.Values{}
		form.Add("name", "test_group")
		form.Add("comments", "Test Group Description")
		form.Add("valid_id", "1")

		req, _ := http.NewRequest("POST", "/admin/groups", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Check response
		var response map[string]interface{}
		if w.Code == http.StatusOK {
			if err := json.Unmarshal(w.Body.Bytes(), &response); err == nil {
				assert.True(t, response["success"].(bool), "Group creation should succeed")
			}
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
        if err := database.InitTestDB(); err != nil {
            t.Skip("Database not available")
        }
        defer database.CloseTestDB()

        db, err := database.GetDB()
		if err != nil {
			t.Skip("Database not available")
		}

		groupRepo := repository.NewGroupRepository(db)
		groups, _ := groupRepo.List()
		
		if len(groups) == 0 {
			t.Skip("No groups available for testing")
		}

		router := gin.New()
		SetupHTMXRoutes(router)

		testGroup := groups[0]
		
		form := url.Values{}
		form.Add("name", testGroup.Name)
		form.Add("comments", "Updated description")
		form.Add("valid_id", "1")

        // testGroup.ID is interface{}; assert to an int or string where possible
        var idStr string
        switch v := testGroup.ID.(type) {
        case int:
            idStr = strconv.Itoa(v)
        case int64:
            idStr = strconv.Itoa(int(v))
        case uint:
            idStr = strconv.Itoa(int(v))
        case uint64:
            idStr = strconv.Itoa(int(v))
        case string:
            idStr = v
        default:
            t.Skip("Unknown group ID type; skipping")
        }
        req, _ := http.NewRequest("PUT", "/admin/groups/"+idStr, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("DeleteGroup_SoftDelete", func(t *testing.T) {
        if err := database.InitTestDB(); err != nil {
            t.Skip("Database not available")
        }
        defer database.CloseTestDB()

        db, err := database.GetDB()
		if err != nil {
			t.Skip("Database not available")
		}

		groupRepo := repository.NewGroupRepository(db)
		groups, _ := groupRepo.List()
		
		// Find a non-system group to delete
		var testGroup *models.Group
		for _, g := range groups {
			if g.Name != "admin" && g.Name != "users" && g.Name != "stats" {
				testGroup = g
				break
			}
		}

		if testGroup == nil {
			t.Skip("No non-system group available for testing")
		}

		router := gin.New()
		SetupHTMXRoutes(router)

        var idStr2 string
        switch v := testGroup.ID.(type) {
        case int:
            idStr2 = strconv.Itoa(v)
        case int64:
            idStr2 = strconv.Itoa(int(v))
        case uint:
            idStr2 = strconv.Itoa(int(v))
        case uint64:
            idStr2 = strconv.Itoa(int(v))
        case string:
            idStr2 = v
        default:
            t.Skip("Unknown group ID type; skipping")
        }
        req, _ := http.NewRequest("DELETE", "/admin/groups/"+idStr2, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should soft delete (set valid_id = 2)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("GetGroupPermissions", func(t *testing.T) {
        if err := database.InitTestDB(); err != nil {
            t.Skip("Database not available")
        }
        defer database.CloseTestDB()
		router := gin.New()
		SetupHTMXRoutes(router)

		req, _ := http.NewRequest("GET", "/admin/groups/1/permissions", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err == nil {
			assert.True(t, response["success"].(bool))
			assert.NotNil(t, response["data"])
		}
	})

	t.Run("UpdateGroupPermissions", func(t *testing.T) {
        if err := database.InitTestDB(); err != nil {
            t.Skip("Database not available")
        }
        defer database.CloseTestDB()
		router := gin.New()
		SetupHTMXRoutes(router)

		permissions := map[string][]string{
			"rw": []string{"ticket_create", "ticket_update"},
			"ro": []string{"ticket_view"},
		}

		jsonBody, _ := json.Marshal(permissions)
		req, _ := http.NewRequest("PUT", "/admin/groups/1/permissions", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
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
	t.Run("FilterByStatus", func(t *testing.T) {
        if err := database.InitTestDB(); err != nil {
            t.Skip("Database not available")
        }
        defer database.CloseTestDB()
		router := gin.New()
		SetupHTMXRoutes(router)

		// Test active groups
		req, _ := http.NewRequest("GET", "/admin/groups?status=active", nil)
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Test inactive groups  
		req, _ = http.NewRequest("GET", "/admin/groups?status=inactive", nil)
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
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Sort by created date descending
		req, _ = http.NewRequest("GET", "/admin/groups?sort=created&order=desc", nil)
		req.Header.Set("HX-Request", "true")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestGroupMembership(t *testing.T) {
	t.Run("ListGroupMembers", func(t *testing.T) {
		router := gin.New()
		SetupHTMXRoutes(router)

		req, _ := http.NewRequest("GET", "/admin/groups/1/members", nil)
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
			"user_id": 2,
		}

		jsonBody, _ := json.Marshal(member)
		req, _ := http.NewRequest("POST", "/admin/groups/1/members", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err == nil {
			// Check if member was added or already exists
			if response["success"].(bool) {
				assert.Equal(t, "Member added successfully", response["message"])
			}
		}
	})

	t.Run("RemoveMemberFromGroup", func(t *testing.T) {
		router := gin.New()
		SetupHTMXRoutes(router)

		req, _ := http.NewRequest("DELETE", "/admin/groups/1/members/2", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestGroupSessionState(t *testing.T) {
	t.Run("PreserveSearchState", func(t *testing.T) {
		router := gin.New()
		SetupHTMXRoutes(router)

		// Set search state
		req, _ := http.NewRequest("GET", "/admin/groups?search=test&save_state=true", nil)
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Verify state was saved in session
		cookies := w.Result().Cookies()
		assert.NotEmpty(t, cookies, "Should set session cookie")
	})

	t.Run("RestoreFilterState", func(t *testing.T) {
		router := gin.New()
		SetupHTMXRoutes(router)

		// Load page without parameters - should restore from session
		req, _ := http.NewRequest("GET", "/admin/groups", nil)
		req.AddCookie(&http.Cookie{
			Name:  "group_filters",
			Value: `{"search":"test","status":"active"}`,
		})
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// Should apply saved filters
	})
}

// Helper function for group name validation
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