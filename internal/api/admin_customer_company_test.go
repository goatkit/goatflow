package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// Uses database.InitTestDB() for consistency with other admin tests.
func getTestDB(t *testing.T) *sql.DB {
	if err := database.InitTestDB(); err != nil {
		t.Fatalf("Test database not available: %v. Run: make test-db-up", err)
	}

	db, err := database.GetDB()
	require.NoError(t, err, "Failed to get test database connection")
	require.NotNil(t, db, "Database connection is nil")

	return db
}

// Helper function to create a test customer company.
func createTestCustomerCompany(t *testing.T, db *sql.DB, customerID string) {
	name := "Test Company " + customerID
	street := "123 Test St"
	city := "Test City"
	country := "Test Country"

	if database.IsMySQL() {
		_, err := db.Exec(`
			INSERT INTO customer_company (customer_id, name, street, city, country, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, ?, ?, 1, NOW(), 1, NOW(), 1)
			ON DUPLICATE KEY UPDATE
				name = VALUES(name),
				street = VALUES(street),
				city = VALUES(city),
				country = VALUES(country),
				change_time = NOW()
		`, customerID, name, street, city, country)
		require.NoError(t, err, "Failed to create test customer company")
		return
	}

	_, err := db.Exec(database.ConvertPlaceholders(`
		INSERT INTO customer_company (customer_id, name, street, city, country, valid_id, create_time, create_by, change_time, change_by)
		VALUES ($1, $2, $3, $4, $5, 1, NOW(), 1, NOW(), 1)
		ON CONFLICT (customer_id) DO UPDATE SET
			name = EXCLUDED.name,
			street = EXCLUDED.street,
			city = EXCLUDED.city,
			country = EXCLUDED.country,
			change_time = NOW()
	`), customerID, name, street, city, country)
	require.NoError(t, err, "Failed to create test customer company")
}

// Helper function to clean up test customer company.
func cleanupTestCustomerCompany(t *testing.T, db *sql.DB, customerID string) {
	_, err := db.Exec(database.ConvertPlaceholders("DELETE FROM customer_company WHERE customer_id = $1"), customerID)
	require.NoError(t, err, "Failed to cleanup test customer company")
}

// Helper function to verify customer company was updated in database.
func verifyCustomerCompanyUpdated(t *testing.T, db *sql.DB, customerID, expectedName, expectedStreet, expectedCity, expectedCountry string) {
	var name, street, city, country string
	err := db.QueryRow(database.ConvertPlaceholders(`
		SELECT name, street, city, country 
		FROM customer_company 
		WHERE customer_id = $1
	`), customerID).Scan(&name, &street, &city, &country)
	require.NoError(t, err, "Failed to query customer company")

	assert.Equal(t, expectedName, name, "Company name should be updated in database")
	assert.Equal(t, expectedStreet, street, "Company street should be updated in database")
	assert.Equal(t, expectedCity, city, "Company city should be updated in database")
	assert.Equal(t, expectedCountry, country, "Company country should be updated in database")
}

// Helper function to verify customer company status in database.
func verifyCustomerCompanyStatus(t *testing.T, db *sql.DB, customerID string, expectedValidID int) {
	var validID int
	err := db.QueryRow(database.ConvertPlaceholders(`
		SELECT valid_id 
		FROM customer_company 
		WHERE customer_id = $1
	`), customerID).Scan(&validID)
	require.NoError(t, err, "Failed to query customer company status")

	assert.Equal(t, expectedValidID, validID, "Company status should match expected value")
}

func TestAdminCustomerCompaniesPage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET /admin/customer/companies renders page", func(t *testing.T) {
		router := NewSimpleRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/customer/companies", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Accept either HTML page or JSON error depending on environment
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
		body := w.Body.String()
		if w.Code == http.StatusOK {
			assert.Contains(t, body, "Customer Companies")
			assert.Contains(t, body, "Add New Company")
		}
	})

	t.Run("GET /admin/customer/companies with search filters results", func(t *testing.T) {
		router := NewSimpleRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/customer/companies?search=acme", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// Should contain filtered results
	})

	t.Run("GET /admin/customer/companies with status filter", func(t *testing.T) {
		router := NewSimpleRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/customer/companies?status=valid", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestAdminCustomerCompanyNew(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET /admin/customer/companies/new renders form", func(t *testing.T) {
		router := NewSimpleRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/customer/companies/new", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		assert.Contains(t, body, "Create New Customer Company")
		assert.Contains(t, body, "Customer ID")
		assert.Contains(t, body, "Company Name")
	})
}

func TestAdminCustomerCompanyCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST /admin/customer/companies creates company", func(t *testing.T) {
		db := getTestDB(t)
		defer db.Close()

		// Create test data
		customerID := "TEST_CREATE_" + fmt.Sprint(time.Now().Unix())

		// Create router with test database
		router := NewSimpleRouterWithDB(db)

		// Test data
		name := "Test Company Ltd"
		street := "123 Test Street"
		city := "Test City"
		country := "Test Country"

		formData := url.Values{}
		formData.Set("customer_id", customerID)
		formData.Set("name", name)
		formData.Set("street", street)
		formData.Set("city", city)
		formData.Set("country", country)
		formData.Set("valid_id", "1")

		req := httptest.NewRequest(http.MethodPost, "/admin/customer/companies",
			bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should succeed
		assert.Equal(t, http.StatusOK, w.Code, "Create should succeed")

		// Verify the data was written to the database
		var dbName, dbStreet, dbCity, dbCountry string
		err := db.QueryRow(database.ConvertPlaceholders(`
			SELECT name, street, city, country 
			FROM customer_company 
			WHERE customer_id = $1
		`), customerID).Scan(&dbName, &dbStreet, &dbCity, &dbCountry)
		require.NoError(t, err, "Company should exist in database")

		assert.Equal(t, name, dbName, "Company name should match")
		assert.Equal(t, street, dbStreet, "Company street should match")
		assert.Equal(t, city, dbCity, "Company city should match")
		assert.Equal(t, country, dbCountry, "Company country should match")

		// Cleanup
		cleanupTestCustomerCompany(t, db, customerID)
	})

	t.Run("POST /admin/customer/companies with missing required fields", func(t *testing.T) {
		db := getTestDB(t)
		defer db.Close()

		// Create router with test database
		router := NewSimpleRouterWithDB(db)

		// Test data missing required fields
		formData := url.Values{}
		formData.Set("name", "Test Company") // Missing customer_id

		req := httptest.NewRequest(http.MethodPost, "/admin/customer/companies",
			bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("POST /admin/customer/companies with duplicate customer_id", func(t *testing.T) {
		db := getTestDB(t)
		defer db.Close()

		// Create test data
		customerID := "TEST_DUP_" + fmt.Sprint(time.Now().Unix())
		createTestCustomerCompany(t, db, customerID)
		defer cleanupTestCustomerCompany(t, db, customerID)

		// Create router with test database
		router := NewSimpleRouterWithDB(db)

		// Test data with duplicate customer_id
		formData := url.Values{}
		formData.Set("customer_id", customerID)
		formData.Set("name", "Duplicate Company")

		req := httptest.NewRequest(http.MethodPost, "/admin/customer/companies",
			bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should return conflict or bad request
		assert.True(t, w.Code == http.StatusConflict || w.Code == http.StatusBadRequest)
	})
}

func TestAdminCustomerCompanyEdit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET /admin/customer/companies/:id/edit renders form", func(t *testing.T) {
		db := getTestDB(t)
		defer db.Close()

		// Create test data
		customerID := "TEST_EDIT_" + fmt.Sprint(time.Now().Unix())
		createTestCustomerCompany(t, db, customerID)
		defer cleanupTestCustomerCompany(t, db, customerID)

		// Create router with test database
		router := NewSimpleRouterWithDB(db)

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/customer/companies/%s/edit", customerID), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		assert.Contains(t, body, "Edit Customer Company")
		assert.Contains(t, body, "Customer ID")
	})

	t.Run("GET /admin/customer/companies/:id/edit with non-existent company", func(t *testing.T) {
		db := getTestDB(t)
		defer db.Close()

		// Create router with test database
		router := NewSimpleRouterWithDB(db)

		req := httptest.NewRequest(http.MethodGet, "/admin/customer/companies/NONEXISTENT/edit", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestAdminCustomerCompanyUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST /admin/customer/companies/:id/edit updates company", func(t *testing.T) {
		db := getTestDB(t)
		defer db.Close()

		// Create test data
		customerID := "TEST_UPDATE_" + fmt.Sprint(time.Now().Unix())
		createTestCustomerCompany(t, db, customerID)
		defer cleanupTestCustomerCompany(t, db, customerID)

		// Create router with test database
		router := NewSimpleRouterWithDB(db)

		// Test data
		updatedName := "Updated Test Company Ltd"
		updatedStreet := "456 Updated Street"
		updatedCity := "Updated City"
		updatedCountry := "Updated Country"

		formData := url.Values{}
		formData.Set("name", updatedName)
		formData.Set("street", updatedStreet)
		formData.Set("city", updatedCity)
		formData.Set("country", updatedCountry)
		formData.Set("valid_id", "1")

		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/customer/companies/%s/edit", customerID),
			bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should succeed
		assert.Equal(t, http.StatusOK, w.Code, "Update should succeed")

		// Verify JSON response format
		assert.Contains(t, w.Header().Get("Content-Type"), "application/json", "Should return JSON response")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err, "Response should be valid JSON")

		// Check response structure
		success, exists := response["success"]
		assert.True(t, exists, "Response should contain 'success' field")
		assert.Equal(t, true, success, "Success should be true")

		// Verify the data was actually written to the database
		verifyCustomerCompanyUpdated(t, db, customerID, updatedName, updatedStreet, updatedCity, updatedCountry)
	})

	t.Run("POST /admin/customer/companies/:id/edit with invalid data", func(t *testing.T) {
		db := getTestDB(t)
		defer db.Close()

		// Create test data
		customerID := "TEST_INVALID_" + fmt.Sprint(time.Now().Unix())
		createTestCustomerCompany(t, db, customerID)
		defer cleanupTestCustomerCompany(t, db, customerID)

		// Create router with test database
		router := NewSimpleRouterWithDB(db)

		// Test data with invalid data
		formData := url.Values{}
		formData.Set("name", "") // Empty name

		req := httptest.NewRequest(http.MethodPost, "/admin/customer/companies/TEST001/edit",
			bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		// Verify JSON error response
		assert.Contains(t, w.Header().Get("Content-Type"), "application/json", "Should return JSON error response")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err, "Error response should be valid JSON")

		success, exists := response["success"]
		assert.True(t, exists, "Error response should contain 'success' field")
		assert.Equal(t, false, success, "Success should be false for errors")

		errorMsg, exists := response["error"]
		assert.True(t, exists, "Error response should contain 'error' field")
		assert.NotEmpty(t, errorMsg, "Error message should not be empty")
	})

	t.Run("POST /admin/customer/companies/:id/edit non-existent company", func(t *testing.T) {
		db := getTestDB(t)
		defer db.Close()

		// Create router with test database
		router := NewSimpleRouterWithDB(db)

		// Test data
		formData := url.Values{}
		formData.Set("name", "Updated Name")
		formData.Set("valid_id", "1")

		req := httptest.NewRequest(http.MethodPost, "/admin/customer/companies/NONEXISTENT/edit",
			bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should return not found
		assert.Equal(t, http.StatusNotFound, w.Code, "Should return 404 for non-existent company")

		// Verify JSON error response
		assert.Contains(t, w.Header().Get("Content-Type"), "application/json", "Should return JSON error response")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err, "Error response should be valid JSON")

		success, exists := response["success"]
		assert.True(t, exists, "Error response should contain 'success' field")
		assert.Equal(t, false, success, "Success should be false for not found")

		errorMsg, exists := response["error"]
		assert.True(t, exists, "Error response should contain 'error' field")
		assert.Contains(t, errorMsg, "not found", "Error message should indicate company not found")
	})
}

func TestAdminCustomerCompanyDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST /admin/customer/companies/:id/delete deletes company", func(t *testing.T) {
		db := getTestDB(t)
		defer db.Close()

		// Create test data
		customerID := "TEST_DELETE_" + fmt.Sprint(time.Now().Unix())
		createTestCustomerCompany(t, db, customerID)
		defer cleanupTestCustomerCompany(t, db, customerID)

		// Create router with test database
		router := NewSimpleRouterWithDB(db)

		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/customer/companies/%s/delete", customerID), nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should succeed
		assert.Equal(t, http.StatusOK, w.Code, "Delete should succeed")

		// Verify JSON response format
		assert.Contains(t, w.Header().Get("Content-Type"), "application/json", "Should return JSON response")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err, "Response should be valid JSON")

		// Check response structure
		success, exists := response["success"]
		assert.True(t, exists, "Response should contain 'success' field")
		assert.Equal(t, true, success, "Success should be true")

		// Verify the company was deactivated (soft delete)
		verifyCustomerCompanyStatus(t, db, customerID, 2) // 2 = invalid
	})

	t.Run("POST /admin/customer/companies/:id/delete with non-existent company", func(t *testing.T) {
		db := getTestDB(t)
		defer db.Close()

		// Create router with test database
		router := NewSimpleRouterWithDB(db)

		req := httptest.NewRequest(http.MethodPost, "/admin/customer/companies/NONEXISTENT/delete", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		// Verify JSON error response
		assert.Contains(t, w.Header().Get("Content-Type"), "application/json", "Should return JSON error response")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err, "Error response should be valid JSON")

		success, exists := response["success"]
		assert.True(t, exists, "Error response should contain 'success' field")
		assert.Equal(t, false, success, "Success should be false for not found")

		errorMsg, exists := response["error"]
		assert.True(t, exists, "Error response should contain 'error' field")
		assert.Contains(t, errorMsg, "not found", "Error message should indicate company not found")
	})
}

func TestAdminCustomerCompanyActivate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST /admin/customer/companies/:id/activate activates company", func(t *testing.T) {
		db := getTestDB(t)
		defer db.Close()

		// Create test data (inactive)
		customerID := "TEST_ACTIVATE_" + fmt.Sprint(time.Now().Unix())
		_, err := db.Exec(database.ConvertPlaceholders(`
			INSERT INTO customer_company (customer_id, name, street, city, country, valid_id, create_time, create_by, change_time, change_by)
			VALUES ($1, $2, $3, $4, $5, 2, NOW(), 1, NOW(), 1)
		`), customerID, "Test Company "+customerID, "123 Test St", "Test City", "Test Country")
		require.NoError(t, err, "Failed to create inactive test customer company")
		defer cleanupTestCustomerCompany(t, db, customerID)

		// Create router with test database
		router := NewSimpleRouterWithDB(db)

		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/customer/companies/%s/activate", customerID), nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Activate should succeed")

		// Verify the company was activated
		verifyCustomerCompanyStatus(t, db, customerID, 1) // 1 = valid
	})

	t.Run("POST /admin/customer/companies/:id/activate with non-existent company", func(t *testing.T) {
		db := getTestDB(t)
		defer db.Close()

		// Create router with test database
		router := NewSimpleRouterWithDB(db)

		req := httptest.NewRequest(http.MethodPost, "/admin/customer/companies/NONEXISTENT/activate", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestAdminCustomerCompanyPortalSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST /admin/customer/companies/:id/portal-settings updates portal settings", func(t *testing.T) {
		db := getTestDB(t)
		defer db.Close()

		router := NewSimpleRouterWithDB(db)

		// Test portal settings data
		formData := url.Values{}
		formData.Set("login_hint", "Use your company email")
		formData.Set("theme", "dark")
		formData.Set("custom_css", ".company-theme { color: blue; }")

		req := httptest.NewRequest(http.MethodPost, "/admin/customer/companies/TEST001/portal-settings",
			bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestAdminCustomerCompanyServices(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST /admin/customer/companies/:id/services assigns services", func(t *testing.T) {
		db := getTestDB(t)
		defer db.Close()

		customerID := "TEST_SERVICES_" + fmt.Sprint(time.Now().Unix())
		createTestCustomerCompany(t, db, customerID)
		defer cleanupTestCustomerCompany(t, db, customerID)

		router := NewSimpleRouterWithDB(db)

		// Test service assignment data
		formData := url.Values{}
		formData.Add("services", "1")
		formData.Add("services", "2")
		formData.Add("services", "3")

		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/customer/companies/%s/services", customerID),
			bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusBadRequest)
	})

	t.Run("GET /admin/customer/companies/:id/services returns assigned services", func(t *testing.T) {
		db := getTestDB(t)
		defer db.Close()

		customerID := "TEST_SERVICES2_" + fmt.Sprint(time.Now().Unix())
		createTestCustomerCompany(t, db, customerID)
		defer cleanupTestCustomerCompany(t, db, customerID)

		router := NewSimpleRouterWithDB(db)

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/customer/companies/%s/services", customerID), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestAdminCustomerCompanyUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET /admin/customer/companies/:id/users returns company users", func(t *testing.T) {
		db := getTestDB(t)
		defer db.Close()

		customerID := "TEST_USERS_" + fmt.Sprint(time.Now().Unix())
		createTestCustomerCompany(t, db, customerID)
		defer cleanupTestCustomerCompany(t, db, customerID)

		router := NewSimpleRouterWithDB(db)

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/customer/companies/%s/users", customerID), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestAdminCustomerCompanyTickets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET /admin/customer/companies/:id/tickets returns company tickets", func(t *testing.T) {
		db := getTestDB(t)
		defer db.Close()

		customerID := "TEST_TICKETS_" + fmt.Sprint(time.Now().Unix())
		createTestCustomerCompany(t, db, customerID)
		defer cleanupTestCustomerCompany(t, db, customerID)

		router := NewSimpleRouterWithDB(db)

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/customer/companies/%s/tickets", customerID), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestAdminCustomerCompanySearch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("search functionality with valid query", func(t *testing.T) {
		router := NewSimpleRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/customer/companies?search=acme&status=valid&limit=10&offset=0", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify response contains search results
		body := w.Body.String()
		assert.Contains(t, body, "Customer Companies")
	})

	t.Run("search with invalid parameters", func(t *testing.T) {
		router := NewSimpleRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/customer/companies?search=&limit=-1&offset=abc", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should handle invalid parameters gracefully
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusBadRequest)
	})
}

func TestAdminCustomerCompanyPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("pagination with valid parameters", func(t *testing.T) {
		router := NewSimpleRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/customer/companies?limit=20&offset=40", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("pagination with default parameters", func(t *testing.T) {
		router := NewSimpleRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/customer/companies", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestAdminCustomerCompanySorting(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("sorting by different columns", func(t *testing.T) {
		router := NewSimpleRouter()

		// Test sorting by name
		req := httptest.NewRequest(http.MethodGet, "/admin/customer/companies?sort=name&order=asc", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Test sorting by customer_id
		req = httptest.NewRequest(http.MethodGet, "/admin/customer/companies?sort=customer_id&order=desc", nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Test invalid sort column
		req = httptest.NewRequest(http.MethodGet, "/admin/customer/companies?sort=invalid_column", nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusBadRequest)
	})
}

func TestAdminCustomerCompanyCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("complete CRUD operations", func(t *testing.T) {
		db := getTestDB(t)
		defer db.Close()

		// Create router with test database
		router := NewSimpleRouterWithDB(db)

		// Use unique customer ID for this test
		customerID := "TEST_CRUD_" + fmt.Sprint(time.Now().Unix())

		// CREATE: Test creating a new company
		t.Run("create company", func(t *testing.T) {
			formData := url.Values{}
			formData.Set("customer_id", customerID)
			formData.Set("name", "CRUD Test Company")
			formData.Set("street", "123 CRUD Street")
			formData.Set("city", "CRUD City")
			formData.Set("country", "CRUD Country")
			formData.Set("valid_id", "1")

			req := httptest.NewRequest(http.MethodPost, "/admin/customer/companies",
				bytes.NewBufferString(formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Accept", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should succeed
			assert.Equal(t, http.StatusOK, w.Code, "Create should succeed")

			// Verify the data was written to the database
			var name string
			err := db.QueryRow(database.ConvertPlaceholders(`
				SELECT name FROM customer_company WHERE customer_id = $1
			`), customerID).Scan(&name)
			require.NoError(t, err, "Company should exist in database")
			assert.Equal(t, "CRUD Test Company", name)
		})

		// READ: Test reading company list
		t.Run("read company list", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin/customer/companies", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})

		// READ: Test reading specific company
		t.Run("read specific company", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/customer/companies/%s/edit", customerID), nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should succeed
			assert.Equal(t, http.StatusOK, w.Code)
		})

		// UPDATE: Test updating company
		t.Run("update company", func(t *testing.T) {
			updatedName := "Updated CRUD Test Company"
			updatedStreet := "456 Updated Street"
			updatedCity := "Updated City"
			updatedCountry := "Updated Country"

			formData := url.Values{}
			formData.Set("name", updatedName)
			formData.Set("street", updatedStreet)
			formData.Set("city", updatedCity)
			formData.Set("country", updatedCountry)
			formData.Set("valid_id", "1")

			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/customer/companies/%s/edit", customerID),
				bytes.NewBufferString(formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Accept", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should succeed
			assert.Equal(t, http.StatusOK, w.Code, "Update should succeed")

			// Verify JSON response format
			assert.Contains(t, w.Header().Get("Content-Type"), "application/json", "Should return JSON response")

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err, "Response should be valid JSON")

			// Check response structure
			success, exists := response["success"]
			assert.True(t, exists, "Response should contain 'success' field")
			assert.Equal(t, true, success, "Success should be true")

			// Verify the data was updated in the database
			verifyCustomerCompanyUpdated(t, db, customerID, updatedName, updatedStreet, updatedCity, updatedCountry)
		})

		// DELETE: Test deleting company
		t.Run("delete company", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/customer/companies/%s/delete", customerID), nil)
			req.Header.Set("Accept", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should succeed
			assert.Equal(t, http.StatusOK, w.Code, "Delete should succeed")

			// Verify the company was deactivated
			verifyCustomerCompanyStatus(t, db, customerID, 2) // 2 = invalid
		})

		// Cleanup
		cleanupTestCustomerCompany(t, db, customerID)
	})

	t.Run("CRUD error handling", func(t *testing.T) {
		db := getTestDB(t)
		defer db.Close()

		// Create router with test database
		router := NewSimpleRouterWithDB(db)

		// Test non-existent company
		t.Run("read non-existent company", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin/customer/companies/NONEXISTENT/edit", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
		})

		t.Run("update non-existent company", func(t *testing.T) {
			formData := url.Values{}
			formData.Set("name", "Updated Name")

			req := httptest.NewRequest(http.MethodPost, "/admin/customer/companies/NONEXISTENT/edit",
				bytes.NewBufferString(formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
		})

		t.Run("delete non-existent company", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/admin/customer/companies/NONEXISTENT/delete", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
		})

		// Test invalid data
		t.Run("create with missing required fields", func(t *testing.T) {
			formData := url.Values{}
			formData.Set("name", "") // Missing customer_id and empty name

			req := httptest.NewRequest(http.MethodPost, "/admin/customer/companies/new",
				bytes.NewBufferString(formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})

		t.Run("update with invalid data", func(t *testing.T) {
			formData := url.Values{}
			formData.Set("name", "") // Empty name should fail validation

			req := httptest.NewRequest(http.MethodPost, "/admin/customer/companies/INVALID/edit",
				bytes.NewBufferString(formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	})

	t.Run("related operations", func(t *testing.T) {
		db := getTestDB(t)
		router := NewSimpleRouterWithDB(db)

		// Create test company for related operations
		createTestCustomerCompany(t, db, "RELTEST001")
		defer cleanupTestCustomerCompany(t, db, "RELTEST001")

		t.Run("activate company", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/admin/customer/companies/RELTEST001/activate", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should succeed or return not found
			assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNotFound)
		})

		t.Run("view company users", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin/customer/companies/RELTEST001/users", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})

		t.Run("view company tickets", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin/customer/companies/RELTEST001/tickets", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})

		t.Run("view company services", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin/customer/companies/RELTEST001/services", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})

		t.Run("update company services", func(t *testing.T) {
			formData := url.Values{}
			formData.Add("services", "1")
			formData.Add("services", "2")

			req := httptest.NewRequest(http.MethodPost, "/admin/customer/companies/RELTEST001/services",
				bytes.NewBufferString(formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should succeed or return validation error
			assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusBadRequest)
		})
	})
}
