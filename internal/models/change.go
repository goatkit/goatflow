package models

import (
	"time"
)

// ChangeType represents the type of change.
type ChangeType string

const (
	ChangeTypeStandard  ChangeType = "standard"  // Pre-approved, low risk
	ChangeTypeNormal    ChangeType = "normal"    // Requires CAB approval
	ChangeTypeEmergency ChangeType = "emergency" // Urgent, expedited approval
	ChangeTypeMajor     ChangeType = "major"     // Significant impact
)

// ChangeStatus represents the current status of a change.
type ChangeStatus string

const (
	ChangeStatusDraft            ChangeStatus = "draft"
	ChangeStatusSubmitted        ChangeStatus = "submitted"
	ChangeStatusPlanning         ChangeStatus = "planning"
	ChangeStatusAwaitingApproval ChangeStatus = "awaiting_approval"
	ChangeStatusApproved         ChangeStatus = "approved"
	ChangeStatusScheduled        ChangeStatus = "scheduled"
	ChangeStatusImplementing     ChangeStatus = "implementing"
	ChangeStatusReview           ChangeStatus = "review"
	ChangeStatusClosed           ChangeStatus = "closed"
	ChangeStatusCancelled        ChangeStatus = "cancelled"
	ChangeStatusRejected         ChangeStatus = "rejected"
)

// ChangeRisk represents the risk level of a change.
type ChangeRisk string

const (
	ChangeRiskLow      ChangeRisk = "low"
	ChangeRiskMedium   ChangeRisk = "medium"
	ChangeRiskHigh     ChangeRisk = "high"
	ChangeRiskCritical ChangeRisk = "critical"
)

// ChangeImpact represents the impact level of a change.
type ChangeImpact string

const (
	ChangeImpactMinor       ChangeImpact = "minor"
	ChangeImpactSignificant ChangeImpact = "significant"
	ChangeImpactMajor       ChangeImpact = "major"
	ChangeImpactExtensive   ChangeImpact = "extensive"
)

// Change represents an ITSM change request.
type Change struct {
	ID           uint         `json:"id" gorm:"primaryKey"`
	ChangeNumber string       `json:"change_number" gorm:"uniqueIndex;not null"`
	Title        string       `json:"title" gorm:"not null"`
	Description  string       `json:"description"`
	Type         ChangeType   `json:"type" gorm:"not null;default:'normal'"`
	Status       ChangeStatus `json:"status" gorm:"not null;default:'draft'"`
	Priority     int          `json:"priority" gorm:"default:3"` // 1-5 scale
	Risk         ChangeRisk   `json:"risk" gorm:"not null;default:'medium'"`
	Impact       ChangeImpact `json:"impact" gorm:"not null;default:'minor'"`
	Category     string       `json:"category"`
	SubCategory  string       `json:"sub_category"`

	// Justification and planning
	BusinessJustification string `json:"business_justification"`
	TechnicalDetails      string `json:"technical_details"`
	ImplementationPlan    string `json:"implementation_plan"`
	TestPlan              string `json:"test_plan"`
	BackoutPlan           string `json:"backout_plan"`
	CommunicationPlan     string `json:"communication_plan"`

	// Risk assessment
	RiskAssessment string `json:"risk_assessment"`
	RiskMitigation string `json:"risk_mitigation"`
	ImpactAnalysis string `json:"impact_analysis"`

	// Assignment and ownership
	RequestedByID     uint   `json:"requested_by_id" gorm:"not null"`
	RequestedBy       *User  `json:"requested_by,omitempty" gorm:"foreignKey:RequestedByID"`
	AssignedToID      *uint  `json:"assigned_to_id"`
	AssignedTo        *User  `json:"assigned_to,omitempty" gorm:"foreignKey:AssignedToID"`
	AssignmentGroupID *uint  `json:"assignment_group_id"`
	AssignmentGroup   *Group `json:"assignment_group,omitempty" gorm:"foreignKey:AssignmentGroupID"`
	ChangeManagerID   *uint  `json:"change_manager_id"`
	ChangeManager     *User  `json:"change_manager,omitempty" gorm:"foreignKey:ChangeManagerID"`

	// Related entities
	RelatedIncidentID *uint     `json:"related_incident_id"`
	RelatedIncident   *Incident `json:"related_incident,omitempty" gorm:"foreignKey:RelatedIncidentID"`
	RelatedProblemID  *uint     `json:"related_problem_id"`
	RelatedProblem    *Problem  `json:"related_problem,omitempty" gorm:"foreignKey:RelatedProblemID"`

	// Configuration items (CMDB)
	ConfigurationItems []ConfigurationItem `json:"configuration_items,omitempty" gorm:"many2many:change_configuration_items;"`

	// Service information
	ServiceID        *uint     `json:"service_id"`
	Service          *Service  `json:"service,omitempty" gorm:"foreignKey:ServiceID"`
	AffectedServices []Service `json:"affected_services,omitempty" gorm:"many2many:change_affected_services;"`

	// Scheduling
	PlannedStartDate  *time.Time `json:"planned_start_date"`
	PlannedEndDate    *time.Time `json:"planned_end_date"`
	ActualStartDate   *time.Time `json:"actual_start_date"`
	ActualEndDate     *time.Time `json:"actual_end_date"`
	MaintenanceWindow string     `json:"maintenance_window"`
	EstimatedDowntime int        `json:"estimated_downtime"` // Minutes
	ActualDowntime    int        `json:"actual_downtime"`    // Minutes

	// Approval process
	RequiresCABApproval bool             `json:"requires_cab_approval" gorm:"default:true"`
	CABMeetingDate      *time.Time       `json:"cab_meeting_date"`
	CABDecision         string           `json:"cab_decision"`
	CABNotes            string           `json:"cab_notes"`
	Approvals           []ChangeApproval `json:"approvals,omitempty" gorm:"foreignKey:ChangeID"`

	// Implementation details
	ImplementedByID      *uint  `json:"implemented_by_id"`
	ImplementedBy        *User  `json:"implemented_by,omitempty" gorm:"foreignKey:ImplementedByID"`
	ImplementationNotes  string `json:"implementation_notes"`
	ImplementationResult string `json:"implementation_result"`

	// Review and closure
	ReviewNotes        string `json:"review_notes"`
	LessonsLearned     string `json:"lessons_learned"`
	SuccessCriteriaMet bool   `json:"success_criteria_met" gorm:"default:false"`
	ClosureCode        string `json:"closure_code"`
	ClosureNotes       string `json:"closure_notes"`

	// Metrics
	EstimatedCost     float64 `json:"estimated_cost"`
	ActualCost        float64 `json:"actual_cost"`
	EstimatedEffort   int     `json:"estimated_effort"` // Hours
	ActualEffort      int     `json:"actual_effort"`    // Hours
	RollbackRequired  bool    `json:"rollback_required" gorm:"default:false"`
	RollbackPerformed bool    `json:"rollback_performed" gorm:"default:false"`

	// Additional fields
	IsEmergency       bool     `json:"is_emergency" gorm:"default:false"`
	IsStandardChange  bool     `json:"is_standard_change" gorm:"default:false"`
	TemplateID        *uint    `json:"template_id"`
	Tags              []string `json:"tags" gorm:"type:text[]"`
	ExternalReference string   `json:"external_reference"`

	// Audit fields
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	CreatedByID      uint      `json:"created_by_id"`
	CreatedBy        *User     `json:"created_by,omitempty" gorm:"foreignKey:CreatedByID"`
	LastModifiedByID *uint     `json:"last_modified_by_id"`
	LastModifiedBy   *User     `json:"last_modified_by,omitempty" gorm:"foreignKey:LastModifiedByID"`
}

// ChangeApproval represents an approval for a change.
type ChangeApproval struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	ChangeID     uint       `json:"change_id" gorm:"not null"`
	Change       *Change    `json:"change,omitempty" gorm:"foreignKey:ChangeID"`
	ApproverID   uint       `json:"approver_id" gorm:"not null"`
	Approver     *User      `json:"approver,omitempty" gorm:"foreignKey:ApproverID"`
	ApprovalType string     `json:"approval_type"` // technical, business, security, etc.
	Status       string     `json:"status"`        // pending, approved, rejected, abstained
	Comments     string     `json:"comments"`
	ApprovedAt   *time.Time `json:"approved_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// ChangeComment represents a comment on a change.
type ChangeComment struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	ChangeID   uint      `json:"change_id" gorm:"not null"`
	Change     *Change   `json:"change,omitempty" gorm:"foreignKey:ChangeID"`
	AuthorID   uint      `json:"author_id" gorm:"not null"`
	Author     *User     `json:"author,omitempty" gorm:"foreignKey:AuthorID"`
	Comment    string    `json:"comment" gorm:"not null"`
	IsPublic   bool      `json:"is_public" gorm:"default:true"`
	IsWorkNote bool      `json:"is_work_note" gorm:"default:false"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ChangeAttachment represents a file attached to a change.
type ChangeAttachment struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	ChangeID     uint      `json:"change_id" gorm:"not null"`
	Change       *Change   `json:"change,omitempty" gorm:"foreignKey:ChangeID"`
	FileName     string    `json:"file_name" gorm:"not null"`
	FilePath     string    `json:"file_path" gorm:"not null"`
	FileSize     int64     `json:"file_size"`
	ContentType  string    `json:"content_type"`
	UploadedByID uint      `json:"uploaded_by_id" gorm:"not null"`
	UploadedBy   *User     `json:"uploaded_by,omitempty" gorm:"foreignKey:UploadedByID"`
	CreatedAt    time.Time `json:"created_at"`
}

// ChangeHistory tracks changes to a change request.
type ChangeHistory struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	ChangeID     uint      `json:"change_id" gorm:"not null"`
	Change       *Change   `json:"change,omitempty" gorm:"foreignKey:ChangeID"`
	FieldName    string    `json:"field_name" gorm:"not null"`
	OldValue     string    `json:"old_value"`
	NewValue     string    `json:"new_value"`
	ChangedByID  uint      `json:"changed_by_id" gorm:"not null"`
	ChangedBy    *User     `json:"changed_by,omitempty" gorm:"foreignKey:ChangedByID"`
	ChangeReason string    `json:"change_reason"`
	CreatedAt    time.Time `json:"created_at"`
}

// ChangeTask represents a task within a change.
type ChangeTask struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	ChangeID     uint       `json:"change_id" gorm:"not null"`
	Change       *Change    `json:"change,omitempty" gorm:"foreignKey:ChangeID"`
	TaskNumber   string     `json:"task_number" gorm:"not null"`
	Title        string     `json:"title" gorm:"not null"`
	Description  string     `json:"description"`
	AssignedToID *uint      `json:"assigned_to_id"`
	AssignedTo   *User      `json:"assigned_to,omitempty" gorm:"foreignKey:AssignedToID"`
	Status       string     `json:"status"` // pending, in_progress, completed, cancelled
	PlannedStart *time.Time `json:"planned_start"`
	PlannedEnd   *time.Time `json:"planned_end"`
	ActualStart  *time.Time `json:"actual_start"`
	ActualEnd    *time.Time `json:"actual_end"`
	Dependencies string     `json:"dependencies"`
	Order        int        `json:"order"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// ChangeListRequest represents a request to list changes.
type ChangeListRequest struct {
	Page          int          `json:"page" form:"page"`
	PerPage       int          `json:"per_page" form:"per_page"`
	Status        ChangeStatus `json:"status" form:"status"`
	Type          ChangeType   `json:"type" form:"type"`
	Risk          ChangeRisk   `json:"risk" form:"risk"`
	RequestedByID uint         `json:"requested_by_id" form:"requested_by_id"`
	AssignedToID  uint         `json:"assigned_to_id" form:"assigned_to_id"`
	Search        string       `json:"search" form:"search"`
	SortBy        string       `json:"sort_by" form:"sort_by"`
	SortOrder     string       `json:"sort_order" form:"sort_order"`
	FromDate      *time.Time   `json:"from_date" form:"from_date"`
	ToDate        *time.Time   `json:"to_date" form:"to_date"`
	RequiresCAB   *bool        `json:"requires_cab" form:"requires_cab"`
	IsEmergency   *bool        `json:"is_emergency" form:"is_emergency"`
}

// ChangeListResponse represents a response containing a list of changes.
type ChangeListResponse struct {
	Changes    []*Change `json:"changes"`
	Total      int64     `json:"total"`
	Page       int       `json:"page"`
	PerPage    int       `json:"per_page"`
	TotalPages int       `json:"total_pages"`
}
