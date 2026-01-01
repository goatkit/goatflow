package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// DBRoleRepository handles database operations for roles.
type DBRoleRepository struct {
	db *sql.DB
}

// NewDBRoleRepository creates a new database role repository.
func NewDBRoleRepository(db *sql.DB) *DBRoleRepository {
	return &DBRoleRepository{db: db}
}

// List returns all roles.
func (r *DBRoleRepository) List() ([]*models.DBRole, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, comments, valid_id, create_time, create_by, change_time, change_by
		FROM roles
		ORDER BY name
	`)

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}
	defer rows.Close()

	var roles []*models.DBRole
	for rows.Next() {
		role := &models.DBRole{}
		var comments sql.NullString
		err := rows.Scan(
			&role.ID,
			&role.Name,
			&comments,
			&role.ValidID,
			&role.CreateTime,
			&role.CreateBy,
			&role.ChangeTime,
			&role.ChangeBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}
		role.Comments = comments.String
		roles = append(roles, role)
	}

	return roles, nil
}

// GetByID returns a role by ID.
func (r *DBRoleRepository) GetByID(id int) (*models.DBRole, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, comments, valid_id, create_time, create_by, change_time, change_by
		FROM roles
		WHERE id = $1
	`)

	role := &models.DBRole{}
	var comments sql.NullString
	err := r.db.QueryRow(query, id).Scan(
		&role.ID,
		&role.Name,
		&comments,
		&role.ValidID,
		&role.CreateTime,
		&role.CreateBy,
		&role.ChangeTime,
		&role.ChangeBy,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("role not found")
		}
		return nil, fmt.Errorf("failed to get role: %w", err)
	}
	role.Comments = comments.String
	return role, nil
}

// GetByName returns a role by name.
func (r *DBRoleRepository) GetByName(name string) (*models.DBRole, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, name, comments, valid_id, create_time, create_by, change_time, change_by
		FROM roles
		WHERE name = $1
	`)

	role := &models.DBRole{}
	var comments sql.NullString
	err := r.db.QueryRow(query, name).Scan(
		&role.ID,
		&role.Name,
		&comments,
		&role.ValidID,
		&role.CreateTime,
		&role.CreateBy,
		&role.ChangeTime,
		&role.ChangeBy,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("role not found")
		}
		return nil, fmt.Errorf("failed to get role: %w", err)
	}
	role.Comments = comments.String
	return role, nil
}

// Create creates a new role.
func (r *DBRoleRepository) Create(role *models.DBRole) error {
	now := time.Now()
	role.CreateTime = now
	role.ChangeTime = now

	if database.IsMySQL() {
		result, err := r.db.Exec(`
			INSERT INTO roles (name, comments, valid_id, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, role.Name, nullString(role.Comments), role.ValidID, role.CreateTime, role.CreateBy, role.ChangeTime, role.ChangeBy)
		if err != nil {
			return fmt.Errorf("failed to create role: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get role ID: %w", err)
		}
		role.ID = int(id)
	} else {
		err := r.db.QueryRow(`
			INSERT INTO roles (name, comments, valid_id, create_time, create_by, change_time, change_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id
		`, role.Name, nullString(role.Comments), role.ValidID, role.CreateTime, role.CreateBy, role.ChangeTime, role.ChangeBy).Scan(&role.ID)
		if err != nil {
			return fmt.Errorf("failed to create role: %w", err)
		}
	}

	return nil
}

// Update updates an existing role.
func (r *DBRoleRepository) Update(role *models.DBRole) error {
	role.ChangeTime = time.Now()

	query := database.ConvertPlaceholders(`
		UPDATE roles
		SET name = $1, comments = $2, valid_id = $3, change_time = $4, change_by = $5
		WHERE id = $6
	`)

	result, err := r.db.Exec(query, role.Name, nullString(role.Comments), role.ValidID, role.ChangeTime, role.ChangeBy, role.ID)
	if err != nil {
		return fmt.Errorf("failed to update role: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("role not found")
	}

	return nil
}

// Delete deletes a role by ID.
func (r *DBRoleRepository) Delete(id int) error {
	query := database.ConvertPlaceholders(`DELETE FROM roles WHERE id = $1`)

	result, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete role: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("role not found")
	}

	return nil
}

// GetUserCount returns the number of users assigned to a role.
func (r *DBRoleRepository) GetUserCount(roleID int) (int, error) {
	query := database.ConvertPlaceholders(`
		SELECT COUNT(*) FROM role_user WHERE role_id = $1
	`)

	var count int
	err := r.db.QueryRow(query, roleID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get user count: %w", err)
	}
	return count, nil
}

// GetGroupCount returns the number of groups assigned to a role.
func (r *DBRoleRepository) GetGroupCount(roleID int) (int, error) {
	query := database.ConvertPlaceholders(`
		SELECT COUNT(DISTINCT group_id) FROM group_role WHERE role_id = $1
	`)

	var count int
	err := r.db.QueryRow(query, roleID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get group count: %w", err)
	}
	return count, nil
}

// ListRoleUsers returns all users assigned to a role.
func (r *DBRoleRepository) ListRoleUsers(roleID int) ([]int, error) {
	query := database.ConvertPlaceholders(`
		SELECT user_id FROM role_user WHERE role_id = $1
	`)

	rows, err := r.db.Query(query, roleID)
	if err != nil {
		return nil, fmt.Errorf("failed to list role users: %w", err)
	}
	defer rows.Close()

	var userIDs []int
	for rows.Next() {
		var userID int
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("failed to scan user ID: %w", err)
		}
		userIDs = append(userIDs, userID)
	}

	return userIDs, nil
}

// ListUserRoles returns all roles assigned to a user.
func (r *DBRoleRepository) ListUserRoles(userID int) ([]*models.DBRole, error) {
	query := database.ConvertPlaceholders(`
		SELECT r.id, r.name, r.comments, r.valid_id, r.create_time, r.create_by, r.change_time, r.change_by
		FROM roles r
		INNER JOIN role_user ru ON r.id = ru.role_id
		WHERE ru.user_id = $1
		ORDER BY r.name
	`)

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list user roles: %w", err)
	}
	defer rows.Close()

	var roles []*models.DBRole
	for rows.Next() {
		role := &models.DBRole{}
		var comments sql.NullString
		err := rows.Scan(
			&role.ID,
			&role.Name,
			&comments,
			&role.ValidID,
			&role.CreateTime,
			&role.CreateBy,
			&role.ChangeTime,
			&role.ChangeBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}
		role.Comments = comments.String
		roles = append(roles, role)
	}

	return roles, nil
}

// AddUserToRole adds a user to a role.
func (r *DBRoleRepository) AddUserToRole(userID, roleID, createdBy int) error {
	now := time.Now()

	if database.IsMySQL() {
		_, err := r.db.Exec(`
			INSERT INTO role_user (user_id, role_id, create_time, create_by, change_time, change_by)
			VALUES (?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE change_time = VALUES(change_time), change_by = VALUES(change_by)
		`, userID, roleID, now, createdBy, now, createdBy)
		if err != nil {
			return fmt.Errorf("failed to add user to role: %w", err)
		}
	} else {
		_, err := r.db.Exec(`
			INSERT INTO role_user (user_id, role_id, create_time, create_by, change_time, change_by)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (user_id, role_id) DO UPDATE SET change_time = EXCLUDED.change_time, change_by = EXCLUDED.change_by
		`, userID, roleID, now, createdBy, now, createdBy)
		if err != nil {
			return fmt.Errorf("failed to add user to role: %w", err)
		}
	}

	return nil
}

// RemoveUserFromRole removes a user from a role.
func (r *DBRoleRepository) RemoveUserFromRole(userID, roleID int) error {
	query := database.ConvertPlaceholders(`DELETE FROM role_user WHERE user_id = $1 AND role_id = $2`)

	_, err := r.db.Exec(query, userID, roleID)
	if err != nil {
		return fmt.Errorf("failed to remove user from role: %w", err)
	}
	return nil
}

// SetRoleUsers sets the users for a role (replaces all existing).
func (r *DBRoleRepository) SetRoleUsers(roleID int, userIDs []int, changedBy int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Delete existing users
	deleteQuery := database.ConvertPlaceholders(`DELETE FROM role_user WHERE role_id = $1`)
	_, err = tx.Exec(deleteQuery, roleID)
	if err != nil {
		return fmt.Errorf("failed to clear role users: %w", err)
	}

	// Insert new users
	now := time.Now()
	for _, userID := range userIDs {
		if database.IsMySQL() {
			_, err = tx.Exec(`
				INSERT INTO role_user (user_id, role_id, create_time, create_by, change_time, change_by)
				VALUES (?, ?, ?, ?, ?, ?)
			`, userID, roleID, now, changedBy, now, changedBy)
		} else {
			_, err = tx.Exec(`
				INSERT INTO role_user (user_id, role_id, create_time, create_by, change_time, change_by)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, userID, roleID, now, changedBy, now, changedBy)
		}
		if err != nil {
			return fmt.Errorf("failed to add user to role: %w", err)
		}
	}

	return tx.Commit()
}

// SetUserRoles sets the roles for a user (replaces all existing).
func (r *DBRoleRepository) SetUserRoles(userID int, roleIDs []int, changedBy int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Delete existing roles
	deleteQuery := database.ConvertPlaceholders(`DELETE FROM role_user WHERE user_id = $1`)
	_, err = tx.Exec(deleteQuery, userID)
	if err != nil {
		return fmt.Errorf("failed to clear user roles: %w", err)
	}

	// Insert new roles
	now := time.Now()
	for _, roleID := range roleIDs {
		if database.IsMySQL() {
			_, err = tx.Exec(`
				INSERT INTO role_user (user_id, role_id, create_time, create_by, change_time, change_by)
				VALUES (?, ?, ?, ?, ?, ?)
			`, userID, roleID, now, changedBy, now, changedBy)
		} else {
			_, err = tx.Exec(`
				INSERT INTO role_user (user_id, role_id, create_time, create_by, change_time, change_by)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, userID, roleID, now, changedBy, now, changedBy)
		}
		if err != nil {
			return fmt.Errorf("failed to add role to user: %w", err)
		}
	}

	return tx.Commit()
}

// Helper function to convert empty strings to NULL.
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
