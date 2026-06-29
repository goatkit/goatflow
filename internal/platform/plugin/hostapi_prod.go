package plugin

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/goatkit/goatflow/internal/platform/cache"
	"github.com/goatkit/goatflow/internal/platform/config"
	"github.com/goatkit/goatflow/internal/platform/customfields"
	"github.com/goatkit/goatflow/internal/platform/database"
	"github.com/goatkit/goatflow/internal/platform/deletion"
	"github.com/goatkit/goatflow/internal/platform/i18n"
	"github.com/goatkit/goatflow/internal/platform/notifications"
	"github.com/goatkit/goatflow/internal/platform/organisation"
	"github.com/goatkit/goatflow/internal/platform/secureconfig"
)

// pluginLogEchoEnabled reports whether plugin log entries should be
// mirrored to the host's stderr. Gated by GOATFLOW_PLUGIN_LOG_ECHO — set
// to "1"/"true"/"yes" to turn on. The default is off because the plugin
// log buffer already covers the admin UI use case; enable when you need
// the logs in `docker logs` for root-cause debugging of generation /
// cascade flows. Evaluated once per process so flipping the env mid-run
// requires a container restart, but that's cheaper than parsing the env
// var on every log call.
var (
	pluginLogEchoOnce sync.Once
	pluginLogEcho     bool
)

func pluginLogEchoEnabled() bool {
	pluginLogEchoOnce.Do(func() {
		v := strings.ToLower(strings.TrimSpace(os.Getenv("GOATFLOW_PLUGIN_LOG_ECHO")))
		pluginLogEcho = v == "1" || v == "true" || v == "yes" || v == "on"
	})
	return pluginLogEcho
}

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
	PluginManager *Manager   // For plugin-to-plugin calls
	SSEBroker     *SSEBroker // For publishing SSE events to browsers
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

	// Optional mirror to stderr so plugin logs show up in `docker logs`.
	// Bypasses the slog-level filter (LOG_LEVEL=warn above silently
	// drops info/debug) — the gate on this side is explicit. Formatted
	// as one line per entry so grep works cleanly.
	if pluginLogEchoEnabled() {
		if len(fields) > 0 {
			log.Printf("[plugin:%s] %s: %s %v", pluginName, strings.ToUpper(level), message, fields)
		} else {
			log.Printf("[plugin:%s] %s: %s", pluginName, strings.ToUpper(level), message)
		}
	}
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

// EntitySoftDelete soft-deletes an entity.
func (h *ProdHostAPI) EntitySoftDelete(ctx context.Context, entityType string, entityID int64, reason string) error {
	svc, err := deletion.NewService()
	if err != nil {
		return err
	}
	userID := 1
	if caller, ok := ctx.Value(PluginCallerKey).(string); ok {
		_ = caller
	}
	return svc.SoftDelete(ctx, entityType, entityID, userID, reason)
}

// EntityRestore restores a soft-deleted entity.
func (h *ProdHostAPI) EntityRestore(ctx context.Context, entityType string, entityID int64) error {
	svc, err := deletion.NewService()
	if err != nil {
		return err
	}
	return svc.Restore(ctx, entityType, entityID, 1)
}

// EntityHardDelete permanently removes an entity.
func (h *ProdHostAPI) EntityHardDelete(ctx context.Context, entityType string, entityID int64, reason string) error {
	svc, err := deletion.NewService()
	if err != nil {
		return err
	}
	return svc.HardDelete(ctx, entityType, entityID, 1, reason)
}

// RecycleBinList lists soft-deleted entities.
func (h *ProdHostAPI) RecycleBinList(ctx context.Context, entityType string) (json.RawMessage, error) {
	svc, err := deletion.NewService()
	if err != nil {
		return nil, err
	}
	entries, err := svc.RecycleBinList(ctx, entityType)
	if err != nil {
		return nil, err
	}
	return deletion.RecycleBinToJSON(entries)
}

// SecureConfigGet retrieves and decrypts a secret value.
func (h *ProdHostAPI) SecureConfigGet(ctx context.Context, key string) (string, error) {
	pluginName, _ := ctx.Value(PluginCallerKey).(string)
	if pluginName == "" {
		return "", fmt.Errorf("secure config: no plugin context")
	}
	repo, err := secureconfig.NewRepository()
	if err != nil {
		return "", fmt.Errorf("secure config: %w", err)
	}
	orgID := organisation.OrgIDFromContext(ctx)
	entry, err := repo.Get(pluginName, key, orgID)
	if err != nil {
		return "", err
	}
	if entry == nil {
		return "", fmt.Errorf("secure config key %q not found", key)
	}
	encKey, err := secureconfig.GetKey()
	if err != nil {
		return "", fmt.Errorf("secure config: %w", err)
	}
	plaintext, err := secureconfig.Decrypt(entry.EncryptedValue, encKey)
	if err != nil {
		return "", fmt.Errorf("secure config: %w", err)
	}
	return string(plaintext), nil
}

// SecureConfigSet encrypts and stores a secret value.
func (h *ProdHostAPI) SecureConfigSet(ctx context.Context, key string, value string) error {
	pluginName, _ := ctx.Value(PluginCallerKey).(string)
	if pluginName == "" {
		return fmt.Errorf("secure config: no plugin context")
	}
	encKey, err := secureconfig.GetKey()
	if err != nil {
		return fmt.Errorf("secure config: %w", err)
	}
	encrypted, err := secureconfig.Encrypt([]byte(value), encKey)
	if err != nil {
		return fmt.Errorf("secure config: %w", err)
	}
	hint := secureconfig.ValueHint(value)
	repo, err := secureconfig.NewRepository()
	if err != nil {
		return fmt.Errorf("secure config: %w", err)
	}
	orgID := organisation.OrgIDFromContext(ctx)
	return repo.Set(pluginName, key, encrypted, hint, orgID, 1)
}

// OrgID returns the active organisation ID from the request context.
func (h *ProdHostAPI) OrgID(ctx context.Context) int64 {
	return organisation.OrgIDFromContext(ctx)
}

// customFieldsRepo returns a Repository using the ProdHostAPI's injected DB connection.
func (h *ProdHostAPI) customFieldsRepo() (*customfields.Repository, error) {
	db, err := h.getDB("")
	if err != nil {
		return nil, fmt.Errorf("custom fields: %w", err)
	}
	return customfields.NewRepositoryWithDB(db), nil
}

// CustomFieldsGet retrieves custom field values for an entity.
func (h *ProdHostAPI) CustomFieldsGet(ctx context.Context, entityType string, objectID int64, fields []string) (map[string]any, error) {
	repo, err := h.customFieldsRepo()
	if err != nil {
		return nil, err
	}
	return repo.GetValues(entityType, objectID, fields)
}

// CustomFieldsSet stores custom field values for an entity.
func (h *ProdHostAPI) CustomFieldsSet(ctx context.Context, entityType string, objectID int64, values map[string]any) error {
	repo, err := h.customFieldsRepo()
	if err != nil {
		return err
	}

	// Load defs for validation.
	defs, err := repo.ListDefs(entityType, "", "", true)
	if err != nil {
		return fmt.Errorf("custom fields: load defs: %w", err)
	}
	defMap := make(map[string]*customfields.FieldDef, len(defs))
	for i := range defs {
		defMap[defs[i].Name] = &defs[i]
	}

	// Validate.
	if errs := customfields.ValidateValues(defMap, values); errs != nil {
		raw, _ := json.Marshal(errs)
		return fmt.Errorf("validation failed: %s", string(raw))
	}

	return repo.SetValues(entityType, objectID, values)
}

// CustomFieldsQuery finds entities by custom field values.
func (h *ProdHostAPI) CustomFieldsQuery(ctx context.Context, entityType string, filters []CustomFieldFilter) ([]int64, error) {
	repo, err := h.customFieldsRepo()
	if err != nil {
		return nil, err
	}

	// Convert plugin filter type to internal type.
	internal := make([]customfields.FieldFilter, len(filters))
	for i, f := range filters {
		internal[i] = customfields.FieldFilter{
			Field:    f.Field,
			Operator: f.Operator,
			Value:    f.Value,
			Value2:   f.Value2,
		}
	}
	return repo.QueryByFields(entityType, internal)
}

// PublishEvent sends an SSE event to a named channel for connected browser clients.
func (h *ProdHostAPI) PublishEvent(ctx context.Context, channel string, eventType string, data string) error {
	if h.SSEBroker == nil {
		return fmt.Errorf("SSE broker not available")
	}
	pluginName := ""
	if caller, ok := ctx.Value(PluginCallerKey).(string); ok {
		pluginName = caller
	}
	h.SSEBroker.Publish(SSEEvent{
		Plugin:  pluginName,
		Channel: channel,
		Type:    eventType,
		Data:    data,
	})
	return nil
}
