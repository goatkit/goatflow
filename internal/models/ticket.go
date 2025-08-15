package models

import (
	"database/sql"
	"time"
)

// TicketState represents the state of a ticket
type TicketState struct {
	ID         uint      `json:"id" db:"id"`
	Name       string    `json:"name" db:"name"`
	TypeID     int       `json:"type_id" db:"type_id"` // 1=new, 2=open, 3=closed, 4=removed, 5=pending
	ValidID    int       `json:"valid_id" db:"valid_id"`
	CreateTime time.Time `json:"create_time" db:"create_time"`
	CreateBy   uint      `json:"create_by" db:"create_by"`
	ChangeTime time.Time `json:"change_time" db:"change_time"`
	ChangeBy   uint      `json:"change_by" db:"change_by"`
}

// TicketPriority represents the priority level of a ticket
type TicketPriority struct {
	ID         uint      `json:"id" db:"id"`
	Name       string    `json:"name" db:"name"`
	ValidID    int       `json:"valid_id" db:"valid_id"`
	CreateTime time.Time `json:"create_time" db:"create_time"`
	CreateBy   uint      `json:"create_by" db:"create_by"`
	ChangeTime time.Time `json:"change_time" db:"change_time"`
	ChangeBy   uint      `json:"change_by" db:"change_by"`
}

// Queue represents a ticket queue
type Queue struct {
	ID              uint      `json:"id" db:"id"`
	Name            string    `json:"name" db:"name"`
	GroupID         uint      `json:"group_id" db:"group_id"`
	Comment         *string   `json:"comment,omitempty" db:"comment"`
	UnlockTimeout   int       `json:"unlock_timeout" db:"unlock_timeout"`
	FollowUpID      int       `json:"follow_up_id" db:"follow_up_id"`
	FollowUpLock    int       `json:"follow_up_lock" db:"follow_up_lock"`
	ValidID         int       `json:"valid_id" db:"valid_id"`
	CreateTime      time.Time `json:"create_time" db:"create_time"`
	CreateBy        uint      `json:"create_by" db:"create_by"`
	ChangeTime      time.Time `json:"change_time" db:"change_time"`
	ChangeBy        uint      `json:"change_by" db:"change_by"`
}

// Ticket represents a support ticket
type Ticket struct {
	ID                     uint       `json:"id" db:"id"`
	TN                     string     `json:"tn" db:"tn"` // Ticket Number
	Title                  string     `json:"title" db:"title"`
	QueueID                uint       `json:"queue_id" db:"queue_id"`
	TicketLockID           int        `json:"ticket_lock_id" db:"ticket_lock_id"` // 1=unlock, 2=lock, 3=tmp_lock
	TypeID                 int        `json:"type_id" db:"type_id"`
	ServiceID              *uint      `json:"service_id,omitempty" db:"service_id"`
	SLAID                  *uint      `json:"sla_id,omitempty" db:"sla_id"`
	UserID                 *uint      `json:"user_id,omitempty" db:"user_id"` // Owner
	ResponsibleUserID      *uint      `json:"responsible_user_id,omitempty" db:"responsible_user_id"`
	CustomerID             *uint      `json:"customer_id,omitempty" db:"customer_id"`
	CustomerUserID         *string    `json:"customer_user_id,omitempty" db:"customer_user_id"`
	TicketStateID          uint       `json:"ticket_state_id" db:"ticket_state_id"`
	TicketPriorityID       uint       `json:"ticket_priority_id" db:"ticket_priority_id"`
	UntilTime              int        `json:"until_time" db:"until_time"`
	EscalationTime         int        `json:"escalation_time" db:"escalation_time"`
	EscalationUpdateTime   int        `json:"escalation_update_time" db:"escalation_update_time"`
	EscalationResponseTime int        `json:"escalation_response_time" db:"escalation_response_time"`
	EscalationSolutionTime int        `json:"escalation_solution_time" db:"escalation_solution_time"`
	ArchiveFlag            int        `json:"archive_flag" db:"archive_flag"` // 0=not archived, 1=archived
	CreateTime             time.Time  `json:"create_time" db:"create_time"`
	CreateBy               uint       `json:"create_by" db:"create_by"`
	ChangeTime             time.Time  `json:"change_time" db:"change_time"`
	ChangeBy               uint       `json:"change_by" db:"change_by"`
	
	// Joined fields (populated when needed)
	Queue          *Queue          `json:"queue,omitempty"`
	State          *TicketState    `json:"state,omitempty"`
	Priority       *TicketPriority `json:"priority,omitempty"`
	Owner          *User           `json:"owner,omitempty"`
	Customer       *User           `json:"customer,omitempty"`
	ResponsibleUser *User          `json:"responsible_user,omitempty"`
}

// Article represents a message/comment within a ticket
type Article struct {
	ID                    uint      `json:"id" db:"id"`
	TicketID              uint      `json:"ticket_id" db:"ticket_id"`
	ArticleTypeID         int       `json:"article_type_id" db:"article_type_id"` // 1=email-external, 2=email-internal, etc.
	SenderTypeID          int       `json:"sender_type_id" db:"sender_type_id"`   // 1=agent, 2=system, 3=customer
	CommunicationChannelID int      `json:"communication_channel_id" db:"communication_channel_id"`
	IsVisibleForCustomer  int       `json:"is_visible_for_customer" db:"is_visible_for_customer"`
	Subject               *string   `json:"subject,omitempty" db:"subject"`
	Body                  string    `json:"body" db:"body"`
	BodyType              string    `json:"body_type" db:"body_type"` // text/plain, text/html
	Charset               string    `json:"charset" db:"charset"`
	MimeType              string    `json:"mime_type" db:"mime_type"`
	ContentPath           *string   `json:"content_path,omitempty" db:"content_path"`
	ValidID               int       `json:"valid_id" db:"valid_id"`
	CreateTime            time.Time `json:"create_time" db:"create_time"`
	CreateBy              uint      `json:"create_by" db:"create_by"`
	ChangeTime            time.Time `json:"change_time" db:"change_time"`
	ChangeBy              uint      `json:"change_by" db:"change_by"`
	
	// Joined fields
	Ticket      *Ticket       `json:"ticket,omitempty"`
	Creator     *User         `json:"creator,omitempty"`
	Attachments []Attachment  `json:"attachments,omitempty"`
}

// Attachment represents a file attachment to an article
type Attachment struct {
	ID                uint      `json:"id" db:"id"`
	ArticleID         uint      `json:"article_id" db:"article_id"`
	Filename          string    `json:"filename" db:"filename"`
	ContentType       string    `json:"content_type" db:"content_type"`
	ContentSize       int       `json:"content_size" db:"content_size"`
	ContentID         *string   `json:"content_id,omitempty" db:"content_id"`
	ContentAlternative *string  `json:"content_alternative,omitempty" db:"content_alternative"`
	Disposition       string    `json:"disposition" db:"disposition"`
	Content           string    `json:"content" db:"content"` // Base64 encoded or file path
	CreateTime        time.Time `json:"create_time" db:"create_time"`
	CreateBy          uint      `json:"create_by" db:"create_by"`
	ChangeTime        time.Time `json:"change_time" db:"change_time"`
	ChangeBy          uint      `json:"change_by" db:"change_by"`
}

// TicketCreateRequest represents a request to create a new ticket
type TicketCreateRequest struct {
	Title            string   `json:"title" binding:"required,min=1,max=255"`
	QueueID          uint     `json:"queue_id" binding:"required"`
	PriorityID       uint     `json:"priority_id" binding:"required"`
	StateID          uint     `json:"state_id,omitempty"`
	CustomerID       *uint    `json:"customer_id,omitempty"`
	CustomerUserID   *string  `json:"customer_user_id,omitempty"`
	Body             string   `json:"body" binding:"required"`
	BodyType         string   `json:"body_type,omitempty"` // defaults to text/plain
	Subject          string   `json:"subject,omitempty"`
	Attachments      []string `json:"attachments,omitempty"` // Base64 encoded files
}

// TicketUpdateRequest represents a request to update a ticket
type TicketUpdateRequest struct {
	Title             *string `json:"title,omitempty" binding:"omitempty,min=1,max=255"`
	QueueID           *uint   `json:"queue_id,omitempty"`
	PriorityID        *uint   `json:"priority_id,omitempty"`
	StateID           *uint   `json:"state_id,omitempty"`
	UserID            *uint   `json:"user_id,omitempty"` // Owner
	ResponsibleUserID *uint   `json:"responsible_user_id,omitempty"`
	CustomerID        *uint   `json:"customer_id,omitempty"`
	CustomerUserID    *string `json:"customer_user_id,omitempty"`
	TicketLockID      *int    `json:"ticket_lock_id,omitempty"`
}

// ArticleCreateRequest represents a request to add an article to a ticket
type ArticleCreateRequest struct {
	TicketID             uint     `json:"ticket_id" binding:"required"`
	ArticleTypeID        int      `json:"article_type_id,omitempty"` // defaults to note-internal
	SenderTypeID         int      `json:"sender_type_id,omitempty"`  // defaults based on user role
	IsVisibleForCustomer int      `json:"is_visible_for_customer,omitempty"`
	Subject              *string  `json:"subject,omitempty"`
	Body                 string   `json:"body" binding:"required"`
	BodyType             string   `json:"body_type,omitempty"` // defaults to text/plain
	Attachments          []string `json:"attachments,omitempty"` // Base64 encoded files
}

// TicketListRequest represents query parameters for listing tickets
type TicketListRequest struct {
	Page         int      `json:"page,omitempty" form:"page"`
	PerPage      int      `json:"per_page,omitempty" form:"per_page"`
	QueueID      *uint    `json:"queue_id,omitempty" form:"queue_id"`
	StateID      *uint    `json:"state_id,omitempty" form:"state_id"`
	PriorityID   *uint    `json:"priority_id,omitempty" form:"priority_id"`
	CustomerID   *uint    `json:"customer_id,omitempty" form:"customer_id"`
	OwnerID      *uint    `json:"owner_id,omitempty" form:"owner_id"`
	Search       string   `json:"search,omitempty" form:"search"`
	SortBy       string   `json:"sort_by,omitempty" form:"sort_by"`
	SortOrder    string   `json:"sort_order,omitempty" form:"sort_order"`
	ArchiveFlag  *int     `json:"archive_flag,omitempty" form:"archive_flag"`
	StartDate    *string  `json:"start_date,omitempty" form:"start_date"`
	EndDate      *string  `json:"end_date,omitempty" form:"end_date"`
}

// TicketListResponse represents a paginated list of tickets
type TicketListResponse struct {
	Tickets    []Ticket `json:"tickets"`
	Total      int      `json:"total"`
	Page       int      `json:"page"`
	PerPage    int      `json:"per_page"`
	TotalPages int      `json:"total_pages"`
}

// Constants for ticket states
const (
	TicketStateNew     = 1
	TicketStateOpen    = 2
	TicketStateClosed  = 3
	TicketStateRemoved = 4
	TicketStatePending = 5
)

// Constants for ticket lock states
const (
	TicketUnlocked  = 1
	TicketLocked    = 2
	TicketTmpLocked = 3
)

// Constants for article types
const (
	ArticleTypeEmailExternal = 1
	ArticleTypeEmailInternal = 2
	ArticleTypePhone         = 3
	ArticleTypeFax           = 4
	ArticleTypeSMS           = 5
	ArticleTypeWebRequest    = 6
	ArticleTypeNoteInternal  = 7
	ArticleTypeNoteExternal  = 8
)

// Constants for sender types
const (
	SenderTypeAgent    = 1
	SenderTypeSystem   = 2
	SenderTypeCustomer = 3
)

// Helper methods

// IsLocked returns true if the ticket is locked
func (t *Ticket) IsLocked() bool {
	return t.TicketLockID != TicketUnlocked
}

// IsClosed returns true if the ticket is in a closed state
func (t *Ticket) IsClosed() bool {
	return t.State != nil && t.State.TypeID == TicketStateClosed
}

// IsArchived returns true if the ticket is archived
func (t *Ticket) IsArchived() bool {
	return t.ArchiveFlag == 1
}

// CanBeEditedBy checks if a user can edit this ticket
func (t *Ticket) CanBeEditedBy(userID uint, role string) bool {
	// Admins can always edit
	if role == "Admin" {
		return true
	}
	
	// Agents can edit if they own the ticket or it's unassigned
	if role == "Agent" {
		if t.UserID == nil || *t.UserID == userID {
			return true
		}
		if t.ResponsibleUserID != nil && *t.ResponsibleUserID == userID {
			return true
		}
	}
	
	// Customers can only edit their own tickets if not locked
	if role == "Customer" {
		if t.CustomerID != nil && *t.CustomerID == userID && !t.IsLocked() {
			return true
		}
	}
	
	return false
}

// NullableUint converts a uint to a nullable uint for database operations
func NullableUint(v uint) *uint {
	if v == 0 {
		return nil
	}
	return &v
}

// NullableString converts a string to a nullable string for database operations
func NullableString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// DerefUint safely dereferences a uint pointer
func DerefUint(p *uint) uint {
	if p == nil {
		return 0
	}
	return *p
}

// DerefString safely dereferences a string pointer
func DerefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ValidateTicketState validates that a state ID is valid
func ValidateTicketState(stateID uint) bool {
	return stateID >= TicketStateNew && stateID <= TicketStatePending
}

// ValidateTicketLock validates that a lock ID is valid
func ValidateTicketLock(lockID int) bool {
	return lockID >= TicketUnlocked && lockID <= TicketTmpLocked
}

// ValidateArticleType validates that an article type ID is valid
func ValidateArticleType(typeID int) bool {
	return typeID >= ArticleTypeEmailExternal && typeID <= ArticleTypeNoteExternal
}

// ValidateSenderType validates that a sender type ID is valid
func ValidateSenderType(typeID int) bool {
	return typeID >= SenderTypeAgent && typeID <= SenderTypeCustomer
}

// ScanTicket is a helper to scan a ticket row from the database
func ScanTicket(rows *sql.Rows) (*Ticket, error) {
	var t Ticket
	err := rows.Scan(
		&t.ID,
		&t.TN,
		&t.Title,
		&t.QueueID,
		&t.TicketLockID,
		&t.TypeID,
		&t.ServiceID,
		&t.SLAID,
		&t.UserID,
		&t.ResponsibleUserID,
		&t.CustomerID,
		&t.CustomerUserID,
		&t.TicketStateID,
		&t.TicketPriorityID,
		&t.UntilTime,
		&t.EscalationTime,
		&t.EscalationUpdateTime,
		&t.EscalationResponseTime,
		&t.EscalationSolutionTime,
		&t.ArchiveFlag,
		&t.CreateTime,
		&t.CreateBy,
		&t.ChangeTime,
		&t.ChangeBy,
	)
	return &t, err
}