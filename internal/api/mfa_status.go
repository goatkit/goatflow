package api

import (
	"database/sql"
	"errors"

	"github.com/flosch/pongo2/v6"

	"github.com/goatkit/goatflow/internal/platform/service"
)

type mfaStatus struct {
	TOTPEnabled     bool
	WebAuthnEnabled bool
	RecoveryCodes   int // unused recovery codes left
}

func (s mfaStatus) Enabled() bool {
	return s.TOTPEnabled || s.WebAuthnEnabled
}

var errMFAStatusNoDatabase = errors.New("second-factor status: database unavailable")

// agentMFAStatus reads an agent's second-factor setup. Any lookup failure is
// returned: login gates must fail closed rather than skip the second factor.
func agentMFAStatus(db *sql.DB, userID int) (mfaStatus, error) {
	if db == nil {
		return mfaStatus{}, errMFAStatusNoDatabase
	}
	totp, codes, err := service.NewTOTPService(db, "GoatFlow").LoginStatus(userID)
	if err != nil {
		return mfaStatus{}, err
	}
	passkeys, err := service.CountWebAuthnCredentials(db, service.WebAuthnUserTypeAgent, service.AgentWebAuthnUserKey(userID))
	if err != nil {
		return mfaStatus{}, err
	}
	return mfaStatus{TOTPEnabled: totp, WebAuthnEnabled: passkeys > 0, RecoveryCodes: codes}, nil
}

// customerMFAStatus is agentMFAStatus for a customer account.
func customerMFAStatus(db *sql.DB, login string) (mfaStatus, error) {
	if db == nil {
		return mfaStatus{}, errMFAStatusNoDatabase
	}
	totp, codes, err := service.NewTOTPService(db, "GoatFlow").LoginStatusForCustomer(login)
	if err != nil {
		return mfaStatus{}, err
	}
	passkeys, err := service.CountWebAuthnCredentials(db, service.WebAuthnUserTypeCustomer, login)
	if err != nil {
		return mfaStatus{}, err
	}
	return mfaStatus{TOTPEnabled: totp, WebAuthnEnabled: passkeys > 0, RecoveryCodes: codes}, nil
}

// mfaLoginPageContext decides what the second-step login page offers. A
// passkey-only account gets "Other ways to sign in": its recovery codes, or a
// pointer to an administrator when it has none. Never the password alone.
func mfaLoginPageContext(status mfaStatus) pongo2.Context {
	showTOTPForm := status.TOTPEnabled || !status.WebAuthnEnabled
	securityKeyOnly := status.WebAuthnEnabled && !status.TOTPEnabled
	return pongo2.Context{
		"totp_enabled":       status.TOTPEnabled,
		"webauthn_enabled":   status.WebAuthnEnabled,
		"security_key_only":  securityKeyOnly,
		"show_totp_form":     showTOTPForm,
		"show_recovery_form": securityKeyOnly && status.RecoveryCodes > 0,
	}
}
