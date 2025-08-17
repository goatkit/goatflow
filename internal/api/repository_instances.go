package api

import (
	"sync"

	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/service"
)

var (
	ticketRepo        repository.ITicketRepository
	simpleTicketService *service.SimpleTicketService
	storageService    service.StorageService
	lookupService     *service.LookupService
	once              sync.Once
)

// InitializeServices initializes singleton service instances
func InitializeServices() {
	once.Do(func() {
		// Initialize repositories
		ticketRepo = repository.NewMemoryTicketRepository()
		
		// Initialize services
		simpleTicketService = service.NewSimpleTicketService(ticketRepo)
		
		// Initialize lookup service
		lookupService = service.NewLookupService()
		
		// Initialize storage service
		var err error
		storageService, err = service.NewLocalStorageService("./storage")
		if err != nil {
			// Fallback to temp directory if storage dir can't be created
			storageService, _ = service.NewLocalStorageService("/tmp/gotrs-storage")
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