package api

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/auth"
	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/ldap"
	"github.com/goatkit/goatflow/internal/middleware"
	"github.com/goatkit/goatflow/internal/routing"
	"github.com/goatkit/goatflow/internal/shared"
)

// SetupHTMXRoutes sets up all HTMX routes on the given router.
func SetupHTMXRoutes(r *gin.Engine) {
	// For testing or when called without auth services
	setupHTMXRoutesWithAuth(r, nil, nil, nil)
}

// NewHTMXRouter creates all routes for the HTMX UI.
func NewHTMXRouter(jwtManager *auth.JWTManager, ldapProvider *ldap.Provider) *gin.Engine {
	r := gin.Default()
	setupHTMXRoutesWithAuth(r, jwtManager, ldapProvider, nil)
	return r
}

// setupHTMXRoutesWithAuth sets up all routes with optional authentication.
func setupHTMXRoutesWithAuth(r *gin.Engine, jwtManager *auth.JWTManager, ldapProvider *ldap.Provider, i18nSvc interface{}) {
	// Initialize pongo2 renderer (non-fatal if templates missing to allow route tests without UI assets)
	templateDir := os.Getenv("TEMPLATES_DIR")
	if templateDir == "" {
		candidates := []string{"./templates", "./web/templates"}
		for _, c := range candidates {
			if fi, err := os.Stat(c); err == nil && fi.IsDir() {
				templateDir = c
				break
			}
		}
	}
	initTemplateRenderer(templateDir)

	// Optional routes watcher (dev only)
	startRoutesWatcher()

	// Note: Auth middleware is now handled by the routing package's RegisterExistingHandlers
	// which sets up all middleware including auth with proper test bypass support
	_ = middleware.NewAuthMiddleware(jwtManager) // Keep for compatibility, actual auth handled by routing

	// Initialize Dynamic Module System (requires database)
	initDynamicModules()

	// Setup API v1 routes (OpenAPI-compliant endpoints)
	SetupAPIv1Routes(r, jwtManager, ldapProvider, i18nSvc)

	// Catch-all for undefined routes
	r.NoRoute(func(c *gin.Context) {
		sendErrorResponse(c, http.StatusNotFound, "Page not found")
	})

	// Register YAML-based routes - ALL routes are now defined in YAML files
	// See routes/*.yaml for route definitions
	// Use the consolidated routing package instead of the duplicate yaml_router_loader
	if err := routing.LoadYAMLRoutesForTesting(r); err != nil {
		log.Printf("Warning: Failed to load YAML routes: %v", err)
	}

	// Selective sub-engine mode (keeps static + YAML separated for targeted reload)
	if useDynamicSubEngine() {
		mountDynamicEngine(r)
	}

	// If hot reload mode requested, install proxy middleware so swapped engines serve new routes
	if os.Getenv("ROUTES_WATCH") != "" && os.Getenv("ROUTES_HOT_RELOAD") != "" && !useDynamicSubEngine() {
		// Store initial engine for swaps
		hotReloadableEngine.Store(r)
		// Mount a top-level handler that always delegates to latest engine (routes registered above)
		r.Any("/*path", engineHandlerMiddleware(r))
	}
}

// initTemplateRenderer initializes the template renderer if the directory is available.
func initTemplateRenderer(templateDir string) {
	if templateDir == "" {
		log.Printf("⚠️ Templates directory not available; continuing without renderer")
		return
	}

	if _, err := os.Stat(templateDir); err != nil {
		log.Printf("⚠️ Templates directory resolved but not accessible (%s): %v", templateDir, err)
		return
	}

	renderer, err := shared.NewTemplateRenderer(templateDir)
	if err != nil {
		log.Printf("⚠️ Failed to initialize template renderer from %s: %v (continuing without templates)", templateDir, err)
		return
	}

	shared.SetGlobalRenderer(renderer)
	log.Printf("Template renderer initialized successfully from %s", templateDir)
}

// initDynamicModules initializes the dynamic module system with database connection.
func initDynamicModules() {
	var (
		dbConn *sql.DB
		dbErr  error
	)
	const (
		maxDynamicDBAttempts = 20
		dynamicDBRetryDelay  = 500 * time.Millisecond
	)
	for attempt := 1; attempt <= maxDynamicDBAttempts; attempt++ {
		dbConn, dbErr = database.GetDB()
		if dbErr == nil && dbConn != nil {
			break
		}
		log.Printf("Dynamic modules waiting for database (attempt %d/%d): %v", attempt, maxDynamicDBAttempts, dbErr)
		time.Sleep(dynamicDBRetryDelay)
	}
	if dbErr == nil && dbConn != nil {
		if err := SetupDynamicModules(dbConn); err != nil {
			log.Printf("WARNING: Failed to setup dynamic modules: %v", err)
		} else {
			log.Println("✅ Dynamic Module System integrated successfully")
		}
	} else {
		log.Printf("WARNING: Cannot setup dynamic modules without database after retries: %v", dbErr)
	}
}

// handleProfile shows user profile page.
func handleProfile(c *gin.Context) {
	user := getUserMapForTemplate(c)

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/profile.pongo2", pongo2.Context{
		"User":       user,
		"ActivePage": "profile",
	})
}

// handleApiTokensPage shows the API tokens management page.
func handleApiTokensPage(c *gin.Context) {
	user := getUserMapForTemplate(c)

	getPongo2Renderer().HTML(c, http.StatusOK, "pages/settings/api_tokens.pongo2", pongo2.Context{
		"User":       user,
		"ActivePage": "settings",
	})
}

// SetupAPIv1Routes configures the v1 API routes.
func SetupAPIv1Routes(r *gin.Engine, jwtManager *auth.JWTManager, ldapProvider *ldap.Provider, i18nSvc interface{}) {
	// Create RBAC instance
	// rbac := auth.NewRBAC()

	// Create LDAP handlers if provider exists
	// var ldapHandlers *ldap.LDAPHandlers
	// if ldapProvider != nil {
	// 	ldapHandlers = ldap.NewLDAPHandlers(ldapProvider)
	// }

	// Create API v1 router
	// apiRouter := v1.NewAPIRouter(rbac, jwtManager, ldapHandlers)

	// Setup the routes
	// apiRouter.SetupV1Routes(r)
}
