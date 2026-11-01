// Package wasm provides a WASM-based plugin runtime using wazero.
//
// WASM plugins are portable, sandboxed modules that communicate with the host
// via exported functions. This is the default/recommended plugin runtime.
//
// Plugin contract:
//   - Export gk_register() -> returns JSON manifest
//   - Export gk_call(fn_ptr, fn_len, args_ptr, args_len) -> returns response ptr
//   - Export gk_malloc(size) -> allocate memory for host-to-plugin data
//   - Export gk_free(ptr) -> free memory
//
// Host provides:
//   - gk_host_call(fn_ptr, fn_len, args_ptr, args_len) -> call host API
//   - gk_log(level, msg_ptr, msg_len) -> logging
package wasm

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/goatkit/goatflow/internal/platform/plugin"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// WASMPlugin implements plugin.Plugin for WASM modules.
type WASMPlugin struct {
	mu       sync.Mutex
	name     string
	manifest plugin.GKRegistration
	host     plugin.HostAPI

	runtime      wazero.Runtime
	compiled     wazero.CompiledModule
	moduleConfig wazero.ModuleConfig
	module       api.Module

	// Exported functions from the plugin
	gkRegister api.Function
	gkCall     api.Function
	gkMalloc   api.Function
	gkFree     api.Function

	// Resource limits
	callTimeout atomic.Int64 // time.Duration; 0 = none
}

// LoadOption is a functional option for loading WASM plugins.
type LoadOption func(*loadOptions)

type loadOptions struct {
	memoryLimitPages uint32        // Memory limit in pages (64KB each)
	callTimeout      time.Duration // Timeout for plugin function calls
}

func defaultLoadOptions() loadOptions {
	return loadOptions{
		memoryLimitPages: 256,              // 16MB default
		callTimeout:      30 * time.Second, // 30s default timeout
	}
}

// WithMemoryLimit sets the maximum memory in pages (64KB each).
// Default is 256 pages (16MB).
func WithMemoryLimit(pages uint32) LoadOption {
	return func(o *loadOptions) {
		o.memoryLimitPages = pages
	}
}

const (
	pagesPerMB     = 16    // 64KB pages
	maxMemoryPages = 65536 // wasm32 address space: 4GB
)

// WithMemoryLimitMB sets the maximum memory in megabytes, capped at the
// wasm32 maximum of 4GB. mb <= 0 keeps the default.
func WithMemoryLimitMB(mb int) LoadOption {
	return func(o *loadOptions) {
		if mb <= 0 {
			return
		}
		o.memoryLimitPages = maxMemoryPages
		if mb < maxMemoryPages/pagesPerMB {
			o.memoryLimitPages = uint32(mb) * pagesPerMB // #nosec G115 -- 0 < mb < 4096
		}
	}
}

// WithCallTimeout sets the timeout for plugin function calls.
// Default is 30 seconds.
func WithCallTimeout(d time.Duration) LoadOption {
	return func(o *loadOptions) {
		o.callTimeout = d
	}
}

// SetCallTimeout changes the deadline for later calls; 0 means none.
func (p *WASMPlugin) SetCallTimeout(d time.Duration) {
	p.callTimeout.Store(int64(d))
}

// LoadFromFile loads a WASM plugin from a file path.
func LoadFromFile(ctx context.Context, path string, opts ...LoadOption) (*WASMPlugin, error) {
	wasmBytes, err := os.ReadFile(path) // #nosec G304 -- .wasm path discovered under the host-configured plugin dir, not request input
	if err != nil {
		return nil, fmt.Errorf("read wasm file: %w", err)
	}
	return Load(ctx, wasmBytes, opts...)
}

// Load creates a WASM plugin from raw bytes.
func Load(ctx context.Context, wasmBytes []byte, opts ...LoadOption) (*WASMPlugin, error) {
	options := defaultLoadOptions()
	for _, opt := range opts {
		opt(&options)
	}

	// Memory limit, and calls interrupted when their context ends so a
	// looping guest cannot hold a host goroutine past its deadline.
	runtimeConfig := wazero.NewRuntimeConfig().
		WithMemoryLimitPages(options.memoryLimitPages).
		WithCloseOnContextDone(true)
	r := wazero.NewRuntimeWithConfig(ctx, runtimeConfig)

	// Instantiate WASI for plugins that need it (filesystem, env, etc.)
	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	p := &WASMPlugin{
		runtime: r,
		// wazero's default clocks are fake (walltime starts at 2022-01-01
		// and advances 1ms per read), so give the guest the real system
		// clocks or time.Now() inside a plugin is wrong.
		moduleConfig: wazero.NewModuleConfig().WithSysWalltime().WithSysNanotime(),
	}
	p.callTimeout.Store(int64(options.callTimeout))

	if err := p.load(ctx, wasmBytes); err != nil {
		_ = r.Close(ctx)
		return nil, err
	}
	return p, nil
}

// load compiles and instantiates the module and reads its manifest.
func (p *WASMPlugin) load(ctx context.Context, wasmBytes []byte) error {
	// Define host functions before compiling the module
	if err := p.defineHostFunctions(ctx); err != nil {
		return fmt.Errorf("define host functions: %w", err)
	}
	compiled, err := p.runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return fmt.Errorf("compile wasm: %w", err)
	}
	p.compiled = compiled
	if err := p.instantiate(ctx); err != nil {
		return err
	}

	manifest, err := p.callRegister(ctx)
	if err != nil {
		return fmt.Errorf("gk_register failed: %w", err)
	}
	p.manifest = manifest
	p.name = manifest.Name
	return nil
}

// instantiate starts a fresh instance of the compiled module and binds its
// exports.
func (p *WASMPlugin) instantiate(ctx context.Context) error {
	module, err := p.runtime.InstantiateModule(ctx, p.compiled, p.moduleConfig)
	if err != nil {
		return fmt.Errorf("instantiate wasm: %w", err)
	}
	gkRegister := module.ExportedFunction("gk_register")
	gkCall := module.ExportedFunction("gk_call")
	gkMalloc := module.ExportedFunction("gk_malloc")
	var missing string
	switch {
	case gkRegister == nil:
		missing = "gk_register"
	case gkCall == nil:
		missing = "gk_call"
	case gkMalloc == nil:
		missing = "gk_malloc"
	}
	if missing != "" {
		_ = module.Close(ctx)
		return fmt.Errorf("plugin missing %s export", missing)
	}
	p.module = module
	p.gkRegister = gkRegister
	p.gkCall = gkCall
	p.gkMalloc = gkMalloc
	p.gkFree = module.ExportedFunction("gk_free")
	return nil
}

// defineHostFunctions registers host API functions the plugin can call.
func (p *WASMPlugin) defineHostFunctions(ctx context.Context) error {
	_, err := p.runtime.NewHostModuleBuilder("gk").
		NewFunctionBuilder().
		WithFunc(p.hostCall).
		Export("host_call").
		NewFunctionBuilder().
		WithFunc(p.hostLog).
		Export("log").
		Instantiate(ctx)
	return err
}

// hostCall handles plugin calls to host API.
// Signature: host_call(fn_ptr, fn_len, args_ptr, args_len) -> result_ptr
func (p *WASMPlugin) hostCall(ctx context.Context, fnPtr, fnLen, argsPtr, argsLen uint32) uint64 {
	if p.host == nil {
		return 0
	}

	// Read function name from plugin memory
	fnName, ok := p.readString(fnPtr, fnLen)
	if !ok {
		return 0
	}

	// Read args from plugin memory
	args, ok := p.readBytes(argsPtr, argsLen)
	if !ok {
		return 0
	}

	// Set caller plugin name in context for better error messages
	ctx = context.WithValue(ctx, plugin.PluginCallerKey, p.name)

	// Dispatch through the shared dispatcher. p.host is the plugin's
	// SandboxedHostAPI (set by the Manager in Init), so WASM plugins get the
	// same methods, wire format and permission checks as gRPC plugins.
	result, err := plugin.DispatchHostCall(ctx, p.host, fnName, args)
	if err != nil {
		// Log with caller context for debugging
		p.host.Log(ctx, "error", fmt.Sprintf("host_call %s failed: %v", fnName, err), map[string]any{
			"caller": p.name,
		})
		return 0
	}

	// Write result to plugin memory
	return p.writeBytes(ctx, result)
}

// hostLog handles plugin logging.
// Signature: log(level, msg_ptr, msg_len)
func (p *WASMPlugin) hostLog(ctx context.Context, level uint32, msgPtr, msgLen uint32) {
	if p.host == nil {
		return
	}

	msg, ok := p.readString(msgPtr, msgLen)
	if !ok {
		return
	}

	levelStr := "info"
	switch level {
	case 0:
		levelStr = "debug"
	case 1:
		levelStr = "info"
	case 2:
		levelStr = "warn"
	case 3:
		levelStr = "error"
	}

	p.host.Log(ctx, levelStr, msg, map[string]any{"plugin": p.name})
}

// callRegister calls gk_register and parses the manifest.
func (p *WASMPlugin) callRegister(ctx context.Context) (plugin.GKRegistration, error) {
	results, err := p.gkRegister.Call(ctx)
	if err != nil {
		return plugin.GKRegistration{}, err
	}
	if len(results) == 0 {
		return plugin.GKRegistration{}, fmt.Errorf("gk_register returned no result")
	}

	// Result is a pointer to JSON string (ptr << 32 | len)
	packed := results[0]
	ptr := uint32(packed >> 32)
	length := uint32(packed & 0xFFFFFFFF)

	jsonBytes, ok := p.readBytes(ptr, length)
	if !ok {
		return plugin.GKRegistration{}, fmt.Errorf("failed to read manifest from memory")
	}

	var manifest plugin.GKRegistration
	if err := json.Unmarshal(jsonBytes, &manifest); err != nil {
		return plugin.GKRegistration{}, fmt.Errorf("parse manifest: %w", err)
	}

	return manifest, nil
}

// GKRegistration implements plugin.Plugin.
func (p *WASMPlugin) GKRegister() plugin.GKRegistration {
	return p.manifest
}

// Init implements plugin.Plugin.
func (p *WASMPlugin) Init(ctx context.Context, host plugin.HostAPI) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.host = host
	return nil
}

// callContext bounds ctx by the plugin's call timeout.
func (p *WASMPlugin) callContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if d := time.Duration(p.callTimeout.Load()); d > 0 {
		return context.WithTimeout(ctx, d)
	}
	return context.WithCancel(ctx)
}

// Call implements plugin.Plugin.
func (p *WASMPlugin) Call(ctx context.Context, fn string, args json.RawMessage) (json.RawMessage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ctx, cancel := p.callContext(ctx)
	defer cancel()

	// A call whose context ended was interrupted and wazero closed the
	// module. Start a fresh instance so the plugin keeps serving; only the
	// guest's in-memory state is lost (Init only hands over the host API,
	// which the new instance reaches through p.host).
	if p.module.IsClosed() {
		if err := p.instantiate(ctx); err != nil {
			return nil, fmt.Errorf("restart wasm module: %w", err)
		}
	}

	// Allocate and write function name
	fnPtr := p.writeString(ctx, fn)
	if fnPtr == 0 {
		return nil, fmt.Errorf("failed to allocate memory for function name")
	}
	defer p.free(ctx, uint32(fnPtr>>32))

	// Allocate and write args (nil/empty args is valid - pass 0, 0)
	var argsPtr uint64
	if len(args) > 0 {
		argsPtr = p.writeBytes(ctx, args)
		if argsPtr == 0 {
			return nil, fmt.Errorf("failed to allocate memory for args")
		}
		defer p.free(ctx, uint32(argsPtr>>32))
	}

	// Call gk_call(fn_ptr, fn_len, args_ptr, args_len)
	results, err := p.gkCall.Call(ctx,
		uint64(fnPtr>>32), uint64(fnPtr&0xFFFFFFFF),
		uint64(argsPtr>>32), uint64(argsPtr&0xFFFFFFFF),
	)
	if err != nil {
		return nil, fmt.Errorf("gk_call failed: %w", err)
	}
	if len(results) == 0 {
		return nil, nil
	}

	// Read result
	packed := results[0]
	ptr := uint32(packed >> 32)
	length := uint32(packed & 0xFFFFFFFF)

	if ptr == 0 || length == 0 {
		return nil, nil
	}

	result, ok := p.readBytes(ptr, length)
	if !ok {
		return nil, fmt.Errorf("failed to read result from memory")
	}

	// Free result memory
	p.free(ctx, ptr)

	return result, nil
}

// Shutdown implements plugin.Plugin.
func (p *WASMPlugin) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var moduleErr error
	if p.module != nil {
		moduleErr = p.module.Close(ctx)
	}
	if p.runtime != nil {
		if err := p.runtime.Close(ctx); err != nil {
			return err
		}
	}
	return moduleErr
}

// Memory helpers

func (p *WASMPlugin) readString(ptr, length uint32) (string, bool) {
	bytes, ok := p.readBytes(ptr, length)
	if !ok {
		return "", false
	}
	return string(bytes), true
}

func (p *WASMPlugin) readBytes(ptr, length uint32) ([]byte, bool) {
	if p.module == nil {
		return nil, false
	}
	mem := p.module.Memory()
	if mem == nil {
		return nil, false
	}
	return mem.Read(ptr, length)
}

func (p *WASMPlugin) writeString(ctx context.Context, s string) uint64 {
	return p.writeBytes(ctx, []byte(s))
}

func (p *WASMPlugin) writeBytes(ctx context.Context, data []byte) uint64 {
	if len(data) == 0 {
		return 0
	}

	// Allocate memory in plugin
	results, err := p.gkMalloc.Call(ctx, uint64(len(data)))
	if err != nil || len(results) == 0 {
		return 0
	}
	// wasm32 pointers are 32-bit; reject anything a misbehaving guest
	// returns outside that range instead of silently truncating it.
	raw := results[0]
	if raw > math.MaxUint32 {
		return 0
	}
	ptr := uint32(raw)
	if ptr == 0 {
		return 0
	}

	// Write data
	mem := p.module.Memory()
	if mem == nil {
		return 0
	}
	if !mem.Write(ptr, data) {
		return 0
	}

	// Return packed ptr|len
	return (uint64(ptr) << 32) | uint64(len(data))
}

func (p *WASMPlugin) free(ctx context.Context, ptr uint32) {
	if p.gkFree != nil && ptr != 0 {
		// Best-effort: a failing guest free only leaks guest memory.
		_, _ = p.gkFree.Call(ctx, uint64(ptr))
	}
}
