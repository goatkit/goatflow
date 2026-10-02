package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Schema rules. The migrations are the only definition of the schema, and
// GoatFlow must run on both drivers, so a table or column counts as existing
// only when the MySQL and the PostgreSQL migrations both create it.
//
//   - sql-unknown-table: an SQL literal reads or writes a table no migration
//     creates on both drivers (a legacy or misspelt name, or a table that only
//     a test creates).
//   - sql-unknown-column: an INSERT column list or UPDATE ... SET names a
//     column that table does not have on both drivers.
//
// SELECT column lists are not checked: aliases and joins make them ambiguous
// without a real SQL parser.

// sqlSchema maps each table to its set of columns.
type sqlSchema map[string]map[string]bool

// sqlSchemaDrivers are the migration directories under migrations/.
var sqlSchemaDrivers = []string{"mysql", "postgres"}

// sqlSystemTables exist without a migration: golang-migrate's bookkeeping and
// the drivers' catalogues.
var sqlSystemTables = map[string]bool{"schema_migrations": true, "dual": true}

var (
	reSQLComment     = regexp.MustCompile(`(?m)--[^\n]*$`)
	reCreateTable    = regexp.MustCompile("(?i)\\bCREATE\\s+TABLE\\s+(?:IF\\s+NOT\\s+EXISTS\\s+)?[`\"]?(\\w+)[`\"]?\\s*\\(")
	reAlterTable     = regexp.MustCompile("(?i)\\bALTER\\s+TABLE\\s+(?:IF\\s+EXISTS\\s+)?(?:ONLY\\s+)?[`\"]?(\\w+)[`\"]?\\s")
	reAlterAdd       = regexp.MustCompile("(?i)\\bADD\\s+(?:COLUMN\\s+)?(?:IF\\s+NOT\\s+EXISTS\\s+)?[`\"]?(\\w+)")
	reAlterDrop      = regexp.MustCompile("(?i)\\bDROP\\s+COLUMN\\s+(?:IF\\s+EXISTS\\s+)?[`\"]?(\\w+)")
	reAlterChange    = regexp.MustCompile("(?i)\\bCHANGE\\s+(?:COLUMN\\s+)?[`\"]?(\\w+)[`\"]?\\s+[`\"]?(\\w+)")
	reAlterRename    = regexp.MustCompile("(?i)\\bRENAME\\s+COLUMN\\s+[`\"]?(\\w+)[`\"]?\\s+TO\\s+[`\"]?(\\w+)")
	reDropTable      = regexp.MustCompile("(?i)\\bDROP\\s+TABLE\\s+(?:IF\\s+EXISTS\\s+)?[`\"]?(\\w+)")
	reTableRef       = regexp.MustCompile(`(?i)\b(FROM|JOIN|INTO|UPDATE|TABLE)\s+(?:IF\s+(?:NOT\s+)?EXISTS\s+)?(?:ONLY\s+)?([A-Za-z_][\w.]*)`)
	reNotTableBefore = regexp.MustCompile(`(?i)\b(KEY|DO|ON|FOR|DISTINCT)\s*$`)
	reWordBefore     = regexp.MustCompile(`(\w+)\s*$`)
	reCTEName        = regexp.MustCompile(`(?i)(?:\bWITH(?:\s+RECURSIVE)?|,)\s*(\w+)\s+AS\s*\(`)
	reSetClauseEnd   = regexp.MustCompile(`(?i)^\s*(WHERE|ORDER|LIMIT|RETURNING|FROM)\b`)
	reIdent          = regexp.MustCompile(`^(?:\w+\.)?(\w+)$`)
	// reSQLStatement matches text with the shape of a full SQL statement, so
	// prose such as "update failed" or "Delete search from history" is skipped.
	reSQLStatement = regexp.MustCompile(`(?is)^\s*(?:SELECT\b.*\bFROM\s+\w|INSERT\s+(?:IGNORE\s+)?INTO\s+\w+\s*(?:\(|VALUES\b|SELECT\b)|REPLACE\s+INTO\s+\w+\s*\(|UPDATE\s+\w+(?:\s+\w+)?\s+SET\s|DELETE\s+FROM\s+\w+(?:\s|;|$)|(?:CREATE|ALTER|DROP|TRUNCATE)\s+(?:TEMPORARY\s+)?TABLE\s|WITH\s+(?:RECURSIVE\s+)?\w+\s+AS\s*\()`)
	// reSQLFragment matches an upper-case FROM/JOIN clause a query is built from.
	reSQLFragment = regexp.MustCompile(`^\s*(?:FROM|(?:LEFT |RIGHT |INNER )?JOIN)\s+[a-z_]\w*(?:\s|$)`)
	reFirstWord   = regexp.MustCompile(`^\s*(\w+)`)
)

// nonColumnDefinitions start a CREATE TABLE item that is not a column.
var nonColumnDefinitions = map[string]bool{
	"PRIMARY": true, "KEY": true, "UNIQUE": true, "INDEX": true, "CONSTRAINT": true,
	"FOREIGN": true, "CHECK": true, "FULLTEXT": true, "SPATIAL": true, "EXCLUDE": true,
}

// alterAddNonColumns follow ADD when it adds something other than a column.
var alterAddNonColumns = map[string]bool{
	"CONSTRAINT": true, "INDEX": true, "KEY": true, "PRIMARY": true, "UNIQUE": true,
	"FOREIGN": true, "FULLTEXT": true, "SPATIAL": true, "CHECK": true, "PARTITION": true,
}

// sqlFunctionsWithFrom take FROM as an argument separator, not a table clause.
var sqlFunctionsWithFrom = map[string]bool{
	"EXTRACT": true, "TRIM": true, "SUBSTRING": true, "SUBSTR": true, "POSITION": true, "OVERLAY": true,
}

// loadSQLSchema builds the schema both drivers share from migrationsDir.
func loadSQLSchema(migrationsDir string) (sqlSchema, error) {
	var shared sqlSchema
	for _, driver := range sqlSchemaDrivers {
		files, err := filepath.Glob(filepath.Join(migrationsDir, driver, "*.up.sql"))
		if err != nil {
			return nil, err
		}
		if len(files) == 0 {
			return nil, fmt.Errorf("no migrations in %s", filepath.Join(migrationsDir, driver))
		}
		sort.Strings(files)
		schema := make(sqlSchema)
		for _, f := range files {
			b, err := os.ReadFile(f) // #nosec G304 -- migration file globbed from the repo's own migrations directory
			if err != nil {
				return nil, err
			}
			applyMigration(schema, string(b))
		}
		if shared == nil {
			shared = schema
			continue
		}
		for table, cols := range shared {
			other, ok := schema[table]
			if !ok {
				delete(shared, table)
				continue
			}
			for col := range cols {
				if !other[col] {
					delete(cols, col)
				}
			}
		}
	}
	return shared, nil
}

// applyMigration applies the table and column changes of one migration file.
func applyMigration(schema sqlSchema, sql string) {
	sql = reSQLComment.ReplaceAllString(sql, "")
	for _, m := range reCreateTable.FindAllStringSubmatchIndex(sql, -1) {
		table := strings.ToLower(sql[m[2]:m[3]])
		body := balancedParens(sql, m[1]-1)
		cols := schema[table]
		if cols == nil {
			cols = make(map[string]bool)
			schema[table] = cols
		}
		for _, item := range splitTopLevel(body, ',') {
			fields := strings.Fields(item)
			if len(fields) == 0 {
				continue
			}
			name := strings.Trim(fields[0], "`\"")
			if nonColumnDefinitions[strings.ToUpper(name)] {
				continue
			}
			cols[strings.ToLower(name)] = true
		}
	}
	for _, m := range reAlterTable.FindAllStringSubmatchIndex(sql, -1) {
		table := strings.ToLower(sql[m[2]:m[3]])
		cols := schema[table]
		if cols == nil {
			continue
		}
		stmt := sql[m[1]:]
		if end := strings.IndexByte(stmt, ';'); end >= 0 {
			stmt = stmt[:end]
		}
		for _, a := range reAlterAdd.FindAllStringSubmatch(stmt, -1) {
			if !alterAddNonColumns[strings.ToUpper(a[1])] {
				cols[strings.ToLower(a[1])] = true
			}
		}
		for _, d := range reAlterDrop.FindAllStringSubmatch(stmt, -1) {
			delete(cols, strings.ToLower(d[1]))
		}
		for _, c := range reAlterChange.FindAllStringSubmatch(stmt, -1) {
			delete(cols, strings.ToLower(c[1]))
			cols[strings.ToLower(c[2])] = true
		}
		for _, r := range reAlterRename.FindAllStringSubmatch(stmt, -1) {
			delete(cols, strings.ToLower(r[1]))
			cols[strings.ToLower(r[2])] = true
		}
	}
	for _, m := range reDropTable.FindAllStringSubmatch(sql, -1) {
		delete(schema, strings.ToLower(m[1]))
	}
}

// balancedParens returns the text inside the parenthesis opening at open,
// skipping quoted strings.
func balancedParens(s string, open int) string {
	depth := 0
	var quote byte
	for i := open; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"' || c == '`':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				return s[open+1 : i]
			}
		}
	}
	return s[open+1:]
}

// splitTopLevel splits s on sep outside parentheses and quotes.
func splitTopLevel(s string, sep byte) []string {
	var parts []string
	depth, start := 0, 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"' || c == '`':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			depth--
		case c == sep && depth == 0:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}

// sqlSchemaFinding is one schema rule violation within an SQL literal.
type sqlSchemaFinding struct {
	kind, detail string
}

// isSQLStatementText reports whether code is an SQL statement or a FROM/JOIN
// fragment rather than prose. The leading keyword must be all upper or all
// lower case: sentences start with a capital letter.
func isSQLStatementText(code string) bool {
	if reSQLFragment.MatchString(code) {
		return true
	}
	m := reFirstWord.FindStringSubmatch(code)
	if m == nil || (m[1] != strings.ToUpper(m[1]) && m[1] != strings.ToLower(m[1])) {
		return false
	}
	return reSQLStatement.MatchString(code)
}

// checkSQLSchema returns the schema rule violations of one SQL statement.
func checkSQLSchema(schema sqlSchema, raw string) []sqlSchemaFinding {
	code := strings.NewReplacer("`", "", `"`, "").Replace(withoutStringLiterals(raw))
	if !isSQLStatementText(code) {
		return nil
	}
	ctes := make(map[string]bool)
	for _, m := range reCTEName.FindAllStringSubmatch(code, -1) {
		ctes[strings.ToLower(m[1])] = true
	}
	var findings []sqlSchemaFinding
	reported := make(map[string]bool)
	for _, m := range reTableRef.FindAllStringSubmatchIndex(code, -1) {
		keyword := strings.ToUpper(code[m[2]:m[3]])
		name := strings.ToLower(code[m[4]:m[5]])
		before := code[:m[0]]
		if keyword != "TABLE" && reNotTableBefore.MatchString(before) {
			continue
		}
		if keyword == "FROM" && insideFunctionWithFrom(before) {
			continue
		}
		if strings.Contains(name, ".") || ctes[name] || sqlSystemTables[name] ||
			strings.HasPrefix(name, "pg_") || name == "select" || name == "values" {
			continue
		}
		cols, ok := schema[name]
		if !ok {
			if !reported[name] {
				reported[name] = true
				findings = append(findings, sqlSchemaFinding{"sql-unknown-table", name})
			}
			continue
		}
		switch keyword {
		case "INTO":
			if strings.HasSuffix(strings.ToUpper(strings.TrimSpace(before)), "INSERT") ||
				strings.HasSuffix(strings.ToUpper(strings.TrimSpace(before)), "IGNORE") {
				rest := strings.TrimSpace(code[m[1]:])
				if strings.HasPrefix(rest, "(") {
					for _, item := range splitTopLevel(balancedParens(rest, 0), ',') {
						findings = appendUnknownColumn(findings, name, cols, item)
					}
				}
			}
		case "UPDATE":
			rest := strings.TrimSpace(code[m[1]:])
			fields := strings.Fields(rest)
			// Skip an optional alias before SET.
			if len(fields) > 1 && !strings.EqualFold(fields[0], "SET") && strings.EqualFold(fields[1], "SET") {
				rest = strings.TrimSpace(rest[strings.Index(rest, fields[1]):])
			}
			if len(rest) < 3 || !strings.EqualFold(rest[:3], "SET") {
				continue
			}
			for _, assignment := range splitTopLevel(rest[3:], ',') {
				if reSetClauseEnd.MatchString(assignment) {
					break
				}
				lhs, _, found := strings.Cut(assignment, "=")
				if !found {
					break
				}
				findings = appendUnknownColumn(findings, name, cols, lhs)
				if tail := strings.ToUpper(assignment); strings.Contains(tail, " WHERE ") {
					break
				}
			}
		}
	}
	return findings
}

func appendUnknownColumn(findings []sqlSchemaFinding, table string, cols map[string]bool, item string) []sqlSchemaFinding {
	m := reIdent.FindStringSubmatch(strings.TrimSpace(item))
	if m == nil {
		return findings
	}
	if col := strings.ToLower(m[1]); !cols[col] {
		findings = append(findings, sqlSchemaFinding{"sql-unknown-column", table + "." + col})
	}
	return findings
}

// insideFunctionWithFrom reports whether the text before a FROM leaves it
// inside EXTRACT(...), TRIM(...) or a similar function call.
func insideFunctionWithFrom(before string) bool {
	depth := 0
	for i := len(before) - 1; i >= 0; i-- {
		switch before[i] {
		case ')':
			depth++
		case '(':
			if depth == 0 {
				m := reWordBefore.FindStringSubmatch(before[:i])
				return m != nil && sqlFunctionsWithFrom[strings.ToUpper(m[1])]
			}
			depth--
		}
	}
	return false
}
