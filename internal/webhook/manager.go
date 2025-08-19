package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager handles webhook operations
type Manager struct {
	webhooks      map[uint]*Webhook
	deliveryQueue chan *WebhookDelivery
	workers       int
	client        *http.Client
	mutex         sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	
	// Repository interfaces (would be injected)
	webhookRepo  WebhookRepository
	deliveryRepo DeliveryRepository
}

// WebhookRepository interface for webhook storage operations
type WebhookRepository interface {
	Create(webhook *Webhook) error
	GetByID(id uint) (*Webhook, error)
	List() ([]*Webhook, error)
	ListActive() ([]*Webhook, error)
	Update(webhook *Webhook) error
	Delete(id uint) error
	UpdateStatistics(id uint, stats WebhookStatistics) error
}

// DeliveryRepository interface for webhook delivery storage operations
type DeliveryRepository interface {
	Create(delivery *WebhookDelivery) error
	GetByID(id uint) (*WebhookDelivery, error)
	ListByWebhookID(webhookID uint, limit int) ([]*WebhookDelivery, error)
	Update(delivery *WebhookDelivery) error
	GetPendingRetries() ([]*WebhookDelivery, error)
	CleanupOldDeliveries(olderThan time.Time) error
}

// NewManager creates a new webhook manager
func NewManager(webhookRepo WebhookRepository, deliveryRepo DeliveryRepository) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	
	manager := &Manager{
		webhooks:      make(map[uint]*Webhook),
		deliveryQueue: make(chan *WebhookDelivery, 1000),
		workers:       5, // Default number of worker goroutines
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		ctx:          ctx,
		cancel:       cancel,
		webhookRepo:  webhookRepo,
		deliveryRepo: deliveryRepo,
	}
	
	// Load existing webhooks
	manager.loadWebhooks()
	
	// Start worker goroutines
	for i := 0; i < manager.workers; i++ {
		go manager.worker()
	}
	
	// Start retry processor
	go manager.retryProcessor()
	
	// Start cleanup routine
	go manager.cleanupRoutine()
	
	return manager
}

// Stop gracefully shuts down the webhook manager
func (m *Manager) Stop() {
	m.cancel()
	close(m.deliveryQueue)
}

// loadWebhooks loads all active webhooks from storage
func (m *Manager) loadWebhooks() error {
	webhooks, err := m.webhookRepo.ListActive()
	if err != nil {
		return fmt.Errorf("failed to load webhooks: %w", err)
	}
	
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	for _, webhook := range webhooks {
		m.webhooks[webhook.ID] = webhook
	}
	
	return nil
}

// CreateWebhook creates a new webhook
func (m *Manager) CreateWebhook(req WebhookRequest) (*Webhook, error) {
	webhook := &Webhook{
		Name:        req.Name,
		URL:         req.URL,
		Secret:      req.Secret,
		Events:      req.Events,
		Status:      StatusActive,
		Description: req.Description,
		Headers:     req.Headers,
		Filters:     req.Filters,
		RetryCount:  req.RetryCount,
		Timeout:     time.Duration(req.Timeout) * time.Second,
		RetryInterval: time.Duration(req.RetryInterval) * time.Second,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	// Set defaults
	if webhook.RetryCount == 0 {
		webhook.RetryCount = 3
	}
	if webhook.Timeout == 0 {
		webhook.Timeout = 30 * time.Second
	}
	if webhook.RetryInterval == 0 {
		webhook.RetryInterval = 60 * time.Second
	}
	
	err := m.webhookRepo.Create(webhook)
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook: %w", err)
	}
	
	m.mutex.Lock()
	m.webhooks[webhook.ID] = webhook
	m.mutex.Unlock()
	
	return webhook, nil
}

// GetWebhook retrieves a webhook by ID
func (m *Manager) GetWebhook(id uint) (*Webhook, error) {
	m.mutex.RLock()
	webhook, exists := m.webhooks[id]
	m.mutex.RUnlock()
	
	if exists {
		return webhook, nil
	}
	
	return m.webhookRepo.GetByID(id)
}

// ListWebhooks returns all webhooks
func (m *Manager) ListWebhooks() ([]*Webhook, error) {
	return m.webhookRepo.List()
}

// UpdateWebhook updates an existing webhook
func (m *Manager) UpdateWebhook(id uint, req WebhookRequest) (*Webhook, error) {
	webhook, err := m.GetWebhook(id)
	if err != nil {
		return nil, err
	}
	
	// Update fields
	webhook.Name = req.Name
	webhook.URL = req.URL
	webhook.Events = req.Events
	webhook.Description = req.Description
	webhook.Headers = req.Headers
	webhook.Filters = req.Filters
	webhook.UpdatedAt = time.Now()
	
	// Update secret only if provided
	if req.Secret != "" {
		webhook.Secret = req.Secret
	}
	
	// Update configuration
	if req.RetryCount > 0 {
		webhook.RetryCount = req.RetryCount
	}
	if req.Timeout > 0 {
		webhook.Timeout = time.Duration(req.Timeout) * time.Second
	}
	if req.RetryInterval > 0 {
		webhook.RetryInterval = time.Duration(req.RetryInterval) * time.Second
	}
	
	err = m.webhookRepo.Update(webhook)
	if err != nil {
		return nil, fmt.Errorf("failed to update webhook: %w", err)
	}
	
	m.mutex.Lock()
	m.webhooks[id] = webhook
	m.mutex.Unlock()
	
	return webhook, nil
}

// DeleteWebhook removes a webhook
func (m *Manager) DeleteWebhook(id uint) error {
	err := m.webhookRepo.Delete(id)
	if err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}
	
	m.mutex.Lock()
	delete(m.webhooks, id)
	m.mutex.Unlock()
	
	return nil
}

// TriggerEvent triggers webhooks for a specific event
func (m *Manager) TriggerEvent(event WebhookEvent, data interface{}, previousData interface{}) error {
	m.mutex.RLock()
	relevantWebhooks := make([]*Webhook, 0)
	
	for _, webhook := range m.webhooks {
		if m.shouldTriggerWebhook(webhook, event, data) {
			relevantWebhooks = append(relevantWebhooks, webhook)
		}
	}
	m.mutex.RUnlock()
	
	for _, webhook := range relevantWebhooks {
		delivery := m.createDelivery(webhook, event, data, previousData)
		select {
		case m.deliveryQueue <- delivery:
			// Successfully queued
		case <-m.ctx.Done():
			return fmt.Errorf("webhook manager is shutting down")
		default:
			// Queue is full, log error
			return fmt.Errorf("webhook delivery queue is full")
		}
	}
	
	return nil
}

// shouldTriggerWebhook determines if a webhook should be triggered for an event
func (m *Manager) shouldTriggerWebhook(webhook *Webhook, event WebhookEvent, data interface{}) bool {
	// Check if webhook is active
	if webhook.Status != StatusActive {
		return false
	}
	
	// Check if event is in webhook's event list
	eventMatches := false
	for _, webhookEvent := range webhook.Events {
		if webhookEvent == event {
			eventMatches = true
			break
		}
	}
	
	if !eventMatches {
		return false
	}
	
	// Apply filters
	return m.applyFilters(webhook.Filters, data)
}

// applyFilters checks if the data matches the webhook filters
func (m *Manager) applyFilters(filters WebhookFilters, data interface{}) bool {
	// If no filters are defined, trigger for all events
	if len(filters.QueueIDs) == 0 && len(filters.Priorities) == 0 && 
	   len(filters.Statuses) == 0 && len(filters.UserIDs) == 0 {
		return true
	}
	
	// Apply filters based on data type
	switch eventData := data.(type) {
	case TicketWebhookData:
		// Check queue filter
		if len(filters.QueueIDs) > 0 {
			queueMatch := false
			for _, queueID := range filters.QueueIDs {
				if queueID == eventData.QueueID {
					queueMatch = true
					break
				}
			}
			if !queueMatch {
				return false
			}
		}
		
		// Check priority filter
		if len(filters.Priorities) > 0 {
			priorityMatch := false
			for _, priority := range filters.Priorities {
				if priority == eventData.Priority {
					priorityMatch = true
					break
				}
			}
			if !priorityMatch {
				return false
			}
		}
		
		// Check status filter
		if len(filters.Statuses) > 0 {
			statusMatch := false
			for _, status := range filters.Statuses {
				if status == eventData.Status {
					statusMatch = true
					break
				}
			}
			if !statusMatch {
				return false
			}
		}
		
		// Check user filter (assigned user)
		if len(filters.UserIDs) > 0 && eventData.AssignedTo != nil {
			userMatch := false
			for _, userID := range filters.UserIDs {
				if userID == *eventData.AssignedTo {
					userMatch = true
					break
				}
			}
			if !userMatch {
				return false
			}
		}
		
	case UserWebhookData:
		// Check user filter
		if len(filters.UserIDs) > 0 {
			userMatch := false
			for _, userID := range filters.UserIDs {
				if userID == eventData.ID {
					userMatch = true
					break
				}
			}
			if !userMatch {
				return false
			}
		}
	}
	
	return true
}

// createDelivery creates a new webhook delivery
func (m *Manager) createDelivery(webhook *Webhook, event WebhookEvent, data interface{}, previousData interface{}) *WebhookDelivery {
	payload := WebhookPayload{
		Event:     event,
		Timestamp: time.Now(),
		ID:        uuid.New().String(),
		Source: struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			URL     string `json:"url"`
		}{
			Name:    "GOTRS",
			Version: "1.0.0",
			URL:     "https://github.com/gotrs-io/gotrs-ce",
		},
		Data:         data,
		PreviousData: previousData,
	}
	
	return &WebhookDelivery{
		WebhookID:    webhook.ID,
		Event:        event,
		Payload:      payload,
		Status:       DeliveryPending,
		RequestURL:   webhook.URL,
		AttemptCount: 0,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// worker processes webhook deliveries from the queue
func (m *Manager) worker() {
	for {
		select {
		case delivery := <-m.deliveryQueue:
			if delivery != nil {
				m.processDelivery(delivery)
			}
		case <-m.ctx.Done():
			return
		}
	}
}

// processDelivery processes a single webhook delivery
func (m *Manager) processDelivery(delivery *WebhookDelivery) {
	webhook, err := m.GetWebhook(delivery.WebhookID)
	if err != nil {
		delivery.Status = DeliveryFailed
		delivery.ErrorMessage = fmt.Sprintf("Failed to get webhook: %v", err)
		m.deliveryRepo.Update(delivery)
		return
	}
	
	// Create HTTP request
	payloadBytes, err := json.Marshal(delivery.Payload)
	if err != nil {
		delivery.Status = DeliveryFailed
		delivery.ErrorMessage = fmt.Sprintf("Failed to marshal payload: %v", err)
		m.deliveryRepo.Update(delivery)
		return
	}
	
	delivery.RequestBody = string(payloadBytes)
	delivery.AttemptCount++
	
	req, err := http.NewRequestWithContext(m.ctx, "POST", webhook.URL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		delivery.Status = DeliveryFailed
		delivery.ErrorMessage = fmt.Sprintf("Failed to create request: %v", err)
		m.deliveryRepo.Update(delivery)
		return
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GOTRS-Webhook/1.0")
	req.Header.Set("X-Webhook-ID", fmt.Sprintf("%d", webhook.ID))
	req.Header.Set("X-Webhook-Event", string(delivery.Event))
	req.Header.Set("X-Webhook-Delivery", delivery.ID)
	
	// Add custom headers
	for key, value := range webhook.Headers {
		req.Header.Set(key, value)
	}
	
	// Add HMAC signature if secret is configured
	if webhook.Secret != "" {
		signature := m.generateSignature(payloadBytes, webhook.Secret)
		req.Header.Set("X-Webhook-Signature", signature)
	}
	
	// Store request headers
	delivery.RequestHeaders = make(map[string]string)
	for key, values := range req.Header {
		if len(values) > 0 {
			delivery.RequestHeaders[key] = values[0]
		}
	}
	
	// Make request
	start := time.Now()
	
	client := &http.Client{Timeout: webhook.Timeout}
	resp, err := client.Do(req)
	
	delivery.Duration = time.Since(start)
	delivery.UpdatedAt = time.Now()
	
	if err != nil {
		delivery.Status = DeliveryFailed
		delivery.ErrorMessage = err.Error()
		m.scheduleRetry(webhook, delivery)
	} else {
		defer resp.Body.Close()
		
		delivery.ResponseStatusCode = resp.StatusCode
		
		// Store response headers
		delivery.ResponseHeaders = make(map[string]string)
		for key, values := range resp.Header {
			if len(values) > 0 {
				delivery.ResponseHeaders[key] = values[0]
			}
		}
		
		// Read response body
		bodyBytes, err := io.ReadAll(resp.Body)
		if err == nil {
			delivery.ResponseBody = string(bodyBytes)
		}
		
		// Check if delivery was successful
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			delivery.Status = DeliverySuccess
			m.updateWebhookStats(webhook, true, delivery.Duration)
		} else {
			delivery.Status = DeliveryFailed
			delivery.ErrorMessage = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)
			m.scheduleRetry(webhook, delivery)
			m.updateWebhookStats(webhook, false, delivery.Duration)
		}
	}
	
	// Save delivery record
	if delivery.ID == 0 {
		m.deliveryRepo.Create(delivery)
	} else {
		m.deliveryRepo.Update(delivery)
	}
}

// generateSignature generates HMAC-SHA256 signature for webhook payload
func (m *Manager) generateSignature(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

// scheduleRetry schedules a retry for a failed delivery
func (m *Manager) scheduleRetry(webhook *Webhook, delivery *WebhookDelivery) {
	if delivery.AttemptCount >= webhook.RetryCount {
		delivery.Status = DeliveryExpired
		return
	}
	
	delivery.Status = DeliveryRetrying
	
	// Calculate next retry time with exponential backoff
	backoffMultiplier := time.Duration(delivery.AttemptCount * delivery.AttemptCount)
	nextRetry := time.Now().Add(webhook.RetryInterval * backoffMultiplier)
	delivery.NextRetryAt = &nextRetry
}

// retryProcessor handles retrying failed deliveries
func (m *Manager) retryProcessor() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			m.processRetries()
		case <-m.ctx.Done():
			return
		}
	}
}

// processRetries finds and processes deliveries that are ready for retry
func (m *Manager) processRetries() {
	retries, err := m.deliveryRepo.GetPendingRetries()
	if err != nil {
		return
	}
	
	for _, delivery := range retries {
		if delivery.NextRetryAt != nil && time.Now().After(*delivery.NextRetryAt) {
			select {
			case m.deliveryQueue <- delivery:
				// Successfully queued for retry
			case <-m.ctx.Done():
				return
			default:
				// Queue is full, will try again next time
			}
		}
	}
}

// updateWebhookStats updates webhook delivery statistics
func (m *Manager) updateWebhookStats(webhook *Webhook, success bool, duration time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	webhook.TotalDeliveries++
	
	now := time.Now()
	webhook.LastDeliveryAt = &now
	
	if success {
		webhook.SuccessfulDeliveries++
		webhook.LastSuccessAt = &now
	} else {
		webhook.FailedDeliveries++
		webhook.LastFailureAt = &now
	}
	
	// Update in database (async)
	go func() {
		stats := WebhookStatistics{
			WebhookID:            webhook.ID,
			TotalDeliveries:      webhook.TotalDeliveries,
			SuccessfulDeliveries: webhook.SuccessfulDeliveries,
			FailedDeliveries:     webhook.FailedDeliveries,
			LastDeliveryAt:       webhook.LastDeliveryAt,
			LastSuccessAt:        webhook.LastSuccessAt,
			LastFailureAt:        webhook.LastFailureAt,
		}
		
		if webhook.TotalDeliveries > 0 {
			stats.SuccessRate = float64(webhook.SuccessfulDeliveries) / float64(webhook.TotalDeliveries) * 100
		}
		
		m.webhookRepo.UpdateStatistics(webhook.ID, stats)
	}()
}

// cleanupRoutine periodically cleans up old delivery records
func (m *Manager) cleanupRoutine() {
	ticker := time.NewTicker(24 * time.Hour) // Run daily
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Clean up deliveries older than 30 days
			cutoff := time.Now().AddDate(0, 0, -30)
			m.deliveryRepo.CleanupOldDeliveries(cutoff)
		case <-m.ctx.Done():
			return
		}
	}
}

// TestWebhook tests a webhook by sending a test payload
func (m *Manager) TestWebhook(webhookID uint) (*WebhookTestResult, error) {
	webhook, err := m.GetWebhook(webhookID)
	if err != nil {
		return nil, err
	}
	
	// Create test payload
	testPayload := WebhookPayload{
		Event:     "system.test",
		Timestamp: time.Now(),
		ID:        uuid.New().String(),
		Source: struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			URL     string `json:"url"`
		}{
			Name:    "GOTRS",
			Version: "1.0.0",
			URL:     "https://github.com/gotrs-io/gotrs-ce",
		},
		Data: map[string]interface{}{
			"test": true,
			"message": "This is a test webhook delivery",
		},
	}
	
	payloadBytes, err := json.Marshal(testPayload)
	if err != nil {
		return &WebhookTestResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to marshal test payload: %v", err),
			TestedAt:     time.Now(),
		}, nil
	}
	
	req, err := http.NewRequestWithContext(context.Background(), "POST", webhook.URL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return &WebhookTestResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to create request: %v", err),
			TestedAt:     time.Now(),
		}, nil
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GOTRS-Webhook/1.0")
	req.Header.Set("X-Webhook-Test", "true")
	
	// Add custom headers
	for key, value := range webhook.Headers {
		req.Header.Set(key, value)
	}
	
	// Add signature if secret is configured
	if webhook.Secret != "" {
		signature := m.generateSignature(payloadBytes, webhook.Secret)
		req.Header.Set("X-Webhook-Signature", signature)
	}
	
	// Make request
	start := time.Now()
	client := &http.Client{Timeout: webhook.Timeout}
	resp, err := client.Do(req)
	responseTime := time.Since(start)
	
	result := &WebhookTestResult{
		ResponseTime: responseTime,
		TestedAt:     time.Now(),
	}
	
	if err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result, nil
	}
	
	defer resp.Body.Close()
	result.StatusCode = resp.StatusCode
	
	bodyBytes, err := io.ReadAll(resp.Body)
	if err == nil {
		result.ResponseBody = string(bodyBytes)
	}
	
	result.Success = resp.StatusCode >= 200 && resp.StatusCode < 300
	if !result.Success {
		result.ErrorMessage = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}
	
	return result, nil
}

// GetDeliveries returns delivery history for a webhook
func (m *Manager) GetDeliveries(webhookID uint, limit int) ([]*WebhookDelivery, error) {
	return m.deliveryRepo.ListByWebhookID(webhookID, limit)
}

// GetWebhookStatistics returns statistics for a webhook
func (m *Manager) GetWebhookStatistics(webhookID uint) (*WebhookStatistics, error) {
	webhook, err := m.GetWebhook(webhookID)
	if err != nil {
		return nil, err
	}
	
	stats := &WebhookStatistics{
		WebhookID:            webhook.ID,
		TotalDeliveries:      webhook.TotalDeliveries,
		SuccessfulDeliveries: webhook.SuccessfulDeliveries,
		FailedDeliveries:     webhook.FailedDeliveries,
		LastDeliveryAt:       webhook.LastDeliveryAt,
		LastSuccessAt:        webhook.LastSuccessAt,
		LastFailureAt:        webhook.LastFailureAt,
	}
	
	if webhook.TotalDeliveries > 0 {
		stats.SuccessRate = float64(webhook.SuccessfulDeliveries) / float64(webhook.TotalDeliveries) * 100
	}
	
	return stats, nil
}