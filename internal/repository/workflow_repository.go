package repository

import (
	"time"

	"github.com/goatkit/goatflow/internal/models"
)

// WorkflowRepository defines the interface for workflow data operations.
type WorkflowRepository interface {
	// Workflow CRUD operations
	Create(workflow *models.Workflow) error
	GetByID(id int) (*models.Workflow, error)
	Update(workflow *models.Workflow) error
	Delete(id int) error

	// Query operations
	GetAll() ([]*models.Workflow, error)
	GetActiveWorkflows() ([]*models.Workflow, error)
	GetByStatus(status models.WorkflowStatus) ([]*models.Workflow, error)
	GetByTriggerType(triggerType models.TriggerType) ([]*models.Workflow, error)
	Search(query string) ([]*models.Workflow, error)

	// Trigger operations
	CreateTrigger(trigger *models.Trigger) error
	UpdateTrigger(trigger *models.Trigger) error
	DeleteTrigger(id int) error
	GetTriggersByWorkflow(workflowID int) ([]*models.Trigger, error)

	// Condition operations
	CreateCondition(condition *models.Condition) error
	UpdateCondition(condition *models.Condition) error
	DeleteCondition(id int) error
	GetConditionsByWorkflow(workflowID int) ([]*models.Condition, error)

	// Action operations
	CreateAction(action *models.Action) error
	UpdateAction(action *models.Action) error
	DeleteAction(id int) error
	GetActionsByWorkflow(workflowID int) ([]*models.Action, error)

	// Execution tracking
	CreateExecution(execution *models.WorkflowExecution) error
	UpdateExecution(execution *models.WorkflowExecution) error
	GetExecutionByID(id int) (*models.WorkflowExecution, error)
	GetExecutionsByWorkflow(workflowID int, limit int) ([]*models.WorkflowExecution, error)
	GetExecutionsByTicket(ticketID int) ([]*models.WorkflowExecution, error)
	GetExecutionsByDateRange(start, end time.Time) ([]*models.WorkflowExecution, error)
	GetFailedExecutions(limit int) ([]*models.WorkflowExecution, error)

	// Schedule operations
	CreateSchedule(schedule *models.WorkflowSchedule) error
	UpdateSchedule(schedule *models.WorkflowSchedule) error
	DeleteSchedule(id int) error
	GetScheduleByWorkflow(workflowID int) (*models.WorkflowSchedule, error)
	GetActiveSchedules() ([]*models.WorkflowSchedule, error)
	GetSchedulesDueForExecution(before time.Time) ([]*models.WorkflowSchedule, error)

	// Template operations
	CreateTemplate(template *models.WorkflowTemplate) error
	GetTemplateByID(id int) (*models.WorkflowTemplate, error)
	GetTemplatesByCategory(category string) ([]*models.WorkflowTemplate, error)
	GetAllTemplates() ([]*models.WorkflowTemplate, error)
	IncrementTemplateUsage(id int) error

	// Statistics
	GetWorkflowStats(workflowID int) (map[string]interface{}, error)
	GetGlobalWorkflowStats() (map[string]interface{}, error)
}
