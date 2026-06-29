package yamlmgmt

import (
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

// ConfigAdapter provides a simplified interface for accessing GoatFlow configuration settings.
// It wraps the VersionManager and provides methods to read config values.
type ConfigAdapter struct {
	vm       *VersionManager
	mu       sync.RWMutex
	settings []map[string]interface{}
}

// NewConfigAdapter creates a new ConfigAdapter wrapping the given VersionManager.
func NewConfigAdapter(vm *VersionManager) *ConfigAdapter {
	return &ConfigAdapter{
		vm:       vm,
		settings: []map[string]interface{}{},
	}
}

// ImportConfigYAML imports a Config.yaml file and stores its settings.
func (ca *ConfigAdapter) ImportConfigYAML(path string) error {
	data, err := os.ReadFile(path) //nolint:gosec // G304 false positive - config file path
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}

	var cfg struct {
		Version  string                   `yaml:"version"`
		Settings []map[string]interface{} `yaml:"settings"`
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse config yaml: %w", err)
	}

	ca.mu.Lock()
	ca.settings = cfg.Settings
	ca.mu.Unlock()

	return nil
}

// GetConfigSettings returns all configuration settings.
func (ca *ConfigAdapter) GetConfigSettings() ([]map[string]interface{}, error) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]map[string]interface{}, len(ca.settings))
	for i, s := range ca.settings {
		result[i] = make(map[string]interface{})
		for k, v := range s {
			result[i][k] = v
		}
	}

	return result, nil
}

// GetConfigValue retrieves a configuration value by name.
// It returns the "value" field if set, otherwise the "default" field.
func (ca *ConfigAdapter) GetConfigValue(name string) (interface{}, error) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	for _, setting := range ca.settings {
		settingName, ok := setting["name"].(string)
		if !ok || settingName != name {
			continue
		}

		// Prefer "value" over "default"
		if value, exists := setting["value"]; exists && value != nil && value != "" {
			return value, nil
		}
		if defaultVal, exists := setting["default"]; exists {
			return defaultVal, nil
		}

		return nil, fmt.Errorf("setting %q has no value or default", name)
	}

	return nil, fmt.Errorf("setting %q not found", name)
}
