package auth

import (
	"database/sql"
	"errors"
	"fmt"

	platformmodels "github.com/goatkit/goatflow/internal/platform/models"
)

// UserLookup is the subset of user repository methods auth providers need.
// Product code injects a concrete *repository.UserRepository, which satisfies this interface.
type UserLookup interface {
	GetByLogin(login string) (*platformmodels.User, error)
	GetByEmail(email string) (*platformmodels.User, error)
}

// ProviderDependencies bundles common resources providers may need.
type ProviderDependencies struct {
	DB       *sql.DB
	UserRepo UserLookup
	// Config adapter kept generic to avoid import cycle; accessed via injected function.
	// We expose a getter so providers wanting config can perform a type assertion.
}

var userRepoFactory func(*sql.DB) UserLookup

// SetUserRepoFactory injects a factory that creates a UserLookup from a *sql.DB.
// Product code calls this at boot to wire repository.UserRepository without a platform import.
func SetUserRepoFactory(f func(*sql.DB) UserLookup) { userRepoFactory = f }

func getUserRepo(db *sql.DB) UserLookup {
	if userRepoFactory != nil {
		return userRepoFactory(db)
	}
	return nil
}

// ProviderFactory builds an AuthProvider given dependencies.
type ProviderFactory func(deps ProviderDependencies) (AuthProvider, error)

var providerRegistry = map[string]ProviderFactory{}

// RegisterProvider registers a provider factory by name (lowercase unique key).
func RegisterProvider(name string, factory ProviderFactory) error {
	if name == "" {
		return errors.New("provider name required")
	}
	if factory == nil {
		return errors.New("provider factory required")
	}
	if _, exists := providerRegistry[name]; exists {
		return fmt.Errorf("auth provider '%s' already registered", name)
	}
	providerRegistry[name] = factory
	return nil
}

// CreateProvider instantiates a provider by name.
func CreateProvider(name string, deps ProviderDependencies) (AuthProvider, error) {
	if f, ok := providerRegistry[name]; ok {
		return f(deps)
	}
	return nil, fmt.Errorf("unknown auth provider: %s", name)
}

// ListProviders returns registered provider names.
func ListProviders() []string {
	names := make([]string, 0, len(providerRegistry))
	for n := range providerRegistry {
		names = append(names, n)
	}
	return names
}
