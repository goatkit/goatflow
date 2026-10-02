package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
	platformservice "github.com/goatkit/goatflow/internal/platform/service"
	"github.com/goatkit/goatflow/internal/platform/shared"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/service"
)

var (
	ticketRepo          repository.ITicketRepository
	queueRepo           *repository.QueueRepository
	priorityRepo        *repository.PriorityRepository
	userRepo            *repository.UserRepository
	simpleTicketService *service.SimpleTicketService
	lookupService       *service.LookupService
	authService         *platformservice.AuthService

	servicesMu          sync.Mutex
	servicesInitialized bool
	servicesDB          *sql.DB
	servicesOverride    bool
)

// InitializeServices (re)builds the singleton services for the current
// database handle. Without a reachable database the DB-backed services stay
// nil and the getters return nil, so handlers report the outage (HTTP 500)
// instead of serving fake data; the next call after the database returns
// rebuilds them.
func InitializeServices() {
	currentOverride := database.IsTestDBOverride()
	db, dbErr := database.GetDB()
	if dbErr == nil && db == nil {
		dbErr = errors.New("database connection is nil")
	}
	if db != nil {
		if pingErr := pingDatabase(db); pingErr != nil {
			dbErr = fmt.Errorf("database ping failed: %w", pingErr)
			db = nil
		}
	}

	servicesMu.Lock()
	defer servicesMu.Unlock()

	if !needsServiceRebuildLocked(db, currentOverride) {
		return
	}

	clearServicesLocked()
	// LookupService resolves its own connection per call.
	lookupService = service.NewLookupService()
	servicesInitialized = true
	servicesOverride = currentOverride

	if db == nil {
		log.Printf("InitializeServices: database unavailable, DB-backed services not initialised: %v", dbErr)
		return
	}

	initDatabaseServicesLocked(db)
	servicesDB = db
}

// GetTicketService returns the singleton simple ticket service instance.
func GetTicketService() *service.SimpleTicketService {
	InitializeServices()
	return simpleTicketService
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
func GetAuthService() *platformservice.AuthService {
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
	lookupService = nil
	authService = nil
	servicesDB = nil
	servicesOverride = false
	servicesInitialized = false
}

func initDatabaseServicesLocked(db *sql.DB) {
	ticketRepo = repository.NewTicketRepository(db)
	queueRepo = repository.NewQueueRepository(db)
	priorityRepo = repository.NewPriorityRepository(db)
	userRepo = repository.NewUserRepository(db)

	simpleTicketService = service.NewSimpleTicketService(ticketRepo, db)

	// Initialize OIDC support
	auth.SetStateStore(auth.NewMemoryStateStore())
	oidcClient := &http.Client{Timeout: 30 * time.Second}
	auth.SetOIDCClient(oidcClient)

	jwtManager := shared.GetJWTManager()
	authService = platformservice.NewAuthService(db, jwtManager, oidcClient, auth.GetStateStore())
	log.Printf("Successfully connected to database")
}
