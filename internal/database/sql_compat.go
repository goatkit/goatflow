package database

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// GetDBDriver returns the current database driver
func GetDBDriver() string {
	driver := os.Getenv("DB_DRIVER")
	if driver == "" {
		driver = "postgres"
	}
	return strings.ToLower(driver)
}

// IsMySQL returns true if using MySQL/MariaDB
func IsMySQL() bool {
	driver := GetDBDriver()
	return driver == "mysql" || driver == "mariadb"
}

// IsPostgreSQL returns true if using PostgreSQL
func IsPostgreSQL() bool {
	return GetDBDriver() == "postgres"
}

// ConvertPlaceholders converts PostgreSQL placeholders ($1, $2) to MySQL placeholders (?)
// This allows us to write queries in PostgreSQL format and auto-convert for MySQL
func ConvertPlaceholders(query string) string {
	if !IsMySQL() {
		return query // No conversion needed for PostgreSQL
	}
	
	// Replace $1, $2, $3 etc with ?
	re := regexp.MustCompile(`\$\d+`)
	
	// Track placeholder positions to ensure correct ordering
	placeholders := re.FindAllString(query, -1)
	
	// Replace each placeholder with ?
	result := query
	for _, placeholder := range placeholders {
		result = strings.Replace(result, placeholder, "?", 1)
	}
	
	return result
}

// ConvertReturning handles RETURNING clause differences
// PostgreSQL: INSERT ... RETURNING *
// MySQL: Use LastInsertId() after insert
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

// QuoteIdentifier quotes table/column names based on database
func QuoteIdentifier(name string) string {
	if IsMySQL() {
		return fmt.Sprintf("`%s`", name)
	}
	// PostgreSQL uses double quotes, but often doesn't need them
	// Only quote if necessary (contains special chars or is reserved word)
	return name
}

// BuildInsertQuery builds an INSERT query compatible with the current database
func BuildInsertQuery(table string, columns []string, returning bool) string {
	quotedTable := QuoteIdentifier(table)
	quotedColumns := make([]string, len(columns))
	placeholders := make([]string, len(columns))
	
	for i, col := range columns {
		quotedColumns[i] = QuoteIdentifier(col)
		if IsMySQL() {
			placeholders[i] = "?"
		} else {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}
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

// BuildUpdateQuery builds an UPDATE query compatible with the current database
func BuildUpdateQuery(table string, setColumns []string, whereClause string) string {
	quotedTable := QuoteIdentifier(table)
	setClauses := make([]string, len(setColumns))
	
	paramOffset := 1
	for i, col := range setColumns {
		quotedCol := QuoteIdentifier(col)
		if IsMySQL() {
			setClauses[i] = fmt.Sprintf("%s = ?", quotedCol)
		} else {
			setClauses[i] = fmt.Sprintf("%s = $%d", quotedCol, paramOffset)
			paramOffset++
		}
	}
	
	// Adjust WHERE clause placeholders
	if whereClause != "" && !IsMySQL() {
		// Update placeholder numbers in WHERE clause for PostgreSQL
		whereClause = adjustPlaceholderNumbers(whereClause, paramOffset)
	}
	
	query := fmt.Sprintf("UPDATE %s SET %s", quotedTable, strings.Join(setClauses, ", "))
	if whereClause != "" {
		query += " WHERE " + whereClause
	}
	
	return query
}

// adjustPlaceholderNumbers updates $1, $2 to start from the given offset
func adjustPlaceholderNumbers(clause string, offset int) string {
	re := regexp.MustCompile(`\$(\d+)`)
	return re.ReplaceAllStringFunc(clause, func(match string) string {
		var num int
		fmt.Sscanf(match, "$%d", &num)
		return fmt.Sprintf("$%d", num + offset - 1)
	})
}