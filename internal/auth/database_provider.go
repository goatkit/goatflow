package auth

import (
	"context"
	"database/sql"
	"strings"

	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

// DatabaseAuthProvider provides authentication against the database
type DatabaseAuthProvider struct {
	userRepo *repository.UserRepository
	db       *sql.DB
}

// NewDatabaseAuthProvider creates a new database authentication provider
func NewDatabaseAuthProvider(db *sql.DB) *DatabaseAuthProvider {
	return &DatabaseAuthProvider{
		userRepo: repository.NewUserRepository(db),
		db:       db,
	}
}

// Authenticate authenticates a user against the database
func (p *DatabaseAuthProvider) Authenticate(ctx context.Context, username, password string) (*models.User, error) {
	// Try to find user by login or email
	var user *models.User
	var err error
	
	// Check if username looks like an email
	if strings.Contains(username, "@") {
		user, err = p.userRepo.GetByEmail(username)
	} else {
		user, err = p.userRepo.GetByLogin(username)
	}
	
	if err != nil {
		// Try the other method if the first fails
		if strings.Contains(username, "@") {
			// Already tried email, don't retry
			return nil, ErrUserNotFound
		}
		// Username might actually be an email, try that
		user, err = p.userRepo.GetByEmail(username)
		if err != nil {
			return nil, ErrUserNotFound
		}
	}
	
	// Check if user is active
	if !user.IsActive {
		return nil, ErrUserDisabled
	}
	
	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	
	// Clear password from user object before returning
	user.Password = ""
	
	return user, nil
}

// GetUser retrieves user details by username or email
func (p *DatabaseAuthProvider) GetUser(ctx context.Context, identifier string) (*models.User, error) {
	var user *models.User
	var err error
	
	// Check if identifier looks like an email
	if strings.Contains(identifier, "@") {
		user, err = p.userRepo.GetByEmail(identifier)
	} else {
		user, err = p.userRepo.GetByLogin(identifier)
	}
	
	if err != nil {
		// Try the other method if the first fails
		if strings.Contains(identifier, "@") {
			return nil, ErrUserNotFound
		}
		user, err = p.userRepo.GetByEmail(identifier)
		if err != nil {
			return nil, ErrUserNotFound
		}
	}
	
	// Clear password before returning
	user.Password = ""
	
	return user, nil
}

// ValidateToken validates a session token (for future implementation)
func (p *DatabaseAuthProvider) ValidateToken(ctx context.Context, token string) (*models.User, error) {
	// TODO: Implement token validation when we add session management
	// For now, return not implemented
	return nil, ErrAuthBackendFailed
}

// Name returns the name of this auth provider
func (p *DatabaseAuthProvider) Name() string {
	return "Database"
}

// Priority returns the priority of this provider
func (p *DatabaseAuthProvider) Priority() int {
	return 10 // Default priority for database auth
}