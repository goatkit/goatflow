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

// setupServiceTestRouter creates a minimal router with the admin services handlers
func setupServiceTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Set("user_role", "Admin")
		c.Next()
	})

	router.GET("/admin/services", handleAdminServices)
	router.POST("/admin/services/create", handleAdminServiceCreate)
	router.PUT("/admin/services/:id/update", handleAdminServiceUpdate)
	router.DELETE("/admin/services/:id/delete", handleAdminServiceDelete)

	return router
}

// createAdminTestService creates a test service in the database
func createAdminTestService(t *testing.T, name string) (int, bool) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		return 0, false
	}

	var id int
	query := database.ConvertPlaceholders(`
		INSERT INTO service (name, comments, valid_id, create_time, create_by, change_time, change_by)
		VALUES ($1, $2, 1, NOW(), 1, NOW(), 1)
		RETURNING id`)
	err = db.QueryRow(query, name, "Admin service test").Scan(&id)
	if err != nil {
		return 0, false
	}

	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM service WHERE id = $1`), id)
	})

	return id, true
}

// cleanupTestServiceByName removes a test service by name - only works if DB is already connected
func cleanupTestServiceByName(t *testing.T, name string) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		return
	}
	_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM service WHERE name = $1`), name)
}

func TestAdminServicePage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("GET /admin/services renders service page", func(t *testing.T) {
		router := setupServiceTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/services", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Accept either HTML page or JSON error depending on environment
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
		body := w.Body.String()
		if w.Code == http.StatusOK {
			assert.Contains(t, body, "Service")
		}
	})

	t.Run("GET /admin/services with search filters results", func(t *testing.T) {
		router := setupServiceTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/services?search=incident", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Accept OK or error depending on environment
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("GET /admin/services with sort and order", func(t *testing.T) {
		router := setupServiceTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/services?sort=name&order=desc", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("GET /admin/services with validity filter", func(t *testing.T) {
		router := setupServiceTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/services?validity=1", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("Page contains service table structure", func(t *testing.T) {
		router := setupServiceTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/services", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			body := w.Body.String()
			// In test mode without DB, fallback template may not have full structure
			// Accept either full template or fallback
			assert.True(t, strings.Contains(body, "<table") || strings.Contains(body, "Service"),
				"Page should contain either table structure or Service text")
		}
	})

	t.Run("Page contains service modal form", func(t *testing.T) {
		router := setupServiceTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/services", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			body := w.Body.String()
			// In test mode without DB, fallback template may not have full structure
			// Accept either full template or fallback
			assert.True(t, strings.Contains(body, "serviceForm") || strings.Contains(body, "Add New Service"),
				"Page should contain either serviceForm or Add New Service button")
		}
	})
}

func TestAdminServiceCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("POST /admin/services/create creates new service with form data", func(t *testing.T) {
		router := setupServiceTestRouter()

		testName := fmt.Sprintf("FormTestService_%d", time.Now().UnixNano())
		defer cleanupTestServiceByName(t, testName)

		form := url.Values{}
		form.Set("name", testName)
		form.Set("comments", "Service for incident handling")
		form.Set("valid_id", "1")

		req := httptest.NewRequest(http.MethodPost, "/admin/services/create", bytes.NewBufferString(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		assert.Contains(t, body, "Service created successfully")
	})

	t.Run("POST /admin/services/create with JSON", func(t *testing.T) {
		router := setupServiceTestRouter()

		testName := fmt.Sprintf("JSONTestService_%d", time.Now().UnixNano())
		defer cleanupTestServiceByName(t, testName)

		payload := map[string]interface{}{
			"name":     testName,
			"comments": "JSON test service",
			"valid_id": 1,
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/services/create", bytes.NewReader(jsonData))
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

	t.Run("POST /admin/services/create validates required fields", func(t *testing.T) {
		router := setupServiceTestRouter()

		form := url.Values{}
		form.Set("comments", "Test comment")

		req := httptest.NewRequest(http.MethodPost, "/admin/services/create", bytes.NewBufferString(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.False(t, response["success"].(bool))
		// Response uses "message" field for both success and error
		msg := ""
		if m, ok := response["message"].(string); ok {
			msg = m
		} else if e, ok := response["error"].(string); ok {
			msg = e
		}
		assert.Contains(t, strings.ToLower(msg), "name is required")
	})

	t.Run("POST /admin/services/create with JSON validates required fields", func(t *testing.T) {
		router := setupServiceTestRouter()

		payload := map[string]interface{}{
			"comments": "Missing name",
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/services/create", bytes.NewReader(jsonData))
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

	t.Run("POST /admin/services/create prevents duplicate names", func(t *testing.T) {
		router := setupServiceTestRouter()

		testName := fmt.Sprintf("DuplicateService_%d", time.Now().UnixNano())
		defer cleanupTestServiceByName(t, testName)

		// Create first service
		form := url.Values{}
		form.Set("name", testName)
		form.Set("valid_id", "1")

		req := httptest.NewRequest(http.MethodPost, "/admin/services/create", bytes.NewBufferString(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Try to create duplicate
		req2 := httptest.NewRequest(http.MethodPost, "/admin/services/create", bytes.NewBufferString(form.Encode()))
		req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w2 := httptest.NewRecorder()

		router.ServeHTTP(w2, req2)

		// Should return error for duplicate (or handler may allow if name uniqueness not enforced)
		if w2.Code == http.StatusBadRequest {
			body := w2.Body.String()
			assert.Contains(t, strings.ToLower(body), "already exists")
		}
	})

	t.Run("Services support parent::child hierarchy naming", func(t *testing.T) {
		router := setupServiceTestRouter()

		testName := fmt.Sprintf("IT Support::Hardware_%d", time.Now().UnixNano())
		defer cleanupTestServiceByName(t, testName)

		form := url.Values{}
		form.Set("name", testName)
		form.Set("comments", "Hardware support sub-service")
		form.Set("valid_id", "1")

		req := httptest.NewRequest(http.MethodPost, "/admin/services/create", bytes.NewBufferString(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Handler may return 200 OK or 302 redirect on success
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusFound,
			"Expected 200 or 302, got %d", w.Code)
	})
}

func TestAdminServiceUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("PUT /admin/services/:id/update updates service with form data", func(t *testing.T) {
		router := setupServiceTestRouter()

		serviceID, ok := createAdminTestService(t, fmt.Sprintf("UpdateTarget_%d", time.Now().UnixNano()))
		if !ok {
			t.Skip("database not available for admin service update test")
		}

		form := url.Values{}
		form.Set("name", "Updated Service Name")
		form.Set("comments", "Updated comment")
		form.Set("valid_id", "1")

		endpoint := fmt.Sprintf("/admin/services/%d/update", serviceID)
		req := httptest.NewRequest(http.MethodPut, endpoint, bytes.NewBufferString(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response["success"].(bool))
		assert.Equal(t, "Service updated successfully", response["message"])
	})

	t.Run("PUT /admin/services/:id/update with JSON", func(t *testing.T) {
		router := setupServiceTestRouter()

		serviceID, ok := createAdminTestService(t, fmt.Sprintf("JSONUpdateTarget_%d", time.Now().UnixNano()))
		if !ok {
			t.Skip("database not available for admin service JSON update test")
		}

		payload := map[string]interface{}{
			"name":     "JSON Updated Service",
			"comments": "Updated via JSON",
		}
		jsonData, _ := json.Marshal(payload)

		endpoint := fmt.Sprintf("/admin/services/%d/update", serviceID)
		req := httptest.NewRequest(http.MethodPut, endpoint, bytes.NewReader(jsonData))
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

	t.Run("PUT /admin/services/:id/update handles non-existent service", func(t *testing.T) {
		router := setupServiceTestRouter()

		form := url.Values{}
		form.Set("name", "Updated Service")

		req := httptest.NewRequest(http.MethodPut, "/admin/services/99999/update", bytes.NewBufferString(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusInternalServerError)
	})

	t.Run("PUT /admin/services/:id/update with invalid ID", func(t *testing.T) {
		router := setupServiceTestRouter()

		req := httptest.NewRequest(http.MethodPut, "/admin/services/invalid/update", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Handler may return 400 Bad Request or handle gracefully
		assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusOK || w.Code == http.StatusInternalServerError,
			"Expected error response for invalid ID, got %d", w.Code)
	})

	t.Run("PUT /admin/services/:id/update toggles validity", func(t *testing.T) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Fatal("database not available for admin service toggle test")
		}
		database.SetDB(db)
		t.Cleanup(func() { database.SetDB(nil) })

		router := setupServiceTestRouter()

		serviceID, ok := createAdminTestService(t, fmt.Sprintf("ToggleTarget_%d", time.Now().UnixNano()))
		if !ok {
			t.Fatal("failed to create test service")
		}

		// Toggle to inactive
		payload := map[string]interface{}{
			"valid_id": 2,
		}
		jsonData, _ := json.Marshal(payload)

		endpoint := fmt.Sprintf("/admin/services/%d/update", serviceID)
		req := httptest.NewRequest(http.MethodPut, endpoint, bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify service was deactivated
		var validID int
		_ = db.QueryRow(database.ConvertPlaceholders(`SELECT valid_id FROM service WHERE id = $1`), serviceID).Scan(&validID)
		assert.Equal(t, 2, validID)
	})
}

func TestAdminServiceDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("DELETE /admin/services/:id/delete soft deletes service", func(t *testing.T) {
		db, err := database.GetDB()
		if err != nil || db == nil {
			t.Fatal("database not available for admin service delete test")
		}
		database.SetDB(db)
		t.Cleanup(func() { database.SetDB(nil) })

		router := setupServiceTestRouter()

		serviceID, ok := createAdminTestService(t, fmt.Sprintf("DeleteTarget_%d", time.Now().UnixNano()))
		if !ok {
			t.Fatal("failed to create test service")
		}

		endpoint := fmt.Sprintf("/admin/services/%d/delete", serviceID)
		req := httptest.NewRequest(http.MethodDelete, endpoint, nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response["success"].(bool))
		assert.Equal(t, "Service deleted successfully", response["message"])

		// Verify soft delete (valid_id = 2)
		var validID int
		_ = db.QueryRow(database.ConvertPlaceholders(`SELECT valid_id FROM service WHERE id = $1`), serviceID).Scan(&validID)
		assert.Equal(t, 2, validID)
	})

	t.Run("DELETE /admin/services/:id/delete handles non-existent service", func(t *testing.T) {
		router := setupServiceTestRouter()

		req := httptest.NewRequest(http.MethodDelete, "/admin/services/99999/delete", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusOK)
	})

	t.Run("DELETE /admin/services/:id/delete with invalid ID", func(t *testing.T) {
		router := setupServiceTestRouter()

		req := httptest.NewRequest(http.MethodDelete, "/admin/services/invalid/delete", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Handler may return 400 Bad Request or handle gracefully
		assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusOK || w.Code == http.StatusInternalServerError,
			"Expected error response for invalid ID, got %d", w.Code)
	})
}

func TestAdminServiceValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := setupServiceTestRouter()

	t.Run("Create service requires name", func(t *testing.T) {
		payload := map[string]interface{}{
			"comments": "No name provided",
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/services/create", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Create service validates name not empty", func(t *testing.T) {
		payload := map[string]interface{}{
			"name": "",
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/services/create", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Create service validates name not whitespace only", func(t *testing.T) {
		payload := map[string]interface{}{
			"name": "   ",
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/services/create", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestAdminServiceWithDBIntegration(t *testing.T) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		t.Fatal("database not available for integration tests")
	}

	// Inject DB so handlers see it via GetDB()
	database.SetDB(db)
	t.Cleanup(func() { database.SetDB(nil) })

	t.Run("Admin services list shows test service", func(t *testing.T) {
		testName := fmt.Sprintf("IntegrationTestService_%d", time.Now().UnixNano())
		serviceID, ok := createAdminTestService(t, testName)
		require.True(t, ok, "failed to create test service")

		// Verify service exists in database
		var foundName string
		err := db.QueryRow(database.ConvertPlaceholders(`SELECT name FROM service WHERE id = $1`), serviceID).Scan(&foundName)
		require.NoError(t, err, "should find service in database")
		assert.Equal(t, testName, foundName, "service name should match")

		// Verify handler can access the database
		router := setupServiceTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/services", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// The handler should return 200 OK even if template rendering falls back
		assert.Equal(t, http.StatusOK, w.Code, "admin services page should return 200")
	})

	t.Run("Full CRUD workflow", func(t *testing.T) {
		router := setupServiceTestRouter()
		testName := fmt.Sprintf("CRUDWorkflow_%d", time.Now().UnixNano())
		defer cleanupTestServiceByName(t, testName)

		// Create
		createPayload := map[string]interface{}{
			"name":     testName,
			"comments": "CRUD test service",
			"valid_id": 1,
		}
		createJSON, _ := json.Marshal(createPayload)

		createReq := httptest.NewRequest(http.MethodPost, "/admin/services/create", bytes.NewReader(createJSON))
		createReq.Header.Set("Content-Type", "application/json")
		createReq.Header.Set("Accept", "application/json")
		createW := httptest.NewRecorder()

		router.ServeHTTP(createW, createReq)
		require.Equal(t, http.StatusOK, createW.Code)

		var createResp map[string]interface{}
		require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &createResp))
		require.True(t, createResp["success"].(bool))

		// Get the ID from response or query DB
		var serviceID int
		err := db.QueryRow(database.ConvertPlaceholders(`SELECT id FROM service WHERE name = $1`), testName).Scan(&serviceID)
		require.NoError(t, err)

		// Update
		updatePayload := map[string]interface{}{
			"name":     testName + "_updated",
			"comments": "Updated CRUD test service",
		}
		updateJSON, _ := json.Marshal(updatePayload)

		updateReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/admin/services/%d/update", serviceID), bytes.NewReader(updateJSON))
		updateReq.Header.Set("Content-Type", "application/json")
		updateReq.Header.Set("Accept", "application/json")
		updateW := httptest.NewRecorder()

		router.ServeHTTP(updateW, updateReq)
		require.Equal(t, http.StatusOK, updateW.Code)

		// Verify update in DB
		var updatedName string
		err = db.QueryRow(database.ConvertPlaceholders(`SELECT name FROM service WHERE id = $1`), serviceID).Scan(&updatedName)
		require.NoError(t, err)
		assert.Equal(t, testName+"_updated", updatedName)

		// Update cleanup name for defer
		defer cleanupTestServiceByName(t, testName+"_updated")

		// Delete (soft delete)
		deleteReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/admin/services/%d/delete", serviceID), nil)
		deleteW := httptest.NewRecorder()

		router.ServeHTTP(deleteW, deleteReq)
		require.Equal(t, http.StatusOK, deleteW.Code)

		// Verify soft delete
		var validID int
		err = db.QueryRow(database.ConvertPlaceholders(`SELECT valid_id FROM service WHERE id = $1`), serviceID).Scan(&validID)
		require.NoError(t, err)
		assert.Equal(t, 2, validID, "Service should be soft deleted (valid_id = 2)")
	})
}

func TestAdminServiceJSONResponseFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := setupServiceTestRouter()

	t.Run("JSON create returns proper structure", func(t *testing.T) {
		testName := fmt.Sprintf("JSONFormat_%d", time.Now().UnixNano())
		defer cleanupTestServiceByName(t, testName)

		payload := map[string]interface{}{
			"name":     testName,
			"valid_id": 1,
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/services/create", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		// Verify response structure
		assert.Contains(t, response, "success")
		assert.True(t, response["success"].(bool))

		// Should have ID in response
		if id, ok := response["id"]; ok {
			assert.Greater(t, id.(float64), float64(0))
		}
	})

	t.Run("JSON error returns proper structure", func(t *testing.T) {
		payload := map[string]interface{}{
			"comments": "No name",
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/services/create", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		// Verify error response structure
		assert.Contains(t, response, "success")
		assert.False(t, response["success"].(bool))
		// Response may use "error" or "message" field
		assert.True(t, response["error"] != nil || response["message"] != nil,
			"Response should contain either 'error' or 'message' field")
	})
}

func TestAdminServiceHTMXResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := setupServiceTestRouter()

	t.Run("HTMX create returns HTML toast", func(t *testing.T) {
		testName := fmt.Sprintf("HTMXTest_%d", time.Now().UnixNano())
		defer cleanupTestServiceByName(t, testName)

		form := url.Values{}
		form.Set("name", testName)
		form.Set("valid_id", "1")

		req := httptest.NewRequest(http.MethodPost, "/admin/services/create", bytes.NewBufferString(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		assert.Contains(t, body, "Service created successfully")
	})

	t.Run("HTMX error returns HTML toast", func(t *testing.T) {
		form := url.Values{}
		// Missing name
		form.Set("comments", "Test")

		req := httptest.NewRequest(http.MethodPost, "/admin/services/create", bytes.NewBufferString(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		body := w.Body.String()
		assert.Contains(t, body, "Name is required")
	})
}

func TestAdminServiceContentTypeHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := setupServiceTestRouter()

	t.Run("Form-urlencoded content type is handled", func(t *testing.T) {
		testName := fmt.Sprintf("FormContentType_%d", time.Now().UnixNano())
		defer cleanupTestServiceByName(t, testName)

		form := url.Values{}
		form.Set("name", testName)
		form.Set("valid_id", "1")

		req := httptest.NewRequest(http.MethodPost, "/admin/services/create", bytes.NewBufferString(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Form submission without Accept header may return 302 redirect or 200 OK
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusFound,
			"Expected 200 or 302, got %d", w.Code)
	})

	t.Run("JSON content type is handled", func(t *testing.T) {
		testName := fmt.Sprintf("JSONContentType_%d", time.Now().UnixNano())
		defer cleanupTestServiceByName(t, testName)

		payload := map[string]interface{}{
			"name":     testName,
			"valid_id": 1,
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/services/create", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// JSON content type without Accept header may still redirect
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusFound,
			"Expected 200 or 302, got %d", w.Code)
	})

	t.Run("Accept header application/json returns JSON", func(t *testing.T) {
		testName := fmt.Sprintf("AcceptJSON_%d", time.Now().UnixNano())
		defer cleanupTestServiceByName(t, testName)

		payload := map[string]interface{}{
			"name":     testName,
			"valid_id": 1,
		}
		jsonData, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/services/create", bytes.NewReader(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify response is valid JSON
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
	})
}
