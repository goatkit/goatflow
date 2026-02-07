package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/models"
	"github.com/goatkit/goatflow/internal/repository"
)

func TestSessionService(t *testing.T) {
	t.Run("CreateSession", func(t *testing.T) {
		repo := repository.NewMemorySessionRepository()
		svc := NewSessionService(repo)

		sessionID, err := svc.CreateSession(1, "testuser", models.UserTypeAgent, "192.168.1.1", "Mozilla/5.0")
		require.NoError(t, err)
		assert.NotEmpty(t, sessionID)

		// Verify session was created
		session, err := svc.GetSession(sessionID)
		require.NoError(t, err)
		assert.Equal(t, 1, session.UserID)
		assert.Equal(t, "testuser", session.UserLogin)
		assert.Equal(t, models.UserTypeAgent, session.UserType)
		assert.Equal(t, "192.168.1.1", session.RemoteAddr)
		assert.Equal(t, "Mozilla/5.0", session.UserAgent)
	})

	t.Run("CreateSession_GeneratesUniqueIDs", func(t *testing.T) {
		repo := repository.NewMemorySessionRepository()
		svc := NewSessionService(repo)

		ids := make(map[string]bool)
		for i := 0; i < 100; i++ {
			sessionID, err := svc.CreateSession(1, "testuser", models.UserTypeAgent, "192.168.1.1", "Mozilla/5.0")
			require.NoError(t, err)
			assert.False(t, ids[sessionID], "Duplicate session ID generated")
			ids[sessionID] = true
		}
	})

	t.Run("GetSession", func(t *testing.T) {
		repo := repository.NewMemorySessionRepository()
		svc := NewSessionService(repo)

		sessionID, _ := svc.CreateSession(42, "agent42", models.UserTypeAgent, "10.0.0.1", "Chrome/100")

		session, err := svc.GetSession(sessionID)
		require.NoError(t, err)
		assert.Equal(t, 42, session.UserID)
		assert.Equal(t, "agent42", session.UserLogin)
	})

	t.Run("GetSession_NotFound", func(t *testing.T) {
		repo := repository.NewMemorySessionRepository()
		svc := NewSessionService(repo)

		_, err := svc.GetSession("nonexistent")
		assert.Error(t, err)
	})

	t.Run("ListSessions", func(t *testing.T) {
		repo := repository.NewMemorySessionRepository()
		svc := NewSessionService(repo)

		// Create several sessions
		for i := 1; i <= 5; i++ {
			svc.CreateSession(i, "user"+string(rune('0'+i)), models.UserTypeAgent, "192.168.1."+string(rune('0'+i)), "Browser")
		}

		sessions, err := svc.ListSessions()
		require.NoError(t, err)
		assert.Len(t, sessions, 5)
	})

	t.Run("TouchSession", func(t *testing.T) {
		repo := repository.NewMemorySessionRepository()
		svc := NewSessionService(repo)

		sessionID, _ := svc.CreateSession(1, "testuser", models.UserTypeAgent, "192.168.1.1", "Mozilla/5.0")

		// Get initial last request time
		session, _ := svc.GetSession(sessionID)
		initialTime := session.LastRequest

		// Wait a tiny bit and touch the session
		time.Sleep(10 * time.Millisecond)
		err := svc.TouchSession(sessionID)
		require.NoError(t, err)

		// Verify last request was updated
		session, _ = svc.GetSession(sessionID)
		assert.True(t, session.LastRequest.After(initialTime))
	})

	t.Run("TouchSession_NotFound", func(t *testing.T) {
		repo := repository.NewMemorySessionRepository()
		svc := NewSessionService(repo)

		err := svc.TouchSession("nonexistent")
		assert.Error(t, err)
	})

	t.Run("KillSession", func(t *testing.T) {
		repo := repository.NewMemorySessionRepository()
		svc := NewSessionService(repo)

		sessionID, _ := svc.CreateSession(1, "testuser", models.UserTypeAgent, "192.168.1.1", "Mozilla/5.0")

		// Verify it exists
		_, err := svc.GetSession(sessionID)
		require.NoError(t, err)

		// Kill it
		err = svc.KillSession(sessionID)
		require.NoError(t, err)

		// Verify it's gone
		_, err = svc.GetSession(sessionID)
		assert.Error(t, err)
	})

	t.Run("KillSession_NotFound", func(t *testing.T) {
		repo := repository.NewMemorySessionRepository()
		svc := NewSessionService(repo)

		err := svc.KillSession("nonexistent")
		assert.Error(t, err)
	})

	t.Run("KillUserSessions", func(t *testing.T) {
		repo := repository.NewMemorySessionRepository()
		svc := NewSessionService(repo)

		// Create multiple sessions for user 10
		for i := 0; i < 3; i++ {
			svc.CreateSession(10, "user10", models.UserTypeAgent, "192.168.1.1", "Mozilla/5.0")
		}

		// Create session for different user
		svc.CreateSession(20, "user20", models.UserTypeAgent, "192.168.1.2", "Mozilla/5.0")

		// Kill all sessions for user 10
		count, err := svc.KillUserSessions(10)
		require.NoError(t, err)
		assert.Equal(t, 3, count)

		// Verify user 10 sessions are gone
		sessions, _ := svc.ListSessions()
		assert.Len(t, sessions, 1)
		assert.Equal(t, 20, sessions[0].UserID)
	})

	t.Run("CleanupExpired", func(t *testing.T) {
		repo := repository.NewMemorySessionRepository()
		svc := NewSessionService(repo)

		// Create old session directly in repo (to control LastRequest time)
		oldSession := &models.Session{
			SessionID:   "old-session",
			UserID:      1,
			UserLogin:   "olduser",
			UserType:    models.UserTypeAgent,
			CreateTime:  time.Now().Add(-2 * time.Hour),
			LastRequest: time.Now().Add(-2 * time.Hour),
		}
		repo.Create(oldSession)

		// Create recent session
		svc.CreateSession(2, "newuser", models.UserTypeAgent, "192.168.1.1", "Mozilla/5.0")

		// Clean up sessions older than 1 hour
		count, err := svc.CleanupExpired(1 * time.Hour)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		// Verify only new session remains
		sessions, _ := svc.ListSessions()
		assert.Len(t, sessions, 1)
		assert.Equal(t, 2, sessions[0].UserID)
	})

	t.Run("GetUserSessions", func(t *testing.T) {
		repo := repository.NewMemorySessionRepository()
		svc := NewSessionService(repo)

		// Create sessions for multiple users
		svc.CreateSession(5, "user5", models.UserTypeAgent, "192.168.1.1", "Mozilla/5.0")
		svc.CreateSession(5, "user5", models.UserTypeAgent, "192.168.1.2", "Chrome/100")
		svc.CreateSession(10, "user10", models.UserTypeAgent, "192.168.1.3", "Safari")

		// Get sessions for user 5
		sessions, err := svc.GetUserSessions(5)
		require.NoError(t, err)
		assert.Len(t, sessions, 2)

		for _, s := range sessions {
			assert.Equal(t, 5, s.UserID)
		}
	})
}
