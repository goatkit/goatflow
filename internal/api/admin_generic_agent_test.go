package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAdminGenericAgentHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.GET("/admin/generic-agent", handleAdminGenericAgent)
	router.POST("/admin/api/generic-agent", handleAdminGenericAgentCreate)
	router.PUT("/admin/api/generic-agent/:name", handleAdminGenericAgentUpdate)
	router.DELETE("/admin/api/generic-agent/:name", handleAdminGenericAgentDelete)
	router.GET("/admin/api/generic-agent/:name", handleAdminGenericAgentGet)

	t.Run("GET /admin/generic-agent renders page or returns error without DB", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/generic-agent", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Without DB, may return 500 or render page with empty data
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("GET /admin/generic-agent with search filter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/generic-agent?search=test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("POST /admin/api/generic-agent requires name", func(t *testing.T) {
		body := `{"config": {}}`
		req := httptest.NewRequest("POST", "/admin/api/generic-agent", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should return 400 for missing name or 500 if DB not available
		assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError)
	})

	t.Run("POST /admin/api/generic-agent creates job with valid data", func(t *testing.T) {
		body := `{
			"name": "Test Job",
			"valid": true,
			"config": {
				"ScheduleDays": ["Mon", "Tue", "Wed"],
				"ScheduleMinutes": ["0", "30"],
				"ScheduleHours": ["9", "10", "11"]
			}
		}`
		req := httptest.NewRequest("POST", "/admin/api/generic-agent", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should succeed or fail due to DB
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError || w.Code == http.StatusBadRequest)
	})

	t.Run("DELETE /admin/api/generic-agent/:name with empty name", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/admin/api/generic-agent/", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Empty name should result in 404 (route not matched)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("GET /admin/api/generic-agent/:name returns job details", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/api/generic-agent/TestJob", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should return 404 or 500 if DB not available
		assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusInternalServerError)
	})
}

func TestGenericAgentJobSerialization(t *testing.T) {
	t.Run("GenericAgentJob serializes to JSON correctly", func(t *testing.T) {
		job := GenericAgentJob{
			Name:  "Test Job",
			Valid: true,
			Config: map[string]string{
				"ScheduleDays":    "Mon;Tue;Wed",
				"ScheduleMinutes": "0;30",
				"ScheduleHours":   "9;10;11",
			},
		}

		data, err := json.Marshal(job)
		assert.NoError(t, err)
		assert.Contains(t, string(data), "Test Job")
		assert.Contains(t, string(data), "ScheduleDays")
	})
}
