package repository

import (
	"context"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// GroupRepository defines the interface for group operations.
type GroupRepository interface {
	CreateGroup(ctx context.Context, group *models.Group) error
	GetGroup(ctx context.Context, id string) (*models.Group, error)
	GetGroupByName(ctx context.Context, name string) (*models.Group, error)
	GetByName(ctx context.Context, name string) (*models.Group, error)
	UpdateGroup(ctx context.Context, group *models.Group) error
	DeleteGroup(ctx context.Context, id string) error
	ListGroups(ctx context.Context) ([]models.Group, error)
	AddUserToGroup(ctx context.Context, groupID, userID string) error
	RemoveUserFromGroup(ctx context.Context, groupID, userID string) error
	GetUserGroups(ctx context.Context, userID string) ([]models.Group, error)
}
