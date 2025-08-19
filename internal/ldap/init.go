package ldap

import (
	"log"
	"sync"
)

var (
	// Global LDAP instances
	globalProvider   *Provider
	globalMiddleware *AuthMiddleware
	globalHandlers   *LDAPHandlers
	initOnce         sync.Once
)

// Initialize sets up the global LDAP components
func Initialize() (*AuthMiddleware, *LDAPHandlers, error) {
	var initErr error
	
	initOnce.Do(func() {
		// Load configuration from environment variables
		config, err := LoadFromEnvironment()
		if err != nil {
			initErr = err
			return
		}
		
		// Check if LDAP is enabled
		enabled := config != nil
		fallbackAuth := true // Enable fallback to local authentication by default
		
		if enabled {
			log.Printf("LDAP authentication enabled for host: %s:%d", config.Host, config.Port)
			
			// Validate configuration
			if errors := ValidateConfig(config); len(errors) > 0 {
				log.Printf("LDAP configuration validation failed: %v", errors)
				// Don't fail initialization, just disable LDAP
				enabled = false
				config = nil
			}
		} else {
			log.Println("LDAP authentication disabled")
		}
		
		// Create middleware
		globalMiddleware = NewAuthMiddleware(config, enabled, fallbackAuth)
		
		// Create handlers
		globalHandlers = NewLDAPHandlers(globalMiddleware)
		
		// Test connection if enabled
		if enabled && config != nil {
			provider := NewProvider(config)
			if err := provider.TestConnection(); err != nil {
				log.Printf("LDAP connection test failed: %v", err)
				log.Println("LDAP authentication will be disabled")
				globalMiddleware.enabled = false
			} else {
				log.Println("LDAP connection test successful")
			}
		}
	})
	
	return globalMiddleware, globalHandlers, initErr
}

// GetProvider returns the global LDAP provider instance
func GetProvider() *Provider {
	return globalProvider
}

// GetMiddleware returns the global LDAP middleware instance
func GetMiddleware() *AuthMiddleware {
	return globalMiddleware
}

// GetHandlers returns the global LDAP handlers instance
func GetHandlers() *LDAPHandlers {
	return globalHandlers
}

// IsEnabled returns true if LDAP authentication is enabled
func IsEnabled() bool {
	if globalMiddleware == nil {
		return false
	}
	return globalMiddleware.enabled
}

// Reinitialize forces reinitialization of LDAP components
// This is useful for configuration updates at runtime
func Reinitialize() error {
	// Reset the sync.Once
	initOnce = sync.Once{}
	
	// Clear global variables
	globalProvider = nil
	globalMiddleware = nil
	globalHandlers = nil
	
	// Reinitialize
	_, _, err := Initialize()
	return err
}