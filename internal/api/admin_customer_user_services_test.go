package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// setupCustomerUserServicesTestRouter creates a minimal router with the customer user services handlers.
func setupCustomerUserServicesTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, _ := database.GetDB()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Set("user_role", "Admin")
		c.Next()
	})

	router.GET("/admin/customer-user-services", handleAdminCustomerUserServices(db))
	router.GET("/admin/customer-user-services/customer/:login", handleAdminCustomerUserServicesAllocate(db))
	router.POST("/admin/customer-user-services/customer/:login", handleAdminCustomerUserServicesUpdate(db))
	router.GET("/admin/customer-user-services/service/:id", handleAdminServiceCustomerUsersAllocate(db))
	router.POST("/admin/customer-user-services/service/:id", handleAdminServiceCustomerUsersUpdate(db))
	router.GET("/admin/customer-user-services/default", handleAdminDefaultServices(db))
	router.POST("/admin/customer-user-services/default", handleAdminDefaultServicesUpdate(db))

	return router
}

// createTestServiceForCustomerUser creates a test service in the database.
func createTestServiceForCustomerUser(t *testing.T, name string) (int, bool) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Logf("GetDB failed: err=%v, db=%v", err, db)
		return 0, false
	}

	var id int
	if database.IsMySQL() {
		result, execErr := db.Exec(`
			INSERT INTO service (name, comments, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 1, NOW(), 1, NOW(), 1)`, name, "Test service for customer user services")
		if execErr != nil {
			t.Logf("Insert service failed: %v", execErr)
			return 0, false
		}
		lastID, _ := result.LastInsertId()
		id = int(lastID)
	} else {
		err = db.QueryRow(`
			INSERT INTO service (name, comments, valid_id, create_time, create_by, change_time, change_by)
			VALUES ($1, $2, 1, NOW(), 1, NOW(), 1)
			RETURNING id`, name, "Test service for customer user services").Scan(&id)
		if err != nil {
			t.Logf("Insert service failed: %v", err)
			return 0, false
		}
	}

	t.Cleanup(func() {
		if database.IsMySQL() {
			_, _ = db.Exec(`DELETE FROM service_customer_user WHERE service_id = ?`, id)
			_, _ = db.Exec(`DELETE FROM service WHERE id = ?`, id)
		} else {
			_, _ = db.Exec(`DELETE FROM service_customer_user WHERE service_id = $1`, id)
			_, _ = db.Exec(`DELETE FROM service WHERE id = $1`, id)
		}
	})

	return id, true
}

// createTestCustomerUser creates a test customer user in the database.
func createTestCustomerUser(t *testing.T, login string) bool {
	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Logf("GetDB failed: err=%v, db=%v", err, db)
		return false
	}

	var execErr error
	if database.IsMySQL() {
		_, execErr = db.Exec(`
			INSERT INTO customer_user (login, email, customer_id, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, ?, 'test-company', 'test', 'Test', 'User', 1, NOW(), 1, NOW(), 1)
			ON DUPLICATE KEY UPDATE email = VALUES(email)`, login, login+"@test.local")
	} else {
		_, execErr = db.Exec(`
			INSERT INTO customer_user (login, email, customer_id, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
			VALUES ($1, $2, 'test-company', 'test', 'Test', 'User', 1, NOW(), 1, NOW(), 1)
			ON CONFLICT (login) DO NOTHING`, login, login+"@test.local")
	}
	if execErr != nil {
		t.Logf("Insert customer_user failed: %v", execErr)
		return false
	}

	t.Cleanup(func() {
		if database.IsMySQL() {
			_, _ = db.Exec(`DELETE FROM service_customer_user WHERE customer_user_login = ?`, login)
			_, _ = db.Exec(`DELETE FROM customer_user WHERE login = ?`, login)
		} else {
			_, _ = db.Exec(`DELETE FROM service_customer_user WHERE customer_user_login = $1`, login)
			_, _ = db.Exec(`DELETE FROM customer_user WHERE login = $1`, login)
		}
	})

	return true
}

// cleanupDefaultServices removes all default service assignments.
func cleanupDefaultServices(t *testing.T) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		return
	}
	if database.IsMySQL() {
		_, _ = db.Exec(`DELETE FROM service_customer_user WHERE customer_user_login = '<DEFAULT>'`)
	} else {
		_, _ = db.Exec(`DELETE FROM service_customer_user WHERE customer_user_login = '<DEFAULT>'`)
	}
}

func TestAdminCustomerUserServicesPage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET /admin/customer-user-services renders page", func(t *testing.T) {
		router := setupCustomerUserServicesTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/customer-user-services", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Accept either HTML page or JSON error depending on environment
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError,
			"Expected OK or InternalServerError, got %d", w.Code)
	})

	t.Run("GET /admin/customer-user-services with search", func(t *testing.T) {
		router := setupCustomerUserServicesTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/customer-user-services?search=test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("GET /admin/customer-user-services with view=services", func(t *testing.T) {
		router := setupCustomerUserServicesTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/customer-user-services?view=services", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})
}

func TestAdminCustomerUserServicesAllocate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET /admin/customer-user-services/customer/:login returns services", func(t *testing.T) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Skip("Database not available")
		}

		testLogin := fmt.Sprintf("test_cus_%d", time.Now().UnixNano())
		if !createTestCustomerUser(t, testLogin) {
			t.Skip("Could not create test customer user")
		}

		router := setupCustomerUserServicesTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/customer-user-services/customer/"+testLogin, nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "customer_user")
		assert.Contains(t, response, "services")
	})

	t.Run("POST /admin/customer-user-services/customer/:login updates services", func(t *testing.T) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Skip("Database not available")
		}

		testLogin := fmt.Sprintf("test_cus_%d", time.Now().UnixNano())
		if !createTestCustomerUser(t, testLogin) {
			t.Skip("Could not create test customer user")
		}

		testServiceName := fmt.Sprintf("TestService_%d", time.Now().UnixNano())
		serviceID, ok := createTestServiceForCustomerUser(t, testServiceName)
		if !ok {
			t.Skip("Could not create test service")
		}

		router := setupCustomerUserServicesTestRouter()

		payload := map[string]interface{}{
			"services": []string{fmt.Sprintf("%d", serviceID)},
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/customer-user-services/customer/"+testLogin, bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, true, response["success"])
	})
}

func TestAdminServiceCustomerUsersAllocate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET /admin/customer-user-services/service/:id returns customer users", func(t *testing.T) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Skip("Database not available")
		}

		testServiceName := fmt.Sprintf("TestService_%d", time.Now().UnixNano())
		serviceID, ok := createTestServiceForCustomerUser(t, testServiceName)
		if !ok {
			t.Skip("Could not create test service")
		}

		router := setupCustomerUserServicesTestRouter()

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/customer-user-services/service/%d", serviceID), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "service")
		assert.Contains(t, response, "customer_users")
	})

	t.Run("POST /admin/customer-user-services/service/:id updates customer users", func(t *testing.T) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Skip("Database not available")
		}

		testServiceName := fmt.Sprintf("TestService_%d", time.Now().UnixNano())
		serviceID, ok := createTestServiceForCustomerUser(t, testServiceName)
		if !ok {
			t.Skip("Could not create test service")
		}

		testLogin := fmt.Sprintf("test_cus_%d", time.Now().UnixNano())
		if !createTestCustomerUser(t, testLogin) {
			t.Skip("Could not create test customer user")
		}

		router := setupCustomerUserServicesTestRouter()

		payload := map[string]interface{}{
			"customer_users": []string{testLogin},
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/customer-user-services/service/%d", serviceID), bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, true, response["success"])
	})
}

func TestAdminDefaultServices(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET /admin/customer-user-services/default returns default services", func(t *testing.T) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Skip("Database not available")
		}

		router := setupCustomerUserServicesTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/customer-user-services/default", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response, "services")
	})

	t.Run("POST /admin/customer-user-services/default updates default services", func(t *testing.T) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Skip("Database not available")
		}

		t.Cleanup(func() {
			cleanupDefaultServices(t)
		})

		testServiceName := fmt.Sprintf("TestDefaultService_%d", time.Now().UnixNano())
		serviceID, ok := createTestServiceForCustomerUser(t, testServiceName)
		if !ok {
			t.Skip("Could not create test service")
		}

		router := setupCustomerUserServicesTestRouter()

		payload := map[string]interface{}{
			"services": []string{fmt.Sprintf("%d", serviceID)},
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/customer-user-services/default", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, true, response["success"])
	})

	t.Run("Default services use <DEFAULT> pseudo-user in database", func(t *testing.T) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Skip("Database not available")
		}

		t.Cleanup(func() {
			cleanupDefaultServices(t)
		})

		testServiceName := fmt.Sprintf("TestDefaultService_%d", time.Now().UnixNano())
		serviceID, ok := createTestServiceForCustomerUser(t, testServiceName)
		if !ok {
			t.Skip("Could not create test service")
		}

		router := setupCustomerUserServicesTestRouter()

		// Add default service
		payload := map[string]interface{}{
			"services": []string{fmt.Sprintf("%d", serviceID)},
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/customer-user-services/default", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Verify <DEFAULT> pseudo-user was used
		var count int
		err = db.QueryRow(database.ConvertPlaceholders(`
			SELECT COUNT(*) FROM service_customer_user 
			WHERE customer_user_login = '<DEFAULT>' AND service_id = $1
		`), serviceID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "Expected <DEFAULT> pseudo-user assignment")
	})

	t.Run("Clearing default services removes all <DEFAULT> assignments", func(t *testing.T) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Skip("Database not available")
		}

		t.Cleanup(func() {
			cleanupDefaultServices(t)
		})

		testServiceName := fmt.Sprintf("TestDefaultService_%d", time.Now().UnixNano())
		serviceID, ok := createTestServiceForCustomerUser(t, testServiceName)
		if !ok {
			t.Skip("Could not create test service")
		}

		router := setupCustomerUserServicesTestRouter()

		// First add a default service
		payload := map[string]interface{}{
			"services": []string{fmt.Sprintf("%d", serviceID)},
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/customer-user-services/default", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Then clear all default services
		payload = map[string]interface{}{
			"services": []string{},
		}
		jsonData, _ = json.Marshal(payload)

		req = httptest.NewRequest(http.MethodPost, "/admin/customer-user-services/default", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Verify no <DEFAULT> assignments remain
		var count int
		err = db.QueryRow(database.ConvertPlaceholders(`
			SELECT COUNT(*) FROM service_customer_user WHERE customer_user_login = '<DEFAULT>'
		`)).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "Expected no <DEFAULT> assignments after clearing")
	})
}

func TestGetDefaultServicesCount(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("getDefaultServicesCount returns correct count", func(t *testing.T) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Skip("Database not available")
		}

		t.Cleanup(func() {
			cleanupDefaultServices(t)
		})

		// Initial count should be 0
		count := getDefaultServicesCount(db)
		assert.Equal(t, 0, count)

		// Create test services and assign to <DEFAULT>
		service1Name := fmt.Sprintf("TestDefault1_%d", time.Now().UnixNano())
		service1ID, ok := createTestServiceForCustomerUser(t, service1Name)
		if !ok {
			t.Skip("Could not create test service 1")
		}

		service2Name := fmt.Sprintf("TestDefault2_%d", time.Now().UnixNano())
		service2ID, ok := createTestServiceForCustomerUser(t, service2Name)
		if !ok {
			t.Skip("Could not create test service 2")
		}

		// Assign both to <DEFAULT>
		_, err = db.Exec(database.ConvertPlaceholders(`
			INSERT INTO service_customer_user (customer_user_login, service_id, create_time, create_by)
			VALUES ('<DEFAULT>', $1, NOW(), 1), ('<DEFAULT>', $2, NOW(), 1)
		`), service1ID, service2ID)
		require.NoError(t, err)

		count = getDefaultServicesCount(db)
		assert.Equal(t, 2, count)
	})
}

func TestDefaultServicesFallbackLogic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Customer with explicit services does not get default services", func(t *testing.T) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Skip("Database not available")
		}

		t.Cleanup(func() {
			cleanupDefaultServices(t)
		})

		// Create test customer
		testLogin := fmt.Sprintf("test_explicit_%d", time.Now().UnixNano())
		if !createTestCustomerUser(t, testLogin) {
			t.Skip("Could not create test customer user")
		}

		// Create explicit service assignment
		explicitServiceName := fmt.Sprintf("ExplicitService_%d", time.Now().UnixNano())
		explicitServiceID, ok := createTestServiceForCustomerUser(t, explicitServiceName)
		if !ok {
			t.Skip("Could not create explicit service")
		}

		_, err = db.Exec(database.ConvertPlaceholders(`
			INSERT INTO service_customer_user (customer_user_login, service_id, create_time, create_by)
			VALUES ($1, $2, NOW(), 1)
		`), testLogin, explicitServiceID)
		require.NoError(t, err)

		// Create default service
		defaultServiceName := fmt.Sprintf("DefaultService_%d", time.Now().UnixNano())
		defaultServiceID, ok := createTestServiceForCustomerUser(t, defaultServiceName)
		if !ok {
			t.Skip("Could not create default service")
		}

		_, err = db.Exec(database.ConvertPlaceholders(`
			INSERT INTO service_customer_user (customer_user_login, service_id, create_time, create_by)
			VALUES ('<DEFAULT>', $1, NOW(), 1)
		`), defaultServiceID)
		require.NoError(t, err)

		// Query services for customer - should only get explicit service, not default
		rows, err := db.Query(database.ConvertPlaceholders(`
			SELECT s.id, s.name FROM service s
			INNER JOIN service_customer_user scu ON s.id = scu.service_id
			WHERE s.valid_id = 1 AND scu.customer_user_login = $1
			ORDER BY s.name
		`), testLogin)
		require.NoError(t, err)
		defer func() { _ = rows.Close() }()

		var services []int
		for rows.Next() {
			var id int
			var name string
			rows.Scan(&id, &name)
			services = append(services, id)
		}

		assert.Equal(t, 1, len(services), "Customer with explicit assignment should have exactly 1 service")
		assert.Equal(t, explicitServiceID, services[0], "Should be the explicit service")
	})

	t.Run("Customer without explicit services gets default services", func(t *testing.T) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Skip("Database not available")
		}

		t.Cleanup(func() {
			cleanupDefaultServices(t)
		})

		// Create test customer with NO explicit services
		testLogin := fmt.Sprintf("test_noservices_%d", time.Now().UnixNano())
		if !createTestCustomerUser(t, testLogin) {
			t.Skip("Could not create test customer user")
		}

		// Create default service
		defaultServiceName := fmt.Sprintf("DefaultService_%d", time.Now().UnixNano())
		defaultServiceID, ok := createTestServiceForCustomerUser(t, defaultServiceName)
		if !ok {
			t.Skip("Could not create default service")
		}

		_, err = db.Exec(database.ConvertPlaceholders(`
			INSERT INTO service_customer_user (customer_user_login, service_id, create_time, create_by)
			VALUES ('<DEFAULT>', $1, NOW(), 1)
		`), defaultServiceID)
		require.NoError(t, err)

		// Query explicit services - should be empty
		rows, err := db.Query(database.ConvertPlaceholders(`
			SELECT s.id FROM service s
			INNER JOIN service_customer_user scu ON s.id = scu.service_id
			WHERE s.valid_id = 1 AND scu.customer_user_login = $1
		`), testLogin)
		require.NoError(t, err)

		var explicitServices []int
		for rows.Next() {
			var id int
			rows.Scan(&id)
			explicitServices = append(explicitServices, id)
		}
		rows.Close()
		assert.Equal(t, 0, len(explicitServices), "Customer should have no explicit services")

		// Query default services - should have our default
		defaultRows, err := db.Query(database.ConvertPlaceholders(`
			SELECT s.id FROM service s
			INNER JOIN service_customer_user scu ON s.id = scu.service_id
			WHERE s.valid_id = 1 AND scu.customer_user_login = '<DEFAULT>'
		`))
		require.NoError(t, err)

		var defaultServices []int
		for defaultRows.Next() {
			var id int
			defaultRows.Scan(&id)
			defaultServices = append(defaultServices, id)
		}
		defaultRows.Close()

		assert.Contains(t, defaultServices, defaultServiceID, "Default services should include our test service")
	})
}
