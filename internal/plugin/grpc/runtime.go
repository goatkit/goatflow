// Package grpc provides gRPC-based plugin runtime using HashiCorp go-plugin.
//
// This enables native Go plugins to run as separate processes, communicating
// with the host via gRPC. Useful for I/O-heavy plugins that benefit from
// native performance and direct system access.
//
// Shared types (handshake, RPC structs, ServePlugin) live in
// pkg/plugin/grpcutil so external plugins can import them. This package
// adds the host-side loading, HostAPI bridging, and bidirectional calls.
package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"log"
	"net/rpc"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"

	"github.com/goatkit/goatflow/internal/plugin"
	"github.com/goatkit/goatflow/pkg/plugin/grpcutil"
)

// Re-export shared types so existing internal callers don't break.
type GKPluginInterface = grpcutil.GKPluginInterface
type GKPluginPlugin = grpcutil.GKPluginPlugin
type GKPluginRPCServer = grpcutil.GKPluginRPCServer
type GKPluginRPCClient = grpcutil.GKPluginRPCClient
type CallRequest = grpcutil.CallRequest
type CallResponse = grpcutil.CallResponse
type InitRequest = grpcutil.InitRequest

// ServePlugin re-exports the public ServePlugin for internal use.
var ServePlugin = grpcutil.ServePlugin

// Handshake re-exports the shared handshake config.
var Handshake = grpcutil.Handshake

// PluginMap is the map of plugin types we support.
var PluginMap = map[string]goplugin.Plugin{
	"gkplugin": &GKPluginPluginHost{},
}

// GRPCPlugin wraps a go-plugin client to implement plugin.Plugin.
type GRPCPlugin struct {
	client       *goplugin.Client
	rpcClient    goplugin.ClientProtocol
	impl         GKPluginInterface
	registration plugin.GKRegistration
	host         plugin.HostAPI
	callTimeout  time.Duration // Per-call deadline (0 = no timeout)
}

// GKPluginPluginHost is the host-side go-plugin.Plugin implementation.
// It extends the base plugin with HostAPI bidirectional call support.
type GKPluginPluginHost struct {
	goplugin.Plugin
	Impl       GKPluginInterface
	Host       plugin.HostAPI // For bidirectional calls
	PluginName string          // Plugin name for caller authentication
}

// Server returns the RPC server for the plugin (plugin side).
func (p *GKPluginPluginHost) Server(b *goplugin.MuxBroker) (interface{}, error) {
	return &GKPluginRPCServerHost{Impl: p.Impl, broker: b}, nil
}

// Client returns the RPC client for the plugin (host side).
func (p *GKPluginPluginHost) Client(b *goplugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	// Start a server for the host API that the plugin can call back to
	hostAPIServer := &HostAPIRPCServer{Host: p.Host, CallerName: p.PluginName}

	// Get an ID for the host API server
	id := b.NextId()
	go b.AcceptAndServe(id, hostAPIServer)

	return &GKPluginRPCClientHost{client: c, broker: b, hostAPIID: id}, nil
}

// GKPluginRPCClientHost is the host-side RPC client with HostAPI bridging.
type GKPluginRPCClientHost struct {
	client    *rpc.Client
	broker    *goplugin.MuxBroker
	hostAPIID uint32
}

func (c *GKPluginRPCClientHost) GKRegister() (*plugin.GKRegistration, error) {
	var respBytes []byte
	err := c.client.Call("Plugin.GKRegister", new(interface{}), &respBytes)
	if err != nil {
		return nil, err
	}
	var reg plugin.GKRegistration
	if err := json.Unmarshal(respBytes, &reg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal registration: %w", err)
	}
	return &reg, nil
}

func (c *GKPluginRPCClientHost) Init(config map[string]string) error {
	// Pass the host API broker ID so the plugin can call back
	req := InitRequest{
		Config:    config,
		HostAPIID: c.hostAPIID,
	}
	var resp interface{}
	return c.client.Call("Plugin.Init", req, &resp)
}

func (c *GKPluginRPCClientHost) Call(fn string, args json.RawMessage) (json.RawMessage, error) {
	req := CallRequest{Function: fn, Args: args}
	var resp CallResponse
	err := c.client.Call("Plugin.Call", req, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	return resp.Result, nil
}

func (c *GKPluginRPCClientHost) Shutdown() error {
	var resp interface{}
	return c.client.Call("Plugin.Shutdown", new(interface{}), &resp)
}

// GKPluginRPCServerHost is the host-side RPC server with HostAPI bridging.
type GKPluginRPCServerHost struct {
	Impl    GKPluginInterface
	broker  *goplugin.MuxBroker
	hostAPI *HostAPIRPCClient // Set after Init connects back to host
}

func (s *GKPluginRPCServerHost) GKRegister(args interface{}, resp *[]byte) error {
	reg, err := s.Impl.GKRegister()
	if err != nil {
		return err
	}
	data, err := json.Marshal(reg)
	if err != nil {
		return fmt.Errorf("failed to marshal registration: %w", err)
	}
	*resp = data
	return nil
}

func (s *GKPluginRPCServerHost) Init(req InitRequest, resp *interface{}) error {
	// Connect back to the host's HostAPI server
	if req.HostAPIID > 0 && s.broker != nil {
		conn, err := s.broker.Dial(req.HostAPIID)
		if err == nil {
			s.hostAPI = NewHostAPIRPCClient(rpc.NewClient(conn))
		}
	}
	return s.Impl.Init(req.Config)
}

func (s *GKPluginRPCServerHost) Call(req CallRequest, resp *CallResponse) error {
	result, err := s.Impl.Call(req.Function, req.Args)
	if err != nil {
		resp.Error = err.Error()
		return nil
	}
	resp.Result = result
	return nil
}

func (s *GKPluginRPCServerHost) Shutdown(args interface{}, resp *interface{}) error {
	return s.Impl.Shutdown()
}

// LoadGRPCPlugin loads a gRPC plugin from an executable path with OS-level sandboxing.
func LoadGRPCPlugin(execPath, pluginName string, host plugin.HostAPI, policy plugin.ResourcePolicy) (*GRPCPlugin, error) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "plugin",
		Output: os.Stdout,
		Level:  hclog.Info,
	})

	// Create plugin map with host API for bidirectional calls
	pluginMap := map[string]goplugin.Plugin{
		"gkplugin": &GKPluginPluginHost{Host: host, PluginName: pluginName},
	}

	// Create command with OS-level sandboxing
	cmd := exec.Command(execPath)
	if err := applyProcessSandbox(cmd, policy, pluginName); err != nil {
		return nil, fmt.Errorf("failed to apply process sandbox: %w", err)
	}

	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig: grpcutil.Handshake,
		Plugins:         pluginMap,
		Cmd:             cmd,
		Logger:          logger,
		AllowedProtocols: []goplugin.Protocol{
			goplugin.ProtocolNetRPC,
		},
	})

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to create RPC client: %w", err)
	}

	raw, err := rpcClient.Dispense("gkplugin")
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to dispense plugin: %w", err)
	}

	impl, ok := raw.(GKPluginInterface)
	if !ok {
		client.Kill()
		return nil, fmt.Errorf("plugin does not implement GKPluginInterface")
	}

	// Get registration
	reg, err := impl.GKRegister()
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to get plugin registration: %w", err)
	}
	log.Printf("🔌 Plugin %q registration: %d routes, %d widgets, %d menu items",
		reg.Name, len(reg.Routes), len(reg.Widgets), len(reg.MenuItems))

	return &GRPCPlugin{
		client:       client,
		rpcClient:    rpcClient,
		impl:         impl,
		registration: *reg,
		host:         host,
	}, nil
}

// GKRegister implements plugin.Plugin.
func (p *GRPCPlugin) GKRegister() plugin.GKRegistration {
	return p.registration
}

// Init implements plugin.Plugin.
func (p *GRPCPlugin) Init(ctx context.Context, host plugin.HostAPI) error {
	p.host = host
	config := buildPluginConfig(p.registration.Name)
	return p.impl.Init(config)
}

// buildPluginConfig collects configuration for a plugin from environment variables.
// Variables prefixed with GOATFLOW_PLUGIN_<NAME>_ are included with the prefix stripped
// and the key lowercased. For example, GOATFLOW_PLUGIN_GOATFICTUS_LLM_URL becomes "llm_url".
func buildPluginConfig(pluginName string) map[string]string {
	// Provide standard paths for the plugin.
	// plugin_dir: where the plugin binary + assets live (read-only)
	// work_dir:   persistent writable directory for plugin data (screenshots, media, etc.)
	pluginDir := filepath.Join("config", "plugins", pluginName)
	workDir := filepath.Join("data", "plugins", pluginName)

	// Ensure work_dir exists.
	os.MkdirAll(workDir, 0755)

	config := map[string]string{
		"host_version": "0.6.4",
		"plugin_name":  pluginName,
		"plugin_dir":   pluginDir,
		"work_dir":     workDir,
	}

	// Collect env vars with prefix GOATFLOW_PLUGIN_<NAME>_
	prefix := "GOATFLOW_PLUGIN_" + strings.ToUpper(strings.ReplaceAll(pluginName, "-", "_")) + "_"
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, prefix) {
			continue
		}
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimPrefix(parts[0], prefix))
		config[key] = parts[1]
	}

	return config
}

// Call implements plugin.Plugin.
func (p *GRPCPlugin) Call(ctx context.Context, fn string, args json.RawMessage) (json.RawMessage, error) {
	if p.callTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.callTimeout)
		defer cancel()
	}

	// Run the call in a goroutine so we can respect context cancellation
	type result struct {
		data json.RawMessage
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		data, err := p.impl.Call(fn, args)
		ch <- result{data, err}
	}()

	select {
	case r := <-ch:
		return r.data, r.err
	case <-ctx.Done():
		return nil, fmt.Errorf("plugin %q call %q: %w", p.registration.Name, fn, ctx.Err())
	}
}

// SetCallTimeout sets the per-call timeout for this plugin.
func (p *GRPCPlugin) SetCallTimeout(d time.Duration) {
	p.callTimeout = d
}

// Shutdown implements plugin.Plugin.
func (p *GRPCPlugin) Shutdown(ctx context.Context) error {
	err := p.impl.Shutdown()
	p.client.Kill()
	return err
}
