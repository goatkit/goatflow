package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

// WebhookService handles webhook operations
type WebhookService struct {
	webhookRepo    repository.WebhookRepository
	httpClient     *http.Client
	mu             sync.RWMutex
	activeWebhooks map[int]*models.Webhook
	retryQueue     chan *WebhookDelivery
	workers        int
	stopChan       chan bool
}

// WebhookDelivery represents a webhook delivery attempt
type WebhookDelivery struct {
	WebhookID    int                    `json:"webhook_id"`
	URL          string                 `json:"url"`
	Method       string                 `json:"method"`
	Headers      map[string]string      `json:"headers"`
	Payload      interface{}            `json:"payload"`
	Event        string                 `json:"event"`
	Signature    string                 `json:"signature"`
	Timestamp    time.Time              `json:"timestamp"`
	RetryCount   int                    `json:"retry_count"`
	MaxRetries   int                    `json:"max_retries"`
	NextRetry    time.Time              `json:"next_retry"`
}

// WebhookLog represents a webhook delivery log
type WebhookLog struct {
	ID           int                    `json:"id"`
	WebhookID    int                    `json:"webhook_id"`
	Event        string                 `json:"event"`
	URL          string                 `json:"url"`
	Method       string                 `json:"method"`
	Headers      map[string]string      `json:"headers"`
	Payload      json.RawMessage        `json:"payload"`
	Response     string                 `json:"response"`
	StatusCode   int                    `json:"status_code"`
	Success      bool                   `json:"success"`
	Error        string                 `json:"error,omitempty"`
	Duration     int64                  `json:"duration_ms"`
	RetryCount   int                    `json:"retry_count"`
	DeliveredAt  time.Time              `json:"delivered_at"`
}

// WebhookEvent represents different webhook events
type WebhookEvent string

const (
	WebhookEventTicketCreated        WebhookEvent = "ticket.created"
	WebhookEventTicketUpdated        WebhookEvent = "ticket.updated"
	WebhookEventTicketAssigned       WebhookEvent = "ticket.assigned"
	WebhookEventTicketClosed         WebhookEvent = "ticket.closed"
	WebhookEventTicketReopened       WebhookEvent = "ticket.reopened"
	WebhookEventTicketEscalated      WebhookEvent = "ticket.escalated"
	WebhookEventMessageCreated       WebhookEvent = "message.created"
	WebhookEventNoteAdded           WebhookEvent = "note.added"
	WebhookEventAttachmentUploaded   WebhookEvent = "attachment.uploaded"
	WebhookEventSLAWarning          WebhookEvent = "sla.warning"
	WebhookEventSLABreach           WebhookEvent = "sla.breach"
	WebhookEventWorkflowTriggered    WebhookEvent = "workflow.triggered"
	WebhookEventWorkflowCompleted    WebhookEvent = "workflow.completed"
	WebhookEventUserCreated         WebhookEvent = "user.created"
	WebhookEventUserUpdated         WebhookEvent = "user.updated"
	WebhookEventOrganizationCreated WebhookEvent = "organization.created"
	WebhookEventOrganizationUpdated WebhookEvent = "organization.updated"
)

// NewWebhookService creates a new webhook service
func NewWebhookService(webhookRepo repository.WebhookRepository) *WebhookService {
	ws := &WebhookService{
		webhookRepo: webhookRepo,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		activeWebhooks: make(map[int]*models.Webhook),
		retryQueue:     make(chan *WebhookDelivery, 1000),
		workers:        5,
		stopChan:       make(chan bool),
	}
	
	// Start workers
	ws.startWorkers()
	
	// Load active webhooks
	ws.loadActiveWebhooks()
	
	return ws
}

// CreateWebhook creates a new webhook
func (s *WebhookService) CreateWebhook(webhook *models.Webhook) error {
	// Validate webhook
	if err := s.validateWebhook(webhook); err != nil {
		return err
	}
	
	// Generate secret if not provided
	if webhook.Secret == "" {
		webhook.Secret = s.generateSecret()
	}
	
	// Set defaults
	webhook.IsActive = true
	webhook.CreatedAt = time.Now()
	webhook.UpdatedAt = time.Now()
	webhook.LastTriggered = nil
	webhook.FailureCount = 0
	
	// Save to repository
	if err := s.webhookRepo.Create(webhook); err != nil {
		return err
	}
	
	// Add to active webhooks if enabled
	if webhook.IsActive {
		s.mu.Lock()
		s.activeWebhooks[webhook.ID] = webhook
		s.mu.Unlock()
	}
	
	return nil
}

// UpdateWebhook updates an existing webhook
func (s *WebhookService) UpdateWebhook(id int, webhook *models.Webhook) error {
	existing, err := s.webhookRepo.GetByID(id)
	if err != nil {
		return err
	}
	
	// Validate webhook
	if err := s.validateWebhook(webhook); err != nil {
		return err
	}
	
	webhook.ID = id
	webhook.UpdatedAt = time.Now()
	
	// Update in repository
	if err := s.webhookRepo.Update(webhook); err != nil {
		return err
	}
	
	// Update active webhooks cache
	s.mu.Lock()
	if webhook.IsActive {
		s.activeWebhooks[webhook.ID] = webhook
	} else {
		delete(s.activeWebhooks, webhook.ID)
	}
	s.mu.Unlock()
	
	return nil
}

// DeleteWebhook deletes a webhook
func (s *WebhookService) DeleteWebhook(id int) error {
	if err := s.webhookRepo.Delete(id); err != nil {
		return err
	}
	
	// Remove from active webhooks
	s.mu.Lock()
	delete(s.activeWebhooks, id)
	s.mu.Unlock()
	
	return nil
}

// TriggerWebhook triggers webhooks for an event
func (s *WebhookService) TriggerWebhook(event WebhookEvent, payload interface{}) error {
	s.mu.RLock()
	webhooks := make([]*models.Webhook, 0)
	for _, webhook := range s.activeWebhooks {
		if s.shouldTrigger(webhook, string(event)) {
			webhooks = append(webhooks, webhook)
		}
	}
	s.mu.RUnlock()
	
	// Prepare payload
	payloadData := map[string]interface{}{
		"event":     event,
		"timestamp": time.Now().Unix(),
		"data":      payload,
	}
	
	// Trigger each webhook
	for _, webhook := range webhooks {
		delivery := &WebhookDelivery{
			WebhookID:  webhook.ID,
			URL:        webhook.URL,
			Method:     webhook.Method,
			Headers:    webhook.Headers,
			Payload:    payloadData,
			Event:      string(event),
			Timestamp:  time.Now(),
			MaxRetries: webhook.MaxRetries,
		}
		
		// Add signature if secret is configured
		if webhook.Secret != "" {
			delivery.Signature = s.generateSignature(webhook.Secret, payloadData)
		}
		
		// Queue for delivery
		select {
		case s.retryQueue <- delivery:
		default:
			// Queue is full, deliver synchronously
			go s.deliverWebhook(delivery)
		}
	}
	
	return nil
}

// deliverWebhook delivers a webhook
func (s *WebhookService) deliverWebhook(delivery *WebhookDelivery) {
	startTime := time.Now()
	
	// Prepare request body
	bodyBytes, err := json.Marshal(delivery.Payload)
	if err != nil {
		s.logDelivery(delivery, 0, "", err.Error(), time.Since(startTime).Milliseconds(), false)
		return
	}
	
	// Create request
	req, err := http.NewRequest(delivery.Method, delivery.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		s.logDelivery(delivery, 0, "", err.Error(), time.Since(startTime).Milliseconds(), false)
		return
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GOTRS-Webhook/1.0")
	req.Header.Set("X-GOTRS-Event", delivery.Event)
	req.Header.Set("X-GOTRS-Delivery", fmt.Sprintf("%d-%d", delivery.WebhookID, time.Now().Unix()))
	
	if delivery.Signature != "" {
		req.Header.Set("X-GOTRS-Signature", delivery.Signature)
	}
	
	for key, value := range delivery.Headers {
		req.Header.Set(key, value)
	}
	
	// Send request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logDelivery(delivery, 0, "", err.Error(), time.Since(startTime).Milliseconds(), false)
		s.scheduleRetry(delivery)
		return
	}
	defer resp.Body.Close()
	
	// Read response
	responseBody, _ := io.ReadAll(resp.Body)
	
	// Check if successful (2xx status code)
	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	
	// Log delivery
	s.logDelivery(delivery, resp.StatusCode, string(responseBody), "", time.Since(startTime).Milliseconds(), success)
	
	// Schedule retry if failed
	if !success && delivery.RetryCount < delivery.MaxRetries {
		s.scheduleRetry(delivery)
	} else if !success {
		// Mark webhook as failed after max retries
		s.markWebhookFailed(delivery.WebhookID)
	}
}

// scheduleRetry schedules a webhook retry
func (s *WebhookService) scheduleRetry(delivery *WebhookDelivery) {
	delivery.RetryCount++
	
	// Calculate backoff delay (exponential backoff)
	delay := time.Duration(delivery.RetryCount*delivery.RetryCount) * time.Minute
	if delay > 60*time.Minute {
		delay = 60 * time.Minute
	}
	
	delivery.NextRetry = time.Now().Add(delay)
	
	// Schedule retry
	time.AfterFunc(delay, func() {
		select {
		case s.retryQueue <- delivery:
		default:
			// Queue is full, drop retry
		}
	})
}

// markWebhookFailed marks a webhook as failed
func (s *WebhookService) markWebhookFailed(webhookID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if webhook, exists := s.activeWebhooks[webhookID]; exists {
		webhook.FailureCount++
		webhook.LastError = time.Now()
		
		// Disable webhook after 5 consecutive failures
		if webhook.FailureCount >= 5 {
			webhook.IsActive = false
			delete(s.activeWebhooks, webhookID)
			
			// Update in repository
			_ = s.webhookRepo.Update(webhook)
		}
	}
}

// logDelivery logs a webhook delivery attempt
func (s *WebhookService) logDelivery(delivery *WebhookDelivery, statusCode int, response string, error string, duration int64, success bool) {
	payloadBytes, _ := json.Marshal(delivery.Payload)
	
	log := &WebhookLog{
		WebhookID:   delivery.WebhookID,
		Event:       delivery.Event,
		URL:         delivery.URL,
		Method:      delivery.Method,
		Headers:     delivery.Headers,
		Payload:     json.RawMessage(payloadBytes),
		Response:    response,
		StatusCode:  statusCode,
		Success:     success,
		Error:       error,
		Duration:    duration,
		RetryCount:  delivery.RetryCount,
		DeliveredAt: time.Now(),
	}
	
	// Save log
	_ = s.webhookRepo.CreateLog(log)
	
	// Update webhook last triggered time
	if success {
		s.mu.Lock()
		if webhook, exists := s.activeWebhooks[delivery.WebhookID]; exists {
			now := time.Now()
			webhook.LastTriggered = &now
			webhook.FailureCount = 0 // Reset failure count on success
		}
		s.mu.Unlock()
	}
}

// TestWebhook tests a webhook with sample data
func (s *WebhookService) TestWebhook(webhookID int) error {
	webhook, err := s.webhookRepo.GetByID(webhookID)
	if err != nil {
		return err
	}
	
	// Create test payload
	testPayload := map[string]interface{}{
		"test": true,
		"message": "This is a test webhook delivery",
		"timestamp": time.Now().Unix(),
	}
	
	delivery := &WebhookDelivery{
		WebhookID:  webhook.ID,
		URL:        webhook.URL,
		Method:     webhook.Method,
		Headers:    webhook.Headers,
		Payload:    testPayload,
		Event:      "test",
		Timestamp:  time.Now(),
		MaxRetries: 0, // No retries for test
	}
	
	if webhook.Secret != "" {
		delivery.Signature = s.generateSignature(webhook.Secret, testPayload)
	}
	
	// Deliver webhook synchronously
	s.deliverWebhook(delivery)
	
	return nil
}

// GetWebhookLogs gets logs for a webhook
func (s *WebhookService) GetWebhookLogs(webhookID int, limit int) ([]*WebhookLog, error) {
	return s.webhookRepo.GetLogs(webhookID, limit)
}

// HandleIncomingWebhook handles incoming webhooks
func (s *WebhookService) HandleIncomingWebhook(id string, headers map[string]string, body []byte) error {
	// Get incoming webhook configuration
	webhook, err := s.webhookRepo.GetIncomingByID(id)
	if err != nil {
		return fmt.Errorf("webhook not found")
	}
	
	if !webhook.IsActive {
		return fmt.Errorf("webhook is disabled")
	}
	
	// Verify signature if secret is configured
	if webhook.Secret != "" {
		signature := headers["X-Hub-Signature-256"] // GitHub style
		if signature == "" {
			signature = headers["X-Webhook-Signature"] // Generic
		}
		
		if !s.verifySignature(webhook.Secret, body, signature) {
			return fmt.Errorf("invalid signature")
		}
	}
	
	// Parse payload based on webhook type
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("invalid payload")
	}
	
	// Process based on webhook type
	switch webhook.Type {
	case "github":
		return s.processGitHubWebhook(webhook, headers, payload)
	case "slack":
		return s.processSlackWebhook(webhook, headers, payload)
	case "custom":
		return s.processCustomWebhook(webhook, headers, payload)
	default:
		return fmt.Errorf("unsupported webhook type")
	}
}

// processGitHubWebhook processes GitHub webhooks
func (s *WebhookService) processGitHubWebhook(webhook *models.Webhook, headers map[string]string, payload map[string]interface{}) error {
	event := headers["X-GitHub-Event"]
	
	switch event {
	case "issues":
		// Create ticket from GitHub issue
		return s.createTicketFromGitHubIssue(payload)
	case "issue_comment":
		// Add comment to ticket
		return s.addCommentFromGitHub(payload)
	default:
		// Log unhandled event
		return nil
	}
}

// processSlackWebhook processes Slack webhooks
func (s *WebhookService) processSlackWebhook(webhook *models.Webhook, headers map[string]string, payload map[string]interface{}) error {
	// Handle Slack events
	if eventType, ok := payload["type"].(string); ok {
		switch eventType {
		case "url_verification":
			// Slack verification challenge
			return nil
		case "event_callback":
			// Process Slack event
			return s.processSlackEvent(payload)
		}
	}
	return nil
}

// processCustomWebhook processes custom webhooks
func (s *WebhookService) processCustomWebhook(webhook *models.Webhook, headers map[string]string, payload map[string]interface{}) error {
	// Process based on webhook configuration
	// This would be customized based on specific integration needs
	return nil
}

// Helper methods

// validateWebhook validates webhook configuration
func (s *WebhookService) validateWebhook(webhook *models.Webhook) error {
	if webhook.Name == "" {
		return fmt.Errorf("webhook name is required")
	}
	
	if webhook.URL == "" {
		return fmt.Errorf("webhook URL is required")
	}
	
	if webhook.Method == "" {
		webhook.Method = "POST"
	}
	
	if webhook.Events == nil || len(webhook.Events) == 0 {
		return fmt.Errorf("at least one event must be specified")
	}
	
	if webhook.MaxRetries == 0 {
		webhook.MaxRetries = 3
	}
	
	if webhook.Headers == nil {
		webhook.Headers = make(map[string]string)
	}
	
	return nil
}

// shouldTrigger checks if webhook should be triggered for an event
func (s *WebhookService) shouldTrigger(webhook *models.Webhook, event string) bool {
	if !webhook.IsActive {
		return false
	}
	
	// Check if webhook subscribes to this event
	for _, e := range webhook.Events {
		if e == event || e == "*" {
			return true
		}
	}
	
	return false
}

// generateSecret generates a webhook secret
func (s *WebhookService) generateSecret() string {
	// Generate random secret
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(time.Now().UnixNano() & 0xFF)
	}
	return hex.EncodeToString(secret)
}

// generateSignature generates HMAC signature for payload
func (s *WebhookService) generateSignature(secret string, payload interface{}) string {
	payloadBytes, _ := json.Marshal(payload)
	
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payloadBytes)
	
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

// verifySignature verifies webhook signature
func (s *WebhookService) verifySignature(secret string, payload []byte, signature string) bool {
	expectedSig := s.generateSignature(secret, payload)
	return hmac.Equal([]byte(expectedSig), []byte(signature))
}

// loadActiveWebhooks loads active webhooks from repository
func (s *WebhookService) loadActiveWebhooks() {
	webhooks, err := s.webhookRepo.GetActiveWebhooks()
	if err != nil {
		return
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for _, webhook := range webhooks {
		s.activeWebhooks[webhook.ID] = webhook
	}
}

// startWorkers starts webhook delivery workers
func (s *WebhookService) startWorkers() {
	for i := 0; i < s.workers; i++ {
		go s.worker()
	}
}

// worker processes webhook deliveries
func (s *WebhookService) worker() {
	for {
		select {
		case delivery := <-s.retryQueue:
			s.deliverWebhook(delivery)
		case <-s.stopChan:
			return
		}
	}
}

// Stop stops the webhook service
func (s *WebhookService) Stop() {
	close(s.stopChan)
}

// Integration helpers

// createTicketFromGitHubIssue creates a ticket from GitHub issue
func (s *WebhookService) createTicketFromGitHubIssue(payload map[string]interface{}) error {
	// Extract issue data and create ticket
	// This would integrate with ticket service
	return nil
}

// addCommentFromGitHub adds a comment from GitHub
func (s *WebhookService) addCommentFromGitHub(payload map[string]interface{}) error {
	// Extract comment data and add to ticket
	// This would integrate with ticket service
	return nil
}

// processSlackEvent processes a Slack event
func (s *WebhookService) processSlackEvent(payload map[string]interface{}) error {
	// Process Slack event based on type
	// This would integrate with appropriate services
	return nil
}