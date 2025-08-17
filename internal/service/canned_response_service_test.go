package service

import (
	"context"
	"strings"
	"testing"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCannedResponseService(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateResponse", func(t *testing.T) {
		repo := repository.NewMemoryCannedResponseRepository()
		service := NewCannedResponseService(repo)

		response := &models.CannedResponse{
			Name:        "Test Response",
			Category:    "Test",
			Content:     "Hello {{customer_name}}, this is a test.",
			ContentType: "text/plain",
			IsActive:    true,
		}

		err := service.CreateResponse(ctx, response)
		require.NoError(t, err)
		assert.NotZero(t, response.ID)

		// Verify variables were extracted
		assert.Equal(t, 1, len(response.Variables))
		assert.Equal(t, "{{customer_name}}", response.Variables[0].Name)
	})

	t.Run("CreateResponse_Validation", func(t *testing.T) {
		repo := repository.NewMemoryCannedResponseRepository()
		service := NewCannedResponseService(repo)

		tests := []struct {
			name     string
			response *models.CannedResponse
			wantErr  string
		}{
			{
				name: "Missing name",
				response: &models.CannedResponse{
					Category: "Test",
					Content:  "Content",
				},
				wantErr: "response name is required",
			},
			{
				name: "Missing category",
				response: &models.CannedResponse{
					Name:    "Test",
					Content: "Content",
				},
				wantErr: "response category is required",
			},
			{
				name: "Missing content",
				response: &models.CannedResponse{
					Name:     "Test",
					Category: "Test",
				},
				wantErr: "response content is required",
			},
			{
				name: "Name too long",
				response: &models.CannedResponse{
					Name:     strings.Repeat("a", 256),
					Category: "Test",
					Content:  "Content",
				},
				wantErr: "response name too long",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := service.CreateResponse(ctx, tt.response)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			})
		}
	})

	t.Run("GetQuickResponses", func(t *testing.T) {
		repo := repository.NewMemoryCannedResponseRepository()
		service := NewCannedResponseService(repo)

		// Create some responses with shortcuts
		responses := []models.CannedResponse{
			{Name: "Greeting", Shortcut: "/hello", Category: "General", Content: "Hello!", IsActive: true},
			{Name: "Goodbye", Shortcut: "/bye", Category: "General", Content: "Goodbye!", IsActive: true},
			{Name: "No Shortcut", Category: "General", Content: "No shortcut", IsActive: true},
			{Name: "Inactive", Shortcut: "/inactive", Category: "General", Content: "Inactive", IsActive: false},
		}

		for i := range responses {
			repo.CreateResponse(ctx, &responses[i])
		}

		// Get quick responses
		quick, err := service.GetQuickResponses(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, len(quick)) // Only active responses with shortcuts
	})

	t.Run("ApplyResponse", func(t *testing.T) {
		repo := repository.NewMemoryCannedResponseRepository()
		service := NewCannedResponseService(repo)

		// Create a response with variables
		response := &models.CannedResponse{
			Name:     "Welcome",
			Category: "Greetings",
			Subject:  "Welcome {{customer_name}}",
			Content:  "Hello {{customer_name}},\n\nYour ticket {{ticket_number}} has been received.\n\nBest regards,\n{{agent_name}}",
			IsActive: true,
			Variables: []models.ResponseVariable{
				{Name: "{{customer_name}}", AutoFill: "customer_name"},
				{Name: "{{ticket_number}}", AutoFill: "ticket_number"},
				{Name: "{{agent_name}}", AutoFill: "agent_name"},
			},
		}
		repo.CreateResponse(ctx, response)

		// Apply the response
		application := &models.CannedResponseApplication{
			ResponseID: response.ID,
			TicketID:   123,
			Variables: map[string]string{
				"{{customer_name}}": "John Doe",
				"{{ticket_number}}": "TICKET-123",
				"{{agent_name}}":    "Support Agent",
			},
		}

		result, err := service.ApplyResponse(ctx, application, 456)
		require.NoError(t, err)
		assert.Equal(t, "Welcome John Doe", result.Subject)
		assert.Contains(t, result.Content, "Hello John Doe")
		assert.Contains(t, result.Content, "Your ticket TICKET-123")
		assert.Contains(t, result.Content, "Support Agent")

		// Verify usage was recorded
		updated, _ := repo.GetResponseByID(ctx, response.ID)
		assert.Equal(t, 1, updated.UsageCount)
	})

	t.Run("ApplyResponse_AutoFill", func(t *testing.T) {
		repo := repository.NewMemoryCannedResponseRepository()
		service := NewCannedResponseService(repo)

		// Create a response with auto-fill variables
		response := &models.CannedResponse{
			Name:     "Auto Response",
			Category: "Test",
			Content:  "Agent: {{agent_name}}, Time: {{current_time}}, Date: {{current_date}}",
			IsActive: true,
		}
		service.CreateResponse(ctx, response)

		// Apply with auto-fill context
		application := &models.CannedResponseApplication{
			ResponseID: response.ID,
			TicketID:   123,
			Variables:  map[string]string{}, // No manual variables
		}

		context := &models.AutoFillContext{
			AgentName:    "John Agent",
			AgentEmail:   "john@example.com",
			TicketNumber: "TICKET-123",
			CustomerName: "Jane Customer",
		}

		result, err := service.ApplyResponseWithContext(ctx, application, 456, context)
		require.NoError(t, err)
		assert.Contains(t, result.Content, "Agent: John Agent")
		assert.Contains(t, result.Content, "Time:")
		assert.Contains(t, result.Content, "Date:")
	})

	t.Run("SearchResponses", func(t *testing.T) {
		repo := repository.NewMemoryCannedResponseRepository()
		service := NewCannedResponseService(repo)

		// Create various responses
		responses := []models.CannedResponse{
			{Name: "Password Reset", Category: "Account", Content: "Reset your password", Tags: []string{"password", "security"}, IsActive: true},
			{Name: "Network Issue", Category: "Technical", Content: "Network troubleshooting", Tags: []string{"network"}, IsActive: true},
			{Name: "Billing Info", Category: "Billing", Content: "Payment information", Tags: []string{"billing", "payment"}, IsActive: true},
			{Name: "Password Change", Category: "Account", Content: "Change password guide", Tags: []string{"password"}, IsActive: true},
		}

		for i := range responses {
			repo.CreateResponse(ctx, &responses[i])
		}

		// Search for "password"
		filter := &models.CannedResponseFilter{
			Query: "password",
			Limit: 10,
		}
		results, err := service.SearchResponses(ctx, filter)
		require.NoError(t, err)
		assert.Equal(t, 2, len(results))

		// Search by category
		filter = &models.CannedResponseFilter{
			Category: "Account",
			Limit:    10,
		}
		results, err = service.SearchResponses(ctx, filter)
		require.NoError(t, err)
		assert.Equal(t, 2, len(results))

		// Search by tag
		filter = &models.CannedResponseFilter{
			Tags:  []string{"billing"},
			Limit: 10,
		}
		results, err = service.SearchResponses(ctx, filter)
		require.NoError(t, err)
		assert.Equal(t, 1, len(results))
	})

	t.Run("GetPopularResponses", func(t *testing.T) {
		repo := repository.NewMemoryCannedResponseRepository()
		service := NewCannedResponseService(repo)

		// Create responses with different usage counts
		responses := []struct {
			name  string
			count int
		}{
			{"Low Usage", 2},
			{"High Usage", 10},
			{"Medium Usage", 5},
			{"No Usage", 0},
		}

		for _, resp := range responses {
			response := &models.CannedResponse{
				Name:       resp.name,
				Category:   "Test",
				Content:    "Content",
				IsActive:   true,
				UsageCount: resp.count,
			}
			repo.CreateResponse(ctx, response)
		}

		// Get popular responses
		popular, err := service.GetPopularResponses(ctx, 3)
		require.NoError(t, err)
		assert.Equal(t, 3, len(popular))
		assert.Equal(t, "High Usage", popular[0].Name)
		assert.Equal(t, "Medium Usage", popular[1].Name)
		assert.Equal(t, "Low Usage", popular[2].Name)
	})

	t.Run("ExportImportResponses", func(t *testing.T) {
		repo := repository.NewMemoryCannedResponseRepository()
		service := NewCannedResponseService(repo)

		// Create some responses
		responses := []models.CannedResponse{
			{Name: "Response 1", Category: "Cat1", Content: "Content 1", Tags: []string{"tag1"}, IsActive: true},
			{Name: "Response 2", Category: "Cat2", Content: "Content 2", Tags: []string{"tag2"}, IsActive: true},
		}

		for i := range responses {
			service.CreateResponse(ctx, &responses[i])
		}

		// Export
		exported, err := service.ExportResponses(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, exported)

		// Clear repository
		repo = repository.NewMemoryCannedResponseRepository()
		service = NewCannedResponseService(repo)

		// Import
		err = service.ImportResponses(ctx, exported)
		require.NoError(t, err)

		// Verify imported
		all, err := service.GetActiveResponses(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, len(all))
	})

	t.Run("VariableExtraction", func(t *testing.T) {
		tests := []struct {
			name      string
			content   string
			expected  int
			variables []string
		}{
			{
				name:      "Simple variables",
				content:   "Hello {{name}}, your ID is {{id}}",
				expected:  2,
				variables: []string{"{{name}}", "{{id}}"},
			},
			{
				name:      "Repeated variables",
				content:   "{{user}} reported. Contact {{user}} at {{email}}",
				expected:  2,
				variables: []string{"{{user}}", "{{email}}"},
			},
			{
				name:      "No variables",
				content:   "This is plain text",
				expected:  0,
				variables: []string{},
			},
			{
				name:      "Complex variables",
				content:   "{{customer_first_name}} {{customer_last_name}} ({{customer_id}})",
				expected:  3,
				variables: []string{"{{customer_first_name}}", "{{customer_last_name}}", "{{customer_id}}"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				service := &CannedResponseService{}
				vars := service.extractVariables(tt.content)
				assert.Equal(t, tt.expected, len(vars))
				
				for _, expectedVar := range tt.variables {
					found := false
					for _, v := range vars {
						if v.Name == expectedVar {
							found = true
							break
						}
					}
					assert.True(t, found, "Variable %s not found", expectedVar)
				}
			})
		}
	})

	t.Run("VariableSubstitution", func(t *testing.T) {
		tests := []struct {
			name      string
			content   string
			variables map[string]string
			expected  string
		}{
			{
				name:    "Simple substitution",
				content: "Hello {{name}}, your ticket is {{status}}.",
				variables: map[string]string{
					"{{name}}":   "John",
					"{{status}}": "open",
				},
				expected: "Hello John, your ticket is open.",
			},
			{
				name:    "Multiple occurrences",
				content: "{{user}} reported. Contact {{user}} at {{email}}.",
				variables: map[string]string{
					"{{user}}":  "Alice",
					"{{email}}": "alice@example.com",
				},
				expected: "Alice reported. Contact Alice at alice@example.com.",
			},
			{
				name:    "Missing variable",
				content: "Hello {{name}}, ID: {{id}}.",
				variables: map[string]string{
					"{{name}}": "Bob",
				},
				expected: "Hello Bob, ID: {{id}}.",
			},
			{
				name:      "No variables",
				content:   "Plain text message.",
				variables: map[string]string{},
				expected:  "Plain text message.",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				service := &CannedResponseService{}
				result := service.substituteVariables(tt.content, tt.variables)
				assert.Equal(t, tt.expected, result)
			})
		}
	})
}