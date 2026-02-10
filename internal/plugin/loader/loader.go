package loader

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"

	"github.com/goatkit/goatflow/internal/plugin"
	"github.com/goatkit/goatflow/internal/plugin/grpc"
	"github.com/goatkit/goatflow/internal/plugin/signing"
	"github.com/goatkit/goatflow/internal/plugin/wasm"
	pkgplugin "github.com/goatkit/goatflow/pkg/plugin"
)

// PluginManifest is an alias for the shared manifest type.
type PluginManifest = pkgplugin.PluginManifest

// DiscoveredPlugin holds info about a plugin found but not yet loaded.
type DiscoveredPlugin struct {
	Name     string // Derived from filename or manifest
	Path     string // Full path to .wasm file or plugin directory
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

	// gRPC plugin manifests (name -> manifest)
	manifests map[string]*PluginManifest

	lazy bool // If true, don't load on discover

	// Signature verification
	signatureVerification bool
	trustedKeys          []ed25519.PublicKey

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

// WithSignatureVerification enables plugin binary signature verification.
// Only plugins signed with one of the trusted keys will be allowed to load.
func WithSignatureVerification(trustedKeys []ed25519.PublicKey) LoaderOption {
	return func(l *Loader) {
		l.signatureVerification = true
		l.trustedKeys = trustedKeys
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
		manifests:  make(map[string]*PluginManifest),
		debounce:   make(map[string]*time.Timer),
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// DiscoverAll scans the plugin directory and records available plugins without loading them.
// Discovers both .wasm files and gRPC plugins (directories with plugin.yaml).
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

	// Walk for .wasm files
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
			l.logger.Debug("discovered WASM plugin", "name", name, "path", path)
			plugin.GetLogBuffer().Log(name, "info", fmt.Sprintf("Plugin discovered: %s (type: wasm)", name), nil)
		}
		return nil
	})
	if err != nil {
		return discovered, err
	}

	// Scan top-level directories for plugin.yaml (gRPC plugins)
	grpcCount, grpcErr := l.discoverGRPCPlugins()
	discovered += grpcCount

	if grpcErr != nil {
		return discovered, grpcErr
	}

	return discovered, nil
}

// discoverGRPCPlugins scans for directories containing plugin.yaml.
func (l *Loader) discoverGRPCPlugins() (int, error) {
	entries, err := os.ReadDir(l.pluginDir)
	if err != nil {
		return 0, fmt.Errorf("read plugin dir: %w", err)
	}

	discovered := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		manifestPath := filepath.Join(l.pluginDir, entry.Name(), "plugin.yaml")
		manifest, err := loadManifest(manifestPath)
		if err != nil {
			continue // No valid plugin.yaml, skip
		}

		if manifest.Runtime != "grpc" {
			continue
		}

		if manifest.Name == "" {
			manifest.Name = entry.Name()
		}

		l.mu.Lock()
		l.discovered[manifest.Name] = &DiscoveredPlugin{
			Name: manifest.Name,
			Path: filepath.Join(l.pluginDir, entry.Name()),
			Type: "grpc",
		}
		l.manifests[manifest.Name] = manifest
		l.mu.Unlock()
		discovered++
		l.logger.Debug("discovered gRPC plugin", "name", manifest.Name, "path", filepath.Join(l.pluginDir, entry.Name()))
		plugin.GetLogBuffer().Log(manifest.Name, "info", fmt.Sprintf("Plugin discovered: %s (type: grpc)", manifest.Name), nil)
	}

	return discovered, nil
}

// loadManifest reads and parses a plugin.yaml file.
func loadManifest(path string) (*PluginManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m PluginManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse plugin.yaml: %w", err)
	}
	return &m, nil
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

	l.logger.Info("lazy loading plugin", "name", name, "type", d.Type)

	var err error
	switch d.Type {
	case "wasm":
		err = l.loadWASMPlugin(ctx, d.Path)
	case "grpc":
		manifest := l.manifests[name]
		if manifest == nil {
			return fmt.Errorf("plugin %q missing manifest", name)
		}
		err = l.loadGRPCPlugin(ctx, d.Path, manifest)
	default:
		return fmt.Errorf("plugin %q has unknown type %q", name, d.Type)
	}

	if err != nil {
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

	// Walk the plugin directory for WASM files
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

	// Load gRPC plugins from directories with plugin.yaml
	grpcLoaded, grpcErrors := l.loadGRPCPlugins(ctx)
	loaded += grpcLoaded
	errors = append(errors, grpcErrors...)

	return loaded, errors
}

// loadGRPCPlugins discovers and loads all gRPC plugins.
func (l *Loader) loadGRPCPlugins(ctx context.Context) (int, []error) {
	entries, err := os.ReadDir(l.pluginDir)
	if err != nil {
		return 0, []error{fmt.Errorf("read plugin dir for gRPC: %w", err)}
	}

	loaded := 0
	var errors []error

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginDir := filepath.Join(l.pluginDir, entry.Name())
		manifestPath := filepath.Join(pluginDir, "plugin.yaml")

		manifest, err := loadManifest(manifestPath)
		if err != nil {
			continue // No valid plugin.yaml, skip
		}

		if manifest.Runtime != "grpc" {
			continue
		}

		if manifest.Name == "" {
			manifest.Name = entry.Name()
		}

		l.mu.Lock()
		l.manifests[manifest.Name] = manifest
		l.mu.Unlock()

		if err := l.loadGRPCPlugin(ctx, pluginDir, manifest); err != nil {
			errors = append(errors, fmt.Errorf("load gRPC plugin %s: %w", manifest.Name, err))
		} else {
			l.mu.Lock()
			l.discovered[manifest.Name] = &DiscoveredPlugin{
				Name:     manifest.Name,
				Path:     pluginDir,
				Type:     "grpc",
				Loaded:   true,
				LoadedAt: time.Now(),
			}
			l.mu.Unlock()
			loaded++
		}
	}

	return loaded, errors
}

// loadGRPCPlugin loads a single gRPC plugin from its directory.
func (l *Loader) loadGRPCPlugin(ctx context.Context, pluginDir string, manifest *PluginManifest) error {
	binaryPath := manifest.Binary
	if binaryPath == "" {
		binaryPath = manifest.Name
	}

	// Resolve relative binary path against plugin directory
	if !filepath.IsAbs(binaryPath) {
		binaryPath = filepath.Join(pluginDir, binaryPath)
	}

	// Check binary exists and is executable
	info, err := os.Stat(binaryPath)
	if err != nil {
		return fmt.Errorf("binary not found at %s: %w", binaryPath, err)
	}
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("binary %s is not executable", binaryPath)
	}

	// Verify binary signature if verification is enabled
	if l.signatureVerification || signing.IsSignatureRequired() {
		sigPath := signing.DefaultSignaturePath(binaryPath)
		if err := signing.VerifyBinary(binaryPath, sigPath, l.trustedKeys); err != nil {
			return fmt.Errorf("signature verification failed for %s: %w", manifest.Name, err)
		}
		l.logger.Info("plugin signature verified", "name", manifest.Name, "binary", binaryPath)
	} else {
		// Log warning about unsigned plugin
		l.logger.Warn("loading unsigned plugin (signature verification disabled)", 
			"name", manifest.Name, 
			"binary", binaryPath,
			"recommendation", "enable signature verification in production")
	}

	l.logger.Info("loading gRPC plugin", "name", manifest.Name, "binary", binaryPath)

	// Get or create the resource policy for this plugin
	policy, _ := l.manager.GetPolicy(manifest.Name)
	if policy.PluginName == "" {
		// Create default policy if none exists
		policy = plugin.DefaultResourcePolicy(manifest.Name)
	}

	gp, err := grpc.LoadGRPCPlugin(binaryPath, manifest.Name, l.manager.Host(), policy)
	if err != nil {
		return fmt.Errorf("load gRPC plugin: %w", err)
	}

	if err := l.manager.Register(ctx, gp); err != nil {
		gp.Shutdown(ctx)
		return fmt.Errorf("register gRPC plugin: %w", err)
	}

	l.logger.Info("loaded gRPC plugin",
		"name", manifest.Name,
		"version", manifest.Version,
	)

	plugin.GetLogBuffer().Log(manifest.Name, "info", fmt.Sprintf("Plugin loaded: %s", manifest.Name), nil)
	return nil
}

// loadWASMPlugin loads a single WASM plugin file.
func (l *Loader) loadWASMPlugin(ctx context.Context, path string) error {
	l.logger.Info("loading WASM plugin", "path", path)

	// Verify WASM signature if verification is enabled
	if l.signatureVerification || signing.IsSignatureRequired() {
		sigPath := signing.DefaultSignaturePath(path)
		if err := signing.VerifyBinary(path, sigPath, l.trustedKeys); err != nil {
			return fmt.Errorf("signature verification failed for WASM plugin %s: %w", filepath.Base(path), err)
		}
		l.logger.Info("WASM plugin signature verified", "path", path)
	} else {
		// Log warning about unsigned plugin
		l.logger.Warn("loading unsigned WASM plugin (signature verification disabled)", 
			"path", path,
			"recommendation", "enable signature verification in production")
	}

	// Load the WASM module
	wp, err := wasm.LoadFromFile(ctx, path)
	if err != nil {
		return fmt.Errorf("load wasm: %w", err)
	}

	// Get the manifest to log what we loaded
	manifest := wp.GKRegister()
	l.logger.Info("loaded plugin",
		"name", manifest.Name,
		"version", manifest.Version,
		"routes", len(manifest.Routes),
		"widgets", len(manifest.Widgets),
		"jobs", len(manifest.Jobs),
	)

	// Register with the manager
	if err := l.manager.Register(ctx, wp); err != nil {
		// Shutdown the plugin if registration fails
		wp.Shutdown(ctx)
		return fmt.Errorf("register: %w", err)
	}

	plugin.GetLogBuffer().Log(manifest.Name, "info", fmt.Sprintf("Plugin loaded: %s", manifest.Name), nil)
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
	err := l.manager.Unregister(ctx, name)
	if err == nil {
		plugin.GetLogBuffer().Log(name, "info", fmt.Sprintf("Plugin unloaded: %s", name), nil)
	}
	return err
}

// Reload unloads and reloads a plugin by name using atomic replacement to avoid race conditions.
func (l *Loader) Reload(ctx context.Context, name string) error {
	l.mu.RLock()
	d, exists := l.discovered[name]
	manifest := l.manifests[name]
	l.mu.RUnlock()

	if !exists {
		// Fall back to legacy WASM reload for backwards compatibility
		return l.reloadWASM(ctx, name)
	}

	// Load the new plugin version first
	var newPlugin plugin.Plugin
	var err error

	switch d.Type {
	case "wasm":
		newPlugin, err = wasm.LoadFromFile(ctx, d.Path)
		if err != nil {
			return fmt.Errorf("failed to load new WASM plugin: %w", err)
		}
	case "grpc":
		if manifest == nil {
			return fmt.Errorf("plugin %q missing manifest", name)
		}
		
		// Get or create the resource policy for this plugin
		policy, _ := l.manager.GetPolicy(manifest.Name)
		if policy.PluginName == "" {
			// Create default policy if none exists
			policy = plugin.DefaultResourcePolicy(manifest.Name)
		}

		binaryPath := manifest.Binary
		if binaryPath == "" {
			binaryPath = manifest.Name
		}
		if !filepath.IsAbs(binaryPath) {
			binaryPath = filepath.Join(d.Path, binaryPath)
		}

		newPlugin, err = grpc.LoadGRPCPlugin(binaryPath, manifest.Name, l.manager.Host(), policy)
		if err != nil {
			return fmt.Errorf("failed to load new gRPC plugin: %w", err)
		}
	default:
		return fmt.Errorf("unknown plugin type %q", d.Type)
	}

	// Check if plugin is currently registered
	if _, found := l.manager.Get(name); found {
		// Use atomic replacement to avoid race conditions
		if err := l.manager.ReplacePlugin(ctx, name, newPlugin); err != nil {
			// Shutdown the new plugin since replacement failed
			newPlugin.Shutdown(ctx)
			return fmt.Errorf("atomic replacement failed: %w", err)
		}
	} else {
		// Plugin not currently registered, just register the new one
		if err := l.manager.Register(ctx, newPlugin); err != nil {
			newPlugin.Shutdown(ctx)
			return fmt.Errorf("register new plugin: %w", err)
		}
	}

	return nil
}

// reloadWASM is the legacy reload path for WASM plugins not yet discovered.
func (l *Loader) reloadWASM(ctx context.Context, name string) error {
	path := filepath.Join(l.pluginDir, name+".wasm")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("plugin file not found: %s", path)
	}

	if _, exists := l.manager.Get(name); exists {
		if err := l.manager.Unregister(ctx, name); err != nil {
			return fmt.Errorf("unload: %w", err)
		}
	}

	return l.loadWASMPlugin(ctx, path)
}

// WatchDir sets up a file watcher for hot reload.
// Watches for .wasm file changes and gRPC binary changes (via plugin.yaml).
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

	// Also watch subdirectories (for gRPC plugin binaries and WASM in folders)
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
	if l.watcher == nil {
		return
	}
	events := l.watcher.Events
	errors := l.watcher.Errors
	for {
		select {
		case <-l.watchCtx.Done():
			return

		case event, ok := <-events:
			if !ok {
				return
			}
			l.handleFSEvent(event)

		case err, ok := <-errors:
			if !ok {
				return
			}
			l.logger.Error("watcher error", "error", err)
		}
	}
}

// handleFSEvent processes a single file system event with debouncing.
func (l *Loader) handleFSEvent(event fsnotify.Event) {
	path := event.Name
	baseName := filepath.Base(path)

	// Check if this is a WASM file
	isWASM := strings.HasSuffix(strings.ToLower(baseName), ".wasm")

	// Check if this is a gRPC plugin binary change
	isGRPC := l.isGRPCBinaryPath(path)

	// Check if a new plugin.yaml was added (new gRPC plugin)
	isManifest := baseName == "plugin.yaml"

	if !isWASM && !isGRPC && !isManifest {
		return
	}

	// Debounce rapid changes (e.g., during build)
	l.watchMu.Lock()
	if timer, exists := l.debounce[path]; exists {
		timer.Stop()
	}
	l.debounce[path] = time.AfterFunc(500*time.Millisecond, func() {
		if isManifest {
			l.processManifestChange(event)
		} else if isGRPC {
			l.processGRPCBinaryChange(event)
		} else {
			l.processFileChange(event)
		}
	})
	l.watchMu.Unlock()
}

// isGRPCBinaryPath checks if the changed file is a known gRPC plugin binary.
func (l *Loader) isGRPCBinaryPath(path string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, manifest := range l.manifests {
		binaryName := manifest.Binary
		if binaryName == "" {
			binaryName = manifest.Name
		}

		// Check if this path matches any known gRPC binary
		dir := filepath.Dir(path)
		pluginDir := filepath.Join(l.pluginDir, manifest.Name)

		// Match by directory + binary name
		if dir == pluginDir && filepath.Base(path) == filepath.Base(binaryName) {
			return true
		}

		// Also match by full resolved path
		resolvedBinary := binaryName
		if !filepath.IsAbs(resolvedBinary) {
			resolvedBinary = filepath.Join(pluginDir, resolvedBinary)
		}
		if path == resolvedBinary {
			return true
		}
	}

	return false
}

// pluginNameForBinaryPath returns the plugin name for a gRPC binary path.
func (l *Loader) pluginNameForBinaryPath(path string) string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for name, manifest := range l.manifests {
		binaryName := manifest.Binary
		if binaryName == "" {
			binaryName = manifest.Name
		}

		dir := filepath.Dir(path)
		pluginDir := filepath.Join(l.pluginDir, name)

		if dir == pluginDir && filepath.Base(path) == filepath.Base(binaryName) {
			return name
		}

		resolvedBinary := binaryName
		if !filepath.IsAbs(resolvedBinary) {
			resolvedBinary = filepath.Join(pluginDir, resolvedBinary)
		}
		if path == resolvedBinary {
			return name
		}
	}

	return ""
}

// processManifestChange handles plugin.yaml create/modify events.
func (l *Loader) processManifestChange(event fsnotify.Event) {
	path := event.Name
	pluginDir := filepath.Dir(path)

	switch {
	case event.Op&(fsnotify.Create|fsnotify.Write) != 0:
		manifest, err := loadManifest(path)
		if err != nil {
			l.logger.Error("failed to parse new plugin.yaml", "path", path, "error", err)
			return
		}

		if manifest.Runtime != "grpc" {
			return
		}

		if manifest.Name == "" {
			manifest.Name = filepath.Base(pluginDir)
		}

		l.logger.Info("🔌 new gRPC plugin detected", "name", manifest.Name)

		l.mu.Lock()
		l.manifests[manifest.Name] = manifest
		l.discovered[manifest.Name] = &DiscoveredPlugin{
			Name: manifest.Name,
			Path: pluginDir,
			Type: "grpc",
		}
		l.mu.Unlock()

		// Try to load it
		if err := l.loadGRPCPlugin(l.watchCtx, pluginDir, manifest); err != nil {
			l.logger.Error("failed to load new gRPC plugin", "name", manifest.Name, "error", err)
		} else {
			l.mu.Lock()
			d := l.discovered[manifest.Name]
			d.Loaded = true
			d.LoadedAt = time.Now()
			l.mu.Unlock()
			l.logger.Info("✅ gRPC plugin loaded", "name", manifest.Name)
		}

		// Watch the plugin subdirectory for binary changes
		if l.watcher != nil {
			l.watcher.Add(pluginDir)
		}

	case event.Op&fsnotify.Remove != 0:
		// plugin.yaml removed — unload the plugin
		name := filepath.Base(pluginDir)

		l.mu.RLock()
		// Try to find by directory name matching a known plugin
		for n, d := range l.discovered {
			if d.Path == pluginDir {
				name = n
				break
			}
		}
		l.mu.RUnlock()

		l.logger.Info("🗑️ gRPC plugin manifest removed", "name", name)
		if err := l.manager.Unregister(l.watchCtx, name); err != nil {
			l.logger.Warn("failed to unregister removed gRPC plugin", "name", name, "error", err)
		}

		l.mu.Lock()
		delete(l.discovered, name)
		delete(l.manifests, name)
		l.mu.Unlock()
	}

	// Clean up debounce timer
	l.watchMu.Lock()
	delete(l.debounce, event.Name)
	l.watchMu.Unlock()
}

// processGRPCBinaryChange handles gRPC plugin binary create/modify events.
func (l *Loader) processGRPCBinaryChange(event fsnotify.Event) {
	// Check if watch context is cancelled before processing
	select {
	case <-l.watchCtx.Done():
		return
	default:
	}

	name := l.pluginNameForBinaryPath(event.Name)
	if name == "" {
		return
	}

	switch {
	case event.Op&(fsnotify.Create|fsnotify.Write) != 0:
		l.logger.Info("🔄 gRPC plugin binary changed, reloading", "name", name)
		if err := l.Reload(l.watchCtx, name); err != nil {
			l.logger.Error("failed to reload gRPC plugin", "name", name, "error", err)
		} else {
			l.mu.Lock()
			if d, ok := l.discovered[name]; ok {
				d.Loaded = true
				d.LoadedAt = time.Now()
			}
			l.mu.Unlock()
			l.logger.Info("✅ gRPC plugin reloaded", "name", name)
		}

	case event.Op&fsnotify.Remove != 0:
		l.logger.Info("🗑️ gRPC plugin binary removed", "name", name)
		if err := l.manager.Unregister(l.watchCtx, name); err != nil {
			l.logger.Warn("failed to unregister gRPC plugin", "name", name, "error", err)
		}

		l.mu.Lock()
		if d, ok := l.discovered[name]; ok {
			d.Loaded = false
		}
		l.mu.Unlock()
	}

	// Clean up debounce timer
	l.watchMu.Lock()
	delete(l.debounce, event.Name)
	l.watchMu.Unlock()
}

// processFileChange handles WASM file changes (original behaviour).
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
