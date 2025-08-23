package api

import (
	"database/sql"
	"log"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/components/dynamic"
)

var dynamicHandler *dynamic.DynamicModuleHandler

// SetupDynamicModules initializes and registers the dynamic module system
// alongside existing static modules for side-by-side testing
func SetupDynamicModules(router *gin.RouterGroup, db *sql.DB) error {
	// Template directory is handled by the existing pongo2 renderer
	
	// Initialize dynamic handler with database and templates
	handler, err := dynamic.NewDynamicModuleHandler(db, pongo2Renderer.templateSet, "modules")
	if err != nil {
		return err
	}
	
	dynamicHandler = handler
	
	// Register dynamic routes under /admin/dynamic prefix for testing
	// This allows side-by-side comparison with static modules
	dynamicRoutes := router.Group("/dynamic")
	{
		// List route - shows all items
		dynamicRoutes.GET("/:module", handler.ServeModule)
		
		// Single item routes
		dynamicRoutes.GET("/:module/:id", handler.ServeModule)
		
		// Create routes
		dynamicRoutes.GET("/:module/new", handler.ServeModule)
		dynamicRoutes.POST("/:module", handler.ServeModule)
		
		// Update routes
		dynamicRoutes.GET("/:module/:id/edit", handler.ServeModule)
		dynamicRoutes.PUT("/:module/:id", handler.ServeModule)
		dynamicRoutes.POST("/:module/:id", handler.ServeModule) // For HTML forms
		
		// Delete routes
		dynamicRoutes.DELETE("/:module/:id", handler.ServeModule)
		
		// Status toggle route (soft delete)
		dynamicRoutes.PUT("/:module/:id/status", handler.ServeModule)
		dynamicRoutes.POST("/:module/:id/status", handler.ServeModule)
		
		// Custom actions (reset password, manage groups, etc)
		dynamicRoutes.POST("/:module/:id/:action", handler.ServeModule)
		dynamicRoutes.GET("/:module/:id/:action", handler.ServeModule)
	}
	
	// Log available dynamic modules
	modules := handler.GetAvailableModules()
	log.Printf("Dynamic Module System loaded with %d modules:", len(modules))
	for _, module := range modules {
		log.Printf("  - /admin/dynamic/%s", module)
	}
	
	// Add comparison dashboard for testing
	dynamicRoutes.GET("/", func(c *gin.Context) {
		// Return JSON list of modules for API requests
		if c.GetHeader("X-Requested-With") == "XMLHttpRequest" {
			modules := handler.GetAvailableModules()
			c.JSON(200, gin.H{
				"success": true,
				"modules": modules,
			})
			return
		}
		
		modules := handler.GetAvailableModules()
		
		// Build comparison data
		comparisons := []map[string]interface{}{}
		for _, module := range modules {
			comparison := map[string]interface{}{
				"name":        module,
				"static_url":  "/admin/" + module,
				"dynamic_url": "/admin/dynamic/" + module,
			}
			
			// Check if static version exists
			switch module {
			case "users", "groups", "queues", "priorities":
				comparison["has_static"] = true
			default:
				comparison["has_static"] = false
			}
			
			comparisons = append(comparisons, comparison)
		}
		
		pongo2Renderer.HTML(c, 200, "pages/admin/dynamic_test.pongo2", pongo2.Context{
			"Modules":     comparisons,
			"User":        getUserMapForTemplate(c),
			"ActivePage":  "admin",
			"Title":       "Dynamic Module Testing",
		})
	})
	
	// Schema discovery route is now in htmx_routes.go handleSchemaDiscovery
	
	return nil
}

// GetDynamicHandler returns the initialized dynamic handler
func GetDynamicHandler() *dynamic.DynamicModuleHandler {
	return dynamicHandler
}