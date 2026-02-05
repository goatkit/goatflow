package routing

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
	"github.com/gotrs-io/gotrs-ce/internal/shared"
)

// with the existing handler functions.
func IntegrateWithExistingSystem(router *gin.Engine, db *sql.DB, jwtManager interface{}) error {
	// Create handler registry
	registry := NewHandlerRegistry()

	// Register all existing handlers
	if err := registerExistingHandlers(registry, db, jwtManager); err != nil {
		return fmt.Errorf("failed to register handlers: %w", err)
	}

	// Register middleware
	if err := registerExistingMiddleware(registry, jwtManager); err != nil {
		return fmt.Errorf("failed to register middleware: %w", err)
	}

	// Create route loader
	loader, err := NewRouteLoader(
		"routes", // Path to route YAML files
		registry,
		router,
		WithHotReload(true),
		WithEnvironment("development"),
	)
	if err != nil {
		return fmt.Errorf("failed to create route loader: %w", err)
	}

	// Load all routes from YAML files
	if err := loader.LoadRoutes(); err != nil {
		return fmt.Errorf("failed to load routes: %w", err)
	}

	log.Printf("Successfully loaded %d route configurations", len(loader.GetLoadedRoutes()))

	return nil
}

// registerExistingHandlers registers all existing handler functions.
func registerExistingHandlers(registry *HandlerRegistry, db *sql.DB, jwtManager interface{}) error {
	handlers := map[string]gin.HandlerFunc{
		// Health checks
		"handleHealthCheck": func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":  "healthy",
				"version": "1.0.0",
			})
		},

		"handleDetailedHealthCheck": func(c *gin.Context) {
			// Check database connection
			dbStatus := "healthy"
			if err := db.Ping(); err != nil {
				dbStatus = "unhealthy"
			}

			c.JSON(http.StatusOK, gin.H{
				"status": "healthy",
				"components": gin.H{
					"database": dbStatus,
					"cache":    "healthy",
					"queue":    "healthy",
				},
			})
		},

		"handleMetrics": func(c *gin.Context) {
			// Placeholder for Prometheus metrics
			c.String(http.StatusOK, "# HELP gotrs_up GOTRS is up\n# TYPE gotrs_up gauge\ngotrs_up 1\n")
		},

		"handleStaticFiles": func(c *gin.Context) {
			// Static file handler - placeholder
			c.String(http.StatusNotFound, "Static files not configured")
		},

		// Attachment handlers (needed for YAML /api attachments routes) - placeholders
		"HandleGetAttachments": func(c *gin.Context) {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
		},
		"HandleUploadAttachment": func(c *gin.Context) {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
		},
		"HandleDownloadAttachment": func(c *gin.Context) {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
		},
		"HandleDeleteAttachment": func(c *gin.Context) {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
		},
		"HandleGetThumbnail": func(c *gin.Context) {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
		},
		"HandleViewAttachment": func(c *gin.Context) {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
		},

		// Authentication handlers would be registered here
		// "handleLogin": func(c *gin.Context) { /* login logic */ },
		// "handleLogout": func(c *gin.Context) { /* logout logic */ },
		// etc...
	}

	// Register core handlers (including redirects)
	registerCoreHandlers(handlers, db)

	// Register admin handlers
	registerAdminHandlers(handlers, db)

	// Register agent handlers
	registerAgentHandlers(handlers, db)

	// Register agent handlers for YAML routing - placeholders
	handlers["handleAgentNewTicket"] = func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
	}
	handlers["handleAgentCreateTicket"] = func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
	}

	// Register customer handlers
	registerCustomerHandlers(handlers, db)

	// Register dev handlers
	registerDevHandlers(handlers, db)

	// Register API v1 handlers - placeholders
	handlers["api_v1_webhooks_list"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
	handlers["api_v1_webhook_register"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
	handlers["api_v1_webhook_get"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
	handlers["api_v1_webhook_update"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
	handlers["api_v1_webhook_delete"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
	handlers["api_v1_webhook_test"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
	handlers["api_v1_webhook_deliveries"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
	handlers["api_v1_webhook_delivery_retry"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }

	handlers["api_v1_states_list"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
	handlers["api_v1_state_create"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
	handlers["api_v1_state_get"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
	handlers["api_v1_state_update"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
	handlers["api_v1_state_delete"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }

	handlers["api_v1_sla_list"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
	handlers["api_v1_sla_create"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
	handlers["api_v1_sla_get"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
	handlers["api_v1_sla_update"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
	handlers["api_v1_sla_delete"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }

	handlers["api_v1_statistics_dashboard"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
	handlers["api_v1_statistics_customer"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
	handlers["api_v1_statistics_export"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }

	// Register all handlers with the registry
	return registry.RegisterBatch(handlers)
}

// registerDevHandlers registers dev-specific handlers.
func registerDevHandlers(handlers map[string]gin.HandlerFunc, db *sql.DB) {
	// Dev dashboard handlers - placeholders
	handlers["HandleDevDashboard"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
	handlers["HandleClaudeTickets"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
	handlers["HandleDevAction"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
	handlers["HandleDevLogs"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
	handlers["HandleDevDatabase"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
}

// registerCoreHandlers registers core handlers including redirects.
func registerCoreHandlers(handlers map[string]gin.HandlerFunc, db *sql.DB) {
	// Redirect handlers
	handlers["redirect_tickets"] = wrapHandler(db, "handleRedirectTickets")
	handlers["redirect_tickets_new"] = wrapHandler(db, "handleRedirectTicketsNew")
	handlers["redirect_queues"] = wrapHandler(db, "handleRedirectQueues")
	handlers["redirect_profile"] = wrapHandler(db, "handleRedirectProfile")
	handlers["redirect_settings"] = wrapHandler(db, "handleRedirectSettings")
}

// registerAdminHandlers registers admin-specific handlers.
func registerAdminHandlers(handlers map[string]gin.HandlerFunc, db *sql.DB) {
	// Customer company handlers
	handlers["handleAdminCustomerCompanies"] = wrapHandler(db, "handleAdminCustomerCompanies")
	handlers["handleAdminNewCustomerCompany"] = wrapHandler(db, "handleAdminNewCustomerCompany")
	handlers["handleAdminCreateCustomerCompany"] = wrapHandler(db, "handleAdminCreateCustomerCompany")
	handlers["handleAdminEditCustomerCompany"] = wrapHandler(db, "handleAdminEditCustomerCompany")
	handlers["handleAdminUpdateCustomerCompany"] = wrapHandler(db, "handleAdminUpdateCustomerCompany")
	handlers["handleAdminDeleteCustomerCompany"] = wrapHandler(db, "handleAdminDeleteCustomerCompany")
	handlers["handleAdminCustomerCompanyUsers"] = wrapHandler(db, "handleAdminCustomerCompanyUsers")
	handlers["handleAdminCustomerCompanyTickets"] = wrapHandler(db, "handleAdminCustomerCompanyTickets")
	handlers["handleAdminCustomerCompanyServices"] = wrapHandler(db, "handleAdminCustomerCompanyServices")
	handlers["handleAdminUpdateCustomerCompanyServices"] = wrapHandler(db, "handleAdminUpdateCustomerCompanyServices")
	handlers["handleAdminCustomerPortalSettings"] = wrapHandler(db, "handleAdminCustomerPortalSettings")
	handlers["handleAdminUpdateCustomerPortalSettings"] = wrapHandler(db, "handleAdminUpdateCustomerPortalSettings")
	handlers["handleAdminUploadCustomerPortalLogo"] = wrapHandler(db, "handleAdminUploadCustomerPortalLogo")

	// Service CRUD handlers
	handlers["handleAdminServices"] = wrapHandler(db, "handleAdminServices")
	handlers["handleAdminServiceCreate"] = wrapHandler(db, "handleAdminServiceCreate")
	handlers["handleAdminServiceUpdate"] = wrapHandler(db, "handleAdminServiceUpdate")
	handlers["handleAdminServiceDelete"] = wrapHandler(db, "handleAdminServiceDelete")

	// Customer user handlers bridge YAML loader to real implementations
	handlers["HandleAdminCustomerUsersList"] = wrapHandler(db, "HandleAdminCustomerUsersList")
	handlers["HandleAdminCustomerUsersGet"] = wrapHandler(db, "HandleAdminCustomerUsersGet")
	handlers["HandleAdminCustomerUsersCreate"] = wrapHandler(db, "HandleAdminCustomerUsersCreate")
	handlers["HandleAdminCustomerUsersUpdate"] = wrapHandler(db, "HandleAdminCustomerUsersUpdate")
	handlers["HandleAdminCustomerUsersDelete"] = wrapHandler(db, "HandleAdminCustomerUsersDelete")
	handlers["HandleAdminCustomerUsersTickets"] = wrapHandler(db, "HandleAdminCustomerUsersTickets")
	handlers["HandleAdminCustomerUsersImportForm"] = wrapHandler(db, "HandleAdminCustomerUsersImportForm")
	handlers["HandleAdminCustomerUsersImport"] = wrapHandler(db, "HandleAdminCustomerUsersImport")
	handlers["HandleAdminCustomerUsersExport"] = wrapHandler(db, "HandleAdminCustomerUsersExport")
	handlers["HandleAdminCustomerUsersBulkAction"] = wrapHandler(db, "HandleAdminCustomerUsersBulkAction")
	// Admin placeholder sections served via HTMX templates
	handlers["handleAdminSettings"] = wrapHandler(db, "handleAdminSettings")
	handlers["handleAdminTemplates"] = wrapHandler(db, "handleAdminTemplates")
	handlers["handleAdminReports"] = wrapHandler(db, "handleAdminReports")
	handlers["handleAdminLogs"] = wrapHandler(db, "handleAdminLogs")
	handlers["handleAdminBackup"] = wrapHandler(db, "handleAdminBackup")

	// Dynamic Fields management handlers
	handlers["handleAdminDynamicFields"] = wrapHandler(db, "handleAdminDynamicFields")
	handlers["handleAdminDynamicFieldNew"] = wrapHandler(db, "handleAdminDynamicFieldNew")
	handlers["handleAdminDynamicFieldEdit"] = wrapHandler(db, "handleAdminDynamicFieldEdit")
	handlers["handleAdminDynamicFieldScreenConfig"] = wrapHandler(db, "handleAdminDynamicFieldScreenConfig")
	handlers["handleCreateDynamicField"] = wrapHandler(db, "handleCreateDynamicField")
	handlers["handleUpdateDynamicField"] = wrapHandler(db, "handleUpdateDynamicField")
	handlers["handleDeleteDynamicField"] = wrapHandler(db, "handleDeleteDynamicField")
	handlers["handleAdminDynamicFieldScreenConfigSave"] = wrapHandler(db, "handleAdminDynamicFieldScreenConfigSave")
	handlers["handleAdminDynamicFieldScreenConfigSingle"] = wrapHandler(db, "handleAdminDynamicFieldScreenConfigSingle")

	// Add other admin handlers...
}

// registerAgentHandlers registers agent-specific handlers.
func registerAgentHandlers(handlers map[string]gin.HandlerFunc, db *sql.DB) {
	// These would come from agent_routes.go
	handlers["handleAgentDashboard"] = wrapHandler(db, "handleAgentDashboard")
	handlers["handleAgentTickets"] = wrapHandler(db, "handleAgentTickets")
	handlers["handleAgentTicketUpdate"] = wrapHandler(db, "handleAgentTicketUpdate")
	handlers["handleAgentAddNote"] = wrapHandler(db, "handleAgentAddNote")
	handlers["handleAgentTicketReply"] = wrapHandler(db, "handleAgentTicketReply")
	handlers["handleAgentAssignTicket"] = wrapHandler(db, "handleAgentAssignTicket")
	handlers["handleAgentCustomers"] = wrapHandler(db, "handleAgentCustomers")
	handlers["handleAgentCustomerView"] = wrapHandler(db, "handleAgentCustomerView")
	handlers["handleAgentCustomerTickets"] = wrapHandler(db, "handleAgentCustomerTickets")

	// New agent ticket creation handlers - placeholders
	handlers["HandleAgentNewTicket"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
	handlers["HandleAgentCreateTicket"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }

	// Time accounting API handler used by YAML - placeholder
	handlers["handleAddTicketTime"] = func(c *gin.Context) { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }

	// Add other agent handlers...
}

// registerCustomerHandlers registers customer-specific handlers.
func registerCustomerHandlers(handlers map[string]gin.HandlerFunc, db *sql.DB) {
	handlers["handleCustomerCloseTicket"] = wrapHandler(db, "handleCustomerCloseTicket")
	handlers["handleCustomerProfile"] = wrapHandler(db, "handleCustomerProfile")
	handlers["handleCustomerUpdateProfile"] = wrapHandler(db, "handleCustomerUpdateProfile")
	handlers["handleCustomerPasswordForm"] = wrapHandler(db, "handleCustomerPasswordForm")
	handlers["handleCustomerChangePassword"] = wrapHandler(db, "handleCustomerChangePassword")
	handlers["handleCustomerKnowledgeBase"] = wrapHandler(db, "handleCustomerKnowledgeBase")
	handlers["handleCustomerKBSearch"] = wrapHandler(db, "handleCustomerKBSearch")
	handlers["handleCustomerKBArticle"] = wrapHandler(db, "handleCustomerKBArticle")
	handlers["handleCustomerCompanyInfo"] = wrapHandler(db, "handleCustomerCompanyInfo")
	handlers["handleCustomerCompanyUsers"] = wrapHandler(db, "handleCustomerCompanyUsers")
}

// registerExistingMiddleware registers all existing middleware.
func registerExistingMiddleware(registry *HandlerRegistry, jwtManager interface{}) error {
	middlewares := map[string]gin.HandlerFunc{
		// Authentication middleware
		"auth": func(c *gin.Context) {
			// Use shared JWT manager for authentication
			jwtManager := shared.GetJWTManager()
			middleware.SessionMiddleware(jwtManager)(c)
		},

		// Customer portal gate (loads config, can allow anonymous if login not required)
		"customer-portal": middleware.CustomerPortalGate(shared.GetJWTManager()),

		// Admin authorization middleware
		"admin": func(c *gin.Context) {
			role, exists := c.Get("user_role")
			if !exists || role != "Admin" {
				c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
				c.Abort()
				return
			}
			c.Next()
		},

		// Agent authorization middleware
		"agent": func(c *gin.Context) {
			role, exists := c.Get("user_role")
			if !exists || (role != "Agent" && role != "Admin") {
				c.JSON(http.StatusForbidden, gin.H{"error": "Agent access required"})
				c.Abort()
				return
			}
			c.Next()
		},

		// Customer authorization middleware
		"customer": func(c *gin.Context) {
			isCustomer, _ := c.Get("is_customer")
			if !isCustomer.(bool) {
				c.JSON(http.StatusForbidden, gin.H{"error": "Customer access required"})
				c.Abort()
				return
			}
			c.Next()
		},

		// Audit logging middleware
		"audit": func(c *gin.Context) {
			// Log the request
			userID, _ := c.Get("user_id")
			log.Printf("Audit: %s %s by user %v", c.Request.Method, c.Request.URL.Path, userID)
			c.Next()
			// Log the response status
			log.Printf("Audit: Response %d for %s %s", c.Writer.Status(), c.Request.Method, c.Request.URL.Path)
		},

		// CORS middleware
		"cors": func(c *gin.Context) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}

			c.Next()
		},

		// Rate limiting middleware
		"rateLimit": middleware.RateLimitMiddleware(),

		// Note: api_token and unified_auth are registered in handlers.go

		// API token scope middleware - restricts API token access
		"scope_tickets_read":   middleware.RequireScope("tickets:read"),
		"scope_tickets_write":  middleware.RequireScope("tickets:write"),
		"scope_tickets_delete": middleware.RequireScope("tickets:delete"),
		"scope_articles_read":  middleware.RequireScope("articles:read"),
		"scope_articles_write": middleware.RequireScope("articles:write"),
		"scope_users_read":     middleware.RequireScope("users:read"),
		"scope_queues_read":    middleware.RequireScope("queues:read"),
		"scope_admin":          middleware.RequireScope("admin:*"),
	}

	return registry.RegisterMiddlewareBatch(middlewares)
}

// Global handler map that will be populated by the API package.
var GlobalHandlerMap = make(map[string]gin.HandlerFunc)

// RegisterHandler allows the API package to register handlers.
func RegisterHandler(name string, handler gin.HandlerFunc) {
	GlobalHandlerMap[name] = handler
}

// It looks up the actual handler function from the global registry and passes the database connection.
func wrapHandler(db *sql.DB, handlerName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Look up the handler in the global map
		if handler, exists := GlobalHandlerMap[handlerName]; exists {
			// Temporarily set the database connection for this handler call
			originalDB, _ := database.GetDB()
			database.SetDB(db)
			defer database.SetDB(originalDB)

			handler(c)
		} else {
			// Placeholder for unimplemented handlers
			c.JSON(http.StatusOK, gin.H{
				"handler": handlerName,
				"message": "Handler not registered",
			})
		}
	}
}

// MigrateExistingRoutes helps migrate existing hardcoded routes to YAML.
func MigrateExistingRoutes() error {
	// This function could analyze existing route registrations
	// and generate corresponding YAML files automatically

	log.Println("Route migration helper - analyzes existing routes and suggests YAML configurations")

	// Example: Parse existing route files and generate YAML
	// This would be a more complex implementation in practice

	return nil
}
