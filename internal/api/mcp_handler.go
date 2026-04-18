package api

import (
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/mcp"
	"github.com/goatkit/goatflow/internal/plugin"
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

		if err := mcp.InitDynamicTools(routes, "./docs/api/openapi.yaml"); err != nil {
			log.Printf("mcp init: failed to generate dynamic tools: %v", err)
		}

		mcpBridge = mcp.NewAPIBridge()

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
				Middleware:   rt.Middleware,
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
// Requires Bearer token authentication via API token.
//
//	@Summary		MCP JSON-RPC endpoint
//	@Description	Model Context Protocol endpoint for AI assistant integration
//	@Tags			MCP
//	@Accept			json
//	@Produce		json
//	@Param			request	body		object	true	"JSON-RPC 2.0 request"
//	@Success		200		{object}	object	"JSON-RPC 2.0 response"
//	@Failure		401		{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/mcp [post]
func HandleMCP(c *gin.Context) {
	ensureMCPInit()

	// Get user context from auth middleware
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userLogin := ""
	if login, ok := c.Get("user_login"); ok {
		userLogin, _ = login.(string)
	}

	userRole := ""
	if role, ok := c.Get("user_role"); ok {
		userRole, _ = role.(string)
	}

	// API token middleware sets user_role to "User" for all agents.
	// Resolve the actual role from admin group membership so RBAC middleware works.
	if userRole == "" || userRole == "User" {
		userRole, _ = resolveUserRole(uint(userID.(int)))
	}

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	// Create MCP server instance for this request
	server := mcp.NewServer(userID.(int), userLogin, userRole, mcpBridge)

	// Handle the message
	response, err := server.HandleMessage(c.Request.Context(), body)
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
//
//	@Summary		MCP endpoint info
//	@Description	Get information about the MCP endpoint and available tools
//	@Tags			MCP
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"MCP info"
//	@Router			/mcp [get]
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
