package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
)

// CannedResponseService handles business logic for canned responses
type CannedResponseService struct {
	repo repository.CannedResponseRepository
}

// NewCannedResponseService creates a new canned response service
func NewCannedResponseService(repo repository.CannedResponseRepository) *CannedResponseService {
	return &CannedResponseService{
		repo: repo,
	}
}

// CreateResponse creates a new canned response
func (s *CannedResponseService) CreateResponse(ctx context.Context, response *models.CannedResponse) error {
	// Validate response
	if err := s.validateResponse(response); err != nil {
		return err
	}

	// Extract variables from content and subject
	content := response.Content
	if response.Subject != "" {
		content = response.Subject + " " + content
	}
	response.Variables = s.extractVariables(content)

	// Set default content type if not specified
	if response.ContentType == "" {
		response.ContentType = "text/plain"
	}

	return s.repo.CreateResponse(ctx, response)
}

// GetResponse retrieves a response by ID
func (s *CannedResponseService) GetResponse(ctx context.Context, id uint) (*models.CannedResponse, error) {
	return s.repo.GetResponseByID(ctx, id)
}

// GetResponseByShortcut retrieves a response by its shortcut
func (s *CannedResponseService) GetResponseByShortcut(ctx context.Context, shortcut string) (*models.CannedResponse, error) {
	return s.repo.GetResponseByShortcut(ctx, shortcut)
}

// GetActiveResponses retrieves all active responses
func (s *CannedResponseService) GetActiveResponses(ctx context.Context) ([]models.CannedResponse, error) {
	return s.repo.GetActiveResponses(ctx)
}

// GetQuickResponses retrieves responses with shortcuts for quick access
func (s *CannedResponseService) GetQuickResponses(ctx context.Context) ([]models.CannedResponse, error) {
	responses, err := s.repo.GetActiveResponses(ctx)
	if err != nil {
		return nil, err
	}

	// Filter to only those with shortcuts
	var quick []models.CannedResponse
	for _, resp := range responses {
		if resp.Shortcut != "" {
			quick = append(quick, resp)
		}
	}

	return quick, nil
}

// GetResponsesByCategory retrieves responses by category
func (s *CannedResponseService) GetResponsesByCategory(ctx context.Context, category string) ([]models.CannedResponse, error) {
	return s.repo.GetResponsesByCategory(ctx, category)
}

// GetResponsesForUser retrieves responses accessible to a specific user
func (s *CannedResponseService) GetResponsesForUser(ctx context.Context, userID uint) ([]models.CannedResponse, error) {
	return s.repo.GetResponsesForUser(ctx, userID)
}

// UpdateResponse updates an existing response
func (s *CannedResponseService) UpdateResponse(ctx context.Context, response *models.CannedResponse) error {
	// Validate response
	if err := s.validateResponse(response); err != nil {
		return err
	}

	// Re-extract variables
	content := response.Content
	if response.Subject != "" {
		content = response.Subject + " " + content
	}
	response.Variables = s.extractVariables(content)

	return s.repo.UpdateResponse(ctx, response)
}

// DeleteResponse deletes a response
func (s *CannedResponseService) DeleteResponse(ctx context.Context, id uint) error {
	return s.repo.DeleteResponse(ctx, id)
}

// SearchResponses searches for responses
func (s *CannedResponseService) SearchResponses(ctx context.Context, filter *models.CannedResponseFilter) ([]models.CannedResponse, error) {
	return s.repo.SearchResponses(ctx, filter)
}

// GetPopularResponses returns the most used responses
func (s *CannedResponseService) GetPopularResponses(ctx context.Context, limit int) ([]models.CannedResponse, error) {
	return s.repo.GetMostUsedResponses(ctx, limit)
}

// GetCategories retrieves all response categories
func (s *CannedResponseService) GetCategories(ctx context.Context) ([]models.CannedResponseCategory, error) {
	return s.repo.GetCategories(ctx)
}

// ApplyResponse applies a canned response to a ticket
func (s *CannedResponseService) ApplyResponse(ctx context.Context, application *models.CannedResponseApplication, userID uint) (*models.AppliedResponse, error) {
	// Get the response
	response, err := s.repo.GetResponseByID(ctx, application.ResponseID)
	if err != nil {
		return nil, fmt.Errorf("response not found: %w", err)
	}

	if !response.IsActive {
		return nil, fmt.Errorf("response is not active")
	}

	// Apply variable substitutions
	subject := s.substituteVariables(response.Subject, application.Variables)
	content := s.substituteVariables(response.Content, application.Variables)

	// Record usage
	usage := &models.CannedResponseUsage{
		ResponseID:        response.ID,
		TicketID:          application.TicketID,
		UserID:            userID,
		ModifiedBefore:    false,
	}
	if err := s.repo.RecordUsage(ctx, usage); err != nil {
		// Log error but don't fail the application
		fmt.Printf("Warning: failed to record response usage: %v\n", err)
	}

	// Increment usage count
	if err := s.repo.IncrementUsageCount(ctx, response.ID); err != nil {
		// Log error but don't fail
		fmt.Printf("Warning: failed to increment usage count: %v\n", err)
	}

	return &models.AppliedResponse{
		Subject:      subject,
		Content:      content,
		ContentType:  response.ContentType,
		Attachments:  response.AttachmentURLs,
		AsInternal:   application.AsInternal,
	}, nil
}

// ApplyResponseWithContext applies a response with auto-fill context
func (s *CannedResponseService) ApplyResponseWithContext(ctx context.Context, application *models.CannedResponseApplication, userID uint, autoFillCtx *models.AutoFillContext) (*models.AppliedResponse, error) {
	// Get the response
	response, err := s.repo.GetResponseByID(ctx, application.ResponseID)
	if err != nil {
		return nil, fmt.Errorf("response not found: %w", err)
	}

	// Build variables map with auto-fill values
	variables := make(map[string]string)
	
	// Copy manual variables
	for k, v := range application.Variables {
		variables[k] = v
	}

	// Apply auto-fill for variables not manually provided
	for _, varDef := range response.Variables {
		if _, exists := variables[varDef.Name]; !exists {
			// Auto-fill based on context
			switch varDef.AutoFill {
			case "agent_name":
				if autoFillCtx.AgentName != "" {
					variables[varDef.Name] = autoFillCtx.AgentName
				}
			case "agent_email":
				if autoFillCtx.AgentEmail != "" {
					variables[varDef.Name] = autoFillCtx.AgentEmail
				}
			case "ticket_number":
				if autoFillCtx.TicketNumber != "" {
					variables[varDef.Name] = autoFillCtx.TicketNumber
				}
			case "customer_name":
				if autoFillCtx.CustomerName != "" {
					variables[varDef.Name] = autoFillCtx.CustomerName
				}
			case "customer_email":
				if autoFillCtx.CustomerEmail != "" {
					variables[varDef.Name] = autoFillCtx.CustomerEmail
				}
			case "current_date":
				variables[varDef.Name] = time.Now().Format("2006-01-02")
			case "current_time":
				variables[varDef.Name] = time.Now().Format("15:04")
			case "current_datetime":
				variables[varDef.Name] = time.Now().Format("2006-01-02 15:04")
			default:
				// Use default value if available
				if varDef.DefaultValue != "" {
					variables[varDef.Name] = varDef.DefaultValue
				}
			}
		}
	}

	// Apply standard auto-fill for common variables
	s.applyStandardAutoFill(response.Content + " " + response.Subject, variables, autoFillCtx)

	// Update application with filled variables
	application.Variables = variables

	return s.ApplyResponse(ctx, application, userID)
}

// applyStandardAutoFill applies standard auto-fill for common variable patterns
func (s *CannedResponseService) applyStandardAutoFill(content string, variables map[string]string, ctx *models.AutoFillContext) {
	// Standard variable mappings
	standardMappings := map[string]string{
		"{{agent_name}}":     ctx.AgentName,
		"{{agent_email}}":    ctx.AgentEmail,
		"{{ticket_number}}":  ctx.TicketNumber,
		"{{customer_name}}":  ctx.CustomerName,
		"{{customer_email}}": ctx.CustomerEmail,
		"{{queue_name}}":     ctx.QueueName,
		"{{current_date}}":   time.Now().Format("2006-01-02"),
		"{{current_time}}":   time.Now().Format("15:04"),
		"{{current_datetime}}": time.Now().Format("2006-01-02 15:04"),
	}

	// Apply standard mappings if variable exists in content and not already set
	for varName, value := range standardMappings {
		if strings.Contains(content, varName) && value != "" {
			if _, exists := variables[varName]; !exists {
				variables[varName] = value
			}
		}
	}
}

// ExportResponses exports all responses to JSON
func (s *CannedResponseService) ExportResponses(ctx context.Context) ([]byte, error) {
	responses, err := s.repo.GetActiveResponses(ctx)
	if err != nil {
		return nil, err
	}

	export := struct {
		Version   string                   `json:"version"`
		Exported  time.Time                `json:"exported"`
		Responses []models.CannedResponse  `json:"responses"`
	}{
		Version:   "1.0",
		Exported:  time.Now(),
		Responses: responses,
	}

	return json.MarshalIndent(export, "", "  ")
}

// ImportResponses imports responses from JSON
func (s *CannedResponseService) ImportResponses(ctx context.Context, data []byte) error {
	var export struct {
		Version   string                   `json:"version"`
		Exported  time.Time                `json:"exported"`
		Responses []models.CannedResponse  `json:"responses"`
	}

	if err := json.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("invalid import format: %w", err)
	}

	// Import each response
	for _, response := range export.Responses {
		// Reset ID and timestamps
		response.ID = 0
		response.CreatedAt = time.Time{}
		response.UpdatedAt = time.Time{}
		response.UsageCount = 0

		if err := s.CreateResponse(ctx, &response); err != nil {
			// Log error but continue with other responses
			fmt.Printf("Warning: failed to import response %s: %v\n", response.Name, err)
		}
	}

	return nil
}

// validateResponse validates a response before saving
func (s *CannedResponseService) validateResponse(response *models.CannedResponse) error {
	if response.Name == "" {
		return fmt.Errorf("response name is required")
	}
	if response.Category == "" {
		return fmt.Errorf("response category is required")
	}
	if response.Content == "" {
		return fmt.Errorf("response content is required")
	}
	if len(response.Name) > 255 {
		return fmt.Errorf("response name too long (max 255 characters)")
	}
	return nil
}

// extractVariables extracts variable placeholders from text
func (s *CannedResponseService) extractVariables(text string) []models.ResponseVariable {
	variableMap := make(map[string]bool)
	var variables []models.ResponseVariable

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

			// Determine auto-fill type based on common patterns
			autoFill := s.determineAutoFill(varName)

			variables = append(variables, models.ResponseVariable{
				Name:        variable,
				Description: fmt.Sprintf("Value for %s", varName),
				Type:        "text",
				AutoFill:    autoFill,
			})
		}

		start = endIdx
	}

	return variables
}

// determineAutoFill determines the auto-fill type based on variable name
func (s *CannedResponseService) determineAutoFill(varName string) string {
	varNameLower := strings.ToLower(varName)

	// Common patterns
	patterns := map[string]string{
		"agent_name":     "agent_name",
		"agent_email":    "agent_email",
		"ticket_number":  "ticket_number",
		"ticket_id":      "ticket_number",
		"customer_name":  "customer_name",
		"customer_email": "customer_email",
		"current_date":   "current_date",
		"current_time":   "current_time",
		"date":           "current_date",
		"time":           "current_time",
		"datetime":       "current_datetime",
		"queue":          "queue_name",
		"queue_name":     "queue_name",
	}

	for pattern, autoFill := range patterns {
		if strings.Contains(varNameLower, pattern) {
			return autoFill
		}
	}

	return ""
}

// substituteVariables replaces variables in text with their values
func (s *CannedResponseService) substituteVariables(text string, variables map[string]string) string {
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