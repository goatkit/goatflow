package marketplace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goatkit/goatflow/pkg/plugin"
)

// ThemeCacheDir is the directory where theme plugins extract their assets.
const ThemeCacheDir = "static/themes/.cache"

// IsThemePlugin checks if a manifest describes a theme plugin.
func IsThemePlugin(manifest *plugin.PluginManifest) bool {
	return manifest.PluginType == "theme" || manifest.Runtime == "theme"
}

// themeDir returns the cache directory for themeName. The name comes from a
// downloaded plugin manifest, so it must be a single path element: anything
// else could write or delete outside ThemeCacheDir.
func themeDir(themeName string) (string, error) {
	if themeName == "" || themeName == "." || themeName == ".." ||
		strings.ContainsAny(themeName, `/\`) || filepath.Base(themeName) != themeName {
		return "", fmt.Errorf("invalid theme name %q", themeName)
	}
	return filepath.Join(ThemeCacheDir, themeName), nil
}

// InstallTheme extracts theme assets from a plugin directory to the theme cache.
// Theme plugins must contain theme.css and optionally theme.yaml + fonts/.
func InstallTheme(pluginDir string, manifest *plugin.PluginManifest) error {
	themeName := manifest.Name
	targetDir, err := themeDir(themeName)
	if err != nil {
		return err
	}

	// Create target directory.
	if err := os.MkdirAll(targetDir, 0o750); err != nil {
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
		data, err := os.ReadFile(src) // #nosec G304 -- fixed file name inside the installed plugin dir
		if err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}
		if err := os.WriteFile(filepath.Join(targetDir, f), data, 0o600); err != nil { // #nosec G703 -- targetDir validated by themeDir, f is a fixed name
			return fmt.Errorf("write %s: %w", f, err)
		}
	}

	// Copy fonts/ directory if present.
	fontsDir := filepath.Join(pluginDir, "fonts")
	if info, err := os.Stat(fontsDir); err == nil && info.IsDir() {
		targetFonts := filepath.Join(targetDir, "fonts")
		if err := os.MkdirAll(targetFonts, 0o750); err != nil {
			return fmt.Errorf("create fonts dir: %w", err)
		}
		entries, err := os.ReadDir(fontsDir)
		if err != nil {
			return fmt.Errorf("read fonts dir: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			data, err := os.ReadFile(filepath.Join(fontsDir, entry.Name())) // #nosec G304 -- name from os.ReadDir of the installed plugin's fonts dir
			if err != nil {
				continue
			}
			if err := os.WriteFile(filepath.Join(targetFonts, entry.Name()), data, 0o600); err != nil { // #nosec G703 -- targetDir validated by themeDir, name from os.ReadDir
				return fmt.Errorf("write font %s: %w", entry.Name(), err)
			}
		}
	}

	return nil
}

// UninstallTheme removes a theme from the cache directory.
func UninstallTheme(themeName string) error {
	targetDir, err := themeDir(themeName)
	if err != nil {
		return err
	}
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return nil // Already gone.
	}
	return os.RemoveAll(targetDir)
}
