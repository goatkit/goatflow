package marketplace

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goatkit/goatflow/pkg/plugin"
)

// ThemeCacheDir is the directory where theme plugins extract their assets.
const ThemeCacheDir = "static/themes/.cache"

// IsThemePlugin checks if a manifest describes a theme plugin.
func IsThemePlugin(manifest *plugin.PluginManifest) bool {
	return manifest.PluginType == "theme" || manifest.Runtime == "theme"
}

// InstallTheme extracts theme assets from a plugin directory to the theme cache.
// Theme plugins must contain theme.css and optionally theme.yaml + fonts/.
func InstallTheme(pluginDir string, manifest *plugin.PluginManifest) error {
	themeName := manifest.Name
	targetDir := filepath.Join(ThemeCacheDir, themeName)

	// Create target directory.
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create theme dir: %w", err)
	}

	// Required: theme.css
	cssPath := filepath.Join(pluginDir, "theme.css")
	if _, err := os.Stat(cssPath); os.IsNotExist(err) {
		return fmt.Errorf("theme plugin %q missing theme.css", themeName)
	}

	// Copy theme files.
	filesToCopy := []string{"theme.css", "theme.yaml"}
	for _, f := range filesToCopy {
		src := filepath.Join(pluginDir, f)
		if _, err := os.Stat(src); err != nil {
			continue // Optional file.
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}
		if err := os.WriteFile(filepath.Join(targetDir, f), data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", f, err)
		}
	}

	// Copy fonts/ directory if present.
	fontsDir := filepath.Join(pluginDir, "fonts")
	if info, err := os.Stat(fontsDir); err == nil && info.IsDir() {
		targetFonts := filepath.Join(targetDir, "fonts")
		if err := os.MkdirAll(targetFonts, 0755); err != nil {
			return fmt.Errorf("create fonts dir: %w", err)
		}
		entries, _ := os.ReadDir(fontsDir)
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			data, err := os.ReadFile(filepath.Join(fontsDir, entry.Name()))
			if err != nil {
				continue
			}
			os.WriteFile(filepath.Join(targetFonts, entry.Name()), data, 0644)
		}
	}

	return nil
}

// UninstallTheme removes a theme from the cache directory.
func UninstallTheme(themeName string) error {
	targetDir := filepath.Join(ThemeCacheDir, themeName)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return nil // Already gone.
	}
	return os.RemoveAll(targetDir)
}
