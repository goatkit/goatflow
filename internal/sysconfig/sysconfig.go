package sysconfig

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Manager handles system configuration loading and deployment
type Manager struct {
	db       *sql.DB
	settings map[string]*Setting
	mutex    sync.RWMutex
	deployed bool
}

// Setting represents a configuration setting
type Setting struct {
	ID                       int       `json:"id"`
	Name                     string    `json:"name"`
	Description              string    `json:"description"`
	Navigation               string    `json:"navigation"`
	IsInvisible              bool      `json:"is_invisible"`
	IsReadonly               bool      `json:"is_readonly"`
	IsRequired               bool      `json:"is_required"`
	IsValid                  bool      `json:"is_valid"`
	HasConfigLevel           int       `json:"has_configlevel"`
	UserModificationPossible bool      `json:"user_modification_possible"`
	UserModificationActive   bool      `json:"user_modification_active"`
	XMLContentRaw            string    `json:"xml_content_raw"`
	XMLContentParsed         string    `json:"xml_content_parsed"`
	XMLFilename              string    `json:"xml_filename"`
	EffectiveValue           string    `json:"effective_value"`
	CreateTime               time.Time `json:"create_time"`
	ChangeTime               time.Time `json:"change_time"`
	CreateBy                 int       `json:"create_by"`
	ChangeBy                 int       `json:"change_by"`
	
	// Parsed configuration data
	Type        string                 `json:"type"`
	Default     interface{}            `json:"default"`
	Options     []Option               `json:"options,omitempty"`
	Min         *int                   `json:"min,omitempty"`
	Max         *int                   `json:"max,omitempty"`
	Validation  string                 `json:"validation,omitempty"`
	DependsOn   string                 `json:"depends_on,omitempty"`
	DependsValue string                `json:"depends_value,omitempty"`
}

// Option represents a select option
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// DeployedConfig represents the deployed configuration file structure
type DeployedConfig struct {
	Version    string                 `yaml:"version"`
	Timestamp  time.Time              `yaml:"timestamp"`
	Settings   map[string]interface{} `yaml:"settings"`
	Metadata   map[string]interface{} `yaml:"metadata"`
}

// NewManager creates a new configuration manager
func NewManager(db *sql.DB) *Manager {
	return &Manager{
		db:       db,
		settings: make(map[string]*Setting),
	}
}

// Load loads all configuration settings from the database
func (m *Manager) Load() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	query := `
		SELECT 
			id, name, description, navigation, is_invisible, is_readonly, 
			is_required, is_valid, has_configlevel, user_modification_possible,
			user_modification_active, xml_content_raw, xml_content_parsed,
			xml_filename, effective_value, create_time, change_time,
			create_by, change_by
		FROM sysconfig_default 
		WHERE is_valid = 1
		ORDER BY navigation, name
	`

	rows, err := m.db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query sysconfig_default: %w", err)
	}
	defer rows.Close()

	settings := make(map[string]*Setting)
	
	for rows.Next() {
		setting := &Setting{}
		err := rows.Scan(
			&setting.ID, &setting.Name, &setting.Description, &setting.Navigation,
			&setting.IsInvisible, &setting.IsReadonly, &setting.IsRequired,
			&setting.IsValid, &setting.HasConfigLevel, &setting.UserModificationPossible,
			&setting.UserModificationActive, &setting.XMLContentRaw, 
			&setting.XMLContentParsed, &setting.XMLFilename, &setting.EffectiveValue,
			&setting.CreateTime, &setting.ChangeTime, &setting.CreateBy, &setting.ChangeBy,
		)
		if err != nil {
			return fmt.Errorf("failed to scan setting: %w", err)
		}

		// Parse the XML content to extract type information
		if err := m.parseSettingConfig(setting); err != nil {
			return fmt.Errorf("failed to parse config for %s: %w", setting.Name, err)
		}

		settings[setting.Name] = setting
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating rows: %w", err)
	}

	m.settings = settings
	return nil
}

// parseSettingConfig parses the JSON configuration from xml_content_parsed
func (m *Manager) parseSettingConfig(setting *Setting) error {
	if setting.XMLContentParsed == "" {
		return nil
	}

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(setting.XMLContentParsed), &config); err != nil {
		return fmt.Errorf("invalid JSON in xml_content_parsed: %w", err)
	}

	// Extract type information
	if t, ok := config["type"].(string); ok {
		setting.Type = t
	}

	// Extract default value
	if def, ok := config["default"]; ok {
		setting.Default = def
	}

	// Extract validation rules
	if val, ok := config["validation"].(string); ok {
		setting.Validation = val
	}

	// Extract min/max for integer fields
	if min, ok := config["min"].(float64); ok {
		minInt := int(min)
		setting.Min = &minInt
	}
	if max, ok := config["max"].(float64); ok {
		maxInt := int(max)
		setting.Max = &maxInt
	}

	// Extract dependencies
	if dep, ok := config["depends_on"].(string); ok {
		setting.DependsOn = dep
	}
	if depVal, ok := config["depends_value"].(string); ok {
		setting.DependsValue = depVal
	}

	// Extract options for select fields
	if opts, ok := config["options"].([]interface{}); ok {
		for _, opt := range opts {
			if optMap, ok := opt.(map[string]interface{}); ok {
				option := Option{}
				if val, ok := optMap["value"].(string); ok {
					option.Value = val
				}
				if label, ok := optMap["label"].(string); ok {
					option.Label = label
				}
				setting.Options = append(setting.Options, option)
			}
		}
	}

	return nil
}

// Get retrieves a configuration value
func (m *Manager) Get(name string) (interface{}, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	setting, exists := m.settings[name]
	if !exists {
		return nil, fmt.Errorf("setting %s not found", name)
	}

	// Check for modified value first
	modifiedValue, err := m.getModifiedValue(name)
	if err != nil {
		return nil, err
	}
	if modifiedValue != nil {
		return m.parseValue(modifiedValue.(string), setting.Type), nil
	}

	// Use default value
	return m.parseValue(setting.EffectiveValue, setting.Type), nil
}

// GetString retrieves a string configuration value
func (m *Manager) GetString(name string) string {
	val, err := m.Get(name)
	if err != nil {
		return ""
	}
	if str, ok := val.(string); ok {
		return str
	}
	return fmt.Sprintf("%v", val)
}

// GetInt retrieves an integer configuration value
func (m *Manager) GetInt(name string) int {
	val, err := m.Get(name)
	if err != nil {
		return 0
	}
	if i, ok := val.(int); ok {
		return i
	}
	if str, ok := val.(string); ok {
		if i, err := strconv.Atoi(str); err == nil {
			return i
		}
	}
	return 0
}

// GetBool retrieves a boolean configuration value
func (m *Manager) GetBool(name string) bool {
	val, err := m.Get(name)
	if err != nil {
		return false
	}
	if b, ok := val.(bool); ok {
		return b
	}
	if str, ok := val.(string); ok {
		return strings.ToLower(str) == "true"
	}
	return false
}

// GetArray retrieves an array configuration value
func (m *Manager) GetArray(name string) []string {
	val, err := m.Get(name)
	if err != nil {
		return nil
	}
	
	if str, ok := val.(string); ok {
		// Try to parse as JSON array
		var arr []string
		if err := json.Unmarshal([]byte(str), &arr); err == nil {
			return arr
		}
		// Fall back to comma-separated
		return strings.Split(str, ",")
	}
	
	return nil
}

// parseValue converts string values to appropriate types
func (m *Manager) parseValue(value, valueType string) interface{} {
	switch valueType {
	case "boolean":
		return strings.ToLower(value) == "true"
	case "integer":
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
		return 0
	case "array":
		var arr []string
		if err := json.Unmarshal([]byte(value), &arr); err == nil {
			return arr
		}
		return strings.Split(value, ",")
	default:
		return value
	}
}

// getModifiedValue checks for user-modified values
func (m *Manager) getModifiedValue(name string) (interface{}, error) {
	query := `
		SELECT effective_value 
		FROM sysconfig_modified 
		WHERE name = $1 AND is_valid = 1
		ORDER BY change_time DESC
		LIMIT 1
	`
	
	var value sql.NullString
	err := m.db.QueryRow(query, name).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No modified value
		}
		return nil, err
	}
	
	if !value.Valid {
		return nil, nil
	}
	
	return value.String, nil
}

// Set updates a configuration value
func (m *Manager) Set(name, value string, userID int) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	setting, exists := m.settings[name]
	if !exists {
		return fmt.Errorf("setting %s not found", name)
	}

	if setting.IsReadonly {
		return fmt.Errorf("setting %s is read-only", name)
	}

	if !setting.UserModificationPossible {
		return fmt.Errorf("setting %s cannot be modified by users", name)
	}

	// Validate the value
	if err := m.validateValue(setting, value); err != nil {
		return fmt.Errorf("invalid value for %s: %w", name, err)
	}

	// Insert or update in sysconfig_modified
	query := `
		INSERT INTO sysconfig_modified 
		(sysconfig_default_id, name, effective_value, is_valid, create_by, change_by)
		VALUES ($1, $2, $3, 1, $4, $4)
		ON CONFLICT (name) DO UPDATE SET
		effective_value = $3, change_by = $4, change_time = CURRENT_TIMESTAMP
	`

	_, err := m.db.Exec(query, setting.ID, name, value, userID)
	if err != nil {
		return fmt.Errorf("failed to update setting: %w", err)
	}

	return nil
}

// validateValue validates a configuration value
func (m *Manager) validateValue(setting *Setting, value string) error {
	switch setting.Type {
	case "integer":
		i, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("must be an integer")
		}
		if setting.Min != nil && i < *setting.Min {
			return fmt.Errorf("must be at least %d", *setting.Min)
		}
		if setting.Max != nil && i > *setting.Max {
			return fmt.Errorf("must be at most %d", *setting.Max)
		}
	case "boolean":
		if value != "true" && value != "false" {
			return fmt.Errorf("must be true or false")
		}
	case "select":
		if len(setting.Options) > 0 {
			for _, opt := range setting.Options {
				if opt.Value == value {
					return nil
				}
			}
			return fmt.Errorf("must be one of the allowed options")
		}
	case "email":
		if !strings.Contains(value, "@") {
			return fmt.Errorf("must be a valid email address")
		}
	}

	// Additional validation using regex
	if setting.Validation != "" {
		// TODO: Add regex validation
	}

	return nil
}

// Deploy generates configuration files from database settings
func (m *Manager) Deploy(outputPath string) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	config := DeployedConfig{
		Version:   "1.0",
		Timestamp: time.Now(),
		Settings:  make(map[string]interface{}),
		Metadata: map[string]interface{}{
			"generated_by": "GoTRS SysConfig Manager",
			"total_settings": len(m.settings),
		},
	}

	// Collect all effective values
	for name, setting := range m.settings {
		modifiedValue, err := m.getModifiedValue(name)
		if err != nil {
			return fmt.Errorf("failed to get modified value for %s: %w", name, err)
		}

		var effectiveValue interface{}
		if modifiedValue != nil {
			effectiveValue = m.parseValue(modifiedValue.(string), setting.Type)
			config.Metadata[name+"_modified"] = true
		} else {
			effectiveValue = m.parseValue(setting.EffectiveValue, setting.Type)
		}

		config.Settings[name] = effectiveValue
	}

	// Write to YAML file
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	m.deployed = true
	return nil
}

// GetSettings returns all settings for UI display
func (m *Manager) GetSettings() map[string]*Setting {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Return a copy to prevent modification
	result := make(map[string]*Setting)
	for k, v := range m.settings {
		result[k] = v
	}
	return result
}

// Reset resets a setting to its default value
func (m *Manager) Reset(name string, userID int) error {
	query := `
		DELETE FROM sysconfig_modified 
		WHERE name = $1
	`
	
	_, err := m.db.Exec(query, name)
	return err
}