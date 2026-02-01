package plugin

import (
	"context"
	"fmt"
	"sync"
)

// Manager handles plugin lifecycle: loading, registration, and invocation.
type Manager struct {
	mu      sync.RWMutex
	plugins map[string]*registeredPlugin
	host    HostAPI
}

type registeredPlugin struct {
	plugin   Plugin
	manifest GKRegistration
	enabled  bool
}

// NewManager creates a plugin manager with the given host API.
func NewManager(host HostAPI) *Manager {
	return &Manager{
		plugins: make(map[string]*registeredPlugin),
		host:    host,
	}
}

// Register loads and initializes a plugin.
func (m *Manager) Register(ctx context.Context, p Plugin) error {
	manifest := p.GKRegister()

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.plugins[manifest.Name]; exists {
		return fmt.Errorf("plugin %q already registered", manifest.Name)
	}

	// Initialize the plugin with host API access
	if err := p.Init(ctx, m.host); err != nil {
		return fmt.Errorf("plugin %q init failed: %w", manifest.Name, err)
	}

	m.plugins[manifest.Name] = &registeredPlugin{
		plugin:   p,
		manifest: manifest,
		enabled:  true,
	}

	return nil
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

// Call invokes a function on a specific plugin.
func (m *Manager) Call(ctx context.Context, pluginName, fn string, args []byte) ([]byte, error) {
	m.mu.RLock()
	rp, exists := m.plugins[pluginName]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("plugin %q not found", pluginName)
	}
	if !rp.enabled {
		return nil, fmt.Errorf("plugin %q is disabled", pluginName)
	}

	return rp.plugin.Call(ctx, fn, args)
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

// Enable enables a previously disabled plugin.
func (m *Manager) Enable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rp, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %q not found", name)
	}
	rp.enabled = true
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
