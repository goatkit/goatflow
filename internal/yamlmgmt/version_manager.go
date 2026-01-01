package yamlmgmt

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

	"gopkg.in/yaml.v3"
)

// VersionManager manages versions for all YAML configuration types.
type VersionManager struct {
	mu          sync.RWMutex
	storageDir  string
	versions    map[string]map[string][]*Version // kind -> name -> versions
	current     map[string]map[string]*Version   // kind -> name -> current version
	maxVersions int
	schemas     map[YAMLKind]*Schema
}

var globalVersionManager *VersionManager

// GetVersionManager returns the last created version manager (best-effort singleton).
func GetVersionManager() *VersionManager { return globalVersionManager }

// NewVersionManager creates a new version manager.
func NewVersionManager(storageDir string) *VersionManager {
	vm := &VersionManager{
		storageDir:  storageDir,
		versions:    make(map[string]map[string][]*Version),
		current:     make(map[string]map[string]*Version),
		maxVersions: 50,
		schemas:     make(map[YAMLKind]*Schema),
	}

	// Ensure storage directories exist
	vm.ensureDirectories()

	// Load existing versions
	vm.loadAllVersions()

	globalVersionManager = vm
	return vm
}

// CreateVersion creates a new version for a YAML document.
func (vm *VersionManager) CreateVersion(kind YAMLKind, name string, doc *YAMLDocument, message string) (*Version, error) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// Ensure document has proper metadata
	if doc.Metadata.Name == "" {
		doc.Metadata.Name = name
	}
	if doc.Metadata.Modified.IsZero() {
		doc.Metadata.Modified = time.Now()
	}

	// Calculate hash
	hash := vm.calculateHash(doc)

	// Check if this exact version already exists
	if versions := vm.getVersionsForDocument(string(kind), name); versions != nil {
		for _, v := range versions {
			if v.Hash == hash {
				return v, nil // Return existing version
			}
		}
	}

	// Get parent hash if there's a current version
	var parentHash string
	if current := vm.getCurrentVersion(string(kind), name); current != nil {
		parentHash = current.Hash
	}

	// Create version
	version := &Version{
		ID:         vm.generateVersionID(),
		Number:     vm.generateVersionNumber(),
		Kind:       kind,
		Name:       name,
		Hash:       hash,
		ParentHash: parentHash,
		Timestamp:  time.Now(),
		Author:     vm.getAuthor(),
		Message:    message,
		Document:   doc,
		Stats:      vm.calculateStats(doc),
	}

	// Calculate changes from parent
	if parentHash != "" {
		if parent := vm.findVersionByHash(string(kind), name, parentHash); parent != nil {
			version.Changes = vm.calculateChanges(parent.Document, doc)
		}
	}

	// Save version to disk
	if err := vm.saveVersion(version); err != nil {
		return nil, fmt.Errorf("failed to save version: %w", err)
	}

	// Add to memory
	vm.addVersion(version)

	// Set as current
	vm.setCurrentVersion(string(kind), name, version)

	// Cleanup old versions
	vm.cleanupOldVersions(string(kind), name)

	return version, nil
}

// GetVersion retrieves a specific version.
func (vm *VersionManager) GetVersion(kind YAMLKind, name string, versionID string) (*Version, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	versions := vm.getVersionsForDocument(string(kind), name)
	if versions == nil {
		return nil, fmt.Errorf("no versions found for %s/%s", kind, name)
	}

	for _, v := range versions {
		if v.ID == versionID || v.Number == versionID || v.Hash == versionID {
			return v, nil
		}
	}

	return nil, fmt.Errorf("version %s not found for %s/%s", versionID, kind, name)
}

// ListVersions returns all versions for a document.
func (vm *VersionManager) ListVersions(kind YAMLKind, name string) ([]*Version, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	versions := vm.getVersionsForDocument(string(kind), name)
	if versions == nil {
		return []*Version{}, nil
	}

	// Sort by timestamp (newest first)
	sorted := make([]*Version, len(versions))
	copy(sorted, versions)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.After(sorted[j].Timestamp)
	})

	return sorted, nil
}

// GetCurrent returns the current version of a document.
func (vm *VersionManager) GetCurrent(kind YAMLKind, name string) (*YAMLDocument, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	current := vm.getCurrentVersion(string(kind), name)
	if current == nil {
		return nil, fmt.Errorf("no current version for %s/%s", kind, name)
	}

	return current.Document, nil
}

// Rollback rolls back to a specific version.
func (vm *VersionManager) Rollback(kind YAMLKind, name string, versionID string) error {
	// Get the target version
	targetVersion, err := vm.GetVersion(kind, name, versionID)
	if err != nil {
		return err
	}

	// Create a rollback version
	rollbackMessage := fmt.Sprintf("Rollback to version %s", targetVersion.Number)
	_, err = vm.CreateVersion(kind, name, targetVersion.Document, rollbackMessage)
	if err != nil {
		return fmt.Errorf("failed to create rollback version: %w", err)
	}

	// Apply the version to the filesystem
	if err := vm.applyVersion(targetVersion); err != nil {
		return fmt.Errorf("failed to apply rollback: %w", err)
	}

	return nil
}

// DiffVersions compares two versions.
func (vm *VersionManager) DiffVersions(kind YAMLKind, name string, fromID, toID string) ([]Change, error) {
	fromVersion, err := vm.GetVersion(kind, name, fromID)
	if err != nil {
		return nil, fmt.Errorf("from version: %w", err)
	}

	toVersion, err := vm.GetVersion(kind, name, toID)
	if err != nil {
		return nil, fmt.Errorf("to version: %w", err)
	}

	return vm.calculateChanges(fromVersion.Document, toVersion.Document), nil
}

// ListAll returns all current documents of a specific kind.
func (vm *VersionManager) ListAll(kind YAMLKind) ([]*YAMLDocument, error) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	kindStr := string(kind)
	documents := []*YAMLDocument{}

	if kindMap, exists := vm.current[kindStr]; exists {
		for _, version := range kindMap {
			if version != nil && version.Document != nil {
				documents = append(documents, version.Document)
			}
		}
	}

	// Sort by name for consistent output
	sort.Slice(documents, func(i, j int) bool {
		return documents[i].Metadata.Name < documents[j].Metadata.Name
	})

	return documents, nil
}

// Helper methods

func (vm *VersionManager) ensureDirectories() {
	dirs := []string{
		filepath.Join(vm.storageDir, ".versions"),
		filepath.Join(vm.storageDir, ".versions", "routes"),
		filepath.Join(vm.storageDir, ".versions", "config"),
		filepath.Join(vm.storageDir, ".versions", "dashboards"),
		filepath.Join(vm.storageDir, ".versions", "compose"),
	}

	for _, dir := range dirs {
		os.MkdirAll(dir, 0755)
	}
}

func (vm *VersionManager) calculateHash(doc *YAMLDocument) string {
	data, _ := json.Marshal(doc)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:12]
}

func (vm *VersionManager) generateVersionID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), vm.generateRandomString(8))
}

func (vm *VersionManager) generateVersionNumber() string {
	now := time.Now()
	return fmt.Sprintf("v%d.%d.%d-%02d%02d",
		now.Year(),
		int(now.Month()),
		now.Day(),
		now.Hour(),
		now.Minute())
}

func (vm *VersionManager) generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

func (vm *VersionManager) getAuthor() string {
	if author := os.Getenv("USER"); author != "" {
		return author
	}
	return "system"
}

func (vm *VersionManager) calculateStats(doc *YAMLDocument) *VersionStats {
	stats := &VersionStats{
		CustomStats: make(map[string]int),
	}

	// Count fields
	stats.TotalFields = vm.countFields(doc)

	// Kind-specific stats
	switch YAMLKind(doc.Kind) {
	case KindRoute:
		if spec, ok := doc.Spec.(map[string]interface{}); ok {
			if routes, ok := spec["routes"].([]interface{}); ok {
				stats.CustomStats["routes"] = len(routes)
			}
		}
	case KindConfig:
		if settings, ok := doc.Data["settings"].([]interface{}); ok {
			stats.CustomStats["settings"] = len(settings)
		}
	case KindDashboard:
		if spec, ok := doc.Spec.(map[string]interface{}); ok {
			if dashboard, ok := spec["dashboard"].(map[string]interface{}); ok {
				if tiles, ok := dashboard["tiles"].([]interface{}); ok {
					stats.CustomStats["tiles"] = len(tiles)
				}
			}
		}
	}

	return stats
}

func (vm *VersionManager) countFields(v interface{}) int {
	count := 0
	switch val := v.(type) {
	case map[string]interface{}:
		for _, v := range val {
			count++
			count += vm.countFields(v)
		}
	case []interface{}:
		for _, item := range val {
			count += vm.countFields(item)
		}
	}
	return count
}

func (vm *VersionManager) calculateChanges(oldDoc, newDoc *YAMLDocument) []Change {
	changes := []Change{}

	// Compare metadata
	if oldDoc.Metadata.Description != newDoc.Metadata.Description {
		changes = append(changes, Change{
			Type:     ChangeTypeModify,
			Path:     "metadata.description",
			OldValue: oldDoc.Metadata.Description,
			NewValue: newDoc.Metadata.Description,
		})
	}

	// Compare spec (simplified for now)
	oldSpec, _ := json.Marshal(oldDoc.Spec)
	newSpec, _ := json.Marshal(newDoc.Spec)
	if string(oldSpec) != string(newSpec) {
		changes = append(changes, Change{
			Type:        ChangeTypeModify,
			Path:        "spec",
			Description: "Specification changed",
		})
	}

	return changes
}

func (vm *VersionManager) saveVersion(version *Version) error {
	kindDir := filepath.Join(vm.storageDir, ".versions", string(version.Kind))
	os.MkdirAll(kindDir, 0755)

	filename := filepath.Join(kindDir, fmt.Sprintf("%s_%s_%s.json",
		version.Name, version.Number, version.Hash[:8]))

	data, err := json.MarshalIndent(version, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

func (vm *VersionManager) applyVersion(version *Version) error {
	// Determine the appropriate directory and format
	var targetPath string
	var data []byte
	var err error

	switch version.Kind {
	case KindRoute:
		targetPath = filepath.Join(vm.storageDir, "routes",
			version.Document.Metadata.Namespace, version.Name+".yaml")
		data, err = yaml.Marshal(version.Document)

	case KindConfig:
		targetPath = filepath.Join(vm.storageDir, "config", version.Name+".yaml")
		data, err = yaml.Marshal(version.Document)

	case KindDashboard:
		targetPath = filepath.Join(vm.storageDir, "dashboards", version.Name+".yaml")
		data, err = yaml.Marshal(version.Document)

	default:
		return fmt.Errorf("unsupported kind: %s", version.Kind)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal document: %w", err)
	}

	// Ensure directory exists
	os.MkdirAll(filepath.Dir(targetPath), 0755)

	// Write file
	return os.WriteFile(targetPath, data, 0644)
}

func (vm *VersionManager) getVersionsForDocument(kind, name string) []*Version {
	if kindMap, exists := vm.versions[kind]; exists {
		if versions, exists := kindMap[name]; exists {
			return versions
		}
	}
	return nil
}

func (vm *VersionManager) getCurrentVersion(kind, name string) *Version {
	if kindMap, exists := vm.current[kind]; exists {
		if version, exists := kindMap[name]; exists {
			return version
		}
	}
	return nil
}

func (vm *VersionManager) setCurrentVersion(kind, name string, version *Version) {
	if _, exists := vm.current[kind]; !exists {
		vm.current[kind] = make(map[string]*Version)
	}
	vm.current[kind][name] = version
}

func (vm *VersionManager) addVersion(version *Version) {
	kind := string(version.Kind)
	name := version.Name

	if _, exists := vm.versions[kind]; !exists {
		vm.versions[kind] = make(map[string][]*Version)
	}
	if _, exists := vm.versions[kind][name]; !exists {
		vm.versions[kind][name] = []*Version{}
	}

	vm.versions[kind][name] = append(vm.versions[kind][name], version)
}

func (vm *VersionManager) findVersionByHash(kind, name, hash string) *Version {
	versions := vm.getVersionsForDocument(kind, name)
	for _, v := range versions {
		if v.Hash == hash {
			return v
		}
	}
	return nil
}

func (vm *VersionManager) cleanupOldVersions(kind, name string) {
	versions := vm.getVersionsForDocument(kind, name)
	if len(versions) <= vm.maxVersions {
		return
	}

	// Sort by timestamp (oldest first)
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Timestamp.Before(versions[j].Timestamp)
	})

	// Remove oldest versions
	toRemove := len(versions) - vm.maxVersions
	for i := 0; i < toRemove; i++ {
		v := versions[i]
		// Delete file
		kindDir := filepath.Join(vm.storageDir, ".versions", kind)
		filename := filepath.Join(kindDir, fmt.Sprintf("%s_%s_%s.json",
			v.Name, v.Number, v.Hash[:8]))
		os.Remove(filename)
	}

	// Update in-memory list
	vm.versions[kind][name] = versions[toRemove:]
}

func (vm *VersionManager) loadAllVersions() {
	versionsDir := filepath.Join(vm.storageDir, ".versions")

	// Walk through all version directories
	filepath.Walk(versionsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}

		// Load version file
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		var version Version
		if err := json.Unmarshal(data, &version); err != nil {
			return nil
		}

		// Add to memory
		vm.addVersion(&version)

		// Track latest version as current
		current := vm.getCurrentVersion(string(version.Kind), version.Name)
		if current == nil || version.Timestamp.After(current.Timestamp) {
			vm.setCurrentVersion(string(version.Kind), version.Name, &version)
		}

		return nil
	})
}
