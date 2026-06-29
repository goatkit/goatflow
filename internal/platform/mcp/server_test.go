package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

func TestServerInitialize(t *testing.T) {
	bridge := NewAPIBridge()
	server := NewServer(1, "admin", "Admin", bridge)

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","clientInfo":{"name":"test","version":"1.0"}}`),
	}

	reqBytes, _ := json.Marshal(req)
	respBytes, err := server.HandleMessage(context.Background(), reqBytes)
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	var resp Response
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("Expected map result, got %T", resp.Result)
	}

	if result["protocolVersion"] != ProtocolVersion {
		t.Errorf("Expected protocol version %s, got %v", ProtocolVersion, result["protocolVersion"])
	}
}

func TestServerToolsList(t *testing.T) {
	// Initialize with some test routes
	initTestTools(t)

	bridge := NewAPIBridge()
	server := NewServer(1, "admin", "Admin", bridge)

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}

	reqBytes, _ := json.Marshal(req)
	respBytes, err := server.HandleMessage(context.Background(), reqBytes)
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	var resp Response
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("Expected map result, got %T", resp.Result)
	}

	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("Expected tools array, got %T", result["tools"])
	}

	if len(tools) == 0 {
		t.Error("Expected at least one tool")
	}
}

func TestServerMethodNotFound(t *testing.T) {
	bridge := NewAPIBridge()
	server := NewServer(1, "admin", "Admin", bridge)

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "unknown/method",
	}

	reqBytes, _ := json.Marshal(req)
	respBytes, err := server.HandleMessage(context.Background(), reqBytes)
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	var resp Response
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("Expected error for unknown method")
	}

	if resp.Error.Code != ErrCodeMethodNotFound {
		t.Errorf("Expected error code %d, got %d", ErrCodeMethodNotFound, resp.Error.Code)
	}
}

func TestServerIgnoresNotifications(t *testing.T) {
	bridge := NewAPIBridge()
	server := NewServer(1, "admin", "Admin", bridge)

	for _, msg := range []string{
		`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":1}}`,
		`{"jsonrpc":"2.0","method":"unknown/notification"}`,
	} {
		respBytes, err := server.HandleMessage(context.Background(), []byte(msg))
		if err != nil {
			t.Fatalf("HandleMessage failed: %v", err)
		}
		if respBytes != nil {
			t.Fatalf("expected no response for notification %s, got %s", msg, string(respBytes))
		}
	}
}

func TestServerPing(t *testing.T) {
	bridge := NewAPIBridge()
	server := NewServer(1, "admin", "Admin", bridge)

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "ping",
	}

	reqBytes, _ := json.Marshal(req)
	respBytes, err := server.HandleMessage(context.Background(), reqBytes)
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	var resp Response
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}
}

// initTestTools sets up a minimal dynamic tools list for testing.
func initTestTools(t *testing.T) {
	t.Helper()
	// Directly populate the dynamic tools for tests
	testTools := []Tool{
		{
			Name:        "list_tickets",
			Description: "List tickets",
			InputSchema: InputSchema{Type: "object", Properties: map[string]Property{}},
		},
	}
	dynamicTools = testTools
	dynamicToolsMap = map[string]*GeneratedTool{
		"list_tickets": {
			Tool:        testTools[0],
			HandlerName: "HandleListTicketsAPI",
			Method:      "GET",
			Path:        "/api/v1/tickets",
			Middleware:  []string{"queue_ro"},
		},
	}
}
