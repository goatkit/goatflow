package audit

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	platformmodels "github.com/goatkit/goatflow/internal/platform/models"
)

// recordedKey marks a request whose handler recorded its own event, so the
// admin request middleware does not add a second, generic one.
const recordedKey = "audit_recorded"

// FromRequest returns an event with the caller (as set by the auth
// middleware), client IP and user agent filled in.
func FromRequest(c *gin.Context, category, action, outcome string) Event {
	e := Event{Category: category, Action: action, Outcome: outcome}
	if c == nil || c.Request == nil {
		return e
	}
	e.IP = c.ClientIP()
	e.UserAgent = c.Request.UserAgent()
	e.ActorType, e.ActorID, e.ActorLogin = actor(c)
	if tok, ok := c.Get("api_token"); ok {
		if t, isTok := tok.(*platformmodels.APIToken); isTok && t != nil {
			e.setDetail("api_token_id", t.ID)
		}
	}
	return e
}

// RecordRequest records e for the request: an empty actor, IP or user agent
// is filled from c, and the request is marked as recorded.
func RecordRequest(c *gin.Context, e Event) {
	base := FromRequest(c, e.Category, e.Action, e.Outcome)
	if e.ActorType == "" {
		e.ActorType, e.ActorID, e.ActorLogin = base.ActorType, base.ActorID, base.ActorLogin
	}
	if e.IP == "" {
		e.IP = base.IP
	}
	if e.UserAgent == "" {
		e.UserAgent = base.UserAgent
	}
	if id, ok := base.Details["api_token_id"]; ok {
		e.setDetail("api_token_id", id)
	}
	if c != nil {
		c.Set(recordedKey, true)
		Record(c.Request.Context(), e)
		return
	}
	Record(context.Background(), e)
}

// Recorded reports whether a handler already recorded an event for c.
func Recorded(c *gin.Context) bool {
	v, ok := c.Get(recordedKey)
	b, _ := v.(bool)
	return ok && b
}

func (e *Event) setDetail(k string, v any) {
	if e.Details == nil {
		e.Details = map[string]any{}
	}
	e.Details[k] = v
}

func actor(c *gin.Context) (string, int64, string) {
	rawID, hasID := c.Get("user_id")
	if !hasID {
		return ActorAnonymous, 0, ""
	}
	id := toInt64(rawID)
	typ := ActorAgent
	if isCustomer(c) {
		typ = ActorCustomer
	}
	login := ""
	if u, ok := c.Get("user"); ok {
		switch v := u.(type) {
		case *platformmodels.User:
			if v != nil {
				login = v.Login
			}
		case platformmodels.User:
			login = v.Login
		}
	}
	if login == "" {
		login = c.GetString("username")
	}
	if login == "" {
		login = c.GetString("customer_login")
	}
	if login == "" {
		login = c.GetString("user_email")
	}
	if login == "" {
		if tok, ok := c.Get("api_token"); ok {
			if t, isTok := tok.(*platformmodels.APIToken); isTok && t != nil {
				login = t.CustomerLogin
			}
		}
	}
	return typ, id, login
}

func isCustomer(c *gin.Context) bool {
	if v, ok := c.Get("is_customer"); ok {
		if b, isBool := v.(bool); isBool && b {
			return true
		}
	}
	if c.GetString("user_role") == "Customer" {
		return true
	}
	if tok, ok := c.Get("api_token"); ok {
		if t, isTok := tok.(*platformmodels.APIToken); isTok && t != nil && t.UserType != platformmodels.APITokenUserAgent {
			return true
		}
	}
	return false
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case uint:
		return int64(n) // #nosec G115 -- user ids are positive auto-increment keys
	case uint64:
		return int64(n) // #nosec G115 -- user ids are positive auto-increment keys
	case int32:
		return int64(n)
	case uint32:
		return int64(n)
	}
	return 0
}

// Redacted replaces the value of a sensitive field in Changes.
const Redacted = "[redacted]"

var sensitiveParts = []string{"password", "passwd", "secret", "token", "private", "credential", "api_key", "apikey", "pw"}

// Sensitive reports whether a field name looks like it holds a secret.
func Sensitive(field string) bool {
	f := strings.ToLower(field)
	for _, p := range sensitiveParts {
		if p == "pw" {
			if f == "pw" || strings.HasSuffix(f, "_pw") || strings.HasPrefix(f, "pw_") {
				return true
			}
			continue
		}
		if strings.Contains(f, p) {
			return true
		}
	}
	return f == "key" || strings.HasSuffix(f, "_key") || strings.HasSuffix(f, "key_hex")
}

// Changes compares before and after field values and returns the fields
// that differ as {"field": {"from": old, "to": new}}. Values of sensitive
// fields are replaced with Redacted, so the log shows that a secret changed
// but not what it was or became. A field missing on one side counts as nil.
// It returns nil when nothing changed.
func Changes(before, after map[string]any) map[string]any {
	keys := make(map[string]struct{}, len(before)+len(after))
	for k := range before {
		keys[k] = struct{}{}
	}
	for k := range after {
		keys[k] = struct{}{}
	}
	names := make([]string, 0, len(keys))
	for k := range keys {
		names = append(names, k)
	}
	sort.Strings(names)

	out := map[string]any{}
	for _, k := range names {
		from, to := before[k], after[k]
		if equal(from, to) {
			continue
		}
		if Sensitive(k) {
			from, to = redact(from), redact(to)
		}
		out[k] = map[string]any{"from": from, "to": to}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Snapshot returns the fields of a single-level map with sensitive values
// replaced, for the details of a create or delete event.
func Snapshot(fields map[string]any) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	out := make(map[string]any, len(fields))
	for k, v := range fields {
		if Sensitive(k) {
			v = redact(v)
		}
		out[k] = v
	}
	return out
}

func redact(v any) any {
	if v == nil || fmt.Sprint(v) == "" {
		return v
	}
	return Redacted
}

func equal(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if reflect.DeepEqual(a, b) {
		return true
	}
	// Form values and database values differ in type (string "1" vs int 1).
	return fmt.Sprint(a) == fmt.Sprint(b)
}
