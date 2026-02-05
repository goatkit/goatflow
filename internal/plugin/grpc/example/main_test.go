package main

import (
	"encoding/json"
	"testing"
)

func TestHelloGRPCPlugin_GKRegister(t *testing.T) {
	p := &HelloGRPCPlugin{}
	reg, err := p.GKRegister()
	if err != nil {
		t.Fatalf("GKRegister error: %v", err)
	}

	if reg.Name != "hello-grpc" {
		t.Errorf("expected name 'hello-grpc', got %s", reg.Name)
	}
	if reg.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", reg.Version)
	}
	if reg.Description == "" {
		t.Error("expected non-empty description")
	}
	if reg.Author != "GOTRS Team" {
		t.Errorf("expected author 'GOTRS Team', got %s", reg.Author)
	}
	if len(reg.Widgets) != 1 {
		t.Errorf("expected 1 widget, got %d", len(reg.Widgets))
	}
	if len(reg.Routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(reg.Routes))
	}

	// Check widget details
	if reg.Widgets[0].ID != "hello-grpc-widget" {
		t.Errorf("expected widget ID 'hello-grpc-widget', got %s", reg.Widgets[0].ID)
	}
	if reg.Widgets[0].Handler != "render_widget" {
		t.Errorf("expected widget handler 'render_widget', got %s", reg.Widgets[0].Handler)
	}

	// Check route details
	if reg.Routes[0].Path != "/api/plugins/hello-grpc/status" {
		t.Errorf("expected route path '/api/plugins/hello-grpc/status', got %s", reg.Routes[0].Path)
	}
	if reg.Routes[0].Handler != "get_status" {
		t.Errorf("expected route handler 'get_status', got %s", reg.Routes[0].Handler)
	}
}

func TestHelloGRPCPlugin_Init(t *testing.T) {
	p := &HelloGRPCPlugin{}

	config := map[string]string{
		"host_version": "0.7.0",
		"setting":      "value",
	}

	err := p.Init(config)
	if err != nil {
		t.Errorf("Init error: %v", err)
	}

	// Check config was stored
	if p.config == nil {
		t.Error("config not stored")
	}
	if p.config["host_version"] != "0.7.0" {
		t.Errorf("expected host_version '0.7.0', got %s", p.config["host_version"])
	}
}

func TestHelloGRPCPlugin_Call_RenderWidget(t *testing.T) {
	p := &HelloGRPCPlugin{}
	p.Init(nil)

	result, err := p.Call("render_widget", nil)
	if err != nil {
		t.Fatalf("Call error: %v", err)
	}

	var resp map[string]string
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if resp["html"] == "" {
		t.Error("expected html in response")
	}

	// Verify HTML contains expected content
	html := resp["html"]
	if !contains(html, "Hello from gRPC") {
		t.Error("expected 'Hello from gRPC' in HTML")
	}
	if !contains(html, "go-plugin") {
		t.Error("expected 'go-plugin' in HTML")
	}
}

func TestHelloGRPCPlugin_Call_GetStatus(t *testing.T) {
	p := &HelloGRPCPlugin{}
	p.Init(nil)

	result, err := p.Call("get_status", nil)
	if err != nil {
		t.Fatalf("Call error: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if resp["status"] != "running" {
		t.Errorf("expected status 'running', got %v", resp["status"])
	}
	if resp["version"] != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %v", resp["version"])
	}
	if resp["type"] != "grpc" {
		t.Errorf("expected type 'grpc', got %v", resp["type"])
	}
}

func TestHelloGRPCPlugin_Call_UnknownFunction(t *testing.T) {
	p := &HelloGRPCPlugin{}
	p.Init(nil)

	_, err := p.Call("unknown_function", nil)
	if err == nil {
		t.Error("expected error for unknown function")
	}
}

func TestHelloGRPCPlugin_Shutdown(t *testing.T) {
	p := &HelloGRPCPlugin{}
	p.Init(nil)

	err := p.Shutdown()
	if err != nil {
		t.Errorf("Shutdown error: %v", err)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
