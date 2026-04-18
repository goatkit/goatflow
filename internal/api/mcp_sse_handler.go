package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/mcp"
)

// mcpSessions is the global MCP session manager for SSE transport.
var mcpSessions *mcp.SessionManager

func ensureMCPSessions() *mcp.SessionManager {
	if mcpSessions == nil {
		mcpSessions = mcp.NewSessionManager(30 * time.Minute)
	}
	return mcpSessions
}

// HandleMCPSSE handles POST /api/mcp/sse for MCP Streamable HTTP transport.
// Creates sessions on "initialize", processes JSON-RPC messages on subsequent requests.
//
//	@Summary		MCP Streamable HTTP endpoint
//	@Description	MCP 2025-03-26 Streamable HTTP transport for AI assistant integration
//	@Tags			MCP
//	@Accept			json
//	@Produce		json
//	@Param			Mcp-Session-Id	header	string	false	"MCP Session ID (required after initialize)"
//	@Param			request			body	object	true	"JSON-RPC 2.0 request"
//	@Success		200				{object}	object	"JSON-RPC 2.0 response"
//	@Failure		401				{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/mcp/sse [post]
func HandleMCPSSE(c *gin.Context) {
	ensureMCPInit()
	sessions := ensureMCPSessions()

	userID, userLogin, userRole, ok := extractMCPUserContext(c)
	if !ok {
		return
	}

	mcp.HandleStreamableHTTPPost(
		c.Writer, c.Request,
		sessions, mcpBridge,
		userID, userLogin, userRole,
	)
}

// HandleMCPSSEStream handles GET /api/mcp/sse for server→client SSE notification stream.
//
//	@Summary		MCP SSE notification stream
//	@Description	Server-sent events stream for MCP server-initiated notifications
//	@Tags			MCP
//	@Produce		text/event-stream
//	@Param			Mcp-Session-Id	header	string	true	"MCP Session ID"
//	@Success		200				{string}	string	"SSE event stream"
//	@Failure		401				{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/mcp/sse [get]
func HandleMCPSSEStream(c *gin.Context) {
	sessions := ensureMCPSessions()

	userID, _, _, ok := extractMCPUserContext(c)
	if !ok {
		return
	}

	mcp.HandleStreamableHTTPGet(
		c.Writer, c.Request,
		sessions, userID,
	)
}

// HandleMCPSSEDelete handles DELETE /api/mcp/sse for session termination.
//
//	@Summary		Terminate MCP session
//	@Description	Terminate an MCP Streamable HTTP session
//	@Tags			MCP
//	@Param			Mcp-Session-Id	header	string	true	"MCP Session ID"
//	@Success		204				"Session terminated"
//	@Failure		401				{object}	map[string]interface{}	"Unauthorized"
//	@Security		BearerAuth
//	@Router			/mcp/sse [delete]
func HandleMCPSSEDelete(c *gin.Context) {
	sessions := ensureMCPSessions()

	userID, _, _, ok := extractMCPUserContext(c)
	if !ok {
		return
	}

	mcp.HandleStreamableHTTPDelete(
		c.Writer, c.Request,
		sessions, userID,
	)
}

// extractMCPUserContext extracts user identity from the Gin context.
// Returns false if the user is not authenticated.
func extractMCPUserContext(c *gin.Context) (userID int, userLogin, userRole string, ok bool) {
	rawID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return 0, "", "", false
	}

	userID, _ = rawID.(int)
	if login, exists := c.Get("user_login"); exists {
		userLogin, _ = login.(string)
	}
	if role, exists := c.Get("user_role"); exists {
		userRole, _ = role.(string)
	}

	// API token middleware sets user_role to "User" for all agents.
	// Resolve the actual role from admin group membership so RBAC middleware works.
	if userRole == "" || userRole == "User" {
		userRole, _ = resolveUserRole(uint(userID))
	}

	return userID, userLogin, userRole, true
}
