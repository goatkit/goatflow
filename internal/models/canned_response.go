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
