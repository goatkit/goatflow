package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

// TicketTemplateService handles business logic for ticket templates
type TicketTemplateService struct {
	repo repository.TicketTemplateRepository
	ticketService *SimpleTicketService
}

// NewTicketTemplateService creates a new ticket template service
func NewTicketTemplateService(repo repository.TicketTemplateRepository, ticketService *SimpleTicketService) *TicketTemplateService {
	return &TicketTemplateService{
		repo: repo,
		ticketService: ticketService,
	}
}

// CreateTemplate creates a new ticket template
func (s *TicketTemplateService) CreateTemplate(ctx context.Context, template *models.TicketTemplate) error {
	// Validate template
	if err := s.validateTemplate(template); err != nil {
		return err
	}
	
	// Extract variables from subject and body
	template.Variables = s.extractVariables(template.Subject + " " + template.Body)
	
	return s.repo.CreateTemplate(ctx, template)
}

// GetTemplate retrieves a template by ID
func (s *TicketTemplateService) GetTemplate(ctx context.Context, id uint) (*models.TicketTemplate, error) {
	return s.repo.GetTemplateByID(ctx, id)
}

// GetActiveTemplates retrieves all active templates
func (s *TicketTemplateService) GetActiveTemplates(ctx context.Context) ([]models.TicketTemplate, error) {
	return s.repo.GetActiveTemplates(ctx)
}

// GetTemplatesByCategory retrieves templates by category
func (s *TicketTemplateService) GetTemplatesByCategory(ctx context.Context, category string) ([]models.TicketTemplate, error) {
	return s.repo.GetTemplatesByCategory(ctx, category)
}

// UpdateTemplate updates an existing template
func (s *TicketTemplateService) UpdateTemplate(ctx context.Context, template *models.TicketTemplate) error {
	// Validate template
	if err := s.validateTemplate(template); err != nil {
		return err
	}
	
	// Re-extract variables
	template.Variables = s.extractVariables(template.Subject + " " + template.Body)
	
	return s.repo.UpdateTemplate(ctx, template)
}

// DeleteTemplate deletes a template
func (s *TicketTemplateService) DeleteTemplate(ctx context.Context, id uint) error {
	return s.repo.DeleteTemplate(ctx, id)
}

// SearchTemplates searches for templates
func (s *TicketTemplateService) SearchTemplates(ctx context.Context, query string) ([]models.TicketTemplate, error) {
	return s.repo.SearchTemplates(ctx, query)
}

// GetCategories retrieves all template categories
func (s *TicketTemplateService) GetCategories(ctx context.Context) ([]models.TemplateCategory, error) {
	return s.repo.GetCategories(ctx)
}

// ApplyTemplate creates a new ticket from a template
func (s *TicketTemplateService) ApplyTemplate(ctx context.Context, application *models.TemplateApplication) (*models.Ticket, error) {
	// Get the template
	template, err := s.repo.GetTemplateByID(ctx, application.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}
	
	if !template.Active {
		return nil, fmt.Errorf("template is not active")
	}
	
	// Apply variable substitutions
	subject := s.substituteVariables(template.Subject, application.Variables)
	body := s.substituteVariables(template.Body, application.Variables)
	
	// Add additional notes if provided
	if application.AdditionalNotes != "" {
		body += "\n\n---\nAdditional Notes:\n" + application.AdditionalNotes
	}
	
	// TODO: Create ticket from template - needs proper ticket model integration
	// For now, just create a basic ticket structure that can be used
	// when the ticket creation is properly integrated with OTRS schema
	ticket := &models.Ticket{
		Title:            subject,
		QueueID:          template.QueueID,
		TypeID:           template.TypeID,
		TicketPriorityID: 3, // Default to normal priority  
	}
	
	// Increment template usage count
	if err := s.repo.IncrementUsageCount(ctx, template.ID); err != nil {
		// Log error but don't fail the ticket creation
		fmt.Printf("Warning: failed to increment template usage count: %v\n", err)
	}
	
	return ticket, nil
}

// validateTemplate validates a template before saving
func (s *TicketTemplateService) validateTemplate(template *models.TicketTemplate) error {
	if template.Name == "" {
		return fmt.Errorf("template name is required")
	}
	if template.Subject == "" {
		return fmt.Errorf("template subject is required")
	}
	if template.Body == "" {
		return fmt.Errorf("template body is required")
	}
	if len(template.Name) > 255 {
		return fmt.Errorf("template name too long (max 255 characters)")
	}
	return nil
}

// extractVariables extracts variable placeholders from text
func (s *TicketTemplateService) extractVariables(text string) []models.TemplateVariable {
	variableMap := make(map[string]bool)
	var variables []models.TemplateVariable
	
	// Find all {{variable}} patterns
	start := 0
	for {
		idx := strings.Index(text[start:], "{{")
		if idx == -1 {
			break
		}
		idx += start
		
		endIdx := strings.Index(text[idx:], "}}")
		if endIdx == -1 {
			break
		}
		endIdx += idx + 2
		
		variable := text[idx:endIdx]
		if !variableMap[variable] {
			variableMap[variable] = true
			
			// Extract variable name (without brackets)
			varName := strings.TrimSpace(variable[2 : len(variable)-2])
			
			variables = append(variables, models.TemplateVariable{
				Name:        variable,
				Description: fmt.Sprintf("Value for %s", varName),
				Required:    false, // Can be set manually later
			})
		}
		
		start = endIdx
	}
	
	return variables
}

// substituteVariables replaces variables in text with their values
func (s *TicketTemplateService) substituteVariables(text string, variables map[string]string) string {
	result := text
	
	// Replace each variable
	for key, value := range variables {
		result = strings.ReplaceAll(result, key, value)
	}
	
	// Also try without the brackets in case they're provided differently
	for key, value := range variables {
		if strings.HasPrefix(key, "{{") && strings.HasSuffix(key, "}}") {
			continue // Already has brackets
		}
		result = strings.ReplaceAll(result, "{{"+key+"}}", value)
	}
	
	return result
}

// GetPopularTemplates returns the most used templates
func (s *TicketTemplateService) GetPopularTemplates(ctx context.Context, limit int) ([]models.TicketTemplate, error) {
	templates, err := s.repo.GetActiveTemplates(ctx)
	if err != nil {
		return nil, err
	}
	
	// Sort by usage count
	for i := 0; i < len(templates); i++ {
		for j := i + 1; j < len(templates); j++ {
			if templates[j].UsageCount > templates[i].UsageCount {
				templates[i], templates[j] = templates[j], templates[i]
			}
		}
	}
	
	// Return top N
	if limit > 0 && limit < len(templates) {
		templates = templates[:limit]
	}
	
	return templates, nil
}