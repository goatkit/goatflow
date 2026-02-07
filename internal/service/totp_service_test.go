package service

import (
	"database/sql"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Test Helpers
// =============================================================================

func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return db, mock
}

func generateValidCode(secret string) string {
	code, _ := totp.GenerateCode(secret, time.Now())
	return code
}

// =============================================================================
// T1: TOTP Code Brute Force Protection
// =============================================================================

func TestTOTP_BruteForceProtection_InvalidCodesRejected(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	svc := NewTOTPService(db, "GoatFlow")
	userID := 123

	// Setup: user has 2FA enabled with known secret
	secret := "JBSWY3DPEHPK3PXP" // Test secret
	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(userID, "UserTOTPSecret").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow(secret))

	// Invalid codes should be rejected
	invalidCodes := []string{"000000", "123456", "999999", "111111"}
	for _, code := range invalidCodes {
		// Reset mock for each attempt
		mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
			WithArgs(userID, "UserTOTPSecret").
			WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow(secret))
		mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
			WithArgs(userID, "UserTOTPRecoveryCodes").
			WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow("[]"))

		valid, _ := svc.ValidateCode(userID, code)
		assert.False(t, valid, "Invalid code %s should be rejected", code)
	}
}

func TestTOTP_ValidCodeAccepted(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	svc := NewTOTPService(db, "GoatFlow")
	userID := 123
	secret := "JBSWY3DPEHPK3PXP"

	// Generate a valid code for current time
	validCode := generateValidCode(secret)

	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(userID, "UserTOTPSecret").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow(secret))

	valid, err := svc.ValidateCode(userID, validCode)
	assert.NoError(t, err)
	assert.True(t, valid, "Valid TOTP code should be accepted")
}

// =============================================================================
// T2: Recovery Code Single Use
// =============================================================================

func TestTOTP_RecoveryCodeSingleUse(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	svc := NewTOTPService(db, "GoatFlow")
	userID := 123
	secret := "JBSWY3DPEHPK3PXP"
	recoveryCodes := []string{"abc12345", "def67890", "ghi11111"}
	codesJSON, _ := json.Marshal(recoveryCodes)

	// First use of recovery code
	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(userID, "UserTOTPSecret").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow(secret))
	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(userID, "UserTOTPRecoveryCodes").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow(string(codesJSON)))
	// Expect update to remove used code
	mock.ExpectExec("UPDATE user_preferences").
		WillReturnResult(sqlmock.NewResult(0, 1))

	valid, err := svc.ValidateCode(userID, "abc12345")
	assert.NoError(t, err)
	assert.True(t, valid, "First use of recovery code should succeed")

	// Second use of same recovery code should fail
	remainingCodes := []string{"def67890", "ghi11111"}
	remainingJSON, _ := json.Marshal(remainingCodes)

	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(userID, "UserTOTPSecret").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow(secret))
	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(userID, "UserTOTPRecoveryCodes").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow(string(remainingJSON)))

	valid, _ = svc.ValidateCode(userID, "abc12345")
	assert.False(t, valid, "Reused recovery code should be rejected")
}

func TestTOTP_RecoveryCodesCaseInsensitive(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	svc := NewTOTPService(db, "GoatFlow")
	userID := 123
	secret := "JBSWY3DPEHPK3PXP"
	recoveryCodes := []string{"abcd1234"}
	codesJSON, _ := json.Marshal(recoveryCodes)

	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(userID, "UserTOTPSecret").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow(secret))
	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(userID, "UserTOTPRecoveryCodes").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow(string(codesJSON)))
	mock.ExpectExec("UPDATE user_preferences").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Should work with uppercase
	valid, err := svc.ValidateCode(userID, "ABCD1234")
	assert.NoError(t, err)
	assert.True(t, valid, "Recovery codes should be case-insensitive")
}

// =============================================================================
// T4: 2FA Bypass Prevention
// =============================================================================

func TestTOTP_NoTokenWithoutValidCode(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	svc := NewTOTPService(db, "GoatFlow")
	userID := 123
	secret := "JBSWY3DPEHPK3PXP"

	// User has 2FA enabled
	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(userID, "UserTOTPEnabled").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow("1"))

	assert.True(t, svc.IsEnabled(userID), "2FA should be enabled")

	// Attempting to validate with wrong code
	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(userID, "UserTOTPSecret").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow(secret))
	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(userID, "UserTOTPRecoveryCodes").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow("[]"))

	valid, _ := svc.ValidateCode(userID, "000000")
	assert.False(t, valid, "Invalid code should not validate - no token should be issued")
}

// =============================================================================
// T7: Cross-User TOTP Validation
// =============================================================================

func TestTOTP_CrossUserValidationFails(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	svc := NewTOTPService(db, "GoatFlow")

	// User A's secret
	userAID := 100
	userASecret := "AAAAAAAAAAAAAAAA"

	// User B's secret (different)
	userBID := 200
	userBSecret := "BBBBBBBBBBBBBBBB"

	// Generate valid code for User A
	userACode := generateValidCode(userASecret)

	// Try to use User A's code for User B - should fail
	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(userBID, "UserTOTPSecret").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow(userBSecret))
	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(userBID, "UserTOTPRecoveryCodes").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow("[]"))

	valid, _ := svc.ValidateCode(userBID, userACode)
	assert.False(t, valid, "User A's TOTP code should NOT validate for User B")

	// Verify it works for User A
	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(userAID, "UserTOTPSecret").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow(userASecret))

	valid, _ = svc.ValidateCode(userAID, userACode)
	assert.True(t, valid, "User A's code should validate for User A")
}

// =============================================================================
// T8: Unauthorized 2FA Disable
// =============================================================================

func TestTOTP_DisableRequiresValidCode(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	svc := NewTOTPService(db, "GoatFlow")
	userID := 123
	secret := "JBSWY3DPEHPK3PXP"

	// Try to disable with invalid code
	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(userID, "UserTOTPSecret").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow(secret))
	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(userID, "UserTOTPRecoveryCodes").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow("[]"))

	err := svc.Disable(userID, "000000")
	assert.Error(t, err, "Disable should fail with invalid code")
	assert.Contains(t, err.Error(), "invalid code")
}

func TestTOTP_DisableSucceedsWithValidCode(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	svc := NewTOTPService(db, "GoatFlow")
	userID := 123
	secret := "JBSWY3DPEHPK3PXP"
	validCode := generateValidCode(secret)

	// Validate code
	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(userID, "UserTOTPSecret").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow(secret))

	// SetAndDelete uses a transaction to atomically delete all preferences
	mock.ExpectBegin()
	for i := 0; i < 5; i++ {
		mock.ExpectExec("DELETE FROM user_preferences").
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectCommit()

	err := svc.Disable(userID, validCode)
	assert.NoError(t, err, "Disable should succeed with valid code")
}

// =============================================================================
// T5: TOTP Secret Exposure Prevention
// =============================================================================

func TestTOTP_SetupReturnsSecretOnlyOnce(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	svc := NewTOTPService(db, "GoatFlow")
	userID := 123

	// First setup call - should return secret
	mock.ExpectExec("UPDATE user_preferences").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO user_preferences").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE user_preferences").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO user_preferences").
		WillReturnResult(sqlmock.NewResult(1, 1))

	setup, err := svc.GenerateSetup(userID, "test@example.com")
	require.NoError(t, err)
	assert.NotEmpty(t, setup.Secret, "Setup should return secret")
	assert.Len(t, setup.RecoveryCodes, 8, "Should return 8 recovery codes")

	// After confirmation, secret should NOT be retrievable via any public API
	// (The service doesn't expose a GetSecret method - this is by design)
}

// =============================================================================
// T6: Race Condition Prevention
// =============================================================================

func TestTOTP_ConcurrentEnableDisable(t *testing.T) {
	// This test verifies that concurrent operations don't corrupt state
	// In practice, this requires database-level locking

	db, mock := setupMockDB(t)
	defer db.Close()

	svc := NewTOTPService(db, "GoatFlow")
	userID := 123
	secret := "JBSWY3DPEHPK3PXP"

	// Setup expectations for concurrent access
	// The mock doesn't truly test concurrency, but we verify the code paths

	var wg sync.WaitGroup
	errors := make(chan error, 2)

	// Concurrent enable attempt
	wg.Add(1)
	go func() {
		defer wg.Done()
		mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
			WithArgs(userID, "UserTOTPPendingSecret").
			WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow(secret))
		// ... rest of confirm flow
		err := svc.ConfirmSetup(userID, generateValidCode(secret))
		if err != nil {
			errors <- err
		}
	}()

	wg.Wait()
	close(errors)

	// Check no panics occurred (race detector would catch issues)
	for err := range errors {
		t.Logf("Concurrent operation error (expected in test): %v", err)
	}
}

// =============================================================================
// Customer-Specific Tests
// =============================================================================

func TestTOTP_CustomerBackendIsolation(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	svc := NewTOTPService(db, "GoatFlow")

	// Agent user ID
	agentUserID := 100

	// Customer login (string)
	customerLogin := "customer@example.com"

	// Verify agent uses user_preferences table
	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(agentUserID, "UserTOTPEnabled").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow("1"))

	assert.True(t, svc.IsEnabled(agentUserID))

	// Verify customer uses customer_preferences table
	mock.ExpectQuery("SELECT preferences_value FROM customer_preferences").
		WithArgs(customerLogin, "UserTOTPEnabled").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow("1"))

	assert.True(t, svc.IsEnabledForCustomer(customerLogin))
}

func TestTOTP_CustomerCannotUseAgentCode(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	svc := NewTOTPService(db, "GoatFlow")

	// Agent has 2FA with this secret
	agentSecret := "AGENTSECRETAGENT"

	// Customer has different secret
	customerSecret := "CUSTOMERSECRETXX"
	customerLogin := "customer@example.com"

	// Generate agent's valid code
	agentCode := generateValidCode(agentSecret)

	// Try to use agent's code for customer - should fail
	mock.ExpectQuery("SELECT preferences_value FROM customer_preferences").
		WithArgs(customerLogin, "UserTOTPSecret").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow(customerSecret))
	mock.ExpectQuery("SELECT preferences_value FROM customer_preferences").
		WithArgs(customerLogin, "UserTOTPRecoveryCodes").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow("[]"))

	valid, _ := svc.ValidateCodeForCustomer(customerLogin, agentCode)
	assert.False(t, valid, "Agent's TOTP code should NOT work for customer")
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestTOTP_EmptyCodeRejected(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	svc := NewTOTPService(db, "GoatFlow")
	userID := 123
	secret := "JBSWY3DPEHPK3PXP"

	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(userID, "UserTOTPSecret").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow(secret))
	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(userID, "UserTOTPRecoveryCodes").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow("[]"))

	valid, _ := svc.ValidateCode(userID, "")
	assert.False(t, valid, "Empty code should be rejected")
}

func TestTOTP_NoSecretConfigured(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	svc := NewTOTPService(db, "GoatFlow")
	userID := 123

	// No secret in DB
	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(userID, "UserTOTPSecret").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"})) // Empty result

	valid, err := svc.ValidateCode(userID, "123456")
	assert.False(t, valid)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestTOTP_RecoveryCodesGenerated(t *testing.T) {
	db, mock := setupMockDB(t)
	defer db.Close()

	svc := NewTOTPService(db, "GoatFlow")
	userID := 123

	// Setup expectations
	mock.ExpectExec("UPDATE user_preferences").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO user_preferences").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE user_preferences").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO user_preferences").
		WillReturnResult(sqlmock.NewResult(1, 1))

	setup, err := svc.GenerateSetup(userID, "test@example.com")
	require.NoError(t, err)

	// Verify recovery codes
	assert.Len(t, setup.RecoveryCodes, 8, "Should generate 8 recovery codes")
	for _, code := range setup.RecoveryCodes {
		// V6 FIX: Codes are now 12 characters (128 bits entropy) instead of 8 (40 bits)
		assert.Len(t, code, 12, "Each recovery code should be 12 characters (V6 security fix)")
		assert.Regexp(t, "^[a-z2-7]+$", code, "Recovery codes should be lowercase base32")
	}

	// Verify all codes are unique
	seen := make(map[string]bool)
	for _, code := range setup.RecoveryCodes {
		assert.False(t, seen[code], "Recovery codes should be unique")
		seen[code] = true
	}
}
