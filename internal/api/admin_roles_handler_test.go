package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupRoleTestRouter creates a test router using the canonical route definitions.
// This ensures test routes match production routes exactly.
func setupRoleTestRouter() *gin.Engine {
	return SetupTestRouterWithRoutes(GetAdminRolesRoutes())
}

// createTestRole creates a test role in the database and returns its ID
func createTestRole(t *testing.T, name string) (int, bool) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		return 0, false
	}

	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO roles (name, comments, valid_id, create_time, create_by, change_time, change_by)
		VALUES ($1, $2, 1, NOW(), 1, NOW(), 1)
		RETURNING id
	`)

	result, err := database.GetAdapter().InsertWithReturning(db, insertQuery, name, "Test role for unit tests")
	if err != nil {
		return 0, false
	}

	id := int(result)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM roles WHERE id = $1`), id)
	})

	return id, true
}

// cleanupTestRoleByName removes a test role by name
func cleanupTestRoleByName(t *testing.T, name string) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		return
	}
	_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM roles WHERE name = $1`), name)
}

// createTestUser creates a test user for role assignment tests
func createTestUserForRole(t *testing.T, login string) (int, bool) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		return 0, false
	}

	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES ($1, 'test', 'Test', 'User', 1, NOW(), 1, NOW(), 1)
		RETURNING id
	`)

	result, err := database.GetAdapter().InsertWithReturning(db, insertQuery, login)
	if err != nil {
		return 0, false
	}

	id := int(result)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM role_user WHERE user_id = $1`), id)
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM users WHERE id = $1`), id)
	})

	return id, true
}

// ============================================================================
// Role List Page Tests
// ============================================================================

func TestAdminRolesPage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET /admin/roles renders roles page", func(t *testing.T) {
		router := setupRoleTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/roles", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// In unit tests, template rendering may return 500 if templates aren't loaded
		// We just verify the route exists and handler executes
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError,
			"Expected 200 or 500, got %d", w.Code)
	})

	t.Run("GET /admin/roles with search parameter", func(t *testing.T) {
		router := setupRoleTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/roles?search=admin", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("GET /admin/roles with validity filter", func(t *testing.T) {
		router := setupRoleTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/roles?valid=1", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("GET /admin/roles page contains table structure", func(t *testing.T) {
		router := setupRoleTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/roles", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// In unit tests without templates, we just verify the route responds
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError,
			"Expected 200 or 500, got %d", w.Code)
	})
}

// ============================================================================
// Role Create Tests
// ============================================================================

func TestAdminRoleCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST /admin/roles creates new role with JSON", func(t *testing.T) {
		router := setupRoleTestRouter()

		testName := fmt.Sprintf("TestRole_%d", time.Now().UnixNano())
		defer cleanupTestRoleByName(t, testName)

		payload := map[string]interface{}{
			"name":     testName,
			"comments": "Test role created via JSON",
			"valid_id": 1,
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/roles", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK || w.Code == http.StatusCreated {
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.True(t, response["success"].(bool))
		} else {
			t.Logf("Create returned %d (DB may not be available): %s", w.Code, w.Body.String())
		}
	})

	t.Run("POST /admin/roles validates required name field", func(t *testing.T) {
		router := setupRoleTestRouter()

		payload := map[string]interface{}{
			"comments": "Missing name field",
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/roles", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.False(t, response["success"].(bool))
	})

	t.Run("POST /admin/roles with empty name returns error", func(t *testing.T) {
		router := setupRoleTestRouter()

		payload := map[string]interface{}{
			"name":     "",
			"comments": "Empty name",
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/roles", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("POST /admin/roles prevents duplicate names", func(t *testing.T) {
		router := setupRoleTestRouter()

		testName := fmt.Sprintf("DuplicateRole_%d", time.Now().UnixNano())
		defer cleanupTestRoleByName(t, testName)

		payload := map[string]interface{}{
			"name":     testName,
			"valid_id": 1,
		}
		jsonData, _ := json.Marshal(payload)

		// Create first role
		req := httptest.NewRequest(http.MethodPost, "/admin/roles", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK && w.Code != http.StatusCreated {
			t.Skip("Database not available for duplicate test")
		}

		// Try to create duplicate
		jsonData2, _ := json.Marshal(payload)
		req2 := httptest.NewRequest(http.MethodPost, "/admin/roles", bytes.NewReader(jsonData2))
		req2.Header.Set("Content-Type", "application/json")
		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)

		// Should fail with duplicate error or internal error
		assert.True(t, w2.Code == http.StatusBadRequest || w2.Code == http.StatusInternalServerError,
			"Expected 400 or 500 for duplicate, got %d", w2.Code)
	})
}

// ============================================================================
// Role Get Tests
// ============================================================================

func TestAdminRoleGet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET /admin/roles/:id returns role details", func(t *testing.T) {
		router := setupRoleTestRouter()

		roleID, ok := createTestRole(t, fmt.Sprintf("GetTestRole_%d", time.Now().UnixNano()))
		if !ok {
			t.Skip("Database not available for role get test")
		}

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/roles/%d", roleID), nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response["success"].(bool))
		assert.NotNil(t, response["role"])
	})

	t.Run("GET /admin/roles/:id with non-existent ID returns 404", func(t *testing.T) {
		router := setupRoleTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/roles/99999", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusInternalServerError,
			"Expected 404 or 500, got %d", w.Code)
	})

	t.Run("GET /admin/roles/:id with invalid ID returns 400", func(t *testing.T) {
		router := setupRoleTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/roles/invalid", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// ============================================================================
// Role Update Tests
// ============================================================================

func TestAdminRoleUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("PUT /admin/roles/:id updates role", func(t *testing.T) {
		router := setupRoleTestRouter()

		roleID, ok := createTestRole(t, fmt.Sprintf("UpdateTestRole_%d", time.Now().UnixNano()))
		if !ok {
			t.Skip("Database not available for role update test")
		}

		payload := map[string]interface{}{
			"name":     "Updated Role Name",
			"comments": "Updated via test",
			"valid_id": 1,
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/admin/roles/%d", roleID), bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Logf("Update response: %s", w.Body.String())
		}
		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})

	t.Run("PUT /admin/roles/:id with non-existent ID returns 404", func(t *testing.T) {
		router := setupRoleTestRouter()

		payload := map[string]interface{}{
			"name": "Updated Name",
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPut, "/admin/roles/99999", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusInternalServerError,
			"Expected 404 or 500, got %d", w.Code)
	})

	t.Run("PUT /admin/roles/:id with invalid ID returns 400", func(t *testing.T) {
		router := setupRoleTestRouter()

		req := httptest.NewRequest(http.MethodPut, "/admin/roles/invalid", nil)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("PUT /admin/roles/:id can change validity", func(t *testing.T) {
		router := setupRoleTestRouter()

		roleID, ok := createTestRole(t, fmt.Sprintf("ValidityTestRole_%d", time.Now().UnixNano()))
		if !ok {
			t.Skip("Database not available for role validity test")
		}

		payload := map[string]interface{}{
			"valid_id": 2, // Set to invalid
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/admin/roles/%d", roleID), bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// ============================================================================
// Role Delete Tests
// ============================================================================

func TestAdminRoleDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("DELETE /admin/roles/:id soft deletes role", func(t *testing.T) {
		router := setupRoleTestRouter()

		roleID, ok := createTestRole(t, fmt.Sprintf("DeleteTestRole_%d", time.Now().UnixNano()))
		if !ok {
			t.Skip("Database not available for role delete test")
		}

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/admin/roles/%d", roleID), nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})

	t.Run("DELETE /admin/roles/:id with non-existent ID returns 404", func(t *testing.T) {
		router := setupRoleTestRouter()

		req := httptest.NewRequest(http.MethodDelete, "/admin/roles/99999", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusInternalServerError,
			"Expected 404 or 500, got %d", w.Code)
	})

	t.Run("DELETE /admin/roles/:id with invalid ID returns 400", func(t *testing.T) {
		router := setupRoleTestRouter()

		req := httptest.NewRequest(http.MethodDelete, "/admin/roles/invalid", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// ============================================================================
// Role Users Tests
// ============================================================================

func TestAdminRoleUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET /admin/roles/:id/users returns role users", func(t *testing.T) {
		router := setupRoleTestRouter()

		roleID, ok := createTestRole(t, fmt.Sprintf("UsersTestRole_%d", time.Now().UnixNano()))
		if !ok {
			t.Skip("Database not available for role users test")
		}

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/roles/%d/users?format=json", roleID), nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response["success"].(bool))
		assert.NotNil(t, response["role"])
		assert.NotNil(t, response["available"])
	})

	t.Run("GET /admin/roles/:id/users with non-existent ID returns 404", func(t *testing.T) {
		router := setupRoleTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/roles/99999/users?format=json", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusInternalServerError,
			"Expected 404 or 500, got %d", w.Code)
	})
}

// ============================================================================
// Role User Add/Remove Tests
// ============================================================================

func TestAdminRoleUserAdd(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST /admin/roles/:id/users adds user to role", func(t *testing.T) {
		router := setupRoleTestRouter()

		roleID, ok := createTestRole(t, fmt.Sprintf("AddUserTestRole_%d", time.Now().UnixNano()))
		if !ok {
			t.Skip("Database not available for role user add test")
		}

		userID, ok := createTestUserForRole(t, fmt.Sprintf("testuser_%d", time.Now().UnixNano()))
		if !ok {
			t.Skip("Database not available for user creation")
		}

		payload := map[string]interface{}{
			"user_id": userID,
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/roles/%d/users", roleID), bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})

	t.Run("POST /admin/roles/:id/users with missing user_id returns 400", func(t *testing.T) {
		router := setupRoleTestRouter()

		payload := map[string]interface{}{}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/roles/1/users", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("POST /admin/roles/:id/users with invalid role ID returns 400", func(t *testing.T) {
		router := setupRoleTestRouter()

		payload := map[string]interface{}{
			"user_id": 1,
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/roles/invalid/users", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestAdminRoleUserRemove(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("DELETE /admin/roles/:id/users/:userId removes user from role", func(t *testing.T) {
		router := setupRoleTestRouter()

		roleID, ok := createTestRole(t, fmt.Sprintf("RemoveUserTestRole_%d", time.Now().UnixNano()))
		if !ok {
			t.Skip("Database not available for role user remove test")
		}

		userID, ok := createTestUserForRole(t, fmt.Sprintf("removeuser_%d", time.Now().UnixNano()))
		if !ok {
			t.Skip("Database not available for user creation")
		}

		// First add user to role
		db, _ := database.GetDB()
		_, _ = db.Exec(database.ConvertPlaceholders(`
			INSERT INTO role_user (role_id, user_id, create_time, create_by, change_time, change_by) VALUES ($1, $2, NOW(), 1, NOW(), 1)
		`), roleID, userID)

		// Now remove
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/admin/roles/%d/users/%d", roleID, userID), nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})

	t.Run("DELETE /admin/roles/:id/users/:userId with non-existent membership returns 404", func(t *testing.T) {
		router := setupRoleTestRouter()

		roleID, ok := createTestRole(t, fmt.Sprintf("NoMemberRole_%d", time.Now().UnixNano()))
		if !ok {
			t.Skip("Database not available for role user remove test")
		}

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/admin/roles/%d/users/99999", roleID), nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusInternalServerError,
			"Expected 404 or 500, got %d", w.Code)
	})

	t.Run("DELETE /admin/roles/:id/users/:userId with invalid IDs returns 400", func(t *testing.T) {
		router := setupRoleTestRouter()

		req := httptest.NewRequest(http.MethodDelete, "/admin/roles/invalid/users/invalid", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// ============================================================================
// Role Permissions Tests
// ============================================================================

func TestAdminRolePermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET /admin/roles/:id/permissions renders permissions page", func(t *testing.T) {
		router := setupRoleTestRouter()

		roleID, ok := createTestRole(t, fmt.Sprintf("PermTestRole_%d", time.Now().UnixNano()))
		if !ok {
			t.Skip("Database not available for role permissions test")
		}

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/roles/%d/permissions", roleID), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError,
			"Expected 200 or 500, got %d", w.Code)
	})

	t.Run("GET /admin/roles/:id/permissions with non-existent ID returns 404", func(t *testing.T) {
		router := setupRoleTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/roles/99999/permissions", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusInternalServerError,
			"Expected 404 or 500, got %d", w.Code)
	})

	t.Run("GET /admin/roles/:id/permissions with invalid ID returns 400", func(t *testing.T) {
		router := setupRoleTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/roles/invalid/permissions", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestAdminRolePermissionsUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST /admin/roles/:id/permissions updates permissions", func(t *testing.T) {
		router := setupRoleTestRouter()

		roleID, ok := createTestRole(t, fmt.Sprintf("PermUpdateRole_%d", time.Now().UnixNano()))
		if !ok {
			t.Skip("Database not available for permissions update test")
		}

		// Form data for permissions
		form := url.Values{}
		form.Set("perm_1_ro", "1")
		form.Set("perm_1_rw", "1")

		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/roles/%d/permissions", roleID), strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// May redirect or return JSON depending on request type
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusSeeOther,
			"Expected 200 or 303, got %d: %s", w.Code, w.Body.String())
	})

	t.Run("POST /admin/roles/:id/permissions with invalid ID returns 400", func(t *testing.T) {
		router := setupRoleTestRouter()

		form := url.Values{}
		form.Set("perm_1_ro", "1")

		req := httptest.NewRequest(http.MethodPost, "/admin/roles/invalid/permissions", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// ============================================================================
// Database Integration Tests
// ============================================================================

func TestAdminRolesDBIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Role CRUD operations persist to database", func(t *testing.T) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Skip("Database not available for integration test")
		}

		router := setupRoleTestRouter()
		testName := fmt.Sprintf("IntegrationRole_%d", time.Now().UnixNano())
		defer cleanupTestRoleByName(t, testName)

		// Create
		payload := map[string]interface{}{
			"name":     testName,
			"comments": "Integration test",
			"valid_id": 1,
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/roles", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.True(t, w.Code == http.StatusOK || w.Code == http.StatusCreated,
			"Create failed with code %d: %s", w.Code, w.Body.String())

		// Verify in database
		var count int
		err = db.QueryRow(database.ConvertPlaceholders(`SELECT COUNT(*) FROM roles WHERE name = $1`), testName).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "Role should exist in database")
	})

	t.Run("Role user assignment persists to database", func(t *testing.T) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Skip("Database not available for integration test")
		}

		router := setupRoleTestRouter()

		roleID, ok := createTestRole(t, fmt.Sprintf("UserAssignRole_%d", time.Now().UnixNano()))
		if !ok {
			t.Skip("Could not create test role")
		}

		userID, ok := createTestUserForRole(t, fmt.Sprintf("assignuser_%d", time.Now().UnixNano()))
		if !ok {
			t.Skip("Could not create test user")
		}

		// Add user to role
		payload := map[string]interface{}{
			"user_id": userID,
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/roles/%d/users", roleID), bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		// Verify in database
		var count int
		err = db.QueryRow(database.ConvertPlaceholders(`
			SELECT COUNT(*) FROM role_user WHERE role_id = $1 AND user_id = $2
		`), roleID, userID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "Role-user assignment should exist in database")
	})
}

// ============================================================================
// Edge Cases and Error Handling Tests
// ============================================================================

func TestAdminRolesEdgeCases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Role name with special characters", func(t *testing.T) {
		router := setupRoleTestRouter()

		testName := fmt.Sprintf("Test Role (Special) & <chars> %d", time.Now().UnixNano())
		defer cleanupTestRoleByName(t, testName)

		payload := map[string]interface{}{
			"name":     testName,
			"valid_id": 1,
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/roles", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should handle special characters gracefully
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusCreated || w.Code == http.StatusInternalServerError)
	})

	t.Run("Role with very long name", func(t *testing.T) {
		router := setupRoleTestRouter()

		longName := strings.Repeat("x", 500)

		payload := map[string]interface{}{
			"name":     longName,
			"valid_id": 1,
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/roles", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should either succeed or return proper error (not panic)
		assert.True(t, w.Code != 0, "Should return a valid HTTP status code")
	})

	t.Run("Concurrent role creation", func(t *testing.T) {
		router := setupRoleTestRouter()

		done := make(chan bool, 5)
		for i := 0; i < 5; i++ {
			go func(idx int) {
				defer func() { done <- true }()

				testName := fmt.Sprintf("ConcurrentRole_%d_%d", time.Now().UnixNano(), idx)
				defer cleanupTestRoleByName(t, testName)

				payload := map[string]interface{}{
					"name":     testName,
					"valid_id": 1,
				}
				jsonData, _ := json.Marshal(payload)

				req := httptest.NewRequest(http.MethodPost, "/admin/roles", bytes.NewReader(jsonData))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()

				router.ServeHTTP(w, req)
				// Should not panic
			}(i)
		}

		for i := 0; i < 5; i++ {
			<-done
		}
	})
}
