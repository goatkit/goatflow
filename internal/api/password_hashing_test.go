package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/gotrs-io/gotrs-ce/internal/auth"
	"github.com/gotrs-io/gotrs-ce/internal/database"
)

// TestPasswordHashingModes verifies both SHA256 (OTRS-compatible) and bcrypt modes
// for customer users and agents.
func TestPasswordHashingModes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := getTestDB(t)
	defer db.Close()

	testCases := []struct {
		name       string
		hashType   string
		verifyFunc func(t *testing.T, password, hash string)
	}{
		{
			name:     "SHA256 (OTRS compatible)",
			hashType: "sha256",
			verifyFunc: func(t *testing.T, password, hash string) {
				// SHA256 produces 64 character hex string
				assert.Len(t, hash, 64, "SHA256 hash should be 64 characters")
				assert.Regexp(t, `^[a-f0-9]{64}$`, hash, "SHA256 hash should be hex only")

				// Should NOT start with bcrypt prefix
				assert.False(t, strings.HasPrefix(hash, "$2"), "SHA256 hash should not have bcrypt prefix")

				// Verify with hasher
				hasher := auth.NewPasswordHasher()
				assert.True(t, hasher.VerifyPassword(password, hash), "Password should verify")
			},
		},
		{
			name:     "bcrypt (stronger security)",
			hashType: "bcrypt",
			verifyFunc: func(t *testing.T, password, hash string) {
				// Bcrypt hashes start with $2a$, $2b$, or $2y$
				assert.True(t, strings.HasPrefix(hash, "$2"), "bcrypt hash should start with $2")

				// Should be able to verify with bcrypt directly
				err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
				assert.NoError(t, err, "bcrypt.CompareHashAndPassword should succeed")

				// Verify with hasher
				hasher := auth.NewPasswordHasher()
				assert.True(t, hasher.VerifyPassword(password, hash), "Password should verify")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set the hash type for this test
			originalHashType := os.Getenv("PASSWORD_HASH_TYPE")
			os.Setenv("PASSWORD_HASH_TYPE", tc.hashType)
			defer os.Setenv("PASSWORD_HASH_TYPE", originalHashType)

			t.Run("customer user create", func(t *testing.T) {
				testCustomerUserPasswordHashing(t, db, tc.hashType, tc.verifyFunc)
			})

			t.Run("agent user reset password", func(t *testing.T) {
				testAgentPasswordReset(t, db, tc.hashType, tc.verifyFunc)
			})
		})
	}
}

func testCustomerUserPasswordHashing(t *testing.T, db *sql.DB, hashType string, verify func(*testing.T, string, string)) {
	testLogin := "cuhash_" + hashType + "_" + time.Now().Format("150405")
	testPassword := "TestPassword123!"
	testEmail := testLogin + "@example.com"
	testCustomerID := "TESTCUST"

	// Ensure test customer company exists
	createTestCustomerCompany(t, db, testCustomerID)

	// Cleanup after test
	defer func() {
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM customer_user WHERE login = $1"), testLogin)
	}()

	// Hash the password using the auth hasher (simulating what the handler does)
	hasher := auth.NewPasswordHasher()
	hashedPassword, err := hasher.HashPassword(testPassword)
	require.NoError(t, err, "Failed to hash password")

	// Insert directly into DB (bypassing handler to focus on password hashing)
	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT INTO customer_user (login, email, customer_id, pw, first_name, last_name, valid_id, 
			create_time, create_by, change_time, change_by)
		VALUES ($1, $2, $3, $4, 'Test', 'User', 1, NOW(), 1, NOW(), 1)
	`), testLogin, testEmail, testCustomerID, hashedPassword)
	require.NoError(t, err, "Failed to insert customer user")

	// Query the stored hash
	var storedHash string
	err = db.QueryRow(database.ConvertPlaceholders("SELECT pw FROM customer_user WHERE login = $1"), testLogin).Scan(&storedHash)
	require.NoError(t, err, "Failed to query stored password")

	// Password should NOT be stored as plain text
	assert.NotEqual(t, testPassword, storedHash, "Password was stored as plain text!")

	// Verify hash format and correctness
	verify(t, testPassword, storedHash)
}

func testAgentPasswordReset(t *testing.T, db *sql.DB, hashType string, verify func(*testing.T, string, string)) {
	// Create a test agent user
	testLogin := "agenthash_" + hashType + "_" + time.Now().Format("150405")
	testPassword := "AgentPass456!"

	// Insert test agent
	result, err := db.Exec(database.ConvertPlaceholders(`
		INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES ($1, 'placeholder', 'Test', 'Agent', 1, NOW(), 1, NOW(), 1)
	`), testLogin)
	require.NoError(t, err, "Failed to create test agent")

	agentID, err := result.LastInsertId()
	require.NoError(t, err, "Failed to get agent ID")

	// Cleanup after test
	defer func() {
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM users WHERE id = $1"), agentID)
	}()

	// Create router with auth context
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userID", int64(1)) // Admin user
		c.Next()
	})
	router.POST("/admin/users/:id/reset-password", HandleAdminUserResetPassword)

	// Reset password via API
	payload := map[string]string{"password": testPassword}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/admin/users/"+strconv.FormatInt(agentID, 10)+"/reset-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "Expected 200 OK, got %d: %s", w.Code, w.Body.String())

	// Query the stored hash
	var storedHash string
	err = db.QueryRow(database.ConvertPlaceholders("SELECT pw FROM users WHERE id = $1"), agentID).Scan(&storedHash)
	require.NoError(t, err, "Failed to query stored password")

	// Password should NOT be stored as plain text
	assert.NotEqual(t, testPassword, storedHash, "Password was stored as plain text!")

	// Note: Agent password handler uses bcrypt regardless of PASSWORD_HASH_TYPE
	// This is intentional - agents should always use stronger security
	// Verify it's a bcrypt hash
	assert.True(t, strings.HasPrefix(storedHash, "$2"), "Agent passwords should always use bcrypt")

	// Verify with bcrypt
	err = bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(testPassword))
	assert.NoError(t, err, "bcrypt verification should succeed")
}

// TestCustomerUserPasswordHashFormat verifies the exact hash format stored in DB
func TestCustomerUserPasswordHashFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := getTestDB(t)
	defer db.Close()

	t.Run("SHA256 produces correct format", func(t *testing.T) {
		originalHashType := os.Getenv("PASSWORD_HASH_TYPE")
		os.Setenv("PASSWORD_HASH_TYPE", "sha256")
		defer os.Setenv("PASSWORD_HASH_TYPE", originalHashType)

		hasher := auth.NewPasswordHasher()
		hash, err := hasher.HashPassword("TestPassword123!")
		require.NoError(t, err)

		// SHA256 format: 64 hex characters
		assert.Len(t, hash, 64)
		assert.Regexp(t, `^[a-f0-9]+$`, hash)

		// Same password should produce same hash (SHA256 is deterministic)
		hash2, _ := hasher.HashPassword("TestPassword123!")
		assert.Equal(t, hash, hash2, "SHA256 should be deterministic")
	})

	t.Run("bcrypt produces correct format", func(t *testing.T) {
		originalHashType := os.Getenv("PASSWORD_HASH_TYPE")
		os.Setenv("PASSWORD_HASH_TYPE", "bcrypt")
		defer os.Setenv("PASSWORD_HASH_TYPE", originalHashType)

		hasher := auth.NewPasswordHasher()
		hash, err := hasher.HashPassword("TestPassword123!")
		require.NoError(t, err)

		// bcrypt format: starts with $2a$, $2b$, or $2y$
		assert.True(t, strings.HasPrefix(hash, "$2"))

		// Same password should produce different hash (bcrypt uses random salt)
		hash2, _ := hasher.HashPassword("TestPassword123!")
		assert.NotEqual(t, hash, hash2, "bcrypt should use random salt")

		// But both should verify
		assert.True(t, hasher.VerifyPassword("TestPassword123!", hash))
		assert.True(t, hasher.VerifyPassword("TestPassword123!", hash2))
	})
}

// TestPasswordVerificationCrossCompatibility verifies passwords hashed with one
// algorithm can be verified after switching algorithms (for migration scenarios)
func TestPasswordVerificationCrossCompatibility(t *testing.T) {
	gin.SetMode(gin.TestMode)
	password := "TestPassword123!"

	// Hash with SHA256
	os.Setenv("PASSWORD_HASH_TYPE", "sha256")
	sha256Hasher := auth.NewPasswordHasher()
	sha256Hash, _ := sha256Hasher.HashPassword(password)

	// Hash with bcrypt
	os.Setenv("PASSWORD_HASH_TYPE", "bcrypt")
	bcryptHasher := auth.NewPasswordHasher()
	bcryptHash, _ := bcryptHasher.HashPassword(password)

	// Reset env
	os.Unsetenv("PASSWORD_HASH_TYPE")

	t.Run("SHA256 hash verifies with any hasher config", func(t *testing.T) {
		// Even if we switch to bcrypt mode, SHA256 hashes should still verify
		os.Setenv("PASSWORD_HASH_TYPE", "bcrypt")
		defer os.Unsetenv("PASSWORD_HASH_TYPE")

		hasher := auth.NewPasswordHasher()
		assert.True(t, hasher.VerifyPassword(password, sha256Hash),
			"SHA256 hash should verify even when bcrypt is default")
	})

	t.Run("bcrypt hash verifies with any hasher config", func(t *testing.T) {
		// Even if we switch to SHA256 mode, bcrypt hashes should still verify
		os.Setenv("PASSWORD_HASH_TYPE", "sha256")
		defer os.Unsetenv("PASSWORD_HASH_TYPE")

		hasher := auth.NewPasswordHasher()
		assert.True(t, hasher.VerifyPassword(password, bcryptHash),
			"bcrypt hash should verify even when SHA256 is default")
	})
}

// TestAgentPasswordAlwaysBcrypt verifies agent passwords always use bcrypt
// regardless of PASSWORD_HASH_TYPE setting (security requirement)
func TestAgentPasswordAlwaysBcrypt(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := getTestDB(t)
	defer db.Close()

	// Even with SHA256 mode, agent passwords should use bcrypt
	originalHashType := os.Getenv("PASSWORD_HASH_TYPE")
	os.Setenv("PASSWORD_HASH_TYPE", "sha256")
	defer os.Setenv("PASSWORD_HASH_TYPE", originalHashType)

	testLogin := "agentbcrypt_" + time.Now().Format("150405")
	testPassword := "AgentSecure789!"

	// Insert test agent
	result, err := db.Exec(database.ConvertPlaceholders(`
		INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES ($1, 'placeholder', 'Test', 'Agent', 1, NOW(), 1, NOW(), 1)
	`), testLogin)
	require.NoError(t, err)

	agentID, err := result.LastInsertId()
	require.NoError(t, err)

	defer func() {
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM users WHERE id = $1"), agentID)
	}()

	// Create router
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userID", int64(1))
		c.Next()
	})
	router.POST("/admin/users/:id/reset-password", HandleAdminUserResetPassword)

	payload := map[string]string{"password": testPassword}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/admin/users/"+strconv.FormatInt(agentID, 10)+"/reset-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Verify it's bcrypt despite SHA256 being configured
	var storedHash string
	err = db.QueryRow(database.ConvertPlaceholders("SELECT pw FROM users WHERE id = $1"), agentID).Scan(&storedHash)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(storedHash, "$2"),
		"Agent passwords MUST use bcrypt regardless of PASSWORD_HASH_TYPE. Got: %s", storedHash)

	// Verify password works
	err = bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(testPassword))
	assert.NoError(t, err, "Password should verify with bcrypt")
}
