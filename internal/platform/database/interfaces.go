package database

import (
	"context"
	"database/sql"
	"time"
)

// DatabaseType represents the supported database backends.
type DatabaseType string

const (
	PostgreSQL DatabaseType = "postgresql"
	MySQL      DatabaseType = "mysql"
	Oracle     DatabaseType = "oracle"
	SQLServer  DatabaseType = "sqlserver"
)

// DatabaseConfig holds database connection configuration.
type DatabaseConfig struct {
	Type     DatabaseType `json:"type" yaml:"type"`
	Host     string       `json:"host" yaml:"host"`
	Port     string       `json:"port" yaml:"port"`
	Database string       `json:"database" yaml:"database"`
	Username string       `json:"username" yaml:"username"`
	Password string       `json:"password" yaml:"password"`
	SSLMode  string       `json:"ssl_mode" yaml:"ssl_mode"`

	// Connection pool settings
	MaxOpenConns    int           `json:"max_open_conns" yaml:"max_open_conns"`
	MaxIdleConns    int           `json:"max_idle_conns" yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime" yaml:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `json:"conn_max_idle_time" yaml:"conn_max_idle_time"`

	// Database-specific options
	Options map[string]string `json:"options,omitempty" yaml:"options,omitempty"`
}

// IDatabase defines the database abstraction interface (inspired by OTRS Kernel::System::DB).
type IDatabase interface {
	// Connection management
	Connect() error
	Close() error
	Ping() error
	GetType() DatabaseType
	GetConfig() DatabaseConfig

	// Query operations
	Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row
	Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error)

	// Transaction support
	Begin(ctx context.Context) (ITransaction, error)
	BeginTx(ctx context.Context, opts *sql.TxOptions) (ITransaction, error)

	// Schema operations
	TableExists(ctx context.Context, tableName string) (bool, error)
	GetTableColumns(ctx context.Context, tableName string) ([]ColumnInfo, error)
	CreateTable(ctx context.Context, definition *TableDefinition) error
	DropTable(ctx context.Context, tableName string) error

	// Index operations
	CreateIndex(ctx context.Context, tableName, indexName string, columns []string, unique bool) error
	DropIndex(ctx context.Context, tableName, indexName string) error

	// Utility methods
	Quote(identifier string) string
	QuoteValue(value interface{}) string
	BuildInsert(tableName string, data map[string]interface{}) (string, []interface{})
	BuildUpdate(tableName string, data map[string]interface{}, where string, whereArgs []interface{}) (string, []interface{})
	BuildSelect(tableName string, columns []string, where string, orderBy string, limit int) string

	// Database-specific SQL generation
	GetLimitClause(limit, offset int) string
	GetDateFunction() string
	GetConcatFunction(fields []string) string
	SupportsReturning() bool

	// Health and monitoring
	Stats() sql.DBStats
	IsHealthy() bool
}

// ITransaction defines the transaction interface.
type ITransaction interface {
	Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row
	Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	Commit() error
	Rollback() error
}

// ColumnInfo represents database column information.
type ColumnInfo struct {
	Name            string
	DataType        string
	IsNullable      bool
	DefaultValue    *string
	MaxLength       *int
	IsPrimaryKey    bool
	IsAutoIncrement bool
}

// TableDefinition represents a database table structure.
type TableDefinition struct {
	Name        string
	Columns     []ColumnDefinition
	Indexes     []IndexDefinition
	Constraints []ConstraintDefinition
}

// ColumnDefinition defines a table column.
type ColumnDefinition struct {
	Name          string
	DataType      string
	Size          *int
	Precision     *int
	Scale         *int
	NotNull       bool
	PrimaryKey    bool
	AutoIncrement bool
	DefaultValue  *string
}

// IndexDefinition defines a table index.
type IndexDefinition struct {
	Name    string
	Columns []string
	Unique  bool
	Type    string // btree, hash, gin, gist for PostgreSQL
}

// ConstraintDefinition defines a table constraint.
type ConstraintDefinition struct {
	Name             string
	Type             string // PRIMARY_KEY, FOREIGN_KEY, UNIQUE, CHECK
	Columns          []string
	ReferenceTable   *string
	ReferenceColumns []string
	OnDelete         *string // CASCADE, SET NULL, RESTRICT
	OnUpdate         *string // CASCADE, SET NULL, RESTRICT
}

// IDatabaseFactory creates database instances.
type IDatabaseFactory interface {
	Create(config DatabaseConfig) (IDatabase, error)
	GetSupportedTypes() []DatabaseType
	ValidateConfig(config DatabaseConfig) error
}

// DatabaseFeatures represents database-specific feature support.
type DatabaseFeatures struct {
	SupportsReturning       bool
	SupportsUpsert          bool
	SupportsJSONColumn      bool
	SupportsArrayColumn     bool
	SupportsWindowFunctions bool
	SupportsCTE             bool // Common Table Expressions
	MaxIdentifierLength     int
	MaxIndexNameLength      int
}
