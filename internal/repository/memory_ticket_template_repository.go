package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// MemoryTicketTemplateRepository is an in-memory implementation of TicketTemplateRepository.
type MemoryTicketTemplateRepository struct {
	mu        sync.RWMutex
	templates map[uint]*models.TicketTemplate
	nextID    uint
}

// NewMemoryTicketTemplateRepository creates a new in-memory ticket template repository.
func NewMemoryTicketTemplateRepository() *MemoryTicketTemplateRepository {
	return &MemoryTicketTemplateRepository{
		templates: make(map[uint]*models.TicketTemplate),
		nextID:    1,
	}
}

// CreateTemplate creates a new template.
func (r *MemoryTicketTemplateRepository) CreateTemplate(ctx context.Context, template *models.TicketTemplate) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	template.ID = r.nextID
	r.nextID++
	template.CreatedAt = time.Now()
	template.UpdatedAt = template.CreatedAt

	// Create a copy to store
	stored := *template
	r.templates[template.ID] = &stored

	return nil
}

// GetTemplateByID retrieves a template by ID.
func (r *MemoryTicketTemplateRepository) GetTemplateByID(ctx context.Context, id uint) (*models.TicketTemplate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	template, exists := r.templates[id]
	if !exists {
		return nil, fmt.Errorf("template with ID %d not found", id)
	}

	// Return a copy
	result := *template
	return &result, nil
}

// GetActiveTemplates retrieves all active templates.
func (r *MemoryTicketTemplateRepository) GetActiveTemplates(ctx context.Context) ([]models.TicketTemplate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var templates []models.TicketTemplate
	for _, tmpl := range r.templates {
		if tmpl.Active {
			templates = append(templates, *tmpl)
		}
	}

	// Sort by name for consistent ordering
	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Name < templates[j].Name
	})

	return templates, nil
}

// GetTemplatesByCategory retrieves templates by category.
func (r *MemoryTicketTemplateRepository) GetTemplatesByCategory(ctx context.Context, category string) ([]models.TicketTemplate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var templates []models.TicketTemplate
	for _, tmpl := range r.templates {
		if tmpl.Category == category {
			templates = append(templates, *tmpl)
		}
	}

	// Sort by name
	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Name < templates[j].Name
	})

	return templates, nil
}

// UpdateTemplate updates an existing template.
func (r *MemoryTicketTemplateRepository) UpdateTemplate(ctx context.Context, template *models.TicketTemplate) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.templates[template.ID]
	if !exists {
		return fmt.Errorf("template with ID %d not found", template.ID)
	}

	// Preserve creation time
	template.CreatedAt = existing.CreatedAt
	template.UpdatedAt = time.Now()

	// Update with a copy
	updated := *template
	r.templates[template.ID] = &updated

	return nil
}

// DeleteTemplate deletes a template.
func (r *MemoryTicketTemplateRepository) DeleteTemplate(ctx context.Context, id uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.templates[id]; !exists {
		return fmt.Errorf("template with ID %d not found", id)
	}

	delete(r.templates, id)
	return nil
}

// IncrementUsageCount increments the usage count of a template.
func (r *MemoryTicketTemplateRepository) IncrementUsageCount(ctx context.Context, templateID uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	template, exists := r.templates[templateID]
	if !exists {
		return fmt.Errorf("template with ID %d not found", templateID)
	}

	template.UsageCount++
	template.UpdatedAt = time.Now()

	return nil
}

// SearchTemplates searches templates by query string.
func (r *MemoryTicketTemplateRepository) SearchTemplates(ctx context.Context, query string) ([]models.TicketTemplate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query = strings.ToLower(query)
	var results []models.TicketTemplate

	for _, tmpl := range r.templates {
		if !tmpl.Active {
			continue
		}

		// Search in name, subject, body, and tags
		if strings.Contains(strings.ToLower(tmpl.Name), query) ||
			strings.Contains(strings.ToLower(tmpl.Subject), query) ||
			strings.Contains(strings.ToLower(tmpl.Body), query) ||
			strings.Contains(strings.ToLower(tmpl.Description), query) {
			results = append(results, *tmpl)
			continue
		}

		// Search in tags
		for _, tag := range tmpl.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				results = append(results, *tmpl)
				break
			}
		}
	}

	// Sort by usage count (most used first), then by name
	sort.Slice(results, func(i, j int) bool {
		if results[i].UsageCount != results[j].UsageCount {
			return results[i].UsageCount > results[j].UsageCount
		}
		return results[i].Name < results[j].Name
	})

	return results, nil
}

// GetCategories retrieves all unique categories.
func (r *MemoryTicketTemplateRepository) GetCategories(ctx context.Context) ([]models.TemplateCategory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Collect unique categories
	categoryMap := make(map[string]int) // category name -> count
	for _, tmpl := range r.templates {
		if tmpl.Active && tmpl.Category != "" {
			categoryMap[tmpl.Category]++
		}
	}

	// Convert to slice of TemplateCategory
	var categories []models.TemplateCategory
	order := 1
	for name, count := range categoryMap {
		categories = append(categories, models.TemplateCategory{
			ID:          order,
			Name:        name,
			Description: fmt.Sprintf("%d templates", count),
			Order:       order,
			Active:      true,
		})
		order++
	}

	// Sort by name for consistent ordering
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Name < categories[j].Name
	})

	// Re-assign IDs after sorting
	for i := range categories {
		categories[i].ID = i + 1
		categories[i].Order = i + 1
	}

	return categories, nil
}
