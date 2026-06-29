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

// rewriteForPostgreSQL converts MySQL-specific SQL functions to PostgreSQL equivalents.
func rewriteForPostgreSQL(query string) string {
	// DATE_SUB(expr, INTERVAL n UNIT) → (expr - INTERVAL 'n UNIT')
	query = reDateSub.ReplaceAllStringFunc(query, func(match string) string {
		parts := reDateSub.FindStringSubmatch(match)
		if len(parts) == 4 {
			return fmt.Sprintf("(%s - INTERVAL '%s %s')", parts[1], parts[2], strings.ToLower(parts[3]))
		}
		return match
	})

	// DATE_ADD(expr, INTERVAL n UNIT) → (expr + INTERVAL 'n UNIT')
	query = reDateAdd.ReplaceAllStringFunc(query, func(match string) string {
		parts := reDateAdd.FindStringSubmatch(match)
		if len(parts) == 4 {
			return fmt.Sprintf("(%s + INTERVAL '%s %s')", parts[1], parts[2], strings.ToLower(parts[3]))
		}
		return match
	})

	// UNIX_TIMESTAMP(expr) → EXTRACT(EPOCH FROM expr)::bigint
	query = reUnixTSExpr.ReplaceAllString(query, "EXTRACT(EPOCH FROM $1)::bigint")

	// UNIX_TIMESTAMP() → EXTRACT(EPOCH FROM NOW())::bigint
	query = reUnixTS.ReplaceAllString(query, "EXTRACT(EPOCH FROM NOW())::bigint")

	// CURDATE() → CURRENT_DATE
	query = reCurdate.ReplaceAllString(query, "CURRENT_DATE")

	return query
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
	reDateSub      = regexp.MustCompile(`(?i)DATE_SUB\(\s*([^,]+?)\s*,\s*INTERVAL\s+(\d+)\s+(\w+)\s*\)`)
	reDateAdd      = regexp.MustCompile(`(?i)DATE_ADD\(\s*([^,]+?)\s*,\s*INTERVAL\s+(\d+)\s+(\w+)\s*\)`)
	reUnixTSExpr   = regexp.MustCompile(`(?i)UNIX_TIMESTAMP\(\s*([^)]+?)\s*\)`)
	reUnixTS       = regexp.MustCompile(`(?i)UNIX_TIMESTAMP\(\s*\)`)
	reCurdate      = regexp.MustCompile(`(?i)CURDATE\(\s*\)`)
	reExtractEpoch = regexp.MustCompile(`(?i)EXTRACT\(\s*EPOCH\s+FROM\s+(.+?)\s*\)::bigint`)
)

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
		strings.Join(placeholders, ", "))

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

	query := fmt.Sprintf("UPDATE %s SET %s", quotedTable, strings.Join(setClauses, ", "))
	if whereClause != "" {
		query += " WHERE " + whereClause
	}

	return query
}
