package shared

import (
	"sync"

	"github.com/gotrs-io/gotrs-ce/internal/database"
	"github.com/gotrs-io/gotrs-ce/internal/repository"
	"github.com/gotrs-io/gotrs-ce/internal/service"
)

var (
	globalSessionService *service.SessionService
	sessionOnce          sync.Once
	sessionInitErr       error
)

// GetSessionService returns the global session service singleton.
// Returns nil if database is not available.
func GetSessionService() *service.SessionService {
	sessionOnce.Do(func() {
		db, err := database.GetDB()
		if err != nil {
			sessionInitErr = err
			return
		}
		repo := repository.NewSessionRepository(db)
		globalSessionService = service.NewSessionService(repo)
	})
	return globalSessionService
}

// SessionServiceAvailable returns true if the session service is available.
func SessionServiceAvailable() bool {
	return GetSessionService() != nil
}
