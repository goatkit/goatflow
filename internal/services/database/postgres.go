package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/lib/pq"

	"github.com/gotrs-io/gotrs-ce/internal/services/registry"
)

// PostgresService implements DatabaseService for PostgreSQL.
type PostgresService struct {
	mu      sync.RWMutex
	config  *registry.ServiceConfig
	db      *sql.DB
	health  *registry.ServiceHealth
	metrics *registry.ServiceMetrics
}

// NewPostgresService creates a new PostgreSQL service instance.
func NewPostgresService(config *registry.ServiceConfig) (DatabaseService, error) {
	return &PostgresService{
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

// Connect establishes connection to PostgreSQL.
func (s *PostgresService) Connect(ctx context.Context) error {
	s.mu.RLock()
	cfg := s.config
	s.mu.RUnlock()

	if cfg == nil {
		return fmt.Errorf("service configuration not set")
	}

	connStr := buildPostgresConnectionString(cfg)

	db, err := sql.Open("postgres", connStr)
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

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
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
func (s *PostgresService) Disconnect(ctx context.Context) error {
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

// Ping checks if the database is reachable.
func (s *PostgresService) Ping(ctx context.Context) error {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db == nil {
		return fmt.Errorf("database not connected")
	}

	return db.PingContext(ctx)
}

// Health returns the health status of the service.
func (s *PostgresService) Health(ctx context.Context) (*registry.ServiceHealth, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db == nil {
		s.mu.Lock()
		s.health.Status = registry.StatusUnhealthy
		s.health.Error = "database not connected"
		s.health.LastChecked = time.Now()
		health := s.health
		s.mu.Unlock()
		return health, fmt.Errorf("database not connected")
	}

	start := time.Now()
	err := db.PingContext(ctx)
	latency := time.Since(start)
	stats := db.Stats()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.health.Latency = latency
	s.health.LastChecked = time.Now()

	if err != nil {
		s.health.Status = registry.StatusUnhealthy
		s.health.Error = err.Error()
		return s.health, err
	}

	s.health.Status = registry.StatusHealthy
	s.health.Error = ""
	s.health.Metadata = map[string]interface{}{
		"open_connections":    stats.OpenConnections,
		"in_use":              stats.InUse,
		"idle":                stats.Idle,
		"wait_count":          stats.WaitCount,
		"wait_duration":       stats.WaitDuration.String(),
		"max_idle_closed":     stats.MaxIdleClosed,
		"max_lifetime_closed": stats.MaxLifetimeClosed,
	}

	return s.health, nil
}

// Metrics returns performance metrics.
func (s *PostgresService) Metrics(ctx context.Context) (*registry.ServiceMetrics, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db == nil {
		s.mu.RLock()
		metrics := s.metrics
		s.mu.RUnlock()
		return metrics, nil
	}

	stats := db.Stats()

	s.mu.Lock()
	s.metrics.Connections = stats.OpenConnections
	s.metrics.CustomMetrics = map[string]interface{}{
		"in_use":              stats.InUse,
		"idle":                stats.Idle,
		"wait_count":          stats.WaitCount,
		"wait_duration_ms":    stats.WaitDuration.Milliseconds(),
		"max_idle_closed":     stats.MaxIdleClosed,
		"max_lifetime_closed": stats.MaxLifetimeClosed,
	}
	metrics := s.metrics
	s.mu.Unlock()

	return metrics, nil
}

// GetConfig returns the service configuration.
func (s *PostgresService) GetConfig() *registry.ServiceConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.config
}

// UpdateConfig updates the service configuration.
func (s *PostgresService) UpdateConfig(config *registry.ServiceConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	s.mu.Lock()
	s.config = config
	s.health.ServiceID = config.ID
	s.health.Status = registry.StatusInitializing
	s.metrics.ServiceID = config.ID
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.Disconnect(ctx); err != nil {
		return err
	}

	return s.Connect(ctx)
}

// Type returns the service type.
func (s *PostgresService) Type() registry.ServiceType {
	return registry.ServiceTypeDatabase
}

// Provider returns the service provider.
func (s *PostgresService) Provider() registry.ServiceProvider {
	return registry.ProviderPostgres
}

// ID returns the service ID.
func (s *PostgresService) ID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.config == nil {
		return ""
	}

	return s.config.ID
}

// Query executes a query that returns rows.
func (s *PostgresService) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	start := time.Now()
	rows, err := db.QueryContext(ctx, query, args...)
	duration := time.Since(start)

	s.mu.Lock()
	s.metrics.Requests++
	s.metrics.Latency = duration
	if err != nil {
		s.metrics.Errors++
	}
	s.mu.Unlock()

	return rows, err
}

// QueryRow executes a query that returns a single row.
func (s *PostgresService) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db == nil {
		return nil
	}

	s.mu.Lock()
	s.metrics.Requests++
	s.mu.Unlock()

	return db.QueryRowContext(ctx, query, args...)
}

// Exec executes a query that doesn't return rows.
func (s *PostgresService) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	start := time.Now()
	result, err := db.ExecContext(ctx, query, args...)
	duration := time.Since(start)

	s.mu.Lock()
	s.metrics.Requests++
	s.metrics.Latency = duration
	if err != nil {
		s.metrics.Errors++
	}
	s.mu.Unlock()

	return result, err
}

// BeginTx starts a database transaction.
func (s *PostgresService) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	return db.BeginTx(ctx, opts)
}

// GetDB returns the underlying database connection.
func (s *PostgresService) GetDB() *sql.DB {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.db
}

// SetMaxOpenConns sets the maximum number of open connections.
func (s *PostgresService) SetMaxOpenConns(n int) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db != nil {
		db.SetMaxOpenConns(n)
	}
}

// SetMaxIdleConns sets the maximum number of idle connections.
func (s *PostgresService) SetMaxIdleConns(n int) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db != nil {
		db.SetMaxIdleConns(n)
	}
}

// SetConnMaxLifetime sets the maximum connection lifetime.
func (s *PostgresService) SetConnMaxLifetime(d time.Duration) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db != nil {
		db.SetConnMaxLifetime(d)
	}
}

// RunMigrations runs database migrations.
func (s *PostgresService) RunMigrations(ctx context.Context, migrationsPath string) error {
	// This would integrate with a migration tool like golang-migrate
	// For now, return not implemented
	return fmt.Errorf("migrations not implemented")
}

// GetSchemaVersion returns the current schema version.
func (s *PostgresService) GetSchemaVersion() (int, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db == nil {
		return 0, fmt.Errorf("database not connected")
	}

	var version int
	err := db.QueryRow("SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1").Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

// Backup creates a database backup.
func (s *PostgresService) Backup(ctx context.Context, path string) error {
	// This would use pg_dump or similar
	// For now, return not implemented
	return fmt.Errorf("backup not implemented")
}

// Restore restores from a backup.
func (s *PostgresService) Restore(ctx context.Context, path string) error {
	// This would use pg_restore or similar
	// For now, return not implemented
	return fmt.Errorf("restore not implemented")
}

// buildConnectionString builds the PostgreSQL connection string.
func buildPostgresConnectionString(config *registry.ServiceConfig) string {
	if config == nil {
		return ""
	}

	sslMode := "disable"
	if config.TLS {
		sslMode = "require"
	}

	if config.Options != nil {
		if mode, ok := config.Options["sslmode"].(string); ok {
			sslMode = mode
		}
	}

	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host,
		config.Port,
		config.Username,
		config.Password,
		config.Database,
		sslMode,
	)
}
