package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/gotrs-io/gotrs-ce/internal/services/registry"
)

// MySQLService implements DatabaseService for MySQL.
type MySQLService struct {
	mu      sync.RWMutex
	config  *registry.ServiceConfig
	db      *sql.DB
	health  *registry.ServiceHealth
	metrics *registry.ServiceMetrics
}

// NewMySQLService creates a new MySQL service instance.
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

// Connect establishes connection to MySQL.
func (s *MySQLService) Connect(ctx context.Context) error {
	s.mu.RLock()
	cfg := s.config
	s.mu.RUnlock()

	if cfg == nil {
		return fmt.Errorf("service configuration not set")
	}

	connStr := buildMySQLConnectionString(cfg)

	db, err := sql.Open("mysql", connStr)
	if err != nil {
		s.mu.Lock()
		s.health.Status = registry.StatusUnhealthy
		s.health.Error = err.Error()
		s.mu.Unlock()
		return fmt.Errorf("failed to open database: %w", err)
	}

	if cfg.MaxConns > 0 {
		db.SetMaxOpenConns(cfg.MaxConns)
	}
	if cfg.MinConns > 0 {
		db.SetMaxIdleConns(cfg.MinConns)
	}
	db.SetConnMaxLifetime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		s.mu.Lock()
		s.health.Status = registry.StatusUnhealthy
		s.health.Error = err.Error()
		s.mu.Unlock()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	s.mu.Lock()
	if s.db != nil {
		_ = s.db.Close()
	}
	s.db = db
	s.health.Status = registry.StatusHealthy
	s.health.Error = ""
	s.health.LastChecked = time.Now()
	s.mu.Unlock()

	return nil
}

// Disconnect closes the database connection.
func (s *MySQLService) Disconnect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		err := s.db.Close()
		s.db = nil
		s.health.Status = registry.StatusUnhealthy
		s.health.Error = ""
		return err
	}
	return nil
}

// GetDB returns the underlying database connection.
func (s *MySQLService) GetDB() *sql.DB {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db == nil {
		if err := s.Connect(context.Background()); err != nil {
			return nil
		}
		s.mu.RLock()
		db = s.db
		s.mu.RUnlock()
	}

	return db
}

// Health returns the service health status.
func (s *MySQLService) Health(ctx context.Context) (*registry.ServiceHealth, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		if err := s.db.PingContext(ctx); err != nil {
			s.health.Status = registry.StatusUnhealthy
			s.health.Error = err.Error()
			s.health.LastChecked = time.Now()
			return s.health, err
		}
		s.health.Status = registry.StatusHealthy
		s.health.Error = ""
		s.health.LastChecked = time.Now()
	}

	return s.health, nil
}

// Metrics returns service metrics.
func (s *MySQLService) Metrics(ctx context.Context) (*registry.ServiceMetrics, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		stats := s.db.Stats()
		s.metrics.Connections = stats.OpenConnections
	}

	return s.metrics, nil
}

// Ping tests the database connection.
func (s *MySQLService) Ping(ctx context.Context) error {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db == nil {
		return fmt.Errorf("database connection not established")
	}

	return db.PingContext(ctx)
}

// GetConfig returns the service configuration.
func (s *MySQLService) GetConfig() *registry.ServiceConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.config
}

// UpdateConfig updates the service configuration.
func (s *MySQLService) UpdateConfig(config *registry.ServiceConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	s.mu.Lock()
	s.config = config
	s.health.ServiceID = config.ID
	s.health.Status = registry.StatusInitializing
	s.metrics.ServiceID = config.ID
	s.mu.Unlock()

	return s.Connect(context.Background())
}

// Type returns the service type.
func (s *MySQLService) Type() registry.ServiceType {
	return registry.ServiceTypeDatabase
}

// Provider returns the service provider.
func (s *MySQLService) Provider() registry.ServiceProvider {
	return registry.ProviderMySQL
}

// ID returns the service ID.
func (s *MySQLService) ID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.config == nil {
		return ""
	}

	return s.config.ID
}

// Query executes a query that returns rows.
func (s *MySQLService) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db == nil {
		return nil, fmt.Errorf("database connection not established")
	}

	return db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row.
func (s *MySQLService) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db == nil {
		return nil
	}

	return db.QueryRowContext(ctx, query, args...)
}

// Exec executes a query without returning any rows.
func (s *MySQLService) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db == nil {
		return nil, fmt.Errorf("database connection not established")
	}

	return db.ExecContext(ctx, query, args...)
}

// BeginTx starts a database transaction.
func (s *MySQLService) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db == nil {
		return nil, fmt.Errorf("database connection not established")
	}

	return db.BeginTx(ctx, opts)
}

// SetMaxOpenConns sets the maximum number of open connections.
func (s *MySQLService) SetMaxOpenConns(n int) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db != nil {
		db.SetMaxOpenConns(n)
	}
}

// SetMaxIdleConns sets the maximum number of idle connections.
func (s *MySQLService) SetMaxIdleConns(n int) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db != nil {
		db.SetMaxIdleConns(n)
	}
}

// SetConnMaxLifetime sets the maximum lifetime of connections.
func (s *MySQLService) SetConnMaxLifetime(d time.Duration) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db != nil {
		db.SetConnMaxLifetime(d)
	}
}

// RunMigrations runs database migrations.
func (s *MySQLService) RunMigrations(ctx context.Context, migrationsPath string) error {
	// TODO: Implement MySQL migration support
	return fmt.Errorf("migrations not yet implemented for MySQL")
}

// GetSchemaVersion returns the current schema version.
func (s *MySQLService) GetSchemaVersion() (int, error) {
	// TODO: Implement schema version tracking for MySQL
	return 0, fmt.Errorf("schema version not yet implemented for MySQL")
}

// Backup performs a database backup.
func (s *MySQLService) Backup(ctx context.Context, path string) error {
	// TODO: Implement MySQL backup using mysqldump
	return fmt.Errorf("backup not yet implemented for MySQL")
}

// Restore restores a database from backup.
func (s *MySQLService) Restore(ctx context.Context, path string) error {
	// TODO: Implement MySQL restore
	return fmt.Errorf("restore not yet implemented for MySQL")
}

// buildConnectionString builds MySQL DSN.
func buildMySQLConnectionString(config *registry.ServiceConfig) string {
	if config == nil {
		return ""
	}

	if config.Options != nil {
		if url, ok := config.Options["connection_url"].(string); ok && url != "" {
			if strings.Contains(url, "@tcp(") {
				return url
			}

			formatted := strings.TrimPrefix(url, "mysql://")
			atIndex := strings.Index(formatted, "@")
			if atIndex > 0 {
				slashIndex := strings.Index(formatted[atIndex+1:], "/")
				if slashIndex > 0 {
					slashIndex += atIndex + 1
					userPass := formatted[:atIndex]
					hostPort := formatted[atIndex+1 : slashIndex]
					database := formatted[slashIndex+1:]
					if strings.Contains(database, "?") {
						return fmt.Sprintf("%s@tcp(%s)/%s", userPass, hostPort, database)
					}
					return fmt.Sprintf("%s@tcp(%s)/%s?parseTime=true", userPass, hostPort, database)
				}
			}
		}
	}

	host := config.Host
	if host == "" {
		host = "localhost"
	}

	port := config.Port
	if port == 0 {
		port = 3306
	}

	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		config.Username,
		config.Password,
		host,
		port,
		config.Database,
	)
}
