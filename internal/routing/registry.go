package routing

import (
	"fmt"
	"sync"

	"github.com/gin-gonic/gin"
)

// HandlerFunc is the standard handler function type.
type HandlerFunc gin.HandlerFunc

// HandlerRegistry manages the mapping between handler names and functions.
type HandlerRegistry struct {
	mu       sync.RWMutex
	handlers map[string]gin.HandlerFunc

	// Middleware registry
	middleware map[string]gin.HandlerFunc

	// Feature flags
	features map[string]bool
}

// NewHandlerRegistry creates a new handler registry.
func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		handlers:   make(map[string]gin.HandlerFunc),
		middleware: make(map[string]gin.HandlerFunc),
		features:   make(map[string]bool),
	}
}

// Register adds a handler to the registry.
func (r *HandlerRegistry) Register(name string, handler gin.HandlerFunc) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[name]; exists {
		return fmt.Errorf("handler %s already registered", name)
	}

	r.handlers[name] = handler
	return nil
}

// Override replaces an existing handler or registers a new one.
func (r *HandlerRegistry) Override(name string, handler gin.HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.handlers[name] = handler
}

// RegisterMiddleware adds a middleware to the registry.
func (r *HandlerRegistry) RegisterMiddleware(name string, middleware gin.HandlerFunc) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.middleware[name]; exists {
		return fmt.Errorf("middleware %s already registered", name)
	}

	r.middleware[name] = middleware
	return nil
}

// Get retrieves a handler by name.
func (r *HandlerRegistry) Get(name string) (gin.HandlerFunc, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, exists := r.handlers[name]
	if !exists {
		return nil, fmt.Errorf("handler %s not found", name)
	}

	return handler, nil
}

// GetMiddleware retrieves middleware by name.
func (r *HandlerRegistry) GetMiddleware(name string) (gin.HandlerFunc, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	middleware, exists := r.middleware[name]
	if !exists {
		return nil, fmt.Errorf("middleware %s not found", name)
	}

	return middleware, nil
}

// MustGet retrieves a handler by name, panics if not found.
func (r *HandlerRegistry) MustGet(name string) gin.HandlerFunc {
	handler, err := r.Get(name)
	if err != nil {
		panic(err)
	}
	return handler
}

// RegisterBatch registers multiple handlers at once.
func (r *HandlerRegistry) RegisterBatch(handlers map[string]gin.HandlerFunc) error {
	for name, handler := range handlers {
		if err := r.Register(name, handler); err != nil {
			return err
		}
	}
	return nil
}

// OverrideBatch replaces multiple existing handlers or registers new ones.
func (r *HandlerRegistry) OverrideBatch(handlers map[string]gin.HandlerFunc) {
	for name, handler := range handlers {
		r.Override(name, handler)
	}
}

// RegisterMiddlewareBatch registers multiple middleware at once.
func (r *HandlerRegistry) RegisterMiddlewareBatch(middlewares map[string]gin.HandlerFunc) error {
	for name, middleware := range middlewares {
		if err := r.RegisterMiddleware(name, middleware); err != nil {
			return err
		}
	}
	return nil
}

// SetFeature enables or disables a feature flag.
func (r *HandlerRegistry) SetFeature(name string, enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.features[name] = enabled
}

// IsFeatureEnabled checks if a feature is enabled.
func (r *HandlerRegistry) IsFeatureEnabled(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.features[name]
}

// ListHandlers returns all registered handler names.
func (r *HandlerRegistry) ListHandlers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.handlers))
	for name := range r.handlers {
		names = append(names, name)
	}
	return names
}

// GetAllHandlers returns all registered handlers as a map.
func (r *HandlerRegistry) GetAllHandlers() map[string]gin.HandlerFunc {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handlers := make(map[string]gin.HandlerFunc)
	for name, handler := range r.handlers {
		handlers[name] = handler
	}
	return handlers
}

// ListMiddleware returns all registered middleware names.
func (r *HandlerRegistry) ListMiddleware() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.middleware))
	for name := range r.middleware {
		names = append(names, name)
	}
	return names
}

// Clear removes all registered handlers and middleware.
func (r *HandlerRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.handlers = make(map[string]gin.HandlerFunc)
	r.middleware = make(map[string]gin.HandlerFunc)
	r.features = make(map[string]bool)
}

// HandlerExists checks if a handler is registered.
func (r *HandlerRegistry) HandlerExists(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.handlers[name]
	return exists
}

// MiddlewareExists checks if middleware is registered.
func (r *HandlerRegistry) MiddlewareExists(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.middleware[name]
	return exists
}

// GetHandlerChain builds a complete handler chain with middleware.
func (r *HandlerRegistry) GetHandlerChain(middlewareNames []string, handlerName string) ([]gin.HandlerFunc, error) {
	chain := make([]gin.HandlerFunc, 0, len(middlewareNames)+1)

	// Add middleware
	for _, name := range middlewareNames {
		middleware, err := r.GetMiddleware(name)
		if err != nil {
			return nil, fmt.Errorf("middleware %s: %w", name, err)
		}
		chain = append(chain, middleware)
	}

	// Add final handler
	handler, err := r.Get(handlerName)
	if err != nil {
		return nil, fmt.Errorf("handler %s: %w", handlerName, err)
	}
	chain = append(chain, handler)

	return chain, nil
}
