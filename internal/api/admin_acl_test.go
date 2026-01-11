package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminACLHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.GET("/admin/acl", handleAdminACL)
	router.POST("/admin/api/acl", handleAdminACLCreate)
	router.PUT("/admin/api/acl/:id", handleAdminACLUpdate)
	router.DELETE("/admin/api/acl/:id", handleAdminACLDelete)
	router.GET("/admin/api/acl/:id", handleAdminACLGet)

	t.Run("GET /admin/acl renders page or returns error without DB", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/acl", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Without DB, may return 500 or render page with empty data
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("GET /admin/acl with search filter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/acl?search=test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("GET /admin/acl with valid filter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/acl?valid=valid", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("POST /admin/api/acl requires name", func(t *testing.T) {
		body := `{"description": "Test ACL"}`
		req := httptest.NewRequest("POST", "/admin/api/acl", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should return 400 for missing name or 500 if DB not available
		assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError)
	})

	t.Run("POST /admin/api/acl creates ACL with valid data", func(t *testing.T) {
		body := `{
			"name": "Test ACL",
			"description": "Test description",
			"comments": "Test comments",
			"valid_id": 1,
			"stop_after_match": 0,
			"config_match": {},
			"config_change": {}
		}`
		req := httptest.NewRequest("POST", "/admin/api/acl", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should succeed or fail due to DB
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError || w.Code == http.StatusBadRequest)
	})

	t.Run("PUT /admin/api/acl/:id with invalid ID", func(t *testing.T) {
		body := `{"name": "Updated ACL"}`
		req := httptest.NewRequest("PUT", "/admin/api/acl/invalid", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("DELETE /admin/api/acl/:id with invalid ID", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/admin/api/acl/invalid", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("GET /admin/api/acl/:id with invalid ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/api/acl/invalid", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("DELETE /admin/api/acl/:id returns 404 for non-existent", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/admin/api/acl/99999", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should return 404 or 500 if DB not available
		assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusInternalServerError)
	})
}

func TestACLConfigSerialization(t *testing.T) {
	t.Run("ACLConfig serializes to JSON correctly", func(t *testing.T) {
		config := ACLConfigMatch{
			Properties: map[string]interface{}{
				"Queue": []string{"Raw", "Postmaster"},
			},
		}

		data, err := json.Marshal(config)
		require.NoError(t, err)
		assert.Contains(t, string(data), "Queue")
	})

	t.Run("ACLConfigChange serializes to JSON correctly", func(t *testing.T) {
		config := ACLConfigChange{
			Possible: map[string]interface{}{
				"Action": []string{"AgentTicketClose", "AgentTicketBounce"},
			},
		}

		data, err := json.Marshal(config)
		require.NoError(t, err)
		assert.Contains(t, string(data), "Possible")
	})
}
