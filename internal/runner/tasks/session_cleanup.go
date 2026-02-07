// Package tasks provides background task implementations for the runner.
package tasks

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/goatkit/goatflow/internal/config"
	"github.com/goatkit/goatflow/internal/constants"
	"github.com/goatkit/goatflow/internal/repository"
	"github.com/goatkit/goatflow/internal/runner"
	"github.com/goatkit/goatflow/internal/service"
)

// Default interval if not configured (5 minutes).
const defaultSessionCleanupInterval = 5 * time.Minute

// SessionCleanupTask cleans up expired sessions from the database.
type SessionCleanupTask struct {
	sessionSvc *service.SessionService
	interval   time.Duration
	logger     *log.Logger
}

// NewSessionCleanupTask creates a new session cleanup task.
func NewSessionCleanupTask(db *sql.DB) runner.Task {
	repo := repository.NewSessionRepository(db)

	// Get interval from config
	interval := defaultSessionCleanupInterval
	if cfg := config.Get(); cfg != nil && cfg.Runner.SessionCleanup.Interval > 0 {
		interval = cfg.Runner.SessionCleanup.Interval
	}

	return &SessionCleanupTask{
		sessionSvc: service.NewSessionService(repo),
		interval:   interval,
		logger:     log.New(log.Writer(), "[SESSION-CLEANUP] ", log.LstdFlags),
	}
}

// Name returns the task name.
func (t *SessionCleanupTask) Name() string {
	return "session-cleanup"
}

// Schedule returns the cron schedule based on configured interval.
func (t *SessionCleanupTask) Schedule() string {
	// Convert interval to cron expression
	// For simplicity, we support minute-based intervals
	minutes := int(t.interval.Minutes())
	if minutes < 1 {
		minutes = 1
	}
	if minutes >= 60 {
		// For hourly or longer, run at the start of each hour
		hours := minutes / 60
		if hours >= 24 {
			// Daily
			return "0 0 0 * * *"
		}
		return fmt.Sprintf("0 0 */%d * * *", hours)
	}
	// Run every N minutes
	return fmt.Sprintf("0 */%d * * * *", minutes)
}

// Timeout returns the task timeout (2 minutes).
func (t *SessionCleanupTask) Timeout() time.Duration {
	return 2 * time.Minute
}

// Run cleans up expired sessions.
func (t *SessionCleanupTask) Run(ctx context.Context) error {
	t.logger.Println("Starting session cleanup...")

	// Clean up sessions that exceed max age (absolute lifetime)
	maxAge := time.Duration(constants.MaxSessionTimeout) * time.Second
	maxAgeCount, err := t.sessionSvc.CleanupByMaxAge(maxAge)
	if err != nil {
		t.logger.Printf("Error cleaning up sessions by max age: %v", err)
	} else if maxAgeCount > 0 {
		t.logger.Printf("Cleaned up %d sessions exceeding max age (%v)", maxAgeCount, maxAge)
	}

	// Clean up sessions that have been idle too long
	idleTimeout := time.Duration(constants.DefaultSessionIdleTimeout) * time.Second
	idleCount, err := t.sessionSvc.CleanupExpired(idleTimeout)
	if err != nil {
		t.logger.Printf("Error cleaning up idle sessions: %v", err)
	} else if idleCount > 0 {
		t.logger.Printf("Cleaned up %d idle sessions (idle > %v)", idleCount, idleTimeout)
	}

	totalCleaned := maxAgeCount + idleCount
	if totalCleaned == 0 {
		t.logger.Println("No expired sessions to clean up")
	} else {
		t.logger.Printf("Session cleanup complete: %d sessions removed", totalCleaned)
	}

	return nil
}
