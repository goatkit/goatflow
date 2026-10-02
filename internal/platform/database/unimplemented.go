package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrDatabaseNotImplemented is returned for database types that are declared
// (Oracle, SQLServer) but have no driver yet. Test for it with errors.Is.
var ErrDatabaseNotImplemented = errors.New("database type not implemented")

// unimplementedDatabase is the placeholder for a declared but unimplemented
// database type. It holds no SQL: Connect and every other error-returning
// method fail with ErrDatabaseNotImplemented, and the remaining methods return
// zero values. Callers must stop when Connect fails.
type unimplementedDatabase struct {
	config DatabaseConfig
}

func newUnimplementedDatabase(config DatabaseConfig) *unimplementedDatabase {
	return &unimplementedDatabase{config: config}
}

func (u *unimplementedDatabase) err() error {
	return fmt.Errorf("%s: %w (implemented: %s)", u.config.Type, ErrDatabaseNotImplemented, supportedTypesList())
}

func (u *unimplementedDatabase) Connect() error            { return u.err() }
func (u *unimplementedDatabase) Close() error              { return nil }
func (u *unimplementedDatabase) Ping() error               { return u.err() }
func (u *unimplementedDatabase) GetType() DatabaseType     { return u.config.Type }
func (u *unimplementedDatabase) GetConfig() DatabaseConfig { return u.config }

func (u *unimplementedDatabase) Query(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, u.err()
}

func (u *unimplementedDatabase) QueryRow(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (u *unimplementedDatabase) Exec(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, u.err()
}

func (u *unimplementedDatabase) Begin(context.Context) (ITransaction, error) { return nil, u.err() }

func (u *unimplementedDatabase) BeginTx(context.Context, *sql.TxOptions) (ITransaction, error) {
	return nil, u.err()
}

func (u *unimplementedDatabase) TableExists(context.Context, string) (bool, error) {
	return false, u.err()
}

func (u *unimplementedDatabase) GetTableColumns(context.Context, string) ([]ColumnInfo, error) {
	return nil, u.err()
}

func (u *unimplementedDatabase) CreateTable(context.Context, *TableDefinition) error { return u.err() }
func (u *unimplementedDatabase) DropTable(context.Context, string) error             { return u.err() }

func (u *unimplementedDatabase) CreateIndex(context.Context, string, string, []string, bool) error {
	return u.err()
}

func (u *unimplementedDatabase) DropIndex(context.Context, string, string) error { return u.err() }

func (u *unimplementedDatabase) Quote(string) string            { return "" }
func (u *unimplementedDatabase) QuoteValue(interface{}) string  { return "" }
func (u *unimplementedDatabase) GetLimitClause(int, int) string { return "" }
func (u *unimplementedDatabase) GetDateFunction() string        { return "" }
func (u *unimplementedDatabase) GetConcatFunction([]string) string {
	return ""
}
func (u *unimplementedDatabase) SupportsReturning() bool { return false }
func (u *unimplementedDatabase) Stats() sql.DBStats      { return sql.DBStats{} }
func (u *unimplementedDatabase) IsHealthy() bool         { return false }

func (u *unimplementedDatabase) BuildInsert(string, map[string]interface{}) (string, []interface{}) {
	return "", nil
}

func (u *unimplementedDatabase) BuildUpdate(string, map[string]interface{}, string, []interface{}) (string, []interface{}) {
	return "", nil
}

func (u *unimplementedDatabase) BuildSelect(string, []string, string, string, int) string {
	return ""
}
