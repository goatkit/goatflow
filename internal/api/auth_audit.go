package api

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/audit"
	"github.com/goatkit/goatflow/internal/platform/auth"
	platformmodels "github.com/goatkit/goatflow/internal/platform/models"
)

// Audit "method" values for sign-ins and second-factor checks.
const (
	methodPassword     = "password"
	methodOIDC         = "oidc"
	methodSAML         = "saml"
	methodPasskey      = "passkey"
	methodTOTP         = "totp"
	methodRecoveryCode = "recovery_code"
)

// recordAuth records an auth event for an explicit actor; IP and user agent
// come from c.
func recordAuth(c *gin.Context, action, outcome, actorType string, actorID int64, login string, details map[string]any) {
	audit.RecordRequest(c, audit.Event{
		Category:   audit.CategoryAuth,
		Action:     action,
		Outcome:    outcome,
		ActorType:  actorType,
		ActorID:    actorID,
		ActorLogin: login,
		Details:    details,
	})
}

// userActor returns the audit actor for a signed-in user record.
func userActor(user *platformmodels.User) (string, int64) {
	if strings.EqualFold(user.Role, "customer") {
		return audit.ActorCustomer, int64(user.ID) // #nosec G115 -- user ids are positive auto-increment keys
	}
	return audit.ActorAgent, int64(user.ID) // #nosec G115 -- user ids are positive auto-increment keys
}

// recordLogin records a completed sign-in by user.
func recordLogin(c *gin.Context, user *platformmodels.User, details map[string]any) {
	typ, id := userActor(user)
	recordAuth(c, "auth.login", audit.OutcomeSuccess, typ, id, user.Login, details)
}

// recordLoginFailure records a refused sign-in attempt under the login name
// that was tried. The caller is not signed in, so the actor is anonymous.
func recordLoginFailure(c *gin.Context, login, outcome string, details map[string]any) {
	recordAuth(c, "auth.login", outcome, audit.ActorAnonymous, 0, login, details)
}

// passwordLoginDetails are the details of a sign-in accepted by provider.
func passwordLoginDetails(provider string) map[string]any {
	return map[string]any{"method": auth.LoginMethod(provider), "provider": strings.ToLower(provider)}
}

// recordPendingSession records an event for the account of a pending
// second-factor session.
func recordPendingSession(c *gin.Context, session *auth.PendingTOTPSession, action, outcome string, details map[string]any) {
	typ, id, login := session.AuditActor()
	recordAuth(c, action, outcome, typ, id, login, details)
}

// secondFactorFailed records a wrong second factor for session, counts it
// against the pending session and, when that locks the session, records the
// lock. It returns the attempts left.
func secondFactorFailed(c *gin.Context, session *auth.PendingTOTPSession, token, method string) int {
	recordPendingSession(c, session, "auth.2fa.verify", audit.OutcomeFailure,
		map[string]any{"method": method, "reason": "invalid_code"})
	remaining := auth.GetTOTPSessionManager().RecordFailedAttempt(token)
	if remaining <= 0 {
		recordPendingSession(c, session, "auth.2fa.locked", audit.OutcomeDenied,
			map[string]any{"method": method, "reason": "too_many_attempts"})
	}
	return remaining
}

// recordSecondFactorLogin records a passed second factor and the sign-in it
// completes. userID fills in the customer id, which pending sessions lack.
func recordSecondFactorLogin(c *gin.Context, session *auth.PendingTOTPSession, method string, userID int64) {
	typ, id, login := session.AuditActor()
	if id == 0 {
		id = userID
	}
	recordAuth(c, "auth.2fa.verify", audit.OutcomeSuccess, typ, id, login, map[string]any{"method": method})
	recordAuth(c, "auth.login", audit.OutcomeSuccess, typ, id, login,
		map[string]any{"method": method, "second_factor": true})
}

// recordPasskeyLogin records a sign-in completed with a passkey, either as
// the second factor of a pending session or as a passwordless sign-in.
func recordPasskeyLogin(c *gin.Context, session *auth.PendingTOTPSession, secondFactor bool, userID int64) {
	if secondFactor {
		recordSecondFactorLogin(c, session, methodPasskey, userID)
		return
	}
	typ, id, login := session.AuditActor()
	if id == 0 {
		id = userID
	}
	recordAuth(c, "auth.login", audit.OutcomeSuccess, typ, id, login, map[string]any{"method": methodPasskey})
}

// rejectSecondFactorIfBlocked is rejectIfLoginBlocked for a second-factor
// check, recording the refused attempt.
func rejectSecondFactorIfBlocked(c *gin.Context, session *auth.PendingTOTPSession, key string) bool {
	if !rejectIfLoginBlocked(c, key) {
		return false
	}
	recordPendingSession(c, session, "auth.2fa.verify", audit.OutcomeDenied, map[string]any{"reason": "rate_limited"})
	return true
}

// codeMethod returns how an accepted code was checked: a recovery code when
// the account has fewer left than before (its use is recorded), otherwise
// the authenticator app.
func codeMethod(c *gin.Context, session *auth.PendingTOTPSession, codesBefore, codesAfter int) string {
	if codesAfter >= codesBefore {
		return methodTOTP
	}
	recordPendingSession(c, session, "auth.2fa.recovery_code_used", audit.OutcomeSuccess,
		map[string]any{"codes_remaining": codesAfter})
	return methodRecoveryCode
}

// recordAccountEvent records an event a signed-in account caused on its own
// second factors or password.
func recordAccountEvent(c *gin.Context, account mfaAccount, action, outcome string, details map[string]any) {
	typ, id, login := audit.ActorAgent, int64(account.userID), c.GetString("username")
	if account.isCustomer() {
		typ, id, login = audit.ActorCustomer, int64(getTOTPUserID(c)), account.login
	}
	if login == "" {
		login = audit.FromRequest(c, "", "", "").ActorLogin
	}
	recordAuth(c, action, outcome, typ, id, login, details)
}
