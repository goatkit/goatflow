package routing

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/api"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
	"github.com/gotrs-io/gotrs-ce/internal/shared"
)

// IntegrateWithExistingSystem demonstrates how to integrate the YAML-based routing
// with the existing handler functions
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

// registerExistingHandlers registers all existing handler functions
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

		"handleStaticFiles": api.HandleStaticFiles,

		// Attachment handlers (needed for YAML /api attachments routes)
		"HandleGetAttachments": api.HandleGetAttachments,
		"HandleUploadAttachment": api.HandleUploadAttachment,
		"HandleDownloadAttachment": api.HandleDownloadAttachment,
		"HandleDeleteAttachment": api.HandleDeleteAttachment,
		"HandleGetThumbnail": api.HandleGetThumbnail,
		"HandleViewAttachment": api.HandleViewAttachment,

		// Authentication handlers would be registered here
		// "handleLogin": api.HandleLogin(db, jwtManager),
		// "handleLogout": api.HandleLogout(),
		// etc...
	}

	// Register core handlers (including redirects)
	registerCoreHandlers(handlers, db)

	// Register admin handlers
	registerAdminHandlers(handlers, db)

	// Register agent handlers
	registerAgentHandlers(handlers, db)

	// Register agent handlers for YAML routing
	api.RegisterAgentHandlersForRouting()
	if api.GlobalAgentHandlers.NewTicket != nil {
		handlers["handleAgentNewTicket"] = api.GlobalAgentHandlers.NewTicket
	}
	if api.GlobalAgentHandlers.CreateTicket != nil {
		handlers["handleAgentCreateTicket"] = api.GlobalAgentHandlers.CreateTicket
	}

	// Register customer handlers
	registerCustomerHandlers(handlers, db)

	// Register dev handlers
	registerDevHandlers(handlers, db)

	// Register API v1 handlers
	handlers["api_v1_webhooks_list"] = api.HandleListWebhooksAPI
	handlers["api_v1_webhook_register"] = api.HandleRegisterWebhookAPI
	handlers["api_v1_webhook_get"] = api.HandleGetWebhookAPI
	handlers["api_v1_webhook_update"] = api.HandleUpdateWebhookAPI
	handlers["api_v1_webhook_delete"] = api.HandleDeleteWebhookAPI
	handlers["api_v1_webhook_test"] = api.HandleTestWebhookAPI
	handlers["api_v1_webhook_deliveries"] = api.HandleWebhookDeliveriesAPI
	handlers["api_v1_webhook_delivery_retry"] = api.HandleRetryWebhookDeliveryAPI

	handlers["api_v1_states_list"] = api.HandleListTicketStatesAPI
	handlers["api_v1_state_create"] = api.HandleCreateTicketStateAPI
	handlers["api_v1_state_get"] = api.HandleGetTicketStateAPI
	handlers["api_v1_state_update"] = api.HandleUpdateTicketStateAPI
	handlers["api_v1_state_delete"] = api.HandleDeleteTicketStateAPI

	handlers["api_v1_sla_list"] = api.HandleListSLAsAPI
	handlers["api_v1_sla_create"] = api.HandleCreateSLAAPI
	handlers["api_v1_sla_get"] = api.HandleGetSLAAPI
	handlers["api_v1_sla_update"] = api.HandleUpdateSLAAPI
	handlers["api_v1_sla_delete"] = api.HandleDeleteSLAAPI

	handlers["api_v1_statistics_dashboard"] = api.HandleDashboardStatisticsAPI
	handlers["api_v1_statistics_customer"] = api.HandleCustomerStatisticsAPI
	handlers["api_v1_statistics_export"] = api.HandleExportStatisticsAPI

	// Register all handlers with the registry
	return registry.RegisterBatch(handlers)
}

// registerDevHandlers registers dev-specific handlers
func registerDevHandlers(handlers map[string]gin.HandlerFunc, db *sql.DB) {
	// Dev dashboard handlers
	handlers["HandleDevDashboard"] = api.HandleDevDashboard
	handlers["HandleClaudeTickets"] = api.HandleClaudeTickets
	handlers["HandleDevAction"] = api.HandleDevAction
	handlers["HandleDevLogs"] = api.HandleDevLogs
	handlers["HandleDevDatabase"] = api.HandleDevDatabase
}

// registerCoreHandlers registers core handlers including redirects
func registerCoreHandlers(handlers map[string]gin.HandlerFunc, db *sql.DB) {
	// Redirect handlers
	handlers["redirect_tickets"] = wrapHandler(db, "handleRedirectTickets")
	handlers["redirect_tickets_new"] = wrapHandler(db, "handleRedirectTicketsNew")
	handlers["redirect_queues"] = wrapHandler(db, "handleRedirectQueues")
	handlers["redirect_profile"] = wrapHandler(db, "handleRedirectProfile")
	handlers["redirect_settings"] = wrapHandler(db, "handleRedirectSettings")
}

// registerAdminHandlers registers admin-specific handlers
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

	// Add other admin handlers...
}

// registerAgentHandlers registers agent-specific handlers
func registerAgentHandlers(handlers map[string]gin.HandlerFunc, db *sql.DB) {
	// These would come from agent_routes.go
	handlers["handleAgentDashboard"] = wrapHandler(db, "handleAgentDashboard")
	handlers["handleAgentTickets"] = wrapHandler(db, "handleAgentTickets")
	handlers["handleAgentTicketView"] = wrapHandler(db, "handleAgentTicketView")
	handlers["handleAgentTicketUpdate"] = wrapHandler(db, "handleAgentTicketUpdate")
	handlers["handleAgentAddNote"] = wrapHandler(db, "handleAgentAddNote")
	handlers["handleAgentTicketReply"] = wrapHandler(db, "handleAgentTicketReply")
	handlers["handleAgentAssignTicket"] = wrapHandler(db, "handleAgentAssignTicket")
	handlers["handleAgentCustomers"] = wrapHandler(db, "handleAgentCustomers")
	handlers["handleAgentCustomerView"] = wrapHandler(db, "handleAgentCustomerView")
	handlers["handleAgentCustomerTickets"] = wrapHandler(db, "handleAgentCustomerTickets")

	// New agent ticket creation handlers - direct registration
	handlers["HandleAgentNewTicket"] = api.HandleAgentNewTicket(db)
	handlers["HandleAgentCreateTicket"] = api.HandleAgentCreateTicket(db)

	// Time accounting API handler used by YAML
	handlers["handleAddTicketTime"] = api.HandleAddTicketTime

	// Add other agent handlers...
}

// registerCustomerHandlers registers customer-specific handlers
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

// registerExistingMiddleware registers all existing middleware
func registerExistingMiddleware(registry *HandlerRegistry, jwtManager interface{}) error {
	middlewares := map[string]gin.HandlerFunc{
		// Authentication middleware
		"auth": func(c *gin.Context) {
			// Use shared JWT manager for authentication
			jwtManager := shared.GetJWTManager()
			middleware.SessionMiddleware(jwtManager)(c)
		},

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

		// Rate limiting middleware (placeholder)
		"rateLimit": func(c *gin.Context) {
			// Implement rate limiting logic here
			c.Next()
		},
	}

	return registry.RegisterMiddlewareBatch(middlewares)
}

// Global handler map that will be populated by the API package
var GlobalHandlerMap = make(map[string]gin.HandlerFunc)

// RegisterHandler allows the API package to register handlers
func RegisterHandler(name string, handler gin.HandlerFunc) {
	GlobalHandlerMap[name] = handler
}

// wrapHandler is a helper to wrap handler functions that need database access
// It looks up the actual handler function from the global registry
func wrapHandler(db *sql.DB, handlerName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Look up the handler in the global map
		if handler, exists := GlobalHandlerMap[handlerName]; exists {
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

// MigrateExistingRoutes helps migrate existing hardcoded routes to YAML
func MigrateExistingRoutes() error {
	// This function could analyze existing route registrations
	// and generate corresponding YAML files automatically

	log.Println("Route migration helper - analyzes existing routes and suggests YAML configurations")

	// Example: Parse existing route files and generate YAML
	// This would be a more complex implementation in practice

	return nil
}
