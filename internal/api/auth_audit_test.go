package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/auth"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/service"
)

type auditRow struct {
	Category, Outcome, ActorType string
	ActorID                      sql.NullInt64
	ActorLogin, IP               sql.NullString
	TargetType, TargetID         sql.NullString
	TargetLabel, Details         sql.NullString
}

// auditRowsWhere returns the gk_audit_event rows for action whose column
// equals value, oldest first.
func auditRowsWhere(t *testing.T, action, column, value string) []auditRow {
	t.Helper()
	db, err := database.GetDB()
	require.NoError(t, err)
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT category, outcome, actor_type, actor_id, actor_login, ip_address,
		       target_type, target_id, target_label, details
		  FROM gk_audit_event WHERE action = ? AND `+column+` = ? ORDER BY id`), action, value)
	require.NoError(t, err)
	defer rows.Close()
	var out []auditRow
	for rows.Next() {
		var r auditRow
		require.NoError(t, rows.Scan(&r.Category, &r.Outcome, &r.ActorType, &r.ActorID, &r.ActorLogin, &r.IP,
			&r.TargetType, &r.TargetID, &r.TargetLabel, &r.Details))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

func auditDetails(t *testing.T, r auditRow) map[string]any {
	t.Helper()
	require.True(t, r.Details.Valid, "event has details")
	var d map[string]any
	require.NoError(t, json.Unmarshal([]byte(r.Details.String), &d))
	return d
}

func deleteAuditRows(t *testing.T, column, value string) {
	t.Cleanup(func() {
		db, err := database.GetDB()
		if err == nil {
			_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM gk_audit_event WHERE `+column+` = ?`), value)
		}
	})
}

// A password sign-in records auth.login: a refused one under the login name
// that was tried, as anonymous, with a reason and never the password; an
// accepted one as the agent, with its id.
func TestPasswordSignInIsAudited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.GetDB()
	require.NoError(t, err)
	require.NotNil(t, GetAuthService(), "auth service needs the test database")

	sfx := fmt.Sprintf("%d", time.Now().UnixNano())
	login, password, wrong := "audit_login_"+sfx, "Audit-Login-"+sfx, "Wrong-Secret-"+sfx
	hash, err := auth.NewPasswordHasher().HashPassword(password)
	require.NoError(t, err)
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, ?, 'Audit', 'Agent', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1) RETURNING id`), login, hash)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM user_preferences WHERE user_id = ?"), id)
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM users WHERE id = ?"), id)
	})
	unknown := "audit_nobody_" + sfx
	deleteAuditRows(t, "actor_login", login)
	deleteAuditRows(t, "actor_login", unknown)

	r := gin.New()
	r.POST("/api/auth/login", HandleAuthLogin)
	post := func(login, password string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login",
			strings.NewReader(fmt.Sprintf(`{"login":%q,"password":%q}`, login, password)))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "203.0.113.41:4000"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	require.Equal(t, http.StatusUnauthorized, post(login, wrong).Code)
	rows := auditRowsWhere(t, "auth.login", "actor_login", login)
	require.Len(t, rows, 1)
	assert.Equal(t, "auth", rows[0].Category)
	assert.Equal(t, "failure", rows[0].Outcome)
	assert.Equal(t, "anonymous", rows[0].ActorType)
	assert.Equal(t, "203.0.113.41", rows[0].IP.String)
	assert.Equal(t, "bad_password", auditDetails(t, rows[0])["reason"])
	assert.NotContains(t, rows[0].Details.String, wrong)

	require.Equal(t, http.StatusUnauthorized, post(unknown, wrong).Code)
	rows = auditRowsWhere(t, "auth.login", "actor_login", unknown)
	require.Len(t, rows, 1)
	assert.Equal(t, "unknown_user", auditDetails(t, rows[0])["reason"])

	w := post(login, password)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	rows = auditRowsWhere(t, "auth.login", "actor_login", login)
	require.Len(t, rows, 2)
	ok := rows[1]
	assert.Equal(t, "success", ok.Outcome)
	assert.Equal(t, "agent", ok.ActorType)
	assert.Equal(t, id, ok.ActorID.Int64)
	assert.Equal(t, "password", auditDetails(t, ok)["method"])
	assert.NotContains(t, ok.Details.String, password)
}

// An admin turning off another agent's 2FA is recorded as admin.2fa.disable
// with the admin as actor, the agent as target and the admin's reason.
func TestAdmin2FAOverrideIsAudited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.GetDB()
	require.NoError(t, err)

	sfx := fmt.Sprintf("%d", time.Now().UnixNano())
	login := "audit_2fa_" + sfx
	id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
		INSERT INTO users (login, pw, first_name, last_name, valid_id, create_time, create_by, change_time, change_by)
		VALUES (?, 'x', 'Audit', 'Agent', 1, CURRENT_TIMESTAMP, 1, CURRENT_TIMESTAMP, 1) RETURNING id`), login)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM user_preferences WHERE user_id = ?"), id)
		_, _ = db.Exec(database.ConvertPlaceholders("DELETE FROM users WHERE id = ?"), id)
	})
	deleteAuditRows(t, "target_label", login)

	totpSvc := service.NewTOTPService(db, "GoatFlow")
	setup, err := totpSvc.GenerateSetup(int(id), login+"@example.test")
	require.NoError(t, err)
	code, err := totp.GenerateCode(setup.Secret, time.Now())
	require.NoError(t, err)
	require.NoError(t, totpSvc.ConfirmSetup(int(id), code))

	r := gin.New()
	r.POST("/admin/users/:id/2fa/disable", func(c *gin.Context) {
		c.Set("user_id", 1)
		c.Set("user_role", "Admin")
		c.Next()
	}, handleAdmin2FAOverride)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/users/%d/2fa/disable", id),
		strings.NewReader(`{"reason":"lost phone, ticket 42"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	rows := auditRowsWhere(t, "admin.2fa.disable", "target_label", login)
	require.Len(t, rows, 1)
	e := rows[0]
	assert.Equal(t, "admin", e.Category)
	assert.Equal(t, "success", e.Outcome)
	assert.Equal(t, "agent", e.ActorType)
	assert.Equal(t, int64(1), e.ActorID.Int64)
	assert.Equal(t, "user", e.TargetType.String)
	assert.Equal(t, fmt.Sprint(id), e.TargetID.String)
	assert.Equal(t, "lost phone, ticket 42", auditDetails(t, e)["reason"])
}
