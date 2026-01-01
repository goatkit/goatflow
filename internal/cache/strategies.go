package cache

import (
	"context"
	"time"
)

// WriteThroughStrategy writes to both local and Redis synchronously.
type WriteThroughStrategy struct {
	local *LocalCache
	redis *RedisCache
}

// Get retrieves from local first, then Redis.
func (s *WriteThroughStrategy) Get(ctx context.Context, key string) (interface{}, error) {
	// Check local cache first
	if s.local != nil {
		if val, found := s.local.Get(key); found {
			return val, nil
		}
	}

	// Check Redis
	if s.redis != nil {
		val, err := s.redis.Get(ctx, key)
		if err != nil {
			return nil, err
		}

		// Store in local cache if found
		if val != nil && s.local != nil {
			s.local.Set(key, val, 5*time.Minute)
		}

		return val, nil
	}

	return nil, nil
}

// Set writes to both caches.
func (s *WriteThroughStrategy) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// Write to local cache
	if s.local != nil {
		s.local.Set(key, value, ttl)
	}

	// Write to Redis
	if s.redis != nil {
		return s.redis.Set(ctx, key, value, ttl)
	}

	return nil
}

// Delete removes from both caches.
func (s *WriteThroughStrategy) Delete(ctx context.Context, key string) error {
	// Delete from local
	if s.local != nil {
		s.local.Delete(key)
	}

	// Delete from Redis
	if s.redis != nil {
		return s.redis.Delete(ctx, key)
	}

	return nil
}

// Clear removes all matching keys.
func (s *WriteThroughStrategy) Clear(ctx context.Context, pattern string) error {
	// Clear local cache
	if s.local != nil {
		s.local.Clear()
	}

	// Clear Redis
	if s.redis != nil {
		return s.redis.Clear(ctx, pattern)
	}

	return nil
}

// GetMulti retrieves multiple keys.
func (s *WriteThroughStrategy) GetMulti(ctx context.Context, keys []string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	missingKeys := []string{}

	// Check local cache first
	if s.local != nil {
		for _, key := range keys {
			if val, found := s.local.Get(key); found {
				result[key] = val
			} else {
				missingKeys = append(missingKeys, key)
			}
		}
	} else {
		missingKeys = keys
	}

	// Get missing keys from Redis
	if len(missingKeys) > 0 && s.redis != nil {
		redisVals, err := s.redis.GetMulti(ctx, missingKeys)
		if err != nil {
			return result, err
		}

		// Add Redis results and update local cache
		for key, val := range redisVals {
			result[key] = val
			if s.local != nil && val != nil {
				s.local.Set(key, val, 5*time.Minute)
			}
		}
	}

	return result, nil
}

// SetMulti sets multiple keys.
func (s *WriteThroughStrategy) SetMulti(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	// Set in local cache
	if s.local != nil {
		for key, val := range items {
			s.local.Set(key, val, ttl)
		}
	}

	// Set in Redis
	if s.redis != nil {
		return s.redis.SetMulti(ctx, items, ttl)
	}

	return nil
}

// WriteBehindStrategy writes to local immediately, Redis asynchronously.
type WriteBehindStrategy struct {
	local *LocalCache
	redis *RedisCache
	queue chan *CacheItem
}

// Get retrieves from local first, then Redis.
func (s *WriteBehindStrategy) Get(ctx context.Context, key string) (interface{}, error) {
	// Same as write-through for reads
	if s.local != nil {
		if val, found := s.local.Get(key); found {
			return val, nil
		}
	}

	if s.redis != nil {
		val, err := s.redis.Get(ctx, key)
		if err != nil {
			return nil, err
		}

		if val != nil && s.local != nil {
			s.local.Set(key, val, 5*time.Minute)
		}

		return val, nil
	}

	return nil, nil
}

// Set writes to local immediately, queues for Redis.
func (s *WriteBehindStrategy) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// Write to local cache immediately
	if s.local != nil {
		s.local.Set(key, value, ttl)
	}

	// Queue for Redis write
	if s.redis != nil {
		select {
		case s.queue <- &CacheItem{
			Key:       key,
			Value:     value,
			TTL:       ttl,
			CreatedAt: time.Now(),
		}:
		default:
			// Queue full, write synchronously
			return s.redis.Set(ctx, key, value, ttl)
		}
	}

	return nil
}

// Delete removes from local immediately, queues for Redis.
func (s *WriteBehindStrategy) Delete(ctx context.Context, key string) error {
	// Delete from local immediately
	if s.local != nil {
		s.local.Delete(key)
	}

	// Queue for Redis delete
	if s.redis != nil {
		select {
		case s.queue <- &CacheItem{
			Key:       key,
			Value:     nil, // nil value signals delete
			CreatedAt: time.Now(),
		}:
		default:
			// Queue full, delete synchronously
			return s.redis.Delete(ctx, key)
		}
	}

	return nil
}

// Clear removes all matching keys.
func (s *WriteBehindStrategy) Clear(ctx context.Context, pattern string) error {
	if s.local != nil {
		s.local.Clear()
	}

	if s.redis != nil {
		return s.redis.Clear(ctx, pattern)
	}

	return nil
}

// GetMulti retrieves multiple keys.
func (s *WriteBehindStrategy) GetMulti(ctx context.Context, keys []string) (map[string]interface{}, error) {
	// Same as write-through
	return (&WriteThroughStrategy{local: s.local, redis: s.redis}).GetMulti(ctx, keys)
}

// SetMulti sets multiple keys.
func (s *WriteBehindStrategy) SetMulti(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	// Set in local cache immediately
	if s.local != nil {
		for key, val := range items {
			s.local.Set(key, val, ttl)
		}
	}

	// Queue for Redis
	if s.redis != nil {
		for key, val := range items {
			select {
			case s.queue <- &CacheItem{
				Key:       key,
				Value:     val,
				TTL:       ttl,
				CreatedAt: time.Now(),
			}:
			default:
				// Queue full, write remaining synchronously
				return s.redis.SetMulti(ctx, items, ttl)
			}
		}
	}

	return nil
}

// Start starts the background writer.
func (s *WriteBehindStrategy) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case item := <-s.queue:
				if item.Value == nil {
					// Delete operation
					s.redis.Delete(ctx, item.Key)
				} else {
					// Set operation
					s.redis.Set(ctx, item.Key, item.Value, item.TTL)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

// ReadThroughStrategy reads from cache, fetches from source on miss.
type ReadThroughStrategy struct {
	local  *LocalCache
	redis  *RedisCache
	loader DataLoader
}

// DataLoader fetches data from the source.
type DataLoader interface {
	Load(ctx context.Context, key string) (interface{}, error)
	LoadMulti(ctx context.Context, keys []string) (map[string]interface{}, error)
}

// Get retrieves from cache or loads from source.
func (s *ReadThroughStrategy) Get(ctx context.Context, key string) (interface{}, error) {
	// Check local cache
	if s.local != nil {
		if val, found := s.local.Get(key); found {
			return val, nil
		}
	}

	// Check Redis
	if s.redis != nil {
		val, err := s.redis.Get(ctx, key)
		if err != nil {
			return nil, err
		}

		if val != nil {
			if s.local != nil {
				s.local.Set(key, val, 5*time.Minute)
			}
			return val, nil
		}
	}

	// Load from source
	if s.loader != nil {
		val, err := s.loader.Load(ctx, key)
		if err != nil {
			return nil, err
		}

		// Cache the loaded value
		if val != nil {
			s.Set(ctx, key, val, 5*time.Minute)
		}

		return val, nil
	}

	return nil, nil
}

// Set stores in cache.
func (s *ReadThroughStrategy) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if s.local != nil {
		s.local.Set(key, value, ttl)
	}

	if s.redis != nil {
		return s.redis.Set(ctx, key, value, ttl)
	}

	return nil
}

// Delete removes from cache.
func (s *ReadThroughStrategy) Delete(ctx context.Context, key string) error {
	if s.local != nil {
		s.local.Delete(key)
	}

	if s.redis != nil {
		return s.redis.Delete(ctx, key)
	}

	return nil
}

// Clear removes all matching keys.
func (s *ReadThroughStrategy) Clear(ctx context.Context, pattern string) error {
	if s.local != nil {
		s.local.Clear()
	}

	if s.redis != nil {
		return s.redis.Clear(ctx, pattern)
	}

	return nil
}

// GetMulti retrieves multiple keys.
func (s *ReadThroughStrategy) GetMulti(ctx context.Context, keys []string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	missingKeys := []string{}

	// Check local cache
	if s.local != nil {
		for _, key := range keys {
			if val, found := s.local.Get(key); found {
				result[key] = val
			} else {
				missingKeys = append(missingKeys, key)
			}
		}
	} else {
		missingKeys = keys
	}

	// Check Redis for missing keys
	if len(missingKeys) > 0 && s.redis != nil {
		redisVals, err := s.redis.GetMulti(ctx, missingKeys)
		if err != nil {
			return result, err
		}

		stillMissing := []string{}
		for _, key := range missingKeys {
			if val, exists := redisVals[key]; exists && val != nil {
				result[key] = val
				if s.local != nil {
					s.local.Set(key, val, 5*time.Minute)
				}
			} else {
				stillMissing = append(stillMissing, key)
			}
		}

		missingKeys = stillMissing
	}

	// Load missing from source
	if len(missingKeys) > 0 && s.loader != nil {
		sourceVals, err := s.loader.LoadMulti(ctx, missingKeys)
		if err != nil {
			return result, err
		}

		// Cache loaded values
		for key, val := range sourceVals {
			result[key] = val
			if val != nil {
				s.Set(ctx, key, val, 5*time.Minute)
			}
		}
	}

	return result, nil
}

// SetMulti sets multiple keys.
func (s *ReadThroughStrategy) SetMulti(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	if s.local != nil {
		for key, val := range items {
			s.local.Set(key, val, ttl)
		}
	}

	if s.redis != nil {
		return s.redis.SetMulti(ctx, items, ttl)
	}

	return nil
}

// CacheAsideStrategy leaves cache management to the application.
type CacheAsideStrategy struct {
	local *LocalCache
	redis *RedisCache
}

// Get retrieves from cache only.
func (s *CacheAsideStrategy) Get(ctx context.Context, key string) (interface{}, error) {
	// Check local cache
	if s.local != nil {
		if val, found := s.local.Get(key); found {
			return val, nil
		}
	}

	// Check Redis
	if s.redis != nil {
		return s.redis.Get(ctx, key)
	}

	return nil, nil
}

// Set stores in cache.
func (s *CacheAsideStrategy) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if s.local != nil {
		s.local.Set(key, value, ttl)
	}

	if s.redis != nil {
		return s.redis.Set(ctx, key, value, ttl)
	}

	return nil
}

// Delete removes from cache.
func (s *CacheAsideStrategy) Delete(ctx context.Context, key string) error {
	if s.local != nil {
		s.local.Delete(key)
	}

	if s.redis != nil {
		return s.redis.Delete(ctx, key)
	}

	return nil
}

// Clear removes all matching keys.
func (s *CacheAsideStrategy) Clear(ctx context.Context, pattern string) error {
	if s.local != nil {
		s.local.Clear()
	}

	if s.redis != nil {
		return s.redis.Clear(ctx, pattern)
	}

	return nil
}

// GetMulti retrieves multiple keys.
func (s *CacheAsideStrategy) GetMulti(ctx context.Context, keys []string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	missingKeys := []string{}

	// Check local cache
	if s.local != nil {
		for _, key := range keys {
			if val, found := s.local.Get(key); found {
				result[key] = val
			} else {
				missingKeys = append(missingKeys, key)
			}
		}
	} else {
		missingKeys = keys
	}

	// Get missing from Redis
	if len(missingKeys) > 0 && s.redis != nil {
		redisVals, err := s.redis.GetMulti(ctx, missingKeys)
		if err != nil {
			return result, err
		}

		for key, val := range redisVals {
			result[key] = val
			// Update local cache
			if val != nil && s.local != nil {
				s.local.Set(key, val, 5*time.Minute)
			}
		}
	}

	return result, nil
}

// SetMulti sets multiple keys.
func (s *CacheAsideStrategy) SetMulti(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	if s.local != nil {
		for key, val := range items {
			s.local.Set(key, val, ttl)
		}
	}

	if s.redis != nil {
		return s.redis.SetMulti(ctx, items, ttl)
	}

	return nil
}
