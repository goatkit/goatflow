package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/goatkit/goatflow/internal/platform/organisation"
	plugin "github.com/goatkit/goatflow/pkg/plugin"
)

// SandboxedHostAPI wraps a HostAPI with per-plugin permission enforcement
// and resource accounting. Each plugin gets its own sandbox instance.
type SandboxedHostAPI struct {
	inner      HostAPI
	pluginName string
	policyMu   sync.RWMutex
	policy     *ResourcePolicy // pointer for live updates

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
		policy:     &policy, // Store pointer for live updates
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

// accessGranted reports whether a permission entry's access level covers the
// wanted access. "readwrite" covers read and write; "hard_delete" is only
// granted by an entry that names it.
func accessGranted(have, want string) bool {
	switch {
	case want == "", have == want:
		return true
	case have == "readwrite":
		return want == "read" || want == "write"
	}
	return false
}

// grant unions every policy entry of permType that grants access. ok is false
// when no entry grants it (or the plugin is blocked); all is true when one of
// the granting entries has no scope; patterns holds the scopes of the others.
func (s *SandboxedHostAPI) grant(permType, access string) (patterns []string, all, ok bool) {
	s.policyMu.RLock()
	defer s.policyMu.RUnlock()

	if s.policy.Status == "blocked" {
		return nil, false, false
	}
	for _, p := range s.policy.Permissions {
		if p.Type != permType || !accessGranted(p.Access, access) {
			continue
		}
		ok = true
		if len(p.Scope) == 0 {
			all = true
		} else {
			patterns = append(patterns, p.Scope...)
		}
	}
	return patterns, all, ok
}

// hasPermission checks if the policy grants a specific permission type and access level.
func (s *SandboxedHostAPI) hasPermission(permType, access string) bool {
	_, _, ok := s.grant(permType, access)
	return ok
}

// checkDBTableAccess validates the tables a statement touches against the
// db grants and blocks DDL for plugins without write access. Tables the
// statement writes need a write grant (also through DBQuery); every other
// table needs a read grant. information_schema is readable with any read
// grant: plugin migrations probe it for existing columns.
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

	readScope, readAll, readOK := s.grant("db", "read")
	writeScope, writeAll, writeOK := s.grant("db", "write")
	read, write := sqlTables(query)
	written := make(map[string]bool, len(write))
	for _, table := range write {
		if !writeOK || !(writeAll || isTableAllowed(table, writeScope)) {
			return fmt.Errorf("plugin %q: write access to table %q not permitted", s.pluginName, table)
		}
		written[table] = true
	}
	for _, table := range read {
		if written[table] {
			continue
		}
		if readOK && (readAll || strings.HasPrefix(table, "information_schema.") || isTableAllowed(table, readScope)) {
			continue
		}
		return fmt.Errorf("plugin %q: access to table %q not permitted", s.pluginName, table)
	}
	return nil
}

// checkHTTPAccess validates that the URL matches the allowed patterns.
func (s *SandboxedHostAPI) checkHTTPAccess(url string) error {
	scope, all, _ := s.grant("http", "")
	if all {
		return nil // No URL restrictions
	}

	for _, pattern := range scope {
		if matchURLPattern(pattern, url) {
			return nil
		}
	}

	return fmt.Errorf("plugin %q: HTTP access to %q not permitted (allowed: %v)", s.pluginName, url, scope)
}

// checkEntityAccess gates the entity deletion methods. Scope lists the
// entity types the plugin may touch; an empty scope allows every type.
func (s *SandboxedHostAPI) checkEntityAccess(access, entityType string) error {
	scope, all, ok := s.grant("entity", access)
	if !ok {
		return fmt.Errorf("plugin %q: entity %s access not granted", s.pluginName, access)
	}
	if all || slices.Contains(scope, entityType) {
		return nil
	}
	return fmt.Errorf("plugin %q: entity %s access to %q not permitted (allowed: %v)", s.pluginName, access, entityType, scope)
}

// requirePermission counts a denial and returns the error when the policy
// does not grant permType with access.
func (s *SandboxedHostAPI) requirePermission(permType, access string) error {
	if s.hasPermission(permType, access) {
		s.stats.LastCallAt.Store(time.Now().UnixMilli())
		return nil
	}
	s.stats.Errors.Add(1)
	return fmt.Errorf("plugin %q: %s %s access not granted", s.pluginName, permType, access)
}

// matchURLPattern checks if a URL matches a scope pattern.
// Patterns: "*" matches every host, "*.example.com" matches the domain and
// its subdomains, "api.example.com" matches exact host.
func matchURLPattern(pattern, url string) bool {
	if pattern == "*" {
		return true
	}
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
	// Auto-scope query by active organisation if applicable.
	query, args = organisation.ScopeQuery(query, args, s.inner.OrgID(ctx))
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
	// Auto-scope query by active organisation if applicable.
	query, args = organisation.ScopeQuery(query, args, s.inner.OrgID(ctx))
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
	// Check scope: which plugins are we allowed to call?
	scope, all, ok := s.grant("plugin_call", "")
	if !ok {
		s.stats.Errors.Add(1)
		return nil, fmt.Errorf("plugin %q: plugin-to-plugin calls not granted", s.pluginName)
	}
	if !all && !slices.Contains(scope, pluginName) && !slices.Contains(scope, "*") {
		s.stats.Errors.Add(1)
		return nil, fmt.Errorf("plugin %q: not permitted to call plugin %q (allowed: %v)", s.pluginName, pluginName, scope)
	}

	// Prevent infinite plugin-to-plugin call loops
	const maxCallDepth = 10
	depth := callDepthFromContext(ctx) + 1
	if depth > maxCallDepth {
		s.stats.Errors.Add(1)
		return nil, fmt.Errorf("plugin call depth exceeded (max %d): %s -> %s", maxCallDepth, s.pluginName, pluginName)
	}
	ctx = contextWithCallDepth(ctx, depth)
	// Stamp the caller: the host then routes through Manager.CallFrom, which
	// never reads caller identity from the plugin-built args.
	ctx = context.WithValue(ctx, PluginCallerKey, s.pluginName)

	s.stats.Calls.Add(1)
	s.stats.LastCallAt.Store(time.Now().UnixMilli())
	return s.inner.CallPlugin(ctx, pluginName, fn, args)
}

// PublishEvent sends an SSE event to a named channel for connected browser clients.
// The plugin name is automatically set from the sandbox context.
func (s *SandboxedHostAPI) PublishEvent(ctx context.Context, channel string, eventType string, data string) error {
	// Inject the plugin name into context so the host knows the source.
	ctx = context.WithValue(ctx, PluginCallerKey, s.pluginName)
	return s.inner.PublishEvent(ctx, channel, eventType, data)
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

// isTableAllowed checks if a table name is allowed by the scope patterns.
func isTableAllowed(table string, scope []string) bool {
	for _, pattern := range scope {
		if matchWildcard(pattern, table) {
			return true
		}
	}
	return false
}

// matchWildcard matches name against pattern case-insensitively. "*" in the
// pattern matches any run of characters; the match covers the whole name, so
// "gk_coach_*" matches "gk_coach_session" but not "xgk_coach_session".
func matchWildcard(pattern, name string) bool {
	pattern, name = strings.ToLower(pattern), strings.ToLower(name)
	if !strings.Contains(pattern, "*") {
		return pattern == name
	}
	re := "^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), `\*`, `.*`) + "$"
	matched, _ := regexp.MatchString(re, name)
	return matched
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
// Keys matching a scope pattern are allowed. When a config grant has no
// scope, any other key that matches no sensitive pattern is allowed too.
func (s *SandboxedHostAPI) isConfigKeyAllowed(key string) bool {
	scope, all, _ := s.grant("config", "read")
	for _, pattern := range scope {
		if matchWildcard(pattern, key) {
			return true
		}
	}
	if !all {
		return false // Not in scope
	}

	keyLower := strings.ToLower(key)
	for _, pattern := range sensitiveConfigPatterns {
		if strings.Contains(keyLower, pattern) {
			return false
		}
	}

	return true
}

// EntitySoftDelete soft-deletes an entity. Needs entity write.
func (s *SandboxedHostAPI) EntitySoftDelete(ctx context.Context, entityType string, entityID int64, reason string) error {
	if err := s.checkEntityAccess("write", entityType); err != nil {
		s.stats.Errors.Add(1)
		return err
	}
	s.stats.LastCallAt.Store(time.Now().UnixMilli())
	ctx = context.WithValue(ctx, PluginCallerKey, s.pluginName)
	return s.inner.EntitySoftDelete(ctx, entityType, entityID, reason)
}

// EntityRestore restores a soft-deleted entity. Needs entity write.
func (s *SandboxedHostAPI) EntityRestore(ctx context.Context, entityType string, entityID int64) error {
	if err := s.checkEntityAccess("write", entityType); err != nil {
		s.stats.Errors.Add(1)
		return err
	}
	s.stats.LastCallAt.Store(time.Now().UnixMilli())
	ctx = context.WithValue(ctx, PluginCallerKey, s.pluginName)
	return s.inner.EntityRestore(ctx, entityType, entityID)
}

// EntityHardDelete permanently removes an entity. Needs an explicit entity
// "hard_delete" grant: "readwrite" does not cover it, and the host never
// grants it from a plugin's own declarations.
func (s *SandboxedHostAPI) EntityHardDelete(ctx context.Context, entityType string, entityID int64, reason string) error {
	if err := s.checkEntityAccess("hard_delete", entityType); err != nil {
		s.stats.Errors.Add(1)
		return err
	}
	s.stats.LastCallAt.Store(time.Now().UnixMilli())
	ctx = context.WithValue(ctx, PluginCallerKey, s.pluginName)
	return s.inner.EntityHardDelete(ctx, entityType, entityID, reason)
}

// RecycleBinList lists soft-deleted entities. Needs entity read.
func (s *SandboxedHostAPI) RecycleBinList(ctx context.Context, entityType string) (json.RawMessage, error) {
	if err := s.checkEntityAccess("read", entityType); err != nil {
		s.stats.Errors.Add(1)
		return nil, err
	}
	s.stats.LastCallAt.Store(time.Now().UnixMilli())
	return s.inner.RecycleBinList(ctx, entityType)
}

// SecureConfigGet retrieves a decrypted secret, scoped to this plugin.
func (s *SandboxedHostAPI) SecureConfigGet(ctx context.Context, key string) (string, error) {
	ctx = context.WithValue(ctx, PluginCallerKey, s.pluginName)
	return s.inner.SecureConfigGet(ctx, key)
}

// SecureConfigSet stores an encrypted secret, scoped to this plugin.
func (s *SandboxedHostAPI) SecureConfigSet(ctx context.Context, key string, value string) error {
	ctx = context.WithValue(ctx, PluginCallerKey, s.pluginName)
	return s.inner.SecureConfigSet(ctx, key, value)
}

// OrgID returns the active organisation ID from the request context.
func (s *SandboxedHostAPI) OrgID(ctx context.Context) int64 {
	return s.inner.OrgID(ctx)
}

// CustomFieldsGet retrieves custom field values, scoped to this plugin's prefixed fields + admin/legacy fields.
func (s *SandboxedHostAPI) CustomFieldsGet(ctx context.Context, entityType string, objectID int64, fields []string) (map[string]any, error) {
	if s.dbQueries.enabled() && !s.dbQueries.allow() {
		s.stats.Errors.Add(1)
		return nil, fmt.Errorf("plugin %q: DB query rate limit exceeded", s.pluginName)
	}
	s.stats.DBQueries.Add(1)
	s.stats.LastCallAt.Store(time.Now().UnixMilli())

	// Prefix field names with plugin name for plugin-owned fields.
	prefixed := s.prefixFieldNames(fields)
	result, err := s.inner.CustomFieldsGet(ctx, entityType, objectID, prefixed)
	if err != nil {
		s.stats.Errors.Add(1)
		return nil, err
	}

	// Strip plugin prefix from result keys.
	return s.stripPrefixFromResults(result), nil
}

// CustomFieldsSet stores custom field values, scoped to this plugin's fields.
func (s *SandboxedHostAPI) CustomFieldsSet(ctx context.Context, entityType string, objectID int64, values map[string]any) error {
	if s.dbQueries.enabled() && !s.dbQueries.allow() {
		s.stats.Errors.Add(1)
		return fmt.Errorf("plugin %q: DB query rate limit exceeded", s.pluginName)
	}
	s.stats.DBExecs.Add(1)
	s.stats.LastCallAt.Store(time.Now().UnixMilli())

	// Prefix field names.
	prefixed := make(map[string]any, len(values))
	for name, val := range values {
		prefixed[s.prefixFieldName(name)] = val
	}
	if err := s.inner.CustomFieldsSet(ctx, entityType, objectID, prefixed); err != nil {
		s.stats.Errors.Add(1)
		return err
	}
	return nil
}

// CustomFieldsQuery finds entities by custom field values, scoped to this plugin's fields.
func (s *SandboxedHostAPI) CustomFieldsQuery(ctx context.Context, entityType string, filters []CustomFieldFilter) ([]int64, error) {
	if s.dbQueries.enabled() && !s.dbQueries.allow() {
		s.stats.Errors.Add(1)
		return nil, fmt.Errorf("plugin %q: DB query rate limit exceeded", s.pluginName)
	}
	s.stats.DBQueries.Add(1)
	s.stats.LastCallAt.Store(time.Now().UnixMilli())

	// Prefix field names in filters.
	prefixed := make([]CustomFieldFilter, len(filters))
	for i, f := range filters {
		prefixed[i] = CustomFieldFilter{
			Field:    s.prefixFieldName(f.Field),
			Operator: f.Operator,
			Value:    f.Value,
			Value2:   f.Value2,
		}
	}
	result, err := s.inner.CustomFieldsQuery(ctx, entityType, prefixed)
	if err != nil {
		s.stats.Errors.Add(1)
		return nil, err
	}
	return result, nil
}

// ---- Articles, tickets and plugin files (plugin name injected into context) ----

// CreateArticleAttachment attaches a file to an article. Needs article write.
func (s *SandboxedHostAPI) CreateArticleAttachment(ctx context.Context, articleID, createdBy int64, filename, contentType string, content []byte) (int64, error) {
	if err := s.requirePermission("article", "write"); err != nil {
		return 0, err
	}
	ctx = context.WithValue(ctx, PluginCallerKey, s.pluginName)
	return s.inner.CreateArticleAttachment(ctx, articleID, createdBy, filename, contentType, content)
}

// ListArticleAttachments lists an article's attachments. Needs article read.
func (s *SandboxedHostAPI) ListArticleAttachments(ctx context.Context, articleID int64) ([]plugin.ArticleAttachment, error) {
	if err := s.requirePermission("article", "read"); err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, PluginCallerKey, s.pluginName)
	return s.inner.ListArticleAttachments(ctx, articleID)
}

// DeleteArticleAttachment removes an attachment. Needs article write.
func (s *SandboxedHostAPI) DeleteArticleAttachment(ctx context.Context, articleID, attachmentID int64) error {
	if err := s.requirePermission("article", "write"); err != nil {
		return err
	}
	ctx = context.WithValue(ctx, PluginCallerKey, s.pluginName)
	return s.inner.DeleteArticleAttachment(ctx, articleID, attachmentID)
}

// RenderMarkdownToPdf forwards to the inner host.
func (s *SandboxedHostAPI) RenderMarkdownToPdf(ctx context.Context, markdown string, options plugin.PdfRenderOptions) ([]byte, error) {
	s.stats.LastCallAt.Store(time.Now().UnixMilli())
	ctx = context.WithValue(ctx, PluginCallerKey, s.pluginName)
	return s.inner.RenderMarkdownToPdf(ctx, markdown, options)
}

// CreateArticle adds an article to a ticket. Needs article write.
func (s *SandboxedHostAPI) CreateArticle(ctx context.Context, ticketID, createdBy int64, subject, body string, visibleToCustomer bool) (int64, error) {
	if err := s.requirePermission("article", "write"); err != nil {
		return 0, err
	}
	ctx = context.WithValue(ctx, PluginCallerKey, s.pluginName)
	return s.inner.CreateArticle(ctx, ticketID, createdBy, subject, body, visibleToCustomer)
}

// ChangeTicketStatus changes a ticket's state. Needs ticket write.
func (s *SandboxedHostAPI) ChangeTicketStatus(ctx context.Context, ticketID, stateID, userID int64, untilTime int64) error {
	if err := s.requirePermission("ticket", "write"); err != nil {
		return err
	}
	ctx = context.WithValue(ctx, PluginCallerKey, s.pluginName)
	return s.inner.ChangeTicketStatus(ctx, ticketID, stateID, userID, untilTime)
}

// ListTicketStates lists the valid ticket states. Needs ticket read.
func (s *SandboxedHostAPI) ListTicketStates(ctx context.Context) ([]plugin.TicketStateInfo, error) {
	if err := s.requirePermission("ticket", "read"); err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, PluginCallerKey, s.pluginName)
	return s.inner.ListTicketStates(ctx)
}

// ListTicketViews lists the plugin ticket views. Needs ticket read.
func (s *SandboxedHostAPI) ListTicketViews(ctx context.Context) ([]plugin.TicketViewInfo, error) {
	if err := s.requirePermission("ticket", "read"); err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, PluginCallerKey, s.pluginName)
	return s.inner.ListTicketViews(ctx)
}

// StoreFile stores a file in the plugin's namespace. Needs file write.
func (s *SandboxedHostAPI) StoreFile(ctx context.Context, key string, data []byte, metadata map[string]string) error {
	if err := s.requirePermission("file", "write"); err != nil {
		return err
	}
	ctx = context.WithValue(ctx, PluginCallerKey, s.pluginName)
	return s.inner.StoreFile(ctx, key, data, metadata)
}

// GetFile reads a file from the plugin's namespace. Needs file read.
func (s *SandboxedHostAPI) GetFile(ctx context.Context, key string) ([]byte, map[string]string, error) {
	if err := s.requirePermission("file", "read"); err != nil {
		return nil, nil, err
	}
	ctx = context.WithValue(ctx, PluginCallerKey, s.pluginName)
	return s.inner.GetFile(ctx, key)
}

// DeleteFile removes a file from the plugin's namespace. Needs file write.
func (s *SandboxedHostAPI) DeleteFile(ctx context.Context, key string) error {
	if err := s.requirePermission("file", "write"); err != nil {
		return err
	}
	ctx = context.WithValue(ctx, PluginCallerKey, s.pluginName)
	return s.inner.DeleteFile(ctx, key)
}

// ListFiles lists files in the plugin's namespace. Needs file read.
func (s *SandboxedHostAPI) ListFiles(ctx context.Context, prefix string) ([]FileInfo, error) {
	if err := s.requirePermission("file", "read"); err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, PluginCallerKey, s.pluginName)
	return s.inner.ListFiles(ctx, prefix)
}

// GenerateThumbnail is pure image processing on bytes the plugin already
// holds, so it needs no permission.
func (s *SandboxedHostAPI) GenerateThumbnail(ctx context.Context, data []byte, contentType string, maxWidth, maxHeight int) ([]byte, string, error) {
	s.stats.LastCallAt.Store(time.Now().UnixMilli())
	ctx = context.WithValue(ctx, PluginCallerKey, s.pluginName)
	return s.inner.GenerateThumbnail(ctx, data, contentType, maxWidth, maxHeight)
}

// fieldPrefix returns the plugin name sanitised for use as a custom field
// prefix. Hyphens are replaced with underscores so the result passes the
// field-name regex (^[a-z][a-z0-9_]*$). Called at both registration time
// (manager.go) and runtime (sandbox.go) so the two always agree.
func fieldPrefix(pluginName string) string {
	return strings.ReplaceAll(pluginName, "-", "_") + "_"
}

// prefixFieldName adds the plugin name prefix to a field name.
// If the name already has the prefix, or is an admin/legacy field, it's returned as-is.
func (s *SandboxedHostAPI) prefixFieldName(name string) string {
	prefix := fieldPrefix(s.pluginName)
	if strings.HasPrefix(name, prefix) {
		return name
	}
	return prefix + name
}

// prefixFieldNames prefixes a slice of field names. nil input returns nil (get all).
func (s *SandboxedHostAPI) prefixFieldNames(fields []string) []string {
	if fields == nil {
		return nil // Return all fields — inner layer handles it.
	}
	result := make([]string, len(fields))
	for i, f := range fields {
		result[i] = s.prefixFieldName(f)
	}
	return result
}

// stripPrefixFromResults removes the plugin prefix from result map keys.
func (s *SandboxedHostAPI) stripPrefixFromResults(m map[string]any) map[string]any {
	prefix := fieldPrefix(s.pluginName)
	result := make(map[string]any, len(m))
	for k, v := range m {
		stripped := strings.TrimPrefix(k, prefix)
		result[stripped] = v
	}
	return result
}

// isEmailRecipientAllowed checks if an email recipient is allowed based on the email permission scope.
func (s *SandboxedHostAPI) isEmailRecipientAllowed(recipient string) bool {
	scope, all, _ := s.grant("email", "")
	if all {
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
