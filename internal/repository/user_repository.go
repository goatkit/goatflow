package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// UserRepository handles database operations for users.
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository creates a new user repository.
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// GetByID retrieves a user by ID.
func (r *UserRepository) GetByID(id uint) (*models.User, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, login, pw, title, first_name, last_name,
		       valid_id, create_time, create_by, change_time, change_by
		FROM users
		WHERE id = $1`)

	var user models.User
	var title sql.NullString
	var password sql.NullString
	err := r.db.QueryRow(query, id).Scan(
		&user.ID,
		&user.Login,
		&password,
		&title,
		&user.FirstName,
		&user.LastName,
		&user.ValidID,
		&user.CreateTime,
		&user.CreateBy,
		&user.ChangeTime,
		&user.ChangeBy,
	)

	if title.Valid {
		user.Title = title.String
	}
	if password.Valid {
		user.Password = password.String
	}

	// Set derived fields
	user.Email = user.Login // In OTRS, login can be email

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}

	// Note: IsActive is now a method based on ValidID

	// TODO: Load role from role_users table
	// For now, set a default role based on some logic
	user.Role = "Agent" // Default to Agent

	return &user, err
}

// GetByEmail retrieves a user by email.
func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	// OTRS schema doesn't have email in users table for agents
	// Agents use login only
	return nil, fmt.Errorf("email lookup not supported for agents")
}

// GetByLogin retrieves a user by login username.
func (r *UserRepository) GetByLogin(login string) (*models.User, error) {
	fmt.Printf("UserRepository.GetByLogin: Looking for user '%s'\n", login)
	query := database.ConvertPlaceholders(`
		SELECT id, login, pw, title, first_name, last_name,
		       valid_id, create_time, create_by, change_time, change_by
		FROM users
		WHERE login = $1 AND valid_id = 1`)

	var user models.User
	var title sql.NullString
	var password sql.NullString
	err := r.db.QueryRow(query, login).Scan(
		&user.ID,
		&user.Login,
		&password,
		&title,
		&user.FirstName,
		&user.LastName,
		&user.ValidID,
		&user.CreateTime,
		&user.CreateBy,
		&user.ChangeTime,
		&user.ChangeBy,
	)

	if title.Valid {
		user.Title = title.String
	}
	if password.Valid {
		user.Password = password.String
	}

	if err == sql.ErrNoRows {
		fmt.Printf("UserRepository.GetByLogin: No rows found for '%s'\n", login)
		return nil, fmt.Errorf("user not found")
	}

	if err != nil {
		fmt.Printf("UserRepository.GetByLogin: Error: %v\n", err)
		return nil, err
	}

	fmt.Printf("UserRepository.GetByLogin: Found user ID=%d, login='%s', pw starts with='%.20s'\n", user.ID, user.Login, user.Password)

	// Note: IsActive is now a method based on ValidID
	user.Email = user.Login // In OTRS, login can be email

	// Load role based on group membership
	roleQuery := database.ConvertPlaceholders(`
		SELECT g.name 
		FROM group_user gu
		JOIN groups g ON gu.group_id = g.id
		WHERE gu.user_id = $1 AND g.valid_id = 1
		ORDER BY 
			CASE g.name 
				WHEN 'admin' THEN 1
				WHEN 'users' THEN 2
				ELSE 3
			END
		LIMIT 1`)

	var groupName string
	err = r.db.QueryRow(roleQuery, user.ID).Scan(&groupName)
	if err == nil {
		// Map group name to role
		switch groupName {
		case "admin":
			user.Role = "Admin"
		case "users":
			user.Role = "Agent"
		default:
			user.Role = "Agent"
		}
	} else {
		// Default to Agent if no group found
		user.Role = "Agent"
	}

	fmt.Printf("UserRepository.GetByLogin: User %s has role %s\n", user.Login, user.Role)

	return &user, nil
}

// Create creates a new user.
func (r *UserRepository) Create(user *models.User) error {
	// Truncate title to fit varchar(50) limit
	if len(user.Title) > 50 {
		user.Title = user.Title[:50]
	}

	rawQuery := `
		INSERT INTO users (
			login, pw, title, first_name, last_name,
			valid_id, create_time, create_by, change_time, change_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		) RETURNING id`

	query, useLastInsert := database.ConvertReturning(rawQuery)
	query = database.ConvertPlaceholders(query)

	args := []interface{}{
		user.Login,
		user.Password,
		user.Title,
		user.FirstName,
		user.LastName,
		user.ValidID,
		user.CreateTime,
		user.CreateBy,
		user.ChangeTime,
		user.ChangeBy,
	}

	if useLastInsert && database.IsMySQL() {
		result, err := r.db.Exec(query, args...)
		if err != nil {
			return err
		}

		lastID, err := result.LastInsertId()
		if err != nil {
			return err
		}
		user.ID = uint(lastID)
		return nil
	}

	return r.db.QueryRow(query, args...).Scan(&user.ID)
}

// Update updates a user.
func (r *UserRepository) Update(user *models.User) error {
	// Truncate title to fit varchar(50) limit
	if len(user.Title) > 50 {
		user.Title = user.Title[:50]
	}

	query := database.ConvertPlaceholders(`
		UPDATE users SET
			login = $2,
			pw = $3,
			title = $4,
			first_name = $5,
			last_name = $6,
			valid_id = $7,
			change_time = $8,
			change_by = $9
		WHERE id = $1`)

	result, err := r.db.Exec(
		query,
		user.ID,
		user.Login,
		user.Password,
		user.Title,
		user.FirstName,
		user.LastName,
		user.ValidID,
		user.ChangeTime,
		user.ChangeBy,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// SetValidID updates only the validity status metadata for a user.
func (r *UserRepository) SetValidID(id uint, validID int, changeBy uint, changeTime time.Time) error {
	query := database.ConvertPlaceholders(`
		UPDATE users SET
			valid_id = $1,
			change_time = $2,
			change_by = $3
		WHERE id = $4`)

	result, err := r.db.Exec(query, validID, changeTime, changeBy, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// ListWithGroups retrieves all users with their associated groups.
func (r *UserRepository) ListWithGroups() ([]*models.User, error) {
	users, err := r.List()
	if err != nil {
		return nil, err
	}

	// Fetch groups for each user
	for _, user := range users {
		groups, err := r.GetUserGroups(user.ID)
		if err != nil {
			// Log error but continue - we don't want to fail the entire list
			// just because one user's groups couldn't be fetched
			continue
		}
		user.Groups = groups
	}

	return users, nil
}

// GetUserGroups retrieves the group names for a specific user.
func (r *UserRepository) GetUserGroups(userID uint) ([]string, error) {
	query := database.ConvertPlaceholders(`
		SELECT g.name 
		FROM groups g
		INNER JOIN group_user gu ON g.id = gu.group_id
		WHERE gu.user_id = $1 AND g.valid_id = 1
		ORDER BY g.name`)

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []string
	for rows.Next() {
		var groupName string
		err := rows.Scan(&groupName)
		if err != nil {
			return nil, err
		}
		groups = append(groups, groupName)
	}

	return groups, nil
}

// List retrieves all users (both active and inactive).
func (r *UserRepository) List() ([]*models.User, error) {
	query := database.ConvertPlaceholders(`
		SELECT id, login, pw, title, first_name, last_name,
		       valid_id, create_time, create_by, change_time, change_by
		FROM users
		ORDER BY last_name, first_name`)

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var user models.User
		var title sql.NullString
		var password sql.NullString
		err := rows.Scan(
			&user.ID,
			&user.Login,
			&password,
			&title,
			&user.FirstName,
			&user.LastName,
			&user.ValidID,
			&user.CreateTime,
			&user.CreateBy,
			&user.ChangeTime,
			&user.ChangeBy,
		)
		if err != nil {
			return nil, err
		}

		// Handle nullable fields
		if title.Valid {
			user.Title = title.String
		}
		if password.Valid {
			user.Password = password.String
		}

		// Set derived fields
		// IsActive is now a method based on ValidID
		user.Role = "Agent"     // Default to Agent
		user.Email = user.Login // In OTRS, login can be email

		users = append(users, &user)
	}

	return users, nil
}

// Delete deletes a user by ID.
func (r *UserRepository) Delete(id uint) error {
	query := database.ConvertPlaceholders(`DELETE FROM users WHERE id = $1`)
	result, err := r.db.Exec(query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}
