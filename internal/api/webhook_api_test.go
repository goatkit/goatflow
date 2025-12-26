//go:build db

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookAPI(t *testing.T) {
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
		t.Skip("Database not available for webhook tests")
	}

	ensureWebhookTables(t)

	t.Run("Register Webhook", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.POST("/api/v1/webhooks", HandleRegisterWebhookAPI)

		payload := map[string]interface{}{
			"name":   "Slack Integration",
			"url":    "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXX",
			"secret": "webhook-secret-key",
			"events": []string{
				"ticket.created",
				"ticket.updated",
				"ticket.closed",
				"article.created",
			},
			"retry_count":     3,
			"timeout_seconds": 30,
			"headers": map[string]string{
				"X-Custom-Header": "CustomValue",
			},
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("POST", "/api/v1/webhooks", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response struct {
			ID     int      `json:"id"`
			Name   string   `json:"name"`
			URL    string   `json:"url"`
			Events []string `json:"events"`
			Active bool     `json:"active"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotZero(t, response.ID)
		assert.Equal(t, "Slack Integration", response.Name)
		assert.True(t, response.Active)
		assert.Contains(t, response.Events, "ticket.created")
	})

	t.Run("List Webhooks", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/webhooks", HandleListWebhooksAPI)

		// Create test webhooks
		webhookQuery := database.ConvertPlaceholders(`
			INSERT INTO webhooks (name, url, secret, events, active, create_by, change_by)
			VALUES 
				($1, $2, $3, $4, true, 1, 1),
				($5, $6, $7, $8, false, 1, 1)
		`)
		db.Exec(webhookQuery,
			"Teams Webhook", "https://teams.microsoft.com/webhook", "secret1", "ticket.created,ticket.updated",
			"Discord Webhook", "https://discord.com/api/webhooks/123", "secret2", "article.created",
		)

		req := httptest.NewRequest("GET", "/api/v1/webhooks", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Webhooks []struct {
				ID     int    `json:"id"`
				Name   string `json:"name"`
				URL    string `json:"url"`
				Active bool   `json:"active"`
			} `json:"webhooks"`
			Total int `json:"total"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotEmpty(t, response.Webhooks)
		assert.Greater(t, response.Total, 0)

		// Test with active filter
		req = httptest.NewRequest("GET", "/api/v1/webhooks?active=true", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Get Webhook", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/webhooks/:id", HandleGetWebhookAPI)

		insertQuery := `
			INSERT INTO webhooks (name, url, secret, events, active, create_by, change_by)
			VALUES ($1, $2, $3, $4, true, 1, 1)
			RETURNING id
		`
		webhookID := insertWebhookRow(t, insertQuery,
			"Test Webhook", "https://example.com/webhook", "secret", "ticket.created",
		)

		req := httptest.NewRequest("GET", "/api/v1/webhooks/"+strconv.Itoa(webhookID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var webhook struct {
			ID     int      `json:"id"`
			Name   string   `json:"name"`
			URL    string   `json:"url"`
			Events []string `json:"events"`
			Active bool     `json:"active"`
		}
		json.Unmarshal(w.Body.Bytes(), &webhook)
		assert.Equal(t, webhookID, webhook.ID)
		assert.Equal(t, "Test Webhook", webhook.Name)
		assert.True(t, webhook.Active)
	})

	t.Run("Update Webhook", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.PUT("/api/v1/webhooks/:id", HandleUpdateWebhookAPI)

		insertQuery := `
			INSERT INTO webhooks (name, url, secret, events, active, create_by, change_by)
			VALUES ($1, $2, $3, $4, true, 1, 1)
			RETURNING id
		`
		webhookID := insertWebhookRow(t, insertQuery,
			"Update Test", "https://old.url/webhook", "oldsecret", "ticket.created",
		)

		payload := map[string]interface{}{
			"name":   "Updated Webhook",
			"url":    "https://new.url/webhook",
			"events": []string{"ticket.created", "ticket.closed"},
			"active": false,
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest("PUT", "/api/v1/webhooks/"+strconv.Itoa(webhookID), bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			ID     int      `json:"id"`
			Name   string   `json:"name"`
			URL    string   `json:"url"`
			Events []string `json:"events"`
			Active bool     `json:"active"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Equal(t, "Updated Webhook", response.Name)
		assert.Equal(t, "https://new.url/webhook", response.URL)
		assert.False(t, response.Active)
	})

	t.Run("Delete Webhook", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.DELETE("/api/v1/webhooks/:id", HandleDeleteWebhookAPI)

		// Create a test webhook
		insertQuery := `
			INSERT INTO webhooks (name, url, secret, events, active, create_by, change_by)
			VALUES ($1, $2, $3, $4, true, 1, 1)
			RETURNING id
		`
		webhookID := insertWebhookRow(t, insertQuery,
			"Delete Test", "https://delete.url/webhook", "secret", "ticket.created",
		)

		req := httptest.NewRequest("DELETE", "/api/v1/webhooks/"+strconv.Itoa(webhookID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify webhook is deleted
		var count int
		countQuery := database.ConvertPlaceholders(`
			SELECT COUNT(*) FROM webhooks WHERE id = $1
		`)
		db.QueryRow(countQuery, webhookID).Scan(&count)
		assert.Equal(t, 0, count)
	})

	t.Run("Test Webhook", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.POST("/api/v1/webhooks/:id/test", HandleTestWebhookAPI)

		// Create a test webhook
		insertQuery := `
			INSERT INTO webhooks (name, url, secret, events, active, create_by, change_by)
			VALUES ($1, $2, $3, $4, true, 1, 1)
			RETURNING id
		`
		webhookID := insertWebhookRow(t, insertQuery,
			"Test Webhook", "https://example.com/webhook", "secret", "ticket.created",
		)

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

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Success      bool   `json:"success"`
			Message      string `json:"message"`
			StatusCode   int    `json:"status_code"`
			ResponseTime int    `json:"response_time_ms"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotNil(t, response)
	})

	t.Run("Webhook Deliveries", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.GET("/api/v1/webhooks/:id/deliveries", HandleWebhookDeliveriesAPI)

		// Create a test webhook with deliveries
		insertQuery := `
			INSERT INTO webhooks (name, url, secret, events, active, create_by, change_by)
			VALUES ($1, $2, $3, $4, true, 1, 1)
			RETURNING id
		`
		webhookID := insertWebhookRow(t, insertQuery,
			"Delivery Test", "https://example.com/webhook", "secret", "ticket.created",
		)

		// Create test deliveries
		rawDeliveryQuery := `
			INSERT INTO webhook_deliveries (webhook_id, event_type, payload, status_code, attempts, delivered_at)
			VALUES 
				($1, 'ticket.created', '{"test": "data1"}', 200, 1, NOW()),
				($1, 'ticket.updated', '{"test": "data2"}', 500, 3, NULL)
		`
		deliveryQuery := database.ConvertPlaceholders(rawDeliveryQuery)
		deliveryArgs := []interface{}{webhookID}
		if database.IsMySQL() {
			deliveryArgs = database.RemapArgsForMySQL(rawDeliveryQuery, deliveryArgs)
		}
		_, err := db.Exec(deliveryQuery, deliveryArgs...)
		require.NoError(t, err)

		req := httptest.NewRequest("GET", "/api/v1/webhooks/"+strconv.Itoa(webhookID)+"/deliveries", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Deliveries []struct {
				ID         int    `json:"id"`
				EventType  string `json:"event_type"`
				StatusCode int    `json:"status_code"`
				Attempts   int    `json:"attempts"`
				Success    bool   `json:"success"`
			} `json:"deliveries"`
			Total int `json:"total"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotEmpty(t, response.Deliveries)
		assert.Equal(t, 2, response.Total)
	})

	t.Run("Retry Failed Delivery", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_id", 1)
			c.Next()
		})
		router.POST("/api/v1/webhooks/deliveries/:id/retry", HandleRetryWebhookDeliveryAPI)

		// Create a failed delivery
		var deliveryID int
		var webhookID int

		insertWebhook := `
			INSERT INTO webhooks (name, url, secret, events, active, create_by, change_by)
			VALUES ($1, $2, $3, $4, true, 1, 1)
			RETURNING id
		`
		webhookID = insertWebhookRow(t, insertWebhook,
			"Retry Test", "https://example.com/webhook", "secret", "ticket.created",
		)

		insertDelivery := `
			INSERT INTO webhook_deliveries (webhook_id, event_type, payload, status_code, attempts)
			VALUES ($1, 'ticket.created', '{"test": "retry"}', 500, 3)
			RETURNING id
		`
		deliveryID = insertWebhookRow(t, insertDelivery, webhookID)

		req := httptest.NewRequest("POST", "/api/v1/webhooks/deliveries/"+strconv.Itoa(deliveryID)+"/retry", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.NotNil(t, response)
	})

	t.Run("Unauthorized Access", func(t *testing.T) {
		router := gin.New()
		router.GET("/api/v1/webhooks", HandleListWebhooksAPI)

		req := httptest.NewRequest("GET", "/api/v1/webhooks", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
