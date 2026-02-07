# Advanced Caching Strategy

## Overview

GoatFlow implements a multi-layered caching strategy to optimize performance and reduce database load. The caching system consists of:

1. **Redis Distributed Cache** - Primary cache layer
2. **Query Result Caching** - Database query optimization
3. **CDN/Edge Caching** - Static asset delivery
4. **Application-Level Caching** - In-memory caching for hot data

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Browser   │────▶│     CDN     │────▶│   Varnish   │
└─────────────┘     └─────────────┘     └─────────────┘
                                               │
                                               ▼
                                         ┌─────────────┐
                                         │    NGINX    │
                                         └─────────────┘
                                               │
                                               ▼
                                         ┌─────────────┐
                                         │ Application │
                                         └─────────────┘
                                               │
                                        ┌──────┴──────┐
                                        ▼             ▼
                                  ┌──────────┐ ┌──────────┐
                                  │  Redis   │ │   Query  │
                                  │  Cache   │ │  Cache   │
                                  └──────────┘ └──────────┘
                                        │             │
                                        └──────┬──────┘
                                               ▼
                                         ┌──────────┐
                                         │    DB    │
                                         └──────────┘
```

## Redis Cache Layer

### Configuration

```go
config := &CacheConfig{
    RedisAddr:     []string{"redis-cluster:6379"},
    ClusterMode:   true,
    DefaultTTL:    5 * time.Minute,
    KeyPrefix:     "goatflow",
    Compression:   true,
    MaxRetries:    3,
    PoolSize:      100,
    MinIdleConns:  10,
    MaxConnAge:    30 * time.Minute,
}
```

### Key Features

- **Cluster Support**: Automatic sharding across Redis nodes
- **Compression**: Automatic gzip compression for values > 1KB
- **Connection Pooling**: Efficient connection management
- **Pipeline Support**: Batch operations for performance
- **Atomic Operations**: Support for distributed locking

### Usage Examples

```go
// Simple get/set
cache.Set(ctx, "user:123", userData, 5*time.Minute)
data, _ := cache.Get(ctx, "user:123")

// Object serialization
cache.SetObject(ctx, "ticket:456", ticketObj, 30*time.Second)
cache.GetObject(ctx, "ticket:456", &ticketObj)

// Atomic increment (for rate limiting)
count, _ := cache.Increment(ctx, "api:rate:user:123")

// Distributed locking
acquired, _ := cache.SetNX(ctx, "lock:resource", []byte("1"), 10*time.Second)

// Batch operations
cache.GetMulti(ctx, []string{"key1", "key2", "key3"})

// Pipeline for efficiency
cache.Pipeline(ctx, func(pipe redis.Pipeliner) error {
    pipe.Set(ctx, "key1", "value1", 0)
    pipe.Set(ctx, "key2", "value2", 0)
    return nil
})
```

## Query Cache Layer

### Cache Invalidation Strategy

The query cache automatically invalidates based on table changes:

| Table Change | Invalidated Patterns |
|-------------|---------------------|
| ticket | ticket:*, article:*, queue:*, dashboard:*, search:* |
| article | article:*, ticket:*, search:* |
| users | users:*, customer_user:*, auth:*, session:* |
| queue | queue:*, ticket:*, dashboard:* |
| customer_user | customer_user:*, customer_company:*, ticket:* |

### TTL Strategy

Different data types have different cache durations:

- **Tickets**: 30 seconds (frequently changing)
- **Users/Customers**: 5 minutes (moderate changes)
- **Queues/Roles/Config**: 30 minutes (rarely changes)
- **Historical Data**: 1 hour (immutable)

### Usage

```go
// Simple query caching
result, _ := queryCache.GetOrSet(ctx, 
    "SELECT * FROM ticket WHERE id = ?", 
    []interface{}{123},
    func() (interface{}, error) {
        return db.Query(...)
    })

// Cacheable query builder
query := queryCache.NewCacheableQuery(
    "SELECT COUNT(*) FROM ticket WHERE queue_id = ?",
    []interface{}{5},
).WithTTL(1 * time.Minute).WithExecutor(func() (interface{}, error) {
    return db.QueryRow(...)
})

result, _ := query.Execute(ctx)

// Invalidation
queryCache.InvalidateTable(ctx, "ticket")  // Invalidate all ticket queries
queryCache.InvalidateAll(ctx)              // Clear entire query cache
```

## CDN Configuration

### CloudFront Setup

The CDN configuration includes:

- **Cache Behaviors**: Different TTLs for different content types
- **Origin Configuration**: Connection to upstream servers
- **Security**: WAF integration and DDoS protection
- **Compression**: Automatic gzip/brotli compression
- **Geographic Distribution**: Edge locations worldwide

### Cache Control Headers

| Content Type | Max-Age | S-MaxAge | Immutable |
|-------------|---------|----------|-----------|
| Images | 1 year | 1 year | Yes |
| CSS/JS | 1 month | 1 month | No |
| Fonts | 1 year | 1 year | Yes |
| Documents | 1 day | 1 day | No |
| API | 60s | 60s | No |

### URL Versioning

Static assets use version strings for cache busting:

```
/static/v123/app.css → /static/app.css?v=123
```

## Varnish Cache Layer

### Features

- **Grace Mode**: Serve stale content during backend issues
- **ESI Support**: Edge Side Includes for dynamic content
- **Load Balancing**: Round-robin across backend servers
- **Device Detection**: Separate cache for mobile/desktop
- **Purge Support**: Instant cache invalidation

### Cache Rules

```vcl
# Static files - cache for 1 year
if (req.url ~ "\.(jpg|jpeg|png|gif|ico|svg)(\?|$)") {
    set beresp.ttl = 365d;
}

# CSS/JS - cache for 30 days
if (req.url ~ "\.(css|js)(\?v=|$)") {
    set beresp.ttl = 30d;
}

# API responses - cache for 60 seconds
if (req.url ~ "^/api/v1/" && beresp.status == 200) {
    set beresp.ttl = 60s;
}
```

## Performance Metrics

### Redis Metrics

- `cache_hits_total` - Total cache hits
- `cache_misses_total` - Total cache misses  
- `cache_errors_total` - Cache operation errors
- `cache_operation_duration_seconds` - Operation latency
- `cache_size_bytes` - Current cache size

### Query Cache Metrics

- Query cache hit ratio
- Average query execution time
- Cache invalidation frequency
- Memory usage

## Cache Warming

Pre-load frequently accessed data on startup:

```go
warmupQueries := []WarmUpQuery{
    {
        Query: "SELECT * FROM queue WHERE active = ?",
        Args: []interface{}{true},
        Executor: func() (interface{}, error) {
            return queueRepo.GetActiveQueues()
        },
    },
    {
        Query: "SELECT * FROM role",
        Executor: func() (interface{}, error) {
            return roleRepo.GetAll()
        },
    },
}

queryCache.WarmUp(ctx, warmupQueries)
```

## Best Practices

### 1. Cache Key Design

- Use hierarchical keys: `type:subtype:id`
- Include version in keys when needed
- Keep keys short but descriptive

### 2. TTL Selection

- Shorter TTL for frequently changing data
- Longer TTL for reference data
- Use grace periods for critical data

### 3. Compression

- Enable for text/JSON data > 1KB
- Disable for already compressed data
- Monitor compression ratios

### 4. Invalidation

- Invalidate at the smallest granularity possible
- Use patterns for bulk invalidation
- Consider async invalidation for performance

### 5. Monitoring

- Track hit/miss ratios (target > 80% hit ratio)
- Monitor cache size and eviction rates
- Alert on high error rates
- Track p95 latencies

## Deployment

### Valkey (Redis-compatible)

GoatFlow uses Valkey for caching. Deploy via Helm chart:

```bash
# Deploy GoatFlow with Valkey enabled (default)
helm install goatflow ./charts/goatflow

# Check Valkey status
kubectl get pods -l app.kubernetes.io/name=valkey

# Connect to Valkey CLI
kubectl exec -it <valkey-pod> -- valkey-cli
```

### Varnish (Optional)

For edge caching, deploy Varnish separately:

```bash
# Check Varnish stats
varnishstat -1

# Purge cache
curl -X PURGE http://varnish-cache/api/v1/tickets
```

### CDN

```bash
# Deploy CloudFront distribution
aws cloudfront create-distribution --distribution-config file://cloudfront-config.json

# Invalidate CDN cache
aws cloudfront create-invalidation --distribution-id ABCDEF --paths "/*"
```

## Troubleshooting

### Cache Misses

1. Check key naming consistency
2. Verify TTL settings
3. Monitor eviction rates
4. Check memory limits

### Performance Issues

1. Enable connection pooling
2. Use pipeline for batch operations
3. Monitor network latency
4. Check Redis CPU/memory usage

### Invalidation Problems

1. Verify invalidation patterns
2. Check table-to-pattern mappings
3. Monitor invalidation frequency
4. Use async invalidation if needed

## Future Enhancements

1. **Multi-tier caching**: L1 (in-memory) + L2 (Redis)
2. **Predictive warming**: ML-based cache warming
3. **Adaptive TTL**: Dynamic TTL based on access patterns
4. **Edge computing**: Compute at CDN edge locations
5. **Cache analytics**: Detailed usage analytics and optimization recommendations