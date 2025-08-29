package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gotrs-io/gotrs-ce/internal/services/registry"
)

// MySQLService implements DatabaseService for MySQL
type MySQLService struct {
	config   *registry.ServiceConfig
	db       *sql.DB
	health   *registry.ServiceHealth
	metrics  *registry.ServiceMetrics
}

// NewMySQLService creates a new MySQL service instance
func NewMySQLService(config *registry.ServiceConfig) (DatabaseService, error) {
	return &MySQLService{
		config: config,
		health: &registry.ServiceHealth{
			ServiceID: config.ID,
			Status:    registry.StatusInitializing,
		},
		metrics: &registry.ServiceMetrics{
			ServiceID: config.ID,
		},
	}, nil
}

// Connect establishes connection to MySQL
func (s *MySQLService) Connect(ctx context.Context) error {
	// Build connection string
	connStr := s.buildConnectionString()
	
	// Open database connection
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		s.health.Status = registry.StatusUnhealthy
		s.health.Error = err.Error()
		return fmt.Errorf("failed to open database: %w", err)
	}
	
	// Configure connection pool
	if s.config.MaxConns > 0 {
		db.SetMaxOpenConns(s.config.MaxConns)
	}
	if s.config.MinConns > 0 {
		db.SetMaxIdleConns(s.config.MinConns)
	}
	db.SetConnMaxLifetime(5 * time.Minute)
	
	// Test connection
	if err := db.PingContext(ctx); err != nil {
		s.health.Status = registry.StatusUnhealthy
		s.health.Error = err.Error()
		return fmt.Errorf("failed to ping database: %w", err)
	}
	
	s.db = db
	s.health.Status = registry.StatusHealthy
	
	return nil
}

// Disconnect closes the database connection
func (s *MySQLService) Disconnect(ctx context.Context) error {
	if s.db != nil {
		err := s.db.Close()
		s.db = nil
		s.health.Status = registry.StatusUnhealthy
		return err
	}
	return nil
}

// GetDB returns the underlying database connection
func (s *MySQLService) GetDB() *sql.DB {
	return s.db
}

// Health returns the service health status
func (s *MySQLService) Health(ctx context.Context) (*registry.ServiceHealth, error) {
	if s.db != nil {
		if err := s.db.PingContext(ctx); err != nil {
			s.health.Status = registry.StatusUnhealthy
			s.health.Error = err.Error()
		} else {
			s.health.Status = registry.StatusHealthy
			s.health.Error = ""
		}
	}
	return s.health, nil
}

// Metrics returns service metrics
func (s *MySQLService) Metrics(ctx context.Context) (*registry.ServiceMetrics, error) {
	if s.db != nil {
		stats := s.db.Stats()
		s.metrics.Connections = stats.OpenConnections
	}
	return s.metrics, nil
}

// Ping tests the database connection
func (s *MySQLService) Ping(ctx context.Context) error {
	if s.db == nil {
		return fmt.Errorf("database connection not established")
	}
	return s.db.PingContext(ctx)
}

// GetConfig returns the service configuration
func (s *MySQLService) GetConfig() *registry.ServiceConfig {
	return s.config
}

// UpdateConfig updates the service configuration
func (s *MySQLService) UpdateConfig(config *registry.ServiceConfig) error {
	s.config = config
	// Reconnect with new config
	return s.Connect(context.Background())
}

// Type returns the service type
func (s *MySQLService) Type() registry.ServiceType {
	return registry.ServiceTypeDatabase
}

// Provider returns the service provider
func (s *MySQLService) Provider() registry.ServiceProvider {
	return registry.ProviderMySQL
}

// ID returns the service ID
func (s *MySQLService) ID() string {
	return s.config.ID
}

// Query executes a query that returns rows
func (s *MySQLService) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database connection not established")
	}
	return s.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row
func (s *MySQLService) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if s.db == nil {
		return nil
	}
	return s.db.QueryRowContext(ctx, query, args...)
}

// Exec executes a query without returning any rows
func (s *MySQLService) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database connection not established")
	}
	return s.db.ExecContext(ctx, query, args...)
}

// BeginTx starts a database transaction
func (s *MySQLService) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database connection not established")
	}
	return s.db.BeginTx(ctx, opts)
}

// SetMaxOpenConns sets the maximum number of open connections
func (s *MySQLService) SetMaxOpenConns(n int) {
	if s.db != nil {
		s.db.SetMaxOpenConns(n)
	}
}

// SetMaxIdleConns sets the maximum number of idle connections
func (s *MySQLService) SetMaxIdleConns(n int) {
	if s.db != nil {
		s.db.SetMaxIdleConns(n)
	}
}

// SetConnMaxLifetime sets the maximum lifetime of connections
func (s *MySQLService) SetConnMaxLifetime(d time.Duration) {
	if s.db != nil {
		s.db.SetConnMaxLifetime(d)
	}
}

// RunMigrations runs database migrations
func (s *MySQLService) RunMigrations(ctx context.Context, migrationsPath string) error {
	// TODO: Implement MySQL migration support
	return fmt.Errorf("migrations not yet implemented for MySQL")
}

// GetSchemaVersion returns the current schema version
func (s *MySQLService) GetSchemaVersion() (int, error) {
	// TODO: Implement schema version tracking for MySQL
	return 0, fmt.Errorf("schema version not yet implemented for MySQL")
}

// Backup performs a database backup
func (s *MySQLService) Backup(ctx context.Context, path string) error {
	// TODO: Implement MySQL backup using mysqldump
	return fmt.Errorf("backup not yet implemented for MySQL")
}

// Restore restores a database from backup
func (s *MySQLService) Restore(ctx context.Context, path string) error {
	// TODO: Implement MySQL restore
	return fmt.Errorf("restore not yet implemented for MySQL")
}

// buildConnectionString builds MySQL DSN
func (s *MySQLService) buildConnectionString() string {
	// Check for complete connection URL first
	if url, ok := s.config.Options["connection_url"].(string); ok && url != "" {
		// Parse MySQL URL format: mysql://user:password@host:port/database
		// Convert to DSN format: user:password@tcp(host:port)/database
		if len(url) > 8 && url[:8] == "mysql://" {
			url = url[8:] // Remove mysql:// prefix
		}
		// Find the @ and / to extract parts
		atIndex := -1
		slashIndex := -1
		for i, ch := range url {
			if ch == '@' && atIndex == -1 {
				atIndex = i
			}
			if ch == '/' && i > atIndex {
				slashIndex = i
				break
			}
		}
		if atIndex > 0 && slashIndex > atIndex {
			userPass := url[:atIndex]
			hostPort := url[atIndex+1 : slashIndex]
			database := url[slashIndex+1:]
			return fmt.Sprintf("%s@tcp(%s)/%s?parseTime=true", userPass, hostPort, database)
		}
	}
	
	// Build from individual components
	host := s.config.Host
	if host == "" {
		host = "localhost"
	}
	
	port := s.config.Port
	if port == 0 {
		port = 3306
	}
	
	// MySQL DSN format: username:password@tcp(host:port)/database?params
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		s.config.Username,
		s.config.Password,
		host,
		port,
		s.config.Database,
	)
	
	return dsn
}