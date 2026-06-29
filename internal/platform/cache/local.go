package cache

import (
	"sync"
	"time"
)

// LocalCache provides an in-memory cache with TTL support.
type LocalCache struct {
	mu      sync.RWMutex
	items   map[string]*LocalCacheItem
	maxSize int
	stats   *LocalCacheStats
	stopCh  chan struct{}
	config  *LocalCacheConfig
}

// LocalCacheItem represents a cached item.
type LocalCacheItem struct {
	Value      interface{}
	ExpiresAt  time.Time
	AccessedAt time.Time
	CreatedAt  time.Time
	Size       int64
}

// LocalCacheStats tracks local cache statistics.
type LocalCacheStats struct {
	Hits      int64
	Misses    int64
	Sets      int64
	Deletes   int64
	Evictions int64
	Size      int64
}

// NewLocalCache creates a new local cache.
func NewLocalCache(config *LocalCacheConfig) *LocalCache {
	lc := &LocalCache{
		items:   make(map[string]*LocalCacheItem),
		maxSize: config.MaxSize,
		stats:   &LocalCacheStats{},
		stopCh:  make(chan struct{}),
		config:  config,
	}

	// Start cleanup goroutine
	go lc.cleanupLoop(config.CleanupInterval)

	return lc
}

// Get retrieves an item from local cache.
func (lc *LocalCache) Get(key string) (interface{}, bool) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	item, exists := lc.items[key]
	if !exists {
		lc.stats.Misses++
		return nil, false
	}

	// Check if expired
	if time.Now().After(item.ExpiresAt) {
		lc.stats.Misses++
		return nil, false
	}

	item.AccessedAt = time.Now()
	lc.stats.Hits++
	return item.Value, true
}

// Set stores an item in local cache.
func (lc *LocalCache) Set(key string, value interface{}, ttl time.Duration) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	// Evict if at capacity
	if len(lc.items) >= lc.maxSize {
		lc.evictLRU()
	}

	expiresAt := time.Now().Add(ttl)
	if ttl == 0 {
		expiresAt = time.Now().Add(lc.config.DefaultTTL)
	}

	lc.items[key] = &LocalCacheItem{
		Value:      value,
		ExpiresAt:  expiresAt,
		AccessedAt: time.Now(),
		CreatedAt:  time.Now(),
		Size:       1, // TODO: Calculate actual size
	}

	lc.stats.Sets++
	lc.stats.Size = int64(len(lc.items))
}

// Delete removes an item from local cache.
func (lc *LocalCache) Delete(key string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if _, exists := lc.items[key]; exists {
		delete(lc.items, key)
		lc.stats.Deletes++
		lc.stats.Size = int64(len(lc.items))
	}
}

// Clear removes all items from local cache.
func (lc *LocalCache) Clear() {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	lc.items = make(map[string]*LocalCacheItem)
	lc.stats.Size = 0
}

// GetStats returns cache statistics.
func (lc *LocalCache) GetStats() *LocalCacheStats {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	return &LocalCacheStats{
		Hits:      lc.stats.Hits,
		Misses:    lc.stats.Misses,
		Sets:      lc.stats.Sets,
		Deletes:   lc.stats.Deletes,
		Evictions: lc.stats.Evictions,
		Size:      int64(len(lc.items)),
	}
}

// evictLRU removes the least recently used item.
func (lc *LocalCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time

	for key, item := range lc.items {
		if oldestKey == "" || item.AccessedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.AccessedAt
		}
	}

	if oldestKey != "" {
		delete(lc.items, oldestKey)
		lc.stats.Evictions++
	}
}

// cleanupLoop periodically removes expired items.
func (lc *LocalCache) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			lc.cleanup()
		case <-lc.stopCh:
			return
		}
	}
}

// cleanup removes expired items.
func (lc *LocalCache) cleanup() {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	now := time.Now()
	for key, item := range lc.items {
		if now.After(item.ExpiresAt) {
			delete(lc.items, key)
			lc.stats.Evictions++
		}
	}

	lc.stats.Size = int64(len(lc.items))
}

// Stop stops the cleanup goroutine.
func (lc *LocalCache) Stop() {
	close(lc.stopCh)
}
