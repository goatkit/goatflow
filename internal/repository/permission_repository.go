package repository

import (
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"database/sql"
	"fmt"
)

// PermissionKey represents the OTRS permission types
type PermissionKey string

const (
	PermissionRO       PermissionKey = "ro"        // Read Only
	PermissionMoveInto PermissionKey = "move_into" // Move tickets into queue
	PermissionCreate   PermissionKey = "create"    // Create tickets in queue
	PermissionNote     PermissionKey = "note"      // Add notes to tickets
	PermissionOwner    PermissionKey = "owner"     // Become ticket owner
	PermissionPriority PermissionKey = "priority"  // Change ticket priority
	PermissionRW       PermissionKey = "rw"        // Read/Write (full access)
)

// UserGroupPermission represents a permission entry
type UserGroupPermission struct {
	UserID          uint
	GroupID         uint
	PermissionKey   string
	PermissionValue int
}

// PermissionRepository handles database operations for permissions
type PermissionRepository struct {
	db *sql.DB
}

// NewPermissionRepository creates a new permission repository
func NewPermissionRepository(db *sql.DB) *PermissionRepository {
	return &PermissionRepository{db: db}
}

// GetUserPermissions retrieves all permissions for a user
func (r *PermissionRepository) GetUserPermissions(userID uint) (map[uint][]string, error) {
	query := database.ConvertPlaceholders(`
		SELECT group_id, permission_key 
		FROM group_user 
		WHERE user_id = $1 AND permission_value = 1
		ORDER BY group_id, permission_key`)

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user permissions: %w", err)
	}
	defer rows.Close()

	permissions := make(map[uint][]string)
	for rows.Next() {
		var groupID uint
		var permKey string
		if err := rows.Scan(&groupID, &permKey); err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		permissions[groupID] = append(permissions[groupID], permKey)
	}

	return permissions, rows.Err()
}

// GetGroupPermissions retrieves all users and their permissions for a group
func (r *PermissionRepository) GetGroupPermissions(groupID uint) (map[uint][]string, error) {
	query := database.ConvertPlaceholders(`
		SELECT user_id, permission_key 
		FROM group_user 
		WHERE group_id = $1 AND permission_value = 1
		ORDER BY user_id, permission_key`)

	rows, err := r.db.Query(query, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group permissions: %w", err)
	}
	defer rows.Close()

	permissions := make(map[uint][]string)
	for rows.Next() {
		var userID uint
		var permKey string
		if err := rows.Scan(&userID, &permKey); err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		permissions[userID] = append(permissions[userID], permKey)
	}

	return permissions, rows.Err()
}

// SetUserGroupPermission sets or updates a permission
func (r *PermissionRepository) SetUserGroupPermission(userID, groupID uint, permKey string, value int) error {
	// First try to update existing permission
	updateQuery := database.ConvertPlaceholders(`
		UPDATE group_user 
		SET permission_value = $4, change_time = CURRENT_TIMESTAMP, change_by = $5
		WHERE user_id = $1 AND group_id = $2 AND permission_key = $3`)

	result, err := r.db.Exec(updateQuery, userID, groupID, permKey, value, userID)
	if err != nil {
		return fmt.Errorf("failed to update permission: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	// If no rows were updated, insert new permission
	if rowsAffected == 0 {
		insertQuery := database.ConvertPlaceholders(`
			INSERT INTO group_user (user_id, group_id, permission_key, permission_value, create_by, change_by)
			VALUES ($1, $2, $3, $4, $5, $6)`)

		_, err = r.db.Exec(insertQuery, userID, groupID, permKey, value, userID, userID)
		if err != nil {
			return fmt.Errorf("failed to insert permission: %w", err)
		}
	}

	return nil
}

// RemoveUserGroupPermission removes a specific permission
func (r *PermissionRepository) RemoveUserGroupPermission(userID, groupID uint, permKey string) error {
	// OTRS doesn't delete, it sets permission_value to 0
	return r.SetUserGroupPermission(userID, groupID, permKey, 0)
}

// GetUserGroupMatrix gets all permissions for a specific user-group combination
func (r *PermissionRepository) GetUserGroupMatrix(userID, groupID uint) (map[string]bool, error) {
	query := database.ConvertPlaceholders(`
		SELECT permission_key, permission_value 
		FROM group_user 
		WHERE user_id = $1 AND group_id = $2`)

	rows, err := r.db.Query(query, userID, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user-group matrix: %w", err)
	}
	defer rows.Close()

	matrix := make(map[string]bool)
	// Initialize all permissions to false
	for _, key := range []PermissionKey{PermissionRO, PermissionMoveInto, PermissionCreate, PermissionNote, PermissionOwner, PermissionPriority, PermissionRW} {
		matrix[string(key)] = false
	}

	for rows.Next() {
		var permKey string
		var permValue int
		if err := rows.Scan(&permKey, &permValue); err != nil {
			return nil, fmt.Errorf("failed to scan matrix: %w", err)
		}
		matrix[permKey] = (permValue == 1)
	}

	return matrix, rows.Err()
}

// SetUserGroupMatrix sets all permissions for a user-group combination
func (r *PermissionRepository) SetUserGroupMatrix(userID, groupID uint, permissions map[string]bool) error {
	// Start transaction for atomic update
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete existing permissions
	deleteQuery := `DELETE FROM group_user WHERE user_id = $1 AND group_id = $2`
	_, err = tx.Exec(deleteQuery, userID, groupID)
	if err != nil {
		return fmt.Errorf("failed to delete existing permissions: %w", err)
	}

	// Insert new permissions
	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO group_user (user_id, group_id, permission_key, permission_value, create_time, create_by, change_time, change_by)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP, $5, CURRENT_TIMESTAMP, $6`)

	stmt, err := tx.Prepare(insertQuery)
	if err != nil {
		return fmt.Errorf("failed to prepare insert: %w", err)
	}
	defer stmt.Close()

	for permKey, enabled := range permissions {
		value := 0
		if enabled {
			value = 1
		}
		_, err = stmt.Exec(userID, groupID, permKey, value, 1, 1) // Using system user (1) for audit fields
		if err != nil {
			return fmt.Errorf("failed to insert permission %s: %w", permKey, err)
		}
	}

	return tx.Commit()
}

// GetAllUserGroupPermissions gets complete permission matrix for all users and groups
func (r *PermissionRepository) GetAllUserGroupPermissions() ([]UserGroupPermission, error) {
	query := database.ConvertPlaceholders(`
		SELECT ug.user_id, ug.group_id, ug.permission_key, ug.permission_value
		FROM group_user ug
		JOIN users u ON ug.user_id = u.id
		JOIN groups g ON ug.group_id = g.id
		WHERE u.valid_id = 1 AND g.valid_id = 1
		ORDER BY ug.user_id, ug.group_id, ug.permission_key`)

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all permissions: %w", err)
	}
	defer rows.Close()

	var permissions []UserGroupPermission
	for rows.Next() {
		var perm UserGroupPermission
		if err := rows.Scan(&perm.UserID, &perm.GroupID, &perm.PermissionKey, &perm.PermissionValue); err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		permissions = append(permissions, perm)
	}

	return permissions, rows.Err()
}