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

func mfaLoginPageContext(status mfaStatus) pongo2.Context {
	showTOTPForm := status.TOTPEnabled || !status.WebAuthnEnabled
	return pongo2.Context{
		"totp_enabled":      status.TOTPEnabled,
		"webauthn_enabled":  status.WebAuthnEnabled,
		"security_key_only": status.WebAuthnEnabled && !status.TOTPEnabled,
		"show_totp_form":    showTOTPForm,
	}
}
