// Package yamlmgmt provides YAML configuration file management.
package yamlmgmt

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// ConfigAdapter adapts the existing Config.yaml format to our unified system.
type ConfigAdapter struct {
	versionMgr *VersionManager
}

// NewConfigAdapter creates a new config adapter.
func NewConfigAdapter(versionMgr *VersionManager) *ConfigAdapter {
	return &ConfigAdapter{
		versionMgr: versionMgr,
	}
}

// ImportConfigYAML imports the existing Config.yaml into the version management system.
func (ca *ConfigAdapter) ImportConfigYAML(filename string) error {
	// Read the file
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse as raw YAML first
	var rawConfig map[string]interface{}
	if err := yaml.Unmarshal(data, &rawConfig); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Convert to our YAMLDocument format
	doc := &YAMLDocument{
		APIVersion: "gotrs.io/v1",
		Kind:       string(KindConfig),
		Metadata: Metadata{
			Name:        "system-config",
			Description: "GOTRS System Configuration",
			Modified:    time.Now(),
			Author:      ca.getAuthor(),
		},
		Data: rawConfig,
	}

	// Extract version if present
	if version, ok := rawConfig["version"].(string); ok {
		doc.Metadata.Version = version
	}

	// Extract metadata if present
	if metadata, ok := rawConfig["metadata"].(map[string]interface{}); ok {
		if desc, ok := metadata["description"].(string); ok {
			doc.Metadata.Description = desc
		}
		if updated, ok := metadata["last_updated"].(string); ok {
			// Try to parse the date
			if t, err := time.Parse("2006-01-02", updated); err == nil {
				doc.Metadata.Modified = t
			}
		}
	}

	// Create initial version
	message := fmt.Sprintf("Imported from %s", filepath.Base(filename))
	version, err := ca.versionMgr.CreateVersion(KindConfig, doc.Metadata.Name, doc, message)
	if err != nil {
		return fmt.Errorf("failed to create version: %w", err)
	}

	fmt.Printf("✅ Successfully imported Config.yaml\n")
	fmt.Printf("   Version: %s\n", version.Number)
	fmt.Printf("   Hash:    %s\n", version.Hash[:8])

	return nil
}

// ApplyConfigChanges applies a specific configuration setting.
func (ca *ConfigAdapter) ApplyConfigChanges(settingName string, value interface{}) error {
	// Get current config
	currentDoc, err := ca.versionMgr.GetCurrent(KindConfig, "system-config")
	if err != nil {
		return fmt.Errorf("failed to get current config: %w", err)
	}

	// Find and update the setting
	settings, ok := currentDoc.Data["settings"].([]interface{})
	if !ok {
		return fmt.Errorf("settings not found in config")
	}

	found := false
	for i, setting := range settings {
		if s, ok := setting.(map[string]interface{}); ok {
			if name, ok := s["name"].(string); ok && name == settingName {
				s["default"] = value
				settings[i] = s
				found = true
				break
			}
		}
	}

	if !found {
		return fmt.Errorf("setting '%s' not found", settingName)
	}

	// Create new version
	message := fmt.Sprintf("Updated setting: %s", settingName)
	_, err = ca.versionMgr.CreateVersion(KindConfig, "system-config", currentDoc, message)
	if err != nil {
		return fmt.Errorf("failed to create version: %w", err)
	}

	return nil
}

// GetConfigValue retrieves a specific configuration value.
func (ca *ConfigAdapter) GetConfigValue(settingName string) (interface{}, error) {
	// Get current config
	currentDoc, err := ca.versionMgr.GetCurrent(KindConfig, "system-config")
	if err != nil {
		return nil, fmt.Errorf("failed to get current config: %w", err)
	}

	// Find the setting
	settings, ok := currentDoc.Data["settings"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("settings not found in config")
	}

	for _, setting := range settings {
		if s, ok := setting.(map[string]interface{}); ok {
			if name, ok := s["name"].(string); ok && name == settingName {
				// Precedence: explicit 'value' overrides 'default' when present and non-empty
				if v, exists := s["value"]; exists {
					// Treat zero-values (empty string) as intentional override; only ignore if nil
					if v != nil {
						return v, nil
					}
				}
				if d, exists := s["default"]; exists {
					return d, nil
				}
				return nil, fmt.Errorf("setting '%s' has no value or default", settingName)
			}
		}
	}

	return nil, fmt.Errorf("setting '%s' not found", settingName)
}

// ExportToConfigYAML exports the current config back to Config.yaml format.
func (ca *ConfigAdapter) ExportToConfigYAML(filename string) error {
	// Get current config
	currentDoc, err := ca.versionMgr.GetCurrent(KindConfig, "system-config")
	if err != nil {
		return fmt.Errorf("failed to get current config: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(currentDoc.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// WatchConfigFile sets up hot reload for Config.yaml.
func (ca *ConfigAdapter) WatchConfigFile(filename string, reloadHandler ReloadHandler) error {
	// This would be integrated with the HotReloadManager
	// For now, return a simple implementation
	return fmt.Errorf("not implemented - use HotReloadManager.WatchDirectory instead")
}

// GetConfigSettings returns all configuration settings.
func (ca *ConfigAdapter) GetConfigSettings() ([]map[string]interface{}, error) {
	// Get current config
	currentDoc, err := ca.versionMgr.GetCurrent(KindConfig, "system-config")
	if err != nil {
		// If no current version exists, return empty
		if err.Error() == fmt.Sprintf("no current version for %s/%s", KindConfig, "system-config") {
			return []map[string]interface{}{}, nil
		}
		return nil, fmt.Errorf("failed to get current config: %w", err)
	}

	// Extract settings
	settings, ok := currentDoc.Data["settings"].([]interface{})
	if !ok {
		return []map[string]interface{}{}, nil
	}

	// Convert to typed slice
	result := make([]map[string]interface{}, 0, len(settings))
	for _, setting := range settings {
		if s, ok := setting.(map[string]interface{}); ok {
			result = append(result, s)
		}
	}

	return result, nil
}

// GetConfigGroups returns all configuration groups.
func (ca *ConfigAdapter) GetConfigGroups() ([]map[string]interface{}, error) {
	// Get current config
	currentDoc, err := ca.versionMgr.GetCurrent(KindConfig, "system-config")
	if err != nil {
		// If no current version exists, return empty
		if err.Error() == fmt.Sprintf("no current version for %s/%s", KindConfig, "system-config") {
			return []map[string]interface{}{}, nil
		}
		return nil, fmt.Errorf("failed to get current config: %w", err)
	}

	// Extract groups
	groups, ok := currentDoc.Data["groups"].([]interface{})
	if !ok {
		return []map[string]interface{}{}, nil
	}

	// Convert to typed slice
	result := make([]map[string]interface{}, 0, len(groups))
	for _, group := range groups {
		if g, ok := group.(map[string]interface{}); ok {
			result = append(result, g)
		}
	}

	return result, nil
}

func (ca *ConfigAdapter) getAuthor() string {
	if author := os.Getenv("USER"); author != "" {
		return author
	}
	return "system"
}
