package graphql

import (
	"encoding/json"
	"time"
)

// Core Types

type User struct {
	ID           string                `json:"id"`
	Email        string                `json:"email"`
	Name         string                `json:"name"`
	Role         *Role                 `json:"role"`
	Groups       []*Group              `json:"groups"`
	Organization *Organization         `json:"organization"`
	Avatar       string                `json:"avatar,omitempty"`
	Locale       string                `json:"locale"`
	Timezone     string                `json:"timezone"`
	IsActive     bool                  `json:"is_active"`
	LastLogin    *time.Time            `json:"last_login,omitempty"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
	Preferences  *UserPreferences      `json:"preferences"`
}

type Ticket struct {
	ID              string           `json:"id"`
	Number          string           `json:"number"`
	Subject         string           `json:"subject"`
	Description     string           `json:"description"`
	Status          TicketStatus     `json:"status"`
	Priority        TicketPriority   `json:"priority"`
	Queue           *Queue           `json:"queue"`
	Assignee        *User            `json:"assignee"`
	Customer        *User            `json:"customer"`
	Organization    *Organization    `json:"organization"`
	Tags            []string         `json:"tags"`
	Watchers        []*User          `json:"watchers"`
	SLA             *SLAStatus       `json:"sla"`
	EscalationLevel int              `json:"escalation_level"`
	MergedInto      *Ticket          `json:"merged_into"`
	SplitFrom       *Ticket          `json:"split_from"`
	RelatedTickets  []*Ticket        `json:"related_tickets"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	ClosedAt        *time.Time       `json:"closed_at,omitempty"`
	ResponseTime    *time.Duration   `json:"response_time,omitempty"`
	ResolutionTime  *time.Duration   `json:"resolution_time,omitempty"`
}

type Queue struct {
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	Description    string           `json:"description,omitempty"`
	Email          string           `json:"email,omitempty"`
	IsActive       bool             `json:"is_active"`
	Agents         []*User          `json:"agents"`
	SLAPolicy      *SLAPolicy       `json:"sla_policy"`
	BusinessHours  *BusinessHours   `json:"business_hours"`
	AutoAssignment bool             `json:"auto_assignment"`
	AutoResponse   *CannedResponse  `json:"auto_response"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	Stats          *QueueStats      `json:"stats"`
}

type Organization struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Domain       string         `json:"domain,omitempty"`
	Logo         string         `json:"logo,omitempty"`
	IsActive     bool           `json:"is_active"`
	Users        []*User        `json:"users"`
	CustomFields []*CustomField `json:"custom_fields"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type Workflow struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Status      WorkflowStatus    `json:"status"`
	Triggers    []*Trigger        `json:"triggers"`
	Conditions  []*Condition      `json:"conditions"`
	Actions     []*Action         `json:"actions"`
	Priority    int               `json:"priority"`
	IsSystem    bool              `json:"is_system"`
	CreatedBy   *User             `json:"created_by"`
	LastRunAt   *time.Time        `json:"last_run_at,omitempty"`
	RunCount    int               `json:"run_count"`
	ErrorCount  int               `json:"error_count"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Analytics   *WorkflowAnalytics `json:"analytics"`
}

type Message struct {
	ID          string         `json:"id"`
	Ticket      *Ticket        `json:"ticket"`
	Author      *User          `json:"author"`
	Content     string         `json:"content"`
	ContentType ContentType    `json:"content_type"`
	IsInternal  bool           `json:"is_internal"`
	Attachments []*Attachment  `json:"attachments"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type Note struct {
	ID         string    `json:"id"`
	Ticket     *Ticket   `json:"ticket"`
	Author     *User     `json:"author"`
	Content    string    `json:"content"`
	IsInternal bool      `json:"is_internal"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Attachment struct {
	ID           string    `json:"id"`
	Filename     string    `json:"filename"`
	ContentType  string    `json:"content_type"`
	Size         int       `json:"size"`
	URL          string    `json:"url"`
	ThumbnailURL string    `json:"thumbnail_url,omitempty"`
	UploadedBy   *User     `json:"uploaded_by"`
	CreatedAt    time.Time `json:"created_at"`
}

// Workflow Types

type Trigger struct {
	ID      string          `json:"id"`
	Type    TriggerType     `json:"type"`
	Config  json.RawMessage `json:"config"`
	Enabled bool            `json:"enabled"`
}

type Condition struct {
	ID       string            `json:"id"`
	Field    string            `json:"field"`
	Operator ConditionOperator `json:"operator"`
	Value    json.RawMessage   `json:"value"`
	Logic    LogicOperator     `json:"logic"`
}

type Action struct {
	ID              string          `json:"id"`
	Type            ActionType      `json:"type"`
	Config          json.RawMessage `json:"config"`
	Order           int             `json:"order"`
	ContinueOnError bool            `json:"continue_on_error"`
	DelaySeconds    int             `json:"delay_seconds,omitempty"`
}

// Supporting Types

type Role struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
}

type Group struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Members   []*User   `json:"members"`
	CreatedAt time.Time `json:"created_at"`
}

type Permission struct {
	ID          string `json:"id"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
	Description string `json:"description,omitempty"`
}

type SLAStatus struct {
	Level           string         `json:"level"`
	TimeRemaining   time.Duration  `json:"time_remaining"`
	IsBreached      bool           `json:"is_breached"`
	ResponseTarget  time.Time      `json:"response_target"`
	ResolutionTarget time.Time     `json:"resolution_target"`
}

type SLAPolicy struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	ResponseTime    time.Duration `json:"response_time"`
	ResolutionTime  time.Duration `json:"resolution_time"`
	BusinessHours   bool          `json:"business_hours"`
}

type BusinessHours struct {
	ID       string              `json:"id"`
	Name     string              `json:"name"`
	Timezone string              `json:"timezone"`
	Hours    map[string][]string `json:"hours"`
	Holidays []time.Time         `json:"holidays"`
}

type CannedResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

type CustomField struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Required bool            `json:"required"`
	Options  []string        `json:"options,omitempty"`
	Value    json.RawMessage `json:"value,omitempty"`
}

type UserPreferences struct {
	Theme               string `json:"theme"`
	Language            string `json:"language"`
	NotificationsEmail  bool   `json:"notifications_email"`
	NotificationsPush   bool   `json:"notifications_push"`
	EmailDigestFrequency string `json:"email_digest_frequency"`
}

type QueueStats struct {
	OpenTickets      int           `json:"open_tickets"`
	PendingTickets   int           `json:"pending_tickets"`
	AverageWaitTime  time.Duration `json:"average_wait_time"`
	AverageHandleTime time.Duration `json:"average_handle_time"`
}

type Notification struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

// Analytics Types

type WorkflowAnalytics struct {
	TotalWorkflows      int                  `json:"total_workflows"`
	ActiveWorkflows     int                  `json:"active_workflows"`
	TotalExecutions     int                  `json:"total_executions"`
	SuccessRate         float64              `json:"success_rate"`
	AverageExecutionTime time.Duration       `json:"average_execution_time"`
	TopWorkflows        []*WorkflowSummary   `json:"top_workflows"`
	ExecutionTrends     []*ExecutionTrend    `json:"execution_trends"`
	ErrorAnalysis       *ErrorAnalysis       `json:"error_analysis"`
	ROIMetrics          *ROIMetrics          `json:"roi_metrics"`
}

type WorkflowSummary struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Executions   int     `json:"executions"`
	SuccessRate  float64 `json:"success_rate"`
}

type ExecutionTrend struct {
	Date       time.Time `json:"date"`
	Executions int       `json:"executions"`
	Successful int       `json:"successful"`
	Failed     int       `json:"failed"`
}

type ErrorAnalysis struct {
	CommonErrors   map[string]int `json:"common_errors"`
	ErrorRate      float64        `json:"error_rate"`
	MostFailedStep string         `json:"most_failed_step"`
}

type ROIMetrics struct {
	TimeSaved      time.Duration `json:"time_saved"`
	TicketsHandled int           `json:"tickets_handled"`
	CostSavings    float64       `json:"cost_savings"`
}

type DashboardStats struct {
	OpenTickets         int               `json:"open_tickets"`
	PendingTickets      int               `json:"pending_tickets"`
	OverdueTickets      int               `json:"overdue_tickets"`
	TodayCreated        int               `json:"today_created"`
	TodayClosed         int               `json:"today_closed"`
	AverageResponseTime time.Duration     `json:"average_response_time"`
	SLACompliance       float64           `json:"sla_compliance"`
	AgentActivity       []*AgentActivity  `json:"agent_activity"`
	RecentTickets       []*Ticket         `json:"recent_tickets"`
}

type AgentActivity struct {
	UserID         string        `json:"user_id"`
	Name           string        `json:"name"`
	TicketsHandled int           `json:"tickets_handled"`
	AverageTime    time.Duration `json:"average_time"`
	Status         string        `json:"status"`
}

// Search Types

type SearchResults struct {
	Tickets       []*Ticket       `json:"tickets"`
	Users         []*User         `json:"users"`
	Organizations []*Organization `json:"organizations"`
	TotalCount    int             `json:"total_count"`
}

// Connection Types (Relay-style pagination)

type TicketConnection struct {
	Edges      []*TicketEdge `json:"edges"`
	PageInfo   *PageInfo     `json:"page_info"`
	TotalCount int           `json:"total_count"`
}

type TicketEdge struct {
	Node   *Ticket `json:"node"`
	Cursor string  `json:"cursor"`
}

type UserConnection struct {
	Edges      []*UserEdge `json:"edges"`
	PageInfo   *PageInfo   `json:"page_info"`
	TotalCount int         `json:"total_count"`
}

type UserEdge struct {
	Node   *User  `json:"node"`
	Cursor string `json:"cursor"`
}

type QueueConnection struct {
	Edges      []*QueueEdge `json:"edges"`
	PageInfo   *PageInfo    `json:"page_info"`
	TotalCount int          `json:"total_count"`
}

type QueueEdge struct {
	Node   *Queue `json:"node"`
	Cursor string `json:"cursor"`
}

type WorkflowConnection struct {
	Edges      []*WorkflowEdge `json:"edges"`
	PageInfo   *PageInfo       `json:"page_info"`
	TotalCount int             `json:"total_count"`
}

type WorkflowEdge struct {
	Node   *Workflow `json:"node"`
	Cursor string    `json:"cursor"`
}

type WorkflowExecutionConnection struct {
	Edges      []*WorkflowExecutionEdge `json:"edges"`
	PageInfo   *PageInfo                `json:"page_info"`
	TotalCount int                      `json:"total_count"`
}

type WorkflowExecutionEdge struct {
	Node   *WorkflowExecution `json:"node"`
	Cursor string             `json:"cursor"`
}

type MessageConnection struct {
	Edges      []*MessageEdge `json:"edges"`
	PageInfo   *PageInfo      `json:"page_info"`
	TotalCount int            `json:"total_count"`
}

type MessageEdge struct {
	Node   *Message `json:"node"`
	Cursor string   `json:"cursor"`
}

type OrganizationConnection struct {
	Edges      []*OrganizationEdge `json:"edges"`
	PageInfo   *PageInfo           `json:"page_info"`
	TotalCount int                 `json:"total_count"`
}

type OrganizationEdge struct {
	Node   *Organization `json:"node"`
	Cursor string        `json:"cursor"`
}

type PageInfo struct {
	HasNextPage     bool   `json:"has_next_page"`
	HasPreviousPage bool   `json:"has_previous_page"`
	StartCursor     string `json:"start_cursor,omitempty"`
	EndCursor       string `json:"end_cursor,omitempty"`
}

// Input Types

type CreateTicketInput struct {
	Subject     string    `json:"subject"`
	Description string    `json:"description"`
	Priority    *TicketPriority `json:"priority,omitempty"`
	QueueID     string    `json:"queue_id"`
	CustomerID  string    `json:"customer_id,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	Attachments []Upload  `json:"attachments,omitempty"`
}

type UpdateTicketInput struct {
	Subject     *string         `json:"subject,omitempty"`
	Description *string         `json:"description,omitempty"`
	Status      *TicketStatus   `json:"status,omitempty"`
	Priority    *TicketPriority `json:"priority,omitempty"`
	QueueID     *string         `json:"queue_id,omitempty"`
	AssigneeID  *string         `json:"assignee_id,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
}

type CreateUserInput struct {
	Email          string   `json:"email"`
	Name           string   `json:"name"`
	Password       string   `json:"password"`
	RoleID         string   `json:"role_id"`
	OrganizationID string   `json:"organization_id,omitempty"`
	GroupIDs       []string `json:"group_ids,omitempty"`
}

type UpdateUserInput struct {
	Name           *string  `json:"name,omitempty"`
	Email          *string  `json:"email,omitempty"`
	RoleID         *string  `json:"role_id,omitempty"`
	OrganizationID *string  `json:"organization_id,omitempty"`
	GroupIDs       []string `json:"group_ids,omitempty"`
	IsActive       *bool    `json:"is_active,omitempty"`
}

type CreateWorkflowInput struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Triggers    []TriggerInput  `json:"triggers"`
	Conditions  []ConditionInput `json:"conditions,omitempty"`
	Actions     []ActionInput   `json:"actions"`
	Priority    int             `json:"priority,omitempty"`
}

type UpdateWorkflowInput struct {
	Name        *string          `json:"name,omitempty"`
	Description *string          `json:"description,omitempty"`
	Triggers    []TriggerInput   `json:"triggers,omitempty"`
	Conditions  []ConditionInput `json:"conditions,omitempty"`
	Actions     []ActionInput    `json:"actions,omitempty"`
	Priority    *int             `json:"priority,omitempty"`
}

type TriggerInput struct {
	Type    TriggerType     `json:"type"`
	Config  json.RawMessage `json:"config"`
	Enabled *bool           `json:"enabled,omitempty"`
}

type ConditionInput struct {
	Field    string            `json:"field"`
	Operator ConditionOperator `json:"operator"`
	Value    json.RawMessage   `json:"value"`
	Logic    *LogicOperator    `json:"logic,omitempty"`
}

type ActionInput struct {
	Type            ActionType      `json:"type"`
	Config          json.RawMessage `json:"config"`
	Order           *int            `json:"order,omitempty"`
	ContinueOnError *bool           `json:"continue_on_error,omitempty"`
	DelaySeconds    *int            `json:"delay_seconds,omitempty"`
}

type CreateMessageInput struct {
	Content     string   `json:"content"`
	IsInternal  bool     `json:"is_internal"`
	Attachments []Upload `json:"attachments,omitempty"`
}

type CreateNoteInput struct {
	Content    string `json:"content"`
	IsInternal bool   `json:"is_internal"`
}

type CreateQueueInput struct {
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`
	Email          string   `json:"email,omitempty"`
	AgentIDs       []string `json:"agent_ids,omitempty"`
	AutoAssignment bool     `json:"auto_assignment"`
}

type UpdateQueueInput struct {
	Name           *string  `json:"name,omitempty"`
	Description    *string  `json:"description,omitempty"`
	Email          *string  `json:"email,omitempty"`
	AgentIDs       []string `json:"agent_ids,omitempty"`
	AutoAssignment *bool    `json:"auto_assignment,omitempty"`
	IsActive       *bool    `json:"is_active,omitempty"`
}

type CreateWebhookInput struct {
	Name     string            `json:"name"`
	URL      string            `json:"url"`
	Events   []string          `json:"events"`
	Headers  map[string]string `json:"headers,omitempty"`
	Secret   string            `json:"secret,omitempty"`
	IsActive bool              `json:"is_active"`
}

type UpdateWebhookInput struct {
	Name     *string           `json:"name,omitempty"`
	URL      *string           `json:"url,omitempty"`
	Events   []string          `json:"events,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Secret   *string           `json:"secret,omitempty"`
	IsActive *bool             `json:"is_active,omitempty"`
}

type SplitTicketItem struct {
	Subject     string   `json:"subject"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
}

// Filter Types

type TicketFilter struct {
	Status         *TicketStatus   `json:"status,omitempty"`
	Priority       *TicketPriority `json:"priority,omitempty"`
	QueueID        *string         `json:"queue_id,omitempty"`
	AssigneeID     *string         `json:"assignee_id,omitempty"`
	CustomerID     *string         `json:"customer_id,omitempty"`
	OrganizationID *string         `json:"organization_id,omitempty"`
	Tags           []string        `json:"tags,omitempty"`
	Search         *string         `json:"search,omitempty"`
	CreatedAfter   *time.Time      `json:"created_after,omitempty"`
	CreatedBefore  *time.Time      `json:"created_before,omitempty"`
}

type UserFilter struct {
	Search   *string `json:"search,omitempty"`
	RoleID   *string `json:"role_id,omitempty"`
	GroupID  *string `json:"group_id,omitempty"`
	IsActive *bool   `json:"is_active,omitempty"`
}

type QueueFilter struct {
	Search   *string `json:"search,omitempty"`
	IsActive *bool   `json:"is_active,omitempty"`
}

type OrganizationFilter struct {
	Search   *string `json:"search,omitempty"`
	IsActive *bool   `json:"is_active,omitempty"`
}

type WorkflowFilter struct {
	Status   *WorkflowStatus `json:"status,omitempty"`
	IsSystem *bool           `json:"is_system,omitempty"`
	Search   *string         `json:"search,omitempty"`
}

type Pagination struct {
	Page      int        `json:"page"`
	Limit     int        `json:"limit"`
	SortBy    string     `json:"sort_by,omitempty"`
	SortOrder SortOrder  `json:"sort_order,omitempty"`
}

// Payload Types

type AuthPayload struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	User         *User  `json:"user"`
}

type WorkflowExecution struct {
	ID           string              `json:"id"`
	Workflow     *Workflow           `json:"workflow"`
	Ticket       *Ticket             `json:"ticket,omitempty"`
	TriggerType  TriggerType         `json:"trigger_type"`
	Status       ExecutionStatus     `json:"status"`
	StartedAt    time.Time           `json:"started_at"`
	CompletedAt  *time.Time          `json:"completed_at,omitempty"`
	Duration     *time.Duration      `json:"duration,omitempty"`
	ActionsRun   int                 `json:"actions_run"`
	ActionsFailed int                `json:"actions_failed"`
	ErrorMessage string              `json:"error_message,omitempty"`
	ExecutionLog []*ExecutionLogEntry `json:"execution_log"`
}

type ExecutionLogEntry struct {
	Timestamp time.Time      `json:"timestamp"`
	Action    string         `json:"action"`
	Status    string         `json:"status"`
	Message   string         `json:"message,omitempty"`
	Duration  *time.Duration `json:"duration,omitempty"`
}

type WorkflowTemplate struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Definition  Workflow `json:"definition"`
}

type WorkflowTestResult struct {
	Success      bool                 `json:"success"`
	ErrorMessage string               `json:"error_message,omitempty"`
	ExecutionLog []*ExecutionLogEntry `json:"execution_log"`
}

type WebhookTestResult struct {
	Success      bool   `json:"success"`
	StatusCode   int    `json:"status_code"`
	Response     string `json:"response"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type IntegrationTestResult struct {
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type Integration struct {
	ID          string           `json:"id"`
	Type        IntegrationType  `json:"type"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Status      IntegrationStatus `json:"status"`
	Enabled     bool             `json:"enabled"`
	Config      json.RawMessage  `json:"config"`
	LastSync    *time.Time       `json:"last_sync,omitempty"`
	Error       string           `json:"error,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

type Webhook struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	URL           string            `json:"url"`
	Events        []string          `json:"events"`
	Headers       map[string]string `json:"headers,omitempty"`
	Secret        string            `json:"secret,omitempty"`
	IsActive      bool              `json:"is_active"`
	FailureCount  int               `json:"failure_count"`
	LastTriggered *time.Time        `json:"last_triggered,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

type WebhookLog struct {
	ID          string        `json:"id"`
	Webhook     *Webhook      `json:"webhook"`
	Event       string        `json:"event"`
	StatusCode  int           `json:"status_code"`
	Success     bool          `json:"success"`
	Error       string        `json:"error,omitempty"`
	Duration    time.Duration `json:"duration"`
	DeliveredAt time.Time     `json:"delivered_at"`
}

type SystemStatus struct {
	Healthy       bool              `json:"healthy"`
	Version       string            `json:"version"`
	Uptime        time.Duration     `json:"uptime"`
	Services      map[string]bool   `json:"services"`
	DatabaseStatus string           `json:"database_status"`
	CacheStatus   string            `json:"cache_status"`
	SearchStatus  string            `json:"search_status"`
}

type SystemAlert struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

type WorkflowEvent struct {
	WorkflowID   string    `json:"workflow_id"`
	WorkflowName string    `json:"workflow_name"`
	Event        string    `json:"event"`
	Timestamp    time.Time `json:"timestamp"`
}

type TicketMetrics struct {
	TotalTickets           int              `json:"total_tickets"`
	OpenTickets            int              `json:"open_tickets"`
	ClosedTickets          int              `json:"closed_tickets"`
	AverageResponseTime    time.Duration    `json:"average_response_time"`
	AverageResolutionTime  time.Duration    `json:"average_resolution_time"`
	SatisfactionScore      float64          `json:"satisfaction_score"`
	TicketsByPriority      []*PriorityCount `json:"tickets_by_priority"`
	TicketsByStatus        []*StatusCount   `json:"tickets_by_status"`
	TrendsOverTime         []*TicketTrend   `json:"trends_over_time"`
}

type PriorityCount struct {
	Priority TicketPriority `json:"priority"`
	Count    int            `json:"count"`
}

type StatusCount struct {
	Status TicketStatus `json:"status"`
	Count  int          `json:"count"`
}

type TicketTrend struct {
	Date    time.Time `json:"date"`
	Created int       `json:"created"`
	Closed  int       `json:"closed"`
	Open    int       `json:"open"`
}

type AgentPerformance struct {
	AgentID               string        `json:"agent_id"`
	Name                  string        `json:"name"`
	TicketsHandled        int           `json:"tickets_handled"`
	TicketsClosed         int           `json:"tickets_closed"`
	AverageResponseTime   time.Duration `json:"average_response_time"`
	AverageResolutionTime time.Duration `json:"average_resolution_time"`
	SatisfactionScore     float64       `json:"satisfaction_score"`
	ProductivityScore     float64       `json:"productivity_score"`
}

// Configuration Input Types

type SlackConfigInput struct {
	WorkspaceID   string         `json:"workspace_id"`
	BotToken      string         `json:"bot_token"`
	AppToken      string         `json:"app_token,omitempty"`
	SigningSecret string         `json:"signing_secret"`
	Channels      []SlackChannel `json:"channels,omitempty"`
}

type SlackChannel struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	QueueID        int    `json:"queue_id"`
	NotifyOnNew    bool   `json:"notify_on_new"`
	NotifyOnUpdate bool   `json:"notify_on_update"`
	NotifyOnClose  bool   `json:"notify_on_close"`
}

type TeamsConfigInput struct {
	TenantID     string            `json:"tenant_id"`
	ClientID     string            `json:"client_id"`
	ClientSecret string            `json:"client_secret"`
	WebhookURLs  map[string]string `json:"webhook_urls,omitempty"`
}

type LDAPConfigInput struct {
	Host           string            `json:"host"`
	Port           int               `json:"port"`
	BaseDN         string            `json:"base_dn"`
	BindDN         string            `json:"bind_dn"`
	BindPassword   string            `json:"bind_password"`
	UserFilter     string            `json:"user_filter"`
	GroupFilter    string            `json:"group_filter,omitempty"`
	UseTLS         bool              `json:"use_tls"`
	StartTLS       bool              `json:"start_tls"`
	AttributeMap   map[string]string `json:"attribute_map,omitempty"`
}

// Enums

type TicketStatus string

const (
	TicketStatusNew      TicketStatus = "NEW"
	TicketStatusOpen     TicketStatus = "OPEN"
	TicketStatusPending  TicketStatus = "PENDING"
	TicketStatusOnHold   TicketStatus = "ON_HOLD"
	TicketStatusResolved TicketStatus = "RESOLVED"
	TicketStatusClosed   TicketStatus = "CLOSED"
)

type TicketPriority string

const (
	TicketPriorityLow      TicketPriority = "LOW"
	TicketPriorityNormal   TicketPriority = "NORMAL"
	TicketPriorityHigh     TicketPriority = "HIGH"
	TicketPriorityUrgent   TicketPriority = "URGENT"
	TicketPriorityCritical TicketPriority = "CRITICAL"
)

type WorkflowStatus string

const (
	WorkflowStatusDraft    WorkflowStatus = "DRAFT"
	WorkflowStatusActive   WorkflowStatus = "ACTIVE"
	WorkflowStatusInactive WorkflowStatus = "INACTIVE"
	WorkflowStatusArchived WorkflowStatus = "ARCHIVED"
)

type TriggerType string

const (
	TriggerTypeTicketCreated   TriggerType = "TICKET_CREATED"
	TriggerTypeTicketUpdated   TriggerType = "TICKET_UPDATED"
	TriggerTypeTicketAssigned  TriggerType = "TICKET_ASSIGNED"
	TriggerTypeStatusChanged   TriggerType = "STATUS_CHANGED"
	TriggerTypePriorityChanged TriggerType = "PRIORITY_CHANGED"
	TriggerTypeCustomerReply   TriggerType = "CUSTOMER_REPLY"
	TriggerTypeAgentReply      TriggerType = "AGENT_REPLY"
	TriggerTypeTimeBased       TriggerType = "TIME_BASED"
	TriggerTypeScheduled       TriggerType = "SCHEDULED"
)

type ActionType string

const (
	ActionTypeAssignTicket      ActionType = "ASSIGN_TICKET"
	ActionTypeChangeStatus      ActionType = "CHANGE_STATUS"
	ActionTypeChangePriority    ActionType = "CHANGE_PRIORITY"
	ActionTypeSendEmail         ActionType = "SEND_EMAIL"
	ActionTypeSendNotification  ActionType = "SEND_NOTIFICATION"
	ActionTypeAddTag            ActionType = "ADD_TAG"
	ActionTypeRemoveTag         ActionType = "REMOVE_TAG"
	ActionTypeAddNote           ActionType = "ADD_NOTE"
	ActionTypeEscalate          ActionType = "ESCALATE"
	ActionTypeWebhookCall       ActionType = "WEBHOOK_CALL"
)

type IntegrationType string

const (
	IntegrationTypeSlack   IntegrationType = "SLACK"
	IntegrationTypeTeams   IntegrationType = "TEAMS"
	IntegrationTypeDiscord IntegrationType = "DISCORD"
	IntegrationTypeGitHub  IntegrationType = "GITHUB"
	IntegrationTypeJira    IntegrationType = "JIRA"
	IntegrationTypeLDAP    IntegrationType = "LDAP"
	IntegrationTypeOAuth   IntegrationType = "OAUTH"
	IntegrationTypeSAML    IntegrationType = "SAML"
)

type IntegrationStatus string

const (
	IntegrationStatusConnected    IntegrationStatus = "CONNECTED"
	IntegrationStatusDisconnected IntegrationStatus = "DISCONNECTED"
	IntegrationStatusError        IntegrationStatus = "ERROR"
	IntegrationStatusPending      IntegrationStatus = "PENDING"
)

type ExecutionStatus string

const (
	ExecutionStatusPending  ExecutionStatus = "PENDING"
	ExecutionStatusRunning  ExecutionStatus = "RUNNING"
	ExecutionStatusSuccess  ExecutionStatus = "SUCCESS"
	ExecutionStatusFailed   ExecutionStatus = "FAILED"
	ExecutionStatusCancelled ExecutionStatus = "CANCELLED"
)

type ConditionOperator string

const (
	ConditionOperatorEquals      ConditionOperator = "EQUALS"
	ConditionOperatorNotEquals   ConditionOperator = "NOT_EQUALS"
	ConditionOperatorContains    ConditionOperator = "CONTAINS"
	ConditionOperatorNotContains ConditionOperator = "NOT_CONTAINS"
	ConditionOperatorGreaterThan ConditionOperator = "GREATER_THAN"
	ConditionOperatorLessThan    ConditionOperator = "LESS_THAN"
	ConditionOperatorIn          ConditionOperator = "IN"
	ConditionOperatorNotIn       ConditionOperator = "NOT_IN"
)

type LogicOperator string

const (
	LogicOperatorAnd LogicOperator = "AND"
	LogicOperatorOr  LogicOperator = "OR"
)

type ContentType string

const (
	ContentTypePlainText ContentType = "PLAIN_TEXT"
	ContentTypeHTML      ContentType = "HTML"
	ContentTypeMarkdown  ContentType = "MARKDOWN"
)

type SearchType string

const (
	SearchTypeAll          SearchType = "ALL"
	SearchTypeTicket       SearchType = "TICKET"
	SearchTypeUser         SearchType = "USER"
	SearchTypeOrganization SearchType = "ORGANIZATION"
)

type SortOrder string

const (
	SortOrderAsc  SortOrder = "ASC"
	SortOrderDesc SortOrder = "DESC"
)

type AnalyticsPeriod string

const (
	AnalyticsPeriodDay     AnalyticsPeriod = "DAY"
	AnalyticsPeriodWeek    AnalyticsPeriod = "WEEK"
	AnalyticsPeriodMonth   AnalyticsPeriod = "MONTH"
	AnalyticsPeriodQuarter AnalyticsPeriod = "QUARTER"
	AnalyticsPeriodYear    AnalyticsPeriod = "YEAR"
)

// Special scalar type for file uploads
type Upload interface{}