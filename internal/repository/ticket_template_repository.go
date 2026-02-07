package repository

import (
	"context"

	"github.com/goatkit/goatflow/internal/models"
)

// TicketTemplateRepository defines the interface for ticket template persistence.
type TicketTemplateRepository interface {
	// Template CRUD operations
	CreateTemplate(ctx context.Context, template *models.TicketTemplate) error
	GetTemplateByID(ctx context.Context, id uint) (*models.TicketTemplate, error)
	GetActiveTemplates(ctx context.Context) ([]models.TicketTemplate, error)
	GetTemplatesByCategory(ctx context.Context, category string) ([]models.TicketTemplate, error)
	UpdateTemplate(ctx context.Context, template *models.TicketTemplate) error
	DeleteTemplate(ctx context.Context, id uint) error

	// Usage tracking
	IncrementUsageCount(ctx context.Context, templateID uint) error

	// Search and filtering
	SearchTemplates(ctx context.Context, query string) ([]models.TicketTemplate, error)

	// Categories
	GetCategories(ctx context.Context) ([]models.TemplateCategory, error)
}
