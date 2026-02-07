package grpc

import (
	"encoding/json"
	"errors"
	"testing"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/goatkit/goatflow/internal/plugin"
)

// mockPlugin implements GKPluginInterface for testing
type mockPlugin struct {
	name    string
	version string
	routes  []plugin.RouteSpec
	initErr error
	callFn  func(fn string, args json.RawMessage) (json.RawMessage, error)
}

func (m *mockPlugin) GKRegister() (*plugin.GKRegistration, error) {
	return &plugin.GKRegistration{
		Name:    m.name,
		Version: m.version,
		Routes:  m.routes,
	}, nil
}

func (m *mockPlugin) Init(config map[string]string) error {
	return m.initErr
}

func (m *mockPlugin) Call(fn string, args json.RawMessage) (json.RawMessage, error) {
	if m.callFn != nil {
		return m.callFn(fn, args)
	}
	return json.Marshal(map[string]string{"fn": fn})
}

func (m *mockPlugin) Shutdown() error {
	return nil
}

func TestGKPluginRPCServer_GKRegister(t *testing.T) {
	impl := &mockPlugin{
		name:    "test-plugin",
		version: "1.0.0",
		routes: []plugin.RouteSpec{
			{Path: "/test", Handler: "handle_test"},
		},
	}
	server := &GKPluginRPCServer{Impl: impl}

	var resp plugin.GKRegistration
	err := server.GKRegister(nil, &resp)
	if err != nil {
		t.Errorf("GKRegister error: %v", err)
	}
	if resp.Name != "test-plugin" {
		t.Errorf("expected name 'test-plugin', got %s", resp.Name)
	}
	if resp.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", resp.Version)
	}
	if len(resp.Routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(resp.Routes))
	}
}

func TestGKPluginRPCServer_Init(t *testing.T) {
	t.Run("successful init", func(t *testing.T) {
		impl := &mockPlugin{name: "test"}
		server := &GKPluginRPCServer{Impl: impl}

		req := InitRequest{
			Config:    map[string]string{"key": "value"},
			HostAPIID: 0, // No broker for unit test
		}
		var resp interface{}

		err := server.Init(req, &resp)
		if err != nil {
			t.Errorf("Init error: %v", err)
		}
	})

	t.Run("init with error", func(t *testing.T) {
		impl := &mockPlugin{
			name:    "test",
			initErr: errors.New("plugin not ready"),
		}
		server := &GKPluginRPCServer{Impl: impl}

		req := InitRequest{Config: nil}
		var resp interface{}

		err := server.Init(req, &resp)
		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestGKPluginRPCServer_Call(t *testing.T) {
	t.Run("successful call", func(t *testing.T) {
		impl := &mockPlugin{
			name: "test",
			callFn: func(fn string, args json.RawMessage) (json.RawMessage, error) {
				return json.Marshal(map[string]string{"result": "ok"})
			},
		}
		server := &GKPluginRPCServer{Impl: impl}

		req := CallRequest{
			Function: "test_func",
			Args:     json.RawMessage(`{"arg": "value"}`),
		}
		var resp CallResponse

		err := server.Call(req, &resp)
		if err != nil {
			t.Errorf("Call error: %v", err)
		}
		if resp.Error != "" {
			t.Errorf("unexpected error in response: %s", resp.Error)
		}
		if resp.Result == nil {
			t.Error("expected result")
		}
	})

	t.Run("call with error", func(t *testing.T) {
		impl := &mockPlugin{
			name: "test",
			callFn: func(fn string, args json.RawMessage) (json.RawMessage, error) {
				return nil, errors.New("function not found")
			},
		}
		server := &GKPluginRPCServer{Impl: impl}

		req := CallRequest{Function: "unknown"}
		var resp CallResponse

		err := server.Call(req, &resp)
		// Call itself doesn't return error, it puts it in response
		if err != nil {
			t.Errorf("Call returned error: %v", err)
		}
		if resp.Error == "" {
			t.Error("expected error in response")
		}
	})
}

func TestGKPluginRPCServer_Shutdown(t *testing.T) {
	impl := &mockPlugin{name: "test"}
	server := &GKPluginRPCServer{Impl: impl}

	var resp interface{}
	err := server.Shutdown(nil, &resp)
	if err != nil {
		t.Errorf("Shutdown error: %v", err)
	}
}

func TestGKPluginPlugin_Server(t *testing.T) {
	host := newMockHostAPI()
	pp := &GKPluginPlugin{Host: host}

	// Server should return an RPC server
	broker := (*goplugin.MuxBroker)(nil) // nil is ok for this test
	_, err := pp.Server(broker)
	if err != nil {
		t.Errorf("Server error: %v", err)
	}
}

func TestGKPluginPlugin_Client(t *testing.T) {
	// Client requires a real broker and connection to test fully
	// This is tested via integration tests in runtime_test.go
	t.Skip("Client requires real broker/connection - tested via integration tests")
}
