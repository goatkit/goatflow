package database

import (
	"fmt"
	"strings"
)

// sqlKeywords are the leading keywords that mark a string as a SQL query.
// Used by IsSQLQuery, the heuristic the lint rule uses to decide whether a
// fmt.Sprintf format string warrants inspection. Mirrors the implicit pattern
// used throughout the codebase — no new heuristic invented here.
var sqlKeywords = []string{
	"SELECT", "INSERT", "UPDATE", "DELETE", "MERGE",
	"WITH", "CREATE", "ALTER", "DROP",
}

// IsSQLQuery returns true if the string looks like a SQL query — i.e. its
// first non-whitespace token is a SQL DML/DDL keyword. Used by the gk-lint
// sql-sprintf rule to decide whether a fmt.Sprintf format string warrants
// inspection, and safe to call on arbitrary input.
func IsSQLQuery(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}
	upper := strings.ToUpper(trimmed)
	for _, kw := range sqlKeywords {
		if upper == kw || strings.HasPrefix(upper, kw+" ") || strings.HasPrefix(upper, kw+"\t") || strings.HasPrefix(upper, kw+"\n") {
			return true
		}
		// WITH (…) and SELECT/INSERT/UPDATE/DELETE may be followed by a paren
		// in some dialects, but the common case is keyword + whitespace.
	}
	return false
}

// CheckStackedQuery returns an error if the query contains more than one
// statement separated by ';' outside of a string literal — the most common
// SQL injection pattern (e.g. "SELECT ... ; DROP TABLE users").
//
// The scan tracks single-quote state so ';' inside string literals (such as
// 'O”Brien' or a quoted semicolon in a value) does not trigger a false
// positive. A query that ends with a single trailing ';' (a common style)
// is accepted; only an interior ';' that would start a second statement is
// flagged.
func CheckStackedQuery(query string) error {
	inString := false
	statementSeen := false // true once we've passed a complete statement

	for i := 0; i < len(query); i++ {
		c := query[i]

		switch c {
		case '\'':
			// Doubled single quote ('') is an escaped quote inside a string
			// literal — consume both characters and stay in string state.
			if inString && i+1 < len(query) && query[i+1] == '\'' {
				i++
				continue
			}
			inString = !inString

		case ';':
			if inString {
				continue
			}
			// First ';' marks the end of the first statement. A second
			// non-trailing ';' (i.e. one followed by more SQL tokens) is a
			// stacked query — flag it.
			if !statementSeen {
				statementSeen = true
				continue
			}
			// We already saw one statement terminator. Anything after this
			// that isn't whitespace is a second statement.
			tail := strings.TrimSpace(query[i+1:])
			if tail != "" {
				return fmt.Errorf("stacked query detected: multiple statements separated by ';'")
			}
		}
	}
	return nil
}
