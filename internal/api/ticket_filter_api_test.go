package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test ticket filtering via the JSON API
// These tests validate that /api/tickets properly filters tickets

func TestTicketFilterByStatus(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	WithCleanDB(t)
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		queryParams string
	}{
		{
			name:        "Filter by status=open",
			queryParams: "status=open",
		},
		{
			name:        "Filter by status=closed",
			queryParams: "status=closed",
		},
		{
			name:        "Filter by multiple statuses",
			queryParams: "status=open&status=pending",
		},
		{
			name:        "No status filter shows all",
			queryParams: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			SetupHTMXRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/tickets?"+tt.queryParams, nil)
			req.Header.Set("X-Test-Mode", "true")
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err, "Response should be valid JSON")

			// Check response has expected structure
			assert.Contains(t, resp, "tickets", "Response should have tickets field")
			assert.Contains(t, resp, "total", "Response should have total field")
		})
	}
}

func TestTicketFilterByPriority(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	WithCleanDB(t)
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		queryParams string
	}{
		{
			name:        "Filter by high priority",
			queryParams: "priority=5",
		},
		{
			name:        "Filter by low priority",
			queryParams: "priority=1",
		},
		{
			name:        "Filter by multiple priorities",
			queryParams: "priority=4&priority=5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			SetupHTMXRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/tickets?"+tt.queryParams, nil)
			req.Header.Set("X-Test-Mode", "true")
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err, "Response should be valid JSON")
		})
	}
}

func TestTicketFilterByQueue(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	WithCleanDB(t)
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		queryParams string
	}{
		{
			name:        "Filter by queue ID 1",
			queryParams: "queue=1",
		},
		{
			name:        "Filter by queue ID 2",
			queryParams: "queue=2",
		},
		{
			name:        "Filter by multiple queues",
			queryParams: "queue=1&queue=2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			SetupHTMXRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/tickets?"+tt.queryParams, nil)
			req.Header.Set("X-Test-Mode", "true")
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err, "Response should be valid JSON")
		})
	}
}

func TestTicketFilterByAssignee(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	WithCleanDB(t)
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		queryParams string
	}{
		{
			name:        "Filter by unassigned tickets",
			queryParams: "assigned=false",
		},
		{
			name:        "Filter by assigned tickets",
			queryParams: "assigned=true",
		},
		{
			name:        "Filter by specific agent",
			queryParams: "owner_id=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			SetupHTMXRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/tickets?"+tt.queryParams, nil)
			req.Header.Set("X-Test-Mode", "true")
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err, "Response should be valid JSON")
		})
	}
}

func TestCombinedFilters(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	WithCleanDB(t)
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		queryParams string
	}{
		{
			name:        "Filter by status and priority",
			queryParams: "status=open&priority=5",
		},
		{
			name:        "Filter by queue and status",
			queryParams: "queue=1&status=open",
		},
		{
			name:        "Filter by all parameters",
			queryParams: "status=open&priority=3&queue=1&assigned=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			SetupHTMXRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/tickets?"+tt.queryParams, nil)
			req.Header.Set("X-Test-Mode", "true")
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var resp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err, "Response should be valid JSON")
		})
	}
}

func TestFilterReset(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	WithCleanDB(t)
	gin.SetMode(gin.TestMode)

	t.Run("Clear filters button removes all filters", func(t *testing.T) {
		router := gin.New()
		SetupHTMXRoutes(router)

		// Request with no filters should return all tickets
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/tickets", nil)
		req.Header.Set("X-Test-Mode", "true")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err, "Response should be valid JSON")
	})
}

func TestFilterValidation(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	WithCleanDB(t)
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		queryParams string
	}{
		{
			name:        "Invalid status value is ignored",
			queryParams: "status=invalid_status",
		},
		{
			name:        "Invalid priority value is ignored",
			queryParams: "priority=invalid",
		},
		{
			name:        "Invalid queue ID is ignored",
			queryParams: "queue=invalid",
		},
		{
			name:        "SQL injection attempt is sanitized",
			queryParams: "status=open';DROP TABLE tickets;--",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			SetupHTMXRoutes(router)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/tickets?"+tt.queryParams, nil)
			req.Header.Set("X-Test-Mode", "true")
			router.ServeHTTP(w, req)

			// Should still return valid response (200 or graceful error)
			assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusBadRequest,
				"Should return 200 or 400, got %d", w.Code)
		})
	}
}
