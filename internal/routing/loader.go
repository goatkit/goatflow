package routing

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// RouteLoader manages loading and registering routes from YAML files
type RouteLoader struct {
	mu         sync.RWMutex
	routesPath string
	configs    map[string]*RouteConfig
	registry   *HandlerRegistry
	router     *gin.Engine
	watcher    *fsnotify.Watcher

	// Options
	hotReload   bool
	strictMode  bool // Fail on missing handlers in strict mode
	environment string
}

// LoaderOption is a functional option for RouteLoader
type LoaderOption func(*RouteLoader)

// WithHotReload enables hot reload of route configurations
func WithHotReload(enabled bool) LoaderOption {
	return func(l *RouteLoader) {
		l.hotReload = enabled
	}
}

// WithStrictMode enables strict mode (fail on missing handlers)
func WithStrictMode(enabled bool) LoaderOption {
	return func(l *RouteLoader) {
		l.strictMode = enabled
	}
}

// WithEnvironment sets the environment for conditional routes
func WithEnvironment(env string) LoaderOption {
	return func(l *RouteLoader) {
		l.environment = env
	}
}

// NewRouteLoader creates a new route loader
func NewRouteLoader(routesPath string, registry *HandlerRegistry, router *gin.Engine, opts ...LoaderOption) (*RouteLoader, error) {
	loader := &RouteLoader{
		routesPath:  routesPath,
		configs:     make(map[string]*RouteConfig),
		registry:    registry,
		router:      router,
		environment: "development",
	}

	// Apply options
	for _, opt := range opts {
		opt(loader)
	}

	// Set up file watcher if hot reload is enabled
	if loader.hotReload {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return nil, fmt.Errorf("failed to create watcher: %w", err)
		}
		loader.watcher = watcher

		// Start watching
		go loader.watchFiles()
	}

	return loader, nil
}

// LoadRoutes loads all route configurations from the routes directory
func (l *RouteLoader) LoadRoutes() error {
	log.Printf("Loading routes from %s", l.routesPath)

	// Walk through all subdirectories
	err := filepath.Walk(l.routesPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-YAML files
		if info.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		// Load the route configuration
		if err := l.loadRouteFile(path); err != nil {
			if l.strictMode {
				return fmt.Errorf("failed to load %s: %w", path, err)
			}
			log.Printf("Warning: Failed to load %s: %v", path, err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk routes directory: %w", err)
	}

	// Register all loaded routes
	return l.registerAllRoutes()
}

// loadRouteFile loads a single route configuration file
func (l *RouteLoader) loadRouteFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	docIndex := 0
	loadedAny := false
	base := filepath.Base(path)

	for {
		var cfg RouteConfig
		err := dec.Decode(&cfg)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to parse YAML (doc %d in %s): %w", docIndex, base, err)
		}

		// Skip completely empty docs
		if cfg.APIVersion == "" && cfg.Kind == "" && cfg.Metadata.Name == "" && len(cfg.Spec.Routes) == 0 {
			docIndex++
			continue
		}

		if err := l.validateConfig(&cfg); err != nil {
			return fmt.Errorf("invalid configuration (doc %d in %s): %w", docIndex, base, err)
		}

		// Store each document under a synthetic key if multiple docs present
		l.mu.Lock()
		key := base
		if docIndex > 0 {
			key = fmt.Sprintf("%s#%d", base, docIndex)
		}
		l.configs[key] = &cfg
		l.mu.Unlock()

		loadedAny = true
		if docIndex == 0 {
			log.Printf("Loaded route configuration: %s (%s)", cfg.Metadata.Name, base)
		} else {
			log.Printf("Loaded route configuration: %s (%s doc %d)", cfg.Metadata.Name, base, docIndex)
		}

		docIndex++
	}

	if !loadedAny {
		log.Printf("Warning: no route configurations found in %s", base)
	} else if docIndex > 1 {
		log.Printf("Note: %s contained %d YAML documents", base, docIndex)
	}

	if l.watcher != nil {
		l.watcher.Add(path)
	}

	return nil
}

// validateConfig validates a route configuration
func (l *RouteLoader) validateConfig(config *RouteConfig) error {
	// Check required fields
	if config.APIVersion == "" {
		return fmt.Errorf("apiVersion is required")
	}

	if config.Kind == "" {
		return fmt.Errorf("kind is required")
	}

	if config.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}

	// Validate routes
	for i, route := range config.Spec.Routes {
		if route.Path == "" {
			return fmt.Errorf("route[%d]: path is required", i)
		}

		// Check if handler or handlers is specified
		if route.Handler == "" && len(route.Handlers) == 0 {
			return fmt.Errorf("route[%d]: handler or handlers must be specified", i)
		}

		// Validate handler exists (if strict mode)
		if l.strictMode {
			if route.Handler != "" && !l.registry.HandlerExists(route.Handler) {
				return fmt.Errorf("route[%d]: handler '%s' not found in registry", i, route.Handler)
			}

			for method, handler := range route.Handlers {
				if !l.registry.HandlerExists(handler) {
					return fmt.Errorf("route[%d]: handler '%s' for method %s not found in registry", i, handler, method)
				}
			}
		}
	}

	return nil
}

// registerAllRoutes registers all loaded routes with the router
func (l *RouteLoader) registerAllRoutes() error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for name, config := range l.configs {
		// Skip disabled routes
		if !config.Metadata.Enabled {
			log.Printf("Skipping disabled route group: %s", name)
			continue
		}

		// Check environment conditions
		if !l.checkEnvironment(config) {
			log.Printf("Skipping route group %s (environment mismatch)", name)
			continue
		}

		// Register routes for this configuration
		if err := l.registerRouteConfig(config); err != nil {
			return fmt.Errorf("failed to register %s: %w", name, err)
		}
	}

	return nil
}

// registerRouteConfig registers routes from a single configuration
func (l *RouteLoader) registerRouteConfig(config *RouteConfig) error {
	// Create route group
	var group *gin.RouterGroup
	if config.Spec.Prefix != "" {
		group = l.router.Group(config.Spec.Prefix)
	} else {
		// Use the router itself as the group
		group = &l.router.RouterGroup
	}

	// Apply group middleware
	for _, middlewareName := range config.Spec.Middleware {
		middleware, err := l.registry.GetMiddleware(middlewareName)
		if err != nil {
			if l.strictMode {
				return fmt.Errorf("middleware '%s' not found", middlewareName)
			}
			log.Printf("Warning: Middleware '%s' not found, skipping", middlewareName)
			continue
		}
		group.Use(middleware)
	}

	// Register individual routes
	for _, route := range config.Spec.Routes {
		if err := l.registerRoute(group, &route, config); err != nil {
			return fmt.Errorf("failed to register route %s: %w", route.Path, err)
		}
	}

	log.Printf("Registered route group: %s with %d routes", config.Metadata.Name, len(config.Spec.Routes))
	return nil
}

// registerRoute registers a single route
func (l *RouteLoader) registerRoute(group *gin.RouterGroup, route *RouteDefinition, config *RouteConfig) error {
	// Check feature flags
	for _, feature := range route.Features {
		if !l.registry.IsFeatureEnabled(feature) {
			log.Printf("Skipping route %s (feature '%s' not enabled)", route.Path, feature)
			return nil
		}
	}

	// Check conditions
	if route.Condition != "" && !l.evaluateCondition(route.Condition) {
		log.Printf("Skipping route %s (condition '%s' not met)", route.Path, route.Condition)
		return nil
	}

	// Build middleware chain for this route
	middlewareChain := make([]gin.HandlerFunc, 0)
	for _, middlewareName := range route.Middleware {
		middleware, err := l.registry.GetMiddleware(middlewareName)
		if err != nil {
			if l.strictMode {
				return fmt.Errorf("middleware '%s' not found", middlewareName)
			}
			log.Printf("Warning: Middleware '%s' not found for route %s", middlewareName, route.Path)
			continue
		}
		middlewareChain = append(middlewareChain, middleware)
	}

	// Register based on method(s)
	if route.Handler != "" {
		// Single handler for all methods
		handler, err := l.registry.Get(route.Handler)
		if err != nil {
			if l.strictMode {
				return err
			}
			log.Printf("Warning: Handler '%s' not found for route %s", route.Handler, route.Path)
			return nil
		}

		// Determine methods
		methods := l.parseMethods(route.Method)
		for _, method := range methods {
			l.registerMethodRoute(group, method, route.Path, append(middlewareChain, handler)...)
		}
	} else if len(route.Handlers) > 0 {
		// Different handlers for different methods
		for method, handlerName := range route.Handlers {
			handler, err := l.registry.Get(handlerName)
			if err != nil {
				if l.strictMode {
					return err
				}
				log.Printf("Warning: Handler '%s' not found for %s %s", handlerName, method, route.Path)
				continue
			}

			l.registerMethodRoute(group, method, route.Path, append(middlewareChain, handler)...)
		}
	}

	return nil
}

// registerMethodRoute registers a route for a specific HTTP method
func (l *RouteLoader) registerMethodRoute(group *gin.RouterGroup, method, path string, handlers ...gin.HandlerFunc) {
	resolved := normalizeRoutePath(group.BasePath(), path)

	switch strings.ToUpper(method) {
	case "GET":
		group.GET(resolved, handlers...)
	case "POST":
		group.POST(resolved, handlers...)
	case "PUT":
		group.PUT(resolved, handlers...)
	case "DELETE":
		group.DELETE(resolved, handlers...)
	case "PATCH":
		group.PATCH(resolved, handlers...)
	case "HEAD":
		group.HEAD(resolved, handlers...)
	case "OPTIONS":
		group.OPTIONS(resolved, handlers...)
	case "ANY":
		group.Any(resolved, handlers...)
	default:
		log.Printf("Warning: Unknown HTTP method '%s' for route %s", method, path)
	}
}

func normalizeRoutePath(prefix, routePath string) string {
	prefix = strings.TrimSpace(prefix)
	cleanedPrefix := strings.Trim(prefix, "/")

	path := strings.TrimSpace(routePath)
	if prefix == "" {
		if path == "" || path == "/" {
			return "/"
		}
		if strings.HasPrefix(path, "/") {
			return path
		}
		return "/" + path
	}

	if path == "" || path == "/" {
		return ""
	}

	path = strings.TrimPrefix(path, "/")

	if cleanedPrefix != "" {
		if path == cleanedPrefix {
			return ""
		}
		prefixWithSlash := cleanedPrefix + "/"
		if strings.HasPrefix(path, prefixWithSlash) {
			path = strings.TrimPrefix(path, prefixWithSlash)
		}
	}

	return path
}

// parseMethods parses the method field which can be string or []string
func (l *RouteLoader) parseMethods(method interface{}) []string {
	if method == nil {
		return []string{"GET"} // Default to GET
	}

	switch v := method.(type) {
	case string:
		return []string{v}
	case []interface{}:
		methods := make([]string, 0, len(v))
		for _, m := range v {
			if str, ok := m.(string); ok {
				methods = append(methods, str)
			}
		}
		return methods
	case []string:
		return v
	default:
		return []string{"GET"}
	}
}

// checkEnvironment checks if the configuration should be loaded in current environment
func (l *RouteLoader) checkEnvironment(config *RouteConfig) bool {
	// If no specific environment is set, load in all environments
	if config.Metadata.Labels == nil {
		return true
	}

	env, exists := config.Metadata.Labels["environment"]
	if !exists {
		return true
	}

	// Check if current environment matches
	envs := strings.Split(env, ",")
	for _, e := range envs {
		if strings.TrimSpace(e) == l.environment {
			return true
		}
	}

	return false
}

// evaluateCondition evaluates a condition expression
func (l *RouteLoader) evaluateCondition(condition string) bool {
	// Simple environment variable check for now
	// Format: ${ENV_VAR_NAME} or ${ENV_VAR_NAME:default}
	if strings.HasPrefix(condition, "${") && strings.HasSuffix(condition, "}") {
		envVar := condition[2 : len(condition)-1]

		// Check for default value
		parts := strings.SplitN(envVar, ":", 2)
		value := os.Getenv(parts[0])

		if value == "" && len(parts) > 1 {
			value = parts[1]
		}

		// Check if value is truthy
		return value != "" && value != "false" && value != "0"
	}

	// Default to true for unknown conditions
	return true
}

// watchFiles watches for changes in route files
func (l *RouteLoader) watchFiles() {
	for {
		select {
		case event, ok := <-l.watcher.Events:
			if !ok {
				return
			}

			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				log.Printf("Route file modified: %s", event.Name)

				// Reload the specific file
				if err := l.loadRouteFile(event.Name); err != nil {
					log.Printf("Error reloading route file %s: %v", event.Name, err)
				} else {
					// Re-register all routes
					// Note: In production, you'd want more sophisticated hot-reload
					log.Printf("Reloading routes after file change")
					if err := l.registerAllRoutes(); err != nil {
						log.Printf("Error re-registering routes: %v", err)
					}
				}
			}

		case err, ok := <-l.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

// Close closes the route loader and stops watching files
func (l *RouteLoader) Close() error {
	if l.watcher != nil {
		return l.watcher.Close()
	}
	return nil
}

// GetLoadedRoutes returns information about loaded routes
func (l *RouteLoader) GetLoadedRoutes() map[string]*RouteConfig {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Return a copy to prevent modifications
	routes := make(map[string]*RouteConfig)
	for k, v := range l.configs {
		routes[k] = v
	}
	return routes
}

// LoadYAMLRoutes is a package-level function that creates a RouteLoader and loads all routes
func LoadYAMLRoutes(router *gin.Engine, routesPath string, registry *HandlerRegistry) error {
	loader, err := NewRouteLoader(routesPath, registry, router, WithHotReload(false))
	if err != nil {
		return fmt.Errorf("failed to create route loader: %w", err)
	}

	return loader.LoadRoutes()
}

// LoadYAMLRoutesFromGlobalMap loads YAML routes using the GlobalHandlerMap as the handler source.
// This is the preferred method - handlers self-register via init() into GlobalHandlerMap,
// eliminating the need for manual handler registration in main.go.
func LoadYAMLRoutesFromGlobalMap(router *gin.Engine, routesPath string) error {
	// Create a registry populated from GlobalHandlerMap
	registry := NewHandlerRegistry()
	for name, handler := range GlobalHandlerMap {
		registry.Override(name, handler)
	}

	// Register standard middleware
	RegisterExistingHandlers(registry)

	loader, err := NewRouteLoader(routesPath, registry, router, WithHotReload(false))
	if err != nil {
		return fmt.Errorf("failed to create route loader: %w", err)
	}

	return loader.LoadRoutes()
}
