package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/repository"
)

// DatabaseAuthProvider provides authentication against the database.
type DatabaseAuthProvider struct {
	userRepo *repository.UserRepository
	db       *sql.DB
	hasher   *PasswordHasher
}

// NewDatabaseAuthProvider creates a new database authentication provider.
func NewDatabaseAuthProvider(db *sql.DB) *DatabaseAuthProvider {
	return &DatabaseAuthProvider{
		userRepo: repository.NewUserRepository(db),
		db:       db,
		hasher:   NewPasswordHasher(),
	}
}

// Authenticate authenticates a user against the database.
func (p *DatabaseAuthProvider) Authenticate(ctx context.Context, username, password string) (*models.User, error) {
	// Try to find user by login or email
	var user *models.User
	var err error
	isCustomer := false

	// In OTRS, agents use login (which can contain @), not separate email field
	// Always try GetByLogin first for agents
	user, err = p.userRepo.GetByLogin(username)

	// If agent lookup fails, try customer_user table
	if err != nil {
		user, err = p.authenticateCustomerUser(ctx, username, password)
		if err != nil {
			return nil, ErrUserNotFound
		}
		isCustomer = true
	}

	// Check if user is active (valid_id = 1 in OTRS)
	if !user.IsActive() {
		return nil, ErrUserDisabled
	}

	// Verify password using our configurable hasher
	if !isCustomer {
		// Use our hasher which auto-detects hash type (SHA256 for OTRS, bcrypt for GoatFlow)
		if !p.hasher.VerifyPassword(password, user.Password) {
			return nil, ErrInvalidCredentials
		}
	}

	// Clear password from user object before returning
	user.Password = ""

	return user, nil
}

// GetUser retrieves user details by username or email.
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

// ValidateToken validates a session token (for future implementation).
func (p *DatabaseAuthProvider) ValidateToken(ctx context.Context, token string) (*models.User, error) {
	// TODO: Implement token validation when we add session management
	// For now, return not implemented
	return nil, ErrAuthBackendFailed
}

// Name returns the name of this auth provider.
func (p *DatabaseAuthProvider) Name() string {
	return "Database"
}

// Priority returns the priority of this provider.
func (p *DatabaseAuthProvider) Priority() int {
	return 10 // Default priority for database auth
}

// authenticateCustomerUser authenticates a customer user from the customer_user table.
func (p *DatabaseAuthProvider) authenticateCustomerUser(ctx context.Context, username, password string) (*models.User, error) {
	// Query customer_user table - allow login by username OR email
	var login, email, customerID, firstName, lastName, pw string
	var validID int
	var id int64

	query := `
		SELECT id, login, email, customer_id, first_name, last_name, pw, valid_id
		FROM customer_user
		WHERE login = ? OR email = ?
	`

	err := p.db.QueryRowContext(ctx, query, username, username).Scan(
		&id, &login, &email, &customerID, &firstName, &lastName, &pw, &validID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("customer user lookup failed: %w", err)
	}

	// Check if customer is active
	if validID != 1 {
		return nil, ErrUserDisabled
	}

	// Verify password
	if !p.hasher.VerifyPassword(password, pw) {
		return nil, ErrInvalidCredentials
	}

	// Convert to models.User format
	user := &models.User{
		ID:        uint(id),
		Login:     login,
		Email:     email,
		FirstName: firstName,
		LastName:  lastName,
		ValidID:   validID,
		Role:      "Customer", // Customer users have Customer role
		Password:  "",         // Clear password
	}

	return user, nil
}

// Register database provider factory.
func init() {
	_ = RegisterProvider("database", func(deps ProviderDependencies) (AuthProvider, error) {
		if deps.DB == nil {
			return nil, errors.New("db required for database auth provider")
		}
		return NewDatabaseAuthProvider(deps.DB), nil
	})
}
