package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

func TestSearchAPI(t *testing.T) {
	// Initialize test database (skip if unavailable)
	if err := database.InitTestDB(); err != nil {
		t.Skip("Database not available, skipping search API test")
	}
	defer database.CloseTestDB()

	// Create test JWT manager
	jwtManager := auth.NewJWTManager("test-secret", time.Hour)

	// Create test token
	token, _ := jwtManager.GenerateToken(1, "testuser@example.com", "Agent", 0)

	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Setup test data
	db, _ := database.GetDB()
	if db == nil {
		t.Skip("Database not available, skipping search API test")
	}

	// Create test tickets
	ticketTypeColumn := database.TicketTypeColumn()
	ticketQuery := database.ConvertPlaceholders(fmt.Sprintf(`
		INSERT INTO ticket (tn, title, queue_id, %s, ticket_state_id, 
			ticket_priority_id, customer_user_id, user_id, responsible_user_id,
			create_time, create_by, change_time, change_by)
		VALUES 
			(?, 'Network connectivity issue', 1, 1, 1, 3, 'customer1@example.com', 1, 1, NOW(), 1, NOW(), 1),
			(?, 'Email server problem', 1, 1, 1, 2, 'customer2@example.com', 1, 1, NOW(), 1, NOW(), 1),
			(?, 'Password reset request', 2, 1, 2, 3, 'customer3@example.com', 1, 1, NOW(), 1, NOW(), 1)
	`, ticketTypeColumn))
	db.Exec(ticketQuery, "2024123100001", "2024123100002", "2024123100003")

	// Create test articles
	articleQuery := database.ConvertPlaceholders(`
		INSERT INTO article (ticket_id, article_type_id, article_sender_type_id,
			from_email, to_email, subject, body, create_time, create_by, change_time, change_by)
		VALUES 
			(1, 1, 3, 'customer1@example.com', 'support@example.com', 
			 'Network down', 'Cannot connect to network since morning', NOW(), 1, NOW(), 1),
			(2, 1, 3, 'customer2@example.com', 'support@example.com',
			 'Email issues', 'Unable to send emails from Outlook', NOW(), 1, NOW(), 1)
	`)
	db.Exec(articleQuery)

	// Create test customers
	customerQuery := database.ConvertPlaceholders(`
		INSERT INTO customer_user (login, email, customer_id, first_name, last_name, 
			valid_id, create_time, create_by, change_time, change_by)
		VALUES 
			('john.doe', 'john.doe@example.com', 'CUST001', 'John', 'Doe', 1, NOW(), 1, NOW(), 1),
			('jane.smith', 'jane.smith@example.com', 'CUST002', 'Jane', 'Smith', 1, NOW(), 1, NOW(), 1)
	`)
	db.Exec(customerQuery)

	t.Run("Search All Types", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.POST("/api/v1/search", HandleSearchAPI)

		payload := map[string]interface{}{
			"query": "network",
			"types": []string{"ticket", "article", "customer"},
			"limit": 10,
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/search", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotNil(t, response["hits"])
		assert.NotNil(t, response["total_hits"])
		assert.NotNil(t, response["took_ms"])
	})

	t.Run("Search Tickets Only", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.POST("/api/v1/search", HandleSearchAPI)

		payload := map[string]interface{}{
			"query": "email",
			"types": []string{"ticket"},
			"limit": 5,
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/search", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Hits []struct {
				ID   string `json:"id"`
				Type string `json:"type"`
			} `json:"hits"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)

		// Verify we only get ticket results
		for _, hit := range response.Hits {
			assert.Equal(t, "ticket", hit.Type)
		}
	})

	t.Run("Search with Filters", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.POST("/api/v1/search", HandleSearchAPI)

		payload := map[string]interface{}{
			"query": "issue",
			"types": []string{"ticket"},
			"filters": map[string]string{
				"queue_id": "1",
			},
			"limit": 10,
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/search", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Search with Pagination", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.POST("/api/v1/search", HandleSearchAPI)

		payload := map[string]interface{}{
			"query":  "customer",
			"types":  []string{"customer"},
			"offset": 0,
			"limit":  2,
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/search", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Hits []interface{} `json:"hits"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.LessOrEqual(t, len(response.Hits), 2)
	})

	t.Run("Search with Highlighting", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.POST("/api/v1/search", HandleSearchAPI)

		payload := map[string]interface{}{
			"query":     "network",
			"types":     []string{"ticket"},
			"highlight": true,
			"limit":     5,
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/search", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Hits []struct {
				Highlights map[string][]string `json:"highlights"`
			} `json:"hits"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)

		// Check if highlights are present
		if len(response.Hits) > 0 && response.Hits[0].Highlights != nil {
			assert.NotEmpty(t, response.Hits[0].Highlights)
		}
	})

	t.Run("Empty Search Query", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.POST("/api/v1/search", HandleSearchAPI)

		payload := map[string]interface{}{
			"query": "",
			"types": []string{"ticket"},
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/search", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Unauthorized Search", func(t *testing.T) {
		router := gin.New()
		router.POST("/api/v1/search", HandleSearchAPI)

		payload := map[string]interface{}{
			"query": "test",
			"types": []string{"ticket"},
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/search", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
