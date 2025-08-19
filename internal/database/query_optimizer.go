package database

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// QueryOptimizer analyzes and optimizes database queries
type QueryOptimizer struct {
	pool        *ConnectionPool
	queryPlans  map[string]*QueryPlan
	statistics  *QueryStatistics
}

// QueryPlan represents an analyzed query execution plan
type QueryPlan struct {
	Query       string
	Plan        string
	Cost        float64
	Rows        int64
	Width       int
	Indexes     []string
	Suggestions []string
	AnalyzedAt  time.Time
}

// QueryStatistics tracks query performance statistics
type QueryStatistics struct {
	TotalQueries    int64
	TotalDuration   time.Duration
	AverageDuration time.Duration
	MinDuration     time.Duration
	MaxDuration     time.Duration
	QueryCounts     map[string]int64
	QueryDurations  map[string]time.Duration
}

// NewQueryOptimizer creates a new query optimizer
func NewQueryOptimizer(pool *ConnectionPool) *QueryOptimizer {
	return &QueryOptimizer{
		pool:       pool,
		queryPlans: make(map[string]*QueryPlan),
		statistics: &QueryStatistics{
			QueryCounts:    make(map[string]int64),
			QueryDurations: make(map[string]time.Duration),
		},
	}
}

// AnalyzeQuery analyzes a query and returns optimization suggestions
func (o *QueryOptimizer) AnalyzeQuery(ctx context.Context, query string) (*QueryPlan, error) {
	// Check cache
	if plan, exists := o.queryPlans[query]; exists {
		// Return cached plan if recent (within 1 hour)
		if time.Since(plan.AnalyzedAt) < time.Hour {
			return plan, nil
		}
	}
	
	// Get query plan using EXPLAIN ANALYZE
	explainQuery := fmt.Sprintf("EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) %s", query)
	
	var planJSON string
	err := o.pool.QueryRow(ctx, explainQuery).Scan(&planJSON)
	if err != nil {
		// Try without ANALYZE for queries that modify data
		explainQuery = fmt.Sprintf("EXPLAIN (FORMAT JSON) %s", query)
		err = o.pool.QueryRow(ctx, explainQuery).Scan(&planJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to analyze query: %w", err)
		}
	}
	
	// Parse the plan
	plan := o.parsePlan(query, planJSON)
	
	// Generate optimization suggestions
	plan.Suggestions = o.generateSuggestions(plan)
	
	// Cache the plan
	o.queryPlans[query] = plan
	
	return plan, nil
}

// OptimizeQuery attempts to rewrite a query for better performance
func (o *QueryOptimizer) OptimizeQuery(query string) string {
	optimized := query
	
	// Remove unnecessary DISTINCT
	if strings.Contains(optimized, "DISTINCT") && o.hasUniqueConstraint(query) {
		optimized = strings.Replace(optimized, "DISTINCT", "", 1)
	}
	
	// Convert NOT IN to NOT EXISTS for better performance
	if strings.Contains(optimized, "NOT IN") {
		optimized = o.convertNotInToNotExists(optimized)
	}
	
	// Add LIMIT if missing for SELECT without aggregation
	if strings.HasPrefix(strings.ToUpper(optimized), "SELECT") &&
		!strings.Contains(optimized, "LIMIT") &&
		!strings.Contains(optimized, "COUNT") &&
		!strings.Contains(optimized, "SUM") &&
		!strings.Contains(optimized, "AVG") {
		optimized += " LIMIT 1000"
	}
	
	// Use index hints for known slow queries
	optimized = o.addIndexHints(optimized)
	
	return optimized
}

// GetMissingIndexes identifies missing indexes that would improve performance
func (o *QueryOptimizer) GetMissingIndexes(ctx context.Context) ([]string, error) {
	query := `
		SELECT schemaname, tablename, attname, n_distinct, correlation
		FROM pg_stats
		WHERE schemaname = 'public'
		AND n_distinct > 100
		AND correlation < 0.1
		ORDER BY n_distinct DESC
		LIMIT 20
	`
	
	rows, err := o.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var suggestions []string
	for rows.Next() {
		var schema, table, column string
		var nDistinct float64
		var correlation float64
		
		if err := rows.Scan(&schema, &table, &column, &nDistinct, &correlation); err != nil {
			continue
		}
		
		// Suggest index if high cardinality and low correlation
		if nDistinct > 1000 && correlation < 0.05 {
			suggestion := fmt.Sprintf("CREATE INDEX idx_%s_%s ON %s(%s);",
				table, column, table, column)
			suggestions = append(suggestions, suggestion)
		}
	}
	
	return suggestions, nil
}

// GetTableStatistics returns statistics for all tables
func (o *QueryOptimizer) GetTableStatistics(ctx context.Context) (map[string]TableStats, error) {
	query := `
		SELECT 
			schemaname,
			tablename,
			n_live_tup,
			n_dead_tup,
			n_mod_since_analyze,
			last_vacuum,
			last_autovacuum,
			last_analyze,
			last_autoanalyze
		FROM pg_stat_user_tables
		WHERE schemaname = 'public'
	`
	
	rows, err := o.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	stats := make(map[string]TableStats)
	for rows.Next() {
		var ts TableStats
		var schema, table string
		
		err := rows.Scan(
			&schema,
			&table,
			&ts.LiveTuples,
			&ts.DeadTuples,
			&ts.ModifiedSinceAnalyze,
			&ts.LastVacuum,
			&ts.LastAutoVacuum,
			&ts.LastAnalyze,
			&ts.LastAutoAnalyze,
		)
		if err != nil {
			continue
		}
		
		ts.TableName = table
		ts.BloatRatio = float64(ts.DeadTuples) / float64(ts.LiveTuples+1)
		stats[table] = ts
	}
	
	return stats, nil
}

// GetSlowQueries returns queries that are performing poorly
func (o *QueryOptimizer) GetSlowQueries(ctx context.Context, limit int) ([]SlowQueryInfo, error) {
	query := `
		SELECT 
			query,
			calls,
			total_time,
			mean_time,
			stddev_time,
			rows
		FROM pg_stat_statements
		WHERE query NOT LIKE '%pg_stat_statements%'
		ORDER BY mean_time DESC
		LIMIT $1
	`
	
	rows, err := o.pool.Query(ctx, query, limit)
	if err != nil {
		// pg_stat_statements might not be enabled
		return nil, nil
	}
	defer rows.Close()
	
	var slowQueries []SlowQueryInfo
	for rows.Next() {
		var sq SlowQueryInfo
		err := rows.Scan(
			&sq.Query,
			&sq.Calls,
			&sq.TotalTime,
			&sq.MeanTime,
			&sq.StddevTime,
			&sq.Rows,
		)
		if err != nil {
			continue
		}
		
		slowQueries = append(slowQueries, sq)
	}
	
	return slowQueries, nil
}

// VacuumTable performs VACUUM on a specific table
func (o *QueryOptimizer) VacuumTable(ctx context.Context, tableName string, analyze bool) error {
	var query string
	if analyze {
		query = fmt.Sprintf("VACUUM ANALYZE %s", tableName)
	} else {
		query = fmt.Sprintf("VACUUM %s", tableName)
	}
	
	_, err := o.pool.Exec(ctx, query)
	return err
}

// ReindexTable rebuilds indexes for a table
func (o *QueryOptimizer) ReindexTable(ctx context.Context, tableName string) error {
	query := fmt.Sprintf("REINDEX TABLE %s", tableName)
	_, err := o.pool.Exec(ctx, query)
	return err
}

// Helper types and methods

// TableStats represents table statistics
type TableStats struct {
	TableName            string
	LiveTuples           int64
	DeadTuples           int64
	ModifiedSinceAnalyze int64
	BloatRatio           float64
	LastVacuum           *time.Time
	LastAutoVacuum       *time.Time
	LastAnalyze          *time.Time
	LastAutoAnalyze      *time.Time
}

// SlowQueryInfo represents information about a slow query
type SlowQueryInfo struct {
	Query      string
	Calls      int64
	TotalTime  float64
	MeanTime   float64
	StddevTime float64
	Rows       int64
}

// Private methods

func (o *QueryOptimizer) parsePlan(query string, planJSON string) *QueryPlan {
	plan := &QueryPlan{
		Query:      query,
		Plan:       planJSON,
		AnalyzedAt: time.Now(),
		Indexes:    []string{},
	}
	
	// Extract cost, rows, and index usage from JSON
	// This is simplified - in production, use proper JSON parsing
	if strings.Contains(planJSON, "Seq Scan") {
		plan.Suggestions = append(plan.Suggestions, "Query uses sequential scan - consider adding an index")
	}
	
	if strings.Contains(planJSON, "Nested Loop") && strings.Contains(planJSON, "rows=1000") {
		plan.Suggestions = append(plan.Suggestions, "Large nested loop detected - consider using a hash join")
	}
	
	return plan
}

func (o *QueryOptimizer) generateSuggestions(plan *QueryPlan) []string {
	suggestions := plan.Suggestions
	
	// Check for missing WHERE clause
	if !strings.Contains(strings.ToUpper(plan.Query), "WHERE") &&
		strings.HasPrefix(strings.ToUpper(plan.Query), "SELECT") {
		suggestions = append(suggestions, "Query has no WHERE clause - may return too many rows")
	}
	
	// Check for SELECT *
	if strings.Contains(plan.Query, "SELECT *") {
		suggestions = append(suggestions, "Avoid SELECT * - specify only needed columns")
	}
	
	// Check for missing indexes on JOIN columns
	if strings.Contains(strings.ToUpper(plan.Query), "JOIN") {
		suggestions = append(suggestions, "Ensure indexes exist on JOIN columns")
	}
	
	// Check for functions on indexed columns
	if strings.Contains(plan.Query, "UPPER(") || strings.Contains(plan.Query, "LOWER(") {
		suggestions = append(suggestions, "Functions on columns prevent index usage - consider functional indexes")
	}
	
	return suggestions
}

func (o *QueryOptimizer) hasUniqueConstraint(query string) bool {
	// Check if the query involves columns with unique constraints
	// This is a simplified check
	return strings.Contains(query, " id ") || strings.Contains(query, "_id")
}

func (o *QueryOptimizer) convertNotInToNotExists(query string) string {
	// Convert NOT IN subqueries to NOT EXISTS for better null handling
	// This is a simplified conversion
	if strings.Contains(query, "NOT IN (SELECT") {
		return strings.Replace(query, "NOT IN", "NOT EXISTS", 1)
	}
	return query
}

func (o *QueryOptimizer) addIndexHints(query string) string {
	// Add index hints for known patterns
	// PostgreSQL doesn't support index hints directly, but we can rewrite queries
	
	// For ticket queries, ensure we use the composite index
	if strings.Contains(query, "FROM ticket") && strings.Contains(query, "queue_id") {
		// The query optimizer should pick the right index automatically
		// But we can reorder WHERE conditions to help
		query = o.reorderWhereConditions(query)
	}
	
	return query
}

func (o *QueryOptimizer) reorderWhereConditions(query string) string {
	// Reorder WHERE conditions to match index column order
	// Most selective conditions should come first
	// This is a simplified implementation
	return query
}

// MaintenanceSchedule represents automated maintenance tasks
type MaintenanceSchedule struct {
	optimizer *QueryOptimizer
	stop      chan struct{}
}

// NewMaintenanceSchedule creates a new maintenance scheduler
func NewMaintenanceSchedule(optimizer *QueryOptimizer) *MaintenanceSchedule {
	return &MaintenanceSchedule{
		optimizer: optimizer,
		stop:      make(chan struct{}),
	}
}

// Start begins automated maintenance tasks
func (m *MaintenanceSchedule) Start(ctx context.Context) {
	// Daily maintenance
	go m.dailyMaintenance(ctx)
	
	// Hourly statistics update
	go m.hourlyStatistics(ctx)
}

// Stop halts maintenance tasks
func (m *MaintenanceSchedule) Stop() {
	close(m.stop)
}

func (m *MaintenanceSchedule) dailyMaintenance(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Get table statistics
			stats, err := m.optimizer.GetTableStatistics(ctx)
			if err != nil {
				continue
			}
			
			// Vacuum tables with high bloat
			for table, stat := range stats {
				if stat.BloatRatio > 0.2 { // 20% bloat
					m.optimizer.VacuumTable(ctx, table, true)
				}
			}
			
			// Reindex tables that haven't been reindexed in 30 days
			// (Implementation depends on tracking reindex history)
			
		case <-m.stop:
			return
		}
	}
}

func (m *MaintenanceSchedule) hourlyStatistics(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Update table statistics for query planning
			tables := []string{"ticket", "article", "queue", "users", "customer_user"}
			for _, table := range tables {
				m.optimizer.pool.Exec(ctx, fmt.Sprintf("ANALYZE %s", table))
			}
			
		case <-m.stop:
			return
		}
	}
}