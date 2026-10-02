package plugin

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/goatkit/goatflow/internal/platform/organisation"
)

// SystemUserID is the users.id the host records for a plugin action that has
// no acting agent: scheduled jobs, plugin init and migrations, calls made for
// a customer, and gRPC callbacks made without a call context. It is the OTRS
// system user (root@localhost).
const SystemUserID int64 = 1

type actingUserKeyType struct{}

var actingUserKey = actingUserKeyType{}

// WithActingUser returns ctx carrying the agent (users.id) on whose behalf a
// plugin call runs.
func WithActingUser(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, actingUserKey, userID)
}

// ActingUserID returns the agent a plugin call runs for, if there is one.
func ActingUserID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(actingUserKey).(int64)
	return id, ok && id > 0
}

// actingUserOrSystem returns the acting agent, or SystemUserID when the call
// has none.
func actingUserOrSystem(ctx context.Context) int64 {
	if id, ok := ActingUserID(ctx); ok {
		return id
	}
	return SystemUserID
}

// withCallEnvelope adds the caller identity of a host-built call envelope to
// ctx, so the plugin's HostAPI callbacks made with the call's context run for
// that caller:
//   - _user_id: the acting agent (customer callers, _customer_login or role
//     "Customer", are not agents and add none);
//   - _org_id: the caller's active organisation (HostAPI OrgID, sandbox
//     ScopeQuery, per-org secure config and file storage);
//   - _lang: the caller's UI language (HostAPI Translate).
//
// The envelope builders strip client-supplied envelope keys, so these are the
// authenticated caller's values.
func withCallEnvelope(ctx context.Context, args []byte) context.Context {
	if len(args) == 0 || args[0] != '{' {
		return ctx
	}
	var env struct {
		UserID        json.RawMessage `json:"_user_id"`
		UserRole      string          `json:"_user_role"`
		CustomerLogin string          `json:"_customer_login"`
		OrgID         json.RawMessage `json:"_org_id"`
		Lang          string          `json:"_lang"`
	}
	if json.Unmarshal(args, &env) != nil {
		return ctx
	}
	if id := envelopeInt(env.UserID); id > 0 && env.CustomerLogin == "" && env.UserRole != "Customer" {
		ctx = WithActingUser(ctx, id)
	}
	if orgID := envelopeInt(env.OrgID); orgID > 0 {
		ctx = organisation.WithOrgID(ctx, orgID)
	}
	if env.Lang != "" {
		ctx = context.WithValue(ctx, PluginLanguageKey, env.Lang)
	}
	return ctx
}

// envelopeInt reads an envelope id sent as a JSON number or numeric string;
// anything else is 0.
func envelopeInt(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	s := string(raw)
	if unquoted, err := strconv.Unquote(s); err == nil {
		s = unquoted
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return id
}
