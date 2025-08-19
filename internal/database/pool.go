package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PoolConfig defines database connection pool configuration
type PoolConfig struct {
	// Connection settings
	Host     string
	Port     int
	Database string
	Username string
	Password string
	SSLMode  string
	
	// Pool settings
	MaxOpenConns        int
	MaxIdleConns        int
	ConnMaxLifetime     time.Duration
	ConnMaxIdleTime     time.Duration
	
	// Health check settings
	HealthCheckInterval time.Duration
	
	// Query settings
	DefaultTimeout      time.Duration
	SlowQueryThreshold  time.Duration
	
	// Retry settings
	MaxRetries          int
	RetryBackoff        time.Duration
}

// ConnectionPool manages database connections with monitoring
type ConnectionPool struct {
	db              *sql.DB
	config          *PoolConfig
	metrics         *PoolMetrics
	slowQueryLog    []SlowQuery
	slowQueryMutex  sync.RWMutex
	healthCheckStop chan struct{}
}

// PoolMetrics tracks connection pool performance
type PoolMetrics struct {
	activeConnections   prometheus.Gauge
	idleConnections     prometheus.Gauge
	waitCount          prometheus.Counter
	waitDuration       prometheus.Histogram
	maxIdleClosed      prometheus.Counter
	maxLifetimeClosed  prometheus.Counter
	queryDuration      prometheus.Histogram
	queryErrors        prometheus.Counter
	slowQueries        prometheus.Counter
	transactions       prometheus.Counter
	rollbacks          prometheus.Counter
	commits            prometheus.Counter
}

// SlowQuery represents a slow database query
type SlowQuery struct {
	Query     string
	Duration  time.Duration
	Timestamp time.Time
	Error     error
}

// NewConnectionPool creates a new database connection pool
func NewConnectionPool(config *PoolConfig) (*ConnectionPool, error) {
	// Build connection string
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.Username, config.Password, config.Database, config.SSLMode)
	
	// Open database connection
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	
	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)
	
	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	
	// Initialize metrics
	metrics := &PoolMetrics{
		activeConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "db_pool_active_connections",
			Help: "Number of active database connections",
		}),
		idleConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "db_pool_idle_connections",
			Help: "Number of idle database connections",
		}),
		waitCount: promauto.NewCounter(prometheus.CounterOpts{
			Name: "db_pool_wait_count_total",
			Help: "Total number of waits for a connection",
		}),
		waitDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "db_pool_wait_duration_seconds",
			Help:    "Time spent waiting for a connection",
			Buckets: prometheus.DefBuckets,
		}),
		maxIdleClosed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "db_pool_max_idle_closed_total",
			Help: "Total connections closed due to max idle",
		}),
		maxLifetimeClosed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "db_pool_max_lifetime_closed_total",
			Help: "Total connections closed due to max lifetime",
		}),
		queryDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query duration",
			Buckets: prometheus.DefBuckets,
		}),
		queryErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "db_query_errors_total",
			Help: "Total number of query errors",
		}),
		slowQueries: promauto.NewCounter(prometheus.CounterOpts{
			Name: "db_slow_queries_total",
			Help: "Total number of slow queries",
		}),
		transactions: promauto.NewCounter(prometheus.CounterOpts{
			Name: "db_transactions_total",
			Help: "Total number of database transactions",
		}),
		rollbacks: promauto.NewCounter(prometheus.CounterOpts{
			Name: "db_rollbacks_total",
			Help: "Total number of transaction rollbacks",
		}),
		commits: promauto.NewCounter(prometheus.CounterOpts{
			Name: "db_commits_total",
			Help: "Total number of transaction commits",
		}),
	}
	
	pool := &ConnectionPool{
		db:              db,
		config:          config,
		metrics:         metrics,
		slowQueryLog:    make([]SlowQuery, 0),
		healthCheckStop: make(chan struct{}),
	}
	
	// Start health check routine
	go pool.healthCheckLoop()
	
	// Start metrics collection
	go pool.collectMetrics()
	
	return pool, nil
}

// Query executes a query with monitoring
func (p *ConnectionPool) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return p.queryWithRetry(ctx, query, args...)
}

// QueryRow executes a query returning a single row
func (p *ConnectionPool) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	timer := prometheus.NewTimer(p.metrics.queryDuration)
	defer timer.ObserveDuration()
	
	start := time.Now()
	row := p.db.QueryRowContext(ctx, query, args...)
	
	// Log slow queries
	duration := time.Since(start)
	if duration > p.config.SlowQueryThreshold {
		p.logSlowQuery(query, duration, nil)
	}
	
	return row
}

// Exec executes a query without returning rows
func (p *ConnectionPool) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return p.execWithRetry(ctx, query, args...)
}

// Begin starts a new transaction
func (p *ConnectionPool) Begin(ctx context.Context) (*Transaction, error) {
	p.metrics.transactions.Inc()
	
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	
	return &Transaction{
		tx:      tx,
		pool:    p,
		started: time.Now(),
	}, nil
}

// Transaction wraps a database transaction with monitoring
type Transaction struct {
	tx      *sql.Tx
	pool    *ConnectionPool
	started time.Time
}

// Commit commits the transaction
func (t *Transaction) Commit() error {
	t.pool.metrics.commits.Inc()
	return t.tx.Commit()
}

// Rollback rolls back the transaction
func (t *Transaction) Rollback() error {
	t.pool.metrics.rollbacks.Inc()
	return t.tx.Rollback()
}

// Query executes a query within the transaction
func (t *Transaction) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	timer := prometheus.NewTimer(t.pool.metrics.queryDuration)
	defer timer.ObserveDuration()
	
	start := time.Now()
	rows, err := t.tx.QueryContext(ctx, query, args...)
	
	duration := time.Since(start)
	if duration > t.pool.config.SlowQueryThreshold {
		t.pool.logSlowQuery(query, duration, err)
	}
	
	if err != nil {
		t.pool.metrics.queryErrors.Inc()
	}
	
	return rows, err
}

// QueryRow executes a query returning a single row within the transaction
func (t *Transaction) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	timer := prometheus.NewTimer(t.pool.metrics.queryDuration)
	defer timer.ObserveDuration()
	
	start := time.Now()
	row := t.tx.QueryRowContext(ctx, query, args...)
	
	duration := time.Since(start)
	if duration > t.pool.config.SlowQueryThreshold {
		t.pool.logSlowQuery(query, duration, nil)
	}
	
	return row
}

// Exec executes a query without returning rows within the transaction
func (t *Transaction) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	timer := prometheus.NewTimer(t.pool.metrics.queryDuration)
	defer timer.ObserveDuration()
	
	start := time.Now()
	result, err := t.tx.ExecContext(ctx, query, args...)
	
	duration := time.Since(start)
	if duration > t.pool.config.SlowQueryThreshold {
		t.pool.logSlowQuery(query, duration, err)
	}
	
	if err != nil {
		t.pool.metrics.queryErrors.Inc()
	}
	
	return result, err
}

// GetStats returns connection pool statistics
func (p *ConnectionPool) GetStats() sql.DBStats {
	return p.db.Stats()
}

// GetSlowQueries returns recent slow queries
func (p *ConnectionPool) GetSlowQueries() []SlowQuery {
	p.slowQueryMutex.RLock()
	defer p.slowQueryMutex.RUnlock()
	
	// Return copy to avoid race conditions
	queries := make([]SlowQuery, len(p.slowQueryLog))
	copy(queries, p.slowQueryLog)
	return queries
}

// Close closes the connection pool
func (p *ConnectionPool) Close() error {
	close(p.healthCheckStop)
	return p.db.Close()
}

// Private methods

func (p *ConnectionPool) queryWithRetry(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	var rows *sql.Rows
	var err error
	
	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		timer := prometheus.NewTimer(p.metrics.queryDuration)
		start := time.Now()
		
		rows, err = p.db.QueryContext(ctx, query, args...)
		
		duration := time.Since(start)
		timer.ObserveDuration()
		
		if duration > p.config.SlowQueryThreshold {
			p.logSlowQuery(query, duration, err)
		}
		
		if err == nil {
			return rows, nil
		}
		
		// Check if error is retryable
		if !isRetryableError(err) {
			p.metrics.queryErrors.Inc()
			return nil, err
		}
		
		// Wait before retry
		if attempt < p.config.MaxRetries {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(p.config.RetryBackoff * time.Duration(attempt+1)):
				// Continue to next attempt
			}
		}
	}
	
	p.metrics.queryErrors.Inc()
	return nil, fmt.Errorf("query failed after %d retries: %w", p.config.MaxRetries, err)
}

func (p *ConnectionPool) execWithRetry(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	var result sql.Result
	var err error
	
	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		timer := prometheus.NewTimer(p.metrics.queryDuration)
		start := time.Now()
		
		result, err = p.db.ExecContext(ctx, query, args...)
		
		duration := time.Since(start)
		timer.ObserveDuration()
		
		if duration > p.config.SlowQueryThreshold {
			p.logSlowQuery(query, duration, err)
		}
		
		if err == nil {
			return result, nil
		}
		
		// Check if error is retryable
		if !isRetryableError(err) {
			p.metrics.queryErrors.Inc()
			return nil, err
		}
		
		// Wait before retry
		if attempt < p.config.MaxRetries {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(p.config.RetryBackoff * time.Duration(attempt+1)):
				// Continue to next attempt
			}
		}
	}
	
	p.metrics.queryErrors.Inc()
	return nil, fmt.Errorf("exec failed after %d retries: %w", p.config.MaxRetries, err)
}

func (p *ConnectionPool) logSlowQuery(query string, duration time.Duration, err error) {
	p.metrics.slowQueries.Inc()
	
	p.slowQueryMutex.Lock()
	defer p.slowQueryMutex.Unlock()
	
	// Keep only last 100 slow queries
	if len(p.slowQueryLog) >= 100 {
		p.slowQueryLog = p.slowQueryLog[1:]
	}
	
	p.slowQueryLog = append(p.slowQueryLog, SlowQuery{
		Query:     query,
		Duration:  duration,
		Timestamp: time.Now(),
		Error:     err,
	})
}

func (p *ConnectionPool) healthCheckLoop() {
	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := p.db.PingContext(ctx); err != nil {
				// Log health check failure
				fmt.Printf("Database health check failed: %v\n", err)
			}
			cancel()
			
		case <-p.healthCheckStop:
			return
		}
	}
}

func (p *ConnectionPool) collectMetrics() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			stats := p.db.Stats()
			p.metrics.activeConnections.Set(float64(stats.InUse))
			p.metrics.idleConnections.Set(float64(stats.Idle))
			p.metrics.waitCount.Add(float64(stats.WaitCount))
			p.metrics.maxIdleClosed.Add(float64(stats.MaxIdleClosed))
			p.metrics.maxLifetimeClosed.Add(float64(stats.MaxLifetimeClosed))
			
		case <-p.healthCheckStop:
			return
		}
	}
}

// isRetryableError checks if an error is retryable
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	// Check for common retryable database errors
	errStr := err.Error()
	retryableErrors := []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"deadline exceeded",
		"timeout",
		"too many connections",
	}
	
	for _, retryable := range retryableErrors {
		if contains(errStr, retryable) {
			return true
		}
	}
	
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[0:len(substr)] == substr
}