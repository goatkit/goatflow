package middleware

import (
	"context"
	"database/sql"

	platformmodels "github.com/goatkit/goatflow/internal/platform/models"
)

// SessionChecker checks session validity (subset of service.SessionService).
type SessionChecker interface {
	GetSession(sessionID string) (*platformmodels.Session, error)
	TouchSession(sessionID string) error
}

// MaintenanceChecker checks for active system maintenance.
type MaintenanceChecker interface {
	IsActive() (*platformmodels.SystemMaintenance, error)
	IsComing(withinMinutes int) (*platformmodels.SystemMaintenance, error)
}

// QueueAccessChecker checks queue permissions (subset of service.QueueAccessService).
type QueueAccessChecker interface {
	IsAdmin(ctx context.Context, userID uint) (bool, error)
	HasQueueAccess(ctx context.Context, userID uint, queueID uint, permType string) (bool, error)
	GetAccessibleQueueIDs(ctx context.Context, userID uint, permType string) ([]uint, error)
}

// ResolvedTicket is a ticket a path identifier names, with its queue.
type ResolvedTicket struct {
	ID      uint64
	QueueID uint
}

// TicketQueueResolver resolves a ticket identifier (tn or id) to the tickets
// it can name. Product code injects a concrete implementation via
// SetTicketQueueResolverFactory. It returns sql.ErrNoRows when none exists.
type TicketQueueResolver interface {
	ResolveTickets(db *sql.DB, ticketIDStr string) ([]ResolvedTicket, error)
}

var (
	sessionServiceFactory      func(*sql.DB) SessionChecker
	maintenanceCheckerFactory  func(*sql.DB) MaintenanceChecker
	queueAccessCheckerFactory  func(*sql.DB) QueueAccessChecker
	ticketQueueResolverFactory func() TicketQueueResolver
)

func SetSessionServiceFactory(f func(*sql.DB) SessionChecker) { sessionServiceFactory = f }

// SetMaintenanceCheckerFactory injects a maintenance checker factory from product code.
func SetMaintenanceCheckerFactory(f func(*sql.DB) MaintenanceChecker) { maintenanceCheckerFactory = f }

// NewMaintenanceChecker creates a MaintenanceChecker from the injected factory.
// Returns nil if no factory is set.
func NewMaintenanceChecker(db *sql.DB) MaintenanceChecker {
	if maintenanceCheckerFactory != nil {
		return maintenanceCheckerFactory(db)
	}
	return nil
}

// SetQueueAccessCheckerFactory injects a queue access checker factory from product code.
func SetQueueAccessCheckerFactory(f func(*sql.DB) QueueAccessChecker) { queueAccessCheckerFactory = f }

// NewQueueAccessChecker creates a QueueAccessChecker from the injected factory.
// Returns nil if no factory is set.
func NewQueueAccessChecker(db *sql.DB) QueueAccessChecker {
	if queueAccessCheckerFactory != nil {
		return queueAccessCheckerFactory(db)
	}
	return nil
}

// SetTicketQueueResolverFactory injects a ticket queue resolver factory from product code.
func SetTicketQueueResolverFactory(f func() TicketQueueResolver) { ticketQueueResolverFactory = f }
