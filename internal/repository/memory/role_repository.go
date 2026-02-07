package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/goatkit/goatflow/internal/models"
)

// RoleRepository provides an in-memory implementation of the RoleRepository interface.
type RoleRepository struct {
	roles map[string]*models.Role
	mu    sync.RWMutex
}

// NewRoleRepository creates a new in-memory role repository.
func NewRoleRepository() *RoleRepository {
	repo := &RoleRepository{
		roles: make(map[string]*models.Role),
	}

	// Add default roles
	repo.roles["admin"] = &models.Role{
		ID:          "admin",
		Name:        "admin",
		Description: "System Administrator",
		Permissions: []string{"*"},
		IsSystem:    true,
		IsActive:    true,
	}

	repo.roles["agent"] = &models.Role{
		ID:          "agent",
		Name:        "agent",
		Description: "Support Agent",
		Permissions: []string{"view_tickets", "create_tickets", "edit_tickets", "assign_tickets"},
		IsSystem:    true,
		IsActive:    true,
	}

	repo.roles["user"] = &models.Role{
		ID:          "user",
		Name:        "user",
		Description: "Regular User",
		Permissions: []string{"view_tickets", "create_tickets"},
		IsSystem:    true,
		IsActive:    true,
	}

	return repo
}

// CreateRole creates a new role.
func (r *RoleRepository) CreateRole(ctx context.Context, role *models.Role) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.roles[role.ID]; exists {
		return fmt.Errorf("role already exists")
	}
	r.roles[role.ID] = role
	return nil
}

// GetRole retrieves a role by ID.
func (r *RoleRepository) GetRole(ctx context.Context, id string) (*models.Role, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	role, exists := r.roles[id]
	if !exists {
		return nil, fmt.Errorf("role not found")
	}
	return role, nil
}

// GetRoleByName retrieves a role by name.
func (r *RoleRepository) GetRoleByName(ctx context.Context, name string) (*models.Role, error) {
	return r.GetByName(ctx, name)
}

// GetByName retrieves a role by name (alias for GetRoleByName).
func (r *RoleRepository) GetByName(ctx context.Context, name string) (*models.Role, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, role := range r.roles {
		if role.Name == name {
			return role, nil
		}
	}
	return nil, fmt.Errorf("role not found")
}

// UpdateRole updates an existing role.
func (r *RoleRepository) UpdateRole(ctx context.Context, role *models.Role) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.roles[role.ID]; !exists {
		return fmt.Errorf("role not found")
	}
	r.roles[role.ID] = role
	return nil
}

// DeleteRole deletes a role.
func (r *RoleRepository) DeleteRole(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.roles[id]; !exists {
		return fmt.Errorf("role not found")
	}
	delete(r.roles, id)
	return nil
}

// ListRoles returns all roles.
func (r *RoleRepository) ListRoles(ctx context.Context) ([]models.Role, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	roles := make([]models.Role, 0, len(r.roles))
	for _, role := range r.roles {
		roles = append(roles, *role)
	}
	return roles, nil
}
