package database

import (
	"context"
	"database/sql"
)

// TableSchema represents a table definition from YAML.
type TableSchema struct {
	Name       string                 `yaml:"name"`
	PK         string                 `yaml:"pk"` // Primary key field name (default: "id")
	Columns    map[string]ColumnDef   `yaml:"columns"`
	Indexes    []string               `yaml:"indexes"`
	Unique     []string               `yaml:"unique"`
	Timestamps bool                   `yaml:"timestamps"` // Add create_time, change_time automatically
	Meta       map[string]interface{} `yaml:"meta"`       // Driver-specific metadata
}

// ColumnDef represents a column definition.
type ColumnDef struct {
	Type     string      `yaml:"type"`     // varchar(200), int, serial, etc.
	Required bool        `yaml:"required"` // NOT NULL
	Unique   bool        `yaml:"unique"`
	Default  interface{} `yaml:"default"`
	Index    bool        `yaml:"index"`
}

// Query represents a SQL query with arguments.
type Query struct {
	SQL  string
	Args []interface{}
}

// Transaction represents a database transaction.
type Transaction interface {
	Commit() error
	Rollback() error
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

// DatabaseDriver interface for database abstraction.
type DatabaseDriver interface {
	// Connection management
	Connect(ctx context.Context, dsn string) error
	Close() error
	Ping(ctx context.Context) error

	// Schema operations
	CreateTable(schema TableSchema) (Query, error)
	DropTable(tableName string) (Query, error)
	TableExists(tableName string) (bool, error)

	// CRUD operations
	Insert(table string, data map[string]interface{}) (Query, error)
	Update(table string, data map[string]interface{}, where string, whereArgs ...interface{}) (Query, error)
	Delete(table string, where string, whereArgs ...interface{}) (Query, error)
	Select(table string, columns []string, where string, whereArgs ...interface{}) (Query, error)

	// Type mapping
	MapType(schemaType string) string // e.g., "serial" -> "AUTO_INCREMENT" or "SERIAL"

	// Feature detection
	SupportsReturning() bool
	SupportsLastInsertId() bool
	SupportsArrays() bool

	// Transaction support
	BeginTx(ctx context.Context) (Transaction, error)

	// Raw execution (for complex queries)
	Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// DumpDriver interface for SQL dump file handling.
type DumpDriver interface {
	// Parse and iterate through a dump file
	Open(filename string) error
	Close() error

	// Read next statement from dump
	NextStatement() (string, error)

	// Get table schemas from dump
	GetSchemas() ([]TableSchema, error)

	// Stream data for a specific table
	StreamTable(tableName string, callback func(row map[string]interface{}) error) error

	// Write operations (for export)
	WriteSchema(schema TableSchema) error
	WriteData(table string, rows []map[string]interface{}) error
}

// DriverRegistry for managing available drivers.
type DriverRegistry struct {
	drivers map[string]func() DatabaseDriver
	dumps   map[string]func() DumpDriver
}

var defaultRegistry = &DriverRegistry{
	drivers: make(map[string]func() DatabaseDriver),
	dumps:   make(map[string]func() DumpDriver),
}

// RegisterDriver registers a database driver.
func RegisterDriver(name string, factory func() DatabaseDriver) {
	defaultRegistry.drivers[name] = factory
}

// RegisterDumpDriver registers a dump file driver.
func RegisterDumpDriver(name string, factory func() DumpDriver) {
	defaultRegistry.dumps[name] = factory
}

// GetDriver returns a database driver by name.
func GetDriver(name string) (DatabaseDriver, error) {
	factory, ok := defaultRegistry.drivers[name]
	if !ok {
		return nil, sql.ErrConnDone
	}
	return factory(), nil
}

// GetDumpDriver returns a dump driver by name.
func GetDumpDriver(name string) (DumpDriver, error) {
	factory, ok := defaultRegistry.dumps[name]
	if !ok {
		return nil, sql.ErrConnDone
	}
	return factory(), nil
}
