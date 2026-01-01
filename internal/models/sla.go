package models

import (
	"time"
)

// SLA represents a Service Level Agreement.
type SLA struct {
	ID                uint       `json:"id"`
	Name              string     `json:"name" binding:"required"`
	Description       string     `json:"description"`
	FirstResponseTime int        `json:"first_response_time"` // minutes
	UpdateTime        int        `json:"update_time"`         // minutes
	SolutionTime      int        `json:"solution_time"`       // minutes
	CalendarName      string     `json:"calendar_name"`
	ValidFrom         time.Time  `json:"valid_from"`
	ValidTo           *time.Time `json:"valid_to,omitempty"`
	IsActive          bool       `json:"is_active"`
	Priority          int        `json:"priority"` // 1-5, higher priority SLAs take precedence
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`

	// Conditions for SLA application
	Conditions SLAConditions `json:"conditions"`

	// Escalation rules
	EscalationRules []SLAEscalationRule `json:"escalation_rules,omitempty"`
}

// SLAConditions defines when an SLA applies.
type SLAConditions struct {
	Queues         []uint   `json:"queues,omitempty"`
	Priorities     []int    `json:"priorities,omitempty"`
	Types          []string `json:"types,omitempty"`
	CustomerGroups []uint   `json:"customer_groups,omitempty"`
	Services       []uint   `json:"services,omitempty"`
	Tags           []string `json:"tags,omitempty"`
}

// SLAEscalationRule defines escalation actions when SLA is breached.
type SLAEscalationRule struct {
	ID             uint     `json:"id"`
	SLAID          uint     `json:"sla_id"`
	Name           string   `json:"name"`
	TriggerPercent int      `json:"trigger_percent"` // Percentage of time elapsed (e.g., 80%)
	NotifyAgents   []uint   `json:"notify_agents,omitempty"`
	NotifyManagers bool     `json:"notify_managers"`
	AutoAssignTo   *uint    `json:"auto_assign_to,omitempty"`
	ChangePriority *int     `json:"change_priority,omitempty"`
	AddTags        []string `json:"add_tags,omitempty"`
	EmailTemplate  string   `json:"email_template,omitempty"`
	IsActive       bool     `json:"is_active"`
}

// BusinessCalendar defines working hours and holidays (SLA data model).
type BusinessCalendar struct {
	ID           uint           `json:"id"`
	Name         string         `json:"name" binding:"required"`
	Description  string         `json:"description"`
	TimeZone     string         `json:"time_zone" binding:"required"`
	WorkingHours []WorkingHours `json:"working_hours"`
	Holidays     []SLAHoliday   `json:"holidays,omitempty"`
	IsDefault    bool           `json:"is_default"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// WorkingHours defines working hours for each day.
type WorkingHours struct {
	DayOfWeek    int    `json:"day_of_week"` // 0=Sunday, 6=Saturday
	StartTime    string `json:"start_time"`  // HH:MM format
	EndTime      string `json:"end_time"`    // HH:MM format
	IsWorkingDay bool   `json:"is_working_day"`
}

// SLAHoliday represents a non-working day.
type SLAHoliday struct {
	ID          uint      `json:"id"`
	CalendarID  uint      `json:"calendar_id"`
	Name        string    `json:"name"`
	Date        time.Time `json:"date"`
	IsRecurring bool      `json:"is_recurring"`
}

// TicketSLA tracks SLA compliance for a ticket.
type TicketSLA struct {
	ID                 uint       `json:"id"`
	TicketID           uint       `json:"ticket_id"`
	SLAID              uint       `json:"sla_id"`
	FirstResponseDue   *time.Time `json:"first_response_due,omitempty"`
	FirstResponseAt    *time.Time `json:"first_response_at,omitempty"`
	NextUpdateDue      *time.Time `json:"next_update_due,omitempty"`
	LastUpdateAt       *time.Time `json:"last_update_at,omitempty"`
	SolutionDue        *time.Time `json:"solution_due,omitempty"`
	SolutionAt         *time.Time `json:"solution_at,omitempty"`
	Status             string     `json:"status"` // "pending", "in_progress", "breached", "met"
	BreachTime         *time.Time `json:"breach_time,omitempty"`
	EscalationLevel    int        `json:"escalation_level"`
	LastEscalationAt   *time.Time `json:"last_escalation_at,omitempty"`
	PausedAt           *time.Time `json:"paused_at,omitempty"`
	TotalPausedMinutes int        `json:"total_paused_minutes"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// SLAMetrics provides SLA performance metrics.
type SLAMetrics struct {
	SLAID              uint    `json:"sla_id"`
	SLAName            string  `json:"sla_name"`
	Period             string  `json:"period"`
	TotalTickets       int     `json:"total_tickets"`
	MetCount           int     `json:"met_count"`
	BreachedCount      int     `json:"breached_count"`
	PendingCount       int     `json:"pending_count"`
	CompliancePercent  float64 `json:"compliance_percent"`
	AvgResponseTime    int     `json:"avg_response_time_minutes"`
	AvgSolutionTime    int     `json:"avg_solution_time_minutes"`
	WorstBreachMinutes int     `json:"worst_breach_minutes"`
	TotalEscalations   int     `json:"total_escalations"`
}

// SLAReport represents a comprehensive SLA report.
type SLAReport struct {
	StartDate         time.Time      `json:"start_date"`
	EndDate           time.Time      `json:"end_date"`
	OverallCompliance float64        `json:"overall_compliance"`
	TotalTickets      int            `json:"total_tickets"`
	TotalBreaches     int            `json:"total_breaches"`
	Metrics           []SLAMetrics   `json:"metrics"`
	TrendData         []SLATrend     `json:"trend_data"`
	TopBreachReasons  []BreachReason `json:"top_breach_reasons"`
}

// SLATrend represents SLA compliance trend over time.
type SLATrend struct {
	Date              time.Time `json:"date"`
	CompliancePercent float64   `json:"compliance_percent"`
	TicketCount       int       `json:"ticket_count"`
	BreachCount       int       `json:"breach_count"`
}

// BreachReason represents reasons for SLA breaches.
type BreachReason struct {
	Reason  string  `json:"reason"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

// SLAEscalationHistory tracks escalation events.
type SLAEscalationHistory struct {
	ID               uint      `json:"id"`
	TicketSLAID      uint      `json:"ticket_sla_id"`
	EscalationRuleID uint      `json:"escalation_rule_id"`
	EscalatedAt      time.Time `json:"escalated_at"`
	NotifiedUsers    []uint    `json:"notified_users"`
	Actions          string    `json:"actions"` // JSON of actions taken
	Success          bool      `json:"success"`
	ErrorMessage     string    `json:"error_message,omitempty"`
}

// SLAPauseReason represents why an SLA was paused.
type SLAPauseReason struct {
	ID          uint       `json:"id"`
	TicketSLAID uint       `json:"ticket_sla_id"`
	Reason      string     `json:"reason" binding:"required"`
	PausedBy    uint       `json:"paused_by"`
	PausedAt    time.Time  `json:"paused_at"`
	ResumedAt   *time.Time `json:"resumed_at,omitempty"`
	Duration    int        `json:"duration_minutes"`
}

// SLAConfiguration represents global SLA settings.
type SLAConfiguration struct {
	EnableSLA               bool   `json:"enable_sla"`
	DefaultCalendarID       uint   `json:"default_calendar_id"`
	AutoAssignSLA           bool   `json:"auto_assign_sla"`
	NotifyOnBreach          bool   `json:"notify_on_breach"`
	NotifyBeforeBreach      bool   `json:"notify_before_breach"`
	NotifyMinutesBefore     int    `json:"notify_minutes_before"`
	PauseOnCustomerReply    bool   `json:"pause_on_customer_reply"`
	PauseOnPendingStatus    bool   `json:"pause_on_pending_status"`
	ExcludeWeekends         bool   `json:"exclude_weekends"`
	ExcludeHolidays         bool   `json:"exclude_holidays"`
	BreachNotificationEmail string `json:"breach_notification_email"`
}
