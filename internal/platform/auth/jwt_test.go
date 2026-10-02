package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTManager(t *testing.T) {
	t.Parallel()
	secretKey := "test-secret-key-for-testing"
	tokenDuration := 1 * time.Hour
	jwtManager := NewJWTManager(secretKey, tokenDuration)

	t.Run("GenerateToken creates valid token", func(t *testing.T) {
		userID := uint(1)
		email := "test@example.com"
		role := "Admin"
		tenantID := uint(10)

		token, err := jwtManager.GenerateToken(userID, email, role, tenantID)
		require.NoError(t, err)
		assert.NotEmpty(t, token)
	})

	t.Run("ValidateToken validates correct token", func(t *testing.T) {
		userID := uint(2)
		email := "user@example.com"
		role := "Agent"
		tenantID := uint(20)

		token, err := jwtManager.GenerateToken(userID, email, role, tenantID)
		require.NoError(t, err)

		claims, err := jwtManager.ValidateToken(token)
		require.NoError(t, err)
		assert.Equal(t, userID, claims.UserID)
		assert.Equal(t, email, claims.Email)
		assert.Equal(t, role, claims.Role)
		assert.Equal(t, tenantID, claims.TenantID)
	})

	t.Run("ValidateToken rejects invalid token", func(t *testing.T) {
		invalidToken := "invalid.token.here"
		_, err := jwtManager.ValidateToken(invalidToken)
		assert.Error(t, err)
	})

	t.Run("ValidateToken rejects expired token", func(t *testing.T) {
		// Create manager with very short duration
		shortManager := NewJWTManager(secretKey, 1*time.Nanosecond)

		token, err := shortManager.GenerateToken(1, "test@example.com", "Admin", 1)
		require.NoError(t, err)

		// Wait for token to expire
		time.Sleep(10 * time.Millisecond)

		_, err = shortManager.ValidateToken(token)
		assert.Error(t, err)
	})

	t.Run("ValidateToken rejects token with wrong signature", func(t *testing.T) {
		// Generate token with one key
		token, err := jwtManager.GenerateToken(1, "test@example.com", "Admin", 1)
		require.NoError(t, err)

		// Try to validate with different key
		wrongManager := NewJWTManager("wrong-secret-key", tokenDuration)
		_, err = wrongManager.ValidateToken(token)
		assert.Error(t, err)
	})

	t.Run("refresh token round-trips and carries the account", func(t *testing.T) {
		token, err := jwtManager.GenerateRefreshToken(AccountKindCustomer, 3, "cust-login")
		require.NoError(t, err)

		claims, err := jwtManager.ValidateRefreshToken(token)
		require.NoError(t, err)
		assert.Equal(t, uint(3), claims.UserID)
		assert.Equal(t, AccountKindCustomer, claims.Kind)
		assert.Equal(t, "cust-login", claims.Subject)
		assert.WithinDuration(t, time.Now().Add(7*24*time.Hour), claims.ExpiresAt.Time, time.Minute)

		other, err := jwtManager.GenerateRefreshToken(AccountKindCustomer, 3, "cust-login")
		require.NoError(t, err)
		assert.NotEqual(t, token, other, "each refresh token has its own id")
	})

	t.Run("refresh token is not an access token", func(t *testing.T) {
		token, err := jwtManager.GenerateRefreshToken(AccountKindAgent, 3, "agent-login")
		require.NoError(t, err)
		_, err = jwtManager.ValidateToken(token)
		assert.ErrorIs(t, err, ErrInvalidToken)
	})

	t.Run("access token is not a refresh token", func(t *testing.T) {
		token, err := jwtManager.GenerateTokenWithAdmin(3, "agent-login", "Admin", true, 0)
		require.NoError(t, err)
		_, err = jwtManager.ValidateRefreshToken(token)
		assert.ErrorIs(t, err, ErrInvalidToken)
	})

	t.Run("refresh token rejects unknown kind, other key and expiry", func(t *testing.T) {
		_, err := jwtManager.GenerateRefreshToken("robot", 3, "x")
		assert.Error(t, err)

		token, err := NewJWTManager("another-secret-key", time.Hour).GenerateRefreshToken(AccountKindAgent, 3, "agent-login")
		require.NoError(t, err)
		_, err = jwtManager.ValidateRefreshToken(token)
		assert.ErrorIs(t, err, ErrInvalidToken)

		short := NewJWTManager(secretKey, time.Hour)
		short.SetRefreshTokenDuration(time.Nanosecond)
		token, err = short.GenerateRefreshToken(AccountKindAgent, 3, "agent-login")
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond)
		_, err = short.ValidateRefreshToken(token)
		assert.ErrorIs(t, err, ErrExpiredToken)
	})
}

func TestJWTManagerConcurrency(t *testing.T) {
	jwtManager := NewJWTManager("test-secret", 1*time.Hour)

	// Test concurrent token generation
	t.Run("Concurrent token generation", func(t *testing.T) {
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func(id int) {
				token, err := jwtManager.GenerateToken(uint(id), "test@example.com", "User", uint(id))
				assert.NoError(t, err)
				assert.NotEmpty(t, token)
				done <- true
			}(i)
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

// Benchmarks are in auth_service_test.go
