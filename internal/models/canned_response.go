package models

import (
	"time"
)

// CannedResponse represents a pre-written response for quick replies.
type CannedResponse struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name" binding:"required"`     // Display name for the response
	Shortcut    string    `json:"shortcut"`                    // Quick access code (e.g., "/greeting")
	Category    string    `json:"category" binding:"required"` // Category for organization
	Subject     string    `json:"subject"`                     // Optional subject line
	Content     string    `json:"content" binding:"required"`  // The response content
	ContentType string    `json:"content_type"`                // text/plain or text/html
	Tags        []string  `json:"tags"`                        // Tags for search/filter
	IsPublic    bool      `json:"is_public"`                   // Available to all agents
	IsActive    bool      `json:"is_active"`                   // Currently active
	UsageCount  int       `json:"usage_count"`                 // Track usage
	CreatedBy   uint      `json:"created_by"`
	UpdatedBy   uint      `json:"updated_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Access control
	OwnerID    uint   `json:"owner_id"`              // User who owns this response
	SharedWith []uint `json:"shared_with,omitempty"` // User IDs who can use this
	QueueIDs   []uint `json:"queue_ids,omitempty"`   // Restrict to specific queues

	// Variables for substitution
	Variables []ResponseVariable `json:"variables,omitempty"`

	// Attachments to include
	AttachmentURLs []string `json:"attachment_urls,omitempty"`
}

// ResponseVariable represents a placeholder in the canned response.
type ResponseVariable struct {
	Name         string   `json:"name"`              // e.g., "{{agent_name}}"
	Description  string   `json:"description"`       // Help text
	Type         string   `json:"type"`              // text, date, number, select
	Options      []string `json:"options,omitempty"` // For select type
	DefaultValue string   `json:"default_value,omitempty"`
	AutoFill     string   `json:"auto_fill,omitempty"` // Auto-fill from context (agent_name, ticket_number, etc.)
}

// CannedResponseCategory represents a category for organizing responses.
type CannedResponseCategory struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon,omitempty"`
	Order       int    `json:"order"`
	ParentID    *uint  `json:"parent_id,omitempty"` // For nested categories
	Active      bool   `json:"active"`
}

// CannedResponseUsage tracks when and where a response was used.
type CannedResponseUsage struct {
	ID             uint      `json:"id"`
	ResponseID     uint      `json:"response_id"`
	TicketID       uint      `json:"ticket_id"`
	UserID         uint      `json:"user_id"`
	UsedAt         time.Time `json:"used_at"`
	ModifiedBefore bool      `json:"modified_before_use"` // Track if agent modified before sending
}

// CannedResponseFilter for searching/filtering responses.
type CannedResponseFilter struct {
	Query      string   `json:"query,omitempty"`
	Category   string   `json:"category,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	QueueID    uint     `json:"queue_id,omitempty"`
	OnlyPublic bool     `json:"only_public,omitempty"`
	OnlyOwned  bool     `json:"only_owned,omitempty"`
	UserID     uint     `json:"user_id,omitempty"`
	Limit      int      `json:"limit,omitempty"`
	Offset     int      `json:"offset,omitempty"`
}

// CannedResponseApplication for applying a response to a ticket.
type CannedResponseApplication struct {
	ResponseID uint              `json:"response_id" binding:"required"`
	TicketID   uint              `json:"ticket_id" binding:"required"`
	Variables  map[string]string `json:"variables,omitempty"`
	AsInternal bool              `json:"as_internal"` // Send as internal note vs customer reply
}

// AppliedResponse represents the result of applying a canned response.
type AppliedResponse struct {
	Subject     string   `json:"subject"`
	Content     string   `json:"content"`
	ContentType string   `json:"content_type"`
	Attachments []string `json:"attachments,omitempty"`
	AsInternal  bool     `json:"as_internal"`
}

// AutoFillContext provides context for auto-filling variables.
type AutoFillContext struct {
	AgentName     string `json:"agent_name,omitempty"`
	AgentEmail    string `json:"agent_email,omitempty"`
	TicketNumber  string `json:"ticket_number,omitempty"`
	CustomerName  string `json:"customer_name,omitempty"`
	CustomerEmail string `json:"customer_email,omitempty"`
	QueueName     string `json:"queue_name,omitempty"`
}
