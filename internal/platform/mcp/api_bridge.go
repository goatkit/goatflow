package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/platform/routing"
)

// UserContext carries the authenticated caller of an MCP request.
type UserContext struct {
	UserID    int
	UserLogin string
	UserEmail string
	UserRole  string
	// Principal identifies who the caller is across requests ("agent:<id>" or
	// "customer:<id>"); SSE sessions are bound to it. users.id and
	// customer_user.id overlap, so the numeric id alone is not enough.
	Principal string
	// AuthKeys are the gin context keys the auth middleware set on the MCP
	// request (user_role, isInAdminGroup, api_token with its scopes,
	// is_customer, customer_login, ...). They are copied onto every tool
	// call so the tool's middleware sees exactly what the REST route would.
	AuthKeys map[string]any
}

// PluginCaller is the interface for calling plugin functions.
// Matches the signature of plugin.Manager.Call to avoid a direct import.
type PluginCaller interface {
	Call(ctx context.Context, pluginName, fn string, args []byte) ([]byte, error)
}

// PluginGate applies a plugin route's access rules to MCP plugin tools. It is
// implemented by the API layer, which owns the plugin route middleware.
type PluginGate interface {
	// Authorize returns the authorization handlers for a plugin route's
	// middleware list (the caller is already authenticated). An error means
	// the route cannot be called this way.
	Authorize(pluginName string, middleware []string) ([]gin.HandlerFunc, error)
	// Envelope writes the host's identity envelope into the plugin args,
	// replacing any client-supplied values under those keys.
	Envelope(c *gin.Context, args map[string]any, pluginName string)
}

// APIBridge executes generated MCP tools by invoking real Gin handlers
// with a synthetic request context. RBAC middleware runs as normal.
// For plugin tools, it applies the plugin route's middleware through the
// PluginGate and then delegates to the PluginCaller.
type APIBridge struct {
	pluginCaller PluginCaller
	pluginGate   PluginGate
}

// NewAPIBridge creates a new API bridge.
func NewAPIBridge() *APIBridge {
	return &APIBridge{}
}

// SetPluginCaller sets the plugin manager for executing plugin tools.
func (b *APIBridge) SetPluginCaller(caller PluginCaller) {
	b.pluginCaller = caller
}

// SetPluginGate sets the authorizer for plugin tools. Without one, plugin
// tools are refused.
func (b *APIBridge) SetPluginGate(gate PluginGate) {
	b.pluginGate = gate
}

// newCallerContext builds a gin context for a synthetic request carrying the
// MCP caller's identity.
func newCallerContext(req *http.Request, user UserContext) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.ReleaseMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = req

	c.Set("user_id", user.UserID)
	c.Set("user_login", user.UserLogin)
	c.Set("user_email", user.UserEmail)
	c.Set("user_role", user.UserRole)
	for k, v := range user.AuthKeys {
		c.Set(k, v)
	}
	c.Set("authenticated", true)
	return c, recorder
}

// deniedResult converts an aborted middleware chain into a tool error.
func deniedResult(recorder *httptest.ResponseRecorder) *ToolCallResult {
	status := recorder.Code
	body := recorder.Body.String()
	msg := "Permission denied"
	if body != "" {
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
	}
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

	// Auth was done at the MCP layer; the caller's auth keys are copied in.
	c, recorder := newCallerContext(req, user)

	// Set gin params
	var ginParams gin.Params
	for _, pp := range tool.PathParams {
		if v, ok := args[pp]; ok {
			ginParams = append(ginParams, gin.Param{Key: pp, Value: fmt.Sprintf("%v", v)})
		}
	}
	c.Params = ginParams

	// Build and run middleware + handler chain. Authentication middleware
	// (unified_auth, api_token, auth) is skipped: the MCP request already
	// authenticated. Every other middleware must exist; running the handler
	// without one would drop the route's access rule.
	var chain []gin.HandlerFunc
	for _, mwName := range filterRBACMiddleware(tool.Middleware) {
		mw, err := registry.GetMiddleware(mwName)
		if err != nil {
			return nil, fmt.Errorf("tool %s: middleware %q is not available", tool.Name, mwName)
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
		return deniedResult(recorder), nil
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

// ExecutePlugin invokes a plugin handler via the plugin manager after
// applying the plugin route's middleware to the caller, and passes the same
// identity envelope the HTTP plugin routes send.
func (b *APIBridge) ExecutePlugin(ctx context.Context, tool *GeneratedTool, args map[string]any, user UserContext) (*ToolCallResult, error) {
	if b.pluginCaller == nil {
		return nil, fmt.Errorf("plugin system not available")
	}
	if b.pluginGate == nil {
		return nil, fmt.Errorf("plugin tool authorization not configured")
	}

	chain, err := b.pluginGate.Authorize(tool.PluginName, tool.Middleware)
	if err != nil {
		return nil, fmt.Errorf("tool %s: %w", tool.Name, err)
	}

	method, path := tool.Method, tool.Path
	if method == "" {
		method = http.MethodPost
	}
	if path == "" {
		path = "/"
	}
	req, err := http.NewRequestWithContext(ctx, method, path, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	c, recorder := newCallerContext(req, user)
	executeChain(c, chain)
	if c.IsAborted() {
		return deniedResult(recorder), nil
	}

	pluginArgs := make(map[string]any, len(args)+8)
	for k, v := range args {
		pluginArgs[k] = v
	}
	b.pluginGate.Envelope(c, pluginArgs, tool.PluginName)
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
	for _, h := range chain {
		h(c)
		if c.IsAborted() {
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
