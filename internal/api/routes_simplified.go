package api

import (
	"context"
	"log"
	"os"
	
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
	"github.com/gotrs-io/gotrs-ce/internal/config"
	"github.com/gotrs-io/gotrs-ce/internal/routing"
)

// Simplified router for HTMX demo
func NewSimpleRouter() *gin.Engine {
	// Create router without default middleware
	r := gin.New()
	
	// Add logging and recovery middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	
	// Add database health check middleware
	// This will show a friendly error page if database is down
	r.Use(middleware.DatabaseHealthCheck())
	
	// Add i18n middleware for language detection
	i18nMiddleware := middleware.NewI18nMiddleware()
	r.Use(i18nMiddleware.Handle())
	
	// Initialize dashboard manager
	config.InitializeDashboardManager("./config")
	
	// Initialize pongo2 renderer for templates
	templateDir := "./templates"
	pongo2Renderer = NewPongo2Renderer(templateDir)
	log.Println("Pongo2 template renderer initialized")
	
	// Serve static files
	r.Static("/static", "./static")
	r.StaticFile("/favicon.ico", "./static/favicon.ico")
	r.StaticFile("/favicon.svg", "./static/favicon.svg")
	
	// Always use YAML routing by default
	log.Println("Initializing YAML routing system...")
	
	// Initialize route analytics
	metrics := routing.InitRouteMetrics()
	log.Println("Route analytics system initialized")
	
	// Setup metrics endpoints (containerized monitoring)
	metrics.SetupMetricsEndpoints(r)
	log.Println("Analytics endpoints available: /metrics/stats, /metrics/dashboard")
	
	// Create handler registry and register existing handlers
	registry := routing.NewHandlerRegistry()
	if err := routing.RegisterExistingHandlers(registry); err != nil {
		log.Printf("Warning: Failed to register existing handlers: %v", err)
	}
	// Register API handlers
	if err := RegisterWithRouting(registry); err != nil {
		log.Printf("Warning: Failed to register API handlers: %v", err)
	}
	
	// Create file-based route manager
	routesDir := os.Getenv("ROUTES_DIR")
	if routesDir == "" {
		routesDir = "./routes"
	}
	
	routeManager := routing.NewSimpleRouteManager(routesDir, r, registry)
	
	// Start the route manager
	if err := routeManager.Start(context.Background()); err != nil {
		log.Printf("Error: Failed to start YAML routing: %v", err)
		log.Println("Fatal: Cannot continue without routing")
		panic("Failed to initialize routing system")
	}
	
	log.Println("YAML routing successfully started")
	
	// Setup LDAP API routes
	SetupLDAPRoutes(r)
	
	// Setup i18n API routes
	i18nHandlers := NewI18nHandlers()
	apiV1 := r.Group("/api/v1")
	i18nHandlers.RegisterRoutes(apiV1)
	
	// API v1 endpoints are now handled via YAML routes
	// See routes/api/v1/*.yaml for API endpoint definitions
	// The following hardcoded routes have been migrated to YAML:
	
	// Commented out - now handled by YAML routes
	// apiV1.GET("/tickets", ...)     -> routes/api/v1/tickets.yaml
	// apiV1.GET("/users/me", ...)    -> routes/api/v1/users.yaml
	// apiV1.GET("/queues", ...)      -> routes/api/v1/queues.yaml
	// apiV1.GET("/priorities", ...)  -> routes/api/v1/priorities.yaml
	// apiV1.GET("/search", ...)      -> routes/api/v1/search.yaml
	
	return r
}