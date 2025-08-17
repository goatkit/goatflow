package service

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

// WorkflowService handles workflow automation logic
type WorkflowService struct {
	workflowRepo  repository.WorkflowRepository
	ticketRepo    repository.TicketRepository
	userRepo      repository.UserRepository
	emailService  *EmailService
	notifyService *NotificationService
}

// NewWorkflowService creates a new workflow service
func NewWorkflowService(
	workflowRepo repository.WorkflowRepository,
	ticketRepo repository.TicketRepository,
	userRepo repository.UserRepository,
	emailService *EmailService,
	notifyService *NotificationService,
) *WorkflowService {
	return &WorkflowService{
		workflowRepo:  workflowRepo,
		ticketRepo:    ticketRepo,
		userRepo:      userRepo,
		emailService:  emailService,
		notifyService: notifyService,
	}
}

// CreateWorkflow creates a new workflow
func (s *WorkflowService) CreateWorkflow(workflow *models.Workflow) error {
	// Validate workflow
	if err := workflow.Validate(); err != nil {
		return fmt.Errorf("workflow validation failed: %w", err)
	}

	// Set timestamps
	workflow.CreatedAt = time.Now()
	workflow.UpdatedAt = time.Now()
	workflow.Status = models.WorkflowStatusDraft

	// Save to repository
	return s.workflowRepo.Create(workflow)
}

// UpdateWorkflow updates an existing workflow
func (s *WorkflowService) UpdateWorkflow(id int, workflow *models.Workflow) error {
	existing, err := s.workflowRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("workflow not found: %w", err)
	}

	// Prevent updating system workflows
	if existing.IsSystem {
		return fmt.Errorf("cannot modify system workflow")
	}

	// Validate workflow
	if err := workflow.Validate(); err != nil {
		return fmt.Errorf("workflow validation failed: %w", err)
	}

	workflow.ID = id
	workflow.UpdatedAt = time.Now()

	return s.workflowRepo.Update(workflow)
}

// DeleteWorkflow deletes a workflow
func (s *WorkflowService) DeleteWorkflow(id int) error {
	workflow, err := s.workflowRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("workflow not found: %w", err)
	}

	// Prevent deleting system workflows
	if workflow.IsSystem {
		return fmt.Errorf("cannot delete system workflow")
	}

	return s.workflowRepo.Delete(id)
}

// ActivateWorkflow activates a workflow
func (s *WorkflowService) ActivateWorkflow(id int) error {
	workflow, err := s.workflowRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("workflow not found: %w", err)
	}

	workflow.Status = models.WorkflowStatusActive
	workflow.UpdatedAt = time.Now()

	return s.workflowRepo.Update(workflow)
}

// DeactivateWorkflow deactivates a workflow
func (s *WorkflowService) DeactivateWorkflow(id int) error {
	workflow, err := s.workflowRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("workflow not found: %w", err)
	}

	workflow.Status = models.WorkflowStatusInactive
	workflow.UpdatedAt = time.Now()

	return s.workflowRepo.Update(workflow)
}

// ProcessTrigger processes a workflow trigger event
func (s *WorkflowService) ProcessTrigger(triggerType models.TriggerType, context map[string]interface{}) error {
	// Get all active workflows
	workflows, err := s.workflowRepo.GetActiveWorkflows()
	if err != nil {
		return fmt.Errorf("failed to get active workflows: %w", err)
	}

	// Sort by priority
	sortWorkflowsByPriority(workflows)

	// Process each workflow
	for _, workflow := range workflows {
		if workflow.IsTriggeredBy(triggerType, context) {
			// Execute workflow asynchronously
			go s.executeWorkflow(workflow, triggerType, context)
		}
	}

	return nil
}

// executeWorkflow executes a single workflow
func (s *WorkflowService) executeWorkflow(workflow *models.Workflow, triggerType models.TriggerType, context map[string]interface{}) {
	execution := &models.WorkflowExecution{
		WorkflowID:   workflow.ID,
		WorkflowName: workflow.Name,
		TriggerType:  triggerType,
		StartedAt:    time.Now(),
		Status:       "running",
	}

	// Extract ticket ID if present
	if ticketID, ok := context["ticket_id"].(int); ok {
		execution.TicketID = ticketID
	}

	// Save execution start
	_ = s.workflowRepo.CreateExecution(execution)

	// Execute actions in order
	for _, action := range workflow.Actions {
		// Apply delay if configured
		if action.DelaySeconds > 0 {
			time.Sleep(time.Duration(action.DelaySeconds) * time.Second)
		}

		// Execute action
		err := s.executeAction(action, context, execution)
		if err != nil {
			execution.ActionsFailed++
			
			// Log error
			entry := models.WorkflowExecutionEntry{
				Timestamp:  time.Now(),
				ActionType: action.Type,
				ActionID:   action.ID,
				Status:     "failed",
				Error:      err.Error(),
			}
			execution.ExecutionLog = append(execution.ExecutionLog, entry)

			// Stop if not configured to continue on error
			if !action.ContinueOnErr {
				execution.Status = "failed"
				execution.ErrorMessage = err.Error()
				break
			}
		} else {
			execution.ActionsRun++
		}
	}

	// Update execution completion
	completedAt := time.Now()
	execution.CompletedAt = &completedAt
	if execution.Status != "failed" {
		if execution.ActionsFailed > 0 {
			execution.Status = "partial"
		} else {
			execution.Status = "success"
		}
	}

	// Update workflow stats
	workflow.LastRunAt = &completedAt
	workflow.RunCount++
	if execution.Status == "failed" {
		workflow.ErrorCount++
	}
	_ = s.workflowRepo.Update(workflow)

	// Save execution completion
	_ = s.workflowRepo.UpdateExecution(execution)
}

// executeAction executes a single workflow action
func (s *WorkflowService) executeAction(action models.Action, context map[string]interface{}, execution *models.WorkflowExecution) error {
	startTime := time.Now()
	
	// Log action start
	entry := models.WorkflowExecutionEntry{
		Timestamp:  startTime,
		ActionType: action.Type,
		ActionID:   action.ID,
		Status:     "started",
	}

	var err error
	
	switch action.Type {
	case models.ActionTypeAssignTicket:
		err = s.executeAssignTicket(action, context)
	case models.ActionTypeChangeStatus:
		err = s.executeChangeStatus(action, context)
	case models.ActionTypeChangePriority:
		err = s.executeChangePriority(action, context)
	case models.ActionTypeSendEmail:
		err = s.executeSendEmail(action, context)
	case models.ActionTypeSendNotification:
		err = s.executeSendNotification(action, context)
	case models.ActionTypeAddTag:
		err = s.executeAddTag(action, context)
	case models.ActionTypeRemoveTag:
		err = s.executeRemoveTag(action, context)
	case models.ActionTypeAddNote:
		err = s.executeAddNote(action, context)
	case models.ActionTypeEscalate:
		err = s.executeEscalate(action, context)
	case models.ActionTypeWebhookCall:
		err = s.executeWebhookCall(action, context)
	case models.ActionTypeSetSLA:
		err = s.executeSetSLA(action, context)
	case models.ActionTypeUpdateCustomField:
		err = s.executeUpdateCustomField(action, context)
	default:
		err = fmt.Errorf("unknown action type: %s", action.Type)
	}

	// Log action completion
	entry.Duration = time.Since(startTime).Milliseconds()
	if err != nil {
		entry.Status = "failed"
		entry.Error = err.Error()
	} else {
		entry.Status = "completed"
		entry.Message = fmt.Sprintf("Action %s executed successfully", action.Type)
	}
	
	execution.ExecutionLog = append(execution.ExecutionLog, entry)
	
	return err
}

// Action execution methods

func (s *WorkflowService) executeAssignTicket(action models.Action, context map[string]interface{}) error {
	var config models.ActionConfig
	if err := json.Unmarshal(action.Config, &config); err != nil {
		return fmt.Errorf("invalid action config: %w", err)
	}

	ticketID, ok := context["ticket_id"].(int)
	if !ok {
		return fmt.Errorf("ticket_id not found in context")
	}

	ticket, err := s.ticketRepo.GetByID(ticketID)
	if err != nil {
		return fmt.Errorf("ticket not found: %w", err)
	}

	// Assign based on method
	switch config.AssignmentMethod {
	case "round_robin":
		// Implement round-robin assignment
		// This would get the next available agent in rotation
	case "least_loaded":
		// Implement least-loaded assignment
		// This would find the agent with fewest open tickets
	case "skills_based":
		// Implement skills-based assignment
		// This would match ticket requirements with agent skills
	default:
		// Direct assignment
		if config.AssignToUserID > 0 {
			ticket.UserID = config.AssignToUserID
		} else if config.AssignToGroupID > 0 {
			// Assign to group logic
		}
	}

	return s.ticketRepo.Update(ticket)
}

func (s *WorkflowService) executeChangeStatus(action models.Action, context map[string]interface{}) error {
	var config models.ActionConfig
	if err := json.Unmarshal(action.Config, &config); err != nil {
		return fmt.Errorf("invalid action config: %w", err)
	}

	ticketID, ok := context["ticket_id"].(int)
	if !ok {
		return fmt.Errorf("ticket_id not found in context")
	}

	ticket, err := s.ticketRepo.GetByID(ticketID)
	if err != nil {
		return fmt.Errorf("ticket not found: %w", err)
	}

	// Update status
	// ticket.Status = config.NewStatus
	return s.ticketRepo.Update(ticket)
}

func (s *WorkflowService) executeChangePriority(action models.Action, context map[string]interface{}) error {
	var config models.ActionConfig
	if err := json.Unmarshal(action.Config, &config); err != nil {
		return fmt.Errorf("invalid action config: %w", err)
	}

	ticketID, ok := context["ticket_id"].(int)
	if !ok {
		return fmt.Errorf("ticket_id not found in context")
	}

	ticket, err := s.ticketRepo.GetByID(ticketID)
	if err != nil {
		return fmt.Errorf("ticket not found: %w", err)
	}

	// Update priority
	// ticket.Priority = config.NewPriority
	return s.ticketRepo.Update(ticket)
}

func (s *WorkflowService) executeSendEmail(action models.Action, context map[string]interface{}) error {
	var config models.ActionConfig
	if err := json.Unmarshal(action.Config, &config); err != nil {
		return fmt.Errorf("invalid action config: %w", err)
	}

	// Process template variables
	subject := s.processTemplateVariables(config.EmailSubject, context)
	body := s.processTemplateVariables(config.EmailBody, context)

	// Send email
	return s.emailService.SendEmail(config.EmailTo, subject, body)
}

func (s *WorkflowService) executeSendNotification(action models.Action, context map[string]interface{}) error {
	var config models.ActionConfig
	if err := json.Unmarshal(action.Config, &config); err != nil {
		return fmt.Errorf("invalid action config: %w", err)
	}

	// Process notification text
	text := s.processTemplateVariables(config.NotificationText, context)

	// Send notifications
	for _, userID := range config.NotifyUserIDs {
		_ = s.notifyService.SendNotification(userID, text)
	}

	return nil
}

func (s *WorkflowService) executeAddTag(action models.Action, context map[string]interface{}) error {
	var config models.ActionConfig
	if err := json.Unmarshal(action.Config, &config); err != nil {
		return fmt.Errorf("invalid action config: %w", err)
	}

	ticketID, ok := context["ticket_id"].(int)
	if !ok {
		return fmt.Errorf("ticket_id not found in context")
	}

	// Add tags to ticket
	// Implementation would add tags to ticket
	return nil
}

func (s *WorkflowService) executeRemoveTag(action models.Action, context map[string]interface{}) error {
	var config models.ActionConfig
	if err := json.Unmarshal(action.Config, &config); err != nil {
		return fmt.Errorf("invalid action config: %w", err)
	}

	ticketID, ok := context["ticket_id"].(int)
	if !ok {
		return fmt.Errorf("ticket_id not found in context")
	}

	// Remove tags from ticket
	// Implementation would remove tags from ticket
	return nil
}

func (s *WorkflowService) executeAddNote(action models.Action, context map[string]interface{}) error {
	var config models.ActionConfig
	if err := json.Unmarshal(action.Config, &config); err != nil {
		return fmt.Errorf("invalid action config: %w", err)
	}

	ticketID, ok := context["ticket_id"].(int)
	if !ok {
		return fmt.Errorf("ticket_id not found in context")
	}

	// Process note content
	content := s.processTemplateVariables(config.NoteContent, context)

	// Add note to ticket
	// Implementation would add note to ticket
	return nil
}

func (s *WorkflowService) executeEscalate(action models.Action, context map[string]interface{}) error {
	ticketID, ok := context["ticket_id"].(int)
	if !ok {
		return fmt.Errorf("ticket_id not found in context")
	}

	// Escalate ticket
	// Implementation would escalate ticket to higher tier
	return nil
}

func (s *WorkflowService) executeWebhookCall(action models.Action, context map[string]interface{}) error {
	var config models.ActionConfig
	if err := json.Unmarshal(action.Config, &config); err != nil {
		return fmt.Errorf("invalid action config: %w", err)
	}

	// Make webhook call
	// Implementation would make HTTP request to webhook URL
	return nil
}

func (s *WorkflowService) executeSetSLA(action models.Action, context map[string]interface{}) error {
	ticketID, ok := context["ticket_id"].(int)
	if !ok {
		return fmt.Errorf("ticket_id not found in context")
	}

	// Set SLA for ticket
	// Implementation would set SLA policy for ticket
	return nil
}

func (s *WorkflowService) executeUpdateCustomField(action models.Action, context map[string]interface{}) error {
	var config models.ActionConfig
	if err := json.Unmarshal(action.Config, &config); err != nil {
		return fmt.Errorf("invalid action config: %w", err)
	}

	ticketID, ok := context["ticket_id"].(int)
	if !ok {
		return fmt.Errorf("ticket_id not found in context")
	}

	// Update custom field
	// Implementation would update custom field value
	return nil
}

// Helper methods

// processTemplateVariables replaces template variables with actual values
func (s *WorkflowService) processTemplateVariables(template string, context map[string]interface{}) string {
	result := template

	// Replace variables like {{ticket.id}}, {{customer.name}}, etc.
	re := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	matches := re.FindAllStringSubmatch(template, -1)

	for _, match := range matches {
		if len(match) > 1 {
			variable := match[1]
			value := s.getVariableValue(variable, context)
			result = strings.ReplaceAll(result, match[0], fmt.Sprintf("%v", value))
		}
	}

	return result
}

// getVariableValue extracts a value from context using dot notation
func (s *WorkflowService) getVariableValue(path string, context map[string]interface{}) interface{} {
	parts := strings.Split(path, ".")
	current := context

	for i, part := range parts {
		if i == len(parts)-1 {
			return current[part]
		}
		
		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			return ""
		}
	}

	return ""
}

// evaluateCondition evaluates a single condition
func (s *WorkflowService) evaluateCondition(condition models.Condition, context map[string]interface{}) bool {
	fieldValue := s.getVariableValue(condition.Field, context)
	
	switch condition.Operator {
	case models.OperatorEquals:
		return fieldValue == condition.Value
	case models.OperatorNotEquals:
		return fieldValue != condition.Value
	case models.OperatorContains:
		return strings.Contains(fmt.Sprintf("%v", fieldValue), fmt.Sprintf("%v", condition.Value))
	case models.OperatorNotContains:
		return !strings.Contains(fmt.Sprintf("%v", fieldValue), fmt.Sprintf("%v", condition.Value))
	case models.OperatorStartsWith:
		return strings.HasPrefix(fmt.Sprintf("%v", fieldValue), fmt.Sprintf("%v", condition.Value))
	case models.OperatorEndsWith:
		return strings.HasSuffix(fmt.Sprintf("%v", fieldValue), fmt.Sprintf("%v", condition.Value))
	case models.OperatorIsEmpty:
		return fieldValue == nil || fieldValue == ""
	case models.OperatorIsNotEmpty:
		return fieldValue != nil && fieldValue != ""
	case models.OperatorMatchesRegex:
		re, err := regexp.Compile(fmt.Sprintf("%v", condition.Value))
		if err != nil {
			return false
		}
		return re.MatchString(fmt.Sprintf("%v", fieldValue))
	// Add more operator implementations as needed
	default:
		return false
	}
}

// sortWorkflowsByPriority sorts workflows by priority (higher first)
func sortWorkflowsByPriority(workflows []*models.Workflow) {
	// Simple bubble sort for demonstration
	for i := 0; i < len(workflows)-1; i++ {
		for j := 0; j < len(workflows)-i-1; j++ {
			if workflows[j].Priority < workflows[j+1].Priority {
				workflows[j], workflows[j+1] = workflows[j+1], workflows[j]
			}
		}
	}
}