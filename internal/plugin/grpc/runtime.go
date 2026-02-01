// Package grpc provides gRPC-based plugin runtime using HashiCorp go-plugin.
//
// This enables native Go plugins to run as separate processes, communicating
// with the host via gRPC. Useful for I/O-heavy plugins that benefit from
// native performance and direct system access.
package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/rpc"
	"os"
	"os/exec"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"

	"github.com/gotrs-io/gotrs-ce/internal/plugin"
)

// Handshake is the shared handshake config for host and plugins.
// Plugins must use the same values to connect.
var Handshake = goplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "GOATKIT_PLUGIN",
	MagicCookieValue: "goatkit-v1",
}

// PluginMap is the map of plugin types we support.
var PluginMap = map[string]goplugin.Plugin{
	"gkplugin": &GKPluginPlugin{},
}

// GRPCPlugin wraps a go-plugin client to implement plugin.Plugin.
type GRPCPlugin struct {
	client       *goplugin.Client
	rpcClient    goplugin.ClientProtocol
	impl         GKPluginInterface
	registration plugin.GKRegistration
	host         plugin.HostAPI
}

// GKPluginInterface is the interface that gRPC plugins implement.
// This is the RPC interface - the actual implementation runs in the plugin process.
type GKPluginInterface interface {
	GKRegister() (*plugin.GKRegistration, error)
	Init(config map[string]string) error
	Call(fn string, args json.RawMessage) (json.RawMessage, error)
	Shutdown() error
}

// GKPluginPlugin is the go-plugin.Plugin implementation.
type GKPluginPlugin struct {
	goplugin.Plugin
	Impl GKPluginInterface
}

// Server returns the RPC server for the plugin (plugin side).
func (p *GKPluginPlugin) Server(*goplugin.MuxBroker) (interface{}, error) {
	return &GKPluginRPCServer{Impl: p.Impl}, nil
}

// Client returns the RPC client for the plugin (host side).
func (p *GKPluginPlugin) Client(b *goplugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &GKPluginRPCClient{client: c}, nil
}

// GKPluginRPCClient is the RPC client implementation (host side).
type GKPluginRPCClient struct {
	client *rpc.Client
}

func (c *GKPluginRPCClient) GKRegister() (*plugin.GKRegistration, error) {
	var resp plugin.GKRegistration
	err := c.client.Call("Plugin.GKRegister", new(interface{}), &resp)
	return &resp, err
}

func (c *GKPluginRPCClient) Init(config map[string]string) error {
	var resp interface{}
	return c.client.Call("Plugin.Init", config, &resp)
}

func (c *GKPluginRPCClient) Call(fn string, args json.RawMessage) (json.RawMessage, error) {
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

func (c *GKPluginRPCClient) Shutdown() error {
	var resp interface{}
	return c.client.Call("Plugin.Shutdown", new(interface{}), &resp)
}

// CallRequest is the RPC request for Call.
type CallRequest struct {
	Function string
	Args     json.RawMessage
}

// CallResponse is the RPC response for Call.
type CallResponse struct {
	Result json.RawMessage
	Error  string
}

// GKPluginRPCServer is the RPC server implementation (plugin side).
type GKPluginRPCServer struct {
	Impl GKPluginInterface
}

func (s *GKPluginRPCServer) GKRegister(args interface{}, resp *plugin.GKRegistration) error {
	reg, err := s.Impl.GKRegister()
	if err != nil {
		return err
	}
	*resp = *reg
	return nil
}

func (s *GKPluginRPCServer) Init(config map[string]string, resp *interface{}) error {
	return s.Impl.Init(config)
}

func (s *GKPluginRPCServer) Call(req CallRequest, resp *CallResponse) error {
	result, err := s.Impl.Call(req.Function, req.Args)
	if err != nil {
		resp.Error = err.Error()
		return nil
	}
	resp.Result = result
	return nil
}

func (s *GKPluginRPCServer) Shutdown(args interface{}, resp *interface{}) error {
	return s.Impl.Shutdown()
}

// LoadGRPCPlugin loads a gRPC plugin from an executable path.
func LoadGRPCPlugin(execPath string, host plugin.HostAPI) (*GRPCPlugin, error) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "plugin",
		Output: os.Stdout,
		Level:  hclog.Info,
	})

	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig: Handshake,
		Plugins:         PluginMap,
		Cmd:             exec.Command(execPath),
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
	// Pass minimal config to the plugin
	config := map[string]string{
		"host_version": "0.6.4",
	}
	return p.impl.Init(config)
}

// Call implements plugin.Plugin.
func (p *GRPCPlugin) Call(ctx context.Context, fn string, args json.RawMessage) (json.RawMessage, error) {
	return p.impl.Call(fn, args)
}

// Shutdown implements plugin.Plugin.
func (p *GRPCPlugin) Shutdown(ctx context.Context) error {
	err := p.impl.Shutdown()
	p.client.Kill()
	return err
}

// ServePlugin is called by plugin executables to serve the plugin.
// Plugin main() should call this with their implementation.
func ServePlugin(impl GKPluginInterface) {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]goplugin.Plugin{
			"gkplugin": &GKPluginPlugin{Impl: impl},
		},
	})
}
