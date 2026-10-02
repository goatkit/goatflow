package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// PermissionKey represents the OTRS permission types.
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

// UserGroupPermission represents a permission entry.
type UserGroupPermission struct {
	UserID          uint
	GroupID         uint
	PermissionKey   string
	PermissionValue int
}

// PermissionRepository handles database operations for permissions.
type PermissionRepository struct {
	db *sql.DB
}

// NewPermissionRepository creates a new permission repository.
func NewPermissionRepository(db *sql.DB) *PermissionRepository {
	return &PermissionRepository{db: db}
}

// GetUserPermissions retrieves all permissions for a user.
func (r *PermissionRepository) GetUserPermissions(userID uint) (map[uint][]string, error) {
	query := database.ConvertPlaceholders(`
		SELECT group_id, permission_key 
		FROM group_user 
		WHERE user_id = ?
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

// GetGroupPermissions retrieves all users and their permissions for a group.
func (r *PermissionRepository) GetGroupPermissions(groupID uint) (map[uint][]string, error) {
	query := database.ConvertPlaceholders(`
		SELECT user_id, permission_key 
		FROM group_user 
		WHERE group_id = ?
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

// SetUserGroupPermission sets or updates a permission.
func (r *PermissionRepository) SetUserGroupPermission(userID, groupID uint, permKey string, value int) error {
	// In OTRS schema, presence of a row means the permission is granted
	// If value is 0, we delete the row; if value is 1, we ensure the row exists
	if value == 0 {
		// Remove permission by deleting the row
		deleteQuery := database.ConvertPlaceholders(`
			DELETE FROM group_user 
			WHERE user_id = ? AND group_id = ? AND permission_key = ?`)

		_, err := r.db.Exec(deleteQuery, userID, groupID, permKey)
		if err != nil {
			return fmt.Errorf("failed to remove permission: %w", err)
		}
		return nil
	}

	// Grant the permission, refreshing the audit fields when it is already
	// granted. group_user has no unique key, so check for the row first.
	var exists bool
	err := r.db.QueryRow(database.ConvertPlaceholders(`
		SELECT EXISTS(SELECT 1 FROM group_user WHERE user_id = ? AND group_id = ? AND permission_key = ?)`),
		userID, groupID, permKey).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check permission: %w", err)
	}

	if exists {
		_, err = r.db.Exec(database.ConvertPlaceholders(`
			UPDATE group_user SET change_time = NOW(), change_by = ?
			WHERE user_id = ? AND group_id = ? AND permission_key = ?`),
			userID, userID, groupID, permKey)
	} else {
		_, err = r.db.Exec(database.ConvertPlaceholders(`
			INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, NOW(), ?, NOW(), ?)`),
			userID, groupID, permKey, userID, userID)
	}
	if err != nil {
		return fmt.Errorf("failed to set permission: %w", err)
	}

	return nil
}

// RemoveUserGroupPermission removes a specific permission.
func (r *PermissionRepository) RemoveUserGroupPermission(userID, groupID uint, permKey string) error {
	// OTRS doesn't delete, it sets permission_value to 0
	return r.SetUserGroupPermission(userID, groupID, permKey, 0)
}

// GetUserGroupMatrix gets all permissions for a specific user-group combination.
func (r *PermissionRepository) GetUserGroupMatrix(userID, groupID uint) (map[string]bool, error) {
	query := database.ConvertPlaceholders(`
		SELECT permission_key
		FROM group_user 
		WHERE user_id = ? AND group_id = ?`)

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
		if err := rows.Scan(&permKey); err != nil {
			return nil, fmt.Errorf("failed to scan matrix: %w", err)
		}
		matrix[permKey] = true // Presence of row means permission is granted
	}

	return matrix, rows.Err()
}

// SetUserGroupMatrix sets all permissions for a user-group combination.
func (r *PermissionRepository) SetUserGroupMatrix(userID, groupID uint, permissions map[string]bool) error {
	// Start transaction for atomic update
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Delete existing permissions
	deleteQuery := database.ConvertPlaceholders(`DELETE FROM group_user WHERE user_id = ? AND group_id = ?`)
	_, err = tx.Exec(deleteQuery, userID, groupID)
	if err != nil {
		return fmt.Errorf("failed to delete existing permissions: %w", err)
	}

	// Insert new permissions
	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, NOW(), ?, NOW(), ?)`)

	for permKey, enabled := range permissions {
		if enabled {
			_, err = tx.Exec(insertQuery, userID, groupID, permKey, 1, 1) // Using system user (1) for audit fields
			if err != nil {
				return fmt.Errorf("failed to insert permission %s: %w", permKey, err)
			}
		}
		// If not enabled, we don't insert anything (absence means no permission)
	}

	return tx.Commit()
}

// ErrPermissionUserNotFound is returned when a user referenced by a permission
// operation does not exist.
var ErrPermissionUserNotFound = errors.New("user not found")

// ReplaceUserPermissionsFrom makes the target user's group permissions an exact
// copy of the source user's: every group_user row of the target is removed and
// each row of the source is recreated for the target, in one transaction.
// changeBy is recorded in the audit columns of the new rows. Both users must
// exist (ErrPermissionUserNotFound otherwise).
func (r *PermissionRepository) ReplaceUserPermissionsFrom(sourceUserID, targetUserID, changeBy uint) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, id := range []uint{sourceUserID, targetUserID} {
		var exists bool
		if err := tx.QueryRow(database.ConvertPlaceholders(
			`SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)`), id).Scan(&exists); err != nil {
			return fmt.Errorf("failed to check user %d: %w", id, err)
		}
		if !exists {
			return fmt.Errorf("%w: %d", ErrPermissionUserNotFound, id)
		}
	}

	rows, err := tx.Query(database.ConvertPlaceholders(`
		SELECT group_id, permission_key FROM group_user WHERE user_id = ?`), sourceUserID)
	if err != nil {
		return fmt.Errorf("failed to read source permissions: %w", err)
	}
	type grant struct {
		groupID uint
		key     string
	}
	var grants []grant
	for rows.Next() {
		var g grant
		if err := rows.Scan(&g.groupID, &g.key); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan source permission: %w", err)
		}
		grants = append(grants, g)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to read source permissions: %w", err)
	}

	if _, err := tx.Exec(database.ConvertPlaceholders(
		`DELETE FROM group_user WHERE user_id = ?`), targetUserID); err != nil {
		return fmt.Errorf("failed to clear target permissions: %w", err)
	}

	insert := database.ConvertPlaceholders(`
		INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, ?)`)
	for _, g := range grants {
		if _, err := tx.Exec(insert, targetUserID, g.groupID, g.key, changeBy, changeBy); err != nil {
			return fmt.Errorf("failed to copy permission %s for group %d: %w", g.key, g.groupID, err)
		}
	}

	return tx.Commit()
}

// GetAllUserGroupPermissions gets complete permission matrix for all users and groups.
func (r *PermissionRepository) GetAllUserGroupPermissions() ([]UserGroupPermission, error) {
	query := database.ConvertPlaceholders(`
		SELECT ug.user_id, ug.group_id, ug.permission_key
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
		if err := rows.Scan(&perm.UserID, &perm.GroupID, &perm.PermissionKey); err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		perm.PermissionValue = 1 // In OTRS schema, presence of row means permission is granted
		permissions = append(permissions, perm)
	}

	return permissions, rows.Err()
}
