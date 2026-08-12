package database

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
)

// TestDB wraps *sql.DB for tests so that SQL can be written portably with
// MySQL-style `?` placeholders (and bare/backticked identifiers) and runs
// against whichever driver the test harness selected (MySQL or PostgreSQL).
//
// Every query-taking method routes through the same driver-aware conversion the
// application uses. Conversion is guarded to be a no-op on already-converted
// ($N) queries, so tests that pre-convert still work.
type TestDB struct {
	*sql.DB
}

var quotedRe = regexp.MustCompile(`\$\d+`)

func (t *TestDB) convert(q string) string {
	if (strings.Contains(q, "?") || strings.Contains(q, "`")) && !quotedRe.MatchString(q) {
		return ConvertPlaceholders(q)
	}
	return q
}

// NewTestDB returns a converting wrapper around the configured test database.
// The error return keeps the `db, err := database.NewTestDB()` idiom symmetrical
// with the code it replaces so tests need only rename the call.
func NewTestDB() (*TestDB, error) {
	db, err := GetDB()
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, &sqlErr{msg: "test database unavailable"}
	}
	return &TestDB{DB: db}, nil
}

// sqlErr is a minimal error so NewTestDB can return an error without importing errors elsewhere.
type sqlErr struct{ msg string }

func (e *sqlErr) Error() string { return e.msg }

func (t *TestDB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return t.DB.Exec(t.convert(query), args...)
}

func (t *TestDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return t.DB.ExecContext(ctx, t.convert(query), args...)
}

func (t *TestDB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return t.DB.Query(t.convert(query), args...)
}

func (t *TestDB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return t.DB.QueryContext(ctx, t.convert(query), args...)
}

func (t *TestDB) QueryRow(query string, args ...interface{}) *sql.Row {
	return t.DB.QueryRow(t.convert(query), args...)
}

func (t *TestDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return t.DB.QueryRowContext(ctx, t.convert(query), args...)
}
