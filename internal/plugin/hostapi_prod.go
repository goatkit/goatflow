package plugin

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/goatkit/goatflow/internal/cache"
	"github.com/goatkit/goatflow/internal/config"
	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/i18n"
	"github.com/goatkit/goatflow/internal/notifications"
)

// PluginLanguageKey is the context key for plugin request language.
type pluginLangKeyType struct{}

// PluginCallerKey is the context key for the calling plugin name.
// Used to provide better error messages for plugin-to-plugin calls.
type pluginCallerKeyType struct{}

// PluginCallerKey is the context key for tracking which plugin is making a call.
var PluginCallerKey = pluginCallerKeyType{}

var PluginLanguageKey = pluginLangKeyType{}

// ProdHostAPI is the production implementation of HostAPI.
// It wires plugins to real database, cache, email, and other services.
type ProdHostAPI struct {
	databases     map[string]*sql.DB // Named database connections
	defaultDB     string             // Name of the default database
	cache         *cache.RedisCache
	httpClient    *http.Client
	logger        *slog.Logger
	PluginManager *Manager // For plugin-to-plugin calls
}

// ProdHostAPIOption is a functional option for ProdHostAPI.
type ProdHostAPIOption func(*ProdHostAPI)

// WithDB adds a named database connection. Use "default" for the primary database.
func WithDB(name string, db *sql.DB) ProdHostAPIOption {
	return func(h *ProdHostAPI) {
		if h.databases == nil {
			h.databases = make(map[string]*sql.DB)
		}
		h.databases[name] = db
		// First DB added becomes the default
		if h.defaultDB == "" {
			h.defaultDB = name
		}
	}
}

// WithDefaultDB sets which named database is the default.
func WithDefaultDB(name string) ProdHostAPIOption {
	return func(h *ProdHostAPI) {
		h.defaultDB = name
	}
}

// WithCache sets the cache client.
func WithCache(c *cache.RedisCache) ProdHostAPIOption {
	return func(h *ProdHostAPI) {
		h.cache = c
	}
}

// WithLogger sets the logger.
func WithLogger(logger *slog.Logger) ProdHostAPIOption {
	return func(h *ProdHostAPI) {
		h.logger = logger
	}
}

// WithPluginManager sets the plugin manager for plugin-to-plugin calls.
func WithPluginManager(mgr *Manager) ProdHostAPIOption {
	return func(h *ProdHostAPI) {
		h.PluginManager = mgr
	}
}

// NewProdHostAPI creates a production host API with the given options.
func NewProdHostAPI(opts ...ProdHostAPIOption) *ProdHostAPI {
	h := &ProdHostAPI{
		databases: make(map[string]*sql.DB),
		defaultDB: "default",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: slog.Default(),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// getDB returns the database for a given name, or the default if name is empty.
func (h *ProdHostAPI) getDB(name string) (*sql.DB, error) {
	if name == "" {
		name = h.defaultDB
	}
	db, ok := h.databases[name]
	if !ok {
		if len(h.databases) == 0 {
			return nil, fmt.Errorf("no databases configured")
		}
		return nil, fmt.Errorf("database %q not found", name)
	}
	return db, nil
}

// parseDBPrefix extracts a database name prefix from a query.
// Format: "@dbname:SELECT..." returns ("dbname", "SELECT...")
// If no prefix, returns ("", query).
func (h *ProdHostAPI) parseDBPrefix(query string) (dbName, cleanQuery string) {
	if len(query) > 0 && query[0] == '@' {
		if idx := indexByte(query, ':'); idx > 1 {
			return query[1:idx], query[idx+1:]
		}
	}
	return "", query
}

// indexByte returns the index of the first instance of c in s, or -1 if not present.
func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// DBQuery executes a SELECT query and returns rows as maps.
// Uses the default database. For named databases, prefix query with "@dbname:" (e.g., "@analytics:SELECT...").
func (h *ProdHostAPI) DBQuery(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	dbName, query := h.parseDBPrefix(query)
	db, err := h.getDB(dbName)
	if err != nil {
		return nil, err
	}

	// Enforce SQL portability - convert ? placeholders for MySQL/PostgreSQL compatibility
	query = database.ConvertPlaceholders(query)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get columns: %w", err)
	}

	var results []map[string]any

	for rows.Next() {
		// Create a slice of interface{} to hold the values
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		// Convert to map
		row := make(map[string]any)
		for i, col := range columns {
			val := values[i]
			// Convert []byte to string for readability
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return results, nil
}

// DBExec executes an INSERT/UPDATE/DELETE and returns affected rows.
// Uses the default database. For named databases, prefix query with "@dbname:" (e.g., "@analytics:INSERT...").
func (h *ProdHostAPI) DBExec(ctx context.Context, query string, args ...any) (int64, error) {
	dbName, query := h.parseDBPrefix(query)
	db, err := h.getDB(dbName)
	if err != nil {
		return 0, err
	}

	// Enforce SQL portability - convert ? placeholders for MySQL/PostgreSQL compatibility
	query = database.ConvertPlaceholders(query)

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("exec failed: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("get affected rows: %w", err)
	}

	return affected, nil
}

// CacheGet retrieves a value from cache.
func (h *ProdHostAPI) CacheGet(ctx context.Context, key string) ([]byte, bool, error) {
	if h.cache == nil {
		return nil, false, nil // No cache configured, return miss
	}

	val, err := h.cache.Get(ctx, key)
	if err != nil {
		// Cache miss or error - treat as miss
		return nil, false, nil
	}

	if val == nil {
		return nil, false, nil
	}

	// Convert to bytes
	switch v := val.(type) {
	case []byte:
		return v, true, nil
	case string:
		return []byte(v), true, nil
	default:
		return nil, false, fmt.Errorf("unexpected cache value type: %T", val)
	}
}

// CacheSet stores a value in cache.
func (h *ProdHostAPI) CacheSet(ctx context.Context, key string, value []byte, ttlSeconds int) error {
	if h.cache == nil {
		return nil // No cache configured, silently succeed
	}

	ttl := time.Duration(ttlSeconds) * time.Second
	return h.cache.Set(ctx, key, value, ttl)
}

// CacheDelete removes a value from cache.
func (h *ProdHostAPI) CacheDelete(ctx context.Context, key string) error {
	if h.cache == nil {
		return nil // No cache configured, silently succeed
	}

	return h.cache.Delete(ctx, key)
}

// HTTPRequest makes an outbound HTTP request.
func (h *ProdHostAPI) HTTPRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) (int, []byte, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return 0, nil, fmt.Errorf("create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response: %w", err)
	}

	return resp.StatusCode, respBody, nil
}

// SendEmail sends an email using the configured provider.
func (h *ProdHostAPI) SendEmail(ctx context.Context, to, subject, body string, html bool) error {
	provider := notifications.GetEmailProvider()
	if provider == nil {
		return fmt.Errorf("email provider not configured")
	}

	msg := notifications.EmailMessage{
		To:      []string{to},
		Subject: subject,
		Body:    body,
		HTML:    html,
	}

	return provider.Send(ctx, msg)
}

// Log writes a structured log entry.
func (h *ProdHostAPI) Log(ctx context.Context, level, message string, fields map[string]any) {
	attrs := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		attrs = append(attrs, k, v)
	}

	switch level {
	case "debug":
		h.logger.Debug(message, attrs...)
	case "info":
		h.logger.Info(message, attrs...)
	case "warn":
		h.logger.Warn(message, attrs...)
	case "error":
		h.logger.Error(message, attrs...)
	default:
		h.logger.Info(message, attrs...)
	}

	// Also add to the plugin log buffer for the admin viewer
	pluginName := ""
	if pn, ok := fields["plugin"].(string); ok {
		pluginName = pn
	} else if pn, ok := ctx.Value(PluginCallerKey).(string); ok {
		pluginName = pn
	}
	GetLogBuffer().Log(pluginName, level, message, fields)
}

// ConfigGet retrieves a configuration value by key path.
// Supports dot notation for nested values (e.g., "app.name").
func (h *ProdHostAPI) ConfigGet(ctx context.Context, key string) (string, error) {
	cfg := config.Get()
	if cfg == nil {
		return "", fmt.Errorf("config not loaded")
	}

	// Map common config keys to their values
	// This is a simplified approach - could be extended with reflection
	switch key {
	case "app.name":
		return cfg.App.Name, nil
	case "app.timezone":
		return cfg.App.Timezone, nil
	case "app.env":
		return cfg.App.Env, nil
	default:
		return "", fmt.Errorf("unknown config key: %s", key)
	}
}

// Translate translates a key to the current locale.
// The language is determined from the context (set via PluginLanguageKey).
// If no language is set, falls back to the default language.
func (h *ProdHostAPI) Translate(ctx context.Context, key string, args ...any) string {
	i18nInst := i18n.GetInstance()
	if i18nInst == nil {
		return key
	}

	// Get language from context
	lang := "en" // default
	if l, ok := ctx.Value(PluginLanguageKey).(string); ok && l != "" {
		lang = l
	}

	return i18nInst.T(lang, key, args...)
}

// CallPlugin calls a function in another plugin.
// This enables plugin-to-plugin communication via the host.
// If PluginCallerKey is set in the context, provides better error messages.
func (h *ProdHostAPI) CallPlugin(ctx context.Context, pluginName, fn string, args json.RawMessage) (json.RawMessage, error) {
	if h.PluginManager == nil {
		return nil, fmt.Errorf("plugin manager not available")
	}

	// Get caller plugin name from context for better error messages
	callerPlugin := ""
	if caller, ok := ctx.Value(PluginCallerKey).(string); ok {
		callerPlugin = caller
	}

	if callerPlugin != "" {
		return h.PluginManager.CallFrom(ctx, callerPlugin, pluginName, fn, args)
	}
	return h.PluginManager.Call(ctx, pluginName, fn, args)
}
