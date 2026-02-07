package repository

import (
	"context"

	"github.com/goatkit/goatflow/internal/models"
)

// RoleRepository defines the interface for role operations.
type RoleRepository interface {
	CreateRole(ctx context.Context, role *models.Role) error
	GetRole(ctx context.Context, id string) (*models.Role, error)
	GetRoleByName(ctx context.Context, name string) (*models.Role, error)
	GetByName(ctx context.Context, name string) (*models.Role, error)
	UpdateRole(ctx context.Context, role *models.Role) error
	DeleteRole(ctx context.Context, id string) error
	ListRoles(ctx context.Context) ([]models.Role, error)
}
