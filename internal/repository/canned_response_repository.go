package repository

import (
	"context"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// CannedResponseRepository defines the interface for canned response persistence
type CannedResponseRepository interface {
	// Response CRUD operations
	CreateResponse(ctx context.Context, response *models.CannedResponse) error
	GetResponseByID(ctx context.Context, id uint) (*models.CannedResponse, error)
	GetResponseByShortcut(ctx context.Context, shortcut string) (*models.CannedResponse, error)
	GetActiveResponses(ctx context.Context) ([]models.CannedResponse, error)
	GetResponsesByCategory(ctx context.Context, category string) ([]models.CannedResponse, error)
	GetResponsesForUser(ctx context.Context, userID uint) ([]models.CannedResponse, error)
	UpdateResponse(ctx context.Context, response *models.CannedResponse) error
	DeleteResponse(ctx context.Context, id uint) error

	// Usage tracking
	IncrementUsageCount(ctx context.Context, responseID uint) error
	RecordUsage(ctx context.Context, usage *models.CannedResponseUsage) error
	GetUsageHistory(ctx context.Context, responseID uint, limit int) ([]models.CannedResponseUsage, error)

	// Search and filtering
	SearchResponses(ctx context.Context, filter *models.CannedResponseFilter) ([]models.CannedResponse, error)
	GetMostUsedResponses(ctx context.Context, limit int) ([]models.CannedResponse, error)

	// Categories
	GetCategories(ctx context.Context) ([]models.CannedResponseCategory, error)
}