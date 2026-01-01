package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
)

// RedisCache implements a distributed caching layer using Redis.
type RedisCache struct {
	client      *redis.Client
	cluster     *redis.ClusterClient
	isCluster   bool
	defaultTTL  time.Duration
	keyPrefix   string
	metrics     *CacheMetrics
	compression bool
	maxRetries  int
}

// CacheMetrics tracks cache performance.
type CacheMetrics struct {
	hits    prometheus.Counter
	misses  prometheus.Counter
	errors  prometheus.Counter
	sets    prometheus.Counter
	deletes prometheus.Counter
	latency prometheus.Histogram
	size    prometheus.Gauge
}

// CacheConfig defines cache configuration.
type CacheConfig struct {
	// Redis connection
	RedisAddr     []string
	RedisPassword string
	RedisDB       int

	// Cluster mode
	ClusterMode bool

	// Cache settings
	DefaultTTL  time.Duration
	KeyPrefix   string
	Compression bool
	MaxRetries  int

	// Pool settings
	PoolSize     int
	MinIdleConns int
	MaxConnAge   time.Duration

	// Timeouts
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// NewRedisCache creates a new Redis cache instance.
func NewRedisCache(config *CacheConfig) (*RedisCache, error) {
	cache := &RedisCache{
		defaultTTL:  config.DefaultTTL,
		keyPrefix:   config.KeyPrefix,
		compression: config.Compression,
		maxRetries:  config.MaxRetries,
		isCluster:   config.ClusterMode,
	}

	// Initialize metrics
	cache.metrics = &CacheMetrics{
		hits: promauto.NewCounter(prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		}),
		misses: promauto.NewCounter(prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		}),
		errors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "cache_errors_total",
			Help: "Total number of cache errors",
		}),
		sets: promauto.NewCounter(prometheus.CounterOpts{
			Name: "cache_sets_total",
			Help: "Total number of cache sets",
		}),
		deletes: promauto.NewCounter(prometheus.CounterOpts{
			Name: "cache_deletes_total",
			Help: "Total number of cache deletes",
		}),
		latency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "cache_operation_duration_seconds",
			Help:    "Cache operation latency",
			Buckets: prometheus.DefBuckets,
		}),
		size: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "cache_size_bytes",
			Help: "Current cache size in bytes",
		}),
	}

	if config.ClusterMode {
		// Create Redis cluster client
		cache.cluster = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:        config.RedisAddr,
			Password:     config.RedisPassword,
			DialTimeout:  config.DialTimeout,
			ReadTimeout:  config.ReadTimeout,
			WriteTimeout: config.WriteTimeout,
			PoolSize:     config.PoolSize,
			MinIdleConns: config.MinIdleConns,
		})

		// Test cluster connection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := cache.cluster.Ping(ctx).Err(); err != nil {
			return nil, fmt.Errorf("failed to connect to Redis cluster: %w", err)
		}
	} else {
		// Create standard Redis client
		cache.client = redis.NewClient(&redis.Options{
			Addr:         config.RedisAddr[0],
			Password:     config.RedisPassword,
			DB:           config.RedisDB,
			DialTimeout:  config.DialTimeout,
			ReadTimeout:  config.ReadTimeout,
			WriteTimeout: config.WriteTimeout,
			PoolSize:     config.PoolSize,
			MinIdleConns: config.MinIdleConns,
		})

		// Test connection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := cache.client.Ping(ctx).Err(); err != nil {
			return nil, fmt.Errorf("failed to connect to Redis: %w", err)
		}
	}

	return cache, nil
}

// getClient returns the appropriate Redis client.
func (rc *RedisCache) getClient() redis.Cmdable {
	if rc.isCluster {
		return rc.cluster
	}
	return rc.client
}

// Get retrieves a value from Redis.
func (rc *RedisCache) Get(ctx context.Context, key string) (interface{}, error) {
	timer := prometheus.NewTimer(rc.metrics.latency)
	defer timer.ObserveDuration()

	fullKey := rc.keyPrefix + key
	client := rc.getClient()

	val, err := client.Get(ctx, fullKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			rc.metrics.misses.Inc()
			return nil, nil
		}
		rc.metrics.errors.Inc()
		return nil, err
	}

	rc.metrics.hits.Inc()

	// Decompress if needed
	if rc.compression {
		val = decompress(val)
	}

	return val, nil
}

// Set stores a value in Redis.
func (rc *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	timer := prometheus.NewTimer(rc.metrics.latency)
	defer timer.ObserveDuration()

	fullKey := rc.keyPrefix + key
	client := rc.getClient()

	// Serialize value
	data, err := json.Marshal(value)
	if err != nil {
		rc.metrics.errors.Inc()
		return err
	}

	// Compress if needed
	if rc.compression {
		data = compress(data)
	}

	// Use default TTL if not specified
	if ttl == 0 {
		ttl = rc.defaultTTL
	}

	err = client.Set(ctx, fullKey, data, ttl).Err()
	if err != nil {
		rc.metrics.errors.Inc()
		return err
	}

	rc.metrics.sets.Inc()
	return nil
}

// Delete removes a value from Redis.
func (rc *RedisCache) Delete(ctx context.Context, key string) error {
	timer := prometheus.NewTimer(rc.metrics.latency)
	defer timer.ObserveDuration()

	fullKey := rc.keyPrefix + key
	client := rc.getClient()

	err := client.Del(ctx, fullKey).Err()
	if err != nil {
		rc.metrics.errors.Inc()
		return err
	}

	rc.metrics.deletes.Inc()
	return nil
}

// Clear removes all keys matching a pattern.
func (rc *RedisCache) Clear(ctx context.Context, pattern string) error {
	timer := prometheus.NewTimer(rc.metrics.latency)
	defer timer.ObserveDuration()

	fullPattern := rc.keyPrefix + pattern
	client := rc.getClient()

	// Use SCAN to find keys (safe for production)
	var cursor uint64
	var keys []string

	for {
		var scanKeys []string
		var err error

		if rc.isCluster {
			// For cluster, we need to scan each node
			err = rc.cluster.ForEachMaster(ctx, func(ctx context.Context, client *redis.Client) error {
				iter := client.Scan(ctx, cursor, fullPattern, 100).Iterator()
				for iter.Next(ctx) {
					scanKeys = append(scanKeys, iter.Val())
				}
				return iter.Err()
			})
		} else {
			// For single node
			iter := rc.client.Scan(ctx, cursor, fullPattern, 100).Iterator()
			for iter.Next(ctx) {
				scanKeys = append(scanKeys, iter.Val())
			}
			err = iter.Err()
		}

		if err != nil {
			rc.metrics.errors.Inc()
			return err
		}

		keys = append(keys, scanKeys...)

		if cursor == 0 {
			break
		}
	}

	// Delete keys in batches
	if len(keys) > 0 {
		pipe := client.Pipeline()
		for _, key := range keys {
			pipe.Del(ctx, key)
		}
		_, err := pipe.Exec(ctx)
		if err != nil {
			rc.metrics.errors.Inc()
			return err
		}
	}

	rc.metrics.deletes.Add(float64(len(keys)))
	return nil
}

// GetMulti retrieves multiple values from Redis.
func (rc *RedisCache) GetMulti(ctx context.Context, keys []string) (map[string]interface{}, error) {
	timer := prometheus.NewTimer(rc.metrics.latency)
	defer timer.ObserveDuration()

	result := make(map[string]interface{})
	if len(keys) == 0 {
		return result, nil
	}

	client := rc.getClient()
	pipe := client.Pipeline()

	// Add all GET commands to pipeline
	fullKeys := make([]string, len(keys))
	for i, key := range keys {
		fullKeys[i] = rc.keyPrefix + key
		pipe.Get(ctx, fullKeys[i])
	}

	// Execute pipeline
	cmds, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		rc.metrics.errors.Inc()
		return result, err
	}

	// Process results
	for i, cmd := range cmds {
		if i >= len(keys) {
			break
		}

		stringCmd := cmd.(*redis.StringCmd)
		val, err := stringCmd.Bytes()
		if err == nil {
			// Decompress if needed
			if rc.compression {
				val = decompress(val)
			}
			result[keys[i]] = val
			rc.metrics.hits.Inc()
		} else if err == redis.Nil {
			rc.metrics.misses.Inc()
		} else {
			rc.metrics.errors.Inc()
		}
	}

	return result, nil
}

// SetMulti stores multiple values in Redis.
func (rc *RedisCache) SetMulti(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	timer := prometheus.NewTimer(rc.metrics.latency)
	defer timer.ObserveDuration()

	if len(items) == 0 {
		return nil
	}

	client := rc.getClient()
	pipe := client.Pipeline()

	// Use default TTL if not specified
	if ttl == 0 {
		ttl = rc.defaultTTL
	}

	// Add all SET commands to pipeline
	for key, value := range items {
		fullKey := rc.keyPrefix + key

		// Serialize value
		data, err := json.Marshal(value)
		if err != nil {
			rc.metrics.errors.Inc()
			return err
		}

		// Compress if needed
		if rc.compression {
			data = compress(data)
		}

		pipe.Set(ctx, fullKey, data, ttl)
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		rc.metrics.errors.Inc()
		return err
	}

	rc.metrics.sets.Add(float64(len(items)))
	return nil
}

// Exists checks if a key exists in Redis.
func (rc *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	timer := prometheus.NewTimer(rc.metrics.latency)
	defer timer.ObserveDuration()

	fullKey := rc.keyPrefix + key
	client := rc.getClient()

	exists, err := client.Exists(ctx, fullKey).Result()
	if err != nil {
		rc.metrics.errors.Inc()
		return false, err
	}

	return exists > 0, nil
}

// TTL returns the TTL of a key.
func (rc *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	fullKey := rc.keyPrefix + key
	client := rc.getClient()

	ttl, err := client.TTL(ctx, fullKey).Result()
	if err != nil {
		rc.metrics.errors.Inc()
		return 0, err
	}

	return ttl, nil
}

// Expire sets a new TTL for a key.
func (rc *RedisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	fullKey := rc.keyPrefix + key
	client := rc.getClient()

	err := client.Expire(ctx, fullKey, ttl).Err()
	if err != nil {
		rc.metrics.errors.Inc()
		return err
	}

	return nil
}

// GetObject retrieves and unmarshals a value from cache.
func (rc *RedisCache) GetObject(ctx context.Context, key string, dest interface{}) error {
	val, err := rc.Get(ctx, key)
	if err != nil {
		return err
	}
	if val == nil {
		return nil
	}

	// Unmarshal JSON data
	data, ok := val.([]byte)
	if !ok {
		return fmt.Errorf("cached value is not []byte")
	}

	return json.Unmarshal(data, dest)
}

// SetObject marshals and stores a value in cache.
func (rc *RedisCache) SetObject(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return rc.Set(ctx, key, value, ttl)
}

// Invalidate removes cache entries matching pattern.
func (rc *RedisCache) Invalidate(ctx context.Context, pattern string) error {
	return rc.Clear(ctx, pattern)
}

// Info returns cache information.
func (rc *RedisCache) Info(ctx context.Context) (map[string]interface{}, error) {
	client := rc.getClient()

	info := make(map[string]interface{})

	// Get basic info from Redis
	if rc.isCluster {
		// For cluster mode, get info from first node
		err := rc.cluster.ForEachMaster(ctx, func(ctx context.Context, client *redis.Client) error {
			result, err := client.Info(ctx).Result()
			if err == nil {
				info["redis_info"] = result
			}
			return err
		})
		if err != nil {
			return nil, err
		}
	} else {
		result, err := client.Info(ctx).Result()
		if err != nil {
			return nil, err
		}
		info["redis_info"] = result
	}

	info["key_prefix"] = rc.keyPrefix
	info["default_ttl"] = rc.defaultTTL.String()
	info["compression"] = rc.compression

	return info, nil
}

// Pipeline executes a batch of operations.
func (rc *RedisCache) Pipeline(ctx context.Context, fn func(pipe redis.Pipeliner) error) error {
	client := rc.getClient()
	pipe := client.Pipeline()

	if err := fn(pipe); err != nil {
		return err
	}

	_, err := pipe.Exec(ctx)
	return err
}

// Close closes the Redis connection.
func (rc *RedisCache) Close() error {
	if rc.isCluster {
		return rc.cluster.Close()
	}
	return rc.client.Close()
}
