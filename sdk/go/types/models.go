// Package types holds the request and response shapes of the GoatFlow REST
// API (/api/v1). Field sets mirror what the handlers actually send; where two
// endpoints describe the same resource differently (e.g. ticket list rows vs.
// a single ticket) they get separate types.
package types

import (
	"time"
)

// Pagination is the "pagination" object sent next to "data" by paginated
// list endpoints.
type Pagination struct {
	Page       int  `json:"page"`
	PerPage    int  `json:"per_page"`
	Total      int  `json:"total"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// GroupRef is a group reference (id and name).
type GroupRef struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// Tickets

// TicketSummary is one row of GET /api/v1/tickets.
type TicketSummary struct {
	ID                uint      `json:"id"`
	TN                string    `json:"tn"`
	TicketNumber      string    `json:"ticket_number"`
	Title             string    `json:"title"`
	QueueID           uint      `json:"queue_id"`
	QueueName         string    `json:"queue_name"`
	StateID           uint      `json:"state_id"`
	StateName         string    `json:"state_name"`
	PriorityID        uint      `json:"priority_id"`
	PriorityName      string    `json:"priority_name"`
	CustomerUserID    string    `json:"customer_user_id"`
	CustomerID        string    `json:"customer_id"`
	UserID            uint      `json:"user_id"` // owner
	ResponsibleUserID *uint     `json:"responsible_user_id"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	// Only with TicketListOptions.Include containing "article_count".
	ArticleCount *int `json:"article_count,omitempty"`
	// Only with TicketListOptions.Include containing "last_article"; nil
	// also when the ticket has no article.
	LastArticle *LastArticle `json:"last_article,omitempty"`
}

// LastArticle is the newest article of a ticket in a list row.
type LastArticle struct {
	Subject   string    `json:"subject"`
	CreatedAt time.Time `json:"created_at"`
}

// TicketListOptions filters GET /api/v1/tickets. Zero values are not sent.
type TicketListOptions struct {
	Page    int
	PerPage int // 1-100, server default 20
	// Status is "open", "closed", "pending" or an exact state name.
	Status         string
	QueueID        uint
	PriorityID     uint
	CustomerUserID string
	AssignedUserID uint // responsible agent
	Search         string
	// Sort is "created" (default), "updated", "priority", "tn" or "title".
	Sort string
	// Order is "asc" or "desc" (default).
	Order string
	// Include may contain "article_count" and "last_article".
	Include []string
}

// TicketList is the result of TicketsService.List.
type TicketList struct {
	Tickets    []TicketSummary
	Pagination Pagination
}

// Ticket is GET /api/v1/tickets/:id.
type Ticket struct {
	ID                uint      `json:"id"`
	TicketNumber      string    `json:"ticket_number"`
	Title             string    `json:"title"`
	StateID           uint      `json:"state_id"`
	State             string    `json:"state"`
	PriorityID        uint      `json:"priority_id"`
	Priority          string    `json:"priority"`
	QueueID           uint      `json:"queue_id"`
	Queue             string    `json:"queue"`
	TypeID            *uint     `json:"type_id,omitempty"`
	CustomerID        string    `json:"customer_id,omitempty"`
	CustomerUserID    string    `json:"customer_user_id,omitempty"`
	OwnerUserID       uint      `json:"owner_user_id"`
	ResponsibleUserID *uint     `json:"responsible_user_id,omitempty"`
	ArticleCount      int       `json:"article_count"`
	CreateTime        time.Time `json:"create_time"`
	ChangeTime        time.Time `json:"change_time"`
}

// TicketCreateRequest is the body of POST /api/v1/tickets. Title and QueueID
// are required; Body becomes the first article.
type TicketCreateRequest struct {
	Title          string `json:"title"`
	QueueID        uint   `json:"queue_id"`
	Body           string `json:"body,omitempty"`
	PriorityID     uint   `json:"priority_id,omitempty"`
	StateID        uint   `json:"state_id,omitempty"`
	TypeID         uint   `json:"type_id,omitempty"`
	CustomerEmail  string `json:"customer_email,omitempty"`
	CustomerID     string `json:"customer_id,omitempty"`
	CustomerUserID string `json:"customer_user_id,omitempty"`
}

// CreatedTicket is the data of POST /api/v1/tickets.
type CreatedTicket struct {
	ID         uint   `json:"id"`
	TN         string `json:"tn"`
	Title      string `json:"title"`
	QueueID    uint   `json:"queue_id"`
	StateID    uint   `json:"ticket_state_id"`
	PriorityID uint   `json:"ticket_priority_id"`
}

// TicketUpdateRequest is the body of PUT /api/v1/tickets/:id; nil fields are
// left unchanged.
type TicketUpdateRequest struct {
	Title             *string `json:"title,omitempty"`
	QueueID           *uint   `json:"queue_id,omitempty"`
	TypeID            *uint   `json:"type_id,omitempty"`
	StateID           *uint   `json:"state_id,omitempty"`
	PriorityID        *uint   `json:"priority_id,omitempty"`
	CustomerUserID    *string `json:"customer_user_id,omitempty"`
	CustomerID        *string `json:"customer_id,omitempty"`
	UserID            *uint   `json:"user_id,omitempty"` // owner
	ResponsibleUserID *uint   `json:"responsible_user_id,omitempty"`
	TicketLockID      *uint   `json:"ticket_lock_id,omitempty"`
}

// TicketRecord is the data of PUT /api/v1/tickets/:id: the ticket row after
// the update.
type TicketRecord struct {
	ID                uint      `json:"id"`
	TN                string    `json:"tn"`
	Title             string    `json:"title"`
	QueueID           uint      `json:"queue_id"`
	TypeID            uint      `json:"type_id"`
	StateID           uint      `json:"state_id"`
	PriorityID        uint      `json:"priority_id"`
	UserID            uint      `json:"user_id"` // owner
	ResponsibleUserID *uint     `json:"responsible_user_id"`
	TicketLockID      uint      `json:"ticket_lock_id"`
	CustomerUserID    string    `json:"customer_user_id"`
	CustomerID        string    `json:"customer_id"`
	CreateTime        time.Time `json:"create_time"`
	CreateBy          uint      `json:"create_by"`
	ChangeTime        time.Time `json:"change_time"`
	ChangeBy          uint      `json:"change_by"`
}

// ReopenResult is the response of POST /api/v1/tickets/:id/reopen.
type ReopenResult struct {
	ID         uint      `json:"id"`
	StateID    uint      `json:"state_id"`
	State      string    `json:"state"`
	Reason     string    `json:"reason"`
	ReopenedAt time.Time `json:"reopened_at"`
}

// Articles

// Article is a ticket article (email, note, phone call, ...).
type Article struct {
	ID                     uint   `json:"id"`
	TicketID               uint   `json:"ticket_id"`
	ArticleSenderTypeID    uint   `json:"article_sender_type_id"`
	SenderType             string `json:"sender_type,omitempty"`
	CommunicationChannelID uint   `json:"communication_channel_id"`
	IsVisibleForCustomer   bool   `json:"is_visible_for_customer"`
	// ArticleType is e.g. "email-external", "note-internal", "phone".
	ArticleType string    `json:"article_type"`
	From        string    `json:"from"`
	To          string    `json:"to"`
	Cc          string    `json:"cc"`
	Subject     string    `json:"subject"`
	Body        string    `json:"body"`
	ContentType string    `json:"content_type"`
	MessageID   string    `json:"message_id,omitempty"`
	CreateTime  time.Time `json:"create_time"`
	CreateBy    uint      `json:"create_by"`
	ChangeTime  time.Time `json:"change_time"`
	ChangeBy    uint      `json:"change_by"`
	// Only when requested with includeAttachments.
	Attachments []ArticleAttachment `json:"attachments,omitempty"`
}

// ArticleAttachment describes a file attached to an article.
type ArticleAttachment struct {
	ID          int64  `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	Disposition string `json:"disposition"`
}

// ArticleList is GET /api/v1/tickets/:id/articles, newest first.
type ArticleList struct {
	Articles []Article `json:"articles"`
	Total    int       `json:"total"`
}

// ArticleCreateRequest is the body of POST /api/v1/tickets/:id/articles.
// Body is required.
type ArticleCreateRequest struct {
	Subject     string `json:"subject,omitempty"`
	Body        string `json:"body"`
	ContentType string `json:"content_type,omitempty"`
	// ArticleType is e.g. "note-internal" (alias "note"), "note-external",
	// "email-external" (alias "email"), "phone".
	ArticleType string `json:"article_type,omitempty"`
	// SenderType is "agent", "customer" or "system".
	SenderType           string  `json:"sender_type,omitempty"`
	IsVisibleForCustomer *bool   `json:"is_visible_for_customer,omitempty"`
	From                 string  `json:"from,omitempty"`
	To                   string  `json:"to,omitempty"`
	Cc                   string  `json:"cc,omitempty"`
	TimeUnit             float64 `json:"time_unit,omitempty"`
}

// ArticleUpdateRequest is the body of PUT /api/v1/tickets/:id/articles/:aid;
// at least one field must be set.
type ArticleUpdateRequest struct {
	Subject *string `json:"subject,omitempty"`
	Body    *string `json:"body,omitempty"`
}

// ArticleUpdate is the response of PUT /api/v1/tickets/:id/articles/:aid.
type ArticleUpdate struct {
	ID       uint   `json:"id"`
	TicketID uint   `json:"ticket_id"`
	Subject  string `json:"subject"`
	Body     string `json:"body"`
}

// Users

// UserGroup is an agent's membership in a group with its permission keys
// (ro, move_into, create, note, owner, priority, rw).
type UserGroup struct {
	ID          uint     `json:"id"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

// User is an agent as returned by GET /api/v1/users and /api/v1/users/:id.
type User struct {
	ID         uint        `json:"id"`
	Login      string      `json:"login"`
	FirstName  string      `json:"first_name,omitempty"`
	LastName   string      `json:"last_name,omitempty"`
	ValidID    int         `json:"valid_id"`
	Valid      bool        `json:"valid"`
	CreateTime *time.Time  `json:"create_time,omitempty"`
	ChangeTime *time.Time  `json:"change_time,omitempty"`
	Groups     []UserGroup `json:"groups"`
	// Email and Preferences are only sent by GET /api/v1/users/:id.
	Email       string            `json:"email,omitempty"`
	Preferences map[string]string `json:"preferences,omitempty"`
}

// UserListOptions filters GET /api/v1/users. Zero values are not sent.
type UserListOptions struct {
	Page    int
	PerPage int
	Search  string
	// Valid is "1" for valid users only, "2" for invalid only.
	Valid   string
	GroupID uint
}

// UserList is the result of UsersService.List.
type UserList struct {
	Users      []User
	Pagination Pagination
}

// CurrentUser is GET /api/v1/users/me.
type CurrentUser struct {
	ID        uint       `json:"id"`
	Login     string     `json:"login"`
	Email     string     `json:"email"`
	FirstName string     `json:"first_name"`
	LastName  string     `json:"last_name"`
	Active    bool       `json:"active"`
	Groups    []GroupRef `json:"groups"`
}

// UserCreateRequest is the body of POST /api/v1/users. Login, Email and
// Password (8+ characters) are required; ValidID defaults to 1.
type UserCreateRequest struct {
	Login     string `json:"login"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	ValidID   int    `json:"valid_id,omitempty"`
	// Groups are group IDs to add the user to.
	Groups []uint `json:"groups,omitempty"`
}

// CreatedUser is the data of POST /api/v1/users.
type CreatedUser struct {
	ID        uint      `json:"id"`
	Login     string    `json:"login"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name,omitempty"`
	LastName  string    `json:"last_name,omitempty"`
	ValidID   int       `json:"valid_id"`
	Valid     bool      `json:"valid"`
	Groups    []uint    `json:"groups"`
	CreatedAt time.Time `json:"created_at"`
}

// UserUpdateRequest is the body of PUT /api/v1/users/:id; nil fields are
// left unchanged.
type UserUpdateRequest struct {
	Email     *string `json:"email,omitempty"`
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	Password  *string `json:"password,omitempty"`
	ValidID   *int    `json:"valid_id,omitempty"`
}

// Queues

// Queue is a ticket queue. A few fields are only sent by one of the two
// endpoints: Valid, Comment, CreateTime, ChangeTime and the ticket counts by
// GET /api/v1/queues; Comments, SalutationID and SignatureID by
// GET /api/v1/queues/:id.
type Queue struct {
	ID              uint       `json:"id"`
	Name            string     `json:"name"`
	ValidID         int        `json:"valid_id"`
	Valid           *bool      `json:"valid,omitempty"`
	GroupID         uint       `json:"group_id"`
	GroupName       string     `json:"group_name"`
	Groups          []GroupRef `json:"groups"`
	SystemAddressID *uint      `json:"system_address_id,omitempty"`
	SalutationID    *uint      `json:"salutation_id,omitempty"`
	SignatureID     *uint      `json:"signature_id,omitempty"`
	UnlockTimeout   *int       `json:"unlock_timeout,omitempty"`
	FollowUpID      *uint      `json:"follow_up_id,omitempty"`
	FollowUpLock    *int       `json:"follow_up_lock,omitempty"`
	Comment         string     `json:"comment,omitempty"`
	Comments        string     `json:"comments,omitempty"`
	CreateTime      *time.Time `json:"create_time,omitempty"`
	ChangeTime      *time.Time `json:"change_time,omitempty"`
	// Ticket counts, only with QueueListOptions.IncludeStats.
	TicketCount    *int `json:"ticket_count,omitempty"`
	OpenTickets    *int `json:"open_tickets,omitempty"`
	ClosedTickets  *int `json:"closed_tickets,omitempty"`
	PendingTickets *int `json:"pending_tickets,omitempty"`
}

// QueueListOptions filters GET /api/v1/queues. Zero values are not sent.
type QueueListOptions struct {
	// Valid is "1" for valid queues only, "2" for invalid only.
	Valid        string
	IncludeStats bool
}

// Statistics

// DashboardStatistics is GET /api/v1/statistics/dashboard, limited to the
// queues the caller can read.
type DashboardStatistics struct {
	Overview struct {
		TotalTickets   int `json:"total_tickets"`
		OpenTickets    int `json:"open_tickets"`
		ClosedTickets  int `json:"closed_tickets"`
		PendingTickets int `json:"pending_tickets"`
	} `json:"overview"`
	ByQueue []struct {
		QueueID   uint   `json:"queue_id"`
		QueueName string `json:"queue_name"`
		Count     int    `json:"count"`
	} `json:"by_queue"`
	ByPriority []struct {
		PriorityID   uint   `json:"priority_id"`
		PriorityName string `json:"priority_name"`
		Count        int    `json:"count"`
	} `json:"by_priority"`
	// RecentActivity holds the ten newest tickets.
	RecentActivity []struct {
		Type      string    `json:"type"`
		TicketID  uint      `json:"ticket_id"`
		TicketTN  string    `json:"ticket_tn"`
		Timestamp time.Time `json:"timestamp"`
	} `json:"recent_activity"`
}

// Search

// SearchQuery is the body of POST /api/v1/search. Query is required.
type SearchQuery struct {
	Query string `json:"query"`
	// Types defaults to ticket, article and customer.
	Types     []string          `json:"types,omitempty"`
	Filters   map[string]string `json:"filters,omitempty"`
	Offset    int               `json:"offset,omitempty"`
	Limit     int               `json:"limit,omitempty"` // default 20, max 100
	SortBy    string            `json:"sort_by,omitempty"`
	SortOrder string            `json:"sort_order,omitempty"`
	Highlight bool              `json:"highlight,omitempty"`
	Facets    []string          `json:"facets,omitempty"`
}

// SearchResults is the response of POST /api/v1/search. Warning is set (and
// Hits empty) when the search backend is unavailable.
type SearchResults struct {
	Query       string                   `json:"query"`
	TotalHits   int                      `json:"total_hits"`
	TookMS      int64                    `json:"took_ms"`
	Hits        []SearchHit              `json:"hits"`
	Facets      map[string][]SearchFacet `json:"facets,omitempty"`
	Suggestions []string                 `json:"suggestions,omitempty"`
	Warning     string                   `json:"warning,omitempty"`
}

// SearchHit is one search result.
type SearchHit struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Score      float64                `json:"score"`
	Title      string                 `json:"title"`
	Content    string                 `json:"content"`
	Highlights map[string][]string    `json:"highlights,omitempty"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// SearchFacet is one facet bucket.
type SearchFacet struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// Webhooks

// Webhook represents an outbound webhook. Secret and header values are
// write-only: responses carry HasSecret and the masked SecretHint, and
// HeaderHints (header name -> masked value) instead of Headers. In a request,
// Headers replaces the custom headers; a nil value keeps the stored value of
// that header, and leaving Headers nil keeps them all. RetryCount,
// TimeoutSeconds and IsActive are pointers so requests can leave them unset
// (server defaults 3, 10 and true; on update, unchanged).
type Webhook struct {
	ID             uint               `json:"id,omitempty"`
	Name           string             `json:"name,omitempty"`
	URL            string             `json:"url,omitempty"`
	Events         []string           `json:"events,omitempty"`
	Secret         *string            `json:"secret,omitempty"`
	HasSecret      bool               `json:"has_secret,omitempty"`
	SecretHint     string             `json:"secret_hint,omitempty"`
	Headers        map[string]*string `json:"headers,omitempty"`
	HeaderHints    map[string]string  `json:"header_hints,omitempty"`
	RetryCount     *int               `json:"retry_count,omitempty"`
	TimeoutSeconds *int               `json:"timeout_seconds,omitempty"`
	IsActive       *bool              `json:"is_active,omitempty"`
	CreatedAt      *time.Time         `json:"created_at,omitempty"`
	CreatedBy      int                `json:"created_by,omitempty"`
	UpdatedAt      *time.Time         `json:"updated_at,omitempty"`
	UpdatedBy      int                `json:"updated_by,omitempty"`
}

// WebhookDelivery is one event sent (or scheduled) to one webhook.
// Status is pending, delivering, delivered or failed. Payload and Response
// are only filled by WebhooksService.GetDelivery.
type WebhookDelivery struct {
	ID            uint       `json:"id"`
	WebhookID     uint       `json:"webhook_id"`
	Event         string     `json:"event"`
	Status        string     `json:"status"`
	Success       bool       `json:"success"`
	Attempts      int        `json:"attempts"`
	StatusCode    *int       `json:"status_code"`
	Error         string     `json:"error,omitempty"`
	DurationMS    *int       `json:"duration_ms"`
	NextAttemptAt *time.Time `json:"next_attempt_at,omitempty"`
	DeliveredAt   *time.Time `json:"delivered_at,omitempty"`
	Payload       string     `json:"payload,omitempty"`
	Response      string     `json:"response,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// Auth

// LoginRequest is the body of POST /api/v1/auth/login.
type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// TokenPair is the response of POST /api/v1/auth/login and
// POST /api/v1/auth/refresh. Every refresh rotates the refresh token.
type TokenPair struct {
	User struct {
		ID        uint   `json:"id"`
		Login     string `json:"login"`
		Email     string `json:"email"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Role      string `json:"role"`
	} `json:"user"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	// ExpiresIn is the access token lifetime in seconds.
	ExpiresIn int `json:"expires_in"`
	// RefreshExpiresIn is the refresh token lifetime in seconds.
	RefreshExpiresIn int `json:"refresh_expires_in"`
}

// ExpiresAt is when the access token expires, given when the response was
// received.
func (p *TokenPair) ExpiresAt(received time.Time) time.Time {
	return received.Add(time.Duration(p.ExpiresIn) * time.Second)
}

// Health is GET /health.
type Health struct {
	Status     string            `json:"status"`
	Components map[string]string `json:"components"`
	Version    string            `json:"version"`
}
