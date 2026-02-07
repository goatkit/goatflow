package api

import (
	"context"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/goatkit/goatflow/internal/config"
	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/service"
	"github.com/goatkit/goatflow/internal/shared"
)

var (
	ticketRepo          repository.ITicketRepository
	queueRepo           *repository.QueueRepository
	priorityRepo        *repository.PriorityRepository
	userRepo            *repository.UserRepository
	simpleTicketService *service.SimpleTicketService
	storageService      service.StorageService
	lookupService       *service.LookupService
	authService         *service.AuthService

	servicesMu          sync.Mutex
	servicesInitialized bool
	servicesDB          *sql.DB
	servicesOverride    bool
)

// InitializeServices initializes singleton service instances.
func InitializeServices() {
	currentOverride := database.IsTestDBOverride()
	db, dbErr := database.GetDB()
	var pingErr error
	if db != nil {
		pingErr = pingDatabase(db)
		if pingErr != nil {
			db = nil
		}
	}

	servicesMu.Lock()
	defer servicesMu.Unlock()

	if !needsServiceRebuildLocked(db, currentOverride) {
		return
	}

	clearServicesLocked()

	env := strings.ToLower(os.Getenv("APP_ENV"))

	if db == nil {
		if env == "test" {
			initFallbackServicesLocked()
			servicesInitialized = true
			servicesDB = nil
			servicesOverride = currentOverride
			return
		}
		if pingErr != nil {
			log.Fatalf("FATAL: Cannot initialize services without database connection: %v", pingErr)
		}
		log.Fatalf("FATAL: Cannot initialize services without database connection: %v", dbErr)
	}

	if env != "test" && pingErr != nil {
		log.Fatalf("FATAL: database ping failed: %v", pingErr)
	}

	initDatabaseServicesLocked(db)
	servicesInitialized = true
	servicesDB = db
	servicesOverride = currentOverride
}

// GetTicketService returns the singleton simple ticket service instance.
func GetTicketService() *service.SimpleTicketService {
	InitializeServices()
	return simpleTicketService
}

// GetStorageService returns the singleton storage service instance.
func GetStorageService() service.StorageService {
	InitializeServices()
	return storageService
}

// GetTicketRepository returns the singleton ticket repository instance.
func GetTicketRepository() repository.ITicketRepository {
	InitializeServices()
	return ticketRepo
}

// GetLookupService returns the singleton lookup service instance.
func GetLookupService() *service.LookupService {
	InitializeServices()
	return lookupService
}

// GetQueueRepository returns the singleton queue repository instance.
func GetQueueRepository() *repository.QueueRepository {
	InitializeServices()
	return queueRepo
}

// GetPriorityRepository returns the singleton priority repository instance.
func GetPriorityRepository() *repository.PriorityRepository {
	InitializeServices()
	return priorityRepo
}

// GetUserRepository returns the singleton user repository instance.
func GetUserRepository() *repository.UserRepository {
	InitializeServices()
	return userRepo
}

// GetAuthService returns the singleton auth service instance.
func GetAuthService() *service.AuthService {
	InitializeServices()
	return authService
}

func needsServiceRebuildLocked(db *sql.DB, override bool) bool {
	if !servicesInitialized {
		return true
	}
	if servicesOverride != override {
		return true
	}
	if servicesDB != db {
		return true
	}
	return false
}

func pingDatabase(db *sql.DB) error {
	if db == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	return db.PingContext(ctx)
}

func clearServicesLocked() {
	ticketRepo = nil
	queueRepo = nil
	priorityRepo = nil
	userRepo = nil
	simpleTicketService = nil
	storageService = nil
	lookupService = nil
	authService = nil
	servicesDB = nil
	servicesOverride = false
	servicesInitialized = false
}

func initFallbackServicesLocked() {
	log.Printf("InitializeServices: using lightweight test services (database unavailable)")
	storagePath := os.Getenv("STORAGE_PATH")
	if storagePath == "" {
		storagePath = "/tmp"
	}
	if ss, err := service.NewLocalStorageService(storagePath); err == nil {
		storageService = ss
	} else {
		log.Printf("WARNING: storage init failed in test: %v", err)
		storageService = nil
	}
	simpleTicketService = service.NewSimpleTicketService(nil)
	lookupService = service.NewLookupService()
}

func initDatabaseServicesLocked(db *sql.DB) {
	ticketRepo = repository.NewTicketRepository(db)
	queueRepo = repository.NewQueueRepository(db)
	priorityRepo = repository.NewPriorityRepository(db)
	userRepo = repository.NewUserRepository(db)

	simpleTicketService = service.NewSimpleTicketService(ticketRepo)
	lookupService = service.NewLookupService()

	cfg := config.Get()
	var err error
	if cfg != nil && cfg.Storage.Type == "db" {
		storageService, err = service.NewDatabaseStorageService()
		if err != nil {
			log.Fatalf("FATAL: Cannot initialize DB storage service: %v", err)
		}
		log.Printf("StorageService: using DB backend")
	} else {
		storagePath := resolveStoragePath(cfg)
		storageService, err = service.NewLocalStorageService(storagePath)
		if err != nil {
			log.Fatalf("FATAL: Cannot initialize storage service: %v", err)
		}
		log.Printf("StorageService: using local backend at %s", storagePath)
	}

	jwtManager := shared.GetJWTManager()
	authService = service.NewAuthService(db, jwtManager)
	log.Printf("Successfully connected to database")
}

func resolveStoragePath(cfg *config.Config) string {
	if env := os.Getenv("STORAGE_PATH"); env != "" {
		return env
	}

	if cfg != nil && cfg.Storage.Local.Path != "" {
		return cfg.Storage.Local.Path
	}

	if isTestProcess() {
		if tmp, err := os.MkdirTemp("", "goatflow-storage-"); err == nil {
			return tmp
		}
	}

	if root := findRepoRoot(); root != "" {
		return filepath.Join(root, "storage")
	}

	return "./storage"
}

func isTestProcess() bool {
	return strings.HasSuffix(os.Args[0], ".test")
}

func findRepoRoot() string {
	dir, err := os.Getwd()
	for err == nil {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}
