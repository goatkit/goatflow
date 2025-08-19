package repository

import (
	"database/sql"
	"fmt"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// UserRepository handles database operations for users
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(id uint) (*models.User, error) {
	query := `
		SELECT id, login, pw, title, first_name, last_name,
		       valid_id, create_time, create_by, change_time, change_by
		FROM users
		WHERE id = $1`

	var user models.User
	var title sql.NullString
	err := r.db.QueryRow(query, id).Scan(
		&user.ID,
		&user.Login,
		&user.Password,
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

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}

	// Set derived fields
	user.IsActive = user.ValidID == 1
	
	// TODO: Load role from role_users table
	// For now, set a default role based on some logic
	user.Role = "Agent" // Default to Agent
	
	return &user, err
}

// GetByEmail retrieves a user by email
func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	// OTRS schema doesn't have email in users table for agents
	// Agents use login only
	return nil, fmt.Errorf("email lookup not supported for agents")
}

// GetByLogin retrieves a user by login username
func (r *UserRepository) GetByLogin(login string) (*models.User, error) {
	query := `
		SELECT id, login, pw, title, first_name, last_name,
		       valid_id, create_time, create_by, change_time, change_by
		FROM users
		WHERE login = $1 AND valid_id = 1`

	var user models.User
	var title sql.NullString
	err := r.db.QueryRow(query, login).Scan(
		&user.ID,
		&user.Login,
		&user.Password,
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

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}

	// Set derived fields
	user.IsActive = user.ValidID == 1
	
	// TODO: Load role from role_users table
	user.Role = "Agent" // Default to Agent
	
	return &user, err
}

// Create creates a new user
func (r *UserRepository) Create(user *models.User) error {
	query := `
		INSERT INTO users (
			login, email, pw, title, first_name, last_name,
			valid_id, create_time, create_by, change_time, change_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		) RETURNING id`

	err := r.db.QueryRow(
		query,
		user.Login,
		user.Email,
		user.Password,
		user.Title,
		user.FirstName,
		user.LastName,
		user.ValidID,
		user.CreateTime,
		user.CreateBy,
		user.ChangeTime,
		user.ChangeBy,
	).Scan(&user.ID)

	return err
}

// Update updates a user
func (r *UserRepository) Update(user *models.User) error {
	query := `
		UPDATE users SET
			login = $2,
			email = $3,
			pw = $4,
			title = $5,
			first_name = $6,
			last_name = $7,
			valid_id = $8,
			change_time = $9,
			change_by = $10
		WHERE id = $1`

	result, err := r.db.Exec(
		query,
		user.ID,
		user.Login,
		user.Email,
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

// List retrieves all active users
func (r *UserRepository) List() ([]*models.User, error) {
	query := `
		SELECT id, login, email, pw, title, first_name, last_name,
		       valid_id, create_time, create_by, change_time, change_by
		FROM users
		WHERE valid_id = 1
		ORDER BY last_name, first_name`

	rows, err := r.db.Query(query)
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
			&user.Email,
			&user.Password,
			&user.Title,
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
		
		// Set derived fields
		user.IsActive = user.ValidID == 1
		user.Role = "Agent" // Default to Agent
		
		users = append(users, &user)
	}

	return users, nil
}