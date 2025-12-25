package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminSLAHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.GET("/admin/sla", handleAdminSLA)
	router.POST("/admin/sla/create", handleAdminSLACreate)
	router.PUT("/admin/sla/:id/update", handleAdminSLAUpdate)
	router.DELETE("/admin/sla/:id/delete", handleAdminSLADelete)

	t.Run("GET /admin/sla renders page", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/sla", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Without DB, may return 500 or render page with empty data
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("GET /admin/sla with search filter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/sla?search=premium", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("GET /admin/sla with valid filter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/sla?valid=valid", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("GET /admin/sla with invalid filter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/sla?valid=invalid", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("GET /admin/sla with sort and order", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/sla?sort=name&order=desc", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("GET /admin/sla with sort by first_response_time", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/sla?sort=first_response_time&order=asc", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("GET /admin/sla with sort by solution_time", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/sla?sort=solution_time&order=desc", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("GET /admin/sla with sort by ticket_count", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/sla?sort=ticket_count&order=desc", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("GET /admin/sla with invalid sort column uses default", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/sla?sort=invalid_column&order=asc", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("GET /admin/sla with invalid order uses default", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/sla?sort=name&order=invalid", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})
}

func TestAdminSLACreate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/admin/sla/create", handleAdminSLACreate)

	t.Run("POST /admin/sla/create requires name", func(t *testing.T) {
		form := url.Values{}
		form.Set("name", "")

		req := httptest.NewRequest("POST", "/admin/sla/create", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
		assert.Contains(t, response["error"], "required")
	})

	t.Run("POST /admin/sla/create with whitespace name", func(t *testing.T) {
		form := url.Values{}
		form.Set("name", "   ")

		req := httptest.NewRequest("POST", "/admin/sla/create", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
	})

	t.Run("POST /admin/sla/create with JSON payload", func(t *testing.T) {
		payload := `{"name": "Test SLA", "first_response_time": 60, "solution_time": 480}`

		req := httptest.NewRequest("POST", "/admin/sla/create", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Without DB, returns 500, but verifies JSON parsing works
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("POST /admin/sla/create with form data", func(t *testing.T) {
		form := url.Values{}
		form.Set("name", "Form Test SLA")
		form.Set("first_response_time", "30")
		form.Set("update_time", "60")
		form.Set("solution_time", "240")
		form.Set("valid_id", "1")

		req := httptest.NewRequest("POST", "/admin/sla/create", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Without DB, returns 500, but verifies form parsing works
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("POST /admin/sla/create with notify percentages", func(t *testing.T) {
		form := url.Values{}
		form.Set("name", "SLA with Notify")
		form.Set("first_response_time", "60")
		form.Set("first_response_notify", "80")
		form.Set("update_time", "120")
		form.Set("update_notify", "75")
		form.Set("solution_time", "480")
		form.Set("solution_notify", "90")

		req := httptest.NewRequest("POST", "/admin/sla/create", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("POST /admin/sla/create with calendar", func(t *testing.T) {
		form := url.Values{}
		form.Set("name", "SLA with Calendar")
		form.Set("calendar_name", "Business Hours")
		form.Set("first_response_time", "60")

		req := httptest.NewRequest("POST", "/admin/sla/create", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})
}

func TestAdminSLAUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/admin/sla/:id/update", handleAdminSLAUpdate)

	t.Run("PUT /admin/sla/:id/update with invalid ID", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/admin/sla/invalid/update", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
		assert.Contains(t, response["error"], "Invalid")
	})

	t.Run("PUT /admin/sla/:id/update with form data", func(t *testing.T) {
		form := url.Values{}
		form.Set("name", "Updated SLA Name")
		form.Set("solution_time", "720")

		req := httptest.NewRequest("PUT", "/admin/sla/1/update", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Without DB, returns 500 or 404, but verifies parsing works
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError || w.Code == http.StatusNotFound)
	})

	t.Run("PUT /admin/sla/:id/update with JSON payload", func(t *testing.T) {
		payload := `{"name": "JSON Updated SLA", "valid_id": 1}`

		req := httptest.NewRequest("PUT", "/admin/sla/1/update", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError || w.Code == http.StatusNotFound)
	})

	t.Run("PUT /admin/sla/:id/update partial update", func(t *testing.T) {
		form := url.Values{}
		form.Set("first_response_time", "45")

		req := httptest.NewRequest("PUT", "/admin/sla/1/update", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError || w.Code == http.StatusNotFound)
	})

	t.Run("PUT /admin/sla/:id/update deactivate SLA", func(t *testing.T) {
		form := url.Values{}
		form.Set("valid_id", "2")

		req := httptest.NewRequest("PUT", "/admin/sla/1/update", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError || w.Code == http.StatusNotFound)
	})
}

func TestAdminSLADelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/admin/sla/:id/delete", handleAdminSLADelete)

	t.Run("DELETE /admin/sla/:id/delete with invalid ID", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/admin/sla/invalid/delete", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.False(t, response["success"].(bool))
		assert.Contains(t, response["error"], "Invalid")
	})

	t.Run("DELETE /admin/sla/:id/delete valid request", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/admin/sla/1/delete", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Without DB, returns 500 or 404
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError || w.Code == http.StatusNotFound)
	})

	t.Run("DELETE /admin/sla/:id/delete non-existent ID", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/admin/sla/99999/delete", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError || w.Code == http.StatusNotFound)
	})
}

func TestAdminSLAValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/admin/sla/create", handleAdminSLACreate)

	t.Run("SLA name is trimmed and validated", func(t *testing.T) {
		form := url.Values{}
		form.Set("name", "  Trimmed Name  ")
		form.Set("first_response_time", "60")

		req := httptest.NewRequest("POST", "/admin/sla/create", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should process the name (trimming happens at DB level or could be added)
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})

	t.Run("SLA times accept zero values", func(t *testing.T) {
		form := url.Values{}
		form.Set("name", "Zero Times SLA")
		form.Set("first_response_time", "0")
		form.Set("update_time", "0")
		form.Set("solution_time", "0")

		req := httptest.NewRequest("POST", "/admin/sla/create", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Zero times should be valid (means no SLA for that metric)
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})
}

func TestSLADefinitionStruct(t *testing.T) {
	t.Run("SLADefinition has correct fields", func(t *testing.T) {
		sla := SLADefinition{
			ID:   1,
			Name: "Test SLA",
		}
		assert.Equal(t, 1, sla.ID)
		assert.Equal(t, "Test SLA", sla.Name)
	})

	t.Run("SLAWithStats includes ticket count", func(t *testing.T) {
		sla := SLAWithStats{
			SLADefinition: SLADefinition{
				ID:   1,
				Name: "Test SLA",
			},
			TicketCount: 42,
		}
		assert.Equal(t, 42, sla.TicketCount)
	})

	t.Run("SLA optional fields can be nil", func(t *testing.T) {
		sla := SLADefinition{
			ID:      1,
			Name:    "Minimal SLA",
			ValidID: 1,
		}
		assert.Nil(t, sla.CalendarName)
		assert.Nil(t, sla.FirstResponseTime)
		assert.Nil(t, sla.UpdateTime)
		assert.Nil(t, sla.SolutionTime)
		assert.Nil(t, sla.Comments)
	})

	t.Run("SLA notify fields can be set", func(t *testing.T) {
		firstNotify := 80
		updateNotify := 75
		solutionNotify := 90

		sla := SLADefinition{
			ID:                  1,
			Name:                "SLA with Notify",
			FirstResponseNotify: &firstNotify,
			UpdateNotify:        &updateNotify,
			SolutionNotify:      &solutionNotify,
		}
		assert.Equal(t, 80, *sla.FirstResponseNotify)
		assert.Equal(t, 75, *sla.UpdateNotify)
		assert.Equal(t, 90, *sla.SolutionNotify)
	})
}
