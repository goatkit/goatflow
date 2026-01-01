package cache

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// QueryCache implements caching for database query results.
type QueryCache struct {
	cache         *RedisCache
	defaultTTL    time.Duration
	invalidations map[string][]string // table -> cache key patterns
}

// QueryCacheConfig defines query cache configuration.
type QueryCacheConfig struct {
	RedisCache *RedisCache
	DefaultTTL time.Duration
}

// QueryResult represents a cached query result.
type QueryResult struct {
	Query     string        `json:"query"`
	Args      []interface{} `json:"args"`
	Result    interface{}   `json:"result"`
	Count     int           `json:"count"`
	CachedAt  time.Time     `json:"cached_at"`
	ExpiresAt time.Time     `json:"expires_at"`
}

// NewQueryCache creates a new query cache instance.
func NewQueryCache(config *QueryCacheConfig) *QueryCache {
	qc := &QueryCache{
		cache:         config.RedisCache,
		defaultTTL:    config.DefaultTTL,
		invalidations: make(map[string][]string),
	}

	// Register invalidation patterns for common tables
	qc.registerInvalidations()

	return qc
}

// Get retrieves a cached query result.
func (qc *QueryCache) Get(ctx context.Context, query string, args ...interface{}) (*QueryResult, error) {
	key := qc.buildQueryKey(query, args...)

	var result QueryResult
	err := qc.cache.GetObject(ctx, key, &result)
	if err != nil {
		return nil, err
	}

	if result.Query == "" {
		return nil, nil // Cache miss
	}

	// Check if still valid
	if time.Now().After(result.ExpiresAt) {
		qc.cache.Delete(ctx, key)
		return nil, nil
	}

	return &result, nil
}

// Set stores a query result in cache.
func (qc *QueryCache) Set(ctx context.Context, query string, result interface{}, args ...interface{}) error {
	key := qc.buildQueryKey(query, args...)

	// Determine TTL based on query type
	ttl := qc.determineTTL(query)

	cacheResult := &QueryResult{
		Query:     query,
		Args:      args,
		Result:    result,
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(ttl),
	}

	// Count results if it's a slice
	if countable, ok := result.([]interface{}); ok {
		cacheResult.Count = len(countable)
	}

	return qc.cache.SetObject(ctx, key, cacheResult, ttl)
}

// InvalidateTable invalidates all queries related to a table.
func (qc *QueryCache) InvalidateTable(ctx context.Context, table string) error {
	patterns, exists := qc.invalidations[table]
	if !exists {
		// Default pattern for unknown tables
		patterns = []string{
			fmt.Sprintf("query:*%s*", table),
		}
	}

	for _, pattern := range patterns {
		if err := qc.cache.Invalidate(ctx, pattern); err != nil {
			return err
		}
	}

	return nil
}

// InvalidateAll clears all query cache.
func (qc *QueryCache) InvalidateAll(ctx context.Context) error {
	return qc.cache.Invalidate(ctx, "query:*")
}

// GetOrSet attempts to get from cache, otherwise executes and caches.
func (qc *QueryCache) GetOrSet(ctx context.Context, query string, args []interface{},
	executor func() (interface{}, error)) (interface{}, error) {
	// Try to get from cache
	cached, err := qc.Get(ctx, query, args...)
	if err == nil && cached != nil {
		return cached.Result, nil
	}

	// Execute query
	result, err := executor()
	if err != nil {
		return nil, err
	}

	// Cache the result
	qc.Set(ctx, query, result, args...)

	return result, nil
}

// WarmUp pre-loads commonly used queries.
func (qc *QueryCache) WarmUp(ctx context.Context, queries []WarmUpQuery) error {
	for _, wq := range queries {
		if wq.Executor == nil {
			continue
		}

		result, err := wq.Executor()
		if err != nil {
			// Log error but continue warming up other queries
			continue
		}

		if err := qc.Set(ctx, wq.Query, result, wq.Args...); err != nil {
			// Log error but continue
			continue
		}
	}

	return nil
}

// WarmUpQuery defines a query to warm up.
type WarmUpQuery struct {
	Query    string
	Args     []interface{}
	Executor func() (interface{}, error)
}

// GetStats returns cache statistics.
func (qc *QueryCache) GetStats(ctx context.Context) (map[string]interface{}, error) {
	info, err := qc.cache.Info(ctx)
	if err != nil {
		return nil, err
	}

	// Count query keys
	var queryCount int64
	_ = qc.cache.Pipeline(ctx, func(pipe redis.Pipeliner) error {
		// This is a simplification - in production, use SCAN
		return nil
	})

	stats := map[string]interface{}{
		"redis_info":  info,
		"query_count": queryCount,
		"default_ttl": qc.defaultTTL.String(),
	}

	return stats, nil
}

// Helper functions

func (qc *QueryCache) buildQueryKey(query string, args ...interface{}) string {
	// Normalize query (remove extra spaces, lowercase)
	normalized := strings.ToLower(strings.TrimSpace(query))
	normalized = strings.ReplaceAll(normalized, "  ", " ")

	// Create hash of query + args
	hasher := md5.New()
	hasher.Write([]byte(normalized))

	// Add args to hash
	for _, arg := range args {
		argBytes, _ := json.Marshal(arg)
		hasher.Write(argBytes)
	}

	hash := hex.EncodeToString(hasher.Sum(nil))

	// Extract table name for better key organization
	table := qc.extractTableName(normalized)

	return fmt.Sprintf("query:%s:%s", table, hash)
}

func (qc *QueryCache) extractTableName(query string) string {
	query = strings.ToLower(query)

	// Try to extract table name from common patterns
	patterns := []string{
		"from ",
		"update ",
		"insert into ",
		"delete from ",
	}

	for _, pattern := range patterns {
		if idx := strings.Index(query, pattern); idx != -1 {
			remaining := query[idx+len(pattern):]
			// Extract table name (until space or special char)
			for i, ch := range remaining {
				if ch == ' ' || ch == '(' || ch == ',' || ch == ';' {
					return remaining[:i]
				}
			}
		}
	}

	return "unknown"
}

func (qc *QueryCache) determineTTL(query string) time.Duration {
	query = strings.ToLower(query)

	// Short TTL for frequently changing data
	if strings.Contains(query, "ticket") && !strings.Contains(query, "ticket_history") {
		return 30 * time.Second
	}

	// Medium TTL for user data
	if strings.Contains(query, "users") || strings.Contains(query, "customer") {
		return 5 * time.Minute
	}

	// Long TTL for configuration/static data
	if strings.Contains(query, "queue") || strings.Contains(query, "role") ||
		strings.Contains(query, "config") || strings.Contains(query, "template") {
		return 30 * time.Minute
	}

	// Historical data can be cached longer
	if strings.Contains(query, "history") || strings.Contains(query, "log") {
		return 1 * time.Hour
	}

	// Default TTL
	return qc.defaultTTL
}

func (qc *QueryCache) registerInvalidations() {
	// Define which cache patterns to invalidate when tables change

	// Ticket changes affect many queries
	qc.invalidations["ticket"] = []string{
		"query:ticket:*",
		"query:article:*",   // Articles are related to tickets
		"query:queue:*",     // Queue counts change
		"query:dashboard:*", // Dashboard queries
		"query:search:*",    // Search results
	}

	// Article changes
	qc.invalidations["article"] = []string{
		"query:article:*",
		"query:ticket:*", // Ticket details include articles
		"query:search:*",
	}

	// User changes
	qc.invalidations["users"] = []string{
		"query:users:*",
		"query:customer_user:*",
		"query:auth:*",
		"query:session:*",
	}

	// Queue changes
	qc.invalidations["queue"] = []string{
		"query:queue:*",
		"query:ticket:*", // Tickets belong to queues
		"query:dashboard:*",
	}

	// Customer changes
	qc.invalidations["customer_user"] = []string{
		"query:customer_user:*",
		"query:customer_company:*",
		"query:ticket:*", // Customer tickets
	}

	// Role/permission changes
	qc.invalidations["role_user"] = []string{
		"query:users:*",
		"query:role:*",
		"query:permission:*",
		"query:auth:*",
	}

	// Configuration changes
	qc.invalidations["system_data"] = []string{
		"query:config:*",
		"query:system:*",
		"query:settings:*",
	}
}

// CacheableQuery wraps a database query for automatic caching.
type CacheableQuery struct {
	cache    *QueryCache
	query    string
	args     []interface{}
	ttl      time.Duration
	executor func() (interface{}, error)
}

// NewCacheableQuery creates a new cacheable query.
func (qc *QueryCache) NewCacheableQuery(query string, args []interface{}) *CacheableQuery {
	return &CacheableQuery{
		cache: qc,
		query: query,
		args:  args,
		ttl:   qc.defaultTTL,
	}
}

// WithTTL sets a custom TTL for this query.
func (cq *CacheableQuery) WithTTL(ttl time.Duration) *CacheableQuery {
	cq.ttl = ttl
	return cq
}

// WithExecutor sets the query executor.
func (cq *CacheableQuery) WithExecutor(executor func() (interface{}, error)) *CacheableQuery {
	cq.executor = executor
	return cq
}

// Execute runs the query with caching.
func (cq *CacheableQuery) Execute(ctx context.Context) (interface{}, error) {
	if cq.executor == nil {
		return nil, fmt.Errorf("no executor defined for cacheable query")
	}

	return cq.cache.GetOrSet(ctx, cq.query, cq.args, cq.executor)
}

// Invalidate removes this specific query from cache.
func (cq *CacheableQuery) Invalidate(ctx context.Context) error {
	key := cq.cache.buildQueryKey(cq.query, cq.args...)
	return cq.cache.cache.Delete(ctx, key)
}
