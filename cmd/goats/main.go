// Package main provides the GOATS CLI tool.
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

	"github.com/gotrs-io/gotrs-ce/internal/api"

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

	customerOnly := strings.EqualFold(os.Getenv("CUSTOMER_FE_ONLY"), "true") || os.Getenv("CUSTOMER_FE_ONLY") == "1"
	if customerOnly {
		r.Use(api.CustomerOnlyGuard(true))
		log.Println("🔒 Customer FE mode: admin routes disabled")

		r.NoRoute(func(c *gin.Context) {
			c.Redirect(http.StatusFound, api.RootRedirectTarget())
		})
	}

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

	// Load YAML routes using GlobalHandlerMap (handlers self-register via init())
	routesDir := os.Getenv("ROUTES_DIR")
	if routesDir == "" {
		routesDir = "/app/routes"
	}

	if err := routing.LoadYAMLRoutesFromGlobalMap(r, routesDir); err != nil {
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

// runRunner starts the background task runner.
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
