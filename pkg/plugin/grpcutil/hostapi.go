// Package grpcutil provides shared types for gRPC plugin communication.
// This file provides the plugin-side HostAPI client that allows plugins
// to call back to the GoatFlow host for database, cache, and other operations.
package grpcutil

import (
	"context"
	"encoding/json"
	"fmt"
	"net/rpc"

	plugin "github.com/goatkit/goatflow/pkg/plugin"
)

// HostAPIRPCRequest is a generic host API request.
type HostAPIRPCRequest struct {
	Method       string          `json:"method"`
	Args         json.RawMessage `json:"args"`
	CallerPlugin string          `json:"caller_plugin"`
}

// HostAPIRPCResponse is a generic host API response.
type HostAPIRPCResponse struct {
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error"`
}

// HostAPIClient implements plugin.HostAPI by making RPC calls back to the host.
// Plugins use this to access database, cache, HTTP, email, config, and i18n.
type HostAPIClient struct {
	client     *rpc.Client
	pluginName string
}

// NewHostAPIClient creates a new HostAPI client for plugin-to-host RPC calls.
func NewHostAPIClient(client *rpc.Client, pluginName string) *HostAPIClient {
	return &HostAPIClient{client: client, pluginName: pluginName}
}

func (c *HostAPIClient) call(method string, args any) (json.RawMessage, error) {
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("marshal args: %w", err)
	}

	req := HostAPIRPCRequest{
		Method:       method,
		Args:         argsJSON,
		CallerPlugin: c.pluginName,
	}
	var resp HostAPIRPCResponse

	if err := c.client.Call("Plugin.Call", req, &resp); err != nil {
		return nil, fmt.Errorf("host rpc: %w", err)
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("host: %s", resp.Error)
	}
	return resp.Result, nil
}

// DBQuery executes a read query via the host's database.
func (c *HostAPIClient) DBQuery(_ context.Context, query string, args ...any) ([]map[string]any, error) {
	result, err := c.call("db_query", map[string]any{"query": query, "args": args})
	if err != nil {
		return nil, err
	}
	var rows []map[string]any
	if err := json.Unmarshal(result, &rows); err != nil {
		return nil, fmt.Errorf("unmarshal rows: %w", err)
	}
	return rows, nil
}

// DBExec executes a write query via the host's database.
func (c *HostAPIClient) DBExec(_ context.Context, query string, args ...any) (int64, error) {
	result, err := c.call("db_exec", map[string]any{"query": query, "args": args})
	if err != nil {
		return 0, err
	}
	var resp struct {
		Affected int64 `json:"affected"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return 0, err
	}
	return resp.Affected, nil
}

// CacheGet retrieves a cached value.
func (c *HostAPIClient) CacheGet(_ context.Context, key string) ([]byte, bool, error) {
	result, err := c.call("cache_get", map[string]any{"key": key})
	if err != nil {
		return nil, false, err
	}
	var resp struct {
		Value []byte `json:"value"`
		Found bool   `json:"found"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, false, err
	}
	return resp.Value, resp.Found, nil
}

// CacheSet stores a cached value with TTL.
func (c *HostAPIClient) CacheSet(_ context.Context, key string, value []byte, ttlSeconds int) error {
	_, err := c.call("cache_set", map[string]any{"key": key, "value": value, "ttl": ttlSeconds})
	return err
}

// CacheDelete removes a cached value.
func (c *HostAPIClient) CacheDelete(_ context.Context, key string) error {
	_, err := c.call("cache_delete", map[string]any{"key": key})
	return err
}

// HTTPRequest makes an HTTP request via the host.
func (c *HostAPIClient) HTTPRequest(_ context.Context, method, url string, headers map[string]string, body []byte) (int, []byte, error) {
	result, err := c.call("http_request", map[string]any{
		"method":  method,
		"url":     url,
		"headers": headers,
		"body":    body,
	})
	if err != nil {
		return 0, nil, err
	}
	var resp struct {
		Status int    `json:"status"`
		Body   []byte `json:"body"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return 0, nil, err
	}
	return resp.Status, resp.Body, nil
}

// SendEmail sends email via the host.
func (c *HostAPIClient) SendEmail(_ context.Context, to, subject, body string, html bool) error {
	_, err := c.call("send_email", map[string]any{
		"to":      to,
		"subject": subject,
		"body":    body,
		"html":    html,
	})
	return err
}

// Log writes a log entry via the host.
func (c *HostAPIClient) Log(_ context.Context, level, message string, fields map[string]any) {
	c.call("log", map[string]any{"level": level, "message": message, "fields": fields}) //nolint:errcheck
}

// ConfigGet retrieves a configuration value.
func (c *HostAPIClient) ConfigGet(_ context.Context, key string) (string, error) {
	result, err := c.call("config_get", map[string]any{"key": key})
	if err != nil {
		return "", err
	}
	var val string
	if err := json.Unmarshal(result, &val); err != nil {
		return "", err
	}
	return val, nil
}

// Translate returns a translated string for the given key.
func (c *HostAPIClient) Translate(_ context.Context, key string, args ...any) string {
	result, err := c.call("translate", map[string]any{"key": key, "args": args})
	if err != nil {
		return key // Fallback to key
	}
	var val string
	if err := json.Unmarshal(result, &val); err != nil {
		return key
	}
	return val
}

// CallPlugin calls another plugin.
func (c *HostAPIClient) CallPlugin(_ context.Context, pluginName, fn string, args json.RawMessage) (json.RawMessage, error) {
	return c.call("plugin_call", map[string]any{
		"plugin":   pluginName,
		"function": fn,
		"args":     args,
	})
}

// PublishEvent sends an SSE event to connected browser clients.
func (c *HostAPIClient) PublishEvent(_ context.Context, eventType string, data string) error {
	_, err := c.call("publish_event", map[string]any{
		"event_type": eventType,
		"data":       data,
	})
	return err
}

// EntitySoftDelete soft-deletes an entity.
func (c *HostAPIClient) EntitySoftDelete(_ context.Context, entityType string, entityID int64, reason string) error {
	_, err := c.call("entity_soft_delete", map[string]any{"entity_type": entityType, "entity_id": entityID, "reason": reason})
	return err
}

// EntityRestore restores a soft-deleted entity.
func (c *HostAPIClient) EntityRestore(_ context.Context, entityType string, entityID int64) error {
	_, err := c.call("entity_restore", map[string]any{"entity_type": entityType, "entity_id": entityID})
	return err
}

// EntityHardDelete permanently removes an entity.
func (c *HostAPIClient) EntityHardDelete(_ context.Context, entityType string, entityID int64, reason string) error {
	_, err := c.call("entity_hard_delete", map[string]any{"entity_type": entityType, "entity_id": entityID, "reason": reason})
	return err
}

// RecycleBinList lists soft-deleted entities.
func (c *HostAPIClient) RecycleBinList(_ context.Context, entityType string) (json.RawMessage, error) {
	return c.call("recycle_bin_list", map[string]any{"entity_type": entityType})
}

// SecureConfigGet retrieves a decrypted secret.
func (c *HostAPIClient) SecureConfigGet(_ context.Context, key string) (string, error) {
	result, err := c.call("secure_config_get", map[string]any{"key": key})
	if err != nil {
		return "", err
	}
	var resp struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", err
	}
	return resp.Value, nil
}

// SecureConfigSet stores an encrypted secret.
func (c *HostAPIClient) SecureConfigSet(_ context.Context, key string, value string) error {
	_, err := c.call("secure_config_set", map[string]any{"key": key, "value": value})
	return err
}

// OrgID returns the active organisation ID.
func (c *HostAPIClient) OrgID(_ context.Context) int64 {
	result, err := c.call("org_id", map[string]any{})
	if err != nil {
		return 0
	}
	var id int64
	if err := json.Unmarshal(result, &id); err != nil {
		return 0
	}
	return id
}

// CustomFieldsGet retrieves custom field values for an entity.
func (c *HostAPIClient) CustomFieldsGet(_ context.Context, entityType string, objectID int64, fields []string) (map[string]any, error) {
	result, err := c.call("custom_fields_get", map[string]any{
		"entity_type": entityType,
		"object_id":   objectID,
		"fields":      fields,
	})
	if err != nil {
		return nil, err
	}
	var vals map[string]any
	if err := json.Unmarshal(result, &vals); err != nil {
		return nil, err
	}
	return vals, nil
}

// CustomFieldsSet stores custom field values for an entity.
func (c *HostAPIClient) CustomFieldsSet(_ context.Context, entityType string, objectID int64, values map[string]any) error {
	_, err := c.call("custom_fields_set", map[string]any{
		"entity_type": entityType,
		"object_id":   objectID,
		"values":      values,
	})
	return err
}

// CustomFieldsQuery finds entities by custom field values.
func (c *HostAPIClient) CustomFieldsQuery(_ context.Context, entityType string, filters []plugin.CustomFieldFilter) ([]int64, error) {
	result, err := c.call("custom_fields_query", map[string]any{
		"entity_type": entityType,
		"filters":     filters,
	})
	if err != nil {
		return nil, err
	}
	var ids []int64
	if err := json.Unmarshal(result, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

// ---- File Storage ----

func (c *HostAPIClient) StoreFile(ctx context.Context, key string, data []byte, metadata map[string]string) error {
	_, err := c.call("store_file", map[string]any{
		"key":      key,
		"data":     data,
		"metadata": metadata,
	})
	return err
}

func (c *HostAPIClient) GetFile(ctx context.Context, key string) ([]byte, map[string]string, error) {
	result, err := c.call("get_file", map[string]any{"key": key})
	if err != nil {
		return nil, nil, err
	}
	var resp struct {
		Data     []byte            `json:"data"`
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, nil, err
	}
	return resp.Data, resp.Metadata, nil
}

func (c *HostAPIClient) DeleteFile(ctx context.Context, key string) error {
	_, err := c.call("delete_file", map[string]any{"key": key})
	return err
}

func (c *HostAPIClient) ListFiles(ctx context.Context, prefix string) ([]plugin.FileInfo, error) {
	result, err := c.call("list_files", map[string]any{"prefix": prefix})
	if err != nil {
		return nil, err
	}
	var files []plugin.FileInfo
	if err := json.Unmarshal(result, &files); err != nil {
		return nil, err
	}
	return files, nil
}

// Verify HostAPIClient implements plugin.HostAPI at compile time.
var _ plugin.HostAPI = (*HostAPIClient)(nil)
