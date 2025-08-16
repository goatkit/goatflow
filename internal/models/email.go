package models

import (
	"time"
)

// EmailAccount represents an email account for sending/receiving emails
type EmailAccount struct {
	ID                     int       `json:"id" db:"id"`
	EmailAddress           string    `json:"email_address" db:"email_address"`
	DisplayName            *string   `json:"display_name,omitempty" db:"display_name"`
	SMTPHost               *string   `json:"smtp_host,omitempty" db:"smtp_host"`
	SMTPPort               *int      `json:"smtp_port,omitempty" db:"smtp_port"`
	SMTPUsername           *string   `json:"smtp_username,omitempty" db:"smtp_username"`
	SMTPPasswordEncrypted  *string   `json:"-" db:"smtp_password_encrypted"`
	IMAPHost               *string   `json:"imap_host,omitempty" db:"imap_host"`
	IMAPPort               *int      `json:"imap_port,omitempty" db:"imap_port"`
	IMAPUsername           *string   `json:"imap_username,omitempty" db:"imap_username"`
	IMAPPasswordEncrypted  *string   `json:"-" db:"imap_password_encrypted"`
	QueueID                *int      `json:"queue_id,omitempty" db:"queue_id"`
	IsActive               bool      `json:"is_active" db:"is_active"`
	CreatedAt              time.Time `json:"created_at" db:"created_at"`
	CreatedBy              int       `json:"created_by" db:"created_by"`
	UpdatedAt              time.Time `json:"updated_at" db:"updated_at"`
	UpdatedBy              int       `json:"updated_by" db:"updated_by"`
	
	// Joined fields
	Queue *Queue `json:"queue,omitempty"`
}

// EmailTemplate represents an email template for automated responses
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

// Organization represents a customer organization
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

// CustomerAccount represents a customer account
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

// TicketCategory represents a category for ticket classification
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

// ArticleAttachment represents a file attachment to an article
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

// Template types
const (
	TemplateTypeGreeting  = "greeting"
	TemplateTypeSignature = "signature"
	TemplateTypeAutoReply = "auto_reply"
	TemplateTypeTicketNew = "ticket_created"
	TemplateTypeTicketUpdate = "ticket_updated"
	TemplateTypeTicketAssigned = "ticket_assigned"
	TemplateTypeTicketClosed = "ticket_closed"
)