// Example gRPC plugin for GoatKit.
//
// Build: go build -o hello-grpc ./internal/plugin/grpc/example
//
// Deploy to the plugins directory alongside plugin.yaml:
//
//	plugins/hello-grpc/
//	  ├── plugin.yaml   # name, version, runtime: grpc, binary: hello-grpc
//	  └── hello-grpc    # the executable
//
// The host discovers plugin.yaml, launches the binary, and communicates via RPC.
// Hot reload: updating the binary triggers automatic unload → reload.
package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/goatkit/goatflow/internal/plugin"
	grpcplugin "github.com/goatkit/goatflow/internal/plugin/grpc"
)

// HelloGRPCPlugin is a simple example gRPC plugin.
type HelloGRPCPlugin struct {
	config    map[string]string
	startedAt time.Time
}

// GKRegister returns the plugin registration.
func (p *HelloGRPCPlugin) GKRegister() (*plugin.GKRegistration, error) {
	return &plugin.GKRegistration{
		Name:        "hello-grpc",
		Version:     "1.0.0",
		Description: "Example gRPC plugin for GoatKit",
		Author:      "GoatFlow Team",
		License:     "Apache-2.0",
		Homepage:    "https://goatflow.io",
		Resources: &plugin.ResourceRequest{
			MemoryMB:        64,
			CallTimeout:     "5s",
			InitTimeout:     "5s",
			ShutdownTimeout: "2s",
		},

		Widgets: []plugin.WidgetSpec{
			{
				ID:          "hello-grpc-widget",
				Title:       "Hello gRPC",
				Description: "A widget from a gRPC plugin",
				Handler:     "render_widget",
				Location:    "dashboard",
				Size:        "medium",
			},
		},

		Routes: []plugin.RouteSpec{
			{
				Method:      "GET",
				Path:        "/api/plugins/hello-grpc/status",
				Handler:     "get_status",
				Middleware:  []string{"auth"},
				Description: "Get plugin status",
			},
		},
	}, nil
}

// Init initializes the plugin.
func (p *HelloGRPCPlugin) Init(config map[string]string) error {
	p.config = config
	p.startedAt = time.Now()
	fmt.Println("[hello-grpc] Initialized with config:", config)
	return nil
}

// Call handles function calls from the host.
func (p *HelloGRPCPlugin) Call(fn string, args json.RawMessage) (json.RawMessage, error) {
	switch fn {
	case "render_widget":
		html := `<div class="text-center p-4">
			<h3 class="text-lg font-semibold" style="color: var(--gk-text-primary);">🔌 Hello from gRPC!</h3>
			<p style="color: var(--gk-text-muted);">This widget runs in a separate process.</p>
			<p class="mt-2 text-sm" style="color: var(--gk-success);">Native Go • HashiCorp go-plugin • RPC</p>
		</div>`
		return json.Marshal(map[string]string{"html": html})

	case "get_status":
		return json.Marshal(map[string]any{
			"status":  "running",
			"version": "1.0.0",
			"type":    "grpc",
		})

	case plugin.HealthPingFunc:
		return json.Marshal(map[string]any{
			"status":     "ok",
			"runtime":    "grpc",
			"version":    "1.0.0",
			"uptime_sec": int64(time.Since(p.startedAt).Seconds()),
		})

	default:
		return nil, fmt.Errorf("unknown function: %s", fn)
	}
}

// Shutdown cleans up the plugin.
func (p *HelloGRPCPlugin) Shutdown() error {
	fmt.Println("[hello-grpc] Shutting down")
	return nil
}

func main() {
	grpcplugin.ServePlugin(&HelloGRPCPlugin{})
}
