package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

func TestMemoryTicketTemplateRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateTemplate", func(t *testing.T) {
		repo := NewMemoryTicketTemplateRepository()

		template := &models.TicketTemplate{
			Name:        "Password Reset Request",
			Description: "Template for password reset tickets",
			Category:    "Account",
			Subject:     "Password Reset for {{username}}",
			Body:        "User {{username}} has requested a password reset.\n\nEmail: {{email}}\nReason: {{reason}}",
			Priority:    "normal",
			QueueID:     1,
			TypeID:      2,
			Tags:        []string{"password", "account", "security"},
			Active:      true,
			Variables: []models.TemplateVariable{
				{Name: "{{username}}", Description: "User's username", Required: true},
				{Name: "{{email}}", Description: "User's email address", Required: true},
				{Name: "{{reason}}", Description: "Reason for reset", Required: false, DefaultValue: "Forgot password"},
			},
		}

		err := repo.CreateTemplate(ctx, template)
		require.NoError(t, err)
		assert.NotZero(t, template.ID)
		assert.NotZero(t, template.CreatedAt)
	})

	t.Run("GetTemplateByID", func(t *testing.T) {
		repo := NewMemoryTicketTemplateRepository()

		// Create a template
		template := &models.TicketTemplate{
			Name:     "Test Template",
			Subject:  "Test Subject",
			Body:     "Test Body",
			Category: "Test",
			Active:   true,
		}
		repo.CreateTemplate(ctx, template)

		// Retrieve it
		retrieved, err := repo.GetTemplateByID(ctx, template.ID)
		require.NoError(t, err)
		assert.Equal(t, template.Name, retrieved.Name)
		assert.Equal(t, template.Subject, retrieved.Subject)
	})

	t.Run("GetTemplateByID_NotFound", func(t *testing.T) {
		repo := NewMemoryTicketTemplateRepository()

		_, err := repo.GetTemplateByID(ctx, 9999)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("GetActiveTemplates", func(t *testing.T) {
		repo := NewMemoryTicketTemplateRepository()

		// Create multiple templates
		for i := 0; i < 5; i++ {
			template := &models.TicketTemplate{
				Name:     "Template " + string(rune('A'+i)),
				Subject:  "Subject",
				Body:     "Body",
				Category: "Test",
				Active:   i%2 == 0, // Only even ones are active
			}
			repo.CreateTemplate(ctx, template)
		}

		// Get active templates
		templates, err := repo.GetActiveTemplates(ctx)
		require.NoError(t, err)
		assert.Equal(t, 3, len(templates)) // Should have 3 active templates

		// Verify all returned templates are active
		for _, tmpl := range templates {
			assert.True(t, tmpl.Active)
		}
	})

	t.Run("GetTemplatesByCategory", func(t *testing.T) {
		repo := NewMemoryTicketTemplateRepository()

		// Create templates in different categories
		categories := []string{"Technical", "Billing", "Technical", "General"}
		for i, cat := range categories {
			template := &models.TicketTemplate{
				Name:     "Template " + string(rune('A'+i)),
				Subject:  "Subject",
				Body:     "Body",
				Category: cat,
				Active:   true,
			}
			repo.CreateTemplate(ctx, template)
		}

		// Get templates by category
		techTemplates, err := repo.GetTemplatesByCategory(ctx, "Technical")
		require.NoError(t, err)
		assert.Equal(t, 2, len(techTemplates))

		billingTemplates, err := repo.GetTemplatesByCategory(ctx, "Billing")
		require.NoError(t, err)
		assert.Equal(t, 1, len(billingTemplates))
	})

	t.Run("UpdateTemplate", func(t *testing.T) {
		repo := NewMemoryTicketTemplateRepository()

		// Create a template
		template := &models.TicketTemplate{
			Name:     "Original Name",
			Subject:  "Original Subject",
			Body:     "Original Body",
			Category: "Test",
			Active:   true,
		}
		repo.CreateTemplate(ctx, template)
		originalID := template.ID

		// Update it
		template.Name = "Updated Name"
		template.Subject = "Updated Subject"
		template.Active = false

		err := repo.UpdateTemplate(ctx, template)
		require.NoError(t, err)

		// Verify update
		updated, err := repo.GetTemplateByID(ctx, originalID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", updated.Name)
		assert.Equal(t, "Updated Subject", updated.Subject)
		assert.False(t, updated.Active)
		assert.True(t, updated.UpdatedAt.After(updated.CreatedAt))
	})

	t.Run("DeleteTemplate", func(t *testing.T) {
		repo := NewMemoryTicketTemplateRepository()

		// Create a template
		template := &models.TicketTemplate{
			Name:     "To Delete",
			Subject:  "Subject",
			Body:     "Body",
			Category: "Test",
			Active:   true,
		}
		repo.CreateTemplate(ctx, template)
		id := template.ID

		// Delete it
		err := repo.DeleteTemplate(ctx, id)
		require.NoError(t, err)

		// Verify it's gone
		_, err = repo.GetTemplateByID(ctx, id)
		assert.Error(t, err)
	})

	t.Run("IncrementUsageCount", func(t *testing.T) {
		repo := NewMemoryTicketTemplateRepository()

		// Create a template
		template := &models.TicketTemplate{
			Name:       "Popular Template",
			Subject:    "Subject",
			Body:       "Body",
			Category:   "Test",
			Active:     true,
			UsageCount: 0,
		}
		repo.CreateTemplate(ctx, template)

		// Increment usage count multiple times
		for i := 0; i < 5; i++ {
			err := repo.IncrementUsageCount(ctx, template.ID)
			require.NoError(t, err)
		}

		// Verify count
		updated, err := repo.GetTemplateByID(ctx, template.ID)
		require.NoError(t, err)
		assert.Equal(t, 5, updated.UsageCount)
	})

	t.Run("SearchTemplates", func(t *testing.T) {
		repo := NewMemoryTicketTemplateRepository()

		// Create templates with different content
		templates := []struct {
			name string
			body string
			tags []string
		}{
			{"Password Reset", "Reset your password", []string{"password", "security"}},
			{"Network Issue", "Network connectivity problem", []string{"network", "technical"}},
			{"Billing Question", "Question about billing", []string{"billing", "payment"}},
			{"Password Change", "Change your password", []string{"password", "account"}},
		}

		for _, tmpl := range templates {
			template := &models.TicketTemplate{
				Name:     tmpl.name,
				Subject:  tmpl.name,
				Body:     tmpl.body,
				Category: "Test",
				Tags:     tmpl.tags,
				Active:   true,
			}
			repo.CreateTemplate(ctx, template)
		}

		// Search for "password"
		results, err := repo.SearchTemplates(ctx, "password")
		require.NoError(t, err)
		assert.Equal(t, 2, len(results))

		// Search for "billing"
		results, err = repo.SearchTemplates(ctx, "billing")
		require.NoError(t, err)
		assert.Equal(t, 1, len(results))

		// Search with no results
		results, err = repo.SearchTemplates(ctx, "nonexistent")
		require.NoError(t, err)
		assert.Equal(t, 0, len(results))
	})

	t.Run("GetCategories", func(t *testing.T) {
		repo := NewMemoryTicketTemplateRepository()

		// Create templates in various categories
		categories := []string{"Technical", "Billing", "Technical", "General", "Billing", "Support"}
		for i, cat := range categories {
			template := &models.TicketTemplate{
				Name:     "Template " + string(rune('A'+i)),
				Subject:  "Subject",
				Body:     "Body",
				Category: cat,
				Active:   true,
			}
			repo.CreateTemplate(ctx, template)
		}

		// Get unique categories
		cats, err := repo.GetCategories(ctx)
		require.NoError(t, err)
		assert.Equal(t, 4, len(cats)) // Technical, Billing, General, Support

		// Verify categories are unique
		catMap := make(map[string]bool)
		for _, cat := range cats {
			assert.False(t, catMap[cat.Name], "Duplicate category found")
			catMap[cat.Name] = true
		}
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		repo := NewMemoryTicketTemplateRepository()
		done := make(chan bool, 100)

		// Concurrent creates
		for i := 0; i < 20; i++ {
			go func(idx int) {
				template := &models.TicketTemplate{
					Name:     "Concurrent " + string(rune('A'+idx)),
					Subject:  "Subject",
					Body:     "Body",
					Category: "Test",
					Active:   true,
				}
				err := repo.CreateTemplate(ctx, template)
				assert.NoError(t, err)
				done <- true
			}(i)
		}

		// Concurrent reads
		for i := 0; i < 30; i++ {
			go func() {
				_, err := repo.GetActiveTemplates(ctx)
				assert.NoError(t, err)
				done <- true
			}()
		}

		// Wait for all operations
		for i := 0; i < 50; i++ {
			<-done
		}

		// Verify all templates were created
		templates, err := repo.GetActiveTemplates(ctx)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(templates), 20)
	})
}

func TestTemplateVariableSubstitution(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		variables map[string]string
		expected  string
	}{
		{
			name:     "Simple substitution",
			template: "Hello {{name}}, your ticket is {{status}}.",
			variables: map[string]string{
				"{{name}}":   "John",
				"{{status}}": "open",
			},
			expected: "Hello John, your ticket is open.",
		},
		{
			name:     "Multiple occurrences",
			template: "{{user}} reported an issue. Please contact {{user}} at {{email}}.",
			variables: map[string]string{
				"{{user}}":  "Alice",
				"{{email}}": "alice@example.com",
			},
			expected: "Alice reported an issue. Please contact Alice at alice@example.com.",
		},
		{
			name:     "Missing variable",
			template: "Hello {{name}}, your ID is {{id}}.",
			variables: map[string]string{
				"{{name}}": "Bob",
			},
			expected: "Hello Bob, your ID is {{id}}.",
		},
		{
			name:      "No variables",
			template:  "This is a plain text template.",
			variables: map[string]string{},
			expected:  "This is a plain text template.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := substituteVariables(tt.template, tt.variables)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper function for variable substitution.
func substituteVariables(template string, variables map[string]string) string {
	result := template
	for key, value := range variables {
		result = strings.ReplaceAll(result, key, value)
	}
	return result
}
