package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/mcp"
	"github.com/goatkit/goatflow/internal/platform/middleware"
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
	if mcpBridge == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "MCP server not initialized"})
		return
	}
	sessions := ensureMCPSessions()

	user, ok := mcpUserContext(c)
	if !ok {
		return
	}

	mcp.HandleStreamableHTTPPost(c.Writer, c.Request, sessions, mcpBridge, user)
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

	user, ok := mcpUserContext(c)
	if !ok {
		return
	}

	mcp.HandleStreamableHTTPGet(c.Writer, c.Request, sessions, user)
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

	user, ok := mcpUserContext(c)
	if !ok {
		return
	}

	mcp.HandleStreamableHTTPDelete(c.Writer, c.Request, sessions, user)
}

// mcpUserContext captures the caller of an MCP request as the auth
// middleware identified it. The role is not re-derived: an agent API token
// keeps role "User" and is an admin only when the token middleware marked it
// so (admin:* scope and an admin owner), exactly as on the REST API, and the
// token itself travels with the auth keys so scope checks still apply.
// Answers 401 and returns false when nobody is authenticated.
func mcpUserContext(c *gin.Context) (mcp.UserContext, bool) {
	var id int
	switch v := c.Value("user_id").(type) {
	case int:
		id = v
	case int64:
		id = int(v)
	case uint:
		id = int(v)
	}
	if id <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return mcp.UserContext{}, false
	}

	user := mcp.UserContext{UserID: id, AuthKeys: make(map[string]any, len(c.Keys))}
	for k, v := range c.Keys {
		if name, ok := k.(string); ok {
			user.AuthKeys[name] = v
		}
	}
	user.UserLogin = firstContextString(c, "user_login", "username", "customer_login")
	user.UserEmail = c.GetString("user_email")
	user.UserRole = c.GetString("user_role")

	kind := "agent"
	if middleware.IsCustomerPrincipal(c) {
		kind = "customer"
	}
	user.Principal = kind + ":" + strconv.Itoa(id)
	return user, true
}

func firstContextString(c *gin.Context, keys ...string) string {
	for _, k := range keys {
		if s := c.GetString(k); s != "" {
			return s
		}
	}
	return ""
}
