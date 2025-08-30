package repository

import (
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"database/sql"
	"fmt"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// GroupSQLRepository handles database operations for groups
type GroupSQLRepository struct {
	db *sql.DB
}

// NewGroupRepository creates a new group repository
func NewGroupRepository(db *sql.DB) *GroupSQLRepository {
	return &GroupSQLRepository{db: db}
}

// List retrieves all groups (both active and inactive)
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

	var groups []*models.Group
	for rows.Next() {
		var group models.Group
		var comments sql.NullString
		err := rows.Scan(
			&group.ID,
			&group.Name,
			&comments,
			&group.ValidID,
			&group.CreateTime,
			&group.CreateBy,
			&group.ChangeTime,
			&group.ChangeBy,
		)
		if err != nil {
			return nil, err
		}
		if comments.Valid {
			group.Comments = comments.String
		}
		groups = append(groups, &group)
	}

	return groups, nil
}

// GetUserGroups retrieves group names for a user
func (r *GroupSQLRepository) GetUserGroups(userID uint) ([]string, error) {
	query := database.ConvertPlaceholders(`
		SELECT g.name 
		FROM groups g
		JOIN group_user ug ON g.id = ug.group_id
		WHERE ug.user_id = $1 AND g.valid_id = 1
		ORDER BY g.name`)

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		groups = append(groups, name)
	}

	return groups, nil
}

// GetByID retrieves a group by ID
func (r *GroupSQLRepository) GetByID(id uint) (*models.Group, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, comments, valid_id, create_time, create_by, change_time, change_by
		FROM groups
		WHERE id = $1`)

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

// AddUserToGroup adds a user to a group
func (r *GroupSQLRepository) AddUserToGroup(userID uint, groupID uint) error {
	// Check if the relationship already exists
	var exists bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM group_user WHERE user_id = $1 AND group_id = $2 AND permission_key = 'rw')`
	err := r.db.QueryRow(checkQuery, userID, groupID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check user-group relationship: %w", err)
	}
	
	if exists {
		return nil // Already a member
	}
	
	// Insert the relationship with required OTRS fields
	insertQuery := database.ConvertPlaceholders(`
		INSERT INTO group_user (user_id, group_id, permission_key, permission_value, create_time, create_by, change_time, change_by) 
		VALUES ($1, $2, 'rw', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)`)
	_, err = r.db.Exec(insertQuery, userID, groupID)
	if err != nil {
		return fmt.Errorf("failed to add user to group: %w", err)
	}
	
	return nil
}

// RemoveUserFromGroup removes a user from a group
func (r *GroupSQLRepository) RemoveUserFromGroup(userID uint, groupID uint) error {
	query := `DELETE FROM group_user WHERE user_id = $1 AND group_id = $2`
	result, err := r.db.Exec(query, userID, groupID)
	if err != nil {
		return fmt.Errorf("failed to remove user from group: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("user not in group")
	}
	
	return nil
}

// Create creates a new group
func (r *GroupSQLRepository) Create(group *models.Group) error {
	query := database.ConvertPlaceholders(`
		INSERT INTO groups (name, comments, valid_id, create_time, create_by, change_time, change_by)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP, $4, CURRENT_TIMESTAMP, $5)
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

// Update updates an existing group
func (r *GroupSQLRepository) Update(group *models.Group) error {
	query := database.ConvertPlaceholders(`
		UPDATE groups 
		SET name = $1, comments = $2, valid_id = $3, change_time = CURRENT_TIMESTAMP, change_by = $4
		WHERE id = $5`)

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

// Delete permanently deletes a group and removes all member associations
func (r *GroupSQLRepository) Delete(id uint) error {
	// First, remove all group members
	_, err := r.db.Exec(database.ConvertPlaceholders(`DELETE FROM group_user WHERE group_id = $1`), id)
	if err != nil {
		return fmt.Errorf("failed to remove group members: %w", err)
	}
	
	// Then delete the group itself
	query := database.ConvertPlaceholders(`DELETE FROM groups WHERE id = $1`)
	
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

// GetByName retrieves a group by name
func (r *GroupSQLRepository) GetByName(name string) (*models.Group, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, comments, valid_id, create_time, create_by, change_time, change_by
		FROM groups
		WHERE name = $1 AND valid_id = 1`)

	var group models.Group
	var comments sql.NullString
	err := r.db.QueryRow(query, name).Scan(
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

// GetGroupMembers retrieves all users in a group
func (r *GroupSQLRepository) GetGroupMembers(groupID uint) ([]*models.User, error) {
	query := database.ConvertPlaceholders(`
		SELECT u.id, u.login, u.first_name, u.last_name, u.valid_id
		FROM users u
		JOIN group_user ug ON u.id = ug.user_id
		WHERE ug.group_id = $1 AND u.valid_id = 1
		ORDER BY u.login`)

	rows, err := r.db.Query(query, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var user models.User
		err := rows.Scan(
			&user.ID,
			&user.Login,
			&user.FirstName,
			&user.LastName,
			&user.ValidID,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, &user)
	}

	return users, nil
}