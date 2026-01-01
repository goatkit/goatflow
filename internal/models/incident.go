package models

import (
	"time"
)

// IncidentSeverity represents the severity level of an incident.
type IncidentSeverity string

const (
	SeverityCritical IncidentSeverity = "critical" // Service completely down
	SeverityHigh     IncidentSeverity = "high"     // Major impact
	SeverityMedium   IncidentSeverity = "medium"   // Moderate impact
	SeverityLow      IncidentSeverity = "low"      // Minor impact
)

// IncidentStatus represents the current status of an incident.
type IncidentStatus string

const (
	IncidentStatusNew        IncidentStatus = "new"
	IncidentStatusAssigned   IncidentStatus = "assigned"
	IncidentStatusInProgress IncidentStatus = "in_progress"
	IncidentStatusPending    IncidentStatus = "pending"
	IncidentStatusResolved   IncidentStatus = "resolved"
	IncidentStatusClosed     IncidentStatus = "closed"
)

// IncidentCategory represents the category of an incident.
type IncidentCategory string

const (
	CategoryHardware    IncidentCategory = "hardware"
	CategorySoftware    IncidentCategory = "software"
	CategoryNetwork     IncidentCategory = "network"
	CategorySecurity    IncidentCategory = "security"
	CategoryAccess      IncidentCategory = "access"
	CategoryPerformance IncidentCategory = "performance"
	CategoryOther       IncidentCategory = "other"
)

// Incident represents an ITSM incident.
type Incident struct {
	ID             uint             `json:"id" gorm:"primaryKey"`
	IncidentNumber string           `json:"incident_number" gorm:"uniqueIndex;not null"`
	Title          string           `json:"title" gorm:"not null"`
	Description    string           `json:"description"`
	Status         IncidentStatus   `json:"status" gorm:"not null;default:'new'"`
	Severity       IncidentSeverity `json:"severity" gorm:"not null;default:'medium'"`
	Category       IncidentCategory `json:"category" gorm:"not null;default:'other'"`
	SubCategory    string           `json:"sub_category"`

	// Impact and urgency for priority calculation
	Impact   int `json:"impact" gorm:"default:2"`  // 1-5 scale
	Urgency  int `json:"urgency" gorm:"default:2"` // 1-5 scale
	Priority int `json:"priority"`                 // Calculated from impact and urgency

	// Assignment and ownership
	AssignedToID      *uint  `json:"assigned_to_id"`
	AssignedTo        *User  `json:"assigned_to,omitempty" gorm:"foreignKey:AssignedToID"`
	AssignmentGroupID *uint  `json:"assignment_group_id"`
	AssignmentGroup   *Group `json:"assignment_group,omitempty" gorm:"foreignKey:AssignmentGroupID"`

	// Related entities
	ReportedByID   uint  `json:"reported_by_id" gorm:"not null"`
	ReportedBy     *User `json:"reported_by,omitempty" gorm:"foreignKey:ReportedByID"`
	AffectedUserID *uint `json:"affected_user_id"`
	AffectedUser   *User `json:"affected_user,omitempty" gorm:"foreignKey:AffectedUserID"`

	// Related tickets and problems
	RelatedTicketID *uint    `json:"related_ticket_id"`
	RelatedTicket   *Ticket  `json:"related_ticket,omitempty" gorm:"foreignKey:RelatedTicketID"`
	ProblemID       *uint    `json:"problem_id"`
	Problem         *Problem `json:"problem,omitempty" gorm:"foreignKey:ProblemID"`

	// Configuration items (CMDB)
	ConfigurationItemID *uint              `json:"configuration_item_id"`
	ConfigurationItem   *ConfigurationItem `json:"configuration_item,omitempty" gorm:"foreignKey:ConfigurationItemID"`

	// Service information
	ServiceID     *uint    `json:"service_id"`
	Service       *Service `json:"service,omitempty" gorm:"foreignKey:ServiceID"`
	ServiceImpact string   `json:"service_impact"` // Description of service impact

	// Resolution details
	ResolutionCode     string `json:"resolution_code"`
	ResolutionNotes    string `json:"resolution_notes"`
	RootCause          string `json:"root_cause"`
	WorkaroundProvided bool   `json:"workaround_provided" gorm:"default:false"`
	WorkaroundDetails  string `json:"workaround_details"`

	// Time tracking
	DetectedAt   *time.Time `json:"detected_at"`
	ReportedAt   time.Time  `json:"reported_at" gorm:"not null"`
	AssignedAt   *time.Time `json:"assigned_at"`
	InProgressAt *time.Time `json:"in_progress_at"`
	ResolvedAt   *time.Time `json:"resolved_at"`
	ClosedAt     *time.Time `json:"closed_at"`

	// SLA tracking
	ResponseDue   *time.Time `json:"response_due"`
	ResolutionDue *time.Time `json:"resolution_due"`
	ResponseMet   bool       `json:"response_met" gorm:"default:false"`
	ResolutionMet bool       `json:"resolution_met" gorm:"default:false"`

	// Metrics
	TimeToRespond   *int `json:"time_to_respond"` // Minutes
	TimeToResolve   *int `json:"time_to_resolve"` // Minutes
	ReopenCount     int  `json:"reopen_count" gorm:"default:0"`
	EscalationLevel int  `json:"escalation_level" gorm:"default:0"`

	// Additional fields
	IsKnownError      bool     `json:"is_known_error" gorm:"default:false"`
	IsMajorIncident   bool     `json:"is_major_incident" gorm:"default:false"`
	ExternalTicketRef string   `json:"external_ticket_ref"`
	Tags              []string `json:"tags" gorm:"type:text[]"`

	// Audit fields
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	CreatedByID      uint      `json:"created_by_id"`
	CreatedBy        *User     `json:"created_by,omitempty" gorm:"foreignKey:CreatedByID"`
	LastModifiedByID *uint     `json:"last_modified_by_id"`
	LastModifiedBy   *User     `json:"last_modified_by,omitempty" gorm:"foreignKey:LastModifiedByID"`
}

// IncidentComment represents a comment on an incident.
type IncidentComment struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	IncidentID uint      `json:"incident_id" gorm:"not null"`
	Incident   *Incident `json:"incident,omitempty" gorm:"foreignKey:IncidentID"`
	AuthorID   uint      `json:"author_id" gorm:"not null"`
	Author     *User     `json:"author,omitempty" gorm:"foreignKey:AuthorID"`
	Comment    string    `json:"comment" gorm:"not null"`
	IsPublic   bool      `json:"is_public" gorm:"default:true"`
	IsWorkNote bool      `json:"is_work_note" gorm:"default:false"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// IncidentAttachment represents a file attached to an incident.
type IncidentAttachment struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	IncidentID   uint      `json:"incident_id" gorm:"not null"`
	Incident     *Incident `json:"incident,omitempty" gorm:"foreignKey:IncidentID"`
	FileName     string    `json:"file_name" gorm:"not null"`
	FilePath     string    `json:"file_path" gorm:"not null"`
	FileSize     int64     `json:"file_size"`
	ContentType  string    `json:"content_type"`
	UploadedByID uint      `json:"uploaded_by_id" gorm:"not null"`
	UploadedBy   *User     `json:"uploaded_by,omitempty" gorm:"foreignKey:UploadedByID"`
	CreatedAt    time.Time `json:"created_at"`
}

// IncidentHistory tracks changes to an incident.
type IncidentHistory struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	IncidentID   uint      `json:"incident_id" gorm:"not null"`
	Incident     *Incident `json:"incident,omitempty" gorm:"foreignKey:IncidentID"`
	FieldName    string    `json:"field_name" gorm:"not null"`
	OldValue     string    `json:"old_value"`
	NewValue     string    `json:"new_value"`
	ChangedByID  uint      `json:"changed_by_id" gorm:"not null"`
	ChangedBy    *User     `json:"changed_by,omitempty" gorm:"foreignKey:ChangedByID"`
	ChangeReason string    `json:"change_reason"`
	CreatedAt    time.Time `json:"created_at"`
}

// IncidentRelation represents relationships between incidents.
type IncidentRelation struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	IncidentID   uint      `json:"incident_id" gorm:"not null"`
	Incident     *Incident `json:"incident,omitempty" gorm:"foreignKey:IncidentID"`
	RelatedID    uint      `json:"related_id" gorm:"not null"`
	Related      *Incident `json:"related,omitempty" gorm:"foreignKey:RelatedID"`
	RelationType string    `json:"relation_type"` // parent, child, related, duplicate
	CreatedAt    time.Time `json:"created_at"`
}

// IncidentListRequest represents a request to list incidents.
type IncidentListRequest struct {
	Page            int              `json:"page" form:"page"`
	PerPage         int              `json:"per_page" form:"per_page"`
	Status          IncidentStatus   `json:"status" form:"status"`
	Severity        IncidentSeverity `json:"severity" form:"severity"`
	Category        IncidentCategory `json:"category" form:"category"`
	AssignedToID    uint             `json:"assigned_to_id" form:"assigned_to_id"`
	ReportedByID    uint             `json:"reported_by_id" form:"reported_by_id"`
	Search          string           `json:"search" form:"search"`
	SortBy          string           `json:"sort_by" form:"sort_by"`
	SortOrder       string           `json:"sort_order" form:"sort_order"`
	IsMajorIncident *bool            `json:"is_major_incident" form:"is_major_incident"`
	FromDate        *time.Time       `json:"from_date" form:"from_date"`
	ToDate          *time.Time       `json:"to_date" form:"to_date"`
}

// IncidentListResponse represents a response containing a list of incidents.
type IncidentListResponse struct {
	Incidents  []*Incident `json:"incidents"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PerPage    int         `json:"per_page"`
	TotalPages int         `json:"total_pages"`
}

// CalculatePriority calculates priority based on impact and urgency matrix.
func (i *Incident) CalculatePriority() {
	// Priority matrix (ITIL standard)
	// Impact/Urgency: 1=Critical, 2=High, 3=Medium, 4=Low, 5=Planning
	matrix := map[int]map[int]int{
		1: {1: 1, 2: 2, 3: 3, 4: 4, 5: 5},
		2: {1: 2, 2: 3, 3: 4, 4: 5, 5: 5},
		3: {1: 3, 2: 4, 3: 4, 4: 5, 5: 5},
		4: {1: 4, 2: 5, 3: 5, 4: 5, 5: 5},
		5: {1: 5, 2: 5, 3: 5, 4: 5, 5: 5},
	}

	if i.Impact < 1 {
		i.Impact = 2
	}
	if i.Urgency < 1 {
		i.Urgency = 2
	}
	if i.Impact > 5 {
		i.Impact = 5
	}
	if i.Urgency > 5 {
		i.Urgency = 5
	}

	i.Priority = matrix[i.Impact][i.Urgency]
}

// GetTimeToRespond calculates time to respond in minutes.
func (i *Incident) GetTimeToRespond() int {
	if i.AssignedAt == nil || i.ReportedAt.IsZero() {
		return 0
	}
	return int(i.AssignedAt.Sub(i.ReportedAt).Minutes())
}

// GetTimeToResolve calculates time to resolve in minutes.
func (i *Incident) GetTimeToResolve() int {
	if i.ResolvedAt == nil || i.ReportedAt.IsZero() {
		return 0
	}
	return int(i.ResolvedAt.Sub(i.ReportedAt).Minutes())
}
