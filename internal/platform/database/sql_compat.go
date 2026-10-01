package database

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// GetDBDriver returns the current database driver.
func GetDBDriver() string {
	// In test mode, prefer TEST_ prefixed environment variables
	driver := os.Getenv("TEST_DB_DRIVER")
	if driver == "" {
		driver = os.Getenv("DB_DRIVER")
	}
	if driver == "" {
		driver = "mysql"
	}
	return strings.ToLower(driver)
}

// IsMySQL returns true if using MySQL/MariaDB.
func IsMySQL() bool {
	driver := GetDBDriver()
	return driver == "mysql" || driver == "mariadb"
}

// IsPostgreSQL returns true if using PostgreSQL.
func IsPostgreSQL() bool {
	return GetDBDriver() == "postgres"
}

// TicketTypeColumn returns the ticket type column name for the active driver.
func TicketTypeColumn() string {
	return "type_id"
}

// QualifiedTicketTypeColumn returns the column name prefixed with the provided alias.
func QualifiedTicketTypeColumn(alias string) string {
	col := TicketTypeColumn()
	if alias == "" {
		return col
	}
	return fmt.Sprintf("%s.%s", alias, col)
}

// ConvertPlaceholders converts SQL placeholders to the format required by the current database.
// This is the ONLY function that should be used for placeholder conversion in the codebase.
// Do NOT use qb.Rebind() directly - always go through this function.
//
// IMPORTANT: Only ? placeholders are allowed. Using $N placeholders will panic.
// - For PostgreSQL: ? → $1, $2, ...
// - For MySQL: ? passed through as-is
//
// Example:
//
//	query := database.ConvertPlaceholders("SELECT * FROM users WHERE id = ? AND name = ?")
//	rows, err := db.Query(query, id, name)
func ConvertPlaceholders(query string) string {
	// Reject $N placeholders - all queries must use ? for portability
	if regexp.MustCompile(`\$\d+`).MatchString(query) {
		panic(fmt.Sprintf("ConvertPlaceholders: $N placeholders are not allowed. Use ? placeholders instead.\nQuery: %s", query))
	}

	// Reject stacked queries (e.g. "SELECT ...; DROP TABLE users"). This is the
	// single runtime injection guard for the whole system — every query through
	// the DB layer passes through here, whether from core code, repositories, or
	// plugin HostAPI. No call site needs changing.
	if err := CheckStackedQuery(query); err != nil {
		panic(fmt.Sprintf("ConvertPlaceholders: %v\nQuery: %s", err, query))
	}

	if IsMySQL() {
		// ? placeholders work directly for MySQL
		// No conversion needed
	} else {
		// PostgreSQL uses $1, $2, etc.
		if strings.Contains(query, "?") {
			// Convert ? to $1, $2, etc.
			result := strings.Builder{}
			paramNum := 1
			for _, c := range query {
				if c == '?' {
					result.WriteString(fmt.Sprintf("$%d", paramNum))
					paramNum++
				} else {
					result.WriteRune(c)
				}
			}
			query = result.String()
		}
	}

	// Convert ILIKE to LIKE for MySQL (MySQL is case-insensitive by default with utf8_general_ci)
	if IsMySQL() {
		query = strings.ReplaceAll(query, " ILIKE ", " LIKE ")
		query = strings.ReplaceAll(query, " ilike ", " LIKE ")
	}

	// Portable SQL function rewriting — allows queries written in MySQL dialect
	// to run on PostgreSQL (and vice versa) without manual branching.
	if IsPostgreSQL() {
		query = rewriteForPostgreSQL(query)
	} else {
		query = rewriteForMySQL(query)
	}

	return query
}

// rewriteForPostgreSQL converts MySQL-specific SQL to PostgreSQL equivalents.
// It runs after ? has become $N, so interval amounts may be placeholders.
func rewriteForPostgreSQL(query string) string {
	// DATE_SUB(expr, INTERVAL n UNIT) → (expr - INTERVAL 'n unit')
	// DATE_SUB(expr, INTERVAL ? UNIT) → (expr - ($N * INTERVAL '1 unit'))
	query = rewriteInterval(reDateSub, "-", query)
	// DATE_ADD(expr, INTERVAL n UNIT) → (expr + INTERVAL 'n unit')
	query = rewriteInterval(reDateAdd, "+", query)

	// UNIX_TIMESTAMP(expr) → EXTRACT(EPOCH FROM expr)::bigint
	query = reUnixTSExpr.ReplaceAllString(query, "EXTRACT(EPOCH FROM $1)::bigint")

	// UNIX_TIMESTAMP() → EXTRACT(EPOCH FROM NOW())::bigint
	query = reUnixTS.ReplaceAllString(query, "EXTRACT(EPOCH FROM NOW())::bigint")

	// CURDATE() → CURRENT_DATE
	query = reCurdate.ReplaceAllString(query, "CURRENT_DATE")

	// SET FOREIGN_KEY_CHECKS = 0|1 → SET session_replication_role = replica|DEFAULT.
	// Used by test fixtures that load or clear rows out of FK order; the
	// PostgreSQL form needs a superuser, which the test database user is.
	if m := reFKChecks.FindStringSubmatch(query); m != nil {
		if m[1] == "0" {
			return "SET session_replication_role = replica"
		}
		return "SET session_replication_role = DEFAULT"
	}

	// FROM_UNIXTIME(expr) → to_timestamp(expr)
	query = reFromUnixtime.ReplaceAllString(query, "to_timestamp(")

	// UUID() → gen_random_uuid()::text (built in since PostgreSQL 13)
	query = reUUID.ReplaceAllString(query, "gen_random_uuid()::text")

	// INSERT IGNORE INTO … → INSERT INTO … ON CONFLICT DO NOTHING. The
	// target-less form skips a row that violates any unique constraint,
	// which is what INSERT IGNORE does, and works on tables without an id.
	if reInsertIgnore.MatchString(query) {
		query = reInsertIgnore.ReplaceAllString(query, "${1}INSERT INTO")
		if !reOnConflict.MatchString(query) {
			query = appendBeforeReturning(query, " ON CONFLICT DO NOTHING")
		}
	}

	// `identifier` → "identifier"
	return backticksToDoubleQuotes(query)
}

func rewriteInterval(re *regexp.Regexp, op, query string) string {
	return re.ReplaceAllStringFunc(query, func(match string) string {
		parts := re.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}
		unit := strings.ToLower(parts[3])
		if strings.HasPrefix(parts[2], "$") {
			return fmt.Sprintf("(%s %s (%s * INTERVAL '1 %s'))", parts[1], op, parts[2], unit)
		}
		return fmt.Sprintf("(%s %s INTERVAL '%s %s')", parts[1], op, parts[2], unit)
	})
}

// appendBeforeReturning adds clause at the end of an INSERT, ahead of any
// RETURNING clause, ignoring a trailing semicolon.
func appendBeforeReturning(query, clause string) string {
	trimmed := strings.TrimRight(query, " \t\r\n;")
	if loc := reReturning.FindStringIndex(trimmed); loc != nil {
		return trimmed[:loc[0]] + clause + trimmed[loc[0]:]
	}
	return trimmed + clause
}

// backticksToDoubleQuotes turns MySQL `quoted` identifiers into standard
// "quoted" ones, leaving backticks inside single-quoted literals alone.
func backticksToDoubleQuotes(query string) string {
	if !strings.Contains(query, "`") {
		return query
	}
	b := []byte(query)
	inString := false
	for i, c := range b {
		switch {
		case c == '\'':
			inString = !inString
		case c == '`' && !inString:
			b[i] = '"'
		}
	}
	return string(b)
}

// rewriteForMySQL converts PostgreSQL-specific SQL functions to MySQL equivalents.
func rewriteForMySQL(query string) string {
	// CURRENT_DATE (without parens) is valid MySQL, no rewrite needed.

	// EXTRACT(EPOCH FROM expr)::bigint → UNIX_TIMESTAMP(expr)
	query = reExtractEpoch.ReplaceAllString(query, "UNIX_TIMESTAMP($1)")

	return query
}

// Compiled regexes for SQL function rewriting (case-insensitive).
var (
	reDateSub      = regexp.MustCompile(`(?i)DATE_SUB\(\s*([^,]+?)\s*,\s*INTERVAL\s+(\d+|\$\d+)\s+(\w+)\s*\)`)
	reDateAdd      = regexp.MustCompile(`(?i)DATE_ADD\(\s*([^,]+?)\s*,\s*INTERVAL\s+(\d+|\$\d+)\s+(\w+)\s*\)`)
	reFromUnixtime = regexp.MustCompile(`(?i)\bFROM_UNIXTIME\(`)
	reFKChecks     = regexp.MustCompile(`(?i)^\s*SET\s+FOREIGN_KEY_CHECKS\s*=\s*([01])\s*;?\s*$`)
	reUUID         = regexp.MustCompile(`(?i)\bUUID\(\s*\)`)
	reInsertIgnore = regexp.MustCompile(`(?i)^(\s*)INSERT\s+IGNORE\s+INTO`)
	reOnConflict   = regexp.MustCompile(`(?i)\bON\s+CONFLICT\b`)
	reReturning    = regexp.MustCompile(`(?i)\s+RETURNING\s`)
	reOnDuplicate  = regexp.MustCompile(`(?is)\s+ON\s+DUPLICATE\s+KEY\s+UPDATE\s+(.*)$`)
	reReplaceInto  = regexp.MustCompile(`(?is)^(\s*)REPLACE\s+INTO\s+(\S+)\s*\(([^)]*)\)`)
	reValuesFunc   = regexp.MustCompile(`(?i)\bVALUES\(\s*([A-Za-z_][A-Za-z0-9_]*)\s*\)`)
	reUnixTSExpr   = regexp.MustCompile(`(?i)UNIX_TIMESTAMP\(\s*([^)]+?)\s*\)`)
	reUnixTS       = regexp.MustCompile(`(?i)UNIX_TIMESTAMP\(\s*\)`)
	reCurdate      = regexp.MustCompile(`(?i)CURDATE\(\s*\)`)
	reExtractEpoch = regexp.MustCompile(`(?i)EXTRACT\(\s*EPOCH\s+FROM\s+(.+?)\s*\)::bigint`)
)

// ConvertUpsert converts a MySQL-dialect upsert for the active driver and
// then applies ConvertPlaceholders. Write the query the MySQL way, either
//
//	INSERT INTO t (a, b, c) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE c = VALUES(c), d = ?
//	REPLACE INTO t (a, b, c) VALUES (?, ?, ?)
//
// and name the unique key that decides a conflict in conflictCols. MySQL
// gets the query unchanged. PostgreSQL needs that key spelled out, so it gets
// ON CONFLICT (conflictCols) DO UPDATE SET …, with VALUES(col) rewritten to
// EXCLUDED.col; REPLACE INTO updates every non-key column. Placeholder order
// is preserved.
func ConvertUpsert(query string, conflictCols ...string) string {
	if IsMySQL() {
		return ConvertPlaceholders(query)
	}
	if len(conflictCols) == 0 {
		panic(fmt.Sprintf("ConvertUpsert: conflict columns are required for PostgreSQL.\nQuery: %s", query))
	}
	target := " ON CONFLICT (" + strings.Join(conflictCols, ", ") + ") DO UPDATE SET "

	if m := reReplaceInto.FindStringSubmatch(query); m != nil {
		isKey := make(map[string]bool, len(conflictCols))
		for _, c := range conflictCols {
			isKey[strings.ToLower(strings.TrimSpace(c))] = true
		}
		var sets []string
		for _, col := range strings.Split(m[3], ",") {
			col = strings.Trim(strings.TrimSpace(col), "`\"")
			if !isKey[strings.ToLower(col)] {
				sets = append(sets, col+" = EXCLUDED."+col)
			}
		}
		query = reReplaceInto.ReplaceAllString(query, "${1}INSERT INTO ${2} (${3})")
		if len(sets) == 0 {
			return ConvertPlaceholders(appendBeforeReturning(query, " ON CONFLICT ("+strings.Join(conflictCols, ", ")+") DO NOTHING"))
		}
		return ConvertPlaceholders(appendBeforeReturning(query, target+strings.Join(sets, ", ")))
	}

	if loc := reOnDuplicate.FindStringSubmatchIndex(query); loc != nil {
		assignments := reValuesFunc.ReplaceAllString(query[loc[2]:loc[3]], "EXCLUDED.$1")
		return ConvertPlaceholders(query[:loc[0]] + target + strings.TrimRight(assignments, " \t\r\n;"))
	}
	return ConvertPlaceholders(query)
}

// MySQL: Use LastInsertId() after insert.
func ConvertReturning(query string) (string, bool) {
	if !IsMySQL() {
		return query, strings.Contains(strings.ToUpper(query), "RETURNING")
	}

	// For MySQL, remove RETURNING clause
	if strings.Contains(strings.ToUpper(query), "RETURNING") {
		// Remove RETURNING clause for MySQL
		re := regexp.MustCompile(`(?i)\s+RETURNING\s+.*$`)
		query = re.ReplaceAllString(query, "")
		return query, true // Indicates we need to use LastInsertId
	}

	return query, false
}

// QuoteIdentifier quotes table/column names based on database.
func QuoteIdentifier(name string) string {
	if IsMySQL() {
		return fmt.Sprintf("`%s`", name)
	}
	// PostgreSQL uses double quotes, but often doesn't need them
	// Only quote if necessary (contains special chars or is reserved word)
	return name
}

// BuildInsertQuery builds an INSERT query compatible with the current database.
// Returns a query with ? placeholders - caller must use ConvertPlaceholders() before executing.
func BuildInsertQuery(table string, columns []string, returning bool) string {
	quotedTable := QuoteIdentifier(table)
	quotedColumns := make([]string, len(columns))
	placeholders := make([]string, len(columns))

	for i, col := range columns {
		quotedColumns[i] = QuoteIdentifier(col)
		placeholders[i] = "?"
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		quotedTable,
		strings.Join(quotedColumns, ", "),
		strings.Join(placeholders, ", ")) //nolint:gk-sql-sprintf // quoted identifier built from validated table/column args

	if returning && IsPostgreSQL() {
		query += " RETURNING *"
	}

	return query
}

// BuildUpdateQuery builds an UPDATE query compatible with the current database.
// Returns a query with ? placeholders - caller must use ConvertPlaceholders() before executing.
// The whereClause should also use ? placeholders.
func BuildUpdateQuery(table string, setColumns []string, whereClause string) string {
	quotedTable := QuoteIdentifier(table)
	setClauses := make([]string, len(setColumns))

	for i, col := range setColumns {
		quotedCol := QuoteIdentifier(col)
		setClauses[i] = fmt.Sprintf("%s = ?", quotedCol)
	}

	query := fmt.Sprintf("UPDATE %s SET %s", quotedTable, strings.Join(setClauses, ", ")) //nolint:gk-sql-sprintf // quoted identifier built from validated table/column args
	if whereClause != "" {
		query += " WHERE " + whereClause
	}

	return query
}
