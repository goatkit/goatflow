package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SandboxedHostAPI wraps a HostAPI with per-plugin permission enforcement
// and resource accounting. Each plugin gets its own sandbox instance.
type SandboxedHostAPI struct {
	inner      HostAPI
	pluginName string
	policyMu   sync.RWMutex
	policy     *ResourcePolicy  // pointer for live updates

	// Rate limiting
	dbQueries      rateLimiter
	httpRequests   rateLimiter
	callRate       rateLimiter
	emailRateLimit rateLimiter

	// Accounting
	stats PluginStats
}

// PluginStats tracks resource usage for a plugin.
type PluginStats struct {
	DBQueries    atomic.Int64
	DBExecs      atomic.Int64
	CacheOps     atomic.Int64
	HTTPRequests atomic.Int64
	Calls        atomic.Int64
	Errors       atomic.Int64
	LastCallAt   atomic.Int64 // unix millis
}

// StatsSnapshot returns a point-in-time copy of plugin stats.
type StatsSnapshot struct {
	PluginName   string `json:"plugin_name"`
	DBQueries    int64  `json:"db_queries"`
	DBExecs      int64  `json:"db_execs"`
	CacheOps     int64  `json:"cache_ops"`
	HTTPRequests int64  `json:"http_requests"`
	Calls        int64  `json:"calls"`
	Errors       int64  `json:"errors"`
	LastCallAt   int64  `json:"last_call_at"`
}

// Snapshot returns a copy of the current stats.
func (s *PluginStats) Snapshot(name string) StatsSnapshot {
	return StatsSnapshot{
		PluginName:   name,
		DBQueries:    s.DBQueries.Load(),
		DBExecs:      s.DBExecs.Load(),
		CacheOps:     s.CacheOps.Load(),
		HTTPRequests: s.HTTPRequests.Load(),
		Calls:        s.Calls.Load(),
		Errors:       s.Errors.Load(),
		LastCallAt:   s.LastCallAt.Load(),
	}
}

// NewSandboxedHostAPI creates a permission-enforcing wrapper around a HostAPI.
func NewSandboxedHostAPI(inner HostAPI, pluginName string, policy ResourcePolicy) *SandboxedHostAPI {
	s := &SandboxedHostAPI{
		inner:      inner,
		pluginName: pluginName,
		policy:     &policy,  // Store pointer for live updates
	}

	// Initialise rate limiters from policy
	if policy.MaxDBQueriesPerMin > 0 {
		s.dbQueries = newRateLimiter(policy.MaxDBQueriesPerMin, time.Minute)
	}
	if policy.MaxHTTPReqPerMin > 0 {
		s.httpRequests = newRateLimiter(policy.MaxHTTPReqPerMin, time.Minute)
	}
	if policy.MaxCallsPerSecond > 0 {
		s.callRate = newRateLimiter(policy.MaxCallsPerSecond, time.Second)
	}
	
	// Default email rate limit: 10 per minute
	s.emailRateLimit = newRateLimiter(10, time.Minute)

	return s
}

// Stats returns the resource accounting stats for this plugin.
func (s *SandboxedHostAPI) Stats() StatsSnapshot {
	return s.stats.Snapshot(s.pluginName)
}

// UpdatePolicy updates the resource policy for this sandbox.
// Policy changes take effect immediately for new requests.
func (s *SandboxedHostAPI) UpdatePolicy(policy ResourcePolicy) {
	s.policyMu.Lock()
	defer s.policyMu.Unlock()
	
	oldPolicy := s.policy
	s.policy = &policy

	// Update rate limiters if limits changed
	if oldPolicy.MaxDBQueriesPerMin != policy.MaxDBQueriesPerMin {
		if policy.MaxDBQueriesPerMin > 0 {
			s.dbQueries = newRateLimiter(policy.MaxDBQueriesPerMin, time.Minute)
		} else {
			s.dbQueries = rateLimiter{disabled: true}
		}
	}
	
	if oldPolicy.MaxHTTPReqPerMin != policy.MaxHTTPReqPerMin {
		if policy.MaxHTTPReqPerMin > 0 {
			s.httpRequests = newRateLimiter(policy.MaxHTTPReqPerMin, time.Minute)
		} else {
			s.httpRequests = rateLimiter{disabled: true}
		}
	}
	
	if oldPolicy.MaxCallsPerSecond != policy.MaxCallsPerSecond {
		if policy.MaxCallsPerSecond > 0 {
			s.callRate = newRateLimiter(policy.MaxCallsPerSecond, time.Second)
		} else {
			s.callRate = rateLimiter{disabled: true}
		}
	}
}

// --- Permission checks ---

// hasPermission checks if the policy grants a specific permission type and access level.
func (s *SandboxedHostAPI) hasPermission(permType, access string) bool {
	s.policyMu.RLock()
	defer s.policyMu.RUnlock()
	
	if s.policy.Status == "blocked" {
		return false
	}
	for _, p := range s.policy.Permissions {
		if p.Type == permType {
			if access == "" {
				return true
			}
			switch p.Access {
			case "readwrite":
				return true
			case access:
				return true
			}
		}
	}
	return false
}

// permissionScope returns the scope for a given permission type, or nil if not granted.
func (s *SandboxedHostAPI) permissionScope(permType string) []string {
	s.policyMu.RLock()
	defer s.policyMu.RUnlock()
	
	for _, p := range s.policy.Permissions {
		if p.Type == permType {
			return p.Scope
		}
	}
	return nil
}

// checkDBTableAccess validates that a query only touches allowed tables
// and blocks DDL statements for plugins without write access.
func (s *SandboxedHostAPI) checkDBTableAccess(query string) error {
	upper := strings.ToUpper(strings.TrimSpace(query))

	// Block dangerous DDL statements unless plugin has write access
	if !s.hasPermission("db", "write") {
		for _, keyword := range []string{"DROP ", "ALTER ", "TRUNCATE ", "CREATE ", "GRANT ", "REVOKE "} {
			if strings.Contains(upper, keyword) {
				return fmt.Errorf("plugin %q: DDL statements not permitted", s.pluginName)
			}
		}
	}

	// Check table access scope if specified
	scope := s.permissionScope("db")
	if len(scope) > 0 {
		tables := extractTableNames(query)
		for _, table := range tables {
			if !isTableAllowed(table, scope) {
				return fmt.Errorf("plugin %q: access to table %q not permitted (allowed: %v)", s.pluginName, table, scope)
			}
		}
	}

	return nil
}

// checkHTTPAccess validates that the URL matches the allowed patterns.
func (s *SandboxedHostAPI) checkHTTPAccess(url string) error {
	scope := s.permissionScope("http")
	if len(scope) == 0 {
		return nil // No URL restrictions
	}

	for _, pattern := range scope {
		if matchURLPattern(pattern, url) {
			return nil
		}
	}

	return fmt.Errorf("plugin %q: HTTP access to %q not permitted (allowed: %v)", s.pluginName, url, scope)
}

// matchURLPattern checks if a URL matches a scope pattern.
// Patterns: "*.example.com" matches subdomains, "api.example.com" matches exact host.
func matchURLPattern(pattern, url string) bool {
	// Extract host from URL
	host := url
	if idx := strings.Index(host, "://"); idx >= 0 {
		host = host[idx+3:]
	}
	if idx := strings.Index(host, "/"); idx >= 0 {
		host = host[:idx]
	}
	if idx := strings.Index(host, ":"); idx >= 0 {
		host = host[:idx]
	}

	host = strings.ToLower(host)
	pattern = strings.ToLower(pattern)

	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // ".example.com"
		return strings.HasSuffix(host, suffix) || host == pattern[2:]
	}

	return host == pattern
}

// --- HostAPI implementation ---

func (s *SandboxedHostAPI) DBQuery(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	if !s.hasPermission("db", "read") {
		s.stats.Errors.Add(1)
		return nil, fmt.Errorf("plugin %q: database read access not granted", s.pluginName)
	}
	if err := s.checkDBTableAccess(query); err != nil {
		s.stats.Errors.Add(1)
		return nil, err
	}
	if s.dbQueries.enabled() && !s.dbQueries.allow() {
		s.stats.Errors.Add(1)
		return nil, fmt.Errorf("plugin %q: DB query rate limit exceeded", s.pluginName)
	}
	s.stats.DBQueries.Add(1)
	s.stats.LastCallAt.Store(time.Now().UnixMilli())
	return s.inner.DBQuery(ctx, query, args...)
}

func (s *SandboxedHostAPI) DBExec(ctx context.Context, query string, args ...any) (int64, error) {
	if !s.hasPermission("db", "write") {
		s.stats.Errors.Add(1)
		return 0, fmt.Errorf("plugin %q: database write access not granted", s.pluginName)
	}
	if err := s.checkDBTableAccess(query); err != nil {
		s.stats.Errors.Add(1)
		return 0, err
	}
	if s.dbQueries.enabled() && !s.dbQueries.allow() {
		s.stats.Errors.Add(1)
		return 0, fmt.Errorf("plugin %q: DB query rate limit exceeded", s.pluginName)
	}
	s.stats.DBExecs.Add(1)
	s.stats.LastCallAt.Store(time.Now().UnixMilli())
	return s.inner.DBExec(ctx, query, args...)
}

func (s *SandboxedHostAPI) CacheGet(ctx context.Context, key string) ([]byte, bool, error) {
	if !s.hasPermission("cache", "read") {
		s.stats.Errors.Add(1)
		return nil, false, fmt.Errorf("plugin %q: cache access not granted", s.pluginName)
	}
	s.stats.CacheOps.Add(1)
	// Auto-namespace cache keys to prevent cross-plugin collisions
	return s.inner.CacheGet(ctx, s.namespacedKey(key))
}

func (s *SandboxedHostAPI) CacheSet(ctx context.Context, key string, value []byte, ttlSeconds int) error {
	if !s.hasPermission("cache", "write") {
		s.stats.Errors.Add(1)
		return fmt.Errorf("plugin %q: cache write access not granted", s.pluginName)
	}
	s.stats.CacheOps.Add(1)
	return s.inner.CacheSet(ctx, s.namespacedKey(key), value, ttlSeconds)
}

func (s *SandboxedHostAPI) CacheDelete(ctx context.Context, key string) error {
	if !s.hasPermission("cache", "write") {
		s.stats.Errors.Add(1)
		return fmt.Errorf("plugin %q: cache delete access not granted", s.pluginName)
	}
	s.stats.CacheOps.Add(1)
	return s.inner.CacheDelete(ctx, s.namespacedKey(key))
}

func (s *SandboxedHostAPI) namespacedKey(key string) string {
	return "plugin:" + s.pluginName + ":" + key
}

func (s *SandboxedHostAPI) HTTPRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) (int, []byte, error) {
	if !s.hasPermission("http", "") {
		s.stats.Errors.Add(1)
		return 0, nil, fmt.Errorf("plugin %q: HTTP outbound access not granted", s.pluginName)
	}
	if err := s.checkHTTPAccess(url); err != nil {
		s.stats.Errors.Add(1)
		return 0, nil, err
	}
	if s.httpRequests.enabled() && !s.httpRequests.allow() {
		s.stats.Errors.Add(1)
		return 0, nil, fmt.Errorf("plugin %q: HTTP request rate limit exceeded", s.pluginName)
	}
	s.stats.HTTPRequests.Add(1)
	s.stats.LastCallAt.Store(time.Now().UnixMilli())
	return s.inner.HTTPRequest(ctx, method, url, headers, body)
}

func (s *SandboxedHostAPI) SendEmail(ctx context.Context, to, subject, body string, html bool) error {
	if !s.hasPermission("email", "") {
		s.stats.Errors.Add(1)
		return fmt.Errorf("plugin %q: email access not granted", s.pluginName)
	}

	// Validate recipient against allowed domains
	if !s.isEmailRecipientAllowed(to) {
		s.stats.Errors.Add(1)
		return fmt.Errorf("plugin %q: email recipient %q not allowed", s.pluginName, to)
	}

	// Apply email rate limiting (default 10 emails per minute if not configured)
	if !s.emailRateLimit.allow() {
		s.stats.Errors.Add(1)
		return fmt.Errorf("plugin %q: email rate limit exceeded", s.pluginName)
	}

	s.stats.LastCallAt.Store(time.Now().UnixMilli())
	return s.inner.SendEmail(ctx, to, subject, body, html)
}

func (s *SandboxedHostAPI) Log(ctx context.Context, level, message string, fields map[string]any) {
	// Logging is always allowed but we tag it with the plugin name
	if fields == nil {
		fields = make(map[string]any)
	}
	fields["plugin"] = s.pluginName
	s.inner.Log(ctx, level, message, fields)
}

func (s *SandboxedHostAPI) ConfigGet(ctx context.Context, key string) (string, error) {
	if !s.hasPermission("config", "read") {
		s.stats.Errors.Add(1)
		return "", fmt.Errorf("plugin %q: config access not granted", s.pluginName)
	}

	// Check if key is allowed by scope or blocked by sensitive patterns
	if !s.isConfigKeyAllowed(key) {
		s.stats.Errors.Add(1)
		return "", fmt.Errorf("plugin %q: config key %q access denied", s.pluginName, key)
	}

	return s.inner.ConfigGet(ctx, key)
}

func (s *SandboxedHostAPI) Translate(ctx context.Context, key string, args ...any) string {
	// Translation is always allowed
	return s.inner.Translate(ctx, key, args...)
}

func (s *SandboxedHostAPI) CallPlugin(ctx context.Context, pluginName, fn string, args json.RawMessage) (json.RawMessage, error) {
	if !s.hasPermission("plugin_call", "") {
		s.stats.Errors.Add(1)
		return nil, fmt.Errorf("plugin %q: plugin-to-plugin calls not granted", s.pluginName)
	}

	// Check scope: which plugins are we allowed to call?
	scope := s.permissionScope("plugin_call")
	if len(scope) > 0 {
		allowed := false
		for _, name := range scope {
			if name == pluginName || name == "*" {
				allowed = true
				break
			}
		}
		if !allowed {
			s.stats.Errors.Add(1)
			return nil, fmt.Errorf("plugin %q: not permitted to call plugin %q (allowed: %v)", s.pluginName, pluginName, scope)
		}
	}

	// Prevent infinite plugin-to-plugin call loops
	const maxCallDepth = 10
	depth := callDepthFromContext(ctx) + 1
	if depth > maxCallDepth {
		s.stats.Errors.Add(1)
		return nil, fmt.Errorf("plugin call depth exceeded (max %d): %s -> %s", maxCallDepth, s.pluginName, pluginName)
	}
	ctx = contextWithCallDepth(ctx, depth)

	s.stats.Calls.Add(1)
	s.stats.LastCallAt.Store(time.Now().UnixMilli())
	return s.inner.CallPlugin(ctx, pluginName, fn, args)
}

// --- Simple sliding window rate limiter ---

type rateLimiter struct {
	mu       sync.Mutex
	max      int
	window   time.Duration
	tokens   []time.Time
	disabled bool
}

func newRateLimiter(max int, window time.Duration) rateLimiter {
	return rateLimiter{
		max:    max,
		window: window,
		tokens: make([]time.Time, 0, max),
	}
}

func (r *rateLimiter) enabled() bool {
	return !r.disabled && r.max > 0
}

func (r *rateLimiter) allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	// Evict expired tokens
	valid := 0
	for _, t := range r.tokens {
		if t.After(cutoff) {
			r.tokens[valid] = t
			valid++
		}
	}
	r.tokens = r.tokens[:valid]

	if len(r.tokens) >= r.max {
		return false
	}

	r.tokens = append(r.tokens, now)
	return true
}

// extractTableNames extracts table names from SQL queries.
// This is a simple parser that handles common SQL patterns but isn't foolproof.
// It's designed as defense-in-depth, not the only security layer.
func extractTableNames(query string) []string {
	// Normalize the query - remove extra whitespace and convert to uppercase
	normalized := regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(strings.ToUpper(query)), " ")
	
	tables := make(map[string]bool)
	
	// Patterns to match table names after key SQL keywords
	patterns := []*regexp.Regexp{
		// FROM clause: SELECT ... FROM table1
		regexp.MustCompile(`\bFROM\s+([a-zA-Z_][a-zA-Z0-9_]*)`),
		// JOIN clauses: ... JOIN table2 ON ...
		regexp.MustCompile(`\b(?:INNER\s+|LEFT\s+|RIGHT\s+|FULL\s+|CROSS\s+)?JOIN\s+([a-zA-Z_][a-zA-Z0-9_]*)`),
		// INSERT INTO: INSERT INTO table_name
		regexp.MustCompile(`\bINSERT\s+INTO\s+([a-zA-Z_][a-zA-Z0-9_]*)`),
		// UPDATE: UPDATE table_name SET
		regexp.MustCompile(`\bUPDATE\s+([a-zA-Z_][a-zA-Z0-9_]*)`),
		// DELETE FROM: DELETE FROM table_name
		regexp.MustCompile(`\bDELETE\s+FROM\s+([a-zA-Z_][a-zA-Z0-9_]*)`),
		// CREATE/DROP/ALTER TABLE
		regexp.MustCompile(`\b(?:CREATE|DROP|ALTER)\s+TABLE\s+([a-zA-Z_][a-zA-Z0-9_]*)`),
		// TRUNCATE TABLE
		regexp.MustCompile(`\bTRUNCATE\s+(?:TABLE\s+)?([a-zA-Z_][a-zA-Z0-9_]*)`),
	}
	
	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(normalized, -1)
		for _, match := range matches {
			if len(match) > 1 {
				tableName := strings.ToLower(match[1]) // Store in lowercase for consistency
				tables[tableName] = true
			}
		}
	}
	
	// Convert map to slice
	result := make([]string, 0, len(tables))
	for table := range tables {
		result = append(result, table)
	}
	
	return result
}

// isTableAllowed checks if a table name is allowed by the scope patterns.
func isTableAllowed(table string, scope []string) bool {
	table = strings.ToLower(table)
	
	for _, pattern := range scope {
		pattern = strings.ToLower(pattern)
		
		// Exact match
		if pattern == table {
			return true
		}
		
		// Wildcard pattern (e.g., "user_*" matches "user_profiles")
		if strings.Contains(pattern, "*") {
			if matched, _ := regexp.MatchString(strings.ReplaceAll(regexp.QuoteMeta(pattern), `\*`, `.*`), table); matched {
				return true
			}
		}
	}
	
	return false
}

// Context keys and helpers for plugin call depth tracking
type contextKey string

const callDepthKey contextKey = "plugin_call_depth"

// callDepthFromContext extracts the current plugin call depth from context.
func callDepthFromContext(ctx context.Context) int {
	if depth, ok := ctx.Value(callDepthKey).(int); ok {
		return depth
	}
	return 0
}

// contextWithCallDepth creates a new context with the given call depth.
func contextWithCallDepth(ctx context.Context, depth int) context.Context {
	return context.WithValue(ctx, callDepthKey, depth)
}

// Sensitive configuration key patterns that plugins should not access
var sensitiveConfigPatterns = []string{
	"database.", "db.", "mysql.", "postgres.", "mariadb.",
	"smtp.", "mail.", "email.", 
	"secret", "password", "credential", "token", "key",
	"private", "auth", "session", "cookie",
	"ldap.", "oauth.", "saml.",
	"aws.", "gcp.", "azure.", "cloud.",
}

// isConfigKeyAllowed checks if a configuration key is allowed for plugin access.
func (s *SandboxedHostAPI) isConfigKeyAllowed(key string) bool {
	keyLower := strings.ToLower(key)
	
	// If plugin has a config scope, only allow keys that match scope patterns
	scope := s.permissionScope("config")
	if len(scope) > 0 {
		for _, pattern := range scope {
			if strings.Contains(pattern, "*") {
				// Wildcard pattern matching
				if matched, _ := regexp.MatchString(strings.ReplaceAll(regexp.QuoteMeta(strings.ToLower(pattern)), `\*`, `.*`), keyLower); matched {
					return true
				}
			} else if strings.ToLower(pattern) == keyLower {
				// Exact match
				return true
			}
		}
		return false // Not in scope
	}
	
	// No scope specified - check against sensitive patterns
	for _, pattern := range sensitiveConfigPatterns {
		if strings.Contains(keyLower, pattern) {
			return false
		}
	}
	
	return true
}

// isEmailRecipientAllowed checks if an email recipient is allowed based on the email permission scope.
func (s *SandboxedHostAPI) isEmailRecipientAllowed(recipient string) bool {
	scope := s.permissionScope("email")
	if len(scope) == 0 {
		return true // No restrictions
	}
	
	recipient = strings.ToLower(recipient)
	
	for _, pattern := range scope {
		pattern = strings.ToLower(pattern)
		
		// Domain pattern: @example.com matches any user at example.com
		if strings.HasPrefix(pattern, "@") {
			if strings.HasSuffix(recipient, pattern) {
				return true
			}
		} else if pattern == recipient {
			// Exact email match
			return true
		}
	}
	
	return false
}
