// Package database provides database service interfaces and implementations.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/goatkit/goatflow/internal/services/registry"
)

// DatabaseService extends the base ServiceInterface with database-specific methods.
type DatabaseService interface {
	registry.ServiceInterface

	// Database operations
	Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row
	Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error)

	// Transaction support
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)

	// Connection pool management
	GetDB() *sql.DB
	SetMaxOpenConns(n int)
	SetMaxIdleConns(n int)
	SetConnMaxLifetime(d time.Duration)

	// Migration support
	RunMigrations(ctx context.Context, migrationsPath string) error
	GetSchemaVersion() (int, error)

	// Backup and restore
	Backup(ctx context.Context, path string) error
	Restore(ctx context.Context, path string) error
}

// QueryBuilder provides a fluent interface for building queries.
type QueryBuilder interface {
	Select(columns ...string) QueryBuilder
	From(table string) QueryBuilder
	Where(condition string, args ...interface{}) QueryBuilder
	Join(joinType, table, condition string) QueryBuilder
	GroupBy(columns ...string) QueryBuilder
	Having(condition string, args ...interface{}) QueryBuilder
	OrderBy(column string, desc bool) QueryBuilder
	Limit(limit int) QueryBuilder
	Offset(offset int) QueryBuilder
	Build() (string, []interface{})
}

// DatabaseFactory creates database service instances.
type DatabaseFactory struct {
	// Map of provider to constructor function
	constructors map[registry.ServiceProvider]func(*registry.ServiceConfig) (DatabaseService, error)
}

// NewDatabaseFactory creates a new database factory.
func NewDatabaseFactory() *DatabaseFactory {
	return &DatabaseFactory{
		constructors: make(map[registry.ServiceProvider]func(*registry.ServiceConfig) (DatabaseService, error)),
	}
}

// RegisterProvider registers a database provider constructor.
func (f *DatabaseFactory) RegisterProvider(provider registry.ServiceProvider, constructor func(*registry.ServiceConfig) (DatabaseService, error)) {
	f.constructors[provider] = constructor
}

// CreateService creates a database service instance.
func (f *DatabaseFactory) CreateService(config *registry.ServiceConfig) (registry.ServiceInterface, error) {
	constructor, exists := f.constructors[config.Provider]
	if !exists {
		return nil, fmt.Errorf("unsupported database provider: %s", config.Provider)
	}

	return constructor(config)
}

// SupportedProviders returns the list of supported database providers.
func (f *DatabaseFactory) SupportedProviders() []registry.ServiceProvider {
	providers := make([]registry.ServiceProvider, 0, len(f.constructors))
	for provider := range f.constructors {
		providers = append(providers, provider)
	}
	return providers
}

// DatabasePool manages multiple database connections with load balancing.
type DatabasePool struct {
	primary  DatabaseService
	replicas []DatabaseService
	strategy LoadBalanceStrategy
}

// LoadBalanceStrategy defines how to distribute read queries.
type LoadBalanceStrategy string

const (
	StrategyRoundRobin LoadBalanceStrategy = "round-robin"
	StrategyLeastConn  LoadBalanceStrategy = "least-conn"
	StrategyRandom     LoadBalanceStrategy = "random"
	StrategyPrimary    LoadBalanceStrategy = "primary-only"
)

// NewDatabasePool creates a new database pool.
func NewDatabasePool(primary DatabaseService, replicas []DatabaseService, strategy LoadBalanceStrategy) *DatabasePool {
	return &DatabasePool{
		primary:  primary,
		replicas: replicas,
		strategy: strategy,
	}
}

// Primary returns the primary database for writes.
func (p *DatabasePool) Primary() DatabaseService {
	return p.primary
}

// Replica returns a replica database for reads.
func (p *DatabasePool) Replica() DatabaseService {
	if len(p.replicas) == 0 || p.strategy == StrategyPrimary {
		return p.primary
	}

	// Simple round-robin for now
	// In production, implement proper strategies
	return p.replicas[0]
}

// DatabaseConfig provides database-specific configuration.
type DatabaseConfig struct {
	*registry.ServiceConfig

	// Database-specific settings
	SSLMode          string `yaml:"ssl_mode" json:"ssl_mode"`
	ApplicationName  string `yaml:"application_name" json:"application_name"`
	SearchPath       string `yaml:"search_path" json:"search_path"`
	StatementTimeout int    `yaml:"statement_timeout" json:"statement_timeout"`
	LockTimeout      int    `yaml:"lock_timeout" json:"lock_timeout"`

	// Connection pool settings
	MaxOpenConns    int           `yaml:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" json:"conn_max_idle_time"`

	// Replication settings
	Role             string   `yaml:"role" json:"role"` // "primary" or "replica"
	ReplicationSlots []string `yaml:"replication_slots" json:"replication_slots"`
}
