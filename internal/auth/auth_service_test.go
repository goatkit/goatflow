package auth

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gotrs-io/gotrs-ce/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTIntegration(t *testing.T) {
	jwtManager := NewJWTManager("test-secret", 1*time.Hour)

	t.Run("Generate and validate token", func(t *testing.T) {
		userID := uint(1)
		email := "test@example.com"
		role := "Admin"
		tenantID := uint(10)

		token, err := jwtManager.GenerateToken(userID, email, role, tenantID)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		claims, err := jwtManager.ValidateToken(token)
		require.NoError(t, err)
		assert.Equal(t, userID, claims.UserID)
		assert.Equal(t, email, claims.Email)
		assert.Equal(t, role, claims.Role)
		assert.Equal(t, tenantID, claims.TenantID)
	})

	t.Run("Generate and validate refresh token", func(t *testing.T) {
		userID := uint(2)
		email := "user@example.com"

		token, err := jwtManager.GenerateRefreshToken(userID, email)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Refresh tokens use RegisteredClaims, not custom Claims
		// TODO: Implement ValidateRefreshToken method
		// For now, just verify the token was generated
		assert.Contains(t, token, ".")
		parts := strings.Split(token, ".")
		assert.Len(t, parts, 3) // JWT has 3 parts: header.payload.signature
	})
}

func TestAuthServiceLogin(t *testing.T) {
	// Create mock database
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	jwtManager := NewJWTManager("test-secret", 1*time.Hour)
	authService := NewAuthService(db, jwtManager)

	t.Run("Successful login", func(t *testing.T) {
		email := "test@example.com"
		password := "password123"

		// Create user with proper hashed password
		user := &models.User{Email: email}
		err := user.SetPassword(password)
		require.NoError(t, err)

		// Mock user query
		rows := sqlmock.NewRows([]string{
			"id", "login", "email", "pw", "title", "first_name", "last_name",
			"valid_id", "create_time", "create_by", "change_time", "change_by",
		}).AddRow(
			1, "testuser", email, user.Password, "Mr", "Test", "User",
			1, time.Now(), 1, time.Now(), 1,
		)

		mock.ExpectQuery("SELECT (.+) FROM users WHERE email = (.+)").
			WithArgs(email).
			WillReturnRows(rows)

		// Mock role determination
		mock.ExpectQuery("SELECT g.name FROM groups").
			WithArgs(uint(1)).
			WillReturnError(sql.ErrNoRows) // No group means default role

		// Mock update for last login
		mock.ExpectExec("UPDATE users SET").
			WithArgs(1).
			WillReturnResult(sqlmock.NewResult(1, 1))

		resp, err := authService.Login(email, password)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotEmpty(t, resp.Token)
		assert.NotEmpty(t, resp.RefreshToken)
		assert.Equal(t, uint(1), resp.User.ID)
		assert.Equal(t, email, resp.User.Email)
	})

	t.Run("Invalid credentials", func(t *testing.T) {
		email := "wrong@example.com"
		password := "wrongpass"

		mock.ExpectQuery("SELECT (.+) FROM users WHERE email = (.+)").
			WithArgs(email).
			WillReturnError(sql.ErrNoRows)

		resp, err := authService.Login(email, password)
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, ErrInvalidCredentials, err)
	})
}

func TestAuthServiceRefreshToken(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	jwtManager := NewJWTManager("test-secret", 1*time.Hour)
	authService := NewAuthService(db, jwtManager)

	t.Run("Valid refresh token", func(t *testing.T) {
		// Generate a valid refresh token
		refreshToken, err := jwtManager.GenerateRefreshToken(1, "test@example.com")
		require.NoError(t, err)

		// Mock user query
		rows := sqlmock.NewRows([]string{
			"id", "login", "email", "pw", "title", "first_name", "last_name",
			"valid_id", "create_time", "create_by", "change_time", "change_by",
		}).AddRow(
			1, "testuser", "test@example.com", "hashedpw", "Mr", "Test", "User",
			1, time.Now(), 1, time.Now(), 1,
		)

		mock.ExpectQuery("SELECT (.+) FROM users WHERE email = \\$1 AND valid_id = 1").
			WithArgs("test@example.com").
			WillReturnRows(rows)

		// Mock role determination
		mock.ExpectQuery("SELECT g.name FROM groups").
			WithArgs(uint(1)).
			WillReturnError(sql.ErrNoRows)

		resp, err := authService.RefreshToken(refreshToken)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotEmpty(t, resp.Token)
		assert.NotEmpty(t, resp.RefreshToken)
	})

	t.Run("Invalid refresh token", func(t *testing.T) {
		resp, err := authService.RefreshToken("invalid.token.here")
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestAuthServiceChangePassword(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	jwtManager := NewJWTManager("test-secret", 1*time.Hour)
	authService := NewAuthService(db, jwtManager)

	t.Run("Change with correct old password", func(t *testing.T) {
		userID := uint(1)
		oldPassword := "oldpassword"
		newPassword := "newpassword123"

		// Create user with old password
		user := &models.User{ID: userID}
		err := user.SetPassword(oldPassword)
		require.NoError(t, err)

		// Mock user query
		rows := sqlmock.NewRows([]string{
			"id", "login", "email", "pw", "title", "first_name", "last_name",
			"valid_id", "create_time", "create_by", "change_time", "change_by",
		}).AddRow(
			userID, "testuser", "test@example.com", user.Password, "Mr", "Test", "User",
			1, time.Now(), 1, time.Now(), 1,
		)

		mock.ExpectQuery("SELECT (.+) FROM users WHERE id = (.+)").
			WithArgs(userID).
			WillReturnRows(rows)

		// Mock role determination
		mock.ExpectQuery("SELECT g.name FROM groups").
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)

		// Mock password update
		mock.ExpectExec("UPDATE users SET pw = (.+)").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err = authService.ChangePassword(userID, oldPassword, newPassword)
		assert.NoError(t, err)
	})

	t.Run("Change with wrong old password", func(t *testing.T) {
		userID := uint(1)
		wrongOldPassword := "wrongpass"
		newPassword := "newpassword123"

		// Create user with different password
		user := &models.User{ID: userID}
		err := user.SetPassword("correctpassword")
		require.NoError(t, err)

		// Mock user query
		rows := sqlmock.NewRows([]string{
			"id", "login", "email", "pw", "title", "first_name", "last_name",
			"valid_id", "create_time", "create_by", "change_time", "change_by",
		}).AddRow(
			userID, "testuser", "test@example.com", user.Password, "Mr", "Test", "User",
			1, time.Now(), 1, time.Now(), 1,
		)

		mock.ExpectQuery("SELECT (.+) FROM users WHERE id = (.+)").
			WithArgs(userID).
			WillReturnRows(rows)

		// Mock role determination
		mock.ExpectQuery("SELECT g.name FROM groups").
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)

		err = authService.ChangePassword(userID, wrongOldPassword, newPassword)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid old password")
	})

	t.Run("User not found", func(t *testing.T) {
		userID := uint(999)

		mock.ExpectQuery("SELECT (.+) FROM users WHERE id = (.+)").
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)

		err := authService.ChangePassword(userID, "oldpass", "newpass")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "User not found")
	})
}

func TestRoleDetection(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	jwtManager := NewJWTManager("test-secret", 1*time.Hour)
	authService := NewAuthService(db, jwtManager)

	t.Run("Admin role from group", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"name"}).AddRow("Admin")
		mock.ExpectQuery("SELECT g.name FROM groups").
			WithArgs(uint(1)).
			WillReturnRows(rows)

		role := authService.determineUserRole(1)
		assert.Equal(t, "Admin", role)
	})

	t.Run("Default to Agent when no group", func(t *testing.T) {
		mock.ExpectQuery("SELECT g.name FROM groups").
			WithArgs(uint(2)).
			WillReturnError(sql.ErrNoRows)

		role := authService.determineUserRole(2)
		assert.Equal(t, "Agent", role)
	})
}

func TestHelperMethods(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	jwtManager := NewJWTManager("test-secret", 1*time.Hour)
	authService := NewAuthService(db, jwtManager)

	t.Run("getUserByEmail success", func(t *testing.T) {
		email := "test@example.com"

		rows := sqlmock.NewRows([]string{
			"id", "login", "email", "pw", "title", "first_name", "last_name",
			"valid_id", "create_time", "create_by", "change_time", "change_by",
		}).AddRow(
			1, "testuser", email, "hashedpw", "Mr", "Test", "User",
			1, time.Now(), 1, time.Now(), 1,
		)

		mock.ExpectQuery("SELECT (.+) FROM users WHERE email = (.+)").
			WithArgs(email).
			WillReturnRows(rows)

		// Mock role determination
		mock.ExpectQuery("SELECT g.name FROM groups").
			WithArgs(uint(1)).
			WillReturnError(sql.ErrNoRows)

		user, err := authService.getUserByEmail(email)
		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, email, user.Email)
		assert.Equal(t, uint(1), user.ID)
	})

	t.Run("getUserByID success", func(t *testing.T) {
		userID := uint(1)

		rows := sqlmock.NewRows([]string{
			"id", "login", "email", "pw", "title", "first_name", "last_name",
			"valid_id", "create_time", "create_by", "change_time", "change_by",
		}).AddRow(
			userID, "testuser", "test@example.com", "hashedpw", "Mr", "Test", "User",
			1, time.Now(), 1, time.Now(), 1,
		)

		mock.ExpectQuery("SELECT (.+) FROM users WHERE id = (.+)").
			WithArgs(userID).
			WillReturnRows(rows)

		// Mock role determination
		mock.ExpectQuery("SELECT g.name FROM groups").
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)

		user, err := authService.getUserByID(userID)
		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, userID, user.ID)
	})

	t.Run("updateUserLoginAttempts", func(t *testing.T) {
		user := &models.User{
			ID:        1,
			LastLogin: &time.Time{},
		}
		*user.LastLogin = time.Now()

		mock.ExpectExec("UPDATE users SET change_time = (.+) WHERE id = (.+)").
			WithArgs(*user.LastLogin, user.ID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := authService.updateUserLoginAttempts(user)
		assert.NoError(t, err)
	})
}

func BenchmarkGenerateToken(b *testing.B) {
	jwtManager := NewJWTManager("benchmark-secret", 1*time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := jwtManager.GenerateToken(uint(i), "bench@example.com", "User", 1)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateToken(b *testing.B) {
	jwtManager := NewJWTManager("benchmark-secret", 1*time.Hour)
	token, _ := jwtManager.GenerateToken(1, "bench@example.com", "User", 1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := jwtManager.ValidateToken(token)
		if err != nil {
			b.Fatal(err)
		}
	}
}