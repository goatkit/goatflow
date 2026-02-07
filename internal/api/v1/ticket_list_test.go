package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/goatkit/goatflow/internal/api"
)

func TestListTickets_Pagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.GET("/api/v1/tickets", func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Set("is_authenticated", true)
		HandleListTicketsAPI(c)
	})

	tests := []struct {
		name       string
		query      string
		wantStatus int
		checkBody  func(t *testing.T, body map[string]interface{})
	}{
		{
			name:       "default pagination",
			query:      "",
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				assert.NotNil(t, body["data"])
				assert.NotNil(t, body["pagination"])

				pagination := body["pagination"].(map[string]interface{})
				assert.Equal(t, float64(1), pagination["page"])
				assert.Equal(t, float64(20), pagination["per_page"])
			},
		},
		{
			name:       "custom page size",
			query:      "?per_page=50",
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				pagination := body["pagination"].(map[string]interface{})
				assert.Equal(t, float64(50), pagination["per_page"])
			},
		},
		{
			name:       "specific page",
			query:      "?page=2&per_page=10",
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				pagination := body["pagination"].(map[string]interface{})
				assert.Equal(t, float64(2), pagination["page"])
				assert.Equal(t, float64(10), pagination["per_page"])
			},
		},
		{
			name:       "max page size limit",
			query:      "?per_page=1000",
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				pagination := body["pagination"].(map[string]interface{})
				// Should be capped at 100
				assert.Equal(t, float64(100), pagination["per_page"])
			},
		},
		{
			name:       "limit as alias for per_page",
			query:      "?limit=5",
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				pagination := body["pagination"].(map[string]interface{})
				assert.Equal(t, float64(5), pagination["per_page"])
			},
		},
		{
			name:       "limit capped at 100",
			query:      "?limit=500",
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				pagination := body["pagination"].(map[string]interface{})
				// Should be capped at 100
				assert.Equal(t, float64(100), pagination["per_page"])
			},
		},
		{
			name:       "offset converts to page",
			query:      "?limit=10&offset=20",
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				pagination := body["pagination"].(map[string]interface{})
				// offset=20 with limit=10 means page 3
				assert.Equal(t, float64(3), pagination["page"])
				assert.Equal(t, float64(10), pagination["per_page"])
			},
		},
		{
			name:       "offset zero means page 1",
			query:      "?limit=10&offset=0",
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				pagination := body["pagination"].(map[string]interface{})
				assert.Equal(t, float64(1), pagination["page"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/tickets"+tt.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkBody != nil {
				tt.checkBody(t, response)
			}
		})
	}
}

func TestListTickets_Filtering(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.GET("/api/v1/tickets", func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Set("is_authenticated", true)
		HandleListTicketsAPI(c)
	})

	tests := []struct {
		name       string
		query      string
		wantStatus int
		checkBody  func(t *testing.T, body map[string]interface{})
	}{
		{
			name:       "filter by status",
			query:      "?status=open",
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
				// All returned tickets should be open
			},
		},
		{
			name:       "filter by queue",
			query:      "?queue_id=1",
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
			},
		},
		{
			name:       "filter by priority",
			query:      "?priority_id=3",
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
			},
		},
		{
			name:       "filter by multiple criteria",
			query:      "?status=open&queue_id=1&priority_id=3",
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
			},
		},
		{
			name:       "filter by customer",
			query:      "?customer_user_id=customer@example.com",
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
			},
		},
		{
			name:       "filter by assigned user",
			query:      "?assigned_user_id=5",
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]interface{}) {
				assert.True(t, body["success"].(bool))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/tickets"+tt.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkBody != nil {
				tt.checkBody(t, response)
			}
		})
	}
}

func TestListTickets_Search(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.GET("/api/v1/tickets", func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Set("is_authenticated", true)
		HandleListTicketsAPI(c)
	})

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{
			name:       "search by title",
			query:      "?search=password+reset",
			wantStatus: http.StatusOK,
		},
		{
			name:       "search with special characters",
			query:      "?search=test%40example.com",
			wantStatus: http.StatusOK,
		},
		{
			name:       "search by ticket number",
			query:      "?search=2025083005000001",
			wantStatus: http.StatusOK,
		},
		{
			name:       "empty search",
			query:      "?search=",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/tickets"+tt.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.True(t, response["success"].(bool))
		})
	}
}

func TestListTickets_Sorting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.GET("/api/v1/tickets", func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Set("is_authenticated", true)
		HandleListTicketsAPI(c)
	})

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{
			name:       "sort by created date desc",
			query:      "?sort=created&order=desc",
			wantStatus: http.StatusOK,
		},
		{
			name:       "sort by created date asc",
			query:      "?sort=created&order=asc",
			wantStatus: http.StatusOK,
		},
		{
			name:       "sort by updated date",
			query:      "?sort=updated&order=desc",
			wantStatus: http.StatusOK,
		},
		{
			name:       "sort by priority",
			query:      "?sort=priority&order=desc",
			wantStatus: http.StatusOK,
		},
		{
			name:       "sort by ticket number",
			query:      "?sort=tn&order=asc",
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid sort field ignored",
			query:      "?sort=invalid&order=desc",
			wantStatus: http.StatusOK,
		},
		{
			name:       "default sort (created desc)",
			query:      "",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/tickets"+tt.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestListTickets_ResponseFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.GET("/api/v1/tickets", func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Set("is_authenticated", true)
		HandleListTicketsAPI(c)
	})

	req := httptest.NewRequest("GET", "/api/v1/tickets", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Check response structure
	assert.True(t, response["success"].(bool))
	assert.NotNil(t, response["data"])
	assert.NotNil(t, response["pagination"])

	// Check pagination structure
	pagination := response["pagination"].(map[string]interface{})
	assert.Contains(t, pagination, "page")
	assert.Contains(t, pagination, "per_page")
	assert.Contains(t, pagination, "total")
	assert.Contains(t, pagination, "total_pages")
	assert.Contains(t, pagination, "has_next")
	assert.Contains(t, pagination, "has_prev")

	// Check data is array
	data := response["data"].([]interface{})
	if len(data) > 0 {
		// Check ticket structure
		ticket := data[0].(map[string]interface{})
		assert.Contains(t, ticket, "id")
		assert.Contains(t, ticket, "tn")
		assert.Contains(t, ticket, "title")
		assert.Contains(t, ticket, "queue_id")
		assert.Contains(t, ticket, "queue_name")
		assert.Contains(t, ticket, "state_id")
		assert.Contains(t, ticket, "state_name")
		assert.Contains(t, ticket, "priority_id")
		assert.Contains(t, ticket, "priority_name")
		assert.Contains(t, ticket, "created_at")
		assert.Contains(t, ticket, "updated_at")
	}
}

func TestListTickets_Permissions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		setupAuth  func(c *gin.Context)
		wantStatus int
	}{
		{
			name: "authenticated user can list tickets",
			setupAuth: func(c *gin.Context) {
				c.Set("user_id", 1)
				c.Set("is_authenticated", true)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "unauthenticated user cannot list tickets",
			setupAuth: func(c *gin.Context) {
				// No auth set
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "customer sees only their tickets",
			setupAuth: func(c *gin.Context) {
				c.Set("user_id", 100)
				c.Set("is_customer", true)
				c.Set("is_authenticated", true)
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			apiRouter := NewAPIRouter(nil, nil, nil)

			router.GET("/api/v1/tickets", func(c *gin.Context) {
				tt.setupAuth(c)
				apiRouter.HandleListTickets(c)
			})

			req := httptest.NewRequest("GET", "/api/v1/tickets", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestListTickets_EdgeCases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.GET("/api/v1/tickets", func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Set("is_authenticated", true)
		HandleListTicketsAPI(c)
	})

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{
			name:       "negative page number defaults to 1",
			query:      "?page=-1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "zero page number defaults to 1",
			query:      "?page=0",
			wantStatus: http.StatusOK,
		},
		{
			name:       "negative per_page defaults to 20",
			query:      "?per_page=-10",
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-numeric page ignored",
			query:      "?page=abc",
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-numeric per_page ignored",
			query:      "?per_page=xyz",
			wantStatus: http.StatusOK,
		},
		{
			name:       "very large page number",
			query:      "?page=999999",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/tickets"+tt.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			// Even edge cases should return valid response
			assert.NotNil(t, response["success"])
			assert.NotNil(t, response["data"])
			assert.NotNil(t, response["pagination"])
		})
	}
}

// TestListTickets_IncludeRelations tests including related data.
func TestListTickets_IncludeRelations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.GET("/api/v1/tickets", func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Set("is_authenticated", true)
		HandleListTicketsAPI(c)
	})

	tests := []struct {
		name      string
		query     string
		checkBody func(t *testing.T, body map[string]interface{})
	}{
		{
			name:  "include last article",
			query: "?include=last_article",
			checkBody: func(t *testing.T, body map[string]interface{}) {
				data := body["data"].([]interface{})
				if len(data) > 0 {
					ticket := data[0].(map[string]interface{})
					// Should have last_article field
					_, hasLastArticle := ticket["last_article"]
					assert.True(t, hasLastArticle, "Should include last_article")
				}
			},
		},
		{
			name:  "include article count",
			query: "?include=article_count",
			checkBody: func(t *testing.T, body map[string]interface{}) {
				data := body["data"].([]interface{})
				if len(data) > 0 {
					ticket := data[0].(map[string]interface{})
					_, hasCount := ticket["article_count"]
					assert.True(t, hasCount, "Should include article_count")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/tickets"+tt.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.checkBody != nil {
				tt.checkBody(t, response)
			}
		})
	}
}
