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
	LandingPage   string   `json:"landing_page,omitempty"`    // URL path to redirect to after login (e.g. "/fictus")

	// Requirements
	MinHostVersion string              `json:"min_host_version,omitempty"` // minimum GoatFlow version
	Permissions    []string            `json:"permissions,omitempty"`      // required host permissions (legacy, use ResourceRequest)
	Resources      *ResourceRequest    `json:"resources,omitempty"`        // requested resource limits
}

// RouteSpec defines an HTTP route the plugin wants to handle.
type RouteSpec struct {
	Method      string   `json:"method"`                 // GET, POST, PUT, DELETE, etc.
	Path        string   `json:"path"`                   // URL path, e.g. "/admin/stats"
	Handler     string   `json:"handler"`                // plugin function to call
	Middleware  []string `json:"middleware,omitempty"`   // middleware chain, e.g. ["auth", "admin"]
	Description string   `json:"description,omitempty"`  // for documentation
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
	// Publishes an event to all connected browser clients.
	// eventType is the SSE event name (e.g. "device-table"); data is the payload (typically HTML).
	PublishEvent(ctx context.Context, eventType string, data string) error
}

// ResourceRequest describes what a plugin asks for from the platform.
// These are requests/defaults — the platform may grant less via ResourcePolicy.
type ResourceRequest struct {
	// Process limits (gRPC plugins only, ignored for WASM)
	MemoryMB       int    `json:"memory_mb,omitempty" yaml:"memory_mb,omitempty"`             // Requested RSS limit in MB
	CallTimeout    string `json:"call_timeout,omitempty" yaml:"call_timeout,omitempty"`        // Per-call deadline, e.g. "30s"
	InitTimeout    string `json:"init_timeout,omitempty" yaml:"init_timeout,omitempty"`        // Init must complete within, e.g. "10s"
	ShutdownTimeout string `json:"shutdown_timeout,omitempty" yaml:"shutdown_timeout,omitempty"` // Shutdown grace period, e.g. "5s"

	// HostAPI permissions requested
	Permissions []Permission `json:"permissions,omitempty" yaml:"permissions,omitempty"`
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
	MaxCallsPerSecond int `json:"max_calls_per_second,omitempty"` // 0 = unlimited
	MaxDBQueriesPerMin int `json:"max_db_queries_per_min,omitempty"`
	MaxHTTPReqPerMin  int `json:"max_http_req_per_min,omitempty"`
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
