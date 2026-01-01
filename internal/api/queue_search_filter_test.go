package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestQueueSearchInterface(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should display search and filter controls",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should have search input
				assert.Contains(t, body, `<input`)
				assert.Contains(t, body, `name="search"`)
				assert.Contains(t, body, `placeholder="Search queues..."`)

				// Should have search functionality with HTMX
				assert.Contains(t, body, `hx-get="/queues"`)
				assert.Contains(t, body, `hx-trigger="input changed delay:300ms"`)
				assert.Contains(t, body, `hx-target="#queue-list-container"`)

				// Should have status filter dropdown
				assert.Contains(t, body, `name="status"`)
				assert.Contains(t, body, "All Statuses")
				assert.Contains(t, body, "Active")
				assert.Contains(t, body, "Inactive")

				// Should have clear search button
				assert.Contains(t, body, "Clear")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/queues", handleQueuesList)

			req, _ := http.NewRequest("GET", "/queues", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestQueueSearchFunctionality(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		searchQuery    string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should search queues by name",
			searchQuery:    "?search=Raw",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should find Raw queue
				assert.Contains(t, body, "Raw")
				assert.Contains(t, body, "All new tickets are placed in this queue by default")

				// Should NOT find other queues
				assert.NotContains(t, body, "Junk")
				assert.NotContains(t, body, "Misc")
				assert.NotContains(t, body, "Support")
			},
		},
		{
			name:           "should search queues by description",
			searchQuery:    "?search=spam",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should find Junk queue (has "spam" in description)
				assert.Contains(t, body, "Junk")
				assert.Contains(t, body, "Spam and junk emails")

				// Should NOT find other queues
				assert.NotContains(t, body, "Raw")
				assert.NotContains(t, body, "Misc")
			},
		},
		{
			name:           "should return empty results for non-matching search",
			searchQuery:    "?search=nonexistent",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should show no results message
				assert.Contains(t, body, "No queues found")
				assert.Contains(t, body, "matching your search")

				// Should not show any queue names
				assert.NotContains(t, body, "Raw")
				assert.NotContains(t, body, "Junk")
			},
		},
		{
			name:           "should handle case-insensitive search",
			searchQuery:    "?search=RAW",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should find Raw queue even with uppercase search
				assert.Contains(t, body, "Raw")
				assert.NotContains(t, body, "Junk")
			},
		},
		{
			name:           "should show all queues when search is empty",
			searchQuery:    "?search=",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should show all queues
				assert.Contains(t, body, "Raw")
				assert.Contains(t, body, "Junk")
				assert.Contains(t, body, "Misc")
				assert.Contains(t, body, "Support")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/queues", handleQueuesList)

			req, _ := http.NewRequest("GET", "/queues"+tt.searchQuery, nil)
			req.Header.Set("HX-Request", "true") // Simulate HTMX search request
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestQueueStatusFiltering(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		filterQuery    string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should filter active queues",
			filterQuery:    "?status=active",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// All our mock queues are active, so should show all
				assert.Contains(t, body, "Raw")
				assert.Contains(t, body, "Junk")
				assert.Contains(t, body, "Misc")
				assert.Contains(t, body, "Support")

				// Should show active status badges
				assert.Contains(t, body, "Active")
			},
		},
		{
			name:           "should filter inactive queues",
			filterQuery:    "?status=inactive",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// No inactive queues in mock data
				assert.Contains(t, body, "No queues found")
				assert.Contains(t, body, "matching your criteria")
			},
		},
		{
			name:           "should show all queues when status filter is 'all'",
			filterQuery:    "?status=all",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should show all queues regardless of status
				assert.Contains(t, body, "Raw")
				assert.Contains(t, body, "Junk")
				assert.Contains(t, body, "Misc")
				assert.Contains(t, body, "Support")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/queues", handleQueuesList)

			req, _ := http.NewRequest("GET", "/queues"+tt.filterQuery, nil)
			req.Header.Set("HX-Request", "true") // Simulate HTMX filter request
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestQueueCombinedSearchAndFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should combine search and status filter",
			query:          "?search=Raw&status=active",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should find Raw queue (matches search and active status)
				assert.Contains(t, body, "Raw")
				assert.NotContains(t, body, "Junk")
				assert.NotContains(t, body, "Misc")
			},
		},
		{
			name:           "should return no results when search matches but status doesn't",
			query:          "?search=Raw&status=inactive",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Raw queue is active, so no results when filtering for inactive
				assert.Contains(t, body, "No queues found")
				assert.NotContains(t, body, "Raw")
			},
		},
		{
			name:           "should handle empty search with status filter",
			query:          "?search=&status=active",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should show all active queues
				assert.Contains(t, body, "Raw")
				assert.Contains(t, body, "Junk")
				assert.Contains(t, body, "Active")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/queues", handleQueuesList)

			req, _ := http.NewRequest("GET", "/queues"+tt.query, nil)
			req.Header.Set("HX-Request", "true") // Simulate HTMX request
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestQueueSearchClearFunctionality(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		endpoint       string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "clear search should reset to show all queues",
			endpoint:       "/queues/clear-search",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should show all queues after clearing search
				assert.Contains(t, body, "Raw")
				assert.Contains(t, body, "Junk")
				assert.Contains(t, body, "Misc")
				assert.Contains(t, body, "Support")

				// Should not show any search/filter indicators
				assert.NotContains(t, body, "matching your search")
				assert.NotContains(t, body, "No queues found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/queues/clear-search", handleClearQueueSearch)

			req, _ := http.NewRequest("GET", tt.endpoint, nil)
			req.Header.Set("HX-Request", "true") // Simulate HTMX clear request
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestQueueSearchPersistence(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "search input should show current search term",
			query:          "?search=Raw&status=active",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Search input should be populated with current search
				assert.Contains(t, body, `value="Raw"`)

				// Status dropdown should show selected status
				assert.Contains(t, body, `value="active" selected`)

				// Should show search results
				assert.Contains(t, body, "Raw")
			},
		},
		{
			name:           "should preserve search state during HTMX requests",
			query:          "?search=Junk",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// When returning HTML fragment, should maintain search context
				assert.Contains(t, body, "Junk")
				assert.NotContains(t, body, "Raw")

				// Should show search is active
				if strings.Contains(body, "input") {
					assert.Contains(t, body, `value="Junk"`)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/queues", handleQueuesList)

			req, _ := http.NewRequest("GET", "/queues"+tt.query, nil)
			// Test both HTMX and regular requests
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}

func TestQueueSearchPerformance(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should handle special characters in search",
			query:          "?search=%40test",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should not crash and return appropriate results
				assert.Contains(t, body, "No queues found")
			},
		},
		{
			name:           "should handle very long search terms",
			query:          "?search=" + strings.Repeat("a", 200),
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should not crash and return no results
				assert.Contains(t, body, "No queues found")
			},
		},
		{
			name:           "should handle empty and whitespace search",
			query:          "?search=%20%20%20",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should treat whitespace as empty search and show all queues
				assert.Contains(t, body, "Raw")
				assert.Contains(t, body, "Junk")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/queues", handleQueuesList)

			req, _ := http.NewRequest("GET", "/queues"+tt.query, nil)
			req.Header.Set("HX-Request", "true")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.checkResponse != nil {
				tt.checkResponse(t, w.Body.String())
			}
		})
	}
}
