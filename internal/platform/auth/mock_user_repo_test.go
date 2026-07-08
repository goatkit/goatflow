package auth

import (
	"fmt"

	"github.com/goatkit/goatflow/internal/platform/models"
)

// mockUserRepo implements UserLookup with an in-memory map.
// Shared between unit tests and integration tests.
type mockUserRepo struct {
	users map[string]*models.User
}

func (m *mockUserRepo) GetByEmail(email string) (*models.User, error) {
	if u, ok := m.users[email]; ok {
		return u, nil
	}
	return nil, fmt.Errorf("user not found: %s", email)
}

func (m *mockUserRepo) GetByLogin(login string) (*models.User, error) {
	for _, u := range m.users {
		if u.Login == login {
			return u, nil
		}
	}
	return nil, fmt.Errorf("user not found: %s", login)
}

func (m *mockUserRepo) Create(u *models.User) error {
	if m.users == nil {
		m.users = make(map[string]*models.User)
	}
	m.users[u.Email] = u
	return nil
}

// SyncGroups is a no-op for the mock since tests don't need real group sync.
func (m *mockUserRepo) SyncGroups(userID uint, groupNames []string) error {
	return nil
}
