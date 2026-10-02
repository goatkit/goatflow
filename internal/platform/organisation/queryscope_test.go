package organisation

import (
	"strings"
	"testing"
)

func mustScope(t *testing.T, query string, args []any, orgID int64) (string, []any) {
	t.Helper()
	scoped, newArgs, err := ScopeQuery(query, args, orgID)
	if err != nil {
		t.Fatalf("ScopeQuery(%q): %v", query, err)
	}
	return scoped, newArgs
}

func TestScopeQuery_SelectWithWhere(t *testing.T) {
	query := "SELECT * FROM gk_org_plugin_access WHERE plugin_name = ? AND group_id = ?"
	args := []any{"p", 2}

	scoped, newArgs := mustScope(t, query, args, 42)

	if scoped != "SELECT * FROM gk_org_plugin_access WHERE gk_org_plugin_access.org_id = ? AND (plugin_name = ? AND group_id = ?)" {
		t.Errorf("unexpected rewrite: %s", scoped)
	}
	if len(newArgs) != 3 || newArgs[0] != int64(42) || newArgs[1] != "p" || newArgs[2] != 2 {
		t.Errorf("args = %v", newArgs)
	}
}

func TestScopeQuery_SelectWithoutWhere(t *testing.T) {
	query := "SELECT * FROM gk_org_plugin_access ORDER BY create_time DESC"
	args := []any{}

	scoped, newArgs := mustScope(t, query, args, 42)

	if scoped != "SELECT * FROM gk_org_plugin_access WHERE gk_org_plugin_access.org_id = ? ORDER BY create_time DESC" {
		t.Errorf("unexpected rewrite: %s", scoped)
	}
	if len(newArgs) != 1 || newArgs[0] != int64(42) {
		t.Errorf("args = %v", newArgs)
	}
}

func TestScopeQuery_SelectWithAlias(t *testing.T) {
	query := "SELECT a.id, a.plugin_name FROM gk_org_plugin_access a WHERE a.group_id = ?"
	args := []any{5}

	scoped, newArgs := mustScope(t, query, args, 10)

	if !strings.Contains(scoped, "WHERE a.org_id = ? AND (a.group_id = ?)") {
		t.Errorf("expected aliased a.org_id, got: %s", scoped)
	}
	if len(newArgs) != 2 || newArgs[0] != int64(10) {
		t.Errorf("args = %v", newArgs)
	}
}

func TestScopeQuery_UpdateWithWhere(t *testing.T) {
	query := "UPDATE sysconfig_org SET effective_value = ? WHERE name = ?"
	args := []any{"v", "Key"}

	scoped, newArgs := mustScope(t, query, args, 42)

	if scoped != "UPDATE sysconfig_org SET effective_value = ? WHERE sysconfig_org.org_id = ? AND (name = ?)" {
		t.Errorf("unexpected rewrite: %s", scoped)
	}
	// The org id lands at the placeholder position of the WHERE, after SET.
	if len(newArgs) != 3 || newArgs[0] != "v" || newArgs[1] != int64(42) || newArgs[2] != "Key" {
		t.Errorf("args = %v", newArgs)
	}
}

func TestScopeQuery_UpdateWithoutWhere(t *testing.T) {
	scoped, newArgs := mustScope(t, "UPDATE sysconfig_org SET effective_value = ?", []any{"v"}, 42)
	if scoped != "UPDATE sysconfig_org SET effective_value = ? WHERE sysconfig_org.org_id = ?" {
		t.Errorf("unexpected rewrite: %s", scoped)
	}
	if len(newArgs) != 2 || newArgs[0] != "v" || newArgs[1] != int64(42) {
		t.Errorf("args = %v", newArgs)
	}
}

func TestScopeQuery_DeleteWithWhere(t *testing.T) {
	query := "DELETE FROM gk_user_organisation WHERE user_id = ?"
	args := []any{99}

	scoped, newArgs := mustScope(t, query, args, 42)

	if scoped != "DELETE FROM gk_user_organisation WHERE gk_user_organisation.org_id = ? AND (user_id = ?)" {
		t.Errorf("unexpected rewrite: %s", scoped)
	}
	if len(newArgs) != 2 || newArgs[0] != int64(42) || newArgs[1] != 99 {
		t.Errorf("args = %v", newArgs)
	}
}

func TestScopeQuery_InsertOwnOrg(t *testing.T) {
	for _, c := range []struct {
		q    string
		args []any
	}{
		{"INSERT INTO gk_org_plugin_access (org_id, plugin_name, group_id) VALUES (?, ?, ?)", []any{42, "p", 1}},
		{"INSERT INTO gk_org_plugin_access (plugin_name, org_id, group_id) VALUES (?, ?, ?)", []any{"p", int64(42), 1}},
		{"INSERT INTO gk_org_plugin_access (plugin_name, org_id, group_id) VALUES (?, 42, ?)", []any{"p", 1}},
		{"INSERT INTO gk_org_plugin_access (org_id, plugin_name, group_id) VALUES (?, ?, ?), (?, ?, ?)", []any{42, "p", 1, "42", "q", 2}},
		{"INSERT INTO sysconfig_org (org_id, name, effective_value) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE effective_value = VALUES(effective_value)", []any{float64(42), "k", "v"}}, // sql-converted: plugin-supplied statement under test, never executed
		{"INSERT INTO `sysconfig_org` (`org_id`, `name`) VALUES (?, ?) RETURNING id", []any{42, "k"}},                                                                                  // sql-converted: plugin-supplied statement under test, never executed
	} {
		scoped, newArgs := mustScope(t, c.q, c.args, 42)
		if scoped != c.q {
			t.Errorf("INSERT should not be modified, got: %s", scoped)
		}
		if len(newArgs) != len(c.args) {
			t.Errorf("args should not change: %v", newArgs)
		}
	}
}

func TestScopeQuery_NonOrgTable(t *testing.T) {
	query := "SELECT * FROM users WHERE id = ?"
	args := []any{1}

	scoped, newArgs := mustScope(t, query, args, 42)

	if scoped != query {
		t.Errorf("non-org table should not be modified, got: %s", scoped)
	}
	if len(newArgs) != 1 {
		t.Errorf("args should not change: %v", newArgs)
	}
}

// TestScopeQuery_CoreTablesWithoutOrgColumn: ticket, queue, customer_user and
// gk_custom_field_value have no org_id column in the schema, so scoping them
// would turn every plugin query on them into an SQL error.
func TestScopeQuery_CoreTablesWithoutOrgColumn(t *testing.T) {
	for _, query := range []string{
		"SELECT * FROM ticket WHERE queue_id = ?",
		"UPDATE queue SET name = ? WHERE id = ?",
		"DELETE FROM customer_user WHERE id = ?",
		"SELECT * FROM gk_custom_field_value WHERE entity_id = ?",
		"INSERT INTO ticket (title) VALUES (?)",
		"DROP TABLE ticket",
		"SELECT 1; SELECT ?",
	} {
		args := make([]any, strings.Count(query, "?"))
		if scoped, _ := mustScope(t, query, args, 42); scoped != query {
			t.Errorf("table without org_id scoped: %s", scoped)
		}
	}
}

func TestScopeQuery_ZeroOrgID(t *testing.T) {
	query := "SELECT * FROM gk_org_plugin_access WHERE id = ?"
	args := []any{1}

	scoped, newArgs := mustScope(t, query, args, 0)

	if scoped != query {
		t.Error("orgID=0 should not modify query")
	}
	if len(newArgs) != 1 {
		t.Errorf("args should not change: %v", newArgs)
	}
}

// TestScopeQuery_PluginOrgFilterIsWrapped: a plugin's own org_id predicate
// is kept but never trusted; the sandbox filter is still added.
func TestScopeQuery_PluginOrgFilterIsWrapped(t *testing.T) {
	query := "SELECT * FROM gk_org_plugin_access WHERE org_id = ? AND group_id = ?"
	args := []any{42, 1}

	scoped, newArgs := mustScope(t, query, args, 42)

	if scoped != "SELECT * FROM gk_org_plugin_access WHERE gk_org_plugin_access.org_id = ? AND (org_id = ? AND group_id = ?)" {
		t.Errorf("unexpected rewrite: %s", scoped)
	}
	if len(newArgs) != 3 || newArgs[0] != int64(42) || newArgs[1] != 42 || newArgs[2] != 1 {
		t.Errorf("args = %v", newArgs)
	}
}

func TestScopeQuery_WithGroupByAndLimit(t *testing.T) {
	query := "SELECT plugin_name, COUNT(*) FROM gk_org_plugin_access GROUP BY plugin_name LIMIT 10"
	args := []any{}

	scoped, newArgs := mustScope(t, query, args, 42)

	if scoped != "SELECT plugin_name, COUNT(*) FROM gk_org_plugin_access WHERE gk_org_plugin_access.org_id = ? GROUP BY plugin_name LIMIT 10" {
		t.Errorf("unexpected rewrite: %s", scoped)
	}
	if len(newArgs) != 1 {
		t.Errorf("expected 1 arg, got %d", len(newArgs))
	}
}

func TestScopeQuery_AllowedShapes(t *testing.T) {
	for _, c := range []struct{ q, want string }{
		{
			"WITH c AS (SELECT user_id FROM gk_user_organisation) SELECT * FROM users WHERE id IN (SELECT user_id FROM c)",
			"WITH c AS (SELECT user_id FROM gk_user_organisation WHERE gk_user_organisation.org_id = ?) SELECT * FROM users WHERE id IN (SELECT user_id FROM c)",
		},
		{
			"@analytics:SELECT name FROM sysconfig_org WHERE name LIKE ?",
			"@analytics:SELECT name FROM sysconfig_org WHERE sysconfig_org.org_id = ? AND (name LIKE ?)",
		},
		{
			"SELECT * FROM otrs.sysconfig_org s LEFT JOIN gk_identity_provider_org AS i ON i.org_id = s.org_id WHERE s.name = ? FOR UPDATE",
			"SELECT * FROM otrs.sysconfig_org s LEFT JOIN gk_identity_provider_org AS i ON i.org_id = s.org_id WHERE s.org_id = ? AND i.org_id = ? AND (s.name = ?) FOR UPDATE",
		},
		{
			"select\n  name -- org_id\nfrom sysconfig_org\nwhere name = ?\norder by name",
			"select\n  name -- org_id\nfrom sysconfig_org\nwhere sysconfig_org.org_id = ? AND (name = ?)\norder by name",
		},
		{
			"DELETE FROM gk_user_organisation",
			"DELETE FROM gk_user_organisation WHERE gk_user_organisation.org_id = ?",
		},
		{
			"SELECT COUNT(*) FROM sysconfig_org;",
			"SELECT COUNT(*) FROM sysconfig_org WHERE sysconfig_org.org_id = ?;",
		},
	} {
		args := make([]any, strings.Count(c.q, "?"))
		if scoped, _ := mustScope(t, c.q, args, 42); scoped != c.want {
			t.Errorf("ScopeQuery(%q)\n got %q\nwant %q", c.q, scoped, c.want)
		}
	}
}
