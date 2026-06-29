package organisation

import (
	"fmt"
	"regexp"
	"strings"
)

// OrgAwareTables is the registry of tables that have an org_id column.
// Queries against these tables are automatically scoped to the active org.
// Tables not in this set pass through unmodified.
var OrgAwareTables = map[string]bool{
	"ticket":                true,
	"queue":                 true,
	"customer_user":         true,
	"gk_custom_field_value": true,
}

// RegisterOrgAwareTable adds a table to the org-scoping registry.
// Plugins can call this for their own tables that have an org_id column.
func RegisterOrgAwareTable(table string) {
	OrgAwareTables[strings.ToLower(table)] = true
}

// ScopeQuery rewrites a SQL query to include org_id filtering.
// Returns the modified query and updated args slice.
//
// For SELECT/UPDATE/DELETE: appends "AND org_id = ?" to the WHERE clause
// (or adds "WHERE org_id = ?" if no WHERE exists).
//
// For INSERT: does NOT modify (caller must include org_id in their INSERT).
//
// If orgID is 0 (single-org mode) or no org-aware tables are found, returns
// the query unmodified.
func ScopeQuery(query string, args []any, orgID int64) (string, []any) {
	if orgID == 0 {
		return query, args
	}

	tables := extractMainTable(query)
	if tables == "" {
		return query, args
	}

	if !OrgAwareTables[strings.ToLower(tables)] {
		return query, args
	}

	upper := strings.ToUpper(strings.TrimSpace(query))

	// Don't scope INSERTs — caller must include org_id.
	if strings.HasPrefix(upper, "INSERT") {
		return query, args
	}

	// Don't scope DDL.
	if strings.HasPrefix(upper, "CREATE") || strings.HasPrefix(upper, "DROP") ||
		strings.HasPrefix(upper, "ALTER") || strings.HasPrefix(upper, "TRUNCATE") {
		return query, args
	}

	// Already has org_id in the query? Don't double-scope.
	if strings.Contains(upper, "ORG_ID") {
		return query, args
	}

	// Determine the table alias (if any) to qualify org_id.
	alias := findTableAlias(query, tables)
	orgCol := "org_id"
	if alias != "" {
		orgCol = alias + ".org_id"
	}

	// Inject org_id filter.
	scopedQuery, newArgs := injectOrgFilter(query, args, orgCol, orgID)
	return scopedQuery, newArgs
}

// extractMainTable extracts the primary table name from a query.
// Returns the first table found after FROM, UPDATE, or DELETE FROM.
func extractMainTable(query string) string {
	normalized := regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(query), " ")
	upper := strings.ToUpper(normalized)

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bFROM\s+` + "`?" + `([a-zA-Z_][a-zA-Z0-9_]*)` + "`?"),
		regexp.MustCompile(`(?i)\bUPDATE\s+` + "`?" + `([a-zA-Z_][a-zA-Z0-9_]*)` + "`?"),
		regexp.MustCompile(`(?i)\bDELETE\s+FROM\s+` + "`?" + `([a-zA-Z_][a-zA-Z0-9_]*)` + "`?"),
	}

	for _, p := range patterns {
		if m := p.FindStringSubmatch(normalized); len(m) > 1 {
			return strings.ToLower(m[1])
		}
	}

	_ = upper
	return ""
}

// findTableAlias finds the alias for a table if one exists.
// e.g., "SELECT * FROM ticket t WHERE ..." returns "t" for table "ticket".
func findTableAlias(query, table string) string {
	// Pattern: FROM table alias  or  FROM `table` alias
	pattern := regexp.MustCompile(`(?i)\b(?:FROM|UPDATE|JOIN)\s+` + "`?" + regexp.QuoteMeta(table) + "`?" + `\s+(?:AS\s+)?([a-zA-Z_][a-zA-Z0-9_]*)`)
	if m := pattern.FindStringSubmatch(query); len(m) > 1 {
		alias := strings.ToUpper(m[1])
		// Exclude SQL keywords that might follow the table name.
		keywords := map[string]bool{
			"SET": true, "WHERE": true, "ON": true, "INNER": true, "LEFT": true,
			"RIGHT": true, "FULL": true, "CROSS": true, "JOIN": true, "ORDER": true,
			"GROUP": true, "HAVING": true, "LIMIT": true, "VALUES": true, "INTO": true,
			"USING": true, "NATURAL": true,
		}
		if keywords[alias] {
			return ""
		}
		return m[1]
	}
	return ""
}

// injectOrgFilter adds "AND org_id = ?" to the WHERE clause.
// If no WHERE clause exists, adds "WHERE org_id = ?".
func injectOrgFilter(query string, args []any, orgCol string, orgID int64) (string, []any) {
	upper := strings.ToUpper(query)

	whereIdx := findWhereIndex(upper)

	if whereIdx >= 0 {
		// Has WHERE — insert "org_id = ? AND " right after "WHERE ".
		// Find the position after "WHERE " in the original query.
		afterWhere := whereIdx + 6 // len("WHERE ")
		// Consume any whitespace.
		for afterWhere < len(query) && query[afterWhere] == ' ' {
			afterWhere++
		}
		scopedQuery := query[:afterWhere] + orgCol + " = ? AND " + query[afterWhere:]
		newArgs := make([]any, 0, len(args)+1)
		newArgs = append(newArgs, orgID)
		newArgs = append(newArgs, args...)
		return scopedQuery, newArgs
	}

	// No WHERE clause. Find where to insert one.
	// Insert before ORDER BY, GROUP BY, HAVING, LIMIT, or at end.
	insertPos := len(query)
	for _, keyword := range []string{" ORDER ", " GROUP ", " HAVING ", " LIMIT ", " FOR "} {
		idx := strings.Index(upper, keyword)
		if idx >= 0 && idx < insertPos {
			insertPos = idx
		}
	}

	scopedQuery := query[:insertPos] + fmt.Sprintf(" WHERE %s = ?", orgCol) + query[insertPos:]
	newArgs := append(args, orgID)
	return scopedQuery, newArgs
}

// findWhereIndex returns the index of the main WHERE keyword, excluding
// WHERE inside subqueries (after opening parens).
func findWhereIndex(upper string) int {
	// Simple approach: find "WHERE" that isn't inside parentheses.
	depth := 0
	for i := 0; i < len(upper)-5; i++ {
		switch upper[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		}
		if depth == 0 && upper[i:i+5] == "WHERE" {
			return i
		}
	}
	return -1
}
