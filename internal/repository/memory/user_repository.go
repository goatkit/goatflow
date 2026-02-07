package memory

import (
	"fmt"
	"sync"

	"github.com/goatkit/goatflow/internal/models"
)

// UserRepository provides an in-memory implementation of the UserRepository interface.
type UserRepository struct {
	users  map[uint]*models.User
	nextID uint
	mu     sync.RWMutex
}

// NewUserRepository creates a new in-memory user repository.
func NewUserRepository() *UserRepository {
	return &UserRepository{
		users:  make(map[uint]*models.User),
		nextID: 1,
	}
}

// Create creates a new user.
func (r *UserRepository) Create(user *models.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	user.ID = r.nextID
	r.nextID++
	r.users[user.ID] = user
	return nil
}

// GetByID retrieves a user by ID.
func (r *UserRepository) GetByID(id uint) (*models.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.users[id]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}
	return user, nil
}

// GetByEmail retrieves a user by email.
func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, user := range r.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

// GetByLogin retrieves a user by login.
func (r *UserRepository) GetByLogin(login string) (*models.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, user := range r.users {
		if user.Login == login {
			return user, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

// Update updates an existing user.
func (r *UserRepository) Update(user *models.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.users[user.ID]; !exists {
		return fmt.Errorf("user not found")
	}
	r.users[user.ID] = user
	return nil
}

// Delete deletes a user.
func (r *UserRepository) Delete(id uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.users[id]; !exists {
		return fmt.Errorf("user not found")
	}
	delete(r.users, id)
	return nil
}

// List returns all users.
func (r *UserRepository) List() ([]models.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	users := make([]models.User, 0, len(r.users))
	for _, user := range r.users {
		users = append(users, *user)
	}
	return users, nil
}
