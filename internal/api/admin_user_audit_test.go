package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/audit"
	"github.com/goatkit/goatflow/internal/platform/database"
)

// An admin user update records one admin.user.update event whose changes
// show the edited name and that the password changed, but never the password.
func TestAdminUserUpdateAudit(t *testing.T) {
	db := isolatedDB(t)
	id, login := createIsolatedAgent(t, "audit_upd")
	targetID := strconv.Itoa(id)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(
			`DELETE FROM gk_audit_event WHERE target_type = 'user' AND target_id = ?`), targetID)
	})

	const password = "AuditSecret-9871-Pw"
	code, response := adminUsersJSON(t, http.MethodPut, "/admin/users/:id", "/admin/users/"+targetID,
		HandleAdminUserUpdate, "application/x-www-form-urlencoded",
		url.Values{"login": {login}, "first_name": {"Renamed"}, "last_name": {"Agent"}, "valid_id": {"1"},
			"password": {password}, "confirm_password": {password}}.Encode())
	require.Equal(t, http.StatusOK, code, response)

	var outcome, gotTarget, details string
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`
		SELECT outcome, target_id, details FROM gk_audit_event
		WHERE action = 'admin.user.update' AND target_type = 'user' AND target_id = ?
		ORDER BY id DESC LIMIT 1`), targetID).Scan(&outcome, &gotTarget, &details))

	assert.Equal(t, audit.OutcomeSuccess, outcome)
	assert.Equal(t, targetID, gotTarget)
	assert.NotContains(t, details, password)
	_, _, hash, _ := adminTestUserRow(t, id)
	assert.False(t, strings.Contains(details, hash), "details must not hold the password hash")

	var parsed struct {
		Changes map[string]map[string]any `json:"changes"`
	}
	require.NoError(t, json.Unmarshal([]byte(details), &parsed), details)
	assert.Equal(t, map[string]any{"from": "Isolated", "to": "Renamed"}, parsed.Changes["first_name"])
	assert.Equal(t, map[string]any{"from": "", "to": audit.Redacted}, parsed.Changes["password"])
	assert.NotContains(t, parsed.Changes, "last_name", "unchanged fields are not listed")
}
