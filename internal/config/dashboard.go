package config

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DashboardConfig represents the complete YAML configuration for a dashboard
type DashboardConfig struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name        string `yaml:"name"`
		Version     string `yaml:"version"`
		Created     string `yaml:"created"`
		Description string `yaml:"description"`
	} `yaml:"metadata"`
	Spec DashboardSpec `yaml:"spec"`
}

// DashboardSpec contains the actual dashboard configuration
type DashboardSpec struct {
	Dashboard    Dashboard              `yaml:"dashboard"`
	Colors       map[string]ColorScheme `yaml:"colors"`
	Icons        map[string]string      `yaml:"icons"`
}

// Dashboard represents the main dashboard configuration
type Dashboard struct {
	Title        string        `yaml:"title"`
	Subtitle     string        `yaml:"subtitle"`
	Theme        string        `yaml:"theme"`
	Stats        []Stat        `yaml:"stats"`
	Tiles        []Tile        `yaml:"tiles"`
	QuickActions []QuickAction `yaml:"quick_actions"`
}

// Stat represents a dashboard statistic
type Stat struct {
	Name    string `yaml:"name"`
	Query   string `yaml:"query,omitempty"`
	Command string `yaml:"command,omitempty"`
	Parser  string `yaml:"parser,omitempty"`
	Suffix  string `yaml:"suffix,omitempty"`
	Icon    string `yaml:"icon"`
	Color   string `yaml:"color"`
}

// Tile represents a dashboard tile/tool
type Tile struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	URL         string `yaml:"url"`
	Icon        string `yaml:"icon"`
	Color       string `yaml:"color"`
	Category    string `yaml:"category"`
	Featured    bool   `yaml:"featured,omitempty"`
}

// QuickAction represents a quick action button
type QuickAction struct {
	Name     string `yaml:"name"`
	Action   string `yaml:"action,omitempty"`
	URL      string `yaml:"url,omitempty"`
	Endpoint string `yaml:"endpoint,omitempty"`
	Icon     string `yaml:"icon"`
	Color    string `yaml:"color"`
	Confirm  string `yaml:"confirm,omitempty"`
}

// ColorScheme represents CSS classes for different color states
type ColorScheme struct {
	Background string `yaml:"bg"`
	Text       string `yaml:"text"`
	Button     string `yaml:"button"`
}

// DashboardManager manages dashboard configurations
type DashboardManager struct {
	configPath string
	configs    map[string]*DashboardConfig
}

// NewDashboardManager creates a new dashboard manager
func NewDashboardManager(configPath string) *DashboardManager {
	return &DashboardManager{
		configPath: configPath,
		configs:    make(map[string]*DashboardConfig),
	}
}

// LoadDashboard loads a dashboard configuration from YAML
func (dm *DashboardManager) LoadDashboard(name string) (*DashboardConfig, error) {
	// Check if already loaded
	if config, exists := dm.configs[name]; exists {
		return config, nil
	}

	// Load from file
	filename := filepath.Join(dm.configPath, fmt.Sprintf("%s.yaml", name))
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read dashboard config %s: %w", filename, err)
	}

	// Parse YAML
	var config DashboardConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse dashboard config %s: %w", filename, err)
	}

	// Validate configuration
	if err := dm.validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid dashboard config %s: %w", filename, err)
	}

	// Cache and return
	dm.configs[name] = &config
	return &config, nil
}

// validateConfig performs basic validation on the dashboard configuration
func (dm *DashboardManager) validateConfig(config *DashboardConfig) error {
	if config.APIVersion == "" {
		return fmt.Errorf("apiVersion is required")
	}
	
	if config.Kind == "" {
		return fmt.Errorf("kind is required")
	}
	
	if config.Spec.Dashboard.Title == "" {
		return fmt.Errorf("dashboard title is required")
	}

	// Validate tiles
	for i, tile := range config.Spec.Dashboard.Tiles {
		if tile.Name == "" {
			return fmt.Errorf("tile[%d]: name is required", i)
		}
		if tile.URL == "" {
			return fmt.Errorf("tile[%d]: url is required", i)
		}
		if tile.Icon == "" {
			return fmt.Errorf("tile[%d]: icon is required", i)
		}
		if tile.Color == "" {
			return fmt.Errorf("tile[%d]: color is required", i)
		}
		
		// Validate color scheme exists
		if _, exists := config.Spec.Colors[tile.Color]; !exists {
			return fmt.Errorf("tile[%d]: color '%s' not defined in color schemes", i, tile.Color)
		}
		
		// Validate icon exists
		if _, exists := config.Spec.Icons[tile.Icon]; !exists {
			return fmt.Errorf("tile[%d]: icon '%s' not defined in icon mappings", i, tile.Icon)
		}
	}

	return nil
}

// GetColorScheme returns the CSS classes for a color
func (dm *DashboardManager) GetColorScheme(config *DashboardConfig, color string) ColorScheme {
	if scheme, exists := config.Spec.Colors[color]; exists {
		return scheme
	}
	
	// Return default gray if color not found
	return ColorScheme{
		Background: "bg-gray-100 dark:bg-gray-700",
		Text:       "text-gray-600 dark:text-gray-300",
		Button:     "bg-gray-600 hover:bg-gray-700",
	}
}

// GetIconPath returns the SVG path for an icon
func (dm *DashboardManager) GetIconPath(config *DashboardConfig, icon string) string {
	if path, exists := config.Spec.Icons[icon]; exists {
		return path
	}
	
	// Return default icon if not found
	return "M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12V15.75z"
}

// ReloadDashboard forces a reload of a dashboard configuration
func (dm *DashboardManager) ReloadDashboard(name string) error {
	delete(dm.configs, name)
	_, err := dm.LoadDashboard(name)
	return err
}

// GetTilesByCategory returns tiles grouped by category
func (dm *DashboardManager) GetTilesByCategory(config *DashboardConfig) map[string][]Tile {
	categories := make(map[string][]Tile)
	
	for _, tile := range config.Spec.Dashboard.Tiles {
		category := tile.Category
		if category == "" {
			category = "general"
		}
		categories[category] = append(categories[category], tile)
	}
	
	return categories
}

// Global dashboard manager instance
var DefaultDashboardManager *DashboardManager

// InitializeDashboardManager initializes the global dashboard manager
func InitializeDashboardManager(configPath string) {
	DefaultDashboardManager = NewDashboardManager(configPath)
}