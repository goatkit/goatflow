package models

import (
	"time"
)

// ProblemStatus represents the current status of a problem.
type ProblemStatus string

const (
	ProblemStatusNew        ProblemStatus = "new"
	ProblemStatusAssigned   ProblemStatus = "assigned"
	ProblemStatusAccepted   ProblemStatus = "accepted"
	ProblemStatusInProgress ProblemStatus = "in_progress"
	ProblemStatusPending    ProblemStatus = "pending"
	ProblemStatusKnownError ProblemStatus = "known_error"
	ProblemStatusResolved   ProblemStatus = "resolved"
	ProblemStatusClosed     ProblemStatus = "closed"
)

// ProblemPriority represents the priority of a problem.
type ProblemPriority string

const (
	ProblemPriorityCritical ProblemPriority = "critical"
	ProblemPriorityHigh     ProblemPriority = "high"
	ProblemPriorityMedium   ProblemPriority = "medium"
	ProblemPriorityLow      ProblemPriority = "low"
	ProblemPriorityPlanning ProblemPriority = "planning"
)

// Problem represents an ITSM problem record.
type Problem struct {
	ID            uint            `json:"id" gorm:"primaryKey"`
	ProblemNumber string          `json:"problem_number" gorm:"uniqueIndex;not null"`
	Title         string          `json:"title" gorm:"not null"`
	Description   string          `json:"description"`
	Status        ProblemStatus   `json:"status" gorm:"not null;default:'new'"`
	Priority      ProblemPriority `json:"priority" gorm:"not null;default:'medium'"`
	Category      string          `json:"category"`
	SubCategory   string          `json:"sub_category"`

	// Impact and urgency
	Impact  int `json:"impact" gorm:"default:2"`  // 1-5 scale
	Urgency int `json:"urgency" gorm:"default:2"` // 1-5 scale

	// Assignment and ownership
	AssignedToID      *uint  `json:"assigned_to_id"`
	AssignedTo        *User  `json:"assigned_to,omitempty" gorm:"foreignKey:AssignedToID"`
	AssignmentGroupID *uint  `json:"assignment_group_id"`
	AssignmentGroup   *Group `json:"assignment_group,omitempty" gorm:"foreignKey:AssignmentGroupID"`

	// Related entities
	ReportedByID uint  `json:"reported_by_id" gorm:"not null"`
	ReportedBy   *User `json:"reported_by,omitempty" gorm:"foreignKey:ReportedByID"`

	// Related incidents
	Incidents     []Incident `json:"incidents,omitempty" gorm:"foreignKey:ProblemID"`
	IncidentCount int        `json:"incident_count" gorm:"default:0"`

	// Configuration items (CMDB)
	ConfigurationItemID *uint              `json:"configuration_item_id"`
	ConfigurationItem   *ConfigurationItem `json:"configuration_item,omitempty" gorm:"foreignKey:ConfigurationItemID"`

	// Service information
	ServiceID     *uint    `json:"service_id"`
	Service       *Service `json:"service,omitempty" gorm:"foreignKey:ServiceID"`
	ServiceImpact string   `json:"service_impact"`

	// Root cause analysis
	RootCause         string `json:"root_cause"`
	RootCauseAnalysis string `json:"root_cause_analysis"`
	IsRootCauseFound  bool   `json:"is_root_cause_found" gorm:"default:false"`

	// Known error database
	IsKnownError        bool   `json:"is_known_error" gorm:"default:false"`
	KnownErrorID        string `json:"known_error_id"`
	KnownErrorArticleID *uint  `json:"known_error_article_id"`
	Workaround          string `json:"workaround"`
	WorkaroundProvided  bool   `json:"workaround_provided" gorm:"default:false"`

	// Resolution details
	ResolutionCode     string `json:"resolution_code"`
	ResolutionNotes    string `json:"resolution_notes"`
	PermanentFix       string `json:"permanent_fix"`
	PreventiveMeasures string `json:"preventive_measures"`

	// Time tracking
	DetectedAt   *time.Time `json:"detected_at"`
	ReportedAt   time.Time  `json:"reported_at" gorm:"not null"`
	AssignedAt   *time.Time `json:"assigned_at"`
	InProgressAt *time.Time `json:"in_progress_at"`
	KnownErrorAt *time.Time `json:"known_error_at"`
	ResolvedAt   *time.Time `json:"resolved_at"`
	ClosedAt     *time.Time `json:"closed_at"`

	// SLA tracking
	ResponseDue   *time.Time `json:"response_due"`
	ResolutionDue *time.Time `json:"resolution_due"`
	ResponseMet   bool       `json:"response_met" gorm:"default:false"`
	ResolutionMet bool       `json:"resolution_met" gorm:"default:false"`

	// Metrics
	TimeToIdentify   *int `json:"time_to_identify"`    // Minutes
	TimeToKnownError *int `json:"time_to_known_error"` // Minutes
	TimeToResolve    *int `json:"time_to_resolve"`     // Minutes
	ReopenCount      int  `json:"reopen_count" gorm:"default:0"`

	// Review and approval
	RequiresReview bool       `json:"requires_review" gorm:"default:false"`
	ReviewedByID   *uint      `json:"reviewed_by_id"`
	ReviewedBy     *User      `json:"reviewed_by,omitempty" gorm:"foreignKey:ReviewedByID"`
	ReviewedAt     *time.Time `json:"reviewed_at"`
	ReviewNotes    string     `json:"review_notes"`

	// Additional fields
	Tags              []string `json:"tags" gorm:"type:text[]"`
	ExternalReference string   `json:"external_reference"`
	EstimatedEffort   int      `json:"estimated_effort"` // Hours
	ActualEffort      int      `json:"actual_effort"`    // Hours

	// Audit fields
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	CreatedByID      uint      `json:"created_by_id"`
	CreatedBy        *User     `json:"created_by,omitempty" gorm:"foreignKey:CreatedByID"`
	LastModifiedByID *uint     `json:"last_modified_by_id"`
	LastModifiedBy   *User     `json:"last_modified_by,omitempty" gorm:"foreignKey:LastModifiedByID"`
}

// ProblemComment represents a comment on a problem.
type ProblemComment struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	ProblemID  uint      `json:"problem_id" gorm:"not null"`
	Problem    *Problem  `json:"problem,omitempty" gorm:"foreignKey:ProblemID"`
	AuthorID   uint      `json:"author_id" gorm:"not null"`
	Author     *User     `json:"author,omitempty" gorm:"foreignKey:AuthorID"`
	Comment    string    `json:"comment" gorm:"not null"`
	IsPublic   bool      `json:"is_public" gorm:"default:true"`
	IsWorkNote bool      `json:"is_work_note" gorm:"default:false"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ProblemAttachment represents a file attached to a problem.
type ProblemAttachment struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	ProblemID    uint      `json:"problem_id" gorm:"not null"`
	Problem      *Problem  `json:"problem,omitempty" gorm:"foreignKey:ProblemID"`
	FileName     string    `json:"file_name" gorm:"not null"`
	FilePath     string    `json:"file_path" gorm:"not null"`
	FileSize     int64     `json:"file_size"`
	ContentType  string    `json:"content_type"`
	UploadedByID uint      `json:"uploaded_by_id" gorm:"not null"`
	UploadedBy   *User     `json:"uploaded_by,omitempty" gorm:"foreignKey:UploadedByID"`
	CreatedAt    time.Time `json:"created_at"`
}

// ProblemHistory tracks changes to a problem.
type ProblemHistory struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	ProblemID    uint      `json:"problem_id" gorm:"not null"`
	Problem      *Problem  `json:"problem,omitempty" gorm:"foreignKey:ProblemID"`
	FieldName    string    `json:"field_name" gorm:"not null"`
	OldValue     string    `json:"old_value"`
	NewValue     string    `json:"new_value"`
	ChangedByID  uint      `json:"changed_by_id" gorm:"not null"`
	ChangedBy    *User     `json:"changed_by,omitempty" gorm:"foreignKey:ChangedByID"`
	ChangeReason string    `json:"change_reason"`
	CreatedAt    time.Time `json:"created_at"`
}

// ProblemListRequest represents a request to list problems.
type ProblemListRequest struct {
	Page         int             `json:"page" form:"page"`
	PerPage      int             `json:"per_page" form:"per_page"`
	Status       ProblemStatus   `json:"status" form:"status"`
	Priority     ProblemPriority `json:"priority" form:"priority"`
	Category     string          `json:"category" form:"category"`
	AssignedToID uint            `json:"assigned_to_id" form:"assigned_to_id"`
	ReportedByID uint            `json:"reported_by_id" form:"reported_by_id"`
	IsKnownError *bool           `json:"is_known_error" form:"is_known_error"`
	Search       string          `json:"search" form:"search"`
	SortBy       string          `json:"sort_by" form:"sort_by"`
	SortOrder    string          `json:"sort_order" form:"sort_order"`
	FromDate     *time.Time      `json:"from_date" form:"from_date"`
	ToDate       *time.Time      `json:"to_date" form:"to_date"`
}

// ProblemListResponse represents a response containing a list of problems.
type ProblemListResponse struct {
	Problems   []*Problem `json:"problems"`
	Total      int64      `json:"total"`
	Page       int        `json:"page"`
	PerPage    int        `json:"per_page"`
	TotalPages int        `json:"total_pages"`
}
