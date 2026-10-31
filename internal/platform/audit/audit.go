// Package audit records security-relevant events (sign-ins, account and
// credential changes, administrative changes and, when enabled, read access)
// in the gk_audit_event table.
//
// Recording never fails the caller's request: a write error is logged and
// the request carries on. Every recorded event is also written to the
// application log as one structured "audit" line, for log shippers.
package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// Categories group actions in the viewer.
const (
	CategoryAuth   = "auth"   // sign-in, sign-out, 2FA, passwords, tokens, sessions
	CategoryAdmin  = "admin"  // changes made through admin pages and admin APIs
	CategoryAccess = "access" // read access (ticket views, downloads, exports); opt-in
	CategorySystem = "system" // retention purges and other unattended actions
)

// Outcomes.
const (
	OutcomeSuccess = "success"
	OutcomeFailure = "failure" // the action was attempted and failed (wrong password, invalid input)
	OutcomeDenied  = "denied"  // the caller was not allowed to do it
)

// Actor types.
const (
	ActorAgent     = "agent"
	ActorCustomer  = "customer"
	ActorAnonymous = "anonymous" // not signed in (failed sign-ins, password reset requests)
	ActorSystem    = "system"
	ActorPlugin    = "plugin"
)

// Column limits; longer values are truncated.
const (
	maxAction    = 100
	maxLogin     = 255
	maxIP        = 45
	maxUserAgent = 255
	maxTarget    = 100
	maxLabel     = 255
)

// Event is one audit record. Category, Action and Outcome are required;
// Time defaults to now.
type Event struct {
	Time     time.Time
	Category string
	// Action is a dotted name: area.object.verb, e.g. "auth.login",
	// "admin.user.update", "access.ticket.view".
	Action  string
	Outcome string

	ActorType  string
	ActorID    int64
	ActorLogin string
	IP         string
	UserAgent  string

	TargetType  string
	TargetID    string
	TargetLabel string

	// Details holds event-specific context: a reason, the request route,
	// field changes from Changes. Secrets must not be put here.
	Details map[string]any
}

// Record writes e to gk_audit_event using the shared database connection.
func Record(ctx context.Context, e Event) {
	db, err := database.GetDB()
	if err != nil || db == nil {
		logEvent(e, fmt.Errorf("no database: %w", err))
		return
	}
	RecordDB(ctx, db, e)
}

// RecordDB writes e to gk_audit_event using db.
func RecordDB(ctx context.Context, db *sql.DB, e Event) {
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	if e.ActorType == "" {
		e.ActorType = ActorSystem
	}
	err := insert(ctx, db, e)
	logEvent(e, err)
}

func insert(ctx context.Context, db *sql.DB, e Event) error {
	if e.Category == "" || e.Action == "" || e.Outcome == "" {
		return fmt.Errorf("audit event needs category, action and outcome")
	}
	var details sql.NullString
	if len(e.Details) > 0 {
		b, err := json.Marshal(e.Details)
		if err != nil {
			return fmt.Errorf("encode details: %w", err)
		}
		details = sql.NullString{String: string(b), Valid: true}
	}
	_, err := db.ExecContext(ctx, database.ConvertPlaceholders(`
		INSERT INTO gk_audit_event (
			create_time, category, action, outcome,
			actor_type, actor_id, actor_login, ip_address, user_agent,
			target_type, target_id, target_label, details
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`),
		e.Time.UTC(), e.Category, truncate(e.Action, maxAction), e.Outcome,
		e.ActorType, nullInt(e.ActorID), nullString(truncate(e.ActorLogin, maxLogin)),
		nullString(truncate(e.IP, maxIP)), nullString(truncate(e.UserAgent, maxUserAgent)),
		nullString(truncate(e.TargetType, maxTarget)), nullString(truncate(e.TargetID, maxTarget)),
		nullString(truncate(e.TargetLabel, maxLabel)), details)
	return err
}

func logEvent(e Event, err error) {
	attrs := []any{
		"category", e.Category, "action", e.Action, "outcome", e.Outcome,
		"actor_type", e.ActorType, "actor_id", e.ActorID, "actor_login", e.ActorLogin,
		"ip", e.IP, "target_type", e.TargetType, "target_id", e.TargetID,
	}
	if err != nil {
		slog.Error("audit: event not stored", append(attrs, "error", err)...)
		return
	}
	slog.Info("audit", attrs...)
}

// Purge deletes events created before cutoff and returns how many it removed.
func Purge(ctx context.Context, db *sql.DB, cutoff time.Time) (int64, error) {
	res, err := db.ExecContext(ctx, database.ConvertPlaceholders(
		`DELETE FROM gk_audit_event WHERE create_time < ?`), cutoff.UTC())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ID formats a numeric id for Event.TargetID.
func ID(id int64) string {
	if id == 0 {
		return ""
	}
	return strconv.FormatInt(id, 10)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	// Cut on a rune boundary.
	for n > 0 && !utf8Start(s[n]) {
		n--
	}
	return s[:n]
}

func utf8Start(b byte) bool { return b&0xC0 != 0x80 }

func nullString(s string) sql.NullString {
	s = strings.TrimSpace(s)
	return sql.NullString{String: s, Valid: s != ""}
}

func nullInt(v int64) sql.NullInt64 { return sql.NullInt64{Int64: v, Valid: v != 0} }
