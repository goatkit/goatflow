package models

import (
	"time"
)

// EmailAccount mirrors Znuny's mail_account table plus derived metadata for the inbound poller.
type EmailAccount struct {
	ID                  int       `json:"id" db:"id"`
	Login               string    `json:"login" db:"login"`
	Host                string    `json:"host" db:"host"`
	AccountType         string    `json:"account_type" db:"account_type"`
	QueueID             int       `json:"queue_id" db:"queue_id"`
	DispatchingMode     string    `json:"dispatching_mode"`
	Trusted             bool      `json:"trusted" db:"trusted"`
	AllowTrustedHeaders bool      `json:"allow_trusted_headers"`
	IMAPFolder          *string   `json:"imap_folder,omitempty" db:"imap_folder"`
	Comments            *string   `json:"comments,omitempty" db:"comments"`
	ValidID             int       `json:"valid_id" db:"valid_id"`
	IsActive            bool      `json:"is_active"`
	PollIntervalSeconds int       `json:"poll_interval_seconds,omitempty"`
	PasswordEncrypted   string    `json:"-" db:"pw"`
	CreatedAt           time.Time `json:"created_at" db:"create_time"`
	CreatedBy           int       `json:"created_by" db:"create_by"`
	UpdatedAt           time.Time `json:"updated_at" db:"change_time"`
	UpdatedBy           int       `json:"updated_by" db:"change_by"`
	Queue               *Queue    `json:"queue,omitempty"`
}

// EmailTemplate represents an email template for automated responses.
type EmailTemplate struct {
	ID              int       `json:"id" db:"id"`
	TemplateName    string    `json:"template_name" db:"template_name"`
	SubjectTemplate *string   `json:"subject_template,omitempty" db:"subject_template"`
	BodyTemplate    *string   `json:"body_template,omitempty" db:"body_template"`
	TemplateType    *string   `json:"template_type,omitempty" db:"template_type"`
	IsActive        bool      `json:"is_active" db:"is_active"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	CreatedBy       int       `json:"created_by" db:"created_by"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
	UpdatedBy       int       `json:"updated_by" db:"updated_by"`
}

// Organization represents a customer organization.
type Organization struct {
	ID            string    `json:"id" db:"id"`
	Name          string    `json:"name" db:"name"`
	AddressLine1  *string   `json:"address_line1,omitempty" db:"address_line1"`
	AddressLine2  *string   `json:"address_line2,omitempty" db:"address_line2"`
	City          *string   `json:"city,omitempty" db:"city"`
	StateProvince *string   `json:"state_province,omitempty" db:"state_province"`
	PostalCode    *string   `json:"postal_code,omitempty" db:"postal_code"`
	Country       *string   `json:"country,omitempty" db:"country"`
	Website       *string   `json:"website,omitempty" db:"website"`
	Notes         *string   `json:"notes,omitempty" db:"notes"`
	IsActive      bool      `json:"is_active" db:"is_active"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	CreatedBy     int       `json:"created_by" db:"created_by"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
	UpdatedBy     int       `json:"updated_by" db:"updated_by"`
}

// CustomerAccount represents a customer account.
type CustomerAccount struct {
	ID             int       `json:"id" db:"id"`
	Username       string    `json:"username" db:"username"`
	Email          string    `json:"email" db:"email"`
	OrganizationID *string   `json:"organization_id,omitempty" db:"organization_id"`
	PasswordHash   *string   `json:"-" db:"password_hash"`
	FullName       *string   `json:"full_name,omitempty" db:"full_name"`
	PhoneNumber    *string   `json:"phone_number,omitempty" db:"phone_number"`
	MobileNumber   *string   `json:"mobile_number,omitempty" db:"mobile_number"`
	IsActive       bool      `json:"is_active" db:"is_active"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	CreatedBy      int       `json:"created_by" db:"created_by"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
	UpdatedBy      int       `json:"updated_by" db:"updated_by"`

	// Joined fields
	Organization *Organization `json:"organization,omitempty"`
}

// TicketCategory represents a category for ticket classification.
type TicketCategory struct {
	ID               int       `json:"id" db:"id"`
	Name             string    `json:"name" db:"name"`
	Description      *string   `json:"description,omitempty" db:"description"`
	ParentCategoryID *int      `json:"parent_category_id,omitempty" db:"parent_category_id"`
	IsActive         bool      `json:"is_active" db:"is_active"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	CreatedBy        int       `json:"created_by" db:"created_by"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
	UpdatedBy        int       `json:"updated_by" db:"updated_by"`

	// Joined fields
	ParentCategory *TicketCategory   `json:"parent_category,omitempty"`
	SubCategories  []*TicketCategory `json:"sub_categories,omitempty"`
}

// ArticleAttachment represents a file attachment to an article.
type ArticleAttachment struct {
	ID                 int       `json:"id" db:"id"`
	ArticleID          int       `json:"article_id" db:"article_id"`
	Filename           string    `json:"filename" db:"filename"`
	ContentType        string    `json:"content_type" db:"content_type"`
	ContentSize        int       `json:"content_size" db:"content_size"`
	ContentID          *string   `json:"content_id,omitempty" db:"content_id"`
	ContentAlternative *string   `json:"content_alternative,omitempty" db:"content_alternative"`
	Disposition        string    `json:"disposition" db:"disposition"`
	Content            string    `json:"content" db:"content"` // Base64 encoded or file path
	CreateTime         time.Time `json:"create_time" db:"create_time"`
	CreateBy           int       `json:"create_by" db:"create_by"`
	ChangeTime         time.Time `json:"change_time" db:"change_time"`
	ChangeBy           int       `json:"change_by" db:"change_by"`
}

// Template types.
const (
	TemplateTypeGreeting       = "greeting"
	TemplateTypeSignature      = "signature"
	TemplateTypeAutoReply      = "auto_reply"
	TemplateTypeTicketNew      = "ticket_created"
	TemplateTypeTicketUpdate   = "ticket_updated"
	TemplateTypeTicketAssigned = "ticket_assigned"
	TemplateTypeTicketClosed   = "ticket_closed"
)
