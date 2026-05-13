// Package plugin defines the unified interface for GoatKit plugins.
//
// Plugins can be implemented as either:
//   - WASM modules (portable, sandboxed, via wazero)
//   - gRPC services (native, for I/O-heavy workloads, via go-plugin)
//
// The host doesn't care which runtime backs a plugin - both implement
// this interface and are managed uniformly by the plugin manager.
package plugin

import (
	"context"
	"encoding/json"
)

// Plugin is the unified interface for WASM and gRPC plugins.
// Both runtime implementations must satisfy this interface.
type Plugin interface {
	// GKRegister returns plugin metadata. Called once at load time.
	// This is how plugins self-describe their capabilities per the GoatKit spec.
	GKRegister() GKRegistration

	// Init is called after loading, before the plugin serves requests.
	// The HostAPI provides access to host services (db, cache, http, etc).
	Init(ctx context.Context, host HostAPI) error

	// Call invokes a plugin function by name with JSON-encoded arguments.
	// Returns JSON-encoded response or error.
	// This is the primary communication channel between host and plugin.
	Call(ctx context.Context, fn string, args json.RawMessage) (json.RawMessage, error)

	// Shutdown is called before unloading the plugin.
	// Plugins should clean up resources and finish pending work.
	Shutdown(ctx context.Context) error
}

// GKRegistration describes what a plugin provides to the host.
// This is returned by GKRegister() - the self-describing plugin protocol.
type GKRegistration struct {
	// Identity
	Name        string `json:"name"`        // unique identifier, e.g. "stats"
	Version     string `json:"version"`     // semver, e.g. "1.0.0"
	Description string `json:"description"` // human-readable description
	Author      string `json:"author"`      // author or organization
	License     string `json:"license"`     // SPDX identifier, e.g. "Apache-2.0"
	Homepage    string `json:"homepage"`    // URL to plugin docs/repo

	// Capabilities - what the plugin exposes to the host
	Routes     []RouteSpec     `json:"routes,omitempty"`      // HTTP routes to register
	MenuItems  []MenuItemSpec  `json:"menu_items,omitempty"`  // navigation menu entries
	Widgets    []WidgetSpec    `json:"widgets,omitempty"`     // dashboard widgets
	Jobs       []JobSpec       `json:"jobs,omitempty"`        // scheduled/cron tasks
	Templates  []TemplateSpec  `json:"templates,omitempty"`   // template overrides/additions
	I18n       *I18nSpec       `json:"i18n,omitempty"`        // translations provided by plugin
	ErrorCodes []ErrorCodeSpec `json:"error_codes,omitempty"` // API error codes provided by plugin

	// Navigation control
	HideMenuItems []string `json:"hide_menu_items,omitempty"` // IDs of default menu items to hide (dashboard, tickets, queues, phone_ticket, email_ticket, admin)
	LandingPage   string   `json:"landing_page,omitempty"`    // URL path to redirect to after login (e.g. "/myplugin")

	// Access control — groups the plugin needs, auto-created on load
	Groups []GroupSpec `json:"groups,omitempty"` // groups for RBAC (use "group:<name>" in route middleware)

	// Custom fields the plugin needs on GoatKit entities.
	// Field names are auto-prefixed with the plugin name to prevent collisions.
	CustomFields []CustomFieldSpec `json:"custom_fields,omitempty"`

	// UIs the plugin provides. Each UI is an independent application
	// with its own routes, branding, navigation, and auth.
	UIs []UISpec `json:"uis,omitempty"`

	// Cascade handlers for entity deletion.
	// Called when platform entities are soft/hard deleted.
	Cascades []CascadeSpec `json:"cascades,omitempty"`

	// MCP (Model Context Protocol) tools.
	// Plugins can optionally declare MCP tools with full input schemas.
	// When declared, these override auto-generated tools from routes.
	// Tool names are auto-prefixed with the plugin name.
	MCPTools []MCPToolSpec `json:"mcp_tools,omitempty"`

	// Org context
	// SkipOrgInjection disables automatic _org_id injection into plugin call
	// params. Set to true for plugins that handle multi-org queries themselves.
	SkipOrgInjection bool `json:"skip_org_injection,omitempty"`

	// Requirements
	MinHostVersion string           `json:"min_host_version,omitempty"` // minimum GoatFlow version
	Permissions    []string         `json:"permissions,omitempty"`      // required host permissions (legacy, use ResourceRequest)
	Resources      *ResourceRequest `json:"resources,omitempty"`        // requested resource limits
}

// GroupSpec defines a group that a plugin requires for access control.
// Groups are auto-created in the GoatFlow groups table when the plugin loads.
// Routes can reference them via middleware: ["auth", "group:myplugin-users"].
type GroupSpec struct {
	Name        string `json:"name"`        // group name, e.g. "myplugin-users"
	Description string `json:"description"` // human-readable description for admin UI
}

// RouteSpec defines an HTTP route the plugin wants to handle.
//
// Middleware options:
//   - "auth"          — session or JWT authentication required
//   - "admin"         — auth + admin role required
//   - "group:<name>"  — auth + group membership required
//   - "webhook"       — no auth, HMAC signature verification, request logging
//   - (empty)         — no middleware (use with caution)
type RouteSpec struct {
	Method      string   `json:"method"`                // GET, POST, PUT, DELETE, etc.
	Path        string   `json:"path"`                  // URL path, e.g. "/admin/stats"
	Handler     string   `json:"handler"`               // plugin function to call
	Middleware  []string `json:"middleware,omitempty"`  // middleware chain
	Description string   `json:"description,omitempty"` // for documentation
}

// MCPToolSpec defines an MCP tool that a plugin provides.
// When declared, these provide richer schemas than auto-generated route tools.
// The tool name is auto-prefixed with the plugin name to prevent collisions.
type MCPToolSpec struct {
	Name        string         `json:"name"`                   // tool name (auto-prefixed with plugin name)
	Description string         `json:"description"`            // LLM-friendly description
	Handler     string         `json:"handler"`                // plugin function to call
	InputSchema map[string]any `json:"input_schema,omitempty"` // JSON Schema for parameters
}

// MenuItemSpec defines a navigation menu entry.
type MenuItemSpec struct {
	ID       string         `json:"id"`                 // unique identifier
	Label    string         `json:"label"`              // display text (can be i18n key)
	Icon     string         `json:"icon,omitempty"`     // icon name or SVG
	Path     string         `json:"path"`               // URL path when clicked
	Location string         `json:"location"`           // where to insert: "admin", "agent", "customer"
	Parent   string         `json:"parent,omitempty"`   // parent menu ID for submenus
	Order    int            `json:"order,omitempty"`    // sort order within location
	Children []MenuItemSpec `json:"children,omitempty"` // nested menu items
}

// WidgetSpec defines a dashboard widget.
type WidgetSpec struct {
	ID          string `json:"id"`                    // unique identifier
	Title       string `json:"title"`                 // display title (can be i18n key)
	Description string `json:"description,omitempty"` // widget description
	Handler     string `json:"handler"`               // plugin function that returns widget HTML
	Location    string `json:"location"`              // dashboard location: "agent_home", "admin_home"
	Size        string `json:"size,omitempty"`        // "small", "medium", "large", "full"
	Order       int    `json:"order,omitempty"`       // sort order within location
	Refreshable bool   `json:"refreshable,omitempty"` // can be refreshed via AJAX
	RefreshSec  int    `json:"refresh_sec,omitempty"` // auto-refresh interval
}

// JobSpec defines a scheduled/cron task.
type JobSpec struct {
	ID          string `json:"id"`                    // unique identifier
	Handler     string `json:"handler"`               // plugin function to call
	Schedule    string `json:"schedule"`              // cron expression, e.g. "0 * * * *"
	Description string `json:"description,omitempty"` // human-readable description
	Enabled     bool   `json:"enabled"`               // whether job runs by default
	Timeout     string `json:"timeout,omitempty"`     // max execution time, e.g. "5m"
}

// TemplateSpec defines a template the plugin provides.
type TemplateSpec struct {
	Name     string `json:"name"`               // template name, e.g. "stats/dashboard.html"
	Path     string `json:"path"`               // path within plugin package
	Override bool   `json:"override,omitempty"` // if true, overrides host template of same name
}

// I18nSpec defines internationalization resources provided by the plugin.
type I18nSpec struct {
	// Namespace prefix for plugin translations (e.g., "stats" -> "stats.dashboard.title")
	Namespace string `json:"namespace,omitempty"`
	// Languages supported by this plugin
	Languages []string `json:"languages,omitempty"`
	// Inline translations (language -> key -> value)
	// For small plugins, translations can be embedded in the manifest
	Translations map[string]map[string]string `json:"translations,omitempty"`
}

// ErrorCodeSpec defines an API error code provided by the plugin.
// The plugin name is automatically prefixed to the code by the host
// (e.g., code "export_failed" in plugin "stats" becomes "stats:export_failed").
type ErrorCodeSpec struct {
	Code       string `json:"code"`        // error code without prefix, e.g. "export_failed"
	Message    string `json:"message"`     // default English message
	HTTPStatus int    `json:"http_status"` // suggested HTTP status code
}

// HostAPI is the interface plugins use to access host services.
// Passed to Plugin.Init() - plugins store this for later use.
type HostAPI interface {
	// Database
	DBQuery(ctx context.Context, query string, args ...any) ([]map[string]any, error)
	DBExec(ctx context.Context, query string, args ...any) (int64, error)

	// Cache
	CacheGet(ctx context.Context, key string) ([]byte, bool, error)
	CacheSet(ctx context.Context, key string, value []byte, ttlSeconds int) error
	CacheDelete(ctx context.Context, key string) error

	// HTTP (outbound)
	HTTPRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) (int, []byte, error)

	// Email
	SendEmail(ctx context.Context, to, subject, body string, html bool) error

	// Logging
	Log(ctx context.Context, level, message string, fields map[string]any)

	// Config
	ConfigGet(ctx context.Context, key string) (string, error)

	// i18n
	Translate(ctx context.Context, key string, args ...any) string

	// Plugin-to-plugin calls
	// Allows one plugin to call functions in another plugin
	CallPlugin(ctx context.Context, pluginName, fn string, args json.RawMessage) (json.RawMessage, error)

	// SSE (Server-Sent Events)
	// Publishes an event to a named channel for connected browser clients.
	// channel scopes the event stream (e.g. "status", "progress");
	// eventType is the SSE event name (e.g. "device-table"); data is the payload (typically HTML).
	PublishEvent(ctx context.Context, channel string, eventType string, data string) error

	// Entity Deletion
	// EntitySoftDelete soft-deletes an entity (recycle bin + PII anonymisation).
	EntitySoftDelete(ctx context.Context, entityType string, entityID int64, reason string) error
	// EntityRestore restores a soft-deleted entity from the recycle bin.
	EntityRestore(ctx context.Context, entityType string, entityID int64) error
	// EntityHardDelete permanently removes an entity and all linked data.
	EntityHardDelete(ctx context.Context, entityType string, entityID int64, reason string) error
	// RecycleBinList lists soft-deleted entities in the recycle bin.
	RecycleBinList(ctx context.Context, entityType string) (json.RawMessage, error)

	// Secure Config
	// SecureConfigGet retrieves a decrypted secret value for this plugin.
	SecureConfigGet(ctx context.Context, key string) (string, error)
	// SecureConfigSet stores an encrypted secret value for this plugin.
	SecureConfigSet(ctx context.Context, key string, value string) error

	// Organisation
	// OrgID returns the active organisation ID for the current request.
	// Returns 0 if no organisation context (single-org mode).
	OrgID(ctx context.Context) int64

	// Custom Fields
	// Get retrieves custom field values for an entity. Plugin prefix is auto-stripped.
	// Pass nil for fields to get all fields; pass specific names to filter.
	CustomFieldsGet(ctx context.Context, entityType string, objectID int64, fields []string) (map[string]any, error)

	// Set stores custom field values for an entity. Plugin prefix is auto-stripped.
	// Validates types and constraints before storing.
	CustomFieldsSet(ctx context.Context, entityType string, objectID int64, values map[string]any) error

	// Query finds entities by custom field values. Returns matching object IDs.
	CustomFieldsQuery(ctx context.Context, entityType string, filters []CustomFieldFilter) ([]int64, error)

	// File Storage
	// StoreFile stores a file under the plugin's namespace.
	// Key is a logical path (e.g. "backups/job-123.backup"). Plugin prefix is auto-added.
	// Metadata is optional key-value pairs stored alongside the file (content-type, description, etc.).
	StoreFile(ctx context.Context, key string, data []byte, metadata map[string]string) error
	// GetFile retrieves a file by key. Returns the data and metadata, or error if not found.
	GetFile(ctx context.Context, key string) ([]byte, map[string]string, error)
	// DeleteFile removes a file by key.
	DeleteFile(ctx context.Context, key string) error
	// ListFiles lists files under a prefix (e.g. "backups/"). Returns keys and metadata.
	ListFiles(ctx context.Context, prefix string) ([]FileInfo, error)
}

// FileInfo describes a stored file.
type FileInfo struct {
	Key         string            `json:"key"`
	Size        int64             `json:"size"`
	ContentType string            `json:"content_type,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	ModifiedAt  string            `json:"modified_at,omitempty"`
}

// CustomFieldSpec declares a custom field a plugin needs on a GoatKit entity.
// The platform creates, stores, validates, indexes, and renders these fields.
type CustomFieldSpec struct {
	Name        string          `json:"name"`              // machine name (auto-prefixed with plugin name)
	Label       string          `json:"label"`             // display label (can be i18n key)
	EntityType  string          `json:"entity_type"`       // "contact", "agent", "organisation", etc.
	FieldType   string          `json:"field_type"`        // "text", "select", "point", etc.
	Section     string          `json:"section,omitempty"` // UI section grouping (default: "custom")
	Order       int             `json:"order,omitempty"`   // sort within section
	Required    bool            `json:"required,omitempty"`
	Description string          `json:"description,omitempty"` // help text below input
	Placeholder string          `json:"placeholder,omitempty"`
	Config      json.RawMessage `json:"config,omitempty"` // type-specific config (options, regex, etc.)
}

// FieldOp represents an atomic operation on a custom field value.
// Pass a FieldOp as a value in CustomFieldsSet to perform an atomic update
// instead of the default overwrite behaviour.
//
// Supported operations:
//   - "increment": Atomically add Value to a numeric field (integer/decimal).
//     Optional Floor/Ceiling enforce bounds; the update is rejected if breached.
//   - "append": Add Value (string) to a multi_select field without overwriting.
//   - "remove": Remove Value (string) from a multi_select field.
//   - "cas": Compare-and-swap — set to Value only if current value equals Expect.
//   - "toggle": Flip a boolean field (no Value needed).
type FieldOp struct {
	Op      string   `json:"op"`                // Operation name.
	Value   any      `json:"value,omitempty"`   // Operand value.
	Expect  any      `json:"expect,omitempty"`  // Expected current value (cas only).
	Floor   *float64 `json:"floor,omitempty"`   // Minimum bound after increment.
	Ceiling *float64 `json:"ceiling,omitempty"` // Maximum bound after increment.
}

// CustomFieldFilter is a query filter used by CustomFieldsQuery.
type CustomFieldFilter struct {
	Field    string `json:"field"`            // field name (plugin prefix stripped)
	Operator string `json:"operator"`         // eq, neq, gt, lt, gte, lte, like, in, between, near
	Value    any    `json:"value"`            // comparison value
	Value2   any    `json:"value2,omitempty"` // second value for between (upper bound) and near (radius km)
}

// UISpec declares an independent UI that a plugin provides.
// Each UI gets its own routes, shell, branding, auth, and optional PWA support.
type UISpec struct {
	ID          string          `json:"id"`   // unique within plugin (auto-prefixed)
	Name        string          `json:"name"` // display name
	Description string          `json:"description,omitempty"`
	Type        string          `json:"type"`                 // admin_page, agent_app, customer_app, public_page, kiosk
	Icon        string          `json:"icon,omitempty"`       // FontAwesome class or SVG
	Shell       string          `json:"shell,omitempty"`      // none, minimal, standard (defaults per type)
	Routes      []UIRouteSpec   `json:"routes"`               // routes relative to /ui/{plugin}_{id}/
	Nav         *UINavSpec      `json:"nav,omitempty"`        // navigation for minimal/standard shells
	Branding    *UIBrandingSpec `json:"branding,omitempty"`   // per-UI branding overrides
	Auth        *UIAuthSpec     `json:"auth,omitempty"`       // auth configuration
	PWA         *UIPWASpec      `json:"pwa,omitempty"`        // PWA manifest configuration
	DataScope   string          `json:"data_scope,omitempty"` // self, org, all (for customer UIs)
	RateLimit   int             `json:"rate_limit,omitempty"` // requests/min for public UIs (0 = default)
}

// UIRouteSpec defines a route within a plugin UI.
type UIRouteSpec struct {
	Path    string `json:"path"`             // relative path, e.g. "/", "/items/:id"
	Method  string `json:"method,omitempty"` // GET, POST, etc. (default: GET)
	Handler string `json:"handler"`          // plugin function name
}

// UINavSpec defines navigation for a plugin UI shell.
type UINavSpec struct {
	Position string      `json:"position"` // bottom, top, side
	Items    []UINavItem `json:"items"`
}

// UINavItem is a navigation entry in a plugin UI.
type UINavItem struct {
	Label string `json:"label"`           // display text (can be i18n key)
	Icon  string `json:"icon"`            // FontAwesome class
	Path  string `json:"path"`            // relative path within this UI
	Badge string `json:"badge,omitempty"` // plugin function returning badge count
	Order int    `json:"order,omitempty"`
}

// UIBrandingSpec holds per-UI branding overrides.
type UIBrandingSpec struct {
	AppName string `json:"app_name,omitempty"`
	Logo    string `json:"logo,omitempty"`    // URL or base64 data URI
	Favicon string `json:"favicon,omitempty"` // URL or base64 data URI
	Color   string `json:"color,omitempty"`   // primary brand colour (hex)
}

// UIAuthSpec holds per-UI auth configuration.
type UIAuthSpec struct {
	Method string   `json:"method,omitempty"` // session, pin, token, none
	Groups []string `json:"groups,omitempty"` // required groups
}

// UIPWASpec holds PWA manifest configuration for a plugin UI.
type UIPWASpec struct {
	Enabled     bool     `json:"enabled"`
	StartURL    string   `json:"start_url,omitempty"` // defaults to UI root
	Display     string   `json:"display,omitempty"`   // standalone, fullscreen, minimal-ui
	ThemeColor  string   `json:"theme_color,omitempty"`
	CacheRoutes []string `json:"cache_routes,omitempty"` // routes to pre-cache for offline
}

// CascadeSpec declares a plugin's cascade handler for entity deletion.
type CascadeSpec struct {
	EntityType   string `json:"entity_type"`              // ticket, contact, agent, organisation, etc.
	OnSoftDelete string `json:"on_soft_delete,omitempty"` // plugin function for soft delete
	OnHardDelete string `json:"on_hard_delete,omitempty"` // plugin function for hard delete
}

// ResourceRequest describes what a plugin asks for from the platform.
// These are requests/defaults — the platform may grant less via ResourcePolicy.
type ResourceRequest struct {
	// Process limits (gRPC plugins only, ignored for WASM)
	MemoryMB        int    `json:"memory_mb,omitempty" yaml:"memory_mb,omitempty"`               // Requested RSS limit in MB
	CallTimeout     string `json:"call_timeout,omitempty" yaml:"call_timeout,omitempty"`         // Per-call deadline, e.g. "30s"
	InitTimeout     string `json:"init_timeout,omitempty" yaml:"init_timeout,omitempty"`         // Init must complete within, e.g. "10s"
	ShutdownTimeout string `json:"shutdown_timeout,omitempty" yaml:"shutdown_timeout,omitempty"` // Shutdown grace period, e.g. "5s"

	// HostAPI permissions requested
	Permissions []Permission `json:"permissions,omitempty" yaml:"permissions,omitempty"`

	// Requested file storage quota. The platform may grant less via ResourcePolicy.
	MaxFileStorageBytes int64 `json:"max_file_storage_bytes,omitempty" yaml:"max_file_storage_bytes,omitempty"`
}

// Permission declares a specific capability a plugin requests.
type Permission struct {
	// Type is the permission category: "db", "cache", "http", "email", "config", "plugin_call"
	Type string `json:"type" yaml:"type"`

	// Access level: "read", "write", "readwrite" (for db/cache)
	Access string `json:"access,omitempty" yaml:"access,omitempty"`

	// Scope constrains the permission. Meaning depends on type:
	//   db:     table allowlist, e.g. ["fictus_*", "ticket"]
	//   http:   URL patterns, e.g. ["*.tenor.com", "api.giphy.com"]
	//   cache:  key prefix (auto-namespaced if empty)
	//   plugin_call: plugin names allowed to call, e.g. ["stats"]
	Scope []string `json:"scope,omitempty" yaml:"scope,omitempty"`
}

// ResourcePolicy is the platform-enforced limits for a plugin.
// Set by admin, stored in sysconfig. Overrides ResourceRequest.
type ResourcePolicy struct {
	// Plugin name this policy applies to
	PluginName string `json:"plugin_name"`

	// Status: "pending_review", "approved", "restricted", "blocked"
	Status string `json:"status"`

	// Process limits (overrides plugin request)
	MemoryMB        int    `json:"memory_mb,omitempty"`
	CallTimeout     string `json:"call_timeout,omitempty"`
	InitTimeout     string `json:"init_timeout,omitempty"`
	ShutdownTimeout string `json:"shutdown_timeout,omitempty"`

	// Granted permissions (may be subset of requested)
	Permissions []Permission `json:"permissions,omitempty"`

	// Rate limits
	MaxCallsPerSecond  int `json:"max_calls_per_second,omitempty"` // 0 = unlimited
	MaxDBQueriesPerMin int `json:"max_db_queries_per_min,omitempty"`
	MaxHTTPReqPerMin   int `json:"max_http_req_per_min,omitempty"`

	// File storage limits
	MaxFileStorageBytes int64 `json:"max_file_storage_bytes,omitempty"` // 0 = default (500MB)
}

// DefaultResourcePolicy returns a restrictive default policy for new plugins.
// Plugins start with minimal access until an admin reviews them.
func DefaultResourcePolicy(pluginName string) ResourcePolicy {
	return ResourcePolicy{
		PluginName:      pluginName,
		Status:          "pending_review",
		MemoryMB:        256,
		CallTimeout:     "30s",
		InitTimeout:     "10s",
		ShutdownTimeout: "5s",
		Permissions: []Permission{
			{Type: "db", Access: "read"},
			{Type: "cache", Access: "readwrite"},
		},
		MaxCallsPerSecond:  100,
		MaxDBQueriesPerMin: 600,
		MaxHTTPReqPerMin:   60,
	}
}
