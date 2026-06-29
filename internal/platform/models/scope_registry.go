package models

import (
	"sort"
	"strings"
	"sync"
)

// ScopeDefinition defines an API token scope
type ScopeDefinition struct {
	Scope       string `json:"scope"`
	Description string `json:"description"`
	Category    string `json:"category"`     // e.g., "core", "plugin:myplugin"
	RequireRole string `json:"require_role"` // e.g., "Admin", "Agent", "" (any)
	AgentOnly   bool   `json:"agent_only"`   // If true, not available to customers
}

// ScopeRegistry manages available API token scopes
type ScopeRegistry struct {
	mu     sync.RWMutex
	scopes map[string]*ScopeDefinition
}

// Global scope registry
var scopeRegistry = &ScopeRegistry{
	scopes: make(map[string]*ScopeDefinition),
}

func init() {
	// Register core scopes
	RegisterScope(&ScopeDefinition{
		Scope:       "*",
		Description: "Full access (inherits all user permissions)",
		Category:    "core",
	})
	RegisterScope(&ScopeDefinition{
		Scope:       "tickets:read",
		Description: "View tickets",
		Category:    "core",
	})
	RegisterScope(&ScopeDefinition{
		Scope:       "tickets:write",
		Description: "Create and update tickets",
		Category:    "core",
	})
	RegisterScope(&ScopeDefinition{
		Scope:       "tickets:delete",
		Description: "Delete tickets",
		Category:    "core",
		AgentOnly:   true, // Customers typically can't delete
	})
	RegisterScope(&ScopeDefinition{
		Scope:       "articles:read",
		Description: "Read ticket articles",
		Category:    "core",
	})
	RegisterScope(&ScopeDefinition{
		Scope:       "articles:write",
		Description: "Add articles and replies",
		Category:    "core",
	})
	RegisterScope(&ScopeDefinition{
		Scope:       "users:read",
		Description: "View user information",
		Category:    "core",
		AgentOnly:   true,
	})
	RegisterScope(&ScopeDefinition{
		Scope:       "queues:read",
		Description: "View queue information",
		Category:    "core",
		AgentOnly:   true,
	})
	RegisterScope(&ScopeDefinition{
		Scope:       "admin:*",
		Description: "Admin operations",
		Category:    "core",
		// Note: RequireRole removed - actual admin check should use RequireAdminGroup() middleware
		// This scope just ensures customers can't use admin endpoints via API tokens
		AgentOnly: true,
	})
}

// RegisterScope adds a scope to the registry (used by plugins)
func RegisterScope(def *ScopeDefinition) {
	scopeRegistry.mu.Lock()
	defer scopeRegistry.mu.Unlock()
	scopeRegistry.scopes[def.Scope] = def
}

// UnregisterScope removes a scope (used when plugins unload)
func UnregisterScope(scope string) {
	scopeRegistry.mu.Lock()
	defer scopeRegistry.mu.Unlock()
	delete(scopeRegistry.scopes, scope)
}

// GetAvailableScopes returns scopes available for a given user context
func GetAvailableScopes(userRole string, isCustomer bool) []*ScopeDefinition {
	scopeRegistry.mu.RLock()
	defer scopeRegistry.mu.RUnlock()

	var result []*ScopeDefinition
	for _, def := range scopeRegistry.scopes {
		// Skip customer-restricted scopes
		if isCustomer && def.AgentOnly {
			continue
		}

		// Skip role-restricted scopes if user doesn't have the role
		if def.RequireRole != "" && !hasRole(userRole, def.RequireRole) {
			continue
		}

		result = append(result, def)
	}

	// Sort by category then scope name
	sort.Slice(result, func(i, j int) bool {
		if result[i].Category != result[j].Category {
			return result[i].Category < result[j].Category
		}
		return result[i].Scope < result[j].Scope
	})

	return result
}

// IsValidScope checks if a scope is valid (exists in registry)
func IsValidScope(scope string) bool {
	scopeRegistry.mu.RLock()
	defer scopeRegistry.mu.RUnlock()
	_, exists := scopeRegistry.scopes[scope]
	return exists
}

// IsScopeAllowed checks if a scope is allowed for a given user context
func IsScopeAllowed(scope string, userRole string, isCustomer bool) bool {
	scopeRegistry.mu.RLock()
	defer scopeRegistry.mu.RUnlock()

	def, exists := scopeRegistry.scopes[scope]
	if !exists {
		// Check for wildcard patterns like "plugin:myplugin:*"
		return isValidWildcardPattern(scope)
	}

	if isCustomer && def.AgentOnly {
		return false
	}

	if def.RequireRole != "" && !hasRole(userRole, def.RequireRole) {
		return false
	}

	return true
}

// hasRole checks if userRole matches or exceeds requiredRole
func hasRole(userRole, requiredRole string) bool {
	// Admin has all roles
	if userRole == "Admin" {
		return true
	}
	return strings.EqualFold(userRole, requiredRole)
}

// isValidWildcardPattern checks for valid wildcard scope patterns
func isValidWildcardPattern(scope string) bool {
	parts := strings.Split(scope, ":")
	if len(parts) >= 2 && parts[len(parts)-1] == "*" {
		// Valid wildcard like "tickets:*" or "plugin:calendar:*"
		return true
	}
	return false
}

// GetAllScopes returns all registered scopes (for admin/debugging)
func GetAllScopes() []*ScopeDefinition {
	scopeRegistry.mu.RLock()
	defer scopeRegistry.mu.RUnlock()

	result := make([]*ScopeDefinition, 0, len(scopeRegistry.scopes))
	for _, def := range scopeRegistry.scopes {
		result = append(result, def)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Scope < result[j].Scope
	})

	return result
}
