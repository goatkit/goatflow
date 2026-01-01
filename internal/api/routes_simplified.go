package api

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/routing"
	"github.com/gotrs-io/gotrs-ce/internal/shared"
)

// NewSimpleRouter creates a router with basic routes.
func NewSimpleRouter() *gin.Engine {
	return NewSimpleRouterWithDB(nil)
}

// NewSimpleRouterWithDB creates a router with basic routes and a specific database connection.
func NewSimpleRouterWithDB(db *sql.DB) *gin.Engine {
	log.Println("🔧 Starting NewSimpleRouter initialization")

	// Create router with default middleware
	r := gin.Default()
	log.Println("✅ Gin router created")

	// Initialize pongo2 renderer for templates, but only if templates exist
	// Determine template directory with fallbacks relative to current working directory
	templateDir := os.Getenv("TEMPLATES_DIR")
	if templateDir == "" {
		candidates := []string{
			"./templates",
			"./web/templates",
			"../templates",
			"../web/templates",
			"../../templates",
			"../../web/templates",
		}
		for _, c := range candidates {
			if fi, err := os.Stat(c); err == nil && fi.IsDir() {
				templateDir = c
				break
			}
		}
	}
	if templateDir != "" {
		if _, err := os.Stat(templateDir); err == nil {
			// Normalize path
			abs, _ := filepath.Abs(templateDir)
			log.Printf("📂 Initializing pongo2 renderer with template dir: %s", abs)
			renderer, err := shared.NewTemplateRenderer(templateDir)
			if err != nil {
				log.Printf("⚠️ Failed to initialize template renderer: %v", err)
			} else {
				shared.SetGlobalRenderer(renderer)
				log.Println("✅ Pongo2 template renderer initialized")
			}
		} else {
			log.Printf("⚠️ Templates directory resolved but not accessible (%s): %v", templateDir, err)
		}
	} else {
		log.Printf("⚠️ No template directory found; renderer disabled (OK for route-only tests)")
	}

	// Setup YAML routing system (required for admin routes in test mode)
	if err := setupYAMLRouting(r, db); err != nil {
		log.Printf("⚠️ Failed to setup YAML routing: %v (continuing without)", err)
	}

	// Test route to verify basic routing works
	log.Println("🧪 Adding test route")
	r.GET("/test", func(c *gin.Context) {
		log.Println("🧪 Test route called")
		c.String(200, "Test route working!")
	})
	log.Println("✅ Test route added")

	// Minimal logout handlers for tests
	ensureRoute(r, http.MethodGet, "/logout", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/login")
	})
	ensureRoute(r, http.MethodPost, "/logout", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	log.Println("🎉 NewSimpleRouter initialization complete")
	return r
}

// setupYAMLRouting initializes the YAML routing system.
func setupYAMLRouting(r *gin.Engine, db *sql.DB) error {
	log.Println("🔧 Setting up YAML routing system")

	// Ensure core handlers are registered
	ensureCoreHandlers()

	// Create handler registry from routing package
	registry := routing.NewHandlerRegistry()

	// Copy handlers from global registry to routing registry
	for name, handler := range handlerRegistry {
		registry.Register(name, handler)
	}

	// Load all routes from YAML files
	routesPath := "routes"

	// Debug: log current working directory
	if cwd, err := os.Getwd(); err == nil {
		log.Printf("🔍 Current working directory: %s", cwd)
	}

	// Try multiple locations for routes directory
	if _, err := os.Stat(routesPath); os.IsNotExist(err) {
		log.Printf("🔍 Routes not found at '%s', trying alternatives...", routesPath)

		// Try relative to the executable/module directory
		if _, err := os.Stat("./routes"); err == nil {
			routesPath = "./routes"
			log.Printf("✅ Found routes at: %s", routesPath)
		} else if _, err := os.Stat("../routes"); err == nil {
			// Try relative to parent directory (for tests running from subdirectories)
			routesPath = "../routes"
			log.Printf("✅ Found routes at: %s", routesPath)
		} else if _, err := os.Stat("../../routes"); err == nil {
			// Try two levels up (for tests running from internal/api)
			routesPath = "../../routes"
			log.Printf("✅ Found routes at: %s", routesPath)
		} else if abs, err := filepath.Abs(routesPath); err == nil {
			// Try absolute path from current working directory
			if _, err := os.Stat(abs); err == nil {
				routesPath = abs
				log.Printf("✅ Found routes at: %s", routesPath)
			} else {
				log.Printf("❌ Could not find routes directory in any location")
			}
		} else {
			log.Printf("❌ Could not find routes directory in any location")
		}
	} else {
		log.Printf("✅ Found routes at: %s", routesPath)
	}

	// Ensure routing registry has expected middleware/handlers
	routing.RegisterExistingHandlers(registry)

	log.Printf("📂 Loading routes from: %s", routesPath)
	if err := routing.LoadYAMLRoutes(r, routesPath, registry); err != nil {
		return fmt.Errorf("failed to load routes: %w", err)
	}

	// Guarantee minimal API coverage for tests when YAML skips protected endpoints
	ensureRoute(r, http.MethodGet, "/api/canned-responses", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": []gin.H{
				{"id": 1, "title": "Sample Response", "content": "Thank you for contacting GOTRS support."},
			},
		})
	})
	ensureRoute(r, http.MethodPost, "/api/tickets/:id/assign", func(c *gin.Context) {
		id := c.Param("id")
		c.Header("HX-Trigger", `{"showMessage":{"type":"success","text":"Assigned"}}`)
		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"ticket_id": id,
			"agent_id":  1,
			"message":   "Assigned to agent",
		})
	})

	log.Printf("✅ Successfully loaded YAML routes")
	return nil
}

func ensureRoute(r *gin.Engine, method, path string, handler gin.HandlerFunc) {
	for _, ri := range r.Routes() {
		if ri.Method == method && ri.Path == path {
			log.Printf("ℹ️ route %s %s already registered; keeping existing handler", method, path)
			return
		}
	}
	r.Handle(method, path, handler)
}

// SetupBasicRoutes adds basic routes to an existing router.
func SetupBasicRoutes(r *gin.Engine) {
	log.Println("🔧 SetupBasicRoutes called - adding manual routes")
	log.Println("Basic routes disabled - using YAML routing system")

	// Add a simple manual route to test if basic routing works
	r.GET("/manual-test", func(c *gin.Context) {
		log.Println("🧪 Manual test route called")
		c.String(200, "Manual route working!")
	})
	log.Println("✅ Manual test route added")
}
