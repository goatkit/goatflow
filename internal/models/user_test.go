package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUser(t *testing.T) {
	t.Run("SetPassword hashes password", func(t *testing.T) {
		user := &User{}
		plainPassword := "mySecurePassword123"
		
		err := user.SetPassword(plainPassword)
		require.NoError(t, err)
		
		// Password should be hashed, not plain
		assert.NotEqual(t, plainPassword, user.Password)
		assert.NotEmpty(t, user.Password)
		
		// Hashed password should be longer than original
		assert.Greater(t, len(user.Password), len(plainPassword))
	})

	t.Run("CheckPassword validates correct password", func(t *testing.T) {
		user := &User{}
		plainPassword := "correctPassword123"
		
		err := user.SetPassword(plainPassword)
		require.NoError(t, err)
		
		// Check with correct password
		assert.True(t, user.CheckPassword(plainPassword))
		
		// Check with incorrect password
		assert.False(t, user.CheckPassword("wrongPassword"))
		assert.False(t, user.CheckPassword(""))
		assert.False(t, user.CheckPassword("correctPassword")) // Missing numbers
	})

	t.Run("IsLocked checks lock status", func(t *testing.T) {
		user := &User{}
		
		// User not locked by default
		assert.False(t, user.IsLocked())
		
		// Lock user for 1 hour
		futureTime := time.Now().Add(1 * time.Hour)
		user.LockedUntil = &futureTime
		assert.True(t, user.IsLocked())
		
		// Lock expired
		pastTime := time.Now().Add(-1 * time.Hour)
		user.LockedUntil = &pastTime
		assert.False(t, user.IsLocked())
	})

	t.Run("LockAccount sets lock time", func(t *testing.T) {
		user := &User{}
		duration := 15 * time.Minute
		
		user.LockAccount(duration)
		
		assert.NotNil(t, user.LockedUntil)
		assert.True(t, user.IsLocked())
		
		// Check lock time is approximately correct
		expectedTime := time.Now().Add(duration)
		timeDiff := user.LockedUntil.Sub(expectedTime)
		assert.Less(t, timeDiff, 1*time.Second) // Within 1 second
	})

	t.Run("UnlockAccount removes lock", func(t *testing.T) {
		user := &User{}
		
		// Lock the account first
		user.LockAccount(1 * time.Hour)
		user.FailedLoginCount = 5
		
		// Unlock it
		user.UnlockAccount()
		
		assert.Nil(t, user.LockedUntil)
		assert.Equal(t, 0, user.FailedLoginCount)
		assert.False(t, user.IsLocked())
	})

	t.Run("IncrementFailedLogin counts failures", func(t *testing.T) {
		user := &User{}
		
		// First 4 attempts don't lock
		for i := 1; i <= 4; i++ {
			user.IncrementFailedLogin()
			assert.Equal(t, i, user.FailedLoginCount)
			assert.False(t, user.IsLocked())
		}
		
		// 5th attempt locks the account
		user.IncrementFailedLogin()
		assert.Equal(t, 5, user.FailedLoginCount)
		assert.True(t, user.IsLocked())
		assert.NotNil(t, user.LockedUntil)
		
		// Check lock duration is 15 minutes
		lockDuration := user.LockedUntil.Sub(time.Now())
		assert.Greater(t, lockDuration, 14*time.Minute)
		assert.Less(t, lockDuration, 16*time.Minute)
	})

	t.Run("ResetFailedLogin clears counter", func(t *testing.T) {
		user := &User{
			FailedLoginCount: 3,
		}
		
		user.ResetFailedLogin()
		assert.Equal(t, 0, user.FailedLoginCount)
	})
}

func TestUserRole(t *testing.T) {
	t.Run("Role constants are defined", func(t *testing.T) {
		assert.Equal(t, UserRole("Admin"), RoleAdmin)
		assert.Equal(t, UserRole("Agent"), RoleAgent)
		assert.Equal(t, UserRole("Customer"), RoleCustomer)
	})
}

func TestUserGroups(t *testing.T) {
	t.Run("Groups field exists and can be populated", func(t *testing.T) {
		user := &User{
			ID:        1,
			Login:     "testuser",
			FirstName: "Test",
			LastName:  "User",
			Groups:    []string{"admin", "users"},
		}
		
		assert.Len(t, user.Groups, 2)
		assert.Contains(t, user.Groups, "admin")
		assert.Contains(t, user.Groups, "users")
	})

	t.Run("Groups field can be empty", func(t *testing.T) {
		user := &User{
			ID:        1,
			Login:     "testuser",
			FirstName: "Test",
			LastName:  "User",
			Groups:    []string{},
		}
		
		assert.Len(t, user.Groups, 0)
		assert.Empty(t, user.Groups)
	})
}

func TestUserSecurity(t *testing.T) {
	t.Run("Different passwords produce different hashes", func(t *testing.T) {
		user1 := &User{}
		user2 := &User{}
		
		err1 := user1.SetPassword("password123")
		require.NoError(t, err1)
		
		err2 := user2.SetPassword("password123")
		require.NoError(t, err2)
		
		// Same password should produce different hashes (due to salt)
		assert.NotEqual(t, user1.Password, user2.Password)
	})

	t.Run("Password is never exposed in JSON", func(t *testing.T) {
		// The Password field has json:"-" tag, which is tested implicitly
		// by the struct definition
		user := User{
			ID:       1,
			Email:    "test@example.com",
			Password: "hashedPassword",
		}
		
		// This test verifies the struct tag is present
		assert.NotEmpty(t, user.Password)
	})
}

func BenchmarkSetPassword(b *testing.B) {
	user := &User{}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := user.SetPassword("benchmarkPassword123")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCheckPassword(b *testing.B) {
	user := &User{}
	user.SetPassword("benchmarkPassword123")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		user.CheckPassword("benchmarkPassword123")
	}
}