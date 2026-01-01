package repository

import (
	"context"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// LookupRepository defines the interface for lookup data persistence.
type LookupRepository interface {
	// Queue operations
	GetQueues(ctx context.Context) ([]models.QueueInfo, error)
	GetQueueByID(ctx context.Context, id int) (*models.QueueInfo, error)
	CreateQueue(ctx context.Context, queue *models.QueueInfo) error
	UpdateQueue(ctx context.Context, queue *models.QueueInfo) error
	DeleteQueue(ctx context.Context, id int) error

	// Priority operations
	GetPriorities(ctx context.Context) ([]models.LookupItem, error)
	GetPriorityByID(ctx context.Context, id int) (*models.LookupItem, error)
	UpdatePriority(ctx context.Context, priority *models.LookupItem) error

	// Type operations
	GetTypes(ctx context.Context) ([]models.LookupItem, error)
	GetTypeByID(ctx context.Context, id int) (*models.LookupItem, error)
	CreateType(ctx context.Context, typ *models.LookupItem) error
	UpdateType(ctx context.Context, typ *models.LookupItem) error
	DeleteType(ctx context.Context, id int) error

	// Status operations
	GetStatuses(ctx context.Context) ([]models.LookupItem, error)
	GetStatusByID(ctx context.Context, id int) (*models.LookupItem, error)
	UpdateStatus(ctx context.Context, status *models.LookupItem) error

	// Audit operations
	LogChange(ctx context.Context, change *LookupAuditLog) error
	GetAuditLogs(ctx context.Context, entityType string, entityID int, limit int) ([]LookupAuditLog, error)

	// Export/Import operations
	ExportConfiguration(ctx context.Context) (*LookupConfiguration, error)
	ImportConfiguration(ctx context.Context, config *LookupConfiguration) error
}

// LookupAuditLog represents an audit log entry for lookup changes.
type LookupAuditLog struct {
	ID         int       `json:"id"`
	EntityType string    `json:"entity_type"` // queue, priority, type, status
	EntityID   int       `json:"entity_id"`
	Action     string    `json:"action"` // create, update, delete
	OldValue   string    `json:"old_value"`
	NewValue   string    `json:"new_value"`
	UserID     int       `json:"user_id"`
	UserEmail  string    `json:"user_email"`
	Timestamp  time.Time `json:"timestamp"`
	IPAddress  string    `json:"ip_address"`
}

// LookupConfiguration represents exportable/importable configuration.
type LookupConfiguration struct {
	Version    string              `json:"version"`
	ExportedAt time.Time           `json:"exported_at"`
	ExportedBy string              `json:"exported_by"`
	Queues     []models.QueueInfo  `json:"queues"`
	Priorities []models.LookupItem `json:"priorities"`
	Types      []models.LookupItem `json:"types"`
	Statuses   []models.LookupItem `json:"statuses"`
}
