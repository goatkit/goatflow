package api

import (
	"database/sql"
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

	"github.com/goatkit/goatflow/internal/database"
)

// TestCustomerGroupsListPage tests the customer groups overview page
func TestCustomerGroupsListPage(t *testing.T) {
	db := getTestDB(t)

	// Setup test data
	customerID := "test-cg-customer-" + time.Now().Format("20060102150405")
	createTestCustomerCompany(t, db, customerID)
	defer cleanupTestCustomerCompany(t, db, customerID)

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin/customer-groups", handleAdminCustomerGroups)

	t.Run("should return 200 for customer groups page", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/admin/customer-groups", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("should return customers and groups in JSON response", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/admin/customer-groups", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "customers")
		assert.Contains(t, w.Body.String(), "groups")
	})
}

// TestCustomerGroupPermissionsCRUD tests full CRUD for customer group permissions
func TestCustomerGroupPermissionsCRUD(t *testing.T) {
	db := getTestDB(t)

	// Setup test data
	customerID := "test-cg-perm-" + time.Now().Format("20060102150405")
	createTestCustomerCompany(t, db, customerID)
	defer cleanupTestCustomerCompany(t, db, customerID)
	defer cleanupCustomerGroupPermissions(t, db, customerID)

	// Get a valid group ID
	var groupID int
	err := db.QueryRow(database.ConvertPlaceholders("SELECT id FROM groups WHERE valid_id = 1 LIMIT 1")).Scan(&groupID)
	require.NoError(t, err, "Need at least one valid group for test")

	t.Run("should load empty permissions for customer", func(t *testing.T) {
		permissions, err := loadCustomerGroupPermissions(db, customerID)
		require.NoError(t, err)
		assert.Empty(t, permissions)
	})

	t.Run("should add permissions for customer", func(t *testing.T) {
		// Add ro and rw permissions
		newPerms := map[int]map[string]bool{
			groupID: {
				"ro": true,
				"rw": true,
			},
		}
		err := updateCustomerGroupPermissions(db, customerID, newPerms, 1)
		require.NoError(t, err)

		// Verify
		permissions, err := loadCustomerGroupPermissions(db, customerID)
		require.NoError(t, err)
		assert.True(t, permissions[groupID]["ro"], "ro permission should be set")
		assert.True(t, permissions[groupID]["rw"], "rw permission should be set")
	})

	t.Run("should update permissions for customer", func(t *testing.T) {
		// Change to only ro
		newPerms := map[int]map[string]bool{
			groupID: {
				"ro": true,
				"rw": false,
			},
		}
		err := updateCustomerGroupPermissions(db, customerID, newPerms, 1)
		require.NoError(t, err)

		// Verify
		permissions, err := loadCustomerGroupPermissions(db, customerID)
		require.NoError(t, err)
		assert.True(t, permissions[groupID]["ro"], "ro permission should still be set")
		assert.False(t, permissions[groupID]["rw"], "rw permission should be removed")
	})

	t.Run("should remove all permissions", func(t *testing.T) {
		// Remove all permissions
		newPerms := map[int]map[string]bool{}
		err := updateCustomerGroupPermissions(db, customerID, newPerms, 1)
		require.NoError(t, err)

		// Verify
		permissions, err := loadCustomerGroupPermissions(db, customerID)
		require.NoError(t, err)
		assert.Empty(t, permissions)
	})
}

// TestGroupCustomerPermissionsCRUD tests CRUD from group perspective
func TestGroupCustomerPermissionsCRUD(t *testing.T) {
	db := getTestDB(t)

	// Setup test data
	customerID := "test-gcg-perm-" + time.Now().Format("20060102150405")
	createTestCustomerCompany(t, db, customerID)
	defer cleanupTestCustomerCompany(t, db, customerID)
	defer cleanupCustomerGroupPermissions(t, db, customerID)

	// Get a valid group ID
	var groupID int
	err := db.QueryRow(database.ConvertPlaceholders("SELECT id FROM groups WHERE valid_id = 1 LIMIT 1")).Scan(&groupID)
	require.NoError(t, err, "Need at least one valid group for test")

	t.Run("should load empty permissions for group", func(t *testing.T) {
		permissions, err := loadGroupCustomerPermissions(db, groupID)
		require.NoError(t, err)
		// May have other customers, just check it doesn't error
		assert.NotNil(t, permissions)
	})

	t.Run("should add customer to group", func(t *testing.T) {
		newPerms := map[string]map[string]bool{
			customerID: {
				"ro": true,
				"rw": false,
			},
		}
		err := updateGroupCustomerPermissions(db, groupID, newPerms, 1)
		require.NoError(t, err)

		// Verify from customer perspective
		permissions, err := loadCustomerGroupPermissions(db, customerID)
		require.NoError(t, err)
		assert.True(t, permissions[groupID]["ro"])
	})
}

// TestCustomerGroupEditHandler tests the edit page handler
func TestCustomerGroupEditHandler(t *testing.T) {
	db := getTestDB(t)

	// Setup test data
	customerID := "test-cg-edit-" + time.Now().Format("20060102150405")
	createTestCustomerCompany(t, db, customerID)
	defer cleanupTestCustomerCompany(t, db, customerID)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin/customer-groups/customer/:id", handleAdminCustomerGroupEdit)

	t.Run("should return customer details and groups", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/admin/customer-groups/customer/"+customerID, nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "customer")
		assert.Contains(t, w.Body.String(), "groups")
		assert.Contains(t, w.Body.String(), "permissions")
	})

	t.Run("should return 404 for non-existent customer", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/admin/customer-groups/customer/nonexistent-customer-id", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// TestCustomerGroupUpdateHandler tests the update endpoint
func TestCustomerGroupUpdateHandler(t *testing.T) {
	db := getTestDB(t)

	// Setup test data
	customerID := "test-cg-update-" + time.Now().Format("20060102150405")
	createTestCustomerCompany(t, db, customerID)
	defer cleanupTestCustomerCompany(t, db, customerID)
	defer cleanupCustomerGroupPermissions(t, db, customerID)

	// Get a valid group ID
	var groupID int
	err := db.QueryRow(database.ConvertPlaceholders("SELECT id FROM groups WHERE valid_id = 1 LIMIT 1")).Scan(&groupID)
	require.NoError(t, err, "Need at least one valid group for test")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/admin/customer-groups/customer/:id", handleAdminCustomerGroupUpdate)

	t.Run("should update permissions via form POST", func(t *testing.T) {
		form := url.Values{}
		form.Set("permissions["+intToStr(groupID)+"][ro]", "1")
		form.Set("permissions["+intToStr(groupID)+"][rw]", "1")

		req, _ := http.NewRequest("POST", "/admin/customer-groups/customer/"+customerID, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "success")

		// Verify in database
		permissions, err := loadCustomerGroupPermissions(db, customerID)
		require.NoError(t, err)
		assert.True(t, permissions[groupID]["ro"])
		assert.True(t, permissions[groupID]["rw"])
	})
}

// TestLoadHelperFunctions tests the database helper functions
func TestLoadHelperFunctions(t *testing.T) {
	db := getTestDB(t)

	t.Run("loadCustomerCompanies should return valid customers", func(t *testing.T) {
		customers, err := loadCustomerCompanies(db, "")
		require.NoError(t, err)
		// Should return at least an empty slice, not nil
		assert.NotNil(t, customers)
	})

	t.Run("loadCustomerCompanies with search should filter", func(t *testing.T) {
		// Create a test customer
		customerID := "searchtest-" + time.Now().Format("20060102150405")
		createTestCustomerCompany(t, db, customerID)
		defer cleanupTestCustomerCompany(t, db, customerID)

		customers, err := loadCustomerCompanies(db, "searchtest")
		require.NoError(t, err)
		found := false
		for _, c := range customers {
			if c.CustomerID == customerID {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find the test customer")
	})

	t.Run("loadGroups should return valid groups", func(t *testing.T) {
		groups, err := loadGroups(db)
		require.NoError(t, err)
		assert.NotNil(t, groups)
		// Should have at least admin group
		assert.True(t, len(groups) > 0, "Should have at least one group")
	})

	t.Run("loadGroup should return specific group", func(t *testing.T) {
		// Get first group ID
		var groupID int
		err := db.QueryRow(database.ConvertPlaceholders("SELECT id FROM groups WHERE valid_id = 1 LIMIT 1")).Scan(&groupID)
		require.NoError(t, err)

		group, err := loadGroup(db, groupID)
		require.NoError(t, err)
		assert.Equal(t, groupID, group.ID)
		assert.NotEmpty(t, group.Name)
	})

	t.Run("loadGroup should return error for invalid ID", func(t *testing.T) {
		_, err := loadGroup(db, 999999)
		assert.Error(t, err)
	})
}

// TestCustomerGroupByGroupHandler tests the group-centric view
func TestCustomerGroupByGroupHandler(t *testing.T) {
	db := getTestDB(t)

	// Get a valid group ID
	var groupID int
	err := db.QueryRow(database.ConvertPlaceholders("SELECT id FROM groups WHERE valid_id = 1 LIMIT 1")).Scan(&groupID)
	require.NoError(t, err, "Need at least one valid group for test")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin/customer-groups/group/:id", handleAdminCustomerGroupByGroup)

	t.Run("should return group details and customers", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/admin/customer-groups/group/"+intToStr(groupID), nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "group")
		assert.Contains(t, w.Body.String(), "customers")
		assert.Contains(t, w.Body.String(), "permissions")
	})

	t.Run("should return 404 for non-existent group", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/admin/customer-groups/group/999999", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// TestPermissionContextField tests that permission_context is set correctly
func TestPermissionContextField(t *testing.T) {
	db := getTestDB(t)

	customerID := "test-context-" + time.Now().Format("20060102150405")
	createTestCustomerCompany(t, db, customerID)
	defer cleanupTestCustomerCompany(t, db, customerID)
	defer cleanupCustomerGroupPermissions(t, db, customerID)

	var groupID int
	err := db.QueryRow(database.ConvertPlaceholders("SELECT id FROM groups WHERE valid_id = 1 LIMIT 1")).Scan(&groupID)
	require.NoError(t, err)

	// Add permission
	newPerms := map[int]map[string]bool{
		groupID: {"ro": true},
	}
	err = updateCustomerGroupPermissions(db, customerID, newPerms, 1)
	require.NoError(t, err)

	// Verify permission_context is set
	var context string
	err = db.QueryRow(database.ConvertPlaceholders(`
		SELECT permission_context FROM group_customer
		WHERE customer_id = ? AND group_id = ?
	`), customerID, groupID).Scan(&context)
	require.NoError(t, err)
	assert.Equal(t, "customer", context, "permission_context should be 'customer'")
}

// Helper to clean up customer group permissions
func cleanupCustomerGroupPermissions(t *testing.T, db *sql.DB, customerID string) {
	_, err := db.Exec(database.ConvertPlaceholders("DELETE FROM group_customer WHERE customer_id = ?"), customerID)
	if err != nil {
		t.Logf("Warning: failed to cleanup group_customer for %s: %v", customerID, err)
	}
}

// Helper to convert int to string for customer groups tests
func intToStr(i int) string {
	return strconv.Itoa(i)
}
