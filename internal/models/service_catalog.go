package models

import (
	"time"
)

// ServiceItemType represents the type of service catalog item.
type ServiceItemType string

const (
	ServiceItemTypeService  ServiceItemType = "service"
	ServiceItemTypeRequest  ServiceItemType = "request"
	ServiceItemTypeIncident ServiceItemType = "incident"
	ServiceItemTypeQuestion ServiceItemType = "question"
)

// ServiceItemStatus represents the status of a service catalog item.
type ServiceItemStatus string

const (
	ServiceItemStatusActive   ServiceItemStatus = "active"
	ServiceItemStatusInactive ServiceItemStatus = "inactive"
	ServiceItemStatusDraft    ServiceItemStatus = "draft"
	ServiceItemStatusRetired  ServiceItemStatus = "retired"
)

// RequestStatus represents the status of a service request.
type RequestStatus string

const (
	RequestStatusSubmitted  RequestStatus = "submitted"
	RequestStatusPending    RequestStatus = "pending"
	RequestStatusApproved   RequestStatus = "approved"
	RequestStatusRejected   RequestStatus = "rejected"
	RequestStatusInProgress RequestStatus = "in_progress"
	RequestStatusFulfilled  RequestStatus = "fulfilled"
	RequestStatusCancelled  RequestStatus = "cancelled"
	RequestStatusClosed     RequestStatus = "closed"
)

// ServiceCatalogItem represents an item in the service catalog.
type ServiceCatalogItem struct {
	ID               uint              `json:"id" gorm:"primaryKey"`
	ItemNumber       string            `json:"item_number" gorm:"uniqueIndex;not null"`
	Name             string            `json:"name" gorm:"not null;index"`
	DisplayName      string            `json:"display_name"`
	Description      string            `json:"description"`
	ShortDescription string            `json:"short_description"`
	Type             ServiceItemType   `json:"type" gorm:"not null;default:'request';index"`
	Status           ServiceItemStatus `json:"status" gorm:"not null;default:'active';index"`

	// Categorization
	CategoryID    uint             `json:"category_id" gorm:"index"`
	Category      *ServiceCategory `json:"category,omitempty" gorm:"foreignKey:CategoryID"`
	SubCategoryID *uint            `json:"sub_category_id"`
	SubCategory   *ServiceCategory `json:"sub_category,omitempty" gorm:"foreignKey:SubCategoryID"`
	Tags          []string         `json:"tags" gorm:"type:text[]"`
	Keywords      []string         `json:"keywords" gorm:"type:text[]"`

	// Display and ordering
	Icon       string `json:"icon"`
	Image      string `json:"image"`
	Color      string `json:"color"`
	Order      int    `json:"order" gorm:"default:0"`
	IsFeatured bool   `json:"is_featured" gorm:"default:false"`
	IsPopular  bool   `json:"is_popular" gorm:"default:false"`

	// Fulfillment details
	FulfillmentGroupID *uint   `json:"fulfillment_group_id"`
	FulfillmentGroup   *Group  `json:"fulfillment_group,omitempty" gorm:"foreignKey:FulfillmentGroupID"`
	EstimatedTime      int     `json:"estimated_time"` // Hours
	EstimatedCost      float64 `json:"estimated_cost"`
	RequiresApproval   bool    `json:"requires_approval" gorm:"default:false"`
	ApprovalLevels     int     `json:"approval_levels" gorm:"default:0"`
	AutoAssign         bool    `json:"auto_assign" gorm:"default:false"`

	// SLA and priority
	DefaultPriority   int `json:"default_priority" gorm:"default:3"`
	SLAResponseTime   int `json:"sla_response_time"`   // Minutes
	SLAResolutionTime int `json:"sla_resolution_time"` // Minutes

	// Form and workflow
	FormTemplateID    *uint         `json:"form_template_id"`
	FormTemplate      *FormTemplate `json:"form_template,omitempty" gorm:"foreignKey:FormTemplateID"`
	WorkflowID        *uint         `json:"workflow_id"`
	FulfillmentScript string        `json:"fulfillment_script"`

	// Access control
	AvailableToAll   bool    `json:"available_to_all" gorm:"default:true"`
	RestrictedGroups []Group `json:"restricted_groups,omitempty" gorm:"many2many:catalog_item_group_access;"`
	RestrictedRoles  []Role  `json:"restricted_roles,omitempty" gorm:"many2many:catalog_item_role_access;"`
	RequiresAuth     bool    `json:"requires_auth" gorm:"default:true"`

	// Related items
	RelatedItems  []ServiceCatalogItem `json:"related_items,omitempty" gorm:"many2many:catalog_item_relationships;"`
	Prerequisites []ServiceCatalogItem `json:"prerequisites,omitempty" gorm:"many2many:catalog_item_prerequisites;"`
	IncludedItems []ServiceCatalogItem `json:"included_items,omitempty" gorm:"many2many:catalog_item_inclusions;"`

	// Knowledge base
	KnowledgeArticles  []KnowledgeArticle `json:"knowledge_articles,omitempty" gorm:"many2many:catalog_item_articles;"`
	Instructions       string             `json:"instructions"`
	TermsAndConditions string             `json:"terms_and_conditions"`

	// Metrics
	RequestCount           int     `json:"request_count" gorm:"default:0"`
	FulfillmentRate        float64 `json:"fulfillment_rate" gorm:"default:0"`
	AverageRating          float64 `json:"average_rating" gorm:"default:0"`
	RatingCount            int     `json:"rating_count" gorm:"default:0"`
	AverageFulfillmentTime int     `json:"average_fulfillment_time"` // Minutes

	// Audit fields
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	CreatedByID      uint      `json:"created_by_id"`
	CreatedBy        *User     `json:"created_by,omitempty" gorm:"foreignKey:CreatedByID"`
	LastModifiedByID *uint     `json:"last_modified_by_id"`
	LastModifiedBy   *User     `json:"last_modified_by,omitempty" gorm:"foreignKey:LastModifiedByID"`
}

// ServiceCategory represents a category in the service catalog.
type ServiceCategory struct {
	ID          uint              `json:"id" gorm:"primaryKey"`
	Name        string            `json:"name" gorm:"not null;uniqueIndex"`
	DisplayName string            `json:"display_name"`
	Description string            `json:"description"`
	Icon        string            `json:"icon"`
	Image       string            `json:"image"`
	Color       string            `json:"color"`
	ParentID    *uint             `json:"parent_id"`
	Parent      *ServiceCategory  `json:"parent,omitempty" gorm:"foreignKey:ParentID"`
	Children    []ServiceCategory `json:"children,omitempty" gorm:"foreignKey:ParentID"`
	Order       int               `json:"order" gorm:"default:0"`
	IsActive    bool              `json:"is_active" gorm:"default:true"`
	ItemCount   int               `json:"item_count" gorm:"default:0"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// ServiceRequest represents a user's service request.
type ServiceRequest struct {
	ID            uint                `json:"id" gorm:"primaryKey"`
	RequestNumber string              `json:"request_number" gorm:"uniqueIndex;not null"`
	CatalogItemID uint                `json:"catalog_item_id" gorm:"not null"`
	CatalogItem   *ServiceCatalogItem `json:"catalog_item,omitempty" gorm:"foreignKey:CatalogItemID"`
	Status        RequestStatus       `json:"status" gorm:"not null;default:'submitted';index"`
	Priority      int                 `json:"priority" gorm:"default:3"`

	// Requester information
	RequesterID    uint   `json:"requester_id" gorm:"not null"`
	Requester      *User  `json:"requester,omitempty" gorm:"foreignKey:RequesterID"`
	RequestedForID *uint  `json:"requested_for_id"`
	RequestedFor   *User  `json:"requested_for,omitempty" gorm:"foreignKey:RequestedForID"`
	Department     string `json:"department"`
	Location       string `json:"location"`

	// Request details
	Title                 string                 `json:"title"`
	Description           string                 `json:"description"`
	BusinessJustification string                 `json:"business_justification"`
	FormData              map[string]interface{} `json:"form_data" gorm:"type:jsonb"`

	// Assignment and fulfillment
	AssignedToID      *uint  `json:"assigned_to_id"`
	AssignedTo        *User  `json:"assigned_to,omitempty" gorm:"foreignKey:AssignedToID"`
	AssignmentGroupID *uint  `json:"assignment_group_id"`
	AssignmentGroup   *Group `json:"assignment_group,omitempty" gorm:"foreignKey:AssignmentGroupID"`
	FulfillmentNotes  string `json:"fulfillment_notes"`

	// Related entities
	RelatedTicketID   *uint     `json:"related_ticket_id"`
	RelatedTicket     *Ticket   `json:"related_ticket,omitempty" gorm:"foreignKey:RelatedTicketID"`
	RelatedIncidentID *uint     `json:"related_incident_id"`
	RelatedIncident   *Incident `json:"related_incident,omitempty" gorm:"foreignKey:RelatedIncidentID"`
	RelatedChangeID   *uint     `json:"related_change_id"`
	RelatedChange     *Change   `json:"related_change,omitempty" gorm:"foreignKey:RelatedChangeID"`

	// Approval process
	ApprovalStatus string            `json:"approval_status"` // pending, approved, rejected
	Approvals      []RequestApproval `json:"approvals,omitempty" gorm:"foreignKey:RequestID"`

	// Time tracking
	SubmittedAt  time.Time  `json:"submitted_at" gorm:"not null"`
	AssignedAt   *time.Time `json:"assigned_at"`
	InProgressAt *time.Time `json:"in_progress_at"`
	FulfilledAt  *time.Time `json:"fulfilled_at"`
	ClosedAt     *time.Time `json:"closed_at"`

	// SLA tracking
	ResponseDue    *time.Time `json:"response_due"`
	FulfillmentDue *time.Time `json:"fulfillment_due"`
	ResponseMet    bool       `json:"response_met" gorm:"default:false"`
	FulfillmentMet bool       `json:"fulfillment_met" gorm:"default:false"`

	// Cost and effort
	EstimatedCost   float64 `json:"estimated_cost"`
	ActualCost      float64 `json:"actual_cost"`
	EstimatedEffort int     `json:"estimated_effort"` // Hours
	ActualEffort    int     `json:"actual_effort"`    // Hours

	// Feedback
	SatisfactionRating *int   `json:"satisfaction_rating"` // 1-5 scale
	Feedback           string `json:"feedback"`

	// Audit fields
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedByID uint      `json:"created_by_id"`
	CreatedBy   *User     `json:"created_by,omitempty" gorm:"foreignKey:CreatedByID"`
}

// RequestApproval represents an approval for a service request.
type RequestApproval struct {
	ID         uint            `json:"id" gorm:"primaryKey"`
	RequestID  uint            `json:"request_id" gorm:"not null"`
	Request    *ServiceRequest `json:"request,omitempty" gorm:"foreignKey:RequestID"`
	ApproverID uint            `json:"approver_id" gorm:"not null"`
	Approver   *User           `json:"approver,omitempty" gorm:"foreignKey:ApproverID"`
	Level      int             `json:"level" gorm:"default:1"`
	Status     string          `json:"status"` // pending, approved, rejected
	Comments   string          `json:"comments"`
	ApprovedAt *time.Time      `json:"approved_at"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// RequestComment represents a comment on a service request.
type RequestComment struct {
	ID         uint            `json:"id" gorm:"primaryKey"`
	RequestID  uint            `json:"request_id" gorm:"not null"`
	Request    *ServiceRequest `json:"request,omitempty" gorm:"foreignKey:RequestID"`
	AuthorID   uint            `json:"author_id" gorm:"not null"`
	Author     *User           `json:"author,omitempty" gorm:"foreignKey:AuthorID"`
	Comment    string          `json:"comment" gorm:"not null"`
	IsPublic   bool            `json:"is_public" gorm:"default:true"`
	IsWorkNote bool            `json:"is_work_note" gorm:"default:false"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// RequestAttachment represents a file attached to a service request.
type RequestAttachment struct {
	ID           uint            `json:"id" gorm:"primaryKey"`
	RequestID    uint            `json:"request_id" gorm:"not null"`
	Request      *ServiceRequest `json:"request,omitempty" gorm:"foreignKey:RequestID"`
	FileName     string          `json:"file_name" gorm:"not null"`
	FilePath     string          `json:"file_path" gorm:"not null"`
	FileSize     int64           `json:"file_size"`
	ContentType  string          `json:"content_type"`
	UploadedByID uint            `json:"uploaded_by_id" gorm:"not null"`
	UploadedBy   *User           `json:"uploaded_by,omitempty" gorm:"foreignKey:UploadedByID"`
	CreatedAt    time.Time       `json:"created_at"`
}

// FormTemplate represents a form template for service requests.
type FormTemplate struct {
	ID              uint                   `json:"id" gorm:"primaryKey"`
	Name            string                 `json:"name" gorm:"not null;uniqueIndex"`
	Description     string                 `json:"description"`
	FormSchema      map[string]interface{} `json:"form_schema" gorm:"type:jsonb"`
	ValidationRules map[string]interface{} `json:"validation_rules" gorm:"type:jsonb"`
	IsActive        bool                   `json:"is_active" gorm:"default:true"`
	Version         int                    `json:"version" gorm:"default:1"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// ServiceCatalogListRequest represents a request to list catalog items.
type ServiceCatalogListRequest struct {
	Page         int               `json:"page" form:"page"`
	PerPage      int               `json:"per_page" form:"per_page"`
	Type         ServiceItemType   `json:"type" form:"type"`
	Status       ServiceItemStatus `json:"status" form:"status"`
	CategoryID   uint              `json:"category_id" form:"category_id"`
	Search       string            `json:"search" form:"search"`
	Tags         []string          `json:"tags" form:"tags"`
	OnlyFeatured bool              `json:"only_featured" form:"only_featured"`
	OnlyPopular  bool              `json:"only_popular" form:"only_popular"`
	SortBy       string            `json:"sort_by" form:"sort_by"`
	SortOrder    string            `json:"sort_order" form:"sort_order"`
}

// ServiceCatalogListResponse represents a response containing catalog items.
type ServiceCatalogListResponse struct {
	Items      []*ServiceCatalogItem `json:"items"`
	Total      int64                 `json:"total"`
	Page       int                   `json:"page"`
	PerPage    int                   `json:"per_page"`
	TotalPages int                   `json:"total_pages"`
}

// ServiceRequestListRequest represents a request to list service requests.
type ServiceRequestListRequest struct {
	Page          int           `json:"page" form:"page"`
	PerPage       int           `json:"per_page" form:"per_page"`
	Status        RequestStatus `json:"status" form:"status"`
	RequesterID   uint          `json:"requester_id" form:"requester_id"`
	AssignedToID  uint          `json:"assigned_to_id" form:"assigned_to_id"`
	CatalogItemID uint          `json:"catalog_item_id" form:"catalog_item_id"`
	Search        string        `json:"search" form:"search"`
	SortBy        string        `json:"sort_by" form:"sort_by"`
	SortOrder     string        `json:"sort_order" form:"sort_order"`
	FromDate      *time.Time    `json:"from_date" form:"from_date"`
	ToDate        *time.Time    `json:"to_date" form:"to_date"`
}

// ServiceRequestListResponse represents a response containing service requests.
type ServiceRequestListResponse struct {
	Requests   []*ServiceRequest `json:"requests"`
	Total      int64             `json:"total"`
	Page       int               `json:"page"`
	PerPage    int               `json:"per_page"`
	TotalPages int               `json:"total_pages"`
}
