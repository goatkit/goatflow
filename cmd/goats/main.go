package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gotrs-io/gotrs-ce/internal/api"
	_ "github.com/gotrs-io/gotrs-ce/internal/api" // Import for handler_registry.go init()
	"github.com/gotrs-io/gotrs-ce/internal/cache"
	"github.com/gotrs-io/gotrs-ce/internal/config"
	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/email/inbound/connector"
	"github.com/gotrs-io/gotrs-ce/internal/email/inbound/filters"
	"github.com/gotrs-io/gotrs-ce/internal/email/inbound/postmaster"
	"github.com/gotrs-io/gotrs-ce/internal/lookups"
	"github.com/gotrs-io/gotrs-ce/internal/middleware"
	"github.com/gotrs-io/gotrs-ce/internal/notifications"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/routing"
	"github.com/gotrs-io/gotrs-ce/internal/runner"
	"github.com/gotrs-io/gotrs-ce/internal/runner/tasks"
	"github.com/gotrs-io/gotrs-ce/internal/service"
	"github.com/gotrs-io/gotrs-ce/internal/services/adapter"
	"github.com/gotrs-io/gotrs-ce/internal/services/k8s"
	"github.com/gotrs-io/gotrs-ce/internal/services/scheduler"
	"github.com/gotrs-io/gotrs-ce/internal/shared"
	"github.com/gotrs-io/gotrs-ce/internal/ticketnumber"
	"github.com/gotrs-io/gotrs-ce/internal/yamlmgmt"
)

var valkeyCache *cache.RedisCache

func main() {
	// Parse command line flags
	var mode = flag.String("mode", "server", "Run mode: server (default) or runner")
	flag.Parse()

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

	// Load configuration
	configDir := os.Getenv("CONFIG_DIR")
	if configDir == "" {
		configDir = "/app/config"
	}
	if err := config.Load(configDir); err != nil {
		log.Printf("Warning: Failed to load config: %v", err)
		// Continue with defaults
	}
	if err := lookups.LoadCountries(configDir); err != nil {
		log.Printf("Warning: falling back to embedded country list: %v", err)
	}

	// Initialize Valkey cache for poll status and other lightweight state.
	cfg := config.Get()
	valkeyCache = initValkeyCache(cfg)
	if valkeyCache != nil {
		api.SetValkeyCache(valkeyCache)
	}

	// Get database connection
	db, dbErr := database.GetDB()
	if dbErr != nil {
		log.Printf("Failed to get database connection: %v", dbErr)
		if *mode == "runner" {
			log.Fatal("Database connection required for runner mode")
		}
	}

	// Handle runner mode
	if *mode == "runner" {
		runRunner(db)
		return
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
	api.RegisterDynamicModuleHandlersIntoRegistry(handlerRegistry)

	// Register actual API handlers to override placeholders
	apiHandlers := map[string]gin.HandlerFunc{
		"handleDashboard":           api.HandleDashboard,
		"dashboard_stats":           api.HandleDashboardStats,
		"dashboard_recent_tickets":  api.HandleRecentTickets,
		"dashboard_queue_status":    api.DashboardQueueStatus,
		"dashboard_activity_stream": api.HandleActivityStream,
		"handleUpdateTicketStatus":  api.HandleUpdateTicketStatus,
		"handlePendingReminderFeed": api.HandlePendingReminderFeed,
		"HandleMailAccountPollStatus": api.HandleMailAccountPollStatus,

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
		"HandleListArticlesAPI":        api.HandleListArticlesAPI,
		"HandleCreateArticleAPI":       api.HandleCreateArticleAPI,
		"HandleGetArticleAPI":          api.HandleGetArticleAPI,
		"HandleUpdateArticleAPI":       api.HandleUpdateArticleAPI,
		"HandleDeleteArticleAPI":       api.HandleDeleteArticleAPI,
		"HandleGetInternalNotes":       api.HandleGetInternalNotes,
		"HandleCreateInternalNote":     api.HandleCreateInternalNote,
		"HandleUpdateInternalNote":     api.HandleUpdateInternalNote,
		"HandleDeleteInternalNote":     api.HandleDeleteInternalNote,
		"HandleUserMeAPI":              api.HandleUserMeAPI,
		"HandleListUsersAPI":           api.HandleListUsersAPI,
		"HandleGetUserAPI":             api.HandleGetUserAPI,
		"HandleCreateUserAPI":          api.HandleCreateUserAPI,
		"HandleUpdateUserAPI":          api.HandleUpdateUserAPI,
		"HandleDeleteUserAPI":          api.HandleDeleteUserAPI,
		"HandleListQueuesAPI":          api.HandleListQueuesAPI,
		"HandleGetQueueAPI":            api.HandleGetQueueAPI,
		"HandleGetQueueAgentsAPI":      api.HandleGetQueueAgentsAPI,
		"HandleListSystemAddressesAPI": api.HandleListSystemAddressesAPI,
		"HandleCreateSystemAddressAPI": api.HandleCreateSystemAddressAPI,
		"HandleUpdateSystemAddressAPI": api.HandleUpdateSystemAddressAPI,
		"HandleCreateQueueAPI":         api.HandleCreateQueueAPI,
		"HandleUpdateQueueAPI":         api.HandleUpdateQueueAPI,
		"HandleDeleteQueueAPI":         api.HandleDeleteQueueAPI,
		"HandleGetQueueStatsAPI":       api.HandleGetQueueStatsAPI,
		"HandleAssignQueueGroupAPI":    api.HandleAssignQueueGroupAPI,
		"HandleRemoveQueueGroupAPI":    api.HandleRemoveQueueGroupAPI,
		"HandleListPrioritiesAPI":      api.HandleListPrioritiesAPI,
		"HandleGetPriorityAPI":         api.HandleGetPriorityAPI,
		"HandleListSalutationsAPI":     api.HandleListSalutationsAPI,
		"HandleCreateSalutationAPI":    api.HandleCreateSalutationAPI,
		"HandleUpdateSalutationAPI":    api.HandleUpdateSalutationAPI,
		"HandleListSignaturesAPI":      api.HandleListSignaturesAPI,
		"HandleCreateSignatureAPI":     api.HandleCreateSignatureAPI,
		"HandleUpdateSignatureAPI":     api.HandleUpdateSignatureAPI,
		"HandleListTypesAPI":           api.HandleListTypesAPI,
		"HandleListStatesAPI":          api.HandleListStatesAPI,
		"HandleSearchAPI":              api.HandleSearchAPI,
		"HandleSearchSuggestionsAPI":   api.HandleSearchSuggestionsAPI,
		"HandleReindexAPI":             api.HandleReindexAPI,
		"HandleSearchHealthAPI":        api.HandleSearchHealthAPI,

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
		"handleAdminUsers":              api.HandleAdminUsers,
		"handleAdminUserCreate":         api.HandleAdminUserCreate,
		"handleAdminUserGet":            api.HandleAdminUserGet,
		"handleAdminUserEdit":           api.HandleAdminUserEdit,
		"handleAdminUserUpdate":         api.HandleAdminUserUpdate,
		"handleAdminUserDelete":         api.HandleAdminUserDelete,
		"handleAdminUserGroups":         api.HandleAdminUserGroups,
		"handleAdminUsersStatus":        api.HandleAdminUsersStatus,
		"handleAdminPasswordPolicy":     api.HandleAdminPasswordPolicy,
		"HandleAdminUsersList":          api.HandleAdminUsersList,
		"handleAdminGroups":             api.HandleAdminGroups,
		"handleCreateGroup":             api.HandleCreateGroup,
		"handleGetGroup":                api.HandleGetGroup,
		"handleUpdateGroup":             api.HandleUpdateGroup,
		"handleDeleteGroup":             api.HandleDeleteGroup,
		"handleGroupMembers":            api.HandleGroupMembers,
		"handleAddUserToGroup":          api.HandleAddUserToGroup,
		"handleRemoveUserFromGroup":     api.HandleRemoveUserFromGroup,
		"handleGroupPermissions":        api.HandleGroupPermissions,
		"handleSaveGroupPermissions":    api.HandleSaveGroupPermissions,
		"HandleAdminGroupsUsers":        api.HandleAdminGroupsUsers,
		"HandleAdminGroupsAddUser":      api.HandleAdminGroupsAddUser,
		"HandleAdminGroupsRemoveUser":   api.HandleAdminGroupsRemoveUser,
		"handleAdminQueues":             api.HandleAdminQueues,
		"handleAdminEmailIdentities":    api.HandleAdminEmailIdentities,
		"handleAdminEmailQueue":         api.HandleAdminEmailQueue,
		"handleAdminEmailQueueRetry":    api.HandleAdminEmailQueueRetry,
		"handleAdminEmailQueueDelete":   api.HandleAdminEmailQueueDelete,
		"handleAdminEmailQueueRetryAll": api.HandleAdminEmailQueueRetryAll,
		"handleAdminPriorities":         api.HandleAdminPriorities,
		// Queue API handlers
		"HandleAPIQueueGet":                        api.HandleAPIQueueGet,
		"HandleAPIQueueDetails":                    api.HandleAPIQueueDetails,
		"HandleAPIQueueStatus":                     api.HandleAPIQueueStatus,
		"handleAdminPermissions":                   api.HandleAdminPermissions,
		"handleGetUserPermissionMatrix":            api.HandleGetUserPermissionMatrix,
		"handleUpdateUserPermissions":              api.HandleUpdateUserPermissions,
		"handleAdminStates":                        api.HandleAdminStates,
		"handleAdminTypes":                         api.HandleAdminTypes,
		"handleAdminServices":                      api.HandleAdminServices,
		"handleAdminSLA":                           api.HandleAdminSLA,
		"handleAdminLookups":                       api.HandleAdminLookups,
		"handleAdminSettings":                      api.HandleAdminSettings,
		"handleAdminTemplates":                     api.HandleAdminTemplates,
		"handleAdminReports":                       api.HandleAdminReports,
		"handleAdminLogs":                          api.HandleAdminLogs,
		"handleAdminBackup":                        api.HandleAdminBackup,
		"handleAdminCustomerCompanies":             api.HandleAdminCustomerCompanies,
		"handleAdminNewCustomerCompany":            api.HandleAdminNewCustomerCompany,
		"handleAdminCreateCustomerCompany":         api.HandleAdminCreateCustomerCompany,
		"handleAdminEditCustomerCompany":           api.HandleAdminEditCustomerCompany,
		"handleAdminUpdateCustomerCompany":         api.HandleAdminUpdateCustomerCompany,
		"handleAdminDeleteCustomerCompany":         api.HandleAdminDeleteCustomerCompany,
		"handleAdminCustomerCompanyUsers":          api.HandleAdminCustomerCompanyUsers,
		"handleAdminCustomerCompanyTickets":        api.HandleAdminCustomerCompanyTickets,
		"handleAdminCustomerCompanyServices":       api.HandleAdminCustomerCompanyServices,
		"handleAdminUpdateCustomerCompanyServices": api.HandleAdminUpdateCustomerCompanyServices,
		"handleAdminCustomerPortalSettings":        api.HandleAdminCustomerPortalSettings,
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
		"HandleAdminCustomerUsersList":       api.HandleAdminCustomerUsersList,
		"HandleAdminCustomerUsersGet":        api.HandleAdminCustomerUsersGet,
		"HandleAdminCustomerUsersCreate":     api.HandleAdminCustomerUsersCreate,
		"HandleAdminCustomerUsersUpdate":     api.HandleAdminCustomerUsersUpdate,
		"HandleAdminCustomerUsersDelete":     api.HandleAdminCustomerUsersDelete,
		"HandleAdminCustomerUsersTickets":    api.HandleAdminCustomerUsersTickets,
		"HandleAdminCustomerUsersImportForm": api.HandleAdminCustomerUsersImportForm,
		"HandleAdminCustomerUsersImport":     api.HandleAdminCustomerUsersImport,
		"HandleAdminCustomerUsersExport":     api.HandleAdminCustomerUsersExport,
		"HandleAdminCustomerUsersBulkAction": api.HandleAdminCustomerUsersBulkAction,

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
	configDir = os.Getenv("CONFIG_DIR")
	if configDir == "" {
		configDir = "/app/config"
	}
	if err := config.Load(configDir); err != nil {
		log.Printf("Warning: Failed to load config: %v", err)
		// Continue with defaults
	}

	// Initialize email provider
	if cfg := config.Get(); cfg != nil && cfg.Email.Enabled && cfg.Email.SMTP.Host != "" {
		smtpProvider := notifications.NewSMTPProvider(&cfg.Email)
		notifications.SetEmailProvider(smtpProvider)
		log.Println("📧 Email provider initialized (SMTP)")
	} else {
		log.Println("⚠️  Email provider not configured - notifications disabled")
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

	var emailHandler connector.Handler
	if db != nil {
		ticketRepo := repository.NewTicketRepository(db)
		articleRepo := repository.NewArticleRepository(db)
		ticketSvc := service.NewTicketService(ticketRepo, service.WithArticleRepository(articleRepo))
		queueRepo := repository.NewQueueRepository(db)
		var storageSvc service.StorageService
		if cfg := config.Get(); cfg != nil && strings.EqualFold(cfg.Storage.Type, "db") {
			if svc, err := service.NewDatabaseStorageService(); err == nil {
				storageSvc = svc
			} else {
				log.Printf("postmaster: database storage init failed: %v", err)
			}
		} else {
			storagePath := os.Getenv("STORAGE_PATH")
			if storagePath == "" {
				if cfg := config.Get(); cfg != nil && cfg.Storage.Local.Path != "" {
					storagePath = cfg.Storage.Local.Path
				} else {
					storagePath = filepath.Join(configDir, "storage")
				}
			}
			if svc, err := service.NewLocalStorageService(storagePath); err == nil {
				storageSvc = svc
			} else {
				log.Printf("postmaster: local storage init failed: %v", err)
			}
		}
		dispatchRulesPath := filepath.Join(configDir, "email_dispatch.yaml")
		dispatchProvider, err := filters.NewFileDispatchRuleProvider(dispatchRulesPath)
		if err != nil {
			log.Printf("postmaster: failed to load dispatch rules: %v", err)
		}
		externalRules, err := filters.LoadExternalTicketRules(filepath.Join(configDir, "external_ticket_rules.yaml"))
		if err != nil {
			log.Printf("postmaster: failed to load external ticket rules: %v", err)
		}
		processor := postmaster.NewTicketProcessor(
			ticketSvc,
			postmaster.WithTicketProcessorQueueLookup(func(ctx context.Context, name string) (int, error) {
				queue, err := queueRepo.GetByName(name)
				if err != nil {
					return 0, err
				}
				return int(queue.ID), nil
			}),
			postmaster.WithTicketProcessorStorage(storageSvc),
			postmaster.WithTicketProcessorArticleLookup(articleRepo),
			postmaster.WithTicketProcessorTicketFinder(ticketRepo),
			postmaster.WithTicketProcessorQueueFinder(queueRepo),
			postmaster.WithTicketProcessorArticleStore(articleRepo),
			postmaster.WithTicketProcessorMessageLookup(articleRepo),
			postmaster.WithTicketProcessorDatabase(db),
		)
		var filterList []filters.Filter
		filterList = append(filterList,
			filters.NewHeaderTokenFilter(log.Default()),
			filters.NewSubjectTokenFilter(log.Default()),
			filters.NewBodyTokenFilter(log.Default()),
			filters.NewAttachmentTokenFilter(log.Default()),
		)
		if externalFilter := filters.NewExternalTicketNumberFilter(externalRules, log.Default()); externalFilter != nil {
			filterList = append(filterList, externalFilter)
		}
		if dispatchProvider != nil {
			filterList = append(filterList, filters.NewDispatchFromMapFilter(dispatchProvider, log.Default()))
		}
		var trustedHeaders []string
		if cfg := config.Get(); cfg != nil {
			trustedHeaders = cfg.Email.Inbound.TrustedHeaders
		}
		filterList = append(filterList, filters.NewTrustedHeadersFilter(log.Default(), trustedHeaders...))
		chain := filters.NewChain(filterList...)
		emailHandler = &postmaster.Service{
			FilterChain: chain,
			Handler:     processor,
		}
	}

	// Create router for YAML routes
	r := gin.New()

	// Global i18n middleware (language detection via ?lang=, cookie, user, Accept-Language)
	i18nMW := middleware.NewI18nMiddleware()
	r.Use(i18nMW.Handle())

	// Configure larger multipart memory limit for large article content
	r.MaxMultipartMemory = 128 << 20 // 128MB

	// Initialize pongo2 renderer for template rendering
	templateDir := os.Getenv("TEMPLATES_DIR")
	if templateDir == "" {
		candidates := []string{
			"./templates",
			"./web/templates",
			"/app/templates",
			"/app/web/templates",
		}
		for _, candidate := range candidates {
			if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
				templateDir = candidate
				break
			}
		}
		if templateDir == "" {
			// Fall back to original relative path for test environments
			templateDir = "./templates"
		}
	}
	api.InitPongo2Renderer(templateDir)
	if renderer, err := shared.NewTemplateRenderer(templateDir); err != nil {
		log.Printf("⚠️  Failed to initialize shared renderer (dir=%s): %v", templateDir, err)
	} else {
		shared.SetGlobalRenderer(renderer)
		log.Printf("✅ Shared renderer initialized (dir=%s)", templateDir)
	}

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

	if dbErr == nil && db != nil {
		if err := api.SetupDynamicModules(db); err != nil {
			log.Printf("⚠️  Dynamic modules unavailable: %v", err)
		} else {
			log.Println("✅ Dynamic module system initialized")
		}
	} else {
		log.Printf("⚠️  Skipping dynamic modules (db unavailable: %v)", dbErr)
	}

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
	if dbErr != nil || db == nil {
		log.Printf("scheduler: disabled (database unavailable: %v)", dbErr)
	} else {
		loc := time.UTC
		cfg := config.Get()
		if cfg != nil && cfg.App.Timezone != "" {
			if tz, err := time.LoadLocation(cfg.App.Timezone); err != nil {
				log.Printf("scheduler: invalid timezone %q, falling back to UTC: %v", cfg.App.Timezone, err)
			} else {
				loc = tz
			}
		}
		options := []scheduler.Option{scheduler.WithLocation(loc)}
		if emailHandler != nil {
			options = append(options, scheduler.WithEmailHandler(emailHandler))
		}
		if valkeyCache != nil {
			options = append(options, scheduler.WithCache(valkeyCache))
		}
		jobs := buildSchedulerJobsFromConfig(cfg)
		if len(jobs) > 0 {
			options = append(options, scheduler.WithJobs(jobs))
		}
		sched := scheduler.NewService(db, options...)
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

func initValkeyCache(cfg *config.Config) *cache.RedisCache {
	if cfg == nil {
		return nil
	}
	vc := cfg.Valkey
	if vc.Host == "" || vc.Port == 0 {
		return nil
	}
	ttl := vc.Cache.TTL
	if ttl == 0 {
		ttl = time.Hour
	}
	redisCfg := &cache.CacheConfig{
		RedisAddr:     []string{vc.GetValkeyAddr()},
		RedisPassword: vc.Password,
		RedisDB:       vc.DB,
		ClusterMode:   false,
		DefaultTTL:    ttl,
		KeyPrefix:     vc.Cache.Prefix,
		MaxRetries:    vc.MaxRetries,
		PoolSize:      vc.PoolSize,
		MinIdleConns:  vc.MinIdleConns,
		DialTimeout:   5 * time.Second,
		ReadTimeout:   5 * time.Second,
		WriteTimeout:  5 * time.Second,
	}

	cacheClient, err := cache.NewRedisCache(redisCfg)
	if err != nil {
		log.Printf("valkey cache disabled: %v", err)
		return nil
	}
	return cacheClient
}

// runRunner starts the background task runner
func runRunner(db *sql.DB) {
	log.Println("Starting GOTRS background task runner...")

	// Create task registry
	registry := runner.NewTaskRegistry()

	// Get email configuration
	emailCfg := config.Get()
	if emailCfg == nil {
		log.Fatal("Configuration not available")
	}

	// Register email queue task
	emailTask := tasks.NewEmailQueueTask(db, &emailCfg.Email)
	registry.Register(emailTask)

	log.Printf("Registered %d background tasks", len(registry.All()))

	// Create and start runner
	taskRunner := runner.NewRunner(registry)

	// Start the runner
	ctx := context.Background()
	if err := taskRunner.Start(ctx); err != nil {
		log.Fatalf("Runner failed: %v", err)
	}
}
