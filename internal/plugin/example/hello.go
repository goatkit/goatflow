// Package example provides example plugin implementations for testing.
package example

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gotrs-io/gotrs-ce/internal/plugin"
)

// HelloPlugin is a simple example plugin that demonstrates the plugin interface.
// This is a native Go implementation for testing - real plugins would be WASM or gRPC.
type HelloPlugin struct {
	host      plugin.HostAPI
	callCount int
}

// NewHelloPlugin creates a new hello plugin instance.
func NewHelloPlugin() *HelloPlugin {
	return &HelloPlugin{}
}

// Manifest implements plugin.Plugin.
func (p *HelloPlugin) GKRegister() plugin.GKRegistration {
	return plugin.GKRegistration{
		Name:        "hello",
		Version:     "1.0.0",
		Description: "A simple hello world plugin for testing",
		Author:      "GOTRS Team",
		License:     "Apache-2.0",
		Homepage:    "https://github.com/gotrs-io/gotrs-ce",

		Routes: []plugin.RouteSpec{
			{
				Method:      "GET",
				Path:        "/api/plugins/hello",
				Handler:     "hello",
				Description: "Returns a hello message",
			},
			{
				Method:      "GET",
				Path:        "/api/plugins/hello/stats",
				Handler:     "stats",
				Description: "Returns plugin statistics",
			},
		},

		MenuItems: []plugin.MenuItemSpec{
			{
				ID:       "hello-menu",
				Label:    "Hello Plugin",
				Icon:     "hand-wave",
				Path:     "/admin/plugins/hello",
				Location: "admin",
				Order:    100,
			},
		},

		Widgets: []plugin.WidgetSpec{
			{
				ID:          "hello-widget",
				Title:       "Hello Widget",
				Description: "Displays a friendly greeting",
				Handler:     "widget",
				Location:    "dashboard",
				Size:        "small",
				Refreshable: true,
				RefreshSec:  60,
			},
		},

		Jobs: []plugin.JobSpec{
			{
				ID:          "hello-job",
				Handler:     "scheduled_hello",
				Schedule:    "0 * * * *", // Every hour
				Description: "Logs a hello message every hour",
				Enabled:     true,
				Timeout:     "30s",
			},
		},

		MinHostVersion: "0.7.0",
		Permissions:    []string{"db:read", "log"},

		ErrorCodes: []plugin.ErrorCodeSpec{
			{Code: "name_required", Message: "Name parameter is required", HTTPStatus: 400},
			{Code: "greeting_failed", Message: "Failed to generate greeting", HTTPStatus: 500},
		},
	}
}

// Init implements plugin.Plugin.
func (p *HelloPlugin) Init(ctx context.Context, host plugin.HostAPI) error {
	p.host = host
	p.host.Log(ctx, "info", "Hello plugin initialized", map[string]any{
		"version": "1.0.0",
	})
	return nil
}

// Call implements plugin.Plugin.
func (p *HelloPlugin) Call(ctx context.Context, fn string, args json.RawMessage) (json.RawMessage, error) {
	p.callCount++

	switch fn {
	case "hello":
		return p.handleHello(ctx, args)
	case "stats":
		return p.handleStats(ctx)
	case "widget":
		return p.handleWidget(ctx)
	case "scheduled_hello":
		return p.handleScheduledHello(ctx)
	default:
		return nil, fmt.Errorf("unknown function: %s", fn)
	}
}

// Shutdown implements plugin.Plugin.
func (p *HelloPlugin) Shutdown(ctx context.Context) error {
	if p.host != nil {
		p.host.Log(ctx, "info", "Hello plugin shutting down", map[string]any{
			"total_calls": p.callCount,
		})
	}
	return nil
}

// Handler implementations

func (p *HelloPlugin) handleHello(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var req struct {
		Name string `json:"name"`
	}
	if len(args) > 0 {
		json.Unmarshal(args, &req)
	}

	name := req.Name
	if name == "" {
		name = "World"
	}

	// Demonstrate using host API
	if p.host != nil {
		greeting := p.host.Translate(ctx, "hello_greeting", name)
		if greeting == "" {
			greeting = fmt.Sprintf("Hello, %s!", name)
		}
		return json.Marshal(map[string]any{
			"message":   greeting,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}

	return json.Marshal(map[string]any{
		"message":   fmt.Sprintf("Hello, %s!", name),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (p *HelloPlugin) handleStats(ctx context.Context) (json.RawMessage, error) {
	return json.Marshal(map[string]any{
		"call_count": p.callCount,
		"uptime":     "running",
	})
}

func (p *HelloPlugin) handleWidget(ctx context.Context) (json.RawMessage, error) {
	// Return HTML fragment for the widget
	html := `<div class="hello-widget">
		<p class="text-lg font-semibold">👋 Hello from the plugin!</p>
		<p class="text-sm text-gray-500">This widget is rendered by the Hello plugin.</p>
	</div>`

	return json.Marshal(map[string]string{
		"html": html,
	})
}

func (p *HelloPlugin) handleScheduledHello(ctx context.Context) (json.RawMessage, error) {
	if p.host != nil {
		p.host.Log(ctx, "info", "Scheduled hello job running", map[string]any{
			"time": time.Now().UTC().Format(time.RFC3339),
		})
	}
	return json.Marshal(map[string]bool{"ok": true})
}
