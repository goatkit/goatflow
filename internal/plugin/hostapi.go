package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
)

// DefaultHostAPI provides a basic implementation of HostAPI.
// In production, this would be wired to actual database, cache, etc.
type DefaultHostAPI struct {
	// Add dependencies here as needed
}

// NewDefaultHostAPI creates a new default host API.
func NewDefaultHostAPI() *DefaultHostAPI {
	return &DefaultHostAPI{}
}

// DBQuery executes a query and returns rows as maps.
func (h *DefaultHostAPI) DBQuery(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	// TODO: Wire to actual database
	return nil, nil
}

// DBExec executes a statement and returns affected rows.
func (h *DefaultHostAPI) DBExec(ctx context.Context, query string, args ...any) (int64, error) {
	// TODO: Wire to actual database
	return 0, nil
}

// CacheGet retrieves a value from cache.
func (h *DefaultHostAPI) CacheGet(ctx context.Context, key string) ([]byte, bool, error) {
	// TODO: Wire to Valkey/Redis
	return nil, false, nil
}

// CacheSet stores a value in cache.
func (h *DefaultHostAPI) CacheSet(ctx context.Context, key string, value []byte, ttlSeconds int) error {
	// TODO: Wire to Valkey/Redis
	return nil
}

// CacheDelete removes a value from cache.
func (h *DefaultHostAPI) CacheDelete(ctx context.Context, key string) error {
	// TODO: Wire to Valkey/Redis
	return nil
}

// HTTPRequest makes an outbound HTTP request.
func (h *DefaultHostAPI) HTTPRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) (int, []byte, error) {
	// TODO: Implement HTTP client
	return 200, nil, nil
}

// SendEmail sends an email.
func (h *DefaultHostAPI) SendEmail(ctx context.Context, to, subject, body string, html bool) error {
	// TODO: Wire to email provider
	return nil
}

// Log writes a log entry.
func (h *DefaultHostAPI) Log(ctx context.Context, level, message string, fields map[string]any) {
	log.Printf("[plugin:%s] %s %v", level, message, fields)
}

// ConfigGet retrieves a configuration value.
func (h *DefaultHostAPI) ConfigGet(ctx context.Context, key string) (string, error) {
	// TODO: Wire to config system
	return "", nil
}

// Translate translates a key to the current locale.
func (h *DefaultHostAPI) Translate(ctx context.Context, key string, args ...any) string {
	// TODO: Wire to i18n system
	return ""
}

// CallPlugin calls a function in another plugin.
func (h *DefaultHostAPI) CallPlugin(ctx context.Context, pluginName, fn string, args json.RawMessage) (json.RawMessage, error) {
	return nil, fmt.Errorf("plugin calls not available in default host")
}
