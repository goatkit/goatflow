package service

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/repository"
)

// SessionService handles session management operations.
type SessionService struct {
	repo repository.SessionRepository
}

// NewSessionService creates a new session service.
func NewSessionService(repo repository.SessionRepository) *SessionService {
	return &SessionService{repo: repo}
}

// CreateSession creates a new session for a user.
// Returns the generated session ID.
func (s *SessionService) CreateSession(userID int, userLogin, userType, remoteAddr, userAgent string) (string, error) {
	return s.CreateSessionWithDetails(userID, userLogin, userType, "", "", remoteAddr, userAgent)
}

// CreateSessionWithDetails creates a new session with additional user details.
// Returns the generated session ID.
func (s *SessionService) CreateSessionWithDetails(userID int, userLogin, userType, userTitle, userFullName, remoteAddr, userAgent string) (string, error) {
	sessionID := s.generateSessionID()
	now := time.Now()

	session := &models.Session{
		SessionID:    sessionID,
		UserID:       userID,
		UserLogin:    userLogin,
		UserType:     userType,
		UserTitle:    userTitle,
		UserFullName: userFullName,
		CreateTime:   now,
		LastRequest:  now,
		RemoteAddr:   remoteAddr,
		UserAgent:    userAgent,
	}

	if err := s.repo.Create(session); err != nil {
		return "", err
	}

	return sessionID, nil
}

// GetSession retrieves a session by its ID.
func (s *SessionService) GetSession(sessionID string) (*models.Session, error) {
	return s.repo.GetByID(sessionID)
}

// GetUserSessions retrieves all sessions for a specific user.
func (s *SessionService) GetUserSessions(userID int) ([]*models.Session, error) {
	return s.repo.GetByUserID(userID)
}

// ListSessions retrieves all active sessions.
func (s *SessionService) ListSessions() ([]*models.Session, error) {
	return s.repo.List()
}

// TouchSession updates the last request time for a session.
// Should be called on each authenticated request.
func (s *SessionService) TouchSession(sessionID string) error {
	return s.repo.UpdateLastRequest(sessionID)
}

// KillSession terminates a specific session.
func (s *SessionService) KillSession(sessionID string) error {
	return s.repo.Delete(sessionID)
}

// KillUserSessions terminates all sessions for a specific user.
// Returns the number of sessions killed.
func (s *SessionService) KillUserSessions(userID int) (int, error) {
	return s.repo.DeleteByUserID(userID)
}

// CleanupExpired removes sessions that have been inactive for longer than maxAge.
// Returns the number of sessions cleaned up.
func (s *SessionService) CleanupExpired(maxAge time.Duration) (int, error) {
	return s.repo.DeleteExpired(maxAge)
}

// CleanupByMaxAge removes sessions that were created more than maxAge ago.
// This enforces the maximum session lifetime regardless of activity.
// Returns the number of sessions cleaned up.
func (s *SessionService) CleanupByMaxAge(maxAge time.Duration) (int, error) {
	return s.repo.DeleteByMaxAge(maxAge)
}

// generateSessionID creates a secure random session ID.
// Returns a 64-character hex string (256 bits of entropy).
func (s *SessionService) generateSessionID() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails (extremely unlikely)
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(bytes)
}
