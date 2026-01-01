// Package memory provides in-memory repository implementations for testing.
package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// GroupRepository provides an in-memory implementation of the GroupRepository interface.
type GroupRepository struct {
	groups map[string]*models.Group
	mu     sync.RWMutex
}

// NewGroupRepository creates a new in-memory group repository.
func NewGroupRepository() *GroupRepository {
	return &GroupRepository{
		groups: make(map[string]*models.Group),
	}
}

// CreateGroup creates a new group.
func (r *GroupRepository) CreateGroup(ctx context.Context, group *models.Group) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Handle ID as interface{} - convert to string for memory storage
	var idStr string
	switch v := group.ID.(type) {
	case string:
		idStr = v
	case int:
		idStr = fmt.Sprintf("%d", v)
	default:
		idStr = fmt.Sprintf("group_%d", len(r.groups)+1)
		group.ID = idStr
	}

	if idStr == "" {
		idStr = fmt.Sprintf("group_%d", len(r.groups)+1)
		group.ID = idStr
	}

	if _, exists := r.groups[idStr]; exists {
		return fmt.Errorf("group already exists")
	}
	r.groups[idStr] = group
	return nil
}

// GetGroup retrieves a group by ID.
func (r *GroupRepository) GetGroup(ctx context.Context, id string) (*models.Group, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	group, exists := r.groups[id]
	if !exists {
		return nil, fmt.Errorf("group not found")
	}
	return group, nil
}

// GetGroupByName retrieves a group by name.
func (r *GroupRepository) GetGroupByName(ctx context.Context, name string) (*models.Group, error) {
	return r.GetByName(ctx, name)
}

// GetByName retrieves a group by name (alias for GetGroupByName).
func (r *GroupRepository) GetByName(ctx context.Context, name string) (*models.Group, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, group := range r.groups {
		if group.Name == name {
			return group, nil
		}
	}
	return nil, fmt.Errorf("group not found")
}

// UpdateGroup updates an existing group.
func (r *GroupRepository) UpdateGroup(ctx context.Context, group *models.Group) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Handle ID as interface{}
	var idStr string
	switch v := group.ID.(type) {
	case string:
		idStr = v
	case int:
		idStr = fmt.Sprintf("%d", v)
	default:
		return fmt.Errorf("invalid group ID type")
	}

	if _, exists := r.groups[idStr]; !exists {
		return fmt.Errorf("group not found")
	}
	r.groups[idStr] = group
	return nil
}

// DeleteGroup deletes a group.
func (r *GroupRepository) DeleteGroup(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.groups[id]; !exists {
		return fmt.Errorf("group not found")
	}
	delete(r.groups, id)
	return nil
}

// ListGroups returns all groups.
func (r *GroupRepository) ListGroups(ctx context.Context) ([]models.Group, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	groups := make([]models.Group, 0, len(r.groups))
	for _, group := range r.groups {
		groups = append(groups, *group)
	}
	return groups, nil
}

// AddUserToGroup adds a user to a group.
func (r *GroupRepository) AddUserToGroup(ctx context.Context, groupID, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	group, exists := r.groups[groupID]
	if !exists {
		return fmt.Errorf("group not found")
	}

	// Check if user is already in group
	for _, member := range group.Members {
		if member == userID {
			return nil // Already a member
		}
	}

	group.Members = append(group.Members, userID)
	return nil
}

// RemoveUserFromGroup removes a user from a group.
func (r *GroupRepository) RemoveUserFromGroup(ctx context.Context, groupID, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	group, exists := r.groups[groupID]
	if !exists {
		return fmt.Errorf("group not found")
	}

	for i, member := range group.Members {
		if member == userID {
			group.Members = append(group.Members[:i], group.Members[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("user not found in group")
}

// GetUserGroups returns all groups for a user.
func (r *GroupRepository) GetUserGroups(ctx context.Context, userID string) ([]models.Group, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var userGroups []models.Group
	for _, group := range r.groups {
		for _, member := range group.Members {
			if member == userID {
				userGroups = append(userGroups, *group)
				break
			}
		}
	}

	return userGroups, nil
}
