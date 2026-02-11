package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockInnerHostAPI records calls for assertion.
type mockInnerHostAPI struct {
	queryCalls  int
	execCalls   int
	cacheCalls  int
	httpCalls   int
	emailCalls  int
	configCalls int
	callCalls   int
	lastCacheKey string
}

func (m *mockInnerHostAPI) DBQuery(_ context.Context, _ string, _ ...any) ([]map[string]any, error) {
	m.queryCalls++
	return nil, nil
}
func (m *mockInnerHostAPI) DBExec(_ context.Context, _ string, _ ...any) (int64, error) {
	m.execCalls++
	return 0, nil
}
func (m *mockInnerHostAPI) CacheGet(_ context.Context, key string) ([]byte, bool, error) {
	m.cacheCalls++
	m.lastCacheKey = key
	return nil, false, nil
}
func (m *mockInnerHostAPI) CacheSet(_ context.Context, key string, _ []byte, _ int) error {
	m.cacheCalls++
	m.lastCacheKey = key
	return nil
}
func (m *mockInnerHostAPI) CacheDelete(_ context.Context, key string) error {
	m.cacheCalls++
	m.lastCacheKey = key
	return nil
}
func (m *mockInnerHostAPI) HTTPRequest(_ context.Context, _, _ string, _ map[string]string, _ []byte) (int, []byte, error) {
	m.httpCalls++
	return 200, nil, nil
}
func (m *mockInnerHostAPI) SendEmail(_ context.Context, _, _, _ string, _ bool) error {
	m.emailCalls++
	return nil
}
func (m *mockInnerHostAPI) Log(_ context.Context, _, _ string, _ map[string]any) {}
func (m *mockInnerHostAPI) ConfigGet(_ context.Context, _ string) (string, error) {
	m.configCalls++
	return "val", nil
}
func (m *mockInnerHostAPI) Translate(_ context.Context, key string, _ ...any) string { return key }
func (m *mockInnerHostAPI) CallPlugin(_ context.Context, _, _ string, _ json.RawMessage) (json.RawMessage, error) {
	m.callCalls++
	return nil, nil
}

func (m *mockInnerHostAPI) PublishEvent(ctx context.Context, eventType string, data string) error {
	return nil
}

func TestSandbox_DBReadOnly(t *testing.T) {
	inner := &mockInnerHostAPI{}
	policy := ResourcePolicy{
		Status:      "approved",
		Permissions: []Permission{{Type: "db", Access: "read"}},
	}
	s := NewSandboxedHostAPI(inner, "test-plugin", policy)
	ctx := context.Background()

	// Read should work
	_, err := s.DBQuery(ctx, "SELECT * FROM tickets")
	if err != nil {
		t.Errorf("DBQuery should work with read permission: %v", err)
	}
	if inner.queryCalls != 1 {
		t.Errorf("expected 1 query call, got %d", inner.queryCalls)
	}

	// Write should be blocked
	_, err = s.DBExec(ctx, "INSERT INTO tickets (title) VALUES ('test')")
	if err == nil {
		t.Error("DBExec should fail with read-only permission")
	}
	if inner.execCalls != 0 {
		t.Error("inner DBExec should not have been called")
	}
}

func TestSandbox_DBReadWrite(t *testing.T) {
	inner := &mockInnerHostAPI{}
	policy := ResourcePolicy{
		Status:      "approved",
		Permissions: []Permission{{Type: "db", Access: "readwrite"}},
	}
	s := NewSandboxedHostAPI(inner, "test-plugin", policy)
	ctx := context.Background()

	_, err := s.DBQuery(ctx, "SELECT 1")
	if err != nil {
		t.Errorf("DBQuery should work: %v", err)
	}

	_, err = s.DBExec(ctx, "UPDATE tickets SET title='x' WHERE id=1")
	if err != nil {
		t.Errorf("DBExec should work with readwrite: %v", err)
	}
}

func TestSandbox_DBBlocksDDL(t *testing.T) {
	inner := &mockInnerHostAPI{}
	policy := ResourcePolicy{
		Status:      "approved",
		Permissions: []Permission{{Type: "db", Access: "read"}},
	}
	s := NewSandboxedHostAPI(inner, "test-plugin", policy)
	ctx := context.Background()

	ddlStatements := []string{
		"DROP TABLE tickets",
		"ALTER TABLE tickets ADD COLUMN x INT",
		"TRUNCATE TABLE tickets",
		"CREATE TABLE evil (id INT)",
		"GRANT ALL ON tickets TO hacker",
	}

	for _, stmt := range ddlStatements {
		_, err := s.DBQuery(ctx, stmt)
		if err == nil {
			t.Errorf("DDL should be blocked: %s", stmt)
			continue
		}
		if !strings.Contains(err.Error(), "DDL") {
			t.Errorf("error should mention DDL: %v", err)
		}
	}
}

func TestSandbox_NoDB(t *testing.T) {
	inner := &mockInnerHostAPI{}
	policy := ResourcePolicy{
		Status:      "approved",
		Permissions: []Permission{}, // No DB permission
	}
	s := NewSandboxedHostAPI(inner, "test-plugin", policy)
	ctx := context.Background()

	_, err := s.DBQuery(ctx, "SELECT 1")
	if err == nil {
		t.Error("should fail without db permission")
	}
}

func TestSandbox_CacheNamespacing(t *testing.T) {
	inner := &mockInnerHostAPI{}
	policy := ResourcePolicy{
		Status:      "approved",
		Permissions: []Permission{{Type: "cache", Access: "readwrite"}},
	}
	s := NewSandboxedHostAPI(inner, "stats", policy)
	ctx := context.Background()

	s.CacheGet(ctx, "mykey")
	if inner.lastCacheKey != "plugin:stats:mykey" {
		t.Errorf("expected namespaced key 'plugin:stats:mykey', got %q", inner.lastCacheKey)
	}

	s.CacheSet(ctx, "another", nil, 60)
	if inner.lastCacheKey != "plugin:stats:another" {
		t.Errorf("expected namespaced key 'plugin:stats:another', got %q", inner.lastCacheKey)
	}

	s.CacheDelete(ctx, "old")
	if inner.lastCacheKey != "plugin:stats:old" {
		t.Errorf("expected namespaced key 'plugin:stats:old', got %q", inner.lastCacheKey)
	}
}

func TestSandbox_CacheBlocked(t *testing.T) {
	inner := &mockInnerHostAPI{}
	policy := ResourcePolicy{
		Status:      "approved",
		Permissions: []Permission{{Type: "cache", Access: "read"}},
	}
	s := NewSandboxedHostAPI(inner, "test-plugin", policy)
	ctx := context.Background()

	// Read should work
	_, _, err := s.CacheGet(ctx, "key")
	if err != nil {
		t.Errorf("CacheGet should work with read: %v", err)
	}

	// Write should fail
	err = s.CacheSet(ctx, "key", []byte("val"), 60)
	if err == nil {
		t.Error("CacheSet should fail with read-only")
	}

	err = s.CacheDelete(ctx, "key")
	if err == nil {
		t.Error("CacheDelete should fail with read-only")
	}
}

func TestSandbox_HTTPWithScope(t *testing.T) {
	inner := &mockInnerHostAPI{}
	policy := ResourcePolicy{
		Status: "approved",
		Permissions: []Permission{
			{Type: "http", Scope: []string{"*.tenor.com", "api.giphy.com"}},
		},
	}
	s := NewSandboxedHostAPI(inner, "test-plugin", policy)
	ctx := context.Background()

	// Allowed URLs
	_, _, err := s.HTTPRequest(ctx, "GET", "https://api.tenor.com/v2/search?q=cat", nil, nil)
	if err != nil {
		t.Errorf("tenor should be allowed: %v", err)
	}

	_, _, err = s.HTTPRequest(ctx, "GET", "https://api.giphy.com/v1/gifs/search", nil, nil)
	if err != nil {
		t.Errorf("giphy should be allowed: %v", err)
	}

	// Blocked URL
	_, _, err = s.HTTPRequest(ctx, "GET", "https://evil.com/steal-data", nil, nil)
	if err == nil {
		t.Error("evil.com should be blocked")
	}
}

func TestSandbox_HTTPNoScope(t *testing.T) {
	inner := &mockInnerHostAPI{}
	policy := ResourcePolicy{
		Status:      "approved",
		Permissions: []Permission{{Type: "http"}}, // No scope = allow all
	}
	s := NewSandboxedHostAPI(inner, "test-plugin", policy)
	ctx := context.Background()

	_, _, err := s.HTTPRequest(ctx, "GET", "https://anywhere.com", nil, nil)
	if err != nil {
		t.Errorf("should allow any URL without scope: %v", err)
	}
}

func TestSandbox_HTTPBlocked(t *testing.T) {
	inner := &mockInnerHostAPI{}
	policy := ResourcePolicy{
		Status:      "approved",
		Permissions: []Permission{}, // No HTTP permission
	}
	s := NewSandboxedHostAPI(inner, "test-plugin", policy)
	ctx := context.Background()

	_, _, err := s.HTTPRequest(ctx, "GET", "https://api.tenor.com/v2/search", nil, nil)
	if err == nil {
		t.Error("should fail without http permission")
	}
}

func TestSandbox_EmailBlocked(t *testing.T) {
	inner := &mockInnerHostAPI{}
	policy := ResourcePolicy{
		Status:      "approved",
		Permissions: []Permission{},
	}
	s := NewSandboxedHostAPI(inner, "test-plugin", policy)
	ctx := context.Background()

	err := s.SendEmail(ctx, "test@example.com", "hi", "body", false)
	if err == nil {
		t.Error("should fail without email permission")
	}
}

func TestSandbox_EmailAllowed(t *testing.T) {
	inner := &mockInnerHostAPI{}
	policy := ResourcePolicy{
		Status:      "approved",
		Permissions: []Permission{{Type: "email"}},
	}
	s := NewSandboxedHostAPI(inner, "test-plugin", policy)
	ctx := context.Background()

	err := s.SendEmail(ctx, "test@example.com", "hi", "body", false)
	if err != nil {
		t.Errorf("should work with email permission: %v", err)
	}
	if inner.emailCalls != 1 {
		t.Error("inner SendEmail should have been called")
	}
}

func TestSandbox_PluginCallScope(t *testing.T) {
	inner := &mockInnerHostAPI{}
	policy := ResourcePolicy{
		Status: "approved",
		Permissions: []Permission{
			{Type: "plugin_call", Scope: []string{"stats"}},
		},
	}
	s := NewSandboxedHostAPI(inner, "fictus", policy)
	ctx := context.Background()

	// Allowed
	_, err := s.CallPlugin(ctx, "stats", "overview", nil)
	if err != nil {
		t.Errorf("calling stats should work: %v", err)
	}

	// Blocked
	_, err = s.CallPlugin(ctx, "evil-plugin", "steal", nil)
	if err == nil {
		t.Error("calling evil-plugin should be blocked")
	}
}

func TestSandbox_PluginCallWildcard(t *testing.T) {
	inner := &mockInnerHostAPI{}
	policy := ResourcePolicy{
		Status: "approved",
		Permissions: []Permission{
			{Type: "plugin_call", Scope: []string{"*"}},
		},
	}
	s := NewSandboxedHostAPI(inner, "fictus", policy)
	ctx := context.Background()

	_, err := s.CallPlugin(ctx, "anything", "fn", nil)
	if err != nil {
		t.Errorf("wildcard should allow any plugin call: %v", err)
	}
}

func TestSandbox_ConfigBlocked(t *testing.T) {
	inner := &mockInnerHostAPI{}
	policy := ResourcePolicy{
		Status:      "approved",
		Permissions: []Permission{},
	}
	s := NewSandboxedHostAPI(inner, "test-plugin", policy)
	ctx := context.Background()

	_, err := s.ConfigGet(ctx, "app.name")
	if err == nil {
		t.Error("should fail without config permission")
	}
}

func TestSandbox_BlockedStatus(t *testing.T) {
	inner := &mockInnerHostAPI{}
	policy := ResourcePolicy{
		Status: "blocked",
		Permissions: []Permission{
			{Type: "db", Access: "readwrite"},
			{Type: "cache", Access: "readwrite"},
			{Type: "http"},
		},
	}
	s := NewSandboxedHostAPI(inner, "blocked-plugin", policy)
	ctx := context.Background()

	// Everything should fail when status is blocked
	_, err := s.DBQuery(ctx, "SELECT 1")
	if err == nil {
		t.Error("blocked plugin should not access DB")
	}
	_, _, err = s.HTTPRequest(ctx, "GET", "https://example.com", nil, nil)
	if err == nil {
		t.Error("blocked plugin should not access HTTP")
	}
}

func TestSandbox_LogAlwaysWorks(t *testing.T) {
	inner := &mockInnerHostAPI{}
	policy := ResourcePolicy{
		Status:      "approved",
		Permissions: []Permission{}, // No permissions at all
	}
	s := NewSandboxedHostAPI(inner, "test-plugin", policy)
	ctx := context.Background()

	// Should not panic
	s.Log(ctx, "info", "test message", nil)
	s.Log(ctx, "error", "with fields", map[string]any{"key": "val"})
}

func TestSandbox_TranslateAlwaysWorks(t *testing.T) {
	inner := &mockInnerHostAPI{}
	policy := ResourcePolicy{
		Status:      "approved",
		Permissions: []Permission{},
	}
	s := NewSandboxedHostAPI(inner, "test-plugin", policy)
	ctx := context.Background()

	result := s.Translate(ctx, "stats.title")
	if result != "stats.title" {
		t.Errorf("expected key passthrough, got %q", result)
	}
}

func TestSandbox_Stats(t *testing.T) {
	inner := &mockInnerHostAPI{}
	policy := ResourcePolicy{
		Status: "approved",
		Permissions: []Permission{
			{Type: "db", Access: "readwrite"},
			{Type: "cache", Access: "readwrite"},
			{Type: "http"},
		},
	}
	s := NewSandboxedHostAPI(inner, "test-plugin", policy)
	ctx := context.Background()

	s.DBQuery(ctx, "SELECT 1")
	s.DBQuery(ctx, "SELECT 2")
	s.DBExec(ctx, "INSERT INTO t VALUES (1)")
	s.CacheGet(ctx, "k")
	s.CacheSet(ctx, "k", nil, 60)
	s.HTTPRequest(ctx, "GET", "https://example.com", nil, nil)

	stats := s.Stats()
	if stats.PluginName != "test-plugin" {
		t.Errorf("expected plugin name test-plugin, got %q", stats.PluginName)
	}
	if stats.DBQueries != 2 {
		t.Errorf("expected 2 queries, got %d", stats.DBQueries)
	}
	if stats.DBExecs != 1 {
		t.Errorf("expected 1 exec, got %d", stats.DBExecs)
	}
	if stats.CacheOps != 2 {
		t.Errorf("expected 2 cache ops, got %d", stats.CacheOps)
	}
	if stats.HTTPRequests != 1 {
		t.Errorf("expected 1 http request, got %d", stats.HTTPRequests)
	}
	if stats.LastCallAt == 0 {
		t.Error("expected LastCallAt to be set")
	}
}

func TestSandbox_RateLimit(t *testing.T) {
	inner := &mockInnerHostAPI{}
	policy := ResourcePolicy{
		Status: "approved",
		Permissions: []Permission{
			{Type: "db", Access: "read"},
		},
		MaxDBQueriesPerMin: 3, // Very low for testing
	}
	s := NewSandboxedHostAPI(inner, "test-plugin", policy)
	ctx := context.Background()

	// First 3 should work
	for i := 0; i < 3; i++ {
		_, err := s.DBQuery(ctx, "SELECT 1")
		if err != nil {
			t.Errorf("query %d should work: %v", i, err)
		}
	}

	// 4th should be rate limited
	_, err := s.DBQuery(ctx, "SELECT 1")
	if err == nil {
		t.Error("4th query should be rate limited")
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("error should mention rate limit: %v", err)
	}
}

func TestSandbox_RateLimitConcurrent(t *testing.T) {
	inner := &mockInnerHostAPI{}
	policy := ResourcePolicy{
		Status: "approved",
		Permissions: []Permission{
			{Type: "db", Access: "read"},
		},
		MaxDBQueriesPerMin: 50,
	}
	s := NewSandboxedHostAPI(inner, "test-plugin", policy)
	ctx := context.Background()

	// Concurrent access should not race
	var wg sync.WaitGroup
	errors := make(chan error, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.DBQuery(ctx, "SELECT 1")
			if err != nil {
				errors <- err
			}
		}()
	}
	wg.Wait()
	close(errors)

	// Some should succeed, some should be rate limited
	errCount := 0
	for range errors {
		errCount++
	}
	// At least 50 should succeed (our limit), so at least 50 errors
	if errCount < 50 {
		t.Errorf("expected at least 50 rate limited errors, got %d", errCount)
	}
}

func TestSandbox_DefaultPolicy(t *testing.T) {
	policy := DefaultResourcePolicy("my-plugin")

	if policy.Status != "pending_review" {
		t.Errorf("default status should be pending_review, got %q", policy.Status)
	}
	if policy.MemoryMB != 256 {
		t.Errorf("default memory should be 256MB, got %d", policy.MemoryMB)
	}
	if policy.MaxCallsPerSecond != 100 {
		t.Errorf("default call rate should be 100/s, got %d", policy.MaxCallsPerSecond)
	}

	// Default should grant db:read and cache:readwrite
	inner := &mockInnerHostAPI{}
	s := NewSandboxedHostAPI(inner, "my-plugin", policy)
	ctx := context.Background()

	_, err := s.DBQuery(ctx, "SELECT 1")
	if err != nil {
		t.Errorf("default should allow db read: %v", err)
	}

	_, err = s.DBExec(ctx, "INSERT INTO t VALUES (1)")
	if err == nil {
		t.Error("default should block db write")
	}
}

func TestMatchURLPattern(t *testing.T) {
	tests := []struct {
		pattern string
		url     string
		want    bool
	}{
		{"api.tenor.com", "https://api.tenor.com/v2/search", true},
		{"api.tenor.com", "https://api.tenor.com:443/v2/search", true},
		{"*.tenor.com", "https://api.tenor.com/v2/search", true},
		{"*.tenor.com", "https://cdn.tenor.com/images/abc.gif", true},
		{"*.tenor.com", "https://tenor.com/search", true},
		{"*.example.com", "https://evil.com/redirect?to=example.com", false},
		{"api.giphy.com", "https://api.giphy.com/v1/gifs", true},
		{"api.giphy.com", "https://cdn.giphy.com/media/abc.gif", false},
		{"*.bbc.co.uk", "https://feeds.bbc.co.uk/rss", true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s", tt.pattern, tt.url), func(t *testing.T) {
			got := matchURLPattern(tt.pattern, tt.url)
			if got != tt.want {
				t.Errorf("matchURLPattern(%q, %q) = %v, want %v", tt.pattern, tt.url, got, tt.want)
			}
		})
	}
}

func TestSandbox_ErrorCounting(t *testing.T) {
	inner := &mockInnerHostAPI{}
	policy := ResourcePolicy{
		Status:      "approved",
		Permissions: []Permission{}, // Nothing allowed
	}
	s := NewSandboxedHostAPI(inner, "test-plugin", policy)
	ctx := context.Background()

	s.DBQuery(ctx, "SELECT 1")
	s.DBExec(ctx, "INSERT INTO t VALUES (1)")
	s.HTTPRequest(ctx, "GET", "https://example.com", nil, nil)
	s.CacheGet(ctx, "k")

	stats := s.Stats()
	if stats.Errors != 4 {
		t.Errorf("expected 4 errors, got %d", stats.Errors)
	}
}

func TestRateLimiter_Disabled(t *testing.T) {
	r := rateLimiter{disabled: true}
	if r.enabled() {
		t.Error("disabled limiter should not be enabled")
	}

	r2 := rateLimiter{max: 0}
	if r2.enabled() {
		t.Error("zero-max limiter should not be enabled")
	}
}

func TestRateLimiter_WindowExpiry(t *testing.T) {
	r := newRateLimiter(2, 50*time.Millisecond)

	if !r.allow() {
		t.Error("first should be allowed")
	}
	if !r.allow() {
		t.Error("second should be allowed")
	}
	if r.allow() {
		t.Error("third should be blocked")
	}

	// Wait for window to expire
	time.Sleep(60 * time.Millisecond)

	if !r.allow() {
		t.Error("should be allowed after window expiry")
	}
}
