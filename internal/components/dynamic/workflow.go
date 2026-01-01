package dynamic

import (
	// "encoding/json".
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
)

// WorkflowEngine manages automated workflows for dynamic modules.
type WorkflowEngine struct {
	cron      *cron.Cron
	workflows map[string]*Workflow
	handlers  map[string]WorkflowHandler
	db        interface{} // Database connection
}

// Workflow represents an automated workflow configuration.
type Workflow struct {
	ID          string                 `json:"id" yaml:"id"`
	Name        string                 `json:"name" yaml:"name"`
	Module      string                 `json:"module" yaml:"module"`
	Description string                 `json:"description" yaml:"description"`
	Enabled     bool                   `json:"enabled" yaml:"enabled"`
	Triggers    []Trigger              `json:"triggers" yaml:"triggers"`
	Conditions  []Condition            `json:"conditions" yaml:"conditions"`
	Actions     []Action               `json:"actions" yaml:"actions"`
	Metadata    map[string]interface{} `json:"metadata" yaml:"metadata"`
}

// Trigger defines when a workflow should execute.
type Trigger struct {
	Type     string                 `json:"type" yaml:"type"` // schedule, event, webhook, manual
	Config   map[string]interface{} `json:"config" yaml:"config"`
	Schedule string                 `json:"schedule,omitempty" yaml:"schedule,omitempty"` // Cron expression
	Event    string                 `json:"event,omitempty" yaml:"event,omitempty"`       // create, update, delete
}

// Condition defines requirements for workflow execution.
type Condition struct {
	Type     string                 `json:"type" yaml:"type"` // field_value, field_change, time_based, record_count
	Field    string                 `json:"field,omitempty" yaml:"field,omitempty"`
	Operator string                 `json:"operator" yaml:"operator"` // equals, not_equals, contains, greater_than, less_than
	Value    interface{}            `json:"value" yaml:"value"`
	Config   map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
}

// Action defines what happens when a workflow executes.
type Action struct {
	Type   string                 `json:"type" yaml:"type"` // update_field, send_email, call_webhook, create_record, delete_record
	Config map[string]interface{} `json:"config" yaml:"config"`
	Order  int                    `json:"order" yaml:"order"`
}

// WorkflowHandler processes workflow actions.
type WorkflowHandler func(ctx *WorkflowContext, action Action) error

// WorkflowContext provides context for workflow execution.
type WorkflowContext struct {
	Workflow  *Workflow
	Module    string
	Record    map[string]interface{}
	OldRecord map[string]interface{} // For update triggers
	Variables map[string]interface{}
	StartTime time.Time
	Logs      []string
}

// NewWorkflowEngine creates a new workflow engine.
func NewWorkflowEngine(db interface{}) *WorkflowEngine {
	return &WorkflowEngine{
		cron:      cron.New(cron.WithSeconds()),
		workflows: make(map[string]*Workflow),
		handlers:  make(map[string]WorkflowHandler),
		db:        db,
	}
}

// RegisterHandler registers a workflow action handler.
func (we *WorkflowEngine) RegisterHandler(actionType string, handler WorkflowHandler) {
	we.handlers[actionType] = handler
}

// RegisterWorkflow registers a new workflow.
func (we *WorkflowEngine) RegisterWorkflow(workflow *Workflow) error {
	we.workflows[workflow.ID] = workflow

	// Setup scheduled triggers
	for _, trigger := range workflow.Triggers {
		if trigger.Type == "schedule" && trigger.Schedule != "" {
			_, err := we.cron.AddFunc(trigger.Schedule, func() {
				we.ExecuteWorkflow(workflow.ID, nil)
			})
			if err != nil {
				return fmt.Errorf("failed to schedule workflow %s: %v", workflow.ID, err)
			}
		}
	}

	return nil
}

// ExecuteWorkflow executes a workflow.
func (we *WorkflowEngine) ExecuteWorkflow(workflowID string, record map[string]interface{}) error {
	workflow, exists := we.workflows[workflowID]
	if !exists {
		return fmt.Errorf("workflow %s not found", workflowID)
	}

	if !workflow.Enabled {
		return fmt.Errorf("workflow %s is disabled", workflowID)
	}

	ctx := &WorkflowContext{
		Workflow:  workflow,
		Module:    workflow.Module,
		Record:    record,
		Variables: make(map[string]interface{}),
		StartTime: time.Now(),
		Logs:      []string{},
	}

	// Check conditions
	if !we.checkConditions(ctx, workflow.Conditions) {
		ctx.Log("Conditions not met, skipping workflow execution")
		return nil
	}

	// Execute actions in order
	for _, action := range workflow.Actions {
		if handler, exists := we.handlers[action.Type]; exists {
			ctx.Log(fmt.Sprintf("Executing action: %s", action.Type))
			if err := handler(ctx, action); err != nil {
				ctx.Log(fmt.Sprintf("Action %s failed: %v", action.Type, err))
				return err
			}
		} else {
			ctx.Log(fmt.Sprintf("No handler for action type: %s", action.Type))
		}
	}

	ctx.Log(fmt.Sprintf("Workflow completed in %v", time.Since(ctx.StartTime)))
	return nil
}

// checkConditions evaluates all conditions for a workflow.
func (we *WorkflowEngine) checkConditions(ctx *WorkflowContext, conditions []Condition) bool {
	for _, condition := range conditions {
		if !we.evaluateCondition(ctx, condition) {
			ctx.Log(fmt.Sprintf("Condition failed: %+v", condition))
			return false
		}
	}
	return true
}

// evaluateCondition evaluates a single condition.
func (we *WorkflowEngine) evaluateCondition(ctx *WorkflowContext, condition Condition) bool {
	switch condition.Type {
	case "field_value":
		return we.evaluateFieldCondition(ctx, condition)
	case "field_change":
		return we.evaluateFieldChangeCondition(ctx, condition)
	case "time_based":
		return we.evaluateTimeCondition(ctx, condition)
	case "record_count":
		return we.evaluateRecordCountCondition(ctx, condition)
	default:
		return false
	}
}

// evaluateFieldCondition checks field value conditions.
func (we *WorkflowEngine) evaluateFieldCondition(ctx *WorkflowContext, condition Condition) bool {
	if ctx.Record == nil {
		return false
	}

	fieldValue, exists := ctx.Record[condition.Field]
	if !exists {
		return false
	}

	switch condition.Operator {
	case "equals":
		return fmt.Sprintf("%v", fieldValue) == fmt.Sprintf("%v", condition.Value)
	case "not_equals":
		return fmt.Sprintf("%v", fieldValue) != fmt.Sprintf("%v", condition.Value)
	case "contains":
		return strings.Contains(fmt.Sprintf("%v", fieldValue), fmt.Sprintf("%v", condition.Value))
	case "greater_than":
		return compareValues(fieldValue, condition.Value) > 0
	case "less_than":
		return compareValues(fieldValue, condition.Value) < 0
	default:
		return false
	}
}

// evaluateFieldChangeCondition checks if a field has changed.
func (we *WorkflowEngine) evaluateFieldChangeCondition(ctx *WorkflowContext, condition Condition) bool {
	if ctx.Record == nil || ctx.OldRecord == nil {
		return false
	}

	newValue := ctx.Record[condition.Field]
	oldValue := ctx.OldRecord[condition.Field]

	return fmt.Sprintf("%v", newValue) != fmt.Sprintf("%v", oldValue)
}

// evaluateTimeCondition checks time-based conditions.
func (we *WorkflowEngine) evaluateTimeCondition(ctx *WorkflowContext, condition Condition) bool {
	// Implementation for time-based conditions
	// e.g., "created more than 7 days ago", "modified in last hour"
	return true // Simplified for now
}

// evaluateRecordCountCondition checks record count conditions.
func (we *WorkflowEngine) evaluateRecordCountCondition(ctx *WorkflowContext, condition Condition) bool {
	// Implementation for record count conditions
	// e.g., "more than 100 records", "less than 5 active users"
	return true // Simplified for now
}

// compareValues compares two values numerically.
func compareValues(a, b interface{}) int {
	// Simplified numeric comparison
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)

	if aStr == bStr {
		return 0
	} else if aStr > bStr {
		return 1
	}
	return -1
}

// Log adds a log entry to the workflow context.
func (ctx *WorkflowContext) Log(message string) {
	timestamp := time.Now().Format("15:04:05")
	ctx.Logs = append(ctx.Logs, fmt.Sprintf("[%s] %s", timestamp, message))
	log.Printf("[Workflow %s] %s", ctx.Workflow.ID, message)
}

// Start starts the workflow engine.
func (we *WorkflowEngine) Start() {
	we.cron.Start()
	log.Println("Workflow engine started")
}

// Stop stops the workflow engine.
func (we *WorkflowEngine) Stop() {
	we.cron.Stop()
	log.Println("Workflow engine stopped")
}

// Built-in Action Handlers

// UpdateFieldHandler updates a field value.
func UpdateFieldHandler(ctx *WorkflowContext, action Action) error {
	field, _ := action.Config["field"].(string)
	value := action.Config["value"]

	if ctx.Record != nil {
		ctx.Record[field] = value
		ctx.Log(fmt.Sprintf("Updated field %s to %v", field, value))
	}

	// TODO: Persist to database
	return nil
}

// SendEmailHandler sends an email notification.
func SendEmailHandler(ctx *WorkflowContext, action Action) error {
	to, _ := action.Config["to"].(string)
	subject, _ := action.Config["subject"].(string)
	body, _ := action.Config["body"].(string)

	// Replace variables in email template
	body = replaceVariables(body, ctx.Variables)
	_ = body // avoid unused until send implemented

	ctx.Log(fmt.Sprintf("Sending email to %s: %s", to, subject))

	// TODO: Implement actual email sending
	return nil
}

// CallWebhookHandler calls an external webhook.
func CallWebhookHandler(ctx *WorkflowContext, action Action) error {
	url, _ := action.Config["url"].(string)
	method, _ := action.Config["method"].(string)

	if method == "" {
		method = "POST"
	}

	ctx.Log(fmt.Sprintf("Calling webhook: %s %s", method, url))

	// TODO: Implement actual HTTP request
	return nil
}

// CreateRecordHandler creates a new record.
func CreateRecordHandler(ctx *WorkflowContext, action Action) error {
	module, _ := action.Config["module"].(string)
	_, _ = action.Config["fields"].(map[string]interface{})

	ctx.Log(fmt.Sprintf("Creating new record in module %s", module))

	// TODO: Implement record creation
	return nil
}

// DeleteRecordHandler soft deletes a record.
func DeleteRecordHandler(ctx *WorkflowContext, action Action) error {
	recordID, _ := action.Config["record_id"].(string)

	ctx.Log(fmt.Sprintf("Soft deleting record %s", recordID))

	// TODO: Implement soft delete (set valid_id = 2)
	return nil
}

// replaceVariables replaces variables in a string template.
func replaceVariables(template string, variables map[string]interface{}) string {
	result := template
	for key, value := range variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
	}
	return result
}

// HTTP Handler for Workflow Management

// HandleWorkflowList lists all workflows.
func (we *WorkflowEngine) HandleWorkflowList(c *gin.Context) {
	workflows := make([]*Workflow, 0, len(we.workflows))
	for _, w := range we.workflows {
		workflows = append(workflows, w)
	}

	c.JSON(200, gin.H{
		"success":   true,
		"workflows": workflows,
		"total":     len(workflows),
	})
}

// HandleWorkflowCreate creates a new workflow.
func (we *WorkflowEngine) HandleWorkflowCreate(c *gin.Context) {
	var workflow Workflow
	if err := c.ShouldBindJSON(&workflow); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	workflow.ID = fmt.Sprintf("wf_%d", time.Now().Unix())

	if err := we.RegisterWorkflow(&workflow); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, gin.H{
		"success":  true,
		"workflow": workflow,
	})
}

// HandleWorkflowExecute manually executes a workflow.
func (we *WorkflowEngine) HandleWorkflowExecute(c *gin.Context) {
	workflowID := c.Param("id")

	var record map[string]interface{}
	c.ShouldBindJSON(&record)

	if err := we.ExecuteWorkflow(workflowID, record); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"success": true,
		"message": "Workflow executed successfully",
	})
}

// HandleWorkflowToggle enables/disables a workflow.
func (we *WorkflowEngine) HandleWorkflowToggle(c *gin.Context) {
	workflowID := c.Param("id")

	workflow, exists := we.workflows[workflowID]
	if !exists {
		c.JSON(404, gin.H{"error": "Workflow not found"})
		return
	}

	workflow.Enabled = !workflow.Enabled

	c.JSON(200, gin.H{
		"success": true,
		"enabled": workflow.Enabled,
	})
}

// HandleWorkflowDelete deletes a workflow.
func (we *WorkflowEngine) HandleWorkflowDelete(c *gin.Context) {
	workflowID := c.Param("id")

	if _, exists := we.workflows[workflowID]; !exists {
		c.JSON(404, gin.H{"error": "Workflow not found"})
		return
	}

	delete(we.workflows, workflowID)

	c.JSON(200, gin.H{
		"success": true,
		"message": "Workflow deleted successfully",
	})
}
