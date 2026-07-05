package mcp

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

// GeneratedTool extends Tool with routing metadata needed for execution.
type GeneratedTool struct {
	Tool
	HandlerName string   // registered handler name, e.g. "HandleGetTicketAPI"
	Method      string   // HTTP method: GET, POST, PUT, DELETE
	Path        string   // full path with Gin params, e.g. "/api/v1/tickets/:id"
	Middleware  []string // combined group + route middleware tokens
	PathParams  []string // extracted from :param segments, e.g. ["id"]
	IsPlugin    bool     // true if this is a plugin-provided tool
	PluginName  string   // plugin name (only if IsPlugin)
}

// RouteInput is the data needed from a YAML route to generate an MCP tool.
// Defined here to avoid importing internal/api (which imports internal/mcp).
type RouteInput struct {
	GroupName       string
	Prefix          string
	GroupMiddleware []string
	Path            string
	Method          string
	HandlerName     string
	Middleware      []string
	Description     string
	MCPDescription  string
	MCPEnabled      *bool // nil = include, false = exclude
	RedirectTo      string
	Websocket       bool
}

// Dynamic tool registry — initialized at startup, refreshed when plugins load.
var (
	dynamicTools    []Tool                    // flat list for tools/list
	dynamicToolsMap map[string]*GeneratedTool // name -> tool for dispatch
	dynamicMu       sync.RWMutex              // protects reads/writes of tools
	dynamicInitOnce sync.Once
	dynamicInitErr  error
	lastOpenAPIPath string // remembered for refresh calls
)

// GetDynamicTools returns the generated tool list. Thread-safe.
func GetDynamicTools() []Tool {
	dynamicMu.RLock()
	defer dynamicMu.RUnlock()
	return dynamicTools
}

// GetDynamicToolsMap returns the tool lookup map. Thread-safe.
func GetDynamicToolsMap() map[string]*GeneratedTool {
	dynamicMu.RLock()
	defer dynamicMu.RUnlock()
	return dynamicToolsMap
}

// InitDynamicTools generates MCP tools from pre-parsed route data and an OpenAPI spec file.
// The caller (in internal/api) is responsible for parsing YAML routes and converting them to
// RouteInput values — this avoids an import cycle between mcp and api packages.
// Safe to call multiple times — only executes once.
func InitDynamicTools(routes []RouteInput, openapiPath string) error {
	dynamicInitOnce.Do(func() {
		lastOpenAPIPath = openapiPath
		dynamicInitErr = initDynamicToolsOnce(routes, openapiPath)
	})
	return dynamicInitErr
}

// RefreshDynamicTools regenerates MCP tools from updated route data.
// Called when plugins lazy-load and register new routes.
func RefreshDynamicTools(routes []RouteInput) {
	if err := initDynamicToolsOnce(routes, lastOpenAPIPath); err != nil {
		log.Printf("mcp: warning: failed to refresh tools: %v", err)
	} else {
		log.Printf("mcp: refreshed %d tools after plugin load", len(dynamicTools))
	}
}

func initDynamicToolsOnce(routes []RouteInput, openapiPath string) error {
	// Load OpenAPI spec (optional — degrade gracefully)
	var spec *OpenAPISpec
	if openapiPath != "" {
		var err error
		spec, err = ParseOpenAPIFile(openapiPath)
		if err != nil {
			log.Printf("mcp: warning: failed to load OpenAPI spec %s: %v (tools will have minimal schemas)", openapiPath, err)
		}
	}

	// Generate tools from API routes
	var generated []*GeneratedTool
	for _, rt := range routes {
		if !isAPIRouteGroup(rt.GroupName) {
			continue
		}
		if rt.Path == "" || rt.Method == "" || rt.HandlerName == "" {
			continue
		}
		// Skip redirects and websockets
		if rt.RedirectTo != "" || rt.Websocket {
			continue
		}
		// Skip auth endpoints
		if strings.Contains(rt.Path, "/auth/") {
			continue
		}
		// Check MCP opt-out
		if rt.MCPEnabled != nil && !*rt.MCPEnabled {
			continue
		}

		fullPath := combinePath(rt.Prefix, rt.Path)
		method := strings.ToUpper(rt.Method)

		// Combine middleware
		middleware := make([]string, 0, len(rt.GroupMiddleware)+len(rt.Middleware))
		middleware = append(middleware, rt.GroupMiddleware...)
		middleware = append(middleware, rt.Middleware...)

		// Extract path params
		pathParams := extractPathParams(fullPath)

		// Generate tool name
		strippedPath := strings.TrimPrefix(fullPath, rt.Prefix)
		toolName := toolNameFromRoute(method, strippedPath)

		// Build description
		description := rt.Description
		if rt.MCPDescription != "" {
			description = rt.MCPDescription
		}

		// Build input schema from OpenAPI
		inputSchema := buildInputSchema(spec, method, fullPath, pathParams)

		tool := &GeneratedTool{
			Tool: Tool{
				Name:        toolName,
				Description: description,
				InputSchema: inputSchema,
			},
			HandlerName: rt.HandlerName,
			Method:      method,
			Path:        fullPath,
			Middleware:  middleware,
			PathParams:  pathParams,
		}
		generated = append(generated, tool)
	}

	// Deduplicate — if names collide, keep the first one and warn
	seen := make(map[string]bool)
	var deduped []*GeneratedTool
	for _, t := range generated {
		if seen[t.Name] {
			log.Printf("mcp: warning: duplicate tool name %q (skipping %s %s)", t.Name, t.Method, t.Path)
			continue
		}
		seen[t.Name] = true
		deduped = append(deduped, t)
	}

	// Build final lists
	toolList := make([]Tool, 0, len(deduped))
	toolMap := make(map[string]*GeneratedTool, len(deduped))
	for _, t := range deduped {
		toolList = append(toolList, t.Tool)
		toolMap[t.Name] = t
	}

	dynamicMu.Lock()
	dynamicTools = toolList
	dynamicToolsMap = toolMap
	dynamicMu.Unlock()

	log.Printf("mcp: generated %d tools from API routes", len(toolList))
	return nil
}

// PluginRegistration holds the data needed to generate MCP tools from a plugin.
// Mirrors the relevant fields from pkg/plugin.GKRegistration to avoid import cycles.
type PluginRegistration struct {
	Name     string
	Routes   []PluginRouteInput
	MCPTools []PluginMCPToolInput
}

// PluginRouteInput mirrors plugin.RouteSpec for MCP tool generation.
type PluginRouteInput struct {
	Method      string
	Path        string
	Handler     string
	Middleware  []string
	Description string
}

// PluginMCPToolInput mirrors plugin.MCPToolSpec for MCP tool generation.
type PluginMCPToolInput struct {
	Name        string
	Description string
	Handler     string
	InputSchema map[string]any
}

// GeneratePluginTools creates MCP tools from plugin registrations.
// Route-based tools are auto-generated; MCPTools override them on name collision.
func GeneratePluginTools(plugins []PluginRegistration) []*GeneratedTool {
	var tools []*GeneratedTool
	seen := make(map[string]bool)

	for _, p := range plugins {
		// First: declared MCPTools (higher priority)
		for _, mt := range p.MCPTools {
			toolName := p.Name + "_" + strings.ToLower(mt.Name)
			toolName = strings.ReplaceAll(toolName, "-", "_")

			schema := InputSchema{
				Type:       "object",
				Properties: make(map[string]Property),
			}
			// Convert input_schema map to MCP InputSchema
			if mt.InputSchema != nil {
				if props, ok := mt.InputSchema["properties"].(map[string]any); ok {
					for name, raw := range props {
						prop := Property{}
						if propMap, ok := raw.(map[string]any); ok {
							if t, ok := propMap["type"].(string); ok {
								prop.Type = t
							}
							if d, ok := propMap["description"].(string); ok {
								prop.Description = d
							}
						}
						schema.Properties[name] = prop
					}
				}
				if req, ok := mt.InputSchema["required"].([]any); ok {
					for _, r := range req {
						if s, ok := r.(string); ok {
							schema.Required = append(schema.Required, s)
						}
					}
				}
			}

			tool := &GeneratedTool{
				Tool: Tool{
					Name:        toolName,
					Description: mt.Description,
					InputSchema: schema,
				},
				HandlerName: mt.Handler,
				IsPlugin:    true,
				PluginName:  p.Name,
			}
			tools = append(tools, tool)
			seen[toolName] = true
		}

		// Second: auto-generate from routes (skip if MCPTool already covers it)
		for _, rt := range p.Routes {
			handlerName := strings.ToLower(rt.Handler)
			handlerName = strings.ReplaceAll(handlerName, "-", "_")
			toolName := p.Name + "_" + handlerName
			toolName = strings.ReplaceAll(toolName, "-", "_")

			if seen[toolName] {
				continue // MCPTool takes priority
			}

			description := rt.Description
			if description == "" {
				description = fmt.Sprintf("Plugin %s: %s %s", p.Name, rt.Method, rt.Path)
			}

			tool := &GeneratedTool{
				Tool: Tool{
					Name:        toolName,
					Description: description,
					InputSchema: InputSchema{
						Type:       "object",
						Properties: make(map[string]Property),
					},
				},
				HandlerName: rt.Handler,
				Method:      strings.ToUpper(rt.Method),
				Path:        rt.Path,
				Middleware:  rt.Middleware,
				PathParams:  extractPathParams(rt.Path),
				IsPlugin:    true,
				PluginName:  p.Name,
			}
			tools = append(tools, tool)
			seen[toolName] = true
		}
	}

	return tools
}

// AddPluginTools adds plugin-provided tools to the dynamic registry.
// Called after plugin loading and on plugin enable/disable.
func AddPluginTools(tools []*GeneratedTool) {
	// Remove existing plugin tools
	newList := make([]Tool, 0, len(dynamicTools))
	newMap := make(map[string]*GeneratedTool, len(dynamicToolsMap))
	for name, t := range dynamicToolsMap {
		if !t.IsPlugin {
			newList = append(newList, t.Tool)
			newMap[name] = t
		}
	}

	// Add plugin tools (plugin tools don't override core tools)
	for _, t := range tools {
		if _, exists := newMap[t.Name]; exists {
			log.Printf("mcp: warning: plugin tool %q conflicts with existing tool (skipping)", t.Name)
			continue
		}
		newList = append(newList, t.Tool)
		newMap[t.Name] = t
	}

	dynamicTools = newList
	dynamicToolsMap = newMap
	log.Printf("mcp: tool registry updated: %d total tools (%d plugin)", len(newList), len(tools))
}

// combinePath joins a prefix and route path. Mirrors api.CombineRoutePath
// but avoids the import cycle.
func combinePath(prefix, route string) string {
	prefix = strings.TrimSpace(prefix)
	route = strings.TrimSpace(route)
	if prefix == "/" {
		prefix = ""
	}
	if prefix != "" {
		if !strings.HasPrefix(prefix, "/") {
			prefix = "/" + prefix
		}
		prefix = strings.TrimSuffix(prefix, "/")
	}
	route = strings.TrimPrefix(route, "/")
	if route == "" {
		if prefix == "" {
			return "/"
		}
		return prefix
	}
	if prefix == "" {
		return "/" + route
	}
	return prefix + "/" + route
}

// isAPIRouteGroup returns true if the group name indicates an API v1 route group.
func isAPIRouteGroup(name string) bool {
	return strings.HasPrefix(name, "api-v1")
}

// extractPathParams extracts parameter names from a Gin-style path.
// e.g. "/api/v1/tickets/:id/articles/:article_id" -> ["id", "article_id"]
func extractPathParams(path string) []string {
	var params []string
	parts := strings.Split(path, "/")
	for _, p := range parts {
		if strings.HasPrefix(p, ":") {
			params = append(params, strings.TrimPrefix(p, ":"))
		}
	}
	return params
}

// toolNameFromRoute generates an MCP tool name from an HTTP method and path.
// Path should have the API prefix already stripped.
//
// Examples:
//
//	GET  /tickets         -> list_tickets
//	POST /tickets         -> create_ticket
//	GET  /tickets/:id     -> get_ticket
//	PUT  /tickets/:id     -> update_ticket
//	DELETE /tickets/:id   -> delete_ticket
//	GET  /tickets/:id/articles -> list_ticket_articles
//	POST /tickets/:id/articles -> create_ticket_article
//	GET  /queues/:id/stats     -> get_queue_stats
func toolNameFromRoute(method, path string) string {
	// Strip leading slash and split
	path = strings.TrimPrefix(path, "/")
	segments := strings.Split(path, "/")

	// Filter out param segments, collect resource segments
	var resources []string
	for _, s := range segments {
		if s == "" || strings.HasPrefix(s, ":") {
			continue
		}
		resources = append(resources, s)
	}

	if len(resources) == 0 {
		return strings.ToLower(method)
	}

	// Determine prefix based on method
	prefix := methodPrefix(method, segments)

	// Build name from resources
	// Singularize the primary resource if we have a path param after it
	name := buildToolName(prefix, resources, segments)

	return name
}

func methodPrefix(method string, segments []string) string {
	switch method {
	case "POST":
		return "create"
	case "PUT", "PATCH":
		return "update"
	case "DELETE":
		return "delete"
	case "GET":
		// If the last non-param segment is plural and the final segment is a param,
		// it's a single-item get. If it's plural with no trailing param, it's a list.
		lastSeg := ""
		lastIsParam := false
		for i := len(segments) - 1; i >= 0; i-- {
			if segments[i] == "" {
				continue
			}
			if strings.HasPrefix(segments[i], ":") {
				lastIsParam = true
				continue
			}
			lastSeg = segments[i]
			break
		}
		if lastIsParam || !isPlural(lastSeg) {
			return "get"
		}
		return "list"
	default:
		return strings.ToLower(method)
	}
}

func buildToolName(prefix string, resources, segments []string) string {
	parts := make([]string, 0, len(resources)+1)
	parts = append(parts, prefix)

	// For non-list operations, singularize the primary (first) resource.
	// For list, keep it plural. Also singularize intermediary resources always.
	singularizeFirst := prefix != "list"
	// For create/update/delete with a single resource and no path param,
	// we're creating/updating an entity type, so singularize.
	// For POST to a sub-resource (e.g. POST /tickets/:id/articles), singularize the last too.
	singularizeLast := prefix == "create" || prefix == "update" || prefix == "delete" || prefix == "get"

	for i, r := range resources {
		name := strings.ReplaceAll(r, "-", "_")
		if i == 0 && (singularizeFirst || len(resources) > 1) {
			name = singularize(name)
		}
		if i > 0 && i < len(resources)-1 {
			// Always singularize intermediate resources
			name = singularize(name)
		}
		if i == len(resources)-1 && i > 0 && singularizeLast {
			name = singularize(name)
		}
		parts = append(parts, name)
	}

	return strings.Join(parts, "_")
}

// isPlural is a simple heuristic — checks if a word ends in 's'.
func isPlural(s string) bool {
	if s == "" {
		return false
	}
	return strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss") && !strings.HasSuffix(s, "us")
}

// singularize removes trailing 's' for simple plurals.
// Handles common patterns: tickets->ticket, articles->article, priorities->priority, etc.
func singularize(s string) string {
	if strings.HasSuffix(s, "ies") && len(s) > 3 {
		return s[:len(s)-3] + "y"
	}
	if strings.HasSuffix(s, "ses") && len(s) > 3 {
		return s[:len(s)-2]
	}
	if strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss") && !strings.HasSuffix(s, "us") && len(s) > 1 {
		return s[:len(s)-1]
	}
	return s
}

// buildInputSchema creates an MCP InputSchema from OpenAPI spec data and path params.
func buildInputSchema(spec *OpenAPISpec, method, ginPath string, pathParams []string) InputSchema {
	schema := InputSchema{
		Type:       "object",
		Properties: make(map[string]Property),
	}

	// Path params are always required
	for _, p := range pathParams {
		schema.Properties[p] = Property{
			Type:        "integer",
			Description: fmt.Sprintf("The %s", p),
		}
		schema.Required = append(schema.Required, p)
	}

	if spec == nil {
		return schema
	}

	// Look up in OpenAPI spec
	openAPIPath := ginPathToOpenAPI(ginPath)
	op := spec.LookupOperation(method, openAPIPath)
	if op == nil {
		return schema
	}

	// Add query parameters (skip path params — already added above)
	for _, p := range op.Parameters {
		if p.In == "path" {
			// Update description from OpenAPI if we have it
			if existing, ok := schema.Properties[p.Name]; ok {
				if p.Description != "" {
					existing.Description = p.Description
				}
				if p.Type != "" {
					existing.Type = p.Type
				}
				schema.Properties[p.Name] = existing
			}
			continue
		}
		if p.In != "query" {
			continue
		}
		prop := Property{
			Type:        p.Type,
			Description: p.Description,
		}
		if prop.Type == "" {
			prop.Type = "string"
		}
		schema.Properties[p.Name] = prop
		if p.Required {
			schema.Required = append(schema.Required, p.Name)
		}
	}

	// Add request body properties for POST/PUT/PATCH
	if op.BodySchema != nil && (method == "POST" || method == "PUT" || method == "PATCH") {
		for name, prop := range op.BodySchema.Properties {
			// Don't override path/query params
			if _, exists := schema.Properties[name]; exists {
				continue
			}
			mcpProp := Property{
				Type:        prop.Type,
				Description: prop.Description,
				Enum:        prop.Enum,
				Default:     prop.Default,
			}
			if mcpProp.Type == "" {
				mcpProp.Type = "string"
			}
			schema.Properties[name] = mcpProp
		}
		// Add required body fields
		for _, req := range op.BodySchema.Required {
			// Don't duplicate
			found := false
			for _, existing := range schema.Required {
				if existing == req {
					found = true
					break
				}
			}
			if !found {
				schema.Required = append(schema.Required, req)
			}
		}
	}

	return schema
}
