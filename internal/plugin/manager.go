package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/goatkit/goatflow/internal/apierrors"
	"github.com/goatkit/goatflow/internal/i18n"
)

// LazyLoader is the interface for lazy-loading plugins on demand.
type LazyLoader interface {
	EnsureLoaded(ctx context.Context, name string) error
	Discovered() []string
}

// Manager handles plugin lifecycle: loading, registration, and invocation.
type Manager struct {
	mu         sync.RWMutex
	plugins    map[string]*registeredPlugin
	host       HostAPI
	lazyLoader LazyLoader // Optional: for lazy loading support

	// Per-plugin resource policies (name -> policy)
	policies map[string]*ResourcePolicy

	// Per-plugin sandboxed HostAPIs (name -> sandbox)
	sandboxes map[string]*SandboxedHostAPI
}

type registeredPlugin struct {
	plugin   Plugin
	manifest GKRegistration
	enabled  bool
}

// NewManager creates a plugin manager with the given host API.
func NewManager(host HostAPI) *Manager {
	return &Manager{
		plugins:   make(map[string]*registeredPlugin),
		host:      host,
		policies:  make(map[string]*ResourcePolicy),
		sandboxes: make(map[string]*SandboxedHostAPI),
	}
}

// Host returns the manager's HostAPI instance.
func (m *Manager) Host() HostAPI {
	return m.host
}

// --- Policy management ---

// getOrCreatePolicy returns the existing policy for a plugin, or creates a default one.
// If the plugin declares resources, they're used as the initial request (but
// platform defaults still apply as the effective policy until admin approves).
func (m *Manager) getOrCreatePolicy(name string, requested *ResourceRequest) *ResourcePolicy {
	if p, ok := m.policies[name]; ok {
		return p
	}
	
	// Try to load from database first
	ctx := context.Background()
	if policy, err := m.loadPolicy(ctx, name); err == nil {
		m.policies[name] = policy
		return policy
	}
	
	// Create default policy if not found in database
	policy := DefaultResourcePolicy(name)
	m.policies[name] = &policy
	return &policy
}

// SetPolicy sets the resource policy for a plugin (admin override).
// Policy changes take effect immediately and are persisted to the database.
func (m *Manager) SetPolicy(name string, policy ResourcePolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	policy.PluginName = name
	m.policies[name] = &policy

	// Persist policy to database
	ctx := context.Background()
	if err := m.savePolicy(ctx, name, &policy); err != nil {
		// Log but don't fail - in-memory state is still correct
		fmt.Printf("Warning: failed to persist plugin policy for %s: %v\n", name, err)
	}

	// If plugin is already loaded, update its sandbox policy immediately
	if sandbox, ok := m.sandboxes[name]; ok {
		sandbox.UpdatePolicy(policy)
	}
}

// GetPolicy returns the current policy for a plugin.
func (m *Manager) GetPolicy(name string) (ResourcePolicy, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.policies[name]
	if !ok {
		return ResourcePolicy{}, false
	}
	return *p, true
}

// AllPolicies returns all current policies.
func (m *Manager) AllPolicies() map[string]ResourcePolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]ResourcePolicy, len(m.policies))
	for k, v := range m.policies {
		result[k] = *v
	}
	return result
}

// PluginStats returns resource usage stats for a plugin.
func (m *Manager) PluginStats(name string) (StatsSnapshot, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sandboxes[name]
	if !ok {
		return StatsSnapshot{}, false
	}
	return s.Stats(), true
}

// AllPluginStats returns resource usage stats for all plugins.
func (m *Manager) AllPluginStats() []StatsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]StatsSnapshot, 0, len(m.sandboxes))
	for _, s := range m.sandboxes {
		result = append(result, s.Stats())
	}
	return result
}

// defaultDisabledPlugins lists plugins that are disabled by default.
// These are development/example plugins not intended for production use.
// They can still be enabled via the admin UI or API.
var defaultDisabledPlugins = map[string]bool{
	"hello":        true,
	"hello-wasm":   true,
	"hello-grpc":   true,
	"test-hostapi": true,
}

// pluginConfigKey returns the sysconfig key for a plugin's enabled state.
func pluginConfigKey(name string) string {
	return "Plugin::" + name + "::Enabled"
}

// loadPluginEnabled checks if a plugin is enabled via sysconfig.
// Queries sysconfig_modified then sysconfig_default for an explicit
// enabled/disabled setting. If a valid "effective_value" is found,
// that value is authoritative. If no valid sysconfig entry exists
// (no DB, mock DB, fresh install without seed data), defaults to enabled.
//
// To disable example plugins in production, seed sysconfig_default with
// effective_value="0" for Plugin::<name>::Enabled during DB migration.
func (m *Manager) loadPluginEnabled(ctx context.Context, name string) bool {
	if m.host == nil {
		return true
	}

	key := pluginConfigKey(name)
	
	// Query sysconfig_modified first (user overrides)
	query := `
		SELECT effective_value FROM sysconfig_modified 
		WHERE name = ? AND is_valid = 1 
		ORDER BY change_time DESC LIMIT 1
	`
	rows, err := m.host.DBQuery(ctx, query, key)
	if err == nil && len(rows) > 0 {
		if val, ok := rows[0]["effective_value"].(string); ok {
			return val != "0" && val != "false"
		}
	}
	
	// Fall back to sysconfig_default
	query = `
		SELECT effective_value FROM sysconfig_default 
		WHERE name = ? AND is_valid = 1 
		LIMIT 1
	`
	rows, err = m.host.DBQuery(ctx, query, key)
	if err == nil && len(rows) > 0 {
		if val, ok := rows[0]["effective_value"].(string); ok {
			return val != "0" && val != "false"
		}
	}
	
	// No sysconfig entry found — default to enabled.
	// Example plugins are disabled via sysconfig seed data in DB migrations,
	// not via hardcoded logic here.
	return true
}

// seedDefaultDisabled inserts a sysconfig_default entry to disable example
// plugins on first registration. Only runs when the DB is reachable and
// no entry already exists. This is a no-op for non-example plugins and
// for test environments with mock hosts.
func (m *Manager) seedDefaultDisabled(ctx context.Context, name string) {
	if m.host == nil || !defaultDisabledPlugins[name] {
		return
	}

	key := pluginConfigKey(name)

	// Check if an entry already exists in sysconfig_default
	rows, err := m.host.DBQuery(ctx, `SELECT 1 FROM sysconfig_default WHERE name = ? LIMIT 1`, key)
	if err != nil || len(rows) > 0 {
		return // DB error (mock/test) or entry already exists
	}

	// Seed disabled state — fill all NOT NULL columns
	m.host.DBExec(ctx, `
		INSERT INTO sysconfig_default 
		(name, description, navigation, is_invisible, is_readonly, is_required, is_valid, 
		 has_configlevel, user_modification_possible, user_modification_active,
		 xml_content_raw, xml_content_parsed, xml_filename, effective_value, 
		 is_dirty, exclusive_lock_guid, create_by, change_by, create_time, change_time)
		VALUES (?, 'Plugin enabled state', 'Admin::Plugins', 1, 0, 0, 1,
		 0, 1, 0,
		 '', '', '', '0',
		 0, '', 1, 1, NOW(), NOW())
	`, key)
}

// savePluginEnabled persists a plugin's enabled state to sysconfig_modified.
func (m *Manager) savePluginEnabled(ctx context.Context, name string, enabled bool) error {
	if m.host == nil {
		return nil // No host API, can't persist
	}

	key := pluginConfigKey(name)
	val := "1"
	if !enabled {
		val = "0"
	}

	// Look up the sysconfig_default ID for this key
	rows, err := m.host.DBQuery(ctx, `SELECT id FROM sysconfig_default WHERE name = ? LIMIT 1`, key)
	if err != nil || len(rows) == 0 {
		return fmt.Errorf("no sysconfig_default entry for %q — seed it first via seedDefaultDisabled", key)
	}

	defaultID := rows[0]["id"]

	// Upsert into sysconfig_modified with the correct FK reference
	query := `
		INSERT INTO sysconfig_modified 
		(sysconfig_default_id, name, effective_value, is_valid, user_modification_active, 
		 is_dirty, reset_to_default, create_by, change_by, create_time, change_time)
		VALUES (?, ?, ?, 1, 0, 0, 0, 1, 1, NOW(), NOW())
		ON DUPLICATE KEY UPDATE effective_value = ?, change_time = NOW(), change_by = 1
	`
	_, err = m.host.DBExec(ctx, query, defaultID, key, val, val)
	return err
}

// SetLazyLoader sets the lazy loader for on-demand plugin loading.
func (m *Manager) SetLazyLoader(loader LazyLoader) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lazyLoader = loader
}

// Discovered returns the names of discovered but not necessarily loaded plugins.
func (m *Manager) Discovered() []string {
	if m.lazyLoader == nil {
		return nil
	}
	return m.lazyLoader.Discovered()
}

// Register loads and initializes a plugin.
func (m *Manager) Register(ctx context.Context, p Plugin) error {
	manifest := p.GKRegister()

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.plugins[manifest.Name]; exists {
		return fmt.Errorf("plugin %q already registered", manifest.Name)
	}

	// Create sandboxed HostAPI for this plugin
	policy := m.getOrCreatePolicy(manifest.Name, manifest.Resources)
	sandbox := NewSandboxedHostAPI(m.host, manifest.Name, *policy)
	m.sandboxes[manifest.Name] = sandbox

	// Initialize the plugin with sandboxed host API access
	if err := p.Init(ctx, sandbox); err != nil {
		delete(m.sandboxes, manifest.Name)
		return fmt.Errorf("plugin %q init failed: %w", manifest.Name, err)
	}

	// Seed default-disabled state for example plugins on first registration
	m.seedDefaultDisabled(ctx, manifest.Name)

	// Check if this plugin is enabled via sysconfig
	isEnabled := m.loadPluginEnabled(ctx, manifest.Name)

	m.plugins[manifest.Name] = &registeredPlugin{
		plugin:   p,
		manifest: manifest,
		enabled:  isEnabled,
	}

	// Load plugin translations if provided
	if manifest.I18n != nil && len(manifest.I18n.Translations) > 0 {
		m.loadPluginTranslations(manifest.Name, manifest.I18n)
	}

	// Register plugin error codes if provided
	if len(manifest.ErrorCodes) > 0 {
		m.loadPluginErrorCodes(manifest.Name, manifest.ErrorCodes)
	}

	// Register template overrides if provided
	if len(manifest.Templates) > 0 {
		if registry := GetTemplateOverrides(); registry != nil {
			registry.Register(manifest.Name, manifest.Templates)
		}
	}

	return nil
}

// loadPluginTranslations adds plugin-provided translations to the i18n system.
func (m *Manager) loadPluginTranslations(pluginName string, i18nSpec *I18nSpec) {
	i18nInst := i18n.GetInstance()
	if i18nInst == nil {
		return
	}

	namespace := i18nSpec.Namespace
	if namespace == "" {
		namespace = pluginName // Default to plugin name as namespace
	}

	// Add each translation with the plugin namespace prefix
	for lang, translations := range i18nSpec.Translations {
		for key, value := range translations {
			fullKey := namespace + "." + key
			i18nInst.AddTranslation(lang, fullKey, value)
		}
	}
}

// loadPluginErrorCodes registers plugin-provided API error codes.
func (m *Manager) loadPluginErrorCodes(pluginName string, codes []ErrorCodeSpec) {
	for _, spec := range codes {
		apierrors.Registry.Register(apierrors.ErrorCode{
			Code:       pluginName + ":" + spec.Code,
			Message:    spec.Message,
			HTTPStatus: spec.HTTPStatus,
		})
	}
}

// Unregister shuts down and removes a plugin.
func (m *Manager) Unregister(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rp, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %q not found", name)
	}

	if err := rp.plugin.Shutdown(ctx); err != nil {
		return fmt.Errorf("plugin %q shutdown failed: %w", name, err)
	}

	delete(m.plugins, name)
	delete(m.sandboxes, name)
	// Note: policy is preserved across reloads so admin settings persist
	return nil
}

// Get returns a plugin by name.
func (m *Manager) Get(name string) (Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rp, exists := m.plugins[name]
	if !exists || !rp.enabled {
		return nil, false
	}
	return rp.plugin, true
}

// PluginNotFoundError is returned when a plugin dependency is missing.
type PluginNotFoundError struct {
	PluginName   string // The missing plugin
	CallerPlugin string // The plugin that tried to call it (if known)
	Function     string // The function that was called
}

func (e *PluginNotFoundError) Error() string {
	if e.CallerPlugin != "" {
		return fmt.Sprintf("plugin %q not found (required by %q to call %q)", 
			e.PluginName, e.CallerPlugin, e.Function)
	}
	return fmt.Sprintf("plugin %q not found", e.PluginName)
}

// PluginDisabledError is returned when trying to call a disabled plugin.
type PluginDisabledError struct {
	PluginName   string
	CallerPlugin string
}

func (e *PluginDisabledError) Error() string {
	if e.CallerPlugin != "" {
		return fmt.Sprintf("plugin %q is disabled (required by %q)", e.PluginName, e.CallerPlugin)
	}
	return fmt.Sprintf("plugin %q is disabled", e.PluginName)
}

// Call invokes a function on a specific plugin.
// If lazy loading is enabled and the plugin isn't loaded yet, it will be loaded first.
func (m *Manager) Call(ctx context.Context, pluginName, fn string, args []byte) ([]byte, error) {
	m.mu.RLock()
	rp, exists := m.plugins[pluginName]
	lazyLoader := m.lazyLoader
	m.mu.RUnlock()

	// Try lazy loading if plugin not found
	if !exists && lazyLoader != nil {
		if err := lazyLoader.EnsureLoaded(ctx, pluginName); err != nil {
			return nil, &PluginNotFoundError{PluginName: pluginName, Function: fn}
		}
		// Re-check after lazy load
		m.mu.RLock()
		rp, exists = m.plugins[pluginName]
		m.mu.RUnlock()
	}

	if !exists {
		return nil, &PluginNotFoundError{PluginName: pluginName, Function: fn}
	}
	if !rp.enabled {
		return nil, &PluginDisabledError{PluginName: pluginName}
	}

	return rp.plugin.Call(ctx, fn, args)
}

// CallFrom invokes a function on a plugin, with caller context for better errors.
// If lazy loading is enabled and the plugin isn't loaded yet, it will be loaded first.
func (m *Manager) CallFrom(ctx context.Context, callerPlugin, targetPlugin, fn string, args []byte) ([]byte, error) {
	m.mu.RLock()
	rp, exists := m.plugins[targetPlugin]
	lazyLoader := m.lazyLoader
	m.mu.RUnlock()

	// Try lazy loading if plugin not found
	if !exists && lazyLoader != nil {
		if err := lazyLoader.EnsureLoaded(ctx, targetPlugin); err != nil {
			return nil, &PluginNotFoundError{
				PluginName:   targetPlugin,
				CallerPlugin: callerPlugin,
				Function:     fn,
			}
		}
		// Re-check after lazy load
		m.mu.RLock()
		rp, exists = m.plugins[targetPlugin]
		m.mu.RUnlock()
	}

	if !exists {
		return nil, &PluginNotFoundError{
			PluginName:   targetPlugin,
			CallerPlugin: callerPlugin,
			Function:     fn,
		}
	}
	if !rp.enabled {
		return nil, &PluginDisabledError{
			PluginName:   targetPlugin,
			CallerPlugin: callerPlugin,
		}
	}

	return rp.plugin.Call(ctx, fn, args)
}

// ReplacePlugin atomically replaces an existing plugin with a new one.
// This prevents race conditions during hot reload.
func (m *Manager) ReplacePlugin(ctx context.Context, oldName string, newPlugin Plugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if old plugin exists
	oldRp, exists := m.plugins[oldName]
	if !exists {
		return fmt.Errorf("plugin %q not found for replacement", oldName)
	}

	newManifest := newPlugin.GKRegister()
	if newManifest.Name != oldName {
		return fmt.Errorf("new plugin name %q doesn't match old name %q", newManifest.Name, oldName)
	}

	// Initialize new plugin with existing policy and settings
	policy := m.getOrCreatePolicy(newManifest.Name, newManifest.Resources)
	sandbox := NewSandboxedHostAPI(m.host, newManifest.Name, *policy)

	if err := newPlugin.Init(ctx, sandbox); err != nil {
		return fmt.Errorf("new plugin %q init failed: %w", newManifest.Name, err)
	}

	// Shutdown old plugin
	if err := oldRp.plugin.Shutdown(ctx); err != nil {
		// Log error but continue - we want to replace it anyway
		fmt.Printf("Warning: old plugin %q shutdown error: %v\n", oldName, err)
	}

	// Atomically replace the plugin
	m.plugins[oldName] = &registeredPlugin{
		plugin:   newPlugin,
		manifest: newManifest,
		enabled:  oldRp.enabled, // Preserve enabled state
	}
	m.sandboxes[oldName] = sandbox

	return nil
}

// List returns all registered plugin manifests.
func (m *Manager) List() []GKRegistration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	manifests := make([]GKRegistration, 0, len(m.plugins))
	for _, rp := range m.plugins {
		manifests = append(manifests, rp.manifest)
	}
	return manifests
}

// IsEnabled returns whether a plugin is enabled.
func (m *Manager) IsEnabled(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rp, exists := m.plugins[name]
	if !exists {
		return false
	}
	return rp.enabled
}

// Enable enables a previously disabled plugin.
// If the plugin is lazy-loaded and not yet registered, it will be loaded first.
func (m *Manager) Enable(name string) error {
	m.mu.Lock()
	rp, exists := m.plugins[name]
	m.mu.Unlock()

	// Try lazy loading if not registered
	if !exists && m.lazyLoader != nil {
		if err := m.lazyLoader.EnsureLoaded(context.Background(), name); err != nil {
			return fmt.Errorf("plugin %q not found", name)
		}
		// Re-check after loading
		m.mu.Lock()
		rp, exists = m.plugins[name]
		m.mu.Unlock()
		if !exists {
			return fmt.Errorf("plugin %q not found", name)
		}
	} else if !exists {
		return fmt.Errorf("plugin %q not found", name)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	rp.enabled = true

	// Persist state to sysconfig
	ctx := context.Background()
	if err := m.savePluginEnabled(ctx, name, true); err != nil {
		// Log but don't fail - in-memory state is still correct
		fmt.Printf("Warning: failed to save plugin state: %v\n", err)
	}
	return nil
}

// Disable disables a plugin without unloading it.
func (m *Manager) Disable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rp, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %q not found", name)
	}
	rp.enabled = false

	// Persist state to sysconfig
	ctx := context.Background()
	if err := m.savePluginEnabled(ctx, name, false); err != nil {
		// Log but don't fail - in-memory state is still correct
		fmt.Printf("Warning: failed to save plugin state: %v\n", err)
	}
	return nil
}

// Routes returns all routes from all enabled plugins.
func (m *Manager) Routes() []PluginRoute {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var routes []PluginRoute
	for name, rp := range m.plugins {
		if !rp.enabled {
			continue
		}
		for _, r := range rp.manifest.Routes {
			routes = append(routes, PluginRoute{
				PluginName: name,
				RouteSpec:  r,
			})
		}
	}
	return routes
}

// PluginRoute pairs a route spec with its plugin name.
type PluginRoute struct {
	PluginName string
	RouteSpec  RouteSpec
}

// MenuItems returns all menu items from all enabled plugins.
func (m *Manager) MenuItems(location string) []PluginMenuItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var items []PluginMenuItem
	for name, rp := range m.plugins {
		if !rp.enabled {
			continue
		}
		for _, mi := range rp.manifest.MenuItems {
			if mi.Location == location {
				items = append(items, PluginMenuItem{
					PluginName:   name,
					MenuItemSpec: mi,
				})
			}
		}
	}
	return items
}

// PluginMenuItem pairs a menu item spec with its plugin name.
type PluginMenuItem struct {
	PluginName string
	MenuItemSpec
}

// Widgets returns all widgets from all enabled plugins for a location.
func (m *Manager) Widgets(location string) []PluginWidget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var widgets []PluginWidget
	for name, rp := range m.plugins {
		if !rp.enabled {
			continue
		}
		for _, w := range rp.manifest.Widgets {
			if w.Location == location {
				widgets = append(widgets, PluginWidget{
					PluginName: name,
					WidgetSpec: w,
				})
			}
		}
	}
	return widgets
}

// PluginWidget pairs a widget spec with its plugin name.
type PluginWidget struct {
	PluginName string
	WidgetSpec
}

// AllWidgets returns widgets from all plugins (including lazy-loaded) for a location.
// This triggers lazy loading for all discovered plugins to ensure complete widget list.
func (m *Manager) AllWidgets(location string) []PluginWidget {
	// First, trigger lazy loading for all discovered plugins
	if m.lazyLoader != nil {
		ctx := context.Background()
		for _, name := range m.lazyLoader.Discovered() {
			// Try to load each discovered plugin (errors are ignored)
			_ = m.lazyLoader.EnsureLoaded(ctx, name)
		}
	}

	// Now return widgets from all loaded plugins
	return m.Widgets(location)
}

// Jobs returns all jobs from all enabled plugins.
func (m *Manager) Jobs() []PluginJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var jobs []PluginJob
	for name, rp := range m.plugins {
		if !rp.enabled {
			continue
		}
		for _, j := range rp.manifest.Jobs {
			jobs = append(jobs, PluginJob{
				PluginName: name,
				JobSpec:    j,
			})
		}
	}
	return jobs
}

// PluginJob pairs a job spec with its plugin name.
type PluginJob struct {
	PluginName string
	JobSpec
}

// policyConfigKey returns the sysconfig key for a plugin's policy.
func policyConfigKey(name string) string {
	return "Plugin::" + name + "::Policy"
}

// loadPolicy loads a plugin's resource policy from the database.
func (m *Manager) loadPolicy(ctx context.Context, name string) (*ResourcePolicy, error) {
	if m.host == nil {
		return nil, fmt.Errorf("no host API available")
	}

	key := policyConfigKey(name)
	
	// Query sysconfig_modified first (admin overrides)
	query := `
		SELECT effective_value FROM sysconfig_modified 
		WHERE name = ? AND is_valid = 1 
		ORDER BY change_time DESC LIMIT 1
	`
	rows, err := m.host.DBQuery(ctx, query, key)
	if err == nil && len(rows) > 0 {
		if jsonStr, ok := rows[0]["effective_value"].(string); ok {
			var policy ResourcePolicy
			if err := json.Unmarshal([]byte(jsonStr), &policy); err != nil {
				return nil, fmt.Errorf("invalid policy JSON in database: %w", err)
			}
			return &policy, nil
		}
	}
	
	return nil, fmt.Errorf("policy not found in database")
}

// savePolicy persists a plugin's resource policy to the database.
func (m *Manager) savePolicy(ctx context.Context, name string, policy *ResourcePolicy) error {
	if m.host == nil {
		return nil // No host API, can't persist
	}

	key := policyConfigKey(name)
	
	// Serialize policy as JSON
	jsonData, err := json.Marshal(policy)
	if err != nil {
		return fmt.Errorf("failed to serialize policy: %w", err)
	}
	
	jsonStr := string(jsonData)

	// Upsert into sysconfig_modified
	query := `
		INSERT INTO sysconfig_modified (sysconfig_default_id, name, effective_value, is_valid, create_by, change_by, create_time, change_time)
		VALUES (0, ?, ?, 1, 1, 1, NOW(), NOW())
		ON DUPLICATE KEY UPDATE effective_value = ?, change_time = NOW(), change_by = 1
	`
	_, err = m.host.DBExec(ctx, query, key, jsonStr, jsonStr)
	return err
}

// HiddenMenuItems returns all menu item IDs that enabled plugins want hidden.
func (m *Manager) HiddenMenuItems() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var items []string
	for _, rp := range m.plugins {
		if !rp.enabled {
			continue
		}
		items = append(items, rp.manifest.HideMenuItems...)
	}
	return items
}

// LandingPage returns the landing page URL from the first enabled plugin that declares one.
// If no plugin sets a landing page, returns empty string.
func (m *Manager) LandingPage() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, rp := range m.plugins {
		if !rp.enabled {
			continue
		}
		if rp.manifest.LandingPage != "" {
			return rp.manifest.LandingPage
		}
	}
	return ""
}

// ShutdownAll shuts down all plugins gracefully.
func (m *Manager) ShutdownAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for name, rp := range m.plugins {
		if err := rp.plugin.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("plugin %q: %w", name, err))
		}
	}

	m.plugins = make(map[string]*registeredPlugin)

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}
