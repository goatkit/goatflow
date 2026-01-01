// Package ticket provides ticket domain models and business logic.
package ticket

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// Ticket represents a support ticket.
type Ticket struct {
	ID              string                 `json:"id" db:"id"`
	Number          string                 `json:"number" db:"number"`
	Title           string                 `json:"title" db:"title"`
	Description     string                 `json:"description" db:"description"`
	Priority        string                 `json:"priority" db:"priority"`
	Status          string                 `json:"status" db:"status"`
	QueueID         string                 `json:"queue_id" db:"queue_id"`
	CustomerID      string                 `json:"customer_id" db:"customer_id"`
	AssignedTo      *string                `json:"assigned_to,omitempty" db:"assigned_to"`
	Tags            []string               `json:"tags" db:"tags"`
	CustomFields    map[string]interface{} `json:"custom_fields" db:"custom_fields"`
	CreatedAt       time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at" db:"updated_at"`
	ResolvedAt      *time.Time             `json:"resolved_at,omitempty" db:"resolved_at"`
	ClosedAt        *time.Time             `json:"closed_at,omitempty" db:"closed_at"`
	DueAt           *time.Time             `json:"due_at,omitempty" db:"due_at"`
	FirstResponseAt *time.Time             `json:"first_response_at,omitempty" db:"first_response_at"`

	// Relationships
	Articles    []Article     `json:"articles,omitempty"`
	Attachments []Attachment  `json:"attachments,omitempty"`
	History     []HistoryItem `json:"history,omitempty"`
}

// Article represents a ticket article/message.
type Article struct {
	ID          string                 `json:"id" db:"id"`
	TicketID    string                 `json:"ticket_id" db:"ticket_id"`
	Type        string                 `json:"type" db:"type"` // email, note, phone, chat
	From        string                 `json:"from" db:"from_address"`
	To          string                 `json:"to" db:"to_address"`
	Subject     string                 `json:"subject" db:"subject"`
	Body        string                 `json:"body" db:"body"`
	IsInternal  bool                   `json:"is_internal" db:"is_internal"`
	CreatedBy   string                 `json:"created_by" db:"created_by"`
	CreatedAt   time.Time              `json:"created_at" db:"created_at"`
	Attachments []Attachment           `json:"attachments,omitempty"`
	Metadata    map[string]interface{} `json:"metadata" db:"metadata"`
}

// Attachment represents a file attachment.
type Attachment struct {
	ID          string    `json:"id" db:"id"`
	TicketID    string    `json:"ticket_id" db:"ticket_id"`
	ArticleID   *string   `json:"article_id,omitempty" db:"article_id"`
	FileName    string    `json:"file_name" db:"file_name"`
	ContentType string    `json:"content_type" db:"content_type"`
	Size        int64     `json:"size" db:"size"`
	StoragePath string    `json:"storage_path" db:"storage_path"`
	UploadedBy  string    `json:"uploaded_by" db:"uploaded_by"`
	UploadedAt  time.Time `json:"uploaded_at" db:"uploaded_at"`
	Checksum    string    `json:"checksum" db:"checksum"`
}

// HistoryItem represents a ticket history entry.
type HistoryItem struct {
	ID        string                 `json:"id" db:"id"`
	TicketID  string                 `json:"ticket_id" db:"ticket_id"`
	Action    string                 `json:"action" db:"action"`
	Field     string                 `json:"field" db:"field"`
	OldValue  *string                `json:"old_value,omitempty" db:"old_value"`
	NewValue  *string                `json:"new_value,omitempty" db:"new_value"`
	ChangedBy string                 `json:"changed_by" db:"changed_by"`
	ChangedAt time.Time              `json:"changed_at" db:"changed_at"`
	Comment   string                 `json:"comment,omitempty" db:"comment"`
	Metadata  map[string]interface{} `json:"metadata" db:"metadata"`
}

// ListFilter represents ticket list filtering options.
type ListFilter struct {
	Status      string    `json:"status,omitempty"`
	Priority    string    `json:"priority,omitempty"`
	QueueID     string    `json:"queue_id,omitempty"`
	AssignedTo  string    `json:"assigned_to,omitempty"`
	CustomerID  string    `json:"customer_id,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	CreatedFrom time.Time `json:"created_from,omitempty"`
	CreatedTo   time.Time `json:"created_to,omitempty"`
	UpdatedFrom time.Time `json:"updated_from,omitempty"`
	UpdatedTo   time.Time `json:"updated_to,omitempty"`
	SortBy      string    `json:"sort_by,omitempty"`
	SortOrder   string    `json:"sort_order,omitempty"`
	Limit       int       `json:"limit,omitempty"`
	Offset      int       `json:"offset,omitempty"`
}

// Clone creates a deep copy of the ticket.
func (t *Ticket) Clone() *Ticket {
	clone := *t

	// Clone slices
	if t.Tags != nil {
		clone.Tags = make([]string, len(t.Tags))
		copy(clone.Tags, t.Tags)
	}

	// Clone maps
	if t.CustomFields != nil {
		clone.CustomFields = make(map[string]interface{})
		for k, v := range t.CustomFields {
			clone.CustomFields[k] = v
		}
	}

	// Clone relationships
	if t.Articles != nil {
		clone.Articles = make([]Article, len(t.Articles))
		copy(clone.Articles, t.Articles)
	}

	if t.Attachments != nil {
		clone.Attachments = make([]Attachment, len(t.Attachments))
		copy(clone.Attachments, t.Attachments)
	}

	if t.History != nil {
		clone.History = make([]HistoryItem, len(t.History))
		copy(clone.History, t.History)
	}

	return &clone
}

// ToProto converts the ticket to protobuf format.
func (t *Ticket) ToProto() *TicketProto {
	proto := &TicketProto{
		Id:          t.ID,
		Number:      t.Number,
		Title:       t.Title,
		Description: t.Description,
		Priority:    t.Priority,
		Status:      t.Status,
		QueueId:     t.QueueID,
		CustomerId:  t.CustomerID,
		CreatedAt:   t.CreatedAt.Unix(),
		UpdatedAt:   t.UpdatedAt.Unix(),
	}

	if t.AssignedTo != nil {
		proto.AssignedTo = *t.AssignedTo
	}

	if t.Tags != nil {
		proto.Tags = t.Tags
	}

	if t.ResolvedAt != nil {
		proto.ResolvedAt = t.ResolvedAt.Unix()
	}

	if t.ClosedAt != nil {
		proto.ClosedAt = t.ClosedAt.Unix()
	}

	if t.DueAt != nil {
		proto.DueAt = t.DueAt.Unix()
	}

	if t.FirstResponseAt != nil {
		proto.FirstResponseAt = t.FirstResponseAt.Unix()
	}

	return proto
}

// Hash generates a hash of the filter for caching.
func (f *ListFilter) Hash() string {
	str := fmt.Sprintf("%s-%s-%s-%s-%s-%v-%v-%v-%v-%v-%s-%s-%d-%d",
		f.Status,
		f.Priority,
		f.QueueID,
		f.AssignedTo,
		f.CustomerID,
		f.Tags,
		f.CreatedFrom,
		f.CreatedTo,
		f.UpdatedFrom,
		f.UpdatedTo,
		f.SortBy,
		f.SortOrder,
		f.Limit,
		f.Offset,
	)

	hash := sha256.Sum256([]byte(str))
	return hex.EncodeToString(hash[:])
}

// Priority levels.
const (
	PriorityLow      = "low"
	PriorityNormal   = "normal"
	PriorityHigh     = "high"
	PriorityUrgent   = "urgent"
	PriorityCritical = "critical"
)

// Status values.
const (
	StatusOpen       = "open"
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusResolved   = "resolved"
	StatusClosed     = "closed"
	StatusOnHold     = "on_hold"
	StatusCancelled  = "cancelled"
)

// Article types.
const (
	ArticleTypeEmail = "email"
	ArticleTypeNote  = "note"
	ArticleTypePhone = "phone"
	ArticleTypeChat  = "chat"
	ArticleTypeWeb   = "web"
)

// History actions.
const (
	ActionCreated         = "created"
	ActionUpdated         = "updated"
	ActionStatusChanged   = "status_changed"
	ActionPriorityChanged = "priority_changed"
	ActionAssigned        = "assigned"
	ActionCommented       = "commented"
	ActionAttached        = "attached"
	ActionMerged          = "merged"
	ActionSplit           = "split"
	ActionEscalated       = "escalated"
)
