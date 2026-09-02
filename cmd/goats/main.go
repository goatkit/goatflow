// Package main provides the GOATS CLI tool.
//
//	@title			GoatFlow API
//	@version		1.0
//	@description	GoatFlow Ticket System REST API
//	@termsOfService	https://goatflow.io/terms/
//
//	@contact.name	GoatFlow Support
//	@contact.url	https://goatflow.io/support
//	@contact.email	hello@goatflow.io
//
//	@license.name	Apache-2.0
//	@license.url	https://www.apache.org/licenses/LICENSE-2.0.html
//
//	@host		localhost:8080
//	@BasePath	/api/v1
//
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				API token (Bearer gf_xxx) or JWT token

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

	"github.com/davidbyttow/govips/v2/vips"
	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/api"

	"github.com/goatkit/goatflow/internal/email/inbound/filters"
	"github.com/goatkit/goatflow/internal/email/inbound/postmaster"
	platformapi "github.com/goatkit/goatflow/internal/platform/api"
	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/cache"
	"github.com/goatkit/goatflow/internal/platform/config"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/email/inbound/connector"
	"github.com/goatkit/goatflow/internal/platform/lookups"
	"github.com/goatkit/goatflow/internal/platform/middleware"
	"github.com/goatkit/goatflow/internal/platform/notifications"
	"github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/goatkit/goatflow/internal/platform/plugin/core"
	"github.com/goatkit/goatflow/internal/platform/plugin/example"
	pluginloader "github.com/goatkit/goatflow/internal/platform/plugin/loader"
	"github.com/goatkit/goatflow/internal/platform/runner"
	platformservice "github.com/goatkit/goatflow/internal/platform/service"
	"github.com/goatkit/goatflow/internal/platform/services/adapter"
	"github.com/goatkit/goatflow/internal/platform/services/k8s"
	"github.com/goatkit/goatflow/internal/platform/shared"
	"github.com/goatkit/goatflow/internal/platform/template"
	"github.com/goatkit/goatflow/internal/platform/yamlmgmt"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/runner/tasks"
	"github.com/goatkit/goatflow/internal/service"
	"github.com/goatkit/goatflow/internal/services/scheduler"
	"github.com/goatkit/goatflow/internal/ticketnumber"
)

var valkeyCache *cache.RedisCache

func main() {
	// Initialize libvips for image processing (AVIF, HEIC, WebP, etc.)
	vips.Startup(nil)
	defer vips.Shutdown()

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

	// Run database migrations automatically on startup
	if db != nil {
		log.Println("Running database migrations...")
		applied, err := database.RunMigrations(db)
		if err != nil {
			log.Printf("⚠️  Migration warning: %v", err)
			// Don't fail startup - migrations use IF NOT EXISTS patterns
		} else if applied > 0 {
			log.Printf("✅ Applied %d migration(s)", applied)
		} else {
			log.Println("✅ Database schema is up to date")
		}

		// Initialize API token service (enables gf_* token authentication)
		api.InitAPITokenService(db)
	}

	// Handle runner mode
	if *mode == "runner" {
		runRunner(db)
		return
	}

	// First-boot admin bootstrap: on a fresh install the seeded admin
	// (root@localhost) is factory-disabled with a random password. If the
	// deployment provided GOATFLOW_ADMIN_PASSWORD (e.g. via the TrueNAS
	// install wizard), apply it as the initial admin password — strictly
	// one-shot, and never overriding a live account.
	bootstrapAdminFromEnv(db)

	// Set Gin mode
	if os.Getenv("APP_ENV") == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// Handlers are self-registered via init() into routing.GlobalHandlerMap.
	// This happens automatically when api package is imported (ensureCoreHandlers runs).
	// No manual handler registration needed here.

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
		platformservice.SetConfigAdapter(adapter)
	}
	auth.SetUserRepoFactory(func(db *sql.DB) auth.UserLookup {
		return repository.NewUserRepository(db)
	})
	template.SetMaintenanceCheckerFactory(func(db *sql.DB) template.MaintenanceChecker {
		return repository.NewSystemMaintenanceRepository(db)
	})
	template.SetContextHelpers(middleware.GetLanguage, shared.GetUserIDFromCtxUint)
	middleware.SetSessionServiceFactory(func(db *sql.DB) middleware.SessionChecker {
		return service.NewSessionService(repository.NewSessionRepository(db))
	})
	middleware.SetMaintenanceCheckerFactory(func(db *sql.DB) middleware.MaintenanceChecker {
		return repository.NewSystemMaintenanceRepository(db)
	})
	middleware.SetQueueAccessCheckerFactory(func(db *sql.DB) middleware.QueueAccessChecker {
		return service.NewQueueAccessService(db)
	})
	middleware.SetTicketQueueResolverFactory(func() middleware.TicketQueueResolver {
		return &repository.TicketQueueResolverImpl{}
	})
	shared.SetSessionManagerFactory(func(db *sql.DB) shared.SessionManager {
		return service.NewSessionService(repository.NewSessionRepository(db))
	})
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
		// DBSourceFilter runs first to apply database-configured postmaster filters
		// (equivalent to OTRS's PostMaster::PreFilterModule###000-MatchDBSource)
		filterList = append(filterList, filters.NewDBSourceFilter(db, log.Default()))
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

	// Initialize OIDC state store (in-memory) and HTTP client for IdP token exchanges
auth.SetStateStore(auth.NewMemoryStateStore())
auth.SetOIDCClient(&http.Client{Timeout: 30 * time.Second})

	// Create router for YAML routes
	r := gin.New()

	customerOnly := strings.EqualFold(os.Getenv("CUSTOMER_FE_ONLY"), "true") || os.Getenv("CUSTOMER_FE_ONLY") == "1"
	if customerOnly {
		r.Use(api.CustomerOnlyGuard(true))
		log.Println("🔒 Customer FE mode: admin routes disabled")

		r.NoRoute(func(c *gin.Context) {
			c.Redirect(http.StatusFound, api.RootRedirectTarget())
		})
	}

	// Security headers (CSP, X-Frame-Options, X-Content-Type-Options, etc.)
	r.Use(middleware.SecurityHeaders())

	// Demo mode middleware (sets is_demo context on all requests when enabled)
	r.Use(middleware.DemoMode())

	// Global i18n middleware (language detection via ?lang=, cookie, user, Accept-Language)
	i18nMW := middleware.NewI18nMiddleware()
	r.Use(i18nMW.Handle())

	// Configure larger multipart memory limit for large article content
	r.MaxMultipartMemory = 128 << 20 // 128MB

	// Initialize template renderer
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
	if renderer, err := shared.NewTemplateRenderer(templateDir); err != nil {
		log.Printf("⚠️  Failed to initialize template renderer (dir=%s): %v", templateDir, err)
	} else {
		shared.SetGlobalRenderer(renderer)
		log.Printf("✅ Template renderer initialized (dir=%s)", templateDir)
	}

	// Initialize plugin system
	log.Println("🔌 Initializing plugin system...")
	pluginHostOpts := []plugin.ProdHostAPIOption{}
	if db != nil {
		pluginHostOpts = append(pluginHostOpts, plugin.WithDB("default", db))
	}
	if valkeyCache != nil {
		pluginHostOpts = append(pluginHostOpts, plugin.WithCache(valkeyCache))
	}
	// Thumbnail service — server-side image thumbnails via libvips (govips v2.16.0).
	// govips is initialised above (vips.Startup) and the Dockerfile ships vips-dev vips-heif.
	// No Redis cache here: the wired closure calls GenerateThumbnail directly; the
	// GetOrCreateThumbnail cache path is unused until a caller adopts it.
	thumbSvc := service.NewThumbnailService(nil)
	pluginHostOpts = append(pluginHostOpts, plugin.WithThumbnailService(
		plugin.NewThumbnailGenerator(func(data []byte, contentType string, maxWidth, maxHeight int) ([]byte, string, error) {
			return thumbSvc.GenerateThumbnail(data, contentType, service.ThumbnailOptions{
				Width: maxWidth, Height: maxHeight, Quality: 85, Format: "jpeg",
			})
		}),
	))
	pluginHost := plugin.NewProdHostAPI(pluginHostOpts...)
	sseBroker := plugin.NewSSEBroker()
	pluginHost.SSEBroker = sseBroker
	api.SetPluginSSEBroker(sseBroker)
	pluginMgr := plugin.NewManager(pluginHost)
	// Wire PluginManager back to HostAPI for plugin-to-plugin calls
	pluginHost.PluginManager = pluginMgr
	api.SetPluginManager(pluginMgr)
	plugin.SetTemplatePluginManager(pluginMgr) // Enable {% use %} template tag
	templateOverrides := plugin.NewTemplateOverrideRegistry(pluginMgr)
	plugin.SetTemplateOverrides(templateOverrides)
	shared.SetTemplateOverrideProvider(templateOverrides) // Enable template overrides

	// Install the plugin-access checker so the template renderer can
	// filter customer-facing nav items against per-user entitlement
	// (see SetPluginAccessChecker docstring).
	shared.SetPluginAccessChecker(api.HasPluginAccess)

	// Wire the captive-plugin landing resolver so CustomerPortalGate can
	// redirect a captive org's customers to the right plugin path. Uses
	// the same plugin manager as every other plugin-related lookup.
	middleware.SetCaptivePluginLandingResolver(func(name string) string {
		if mgr := api.GetPluginManager(); mgr != nil {
			return mgr.LandingPageFor(name)
		}
		return ""
	})

	// Provide plugin menu items to all templates (sidebar/nav)
	shared.SetPluginMenuProvider(func(location string) []map[string]any {
		items := pluginMgr.MenuItems(location)
		result := make([]map[string]any, 0, len(items))
		for _, item := range items {
			entry := map[string]any{
				"PluginName": item.PluginName,
				"ID":         item.MenuItemSpec.ID,
				"Label":      item.MenuItemSpec.Label,
				"Icon":       item.MenuItemSpec.Icon,
				"Path":       item.MenuItemSpec.Path,
				"Order":      item.MenuItemSpec.Order,
			}
			if len(item.MenuItemSpec.Children) > 0 {
				children := make([]map[string]any, 0, len(item.MenuItemSpec.Children))
				for _, child := range item.MenuItemSpec.Children {
					children = append(children, map[string]any{
						"ID":    child.ID,
						"Label": child.Label,
						"Icon":  child.Icon,
						"Path":  child.Path,
					})
				}
				entry["Children"] = children
			}
			result = append(result, entry)
		}
		return result
	})

	// Provide hidden menu items to all templates (nav control by plugins)
	shared.SetHiddenMenuProvider(func() map[string]bool {
		items := pluginMgr.HiddenMenuItems()
		result := make(map[string]bool, len(items))
		for _, id := range items {
			result[id] = true
		}
		return result
	})

	// Provide plugin landing page for post-login redirect
	shared.SetLandingPageProvider(func() string {
		return pluginMgr.LandingPage()
	})

	// Register built-in example plugin (for development/testing)
	helloPlugin := example.NewHelloPlugin()
	if err := pluginMgr.Register(context.Background(), helloPlugin); err != nil {
		log.Printf("⚠️  Failed to register hello plugin: %v", err)
	} else {
		log.Printf("✅ Plugin registered: %s v%s", helloPlugin.GKRegister().Name, helloPlugin.GKRegister().Version)
	}

	// Register core dashboard widgets plugin
	dashboardPlugin := core.NewDashboardPlugin()
	if err := pluginMgr.Register(context.Background(), dashboardPlugin); err != nil {
		log.Printf("⚠️  Failed to register dashboard-core plugin: %v", err)
	} else {
		log.Printf("✅ Plugin registered: %s v%s", dashboardPlugin.GKRegister().Name, dashboardPlugin.GKRegister().Version)
	}

	// Load WASM plugins from plugins directory
	pluginDir := os.Getenv("PLUGIN_DIR")
	if pluginDir == "" {
		pluginDir = filepath.Join(configDir, "plugins")
	}
	api.SetPluginDir(pluginDir) // Enable plugin uploads

	// Configure loader options
	var loaderOpts []pluginloader.LoaderOption
	if os.Getenv("GOATFLOW_PLUGIN_LAZY_LOAD") == "true" {
		loaderOpts = append(loaderOpts, pluginloader.WithLazyLoading())
	}

	pluginLoader := pluginloader.NewLoader(pluginDir, pluginMgr, nil, loaderOpts...)
	loadedCount, loadErrs := pluginLoader.LoadAll(context.Background())

	// Wire lazy loader to manager for on-demand loading
	pluginMgr.SetLazyLoader(pluginLoader)
	api.SetPluginReloader(pluginLoader.LoadOrReload) // Enable load/reload on upload
	api.SetPluginUnloader(pluginLoader.Unload)       // Enable stop before binary replacement

	if os.Getenv("GOATFLOW_PLUGIN_LAZY_LOAD") == "true" {
		log.Printf("🔌 Discovered %d WASM plugin(s) (lazy loading enabled)", loadedCount)
	} else if loadedCount > 0 {
		log.Printf("✅ Loaded %d WASM plugin(s) from %s", loadedCount, pluginDir)
	}
	for _, err := range loadErrs {
		log.Printf("⚠️  Plugin load error: %v", err)
	}

	// Enable hot reload for plugins in development mode
	// Hot reload: enabled by default, disable with GOATFLOW_PLUGIN_HOT_RELOAD=false
	if os.Getenv("GOATFLOW_PLUGIN_HOT_RELOAD") != "false" {
		if err := pluginLoader.WatchDir(context.Background()); err != nil {
			log.Printf("⚠️  Plugin hot reload disabled: %v", err)
		}
	}

	// Plugin health checker: periodically probes every loaded plugin
	// and marks it unhealthy after N consecutive failures. Surfaces
	// silent zombie plugins (gRPC channel alive but process wedged)
	// that otherwise only get noticed when user traffic hits them.
	//
	// Auto-recovery: with the loader wired as the Restarter, unhealthy
	// plugins are auto-reloaded with exponential backoff (5s → 5min)
	// and a crash-loop guard (>5 attempts in 10min → abandon, requires
	// admin reset). Disable health checking entirely with
	// GOATFLOW_PLUGIN_HEALTH_CHECK=false; disable just auto-restart with
	// GOATFLOW_PLUGIN_AUTO_RESTART=false.
	if os.Getenv("GOATFLOW_PLUGIN_HEALTH_CHECK") != "false" {
		if os.Getenv("GOATFLOW_PLUGIN_AUTO_RESTART") != "false" {
			pluginMgr.SetRestarter(pluginLoader)
		}
		healthStop := pluginMgr.StartHealthChecker(0, 0) // defaults (60s interval, 5s probe)
		defer healthStop()
	}

	// Register plugin-defined routes with the router
	// (done later after router is created)

	// Load YAML routes through the API-backed routing resolver.
	routesDir := os.Getenv("ROUTES_DIR")
	if routesDir == "" {
		routesDir = "/app/routes"
	}

	// Mount unified dynamic engine (YAML routes + plugin routes, hot-reloadable)
	api.MountDynamicEngine(r, routesDir)

	// Auto-create groups declared by plugins (idempotent).
	api.EnsurePluginGroups()

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

	// Scheduler ownership: the customer-facing frontend is UI-only. Running
	// the scheduler there duplicates cron jobs against the main app (built-in
	// and plugin-registered alike) — same-DB races, wasted CPU, and silent
	// row loss when a plugin uses `INSERT IGNORE` to dedupe composite keys.
	var schedulerCancel context.CancelFunc
	switch {
	case customerOnly:
		log.Println("scheduler: disabled (CUSTOMER_FE_ONLY)")
	case dbErr != nil || db == nil:
		log.Printf("scheduler: disabled (database unavailable: %v)", dbErr)
	default:
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

		pluginAdapter := scheduler.NewPluginAdapter(sched)
		if pluginJobCount := plugin.RegisterPluginJobs(pluginMgr, pluginAdapter); pluginJobCount > 0 {
			log.Printf("✅ Registered %d plugin job(s) with scheduler", pluginJobCount)
		}

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
	i18nHandlers := platformapi.NewI18nHandlers()
	i18nHandlers.RegisterRoutes(v1Group)

	// Register plugin management API routes
	api.RegisterPluginAPIRoutes(v1Group)

	// Direct debug route for ticket number generator introspection
	r.GET("/admin/debug/ticket-number", api.HandleDebugTicketNumber)
	// Config sources introspection
	r.GET("/admin/debug/config-sources", api.HandleDebugConfigSources)

	// SSE endpoint for real-time plugin updates
	r.GET("/api/v1/sse", gin.WrapF(sseBroker.ServeHTTP))

	// Example of using generator early (warm path) – ensure repository updated elsewhere to accept it
	_ = ticketNumGen

	// Start server
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Starting GoatFlow HTMX server on port %s\n", port)
	fmt.Println("Available routes:")
	fmt.Printf("  GET  /          -> Redirect to %s\n", api.RootRedirectTarget())
	fmt.Println("  GET  /customer  -> Customer dashboard")
	fmt.Println("  GET  /customer/login -> Customer login page")
	fmt.Println("  POST /customer/login -> Customer login submit")
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
		// Stop plugin hot reload watcher
		pluginLoader.StopWatch()
		// Shutdown plugins gracefully — bounded so one hung plugin
		// can't delay the process exit indefinitely. Per-plugin
		// timeouts are set from each plugin's ResourcePolicy inside
		// ShutdownAll; this outer deadline is the hard cap.
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
		if err := pluginMgr.ShutdownAll(shutdownCtx); err != nil {
			log.Printf("⚠️  Plugin shutdown error: %v", err)
		}
		shutdownCancel()
		log.Fatalf("server failed: %v", err)
	}
	if schedulerCancel != nil {
		schedulerCancel()
	}
	// Stop plugin hot reload watcher
	pluginLoader.StopWatch()
	// Shutdown plugins gracefully — see note above.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	if err := pluginMgr.ShutdownAll(shutdownCtx); err != nil {
		log.Printf("⚠️  Plugin shutdown error: %v", err)
	}
	shutdownCancel()
}

// gracefulShutdownTimeout is the overall ceiling on how long goatflow
// will wait for plugins to shut down before exiting. Per-plugin
// deadlines are shorter (from each plugin's ResourcePolicy) and are
// applied inside Manager.ShutdownAll; this value just bounds the total.
const gracefulShutdownTimeout = 30 * time.Second

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

// runRunner starts the background task runner.
func runRunner(db *sql.DB) {
	log.Println("Starting GoatFlow background task runner...")

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

	// Register session cleanup task
	sessionCleanupTask := tasks.NewSessionCleanupTask(db)
	registry.Register(sessionCleanupTask)

	log.Printf("Registered %d background tasks", len(registry.All()))

	// Create and start runner
	taskRunner := runner.NewRunner(registry)

	// Start the runner
	ctx := context.Background()
	if err := taskRunner.Start(ctx); err != nil {
		log.Fatalf("Runner failed: %v", err)
	}
}
