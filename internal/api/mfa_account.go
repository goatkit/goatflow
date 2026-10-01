package api

import (
	"database/sql"
	"net/http"

	"github.com/goatkit/goatflow/internal/platform/service"
)

// mfaAccount names the account whose second factors (authenticator app,
// passkeys/security keys, recovery codes) a request reads or changes. Agents
// are keyed by user ID, customers by login.
type mfaAccount struct {
	userType string // service.WebAuthnUserTypeAgent or service.WebAuthnUserTypeCustomer
	userID   int    // agents only
	login    string // customers only
}

func agentMFAAccount(userID int) mfaAccount {
	return mfaAccount{userType: service.WebAuthnUserTypeAgent, userID: userID}
}

func customerMFAAccount(login string) mfaAccount {
	return mfaAccount{userType: service.WebAuthnUserTypeCustomer, login: login}
}

func (a mfaAccount) isCustomer() bool {
	return a.userType == service.WebAuthnUserTypeCustomer
}

func (a mfaAccount) webAuthnKey() string {
	if a.isCustomer() {
		return a.login
	}
	return service.AgentWebAuthnUserKey(a.userID)
}

// totp returns a TOTPService bound to this account's preference store, so its
// userID-taking methods address the right account for agents and customers.
func (a mfaAccount) totp(db *sql.DB) *service.TOTPService {
	base := service.NewTOTPService(db, "GoatFlow")
	if a.isCustomer() {
		return base.ForCustomer(a.login)
	}
	return base.ForUser(a.userID)
}

// hasAnySecondFactor reports whether the account still has an authenticator
// app or at least one passkey.
func (a mfaAccount) hasAnySecondFactor(db *sql.DB, r *http.Request) bool {
	if a.totp(db).IsEnabled(a.userID) {
		return true
	}
	wa, err := service.NewWebAuthnService(db, r)
	return err == nil && wa.IsEnabled(a.userType, a.webAuthnKey())
}

// clearRecoveryCodesIfNoSecondFactor drops recovery codes once the last second
// factor is gone, so 2FA is fully off rather than left half-on.
func (a mfaAccount) clearRecoveryCodesIfNoSecondFactor(db *sql.DB, r *http.Request) error {
	if a.hasAnySecondFactor(db, r) {
		return nil
	}
	return a.totp(db).ClearRecoveryCodes(a.userID)
}

// deleteAllPasskeys removes every passkey/security key on the account.
func (a mfaAccount) deleteAllPasskeys(db *sql.DB, r *http.Request) error {
	wa, err := service.NewWebAuthnService(db, r)
	if err != nil {
		return err
	}
	return wa.DeleteAllCredentials(a.userType, a.webAuthnKey())
}
