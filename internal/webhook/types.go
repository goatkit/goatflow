package webhook

import (
	"time"
)

// WebhookEvent represents the type of event that triggers a webhook
type WebhookEvent string

const (
	EventTicketCreated      WebhookEvent = "ticket.created"
	EventTicketUpdated      WebhookEvent = "ticket.updated"
	EventTicketClosed       WebhookEvent = "ticket.closed"
	EventTicketReopened     WebhookEvent = "ticket.reopened"
	EventTicketAssigned     WebhookEvent = "ticket.assigned"
	EventTicketEscalated    WebhookEvent = "ticket.escalated"
	EventTicketPriorityChanged WebhookEvent = "ticket.priority_changed"
	EventTicketStatusChanged   WebhookEvent = "ticket.status_changed"
	EventTicketQueueMoved      WebhookEvent = "ticket.queue_moved"
	
	EventArticleAdded       WebhookEvent = "article.added"
	EventArticleUpdated     WebhookEvent = "article.updated"
	
	EventUserCreated        WebhookEvent = "user.created"
	EventUserUpdated        WebhookEvent = "user.updated"
	EventUserActivated      WebhookEvent = "user.activated"
	EventUserDeactivated    WebhookEvent = "user.deactivated"
	
	EventQueueCreated       WebhookEvent = "queue.created"
	EventQueueUpdated       WebhookEvent = "queue.updated"
	EventQueueDeleted       WebhookEvent = "queue.deleted"
	
	EventAttachmentUploaded WebhookEvent = "attachment.uploaded"
	EventAttachmentDeleted  WebhookEvent = "attachment.deleted"
	
	EventSystemMaintenance  WebhookEvent = "system.maintenance"
	EventSystemBackup       WebhookEvent = "system.backup"
	EventSystemAlert        WebhookEvent = "system.alert"
)

// WebhookStatus represents the status of a webhook endpoint
type WebhookStatus string

const (
	StatusActive   WebhookStatus = "active"
	StatusInactive WebhookStatus = "inactive"
	StatusFailed   WebhookStatus = "failed"
	StatusDisabled WebhookStatus = "disabled"
)

// WebhookDeliveryStatus represents the status of a webhook delivery attempt
type WebhookDeliveryStatus string

const (
	DeliveryPending  WebhookDeliveryStatus = "pending"
	DeliverySuccess  WebhookDeliveryStatus = "success"
	DeliveryFailed   WebhookDeliveryStatus = "failed"
	DeliveryRetrying WebhookDeliveryStatus = "retrying"
	DeliveryExpired  WebhookDeliveryStatus = "expired"
)

// Webhook represents a webhook endpoint configuration
type Webhook struct {
	ID          uint          `json:"id" db:"id"`
	Name        string        `json:"name" db:"name"`
	URL         string        `json:"url" db:"url"`
	Secret      string        `json:"secret,omitempty" db:"secret"` // HMAC secret for signature verification
	Events      []WebhookEvent `json:"events" db:"events"`
	Status      WebhookStatus `json:"status" db:"status"`
	Description string        `json:"description" db:"description"`
	
	// Configuration
	RetryCount    int           `json:"retry_count" db:"retry_count"`
	RetryInterval time.Duration `json:"retry_interval" db:"retry_interval"`
	Timeout       time.Duration `json:"timeout" db:"timeout"`
	
	// Headers to include in webhook requests
	Headers map[string]string `json:"headers,omitempty" db:"headers"`
	
	// Filters for conditional webhook execution
	Filters WebhookFilters `json:"filters,omitempty" db:"filters"`
	
	// Statistics
	TotalDeliveries     int       `json:"total_deliveries" db:"total_deliveries"`
	SuccessfulDeliveries int      `json:"successful_deliveries" db:"successful_deliveries"`
	FailedDeliveries    int       `json:"failed_deliveries" db:"failed_deliveries"`
	LastDeliveryAt      *time.Time `json:"last_delivery_at,omitempty" db:"last_delivery_at"`
	LastSuccessAt       *time.Time `json:"last_success_at,omitempty" db:"last_success_at"`
	LastFailureAt       *time.Time `json:"last_failure_at,omitempty" db:"last_failure_at"`
	
	// Metadata
	CreatedBy  uint      `json:"created_by" db:"created_by"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// WebhookFilters defines conditions for when webhooks should be triggered
type WebhookFilters struct {
	// Queue filters - only trigger for specific queues
	QueueIDs []int `json:"queue_ids,omitempty"`
	
	// Priority filters - only trigger for specific priorities
	Priorities []string `json:"priorities,omitempty"`
	
	// Status filters - only trigger for specific statuses
	Statuses []string `json:"statuses,omitempty"`
	
	// User filters - only trigger for specific users
	UserIDs []int `json:"user_ids,omitempty"`
	
	// Custom field filters
	CustomFields map[string]interface{} `json:"custom_fields,omitempty"`
}

// WebhookDelivery represents a webhook delivery attempt
type WebhookDelivery struct {
	ID         uint                  `json:"id" db:"id"`
	WebhookID  uint                  `json:"webhook_id" db:"webhook_id"`
	Event      WebhookEvent          `json:"event" db:"event"`
	Payload    interface{}           `json:"payload" db:"payload"`
	Status     WebhookDeliveryStatus `json:"status" db:"status"`
	
	// Request details
	RequestURL     string            `json:"request_url" db:"request_url"`
	RequestHeaders map[string]string `json:"request_headers" db:"request_headers"`
	RequestBody    string            `json:"request_body" db:"request_body"`
	
	// Response details
	ResponseStatusCode int               `json:"response_status_code,omitempty" db:"response_status_code"`
	ResponseHeaders    map[string]string `json:"response_headers,omitempty" db:"response_headers"`
	ResponseBody       string            `json:"response_body,omitempty" db:"response_body"`
	
	// Timing
	AttemptCount int           `json:"attempt_count" db:"attempt_count"`
	Duration     time.Duration `json:"duration" db:"duration"`
	NextRetryAt  *time.Time    `json:"next_retry_at,omitempty" db:"next_retry_at"`
	
	// Error information
	ErrorMessage string `json:"error_message,omitempty" db:"error_message"`
	
	// Metadata
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// WebhookPayload represents the structure of webhook payloads sent to external systems
type WebhookPayload struct {
	// Event information
	Event     WebhookEvent `json:"event"`
	Timestamp time.Time    `json:"timestamp"`
	ID        string       `json:"id"` // Unique delivery ID
	
	// Source information
	Source struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		URL     string `json:"url"`
	} `json:"source"`
	
	// Event data - the actual payload varies by event type
	Data interface{} `json:"data"`
	
	// Previous data (for update events)
	PreviousData interface{} `json:"previous_data,omitempty"`
}

// TicketWebhookData represents ticket data in webhook payloads
type TicketWebhookData struct {
	ID          int       `json:"id"`
	Number      string    `json:"number"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Status      string    `json:"status"`
	Priority    string    `json:"priority"`
	QueueID     int       `json:"queue_id"`
	QueueName   string    `json:"queue_name"`
	AssignedTo  *int      `json:"assigned_to"`
	AssignedName *string  `json:"assigned_name"`
	CustomerID  string    `json:"customer_id"`
	CustomerEmail string  `json:"customer_email"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	SLADue      *time.Time `json:"sla_due,omitempty"`
}

// ArticleWebhookData represents article data in webhook payloads
type ArticleWebhookData struct {
	ID       int    `json:"id"`
	TicketID int    `json:"ticket_id"`
	From     string `json:"from"`
	To       string `json:"to"`
	Subject  string `json:"subject"`
	Body     string `json:"body"`
	Type     string `json:"type"`
	Visible  bool   `json:"visible"`
	CreatedAt time.Time `json:"created_at"`
}

// UserWebhookData represents user data in webhook payloads
type UserWebhookData struct {
	ID        int    `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
	Active    bool   `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// QueueWebhookData represents queue data in webhook payloads
type QueueWebhookData struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	GroupID     int    `json:"group_id"`
	Active      bool   `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WebhookRequest represents a request to create or update a webhook
type WebhookRequest struct {
	Name        string            `json:"name" binding:"required"`
	URL         string            `json:"url" binding:"required,url"`
	Events      []WebhookEvent    `json:"events" binding:"required,min=1"`
	Secret      string            `json:"secret"`
	Description string            `json:"description"`
	Headers     map[string]string `json:"headers"`
	Filters     WebhookFilters    `json:"filters"`
	
	// Configuration
	RetryCount    int `json:"retry_count"`
	RetryInterval int `json:"retry_interval"` // seconds
	Timeout       int `json:"timeout"`       // seconds
}

// WebhookStatistics represents webhook usage statistics
type WebhookStatistics struct {
	WebhookID            uint    `json:"webhook_id"`
	TotalDeliveries      int     `json:"total_deliveries"`
	SuccessfulDeliveries int     `json:"successful_deliveries"`
	FailedDeliveries     int     `json:"failed_deliveries"`
	SuccessRate          float64 `json:"success_rate"`
	AverageResponseTime  int     `json:"average_response_time_ms"`
	LastDeliveryAt       *time.Time `json:"last_delivery_at"`
	LastSuccessAt        *time.Time `json:"last_success_at"`
	LastFailureAt        *time.Time `json:"last_failure_at"`
}

// WebhookTestResult represents the result of testing a webhook
type WebhookTestResult struct {
	Success            bool          `json:"success"`
	StatusCode         int           `json:"status_code"`
	ResponseTime       time.Duration `json:"response_time_ms"`
	ResponseBody       string        `json:"response_body"`
	ErrorMessage       string        `json:"error_message,omitempty"`
	TestedAt           time.Time     `json:"tested_at"`
}