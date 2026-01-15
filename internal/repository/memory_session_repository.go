package repository

import (
	"errors"
	"sync"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

// MemorySessionRepository is an in-memory implementation of session storage for testing.
type MemorySessionRepository struct {
	sessions map[string]*models.Session
	mu       sync.RWMutex
}

// NewMemorySessionRepository creates a new in-memory session repository.
func NewMemorySessionRepository() *MemorySessionRepository {
	return &MemorySessionRepository{
		sessions: make(map[string]*models.Session),
	}
}

// Create stores a new session.
func (r *MemorySessionRepository) Create(session *models.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.sessions[session.SessionID]; exists {
		return errors.New("session already exists")
	}

	// Make a copy to avoid external modifications
	sessionCopy := *session
	r.sessions[session.SessionID] = &sessionCopy
	return nil
}

// GetByID retrieves a session by its ID.
func (r *MemorySessionRepository) GetByID(sessionID string) (*models.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	session, exists := r.sessions[sessionID]
	if !exists {
		return nil, errors.New("session not found")
	}

	// Return a copy to avoid external modifications
	sessionCopy := *session
	return &sessionCopy, nil
}

// GetByUserID retrieves all sessions for a specific user.
func (r *MemorySessionRepository) GetByUserID(userID int) ([]*models.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sessions []*models.Session
	for _, session := range r.sessions {
		if session.UserID == userID {
			sessionCopy := *session
			sessions = append(sessions, &sessionCopy)
		}
	}
	return sessions, nil
}

// List retrieves all sessions.
func (r *MemorySessionRepository) List() ([]*models.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sessions := make([]*models.Session, 0, len(r.sessions))
	for _, session := range r.sessions {
		sessionCopy := *session
		sessions = append(sessions, &sessionCopy)
	}
	return sessions, nil
}

// UpdateLastRequest updates the last request time for a session.
func (r *MemorySessionRepository) UpdateLastRequest(sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, exists := r.sessions[sessionID]
	if !exists {
		return errors.New("session not found")
	}

	session.LastRequest = time.Now()
	return nil
}

// Delete removes a session by its ID.
func (r *MemorySessionRepository) Delete(sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.sessions[sessionID]; !exists {
		return errors.New("session not found")
	}

	delete(r.sessions, sessionID)
	return nil
}

// DeleteByUserID removes all sessions for a specific user.
func (r *MemorySessionRepository) DeleteByUserID(userID int) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for sessionID, session := range r.sessions {
		if session.UserID == userID {
			delete(r.sessions, sessionID)
			count++
		}
	}
	return count, nil
}

// DeleteExpired removes all sessions older than the specified duration.
func (r *MemorySessionRepository) DeleteExpired(maxAge time.Duration) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	count := 0
	for sessionID, session := range r.sessions {
		if session.LastRequest.Before(cutoff) {
			delete(r.sessions, sessionID)
			count++
		}
	}
	return count, nil
}
