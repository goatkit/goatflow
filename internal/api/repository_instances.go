package api

import (
	"log"
	"sync"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/service"
)

var (
	ticketRepo        repository.ITicketRepository
	queueRepo         *repository.QueueRepository
	userRepo          *repository.UserRepository
	simpleTicketService *service.SimpleTicketService
	storageService    service.StorageService
	lookupService     *service.LookupService
	authService       *service.AuthService
	once              sync.Once
)

// InitializeServices initializes singleton service instances
func InitializeServices() {
	once.Do(func() {
		// Get database connection
		db, err := database.GetDB()
		if err != nil {
			log.Printf("Warning: Could not connect to database, using in-memory repositories: %v", err)
			// Fallback to memory repositories
			ticketRepo = repository.NewMemoryTicketRepository()
		} else {
			log.Printf("Successfully connected to database")
			// Initialize real database repositories
			ticketRepo = repository.NewTicketRepository(db)
			queueRepo = repository.NewQueueRepository(db)
			userRepo = repository.NewUserRepository(db)
		}
		
		// Initialize services
		simpleTicketService = service.NewSimpleTicketService(ticketRepo)
		
		// Initialize lookup service (this will connect to database if available)
		lookupService = service.NewLookupService()
		
		// Initialize storage service
		storageService, err = service.NewLocalStorageService("./storage")
		if err != nil {
			// Fallback to temp directory if storage dir can't be created
			storageService, _ = service.NewLocalStorageService("/tmp/gotrs-storage")
		}
		
		// Initialize auth service if database is available
		if db != nil {
			authService = service.NewAuthService(db)
		}
	})
}

// GetTicketService returns the singleton simple ticket service instance
func GetTicketService() *service.SimpleTicketService {
	InitializeServices()
	return simpleTicketService
}

// GetStorageService returns the singleton storage service instance
func GetStorageService() service.StorageService {
	InitializeServices()
	return storageService
}

// GetTicketRepository returns the singleton ticket repository instance
func GetTicketRepository() repository.ITicketRepository {
	InitializeServices()
	return ticketRepo
}

// GetLookupService returns the singleton lookup service instance
func GetLookupService() *service.LookupService {
	InitializeServices()
	return lookupService
}

// GetQueueRepository returns the singleton queue repository instance
func GetQueueRepository() *repository.QueueRepository {
	InitializeServices()
	return queueRepo
}

// GetUserRepository returns the singleton user repository instance
func GetUserRepository() *repository.UserRepository {
	InitializeServices()
	return userRepo
}

// GetAuthService returns the singleton auth service instance
func GetAuthService() *service.AuthService {
	InitializeServices()
	return authService
}