package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

func newMFATestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "https://example.com/api/preferences/2fa", nil)
	return c, w
}

func newMFATestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

// "Turn off 2FA" must remove passkeys as well. Leaving them made the login
// page demand a passkey after the user chose "off" (and locked out a user
// whose passkey belonged to another host name).
func TestTurnOff2FARemovesPasskeysToo(t *testing.T) {
	db, mock := newMFATestDB(t)
	secret := "JBSWY3DPEHPK3PXP"
	code, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)

	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(42, "UserTOTPSecret").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow(secret))
	mock.ExpectBegin()
	for _, key := range []string{"UserTOTPSecret", "UserTOTPEnabled", "UserTOTPRecoveryCodes", "UserTOTPPendingSecret", "UserTOTPPendingRecoveryCodes"} {
		mock.ExpectExec("DELETE FROM user_preferences").WithArgs(42, key).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectCommit()
	mock.ExpectExec("DELETE FROM gk_webauthn_credential WHERE user_type").
		WithArgs("agent", "42").
		WillReturnResult(sqlmock.NewResult(0, 1))

	c, w := newMFATestContext(t)
	require.True(t, turnOffSecondFactors(c, db, agentMFAAccount(42), code), w.Body.String())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTurnOff2FARejectsWrongCodeAndKeepsPasskeys(t *testing.T) {
	db, mock := newMFATestDB(t)
	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(42, "UserTOTPSecret").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}).AddRow("JBSWY3DPEHPK3PXP"))
	mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
		WithArgs(42, "UserTOTPRecoveryCodes").
		WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}))

	c, w := newMFATestContext(t)
	assert.False(t, turnOffSecondFactors(c, db, agentMFAAccount(42), "000000"))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRemovingPasskeyClearsRecoveryCodesOnlyWithLastFactor(t *testing.T) {
	expectDelete := func(mock sqlmock.Sqlmock) {
		mock.ExpectExec("DELETE FROM gk_webauthn_credential WHERE id").
			WithArgs(int64(7), "agent", "42").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery("SELECT preferences_value FROM user_preferences").
			WithArgs(42, "UserTOTPEnabled").
			WillReturnRows(sqlmock.NewRows([]string{"preferences_value"}))
	}

	t.Run("last factor: 2FA off, codes deleted", func(t *testing.T) {
		db, mock := newMFATestDB(t)
		expectDelete(mock)
		mock.ExpectQuery("SELECT COUNT").WithArgs("agent", "42").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectExec("DELETE FROM user_preferences").WithArgs(42, "UserTOTPRecoveryCodes").
			WillReturnResult(sqlmock.NewResult(0, 1))

		c, w := newMFATestContext(t)
		c.Params = gin.Params{{Key: "id", Value: "7"}}
		deleteWebAuthnCredential(c, db, agentMFAAccount(42))
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("another passkey left: codes kept", func(t *testing.T) {
		db, mock := newMFATestDB(t)
		expectDelete(mock)
		mock.ExpectQuery("SELECT COUNT").WithArgs("agent", "42").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		c, w := newMFATestContext(t)
		c.Params = gin.Params{{Key: "id", Value: "7"}}
		deleteWebAuthnCredential(c, db, agentMFAAccount(42))
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

// Turning 2FA off removes stored passkeys with plain SQL; it must not depend
// on a WebAuthn relying-party config, which cannot be built for a single-label
// host name such as an intranet "helpdesk" (it returned 500 after the
// authenticator app was already gone).
func TestTurnOff2FAOnSingleLabelHost(t *testing.T) {
	db := getTestDB(t)
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 'x', 'Desk', 'Agent', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1)
		RETURNING id`), fmt.Sprintf("mfa-desk-%d", time.Now().UnixNano()))
	require.NoError(t, err)
	userID := int(id)
	account := agentMFAAccount(userID)
	key := account.webAuthnKey()
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM user_preferences WHERE user_id = ?"), userID)
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM gk_webauthn_credential WHERE user_type = ? AND user_key = ?"), "agent", key)
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM users WHERE id = ?"), userID)
	})

	totpSvc := account.totp(db)
	setup, err := totpSvc.GenerateSetup(userID, "helpdesk@example.com")
	require.NoError(t, err)
	code, err := totp.GenerateCode(setup.Secret, time.Now())
	require.NoError(t, err)
	require.NoError(t, totpSvc.ConfirmSetup(userID, code))
	_, err = db.Exec(database.ConvertPlaceholders(`
		INSERT INTO gk_webauthn_credential (user_type, user_key, credential_id, credential_json, name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`), "agent", key, fmt.Sprintf("cred-%d", userID), "{}", "Desk key")
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "http://helpdesk:8080/api/preferences/2fa/disable", nil)

	require.True(t, turnOffSecondFactors(c, db, account, code), w.Body.String())
	assert.False(t, totpSvc.IsEnabled(userID), "authenticator app should be off")
	var passkeys int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		"SELECT COUNT(*) FROM gk_webauthn_credential WHERE user_type = ? AND user_key = ?"), "agent", key).Scan(&passkeys))
	assert.Zero(t, passkeys, "passkeys should be removed")
}

// A passkey-only account must be offered its recovery codes on the second
// step, and never a password-only way in.
func TestMFALoginPageOffersRecoveryCodesToPasskeyOnlyAccounts(t *testing.T) {
	tests := []struct {
		name         string
		status       mfaStatus
		showTOTP     bool
		keyOnly      bool
		showRecovery bool
	}{
		{"passkey only with codes", mfaStatus{WebAuthnEnabled: true, RecoveryCodes: 8}, false, true, true},
		{"passkey only, no codes", mfaStatus{WebAuthnEnabled: true}, false, true, false},
		{"app and passkey", mfaStatus{TOTPEnabled: true, WebAuthnEnabled: true, RecoveryCodes: 8}, true, false, false},
		{"app only", mfaStatus{TOTPEnabled: true, RecoveryCodes: 8}, true, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := mfaLoginPageContext(tt.status)
			assert.Equal(t, tt.showTOTP, ctx["show_totp_form"])
			assert.Equal(t, tt.keyOnly, ctx["security_key_only"])
			assert.Equal(t, tt.showRecovery, ctx["show_recovery_form"])
		})
	}
}
