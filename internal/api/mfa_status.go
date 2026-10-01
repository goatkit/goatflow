package api

import (
	"database/sql"
	"net/http"

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

func agentMFAStatus(db *sql.DB, r *http.Request, userID int) mfaStatus {
	status := mfaStatus{}
	if db == nil {
		return status
	}
	totpService := service.NewTOTPService(db, "GoatFlow")
	status.TOTPEnabled = totpService.IsEnabled(userID)
	status.RecoveryCodes = totpService.GetRemainingRecoveryCodes(userID)
	wa, err := service.NewWebAuthnService(db, r)
	if err == nil && wa != nil {
		status.WebAuthnEnabled = wa.IsEnabled(service.WebAuthnUserTypeAgent, service.AgentWebAuthnUserKey(userID))
	}
	return status
}

func customerMFAStatus(db *sql.DB, r *http.Request, login string) mfaStatus {
	status := mfaStatus{}
	if db == nil {
		return status
	}
	totpService := service.NewTOTPService(db, "GoatFlow")
	status.TOTPEnabled = totpService.IsEnabledForCustomer(login)
	status.RecoveryCodes = totpService.GetRemainingRecoveryCodesForCustomer(login)
	wa, err := service.NewWebAuthnService(db, r)
	if err == nil && wa != nil {
		status.WebAuthnEnabled = wa.IsEnabled(service.WebAuthnUserTypeCustomer, login)
	}
	return status
}

func isAgentMFAEnabled(db *sql.DB, r *http.Request, userID int) bool {
	return agentMFAStatus(db, r, userID).Enabled()
}

func isCustomerMFAEnabled(db *sql.DB, r *http.Request, login string) bool {
	return customerMFAStatus(db, r, login).Enabled()
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
