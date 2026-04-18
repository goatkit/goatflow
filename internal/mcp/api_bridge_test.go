package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/goatkit/goatflow/internal/routing"
)

func TestAPIBridge_Execute_RegistryNotInitialized(t *testing.T) {
	// Ensure global registry is nil — simulates the bug where the server
	// init path didn't set it.
	oldRegistry := routing.GetGlobalRegistry()
	routing.SetGlobalRegistryForTest(nil)
	defer routing.SetGlobalRegistryForTest(oldRegistry)

	bridge := NewAPIBridge()
	tool := &GeneratedTool{
		Tool: Tool{
			Name:        "list_tickets",
			Description: "List tickets",
		},
		HandlerName: "HandleListTicketsAPI",
		Method:      "GET",
		Path:        "/api/v1/tickets",
	}

	_, err := bridge.Execute(context.Background(), tool, nil, UserContext{UserID: 1})
	if err == nil {
		t.Fatal("Expected error when registry is nil")
	}
	if err.Error() != "handler registry not initialized" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestAPIBridge_Execute_HandlerNotFound(t *testing.T) {
	// Set up a registry with no handlers
	registry := routing.NewHandlerRegistry()
	routing.SetGlobalRegistryForTest(registry)
	defer routing.SetGlobalRegistryForTest(nil)

	bridge := NewAPIBridge()
	tool := &GeneratedTool{
		Tool:        Tool{Name: "test_tool"},
		HandlerName: "NonExistentHandler",
		Method:      "GET",
		Path:        "/api/v1/test",
	}

	_, err := bridge.Execute(context.Background(), tool, nil, UserContext{UserID: 1})
	if err == nil {
		t.Fatal("Expected error for missing handler")
	}
}

func TestAPIBridge_Execute_Success(t *testing.T) {
	// Set up a registry with a test handler
	registry := routing.NewHandlerRegistry()
	routing.SetGlobalRegistryForTest(registry)
	defer routing.SetGlobalRegistryForTest(nil)

	// Register a handler that returns JSON
	routing.GlobalHandlerMap["TestHandler"] = func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"user_id": userID,
			"param":   c.Query("status"),
		})
	}
	defer delete(routing.GlobalHandlerMap, "TestHandler")

	bridge := NewAPIBridge()
	tool := &GeneratedTool{
		Tool:        Tool{Name: "test_tool"},
		HandlerName: "TestHandler",
		Method:      "GET",
		Path:        "/api/v1/test",
	}

	result, err := bridge.Execute(context.Background(), tool, map[string]any{
		"status": "open",
	}, UserContext{UserID: 42, UserRole: "Admin"})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Expected success, got error: %s", result.Content[0].Text)
	}

	// Verify response contains the expected JSON
	var resp map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].Text), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp["success"] != true {
		t.Errorf("Expected success=true, got %v", resp["success"])
	}
}

func TestAPIBridge_Execute_WithPathParams(t *testing.T) {
	registry := routing.NewHandlerRegistry()
	routing.SetGlobalRegistryForTest(registry)
	defer routing.SetGlobalRegistryForTest(nil)

	routing.GlobalHandlerMap["GetTicketHandler"] = func(c *gin.Context) {
		id := c.Param("id")
		c.JSON(http.StatusOK, gin.H{"ticket_id": id})
	}
	defer delete(routing.GlobalHandlerMap, "GetTicketHandler")

	bridge := NewAPIBridge()
	tool := &GeneratedTool{
		Tool:        Tool{Name: "get_ticket"},
		HandlerName: "GetTicketHandler",
		Method:      "GET",
		Path:        "/api/v1/tickets/:id",
		PathParams:  []string{"id"},
	}

	result, err := bridge.Execute(context.Background(), tool, map[string]any{
		"id": 42,
	}, UserContext{UserID: 1})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Expected success, got error: %s", result.Content[0].Text)
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].Text), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp["ticket_id"] != "42" {
		t.Errorf("Expected ticket_id=42, got %v", resp["ticket_id"])
	}
}

func TestAPIBridge_Execute_AdminMiddleware_WithAdminRole(t *testing.T) {
	// This test verifies that admin middleware passes when user_role is "Admin".
	// The bug was: API token middleware sets user_role="User" for all agents,
	// so admin users were rejected. The MCP handler now resolves the real role
	// before passing it to the bridge.
	registry := routing.NewHandlerRegistry()
	routing.SetGlobalRegistryForTest(registry)
	defer routing.SetGlobalRegistryForTest(nil)

	// Register admin middleware that checks user_role == "Admin" (same as production)
	registry.RegisterMiddleware("admin", func(c *gin.Context) { //nolint:errcheck
		role, exists := c.Get("user_role")
		if !exists || role != "Admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}
		c.Next()
	})

	routing.GlobalHandlerMap["AdminSQLHandler"] = func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
	defer delete(routing.GlobalHandlerMap, "AdminSQLHandler")

	bridge := NewAPIBridge()
	tool := &GeneratedTool{
		Tool:        Tool{Name: "create_admin_sql"},
		HandlerName: "AdminSQLHandler",
		Method:      "POST",
		Path:        "/api/v1/admin/sql",
		Middleware:  []string{"unified_auth", "admin"},
	}

	// With role "Admin" — should succeed
	result, err := bridge.Execute(context.Background(), tool, nil, UserContext{
		UserID:   1,
		UserRole: "Admin",
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Admin user should pass admin middleware, got: %s", result.Content[0].Text)
	}

	// With role "User" (the buggy api_token value) — should be rejected
	result, err = bridge.Execute(context.Background(), tool, nil, UserContext{
		UserID:   1,
		UserRole: "User",
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("User with role 'User' should be rejected by admin middleware")
	}

	// With role "Agent" — should also be rejected
	result, err = bridge.Execute(context.Background(), tool, nil, UserContext{
		UserID:   1,
		UserRole: "Agent",
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("User with role 'Agent' should be rejected by admin middleware")
	}
}

func TestAPIBridge_Execute_MiddlewareAbort(t *testing.T) {
	registry := routing.NewHandlerRegistry()
	routing.SetGlobalRegistryForTest(registry)
	defer routing.SetGlobalRegistryForTest(nil)

	// Register middleware that denies access
	registry.RegisterMiddleware("admin", func(c *gin.Context) { //nolint:errcheck
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
	})

	routing.GlobalHandlerMap["AdminHandler"] = func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
	defer delete(routing.GlobalHandlerMap, "AdminHandler")

	bridge := NewAPIBridge()
	tool := &GeneratedTool{
		Tool:        Tool{Name: "admin_tool"},
		HandlerName: "AdminHandler",
		Method:      "POST",
		Path:        "/api/v1/admin/action",
		Middleware:  []string{"unified_auth", "admin"}, // unified_auth is filtered, admin runs
	}

	result, err := bridge.Execute(context.Background(), tool, nil, UserContext{
		UserID:   99,
		UserRole: "Agent",
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("Expected error from middleware abort")
	}
	if result.Content[0].Text == "" {
		t.Error("Expected error message in content")
	}
}

func TestAPIBridge_Execute_PostWithBody(t *testing.T) {
	registry := routing.NewHandlerRegistry()
	routing.SetGlobalRegistryForTest(registry)
	defer routing.SetGlobalRegistryForTest(nil)

	routing.GlobalHandlerMap["CreateHandler"] = func(c *gin.Context) {
		var body map[string]any
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"created": true, "title": body["title"]})
	}
	defer delete(routing.GlobalHandlerMap, "CreateHandler")

	bridge := NewAPIBridge()
	tool := &GeneratedTool{
		Tool:        Tool{Name: "create_ticket"},
		HandlerName: "CreateHandler",
		Method:      "POST",
		Path:        "/api/v1/tickets",
	}

	result, err := bridge.Execute(context.Background(), tool, map[string]any{
		"title":    "Test ticket",
		"queue_id": 1,
	}, UserContext{UserID: 1})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Expected success, got error: %s", result.Content[0].Text)
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].Text), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp["title"] != "Test ticket" {
		t.Errorf("Expected title='Test ticket', got %v", resp["title"])
	}
}

func TestAPIBridge_ExecutePlugin_NoCaller(t *testing.T) {
	bridge := NewAPIBridge()
	tool := &GeneratedTool{
		Tool:       Tool{Name: "plugin_tool"},
		IsPlugin:   true,
		PluginName: "test",
	}

	_, err := bridge.ExecutePlugin(context.Background(), tool, nil, UserContext{UserID: 1})
	if err == nil {
		t.Fatal("Expected error when plugin caller is nil")
	}
}

func TestAPIBridge_ExecutePlugin_Success(t *testing.T) {
	bridge := NewAPIBridge()
	bridge.SetPluginCaller(&mockPluginCaller{
		response: `{"result": "ok"}`,
	})

	tool := &GeneratedTool{
		Tool:        Tool{Name: "test_plugin_action"},
		HandlerName: "DoAction",
		IsPlugin:    true,
		PluginName:  "testplugin",
		Method:      "POST",
		Path:        "/plugin/action",
	}

	result, err := bridge.ExecutePlugin(context.Background(), tool, map[string]any{
		"input": "hello",
	}, UserContext{UserID: 1, UserLogin: "admin", UserRole: "Admin"})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Expected success, got error: %s", result.Content[0].Text)
	}
	if result.Content[0].Text != `{"result": "ok"}` {
		t.Errorf("Unexpected response: %s", result.Content[0].Text)
	}
}

// mockPluginCaller implements PluginCaller for testing.
type mockPluginCaller struct {
	response string
	err      error
	// Captured call args
	lastPlugin  string
	lastFn      string
	lastArgs    []byte
}

func (m *mockPluginCaller) Call(ctx context.Context, pluginName, fn string, args []byte) ([]byte, error) {
	m.lastPlugin = pluginName
	m.lastFn = fn
	m.lastArgs = args
	if m.err != nil {
		return nil, m.err
	}
	return []byte(m.response), nil
}
