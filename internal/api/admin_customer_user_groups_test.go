package api

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/database"
)

// TestCustomerUserGroupsListPage tests the customer user groups overview page
func TestCustomerUserGroupsListPage(t *testing.T) {
	db := getTestDB(t)

	// Setup test data
	userLogin := "test-cug-user-" + time.Now().Format("20060102150405")
	createTestCustomerUser(t, db, userLogin)
	defer cleanupTestCustomerUser(t, db, userLogin)

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin/customer-user-groups", handleAdminCustomerUserGroups)

	t.Run("should return 200 for customer user groups page", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/admin/customer-user-groups", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("should return customerUsers and groups in JSON response", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/admin/customer-user-groups", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "customerUsers")
		assert.Contains(t, w.Body.String(), "groups")
	})
}

// TestCustomerUserGroupPermissionsCRUD tests full CRUD for customer user group permissions
func TestCustomerUserGroupPermissionsCRUD(t *testing.T) {
	db := getTestDB(t)

	// Setup test data
	userLogin := "test-cug-perm-" + time.Now().Format("20060102150405")
	createTestCustomerUser(t, db, userLogin)
	defer cleanupTestCustomerUser(t, db, userLogin)
	defer cleanupCustomerUserGroupPermissions(t, db, userLogin)

	// Get a valid group ID
	var groupID int
	err := db.QueryRow(database.ConvertPlaceholders("SELECT id FROM groups WHERE valid_id = 1 LIMIT 1")).Scan(&groupID)
	require.NoError(t, err, "Need at least one valid group for test")

	t.Run("should load empty permissions for customer user", func(t *testing.T) {
		permissions, err := loadCustomerUserGroupPermissions(db, userLogin)
		require.NoError(t, err)
		assert.Empty(t, permissions)
	})

	t.Run("should add permissions for customer user", func(t *testing.T) {
		// Add ro and rw permissions
		newPerms := map[int]map[string]bool{
			groupID: {
				"ro": true,
				"rw": true,
			},
		}
		err := updateCustomerUserGroupPermissions(db, userLogin, newPerms, 1)
		require.NoError(t, err)

		// Verify
		permissions, err := loadCustomerUserGroupPermissions(db, userLogin)
		require.NoError(t, err)
		assert.True(t, permissions[groupID]["ro"], "ro permission should be set")
		assert.True(t, permissions[groupID]["rw"], "rw permission should be set")
	})

	t.Run("should update permissions for customer user", func(t *testing.T) {
		// Change to only ro
		newPerms := map[int]map[string]bool{
			groupID: {
				"ro": true,
				"rw": false,
			},
		}
		err := updateCustomerUserGroupPermissions(db, userLogin, newPerms, 1)
		require.NoError(t, err)

		// Verify
		permissions, err := loadCustomerUserGroupPermissions(db, userLogin)
		require.NoError(t, err)
		assert.True(t, permissions[groupID]["ro"], "ro permission should still be set")
		assert.False(t, permissions[groupID]["rw"], "rw permission should be removed")
	})

	t.Run("should remove all permissions", func(t *testing.T) {
		// Remove all permissions
		newPerms := map[int]map[string]bool{}
		err := updateCustomerUserGroupPermissions(db, userLogin, newPerms, 1)
		require.NoError(t, err)

		// Verify
		permissions, err := loadCustomerUserGroupPermissions(db, userLogin)
		require.NoError(t, err)
		assert.Empty(t, permissions)
	})
}

// TestGroupCustomerUserPermissionsCRUD tests CRUD from group perspective
func TestGroupCustomerUserPermissionsCRUD(t *testing.T) {
	db := getTestDB(t)

	// Setup test data
	userLogin := "test-gcug-perm-" + time.Now().Format("20060102150405")
	createTestCustomerUser(t, db, userLogin)
	defer cleanupTestCustomerUser(t, db, userLogin)
	defer cleanupCustomerUserGroupPermissions(t, db, userLogin)

	// Get a valid group ID
	var groupID int
	err := db.QueryRow(database.ConvertPlaceholders("SELECT id FROM groups WHERE valid_id = 1 LIMIT 1")).Scan(&groupID)
	require.NoError(t, err, "Need at least one valid group for test")

	t.Run("should load permissions for group", func(t *testing.T) {
		permissions, err := loadGroupCustomerUserPermissions(db, groupID)
		require.NoError(t, err)
		// May have other users, just check it doesn't error
		assert.NotNil(t, permissions)
	})

	t.Run("should add customer user to group", func(t *testing.T) {
		newPerms := map[string]map[string]bool{
			userLogin: {
				"ro": true,
				"rw": false,
			},
		}
		err := updateGroupCustomerUserPermissions(db, groupID, newPerms, 1)
		require.NoError(t, err)

		// Verify from customer user perspective
		permissions, err := loadCustomerUserGroupPermissions(db, userLogin)
		require.NoError(t, err)
		assert.True(t, permissions[groupID]["ro"])
	})
}

// TestCustomerUserGroupEditHandler tests the edit page handler
func TestCustomerUserGroupEditHandler(t *testing.T) {
	db := getTestDB(t)

	// Setup test data
	userLogin := "test-cug-edit-" + time.Now().Format("20060102150405")
	createTestCustomerUser(t, db, userLogin)
	defer cleanupTestCustomerUser(t, db, userLogin)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin/customer-user-groups/user/:id", handleAdminCustomerUserGroupEdit)

	t.Run("should return customer user details and groups", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/admin/customer-user-groups/user/"+userLogin, nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "customerUser")
		assert.Contains(t, w.Body.String(), "groups")
		assert.Contains(t, w.Body.String(), "permissions")
	})

	t.Run("should return 404 for non-existent customer user", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/admin/customer-user-groups/user/nonexistent-user-id", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// TestCustomerUserGroupUpdateHandler tests the update endpoint
func TestCustomerUserGroupUpdateHandler(t *testing.T) {
	db := getTestDB(t)

	// Setup test data
	userLogin := "test-cug-update-" + time.Now().Format("20060102150405")
	createTestCustomerUser(t, db, userLogin)
	defer cleanupTestCustomerUser(t, db, userLogin)
	defer cleanupCustomerUserGroupPermissions(t, db, userLogin)

	// Get a valid group ID
	var groupID int
	err := db.QueryRow(database.ConvertPlaceholders("SELECT id FROM groups WHERE valid_id = 1 LIMIT 1")).Scan(&groupID)
	require.NoError(t, err, "Need at least one valid group for test")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/admin/customer-user-groups/user/:id", handleAdminCustomerUserGroupUpdate)

	t.Run("should update permissions via form POST", func(t *testing.T) {
		form := url.Values{}
		form.Set("permissions["+intToStr(groupID)+"][ro]", "1")
		form.Set("permissions["+intToStr(groupID)+"][rw]", "1")

		req, _ := http.NewRequest("POST", "/admin/customer-user-groups/user/"+userLogin, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "success")

		// Verify in database
		permissions, err := loadCustomerUserGroupPermissions(db, userLogin)
		require.NoError(t, err)
		assert.True(t, permissions[groupID]["ro"])
		assert.True(t, permissions[groupID]["rw"])
	})
}

// TestLoadCustomerUserHelperFunctions tests the database helper functions
func TestLoadCustomerUserHelperFunctions(t *testing.T) {
	db := getTestDB(t)

	t.Run("loadCustomerUsers should return valid customer users", func(t *testing.T) {
		users, err := loadCustomerUsers(db, "")
		require.NoError(t, err)
		// Should return at least an empty slice, not nil
		assert.NotNil(t, users)
	})

	t.Run("loadCustomerUsers with search should filter", func(t *testing.T) {
		// Create a test customer user
		userLogin := "cug-searchtest-" + time.Now().Format("20060102150405")
		createTestCustomerUser(t, db, userLogin)
		defer cleanupTestCustomerUser(t, db, userLogin)

		users, err := loadCustomerUsers(db, "cug-searchtest")
		require.NoError(t, err)
		found := false
		for _, u := range users {
			if u.Login == userLogin {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find the test customer user")
	})

	t.Run("loadCustomerUser should return specific user", func(t *testing.T) {
		// Create a test customer user
		userLogin := "cug-specific-" + time.Now().Format("20060102150405")
		createTestCustomerUser(t, db, userLogin)
		defer cleanupTestCustomerUser(t, db, userLogin)

		user, err := loadCustomerUser(db, userLogin)
		require.NoError(t, err)
		assert.Equal(t, userLogin, user.Login)
	})

	t.Run("loadCustomerUser should return error for invalid login", func(t *testing.T) {
		_, err := loadCustomerUser(db, "nonexistent-user-999999")
		assert.Error(t, err)
	})
}

// TestCustomerUserGroupByGroupHandler tests the group-centric view
func TestCustomerUserGroupByGroupHandler(t *testing.T) {
	db := getTestDB(t)

	// Get a valid group ID
	var groupID int
	err := db.QueryRow(database.ConvertPlaceholders("SELECT id FROM groups WHERE valid_id = 1 LIMIT 1")).Scan(&groupID)
	require.NoError(t, err, "Need at least one valid group for test")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin/customer-user-groups/group/:id", handleAdminCustomerUserGroupByGroup)

	t.Run("should return group details and customer users", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/admin/customer-user-groups/group/"+intToStr(groupID), nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "group")
		assert.Contains(t, w.Body.String(), "customerUsers")
		assert.Contains(t, w.Body.String(), "permissions")
	})

	t.Run("should return 404 for non-existent group", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/admin/customer-user-groups/group/999999", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// TestCustomerUserGroupGetPermissionsAPI tests the permissions API endpoint
func TestCustomerUserGroupGetPermissionsAPI(t *testing.T) {
	db := getTestDB(t)

	// Setup test data
	userLogin := "test-cug-api-" + time.Now().Format("20060102150405")
	createTestCustomerUser(t, db, userLogin)
	defer cleanupTestCustomerUser(t, db, userLogin)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin/api/customer-user-groups/permissions", handleGetCustomerUserGroupPermissions)

	t.Run("should return permissions for user_login", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/admin/api/customer-user-groups/permissions?user_login="+userLogin, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "success")
		assert.Contains(t, w.Body.String(), "permissions")
	})

	t.Run("should return permissions for group_id", func(t *testing.T) {
		var groupID int
		err := db.QueryRow(database.ConvertPlaceholders("SELECT id FROM groups WHERE valid_id = 1 LIMIT 1")).Scan(&groupID)
		require.NoError(t, err)

		req, _ := http.NewRequest("GET", "/admin/api/customer-user-groups/permissions?group_id="+intToStr(groupID), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "success")
	})

	t.Run("should return error when no params provided", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/admin/api/customer-user-groups/permissions", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// Helper to create a test customer user
func createTestCustomerUser(t *testing.T, db *sql.DB, login string) {
	query := database.ConvertPlaceholders(`
		INSERT INTO customer_user (login, email, first_name, last_name, customer_id, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, ?, ?, 1, NOW(), 1, NOW(), 1)
	`)
	_, err := db.Exec(query, login, login+"@test.com", "Test", "User", "test-company")
	if err != nil {
		t.Fatalf("Failed to create test customer user: %v", err)
	}
}

// Helper to clean up test customer user
func cleanupTestCustomerUser(t *testing.T, db *sql.DB, login string) {
	_, err := db.Exec(database.ConvertPlaceholders("DELETE FROM customer_user WHERE login = ?"), login)
	if err != nil {
		t.Logf("Warning: failed to cleanup customer_user %s: %v", login, err)
	}
}

// Helper to clean up customer user group permissions
func cleanupCustomerUserGroupPermissions(t *testing.T, db *sql.DB, userLogin string) {
	_, err := db.Exec(database.ConvertPlaceholders("DELETE FROM group_customer_user WHERE user_id = ?"), userLogin)
	if err != nil {
		t.Logf("Warning: failed to cleanup group_customer_user for %s: %v", userLogin, err)
	}
}
