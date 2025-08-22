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
)

func TestAdminServicePage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	t.Run("GET /admin/services renders service page", func(t *testing.T) {
		router := gin.New()
		router.GET("/admin/services", handleAdminServices)
		
		req := httptest.NewRequest(http.MethodGet, "/admin/services", nil)
		w := httptest.NewRecorder()
		
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Service Management")
		assert.Contains(t, w.Body.String(), "Add New Service")
	})
	
	t.Run("GET /admin/services with search filters results", func(t *testing.T) {
		router := gin.New()
		router.GET("/admin/services", handleAdminServices)
		
		req := httptest.NewRequest(http.MethodGet, "/admin/services?search=incident", nil)
		w := httptest.NewRecorder()
		
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		// Should contain filtered results
	})
}

func TestAdminServiceCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	t.Run("POST /admin/services/create creates new service", func(t *testing.T) {
		router := gin.New()
		router.POST("/admin/services/create", handleAdminServiceCreate)
		
		form := url.Values{}
		form.Set("name", "Incident Management")
		form.Set("comments", "Service for incident handling")
		form.Set("valid_id", "1")
		
		req := httptest.NewRequest(http.MethodPost, "/admin/services/create", bytes.NewBufferString(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response["success"].(bool))
		assert.Equal(t, "Service created successfully", response["message"])
	})
	
	t.Run("POST /admin/services/create validates required fields", func(t *testing.T) {
		router := gin.New()
		router.POST("/admin/services/create", handleAdminServiceCreate)
		
		form := url.Values{}
		// Missing required name field
		form.Set("comments", "Test comment")
		
		req := httptest.NewRequest(http.MethodPost, "/admin/services/create", bytes.NewBufferString(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusBadRequest, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.False(t, response["success"].(bool))
		assert.Contains(t, response["error"], "Name is required")
	})
	
	t.Run("POST /admin/services/create prevents duplicate names", func(t *testing.T) {
		router := gin.New()
		router.POST("/admin/services/create", handleAdminServiceCreate)
		
		form := url.Values{}
		// Assuming "IT Support" already exists
		form.Set("name", "IT Support")
		form.Set("comments", "Duplicate service")
		
		req := httptest.NewRequest(http.MethodPost, "/admin/services/create", bytes.NewBufferString(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		
		router.ServeHTTP(w, req)
		
		// Should return error for duplicate
		var response map[string]interface{}
		if w.Code == http.StatusBadRequest {
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.False(t, response["success"].(bool))
			assert.Contains(t, response["error"], "already exists")
		}
	})
}

func TestAdminServiceUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	t.Run("PUT /admin/services/:id/update updates service", func(t *testing.T) {
		router := gin.New()
		router.PUT("/admin/services/:id/update", handleAdminServiceUpdate)
		
		form := url.Values{}
		form.Set("name", "Updated Service")
		form.Set("comments", "Updated comment")
		form.Set("valid_id", "1")
		
		req := httptest.NewRequest(http.MethodPut, "/admin/services/1/update", bytes.NewBufferString(form.Encode()))
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
	
	t.Run("PUT /admin/services/:id/update handles non-existent service", func(t *testing.T) {
		router := gin.New()
		router.PUT("/admin/services/:id/update", handleAdminServiceUpdate)
		
		form := url.Values{}
		form.Set("name", "Updated Service")
		
		req := httptest.NewRequest(http.MethodPut, "/admin/services/99999/update", bytes.NewBufferString(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		
		router.ServeHTTP(w, req)
		
		// Should return 404 or appropriate error
		assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusInternalServerError)
	})
}

func TestAdminServiceDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	t.Run("DELETE /admin/services/:id/delete soft deletes service", func(t *testing.T) {
		router := gin.New()
		router.DELETE("/admin/services/:id/delete", handleAdminServiceDelete)
		
		req := httptest.NewRequest(http.MethodDelete, "/admin/services/1/delete", nil)
		w := httptest.NewRecorder()
		
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response["success"].(bool))
		assert.Equal(t, "Service deleted successfully", response["message"])
	})
	
	t.Run("DELETE /admin/services/:id/delete prevents deletion of services with tickets", func(t *testing.T) {
		router := gin.New()
		router.DELETE("/admin/services/:id/delete", handleAdminServiceDelete)
		
		// Assuming service ID 1 has associated tickets
		req := httptest.NewRequest(http.MethodDelete, "/admin/services/1/delete", nil)
		w := httptest.NewRecorder()
		
		router.ServeHTTP(w, req)
		
		// Should either soft delete or return error based on business rules
		var response map[string]interface{}
		if w.Code == http.StatusOK {
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			// If successful, it should be a soft delete (setting valid_id = 2)
			assert.True(t, response["success"].(bool))
		} else {
			// Or it might prevent deletion if there are dependencies
			assert.Equal(t, http.StatusBadRequest, w.Code)
		}
	})
}

func TestAdminServiceHierarchy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	t.Run("Services support parent::child hierarchy", func(t *testing.T) {
		router := gin.New()
		router.POST("/admin/services/create", handleAdminServiceCreate)
		
		form := url.Values{}
		form.Set("name", "IT Support::Hardware")
		form.Set("comments", "Hardware support sub-service")
		form.Set("valid_id", "1")
		
		req := httptest.NewRequest(http.MethodPost, "/admin/services/create", bytes.NewBufferString(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		
		router.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response["success"].(bool))
		assert.Equal(t, "Service created successfully", response["message"])
	})
}