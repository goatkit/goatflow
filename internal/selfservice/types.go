// Package selfservice implements the public self-service account pages:
// forgotten-password reset for agents and customers, and customer
// self-registration with email verification.
//
// Both flows are switched by config/default.yaml: features.lost_password
// (agent + customer password reset) and features.registration (customer
// sign-up). Links in emails are built from BASE_URL, the operator-set public
// URL of the instance; the request Host header is never trusted for that.
package selfservice

import "time"

// Token types stored in gk_auth_token.token_type.
const (
	TokenPasswordReset = "password_reset"
	TokenEmailVerify   = "email_verify"
)

// Account types stored in gk_auth_token.user_type.
const (
	UserAgent    = "agent"
	UserCustomer = "customer"
)

// Registration request statuses (gk_registration_request.status).
const (
	StatusPending  = "pending"
	StatusApproved = "approved"
)

// Token lifetimes and request budgets.
const (
	// ResetTokenTTL is how long a password reset link stays valid.
	ResetTokenTTL = 1 * time.Hour
	// VerifyTokenTTL is how long a registration confirmation link stays valid.
	VerifyTokenTTL = 24 * time.Hour

	// ipRequestsPerHour caps forgot-password, reset-password and registration
	// submissions per client IP (one shared budget).
	ipRequestsPerHour = 10
	// mailsPerTargetPerHour caps emails sent to one account or address, so the
	// public forms cannot be used to flood someone's inbox.
	mailsPerTargetPerHour = 3
	// maxAccountsPerRequest bounds how many accounts one identifier may match.
	maxAccountsPerRequest = 5
)

// Account is an agent (users) or customer (customer_user) that can receive a
// reset link. Email is where the link goes; the token is bound to it.
type Account struct {
	Type      string
	ID        int
	Login     string
	Email     string
	FirstName string
	LastName  string
}

// sessionUserType is the UserType value the session layer stores for the account type.
func (a Account) sessionUserType() string {
	if a.Type == UserCustomer {
		return "Customer"
	}
	return "User"
}
