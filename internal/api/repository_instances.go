package api

import (
	"log"
	"sync"
	"os"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/service"
	"github.com/gotrs-io/gotrs-ce/internal/shared"
)

var (
	ticketRepo        repository.ITicketRepository
	queueRepo         *repository.QueueRepository
	priorityRepo      *repository.PriorityRepository
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
		// Get database connection - no fallback, service registry is single source of truth
		db, err := database.GetDB()
		if err != nil {
			log.Fatalf("FATAL: Cannot initialize services without database connection: %v", err)
		}
		
		log.Printf("Successfully connected to database")
		// Initialize real database repositories
		ticketRepo = repository.NewTicketRepository(db)
		queueRepo = repository.NewQueueRepository(db)
		priorityRepo = repository.NewPriorityRepository(db)
		userRepo = repository.NewUserRepository(db)
		
		// Initialize services
		simpleTicketService = service.NewSimpleTicketService(ticketRepo)
		
		// Initialize lookup service
		lookupService = service.NewLookupService()
		
        // Initialize storage service - respect STORAGE_PATH env, fallback to ./storage
        storagePath := os.Getenv("STORAGE_PATH")
        if storagePath == "" {
            storagePath = "./storage"
        }
        storageService, err = service.NewLocalStorageService(storagePath)
		if err != nil {
			log.Fatalf("FATAL: Cannot initialize storage service: %v", err)
		}
		
		// Initialize auth service
		// Use the shared JWT manager for auth service
		jwtManager := shared.GetJWTManager()
		authService = service.NewAuthService(db, jwtManager)
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

// GetPriorityRepository returns the singleton priority repository instance
func GetPriorityRepository() *repository.PriorityRepository {
	InitializeServices()
	return priorityRepo
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