package api

import (
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/mcp"
	"github.com/goatkit/goatflow/internal/platform/plugin"
)

var (
	mcpBridge   *mcp.APIBridge
	mcpInitOnce sync.Once
)

// ensureMCPInit initializes the dynamic tool generator once.
func ensureMCPInit() {
	mcpInitOnce.Do(func() {
		// Load YAML route groups and convert to RouteInput for the MCP tool generator.
		docs, err := LoadYAMLRouteGroups("./routes")
		if err != nil {
			log.Printf("mcp init: failed to load route groups: %v", err)
			return
		}

		var routes []mcp.RouteInput
		for _, doc := range docs {
			for _, rt := range doc.Spec.Routes {
				routes = append(routes, mcp.RouteInput{
					GroupName:       doc.Metadata.Name,
					Prefix:          doc.Spec.Prefix,
					GroupMiddleware: doc.Spec.Middleware,
					Path:            rt.Path,
					Method:          rt.Method,
					HandlerName:     rt.HandlerName,
					Middleware:      rt.Middleware,
					Description:     rt.Description,
					MCPDescription:  rt.MCPDescription,
					MCPEnabled:      rt.MCP,
					RedirectTo:      rt.RedirectTo,
					Websocket:       rt.Websocket,
				})
			}
		}

		if err := mcp.InitDynamicTools(routes, "./api/openapi.yaml"); err != nil {
			log.Printf("mcp init: failed to generate dynamic tools: %v", err)
		}

		mcpBridge = mcp.NewAPIBridge()
		mcpBridge.SetPluginGate(pluginMCPGate{})

		// Wire up plugin tools if plugin manager is available
		if mgr := GetPluginManager(); mgr != nil {
			mcpBridge.SetPluginCaller(mgr)
			refreshPluginMCPTools(mgr)
		}
	})
}

// RefreshPluginMCPTools rebuilds the plugin tools in the MCP registry.
// Called from plugin enable/disable/upload handlers.
func RefreshPluginMCPTools() {
	if mgr := GetPluginManager(); mgr != nil {
		refreshPluginMCPTools(mgr)
		if mcpBridge != nil {
			mcpBridge.SetPluginCaller(mgr)
		}
	}
}

// pluginMCPGate applies plugin route middleware and the plugin args envelope
// to MCP plugin tool calls, the same rules the HTTP plugin routes use.
type pluginMCPGate struct{}

func (pluginMCPGate) Authorize(pluginName string, middleware []string) ([]gin.HandlerFunc, error) {
	return pluginRouteMiddleware(pluginName, middleware, false)
}

func (pluginMCPGate) Envelope(c *gin.Context, args map[string]any, pluginName string) {
	setPluginEnvelope(c, args, pluginName)
}

func refreshPluginMCPTools(mgr *plugin.Manager) {
	var registrations []mcp.PluginRegistration
	for _, manifest := range mgr.List() {
		reg := mcp.PluginRegistration{
			Name: manifest.Name,
		}
		for _, rt := range manifest.Routes {
			reg.Routes = append(reg.Routes, mcp.PluginRouteInput{
				Method:      rt.Method,
				Path:        rt.Path,
				Handler:     rt.Handler,
				Middleware:  rt.Middleware,
				Description: rt.Description,
			})
		}
		for _, mt := range manifest.MCPTools {
			reg.MCPTools = append(reg.MCPTools, mcp.PluginMCPToolInput{
				Name:        mt.Name,
				Description: mt.Description,
				Handler:     mt.Handler,
				InputSchema: mt.InputSchema,
			})
		}
		registrations = append(registrations, reg)
	}

	pluginTools := mcp.GeneratePluginTools(registrations)
	mcp.AddPluginTools(pluginTools)
}

// HandleMCP handles POST /api/mcp for MCP JSON-RPC messages.
// Requires Bearer token authentication via API token. Documented in
// docs/api/MCP.md (the route is outside the /api/v1 Swagger base path).
func HandleMCP(c *gin.Context) {
	ensureMCPInit()
	if mcpBridge == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "MCP server not initialized"})
		return
	}

	user, ok := mcpUserContext(c)
	if !ok {
		return
	}

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	// Create MCP server instance for this request
	server := mcp.NewServer(mcpBridge)

	// Handle the message
	response, err := server.HandleMessage(c.Request.Context(), user, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// No response for notifications (e.g., "initialized")
	if response == nil {
		c.Status(http.StatusNoContent)
		return
	}

	c.Data(http.StatusOK, "application/json", response)
}

// HandleMCPInfo returns information about the MCP endpoint.
func HandleMCPInfo(c *gin.Context) {
	ensureMCPInit()

	dynamicTools := mcp.GetDynamicTools()
	tools := make([]map[string]string, len(dynamicTools))
	for i, tool := range dynamicTools {
		tools[i] = map[string]string{
			"name":        tool.Name,
			"description": tool.Description,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"name":             mcp.ServerName,
		"version":          mcp.ServerVersion,
		"protocol_version": mcp.ProtocolVersion,
		"tools_count":      len(dynamicTools),
		"tools":            tools,
		"authentication":   "Bearer token (API token)",
		"endpoint":         "POST /api/mcp",
	})
}
