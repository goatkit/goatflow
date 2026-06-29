package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/flosch/pongo2/v6"
)

// pluginManager reference for template tags - set via SetTemplatePluginManager
var templatePluginManager *Manager

// SetTemplatePluginManager sets the plugin manager for template tags.
// Call this during app initialization after creating the plugin manager.
func SetTemplatePluginManager(mgr *Manager) {
	templatePluginManager = mgr
}

// useTag implements the {% use "plugin_name" %} template tag.
// It makes plugin functions available in the template context.
type useTag struct {
	pluginName string
}

// tagUseParser parses {% use "plugin_name" %}
func tagUseParser(doc *pongo2.Parser, start *pongo2.Token, arguments *pongo2.Parser) (pongo2.INodeTag, *pongo2.Error) {
	tag := &useTag{}

	// Expect plugin name as string
	nameToken := arguments.MatchType(pongo2.TokenString)
	if nameToken == nil {
		return nil, arguments.Error("{% use %} requires a plugin name as string argument", nil)
	}
	tag.pluginName = nameToken.Val

	return tag, nil
}

// Execute adds the plugin functions to the context.
func (t *useTag) Execute(ctx *pongo2.ExecutionContext, writer pongo2.TemplateWriter) *pongo2.Error {
	if templatePluginManager == nil {
		return ctx.Error("plugin manager not initialized", nil)
	}

	p, exists := templatePluginManager.Get(t.pluginName)
	if !exists {
		return ctx.Error(fmt.Sprintf("plugin %q not found or disabled", t.pluginName), nil)
	}

	manifest := p.GKRegister()

	// Create a plugin caller function that templates can use
	caller := &PluginCaller{
		Manager:    templatePluginManager,
		PluginName: t.pluginName,
	}

	// Add to private context (available to this template and children)
	ctx.Private[t.pluginName] = caller

	// Also add plugin metadata
	ctx.Private[t.pluginName+"_meta"] = map[string]interface{}{
		"name":        manifest.Name,
		"version":     manifest.Version,
		"description": manifest.Description,
	}

	return nil
}

// PluginCaller wraps plugin function calls for templates.
// Exported so it can be used directly in template contexts.
type PluginCaller struct {
	Manager    *Manager
	PluginName string
	Ctx        context.Context // Optional context for i18n
}

// Call invokes a plugin function. Used via {{ plugin.Call("fn", args) }}
func (pc *PluginCaller) Call(fn string, args ...interface{}) interface{} {
	if pc.Manager == nil {
		return map[string]string{"error": "plugin manager not available"}
	}

	ctx := pc.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	// Convert args to JSON if needed
	var argsJSON []byte
	if len(args) > 0 {
		argsJSON, _ = json.Marshal(args[0])
	}

	result, err := pc.Manager.Call(ctx, pc.PluginName, fn, argsJSON)
	if err != nil {
		return map[string]string{"error": err.Error()}
	}

	// Parse result JSON
	var parsed interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return string(result) // Return as string if not valid JSON
	}
	return parsed
}

// Widget renders a plugin widget by ID. Returns HTML string.
func (pc *PluginCaller) Widget(widgetID string) string {
	if pc.Manager == nil {
		return "<!-- plugin manager not available -->"
	}

	ctx := pc.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	// Find the widget in the plugin's manifest
	p, exists := pc.Manager.Get(pc.PluginName)
	if !exists {
		return fmt.Sprintf("<!-- plugin %q not found -->", pc.PluginName)
	}

	manifest := p.GKRegister()
	for _, w := range manifest.Widgets {
		if w.ID == widgetID {
			result, err := pc.Manager.Call(ctx, pc.PluginName, w.Handler, nil)
			if err != nil {
				return fmt.Sprintf("<!-- widget error: %v -->", err)
			}

			var response map[string]interface{}
			if err := json.Unmarshal(result, &response); err != nil {
				return fmt.Sprintf("<!-- widget parse error: %v -->", err)
			}

			if html, ok := response["html"].(string); ok {
				return html
			}
			return fmt.Sprintf("<!-- widget %q did not return html -->", widgetID)
		}
	}

	return fmt.Sprintf("<!-- widget %q not found in plugin %q -->", widgetID, pc.PluginName)
}

// Translate calls the plugin's translation with the current context language.
func (pc *PluginCaller) Translate(key string, args ...interface{}) string {
	if pc.Manager == nil {
		return key
	}

	// Get the host API from manager and call Translate
	// For now, return key - full implementation needs host API access
	return key
}

func init() {
	// Register the {% use %} tag with pongo2
	pongo2.RegisterTag("use", tagUseParser)
}
