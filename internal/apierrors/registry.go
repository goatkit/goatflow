package apierrors

import (
	"net/http"
	"strings"
	"sync"
)

// ErrorCode represents a registered API error code
type ErrorCode struct {
	Code       string `json:"code"`        // Full namespaced code (e.g., "core:not_found")
	Message    string `json:"message"`     // Default English message
	HTTPStatus int    `json:"http_status"` // Suggested HTTP status code
}

// ErrorEnumerator is implemented by plugins to declare their error codes
type ErrorEnumerator interface {
	EnumerateErrors() []ErrorCode
}

// registry holds all registered error codes
type registry struct {
	mu     sync.RWMutex
	codes  map[string]ErrorCode // code -> ErrorCode
	byNS   map[string][]string  // namespace -> []code
}

// Registry is the global error code registry
var Registry = &registry{
	codes: make(map[string]ErrorCode),
	byNS:  make(map[string][]string),
}

// Register adds an error code to the registry
func (r *registry) Register(e ErrorCode) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.codes[e.Code] = e

	// Extract namespace
	ns := "core"
	if idx := strings.Index(e.Code, ":"); idx > 0 {
		ns = e.Code[:idx]
	}
	r.byNS[ns] = append(r.byNS[ns], e.Code)
}

// RegisterPlugin registers all error codes from a plugin
// Plugin codes are automatically prefixed with the plugin name
func (r *registry) RegisterPlugin(pluginName string, enumerator ErrorEnumerator) {
	codes := enumerator.EnumerateErrors()
	for _, e := range codes {
		// Add plugin prefix if not already present
		if !strings.Contains(e.Code, ":") {
			e.Code = pluginName + ":" + e.Code
		}
		r.Register(e)
	}
}

// Get returns an error code by its full code string
func (r *registry) Get(code string) (ErrorCode, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.codes[code]
	return e, ok
}

// All returns all registered error codes
func (r *registry) All() []ErrorCode {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ErrorCode, 0, len(r.codes))
	for _, e := range r.codes {
		result = append(result, e)
	}
	return result
}

// ByNamespace returns all error codes for a given namespace
func (r *registry) ByNamespace(ns string) []ErrorCode {
	r.mu.RLock()
	defer r.mu.RUnlock()

	codes, ok := r.byNS[ns]
	if !ok {
		return nil
	}

	result := make([]ErrorCode, 0, len(codes))
	for _, code := range codes {
		if e, ok := r.codes[code]; ok {
			result = append(result, e)
		}
	}
	return result
}

// Namespaces returns all registered namespaces
func (r *registry) Namespaces() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, 0, len(r.byNS))
	for ns := range r.byNS {
		result = append(result, ns)
	}
	return result
}

// HTTPStatus returns the suggested HTTP status for a code, or 500 if unknown
func (r *registry) HTTPStatus(code string) int {
	if e, ok := r.Get(code); ok {
		return e.HTTPStatus
	}
	return http.StatusInternalServerError
}

// Message returns the default message for a code, or the code itself if unknown
func (r *registry) Message(code string) string {
	if e, ok := r.Get(code); ok {
		return e.Message
	}
	return code
}
