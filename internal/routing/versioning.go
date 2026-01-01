package routing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// RouteVersion represents a version of route configuration.
type RouteVersion struct {
	Version    string                  `json:"version"`
	Timestamp  time.Time               `json:"timestamp"`
	Hash       string                  `json:"hash"`
	Author     string                  `json:"author"`
	Message    string                  `json:"message"`
	Routes     map[string]*RouteConfig `json:"routes"`
	Stats      *VersionStats           `json:"stats"`
	ParentHash string                  `json:"parent_hash,omitempty"`
}

// VersionStats contains statistics about a route version.
type VersionStats struct {
	TotalRoutes     int            `json:"total_routes"`
	TotalEndpoints  int            `json:"total_endpoints"`
	MethodBreakdown map[string]int `json:"method_breakdown"`
	NamespaceCount  map[string]int `json:"namespace_count"`
	EnabledRoutes   int            `json:"enabled_routes"`
	DisabledRoutes  int            `json:"disabled_routes"`
}

// VersionDiff represents changes between versions.
type VersionDiff struct {
	FromVersion string              `json:"from_version"`
	ToVersion   string              `json:"to_version"`
	Added       []string            `json:"added"`
	Modified    []string            `json:"modified"`
	Deleted     []string            `json:"deleted"`
	Changes     map[string][]string `json:"changes"`
}

// RouteVersionManager manages route configuration versions.
type RouteVersionManager struct {
	mu          sync.RWMutex
	storageDir  string
	versions    map[string]*RouteVersion
	current     *RouteVersion
	maxVersions int
	autoCommit  bool
}

// NewRouteVersionManager creates a new version manager.
func NewRouteVersionManager(storageDir string) *RouteVersionManager {
	vm := &RouteVersionManager{
		storageDir:  storageDir,
		versions:    make(map[string]*RouteVersion),
		maxVersions: 50, // Keep last 50 versions by default
		autoCommit:  true,
	}

	// Ensure storage directory exists
	versionsDir := filepath.Join(storageDir, ".versions")
	os.MkdirAll(versionsDir, 0755)

	// Load existing versions
	vm.loadVersions()

	return vm
}

// CreateVersion creates a new version from current routes.
func (vm *RouteVersionManager) CreateVersion(routes map[string]*RouteConfig, message string) (*RouteVersion, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Calculate hash of routes
	hash := vm.calculateHash(routes)

	// Check if this exact configuration already exists
	if existing, exists := vm.versions[hash]; exists {
		return existing, nil
	}

	// Generate version number
	version := vm.generateVersionNumber()

	// Calculate statistics
	stats := vm.calculateStats(routes)

	// Get author from environment or use default
	author := os.Getenv("USER")
	if author == "" {
		author = "system"
	}

	// Create version
	v := &RouteVersion{
		Version:   version,
		Timestamp: time.Now(),
		Hash:      hash,
		Author:    author,
		Message:   message,
		Routes:    routes,
		Stats:     stats,
	}

	// Set parent hash if there's a current version
	if vm.current != nil {
		v.ParentHash = vm.current.Hash
	}

	// Save version to disk
	if err := vm.saveVersion(v); err != nil {
		return nil, fmt.Errorf("failed to save version: %w", err)
	}

	// Add to versions map
	vm.versions[hash] = v
	vm.current = v

	// Cleanup old versions if needed
	vm.cleanupOldVersions()

	return v, nil
}

// GetVersion retrieves a specific version.
func (vm *RouteVersionManager) GetVersion(versionOrHash string) (*RouteVersion, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	// Try hash first
	if v, exists := vm.versions[versionOrHash]; exists {
		return v, nil
	}

	// Try version number
	for _, v := range vm.versions {
		if v.Version == versionOrHash {
			return v, nil
		}
	}

	return nil, fmt.Errorf("version not found: %s", versionOrHash)
}

// ListVersions returns all versions sorted by timestamp.
func (vm *RouteVersionManager) ListVersions() []*RouteVersion {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	versions := make([]*RouteVersion, 0, len(vm.versions))
	for _, v := range vm.versions {
		versions = append(versions, v)
	}

	// Sort by timestamp (newest first)
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Timestamp.After(versions[j].Timestamp)
	})

	return versions
}

// Rollback rolls back to a specific version.
func (vm *RouteVersionManager) Rollback(versionOrHash string) error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Find the version
	var targetVersion *RouteVersion
	if v, exists := vm.versions[versionOrHash]; exists {
		targetVersion = v
	} else {
		// Try version number
		for _, v := range vm.versions {
			if v.Version == versionOrHash {
				targetVersion = v
				break
			}
		}
	}

	if targetVersion == nil {
		return fmt.Errorf("version not found: %s", versionOrHash)
	}

	// Create a rollback version
	rollbackMessage := fmt.Sprintf("Rollback to version %s", targetVersion.Version)
	rollbackVersion := &RouteVersion{
		Version:    vm.generateVersionNumber(),
		Timestamp:  time.Now(),
		Hash:       vm.calculateHash(targetVersion.Routes),
		Author:     os.Getenv("USER"),
		Message:    rollbackMessage,
		Routes:     targetVersion.Routes,
		Stats:      targetVersion.Stats,
		ParentHash: vm.current.Hash,
	}

	// Save rollback version
	if err := vm.saveVersion(rollbackVersion); err != nil {
		return fmt.Errorf("failed to save rollback version: %w", err)
	}

	// Apply routes to filesystem
	if err := vm.applyVersion(rollbackVersion); err != nil {
		return fmt.Errorf("failed to apply rollback: %w", err)
	}

	vm.versions[rollbackVersion.Hash] = rollbackVersion
	vm.current = rollbackVersion

	return nil
}

// DiffVersions compares two versions.
func (vm *RouteVersionManager) DiffVersions(fromVersion, toVersion string) (*VersionDiff, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	from, err := vm.GetVersion(fromVersion)
	if err != nil {
		return nil, fmt.Errorf("from version not found: %w", err)
	}

	to, err := vm.GetVersion(toVersion)
	if err != nil {
		return nil, fmt.Errorf("to version not found: %w", err)
	}

	diff := &VersionDiff{
		FromVersion: from.Version,
		ToVersion:   to.Version,
		Added:       []string{},
		Modified:    []string{},
		Deleted:     []string{},
		Changes:     make(map[string][]string),
	}

	// Find added and modified routes
	for name, toRoute := range to.Routes {
		if fromRoute, exists := from.Routes[name]; exists {
			// Check if modified
			if vm.calculateRouteHash(toRoute) != vm.calculateRouteHash(fromRoute) {
				diff.Modified = append(diff.Modified, name)
				diff.Changes[name] = vm.detectChanges(fromRoute, toRoute)
			}
		} else {
			// Added
			diff.Added = append(diff.Added, name)
		}
	}

	// Find deleted routes
	for name := range from.Routes {
		if _, exists := to.Routes[name]; !exists {
			diff.Deleted = append(diff.Deleted, name)
		}
	}

	return diff, nil
}

// AutoCommit enables or disables automatic version creation.
func (vm *RouteVersionManager) SetAutoCommit(enabled bool) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.autoCommit = enabled
}

// Helper methods

func (vm *RouteVersionManager) calculateHash(routes map[string]*RouteConfig) string {
	// Sort route names for consistent hashing
	names := make([]string, 0, len(routes))
	for name := range routes {
		names = append(names, name)
	}
	sort.Strings(names)

	h := sha256.New()
	for _, name := range names {
		h.Write([]byte(name))
		h.Write([]byte(vm.calculateRouteHash(routes[name])))
	}

	return hex.EncodeToString(h.Sum(nil))[:12]
}

func (vm *RouteVersionManager) calculateRouteHash(route *RouteConfig) string {
	data, _ := json.Marshal(route)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:8]
}

func (vm *RouteVersionManager) calculateStats(routes map[string]*RouteConfig) *VersionStats {
	stats := &VersionStats{
		TotalRoutes:     len(routes),
		TotalEndpoints:  0,
		MethodBreakdown: make(map[string]int),
		NamespaceCount:  make(map[string]int),
		EnabledRoutes:   0,
		DisabledRoutes:  0,
	}

	for _, route := range routes {
		// Count endpoints
		stats.TotalEndpoints += len(route.Spec.Routes)

		// Count by namespace
		ns := route.Metadata.Namespace
		if ns == "" {
			ns = "default"
		}
		stats.NamespaceCount[ns]++

		// Count enabled/disabled
		if route.Metadata.Enabled {
			stats.EnabledRoutes++
		} else {
			stats.DisabledRoutes++
		}

		// Count methods
		for _, r := range route.Spec.Routes {
			methods := []string{}
			switch v := r.Method.(type) {
			case string:
				methods = append(methods, v)
			case []interface{}:
				for _, m := range v {
					if ms, ok := m.(string); ok {
						methods = append(methods, ms)
					}
				}
			}

			for _, method := range methods {
				stats.MethodBreakdown[method]++
			}
		}
	}

	return stats
}

func (vm *RouteVersionManager) generateVersionNumber() string {
	// Generate semantic version number
	now := time.Now()
	return fmt.Sprintf("v%d.%d.%d",
		now.Year(),
		int(now.Month()),
		now.Day()*100+now.Hour())
}

func (vm *RouteVersionManager) saveVersion(v *RouteVersion) error {
	versionsDir := filepath.Join(vm.storageDir, ".versions")
	filename := filepath.Join(versionsDir, fmt.Sprintf("%s_%s.json", v.Version, v.Hash))

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

func (vm *RouteVersionManager) loadVersions() {
	versionsDir := filepath.Join(vm.storageDir, ".versions")

	files, err := os.ReadDir(versionsDir)
	if err != nil {
		return
	}

	var latest *RouteVersion

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(versionsDir, file.Name()))
		if err != nil {
			continue
		}

		var v RouteVersion
		if err := json.Unmarshal(data, &v); err != nil {
			continue
		}

		vm.versions[v.Hash] = &v

		// Track latest version
		if latest == nil || v.Timestamp.After(latest.Timestamp) {
			latest = &v
		}
	}

	vm.current = latest
}

func (vm *RouteVersionManager) applyVersion(v *RouteVersion) error {
	// Write route files back to filesystem
	for name, route := range v.Routes {
		// Determine file path based on namespace
		namespace := route.Metadata.Namespace
		if namespace == "" {
			namespace = "core"
		}

		dir := filepath.Join(vm.storageDir, namespace)
		os.MkdirAll(dir, 0755)

		filename := filepath.Join(dir, name+".yaml")

		// Marshal to YAML
		data, err := marshalRouteToYAML(route)
		if err != nil {
			return fmt.Errorf("failed to marshal route %s: %w", name, err)
		}

		if err := os.WriteFile(filename, data, 0644); err != nil {
			return fmt.Errorf("failed to write route %s: %w", name, err)
		}
	}

	return nil
}

func (vm *RouteVersionManager) cleanupOldVersions() {
	if len(vm.versions) <= vm.maxVersions {
		return
	}

	// Get all versions sorted by timestamp
	versions := vm.ListVersions()

	// Keep only the most recent maxVersions
	for i := vm.maxVersions; i < len(versions); i++ {
		v := versions[i]
		delete(vm.versions, v.Hash)

		// Delete file
		versionsDir := filepath.Join(vm.storageDir, ".versions")
		filename := filepath.Join(versionsDir, fmt.Sprintf("%s_%s.json", v.Version, v.Hash))
		os.Remove(filename)
	}
}

func (vm *RouteVersionManager) detectChanges(from, to *RouteConfig) []string {
	changes := []string{}

	if from.Metadata.Enabled != to.Metadata.Enabled {
		changes = append(changes, fmt.Sprintf("enabled: %v -> %v", from.Metadata.Enabled, to.Metadata.Enabled))
	}

	if from.Metadata.Description != to.Metadata.Description {
		changes = append(changes, "description updated")
	}

	if len(from.Spec.Routes) != len(to.Spec.Routes) {
		changes = append(changes, fmt.Sprintf("routes: %d -> %d", len(from.Spec.Routes), len(to.Spec.Routes)))
	}

	return changes
}

// marshalRouteToYAML converts RouteConfig to YAML.
func marshalRouteToYAML(route *RouteConfig) ([]byte, error) {
	// Use yaml.v3 for marshaling
	// This is a simplified version - you'd want to use the actual YAML library
	return json.MarshalIndent(route, "", "  ")
}
