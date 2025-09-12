package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/webhooks"
	"strconv"
	"strings"
    "time"
)

// TestWebhookAPIContract verifies the webhook API meets its contract requirements
func TestWebhookAPIContract(t *testing.T) {
	// Initialize test database
	database.InitTestDB()
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
        t.Skip("Database not available for webhook contract tests")
    }
	
	// Create webhook table if not exists
	db.Exec(`
		CREATE TABLE IF NOT EXISTS webhooks (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			url VARCHAR(1024) NOT NULL,
			secret VARCHAR(255),
			events TEXT,
			active BOOLEAN DEFAULT true,
			retry_count INTEGER DEFAULT 3,
			timeout_seconds INTEGER DEFAULT 30,
			headers TEXT,
			create_time TIMESTAMP DEFAULT NOW(),
			create_by INTEGER,
			change_time TIMESTAMP DEFAULT NOW(),
			change_by INTEGER
		)
	`)

	// Create webhook_deliveries table
	db.Exec(`
		CREATE TABLE IF NOT EXISTS webhook_deliveries (
			id SERIAL PRIMARY KEY,
			webhook_id INTEGER REFERENCES webhooks(id),
			event_type VARCHAR(100),
			payload TEXT,
			status_code INTEGER,
			response TEXT,
			attempts INTEGER DEFAULT 0,
			delivered_at TIMESTAMP,
			next_retry TIMESTAMP,
			created_at TIMESTAMP DEFAULT NOW(),
			success BOOLEAN DEFAULT false
		)
	`)

	t.Run("RegisterWebhook Contract", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.POST("/api/v1/webhooks", HandleRegisterWebhookAPI)

		// Test valid webhook registration
		payload := map[string]interface{}{
			"name": "Test Webhook",
			"url":  "https://example.com/webhook",
			"secret": "webhook-secret",
			"events": []string{"ticket.created", "ticket.updated"},
			"retry_count":     3,
			"timeout_seconds": 30,
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/webhooks", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Contract: Must return 201 Created
		assert.Equal(t, http.StatusCreated, w.Code, "Should return 201 Created")

		// Contract: Response must contain webhook ID and match input
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotNil(t, response["id"], "Response must contain webhook ID")
		assert.Equal(t, "Test Webhook", response["name"], "Name must match input")
		assert.Equal(t, "https://example.com/webhook", response["url"], "URL must match input")
		assert.Equal(t, true, response["active"], "Webhook should be active by default")

		// Test invalid URL
		payload["url"] = "not-a-valid-url"
		body, _ = json.Marshal(payload)
		req = httptest.NewRequest("POST", "/api/v1/webhooks", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Contract: Invalid URL must return 400
		assert.Equal(t, http.StatusBadRequest, w.Code, "Invalid URL should return 400")

		// Test missing required fields
		invalidPayload := map[string]interface{}{"name": "No URL"}
		body, _ = json.Marshal(invalidPayload)
		req = httptest.NewRequest("POST", "/api/v1/webhooks", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Contract: Missing URL must return 400
		assert.Equal(t, http.StatusBadRequest, w.Code, "Missing URL should return 400")
	})

	t.Run("ListWebhooks Contract", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/webhooks", HandleListWebhooksAPI)

		// Create test webhooks
		db.Exec(database.ConvertPlaceholders(`
			INSERT INTO webhooks (name, url, events, active, create_by, change_by)
			VALUES ($1, $2, $3, true, 1, 1), ($4, $5, $6, false, 1, 1)
		`), "Active Hook", "https://active.com", "ticket.created",
			"Inactive Hook", "https://inactive.com", "ticket.closed")

		// Test listing all webhooks
		req := httptest.NewRequest("GET", "/api/v1/webhooks", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Contract: Must return 200 OK
		assert.Equal(t, http.StatusOK, w.Code, "Should return 200 OK")

		// Contract: Response must contain webhooks array and total
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotNil(t, response["webhooks"], "Response must contain webhooks array")
		assert.NotNil(t, response["total"], "Response must contain total count")

		// Test filtering by active status
		req = httptest.NewRequest("GET", "/api/v1/webhooks?active=true", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Filtered request should return 200")
		json.Unmarshal(w.Body.Bytes(), &response)
		webhooks := response["webhooks"].([]interface{})
		// Contract: Filter should work correctly
		for _, webhook := range webhooks {
			w := webhook.(map[string]interface{})
			assert.Equal(t, true, w["active"], "Filtered results should only contain active webhooks")
		}
	})

	t.Run("GetWebhook Contract", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/webhooks/:id", HandleGetWebhookAPI)

		// Create a test webhook
		var webhookID int
		insertQuery := database.ConvertPlaceholders(`
			INSERT INTO webhooks (name, url, secret, events, active, create_by, change_by)
			VALUES ($1, $2, $3, $4, true, 1, 1)
			RETURNING id
		`)
		db.QueryRow(insertQuery, "Get Test", "https://get.test", "secret", "ticket.created").Scan(&webhookID)

		// Test getting existing webhook
		req := httptest.NewRequest("GET", "/api/v1/webhooks/"+strconv.Itoa(webhookID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Contract: Must return 200 OK for existing webhook
		assert.Equal(t, http.StatusOK, w.Code, "Should return 200 for existing webhook")

		// Contract: Response must contain all webhook fields
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, float64(webhookID), response["id"], "ID must match requested webhook")
		assert.Equal(t, "Get Test", response["name"], "Name must match")
		assert.Equal(t, "https://get.test", response["url"], "URL must match")

		// Test getting non-existent webhook
		req = httptest.NewRequest("GET", "/api/v1/webhooks/99999", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Contract: Non-existent webhook must return 404
		assert.Equal(t, http.StatusNotFound, w.Code, "Non-existent webhook should return 404")
	})

	t.Run("UpdateWebhook Contract", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.PUT("/api/v1/webhooks/:id", HandleUpdateWebhookAPI)

		// Create a test webhook
		var webhookID int
		insertQuery := database.ConvertPlaceholders(`
			INSERT INTO webhooks (name, url, events, active, create_by, change_by)
			VALUES ($1, $2, $3, true, 1, 1)
			RETURNING id
		`)
		db.QueryRow(insertQuery, "Update Test", "https://old.url", "ticket.created").Scan(&webhookID)

		// Test updating webhook
		payload := map[string]interface{}{
			"name":   "Updated Name",
			"url":    "https://new.url",
			"active": false,
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("PUT", "/api/v1/webhooks/"+strconv.Itoa(webhookID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Contract: Must return 200 OK
		assert.Equal(t, http.StatusOK, w.Code, "Should return 200 for successful update")

		// Contract: Response must reflect updates
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "Updated Name", response["name"], "Name should be updated")
		assert.Equal(t, "https://new.url", response["url"], "URL should be updated")
		assert.Equal(t, false, response["active"], "Active status should be updated")

		// Test updating non-existent webhook
		req = httptest.NewRequest("PUT", "/api/v1/webhooks/99999", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Contract: Non-existent webhook update must return 404
		assert.Equal(t, http.StatusNotFound, w.Code, "Non-existent webhook update should return 404")
	})

	t.Run("DeleteWebhook Contract", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.DELETE("/api/v1/webhooks/:id", HandleDeleteWebhookAPI)

		// Create a test webhook
		var webhookID int
		insertQuery := database.ConvertPlaceholders(`
			INSERT INTO webhooks (name, url, events, active, create_by, change_by)
			VALUES ($1, $2, $3, true, 1, 1)
			RETURNING id
		`)
		db.QueryRow(insertQuery, "Delete Test", "https://delete.test", "ticket.created").Scan(&webhookID)

		// Test deleting webhook
		req := httptest.NewRequest("DELETE", "/api/v1/webhooks/"+strconv.Itoa(webhookID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Contract: Must return 200 OK
		assert.Equal(t, http.StatusOK, w.Code, "Should return 200 for successful deletion")

		// Contract: Webhook must be deleted from database
		var count int
		db.QueryRow("SELECT COUNT(*) FROM webhooks WHERE id = $1", webhookID).Scan(&count)
		assert.Equal(t, 0, count, "Webhook should be deleted from database")

		// Test deleting non-existent webhook
		req = httptest.NewRequest("DELETE", "/api/v1/webhooks/99999", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Contract: Non-existent webhook deletion must return 404
		assert.Equal(t, http.StatusNotFound, w.Code, "Non-existent webhook deletion should return 404")
	})

	t.Run("TestWebhook Contract", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.POST("/api/v1/webhooks/:id/test", HandleTestWebhookAPI)

		// Create a test webhook
		var webhookID int
		insertQuery := database.ConvertPlaceholders(`
			INSERT INTO webhooks (name, url, secret, events, active, create_by, change_by)
			VALUES ($1, $2, $3, $4, true, 1, 1)
			RETURNING id
		`)
		db.QueryRow(insertQuery, "Test Webhook", "https://httpbin.org/post", "secret", "ticket.created").Scan(&webhookID)

		// Test webhook with valid payload
		payload := map[string]interface{}{
			"event_type": "ticket.created",
			"test_payload": map[string]interface{}{
				"ticket_id": 123,
				"title":     "Test Ticket",
			},
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/webhooks/"+strconv.Itoa(webhookID)+"/test", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Contract: Must return 200 OK for test
		assert.Equal(t, http.StatusOK, w.Code, "Test webhook should return 200")

		// Contract: Response must contain test results
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotNil(t, response["status_code"], "Response must contain status code")
		assert.NotNil(t, response["response_time_ms"], "Response must contain response time")
	})

	t.Run("WebhookDeliveries Contract", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/webhooks/:id/deliveries", HandleWebhookDeliveriesAPI)

		// Create a test webhook with deliveries
		var webhookID int
		insertQuery := database.ConvertPlaceholders(`
			INSERT INTO webhooks (name, url, events, active, create_by, change_by)
			VALUES ($1, $2, $3, true, 1, 1)
			RETURNING id
		`)
		db.QueryRow(insertQuery, "Delivery Test", "https://example.com", "ticket.created").Scan(&webhookID)

		// Create test deliveries
		deliveryQuery := database.ConvertPlaceholders(`
			INSERT INTO webhook_deliveries (webhook_id, event_type, payload, status_code, attempts, success)
			VALUES ($1, 'ticket.created', '{"test": "data1"}', 200, 1, true),
			       ($1, 'ticket.updated', '{"test": "data2"}', 500, 3, false)
		`)
		db.Exec(deliveryQuery, webhookID)

		req := httptest.NewRequest("GET", "/api/v1/webhooks/"+strconv.Itoa(webhookID)+"/deliveries", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Contract: Must return 200 OK
		assert.Equal(t, http.StatusOK, w.Code, "Should return 200 for deliveries")

		// Contract: Response must contain deliveries array
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotNil(t, response["deliveries"], "Response must contain deliveries array")
		assert.NotNil(t, response["total"], "Response must contain total count")

		deliveries := response["deliveries"].([]interface{})
		assert.Len(t, deliveries, 2, "Should have 2 deliveries")
	})

	t.Run("RetryWebhookDelivery Contract", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.POST("/api/v1/webhooks/deliveries/:id/retry", HandleRetryWebhookDeliveryAPI)

		// Create a failed delivery
		var deliveryID int
		var webhookID int
		
		// First create webhook
		insertWebhook := database.ConvertPlaceholders(`
			INSERT INTO webhooks (name, url, events, active, create_by, change_by)
			VALUES ($1, $2, $3, true, 1, 1)
			RETURNING id
		`)
		db.QueryRow(insertWebhook, "Retry Test", "https://httpbin.org/post", "ticket.created").Scan(&webhookID)
		
		// Then create failed delivery
		insertDelivery := database.ConvertPlaceholders(`
			INSERT INTO webhook_deliveries (webhook_id, event_type, payload, status_code, attempts, success)
			VALUES ($1, 'ticket.created', '{"test": "retry"}', 500, 3, false)
			RETURNING id
		`)
		db.QueryRow(insertDelivery, webhookID).Scan(&deliveryID)

		req := httptest.NewRequest("POST", "/api/v1/webhooks/deliveries/"+strconv.Itoa(deliveryID)+"/retry", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Contract: Must return 200 OK for retry
		assert.Equal(t, http.StatusOK, w.Code, "Retry should return 200")

		// Contract: Response must indicate retry queued
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, true, response["success"], "Retry should be successful")
		assert.NotNil(t, response["message"], "Response must contain message")
	})

	t.Run("Event Types Contract", func(t *testing.T) {
		// Verify all event types are valid
		allEvents := webhooks.AllEventTypes()
		assert.NotEmpty(t, allEvents, "Should have event types defined")

		// Contract: Event types must follow naming convention
		for _, event := range allEvents {
			assert.Contains(t, string(event), ".", "Event type must contain domain separator")
			parts := strings.Split(string(event), ".")
			assert.Len(t, parts, 2, "Event type must be in format 'domain.action'")
		}

		// Contract: Required event types must exist
		requiredEvents := []webhooks.EventType{
			webhooks.EventTicketCreated,
			webhooks.EventTicketUpdated,
			webhooks.EventTicketClosed,
			webhooks.EventArticleCreated,
		}

		for _, required := range requiredEvents {
			assert.Contains(t, allEvents, required, "Required event type must exist: "+string(required))
		}
	})

	t.Run("Authorization Contract", func(t *testing.T) {
		router := gin.New()
		router.GET("/api/v1/webhooks", HandleListWebhooksAPI)

		// Test without authorization
		req := httptest.NewRequest("GET", "/api/v1/webhooks", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Contract: Unauthorized request must return 401
		assert.Equal(t, http.StatusUnauthorized, w.Code, "Unauthorized request should return 401")
	})
}
