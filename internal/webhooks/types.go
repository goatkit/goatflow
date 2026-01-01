// Package webhooks provides webhook event types and payload definitions.
package webhooks

import (
	"time"
)

// EventType represents the type of event that triggers a webhook.
type EventType string

const (
	// Ticket events.
	EventTicketCreated   EventType = "ticket.created"
	EventTicketUpdated   EventType = "ticket.updated"
	EventTicketClosed    EventType = "ticket.closed"
	EventTicketReopened  EventType = "ticket.reopened"
	EventTicketAssigned  EventType = "ticket.assigned"
	EventTicketEscalated EventType = "ticket.escalated"

	// Article events.
	EventArticleCreated EventType = "article.created"
	EventArticleUpdated EventType = "article.updated"
	EventArticleDeleted EventType = "article.deleted"

	// Customer events.
	EventCustomerCreated EventType = "customer.created"
	EventCustomerUpdated EventType = "customer.updated"

	// SLA events.
	EventSLABreached EventType = "sla.breached"
	EventSLAWarning  EventType = "sla.warning"

	// Queue events.
	EventQueueThreshold EventType = "queue.threshold"
)

// AllEventTypes returns all available event types.
func AllEventTypes() []EventType {
	return []EventType{
		EventTicketCreated, EventTicketUpdated, EventTicketClosed,
		EventTicketReopened, EventTicketAssigned, EventTicketEscalated,
		EventArticleCreated, EventArticleUpdated, EventArticleDeleted,
		EventCustomerCreated, EventCustomerUpdated,
		EventSLABreached, EventSLAWarning,
		EventQueueThreshold,
	}
}

// Event represents an event that can trigger webhooks.
type Event struct {
	Type      EventType              `json:"event_type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	UserID    int                    `json:"user_id,omitempty"`
	Source    string                 `json:"source"`
}

// Webhook represents a configured webhook endpoint.
type Webhook struct {
	ID             int               `json:"id"`
	Name           string            `json:"name"`
	URL            string            `json:"url"`
	Secret         string            `json:"secret,omitempty"`
	Events         []string          `json:"events"`
	Active         bool              `json:"active"`
	RetryCount     int               `json:"retry_count"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	Headers        map[string]string `json:"headers,omitempty"`
	CreateTime     time.Time         `json:"create_time"`
	CreateBy       int               `json:"create_by"`
	ChangeTime     time.Time         `json:"change_time"`
	ChangeBy       int               `json:"change_by"`
}

// WebhookDelivery represents a webhook delivery attempt.
type WebhookDelivery struct {
	ID          int        `json:"id"`
	WebhookID   int        `json:"webhook_id"`
	EventType   string     `json:"event_type"`
	Payload     string     `json:"payload"`
	StatusCode  int        `json:"status_code"`
	Response    string     `json:"response,omitempty"`
	Attempts    int        `json:"attempts"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
	NextRetry   *time.Time `json:"next_retry,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	Success     bool       `json:"success"`
}

// WebhookPayload represents the payload sent to webhook endpoints.
type WebhookPayload struct {
	Event     EventType              `json:"event"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Signature string                 `json:"signature,omitempty"`
}
