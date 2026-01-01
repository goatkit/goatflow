package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestQueueSortingInterface(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should display sort controls",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should have sort dropdown
				assert.Contains(t, body, `name="sort"`)
				assert.Contains(t, body, `id="queue-sort"`)

				// Should have sort options
				assert.Contains(t, body, "Sort by")
				assert.Contains(t, body, `value="name_asc"`)
				assert.Contains(t, body, "Name (A-Z)")
				assert.Contains(t, body, `value="name_desc"`)
				assert.Contains(t, body, "Name (Z-A)")
				assert.Contains(t, body, `value="tickets_asc"`)
				assert.Contains(t, body, "Tickets (Low to High)")
				assert.Contains(t, body, `value="tickets_desc"`)
				assert.Contains(t, body, "Tickets (High to Low)")
				assert.Contains(t, body, `value="status_asc"`)
				assert.Contains(t, body, "Status")

				// Should have HTMX attributes for dynamic sorting
				assert.Contains(t, body, `hx-get="/queues"`)
				assert.Contains(t, body, `hx-trigger="change"`)
				assert.Contains(t, body, `hx-target="#queue-list-container"`)
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

func TestQueueSortingFunctionality(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		sortParam      string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should sort queues by name ascending",
			sortParam:      "?sort=name_asc",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Check that queues appear in alphabetical order
				// Using indexOf to verify order
				junkIndex := indexOf(body, "Junk")
				miscIndex := indexOf(body, "Misc")
				rawIndex := indexOf(body, "Raw")
				supportIndex := indexOf(body, "Support")

				// Alphabetical order: Junk, Misc, Raw, Support
				assert.True(t, junkIndex < miscIndex, "Junk should appear before Misc")
				assert.True(t, miscIndex < rawIndex, "Misc should appear before Raw")
				assert.True(t, rawIndex < supportIndex, "Raw should appear before Support")
			},
		},
		{
			name:           "should sort queues by name descending",
			sortParam:      "?sort=name_desc",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Check reverse alphabetical order
				junkIndex := indexOf(body, "Junk")
				miscIndex := indexOf(body, "Misc")
				rawIndex := indexOf(body, "Raw")
				supportIndex := indexOf(body, "Support")

				// Reverse alphabetical order: Support, Raw, Misc, Junk
				assert.True(t, supportIndex < rawIndex, "Support should appear before Raw")
				assert.True(t, rawIndex < miscIndex, "Raw should appear before Misc")
				assert.True(t, miscIndex < junkIndex, "Misc should appear before Junk")
			},
		},
		{
			name:           "should sort queues by ticket count ascending",
			sortParam:      "?sort=tickets_asc",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Misc (0) should appear first, then Junk (1), Raw (2), Support (3)
				miscIndex := indexOf(body, "Misc")
				junkIndex := indexOf(body, "Junk")
				rawIndex := indexOf(body, "Raw")
				supportIndex := indexOf(body, "Support")

				assert.True(t, miscIndex < junkIndex, "Misc (0 tickets) should appear first")
				assert.True(t, junkIndex < rawIndex, "Junk (1 ticket) should appear before Raw (2 tickets)")
				assert.True(t, rawIndex < supportIndex, "Raw (2 tickets) should appear before Support (3 tickets)")
			},
		},
		{
			name:           "should sort queues by ticket count descending",
			sortParam:      "?sort=tickets_desc",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Support (3) should appear first, then Raw (2), Junk (1), Misc (0)
				miscIndex := indexOf(body, "Misc")
				junkIndex := indexOf(body, "Junk")
				rawIndex := indexOf(body, "Raw")
				supportIndex := indexOf(body, "Support")

				assert.True(t, supportIndex < rawIndex, "Support (3 tickets) should appear first")
				assert.True(t, rawIndex < junkIndex, "Raw (2 tickets) should appear before Junk (1 ticket)")
				assert.True(t, junkIndex < miscIndex, "Junk (1 ticket) should appear before Misc (0 tickets)")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/queues", handleQueuesList)

			req, _ := http.NewRequest("GET", "/queues"+tt.sortParam, nil)
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

func TestQueuePaginationInterface(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should display pagination controls",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should have pagination container
				assert.Contains(t, body, `id="queue-pagination"`)

				// Should show page info
				assert.Contains(t, body, "Showing")
				assert.Contains(t, body, "of")
				assert.Contains(t, body, "queues")

				// Should have items per page selector
				assert.Contains(t, body, `name="per_page"`)
				assert.Contains(t, body, `value="10"`)
				assert.Contains(t, body, `value="25"`)
				assert.Contains(t, body, `value="50"`)
				assert.Contains(t, body, `value="100"`)

				// Should have HTMX attributes for per page change
				assert.Contains(t, body, `hx-trigger="change"`)
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

func TestQueuePaginationFunctionality(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should show first page by default",
			query:          "",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should show page 1 indicator
				assert.Contains(t, body, "Page 1")

				// Should show all 4 queues when per_page not specified
				assert.Contains(t, body, "Raw")
				assert.Contains(t, body, "Junk")
				assert.Contains(t, body, "Misc")
				assert.Contains(t, body, "Support")
			},
		},
		{
			name:           "should limit results per page",
			query:          "?per_page=2",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should only show 2 queues
				queueCount := countOccurrences(body, `<li>`)
				assert.LessOrEqual(t, queueCount, 2, "Should show maximum 2 queues")

				// Should show Next button
				assert.Contains(t, body, "Next")
				assert.Contains(t, body, `hx-get="/queues?page=2&per_page=2"`)
			},
		},
		{
			name:           "should navigate to second page",
			query:          "?page=2&per_page=2",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should show page 2 indicator
				assert.Contains(t, body, "Page 2")

				// Should show Previous button
				assert.Contains(t, body, "Previous")
				assert.Contains(t, body, `hx-get="/queues?page=1&per_page=2"`)
			},
		},
		{
			name:           "should maintain sort order across pages",
			query:          "?page=2&per_page=2&sort=name_asc",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should maintain sort parameter in pagination links
				assert.Contains(t, body, `sort=name_asc`)

				// On page 2 with alphabetical sort, should show Raw and Support
				assert.Contains(t, body, "Raw")
				assert.Contains(t, body, "Support")
			},
		},
		{
			name:           "should maintain search filter across pages",
			query:          "?page=1&per_page=10&search=default",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should maintain search parameter in pagination links
				if countOccurrences(body, `<li>`) > 1 {
					assert.Contains(t, body, `search=default`)
				}

				// Should only show queues containing 'default' (Raw has "by default" in comment)
				assert.Contains(t, body, "Raw")
				assert.NotContains(t, body, "Junk")
				assert.NotContains(t, body, "Misc")
				assert.NotContains(t, body, "Support")
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

func TestQueuePaginationEdgeCases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should handle invalid page number",
			query:          "?page=invalid",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should default to page 1
				assert.Contains(t, body, "Page 1")
			},
		},
		{
			name:           "should handle negative page number",
			query:          "?page=-1",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should default to page 1
				assert.Contains(t, body, "Page 1")
			},
		},
		{
			name:           "should handle page number beyond total pages",
			query:          "?page=999&per_page=10",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should show last available page or empty result
				assert.Contains(t, body, "No queues")
			},
		},
		{
			name:           "should handle invalid per_page value",
			query:          "?per_page=invalid",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should default to reasonable per_page (e.g., 10)
				assert.Contains(t, body, "Page 1")
			},
		},
		{
			name:           "should limit maximum per_page value",
			query:          "?per_page=1000",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Should cap at maximum (e.g., 100)
				// All 4 queues should be visible on one page
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

func TestQueueSortingWithPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		query          string
		expectedStatus int
		checkResponse  func(t *testing.T, body string)
	}{
		{
			name:           "should preserve sort order when changing pages",
			query:          "?sort=name_desc&page=1&per_page=2",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// First page with descending sort should show Support and Raw
				supportIndex := indexOf(body, "Support")
				rawIndex := indexOf(body, "Raw")

				// Support should appear before Raw in descending order
				if supportIndex != -1 && rawIndex != -1 {
					assert.True(t, supportIndex < rawIndex, "Support should appear before Raw")
				}

				// Pagination links should preserve sort parameter
				assert.Contains(t, body, "sort=name_desc")
			},
		},
		{
			name:           "should update pagination when sort changes",
			query:          "?sort=tickets_desc&per_page=2",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// With tickets descending, should show queues with most tickets first
				// Should reset to page 1 when sort changes
				assert.Contains(t, body, "Page 1")
			},
		},
		{
			name:           "should maintain all parameters in pagination links",
			query:          "?sort=name_asc&search=a&status=active&page=1&per_page=10",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, body string) {
				// Pagination links should include all parameters
				if strings.Contains(body, "hx-get") {
					assert.Contains(t, body, "sort=name_asc")
					assert.Contains(t, body, "search=a")
					assert.Contains(t, body, "status=active")
					assert.Contains(t, body, "per_page=10")
				}
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

// Helper function to find index of a substring.
func indexOf(s, substr string) int {
	index := -1
	if i := strings.Index(s, substr); i >= 0 {
		index = i
	}
	return index
}

// Helper function to count occurrences of a substring.
func countOccurrences(s, substr string) int {
	return strings.Count(s, substr)
}
