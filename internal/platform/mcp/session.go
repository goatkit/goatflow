package mcp

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Session holds the state for a single MCP SSE connection.
type Session struct {
	ID         string
	UserID     int
	UserLogin  string
	UserRole   string
	Server     *Server
	Created    time.Time
	LastActive time.Time
	NotifyChan chan []byte // server→client notifications
}

// SessionManager manages MCP SSE sessions.
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	maxAge   time.Duration // inactivity timeout
}

// NewSessionManager creates a session manager with a given inactivity timeout.
func NewSessionManager(maxAge time.Duration) *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*Session),
		maxAge:   maxAge,
	}
	go sm.cleanupLoop()
	return sm
}

// Create creates a new session for the given user.
func (sm *SessionManager) Create(userID int, userLogin, userRole string, bridge *APIBridge) *Session {
	id := generateSessionID()
	now := time.Now()
	server := NewServer(userID, userLogin, userRole, bridge)

	session := &Session{
		ID:         id,
		UserID:     userID,
		UserLogin:  userLogin,
		UserRole:   userRole,
		Server:     server,
		Created:    now,
		LastActive: now,
		NotifyChan: make(chan []byte, 16),
	}

	sm.mu.Lock()
	sm.sessions[id] = session
	sm.mu.Unlock()

	return session
}

// Get retrieves a session by ID and updates its last active time.
func (sm *SessionManager) Get(id string) *Session {
	sm.mu.RLock()
	session, ok := sm.sessions[id]
	sm.mu.RUnlock()

	if !ok {
		return nil
	}

	sm.mu.Lock()
	session.LastActive = time.Now()
	sm.mu.Unlock()

	return session
}

// Delete removes a session.
func (sm *SessionManager) Delete(id string) {
	sm.mu.Lock()
	if session, ok := sm.sessions[id]; ok {
		close(session.NotifyChan)
		delete(sm.sessions, id)
	}
	sm.mu.Unlock()
}

// Count returns the number of active sessions.
func (sm *SessionManager) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

// cleanupLoop removes expired sessions every minute.
func (sm *SessionManager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		sm.cleanup()
	}
}

func (sm *SessionManager) cleanup() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	cutoff := time.Now().Add(-sm.maxAge)
	for id, session := range sm.sessions {
		if session.LastActive.Before(cutoff) {
			close(session.NotifyChan)
			delete(sm.sessions, id)
		}
	}
}

func generateSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
