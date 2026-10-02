package database

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/jmoiron/sqlx"
)

// QueryBuilder provides a safe, sqlx-based query builder that eliminates SQL injection risks.
// It wraps the standard sql.DB with sqlx functionality and handles placeholder conversion.
type QueryBuilder struct {
	db         *sqlx.DB
	bindType   int
	driverName string
}

// NewQueryBuilder creates a QueryBuilder from an existing *sql.DB connection.
func NewQueryBuilder(db *sql.DB) (*QueryBuilder, error) {
	driverName := GetDBDriver()
	sqlxDB := sqlx.NewDb(db, driverName)

	bindType := sqlx.DOLLAR // PostgreSQL default
	if IsMySQL() {
		bindType = sqlx.QUESTION
	}

	return &QueryBuilder{
		db:         sqlxDB,
		bindType:   bindType,
		driverName: driverName,
	}, nil
}

// GetQueryBuilder returns a QueryBuilder using the default database connection.
func GetQueryBuilder() (*QueryBuilder, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}
	return NewQueryBuilder(db)
}

// DB returns the underlying sqlx.DB for advanced operations.
func (qb *QueryBuilder) DB() *sqlx.DB {
	return qb.db
}

// dollarPlaceholderRe matches PostgreSQL $N placeholders, the mark of SQL that
// has already been through ConvertPlaceholders.
var dollarPlaceholderRe = regexp.MustCompile(`\$\d+`)

// Rebind converts a query with ? placeholders for the active driver through
// ConvertPlaceholders, so MySQL-dialect SQL gets the same rewrites as the rest
// of the codebase. Already-converted ($N) queries pass through unchanged, so the
// output of In and ToSQL can be handed back to Query/Exec.
func (qb *QueryBuilder) Rebind(query string) string {
	if dollarPlaceholderRe.MatchString(query) {
		return query
	}
	return ConvertPlaceholders(query)
}

// Exec executes a query without returning rows.
func (qb *QueryBuilder) Exec(query string, args ...interface{}) (sql.Result, error) {
	return qb.db.Exec(qb.Rebind(query), args...) // sql-converted: qb.Rebind applies ConvertPlaceholders
}

// QueryRow executes a query expecting a single row.
func (qb *QueryBuilder) QueryRow(query string, args ...interface{}) *sql.Row {
	return qb.db.QueryRow(qb.Rebind(query), args...) // sql-converted: qb.Rebind applies ConvertPlaceholders
}

// Query executes a query returning multiple rows.
func (qb *QueryBuilder) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return qb.db.Query(qb.Rebind(query), args...) // sql-converted: qb.Rebind applies ConvertPlaceholders
}

// In expands slice arguments for IN clauses.
// Example: In("SELECT * FROM users WHERE id IN (?)", []int{1,2,3}).
// Returns: "SELECT * FROM users WHERE id IN (?, ?, ?)", [1, 2, 3].
func (qb *QueryBuilder) In(query string, args ...interface{}) (string, []interface{}, error) {
	q, a, err := sqlx.In(query, args...)
	if err != nil {
		return "", nil, err
	}
	return qb.Rebind(q), a, nil
}

// SelectBuilder provides a fluent interface for building SELECT queries safely.
type SelectBuilder struct {
	qb        *QueryBuilder
	columns   []string
	table     string
	joins     []string
	where     []string
	args      []interface{}
	orderBy   []string
	limit     int
	offset    int
	hasLimit  bool
	hasOffset bool
}

// NewSelect creates a new SelectBuilder.
func (qb *QueryBuilder) NewSelect(columns ...string) *SelectBuilder {
	return &SelectBuilder{
		qb:      qb,
		columns: columns,
	}
}

// From sets the table to select from.
func (sb *SelectBuilder) From(table string) *SelectBuilder {
	sb.table = table
	return sb
}

// LeftJoin adds a LEFT JOIN clause.
func (sb *SelectBuilder) LeftJoin(join string) *SelectBuilder {
	sb.joins = append(sb.joins, "LEFT JOIN "+join)
	return sb
}

// Where adds a WHERE condition with parameterized values.
func (sb *SelectBuilder) Where(condition string, args ...interface{}) *SelectBuilder {
	sb.where = append(sb.where, condition)
	sb.args = append(sb.args, args...)
	return sb
}

// OrderBy adds ORDER BY columns.
func (sb *SelectBuilder) OrderBy(columns ...string) *SelectBuilder {
	sb.orderBy = append(sb.orderBy, columns...)
	return sb
}

// Limit sets the LIMIT clause.
func (sb *SelectBuilder) Limit(limit int) *SelectBuilder {
	sb.limit = limit
	sb.hasLimit = true
	return sb
}

// Offset sets the OFFSET clause.
func (sb *SelectBuilder) Offset(offset int) *SelectBuilder {
	sb.offset = offset
	sb.hasOffset = true
	return sb
}

// ToSQL builds the SQL query and returns it with arguments.
func (sb *SelectBuilder) ToSQL() (string, []interface{}, error) {
	if sb.table == "" {
		return "", nil, fmt.Errorf("table not specified")
	}

	var query strings.Builder
	query.WriteString("SELECT ")
	if len(sb.columns) == 0 {
		query.WriteString("*")
	} else {
		query.WriteString(strings.Join(sb.columns, ", "))
	}
	query.WriteString(" FROM ")
	query.WriteString(sb.table)

	for _, join := range sb.joins {
		query.WriteString(" ")
		query.WriteString(join)
	}

	allArgs := make([]interface{}, 0, len(sb.args)+2)
	allArgs = append(allArgs, sb.args...)

	if len(sb.where) > 0 {
		query.WriteString(" WHERE ")
		query.WriteString(strings.Join(sb.where, " AND "))
	}

	if len(sb.orderBy) > 0 {
		query.WriteString(" ORDER BY ")
		query.WriteString(strings.Join(sb.orderBy, ", "))
	}

	if sb.hasLimit {
		query.WriteString(" LIMIT ?")
		allArgs = append(allArgs, sb.limit)
	}

	if sb.hasOffset {
		query.WriteString(" OFFSET ?")
		allArgs = append(allArgs, sb.offset)
	}

	// Handle IN clause expansion
	q, args, err := sb.qb.In(query.String(), allArgs...)
	if err != nil {
		return "", nil, err
	}

	return q, args, nil
}
