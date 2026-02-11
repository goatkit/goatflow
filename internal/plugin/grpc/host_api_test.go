package grpc

import (
	"context"
	"encoding/json"
	"net"
	"net/rpc"
	"testing"

	"github.com/goatkit/goatflow/internal/plugin"
)

// mockHostAPI for testing
type mockHostAPI struct {
	queryResults []map[string]any
	execAffected int64
	cacheData    map[string][]byte
	logs         []string
	configData   map[string]string
}

func newMockHostAPI() *mockHostAPI {
	return &mockHostAPI{
		queryResults: []map[string]any{{"id": 1, "name": "test"}},
		execAffected: 1,
		cacheData:    make(map[string][]byte),
		configData:   map[string]string{"app.name": "GoatFlow"},
	}
}

func (m *mockHostAPI) DBQuery(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	return m.queryResults, nil
}

func (m *mockHostAPI) DBExec(ctx context.Context, query string, args ...any) (int64, error) {
	return m.execAffected, nil
}

func (m *mockHostAPI) CacheGet(ctx context.Context, key string) ([]byte, bool, error) {
	v, ok := m.cacheData[key]
	return v, ok, nil
}

func (m *mockHostAPI) CacheSet(ctx context.Context, key string, value []byte, ttlSeconds int) error {
	m.cacheData[key] = value
	return nil
}

func (m *mockHostAPI) CacheDelete(ctx context.Context, key string) error {
	delete(m.cacheData, key)
	return nil
}

func (m *mockHostAPI) HTTPRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) (int, []byte, error) {
	return 200, []byte(`{"ok":true}`), nil
}

func (m *mockHostAPI) SendEmail(ctx context.Context, to, subject, body string, html bool) error {
	return nil
}

func (m *mockHostAPI) Log(ctx context.Context, level, message string, fields map[string]any) {
	m.logs = append(m.logs, message)
}

func (m *mockHostAPI) ConfigGet(ctx context.Context, key string) (string, error) {
	return m.configData[key], nil
}

func (m *mockHostAPI) Translate(ctx context.Context, key string, args ...any) string {
	return key
}

func (m *mockHostAPI) CallPlugin(ctx context.Context, pluginName, function string, args json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(map[string]string{"result": "ok"})
}

func (m *mockHostAPI) PublishEvent(ctx context.Context, eventType string, data string) error {
	return nil
}

func TestDispatchHostCall(t *testing.T) {
	host := newMockHostAPI()
	ctx := context.Background()

	t.Run("db_query", func(t *testing.T) {
		args, _ := json.Marshal(map[string]any{
			"query": "SELECT * FROM test",
			"args":  []any{},
		})
		result, err := dispatchHostCall(ctx, host, "db_query", args)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result == nil {
			t.Error("expected result")
		}
	})

	t.Run("db_exec", func(t *testing.T) {
		args, _ := json.Marshal(map[string]any{
			"query": "UPDATE test SET name = ?",
			"args":  []any{"new"},
		})
		result, err := dispatchHostCall(ctx, host, "db_exec", args)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		var resp map[string]int64
		json.Unmarshal(result, &resp)
		if resp["affected"] != 1 {
			t.Errorf("expected 1 affected, got %d", resp["affected"])
		}
	})

	t.Run("cache_get_miss", func(t *testing.T) {
		args, _ := json.Marshal(map[string]string{"key": "nonexistent"})
		result, err := dispatchHostCall(ctx, host, "cache_get", args)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		var resp map[string]any
		json.Unmarshal(result, &resp)
		if resp["found"] != false {
			t.Error("expected found=false")
		}
	})

	t.Run("cache_set_and_get", func(t *testing.T) {
		// Set
		setArgs, _ := json.Marshal(map[string]any{
			"key":   "testkey",
			"value": "dGVzdHZhbHVl", // base64 of "testvalue"
			"ttl":   60,
		})
		_, err := dispatchHostCall(ctx, host, "cache_set", setArgs)
		if err != nil {
			t.Errorf("cache_set error: %v", err)
		}

		// Get
		getArgs, _ := json.Marshal(map[string]string{"key": "testkey"})
		result, err := dispatchHostCall(ctx, host, "cache_get", getArgs)
		if err != nil {
			t.Errorf("cache_get error: %v", err)
		}

		var resp map[string]any
		json.Unmarshal(result, &resp)
		if resp["found"] != true {
			t.Error("expected found=true after set")
		}
	})

	t.Run("http_request", func(t *testing.T) {
		args, _ := json.Marshal(map[string]any{
			"method":  "GET",
			"url":     "https://example.com",
			"headers": map[string]string{},
		})
		result, err := dispatchHostCall(ctx, host, "http_request", args)
		if err != nil {
			t.Errorf("http_request error: %v", err)
		}

		var resp map[string]any
		json.Unmarshal(result, &resp)
		if resp["status"].(float64) != 200 {
			t.Error("expected status 200")
		}
	})

	t.Run("send_email", func(t *testing.T) {
		args, _ := json.Marshal(map[string]any{
			"to":      "test@example.com",
			"subject": "Test Subject",
			"body":    "Test body",
			"html":    false,
		})
		_, err := dispatchHostCall(ctx, host, "send_email", args)
		if err != nil {
			t.Errorf("send_email error: %v", err)
		}
	})

	t.Run("translate", func(t *testing.T) {
		args, _ := json.Marshal(map[string]any{
			"key":  "hello.world",
			"args": []any{},
		})
		result, err := dispatchHostCall(ctx, host, "translate", args)
		if err != nil {
			t.Errorf("translate error: %v", err)
		}
		if result == nil {
			t.Error("expected result")
		}
	})

	t.Run("config_get", func(t *testing.T) {
		args, _ := json.Marshal(map[string]string{"key": "app.name"})
		result, err := dispatchHostCall(ctx, host, "config_get", args)
		if err != nil {
			t.Errorf("config_get error: %v", err)
		}

		var resp map[string]string
		json.Unmarshal(result, &resp)
		if resp["value"] != "GoatFlow" {
			t.Errorf("expected GoatFlow, got %s", resp["value"])
		}
	})

	t.Run("plugin_call", func(t *testing.T) {
		args, _ := json.Marshal(map[string]any{
			"plugin":   "other-plugin",
			"function": "test_func",
			"args":     json.RawMessage(`{}`),
		})
		result, err := dispatchHostCall(ctx, host, "plugin_call", args)
		if err != nil {
			t.Errorf("plugin_call error: %v", err)
		}

		// Result is the raw return from CallPlugin
		if result == nil {
			t.Error("expected result")
		}
	})

	t.Run("unknown_method", func(t *testing.T) {
		_, err := dispatchHostCall(ctx, host, "unknown_method", nil)
		if err == nil {
			t.Error("expected error for unknown method")
		}
	})
}

func TestHostAPIRPCServer_Call(t *testing.T) {
	host := newMockHostAPI()
	server := &HostAPIRPCServer{Host: host}

	t.Run("successful call", func(t *testing.T) {
		req := HostAPIRequest{
			Method:       "db_query",
			Args:         json.RawMessage(`{"query":"SELECT 1","args":[]}`),
			CallerPlugin: "test-plugin",
		}
		var resp HostAPIResponse

		err := server.Call(req, &resp)
		if err != nil {
			t.Errorf("Call returned error: %v", err)
		}
		if resp.Error != "" {
			t.Errorf("Response has error: %s", resp.Error)
		}
		if resp.Result == nil {
			t.Error("expected result")
		}
	})

	t.Run("call with error", func(t *testing.T) {
		req := HostAPIRequest{
			Method:       "unknown",
			Args:         nil,
			CallerPlugin: "test-plugin",
		}
		var resp HostAPIResponse

		err := server.Call(req, &resp)
		if err != nil {
			t.Errorf("Call should not return error: %v", err)
		}
		if resp.Error == "" {
			t.Error("expected error in response")
		}
	})

	t.Run("call without caller plugin", func(t *testing.T) {
		req := HostAPIRequest{
			Method: "log",
			Args:   json.RawMessage(`{"level":"info","message":"test","fields":{}}`),
		}
		var resp HostAPIResponse

		err := server.Call(req, &resp)
		if err != nil {
			t.Errorf("Call returned error: %v", err)
		}
	})
}

func TestHostAPIRPCClient(t *testing.T) {
	// Set up a real RPC server for the client to talk to
	host := newMockHostAPI()
	server := &HostAPIRPCServer{Host: host}

	rpcServer := rpc.NewServer()
	err := rpcServer.RegisterName("HostAPI", server)
	if err != nil {
		t.Fatalf("Failed to register RPC server: %v", err)
	}

	// Create pipe for communication
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	// Start server in background
	go rpcServer.ServeConn(serverConn)

	// Create client
	rpcClient := rpc.NewClient(clientConn)
	client := NewHostAPIRPCClient(rpcClient)

	t.Run("Call with valid method", func(t *testing.T) {
		result, err := client.Call("db_query", map[string]any{
			"query": "SELECT 1",
			"args":  []any{},
		})
		if err != nil {
			t.Errorf("Call error: %v", err)
		}
		if result == nil {
			t.Error("expected result")
		}
	})

	t.Run("Call with unknown method returns HostError", func(t *testing.T) {
		_, err := client.Call("unknown_method", nil)
		if err == nil {
			t.Error("expected error for unknown method")
		}
		hostErr, ok := err.(*HostError)
		if !ok {
			t.Errorf("expected HostError, got %T", err)
		}
		if hostErr != nil && hostErr.Message == "" {
			t.Error("expected non-empty error message")
		}
	})

	t.Run("DBQuery convenience method", func(t *testing.T) {
		rows, err := client.DBQuery("SELECT * FROM users WHERE id = ?", 1)
		if err != nil {
			t.Errorf("DBQuery error: %v", err)
		}
		if rows == nil {
			t.Error("expected rows")
		}
	})

	t.Run("DBExec convenience method", func(t *testing.T) {
		affected, err := client.DBExec("UPDATE users SET name = ? WHERE id = ?", "test", 1)
		if err != nil {
			t.Errorf("DBExec error: %v", err)
		}
		if affected != 1 {
			t.Errorf("expected 1 affected, got %d", affected)
		}
	})

	t.Run("CallPlugin convenience method", func(t *testing.T) {
		result, err := client.CallPlugin("other", "func", json.RawMessage(`{}`))
		if err != nil {
			t.Errorf("CallPlugin error: %v", err)
		}
		if result == nil {
			t.Error("expected result")
		}
	})
}

// Verify interface compliance
var _ plugin.HostAPI = (*mockHostAPI)(nil)

func TestHostError(t *testing.T) {
	err := &HostError{Message: "test error"}
	
	if err.Error() != "test error" {
		t.Errorf("expected 'test error', got %s", err.Error())
	}
}

func TestHostAPIRequestResponse(t *testing.T) {
	// Test request struct
	req := HostAPIRequest{
		Method:       "db_query",
		Args:         json.RawMessage(`{"query":"SELECT 1"}`),
		CallerPlugin: "test-plugin",
	}

	if req.Method != "db_query" {
		t.Error("method not set")
	}
	if req.CallerPlugin != "test-plugin" {
		t.Error("caller plugin not set")
	}

	// Test response struct
	resp := HostAPIResponse{
		Result: json.RawMessage(`{"rows":[]}`),
		Error:  "",
	}

	if resp.Error != "" {
		t.Error("error should be empty")
	}
	if resp.Result == nil {
		t.Error("result should not be nil")
	}

	// Test error response
	errResp := HostAPIResponse{
		Result: nil,
		Error:  "something went wrong",
	}

	if errResp.Error == "" {
		t.Error("error should not be empty")
	}
}

func TestDispatchHostCall_CacheSetVariants(t *testing.T) {
	host := newMockHostAPI()
	ctx := context.Background()

	t.Run("cache_set with different TTL", func(t *testing.T) {
		args, _ := json.Marshal(map[string]any{
			"key":   "ttl-test",
			"value": "dGVzdA==", // base64 of "test"
			"ttl":   3600,       // 1 hour
		})
		_, err := dispatchHostCall(ctx, host, "cache_set", args)
		if err != nil {
			t.Errorf("cache_set error: %v", err)
		}
	})

	t.Run("cache_get after set", func(t *testing.T) {
		// First set
		setArgs, _ := json.Marshal(map[string]any{
			"key":   "get-test",
			"value": "aGVsbG8=", // base64 of "hello"
			"ttl":   60,
		})
		dispatchHostCall(ctx, host, "cache_set", setArgs)

		// Then get
		getArgs, _ := json.Marshal(map[string]string{"key": "get-test"})
		result, err := dispatchHostCall(ctx, host, "cache_get", getArgs)
		if err != nil {
			t.Errorf("cache_get error: %v", err)
		}

		var resp map[string]any
		json.Unmarshal(result, &resp)
		if resp["found"] != true {
			t.Error("expected found=true")
		}
	})
}

func TestDispatchHostCall_HTTPRequestVariants(t *testing.T) {
	host := newMockHostAPI()
	ctx := context.Background()

	t.Run("POST request with body", func(t *testing.T) {
		args, _ := json.Marshal(map[string]any{
			"method":  "POST",
			"url":     "https://api.example.com/data",
			"headers": map[string]string{"Content-Type": "application/json"},
			"body":    "eyJkYXRhIjoidGVzdCJ9", // base64
		})
		result, err := dispatchHostCall(ctx, host, "http_request", args)
		if err != nil {
			t.Errorf("http_request error: %v", err)
		}

		var resp map[string]any
		json.Unmarshal(result, &resp)
		if resp["status"].(float64) != 200 {
			t.Error("expected status 200")
		}
	})
}

func TestDispatchHostCall_InvalidJSON(t *testing.T) {
	host := newMockHostAPI()
	ctx := context.Background()

	// Test with invalid JSON args
	_, err := dispatchHostCall(ctx, host, "db_query", []byte("not valid json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestCallRequest(t *testing.T) {
	req := CallRequest{
		Function: "hello",
		Args:     json.RawMessage(`{"name":"test"}`),
	}

	if req.Function != "hello" {
		t.Error("function not set")
	}
	if string(req.Args) != `{"name":"test"}` {
		t.Error("args not set correctly")
	}
}

func TestCallResponse(t *testing.T) {
	// Success response
	resp := CallResponse{
		Result: json.RawMessage(`{"message":"hello"}`),
		Error:  "",
	}
	if resp.Error != "" {
		t.Error("error should be empty")
	}

	// Error response
	errResp := CallResponse{
		Result: nil,
		Error:  "function not found",
	}
	if errResp.Error == "" {
		t.Error("error should not be empty")
	}
}

func TestInitRequest(t *testing.T) {
	req := InitRequest{
		Config:    map[string]string{"host_version": "0.7.0"},
		HostAPIID: 12345,
	}

	if req.Config["host_version"] != "0.7.0" {
		t.Error("config not set")
	}
	if req.HostAPIID != 12345 {
		t.Error("host API ID not set")
	}
}

func TestHandshakeConfig(t *testing.T) {
	if Handshake.ProtocolVersion != 1 {
		t.Error("expected protocol version 1")
	}
	if Handshake.MagicCookieKey != "GOATKIT_PLUGIN" {
		t.Error("expected magic cookie key GOATKIT_PLUGIN")
	}
	if Handshake.MagicCookieValue != "goatkit-v1" {
		t.Error("expected magic cookie value goatkit-v1")
	}
}

func TestPluginMap(t *testing.T) {
	if _, exists := PluginMap["gkplugin"]; !exists {
		t.Error("expected gkplugin in plugin map")
	}
}
