package auth

import (
	"database/sql"
	"errors"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrAccountLocked      = errors.New("account is locked due to multiple failed login attempts")
	ErrUserNotFound       = errors.New("user not found")
	ErrUserInactive       = errors.New("user account is inactive")
)

type AuthService struct {
	db         *sql.DB
	jwtManager *JWTManager
}

func NewAuthService(db *sql.DB, jwtManager *JWTManager) *AuthService {
	return &AuthService{
		db:         db,
		jwtManager: jwtManager,
	}
}

func (s *AuthService) Login(email, password string) (*models.LoginResponse, error) {
	// Find user by email
	user, err := s.getUserByEmail(email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	// Check if account is locked
	if user.IsLocked() {
		return nil, ErrAccountLocked
	}

	// Check if user is active
	if !user.IsActive {
		return nil, ErrUserInactive
	}

	// Verify password
	if !user.CheckPassword(password) {
		// Increment failed login count
		user.IncrementFailedLogin()
		s.updateUserLoginAttempts(user)
		return nil, ErrInvalidCredentials
	}

	// Reset failed login count on successful login
	user.ResetFailedLogin()
	now := time.Now()
	user.LastLogin = &now
	s.updateUserLoginAttempts(user)

	// Generate tokens
	token, err := s.jwtManager.GenerateToken(user.ID, user.Email, user.Role, user.TenantID)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	return &models.LoginResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User:         user,
		ExpiresAt:    time.Now().Add(s.jwtManager.tokenDuration),
	}, nil
}

func (s *AuthService) RefreshToken(refreshToken string) (*models.LoginResponse, error) {
	// Validate refresh token
	claims, err := s.jwtManager.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}

	// Get user from database
	user, err := s.getUserByEmail(claims.Subject)
	if err != nil {
		return nil, err
	}

	if !user.IsActive {
		return nil, ErrUserInactive
	}

	// Generate new access token
	token, err := s.jwtManager.GenerateToken(user.ID, user.Email, user.Role, user.TenantID)
	if err != nil {
		return nil, err
	}

	// Generate new refresh token
	newRefreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	return &models.LoginResponse{
		Token:        token,
		RefreshToken: newRefreshToken,
		User:         user,
		ExpiresAt:    time.Now().Add(s.jwtManager.tokenDuration),
	}, nil
}

func (s *AuthService) ChangePassword(userID uint, oldPassword, newPassword string) error {
	// Get user
	user, err := s.getUserByID(userID)
	if err != nil {
		return err
	}

	// Verify old password
	if !user.CheckPassword(oldPassword) {
		return ErrInvalidCredentials
	}

	// Set new password
	if err := user.SetPassword(newPassword); err != nil {
		return err
	}

	// Update in database
	query := `UPDATE users SET pw = $1, change_time = $2 WHERE id = $3`
	_, err = s.db.Exec(query, user.Password, time.Now(), userID)
	return err
}

func (s *AuthService) getUserByEmail(email string) (*models.User, error) {
	query := `
		SELECT id, login, email, pw, title, first_name, last_name, 
		       valid_id, create_time, create_by, change_time, change_by
		FROM users 
		WHERE email = $1 AND valid_id = 1
		LIMIT 1
	`
	
	user := &models.User{}
	err := s.db.QueryRow(query, email).Scan(
		&user.ID, &user.Login, &user.Email, &user.Password, 
		&user.Title, &user.FirstName, &user.LastName,
		&user.ValidID, &user.CreateTime, &user.CreateBy, 
		&user.ChangeTime, &user.ChangeBy,
	)
	
	if err != nil {
		return nil, err
	}

	// Set additional fields
	user.IsActive = user.ValidID == 1
	user.Role = s.determineUserRole(user.ID)
	
	return user, nil
}

func (s *AuthService) getUserByID(userID uint) (*models.User, error) {
	query := `
		SELECT id, login, email, pw, title, first_name, last_name, 
		       valid_id, create_time, create_by, change_time, change_by
		FROM users 
		WHERE id = $1
		LIMIT 1
	`
	
	user := &models.User{}
	err := s.db.QueryRow(query, userID).Scan(
		&user.ID, &user.Login, &user.Email, &user.Password, 
		&user.Title, &user.FirstName, &user.LastName,
		&user.ValidID, &user.CreateTime, &user.CreateBy, 
		&user.ChangeTime, &user.ChangeBy,
	)
	
	if err != nil {
		return nil, err
	}

	user.IsActive = user.ValidID == 1
	user.Role = s.determineUserRole(user.ID)
	
	return user, nil
}

func (s *AuthService) determineUserRole(userID uint) string {
	// Check group memberships to determine role
	// For MVP, we'll use a simplified approach
	
	query := `
		SELECT g.name 
		FROM groups g
		JOIN group_user gu ON g.id = gu.group_id
		WHERE gu.user_id = $1 AND g.valid_id = 1
		LIMIT 1
	`
	
	var groupName string
	err := s.db.QueryRow(query, userID).Scan(&groupName)
	if err != nil {
		return string(models.RoleCustomer) // Default to Customer
	}

	// Map group names to roles
	switch groupName {
	case "admin", "Admin":
		return string(models.RoleAdmin)
	case "users", "agent", "Agent":
		return string(models.RoleAgent)
	default:
		return string(models.RoleCustomer)
	}
}

func (s *AuthService) updateUserLoginAttempts(user *models.User) error {
	// For now, we'll just update the last login time
	// In a full implementation, we'd track failed attempts in a separate table
	if user.LastLogin != nil {
		query := `UPDATE users SET change_time = $1 WHERE id = $2`
		_, err := s.db.Exec(query, *user.LastLogin, user.ID)
		return err
	}
	return nil
}