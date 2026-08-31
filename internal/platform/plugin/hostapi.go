package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	plugin "github.com/goatkit/goatflow/pkg/plugin"
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

// PublishEvent sends an SSE event to a named channel for connected browser clients.
func (h *DefaultHostAPI) PublishEvent(ctx context.Context, channel string, eventType string, data string) error {
	return fmt.Errorf("SSE not available in default host")
}

// EntitySoftDelete is not available in default host.
func (h *DefaultHostAPI) EntitySoftDelete(ctx context.Context, entityType string, entityID int64, reason string) error {
	return fmt.Errorf("entity deletion not available in default host")
}

// EntityRestore is not available in default host.
func (h *DefaultHostAPI) EntityRestore(ctx context.Context, entityType string, entityID int64) error {
	return fmt.Errorf("entity deletion not available in default host")
}

// EntityHardDelete is not available in default host.
func (h *DefaultHostAPI) EntityHardDelete(ctx context.Context, entityType string, entityID int64, reason string) error {
	return fmt.Errorf("entity deletion not available in default host")
}

// RecycleBinList is not available in default host.
func (h *DefaultHostAPI) RecycleBinList(ctx context.Context, entityType string) (json.RawMessage, error) {
	return nil, fmt.Errorf("entity deletion not available in default host")
}

// SecureConfigGet returns empty (not available in default host).
func (h *DefaultHostAPI) SecureConfigGet(ctx context.Context, key string) (string, error) {
	return "", fmt.Errorf("secure config not available in default host")
}

// SecureConfigSet returns error (not available in default host).
func (h *DefaultHostAPI) SecureConfigSet(ctx context.Context, key string, value string) error {
	return fmt.Errorf("secure config not available in default host")
}

// OrgID returns 0 (no org context in default host).
func (h *DefaultHostAPI) OrgID(ctx context.Context) int64 {
	return 0
}

// CustomFieldsGet retrieves custom field values for an entity.
func (h *DefaultHostAPI) CustomFieldsGet(ctx context.Context, entityType string, objectID int64, fields []string) (map[string]any, error) {
	return nil, nil
}

// CustomFieldsSet stores custom field values for an entity.
func (h *DefaultHostAPI) CustomFieldsSet(ctx context.Context, entityType string, objectID int64, values map[string]any) error {
	return nil
}

// CustomFieldsQuery finds entities by custom field values.
func (h *DefaultHostAPI) CustomFieldsQuery(ctx context.Context, entityType string, filters []CustomFieldFilter) ([]int64, error) {
	return nil, nil
}

// CreateArticleAttachment is not available in default host.
func (h *DefaultHostAPI) CreateArticleAttachment(ctx context.Context, articleID, createdBy int64, filename, contentType string, content []byte) (int64, error) {
	return 0, fmt.Errorf("article attachments not available in default host")
}

// ListArticleAttachments is not available in default host.
func (h *DefaultHostAPI) ListArticleAttachments(ctx context.Context, articleID int64) ([]plugin.ArticleAttachment, error) {
	return nil, fmt.Errorf("article attachments not available in default host")
}

// DeleteArticleAttachment is not available in default host.
func (h *DefaultHostAPI) DeleteArticleAttachment(ctx context.Context, articleID, attachmentID int64) error {
	return fmt.Errorf("article attachments not available in default host")
}

// CreateArticle is not available in default host.
func (h *DefaultHostAPI) CreateArticle(ctx context.Context, ticketID, createdBy int64, subject, body string, visibleToCustomer bool) (int64, error) {
	return 0, fmt.Errorf("article creation not available in default host")
}

// ChangeTicketStatus is not available in default host.
func (h *DefaultHostAPI) ChangeTicketStatus(ctx context.Context, ticketID, stateID, userID int64, untilTime int64) error {
	return fmt.Errorf("ticket state operations not available in default host")
}

// ListTicketStates is not available in default host.
func (h *DefaultHostAPI) ListTicketStates(ctx context.Context) ([]plugin.TicketStateInfo, error) {
	return nil, fmt.Errorf("ticket state operations not available in default host")
}

// ListTicketViews is not available in default host.
func (h *DefaultHostAPI) ListTicketViews(ctx context.Context) ([]plugin.TicketViewInfo, error) {
	return nil, fmt.Errorf("ticket view operations not available in default host")
}

// RenderMarkdownToPdf is not available in default host.
func (h *DefaultHostAPI) RenderMarkdownToPdf(ctx context.Context, markdown string, options plugin.PdfRenderOptions) ([]byte, error) {
	return nil, fmt.Errorf("pdf rendering not available in default host")
}

// StoreFile is not available in default host.
func (h *DefaultHostAPI) StoreFile(ctx context.Context, key string, data []byte, metadata map[string]string) error {
	return fmt.Errorf("file storage not available in default host")
}

// GetFile is not available in default host.
func (h *DefaultHostAPI) GetFile(ctx context.Context, key string) ([]byte, map[string]string, error) {
	return nil, nil, fmt.Errorf("file storage not available in default host")
}

// DeleteFile is not available in default host.
func (h *DefaultHostAPI) DeleteFile(ctx context.Context, key string) error {
	return fmt.Errorf("file storage not available in default host")
}

// ListFiles is not available in default host.
func (h *DefaultHostAPI) ListFiles(ctx context.Context, prefix string) ([]FileInfo, error) {
	return nil, fmt.Errorf("file storage not available in default host")
}

func (m *DefaultHostAPI) GenerateThumbnail(_ context.Context, _ []byte, _ string, _, _ int) ([]byte, string, error) {
	return nil, "", fmt.Errorf("thumbnail generation not available in default host")
}
