package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTManager(t *testing.T) {
	t.Parallel()
	secretKey := "test-secret-key-for-testing"
	tokenDuration := 1 * time.Hour
	jwtManager := NewJWTManager(secretKey, tokenDuration)

	t.Run("access token carries the session and round-trips", func(t *testing.T) {
		userID := uint(2)
		email := "user@example.com"
		role := "Agent"
		tenantID := uint(20)

		token, err := jwtManager.GenerateTokenWithLogin("sess-1", userID, "user", email, role, false, tenantID)
		require.NoError(t, err)

		claims, err := jwtManager.ValidateToken(token)
		require.NoError(t, err)
		assert.Equal(t, userID, claims.UserID)
		assert.Equal(t, email, claims.Email)
		assert.Equal(t, role, claims.Role)
		assert.Equal(t, tenantID, claims.TenantID)
		assert.Equal(t, "sess-1", claims.SessionID)
	})

	t.Run("tokens without a session are neither issued nor accepted", func(t *testing.T) {
		_, err := jwtManager.GenerateTokenWithLogin("", 1, "u", "u@example.com", "Agent", false, 0)
		assert.Error(t, err)
		_, err = jwtManager.GenerateRefreshToken(AccountKindAgent, 1, "u", "")
		assert.Error(t, err)

		// A pre-sid access token (as issued before this release) is rejected.
		legacy := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
			UserID: 1, Email: "u@example.com", Role: "Agent", TokenType: TokenTypeAccess,
			RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
		})
		signed, err := legacy.SignedString([]byte(secretKey))
		require.NoError(t, err)
		_, err = jwtManager.ValidateToken(signed)
		assert.ErrorIs(t, err, ErrInvalidToken)
	})

	t.Run("ValidateToken rejects invalid token", func(t *testing.T) {
		invalidToken := "invalid.token.here"
		_, err := jwtManager.ValidateToken(invalidToken)
		assert.Error(t, err)
	})

	t.Run("ValidateToken rejects expired token", func(t *testing.T) {
		// Create manager with very short duration
		shortManager := NewJWTManager(secretKey, 1*time.Nanosecond)

		token, err := shortManager.GenerateTokenWithLogin("sess-1", 1, "test@example.com", "test@example.com", "Admin", false, 1)
		require.NoError(t, err)

		// Wait for token to expire
		time.Sleep(10 * time.Millisecond)

		_, err = shortManager.ValidateToken(token)
		assert.Error(t, err)
	})

	t.Run("ValidateToken rejects token with wrong signature", func(t *testing.T) {
		// Generate token with one key
		token, err := jwtManager.GenerateTokenWithLogin("sess-1", 1, "test@example.com", "test@example.com", "Admin", false, 1)
		require.NoError(t, err)

		// Try to validate with different key
		wrongManager := NewJWTManager("wrong-secret-key", tokenDuration)
		_, err = wrongManager.ValidateToken(token)
		assert.Error(t, err)
	})

	t.Run("refresh token round-trips and carries the account", func(t *testing.T) {
		token, err := jwtManager.GenerateRefreshToken(AccountKindCustomer, 3, "cust-login", "sess-9")
		require.NoError(t, err)

		claims, err := jwtManager.ValidateRefreshToken(token)
		require.NoError(t, err)
		assert.Equal(t, uint(3), claims.UserID)
		assert.Equal(t, AccountKindCustomer, claims.Kind)
		assert.Equal(t, "cust-login", claims.Subject)
		assert.Equal(t, "sess-9", claims.SessionID)
		assert.WithinDuration(t, time.Now().Add(7*24*time.Hour), claims.ExpiresAt.Time, time.Minute)

		other, err := jwtManager.GenerateRefreshToken(AccountKindCustomer, 3, "cust-login", "sess-9")
		require.NoError(t, err)
		assert.NotEqual(t, token, other, "each refresh token has its own id")
	})

	t.Run("refresh token is not an access token", func(t *testing.T) {
		token, err := jwtManager.GenerateRefreshToken(AccountKindAgent, 3, "agent-login", "sess-1")
		require.NoError(t, err)
		_, err = jwtManager.ValidateToken(token)
		assert.ErrorIs(t, err, ErrInvalidToken)
	})

	t.Run("access token is not a refresh token", func(t *testing.T) {
		token, err := jwtManager.GenerateTokenWithLogin("sess-1", 3, "agent-login", "agent-login", "Admin", true, 0)
		require.NoError(t, err)
		_, err = jwtManager.ValidateRefreshToken(token)
		assert.ErrorIs(t, err, ErrInvalidToken)
	})

	t.Run("refresh token rejects unknown kind, other key and expiry", func(t *testing.T) {
		_, err := jwtManager.GenerateRefreshToken("robot", 3, "x", "sess-1")
		assert.Error(t, err)

		token, err := NewJWTManager("another-secret-key", time.Hour).GenerateRefreshToken(AccountKindAgent, 3, "agent-login", "sess-1")
		require.NoError(t, err)
		_, err = jwtManager.ValidateRefreshToken(token)
		assert.ErrorIs(t, err, ErrInvalidToken)

		short := NewJWTManager(secretKey, time.Hour)
		short.SetRefreshTokenDuration(time.Nanosecond)
		token, err = short.GenerateRefreshToken(AccountKindAgent, 3, "agent-login", "sess-1")
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
				token, err := jwtManager.GenerateTokenWithLogin("sess-1", uint(id), "test@example.com", "test@example.com", "User", false, uint(id))
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
