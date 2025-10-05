package auth

import (
    "database/sql"
    "errors"
    "fmt"
)

// ProviderDependencies bundles common resources providers may need.
type ProviderDependencies struct {
    DB     *sql.DB
    // Config adapter kept generic to avoid import cycle; accessed via injected function.
    // We expose a getter so providers wanting config can perform a type assertion.
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
