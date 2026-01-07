package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// GroupSQLRepository handles database operations for groups.
type GroupSQLRepository struct {
	db *sql.DB
}

// NewGroupRepository creates a new group repository.
func NewGroupRepository(db *sql.DB) *GroupSQLRepository {
	return &GroupSQLRepository{db: db}
}

// List retrieves all groups (both active and inactive).
func (r *GroupSQLRepository) List() ([]*models.Group, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, comments, valid_id, create_time, create_by, change_time, change_by
		FROM groups
		ORDER BY name`)

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups, err := database.CollectRows(rows, func(r *sql.Rows) (*models.Group, error) {
		var group models.Group
		var comments sql.NullString
		if err := r.Scan(
			&group.ID,
			&group.Name,
			&comments,
			&group.ValidID,
			&group.CreateTime,
			&group.CreateBy,
			&group.ChangeTime,
			&group.ChangeBy,
		); err != nil {
			return nil, err
		}
		if comments.Valid {
			group.Comments = comments.String
		}
		return &group, nil
	})
	return groups, err
}

// GetUserGroups retrieves group names for a user.
func (r *GroupSQLRepository) GetUserGroups(userID uint) ([]string, error) {
	query := database.ConvertPlaceholders(`
		SELECT g.name 
		FROM groups g
		JOIN group_user ug ON g.id = ug.group_id
		WHERE ug.user_id = ? AND g.valid_id = 1
		ORDER BY g.name`)

	rows, err := r.db.Query(query, userID) //nolint:rowserrcheck // rows.Err checked inside CollectStrings
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return database.CollectStrings(rows)
}

// GetByID retrieves a group by ID.
func (r *GroupSQLRepository) GetByID(id uint) (*models.Group, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, comments, valid_id, create_time, create_by, change_time, change_by
		FROM groups
		WHERE id = ?`)

	var group models.Group
	var comments sql.NullString
	err := r.db.QueryRow(query, id).Scan(
		&group.ID,
		&group.Name,
		&comments,
		&group.ValidID,
		&group.CreateTime,
		&group.CreateBy,
		&group.ChangeTime,
		&group.ChangeBy,
	)

	if comments.Valid {
		group.Comments = comments.String
	}

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("group not found")
	}

	return &group, err
}

// AddUserToGroup adds a user to a group.
func (r *GroupSQLRepository) AddUserToGroup(userID uint, groupID uint) error {
	// Check if the relationship already exists
	var exists bool
	checkQuery := database.ConvertPlaceholders(`SELECT EXISTS(SELECT 1 FROM group_user WHERE user_id = ? AND group_id = ? AND permission_key = 'rw')`)
	err := r.db.QueryRow(checkQuery, userID, groupID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check user-group relationship: %w", err)
	}

	if exists {
		return nil // Already a member
	}

	// Insert the relationship with required OTRS fields
	insertQuery := database.ConvertPlaceholders(`
        INSERT INTO group_user (user_id, group_id, permission_key, create_time, create_by, change_time, change_by) 
        VALUES (?, ?, 'rw', CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)`)
	_, err = r.db.Exec(insertQuery, userID, groupID)
	if err != nil {
		return fmt.Errorf("failed to add user to group: %w", err)
	}

	return nil
}

// RemoveUserFromGroup removes a user from a group.
func (r *GroupSQLRepository) RemoveUserFromGroup(userID uint, groupID uint) error {
	query := database.ConvertPlaceholders(`DELETE FROM group_user WHERE user_id = ? AND group_id = ?`)
	result, err := r.db.Exec(query, userID, groupID)
	if err != nil {
		return fmt.Errorf("failed to remove user from group: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user-group relationship not found")
	}

	return nil
}

// Create creates a new group.
func (r *GroupSQLRepository) Create(group *models.Group) error {
	if group == nil {
		return errors.New("group is required")
	}
	if strings.TrimSpace(group.Name) == "" {
		return errors.New("group name is required")
	}

	query := database.ConvertPlaceholders(`
		INSERT INTO groups (name, comments, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, ?, CURRENT_TIMESTAMP, ?)
		RETURNING id, create_time, change_time`)

	err := r.db.QueryRow(
		query,
		group.Name,
		group.Comments,
		group.ValidID,
		group.CreateBy,
		group.ChangeBy,
	).Scan(&group.ID, &group.CreateTime, &group.ChangeTime)

	if err != nil {
		return fmt.Errorf("failed to create group: %w", err)
	}

	return nil
}

// Update updates an existing group.
func (r *GroupSQLRepository) Update(group *models.Group) error {
	query := database.ConvertPlaceholders(`
		UPDATE groups 
		SET name = ?, comments = ?, valid_id = ?, change_time = CURRENT_TIMESTAMP, change_by = ?
		WHERE id = ?`)

	result, err := r.db.Exec(
		query,
		group.Name,
		group.Comments,
		group.ValidID,
		group.ChangeBy,
		group.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update group: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("group not found")
	}

	return nil
}

// Delete permanently deletes a group and removes all member associations.
func (r *GroupSQLRepository) Delete(id uint) error {
	// First, remove all group members
	_, err := r.db.Exec(database.ConvertPlaceholders(`DELETE FROM group_user WHERE group_id = ?`), id)
	if err != nil {
		return fmt.Errorf("failed to remove group members: %w", err)
	}

	// Then delete the group itself
	query := database.ConvertPlaceholders(`DELETE FROM groups WHERE id = ?`)

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("group not found")
	}

	return nil
}

// GetByName retrieves a group by name.
func (r *GroupSQLRepository) GetByName(name string) (*models.Group, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil, errors.New("group name is required")
	}

	baseQuery := `
		SELECT id, name, comments, valid_id, create_time, create_by, change_time, change_by
		FROM groups
		WHERE name = ? AND valid_id = 1`
	if database.IsMySQL() {
		baseQuery = `
		SELECT id, name, comments, valid_id, create_time, create_by, change_time, change_by
		FROM groups
		WHERE BINARY name = ? AND valid_id = 1`
	}
	query := database.ConvertPlaceholders(baseQuery)

	var group models.Group
	var comments sql.NullString
	err := r.db.QueryRow(query, trimmed).Scan(
		&group.ID,
		&group.Name,
		&comments,
		&group.ValidID,
		&group.CreateTime,
		&group.CreateBy,
		&group.ChangeTime,
		&group.ChangeBy,
	)

	if comments.Valid {
		group.Comments = comments.String
	}

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("group not found")
	}

	return &group, err
}

// GetGroupMembers retrieves all users in a group.
func (r *GroupSQLRepository) GetGroupMembers(groupID uint) ([]*models.User, error) {
	query := database.ConvertPlaceholders(`
		SELECT u.id, u.login, u.first_name, u.last_name, u.valid_id
		FROM users u
		JOIN group_user ug ON u.id = ug.user_id
		WHERE ug.group_id = ? AND u.valid_id = 1
		ORDER BY u.login`)

	rows, err := r.db.Query(query, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return database.CollectRows(rows, func(r *sql.Rows) (*models.User, error) {
		var user models.User
		if err := r.Scan(
			&user.ID,
			&user.Login,
			&user.FirstName,
			&user.LastName,
			&user.ValidID,
		); err != nil {
			return nil, err
		}
		return &user, nil
	})
}
