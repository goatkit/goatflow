package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/goatkit/goatflow/internal/models"
)

func TestSystemMaintenanceModelStruct(t *testing.T) {
	t.Run("Basic fields", func(t *testing.T) {
		loginMsg := "Login message"
		notifyMsg := "Notify message"
		now := time.Now()

		m := &models.SystemMaintenance{
			ID:               1,
			StartDate:        time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC).Unix(),
			StopDate:         time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC).Unix(),
			Comments:         "Scheduled maintenance",
			LoginMessage:     &loginMsg,
			ShowLoginMessage: 1,
			NotifyMessage:    &notifyMsg,
			ValidID:          1,
			CreateTime:       now,
			CreateBy:         1,
			ChangeTime:       now,
			ChangeBy:         1,
		}

		assert.Equal(t, 1, m.ID)
		assert.Equal(t, "Scheduled maintenance", m.Comments)
		assert.Equal(t, "Login message", m.GetLoginMessage())
		assert.Equal(t, "Notify message", m.GetNotifyMessage())
		assert.True(t, m.ShowsLoginMessage())
		assert.True(t, m.IsValid())
		assert.Equal(t, 120, m.Duration()) // 2 hours = 120 minutes
	})

	t.Run("Formatted dates", func(t *testing.T) {
		m := &models.SystemMaintenance{
			StartDate: time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC).Unix(),
			StopDate:  time.Date(2025, 6, 15, 18, 0, 0, 0, time.UTC).Unix(),
		}

		assert.Equal(t, "2025-06-15 14:30", m.StartDateFormatted())
		assert.Equal(t, "2025-06-15 18:00", m.StopDateFormatted())
	})

	t.Run("Nil message fields", func(t *testing.T) {
		m := &models.SystemMaintenance{
			LoginMessage:  nil,
			NotifyMessage: nil,
		}

		assert.Equal(t, "", m.GetLoginMessage())
		assert.Equal(t, "", m.GetNotifyMessage())
	})
}

func TestSystemMaintenanceActiveStatus(t *testing.T) {
	now := time.Now()

	t.Run("Currently active", func(t *testing.T) {
		m := &models.SystemMaintenance{
			StartDate: now.Add(-30 * time.Minute).Unix(),
			StopDate:  now.Add(30 * time.Minute).Unix(),
			ValidID:   1,
		}
		assert.True(t, m.IsCurrentlyActive())
		assert.False(t, m.IsPast())
		assert.False(t, m.IsUpcoming(60))
	})

	t.Run("Upcoming within 30 minutes", func(t *testing.T) {
		m := &models.SystemMaintenance{
			StartDate: now.Add(20 * time.Minute).Unix(),
			StopDate:  now.Add(80 * time.Minute).Unix(),
			ValidID:   1,
		}
		assert.False(t, m.IsCurrentlyActive())
		assert.False(t, m.IsPast())
		assert.True(t, m.IsUpcoming(30))
	})

	t.Run("Upcoming but beyond threshold", func(t *testing.T) {
		m := &models.SystemMaintenance{
			StartDate: now.Add(2 * time.Hour).Unix(),
			StopDate:  now.Add(3 * time.Hour).Unix(),
			ValidID:   1,
		}
		assert.False(t, m.IsCurrentlyActive())
		assert.False(t, m.IsPast())
		assert.False(t, m.IsUpcoming(30))
		assert.True(t, m.IsUpcoming(150)) // Within 2.5 hours
	})

	t.Run("Past maintenance", func(t *testing.T) {
		m := &models.SystemMaintenance{
			StartDate: now.Add(-2 * time.Hour).Unix(),
			StopDate:  now.Add(-1 * time.Hour).Unix(),
			ValidID:   1,
		}
		assert.False(t, m.IsCurrentlyActive())
		assert.True(t, m.IsPast())
		assert.False(t, m.IsUpcoming(60))
	})

	t.Run("Invalid maintenance not active", func(t *testing.T) {
		m := &models.SystemMaintenance{
			StartDate: now.Add(-30 * time.Minute).Unix(),
			StopDate:  now.Add(30 * time.Minute).Unix(),
			ValidID:   2, // Invalid
		}
		assert.False(t, m.IsCurrentlyActive())
	})
}

func TestHandleCreateSystemMaintenanceValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Missing required fields", func(t *testing.T) {
		router := gin.New()
		router.POST("/admin/api/system-maintenance", handleCreateSystemMaintenance)

		body := `{}`
		req, _ := http.NewRequest("POST", "/admin/api/system-maintenance", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusBadRequest, resp.Code)

		var result map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &result)
		assert.False(t, result["success"].(bool))
		assert.Contains(t, result["error"], "required")
	})

	t.Run("Invalid start date format", func(t *testing.T) {
		router := gin.New()
		router.POST("/admin/api/system-maintenance", handleCreateSystemMaintenance)

		body := `{
			"start_date": "invalid-date",
			"stop_date": "2025-01-15T12:00",
			"comments": "Test maintenance"
		}`
		req, _ := http.NewRequest("POST", "/admin/api/system-maintenance", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusBadRequest, resp.Code)

		var result map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &result)
		assert.False(t, result["success"].(bool))
		assert.Contains(t, result["error"], "start date")
	})

	t.Run("Invalid stop date format", func(t *testing.T) {
		router := gin.New()
		router.POST("/admin/api/system-maintenance", handleCreateSystemMaintenance)

		body := `{
			"start_date": "2025-01-15T10:00",
			"stop_date": "invalid-date",
			"comments": "Test maintenance"
		}`
		req, _ := http.NewRequest("POST", "/admin/api/system-maintenance", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusBadRequest, resp.Code)

		var result map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &result)
		assert.False(t, result["success"].(bool))
		assert.Contains(t, result["error"], "stop date")
	})

	t.Run("Start date after stop date", func(t *testing.T) {
		router := gin.New()
		router.POST("/admin/api/system-maintenance", handleCreateSystemMaintenance)

		body := `{
			"start_date": "2025-01-15T14:00",
			"stop_date": "2025-01-15T10:00",
			"comments": "Test maintenance"
		}`
		req, _ := http.NewRequest("POST", "/admin/api/system-maintenance", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusBadRequest, resp.Code)

		var result map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &result)
		assert.False(t, result["success"].(bool))
		assert.Contains(t, result["error"], "before stop date")
	})

	t.Run("Comments too long", func(t *testing.T) {
		router := gin.New()
		router.POST("/admin/api/system-maintenance", handleCreateSystemMaintenance)

		longComments := strings.Repeat("a", 300) // 300 chars, max is 250
		body := `{
			"start_date": "2025-01-15T10:00",
			"stop_date": "2025-01-15T12:00",
			"comments": "` + longComments + `"
		}`
		req, _ := http.NewRequest("POST", "/admin/api/system-maintenance", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusBadRequest, resp.Code)

		var result map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &result)
		assert.False(t, result["success"].(bool))
		assert.Contains(t, result["error"], "250")
	})
}

func TestHandleUpdateSystemMaintenanceValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Invalid ID", func(t *testing.T) {
		router := gin.New()
		router.PUT("/admin/api/system-maintenance/:id", handleUpdateSystemMaintenance)

		body := `{"comments": "Updated"}`
		req, _ := http.NewRequest("PUT", "/admin/api/system-maintenance/invalid", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusBadRequest, resp.Code)

		var result map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &result)
		assert.False(t, result["success"].(bool))
		assert.Contains(t, result["error"], "Invalid")
	})
}

func TestHandleDeleteSystemMaintenanceValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Invalid ID", func(t *testing.T) {
		router := gin.New()
		router.DELETE("/admin/api/system-maintenance/:id", handleDeleteSystemMaintenance)

		req, _ := http.NewRequest("DELETE", "/admin/api/system-maintenance/abc", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusBadRequest, resp.Code)

		var result map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &result)
		assert.False(t, result["success"].(bool))
		assert.Contains(t, result["error"], "Invalid")
	})
}

func TestHandleGetSystemMaintenanceValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Invalid ID", func(t *testing.T) {
		router := gin.New()
		router.GET("/admin/api/system-maintenance/:id", handleGetSystemMaintenance)

		req, _ := http.NewRequest("GET", "/admin/api/system-maintenance/xyz", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		assert.Equal(t, http.StatusBadRequest, resp.Code)

		var result map[string]interface{}
		json.Unmarshal(resp.Body.Bytes(), &result)
		assert.False(t, result["success"].(bool))
		assert.Contains(t, result["error"], "Invalid")
	})
}

func TestSystemMaintenanceJSONSerialization(t *testing.T) {
	loginMsg := "Please log in later"
	notifyMsg := "Maintenance in progress"
	now := time.Now()

	m := &models.SystemMaintenance{
		ID:               42,
		StartDate:        1704067200,
		StopDate:         1704074400,
		Comments:         "Test comment",
		LoginMessage:     &loginMsg,
		ShowLoginMessage: 1,
		NotifyMessage:    &notifyMsg,
		ValidID:          1,
		CreateTime:       now,
		CreateBy:         1,
		ChangeTime:       now,
		ChangeBy:         2,
	}

	jsonBytes, err := json.Marshal(m)
	assert.NoError(t, err)

	var decoded models.SystemMaintenance
	err = json.Unmarshal(jsonBytes, &decoded)
	assert.NoError(t, err)

	assert.Equal(t, 42, decoded.ID)
	assert.Equal(t, int64(1704067200), decoded.StartDate)
	assert.Equal(t, int64(1704074400), decoded.StopDate)
	assert.Equal(t, "Test comment", decoded.Comments)
	assert.Equal(t, "Please log in later", *decoded.LoginMessage)
	assert.Equal(t, 1, decoded.ShowLoginMessage)
	assert.Equal(t, "Maintenance in progress", *decoded.NotifyMessage)
	assert.Equal(t, 1, decoded.ValidID)
}
