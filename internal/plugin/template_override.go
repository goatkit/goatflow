package plugin

import (
	"context"
	"encoding/json"
	"sync"
)

// TemplateOverride stores information about a template override.
type TemplateOverride struct {
	PluginName   string // Plugin providing the override
	TemplateName string // Original template name being overridden
	Handler      string // Plugin function to call for template content
}

// TemplateOverrideRegistry manages template overrides from plugins.
type TemplateOverrideRegistry struct {
	mu        sync.RWMutex
	overrides map[string]*TemplateOverride // template name -> override
	manager   *Manager
}

// NewTemplateOverrideRegistry creates a new template override registry.
func NewTemplateOverrideRegistry(mgr *Manager) *TemplateOverrideRegistry {
	return &TemplateOverrideRegistry{
		overrides: make(map[string]*TemplateOverride),
		manager:   mgr,
	}
}

// Register registers template overrides from a plugin.
func (r *TemplateOverrideRegistry) Register(pluginName string, templates []TemplateSpec) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, t := range templates {
		if t.Override {
			r.overrides[t.Name] = &TemplateOverride{
				PluginName:   pluginName,
				TemplateName: t.Name,
				Handler:      "template_" + sanitizeTemplateName(t.Name),
			}
		}
	}
}

// Unregister removes template overrides for a plugin.
func (r *TemplateOverrideRegistry) Unregister(pluginName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for name, override := range r.overrides {
		if override.PluginName == pluginName {
			delete(r.overrides, name)
		}
	}
}

// HasOverride checks if a template has a plugin override.
func (r *TemplateOverrideRegistry) HasOverride(templateName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.overrides[templateName]
	return exists
}

// GetOverride returns the override for a template, if any.
func (r *TemplateOverrideRegistry) GetOverride(templateName string) *TemplateOverride {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.overrides[templateName]
}

// RenderOverride renders a template override by calling the plugin.
// Returns the rendered HTML and true if an override exists, empty string and false otherwise.
func (r *TemplateOverrideRegistry) RenderOverride(ctx context.Context, templateName string, data map[string]any) (string, bool) {
	r.mu.RLock()
	override, exists := r.overrides[templateName]
	r.mu.RUnlock()

	if !exists || r.manager == nil {
		return "", false
	}

	// Call the plugin's template handler
	args, _ := json.Marshal(map[string]any{
		"template": templateName,
		"data":     data,
	})

	result, err := r.manager.Call(ctx, override.PluginName, override.Handler, args)
	if err != nil {
		// Log error but don't fail - fall back to host template
		GetLogBuffer().Log(override.PluginName, "error", 
			"template override failed: "+err.Error(), 
			map[string]any{"template": templateName})
		return "", false
	}

	// Parse result - expect {"html": "..."}
	var response struct {
		HTML string `json:"html"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return "", false
	}

	return response.HTML, true
}

// List returns all registered overrides.
func (r *TemplateOverrideRegistry) List() map[string]*TemplateOverride {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*TemplateOverride, len(r.overrides))
	for k, v := range r.overrides {
		result[k] = v
	}
	return result
}

// sanitizeTemplateName converts a template name to a valid function name.
func sanitizeTemplateName(name string) string {
	result := make([]byte, 0, len(name))
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			result = append(result, byte(c))
		} else {
			result = append(result, '_')
		}
	}
	return string(result)
}

// Global template override registry
var globalTemplateOverrides *TemplateOverrideRegistry
var templateOverridesOnce sync.Once

// GetTemplateOverrides returns the global template override registry.
func GetTemplateOverrides() *TemplateOverrideRegistry {
	return globalTemplateOverrides
}

// SetTemplateOverrides sets the global template override registry.
func SetTemplateOverrides(registry *TemplateOverrideRegistry) {
	globalTemplateOverrides = registry
}
