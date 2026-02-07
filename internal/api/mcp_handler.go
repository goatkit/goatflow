package api

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/database"
	"github.com/goatkit/goatflow/internal/mcp"
)

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

	// Get database connection
	db, err := database.GetDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database unavailable"})
		return
	}

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	// Create MCP server instance for this request
	server := mcp.NewServer(db, userID.(int), userLogin)

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
	tools := make([]map[string]string, len(mcp.ToolRegistry))
	for i, tool := range mcp.ToolRegistry {
		tools[i] = map[string]string{
			"name":        tool.Name,
			"description": tool.Description,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"name":             mcp.ServerName,
		"version":          mcp.ServerVersion,
		"protocol_version": mcp.ProtocolVersion,
		"tools_count":      len(mcp.ToolRegistry),
		"tools":            tools,
		"authentication":   "Bearer token (API token)",
		"endpoint":         "POST /api/mcp",
	})
}
