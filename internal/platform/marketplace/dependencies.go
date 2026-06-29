package marketplace

import (
	"fmt"
	"strings"
)

// ResolveDependencies checks that all dependencies for a plugin are satisfied
// by installed plugins. Returns missing dependency names.
func ResolveDependencies(pluginName string, index *Index, installed []InstalledPlugin) ([]string, error) {
	// Find the plugin entry.
	var entry *PluginEntry
	for i := range index.Plugins {
		if index.Plugins[i].Name == pluginName {
			entry = &index.Plugins[i]
			break
		}
	}
	if entry == nil {
		return nil, fmt.Errorf("plugin %q not found in index", pluginName)
	}

	if len(entry.Dependencies) == 0 {
		return nil, nil
	}

	// Build installed set.
	installedSet := make(map[string]bool, len(installed))
	for _, inst := range installed {
		installedSet[strings.ToLower(inst.Name)] = true
	}

	var missing []string
	for _, dep := range entry.Dependencies {
		if !installedSet[strings.ToLower(dep)] {
			missing = append(missing, dep)
		}
	}
	return missing, nil
}

// TopologicalSort orders plugins by dependency graph (dependencies first).
// Returns an error if a circular dependency is detected.
func TopologicalSort(plugins []PluginEntry) ([]PluginEntry, error) {
	// Build adjacency map.
	byName := make(map[string]*PluginEntry, len(plugins))
	for i := range plugins {
		byName[plugins[i].Name] = &plugins[i]
	}

	visited := make(map[string]bool)
	inStack := make(map[string]bool)
	var result []PluginEntry

	var visit func(name string) error
	visit = func(name string) error {
		if inStack[name] {
			return fmt.Errorf("circular dependency detected: %s", name)
		}
		if visited[name] {
			return nil
		}

		inStack[name] = true

		entry, ok := byName[name]
		if ok {
			for _, dep := range entry.Dependencies {
				if err := visit(dep); err != nil {
					return err
				}
			}
		}

		inStack[name] = false
		visited[name] = true
		if ok {
			result = append(result, *entry)
		}
		return nil
	}

	for _, p := range plugins {
		if err := visit(p.Name); err != nil {
			return nil, err
		}
	}

	return result, nil
}
