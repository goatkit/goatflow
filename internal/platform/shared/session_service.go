package shared

import (
	"database/sql"
	"sync"

	"github.com/goatkit/goatflow/internal/platform/database"
	platformmodels "github.com/goatkit/goatflow/internal/platform/models"
)

// SessionManager is the subset of session service methods shared callers need.
// Product code injects a concrete *service.SessionService via SetSessionManagerFactory.
type SessionManager interface {
	CreateSession(userID int, userLogin, userType, remoteAddr, userAgent string) (string, error)
	GetSession(sessionID string) (*platformmodels.Session, error)
	TouchSession(sessionID string) error
	KillSession(sessionID string) error
}

var (
	sessionManagerFactory func(*sql.DB) SessionManager
	sessionManagerOnce    sync.Once
	globalSessionManager  SessionManager
)

// SetSessionManagerFactory injects a session manager factory from product code.
func SetSessionManagerFactory(f func(*sql.DB) SessionManager) { sessionManagerFactory = f }

// GetSessionService returns the global session manager singleton.
func GetSessionService() SessionManager {
	sessionManagerOnce.Do(func() {
		if sessionManagerFactory != nil {
			db, err := database.GetDB()
			if err == nil {
				globalSessionManager = sessionManagerFactory(db)
			}
		}
	})
	return globalSessionManager
}

// SessionServiceAvailable returns true if the session service is available.
func SessionServiceAvailable() bool {
	return GetSessionService() != nil
}
