package models

import "time"

// CreateTicketRequest represents a request to create a new ticket.
type CreateTicketRequest struct {
	Title             string                `json:"title" binding:"required"`
	QueueID           uint                  `json:"queue_id"`
	StateID           uint                  `json:"state_id"`
	PriorityID        uint                  `json:"priority_id"`
	CustomerUserID    uint                  `json:"customer_user_id"`
	CustomerID        string                `json:"customer_id"`
	OwnerID           uint                  `json:"owner_id"`
	ResponsibleUserID uint                  `json:"responsible_user_id"`
	TypeID            uint                  `json:"type_id"`
	TenantID          uint                  `json:"tenant_id" binding:"required"`
	CreateBy          uint                  `json:"-"`
	InitialArticle    *CreateArticleRequest `json:"initial_article"`
}

// UpdateTicketRequest represents a request to update a ticket.
type UpdateTicketRequest struct {
	Title             string `json:"title"`
	QueueID           uint   `json:"queue_id"`
	StateID           uint   `json:"state_id"`
	PriorityID        uint   `json:"priority_id"`
	OwnerID           uint   `json:"owner_id"`
	ResponsibleUserID uint   `json:"responsible_user_id"`
	TypeID            uint   `json:"type_id"`
	UpdateBy          uint   `json:"-"`
}

// CreateArticleRequest represents a request to create a new article.
type CreateArticleRequest struct {
	ArticleTypeID uint                      `json:"article_type_id" binding:"required"`
	SenderTypeID  uint                      `json:"sender_type_id" binding:"required"`
	From          string                    `json:"from"`
	To            string                    `json:"to"`
	CC            string                    `json:"cc"`
	Subject       string                    `json:"subject" binding:"required"`
	Body          string                    `json:"body" binding:"required"`
	ContentType   string                    `json:"content_type"`
	CreateBy      uint                      `json:"-"`
	Attachments   []CreateAttachmentRequest `json:"attachments"`
}

// CreateAttachmentRequest represents a request to create an attachment.
type CreateAttachmentRequest struct {
	Filename    string `json:"filename" binding:"required"`
	ContentSize int    `json:"content_size"`
	ContentType string `json:"content_type"`
	Content     []byte `json:"content" binding:"required"`
}

// MergeTicketsRequest represents a request to merge tickets.
type MergeTicketsRequest struct {
	TargetTicketID uint `json:"target_ticket_id" binding:"required"`
	SourceTicketID uint `json:"source_ticket_id" binding:"required"`
}

// AssignTicketRequest represents a request to assign a ticket.
type AssignTicketRequest struct {
	AgentID uint `json:"agent_id" binding:"required"`
}

// EscalateTicketRequest represents a request to escalate a ticket.
type EscalateTicketRequest struct {
	PriorityID uint   `json:"priority_id" binding:"required"`
	Reason     string `json:"reason" binding:"required"`
}

// TicketHistory represents a ticket history entry.
type TicketHistory struct {
	ID         uint      `json:"id"`
	TicketID   uint      `json:"ticket_id"`
	ArticleID  *uint     `json:"article_id,omitempty"`
	Name       string    `json:"name"`
	Value      string    `json:"value"`
	CreateTime time.Time `json:"create_time"`
	CreateBy   uint      `json:"create_by"`
	ChangeTime time.Time `json:"change_time"`
	ChangeBy   uint      `json:"change_by"`
}
