package wasm_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/goatkit/goatflow/internal/platform/plugin/wasm"
	"github.com/goatkit/goatflow/internal/platform/plugin/wasm/wasmtest"
)

const limitsManifest = `{"name":"limits","version":"1.0.0"}`

func loadLimits(t *testing.T, growPages uint32, opts ...wasm.LoadOption) *wasm.WASMPlugin {
	t.Helper()
	ctx := context.Background()
	p, err := wasm.Load(ctx, wasmtest.Module(limitsManifest, growPages), opts...)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	t.Cleanup(func() { _ = p.Shutdown(ctx) })
	if err := p.Init(ctx, &mockHostAPI{}); err != nil {
		t.Fatal(err)
	}
	return p
}

func call(t *testing.T, p *wasm.WASMPlugin, fn string) string {
	t.Helper()
	out, err := p.Call(context.Background(), fn, nil)
	if err != nil {
		t.Fatalf("%s: %v", fn, err)
	}
	return string(out)
}

// TestMemoryLimitMB: a module cannot grow its memory past the limit in MB;
// 64 pages are 4MB.
func TestMemoryLimitMB(t *testing.T) {
	if got := call(t, loadLimits(t, 64, wasm.WithMemoryLimitMB(1)), "grow"); got != "false" {
		t.Errorf("growing 4MB under a 1MB limit = %s, want false", got)
	}
	if got := call(t, loadLimits(t, 64, wasm.WithMemoryLimitMB(8)), "grow"); got != "true" {
		t.Errorf("growing 4MB under an 8MB limit = %s, want true", got)
	}
	// Beyond the wasm32 maximum the limit is capped rather than refused.
	if got := call(t, loadLimits(t, 64, wasm.WithMemoryLimitMB(1<<20)), "grow"); got != "true" {
		t.Errorf("growing 4MB under a capped limit = %s, want true", got)
	}
}

// TestCallTimeoutInterruptsBusyLoop: a guest that never returns is stopped
// at its deadline, and the plugin answers the next call.
func TestCallTimeoutInterruptsBusyLoop(t *testing.T) {
	p := loadLimits(t, 1, wasm.WithCallTimeout(100*time.Millisecond))

	start := time.Now()
	_, err := p.Call(context.Background(), "spin", nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("spin error = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("spin returned after %s", elapsed)
	}

	if got := call(t, p, "hello"); got != `"ok"` {
		t.Errorf("call after interrupted one = %s, want \"ok\"", got)
	}

	p.SetCallTimeout(50 * time.Millisecond)
	if _, err := p.Call(context.Background(), "spin", nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("spin after SetCallTimeout error = %v", err)
	}
	if got := call(t, p, "hello"); got != `"ok"` {
		t.Errorf("second recovery = %s", got)
	}
}

// TestCallerCancelInterruptsBusyLoop: the caller's own cancellation also
// stops a looping guest.
func TestCallerCancelInterruptsBusyLoop(t *testing.T) {
	p := loadLimits(t, 1, wasm.WithCallTimeout(0))
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	if _, err := p.Call(ctx, "spin", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("spin error = %v, want canceled", err)
	}
	if got := call(t, p, "hello"); got != `"ok"` {
		t.Errorf("call after cancelled one = %s", got)
	}
}

// TestShutdownPluginIsNotRestarted: once shut down, a plugin refuses calls
// rather than starting a new instance.
func TestShutdownPluginIsNotRestarted(t *testing.T) {
	p := loadLimits(t, 1)
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Call(context.Background(), "hello", nil); err == nil {
		t.Fatal("call after shutdown succeeded")
	}
}
