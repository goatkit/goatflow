package loader

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/gotrs-io/gotrs-ce/internal/plugin"
	"github.com/gotrs-io/gotrs-ce/internal/plugin/wasm"
)

// DiscoveredPlugin holds info about a plugin found but not yet loaded.
type DiscoveredPlugin struct {
	Name     string // Derived from filename
	Path     string // Full path to .wasm file
	Type     string // "wasm" or "grpc"
	Loaded   bool   // Whether it's been loaded
	LoadedAt time.Time
}

// Loader handles discovery and loading of plugins from the filesystem.
type Loader struct {
	pluginDir string
	manager   *plugin.Manager
	logger    *slog.Logger

	// Lazy loading
	mu         sync.RWMutex
	discovered map[string]*DiscoveredPlugin // name -> discovery info
	lazy       bool                         // If true, don't load on discover

	// Hot reload
	watcher     *fsnotify.Watcher
	watchCtx    context.Context
	watchCancel context.CancelFunc
	watchMu     sync.Mutex
	debounce    map[string]*time.Timer // Debounce rapid file changes
}

// LoaderOption configures a Loader.
type LoaderOption func(*Loader)

// WithLazyLoading enables lazy loading - plugins are discovered but not loaded until first use.
func WithLazyLoading() LoaderOption {
	return func(l *Loader) {
		l.lazy = true
	}
}

// NewLoader creates a plugin loader for the given directory.
func NewLoader(pluginDir string, manager *plugin.Manager, logger *slog.Logger, opts ...LoaderOption) *Loader {
	if logger == nil {
		logger = slog.Default()
	}
	l := &Loader{
		pluginDir:  pluginDir,
		manager:    manager,
		logger:     logger,
		discovered: make(map[string]*DiscoveredPlugin),
		debounce:   make(map[string]*time.Timer),
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// DiscoverAll scans the plugin directory and records available plugins without loading them.
// Use with lazy loading - plugins will be loaded on first use.
func (l *Loader) DiscoverAll() (int, error) {
	// Ensure plugin directory exists
	if _, err := os.Stat(l.pluginDir); os.IsNotExist(err) {
		l.logger.Info("plugin directory does not exist, creating", "path", l.pluginDir)
		if err := os.MkdirAll(l.pluginDir, 0755); err != nil {
			return 0, fmt.Errorf("create plugin dir: %w", err)
		}
		return 0, nil
	}

	discovered := 0
	err := filepath.WalkDir(l.pluginDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".wasm" {
			name := strings.TrimSuffix(filepath.Base(path), ".wasm")
			l.mu.Lock()
			l.discovered[name] = &DiscoveredPlugin{
				Name: name,
				Path: path,
				Type: "wasm",
			}
			l.mu.Unlock()
			discovered++
			l.logger.Debug("discovered plugin", "name", name, "path", path)
		}
		return nil
	})

	return discovered, err
}

// Discovered returns the names of discovered (but possibly not loaded) plugins.
// Implements plugin.LazyLoader interface.
func (l *Loader) Discovered() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	result := make([]string, 0, len(l.discovered))
	for name := range l.discovered {
		result = append(result, name)
	}
	return result
}

// DiscoveredPlugins returns detailed info about discovered plugins.
func (l *Loader) DiscoveredPlugins() []*DiscoveredPlugin {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	result := make([]*DiscoveredPlugin, 0, len(l.discovered))
	for _, d := range l.discovered {
		result = append(result, d)
	}
	return result
}

// EnsureLoaded loads a plugin by name if not already loaded (for lazy loading).
func (l *Loader) EnsureLoaded(ctx context.Context, name string) error {
	l.mu.RLock()
	d, exists := l.discovered[name]
	l.mu.RUnlock()

	if !exists {
		return fmt.Errorf("plugin %q not discovered", name)
	}
	if d.Loaded {
		return nil // Already loaded
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Double-check after acquiring write lock
	if d.Loaded {
		return nil
	}

	l.logger.Info("lazy loading plugin", "name", name)
	if err := l.loadWASMPlugin(ctx, d.Path); err != nil {
		return err
	}

	d.Loaded = true
	d.LoadedAt = time.Now()
	return nil
}

// LoadAll discovers and loads all plugins from the plugin directory.
// If lazy loading is enabled, only discovers plugins without loading them.
// Returns the number of successfully loaded/discovered plugins and any errors encountered.
func (l *Loader) LoadAll(ctx context.Context) (int, []error) {
	// If lazy loading, just discover
	if l.lazy {
		count, err := l.DiscoverAll()
		if err != nil {
			return count, []error{err}
		}
		l.logger.Info("lazy loading enabled", "discovered", count)
		return count, nil
	}

	var errors []error
	loaded := 0

	// Ensure plugin directory exists
	if _, err := os.Stat(l.pluginDir); os.IsNotExist(err) {
		l.logger.Info("plugin directory does not exist, creating", "path", l.pluginDir)
		if err := os.MkdirAll(l.pluginDir, 0755); err != nil {
			return 0, []error{fmt.Errorf("create plugin dir: %w", err)}
		}
		return 0, nil
	}

	// Walk the plugin directory
	err := filepath.WalkDir(l.pluginDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories (but descend into them)
		if d.IsDir() {
			return nil
		}

		// Load based on file extension
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".wasm":
			name := strings.TrimSuffix(filepath.Base(path), ".wasm")
			// Track as discovered
			l.mu.Lock()
			l.discovered[name] = &DiscoveredPlugin{
				Name:     name,
				Path:     path,
				Type:     "wasm",
				Loaded:   true,
				LoadedAt: time.Now(),
			}
			l.mu.Unlock()

			if err := l.loadWASMPlugin(ctx, path); err != nil {
				errors = append(errors, fmt.Errorf("load %s: %w", filepath.Base(path), err))
			} else {
				loaded++
			}
		case ".zip":
			// TODO: Extract and load plugin package
			l.logger.Debug("skipping zip package (not yet implemented)", "path", path)
		}

		return nil
	})

	if err != nil {
		errors = append(errors, fmt.Errorf("walk plugin dir: %w", err))
	}

	return loaded, errors
}

// loadWASMPlugin loads a single WASM plugin file.
func (l *Loader) loadWASMPlugin(ctx context.Context, path string) error {
	l.logger.Info("loading WASM plugin", "path", path)

	// Load the WASM module
	plugin, err := wasm.LoadFromFile(ctx, path)
	if err != nil {
		return fmt.Errorf("load wasm: %w", err)
	}

	// Get the manifest to log what we loaded
	manifest := plugin.GKRegister()
	l.logger.Info("loaded plugin",
		"name", manifest.Name,
		"version", manifest.Version,
		"routes", len(manifest.Routes),
		"widgets", len(manifest.Widgets),
		"jobs", len(manifest.Jobs),
	)

	// Register with the manager
	if err := l.manager.Register(ctx, plugin); err != nil {
		// Shutdown the plugin if registration fails
		plugin.Shutdown(ctx)
		return fmt.Errorf("register: %w", err)
	}

	return nil
}

// LoadWASM loads a single WASM plugin by name (without .wasm extension).
func (l *Loader) LoadWASM(ctx context.Context, name string) error {
	path := filepath.Join(l.pluginDir, name+".wasm")
	return l.loadWASMPlugin(ctx, path)
}

// LoadWASMFromPath loads a WASM plugin from an arbitrary path.
func (l *Loader) LoadWASMFromPath(ctx context.Context, path string) error {
	return l.loadWASMPlugin(ctx, path)
}

// Unload unloads a plugin by name.
func (l *Loader) Unload(ctx context.Context, name string) error {
	return l.manager.Unregister(ctx, name)
}

// Reload unloads and reloads a plugin by name.
func (l *Loader) Reload(ctx context.Context, name string) error {
	// Find the plugin file
	path := filepath.Join(l.pluginDir, name+".wasm")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("plugin file not found: %s", path)
	}

	// Unload if currently loaded
	if _, exists := l.manager.Get(name); exists {
		if err := l.manager.Unregister(ctx, name); err != nil {
			return fmt.Errorf("unload: %w", err)
		}
	}

	// Load fresh
	return l.loadWASMPlugin(ctx, path)
}

// WatchDir sets up a file watcher for hot reload.
// When WASM files are created, modified, or removed, the corresponding plugin is reloaded.
func (l *Loader) WatchDir(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}

	l.watchMu.Lock()
	l.watcher = watcher
	l.watchCtx, l.watchCancel = context.WithCancel(ctx)
	l.watchMu.Unlock()

	// Watch the main plugin directory
	if err := watcher.Add(l.pluginDir); err != nil {
		watcher.Close()
		return fmt.Errorf("watch plugin dir: %w", err)
	}

	// Also watch subdirectories (for plugins in folders)
	filepath.WalkDir(l.pluginDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		watcher.Add(path)
		return nil
	})

	l.logger.Info("🔄 hot reload enabled", "path", l.pluginDir)

	go l.watchLoop()
	return nil
}

// StopWatch stops the file watcher.
func (l *Loader) StopWatch() {
	l.watchMu.Lock()
	defer l.watchMu.Unlock()

	if l.watchCancel != nil {
		l.watchCancel()
	}
	if l.watcher != nil {
		l.watcher.Close()
		l.watcher = nil
	}
}

// watchLoop processes file system events.
func (l *Loader) watchLoop() {
	for {
		select {
		case <-l.watchCtx.Done():
			return

		case event, ok := <-l.watcher.Events:
			if !ok {
				return
			}
			l.handleFSEvent(event)

		case err, ok := <-l.watcher.Errors:
			if !ok {
				return
			}
			l.logger.Error("watcher error", "error", err)
		}
	}
}

// handleFSEvent processes a single file system event with debouncing.
func (l *Loader) handleFSEvent(event fsnotify.Event) {
	// Only care about WASM files
	if !strings.HasSuffix(strings.ToLower(event.Name), ".wasm") {
		return
	}

	// Debounce rapid changes (e.g., during build)
	l.watchMu.Lock()
	if timer, exists := l.debounce[event.Name]; exists {
		timer.Stop()
	}
	l.debounce[event.Name] = time.AfterFunc(500*time.Millisecond, func() {
		l.processFileChange(event)
	})
	l.watchMu.Unlock()
}

// processFileChange handles the actual plugin reload after debounce.
func (l *Loader) processFileChange(event fsnotify.Event) {
	path := event.Name
	name := strings.TrimSuffix(filepath.Base(path), ".wasm")

	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		l.logger.Info("🔌 new plugin detected", "name", name)
		if err := l.loadWASMPlugin(l.watchCtx, path); err != nil {
			l.logger.Error("failed to load new plugin", "name", name, "error", err)
		} else {
			l.logger.Info("✅ plugin loaded", "name", name)
		}

	case event.Op&fsnotify.Write == fsnotify.Write:
		l.logger.Info("🔄 plugin modified, reloading", "name", name)
		if err := l.Reload(l.watchCtx, name); err != nil {
			l.logger.Error("failed to reload plugin", "name", name, "error", err)
		} else {
			l.logger.Info("✅ plugin reloaded", "name", name)
		}

	case event.Op&fsnotify.Remove == fsnotify.Remove:
		l.logger.Info("🗑️ plugin removed", "name", name)
		if err := l.manager.Unregister(l.watchCtx, name); err != nil {
			l.logger.Warn("failed to unregister removed plugin", "name", name, "error", err)
		}

	case event.Op&fsnotify.Rename == fsnotify.Rename:
		// Treat rename as remove (the new name will trigger a Create)
		l.logger.Info("🔄 plugin renamed/moved", "name", name)
		l.manager.Unregister(l.watchCtx, name)
	}

	// Clean up debounce timer
	l.watchMu.Lock()
	delete(l.debounce, path)
	l.watchMu.Unlock()
}
