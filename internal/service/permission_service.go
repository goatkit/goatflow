package service

import (
	"database/sql"
	"fmt"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/repository"
)

// PermissionService handles business logic for permissions.
type PermissionService struct {
	permRepo  *repository.PermissionRepository
	userRepo  *repository.UserRepository
	groupRepo *repository.GroupSQLRepository
}

// NewPermissionService creates a new permission service.
func NewPermissionService(db *sql.DB) *PermissionService {
	return &PermissionService{
		permRepo:  repository.NewPermissionRepository(db),
		userRepo:  repository.NewUserRepository(db),
		groupRepo: repository.NewGroupRepository(db),
	}
}

// PermissionMatrix represents a user's permissions across all groups.
type PermissionMatrix struct {
	User   *models.User
	Groups []*GroupPermissions
}

// GroupPermissions represents permissions for a single group.
type GroupPermissions struct {
	Group       *models.Group
	Permissions map[string]bool
}

// GetUserPermissionMatrix gets complete permission matrix for a user.
func (s *PermissionService) GetUserPermissionMatrix(userID uint) (*PermissionMatrix, error) {
	// Get user
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Get all groups
	groups, err := s.groupRepo.List()
	if err != nil {
		return nil, fmt.Errorf("failed to get groups: %w", err)
	}

	matrix := &PermissionMatrix{
		User:   user,
		Groups: make([]*GroupPermissions, 0, len(groups)),
	}

	// Get permissions for each group
	for _, group := range groups {
		// Convert group.ID to uint
		var groupID uint
		switch v := group.ID.(type) {
		case int:
			groupID = uint(v)
		case uint:
			groupID = v
		case int64:
			groupID = uint(v)
		case string:
			// Skip string IDs (LDAP groups)
			continue
		default:
			continue
		}

		perms, err := s.permRepo.GetUserGroupMatrix(userID, groupID)
		if err != nil {
			return nil, fmt.Errorf("failed to get permissions for group %d: %w", groupID, err)
		}

		matrix.Groups = append(matrix.Groups, &GroupPermissions{
			Group:       group,
			Permissions: perms,
		})
	}

	return matrix, nil
}

// UpdateUserPermissions updates all permissions for a user.
func (s *PermissionService) UpdateUserPermissions(userID uint, permissions map[uint]map[string]bool) error {
	// Validate user exists
	_, err := s.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Update permissions for each group that was sent
	for groupID, perms := range permissions {
		// Validate group exists
		_, err := s.groupRepo.GetByID(groupID)
		if err != nil {
			return fmt.Errorf("group %d not found: %w", groupID, err)
		}

		// Debug: log permissions before applying rules
		fmt.Printf("DEBUG: Before rules for group %d: %+v\n", groupID, perms)

		// Apply permission rules
		perms = s.applyPermissionRules(perms)

		// Debug: log permissions after applying rules
		fmt.Printf("DEBUG: After rules for group %d: %+v\n", groupID, perms)

		// Update permissions
		err = s.permRepo.SetUserGroupMatrix(userID, groupID, perms)
		if err != nil {
			return fmt.Errorf("failed to update permissions for group %d: %w", groupID, err)
		}
	}

	return nil
}

// applyPermissionRules applies business rules to permissions.
func (s *PermissionService) applyPermissionRules(perms map[string]bool) map[string]bool {
	// If RW is set, all other permissions should be true
	if perms["rw"] {
		perms["ro"] = true
		perms["move_into"] = true
		perms["create"] = true
		perms["note"] = true
		perms["owner"] = true
		perms["priority"] = true
	}

	// If user can create, they should also be able to read
	if perms["create"] {
		perms["ro"] = true
	}

	// If user can be owner, they need read access
	if perms["owner"] {
		perms["ro"] = true
	}

	return perms
}

// GroupUserMatrix represents permissions for all users in a group.
type GroupUserMatrix struct {
	Group *models.Group
	Users []*UserPermissions
}

// UserPermissions represents a user's permissions in a group.
type UserPermissions struct {
	User        *models.User
	Permissions map[string]bool
}

// GetGroupPermissionMatrix gets all users and their permissions for a group.
func (s *PermissionService) GetGroupPermissionMatrix(groupID uint) (*GroupUserMatrix, error) {
	// Get group
	group, err := s.groupRepo.GetByID(groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group: %w", err)
	}

	// Get all users
	users, err := s.userRepo.List()
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}

	matrix := &GroupUserMatrix{
		Group: group,
		Users: make([]*UserPermissions, 0, len(users)),
	}

	// Get permissions for each user
	for _, user := range users {
		perms, err := s.permRepo.GetUserGroupMatrix(user.ID, groupID)
		if err != nil {
			return nil, fmt.Errorf("failed to get permissions for user %d: %w", user.ID, err)
		}

		// Only include users with at least one permission
		hasPermission := false
		for _, enabled := range perms {
			if enabled {
				hasPermission = true
				break
			}
		}

		if hasPermission || user.ID == 1 { // Always show admin user
			matrix.Users = append(matrix.Users, &UserPermissions{
				User:        user,
				Permissions: perms,
			})
		}
	}

	return matrix, nil
}

// CloneUserPermissions copies all permissions from one user to another.
func (s *PermissionService) CloneUserPermissions(sourceUserID, targetUserID uint) error {
	// Get source user permissions
	permissions, err := s.permRepo.GetUserPermissions(sourceUserID)
	if err != nil {
		return fmt.Errorf("failed to get source permissions: %w", err)
	}

	// Apply permissions to target user
	for groupID, perms := range permissions {
		for _, permKey := range perms {
			err = s.permRepo.SetUserGroupPermission(targetUserID, groupID, permKey, 1)
			if err != nil {
				return fmt.Errorf("failed to set permission %s for group %d: %w", permKey, groupID, err)
			}
		}
	}

	return nil
}

// RemoveAllUserPermissions removes all permissions for a user.
func (s *PermissionService) RemoveAllUserPermissions(userID uint) error {
	// Get all groups
	groups, err := s.groupRepo.List()
	if err != nil {
		return fmt.Errorf("failed to get groups: %w", err)
	}

	// Remove permissions for each group
	for _, group := range groups {
		// Convert group.ID to uint
		var groupID uint
		switch v := group.ID.(type) {
		case int:
			groupID = uint(v)
		case uint:
			groupID = v
		case string:
			// Skip string IDs (LDAP groups)
			continue
		default:
			continue
		}

		for _, permKey := range []string{"ro", "move_into", "create", "note", "owner", "priority", "rw"} {
			err = s.permRepo.RemoveUserGroupPermission(userID, groupID, permKey)
			if err != nil {
				return fmt.Errorf("failed to remove permission %s for group %d: %w", permKey, groupID, err)
			}
		}
	}

	return nil
}

// ValidatePermissions checks if a user has specific permissions for a group.
func (s *PermissionService) ValidatePermissions(userID, groupID uint, requiredPerms []string) (bool, error) {
	perms, err := s.permRepo.GetUserGroupMatrix(userID, groupID)
	if err != nil {
		return false, fmt.Errorf("failed to get permissions: %w", err)
	}

	// Check if user has all required permissions
	for _, reqPerm := range requiredPerms {
		if !perms[reqPerm] {
			return false, nil
		}
	}

	return true, nil
}

// GetEffectivePermissions calculates effective permissions considering inheritance.
func (s *PermissionService) GetEffectivePermissions(userID, groupID uint) (map[string]bool, error) {
	perms, err := s.permRepo.GetUserGroupMatrix(userID, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get permissions: %w", err)
	}

	// Apply permission rules to get effective permissions
	return s.applyPermissionRules(perms), nil
}
