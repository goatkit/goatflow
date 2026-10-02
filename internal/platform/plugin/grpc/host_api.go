package grpc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"

	"github.com/goatkit/goatflow/internal/platform/plugin"
)

// hostBinding connects one plugin process's host callbacks to the plugin's
// SandboxedHostAPI and to the host calls currently in flight.
//
// The plugin process is started (and its callback server created) before the
// Manager wraps the host in the plugin's sandbox, so the host is bound later,
// in GRPCPlugin.Init. Callbacks before Init are refused.
//
// Each GRPCPlugin.Call registers its context under a random call token that
// travels with the call (CallRequest.CallToken). A plugin that passes the
// token back on its callbacks (grpcutil does this for contexts built with
// the call's context) gets the call's context: acting user, language, call
// depth and deadline. Callbacks without a token run without a call context.
type hostBinding struct {
	mu    sync.RWMutex
	host  plugin.HostAPI
	calls map[string]context.Context
}

func newHostBinding() *hostBinding {
	return &hostBinding{calls: make(map[string]context.Context)}
}

func (b *hostBinding) setHost(host plugin.HostAPI) {
	b.mu.Lock()
	b.host = host
	b.mu.Unlock()
}

// begin registers ctx for one plugin call and returns the call token and
// the function that unregisters it when the call ends.
func (b *hostBinding) begin(ctx context.Context) (string, func()) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		// No randomness: run the call without a call context.
		return "", func() {}
	}
	token := hex.EncodeToString(raw[:])
	b.mu.Lock()
	b.calls[token] = ctx
	b.mu.Unlock()
	return token, func() {
		b.mu.Lock()
		delete(b.calls, token)
		b.mu.Unlock()
	}
}

var (
	errHostNotBound    = errors.New("host API not available before plugin Init")
	errUnknownCallCtxt = errors.New("unknown or finished call context")
)

// resolve returns the bound host and the context for a callback.
func (b *hostBinding) resolve(token string) (plugin.HostAPI, context.Context, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.host == nil {
		return nil, nil, errHostNotBound
	}
	if token == "" {
		return b.host, context.Background(), nil
	}
	ctx, ok := b.calls[token]
	if !ok {
		return nil, nil, errUnknownCallCtxt
	}
	return b.host, ctx, nil
}

// HostAPIRPCServer exposes HostAPI to one plugin process via RPC.
// This runs on the host side and handles plugin callbacks.
type HostAPIRPCServer struct {
	binding    *hostBinding
	CallerName string // Authenticated plugin name (set by host, not trusted from plugin)
}

// HostAPIRequest is a generic host API request. Field names match
// grpcutil.HostAPIRPCRequest (net/rpc gob encoding matches by name).
type HostAPIRequest struct {
	Method       string          // Method name (e.g., "db_query", "cache_get")
	Args         json.RawMessage // JSON-encoded arguments
	CallerPlugin string          // Name the plugin claims; ignored, CallerName is used
	CallToken    string          // Token of the host call this callback belongs to
}

// HostAPIResponse is a generic host API response.
type HostAPIResponse struct {
	Result json.RawMessage
	Error  string
}

// Call handles all host API calls from the plugin.
func (s *HostAPIRPCServer) Call(req HostAPIRequest, resp *HostAPIResponse) error {
	// SECURITY: the caller is the plugin this server was created for, never
	// the name the plugin sends.
	if s.CallerName == "" {
		resp.Error = "host API server has no plugin identity"
		return nil
	}
	host, ctx, err := s.binding.resolve(req.CallToken)
	if err != nil {
		resp.Error = err.Error()
		return nil
	}
	ctx = context.WithValue(ctx, plugin.PluginCallerKey, s.CallerName)

	result, err := plugin.DispatchHostCall(ctx, host, req.Method, req.Args)
	if err != nil {
		resp.Error = err.Error()
		return nil
	}
	resp.Result = result
	return nil
}
