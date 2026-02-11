// Package grpcutil provides the gRPC plugin serving utilities for GoatKit.
//
// External gRPC plugins import this package to serve their implementation.
// The host-side loading and management stays in internal/plugin/grpc.
//
// Usage:
//
//	func main() {
//	    grpcutil.ServePlugin(&MyPlugin{})
//	}
package grpcutil

import (
	"encoding/json"
	"fmt"
	"log"
	"net/rpc"

	goplugin "github.com/hashicorp/go-plugin"

	"github.com/goatkit/goatflow/pkg/plugin"
)

// Handshake is the shared handshake config for host and plugins.
// Plugins must use the same values to connect.
var Handshake = goplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "GOATKIT_PLUGIN",
	MagicCookieValue: "goatkit-v1",
}

// GKPluginInterface is the interface that gRPC plugins implement.
// Plugins that need HostAPI access should also implement GKPluginWithHost.
type GKPluginInterface interface {
	GKRegister() (*plugin.GKRegistration, error)
	Init(config map[string]string) error
	Call(fn string, args json.RawMessage) (json.RawMessage, error)
	Shutdown() error
}

// GKPluginWithHost is an optional interface plugins implement to receive
// the host's HostAPI during initialization. This allows plugins to use
// the platform's database, cache, HTTP, email, and other services.
type GKPluginWithHost interface {
	GKPluginInterface
	InitWithHost(config map[string]string, host plugin.HostAPI) error
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

// GKPluginPlugin is the go-plugin.Plugin implementation.
type GKPluginPlugin struct {
	goplugin.Plugin
	Impl GKPluginInterface
}

// Server returns the RPC server for the plugin (plugin side).
func (p *GKPluginPlugin) Server(b *goplugin.MuxBroker) (interface{}, error) {
	return &GKPluginRPCServer{Impl: p.Impl, broker: b}, nil
}

// Client returns the RPC client for the plugin (host side).
func (p *GKPluginPlugin) Client(b *goplugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &GKPluginRPCClient{client: c, broker: b}, nil
}

// GKPluginRPCClient is the RPC client implementation (host side).
type GKPluginRPCClient struct {
	client *rpc.Client
	broker *goplugin.MuxBroker
}

func (c *GKPluginRPCClient) GKRegister() (*plugin.GKRegistration, error) {
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

func (c *GKPluginRPCClient) Init(config map[string]string) error {
	req := InitRequest{Config: config}
	var resp interface{}
	return c.client.Call("Plugin.Init", req, &resp)
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

// InitRequest contains initialization data for the plugin.
type InitRequest struct {
	Config    map[string]string
	HostAPIID uint32
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
	Impl   GKPluginInterface
	broker *goplugin.MuxBroker
}

func (s *GKPluginRPCServer) GKRegister(args interface{}, resp *[]byte) error {
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

func (s *GKPluginRPCServer) Init(req InitRequest, resp *interface{}) error {
	// If the plugin implements GKPluginWithHost and the host provided a HostAPI
	// broker ID, connect back to the host and pass the HostAPI to the plugin.
	if withHost, ok := s.Impl.(GKPluginWithHost); ok && req.HostAPIID > 0 && s.broker != nil {
		conn, err := s.broker.Dial(req.HostAPIID)
		if err != nil {
			log.Printf("[plugin] failed to dial host API: %v", err)
			// Fall back to Init without host
			return s.Impl.Init(req.Config)
		}
		hostClient := NewHostAPIClient(rpc.NewClient(conn), req.Config["plugin_name"])
		return withHost.InitWithHost(req.Config, hostClient)
	}

	return s.Impl.Init(req.Config)
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
