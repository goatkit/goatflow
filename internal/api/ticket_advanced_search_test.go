package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test-Driven Development for Advanced Ticket Search
// This feature enables full-text search, filtering, and advanced queries

func TestAdvancedTicketSearch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		query      url.Values
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name: "Simple text search",
			query: url.Values{
				"q": []string{"network issue"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				results := resp["results"].([]interface{})
				assert.Greater(t, len(results), 0)
				assert.Contains(t, resp, "total")
				assert.Contains(t, resp, "took_ms")
			},
		},
		{
			name: "Search with status filter",
			query: url.Values{
				"q":      []string{"urgent"},
				"status": []string{"open"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				results := resp["results"].([]interface{})
				for _, r := range results {
					ticket := r.(map[string]interface{})
					assert.Equal(t, "open", ticket["status"])
				}
			},
		},
		{
			name: "Search with priority filter",
			query: url.Values{
				"q":        []string{"*"},
				"priority": []string{"high"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				results := resp["results"].([]interface{})
				for _, r := range results {
					ticket := r.(map[string]interface{})
					assert.Equal(t, "high", ticket["priority"])
				}
			},
		},
		{
			name: "Search with date range",
			query: url.Values{
				"q":            []string{"*"},
				"created_from": []string{time.Now().Add(-7 * 24 * time.Hour).Format("2006-01-02")},
				"created_to":   []string{time.Now().Format("2006-01-02")},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				results := resp["results"].([]interface{})
				for _, r := range results {
					ticket := r.(map[string]interface{})
					createdAt, _ := time.Parse(time.RFC3339, ticket["created_at"].(string))
					assert.WithinDuration(t, time.Now(), createdAt, 7*24*time.Hour)
				}
			},
		},
		{
			name: "Search by queue",
			query: url.Values{
				"q":     []string{"*"},
				"queue": []string{"1"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				results := resp["results"].([]interface{})
				for _, r := range results {
					ticket := r.(map[string]interface{})
					assert.Equal(t, float64(1), ticket["queue_id"])
				}
			},
		},
		{
			name: "Search by assignee",
			query: url.Values{
				"q":        []string{"*"},
				"assignee": []string{"1"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				results := resp["results"].([]interface{})
				for _, r := range results {
					ticket := r.(map[string]interface{})
					assert.Equal(t, float64(1), ticket["assignee_id"])
				}
			},
		},
		{
			name: "Search with multiple filters",
			query: url.Values{
				"q":        []string{"email"},
				"status":   []string{"open,pending"},
				"priority": []string{"high,critical"},
				"queue":    []string{"1,2"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				results := resp["results"].([]interface{})
				for _, r := range results {
					ticket := r.(map[string]interface{})
					status := ticket["status"].(string)
					priority := ticket["priority"].(string)
					assert.Contains(t, []string{"open", "pending"}, status)
					assert.Contains(t, []string{"high", "critical"}, priority)
				}
			},
		},
		{
			name: "Search with pagination",
			query: url.Values{
				"q":     []string{"*"},
				"page":  []string{"2"},
				"limit": []string{"10"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, float64(2), resp["page"])
				assert.Equal(t, float64(10), resp["limit"])
				results := resp["results"].([]interface{})
				assert.LessOrEqual(t, len(results), 10)
			},
		},
		{
			name: "Search with sorting",
			query: url.Values{
				"q":     []string{"*"},
				"sort":  []string{"created_at"},
				"order": []string{"desc"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				results := resp["results"].([]interface{})
				if len(results) > 1 {
					for i := 1; i < len(results); i++ {
						prev := results[i-1].(map[string]interface{})
						curr := results[i].(map[string]interface{})
						prevTime, _ := time.Parse(time.RFC3339, prev["created_at"].(string))
						currTime, _ := time.Parse(time.RFC3339, curr["created_at"].(string))
						assert.True(t, prevTime.After(currTime) || prevTime.Equal(currTime))
					}
				}
			},
		},
		{
			name: "Empty search query",
			query: url.Values{
				"q": []string{""},
			},
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				assert.Contains(t, resp["error"], "Search query is required")
			},
		},
		{
			name: "Search with highlighting",
			query: url.Values{
				"q":         []string{"server error"},
				"highlight": []string{"true"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				results := resp["results"].([]interface{})
				if len(results) > 0 {
					ticket := results[0].(map[string]interface{})
					assert.Contains(t, ticket, "highlights")
					highlights := ticket["highlights"].(map[string]interface{})
					// Check for highlighted fields
					for _, v := range highlights {
						highlightText := v.(string)
						assert.Contains(t, highlightText, "<mark>")
						assert.Contains(t, highlightText, "</mark>")
					}
				}
			},
		},
		{
			name: "Search with field-specific queries",
			query: url.Values{
				"q": []string{"subject:\"password reset\" AND body:urgent"},
			},
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				results := resp["results"].([]interface{})
				for _, r := range results {
					ticket := r.(map[string]interface{})
					subject := ticket["subject"].(string)
					assert.Contains(t, subject, "password reset")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/tickets/search", handleAdvancedTicketSearch)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/tickets/search?"+tt.query.Encode(), nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkResp != nil {
				tt.checkResp(t, response)
			}
		})
	}
}

func TestSearchSuggestions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		query      string
		wantStatus int
		checkResp  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:       "Get suggestions for partial query",
			query:      "pass",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				suggestions := resp["suggestions"].([]interface{})
				assert.Greater(t, len(suggestions), 0)
				// Should include "password", "password reset", etc.
				foundPassword := false
				for _, s := range suggestions {
					if s.(string) == "password" || s.(string) == "password reset" {
						foundPassword = true
						break
					}
				}
				assert.True(t, foundPassword)
			},
		},
		{
			name:       "No suggestions for very short query",
			query:      "p",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				suggestions := resp["suggestions"].([]interface{})
				assert.Len(t, suggestions, 0)
			},
		},
		{
			name:       "Suggestions include popular searches",
			query:      "login",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, resp map[string]interface{}) {
				suggestions := resp["suggestions"].([]interface{})
				// Should prioritize popular/recent searches
				assert.Greater(t, len(suggestions), 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/tickets/search/suggestions", handleSearchSuggestions)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", fmt.Sprintf("/api/tickets/search/suggestions?q=%s", tt.query), nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkResp != nil {
				tt.checkResp(t, response)
			}
		})
	}
}

func TestSearchHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Save search to history", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.POST("/api/tickets/search/history", handleSaveSearchHistory)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/tickets/search/history?q=server+error&name=Server+Issues", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "Search saved to history", response["message"])
		assert.Contains(t, response, "id")
	})

	t.Run("Get search history", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/tickets/search/history", handleGetSearchHistory)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/tickets/search/history", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		history := response["history"].([]interface{})
		assert.GreaterOrEqual(t, len(history), 0)
	})

	t.Run("Delete search from history", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.DELETE("/api/tickets/search/history/:id", handleDeleteSearchHistory)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/tickets/search/history/1", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "Search removed from history", response["message"])
	})
}

func TestSavedSearches(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Create saved search", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.POST("/api/tickets/search/saved", handleCreateSavedSearch)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/tickets/search/saved?name=Critical+Open&q=status:open+priority:critical", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "Search saved successfully", response["message"])
		assert.Contains(t, response, "id")
	})

	t.Run("Get saved searches", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/tickets/search/saved", handleGetSavedSearches)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/tickets/search/saved", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		searches := response["searches"].([]interface{})
		assert.GreaterOrEqual(t, len(searches), 0)
	})

	t.Run("Execute saved search", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/tickets/search/saved/:id/execute", handleExecuteSavedSearch)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/tickets/search/saved/1/execute", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(t, response, "results")
		assert.Contains(t, response, "total")
	})

	t.Run("Update saved search", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.PUT("/api/tickets/search/saved/:id", handleUpdateSavedSearch)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/tickets/search/saved/1?name=Updated+Search&q=status:closed", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "Saved search updated", response["message"])
	})

	t.Run("Delete saved search", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.DELETE("/api/tickets/search/saved/:id", handleDeleteSavedSearch)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/tickets/search/saved/1", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "Saved search deleted", response["message"])
	})
}

func TestExportSearchResults(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		format     string
		wantStatus int
		checkResp  func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name:       "Export as CSV",
			format:     "csv",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Header().Get("Content-Type"), "text/csv")
				assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
				assert.Contains(t, w.Header().Get("Content-Disposition"), ".csv")
				// Check CSV headers
				body := w.Body.String()
				assert.Contains(t, body, "ID,Subject,Status,Priority")
			},
		},
		{
			name:       "Export as JSON",
			format:     "json",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
				assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
				var data map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &data)
				assert.NoError(t, err)
				assert.Contains(t, data, "tickets")
			},
		},
		{
			name:       "Export as Excel",
			format:     "xlsx",
			wantStatus: http.StatusOK,
			checkResp: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Header().Get("Content-Type"), "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
				assert.Contains(t, w.Header().Get("Content-Disposition"), ".xlsx")
			},
		},
		{
			name:       "Invalid export format",
			format:     "invalid",
			wantStatus: http.StatusBadRequest,
			checkResp: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)
				assert.Contains(t, response["error"], "Invalid export format")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/api/tickets/search/export", handleExportSearchResults)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", fmt.Sprintf("/api/tickets/search/export?q=*&format=%s", tt.format), nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if tt.checkResp != nil {
				tt.checkResp(t, w)
			}
		})
	}
}
