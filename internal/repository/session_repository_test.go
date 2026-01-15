package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gotrs-io/gotrs-ce/internal/models"
)

func TestMemorySessionRepository(t *testing.T) {
	t.Run("Create", func(t *testing.T) {
		repo := NewMemorySessionRepository()

		session := &models.Session{
			SessionID:   "test-session-123",
			UserID:      1,
			UserLogin:   "testuser",
			UserType:    models.UserTypeAgent,
			CreateTime:  time.Now(),
			LastRequest: time.Now(),
			RemoteAddr:  "192.168.1.100",
			UserAgent:   "Mozilla/5.0",
		}

		err := repo.Create(session)
		require.NoError(t, err)

		// Verify it was stored
		retrieved, err := repo.GetByID(session.SessionID)
		require.NoError(t, err)
		assert.Equal(t, session.SessionID, retrieved.SessionID)
		assert.Equal(t, session.UserID, retrieved.UserID)
		assert.Equal(t, session.UserLogin, retrieved.UserLogin)
	})

	t.Run("Create_DuplicateID", func(t *testing.T) {
		repo := NewMemorySessionRepository()

		session := &models.Session{
			SessionID: "duplicate-id",
			UserID:    1,
			UserLogin: "testuser",
		}

		err := repo.Create(session)
		require.NoError(t, err)

		// Try to create another with same ID
		err = repo.Create(session)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("GetByID", func(t *testing.T) {
		repo := NewMemorySessionRepository()

		session := &models.Session{
			SessionID:   "get-by-id-test",
			UserID:      42,
			UserLogin:   "agent42",
			UserType:    models.UserTypeAgent,
			CreateTime:  time.Now(),
			LastRequest: time.Now(),
			RemoteAddr:  "10.0.0.1",
			UserAgent:   "Chrome/100",
		}

		repo.Create(session)

		retrieved, err := repo.GetByID("get-by-id-test")
		require.NoError(t, err)
		assert.Equal(t, 42, retrieved.UserID)
		assert.Equal(t, "agent42", retrieved.UserLogin)
	})

	t.Run("GetByID_NotFound", func(t *testing.T) {
		repo := NewMemorySessionRepository()

		_, err := repo.GetByID("nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("GetByUserID", func(t *testing.T) {
		repo := NewMemorySessionRepository()

		// Create multiple sessions for the same user
		for i := 0; i < 3; i++ {
			session := &models.Session{
				SessionID:   "user5-session-" + string(rune('a'+i)),
				UserID:      5,
				UserLogin:   "multidevice",
				UserType:    models.UserTypeAgent,
				CreateTime:  time.Now(),
				LastRequest: time.Now(),
			}
			repo.Create(session)
		}

		// Create session for different user
		otherSession := &models.Session{
			SessionID: "other-user-session",
			UserID:    99,
			UserLogin: "otheruser",
		}
		repo.Create(otherSession)

		sessions, err := repo.GetByUserID(5)
		require.NoError(t, err)
		assert.Len(t, sessions, 3)

		for _, s := range sessions {
			assert.Equal(t, 5, s.UserID)
		}
	})

	t.Run("List", func(t *testing.T) {
		repo := NewMemorySessionRepository()

		// Create several sessions
		for i := 1; i <= 5; i++ {
			session := &models.Session{
				SessionID:   "list-session-" + string(rune('0'+i)),
				UserID:      i,
				UserLogin:   "user" + string(rune('0'+i)),
				UserType:    models.UserTypeAgent,
				CreateTime:  time.Now(),
				LastRequest: time.Now(),
			}
			repo.Create(session)
		}

		sessions, err := repo.List()
		require.NoError(t, err)
		assert.Len(t, sessions, 5)
	})

	t.Run("UpdateLastRequest", func(t *testing.T) {
		repo := NewMemorySessionRepository()

		originalTime := time.Now().Add(-1 * time.Hour)
		session := &models.Session{
			SessionID:   "update-test",
			UserID:      1,
			UserLogin:   "testuser",
			CreateTime:  originalTime,
			LastRequest: originalTime,
		}
		repo.Create(session)

		// Update last request
		err := repo.UpdateLastRequest("update-test")
		require.NoError(t, err)

		// Verify it was updated
		retrieved, err := repo.GetByID("update-test")
		require.NoError(t, err)
		assert.True(t, retrieved.LastRequest.After(originalTime))
	})

	t.Run("UpdateLastRequest_NotFound", func(t *testing.T) {
		repo := NewMemorySessionRepository()

		err := repo.UpdateLastRequest("nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Delete", func(t *testing.T) {
		repo := NewMemorySessionRepository()

		session := &models.Session{
			SessionID: "to-delete",
			UserID:    1,
			UserLogin: "testuser",
		}
		repo.Create(session)

		// Verify it exists
		_, err := repo.GetByID("to-delete")
		require.NoError(t, err)

		// Delete it
		err = repo.Delete("to-delete")
		require.NoError(t, err)

		// Verify it's gone
		_, err = repo.GetByID("to-delete")
		assert.Error(t, err)
	})

	t.Run("Delete_NotFound", func(t *testing.T) {
		repo := NewMemorySessionRepository()

		err := repo.Delete("nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("DeleteByUserID", func(t *testing.T) {
		repo := NewMemorySessionRepository()

		// Create multiple sessions for user 10
		for i := 0; i < 3; i++ {
			session := &models.Session{
				SessionID: "user10-session-" + string(rune('a'+i)),
				UserID:    10,
				UserLogin: "user10",
			}
			repo.Create(session)
		}

		// Create session for different user
		otherSession := &models.Session{
			SessionID: "user20-session",
			UserID:    20,
			UserLogin: "user20",
		}
		repo.Create(otherSession)

		// Delete all sessions for user 10
		count, err := repo.DeleteByUserID(10)
		require.NoError(t, err)
		assert.Equal(t, 3, count)

		// Verify user 10 sessions are gone
		sessions, err := repo.GetByUserID(10)
		require.NoError(t, err)
		assert.Len(t, sessions, 0)

		// Verify user 20 session still exists
		_, err = repo.GetByID("user20-session")
		require.NoError(t, err)
	})

	t.Run("DeleteExpired", func(t *testing.T) {
		repo := NewMemorySessionRepository()

		now := time.Now()

		// Create some expired sessions
		for i := 0; i < 3; i++ {
			session := &models.Session{
				SessionID:   "expired-" + string(rune('a'+i)),
				UserID:      i + 1,
				UserLogin:   "expireduser",
				LastRequest: now.Add(-2 * time.Hour), // 2 hours ago
			}
			repo.Create(session)
		}

		// Create some active sessions
		for i := 0; i < 2; i++ {
			session := &models.Session{
				SessionID:   "active-" + string(rune('a'+i)),
				UserID:      i + 10,
				UserLogin:   "activeuser",
				LastRequest: now.Add(-30 * time.Minute), // 30 minutes ago
			}
			repo.Create(session)
		}

		// Delete sessions older than 1 hour
		count, err := repo.DeleteExpired(1 * time.Hour)
		require.NoError(t, err)
		assert.Equal(t, 3, count)

		// Verify only active sessions remain
		sessions, err := repo.List()
		require.NoError(t, err)
		assert.Len(t, sessions, 2)
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		repo := NewMemorySessionRepository()
		done := make(chan bool, 100)

		// Concurrent creates
		for i := 0; i < 20; i++ {
			go func(idx int) {
				session := &models.Session{
					SessionID:   "concurrent-" + string(rune('A'+idx)),
					UserID:      idx,
					UserLogin:   "user" + string(rune('A'+idx)),
					CreateTime:  time.Now(),
					LastRequest: time.Now(),
				}
				err := repo.Create(session)
				assert.NoError(t, err)
				done <- true
			}(i)
		}

		// Concurrent reads
		for i := 0; i < 30; i++ {
			go func() {
				_, err := repo.List()
				assert.NoError(t, err)
				done <- true
			}()
		}

		// Wait for all operations
		for i := 0; i < 50; i++ {
			<-done
		}

		// Verify all sessions were created
		sessions, err := repo.List()
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(sessions), 20)
	})
}
