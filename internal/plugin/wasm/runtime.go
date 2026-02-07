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
	"os"
	"sync"
	"time"

	"github.com/goatkit/goatflow/internal/plugin"
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

	runtime wazero.Runtime
	module  api.Module

	// Exported functions from the plugin
	gkRegister api.Function
	gkCall     api.Function
	gkMalloc   api.Function
	gkFree     api.Function

	// Resource limits
	callTimeout time.Duration
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

// WithCallTimeout sets the timeout for plugin function calls.
// Default is 30 seconds.
func WithCallTimeout(d time.Duration) LoadOption {
	return func(o *loadOptions) {
		o.callTimeout = d
	}
}

// LoadFromFile loads a WASM plugin from a file path.
func LoadFromFile(ctx context.Context, path string, opts ...LoadOption) (*WASMPlugin, error) {
	wasmBytes, err := os.ReadFile(path)
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

	// Create runtime with memory limits
	runtimeConfig := wazero.NewRuntimeConfig().
		WithMemoryLimitPages(options.memoryLimitPages)
	r := wazero.NewRuntimeWithConfig(ctx, runtimeConfig)

	// Instantiate WASI for plugins that need it (filesystem, env, etc.)
	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	p := &WASMPlugin{
		runtime:     r,
		callTimeout: options.callTimeout,
	}

	// Define host functions before compiling the module
	if err := p.defineHostFunctions(ctx); err != nil {
		r.Close(ctx)
		return nil, fmt.Errorf("define host functions: %w", err)
	}

	// Compile and instantiate the module
	module, err := r.Instantiate(ctx, wasmBytes)
	if err != nil {
		r.Close(ctx)
		return nil, fmt.Errorf("instantiate wasm: %w", err)
	}
	p.module = module

	// Get required exports
	p.gkRegister = module.ExportedFunction("gk_register")
	p.gkCall = module.ExportedFunction("gk_call")
	p.gkMalloc = module.ExportedFunction("gk_malloc")
	p.gkFree = module.ExportedFunction("gk_free")

	if p.gkRegister == nil {
		return nil, fmt.Errorf("plugin missing gk_register export")
	}
	if p.gkCall == nil {
		return nil, fmt.Errorf("plugin missing gk_call export")
	}
	if p.gkMalloc == nil {
		return nil, fmt.Errorf("plugin missing gk_malloc export")
	}

	// Call gk_register to get manifest
	manifest, err := p.callRegister(ctx)
	if err != nil {
		r.Close(ctx)
		return nil, fmt.Errorf("gk_register failed: %w", err)
	}
	p.manifest = manifest
	p.name = manifest.Name

	return p, nil
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

	// Dispatch to host API
	result, err := p.dispatchHostCall(ctx, fnName, args)
	if err != nil {
		// Log with caller context for debugging
		p.host.Log(ctx, "error", fmt.Sprintf("host_call %s failed: %v", fnName, err), map[string]any{
			"caller": p.name,
		})
		return 0
	}

	// Write result to plugin memory
	return p.writeBytes(result)
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

// dispatchHostCall routes host API calls to the appropriate method.
func (p *WASMPlugin) dispatchHostCall(ctx context.Context, fn string, args []byte) ([]byte, error) {
	switch fn {
	case "db_query":
		var req struct {
			Query string `json:"query"`
			Args  []any  `json:"args"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		rows, err := p.host.DBQuery(ctx, req.Query, req.Args...)
		if err != nil {
			return nil, err
		}
		return json.Marshal(rows)

	case "db_exec":
		var req struct {
			Query string `json:"query"`
			Args  []any  `json:"args"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		affected, err := p.host.DBExec(ctx, req.Query, req.Args...)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]int64{"affected": affected})

	case "cache_get":
		var req struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		val, found, err := p.host.CacheGet(ctx, req.Key)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"value": val, "found": found})

	case "cache_set":
		var req struct {
			Key   string `json:"key"`
			Value []byte `json:"value"`
			TTL   int    `json:"ttl"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		err := p.host.CacheSet(ctx, req.Key, req.Value, req.TTL)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]bool{"ok": true})

	case "http_request":
		var req struct {
			Method  string            `json:"method"`
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
			Body    []byte            `json:"body"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		status, body, err := p.host.HTTPRequest(ctx, req.Method, req.URL, req.Headers, req.Body)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"status": status, "body": body})

	case "send_email":
		var req struct {
			To      string `json:"to"`
			Subject string `json:"subject"`
			Body    string `json:"body"`
			HTML    bool   `json:"html"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		err := p.host.SendEmail(ctx, req.To, req.Subject, req.Body, req.HTML)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]bool{"ok": true})

	case "config_get":
		var req struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		val, err := p.host.ConfigGet(ctx, req.Key)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"value": val})

	case "translate":
		var req struct {
			Key  string `json:"key"`
			Args []any  `json:"args"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		val := p.host.Translate(ctx, req.Key, req.Args...)
		return json.Marshal(map[string]string{"value": val})

	case "plugin_call":
		var req struct {
			Plugin   string          `json:"plugin"`
			Function string          `json:"function"`
			Args     json.RawMessage `json:"args"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		result, err := p.host.CallPlugin(ctx, req.Plugin, req.Function, req.Args)
		if err != nil {
			return nil, err
		}
		return result, nil

	default:
		return nil, fmt.Errorf("unknown host function: %s", fn)
	}
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

// Call implements plugin.Plugin.
func (p *WASMPlugin) Call(ctx context.Context, fn string, args json.RawMessage) (json.RawMessage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Apply timeout if configured and not already set on context
	if p.callTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.callTimeout)
		defer cancel()
	}

	// Allocate and write function name
	fnPtr := p.writeString(fn)
	if fnPtr == 0 {
		return nil, fmt.Errorf("failed to allocate memory for function name")
	}
	defer p.free(uint32(fnPtr >> 32))

	// Allocate and write args (nil/empty args is valid - pass 0, 0)
	var argsPtr uint64
	if len(args) > 0 {
		argsPtr = p.writeBytes(args)
		if argsPtr == 0 {
			return nil, fmt.Errorf("failed to allocate memory for args")
		}
		defer p.free(uint32(argsPtr >> 32))
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
	p.free(ptr)

	return result, nil
}

// Shutdown implements plugin.Plugin.
func (p *WASMPlugin) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.module != nil {
		p.module.Close(ctx)
	}
	if p.runtime != nil {
		return p.runtime.Close(ctx)
	}
	return nil
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

func (p *WASMPlugin) writeString(s string) uint64 {
	return p.writeBytes([]byte(s))
}

func (p *WASMPlugin) writeBytes(data []byte) uint64 {
	if len(data) == 0 {
		return 0
	}

	// Allocate memory in plugin
	results, err := p.gkMalloc.Call(context.Background(), uint64(len(data)))
	if err != nil || len(results) == 0 {
		return 0
	}
	ptr := uint32(results[0])
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

func (p *WASMPlugin) free(ptr uint32) {
	if p.gkFree != nil && ptr != 0 {
		p.gkFree.Call(context.Background(), uint64(ptr))
	}
}
