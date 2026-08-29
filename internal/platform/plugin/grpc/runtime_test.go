package grpc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/platform/plugin"
	grpcplugin "github.com/goatkit/goatflow/internal/platform/plugin/grpc"
)

// mockHostAPI for integration tests
type mockHostAPI struct{}

func (m *mockHostAPI) DBQuery(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	return []map[string]any{{"id": 1}}, nil
}
func (m *mockHostAPI) DBExec(ctx context.Context, query string, args ...any) (int64, error) {
	return 1, nil
}
func (m *mockHostAPI) CacheGet(ctx context.Context, key string) ([]byte, bool, error) {
	return nil, false, nil
}
func (m *mockHostAPI) CacheSet(ctx context.Context, key string, value []byte, ttlSeconds int) error {
	return nil
}
func (m *mockHostAPI) CacheDelete(ctx context.Context, key string) error { return nil }
func (m *mockHostAPI) HTTPRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) (int, []byte, error) {
	return 200, []byte(`{"ok":true}`), nil
}
func (m *mockHostAPI) SendEmail(ctx context.Context, to, subject, body string, html bool) error {
	return nil
}
func (m *mockHostAPI) Log(ctx context.Context, level, message string, fields map[string]any) {}
func (m *mockHostAPI) ConfigGet(ctx context.Context, key string) (string, error) {
	return "", nil
}
func (m *mockHostAPI) Translate(ctx context.Context, key string, args ...any) string { return key }
func (m *mockHostAPI) CallPlugin(ctx context.Context, pluginName, function string, args json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(map[string]string{"result": "ok"})
}

func (m *mockHostAPI) PublishEvent(ctx context.Context, channel string, eventType string, data string) error {
	return nil
}

func (m *mockHostAPI) EntitySoftDelete(ctx context.Context, entityType string, entityID int64, reason string) error {
	return nil
}

func (m *mockHostAPI) EntityRestore(ctx context.Context, entityType string, entityID int64) error {
	return nil
}

func (m *mockHostAPI) EntityHardDelete(ctx context.Context, entityType string, entityID int64, reason string) error {
	return nil
}

func (m *mockHostAPI) RecycleBinList(ctx context.Context, entityType string) (json.RawMessage, error) {
	return nil, nil
}

func (m *mockHostAPI) SecureConfigGet(ctx context.Context, key string) (string, error) {
	return "", nil
}

func (m *mockHostAPI) SecureConfigSet(ctx context.Context, key string, value string) error {
	return nil
}

func (m *mockHostAPI) OrgID(ctx context.Context) int64 {
	return 0
}

func (m *mockHostAPI) CustomFieldsGet(ctx context.Context, entityType string, objectID int64, fields []string) (map[string]any, error) {
	return nil, nil
}

func (m *mockHostAPI) CustomFieldsSet(ctx context.Context, entityType string, objectID int64, values map[string]any) error {
	return nil
}

func (m *mockHostAPI) CustomFieldsQuery(ctx context.Context, entityType string, filters []plugin.CustomFieldFilter) ([]int64, error) {
	return nil, nil
}

func buildGRPCPlugin(t *testing.T) string {
	t.Helper()

	// Find repo root
	_, filename, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..")

	// Build to temp location
	pluginPath := filepath.Join(t.TempDir(), "hello-grpc-plugin")
	if runtime.GOOS == "windows" {
		pluginPath += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", pluginPath, "./internal/platform/plugin/grpc/example")
	cmd.Dir = repoRoot
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build gRPC plugin: %v\n%s", err, output)
	}

	return pluginPath
}

func TestLoadGRPCPlugin(t *testing.T) {
	pluginPath := buildGRPCPlugin(t)
	host := &mockHostAPI{}

	t.Run("load and register", func(t *testing.T) {
		p, err := grpcplugin.LoadGRPCPlugin(pluginPath, "test-plugin", host, plugin.DefaultResourcePolicy("test-plugin"))
		if err != nil {
			t.Fatalf("LoadGRPCPlugin failed: %v", err)
		}
		defer p.Shutdown(context.Background())

		reg := p.GKRegister()
		if reg.Name != "hello-grpc" {
			t.Errorf("expected name 'hello-grpc', got %q", reg.Name)
		}
		if reg.Version != "1.0.0" {
			t.Errorf("expected version '1.0.0', got %q", reg.Version)
		}
	})

	t.Run("init", func(t *testing.T) {
		p, err := grpcplugin.LoadGRPCPlugin(pluginPath, "test-plugin", host, plugin.DefaultResourcePolicy("test-plugin"))
		if err != nil {
			t.Fatalf("LoadGRPCPlugin failed: %v", err)
		}
		defer p.Shutdown(context.Background())

		ctx := context.Background()
		err = p.Init(ctx, host)
		if err != nil {
			t.Errorf("Init failed: %v", err)
		}
	})

	t.Run("call function", func(t *testing.T) {
		p, err := grpcplugin.LoadGRPCPlugin(pluginPath, "test-plugin", host, plugin.DefaultResourcePolicy("test-plugin"))
		if err != nil {
			t.Fatalf("LoadGRPCPlugin failed: %v", err)
		}
		defer p.Shutdown(context.Background())

		ctx := context.Background()
		p.Init(ctx, host)

		result, err := p.Call(ctx, "get_status", nil)
		if err != nil {
			t.Fatalf("Call failed: %v", err)
		}

		var resp map[string]any
		if err := json.Unmarshal(result, &resp); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if resp["status"] != "running" {
			t.Errorf("expected status 'running', got %v", resp["status"])
		}
	})

	t.Run("call render_widget", func(t *testing.T) {
		p, err := grpcplugin.LoadGRPCPlugin(pluginPath, "test-plugin", host, plugin.DefaultResourcePolicy("test-plugin"))
		if err != nil {
			t.Fatalf("LoadGRPCPlugin failed: %v", err)
		}
		defer p.Shutdown(context.Background())

		ctx := context.Background()
		p.Init(ctx, host)

		result, err := p.Call(ctx, "render_widget", nil)
		if err != nil {
			t.Fatalf("Call failed: %v", err)
		}

		var resp map[string]string
		json.Unmarshal(result, &resp)

		if resp["html"] == "" {
			t.Error("expected html in response")
		}
	})

	t.Run("call unknown function", func(t *testing.T) {
		p, err := grpcplugin.LoadGRPCPlugin(pluginPath, "test-plugin", host, plugin.DefaultResourcePolicy("test-plugin"))
		if err != nil {
			t.Fatalf("LoadGRPCPlugin failed: %v", err)
		}
		defer p.Shutdown(context.Background())

		ctx := context.Background()
		_, err = p.Call(ctx, "nonexistent", nil)
		if err == nil {
			t.Error("expected error for unknown function")
		}
	})

	t.Run("shutdown", func(t *testing.T) {
		p, err := grpcplugin.LoadGRPCPlugin(pluginPath, "test-plugin", host, plugin.DefaultResourcePolicy("test-plugin"))
		if err != nil {
			t.Fatalf("LoadGRPCPlugin failed: %v", err)
		}

		ctx := context.Background()
		err = p.Shutdown(ctx)
		if err != nil {
			t.Errorf("Shutdown failed: %v", err)
		}
	})
}

func TestLoadGRPCPlugin_InvalidPath(t *testing.T) {
	host := &mockHostAPI{}

	_, err := grpcplugin.LoadGRPCPlugin("/nonexistent/plugin", "test-plugin", host, plugin.DefaultResourcePolicy("test-plugin"))
	if err == nil {
		t.Error("expected error for nonexistent plugin")
	}
}

func TestLoadGRPCPlugin_NotExecutable(t *testing.T) {
	host := &mockHostAPI{}

	// Create a non-executable file
	tmpFile := filepath.Join(t.TempDir(), "not-a-plugin")
	os.WriteFile(tmpFile, []byte("not executable"), 0644)

	_, err := grpcplugin.LoadGRPCPlugin(tmpFile, "test-plugin", host, plugin.DefaultResourcePolicy("test-plugin"))
	if err == nil {
		t.Error("expected error for non-executable file")
	}
}

func TestGRPCPluginWithManager(t *testing.T) {
	pluginPath := buildGRPCPlugin(t)
	host := &mockHostAPI{}
	mgr := plugin.NewManager(host)

	ctx := context.Background()

	p, err := grpcplugin.LoadGRPCPlugin(pluginPath, "test-plugin", host, plugin.DefaultResourcePolicy("test-plugin"))
	if err != nil {
		t.Fatalf("LoadGRPCPlugin failed: %v", err)
	}

	// Register with manager
	if err := mgr.Register(ctx, p); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// List should include the plugin
	plugins := mgr.List()
	found := false
	for _, info := range plugins {
		if info.Name == "hello-grpc" {
			found = true
			break
		}
	}
	if !found {
		t.Error("plugin not found in manager list")
	}

	// Call via manager
	result, err := mgr.Call(ctx, "hello-grpc", "get_status", nil)
	if err != nil {
		t.Fatalf("Call via manager failed: %v", err)
	}

	var resp map[string]any
	json.Unmarshal(result, &resp)
	if resp["type"] != "grpc" {
		t.Errorf("expected type 'grpc', got %v", resp["type"])
	}

	// Cleanup
	mgr.ShutdownAll(ctx)
}

func TestGRPCPluginConcurrent(t *testing.T) {
	pluginPath := buildGRPCPlugin(t)
	host := &mockHostAPI{}

	p, err := grpcplugin.LoadGRPCPlugin(pluginPath, "test-plugin", host, plugin.DefaultResourcePolicy("test-plugin"))
	if err != nil {
		t.Fatalf("LoadGRPCPlugin failed: %v", err)
	}
	defer p.Shutdown(context.Background())

	ctx := context.Background()
	p.Init(ctx, host)

	// Concurrent calls
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := p.Call(ctx, "get_status", nil)
			if err != nil {
				t.Errorf("concurrent call failed: %v", err)
			}
			done <- true
		}()
	}

	// Wait with timeout
	timeout := time.After(5 * time.Second)
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-timeout:
			t.Fatal("timeout waiting for concurrent calls")
		}
	}
}

// File storage stubs
func (m *mockHostAPI) CreateArticleAttachment(ctx context.Context, articleID, createdBy int64, filename, contentType string, content []byte) (int64, error) {
	return 0, fmt.Errorf("not implemented")
}
func (m *mockHostAPI) ListArticleAttachments(ctx context.Context, articleID int64) ([]plugin.ArticleAttachment, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockHostAPI) DeleteArticleAttachment(ctx context.Context, articleID, attachmentID int64) error {
	return fmt.Errorf("not implemented")
}
func (m *mockHostAPI) StoreFile(ctx context.Context, key string, data []byte, metadata map[string]string) error {
	return nil
}
func (m *mockHostAPI) GetFile(ctx context.Context, key string) ([]byte, map[string]string, error) {
	return nil, nil, nil
}
func (m *mockHostAPI) DeleteFile(ctx context.Context, key string) error { return nil }
func (m *mockHostAPI) ListFiles(ctx context.Context, prefix string) ([]plugin.FileInfo, error) {
	return nil, nil
}

func (m *mockHostAPI) GenerateThumbnail(_ context.Context, _ []byte, _ string, _, _ int) ([]byte, string, error) {
	return nil, "", fmt.Errorf("not implemented")
}
