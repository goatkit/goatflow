package models

import (
	"encoding/json"
	"time"
)

// WorkflowStatus represents the status of a workflow
type WorkflowStatus string

const (
	WorkflowStatusDraft    WorkflowStatus = "draft"
	WorkflowStatusActive   WorkflowStatus = "active"
	WorkflowStatusInactive WorkflowStatus = "inactive"
	WorkflowStatusArchived WorkflowStatus = "archived"
)

// TriggerType represents the type of trigger for a workflow
type TriggerType string

const (
	TriggerTypeTicketCreated  TriggerType = "ticket_created"
	TriggerTypeTicketUpdated  TriggerType = "ticket_updated"
	TriggerTypeTicketAssigned TriggerType = "ticket_assigned"
	TriggerTypeStatusChanged  TriggerType = "status_changed"
	TriggerTypePriorityChanged TriggerType = "priority_changed"
	TriggerTypeCustomerReply  TriggerType = "customer_reply"
	TriggerTypeAgentReply     TriggerType = "agent_reply"
	TriggerTypeTimeBasedSLA   TriggerType = "time_based_sla"
	TriggerTypeScheduled      TriggerType = "scheduled"
	TriggerTypeWebhook        TriggerType = "webhook"
)

// ActionType represents the type of action in a workflow
type ActionType string

const (
	ActionTypeAssignTicket      ActionType = "assign_ticket"
	ActionTypeChangeStatus      ActionType = "change_status"
	ActionTypeChangePriority    ActionType = "change_priority"
	ActionTypeSendEmail         ActionType = "send_email"
	ActionTypeSendNotification  ActionType = "send_notification"
	ActionTypeAddTag            ActionType = "add_tag"
	ActionTypeRemoveTag         ActionType = "remove_tag"
	ActionTypeAddNote           ActionType = "add_note"
	ActionTypeEscalate          ActionType = "escalate"
	ActionTypeRunScript         ActionType = "run_script"
	ActionTypeWebhookCall       ActionType = "webhook_call"
	ActionTypeCreateTicket      ActionType = "create_ticket"
	ActionTypeMergeTicket       ActionType = "merge_ticket"
	ActionTypeSetSLA            ActionType = "set_sla"
	ActionTypeUpdateCustomField ActionType = "update_custom_field"
)

// ConditionOperator represents logical operators for conditions
type ConditionOperator string

const (
	OperatorEquals           ConditionOperator = "equals"
	OperatorNotEquals        ConditionOperator = "not_equals"
	OperatorContains         ConditionOperator = "contains"
	OperatorNotContains      ConditionOperator = "not_contains"
	OperatorStartsWith       ConditionOperator = "starts_with"
	OperatorEndsWith         ConditionOperator = "ends_with"
	OperatorGreaterThan      ConditionOperator = "greater_than"
	OperatorLessThan         ConditionOperator = "less_than"
	OperatorGreaterOrEqual   ConditionOperator = "greater_or_equal"
	OperatorLessOrEqual      ConditionOperator = "less_or_equal"
	OperatorIn               ConditionOperator = "in"
	OperatorNotIn            ConditionOperator = "not_in"
	OperatorIsEmpty          ConditionOperator = "is_empty"
	OperatorIsNotEmpty       ConditionOperator = "is_not_empty"
	OperatorMatchesRegex     ConditionOperator = "matches_regex"
)

// Workflow represents an automation workflow
type Workflow struct {
	ID          int            `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Status      WorkflowStatus `json:"status"`
	Priority    int            `json:"priority"` // Higher priority workflows execute first
	Triggers    []Trigger      `json:"triggers"`
	Conditions  []Condition    `json:"conditions"`
	Actions     []Action       `json:"actions"`
	CreatedBy   int            `json:"created_by"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	LastRunAt   *time.Time     `json:"last_run_at"`
	RunCount    int            `json:"run_count"`
	ErrorCount  int            `json:"error_count"`
	IsSystem    bool           `json:"is_system"` // System workflows cannot be deleted
	Tags        []string       `json:"tags"`
}

// Trigger represents a workflow trigger
type Trigger struct {
	ID         int             `json:"id"`
	WorkflowID int             `json:"workflow_id"`
	Type       TriggerType     `json:"type"`
	Config     json.RawMessage `json:"config"` // Trigger-specific configuration
	IsActive   bool            `json:"is_active"`
}

// Condition represents a workflow condition
type Condition struct {
	ID         int               `json:"id"`
	WorkflowID int               `json:"workflow_id"`
	Field      string            `json:"field"`      // Field to check (e.g., "ticket.priority")
	Operator   ConditionOperator `json:"operator"`
	Value      interface{}       `json:"value"`      // Expected value
	LogicalOp  string            `json:"logical_op"` // AND, OR for combining conditions
	GroupID    int               `json:"group_id"`   // For grouping conditions
}

// Action represents a workflow action
type Action struct {
	ID            int             `json:"id"`
	WorkflowID    int             `json:"workflow_id"`
	Type          ActionType      `json:"type"`
	Config        json.RawMessage `json:"config"` // Action-specific configuration
	Order         int             `json:"order"`  // Execution order
	ContinueOnErr bool            `json:"continue_on_error"`
	DelaySeconds  int             `json:"delay_seconds"` // Delay before executing
}

// WorkflowExecution represents a single execution of a workflow
type WorkflowExecution struct {
	ID           int                      `json:"id"`
	WorkflowID   int                      `json:"workflow_id"`
	WorkflowName string                   `json:"workflow_name"`
	TicketID     int                      `json:"ticket_id"`
	TriggerType  TriggerType              `json:"trigger_type"`
	StartedAt    time.Time                `json:"started_at"`
	CompletedAt  *time.Time               `json:"completed_at"`
	Status       string                   `json:"status"` // success, failed, partial
	ActionsRun   int                      `json:"actions_run"`
	ActionsFailed int                     `json:"actions_failed"`
	ExecutionLog []WorkflowExecutionEntry `json:"execution_log"`
	ErrorMessage string                   `json:"error_message,omitempty"`
}

// WorkflowExecutionEntry represents a single step in workflow execution
type WorkflowExecutionEntry struct {
	Timestamp   time.Time   `json:"timestamp"`
	ActionType  ActionType  `json:"action_type"`
	ActionID    int         `json:"action_id"`
	Status      string      `json:"status"` // started, completed, failed
	Message     string      `json:"message"`
	Error       string      `json:"error,omitempty"`
	Duration    int64       `json:"duration_ms"`
	RetryCount  int         `json:"retry_count"`
}

// WorkflowSchedule represents a scheduled workflow
type WorkflowSchedule struct {
	ID           int       `json:"id"`
	WorkflowID   int       `json:"workflow_id"`
	CronExpr     string    `json:"cron_expression"` // Cron expression for scheduling
	Timezone     string    `json:"timezone"`
	IsActive     bool      `json:"is_active"`
	LastRunAt    *time.Time `json:"last_run_at"`
	NextRunAt    time.Time  `json:"next_run_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

// WorkflowTemplate represents a pre-built workflow template
type WorkflowTemplate struct {
	ID          int        `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Category    string     `json:"category"` // escalation, notification, assignment, etc.
	Icon        string     `json:"icon"`
	Config      Workflow   `json:"config"`
	IsPublic    bool       `json:"is_public"`
	UsageCount  int        `json:"usage_count"`
	CreatedAt   time.Time  `json:"created_at"`
}

// TriggerConfig represents trigger-specific configuration
type TriggerConfig struct {
	// For time-based triggers
	DelayMinutes     int    `json:"delay_minutes,omitempty"`
	BusinessHoursOnly bool   `json:"business_hours_only,omitempty"`
	
	// For status change triggers
	FromStatus       string `json:"from_status,omitempty"`
	ToStatus         string `json:"to_status,omitempty"`
	
	// For priority change triggers
	FromPriority     string `json:"from_priority,omitempty"`
	ToPriority       string `json:"to_priority,omitempty"`
	
	// For webhook triggers
	WebhookURL       string `json:"webhook_url,omitempty"`
	WebhookSecret    string `json:"webhook_secret,omitempty"`
	
	// For scheduled triggers
	ScheduleExpr     string `json:"schedule_expr,omitempty"`
}

// ActionConfig represents action-specific configuration
type ActionConfig struct {
	// For assignment actions
	AssignToUserID   int    `json:"assign_to_user_id,omitempty"`
	AssignToGroupID  int    `json:"assign_to_group_id,omitempty"`
	AssignmentMethod string `json:"assignment_method,omitempty"` // round_robin, least_loaded, skills_based
	
	// For status/priority changes
	NewStatus        string `json:"new_status,omitempty"`
	NewPriority      string `json:"new_priority,omitempty"`
	
	// For email actions
	EmailTo          []string `json:"email_to,omitempty"`
	EmailCC          []string `json:"email_cc,omitempty"`
	EmailSubject     string   `json:"email_subject,omitempty"`
	EmailBody        string   `json:"email_body,omitempty"`
	EmailTemplateID  int      `json:"email_template_id,omitempty"`
	
	// For notification actions
	NotifyUserIDs    []int  `json:"notify_user_ids,omitempty"`
	NotifyGroupIDs   []int  `json:"notify_group_ids,omitempty"`
	NotificationText string `json:"notification_text,omitempty"`
	
	// For tag actions
	Tags             []string `json:"tags,omitempty"`
	
	// For note actions
	NoteContent      string `json:"note_content,omitempty"`
	NoteIsInternal   bool   `json:"note_is_internal,omitempty"`
	
	// For webhook actions
	WebhookURL       string            `json:"webhook_url,omitempty"`
	WebhookMethod    string            `json:"webhook_method,omitempty"`
	WebhookHeaders   map[string]string `json:"webhook_headers,omitempty"`
	WebhookBody      string            `json:"webhook_body,omitempty"`
	
	// For script actions
	ScriptCommand    string   `json:"script_command,omitempty"`
	ScriptArgs       []string `json:"script_args,omitempty"`
	ScriptTimeout    int      `json:"script_timeout_seconds,omitempty"`
	
	// For custom field updates
	CustomFieldName  string      `json:"custom_field_name,omitempty"`
	CustomFieldValue interface{} `json:"custom_field_value,omitempty"`
}

// WorkflowValidation performs validation on a workflow
func (w *Workflow) Validate() error {
	// Validation logic here
	return nil
}

// IsTriggeredBy checks if workflow should be triggered by given event
func (w *Workflow) IsTriggeredBy(triggerType TriggerType, context map[string]interface{}) bool {
	for _, trigger := range w.Triggers {
		if trigger.Type == triggerType && trigger.IsActive {
			// Check if conditions are met
			if w.EvaluateConditions(context) {
				return true
			}
		}
	}
	return false
}

// EvaluateConditions evaluates all conditions for the workflow
func (w *Workflow) EvaluateConditions(context map[string]interface{}) bool {
	if len(w.Conditions) == 0 {
		return true // No conditions means always execute
	}
	
	// Group conditions and evaluate
	// Implementation would handle AND/OR logic between conditions
	return true // Placeholder
}