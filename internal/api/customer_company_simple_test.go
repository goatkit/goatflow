//go:build db

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCustomerCompanyJSONResponses tests JSON response formats without database dependencies
func TestCustomerCompanyJSONResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Update non-existent company returns proper JSON error", func(t *testing.T) {
		// Create a minimal router with just the customer company routes
		router := gin.New()

		// Register only the update route with a mock database (nil)
		router.POST("/admin/customer/companies/:id/edit", handleAdminUpdateCustomerCompany(nil))

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

		// Verify JSON error response format
		assert.Contains(t, w.Header().Get("Content-Type"), "application/json", "Should return JSON error response")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err, "Error response should be valid JSON")

		// Check response structure
		success, exists := response["success"]
		assert.True(t, exists, "Error response should contain 'success' field")
		assert.Equal(t, false, success, "Success should be false for not found")

		errorMsg, exists := response["error"]
		assert.True(t, exists, "Error response should contain 'error' field")
		assert.NotEmpty(t, errorMsg, "Error message should not be empty")

		// Verify the error message indicates the company was not found
		assert.Contains(t, errorMsg, "not found", "Error message should indicate company not found")
	})

	t.Run("Update with empty name returns proper JSON error", func(t *testing.T) {
		// Create a minimal router with just the customer company routes
		router := gin.New()

		// Register only the update route with a mock database (nil)
		router.POST("/admin/customer/companies/:id/edit", handleAdminUpdateCustomerCompany(nil))

		// Test data with empty name
		formData := url.Values{}
		formData.Set("name", "") // Empty name

		req := httptest.NewRequest(http.MethodPost, "/admin/customer/companies/ANY_ID/edit",
			bytes.NewBufferString(formData.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should return bad request
		assert.Equal(t, http.StatusBadRequest, w.Code, "Should return 400 for empty name")

		// Verify JSON error response format
		assert.Contains(t, w.Header().Get("Content-Type"), "application/json", "Should return JSON error response")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err, "Error response should be valid JSON")

		// Check response structure
		success, exists := response["success"]
		assert.True(t, exists, "Error response should contain 'success' field")
		assert.Equal(t, false, success, "Success should be false for validation errors")

		errorMsg, exists := response["error"]
		assert.True(t, exists, "Error response should contain 'error' field")
		assert.NotEmpty(t, errorMsg, "Error message should not be empty")

		// Verify the error message indicates name is required
		assert.Contains(t, errorMsg, "Name is required", "Error message should indicate name is required")
	})

	t.Run("Delete non-existent company returns proper JSON error", func(t *testing.T) {
		// Create a minimal router with just the customer company routes
		router := gin.New()

		// Register only the delete route with a mock database (nil)
		router.POST("/admin/customer/companies/:id/delete", handleAdminDeleteCustomerCompany(nil))

		req := httptest.NewRequest(http.MethodPost, "/admin/customer/companies/NONEXISTENT/delete", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should return not found
		assert.Equal(t, http.StatusNotFound, w.Code, "Should return 404 for non-existent company")

		// Verify JSON error response format
		assert.Contains(t, w.Header().Get("Content-Type"), "application/json", "Should return JSON error response")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err, "Error response should be valid JSON")

		// Check response structure
		success, exists := response["success"]
		assert.True(t, exists, "Error response should contain 'success' field")
		assert.Equal(t, false, success, "Success should be false for not found")

		errorMsg, exists := response["error"]
		assert.True(t, exists, "Error response should contain 'error' field")
		assert.NotEmpty(t, errorMsg, "Error message should not be empty")

		// Verify the error message indicates the company was not found
		assert.Contains(t, errorMsg, "not found", "Error message should indicate company not found")
	})

	t.Run("Activate non-existent company returns proper JSON error", func(t *testing.T) {
		// Create a minimal router with just the customer company routes
		router := gin.New()

		// Register only the activate route with a mock database (nil)
		router.POST("/admin/customer/companies/:id/activate", handleAdminActivateCustomerCompany(nil))

		req := httptest.NewRequest(http.MethodPost, "/admin/customer/companies/NONEXISTENT/activate", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should return not found
		assert.Equal(t, http.StatusNotFound, w.Code, "Should return 404 for non-existent company")

		// Verify JSON error response format
		assert.Contains(t, w.Header().Get("Content-Type"), "application/json", "Should return JSON error response")

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err, "Error response should be valid JSON")

		// Check response structure
		success, exists := response["success"]
		assert.True(t, exists, "Error response should contain 'success' field")
		assert.Equal(t, false, success, "Success should be false for not found")

		errorMsg, exists := response["error"]
		assert.True(t, exists, "Error response should contain 'error' field")
		assert.NotEmpty(t, errorMsg, "Error message should not be empty")

		// Verify the error message indicates the company was not found
		assert.Contains(t, errorMsg, "not found", "Error message should indicate company not found")
	})
}
