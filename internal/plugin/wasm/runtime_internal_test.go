package wasm

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/goatkit/goatflow/internal/plugin"
)

// mockHostAPIForUnit is a simple mock for unit testing internal functions
type mockHostAPIForUnit struct{}

func (m *mockHostAPIForUnit) DBQuery(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	return nil, nil
}
func (m *mockHostAPIForUnit) DBExec(ctx context.Context, query string, args ...any) (int64, error) {
	return 0, nil
}
func (m *mockHostAPIForUnit) CacheGet(ctx context.Context, key string) ([]byte, bool, error) {
	return nil, false, nil
}
func (m *mockHostAPIForUnit) CacheSet(ctx context.Context, key string, value []byte, ttlSeconds int) error {
	return nil
}
func (m *mockHostAPIForUnit) CacheDelete(ctx context.Context, key string) error {
	return nil
}
func (m *mockHostAPIForUnit) HTTPRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) (int, []byte, error) {
	return 200, nil, nil
}
func (m *mockHostAPIForUnit) SendEmail(ctx context.Context, to, subject, body string, html bool) error {
	return nil
}
func (m *mockHostAPIForUnit) Log(ctx context.Context, level, message string, fields map[string]any) {}
func (m *mockHostAPIForUnit) ConfigGet(ctx context.Context, key string) (string, error) {
	return "", nil
}
func (m *mockHostAPIForUnit) Translate(ctx context.Context, key string, args ...any) string {
	return ""
}
func (m *mockHostAPIForUnit) CallPlugin(ctx context.Context, pluginName, function string, args json.RawMessage) (json.RawMessage, error) {
	return nil, nil
}

func TestHostCallWithNilHost(t *testing.T) {
	p := &WASMPlugin{
		host: nil, // nil host
	}

	result := p.hostCall(context.Background(), 0, 0, 0, 0)
	if result != 0 {
		t.Errorf("expected 0 for nil host, got %d", result)
	}
}

func TestHostLogWithNilHost(t *testing.T) {
	p := &WASMPlugin{
		host: nil, // nil host
	}

	// Should not panic
	p.hostLog(context.Background(), 1, 0, 0)
}

func TestReadBytesWithNilModule(t *testing.T) {
	p := &WASMPlugin{
		module: nil, // nil module
	}

	bytes, ok := p.readBytes(100, 10)
	if ok {
		t.Error("expected ok=false for nil module")
	}
	if bytes != nil {
		t.Error("expected nil bytes for nil module")
	}
}

func TestReadStringWithNilModule(t *testing.T) {
	p := &WASMPlugin{
		module: nil,
	}

	s, ok := p.readString(100, 10)
	if ok {
		t.Error("expected ok=false for nil module")
	}
	if s != "" {
		t.Error("expected empty string for nil module")
	}
}

func TestWriteBytesEmpty(t *testing.T) {
	p := &WASMPlugin{
		module: nil,
	}

	// Empty data should return 0
	result := p.writeBytes(nil)
	if result != 0 {
		t.Errorf("expected 0 for empty data, got %d", result)
	}

	result = p.writeBytes([]byte{})
	if result != 0 {
		t.Errorf("expected 0 for empty slice, got %d", result)
	}
}

func TestWriteStringEmpty(t *testing.T) {
	p := &WASMPlugin{
		module: nil,
	}

	result := p.writeString("")
	if result != 0 {
		t.Errorf("expected 0 for empty string, got %d", result)
	}
}

func TestFreeWithNilFunction(t *testing.T) {
	p := &WASMPlugin{
		gkFree: nil,
	}

	// Should not panic
	p.free(0)
	p.free(100)
}

func TestDispatchHostCallUnknown(t *testing.T) {
	host := &mockHostAPIForUnit{}
	p := &WASMPlugin{
		host: host,
		name: "test-plugin",
	}

	ctx := context.Background()
	_, err := p.dispatchHostCall(ctx, "unknown_function", nil)
	if err == nil {
		t.Error("expected error for unknown function")
	}
}

func TestDispatchHostCallInvalidJSON(t *testing.T) {
	host := &mockHostAPIForUnit{}
	p := &WASMPlugin{
		host: host,
		name: "test-plugin",
	}

	ctx := context.Background()

	// Test each method with invalid JSON
	methods := []string{
		"db_query", "db_exec", "cache_get", "cache_set",
		"http_request", "send_email", "config_get", "translate", "plugin_call",
	}

	for _, method := range methods {
		_, err := p.dispatchHostCall(ctx, method, []byte("not valid json"))
		if err == nil {
			t.Errorf("expected error for %s with invalid JSON", method)
		}
	}
}

func TestWASMPluginRegistration(t *testing.T) {
	// GKRegister returns the cached manifest
	p := &WASMPlugin{
		name: "test-plugin",
		manifest: plugin.GKRegistration{
			Name:    "test-plugin",
			Version: "1.0.0",
		},
	}

	reg := p.GKRegister()
	if reg.Name != "test-plugin" {
		t.Errorf("expected name 'test-plugin', got %s", reg.Name)
	}
	if reg.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", reg.Version)
	}
}

func TestWithMemoryLimit(t *testing.T) {
	opts := defaultLoadOptions()
	WithMemoryLimit(256)(&opts) // 256 pages

	if opts.memoryLimitPages != 256 {
		t.Errorf("expected memoryLimitPages 256, got %d", opts.memoryLimitPages)
	}
}

func TestWithCallTimeout(t *testing.T) {
	opts := defaultLoadOptions()
	WithCallTimeout(60 * 1000000000)(&opts) // 60 seconds in nanoseconds

	if opts.callTimeout != 60*1000000000 {
		t.Errorf("expected callTimeout 60s, got %d", opts.callTimeout)
	}
}

var _ plugin.HostAPI = (*mockHostAPIForUnit)(nil)
