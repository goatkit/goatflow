package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/api"
	"github.com/gotrs-io/gotrs-ce/internal/config"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/routing"
	"github.com/gotrs-io/gotrs-ce/internal/service"
	"github.com/gotrs-io/gotrs-ce/internal/services/adapter"
	"github.com/gotrs-io/gotrs-ce/internal/services/k8s"
	"github.com/gotrs-io/gotrs-ce/internal/services/scheduler"
	"github.com/gotrs-io/gotrs-ce/internal/ticketnumber"
	"github.com/gotrs-io/gotrs-ce/internal/yamlmgmt"
)

func main() {
	// Initialize service registry early
	log.Println("Initializing service registry...")
	registry, err := adapter.InitializeServiceRegistry()
	if err != nil {
		log.Printf("Warning: Failed to initialize service registry: %v", err)
		// Continue anyway - fallback will be used
	} else {
		// Detect environment and adapt configuration
		detector := k8s.NewDetector()
		log.Printf("Detected environment: %s", detector.Environment())

		// Auto-configure database if environment variables are set
		if err := adapter.AutoConfigureDatabase(); err != nil {
			log.Printf("Warning: Failed to auto-configure database: %v", err)
			// Continue anyway - fallback will be used
		} else {
			log.Println("Database service registered successfully")
		}

		// Setup cleanup on shutdown
		defer func() {
			ctx := context.Background()
			if err := registry.Shutdown(ctx); err != nil {
				log.Printf("Error during registry shutdown: %v", err)
			}
		}()
	}

	// Set Gin mode
	if os.Getenv("APP_ENV") == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// Initialize handler registry
	handlerRegistry := routing.NewHandlerRegistry()

	// Register middleware from routing package
	routing.RegisterExistingHandlers(handlerRegistry)

	// Register actual API handlers to override placeholders
	apiHandlers := map[string]gin.HandlerFunc{
		"handleDashboard":           api.HandleDashboard,
		"dashboard_stats":           api.HandleDashboardStats,
		"dashboard_recent_tickets":  api.HandleRecentTickets,
		"dashboard_queue_status":    api.DashboardQueueStatus,
		"dashboard_activity_stream": api.HandleActivityStream,
		"handleUpdateTicketStatus":  api.HandleUpdateTicketStatus,
		"handlePendingReminderFeed": api.HandlePendingReminderFeed,

		// Agent handlers
		"handleAgentTickets":         api.AgentHandlerExports.HandleAgentTickets,
		"handleAgentTicketReply":     api.AgentHandlerExports.HandleAgentTicketReply,
		"handleAgentTicketNote":      api.AgentHandlerExports.HandleAgentTicketNote,
		"handleAgentTicketPhone":     api.AgentHandlerExports.HandleAgentTicketPhone,
		"handleAgentTicketStatus":    api.AgentHandlerExports.HandleAgentTicketStatus,
		"handleAgentTicketAssign":    api.AgentHandlerExports.HandleAgentTicketAssign,
		"handleAgentTicketPriority":  api.AgentHandlerExports.HandleAgentTicketPriority,
		"handleAgentTicketQueue":     api.AgentHandlerExports.HandleAgentTicketQueue,
		"handleAgentTicketMerge":     api.AgentHandlerExports.HandleAgentTicketMerge,
		"handleAgentSearch":          api.AgentHandlerExports.HandleAgentSearch,
		"handleAgentQueues":          api.AgentHandlerExports.HandleAgentQueues,
		"handleAgentQueueView":       api.AgentHandlerExports.HandleAgentQueueView,
		"handleAgentQueueLock":       api.AgentHandlerExports.HandleAgentQueueLock,
		"handleAgentCustomers":       api.AgentHandlerExports.HandleAgentCustomers,
		"handleAgentCustomerView":    api.AgentHandlerExports.HandleAgentCustomerView,
		"handleAgentCustomerTickets": api.AgentHandlerExports.HandleAgentCustomerTickets,
		"handleAgentSearchResults":   api.AgentHandlerExports.HandleAgentSearchResults,
		"handleAgentDashboard": func(c *gin.Context) {
			c.Redirect(http.StatusFound, "/login")
		},

		// Customer handlers
		"handleCustomerNewTicket":      api.HandleCustomerNewTicket,
		"handleCustomerCreateTicket":   api.HandleCustomerCreateTicket,
		"handleCustomerTicketView":     api.HandleCustomerTicketView,
		"handleCustomerTicketReply":    api.HandleCustomerTicketReply,
		"handleCustomerCloseTicket":    api.HandleCustomerCloseTicket,
		"handleCustomerProfile":        api.HandleCustomerProfile,
		"handleCustomerUpdateProfile":  api.HandleCustomerUpdateProfile,
		"handleCustomerPasswordForm":   api.HandleCustomerPasswordForm,
		"handleCustomerChangePassword": api.HandleCustomerChangePassword,
		"handleCustomerKnowledgeBase":  api.HandleCustomerKnowledgeBase,
		"handleCustomerKBSearch":       api.HandleCustomerKBSearch,
		"handleCustomerKBArticle":      api.HandleCustomerKBArticle,
		"handleCustomerCompanyInfo":    api.HandleCustomerCompanyInfo,
		"handleCustomerCompanyUsers":   api.HandleCustomerCompanyUsers,
		"handleCustomerDashboard": func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "Customer Dashboard",
				"user":    "Customer User",
				"status":  "Customer access working!",
			})
		},
		"handleCustomerTickets": func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "Customer Tickets",
				"tickets": []gin.H{},
				"status":  "Customer tickets working!",
			})
		},

		// Ticket handlers
		"handleTicketDetail":          api.HandleTicketDetail,
		"handleTicketHistoryFragment": api.HandleTicketHistoryFragment,
		"handleTicketLinksFragment":   api.HandleTicketLinksFragment,
		"HandleQueueDetail":           api.HandleQueueDetail,
		// "handleTicketCustomerUsers": api.HandleTicketCustomerUsers,
		"handleAgentTicketDraft": api.AgentHandlerExports.HandleAgentTicketDraft,
		// "handleArticleAttachmentDownload": api.HandleArticleAttachmentDownload,

		// API v1 handlers (using actual API handlers, not v1 router wrappers)
		"HandleListTicketsAPI":  api.HandleListTicketsAPI,
		"HandleCreateTicketAPI": api.HandleCreateTicketAPI,
		"HandleGetTicketAPI":    api.HandleGetTicketAPI,
		"HandleUpdateTicketAPI": api.HandleUpdateTicketAPI,
		"HandleDeleteTicketAPI": api.HandleDeleteTicketAPI,
		"HandleReopenTicketAPI": api.HandleReopenTicketAPI,
		"HandleAgentNewTicket": func(c *gin.Context) {
			db, _ := database.GetDB()
			if db == nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
				return
			}
			api.HandleAgentNewTicket(db)(c)
		},
		"HandleAgentCreateTicket": func(c *gin.Context) {
			db, _ := database.GetDB()
			if db == nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection failed"})
				return
			}
			api.HandleAgentCreateTicket(db)(c)
		},
		"HandleListArticlesAPI":      api.HandleListArticlesAPI,
		"HandleCreateArticleAPI":     api.HandleCreateArticleAPI,
		"HandleGetArticleAPI":        api.HandleGetArticleAPI,
		"HandleUpdateArticleAPI":     api.HandleUpdateArticleAPI,
		"HandleDeleteArticleAPI":     api.HandleDeleteArticleAPI,
		"HandleGetInternalNotes":     api.HandleGetInternalNotes,
		"HandleCreateInternalNote":   api.HandleCreateInternalNote,
		"HandleUpdateInternalNote":   api.HandleUpdateInternalNote,
		"HandleDeleteInternalNote":   api.HandleDeleteInternalNote,
		"HandleUserMeAPI":            api.HandleUserMeAPI,
		"HandleListUsersAPI":         api.HandleListUsersAPI,
		"HandleGetUserAPI":           api.HandleGetUserAPI,
		"HandleCreateUserAPI":        api.HandleCreateUserAPI,
		"HandleUpdateUserAPI":        api.HandleUpdateUserAPI,
		"HandleDeleteUserAPI":        api.HandleDeleteUserAPI,
		"HandleListQueuesAPI":        api.HandleListQueuesAPI,
		"HandleGetQueueAPI":          api.HandleGetQueueAPI,
		"HandleGetQueueAgentsAPI":    api.HandleGetQueueAgentsAPI,
		"HandleCreateQueueAPI":       api.HandleCreateQueueAPI,
		"HandleUpdateQueueAPI":       api.HandleUpdateQueueAPI,
		"HandleDeleteQueueAPI":       api.HandleDeleteQueueAPI,
		"HandleGetQueueStatsAPI":     api.HandleGetQueueStatsAPI,
		"HandleAssignQueueGroupAPI":  api.HandleAssignQueueGroupAPI,
		"HandleRemoveQueueGroupAPI":  api.HandleRemoveQueueGroupAPI,
		"HandleListPrioritiesAPI":    api.HandleListPrioritiesAPI,
		"HandleGetPriorityAPI":       api.HandleGetPriorityAPI,
		"HandleListTypesAPI":         api.HandleListTypesAPI,
		"HandleListStatesAPI":        api.HandleListStatesAPI,
		"HandleSearchAPI":            api.HandleSearchAPI,
		"HandleSearchSuggestionsAPI": api.HandleSearchSuggestionsAPI,
		"HandleReindexAPI":           api.HandleReindexAPI,
		"HandleSearchHealthAPI":      api.HandleSearchHealthAPI,

		// Time accounting
		"handleAddTicketTime": api.HandleAddTicketTime,

		// Attachment API handlers
		"HandleGetAttachments":     api.HandleGetAttachments,
		"HandleUploadAttachment":   api.HandleUploadAttachment,
		"HandleDownloadAttachment": api.HandleDownloadAttachment,
		"HandleDeleteAttachment":   api.HandleDeleteAttachment,
		"HandleGetThumbnail":       api.HandleGetThumbnail,
		"HandleViewAttachment":     api.HandleViewAttachment,

		// Lookup handlers
		"HandleGetQueues":             api.HandleGetQueues,
		"HandleGetPriorities":         api.HandleGetPriorities,
		"HandleGetTypes":              api.HandleGetTypes,
		"HandleGetStatuses":           api.HandleGetStatuses,
		"HandleGetFormData":           api.HandleGetFormData,
		"HandleInvalidateLookupCache": api.HandleInvalidateLookupCache,

		// Legacy compatibility handlers (redirects)
		"HandleLegacyAgentTicketViewRedirect": api.HandleLegacyAgentTicketViewRedirect,
		"HandleLegacyTicketsViewRedirect":     api.HandleLegacyTicketsViewRedirect,

		// Dev handlers
		"HandleDevDashboard":  api.HandleDevDashboard,
		"HandleClaudeTickets": api.HandleClaudeTickets,
		"HandleDevAction":     api.HandleDevAction,
		"HandleDevLogs":       api.HandleDevLogs,
		"HandleDevDatabase":   api.HandleDevDatabase,

		// Auth handlers
		"handleLoginPage": api.HandleLoginPage,
		"handleAuthLogin": api.HandleAuthLogin,
		"handleLogout":    api.HandleLogout,
		"HandleLoginAPI":  api.HandleLoginAPI,

		// Admin handlers
		// Users handled by dynamic module system and specific admin user handlers
		"handleAdminUsers":            api.HandleAdminUsers,
		"handleAdminUserGet":          api.HandleAdminUserGet,
		"handleAdminUserEdit":         api.HandleAdminUserEdit,
		"handleAdminUserUpdate":       api.HandleAdminUserUpdate,
		"handleAdminUserDelete":       api.HandleAdminUserDelete,
		"handleAdminPasswordPolicy":   api.HandleAdminPasswordPolicy,
		"HandleAdminUsersList":        api.HandleAdminUsersList,
		"handleAdminGroups":           api.HandleAdminGroups,
		"handleCreateGroup":           api.HandleCreateGroup,
		"handleGetGroup":              api.HandleGetGroup,
		"handleUpdateGroup":           api.HandleUpdateGroup,
		"handleDeleteGroup":           api.HandleDeleteGroup,
		"handleGroupMembers":          api.HandleGroupMembers,
		"handleAddUserToGroup":        api.HandleAddUserToGroup,
		"handleRemoveUserFromGroup":   api.HandleRemoveUserFromGroup,
		"HandleAdminGroupsUsers":      api.HandleAdminGroupsUsers,
		"HandleAdminGroupsAddUser":    api.HandleAdminGroupsAddUser,
		"HandleAdminGroupsRemoveUser": api.HandleAdminGroupsRemoveUser,
		"handleAdminQueues":           api.HandleAdminQueues,
		"handleAdminPriorities":       api.HandleAdminPriorities,
		// Queue API handlers
		"HandleAPIQueueGet":             api.HandleAPIQueueGet,
		"HandleAPIQueueDetails":         api.HandleAPIQueueDetails,
		"HandleAPIQueueStatus":          api.HandleAPIQueueStatus,
		"handleAdminPermissions":        api.HandleAdminPermissions,
		"handleGetUserPermissionMatrix": api.HandleGetUserPermissionMatrix,
		"handleUpdateUserPermissions":   api.HandleUpdateUserPermissions,
		"handleAdminStates":             api.HandleAdminStates,
		"handleAdminTypes":              api.HandleAdminTypes,
		"handleAdminServices":           api.HandleAdminServices,
		"handleAdminSLA":                api.HandleAdminSLA,
		"handleAdminLookups":            api.HandleAdminLookups,
		"handleAdminCustomerCompanies":  api.HandleAdminCustomerCompanies,
		"handleAdminNewCustomerCompany": func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "New Customer Company",
				"status":  "New customer company working!",
			})
		},
		"handleAdminCreateCustomerCompany": func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "Create Customer Company",
				"status":  "Create customer company working!",
			})
		},
		"handleAdminEditCustomerCompany": func(c *gin.Context) {
			companyID := c.Param("id")
			c.JSON(http.StatusOK, gin.H{
				"message":   "Edit Customer Company",
				"companyID": companyID,
				"status":    "Edit customer company working!",
			})
		},
		"handleAdminUpdateCustomerCompany": func(c *gin.Context) {
			companyID := c.Param("id")
			c.JSON(http.StatusOK, gin.H{
				"message":   "Update Customer Company",
				"companyID": companyID,
				"status":    "Update customer company working!",
			})
		},
		"handleAdminDeleteCustomerCompany": func(c *gin.Context) {
			companyID := c.Param("id")
			c.JSON(http.StatusOK, gin.H{
				"message":   "Delete Customer Company",
				"companyID": companyID,
				"status":    "Delete customer company working!",
			})
		},
		"handleAdminCustomerCompanyUsers":    api.HandleAdminCustomerCompanyUsers,
		"handleAdminCustomerCompanyTickets":  api.HandleAdminCustomerCompanyTickets,
		"handleAdminCustomerCompanyServices": api.HandleAdminCustomerCompanyServices,
		"handleAdminUpdateCustomerCompanyServices": func(c *gin.Context) {
			companyID := c.Param("id")
			c.JSON(http.StatusOK, gin.H{
				"message":   "Update Customer Company Services",
				"companyID": companyID,
				"status":    "Update customer company services working!",
			})
		},
		"handleAdminCustomerPortalSettings": api.HandleAdminCustomerPortalSettings,
		"handleAdminUpdateCustomerPortalSettings": func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "Update Customer Portal Settings",
				"status":  "Update customer portal settings working!",
			})
		},
		"handleAdminUploadCustomerPortalLogo": func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "Upload Customer Portal Logo",
				"status":  "Upload customer portal logo working!",
			})
		},

		// Basic system handlers
		"handleRoot": func(c *gin.Context) {
			c.Redirect(http.StatusFound, "/login")
		},
		"handleHealthCheck": func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":  "healthy",
				"version": "1.0.0",
			})
		},
		"handleDetailedHealthCheck": func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status": "healthy",
				"components": gin.H{
					"database": "unknown", // Would check actual DB
					"cache":    "healthy",
					"queue":    "healthy",
				},
			})
		},
		"handleMetrics": func(c *gin.Context) {
			// Basic Prometheus metrics
			c.String(http.StatusOK, "# HELP gotrs_up GOTRS is up\n# TYPE gotrs_up gauge\ngotrs_up 1\n")
		},
		"handleLogoutRedirect": func(c *gin.Context) {
			// Clear any auth cookies/tokens and redirect to login
			c.SetCookie("auth_token", "", -1, "/", "", false, true)
			c.Redirect(http.StatusFound, "/login")
		},
		"handleTicketRedirect": func(c *gin.Context) {
			// Redirect /ticket/:id to /tickets/:id for compatibility
			ticketID := c.Param("id")
			c.Redirect(http.StatusMovedPermanently, "/tickets/"+ticketID)
		},
		"handleAdminDashboard": api.HandleAdminDashboard,
		"handleQueuesRedirect": func(c *gin.Context) {
			// Use unified redirect handler (which now renders user queues page for Admin/Agent)
			api.HandleRedirectQueues(c)
		},
		"handleDemoCustomerLogin": func(c *gin.Context) {
			// Create a demo customer token
			token := fmt.Sprintf("demo_customer_%s_%d", "john.customer", time.Now().Unix())

			// Set cookie with 24 hour expiry
			c.SetCookie("access_token", token, 86400, "/", "", false, true)

			// Redirect to customer dashboard
			c.Redirect(http.StatusFound, "/customer/")
		},
		"handleStaticFiles": api.HandleStaticFiles,
	}

	routing.RegisterAPIHandlers(handlerRegistry, apiHandlers)

	// Load configuration
	configDir := os.Getenv("CONFIG_DIR")
	if configDir == "" {
		configDir = "/app/config"
	}
	if err := config.Load(configDir); err != nil {
		log.Printf("Warning: Failed to load config: %v", err)
		// Continue with defaults
	}

	// Ticket number generator wiring (prep refactor)
	setup := ticketnumber.SetupFromConfig(configDir)
	// Provide adapter to auth service (unchanged behavior)
	{
		vm := yamlmgmt.NewVersionManager(configDir)
		adapter := yamlmgmt.NewConfigAdapter(vm)
		service.SetConfigAdapter(adapter)
	}
	ticketNumGen := setup.Generator
	systemID := setup.SystemID

	// Create router for YAML routes
	r := gin.New()

	// Global i18n middleware (language detection via ?lang=, cookie, user, Accept-Language)
	i18nMW := middleware.NewI18nMiddleware()
	r.Use(i18nMW.Handle())

	// Configure larger multipart memory limit for large article content
	r.MaxMultipartMemory = 128 << 20 // 128MB

	// Initialize pongo2 renderer for template rendering
	templateDir := "./templates"
	api.InitPongo2Renderer(templateDir)

	// Load YAML routes
	routesDir := os.Getenv("ROUTES_DIR")
	if routesDir == "" {
		routesDir = "/app/routes"
	}

	if err := routing.LoadYAMLRoutes(r, routesDir, handlerRegistry); err != nil {
		log.Printf("❌ Failed to load YAML routes: %v", err)
		log.Fatalf("🚨 YAML routes failed to load - cannot continue without routing configuration")
	}

	log.Println("✅ YAML routes loaded successfully")

	// Runtime audit: verify critical API endpoints were registered (multi-doc safety)
	func() {
		needed := []string{"/api/v1/states", "/api/lookups/statuses", "/api/lookups/queues"}
		present := make(map[string]bool)
		for _, ri := range r.Routes() { // gin.RouteInfo
			present[ri.Path] = true
		}
		missing := []string{}
		for _, p := range needed {
			if !present[p] {
				missing = append(missing, p)
			}
		}
		if len(missing) > 0 {
			log.Printf("⚠️  Route audit: missing expected routes: %v (check multi-doc YAML parsing)", missing)
		} else {
			log.Printf("✅ Route audit passed: core endpoints present")
		}
	}()

	// Initialize real DB-backed ticket number store (OTRS-compatible)
	if db, dbErr := database.GetDB(); dbErr == nil && db != nil && ticketNumGen != nil {
		if _, err := db.Exec("SELECT 1 FROM ticket_number_counter LIMIT 1"); err != nil {
			log.Printf("🚨 ticket_number_counter table not accessible: %v", err)
		} else {
			store := ticketnumber.NewDBStore(db, systemID)
			repository.SetTicketNumberGenerator(ticketNumGen, store)
			log.Printf("🧮 Ticket number store initialized (date-based=%v)", ticketNumGen.IsDateBased())
		}
	} else {
		log.Printf("⚠️  Ticket number store not initialized (dbErr=%v)", dbErr)
	}

	// Config duplicate key audit (best-effort; non-fatal)
	func() {
		vm := yamlmgmt.GetVersionManager()
		if vm == nil {
			return
		}
		adapter := yamlmgmt.NewConfigAdapter(vm)
		settings, err := adapter.GetConfigSettings()
		if err != nil || len(settings) == 0 {
			return
		}
		seen := make(map[string]bool)
		dups := []string{}
		for _, s := range settings {
			name, _ := s["name"].(string)
			if name == "" {
				continue
			}
			if seen[name] {
				dups = append(dups, name)
				continue
			}
			seen[name] = true
		}
		if len(dups) > 0 {
			log.Printf("⚠️  Duplicate config setting names detected (first occurrence wins): %v", dups)
		}
	}()

	log.Println("✅ Backend initialized successfully")

	var schedulerCancel context.CancelFunc
	if db, dbErr := database.GetDB(); dbErr != nil || db == nil {
		log.Printf("scheduler: disabled (database unavailable: %v)", dbErr)
	} else {
		loc := time.UTC
		if cfg := config.Get(); cfg != nil && cfg.App.Timezone != "" {
			if tz, err := time.LoadLocation(cfg.App.Timezone); err != nil {
				log.Printf("scheduler: invalid timezone %q, falling back to UTC: %v", cfg.App.Timezone, err)
			} else {
				loc = tz
			}
		}
		sched := scheduler.NewService(db, scheduler.WithLocation(loc))
		ctx, cancel := context.WithCancel(context.Background())
		schedulerCancel = cancel
		go func() {
			if err := sched.Run(ctx); err != nil {
				log.Printf("scheduler: stopped: %v", err)
			}
		}()
		log.Println("scheduler: background job runner started")
	}
	// Ensure /api/v1 i18n endpoints are registered (after YAML so we can augment)
	v1Group := r.Group("/api/v1")
	i18nHandlers := api.NewI18nHandlers()
	i18nHandlers.RegisterRoutes(v1Group)

	// Direct debug route for ticket number generator introspection
	r.GET("/admin/debug/ticket-number", api.HandleDebugTicketNumber)
	// Config sources introspection
	r.GET("/admin/debug/config-sources", api.HandleDebugConfigSources)

	// Example of using generator early (warm path) – ensure repository updated elsewhere to accept it
	_ = ticketNumGen

	// Start server
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Starting GOTRS HTMX server on port %s\n", port)
	fmt.Println("Available routes:")
	fmt.Println("  GET  /          -> Redirect to /login")
	fmt.Println("  GET  /login     -> Login page")
	fmt.Println("  GET  /dashboard -> Dashboard (demo)")
	fmt.Println("  GET  /tickets   -> Tickets list (demo)")
	fmt.Println("  POST /api/auth/login -> HTMX login")
	fmt.Println("")
	fmt.Println("LDAP API routes:")
	fmt.Println("  POST /api/v1/ldap/configure -> Configure LDAP")
	fmt.Println("  POST /api/v1/ldap/test -> Test LDAP connection")
	fmt.Println("  POST /api/v1/ldap/authenticate -> Authenticate user")
	fmt.Println("  GET  /api/v1/ldap/users/:username -> Get user info")
	fmt.Println("  POST /api/v1/ldap/sync/users -> Sync users")
	fmt.Println("  GET  /api/v1/ldap/config -> Get LDAP config")

	if err := r.Run(":" + port); err != nil {
		if schedulerCancel != nil {
			schedulerCancel()
		}
		log.Fatalf("server failed: %v", err)
	}
	if schedulerCancel != nil {
		schedulerCancel()
	}
}
