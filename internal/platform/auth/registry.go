package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	platformmodels "github.com/goatkit/goatflow/internal/platform/models"
)

// UserLookup is the subset of user repository methods auth providers need.
// Product code injects a concrete *repository.UserRepository, which satisfies this interface.
type UserLookup interface {
	GetByLogin(login string) (*platformmodels.User, error)
	GetByEmail(email string) (*platformmodels.User, error)
	Create(user *platformmodels.User) error
	SyncGroups(userID uint, groupNames []string) error
}

// ProviderDependencies bundles common resources providers may need.
type ProviderDependencies struct {
	DB         *sql.DB
	UserRepo   UserLookup
	OIDCClient *http.Client // shared HTTP client for OIDC token exchanges and JWKS fetches
	StateStore StateStore   // state token store for OIDC OAuth2 state protection
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

// Global accessors for handlers that need the OIDC client and state store.
// Set via SetOIDCClient/SetStateStore from main.go during boot.

var (
	globalOIDCClient *http.Client
	globalStateStore StateStore
)

// GetOIDCClient returns the globally-configured OIDC HTTP client.
func GetOIDCClient() *http.Client {
	return globalOIDCClient
}

// SetOIDCClient sets the global OIDC HTTP client.
func SetOIDCClient(client *http.Client) { globalOIDCClient = client }

// GetStateStore returns the globally-configured state store.
func GetStateStore() StateStore {
	return globalStateStore
}

// SetStateStore sets the global state store.
func SetStateStore(store StateStore) { globalStateStore = store }
