package organisation

import (
	"strings"
	"testing"
)

func TestScopeQuery_SelectWithWhere(t *testing.T) {
	query := "SELECT * FROM gk_org_plugin_access WHERE plugin_name = ? AND group_id = ?"
	args := []any{"p", 2}

	scoped, newArgs := ScopeQuery(query, args, 42)

	if !strings.Contains(scoped, "org_id = ?") {
		t.Errorf("expected org_id filter, got: %s", scoped)
	}
	// org_id arg should be prepended (first in args) since it's injected after WHERE.
	if len(newArgs) != 3 {
		t.Fatalf("expected 3 args, got %d", len(newArgs))
	}
	if newArgs[0] != int64(42) {
		t.Errorf("first arg should be org_id=42, got %v", newArgs[0])
	}
	if newArgs[1] != "p" || newArgs[2] != 2 {
		t.Errorf("original args should follow: %v", newArgs)
	}
}

func TestScopeQuery_SelectWithoutWhere(t *testing.T) {
	query := "SELECT * FROM gk_org_plugin_access ORDER BY create_time DESC"
	args := []any{}

	scoped, newArgs := ScopeQuery(query, args, 42)

	if !strings.Contains(scoped, "WHERE org_id = ?") {
		t.Errorf("expected WHERE org_id, got: %s", scoped)
	}
	if !strings.Contains(scoped, "ORDER BY") {
		t.Error("ORDER BY should be preserved")
	}
	if len(newArgs) != 1 || newArgs[0] != int64(42) {
		t.Errorf("args = %v", newArgs)
	}
}

func TestScopeQuery_SelectWithAlias(t *testing.T) {
	query := "SELECT a.id, a.plugin_name FROM gk_org_plugin_access a WHERE a.group_id = ?"
	args := []any{5}

	scoped, newArgs := ScopeQuery(query, args, 10)

	if !strings.Contains(scoped, "a.org_id = ?") {
		t.Errorf("expected aliased a.org_id, got: %s", scoped)
	}
	if len(newArgs) != 2 {
		t.Fatalf("expected 2 args, got %d", len(newArgs))
	}
}

func TestScopeQuery_UpdateWithWhere(t *testing.T) {
	query := "UPDATE sysconfig_org SET effective_value = ? WHERE name = ?"
	args := []any{"v", "Key"}

	scoped, newArgs := ScopeQuery(query, args, 42)

	if !strings.Contains(scoped, "org_id = ?") {
		t.Errorf("expected org_id filter, got: %s", scoped)
	}
	if len(newArgs) != 3 {
		t.Fatalf("expected 3 args, got %d", len(newArgs))
	}
}

func TestScopeQuery_DeleteWithWhere(t *testing.T) {
	query := "DELETE FROM gk_user_organisation WHERE user_id = ?"
	args := []any{99}

	scoped, newArgs := ScopeQuery(query, args, 42)

	if !strings.Contains(scoped, "org_id = ?") {
		t.Errorf("expected org_id filter, got: %s", scoped)
	}
	if len(newArgs) != 2 {
		t.Fatalf("expected 2 args, got %d", len(newArgs))
	}
}

func TestScopeQuery_InsertNotModified(t *testing.T) {
	query := "INSERT INTO gk_org_plugin_access (org_id, plugin_name, group_id) VALUES (?, ?, ?)"
	args := []any{42, "p", 1}

	scoped, newArgs := ScopeQuery(query, args, 42)

	if scoped != query {
		t.Errorf("INSERT should not be modified, got: %s", scoped)
	}
	if len(newArgs) != 3 {
		t.Errorf("args should not change: %v", newArgs)
	}
}

func TestScopeQuery_NonOrgTable(t *testing.T) {
	query := "SELECT * FROM users WHERE id = ?"
	args := []any{1}

	scoped, newArgs := ScopeQuery(query, args, 42)

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
		"SELECT * FROM customer_user WHERE login = ?",
		"SELECT * FROM gk_custom_field_value WHERE field_id = ?",
	} {
		if scoped, _ := ScopeQuery(query, []any{1}, 42); scoped != query {
			t.Errorf("table without org_id scoped: %s", scoped)
		}
	}
}

func TestScopeQuery_ZeroOrgID(t *testing.T) {
	query := "SELECT * FROM gk_org_plugin_access WHERE id = ?"
	args := []any{1}

	scoped, newArgs := ScopeQuery(query, args, 0)

	if scoped != query {
		t.Error("orgID=0 should not modify query")
	}
	if len(newArgs) != 1 {
		t.Errorf("args should not change: %v", newArgs)
	}
}

func TestScopeQuery_AlreadyHasOrgID(t *testing.T) {
	query := "SELECT * FROM gk_org_plugin_access WHERE org_id = ? AND group_id = ?"
	args := []any{42, 1}

	scoped, newArgs := ScopeQuery(query, args, 42)

	if scoped != query {
		t.Error("query already has org_id — should not double-scope")
	}
	if len(newArgs) != 2 {
		t.Errorf("args should not change: %v", newArgs)
	}
}

func TestScopeQuery_DDLNotModified(t *testing.T) {
	queries := []string{
		"CREATE TABLE sysconfig_org (id INT)",
		"DROP TABLE sysconfig_org",
		"ALTER TABLE sysconfig_org ADD COLUMN x INT",
		"TRUNCATE TABLE sysconfig_org",
	}
	for _, query := range queries {
		t.Run(query[:10], func(t *testing.T) {
			scoped, _ := ScopeQuery(query, nil, 42)
			if scoped != query {
				t.Errorf("DDL should not be modified: %s", scoped)
			}
		})
	}
}

func TestScopeQuery_WithGroupByAndLimit(t *testing.T) {
	query := "SELECT plugin_name, COUNT(*) FROM gk_org_plugin_access GROUP BY plugin_name LIMIT 10"
	args := []any{}

	scoped, newArgs := ScopeQuery(query, args, 42)

	if !strings.Contains(scoped, "WHERE org_id = ?") {
		t.Errorf("expected WHERE org_id, got: %s", scoped)
	}
	if !strings.Contains(scoped, "GROUP BY") {
		t.Error("GROUP BY should be preserved")
	}
	if !strings.Contains(scoped, "LIMIT") {
		t.Error("LIMIT should be preserved")
	}
	if len(newArgs) != 1 {
		t.Errorf("expected 1 arg, got %d", len(newArgs))
	}
}

func TestExtractMainTable(t *testing.T) {
	tests := []struct {
		query string
		want  string
	}{
		{"SELECT * FROM ticket WHERE id = 1", "ticket"},
		{"SELECT * FROM `ticket` WHERE id = 1", "ticket"},
		{"UPDATE ticket SET title = 'x'", "ticket"},
		{"DELETE FROM ticket WHERE id = 1", "ticket"},
		{"INSERT INTO ticket (title) VALUES ('x')", ""}, // extractMainTable doesn't extract INSERT targets
		{"SELECT 1", ""},
	}
	for _, tt := range tests {
		name := tt.query
		if len(name) > 30 {
			name = name[:30]
		}
		t.Run(name, func(t *testing.T) {
			if got := extractMainTable(tt.query); got != tt.want {
				t.Errorf("extractMainTable(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}

func TestFindTableAlias(t *testing.T) {
	tests := []struct {
		query string
		table string
		want  string
	}{
		{"SELECT t.id FROM ticket t WHERE t.id = 1", "ticket", "t"},
		{"SELECT * FROM ticket AS t WHERE t.id = 1", "ticket", "t"},
		{"SELECT * FROM ticket WHERE id = 1", "ticket", ""},
		{"SELECT * FROM ticket SET title = 'x'", "ticket", ""},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := findTableAlias(tt.query, tt.table); got != tt.want {
				t.Errorf("findTableAlias(%q, %q) = %q, want %q", tt.query, tt.table, got, tt.want)
			}
		})
	}
}
