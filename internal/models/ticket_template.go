package models

import (
	"time"
)

// TicketTemplate represents a reusable template for common ticket types.
type TicketTemplate struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name" binding:"required"`
	Description string    `json:"description"`
	Category    string    `json:"category"` // e.g., "Technical", "Billing", "General"
	Subject     string    `json:"subject" binding:"required"`
	Body        string    `json:"body" binding:"required"`
	Priority    string    `json:"priority"`
	QueueID     int       `json:"queue_id"`
	TypeID      int       `json:"type_id"`
	Tags        []string  `json:"tags"`
	Active      bool      `json:"active"`
	UsageCount  int       `json:"usage_count"`
	CreatedBy   int       `json:"created_by"`
	UpdatedBy   int       `json:"updated_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Template variables that can be replaced
	Variables []TemplateVariable `json:"variables"`

	// Attachments that should be included
	AttachmentURLs []string `json:"attachment_urls,omitempty"`
}

// TemplateVariable represents a placeholder in the template.
type TemplateVariable struct {
	Name         string `json:"name"`        // e.g., "{{customer_name}}"
	Description  string `json:"description"` // Help text for the variable
	Required     bool   `json:"required"`
	DefaultValue string `json:"default_value,omitempty"`
}

// TemplateCategory represents a category for organizing templates.
type TemplateCategory struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon,omitempty"` // Icon class for UI
	Order       int    `json:"order"`
	Active      bool   `json:"active"`
}

// ApplyTemplate creates a new ticket from a template with variable substitution.
type TemplateApplication struct {
	TemplateID      uint              `json:"template_id" binding:"required"`
	Variables       map[string]string `json:"variables"`
	CustomerEmail   string            `json:"customer_email" binding:"required,email"`
	CustomerName    string            `json:"customer_name"`
	AdditionalNotes string            `json:"additional_notes,omitempty"`
}
