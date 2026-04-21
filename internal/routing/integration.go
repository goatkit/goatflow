package routing

import (
	"github.com/gin-gonic/gin"
)

// globalRegistry stores the handler registry instance for use by subsystems
// like the MCP bridge. Populated by LoadYAMLRoutesFromGlobalMap in loader.go.
var globalRegistry *HandlerRegistry

// GetGlobalRegistry returns the handler registry, available after
// LoadYAMLRoutesFromGlobalMap has run.
func GetGlobalRegistry() *HandlerRegistry {
	return globalRegistry
}

// SetGlobalRegistryForTest allows tests to inject or reset the global registry.
func SetGlobalRegistryForTest(r *HandlerRegistry) *HandlerRegistry {
	old := globalRegistry
	globalRegistry = r
	return old
}

// GlobalHandlerMap is populated by the API package's init() functions so the
// YAML route loader can resolve handler names at load time without a
// manual registration pass.
var GlobalHandlerMap = make(map[string]gin.HandlerFunc)

// RegisterHandler allows the API package to register handlers.
func RegisterHandler(name string, handler gin.HandlerFunc) {
	GlobalHandlerMap[name] = handler
}
