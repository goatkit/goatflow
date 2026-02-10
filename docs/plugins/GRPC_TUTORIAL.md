# gRPC Plugin Tutorial

Build a native Go plugin using HashiCorp go-plugin for maximum flexibility.

## Why gRPC Plugins?

gRPC plugins run as separate processes, giving you:

- **Full Go stdlib** — All packages, goroutines, channels
- **Process isolation** — Plugin crashes don't affect core
- **Hot reload** — Binary changes trigger automatic reload via fsnotify
- **Bidirectional RPC** — Plugins can call back to the HostAPI
- **Easy debugging** — Standard Go debuggers work

Trade-off: Slightly higher latency than WASM (~1ms per call).

**Important:** Despite the name "gRPC", the actual wire protocol is HashiCorp go-plugin with **net/rpc** — no protoc, proto files, or gRPC wire protocol needed.

## Prerequisites

- Go 1.21+
- GoatFlow running locally

## Step 1: Scaffold the Plugin

```bash
gk init my-plugin --type grpc
cd my-plugin
```

Or create the directory structure manually:

```
plugins/my-plugin/
├── plugin.yaml    # Plugin manifest (required)
├── main.go        # Entry point
└── build.sh       # Build script
```

## Step 2: Create plugin.yaml

Every gRPC plugin needs a `plugin.yaml` in its directory:

```yaml
name: my-plugin
version: "1.0.0"
runtime: grpc
binary: my-plugin
resources:
  memory_mb: 512
  call_timeout: 30s
  permissions:
    - type: db
      access: read
    - type: cache
      access: readwrite
    - type: http
      scope: ["api.example.com"]
```

Fields:
- `name` — Unique plugin identifier
- `version` — Semver version string
- `runtime` — Must be `grpc`
- `binary` — Path to executable (relative to plugin directory)
- `resources` — Requested resource limits and permissions

## Step 3: Implement the Plugin

Create `main.go`:

```go
package main

import (
	"encoding/json"
	"fmt"

	"github.com/goatkit/goatflow/pkg/plugin"
	"github.com/goatkit/goatflow/pkg/plugin/grpcutil"
)

type MyPlugin struct {
	config map[string]string
}

// GKRegister returns plugin metadata and capabilities.
func (p *MyPlugin) GKRegister() (*plugin.GKRegistration, error) {
	return &plugin.GKRegistration{
		Name:        "my-plugin",
		Version:     "1.0.0",
		Description: "My custom gRPC plugin",
		Author:      "Your Name",
		License:     "Apache-2.0",

		Routes: []plugin.RouteSpec{
			{
				Method:      "GET",
				Path:        "/api/plugins/my-plugin/status",
				Handler:     "get_status",
				Middleware:  []string{"auth"},
				Description: "Get plugin status",
			},
		},

		Widgets: []plugin.WidgetSpec{
			{
				ID:       "my-widget",
				Title:    "My Widget",
				Handler:  "render_widget",
				Location: "agent_home",
				Size:     "medium",
			},
		},
	}, nil
}

// Init is called once when the plugin is loaded.
func (p *MyPlugin) Init(config map[string]string) error {
	p.config = config
	fmt.Println("[my-plugin] Initialized")
	return nil
}

// Call handles function calls from the host.
func (p *MyPlugin) Call(fn string, args json.RawMessage) (json.RawMessage, error) {
	switch fn {
	case "get_status":
		return json.Marshal(map[string]any{
			"status":  "running",
			"version": "1.0.0",
		})

	case "render_widget":
		html := `<div class="text-center p-4">
			<h3>🔌 My Plugin</h3>
			<p>Running as a native Go process.</p>
		</div>`
		return json.Marshal(map[string]string{"html": html})

	default:
		return nil, fmt.Errorf("unknown function: %s", fn)
	}
}

// Shutdown is called before the plugin is unloaded.
func (p *MyPlugin) Shutdown() error {
	fmt.Println("[my-plugin] Shutting down")
	return nil
}

func main() {
	grpcutil.ServePlugin(&MyPlugin{})
}
```

### Key Points

- Import `pkg/plugin` for types (`GKRegistration`, etc.)
- Import `pkg/plugin/grpcutil` for `ServePlugin()`
- Implement the `grpcutil.GKPluginInterface`: `GKRegister()`, `Init()`, `Call()`, `Shutdown()`
- `main()` just calls `grpcutil.ServePlugin(&YourPlugin{})`
- No proto files, no protoc, no gRPC imports needed

## Step 4: Build

```bash
go build -o my-plugin .
```

Output: `my-plugin` binary.

## Step 5: Deploy

```bash
# Create plugin directory in GoatFlow's plugins folder
mkdir -p /path/to/goatflow/plugins/my-plugin

# Copy binary and manifest
cp my-plugin plugin.yaml /path/to/goatflow/plugins/my-plugin/

# Make executable
chmod +x /path/to/goatflow/plugins/my-plugin/my-plugin
```

Final layout:
```
plugins/my-plugin/
├── plugin.yaml    # Manifest
└── my-plugin      # Executable
```

GoatFlow discovers `plugin.yaml` during startup (or immediately via hot reload) and launches the binary.

## Step 6: Test

```bash
# Check plugin status
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/plugins | jq '.[] | select(.Name=="my-plugin")'

# Call your route
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/plugins/my-plugin/status
```

## Hot Reload

The loader watches the plugin directory via fsnotify. To reload your plugin during development:

1. Rebuild the binary: `go build -o my-plugin .`
2. Copy to plugins dir (overwriting the old binary)
3. GoatFlow detects the change, unloads the old plugin, and loads the new one

Changes are debounced by 500ms to handle rapid rebuilds.

You can also modify `plugin.yaml` — the loader picks up manifest changes too. Removing `plugin.yaml` unloads the plugin.

## How It Works Under the Hood

1. **Discovery**: Loader scans `plugins/` for directories containing `plugin.yaml`
2. **Launch**: Loader runs the binary as a child process via `exec.Command`
3. **Handshake**: HashiCorp go-plugin establishes a net/rpc connection (magic cookie: `GOATKIT_PLUGIN=goatkit-v1`)
4. **Registration**: Host calls `GKRegister()` to get plugin capabilities
5. **Initialization**: Host calls `Init()` with host version info
6. **Sandboxing**: Plugin gets a `SandboxedHostAPI` enforcing its `ResourcePolicy`
7. **Serving**: Host routes calls to `Call()` with function name and JSON args
8. **Call timeouts**: Context-based deadlines with goroutine + select pattern
9. **Shutdown**: Host calls `Shutdown()` then kills the process

## Process Isolation & Security

### OS-Level Sandboxing (Linux)

On Linux, gRPC plugin processes are launched with OS-level restrictions:

- **PID and mount namespace isolation** (`CLONE_NEWNS | CLONE_NEWPID`) — plugins can't see host processes or mounts
- **Parent death signal** (`Pdeathsig: SIGKILL`) — if GoatFlow crashes, plugin processes are killed immediately (no orphans)
- **Minimal environment** — your process receives only:
  - `PATH=/usr/local/bin:/usr/bin:/bin`
  - `HOME=/tmp/goatflow-plugin-<name>` (plugin-specific temp dir)
  - `TMPDIR=/tmp/goatflow-plugin-<name>`
  - `TZ` (if set on host)
  - `GOATFLOW_NO_NETWORK=1` (if you don't have `http` permission)

**Important:** Database credentials, API keys, and other host environment variables are **not** passed to plugin processes. Access host resources through the HostAPI only.

On non-Linux platforms (macOS, Windows), process sandboxing is not available. A warning is logged and the plugin runs with full system access.

### Plugin Signing

You can sign your plugin binary with ed25519 for tamper detection:

```bash
# During your build/release process:
# 1. Build the binary
go build -o my-plugin .

# 2. Sign it (using the signing package or CLI tool)
# This creates my-plugin.sig containing the hex-encoded ed25519 signature
```

Ship both `my-plugin` and `my-plugin.sig` in your plugin directory. The host verifies the signature against trusted public keys on load.

Signing is opt-in by default. Set `GOATFLOW_REQUIRE_SIGNATURES=1` on the host to enforce it.

### Caller Identity

The host stamps your plugin's authenticated name on all HostAPI RPC calls. This is set server-side — your plugin cannot impersonate another plugin when making host API calls.

## Bidirectional Calls (HostAPI from Plugin)

The go-plugin MuxBroker enables plugins to call back to the host's HostAPI. The host starts a HostAPI RPC server, and the plugin receives the broker ID during `Init()`. This allows plugins to make DB queries, cache operations, HTTP requests, etc. from within their handlers.

## Advanced Patterns

### Using Host Database

```go
func (p *MyPlugin) Call(fn string, args json.RawMessage) (json.RawMessage, error) {
	switch fn {
	case "get_ticket_count":
		// This calls back to the host via the bidirectional RPC connection
		rows, err := p.hostAPI.DBQuery(ctx, "SELECT COUNT(*) as count FROM tickets WHERE state_id = ?", 1)
		if err != nil {
			return nil, err
		}
		return json.Marshal(rows)
	}
}
```

### Concurrent Processing

```go
func (p *MyPlugin) runBatchReports(reportIDs []string) []ReportResult {
	var wg sync.WaitGroup
	results := make(chan ReportResult, len(reportIDs))
	for _, id := range reportIDs {
		wg.Add(1)
		go func(reportID string) {
			defer wg.Done()
			results <- p.runSingleReport(reportID)
		}(id)
	}
	wg.Wait()
	close(results)
	// collect results...
}
```

### Graceful Shutdown

```go
func (p *MyPlugin) Shutdown() error {
	// Clean up resources, flush buffers, close connections
	p.db.Close()
	return nil
}
```

## Debugging

### Run Standalone (won't connect to host, but tests compilation)

```bash
go build -o my-plugin . && ./my-plugin
```

### Use Delve Debugger

```bash
dlv exec ./my-plugin
```

### Verbose Logging

```go
func (p *MyPlugin) Call(fn string, args json.RawMessage) (json.RawMessage, error) {
	log.Printf("Call: fn=%s args=%s", fn, string(args))
	result, err := p.dispatch(fn, args)
	if err != nil {
		log.Printf("Call failed: %v", err)
	}
	return result, err
}
```

## Real-World Example

See `internal/plugin/grpc/example/main.go` for the built-in hello-grpc plugin that demonstrates:
- Widget rendering with GoatKit CSS variables
- API route handling
- Plugin metadata declaration

## Next Steps

- [Host API Reference](./HOST_API.md) — Full API documentation
- [Plugin Author Guide](./AUTHOR_GUIDE.md) — Best practices and security model
- [WASM Tutorial](./WASM_TUTORIAL.md) — Lighter-weight alternative
- [Plugin Platform Overview](../PLUGIN_PLATFORM.md) — Architecture details
