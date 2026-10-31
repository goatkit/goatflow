package dynamic

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/audit"
	"github.com/goatkit/goatflow/internal/platform/database"
)

// A module update records one admin event whose field changes show that the
// password changed without its value or hash.
func TestDynamicModuleUpdateAuditRedactsPassword(t *testing.T) {
	r := newModulesTestRouter(t)
	db, err := database.GetDB()
	require.NoError(t, err)

	login := fmt.Sprintf("dynaudit%d", time.Now().UnixNano())
	code, body := serveDynamic(t, r, http.MethodPost, "/dynamic/users", url.Values{
		"login": {login}, "pw": {"Create-Pass-1"}, "first_name": {"Before"}, "last_name": {"Audit"},
	})
	require.Equal(t, http.StatusCreated, code, body)
	userID := idByColumn(t, "users", "login", login)
	t.Cleanup(func() {
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM gk_audit_event WHERE target_type = ? AND target_id = ?`), "users", strconv.Itoa(userID)) //nolint:errcheck // cleanup
		_, _ = db.Exec(database.ConvertPlaceholders(`DELETE FROM users WHERE id = ?`), userID)                                                            //nolint:errcheck // cleanup
	})

	code, body = serveDynamic(t, r, http.MethodPut, fmt.Sprintf("/dynamic/users/%d", userID), url.Values{
		"login": {login}, "pw": {"Update-Pass-2"}, "first_name": {"After"}, "last_name": {"Audit"},
	})
	require.Equal(t, http.StatusOK, code, body)

	var outcome, label, raw string
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`
		SELECT outcome, target_label, details FROM gk_audit_event
		WHERE action = ? AND target_id = ? ORDER BY id DESC LIMIT 1`),
		"admin.users.update", strconv.Itoa(userID)).Scan(&outcome, &label, &raw))
	assert.Equal(t, audit.OutcomeSuccess, outcome)
	assert.Equal(t, login, label)

	var pwHash string
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(`SELECT pw FROM users WHERE id = ?`), userID).Scan(&pwHash))
	assert.NotContains(t, raw, "Update-Pass-2")
	assert.NotContains(t, raw, "Create-Pass-1")
	assert.NotContains(t, raw, pwHash)

	var details struct {
		Changes map[string]map[string]any `json:"changes"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &details))
	assert.Equal(t, map[string]any{"from": "Before", "to": "After"}, details.Changes["first_name"])
	assert.Equal(t, map[string]any{"from": audit.Redacted, "to": audit.Redacted}, details.Changes["pw"])
}
