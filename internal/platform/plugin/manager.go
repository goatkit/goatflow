package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/goatkit/goatflow/internal/platform/apierrors"
	"github.com/goatkit/goatflow/internal/platform/customfields"
	"github.com/goatkit/goatflow/internal/platform/deletion"
	"github.com/goatkit/goatflow/internal/platform/i18n"
	"github.com/goatkit/goatflow/internal/platform/pluginui"
	"github.com/goatkit/goatflow/internal/platform/version"
)

// LazyLoader is the interface for lazy-loading plugins on demand.
type LazyLoader interface {
	EnsureLoaded(ctx context.Context, name string) error
	Discovered() []string
	Forget(name string)
}

// Manager handles plugin lifecycle: loading, registration, and invocation.
type Manager struct {
	mu         sync.RWMutex
	plugins    map[string]*registeredPlugin
	host       HostAPI
	lazyLoader LazyLoader // Optional: for lazy loading support

	// OnPluginLoaded is called after a plugin is lazy-loaded successfully.
	// The API layer sets this to refresh MCP tools, rebuild dynamic engine, etc.
	OnPluginLoaded func()

	// Per-plugin resource policies (name -> policy)
	policies map[string]*ResourcePolicy

	// declaredPolicy marks policies built from the plugin's own declarations
	// (no admin-stored policy). They are rebuilt on every (re)load so a new
	// plugin version's declarations take effect.
	declaredPolicy map[string]bool

	// Per-plugin sandboxed HostAPIs (name -> sandbox)
	sandboxes map[string]*SandboxedHostAPI

	// healthCheckerStarted guards StartHealthChecker so it can't be
	// spun up twice. See internal/platform/plugin/health.go for the check loop.
	healthCheckerStarted bool

	// restarter is the auto-recovery hook (loader.Reload). When non-nil,
	// the health checker will dispatch restart attempts for unhealthy
	// plugins with exponential backoff. Set via SetRestarter.
	restarter Restarter

	// scheduler is the platform scheduler used for registering plugin jobs.
	// Set via SetScheduler before calling RegisterPluginJobs.
	scheduler PlatformScheduler
}

type registeredPlugin struct {
	plugin   Plugin
	manifest GKRegistration
	enabled  bool

	// appMode is the administrator switch for the manifest's AppMode
	// (Plugin::<name>::AppMode); meaningless when none is declared.
	appMode bool

	// health tracks the most recent Ping probe outcome for this
	// plugin. Guarded by Manager.mu. See health.go for the probe
	// loop that updates it.
	health healthState
}

// installDeletionPreDispatchHook tells the deletion service to call back
// into the manager just before dispatching cascades so any
// discovered-but-unloaded plugin gets loaded (and therefore registered
// with the cascade registry) in time. Called once per Manager instance.
func (m *Manager) installDeletionPreDispatchHook() {
	deletion.SetPreDispatchHook(func(ctx context.Context, entityType string) {
		if m.lazyLoader == nil {
			return
		}
		loaded := 0
		for _, name := range m.lazyLoader.Discovered() {
			m.mu.RLock()
			_, present := m.plugins[name]
			m.mu.RUnlock()
			if present {
				continue
			}
			if err := m.lazyLoader.EnsureLoaded(ctx, name); err != nil {
				slog.Warn("pre-cascade load failed", "plugin", name, "error", err)
				continue
			}
			loaded++
		}
		if loaded > 0 {
			slog.Info("pre-cascade loaded plugins", "count", loaded, "entity_type", entityType)
		}
	})
}

// NewManager creates a plugin manager with the given host API.
func NewManager(host HostAPI) *Manager {
	m := &Manager{
		plugins:   make(map[string]*registeredPlugin),
		host:      host,
		policies:  make(map[string]*ResourcePolicy),
		sandboxes: make(map[string]*SandboxedHostAPI),
	}
	SetGlobalManager(m)
	m.installDeletionPreDispatchHook()
	return m
}

// Host returns the manager's HostAPI instance.
func (m *Manager) Host() HostAPI {
	return m.host
}

// --- Policy management ---

// getOrCreatePolicy returns the existing policy for a plugin, or creates one.
// An admin-stored policy always wins. Without one, the plugin gets the
// limits it declares in its ResourceRequest (defaults for the rest) and
// exactly the permissions it declares (the default permissions when it
// declares none). A declared entity "hard_delete" is dropped: permanent
// deletion needs an explicit admin grant in a stored policy.
func (m *Manager) getOrCreatePolicy(name string, requested *ResourceRequest) *ResourcePolicy {
	if p, ok := m.policies[name]; ok && !m.declaredPolicy[name] {
		return p
	}

	// Try to load from database first
	ctx := context.Background()
	if policy, err := m.loadPolicy(ctx, name); err == nil {
		m.policies[name] = policy
		return policy
	}

	policy := policyFromDeclarations(name, requested)
	m.policies[name] = &policy
	m.markDeclared(name)
	return &policy
}

func (m *Manager) markDeclared(name string) {
	if m.declaredPolicy == nil {
		m.declaredPolicy = make(map[string]bool)
	}
	m.declaredPolicy[name] = true
}

// ResolvePolicy returns the policy a plugin runs under: the admin-stored
// policy when there is one, otherwise requested applied to the defaults
// (see policyFromDeclarations). Nothing is cached.
func (m *Manager) ResolvePolicy(name string, requested *ResourceRequest) ResourcePolicy {
	m.mu.RLock()
	var stored *ResourcePolicy
	if p, ok := m.policies[name]; ok && !m.declaredPolicy[name] {
		cp := *p
		stored = &cp
	}
	m.mu.RUnlock()
	if stored != nil {
		return *stored
	}
	if p, err := m.loadPolicy(context.Background(), name); err == nil {
		return *p
	}
	return policyFromDeclarations(name, requested)
}

// policyFromDeclarations builds the effective policy for a plugin that has
// no admin-stored policy: default limits overridden by the declared ones,
// plus the declared permissions.
func policyFromDeclarations(name string, requested *ResourceRequest) ResourcePolicy {
	policy := DefaultResourcePolicy(name)
	if requested == nil {
		return policy
	}
	if requested.MemoryMB > 0 {
		policy.MemoryMB = requested.MemoryMB
	}
	if requested.CPUMillis > 0 {
		policy.CPUMillis = requested.CPUMillis
	}
	if requested.CallTimeout != "" {
		policy.CallTimeout = requested.CallTimeout
	}
	if requested.InitTimeout != "" {
		policy.InitTimeout = requested.InitTimeout
	}
	if requested.ShutdownTimeout != "" {
		policy.ShutdownTimeout = requested.ShutdownTimeout
	}
	if requested.MaxFileStorageBytes > 0 {
		policy.MaxFileStorageBytes = requested.MaxFileStorageBytes
	}
	if requested.MaxHTTPResponseMB > 0 {
		policy.MaxHTTPResponseMB = requested.MaxHTTPResponseMB
	}
	if len(requested.Permissions) > 0 {
		perms := make([]Permission, 0, len(requested.Permissions))
		for _, p := range requested.Permissions {
			if p.Type == "entity" && p.Access == "hard_delete" {
				continue
			}
			perms = append(perms, p)
		}
		policy.Permissions = perms
	}
	return policy
}

// Fallbacks for a policy duration that is empty or invalid.
const (
	DefaultCallTimeout = 30 * time.Second
	defaultInitTimeout = 10 * time.Second
)

// LimitDuration parses a policy duration such as CallTimeout, returning
// fallback when it is empty, invalid or not positive.
func LimitDuration(value string, fallback time.Duration) time.Duration {
	if d, err := time.ParseDuration(value); err == nil && d > 0 {
		return d
	}
	return fallback
}

// callTimeoutSetter is a plugin runtime that enforces a per-call deadline
// itself (gRPC and WASM plugins).
type callTimeoutSetter interface {
	SetCallTimeout(time.Duration)
}

// applyCallTimeout gives a plugin its policy's per-call deadline. The
// deadline is not a launch limit, so a running plugin picks up a change.
func applyCallTimeout(p Plugin, policy ResourcePolicy) {
	if s, ok := p.(callTimeoutSetter); ok {
		s.SetCallTimeout(LimitDuration(policy.CallTimeout, DefaultCallTimeout))
	}
}

// applyLivePolicy pushes a changed policy to a loaded plugin's sandbox and
// call deadline. Launch limits wait for the next load. Caller holds m.mu.
func (m *Manager) applyLivePolicy(name string, policy ResourcePolicy) {
	if sandbox, ok := m.sandboxes[name]; ok {
		sandbox.UpdatePolicy(policy)
	}
	if rp, ok := m.plugins[name]; ok {
		applyCallTimeout(rp.plugin, policy)
	}
}

// SetPolicy sets the resource policy for a plugin (admin override).
// Policy changes take effect immediately and are persisted to the database;
// launch limits (see ResourcePolicy.Launch) apply from the next load.
func (m *Manager) SetPolicy(name string, policy ResourcePolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	policy.PluginName = name
	m.policies[name] = &policy
	delete(m.declaredPolicy, name)

	// Persist policy to database
	ctx := context.Background()
	if err := m.savePolicy(ctx, name, &policy); err != nil {
		// Log but don't fail - in-memory state is still correct
		fmt.Printf("Warning: failed to persist plugin policy for %s: %v\n", name, err)
	}

	m.applyLivePolicy(name, policy)
}

// HasStoredPolicy reports whether an admin-stored policy is in effect for a
// plugin, rather than one built from its declarations.
func (m *Manager) HasStoredPolicy(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.policies[name]
	return ok && !m.declaredPolicy[name]
}

// ResetPolicy deletes a plugin's admin-stored policy and returns the policy
// built from the loaded plugin's declarations (the defaults when it is not
// loaded), applied like SetPolicy.
func (m *Manager) ResetPolicy(name string) (ResourcePolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.host != nil {
		if _, err := m.host.DBExec(context.Background(),
			`DELETE FROM sysconfig_modified WHERE name = ?`, policyConfigKey(name)); err != nil {
			return ResourcePolicy{}, fmt.Errorf("delete stored policy: %w", err)
		}
	}
	var requested *ResourceRequest
	if rp, ok := m.plugins[name]; ok {
		requested = rp.manifest.Resources
	}
	policy := policyFromDeclarations(name, requested)
	m.policies[name] = &policy
	m.markDeclared(name)
	m.applyLivePolicy(name, policy)
	return policy, nil
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

// PolicyFor returns the policy for a plugin, or nil if not found.
func (m *Manager) PolicyFor(name string) *ResourcePolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.policies[name]
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

// loadPluginEnabled checks if a plugin is enabled via sysconfig; see
// loadPluginSwitch. To disable example plugins in production, seed
// sysconfig_default with effective_value="0" for Plugin::<name>::Enabled
// during DB migration.
func (m *Manager) loadPluginEnabled(ctx context.Context, name string) bool {
	return m.loadPluginSwitch(ctx, name, pluginConfigKey(name))
}

// loadPluginSwitch reads an administrator switch for a plugin from sysconfig:
// sysconfig_modified (overrides) first, then sysconfig_default. A valid
// effective_value is authoritative; no entry (fresh install without seed
// data) means on. A failed lookup leaves the switch off: treating a DB
// error as "on" would re-enable what an administrator switched off.
func (m *Manager) loadPluginSwitch(ctx context.Context, name, key string) bool {
	if m.host == nil {
		return true
	}

	for _, query := range []string{`
		SELECT effective_value FROM sysconfig_modified 
		WHERE name = ? AND is_valid = 1 
		ORDER BY change_time DESC LIMIT 1
	`, `
		SELECT effective_value FROM sysconfig_default 
		WHERE name = ? AND is_valid = 1 
		LIMIT 1
	`} {
		rows, err := m.host.DBQuery(ctx, query, key)
		if err != nil {
			slog.Error("plugin switch lookup failed; leaving it off", "plugin", name, "key", key, "error", err)
			return false
		}
		if len(rows) > 0 {
			if val, ok := rows[0]["effective_value"].(string); ok {
				return val != "0" && val != "false"
			}
		}
	}

	// No sysconfig entry found — on. Example plugins are disabled via
	// sysconfig seed data in DB migrations, not via hardcoded logic here.
	return true
}

// seedDefaultDisabled inserts a sysconfig_default entry to disable example
// plugins on first registration. This is a no-op for non-example plugins.
func (m *Manager) seedDefaultDisabled(ctx context.Context, name string) {
	if !defaultDisabledPlugins[name] {
		return
	}
	m.seedPluginSwitch(ctx, name, pluginConfigKey(name), "Plugin enabled state", "0")
}

// seedPluginSwitch inserts the sysconfig_default row behind an administrator
// switch, with the given initial value, so savePluginSwitch has a row to
// modify. Only runs when the DB is reachable and no entry exists yet; a
// no-op for test environments with mock hosts.
func (m *Manager) seedPluginSwitch(ctx context.Context, name, key, description, value string) {
	if m.host == nil {
		return
	}

	// Check if an entry already exists in sysconfig_default
	rows, err := m.host.DBQuery(ctx, `SELECT 1 FROM sysconfig_default WHERE name = ? LIMIT 1`, key)
	if err != nil || len(rows) > 0 {
		return // DB error (mock/test) or entry already exists
	}

	// Fill all NOT NULL columns
	if _, err := m.host.DBExec(ctx, `
		INSERT INTO sysconfig_default 
		(name, description, navigation, is_invisible, is_readonly, is_required, is_valid, 
		 has_configlevel, user_modification_possible, user_modification_active,
		 xml_content_raw, xml_content_parsed, xml_filename, effective_value, 
		 is_dirty, exclusive_lock_guid, create_by, change_by, create_time, change_time)
		VALUES (?, ?, ?, 1, 0, 0, 1,
		 0, 1, 0,
		 '', '', '', ?,
		 0, '', 1, 1, NOW(), NOW())
	`, key, description, "Admin::Plugins", value); err != nil {
		slog.Warn("failed to seed plugin switch", "plugin", name, "key", key, "error", err)
	}
}

// savePluginEnabled persists a plugin's enabled state to sysconfig_modified.
func (m *Manager) savePluginEnabled(ctx context.Context, name string, enabled bool) error {
	return m.savePluginSwitch(ctx, pluginConfigKey(name), enabled)
}

// savePluginSwitch persists an administrator switch to sysconfig_modified.
// The sysconfig_default row must exist (seedPluginSwitch).
func (m *Manager) savePluginSwitch(ctx context.Context, key string, on bool) error {
	if m.host == nil {
		return nil // No host API, can't persist
	}

	val := "1"
	if !on {
		val = "0"
	}

	// Look up the sysconfig_default ID for this key
	rows, err := m.host.DBQuery(ctx, `SELECT id FROM sysconfig_default WHERE name = ? LIMIT 1`, key)
	if err != nil || len(rows) == 0 {
		return fmt.Errorf("no sysconfig_default entry for %q — seed it first via seedPluginSwitch", key)
	}

	defaultID := rows[0]["id"]

	return m.setGlobalSysconfig(ctx, defaultID, key, val)
}

// SetLazyLoader sets the lazy loader for on-demand plugin loading.
func (m *Manager) SetLazyLoader(loader LazyLoader) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lazyLoader = loader
}

// SetScheduler sets the platform scheduler for plugin job registration.
func (m *Manager) SetScheduler(sched PlatformScheduler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scheduler = sched
}

// Scheduler returns the platform scheduler, or nil if not set.
func (m *Manager) Scheduler() PlatformScheduler {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.scheduler
}

// Discovered returns the names of discovered but not necessarily loaded plugins.
func (m *Manager) Discovered() []string {
	if m.lazyLoader == nil {
		return nil
	}
	return m.lazyLoader.Discovered()
}

// Forget tells the lazy loader to forget a plugin so it no longer appears in Discovered().
func (m *Manager) Forget(name string) {
	if m.lazyLoader != nil {
		m.lazyLoader.Forget(name)
	}
}

// Register loads and initializes a plugin. A plugin whose MinHostVersion is
// newer than the running GoatFlow is refused before Init.
func (m *Manager) Register(ctx context.Context, p Plugin) error {
	manifest := p.GKRegister()

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.plugins[manifest.Name]; exists {
		return fmt.Errorf("plugin %q already registered", manifest.Name)
	}
	if err := checkHostVersion(manifest); err != nil {
		return err
	}

	// Create sandboxed HostAPI for this plugin
	policy := m.getOrCreatePolicy(manifest.Name, manifest.Resources)
	sandbox := NewSandboxedHostAPI(m.host, manifest.Name, *policy)
	m.sandboxes[manifest.Name] = sandbox

	// Side effects that must exist before Init — the plugin's
	// InitWithHost commonly reads from / writes to custom fields during
	// its own migrations and seeding.
	m.applyManifestSideEffectsPreInit(ctx, manifest)

	// Initialize the plugin with sandboxed host API access
	if err := initWithTimeout(ctx, p, sandbox, *policy); err != nil {
		delete(m.sandboxes, manifest.Name)
		return fmt.Errorf("plugin %q init failed: %w", manifest.Name, err)
	}
	applyCallTimeout(p, *policy)

	// Seed default-disabled state for example plugins on first registration
	m.seedDefaultDisabled(ctx, manifest.Name)

	// Check if this plugin is enabled via sysconfig
	isEnabled := m.loadPluginEnabled(ctx, manifest.Name)

	m.plugins[manifest.Name] = &registeredPlugin{
		plugin:   p,
		manifest: manifest,
		enabled:  isEnabled,
		appMode:  m.loadAppModeEnabled(ctx, manifest),
		health:   healthInitForRegister(),
	}

	// Remaining side effects (translations, error codes, templates,
	// cascades, UIs) — safe to run after the plugin is in the map.
	m.applyManifestSideEffectsPostInit(ctx, manifest)

	return nil
}

// checkHostVersion refuses a plugin that declares a MinHostVersion newer than
// the running GoatFlow, recording why in the log and the plugin log buffer.
func checkHostVersion(manifest GKRegistration) error {
	err := version.RequireHost(manifest.Name, manifest.Version, manifest.MinHostVersion)
	if err != nil {
		slog.Error("plugin refused: incompatible GoatFlow version", "plugin", manifest.Name, "error", err)
		GetLogBuffer().Log(manifest.Name, "error", "Plugin not loaded: "+err.Error(), nil)
	}
	return err
}

// initWithTimeout runs p.Init within the policy's InitTimeout. A plugin
// whose Init ignores its context is abandoned at the deadline; the caller
// shuts it down.
func initWithTimeout(ctx context.Context, p Plugin, host HostAPI, policy ResourcePolicy) error {
	timeout := LimitDuration(policy.InitTimeout, defaultInitTimeout)
	ictx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- p.Init(ictx, host) }()
	var err error
	select {
	case err = <-done:
	case <-ictx.Done():
		err = ictx.Err()
	}
	if err != nil && ctx.Err() == nil && errors.Is(ictx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("init timed out after %s: %w", timeout, err)
	}
	return err
}

// Unload removes a plugin from the manager's in-memory state.
// The plugin directory is removed by HandlePluginUnfull; this clears
// the registry, sandbox, and policy entries so the plugin is fully
// uninstalled without requiring a restart.
func (m *Manager) Unload(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.plugins, name)
	delete(m.sandboxes, name)
	delete(m.policies, name)
	delete(m.declaredPolicy, name)
}

// applyManifestSideEffectsPreInit runs every side effect that the
// plugin's own Init expects to already be in place. Currently just
// custom-field definitions.
func (m *Manager) applyManifestSideEffectsPreInit(ctx context.Context, manifest GKRegistration) {
	if len(manifest.CustomFields) > 0 {
		if err := m.registerCustomFields(ctx, manifest.Name, manifest.CustomFields); err != nil {
			slog.Warn("failed to register custom fields", "plugin", manifest.Name, "error", err)
		}
	}
}

// applyManifestSideEffectsPostInit installs the side effects that do
// not need to be visible to the plugin's own Init: translations, error
// codes, template overrides, cascade handlers, and UIs. Shared by the
// Register and ReplacePlugin paths so hot-reload reliably picks up
// manifest changes (previously only cascades re-registered on replace,
// leaving stale translations / error codes / CF defs / UIs behind).
func (m *Manager) applyManifestSideEffectsPostInit(ctx context.Context, manifest GKRegistration) {
	if manifest.I18n != nil && len(manifest.I18n.Translations) > 0 {
		m.loadPluginTranslations(manifest.Name, manifest.I18n)
	}
	if len(manifest.ErrorCodes) > 0 {
		m.loadPluginErrorCodes(manifest.Name, manifest.ErrorCodes)
	}
	if len(manifest.Templates) > 0 {
		if registry := GetTemplateOverrides(); registry != nil {
			registry.Register(manifest.Name, manifest.Templates)
		}
	}
	// Cascade re-registration is idempotent — RegisterPluginCascade
	// overwrites on the same (entityType, pluginName) key — so it is
	// safe to call on both fresh registers and hot reloads. When a
	// reload strips cascades from a manifest, unregister so stale
	// closures stop dispatching.
	if len(manifest.Cascades) > 0 {
		m.registerPluginCascades(manifest.Name, manifest.Cascades)
	} else {
		deletion.UnregisterPluginCascades(manifest.Name)
	}
	if len(manifest.UIs) > 0 {
		if err := m.registerPluginUIs(ctx, manifest.Name, manifest.UIs); err != nil {
			slog.Warn("failed to register plugin UIs", "plugin", manifest.Name, "error", err)
		}
	}
}

// registerCustomFields ensures all custom fields declared by a plugin exist in gk_custom_field_def.
// Field names are auto-prefixed with the plugin name to prevent collisions.
func (m *Manager) registerCustomFields(ctx context.Context, pluginName string, specs []CustomFieldSpec) error {
	repo, err := customfields.NewRepository()
	if err != nil {
		return fmt.Errorf("get db: %w", err)
	}

	for _, spec := range specs {
		prefixedName := fieldPrefix(pluginName) + spec.Name

		existing, err := repo.GetDefByEntityAndName(spec.EntityType, prefixedName)
		if err != nil {
			return fmt.Errorf("check field %q: %w", prefixedName, err)
		}

		ownerName := pluginName
		section := spec.Section
		if section == "" {
			section = "custom"
		}

		if existing != nil {
			// Update label, config, etc. if owner matches.
			if existing.OwnerType == customfields.OwnerPlugin && existing.OwnerName != nil && *existing.OwnerName == pluginName {
				existing.Label = spec.Label
				existing.Section = section
				existing.FieldOrder = spec.Order
				existing.Required = spec.Required
				if spec.Description != "" {
					existing.Description = &spec.Description
				}
				if spec.Placeholder != "" {
					existing.Placeholder = &spec.Placeholder
				}
				if spec.Config != nil {
					cfg := json.RawMessage(spec.Config)
					existing.Config = &cfg
				}
				existing.ValidID = 1 // Re-enable if it was soft-deleted.
				if err := repo.UpdateDef(existing, 1); err != nil {
					return fmt.Errorf("update field %q: %w", prefixedName, err)
				}
			}
			continue
		}

		// Create new field.
		def := &customfields.FieldDef{
			Name:       prefixedName,
			Label:      spec.Label,
			EntityType: spec.EntityType,
			FieldType:  spec.FieldType,
			OwnerType:  customfields.OwnerPlugin,
			OwnerName:  &ownerName,
			Section:    section,
			FieldOrder: spec.Order,
			Required:   spec.Required,
			ValidID:    1,
		}
		if spec.Description != "" {
			def.Description = &spec.Description
		}
		if spec.Placeholder != "" {
			def.Placeholder = &spec.Placeholder
		}
		if spec.Config != nil {
			cfg := json.RawMessage(spec.Config)
			def.Config = &cfg
		}

		if err := customfields.ValidateFieldDef(def); err != nil {
			slog.Warn("invalid custom field spec, skipping", "plugin", pluginName, "field", spec.Name, "error", err)
			continue
		}

		if _, err := repo.CreateDef(def, 1); err != nil {
			return fmt.Errorf("create field %q: %w", prefixedName, err)
		}
		slog.Info("registered custom field", "plugin", pluginName, "field", prefixedName, "entity", spec.EntityType)
	}
	return nil
}

// registerPluginUIs ensures all UIs declared by a plugin exist in gk_plugin_ui.
func (m *Manager) registerPluginUIs(ctx context.Context, pluginName string, specs []UISpec) error {
	repo, err := pluginui.NewRepository()
	if err != nil {
		return fmt.Errorf("get db: %w", err)
	}

	for _, spec := range specs {
		fullID := pluginui.BuildFullID(pluginName, spec.ID)

		if !pluginui.IsValidUIType(spec.Type) {
			slog.Warn("invalid UI type, skipping", "plugin", pluginName, "ui", spec.ID, "type", spec.Type)
			continue
		}

		shell := spec.Shell
		if shell == "" {
			shell = pluginui.DefaultShell(spec.Type)
		}

		existing, err := repo.GetByFullID(fullID)
		if err != nil {
			return fmt.Errorf("check UI %q: %w", fullID, err)
		}

		// Build config JSON from the spec.
		cfgJSON, err := pluginui.UISpecToConfig(map[string]any{
			"routes":     spec.Routes,
			"nav":        spec.Nav,
			"branding":   spec.Branding,
			"auth":       spec.Auth,
			"pwa":        spec.PWA,
			"data_scope": spec.DataScope,
			"rate_limit": spec.RateLimit,
		})
		if err != nil {
			return fmt.Errorf("marshal UI config %q: %w", fullID, err)
		}

		if existing != nil {
			// Update name, config, shell — preserve admin overrides (enabled, custom_domain).
			existing.Name = spec.Name
			existing.Shell = shell
			existing.Config = cfgJSON
			if spec.Description != "" {
				existing.Description = &spec.Description
			}
			if spec.Icon != "" {
				existing.Icon = &spec.Icon
			}
			existing.ValidID = 1
			if err := repo.Update(existing, 1); err != nil {
				return fmt.Errorf("update UI %q: %w", fullID, err)
			}
			continue
		}

		// Create new UI entry.
		u := &pluginui.PluginUI{
			PluginName: pluginName,
			UIID:       spec.ID,
			FullID:     fullID,
			Name:       spec.Name,
			UIType:     spec.Type,
			Shell:      shell,
			Config:     cfgJSON,
			Enabled:    true,
			ValidID:    1,
		}
		if spec.Description != "" {
			u.Description = &spec.Description
		}
		if spec.Icon != "" {
			u.Icon = &spec.Icon
		}

		if _, err := repo.Create(u, 1); err != nil {
			return fmt.Errorf("create UI %q: %w", fullID, err)
		}
		slog.Info("registered plugin UI", "plugin", pluginName, "ui", fullID, "type", spec.Type)
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

	// Cascade closures hold a reference to *this* Manager and would
	// keep dispatching to the gone plugin unless cleared explicitly.
	deletion.UnregisterPluginCascades(name)

	delete(m.plugins, name)
	delete(m.sandboxes, name)
	// Note: policy is preserved across reloads so admin settings persist
	return nil
}

// registerPluginCascades builds per-spec closures that dispatch to the
// plugin's named handler and installs them in the deletion package's
// shared registry. The cascade arg is a minimal {"id": entityID} object
// — plugins use their local extractID helper to parse it.
func (m *Manager) registerPluginCascades(pluginName string, specs []CascadeSpec) {
	for _, spec := range specs {
		spec := spec // capture
		var soft, hard deletion.CascadeHandler
		if spec.OnSoftDelete != "" {
			handlerName := spec.OnSoftDelete
			soft = func(ctx context.Context, _ string, entityID int64) error {
				args, _ := json.Marshal(map[string]int64{"id": entityID})
				_, err := m.Call(ctx, pluginName, handlerName, args)
				return err
			}
		}
		if spec.OnHardDelete != "" {
			handlerName := spec.OnHardDelete
			hard = func(ctx context.Context, _ string, entityID int64) error {
				args, _ := json.Marshal(map[string]int64{"id": entityID})
				_, err := m.Call(ctx, pluginName, handlerName, args)
				return err
			}
		}
		deletion.RegisterPluginCascade(spec.EntityType, pluginName, soft, hard)
	}
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

// Lookup returns a registered plugin by name, enabled or not.
func (m *Manager) Lookup(name string) (Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rp, exists := m.plugins[name]
	if !exists {
		return nil, false
	}
	return rp.plugin, true
}

// IsRegistered reports whether a plugin is in the registry, enabled or not.
func (m *Manager) IsRegistered(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.plugins[name]
	return exists
}

// SkipsOrgInjection returns true if the named plugin has opted out of
// automatic _org_id parameter injection.
func (m *Manager) SkipsOrgInjection(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rp, exists := m.plugins[name]
	if !exists {
		return false
	}
	return rp.manifest.SkipOrgInjection
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
// args is the host-built call envelope: its _user_id (acting user), _org_id
// (active organisation) and _lang (UI language) go into the call's context,
// so the plugin's HostAPI callbacks made with that context run for the
// caller (see withCallEnvelope).
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

		// Notify listeners that a plugin was lazy-loaded (refreshes MCP tools, etc.)
		if exists && m.OnPluginLoaded != nil {
			m.OnPluginLoaded()
		}
	}

	if !exists {
		return nil, &PluginNotFoundError{PluginName: pluginName, Function: fn}
	}
	if !rp.enabled {
		return nil, &PluginDisabledError{PluginName: pluginName}
	}

	return rp.plugin.Call(withCallEnvelope(ctx, args), fn, args)
}

// CallFrom invokes a function on a plugin, with caller context for better errors.
// If lazy loading is enabled and the plugin isn't loaded yet, it will be loaded first.
// The args come from the calling plugin, so the acting user is not read from
// them: the target runs for the caller's acting user carried in ctx.
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
	if err := checkHostVersion(newManifest); err != nil {
		return err
	}

	// Initialize new plugin with existing policy and settings
	policy := m.getOrCreatePolicy(newManifest.Name, newManifest.Resources)
	sandbox := NewSandboxedHostAPI(m.host, newManifest.Name, *policy)

	// Re-register pre-Init side effects (custom fields) so the new
	// plugin's Init sees its own declared CFs — mirrors the Register
	// path. Without this, hot-reloading a plugin that had added a new
	// CustomFieldSpec would leave the def missing until a full restart.
	m.applyManifestSideEffectsPreInit(ctx, newManifest)

	if err := initWithTimeout(ctx, newPlugin, sandbox, *policy); err != nil {
		return fmt.Errorf("new plugin %q init failed: %w", newManifest.Name, err)
	}
	applyCallTimeout(newPlugin, *policy)

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
		health:   healthInitForRegister(),
	}
	m.sandboxes[oldName] = sandbox

	// Re-apply post-Init side effects (translations, error codes,
	// template overrides, cascade handlers, UIs) so every kind of
	// manifest change takes effect on hot reload — previously only
	// cascades were re-registered here.
	m.applyManifestSideEffectsPostInit(ctx, newManifest)

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

// ListTicketViews returns the ticket-view URL templates declared by enabled
// plugins' enabled UIs. Each URL contains the literal {ticket_id}
// placeholder. Nil when no plugin declares a ticket view.
func (m *Manager) ListTicketViews(ctx context.Context) ([]TicketViewInfo, error) {
	m.mu.RLock()
	names := make([]string, 0, len(m.plugins))
	for name, rp := range m.plugins {
		if rp.enabled {
			names = append(names, name)
		}
	}
	m.mu.RUnlock()
	if len(names) == 0 {
		return nil, nil
	}

	repo, err := pluginui.NewRepository()
	if err != nil {
		return nil, fmt.Errorf("get db: %w", err)
	}

	var out []TicketViewInfo
	for _, name := range names {
		uis, err := repo.List(name, "", true)
		if err != nil {
			return nil, fmt.Errorf("list ticket views for %q: %w", name, err)
		}
		m.mu.RLock()
		rp, exists := m.plugins[name]
		m.mu.RUnlock()
		if !exists {
			continue
		}
		for _, u := range uis {
			for _, spec := range rp.manifest.UIs {
				if spec.ID != u.UIID || spec.TicketView == nil {
					continue
				}
				label := spec.TicketView.Label
				if label == "" {
					label = spec.Name
				}
				out = append(out, TicketViewInfo{
					PluginName: name,
					UIID:       spec.ID,
					Label:      label,
					URL:        fmt.Sprintf("/ui/%s/%s?%s={ticket_id}", u.FullID, strings.TrimPrefix(spec.TicketView.Path, "/"), spec.TicketView.Param),
				})
			}
		}
	}
	return out, nil
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
	// m.plugins is a map, so without a sort the nav order changed between
	// restarts. Order is the author's intent; plugin name and item ID break
	// ties so equal Orders still render the same way every time.
	sort.Slice(items, func(a, b int) bool {
		x, y := items[a], items[b]
		if x.Order != y.Order {
			return x.Order < y.Order
		}
		if x.PluginName != y.PluginName {
			return x.PluginName < y.PluginName
		}
		return x.ID < y.ID
	})
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

	return m.setGlobalSysconfig(ctx, 0, key, string(jsonData))
}

// setGlobalSysconfig writes the global (user_id NULL) sysconfig_modified
// value for key. The table's only unique key is (sysconfig_default_id,
// user_id), which never conflicts on NULL user_id, so no upsert can target
// it: update the existing row and insert when there is none.
func (m *Manager) setGlobalSysconfig(ctx context.Context, defaultID any, key, val string) error {
	affected, err := m.host.DBExec(ctx, `
		UPDATE sysconfig_modified SET effective_value = ?, change_time = CURRENT_TIMESTAMP, change_by = 1
		WHERE name = ? AND user_id IS NULL`, val, key)
	if err != nil || affected > 0 {
		return err
	}
	_, err = m.host.DBExec(ctx, `
		INSERT INTO sysconfig_modified
		(sysconfig_default_id, name, user_id, effective_value, is_valid, user_modification_active,
		 is_dirty, reset_to_default, create_by, change_by, create_time, change_time)
		VALUES (?, ?, NULL, ?, 1, 0, 0, 0, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, defaultID, key, val)
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

// LandingPageFor returns the declared LandingPage for a specific
// enabled plugin, or empty string if the plugin isn't loaded or didn't
// declare one. Used by the captive-portal feature to know where to
// redirect a customer whose org is captive to that plugin.
func (m *Manager) LandingPageFor(name string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rp, ok := m.plugins[name]
	if !ok || !rp.enabled {
		return ""
	}
	return rp.manifest.LandingPage
}

// InstalledPluginNames lists the names of every enabled plugin that has
// completed registration. Used by the admin UI to populate the
// captive-plugin dropdown.
func (m *Manager) InstalledPluginNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]string, 0, len(m.plugins))
	for name, rp := range m.plugins {
		if rp.enabled {
			out = append(out, name)
		}
	}
	return out
}

// defaultShutdownTimeout is the per-plugin shutdown grace period used
// when the plugin's ResourcePolicy doesn't specify one. Chosen to be
// long enough for a well-behaved plugin to flush in-flight state (e.g.
// close DB connections, drain a WireGuard peer map) without letting a
// misbehaving plugin wedge ShutdownAll for too long.
const defaultShutdownTimeout = 10 * time.Second

// ShutdownAll shuts down all plugins in parallel, applying a per-plugin
// timeout derived from the plugin's ResourcePolicy.ShutdownTimeout. A
// plugin that doesn't honour its own deadline is force-killed by the
// underlying supervisor (GRPCPlugin.Shutdown falls through to Kill()).
//
// The outer ctx still matters: if the caller imposes a shorter
// deadline, every per-plugin timeout is additionally clamped by the
// outer context. This lets goatflow guarantee a hard ceiling on total
// shutdown time (e.g. 30s for the whole process) regardless of how
// many plugins are registered.
//
// Plugins are shut down concurrently so the worst-case total is the
// max per-plugin timeout, not the sum. With N plugins each at 10s the
// old serial path took 10N seconds; the parallel path takes ~10s.
func (m *Manager) ShutdownAll(ctx context.Context) error {
	m.mu.Lock()
	plugins := m.plugins
	m.plugins = make(map[string]*registeredPlugin)
	policies := make(map[string]*ResourcePolicy, len(m.policies))
	for k, v := range m.policies {
		policies[k] = v
	}
	m.mu.Unlock()

	if len(plugins) == 0 {
		return nil
	}

	var (
		wg     sync.WaitGroup
		errsMu sync.Mutex
		errs   []error
	)

	for name, rp := range plugins {
		timeout := defaultShutdownTimeout
		if pol, ok := policies[name]; ok && pol != nil {
			timeout = LimitDuration(pol.ShutdownTimeout, defaultShutdownTimeout)
		}

		wg.Add(1)
		go func(name string, p Plugin, timeout time.Duration) {
			defer wg.Done()
			pctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			if err := p.Shutdown(pctx); err != nil {
				errsMu.Lock()
				errs = append(errs, fmt.Errorf("plugin %q: %w", name, err))
				errsMu.Unlock()
			}
		}(name, rp.plugin, timeout)
	}

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}
