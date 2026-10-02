// Package mcp implements the Model Context Protocol server for GoatFlow.
// This enables AI assistants to interact with GoatFlow via API tokens.
//
// All MCP tools are dynamically generated from YAML route definitions and
// the OpenAPI spec. Tool execution invokes the real Gin handlers via the
// API bridge, so RBAC is enforced by the same middleware stack as the REST API.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

const (
	ProtocolVersion = "2024-11-05"
	ServerName      = "goatflow-mcp"
	ServerVersion   = "0.7.0"
)

// Server handles MCP protocol messages. It holds no identity: every message
// is handled for the principal that authenticated that request, so tool calls
// always run with the caller's current permissions and token scopes.
type Server struct {
	bridge *APIBridge

	initialized bool
}

// NewServer creates a new MCP server instance.
func NewServer(bridge *APIBridge) *Server {
	return &Server{bridge: bridge}
}

// HandleMessage processes a JSON-RPC message for user and returns a response.
func (s *Server) HandleMessage(ctx context.Context, user UserContext, msg []byte) ([]byte, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(msg, &raw); err != nil {
		resp := ErrorResponse(nil, ErrCodeParse, "Parse error: "+err.Error())
		return json.Marshal(resp)
	}
	_, hasID := raw["id"]

	var req Request
	if err := json.Unmarshal(msg, &req); err != nil {
		resp := ErrorResponse(nil, ErrCodeParse, "Parse error: "+err.Error())
		return json.Marshal(resp)
	}

	if req.JSONRPC != "2.0" {
		if !hasID {
			return nil, nil
		}
		resp := ErrorResponse(req.ID, ErrCodeInvalidRequest, "Invalid JSON-RPC version")
		return json.Marshal(resp)
	}

	// JSON-RPC notifications do not include an id and must not receive a
	// response, including error responses. MCP clients send initialized as
	// notifications/initialized after the initialize request.
	if !hasID {
		switch req.Method {
		case "initialized", "notifications/initialized":
			s.initialized = true
		}
		return nil, nil
	}

	var resp Response
	switch req.Method {
	case "initialize":
		resp = s.handleInitialize(req)
	case "initialized":
		// Client acknowledgment, no response needed
		return nil, nil
	case "tools/list":
		resp = s.handleToolsList(req)
	case "tools/call":
		resp = s.handleToolsCall(ctx, user, req)
	case "ping":
		resp = SuccessResponse(req.ID, map[string]string{})
	default:
		resp = ErrorResponse(req.ID, ErrCodeMethodNotFound, "Method not found: "+req.Method)
	}

	return json.Marshal(resp)
}

func (s *Server) handleInitialize(req Request) Response {
	var params InitializeParams
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return ErrorResponse(req.ID, ErrCodeInvalidParams, "Invalid params: "+err.Error())
		}
	}

	s.initialized = true

	// Negotiate protocol version with client
	negotiatedVersion := NegotiateProtocolVersion(params.ProtocolVersion)

	return SuccessResponse(req.ID, InitializeResult{
		ProtocolVersion: negotiatedVersion,
		ServerInfo: ServerInfo{
			Name:    ServerName,
			Version: ServerVersion,
		},
		Capabilities: ServerCapabilities{
			Tools: &ToolsCapability{
				ListChanged: true,
			},
		},
	})
}

func (s *Server) handleToolsList(req Request) Response {
	return SuccessResponse(req.ID, ToolsListResult{
		Tools: GetDynamicTools(),
	})
}

func (s *Server) handleToolsCall(ctx context.Context, user UserContext, req Request) Response {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return ErrorResponse(req.ID, ErrCodeInvalidParams, "Invalid params: "+err.Error())
	}

	result, err := s.executeTool(ctx, user, params.Name, params.Arguments)
	if err != nil {
		return SuccessResponse(req.ID, ToolCallResult{
			Content: []ContentBlock{TextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		})
	}

	return SuccessResponse(req.ID, result)
}

func (s *Server) executeTool(ctx context.Context, user UserContext, name string, args map[string]any) (*ToolCallResult, error) {
	toolsMap := GetDynamicToolsMap()
	tool, ok := toolsMap[name]
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}

	if tool.IsPlugin {
		return s.bridge.ExecutePlugin(ctx, tool, args, user)
	}

	return s.bridge.Execute(ctx, tool, args, user)
}
