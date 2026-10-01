package api

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	mock.ExpectExec(`DELETE FROM gk_webauthn_credential WHERE user_type = \? AND user_key = \?`).
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
		mock.ExpectExec(`DELETE FROM gk_webauthn_credential WHERE id = \?`).
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
