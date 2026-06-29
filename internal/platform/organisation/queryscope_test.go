package organisation

import (
	"strings"
	"testing"
)

func TestScopeQuery_SelectWithWhere(t *testing.T) {
	query := "SELECT * FROM ticket WHERE queue_id = ? AND state_id = ?"
	args := []any{1, 2}

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
	if newArgs[1] != 1 || newArgs[2] != 2 {
		t.Errorf("original args should follow: %v", newArgs)
	}
}

func TestScopeQuery_SelectWithoutWhere(t *testing.T) {
	query := "SELECT * FROM ticket ORDER BY create_time DESC"
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
	query := "SELECT t.id, t.title FROM ticket t WHERE t.queue_id = ?"
	args := []any{5}

	scoped, newArgs := ScopeQuery(query, args, 10)

	if !strings.Contains(scoped, "t.org_id = ?") {
		t.Errorf("expected aliased t.org_id, got: %s", scoped)
	}
	if len(newArgs) != 2 {
		t.Fatalf("expected 2 args, got %d", len(newArgs))
	}
}

func TestScopeQuery_UpdateWithWhere(t *testing.T) {
	query := "UPDATE ticket SET title = ? WHERE id = ?"
	args := []any{"new title", 99}

	scoped, newArgs := ScopeQuery(query, args, 42)

	if !strings.Contains(scoped, "org_id = ?") {
		t.Errorf("expected org_id filter, got: %s", scoped)
	}
	if len(newArgs) != 3 {
		t.Fatalf("expected 3 args, got %d", len(newArgs))
	}
}

func TestScopeQuery_DeleteWithWhere(t *testing.T) {
	query := "DELETE FROM ticket WHERE id = ?"
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
	query := "INSERT INTO ticket (title, queue_id, org_id) VALUES (?, ?, ?)"
	args := []any{"test", 1, 42}

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

func TestScopeQuery_ZeroOrgID(t *testing.T) {
	query := "SELECT * FROM ticket WHERE id = ?"
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
	query := "SELECT * FROM ticket WHERE org_id = ? AND queue_id = ?"
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
		"CREATE TABLE ticket (id INT)",
		"DROP TABLE ticket",
		"ALTER TABLE ticket ADD COLUMN x INT",
		"TRUNCATE TABLE ticket",
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
	query := "SELECT queue_id, COUNT(*) FROM ticket GROUP BY queue_id LIMIT 10"
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

func TestScopeQuery_CustomerUserTable(t *testing.T) {
	query := "SELECT * FROM customer_user WHERE login = ?"
	args := []any{"alice@example.com"}

	scoped, newArgs := ScopeQuery(query, args, 99)

	if !strings.Contains(scoped, "org_id = ?") {
		t.Errorf("customer_user should be scoped, got: %s", scoped)
	}
	if len(newArgs) != 2 {
		t.Fatalf("expected 2 args, got %d", len(newArgs))
	}
}

func TestScopeQuery_CustomFieldValueTable(t *testing.T) {
	query := "SELECT * FROM gk_custom_field_value WHERE field_id = ?"
	args := []any{5}

	scoped, newArgs := ScopeQuery(query, args, 7)

	if !strings.Contains(scoped, "org_id = ?") {
		t.Errorf("gk_custom_field_value should be scoped, got: %s", scoped)
	}
	if len(newArgs) != 2 {
		t.Fatalf("expected 2 args, got %d", len(newArgs))
	}
}

func TestRegisterOrgAwareTable(t *testing.T) {
	tableName := "test_custom_table_xyz"
	// Should not be org-aware before registration.
	query := "SELECT * FROM test_custom_table_xyz WHERE id = ?"
	scoped, _ := ScopeQuery(query, []any{1}, 42)
	if scoped != query {
		t.Error("unregistered table should not be scoped")
	}

	// Register and verify.
	RegisterOrgAwareTable(tableName)
	scoped, _ = ScopeQuery(query, []any{1}, 42)
	if !strings.Contains(scoped, "org_id = ?") {
		t.Error("registered table should now be scoped")
	}

	// Clean up.
	delete(OrgAwareTables, tableName)
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
		{"INSERT INTO ticket (title) VALUES ('x')", ""},  // extractMainTable doesn't extract INSERT targets
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
