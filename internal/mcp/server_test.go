package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

func TestServerInitialize(t *testing.T) {
	server := NewServer(nil, 1, "admin")

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
	server := NewServer(nil, 1, "admin")

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

	// Check that list_tickets tool exists
	found := false
	for _, tool := range tools {
		if toolMap, ok := tool.(map[string]any); ok {
			if toolMap["name"] == "list_tickets" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("Expected to find list_tickets tool")
	}
}

func TestServerMethodNotFound(t *testing.T) {
	server := NewServer(nil, 1, "admin")

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

func TestServerPing(t *testing.T) {
	server := NewServer(nil, 1, "admin")

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

func TestToolRegistry(t *testing.T) {
	// Verify all expected tools are registered
	expectedTools := []string{
		"list_tickets",
		"get_ticket",
		"create_ticket",
		"update_ticket",
		"add_article",
		"list_queues",
		"list_users",
		"search_tickets",
		"get_statistics",
		"execute_sql",
	}

	toolNames := make(map[string]bool)
	for _, tool := range ToolRegistry {
		toolNames[tool.Name] = true
	}

	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("Missing expected tool: %s", expected)
		}
	}
}
