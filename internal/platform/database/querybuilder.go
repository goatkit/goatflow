package database

import (
	"context"
	"database/sql"
	"fmt"
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

// Rebind converts a query with ? placeholders to the appropriate format for the database.
// This allows writing queries in MySQL format and auto-converting for PostgreSQL.
func (qb *QueryBuilder) Rebind(query string) string {
	return qb.db.Rebind(query)
}

// Select executes a query and scans results into dest (slice of structs).
func (qb *QueryBuilder) Select(dest interface{}, query string, args ...interface{}) error {
	return qb.db.Select(dest, qb.Rebind(query), args...)
}

// SelectContext executes a query with context and scans results into dest.
func (qb *QueryBuilder) SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return qb.db.SelectContext(ctx, dest, qb.Rebind(query), args...)
}

// Get executes a query expecting a single row and scans into dest (struct).
func (qb *QueryBuilder) Get(dest interface{}, query string, args ...interface{}) error {
	return qb.db.Get(dest, qb.Rebind(query), args...)
}

// GetContext executes a query with context expecting a single row.
func (qb *QueryBuilder) GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return qb.db.GetContext(ctx, dest, qb.Rebind(query), args...)
}

// Exec executes a query without returning rows.
func (qb *QueryBuilder) Exec(query string, args ...interface{}) (sql.Result, error) {
	return qb.db.Exec(qb.Rebind(query), args...)
}

// ExecContext executes a query with context without returning rows.
func (qb *QueryBuilder) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return qb.db.ExecContext(ctx, qb.Rebind(query), args...)
}

// QueryRow executes a query expecting a single row.
func (qb *QueryBuilder) QueryRow(query string, args ...interface{}) *sql.Row {
	return qb.db.QueryRow(qb.Rebind(query), args...)
}

// QueryRowContext executes a query with context expecting a single row.
func (qb *QueryBuilder) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return qb.db.QueryRowContext(ctx, qb.Rebind(query), args...)
}

// Query executes a query returning multiple rows.
func (qb *QueryBuilder) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return qb.db.Query(qb.Rebind(query), args...)
}

// QueryContext executes a query with context returning multiple rows.
func (qb *QueryBuilder) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return qb.db.QueryContext(ctx, qb.Rebind(query), args...)
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

// Named returns a query with named parameter support.
// Example: Named("SELECT * FROM users WHERE name = :name", map[string]interface{}{"name": "john"}).
func (qb *QueryBuilder) Named(query string, arg interface{}) (string, []interface{}, error) {
	return sqlx.Named(query, arg)
}

// NamedExec executes a named query.
func (qb *QueryBuilder) NamedExec(query string, arg interface{}) (sql.Result, error) {
	return qb.db.NamedExec(query, arg)
}

// NamedExecContext executes a named query with context.
func (qb *QueryBuilder) NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error) {
	return qb.db.NamedExecContext(ctx, query, arg)
}

// SelectBuilder provides a fluent interface for building SELECT queries safely.
type SelectBuilder struct {
	qb         *QueryBuilder
	columns    []string
	table      string
	joins      []string
	where      []string
	args       []interface{}
	groupBy    []string
	having     []string
	havingArgs []interface{}
	orderBy    []string
	limit      int
	offset     int
	hasLimit   bool
	hasOffset  bool
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

// Join adds a JOIN clause.
func (sb *SelectBuilder) Join(join string) *SelectBuilder {
	sb.joins = append(sb.joins, "JOIN "+join)
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

// WhereIn adds a WHERE IN condition.
func (sb *SelectBuilder) WhereIn(column string, values interface{}) *SelectBuilder {
	sb.where = append(sb.where, column+" IN (?)")
	sb.args = append(sb.args, values)
	return sb
}

// GroupBy adds GROUP BY columns.
func (sb *SelectBuilder) GroupBy(columns ...string) *SelectBuilder {
	sb.groupBy = append(sb.groupBy, columns...)
	return sb
}

// Having adds a HAVING condition.
func (sb *SelectBuilder) Having(condition string, args ...interface{}) *SelectBuilder {
	sb.having = append(sb.having, condition)
	sb.havingArgs = append(sb.havingArgs, args...)
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

	allArgs := make([]interface{}, 0, len(sb.args)+len(sb.havingArgs)+2)
	allArgs = append(allArgs, sb.args...)

	if len(sb.where) > 0 {
		query.WriteString(" WHERE ")
		query.WriteString(strings.Join(sb.where, " AND "))
	}

	if len(sb.groupBy) > 0 {
		query.WriteString(" GROUP BY ")
		query.WriteString(strings.Join(sb.groupBy, ", "))
	}

	if len(sb.having) > 0 {
		query.WriteString(" HAVING ")
		query.WriteString(strings.Join(sb.having, " AND "))
		allArgs = append(allArgs, sb.havingArgs...)
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

// Select executes the query and scans into dest.
func (sb *SelectBuilder) Select(dest interface{}) error {
	query, args, err := sb.ToSQL()
	if err != nil {
		return err
	}
	return sb.qb.db.Select(dest, query, args...)
}

// SelectContext executes the query with context and scans into dest.
func (sb *SelectBuilder) SelectContext(ctx context.Context, dest interface{}) error {
	query, args, err := sb.ToSQL()
	if err != nil {
		return err
	}
	return sb.qb.db.SelectContext(ctx, dest, query, args...)
}

// Get executes the query expecting a single row.
func (sb *SelectBuilder) Get(dest interface{}) error {
	query, args, err := sb.ToSQL()
	if err != nil {
		return err
	}
	return sb.qb.db.Get(dest, query, args...)
}

// GetContext executes the query with context expecting a single row.
func (sb *SelectBuilder) GetContext(ctx context.Context, dest interface{}) error {
	query, args, err := sb.ToSQL()
	if err != nil {
		return err
	}
	return sb.qb.db.GetContext(ctx, dest, query, args...)
}
