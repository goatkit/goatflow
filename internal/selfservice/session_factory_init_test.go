package selfservice_test

import (
	"database/sql"

	"github.com/goatkit/goatflow/internal/platform/middleware"
	"github.com/goatkit/goatflow/internal/platform/shared"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/service"
)

// The tests log in through the real login handlers, which create a sessions
// row before issuing a token. cmd/goats wires these factories in production.
func init() {
	middleware.SetSessionServiceFactory(func(db *sql.DB) middleware.SessionChecker {
		return service.NewSessionService(repository.NewSessionRepository(db))
	})
	shared.SetSessionManagerFactory(func(db *sql.DB) shared.SessionManager {
		return service.NewSessionService(repository.NewSessionRepository(db))
	})
}
