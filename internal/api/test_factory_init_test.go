package api

import (
	"database/sql"

	"github.com/goatkit/goatflow/internal/platform/middleware"
	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/shared"
	"github.com/goatkit/goatflow/internal/platform/template"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/service"
)

func init() {
	auth.SetUserRepoFactory(func(db *sql.DB) auth.UserLookup {
		return repository.NewUserRepository(db)
	})
	template.SetMaintenanceCheckerFactory(func(db *sql.DB) template.MaintenanceChecker {
		return repository.NewSystemMaintenanceRepository(db)
	})
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
}