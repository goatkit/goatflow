package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/webhooks"
)

// HandleRegisterWebhookAPI handles POST /api/v1/webhooks.
func HandleRegisterWebhookAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		Name           string            `json:"name" binding:"required"`
		URL            string            `json:"url" binding:"required,url"`
		Secret         string            `json:"secret"`
		Events         []string          `json:"events" binding:"required"`
		RetryCount     int               `json:"retry_count"`
		TimeoutSeconds int               `json:"timeout_seconds"`
		Headers        map[string]string `json:"headers"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set defaults
	if req.RetryCount == 0 {
		req.RetryCount = 3
	}
	if req.TimeoutSeconds == 0 {
		req.TimeoutSeconds = 30
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Convert events and headers to JSON strings for storage
	eventsJSON, err := json.Marshal(req.Events)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid events format"})
		return
	}
	headersJSON, err := json.Marshal(req.Headers)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid headers format"})
		return
	}

	// Insert webhook - adapter handles placeholder conversion and arg remapping for ? repeated
	insertQuery := `
		INSERT INTO webhooks (
			name, url, secret, events, active,
			retry_count, timeout_seconds, headers,
			create_time, create_by, change_time, change_by
		) VALUES (
			?, ?, ?, ?, true, ?, ?, ?,
			NOW(), ?, NOW(), ?
		) RETURNING id
	`
	// Args: name, url, secret, events, retry_count, timeout_seconds, headers, create_by, change_by
	webhookID64, err := database.GetAdapter().InsertWithReturning(db, insertQuery,
		req.Name, req.URL, req.Secret, string(eventsJSON),
		req.RetryCount, req.TimeoutSeconds, string(headersJSON),
		userID, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to register webhook",
			"details": err.Error(),
		})
		return
	}
	webhookID := int(webhookID64)

	c.JSON(http.StatusCreated, gin.H{
		"id":              webhookID,
		"name":            req.Name,
		"url":             req.URL,
		"events":          req.Events,
		"active":          true,
		"retry_count":     req.RetryCount,
		"timeout_seconds": req.TimeoutSeconds,
	})
}

// HandleListWebhooksAPI handles GET /api/v1/webhooks.
func HandleListWebhooksAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Build query
	query := database.ConvertPlaceholders(`
		SELECT id, name, url, events, active, retry_count, timeout_seconds, create_time
		FROM webhooks
		WHERE 1=1
	`)
	args := []interface{}{}

	// Filter by active status
	if activeFilter := c.Query("active"); activeFilter != "" {
		if activeFilter == "true" {
			query += database.ConvertPlaceholders(` AND active = true`)
		} else {
			query += database.ConvertPlaceholders(` AND active = false`)
		}
	}

	query += ` ORDER BY id DESC`

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch webhooks"})
		return
	}
	defer rows.Close()

	webhookList := []gin.H{}
	for rows.Next() {
		var webhook struct {
			ID             int
			Name           string
			URL            string
			EventsJSON     string
			Active         bool
			RetryCount     int
			TimeoutSeconds int
			CreateTime     time.Time
		}

		if err := rows.Scan(
			&webhook.ID, &webhook.Name, &webhook.URL, &webhook.EventsJSON,
			&webhook.Active, &webhook.RetryCount, &webhook.TimeoutSeconds,
			&webhook.CreateTime,
		); err != nil {
			continue
		}

		// Parse events JSON
		var events []string
		if err := json.Unmarshal([]byte(webhook.EventsJSON), &events); err != nil {
			events = []string{}
		}

		webhookList = append(webhookList, gin.H{
			"id":              webhook.ID,
			"name":            webhook.Name,
			"url":             webhook.URL,
			"events":          events,
			"active":          webhook.Active,
			"retry_count":     webhook.RetryCount,
			"timeout_seconds": webhook.TimeoutSeconds,
			"create_time":     webhook.CreateTime,
		})
	}
	if err := rows.Err(); err != nil {
		// Log or handle iteration errors
	}

	c.JSON(http.StatusOK, gin.H{
		"webhooks": webhookList,
		"total":    len(webhookList),
	})
}

// HandleGetWebhookAPI handles GET /api/v1/webhooks/:id.
func HandleGetWebhookAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID

	webhookID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	var webhook struct {
		ID             int
		Name           string
		URL            string
		Secret         sql.NullString
		EventsJSON     sql.NullString
		Active         bool
		RetryCount     int
		TimeoutSeconds int
		HeadersJSON    sql.NullString
		CreateTime     time.Time
	}

	query := database.ConvertPlaceholders(`
		SELECT id, name, url, secret, events, active, 
			   retry_count, timeout_seconds, headers, create_time
		FROM webhooks
		WHERE id = ?
	`)

	err = db.QueryRow(query, webhookID).Scan(
		&webhook.ID, &webhook.Name, &webhook.URL, &webhook.Secret,
		&webhook.EventsJSON, &webhook.Active, &webhook.RetryCount,
		&webhook.TimeoutSeconds, &webhook.HeadersJSON, &webhook.CreateTime,
	)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Webhook not found"})
		return
	}

	// Parse JSON fields
	var events []string
	if webhook.EventsJSON.Valid {
		if err := json.Unmarshal([]byte(webhook.EventsJSON.String), &events); err != nil {
			events = []string{}
		}
	}

	var headers map[string]string
	if webhook.HeadersJSON.Valid {
		if err := json.Unmarshal([]byte(webhook.HeadersJSON.String), &headers); err != nil {
			headers = map[string]string{}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"id":              webhook.ID,
		"name":            webhook.Name,
		"url":             webhook.URL,
		"events":          events,
		"active":          webhook.Active,
		"retry_count":     webhook.RetryCount,
		"timeout_seconds": webhook.TimeoutSeconds,
		"headers":         headers,
		"create_time":     webhook.CreateTime,
	})
}

// HandleUpdateWebhookAPI handles PUT /api/v1/webhooks/:id.
func HandleUpdateWebhookAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	webhookID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID"})
		return
	}

	var req struct {
		Name           string            `json:"name"`
		URL            string            `json:"url"`
		Secret         string            `json:"secret"`
		Events         []string          `json:"events"`
		Active         *bool             `json:"active"`
		RetryCount     int               `json:"retry_count"`
		TimeoutSeconds int               `json:"timeout_seconds"`
		Headers        map[string]string `json:"headers"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Check if webhook exists
	var count int
	checkQuery := database.ConvertPlaceholders(`SELECT 1 FROM webhooks WHERE id = ?`)
	if err := db.QueryRow(checkQuery, webhookID).Scan(&count); err != nil || count != 1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Webhook not found"})
		return
	}

	// Build update query dynamically
	updateParts := []string{"change_time = NOW()", "change_by = ?"}
	args := []interface{}{userID}
	paramCount := 1

	if req.Name != "" {
		paramCount++
		updateParts = append(updateParts, fmt.Sprintf("name = $%d", paramCount))
		args = append(args, req.Name)
	}
	if req.URL != "" {
		paramCount++
		updateParts = append(updateParts, fmt.Sprintf("url = $%d", paramCount))
		args = append(args, req.URL)
	}
	if req.Secret != "" {
		paramCount++
		updateParts = append(updateParts, fmt.Sprintf("secret = $%d", paramCount))
		args = append(args, req.Secret)
	}
	if len(req.Events) > 0 {
		eventsJSON, err := json.Marshal(req.Events)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid events format"})
			return
		}
		paramCount++
		updateParts = append(updateParts, fmt.Sprintf("events = $%d", paramCount))
		args = append(args, string(eventsJSON))
	}
	if req.Active != nil {
		paramCount++
		updateParts = append(updateParts, fmt.Sprintf("active = $%d", paramCount))
		args = append(args, *req.Active)
	}
	if req.RetryCount > 0 {
		paramCount++
		updateParts = append(updateParts, fmt.Sprintf("retry_count = $%d", paramCount))
		args = append(args, req.RetryCount)
	}
	if req.TimeoutSeconds > 0 {
		paramCount++
		updateParts = append(updateParts, fmt.Sprintf("timeout_seconds = $%d", paramCount))
		args = append(args, req.TimeoutSeconds)
	}
	if req.Headers != nil {
		headersJSON, err := json.Marshal(req.Headers)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid headers format"})
			return
		}
		paramCount++
		updateParts = append(updateParts, fmt.Sprintf("headers = $%d", paramCount))
		args = append(args, string(headersJSON))
	}

	paramCount++
	args = append(args, webhookID)

	updateQuery := database.ConvertPlaceholders(
		fmt.Sprintf("UPDATE webhooks SET %s WHERE id = $%d",
			strings.Join(updateParts, ", "), paramCount),
	)

	_, err = db.Exec(updateQuery, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update webhook"})
		return
	}

	// Return updated webhook
	c.JSON(http.StatusOK, gin.H{
		"id":      webhookID,
		"name":    req.Name,
		"url":     req.URL,
		"events":  req.Events,
		"active":  req.Active,
		"message": "Webhook updated successfully",
	})
}

// HandleDeleteWebhookAPI handles DELETE /api/v1/webhooks/:id.
func HandleDeleteWebhookAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID

	webhookID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Delete webhook and its deliveries
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer func() { _ = tx.Rollback() }()

	// Delete deliveries first
	deleteDeliveriesQuery := database.ConvertPlaceholders(`
		DELETE FROM webhook_deliveries WHERE webhook_id = ?
	`)
	if _, err := tx.Exec(deleteDeliveriesQuery, webhookID); err != nil {
		// Non-fatal, continue with webhook deletion
	}

	// Delete webhook
	deleteWebhookQuery := database.ConvertPlaceholders(`
		DELETE FROM webhooks WHERE id = ?
	`)
	result, err := tx.Exec(deleteWebhookQuery, webhookID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete webhook"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Webhook not found"})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Webhook deleted successfully",
		"id":      webhookID,
	})
}

// HandleTestWebhookAPI handles POST /api/v1/webhooks/:id/test.
func HandleTestWebhookAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID

	webhookID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID"})
		return
	}

	var req struct {
		EventType   string                 `json:"event_type"`
		TestPayload map[string]interface{} `json:"test_payload"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		// Use default test payload
		req.EventType = "test.webhook"
		req.TestPayload = map[string]interface{}{
			"message":   "This is a test webhook delivery",
			"timestamp": time.Now(),
		}
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Get webhook details
	var webhook struct {
		URL            string
		Secret         sql.NullString
		TimeoutSeconds int
		HeadersJSON    sql.NullString
	}

	query := database.ConvertPlaceholders(`
		SELECT url, secret, timeout_seconds, headers
		FROM webhooks
		WHERE id = ?
	`)

	err = db.QueryRow(query, webhookID).Scan(
		&webhook.URL, &webhook.Secret, &webhook.TimeoutSeconds, &webhook.HeadersJSON,
	)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Webhook not found"})
		return
	}

	// Parse headers
	var headers map[string]string
	if webhook.HeadersJSON.Valid {
		if err := json.Unmarshal([]byte(webhook.HeadersJSON.String), &headers); err != nil {
			headers = map[string]string{}
		}
	}

	// Prepare payload
	payload := webhooks.WebhookPayload{
		Event:     webhooks.EventType(req.EventType),
		Timestamp: time.Now(),
		Data:      req.TestPayload,
	}

	// Generate signature if secret is configured
	if webhook.Secret.Valid && webhook.Secret.String != "" {
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal payload"})
			return
		}
		h := hmac.New(sha256.New, []byte(webhook.Secret.String))
		h.Write(payloadJSON)
		payload.Signature = hex.EncodeToString(h.Sum(nil))
	}

	// Send test webhook
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal payload"})
		return
	}
	client := &http.Client{
		Timeout: time.Duration(webhook.TimeoutSeconds) * time.Second,
	}

	startTime := time.Now()
	httpReq, err := http.NewRequest("POST", webhook.URL, bytes.NewBuffer(payloadJSON))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Add custom headers
	for key, value := range headers {
		httpReq.Header.Set(key, value)
	}

	// Add signature header if available
	if payload.Signature != "" {
		httpReq.Header.Set("X-Webhook-Signature", payload.Signature)
	}

	resp, err := client.Do(httpReq)
	responseTime := time.Since(startTime).Milliseconds()

	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success":          false,
			"message":          "Failed to deliver test webhook",
			"error":            err.Error(),
			"status_code":      0,
			"response":         "",
			"response_time_ms": responseTime,
		})
		return
	}
	defer resp.Body.Close()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		responseBody = []byte("Failed to read response body")
	}

	c.JSON(http.StatusOK, gin.H{
		"success":          resp.StatusCode >= 200 && resp.StatusCode < 300,
		"message":          "Test webhook delivered",
		"status_code":      resp.StatusCode,
		"response":         string(responseBody),
		"response_time_ms": responseTime,
	})
}

// HandleWebhookDeliveriesAPI handles GET /api/v1/webhooks/:id/deliveries.
func HandleWebhookDeliveriesAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID

	webhookID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Get deliveries
	query := database.ConvertPlaceholders(`
		SELECT id, event_type, status_code, attempts, delivered_at, created_at
		FROM webhook_deliveries
		WHERE webhook_id = ?
		ORDER BY created_at DESC
		LIMIT 100
	`)

	rows, err := db.Query(query, webhookID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch deliveries"})
		return
	}
	defer rows.Close()

	deliveries := []gin.H{}
	for rows.Next() {
		var delivery struct {
			ID          int
			EventType   string
			StatusCode  int
			Attempts    int
			DeliveredAt *time.Time
			CreatedAt   time.Time
		}

		if err := rows.Scan(
			&delivery.ID, &delivery.EventType, &delivery.StatusCode,
			&delivery.Attempts, &delivery.DeliveredAt, &delivery.CreatedAt,
		); err != nil {
			continue
		}

		deliveryData := gin.H{
			"id":          delivery.ID,
			"event_type":  delivery.EventType,
			"status_code": delivery.StatusCode,
			"attempts":    delivery.Attempts,
			"success":     delivery.StatusCode >= 200 && delivery.StatusCode < 300,
			"created_at":  delivery.CreatedAt,
		}

		if delivery.DeliveredAt != nil {
			deliveryData["delivered_at"] = delivery.DeliveredAt
		}

		deliveries = append(deliveries, deliveryData)
	}
	if err := rows.Err(); err != nil {
		// Log or handle iteration errors
	}

	c.JSON(http.StatusOK, gin.H{
		"deliveries": deliveries,
		"total":      len(deliveries),
	})
}

// HandleRetryWebhookDeliveryAPI handles POST /api/v1/webhooks/deliveries/:id/retry.
func HandleRetryWebhookDeliveryAPI(c *gin.Context) {
	// Check authentication
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	_ = userID

	deliveryID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid delivery ID"})
		return
	}

	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
		return
	}

	// Get delivery and webhook details
	var delivery struct {
		WebhookID int
		Payload   string
		EventType string
	}

	query := database.ConvertPlaceholders(`
		SELECT webhook_id, payload, event_type
		FROM webhook_deliveries
		WHERE id = ?
	`)

	err = db.QueryRow(query, deliveryID).Scan(
		&delivery.WebhookID, &delivery.Payload, &delivery.EventType,
	)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Delivery not found"})
		return
	}

	// Queue for retry (in production, this would use a job queue)
	// For now, just update the retry time
	nextRetryExpr := database.GetAdapter().IntervalAdd("NOW()", 1, "MINUTE")

	updateQuery := fmt.Sprintf(`
		UPDATE webhook_deliveries
		SET next_retry = %s,
			attempts = attempts + 1
		WHERE id = ?
	`, nextRetryExpr)

	_, err = database.GetAdapter().Exec(db, updateQuery, deliveryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to queue retry"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     "Webhook delivery queued for retry",
		"delivery_id": deliveryID,
	})
}
