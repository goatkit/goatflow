package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/routing"
)

// UserContext carries the authenticated user's identity for bridge calls.
type UserContext struct {
	UserID    int
	UserLogin string
	UserEmail string
	UserRole  string
}

// PluginCaller is the interface for calling plugin functions.
// Matches the signature of plugin.Manager.Call to avoid a direct import.
type PluginCaller interface {
	Call(ctx context.Context, pluginName, fn string, args []byte) ([]byte, error)
}

// APIBridge executes generated MCP tools by invoking real Gin handlers
// with a synthetic request context. RBAC middleware runs as normal.
// For plugin tools, it delegates to the PluginCaller.
type APIBridge struct {
	pluginCaller PluginCaller
}

// NewAPIBridge creates a new API bridge.
func NewAPIBridge() *APIBridge {
	return &APIBridge{}
}

// SetPluginCaller sets the plugin manager for executing plugin tools.
func (b *APIBridge) SetPluginCaller(caller PluginCaller) {
	b.pluginCaller = caller
}

// Execute invokes the Gin handler for a generated tool, running RBAC middleware.
func (b *APIBridge) Execute(ctx context.Context, tool *GeneratedTool, args map[string]any, user UserContext) (*ToolCallResult, error) {
	registry := routing.GetGlobalRegistry()
	if registry == nil {
		return nil, fmt.Errorf("handler registry not initialized")
	}

	// Build the resolved path (substitute :params)
	resolvedPath := tool.Path
	queryParams := url.Values{}
	bodyArgs := make(map[string]any)

	for k, v := range args {
		// Check if this is a path parameter
		isPathParam := false
		for _, pp := range tool.PathParams {
			if k == pp {
				isPathParam = true
				resolvedPath = strings.Replace(resolvedPath, ":"+pp, fmt.Sprintf("%v", v), 1)
				break
			}
		}
		if isPathParam {
			continue
		}
		// GET/DELETE: remaining args go to query string
		// POST/PUT/PATCH: remaining args go to JSON body
		if tool.Method == "GET" || tool.Method == "DELETE" {
			queryParams.Set(k, fmt.Sprintf("%v", v))
		} else {
			bodyArgs[k] = v
		}
	}

	// Build request
	var bodyReader io.Reader
	if len(bodyArgs) > 0 {
		bodyJSON, err := json.Marshal(bodyArgs)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyJSON)
	}

	reqURL := resolvedPath
	if len(queryParams) > 0 {
		reqURL += "?" + queryParams.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, tool.Method, reqURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	// Create response recorder
	recorder := httptest.NewRecorder()

	// Create gin context
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = req

	// Set gin params
	var ginParams gin.Params
	for _, pp := range tool.PathParams {
		if v, ok := args[pp]; ok {
			ginParams = append(ginParams, gin.Param{Key: pp, Value: fmt.Sprintf("%v", v)})
		}
	}
	c.Params = ginParams

	// Pre-populate user context (auth already done at MCP layer)
	c.Set("user_id", user.UserID)
	c.Set("user_login", user.UserLogin)
	c.Set("user_email", user.UserEmail)
	c.Set("user_role", user.UserRole)
	// Mark as authenticated so RBAC middleware doesn't reject
	c.Set("authenticated", true)

	_ = engine // Ensure gin.SetMode takes effect

	// Build and run middleware + handler chain
	// Skip auth middleware (unified_auth, api_token, auth) — already authenticated
	rbacMiddleware := filterRBACMiddleware(tool.Middleware)

	var chain []gin.HandlerFunc
	for _, mwName := range rbacMiddleware {
		mw, err := registry.GetMiddleware(mwName)
		if err != nil {
			log.Printf("mcp bridge: warning: middleware %q not found (skipping)", mwName)
			continue
		}
		chain = append(chain, mw)
	}

	// Look up the handler
	handler, err := registry.Get(tool.HandlerName)
	if err != nil {
		// Fallback to GlobalHandlerMap
		if h, ok := routing.GlobalHandlerMap[tool.HandlerName]; ok {
			handler = h
		} else {
			return nil, fmt.Errorf("handler %q not found", tool.HandlerName)
		}
	}
	chain = append(chain, handler)

	// Execute the chain
	c.Set("_gin_handler_chain", chain)
	executeChain(c, chain)

	// Check for abort (middleware denied access)
	if c.IsAborted() {
		status := recorder.Code
		body := recorder.Body.String()
		msg := "Permission denied"
		if body != "" {
			// Try to extract error message from JSON
			var errResp map[string]any
			if err := json.Unmarshal([]byte(body), &errResp); err == nil {
				if errMsg, ok := errResp["error"].(string); ok {
					msg = errMsg
				}
			}
		}
		if status == 0 {
			status = http.StatusForbidden
		}
		return &ToolCallResult{
			Content: []ContentBlock{TextContent(fmt.Sprintf("Error (%d): %s", status, msg))},
			IsError: true,
		}, nil
	}

	// Convert response
	status := recorder.Code
	body := recorder.Body.String()

	if status >= 400 {
		msg := body
		// Try to extract structured error
		var errResp map[string]any
		if err := json.Unmarshal([]byte(body), &errResp); err == nil {
			if errMsg, ok := errResp["error"].(string); ok {
				msg = errMsg
			}
		}
		return &ToolCallResult{
			Content: []ContentBlock{TextContent(fmt.Sprintf("Error (%d): %s", status, msg))},
			IsError: true,
		}, nil
	}

	// Success — return JSON body as-is
	if body == "" {
		body = "{}"
	}
	return &ToolCallResult{
		Content: []ContentBlock{TextContent(body)},
	}, nil
}

// ExecutePlugin invokes a plugin handler via the plugin manager.
// It constructs the same args object that buildPluginArgs does in plugin_handlers.go,
// merging tool arguments with user context fields.
func (b *APIBridge) ExecutePlugin(ctx context.Context, tool *GeneratedTool, args map[string]any, user UserContext) (*ToolCallResult, error) {
	if b.pluginCaller == nil {
		return nil, fmt.Errorf("plugin system not available")
	}

	// Build args matching buildPluginArgs pattern
	pluginArgs := make(map[string]any)
	for k, v := range args {
		pluginArgs[k] = v
	}

	// Inject user context (same fields as buildPluginArgs)
	pluginArgs["_user_id"] = user.UserID
	pluginArgs["_user_login"] = user.UserLogin
	pluginArgs["_user_email"] = user.UserEmail
	pluginArgs["_user_role"] = user.UserRole
	pluginArgs["_method"] = tool.Method
	pluginArgs["_path"] = tool.Path

	argsJSON, err := json.Marshal(pluginArgs)
	if err != nil {
		return nil, fmt.Errorf("marshal plugin args: %w", err)
	}

	result, err := b.pluginCaller.Call(ctx, tool.PluginName, tool.HandlerName, argsJSON)
	if err != nil {
		return &ToolCallResult{
			Content: []ContentBlock{TextContent(fmt.Sprintf("Plugin error: %v", err))},
			IsError: true,
		}, nil
	}

	return &ToolCallResult{
		Content: []ContentBlock{TextContent(string(result))},
	}, nil
}

// executeChain runs a chain of gin handlers sequentially, respecting c.Abort().
func executeChain(c *gin.Context, chain []gin.HandlerFunc) {
	for i, h := range chain {
		h(c)
		if c.IsAborted() {
			_ = i
			return
		}
	}
}

// filterRBACMiddleware returns only the RBAC middleware tokens,
// skipping authentication middleware (already handled by MCP layer).
func filterRBACMiddleware(middleware []string) []string {
	authMiddleware := map[string]bool{
		"unified_auth": true,
		"api_token":    true,
		"auth":         true,
	}
	var rbac []string
	for _, mw := range middleware {
		if authMiddleware[mw] {
			continue
		}
		rbac = append(rbac, mw)
	}
	return rbac
}
