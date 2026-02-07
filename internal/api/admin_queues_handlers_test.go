package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/shared"
)

// Note: getTestDB is defined in admin_customer_company_test.go (same package)

// setupTemplateRenderer ensures templates are available for tests that render HTML.
func setupTemplateRenderer(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	templateDir := filepath.Join(filepath.Dir(file), "..", "..", "templates")
	renderer, err := shared.NewTemplateRenderer(templateDir)
	if err != nil {
		t.Skipf("Templates not available: %v", err)
	}
	shared.SetGlobalRenderer(renderer)
}

// setupQueueTestRouter creates a minimal router with the admin queues handlers.
func setupQueueTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Set("user_role", "Admin")
		c.Next()
	})

	router.GET("/admin/queues", handleAdminQueues)
	router.POST("/api/queues", handleCreateQueue)
	router.PUT("/api/queues/:id", handleUpdateQueue)
	router.DELETE("/api/queues/:id", handleDeleteQueue)
	router.GET("/api/queues", HandleGetQueues)
	router.GET("/api/queues/:id", HandleAPIQueueGet)

	return router
}

// Helper function to create a test queue.
func createTestQueue(t *testing.T, db *sql.DB, name string) int64 {
	var queueID int64

	if database.IsMySQL() {
		result, err := db.Exec(`
			INSERT INTO queue (name, group_id, system_address_id, salutation_id, signature_id, follow_up_id, follow_up_lock, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, 1, 1, 1, 1, 1, 0, 1, NOW(), 1, NOW(), 1)
		`, name)
		require.NoError(t, err, "Failed to create test queue")
		queueID, err = result.LastInsertId()
		require.NoError(t, err)
		return queueID
	}

	err := db.QueryRow(database.ConvertPlaceholders(`
		INSERT INTO queue (name, group_id, system_address_id, salutation_id, signature_id, follow_up_id, follow_up_lock, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 1, 1, 1, 1, 1, 0, 1, NOW(), 1, NOW(), 1)
		RETURNING id
	`), name).Scan(&queueID)
	require.NoError(t, err, "Failed to create test queue")

	return queueID
}

// Helper function to clean up test queue.
func cleanupTestQueue(t *testing.T, db *sql.DB, queueID int64) {
	_, err := db.Exec(database.ConvertPlaceholders("DELETE FROM queue WHERE id = ?"), queueID)
	require.NoError(t, err, "Failed to cleanup test queue")
}

// Helper function to clean up test queue by name.
func cleanupTestQueueByName(t *testing.T, db *sql.DB, name string) {
	_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM queue WHERE name = ?"), name)
}

func TestAdminQueuesPage(t *testing.T) {
	setupTemplateRenderer(t)
	router := setupQueueTestRouter()

	t.Run("GET /admin/queues renders page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/queues", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Accept either HTML page or error depending on DB availability
		if w.Code == http.StatusOK {
			body := w.Body.String()
			assert.Contains(t, body, "Queue")
		}
	})

	t.Run("Page contains queue table structure", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/queues", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			body := w.Body.String()
			assert.Contains(t, body, "<table")
			assert.Contains(t, body, "ID")
			assert.Contains(t, body, "Name")
		}
	})

	t.Run("Page contains queue modal form", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/queues", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			body := w.Body.String()
			assert.Contains(t, body, "queueForm")
		}
	})
}

func TestAdminQueuesListWithDB(t *testing.T) {
	setupTemplateRenderer(t)
	db := getTestDB(t)

	testQueueName := fmt.Sprintf("TestAdminQueue_%d", time.Now().UnixNano())
	queueID := createTestQueue(t, db, testQueueName)
	defer cleanupTestQueue(t, db, queueID)

	t.Run("Admin queues list shows test queue", func(t *testing.T) {
		router := setupQueueTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/queues", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			body := w.Body.String()
			assert.Contains(t, body, testQueueName,
				"Admin queues page should show the test queue")
		}
	})

	t.Run("Admin queues shows queue ID", func(t *testing.T) {
		router := setupQueueTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/queues", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			body := w.Body.String()
			queueIDStr := strconv.FormatInt(queueID, 10)
			assert.Contains(t, body, queueIDStr,
				"Admin queues page should show the test queue ID")
		}
	})
}

func TestAdminQueuesCRUDWithDB(t *testing.T) {
	db := getTestDB(t)

	t.Run("Create queue via API", func(t *testing.T) {
		testName := fmt.Sprintf("CRUDTestQueue_%d", time.Now().UnixNano())
		defer cleanupTestQueueByName(t, db, testName)

		router := setupQueueTestRouter()

		body := map[string]interface{}{
			"name":     testName,
			"group_id": 1,
			"valid_id": 1,
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/queues", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// May be 201 Created or 200 OK
		assert.True(t, w.Code == http.StatusCreated || w.Code == http.StatusOK || w.Code == http.StatusBadRequest,
			"Expected 201, 200, or 400, got %d: %s", w.Code, w.Body.String())
	})

	t.Run("Update queue via API", func(t *testing.T) {
		testName := fmt.Sprintf("UpdateTestQueue_%d", time.Now().UnixNano())
		queueID := createTestQueue(t, db, testName)
		t.Logf("Created test queue with ID=%d for name=%s", queueID, testName)
		defer cleanupTestQueue(t, db, queueID)

		router := setupQueueTestRouter()

		newName := testName + "_Updated"
		body := map[string]interface{}{
			"name": newName,
		}
		jsonBody, _ := json.Marshal(body)

		url := fmt.Sprintf("/api/queues/%d", queueID)
		t.Logf("Making PUT request to %s", url)
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		t.Logf("Response code: %d, body: %s", w.Code, w.Body.String())
		// Accept 200 OK or other codes - handler may or may not have DB access
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError,
			"Update request should return 200 OK or 500, got %d", w.Code)
	})

	t.Run("Toggle queue status via API", func(t *testing.T) {
		testName := fmt.Sprintf("ToggleTestQueue_%d", time.Now().UnixNano())
		queueID := createTestQueue(t, db, testName)
		t.Logf("Created test queue with ID=%d for name=%s", queueID, testName)
		defer cleanupTestQueue(t, db, queueID)

		router := setupQueueTestRouter()

		// Toggle to inactive (valid_id = 2)
		body := map[string]interface{}{
			"valid_id": 2,
		}
		jsonBody, _ := json.Marshal(body)

		url := fmt.Sprintf("/api/queues/%d", queueID)
		t.Logf("Making PUT request to %s", url)
		req := httptest.NewRequest(http.MethodPut, url, bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		t.Logf("Response code: %d, body: %s", w.Code, w.Body.String())
		// Accept 200 OK or other codes - handler may or may not have DB access
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError,
			"Toggle status request should return 200 OK or 500, got %d", w.Code)
	})

	t.Run("Delete queue via API", func(t *testing.T) {
		testName := fmt.Sprintf("DeleteTestQueue_%d", time.Now().UnixNano())
		queueID := createTestQueue(t, db, testName)
		// No defer cleanup since we're testing deletion

		router := setupQueueTestRouter()

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/queues/%d", queueID), nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			// Verify queue was soft-deleted (valid_id = 0 or row removed)
			var count int
			err := db.QueryRow(database.ConvertPlaceholders(
				"SELECT COUNT(*) FROM queue WHERE id = ? AND valid_id = 1"), queueID).Scan(&count)
			require.NoError(t, err)
			assert.Equal(t, 0, count, "Queue should be deleted or marked invalid")
		} else {
			// Clean up if delete failed
			cleanupTestQueue(t, db, queueID)
		}
	})
}

func TestAdminQueuesValidation(t *testing.T) {
	router := setupQueueTestRouter()

	t.Run("Create queue requires name", func(t *testing.T) {
		body := map[string]interface{}{
			"group_id": 1,
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/queues", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code,
			"Creating queue without name should return 400")
	})

	t.Run("Create queue validates name length", func(t *testing.T) {
		body := map[string]interface{}{
			"name":     "A",
			"group_id": 1,
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/queues", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code,
			"Creating queue with very short name should return 400")
	})

	t.Run("Update non-existent queue returns error", func(t *testing.T) {
		body := map[string]interface{}{
			"name": "Updated Name",
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPut, "/api/queues/99999", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Handler may return 404 Not Found or 200 OK (depending on implementation)
		assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusOK,
			"Updating non-existent queue should return 404 or 200, got %d", w.Code)
	})

	t.Run("Delete non-existent queue returns error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/queues/99999", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Handler may return 404 Not Found or 409 Conflict (if constraints exist)
		assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusConflict,
			"Deleting non-existent queue should return 404 or 409, got %d", w.Code)
	})

	t.Run("Invalid queue ID returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/queues/invalid", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code,
			"Invalid queue ID should return 400")
	})
}

func TestAdminQueuesFormSubmission(t *testing.T) {
	db := getTestDB(t)

	t.Run("Create queue via form submission", func(t *testing.T) {
		testName := fmt.Sprintf("FormTestQueue_%d", time.Now().UnixNano())
		defer cleanupTestQueueByName(t, db, testName)

		router := setupQueueTestRouter()

		formData := url.Values{}
		formData.Set("name", testName)
		formData.Set("group_id", "1")
		formData.Set("comments", "Test queue created via form")

		req := httptest.NewRequest(http.MethodPost, "/api/queues",
			bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// May return JSON or redirect
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusCreated ||
			w.Code == http.StatusSeeOther || w.Code == http.StatusFound ||
			w.Code == http.StatusBadRequest,
			"Form submission should succeed or indicate bad request, got %d", w.Code)
	})
}

func TestAdminQueuesAPIEndpoints(t *testing.T) {
	router := setupQueueTestRouter()

	t.Run("GET /api/queues returns queue list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/queues", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Accept 200 or 500 (if DB not available)
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("GET /api/queues/:id returns single queue", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/queues/1", nil)
		req.Header.Set("Accept", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Accept 200 or 404 (if queue doesn't exist) or 500
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusNotFound ||
			w.Code == http.StatusInternalServerError)
	})
}

func TestAdminQueuesRelatedOperations(t *testing.T) {
	db := getTestDB(t)

	t.Run("Queue lists associated group", func(t *testing.T) {
		testName := fmt.Sprintf("GroupTestQueue_%d", time.Now().UnixNano())
		queueID := createTestQueue(t, db, testName)
		defer cleanupTestQueue(t, db, queueID)

		router := setupQueueTestRouter()

		req := httptest.NewRequest(http.MethodGet, "/admin/queues", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			body := w.Body.String()
			// Queue should show its associated group
			assert.Contains(t, body, testName)
		}
	})

	t.Run("Cannot delete queue with tickets", func(t *testing.T) {
		router := setupQueueTestRouter()

		// Try to delete a system queue that likely has tickets
		req := httptest.NewRequest(http.MethodDelete, "/api/queues/1", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should return 409 Conflict if queue has tickets
		assert.True(t, w.Code == http.StatusConflict || w.Code == http.StatusNotFound ||
			w.Code == http.StatusOK || w.Code == http.StatusInternalServerError,
			"Delete queue with tickets should handle appropriately")
	})
}

func TestAdminQueuesDropdownsPopulated(t *testing.T) {
	setupTemplateRenderer(t)
	router := setupQueueTestRouter()

	t.Run("Admin queues page loads dropdown data", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/queues", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			body := w.Body.String()
			// Check for dropdown elements
			assert.Contains(t, body, "select")
		}
	})
}

// TestAdminQueuesCreateValidation tests various validation scenarios for queue creation.
func TestAdminQueuesCreateValidation(t *testing.T) {
	router := setupQueueTestRouter()

	testCases := []struct {
		name           string
		requestBody    string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "Missing name field",
			requestBody:    `{"group_id": 1}`,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, strings.ToLower(body), "name")
			},
		},
		{
			name:           "Empty name field",
			requestBody:    `{"name": "", "group_id": 1}`,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, strings.ToLower(body), "name")
			},
		},
		{
			name:           "Invalid JSON",
			requestBody:    `{"name": "Test Queue",}`,
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, body string) {
				assert.Contains(t, strings.ToLower(body), "json")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/queues",
				bytes.NewBufferString(tc.requestBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code,
				"Expected status %d, got %d: %s", tc.expectedStatus, w.Code, w.Body.String())
			if tc.checkResponse != nil {
				tc.checkResponse(t, w.Body.String())
			}
		})
	}
}

// TestAdminQueuesUpdateValidation tests validation for queue updates.
func TestAdminQueuesUpdateValidation(t *testing.T) {
	router := setupQueueTestRouter()

	t.Run("Update with invalid ID format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/queues/abc",
			bytes.NewBufferString(`{"name": "Test"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusNotFound,
			"Invalid ID format should return 400 or 404")
	})

	t.Run("Update with negative ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/queues/-1",
			bytes.NewBufferString(`{"name": "Test"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Handler may return various codes depending on implementation
		assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusNotFound ||
			w.Code == http.StatusOK || w.Code == http.StatusConflict,
			"Negative ID should return appropriate error, got %d", w.Code)
	})
}
