package organisation

import (
	"strings"
	"testing"
)

// scope runs ScopeQuery as organisation 42.
func scope(q string, args []any) (string, []any, error) {
	return ScopeQuery(q, args, 42)
}

// orgScoped reports whether every org-aware table reference in scoped is
// filtered by the caller's org and the plugin's own predicate cannot escape
// it: the scoped query must read "WHERE <q>.org_id = ? AND ( <plugin> )".
func orgScoped(t *testing.T, scoped string, qualifiers ...string) {
	t.Helper()
	for _, q := range qualifiers {
		if !strings.Contains(scoped, q+".org_id = ?") {
			t.Errorf("missing %s.org_id filter: %s", q, scoped)
		}
	}
}

// TestScopeQuery_Adversarial: shapes a plugin can use to read or write rows
// of another organisation. Each must either be scoped by the caller's org
// or be refused.
func TestScopeQuery_Adversarial(t *testing.T) {
	t.Run("org_id mention does not disable scoping", func(t *testing.T) {
		scoped, args, err := scope("SELECT org_id, COUNT(*) FROM sysconfig_org WHERE org_id <> 0 GROUP BY org_id", nil)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(scoped, "WHERE sysconfig_org.org_id = ? AND (org_id <> 0) GROUP BY") {
			t.Errorf("predicate not wrapped: %s", scoped)
		}
		if len(args) != 1 || args[0] != int64(42) {
			t.Errorf("args = %v", args)
		}
	})
	t.Run("org_id in a comment does not disable scoping", func(t *testing.T) {
		scoped, _, err := scope("SELECT * /* org_id */ FROM sysconfig_org", nil)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(scoped, "FROM sysconfig_org WHERE sysconfig_org.org_id = ?") {
			t.Errorf("not scoped: %s", scoped)
		}
	})
	t.Run("OR cannot escape the org filter", func(t *testing.T) {
		scoped, args, err := scope("SELECT * FROM gk_org_plugin_access WHERE plugin_name = ? OR 1=1", []any{"p"})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(scoped, "WHERE gk_org_plugin_access.org_id = ? AND (plugin_name = ? OR 1=1)") {
			t.Errorf("predicate not parenthesised: %s", scoped)
		}
		if len(args) != 2 || args[0] != int64(42) || args[1] != "p" {
			t.Errorf("args = %v", args)
		}
	})
	t.Run("join from a non-org table scopes the org table", func(t *testing.T) {
		scoped, args, err := scope("SELECT u.login, o.org_id FROM users u JOIN gk_user_organisation o ON o.user_id = u.id WHERE u.valid_id = ?", []any{1})
		if err != nil {
			t.Fatal(err)
		}
		orgScoped(t, scoped, "o")
		if !strings.Contains(scoped, "WHERE o.org_id = ? AND (u.valid_id = ?)") {
			t.Errorf("unexpected rewrite: %s", scoped)
		}
		if len(args) != 2 || args[0] != int64(42) || args[1] != 1 {
			t.Errorf("args = %v", args)
		}
	})
	t.Run("subquery on an org table is scoped", func(t *testing.T) {
		scoped, args, err := scope("SELECT login FROM users WHERE id IN (SELECT user_id FROM gk_user_organisation WHERE role = ?) AND valid_id = ?", []any{"admin", 1})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(scoped, "(SELECT user_id FROM gk_user_organisation WHERE gk_user_organisation.org_id = ? AND (role = ?)) AND valid_id = ?") {
			t.Errorf("subquery not scoped: %s", scoped)
		}
		if len(args) != 3 || args[0] != int64(42) || args[1] != "admin" || args[2] != 1 {
			t.Errorf("args = %v", args)
		}
	})
	t.Run("scalar subquery without WHERE is scoped", func(t *testing.T) {
		scoped, _, err := scope("SELECT (SELECT COUNT(*) FROM sysconfig_org) AS n", nil)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(scoped, "(SELECT COUNT(*) FROM sysconfig_org WHERE sysconfig_org.org_id = ?) AS n") {
			t.Errorf("subquery not scoped: %s", scoped)
		}
	})
	t.Run("insert for another org is refused", func(t *testing.T) {
		for _, c := range []struct {
			q    string
			args []any
		}{
			{"INSERT INTO gk_org_plugin_access (org_id, plugin_name, group_id) VALUES (?, ?, ?)", []any{7, "p", 1}},
			{"INSERT INTO gk_org_plugin_access (plugin_name, org_id, group_id) VALUES (?, 7, ?)", []any{"p", 1}},
			{"INSERT INTO gk_org_plugin_access (org_id, plugin_name, group_id) VALUES (?, ?, ?), (?, ?, ?)", []any{42, "p", 1, 7, "p", 1}},
			{"INSERT INTO gk_org_plugin_access (plugin_name, group_id) VALUES (?, ?)", []any{"p", 1}},
			{"INSERT INTO gk_org_plugin_access (org_id, plugin_name, group_id) SELECT 7, plugin_name, group_id FROM gk_org_plugin_access", nil},
			{"INSERT INTO gk_org_plugin_access (org_id, plugin_name, group_id) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE org_id = 7", []any{42, "p", 1}}, // sql-converted: plugin-supplied statement under test, never executed
		} {
			if _, _, err := scope(c.q, c.args); err == nil {
				t.Errorf("accepted: %s", c.q)
			}
		}
	})
	t.Run("update cannot move rows to another org", func(t *testing.T) {
		for _, q := range []string{
			"UPDATE sysconfig_org SET org_id = 7 WHERE name = ?",
			"UPDATE sysconfig_org s SET s.org_id = 7 WHERE s.name = ?",
			"UPDATE sysconfig_org SET (org_id, effective_value) = (7, ?) WHERE name = ?",
		} {
			if _, _, err := scope(q, []any{"x", "y"}); err == nil {
				t.Errorf("accepted: %s", q)
			}
		}
	})
	t.Run("alias usage is scoped through the alias", func(t *testing.T) {
		scoped, _, err := scope("SELECT s.name FROM sysconfig_org AS s, gk_org_plugin_access p WHERE s.name = p.plugin_name OR 1=1", nil)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(scoped, "WHERE s.org_id = ? AND p.org_id = ? AND (s.name = p.plugin_name OR 1=1)") {
			t.Errorf("unexpected rewrite: %s", scoped)
		}
	})
	t.Run("union branches are each scoped", func(t *testing.T) {
		scoped, _, err := scope("SELECT name FROM sysconfig_org WHERE name = ? UNION ALL SELECT plugin_name FROM gk_org_plugin_access", []any{"x"})
		if err != nil {
			t.Fatal(err)
		}
		if scoped != "SELECT name FROM sysconfig_org WHERE sysconfig_org.org_id = ? AND (name = ?) UNION ALL SELECT plugin_name FROM gk_org_plugin_access WHERE gk_org_plugin_access.org_id = ?" {
			t.Errorf("unexpected rewrite: %s", scoped)
		}
	})
	t.Run("unscopable shapes are refused", func(t *testing.T) {
		for _, q := range []string{
			"SELECT * FROM (sysconfig_org) WHERE name = ?",
			"SELECT * FROM (sysconfig_org s JOIN users u ON u.id = s.id)",
			"SELECT * FROM sysconfig_org WHERE name = $1", // sql-ok: asserts raw $N is refused
			"SELECT 1; SELECT * FROM sysconfig_org",
			"DROP TABLE sysconfig_org",
			"TABLE sysconfig_org",
			"DELETE sysconfig_org FROM sysconfig_org",
			"SELECT * FROM sysconfig_org WHERE",
		} {
			if _, _, err := scope(q, []any{"x"}); err == nil {
				t.Errorf("accepted: %s", q)
			}
		}
	})
	t.Run("delete and update are wrapped", func(t *testing.T) {
		scoped, args, err := scope("DELETE FROM gk_user_organisation WHERE user_id = ? OR 1=1", []any{9})
		if err != nil {
			t.Fatal(err)
		}
		if scoped != "DELETE FROM gk_user_organisation WHERE gk_user_organisation.org_id = ? AND (user_id = ? OR 1=1)" {
			t.Errorf("unexpected rewrite: %s", scoped)
		}
		if len(args) != 2 || args[0] != int64(42) {
			t.Errorf("args = %v", args)
		}
		scoped, args, err = scope("UPDATE sysconfig_org SET effective_value = ? WHERE name = ? OR 1=1 LIMIT 1", []any{"v", "k"})
		if err != nil {
			t.Fatal(err)
		}
		if scoped != "UPDATE sysconfig_org SET effective_value = ? WHERE sysconfig_org.org_id = ? AND (name = ? OR 1=1) LIMIT 1" {
			t.Errorf("unexpected rewrite: %s", scoped)
		}
		if len(args) != 3 || args[0] != "v" || args[1] != int64(42) || args[2] != "k" {
			t.Errorf("args = %v", args)
		}
	})
}
