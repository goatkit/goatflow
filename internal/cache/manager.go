package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Manager provides a high-level caching interface with multiple strategies.
type Manager struct {
	redis      *RedisCache
	localCache *LocalCache
	strategies map[string]CacheStrategy
	mu         sync.RWMutex
	config     *ManagerConfig
}

// ManagerConfig defines cache manager configuration.
type ManagerConfig struct {
	RedisConfig *CacheConfig
	LocalConfig *LocalCacheConfig

	// Strategy settings
	DefaultStrategy string
	EnableLocal     bool
	EnableRedis     bool

	// Cache warming
	WarmOnStartup   bool
	WarmupFunctions map[string]WarmupFunc
}

// LocalCacheConfig defines local cache settings.
type LocalCacheConfig struct {
	MaxSize         int
	DefaultTTL      time.Duration
	CleanupInterval time.Duration
}

// CacheStrategy defines how data should be cached.
type CacheStrategy interface {
	Get(ctx context.Context, key string) (interface{}, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context, pattern string) error
	GetMulti(ctx context.Context, keys []string) (map[string]interface{}, error)
	SetMulti(ctx context.Context, items map[string]interface{}, ttl time.Duration) error
}

// WarmupFunc is a function that pre-populates cache.
type WarmupFunc func(ctx context.Context, cache *Manager) error

// CacheItem represents a cached item with metadata.
type CacheItem struct {
	Key       string
	Value     interface{}
	TTL       time.Duration
	Tags      []string
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewManager creates a new cache manager.
func NewManager(config *ManagerConfig) (*Manager, error) {
	m := &Manager{
		config:     config,
		strategies: make(map[string]CacheStrategy),
	}

	// Initialize Redis cache if enabled
	if config.EnableRedis && config.RedisConfig != nil {
		redisCache, err := NewRedisCache(config.RedisConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create Redis cache: %w", err)
		}
		m.redis = redisCache
	}

	// Initialize local cache if enabled
	if config.EnableLocal && config.LocalConfig != nil {
		m.localCache = NewLocalCache(config.LocalConfig)
	}

	// Setup default strategies
	m.setupStrategies()

	// Warm cache if configured
	if config.WarmOnStartup {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := m.WarmCache(ctx); err != nil {
			// Log error but don't fail initialization
			fmt.Printf("Cache warmup failed: %v\n", err)
		}
	}

	return m, nil
}

// setupStrategies configures caching strategies.
func (m *Manager) setupStrategies() {
	// Write-through strategy: Write to both local and Redis
	m.strategies["write-through"] = &WriteThroughStrategy{
		local: m.localCache,
		redis: m.redis,
	}

	// Write-behind strategy: Write to local immediately, Redis async
	m.strategies["write-behind"] = &WriteBehindStrategy{
		local: m.localCache,
		redis: m.redis,
		queue: make(chan *CacheItem, 1000),
	}

	// Read-through strategy: Check local first, then Redis, then source
	m.strategies["read-through"] = &ReadThroughStrategy{
		local: m.localCache,
		redis: m.redis,
	}

	// Cache-aside strategy: Application manages cache explicitly
	m.strategies["cache-aside"] = &CacheAsideStrategy{
		local: m.localCache,
		redis: m.redis,
	}
}

// GetStrategy returns a caching strategy by name.
func (m *Manager) GetStrategy(name string) CacheStrategy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if strategy, exists := m.strategies[name]; exists {
		return strategy
	}

	// Return default strategy
	if m.config.DefaultStrategy != "" {
		return m.strategies[m.config.DefaultStrategy]
	}

	// Fallback to cache-aside
	return m.strategies["cache-aside"]
}

// Ticket caching methods

// GetTicket retrieves a ticket from cache.
func (m *Manager) GetTicket(ctx context.Context, ticketID int64) (*CachedTicket, error) {
	key := fmt.Sprintf("ticket:%d", ticketID)
	strategy := m.GetStrategy("read-through")

	data, err := strategy.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, nil
	}

	ticket := &CachedTicket{}
	if err := json.Unmarshal(data.([]byte), ticket); err != nil {
		return nil, err
	}

	return ticket, nil
}

// SetTicket caches a ticket.
func (m *Manager) SetTicket(ctx context.Context, ticket *CachedTicket) error {
	key := fmt.Sprintf("ticket:%d", ticket.ID)
	strategy := m.GetStrategy("write-through")

	data, err := json.Marshal(ticket)
	if err != nil {
		return err
	}

	return strategy.Set(ctx, key, data, 5*time.Minute)
}

// InvalidateTicket removes a ticket from cache.
func (m *Manager) InvalidateTicket(ctx context.Context, ticketID int64) error {
	key := fmt.Sprintf("ticket:%d", ticketID)
	strategy := m.GetStrategy("write-through")

	// Also invalidate related caches
	relatedKeys := []string{
		fmt.Sprintf("ticket:%d:articles", ticketID),
		fmt.Sprintf("ticket:%d:attachments", ticketID),
		fmt.Sprintf("ticket:%d:history", ticketID),
	}

	// Delete main key
	if err := strategy.Delete(ctx, key); err != nil {
		return err
	}

	// Delete related keys
	for _, relKey := range relatedKeys {
		strategy.Delete(ctx, relKey)
	}

	return nil
}

// Queue caching methods

// GetQueueTickets retrieves tickets for a queue from cache.
func (m *Manager) GetQueueTickets(ctx context.Context, queueID int, page, limit int) ([]*CachedTicket, error) {
	key := fmt.Sprintf("queue:%d:tickets:page:%d:limit:%d", queueID, page, limit)
	strategy := m.GetStrategy("read-through")

	data, err := strategy.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, nil
	}

	var tickets []*CachedTicket
	if err := json.Unmarshal(data.([]byte), &tickets); err != nil {
		return nil, err
	}

	return tickets, nil
}

// SetQueueTickets caches tickets for a queue.
func (m *Manager) SetQueueTickets(ctx context.Context, queueID int, page, limit int, tickets []*CachedTicket) error {
	key := fmt.Sprintf("queue:%d:tickets:page:%d:limit:%d", queueID, page, limit)
	strategy := m.GetStrategy("write-behind")

	data, err := json.Marshal(tickets)
	if err != nil {
		return err
	}

	// Cache for shorter time as list data changes frequently
	return strategy.Set(ctx, key, data, 1*time.Minute)
}

// User caching methods

// GetUser retrieves a user from cache.
func (m *Manager) GetUser(ctx context.Context, userID int) (*CachedUser, error) {
	key := fmt.Sprintf("user:%d", userID)
	strategy := m.GetStrategy("read-through")

	data, err := strategy.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, nil
	}

	user := &CachedUser{}
	if err := json.Unmarshal(data.([]byte), user); err != nil {
		return nil, err
	}

	return user, nil
}

// SetUser caches a user.
func (m *Manager) SetUser(ctx context.Context, user *CachedUser) error {
	key := fmt.Sprintf("user:%d", user.ID)
	strategy := m.GetStrategy("write-through")

	data, err := json.Marshal(user)
	if err != nil {
		return err
	}

	// Users don't change often, cache for longer
	return strategy.Set(ctx, key, data, 30*time.Minute)
}

// Session caching methods

// GetSession retrieves a session from cache.
func (m *Manager) GetSession(ctx context.Context, sessionID string) (*CachedSession, error) {
	key := fmt.Sprintf("session:%s", sessionID)
	strategy := m.GetStrategy("write-through")

	data, err := strategy.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, nil
	}

	session := &CachedSession{}
	if err := json.Unmarshal(data.([]byte), session); err != nil {
		return nil, err
	}

	return session, nil
}

// SetSession caches a session.
func (m *Manager) SetSession(ctx context.Context, session *CachedSession) error {
	key := fmt.Sprintf("session:%s", session.ID)
	strategy := m.GetStrategy("write-through")

	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	// Sessions expire after inactivity
	return strategy.Set(ctx, key, data, 2*time.Hour)
}

// Search result caching

// GetSearchResults retrieves cached search results.
func (m *Manager) GetSearchResults(ctx context.Context, query string, filters map[string]interface{}) (*CachedSearchResults, error) {
	// Generate cache key from query and filters
	key := m.generateSearchKey(query, filters)
	strategy := m.GetStrategy("cache-aside")

	data, err := strategy.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return nil, nil
	}

	results := &CachedSearchResults{}
	if err := json.Unmarshal(data.([]byte), results); err != nil {
		return nil, err
	}

	return results, nil
}

// SetSearchResults caches search results.
func (m *Manager) SetSearchResults(ctx context.Context, query string, filters map[string]interface{}, results *CachedSearchResults) error {
	key := m.generateSearchKey(query, filters)
	strategy := m.GetStrategy("cache-aside")

	data, err := json.Marshal(results)
	if err != nil {
		return err
	}

	// Search results cached for short time
	return strategy.Set(ctx, key, data, 5*time.Minute)
}

// generateSearchKey creates a cache key from search parameters.
func (m *Manager) generateSearchKey(query string, filters map[string]interface{}) string {
	// Simple implementation - in production use a hash
	filterStr, _ := json.Marshal(filters)
	return fmt.Sprintf("search:%s:%s", query, string(filterStr))
}

// Bulk operations

// GetMultiTickets retrieves multiple tickets from cache.
func (m *Manager) GetMultiTickets(ctx context.Context, ticketIDs []int64) (map[int64]*CachedTicket, error) {
	keys := make([]string, len(ticketIDs))
	for i, id := range ticketIDs {
		keys[i] = fmt.Sprintf("ticket:%d", id)
	}

	strategy := m.GetStrategy("read-through")
	data, err := strategy.GetMulti(ctx, keys)
	if err != nil {
		return nil, err
	}

	tickets := make(map[int64]*CachedTicket)
	for _, id := range ticketIDs {
		key := fmt.Sprintf("ticket:%d", id)
		if val, exists := data[key]; exists && val != nil {
			ticket := &CachedTicket{}
			if err := json.Unmarshal(val.([]byte), ticket); err == nil {
				tickets[id] = ticket
			}
		}
	}

	return tickets, nil
}

// Cache warming

// WarmCache pre-populates cache with frequently accessed data.
func (m *Manager) WarmCache(ctx context.Context) error {
	var wg sync.WaitGroup
	errors := make(chan error, len(m.config.WarmupFunctions))

	for name, fn := range m.config.WarmupFunctions {
		wg.Add(1)
		go func(n string, f WarmupFunc) {
			defer wg.Done()
			if err := f(ctx, m); err != nil {
				errors <- fmt.Errorf("warmup %s failed: %w", n, err)
			}
		}(name, fn)
	}

	wg.Wait()
	close(errors)

	// Collect any errors
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("cache warmup had %d errors: %v", len(errs), errs)
	}

	return nil
}

// Cache invalidation

// InvalidatePattern invalidates all keys matching a pattern.
func (m *Manager) InvalidatePattern(ctx context.Context, pattern string) error {
	strategy := m.GetStrategy("write-through")
	return strategy.Clear(ctx, pattern)
}

// InvalidateAll clears all caches.
func (m *Manager) InvalidateAll(ctx context.Context) error {
	// Clear local cache
	if m.localCache != nil {
		m.localCache.Clear()
	}

	// Clear Redis cache with pattern
	if m.redis != nil {
		return m.InvalidatePattern(ctx, "*")
	}

	return nil
}

// Statistics

// GetStats returns cache statistics.
func (m *Manager) GetStats() *CacheStats {
	stats := &CacheStats{
		Timestamp: time.Now(),
	}

	// Get local cache stats
	if m.localCache != nil {
		localStats := m.localCache.GetStats()
		stats.LocalHits = localStats.Hits
		stats.LocalMisses = localStats.Misses
		stats.LocalSize = localStats.Size
	}

	// Get Redis stats (would need to track in metrics)
	if m.redis != nil && m.redis.metrics != nil {
		// These would come from Prometheus metrics
		// For now, just placeholder
		stats.RedisHits = 0
		stats.RedisMisses = 0
	}

	// Calculate hit rate
	totalHits := stats.LocalHits + stats.RedisHits
	totalRequests := totalHits + stats.LocalMisses + stats.RedisMisses
	if totalRequests > 0 {
		stats.HitRate = float64(totalHits) / float64(totalRequests)
	}

	return stats
}

// Cached data types

// CachedTicket represents a cached ticket.
type CachedTicket struct {
	ID           int64     `json:"id"`
	TicketNumber string    `json:"ticket_number"`
	Title        string    `json:"title"`
	QueueID      int       `json:"queue_id"`
	StateID      int       `json:"state_id"`
	PriorityID   int       `json:"priority_id"`
	CustomerID   string    `json:"customer_id"`
	OwnerID      int       `json:"owner_id"`
	CachedAt     time.Time `json:"cached_at"`
}

// CachedUser represents a cached user.
type CachedUser struct {
	ID       int       `json:"id"`
	Login    string    `json:"login"`
	Email    string    `json:"email"`
	Name     string    `json:"name"`
	RoleIDs  []int     `json:"role_ids"`
	GroupIDs []int     `json:"group_ids"`
	CachedAt time.Time `json:"cached_at"`
}

// CachedSession represents a cached session.
type CachedSession struct {
	ID        string                 `json:"id"`
	UserID    int                    `json:"user_id"`
	Data      map[string]interface{} `json:"data"`
	ExpiresAt time.Time              `json:"expires_at"`
	CachedAt  time.Time              `json:"cached_at"`
}

// CachedSearchResults represents cached search results.
type CachedSearchResults struct {
	Query      string        `json:"query"`
	TotalCount int           `json:"total_count"`
	Results    []interface{} `json:"results"`
	CachedAt   time.Time     `json:"cached_at"`
}

// CacheStats represents cache statistics.
type CacheStats struct {
	LocalHits   int64     `json:"local_hits"`
	LocalMisses int64     `json:"local_misses"`
	LocalSize   int64     `json:"local_size"`
	RedisHits   int64     `json:"redis_hits"`
	RedisMisses int64     `json:"redis_misses"`
	HitRate     float64   `json:"hit_rate"`
	Timestamp   time.Time `json:"timestamp"`
}
