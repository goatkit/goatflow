package models

import (
	"time"
)

// EscalationLevel represents a level in the escalation hierarchy.
type EscalationLevel int

const (
	EscalationLevel1 EscalationLevel = 1 // Tier 1 Support
	EscalationLevel2 EscalationLevel = 2 // Tier 2 Support
	EscalationLevel3 EscalationLevel = 3 // Tier 3 Support / Engineering
	EscalationLevel4 EscalationLevel = 4 // Management
	EscalationLevel5 EscalationLevel = 5 // Executive
)

// EscalationTrigger represents what triggers an escalation.
type EscalationTrigger string

const (
	EscalationTriggerSLAWarning  EscalationTrigger = "sla_warning"
	EscalationTriggerSLABreach   EscalationTrigger = "sla_breach"
	EscalationTriggerTimeElapsed EscalationTrigger = "time_elapsed"
	EscalationTriggerPriority    EscalationTrigger = "priority"
	EscalationTriggerCustomerVIP EscalationTrigger = "customer_vip"
	EscalationTriggerKeyword     EscalationTrigger = "keyword"
	EscalationTriggerManual      EscalationTrigger = "manual"
	EscalationTriggerNoResponse  EscalationTrigger = "no_response"
	EscalationTriggerReopenCount EscalationTrigger = "reopen_count"
	EscalationTriggerSentiment   EscalationTrigger = "sentiment"
)

// EscalationPolicy defines an escalation policy.
type EscalationPolicy struct {
	ID                int                    `json:"id"`
	Name              string                 `json:"name"`
	Description       string                 `json:"description"`
	Priority          int                    `json:"priority"` // Higher priority policies execute first
	IsActive          bool                   `json:"is_active"`
	ApplyToQueues     []int                  `json:"apply_to_queues"`     // Queue IDs this policy applies to
	ApplyToCategories []string               `json:"apply_to_categories"` // Ticket categories
	Rules             []EscalationRule       `json:"rules"`
	Notifications     EscalationNotification `json:"notifications"`
	CreatedBy         int                    `json:"created_by"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

// EscalationRule defines a single escalation rule.
type EscalationRule struct {
	ID             int                   `json:"id"`
	PolicyID       int                   `json:"policy_id"`
	Level          EscalationLevel       `json:"level"`
	Trigger        EscalationTrigger     `json:"trigger"`
	Conditions     []EscalationCondition `json:"conditions"`
	Actions        []EscalationAction    `json:"actions"`
	TimeThreshold  int                   `json:"time_threshold_minutes"` // For time-based triggers
	BusinessHours  bool                  `json:"business_hours_only"`
	AutoEscalate   bool                  `json:"auto_escalate"`    // Automatically escalate without confirmation
	SkipIfAssigned bool                  `json:"skip_if_assigned"` // Skip if ticket already assigned
	Order          int                   `json:"order"`            // Execution order within policy
}

// EscalationCondition defines conditions for escalation.
type EscalationCondition struct {
	Field    string      `json:"field"`    // e.g., "priority", "customer.tier", "ticket.reopen_count"
	Operator string      `json:"operator"` // e.g., "equals", "greater_than", "contains"
	Value    interface{} `json:"value"`
}

// EscalationAction defines what happens during escalation.
type EscalationAction struct {
	Type   string      `json:"type"` // assign_to_user, assign_to_group, change_priority, notify, etc.
	Config interface{} `json:"config"`
}

// EscalationNotification defines notification settings.
type EscalationNotification struct {
	NotifyCustomer    bool     `json:"notify_customer"`
	NotifyAssignee    bool     `json:"notify_assignee"`
	NotifyManager     bool     `json:"notify_manager"`
	NotifyCustomList  []string `json:"notify_custom_list"` // Email addresses
	CustomerTemplate  int      `json:"customer_template_id"`
	InternalTemplate  int      `json:"internal_template_id"`
	IncludeTicketInfo bool     `json:"include_ticket_info"`
	IncludeHistory    bool     `json:"include_history"`
}

// EscalationPath represents the escalation hierarchy.
type EscalationPath struct {
	ID          int                   `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	QueueID     int                   `json:"queue_id"`
	Levels      []EscalationPathLevel `json:"levels"`
	IsDefault   bool                  `json:"is_default"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
}

// EscalationPathLevel defines a level in the escalation path.
type EscalationPathLevel struct {
	Level            EscalationLevel `json:"level"`
	Name             string          `json:"name"`
	AssignToUserID   *int            `json:"assign_to_user_id,omitempty"`
	AssignToGroupID  *int            `json:"assign_to_group_id,omitempty"`
	AssignmentMethod string          `json:"assignment_method"` // specific_user, group_round_robin, skills_based
	ResponseTime     int             `json:"response_time_minutes"`
	ResolutionTime   int             `json:"resolution_time_minutes"`
	NotifyUsers      []int           `json:"notify_users"`
	NotifyGroups     []int           `json:"notify_groups"`
}

// EscalationHistory records escalation events.
type EscalationHistory struct {
	ID               int               `json:"id"`
	TicketID         int               `json:"ticket_id"`
	PolicyID         int               `json:"policy_id"`
	RuleID           int               `json:"rule_id"`
	FromLevel        EscalationLevel   `json:"from_level"`
	ToLevel          EscalationLevel   `json:"to_level"`
	Trigger          EscalationTrigger `json:"trigger"`
	TriggerDetails   string            `json:"trigger_details"`
	PreviousAssignee *int              `json:"previous_assignee,omitempty"`
	NewAssignee      *int              `json:"new_assignee,omitempty"`
	EscalatedBy      *int              `json:"escalated_by,omitempty"` // User who triggered manual escalation
	EscalatedAt      time.Time         `json:"escalated_at"`
	Notes            string            `json:"notes"`
	AutoEscalated    bool              `json:"auto_escalated"`
}

// EscalationMetrics tracks escalation performance.
type EscalationMetrics struct {
	TicketID            int             `json:"ticket_id"`
	TotalEscalations    int             `json:"total_escalations"`
	CurrentLevel        EscalationLevel `json:"current_level"`
	TimeAtLevel1        time.Duration   `json:"time_at_level1"`
	TimeAtLevel2        time.Duration   `json:"time_at_level2"`
	TimeAtLevel3        time.Duration   `json:"time_at_level3"`
	TimeAtLevel4        time.Duration   `json:"time_at_level4"`
	TimeAtLevel5        time.Duration   `json:"time_at_level5"`
	FirstEscalationTime time.Duration   `json:"first_escalation_time"`
	ResolutionAfterEsc  time.Duration   `json:"resolution_after_escalation"`
	EscalationEffective bool            `json:"escalation_effective"` // Was escalation helpful
}

// EscalationMatrix defines escalation relationships between teams.
type EscalationMatrix struct {
	ID          int                       `json:"id"`
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	Mappings    []EscalationMatrixMapping `json:"mappings"`
	IsActive    bool                      `json:"is_active"`
	CreatedAt   time.Time                 `json:"created_at"`
	UpdatedAt   time.Time                 `json:"updated_at"`
}

// EscalationMatrixMapping defines team escalation relationships.
type EscalationMatrixMapping struct {
	FromTeamID      int    `json:"from_team_id"`
	ToTeamID        int    `json:"to_team_id"`
	EscalationLevel int    `json:"escalation_level"`
	Conditions      string `json:"conditions"` // JSON string of conditions
	AutoApprove     bool   `json:"auto_approve"`
}

// EscalationRequest represents a request for escalation (for approval workflows).
type EscalationRequest struct {
	ID            int             `json:"id"`
	TicketID      int             `json:"ticket_id"`
	RequestedBy   int             `json:"requested_by"`
	RequestedAt   time.Time       `json:"requested_at"`
	TargetLevel   EscalationLevel `json:"target_level"`
	TargetTeamID  *int            `json:"target_team_id,omitempty"`
	TargetUserID  *int            `json:"target_user_id,omitempty"`
	Reason        string          `json:"reason"`
	Priority      string          `json:"priority"`
	Status        string          `json:"status"` // pending, approved, rejected, cancelled
	ApprovedBy    *int            `json:"approved_by,omitempty"`
	ApprovedAt    *time.Time      `json:"approved_at,omitempty"`
	ApprovalNotes string          `json:"approval_notes"`
	ExpiresAt     time.Time       `json:"expires_at"`
}

// Helper methods

// GetLevelName returns the name of an escalation level.
func (el EscalationLevel) GetLevelName() string {
	switch el {
	case EscalationLevel1:
		return "Tier 1 Support"
	case EscalationLevel2:
		return "Tier 2 Support"
	case EscalationLevel3:
		return "Tier 3 Support / Engineering"
	case EscalationLevel4:
		return "Management"
	case EscalationLevel5:
		return "Executive"
	default:
		return "Unknown"
	}
}

// IsHigherThan checks if this level is higher than another.
func (el EscalationLevel) IsHigherThan(other EscalationLevel) bool {
	return el > other
}

// CanEscalateTo checks if escalation to target level is valid.
func (el EscalationLevel) CanEscalateTo(target EscalationLevel) bool {
	return target > el && target <= EscalationLevel5
}

// ShouldTrigger evaluates if escalation should trigger based on conditions.
func (er *EscalationRule) ShouldTrigger(context map[string]interface{}) bool {
	// Evaluate all conditions
	for _, condition := range er.Conditions {
		if !evaluateCondition(condition, context) {
			return false
		}
	}
	return true
}

// evaluateCondition checks if a single condition is met.
func evaluateCondition(condition EscalationCondition, context map[string]interface{}) bool {
	// Get field value from context
	value, exists := context[condition.Field]
	if !exists {
		return false
	}

	// Evaluate based on operator
	switch condition.Operator {
	case "equals":
		return value == condition.Value
	case "not_equals":
		return value != condition.Value
	case "greater_than":
		// Type assertion and comparison would be needed
		return false
	case "less_than":
		// Type assertion and comparison would be needed
		return false
	case "contains":
		// String contains check
		return false
	default:
		return false
	}
}

// GetNextLevel returns the next escalation level.
func (ep *EscalationPath) GetNextLevel(currentLevel EscalationLevel) *EscalationPathLevel {
	for i, level := range ep.Levels {
		if level.Level == currentLevel && i+1 < len(ep.Levels) {
			return &ep.Levels[i+1]
		}
	}
	return nil
}

// GetLevelConfig returns configuration for a specific level.
func (ep *EscalationPath) GetLevelConfig(level EscalationLevel) *EscalationPathLevel {
	for _, l := range ep.Levels {
		if l.Level == level {
			return &l
		}
	}
	return nil
}
