package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminStateHandlers(t *testing.T) {
	// Set up test router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Register routes
	router.GET("/admin/states", handleAdminStates)
	router.POST("/admin/states/create", handleAdminStateCreate)
	router.POST("/admin/states/:id/update", handleAdminStateUpdate)
	router.POST("/admin/states/:id/delete", handleAdminStateDelete)
	router.GET("/admin/states/types", handleGetStateTypes)

	t.Run("GET /admin/states - renders admin states page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/states", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "State Management")
		assert.Contains(t, w.Body.String(), "data-states-table")
		assert.Contains(t, w.Body.String(), "Add New State")
	})

	t.Run("POST /admin/states/create - creates new state", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":     "test-state",
			"type_id":  2,
			"comments": "Test state for unit tests",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/states/create", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})

	t.Run("POST /admin/states/:id/update - updates existing state", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":     "updated-state",
			"type_id":  3,
			"comments": "Updated comment",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/states/1/update", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})

	t.Run("POST /admin/states/:id/delete - soft deletes state", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/states/1/delete", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
	})

	t.Run("GET /admin/states/types - returns state types", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/states/types", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))
		assert.NotNil(t, response["data"])
	})
}

func TestAdminStateUI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin/states", handleAdminStates)

	t.Run("UI contains required elements", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/states", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		body := w.Body.String()

		// Check for essential UI elements
		assert.Contains(t, body, "id=\"searchInput\"", "Search input must be present")
		assert.Contains(t, body, "id=\"clearSearchBtn\"", "Clear search button must be present")
		assert.Contains(t, body, "id=\"addStateBtn\"", "Add state button must be present")
		assert.Contains(t, body, "id=\"stateModal\"", "State modal must be present")
		assert.Contains(t, body, "id=\"deleteModal\"", "Delete confirmation modal must be present")
		
		// Check for table headers
		assert.Contains(t, body, "Name", "Name column header must be present")
		assert.Contains(t, body, "Type", "Type column header must be present")
		assert.Contains(t, body, "Comments", "Comments column header must be present")
		assert.Contains(t, body, "Actions", "Actions column header must be present")
		
		// Check for dark mode support
		assert.Contains(t, body, "dark:bg-gray-800", "Dark mode classes must be present")
		
		// Check for accessibility
		assert.Contains(t, body, "aria-label", "ARIA labels must be present")
		assert.Contains(t, body, "role=\"button\"", "Button roles must be defined")
	})

	t.Run("UI supports session storage", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/states", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		body := w.Body.String()

		// Check for session storage JavaScript
		assert.Contains(t, body, "sessionStorage.setItem", "Session storage save must be present")
		assert.Contains(t, body, "sessionStorage.getItem", "Session storage restore must be present")
		assert.Contains(t, body, "restoreSearchState", "Search state restoration function must be present")
	})

	t.Run("UI has proper form validation", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/states", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		body := w.Body.String()

		// Check for form validation
		assert.Contains(t, body, "required", "Required field validation must be present")
		assert.Contains(t, body, "validateForm", "Form validation function must be present")
		assert.Contains(t, body, "showError", "Error display function must be present")
		assert.Contains(t, body, "showSuccess", "Success display function must be present")
	})

	t.Run("UI has keyboard support", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/states", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		body := w.Body.String()

		// Check for keyboard event handlers
		assert.Contains(t, body, "addEventListener('keydown'", "Keyboard event listeners must be present")
		assert.Contains(t, body, "event.key === 'Enter'", "Enter key handling must be present")
		assert.Contains(t, body, "event.key === 'Escape'", "Escape key handling must be present")
	})
}

func TestAdminStateValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/admin/states/create", handleAdminStateCreate)

	t.Run("Validates required fields", func(t *testing.T) {
		// Missing name
		payload := map[string]interface{}{
			"type_id": 2,
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/states/create", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Name is required")
	})

	t.Run("Validates unique state names", func(t *testing.T) {
		// Try to create duplicate state name
		payload := map[string]interface{}{
			"name":    "new", // This should already exist
			"type_id": 1,
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/states/create", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), "already exists")
	})

	t.Run("Validates type_id exists", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":    "test-invalid-type",
			"type_id": 999, // Invalid type ID
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/admin/states/create", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid state type")
	})
}

func TestAdminStateSearch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin/states", handleAdminStates)

	t.Run("Filters states by search query", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/states?search=open", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		
		// Should contain states with "open" in the name
		assert.Contains(t, body, "open")
		// Should not contain unrelated states
		assert.NotContains(t, body, "merged")
	})

	t.Run("Filters states by type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/states?type=2", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// Should only show states of type 2 (open)
	})

	t.Run("Sorts states", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/states?sort=name&order=desc", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		// States should be sorted by name in descending order
	})
}

func TestAdminStateIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	// Register all state-related routes
	router.GET("/admin/states", handleAdminStates)
	router.POST("/admin/states/create", handleAdminStateCreate)
	router.POST("/admin/states/:id/update", handleAdminStateUpdate)
	router.POST("/admin/states/:id/delete", handleAdminStateDelete)
	router.GET("/api/states", handleGetStates)

	t.Run("Complete CRUD workflow", func(t *testing.T) {
		// 1. Create a new state
		createPayload := map[string]interface{}{
			"name":     "integration-test-state",
			"type_id":  2,
			"comments": "Created for integration test",
		}
		body, _ := json.Marshal(createPayload)
		
		req := httptest.NewRequest(http.MethodPost, "/admin/states/create", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		require.Equal(t, http.StatusCreated, w.Code)
		
		var createResponse map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &createResponse)
		stateID := int(createResponse["data"].(map[string]interface{})["id"].(float64))
		
		// 2. Update the state
		updatePayload := map[string]interface{}{
			"name":     "updated-integration-state",
			"comments": "Updated during integration test",
		}
		body, _ = json.Marshal(updatePayload)
		
		req = httptest.NewRequest(http.MethodPost, "/admin/states/"+string(stateID)+"/update", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		// 3. Verify state appears in list
		req = httptest.NewRequest(http.MethodGet, "/api/states", nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "updated-integration-state")
		
		// 4. Delete the state
		req = httptest.NewRequest(http.MethodPost, "/admin/states/"+string(stateID)+"/delete", nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		// 5. Verify state is soft-deleted (not in active list)
		req = httptest.NewRequest(http.MethodGet, "/api/states", nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotContains(t, w.Body.String(), "updated-integration-state")
	})
}