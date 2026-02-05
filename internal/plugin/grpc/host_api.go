package grpc

import (
	"context"
	"encoding/json"
	"net/rpc"

	"github.com/gotrs-io/gotrs-ce/internal/plugin"
)

// HostAPIRPCServer exposes HostAPI to plugins via RPC.
// This runs on the host side and handles plugin callbacks.
type HostAPIRPCServer struct {
	Host plugin.HostAPI
}

// HostAPIRequest is a generic host API request.
type HostAPIRequest struct {
	Method       string          // Method name (e.g., "db_query", "cache_get")
	Args         json.RawMessage // JSON-encoded arguments
	CallerPlugin string          // Name of the calling plugin (for error context)
}

// HostAPIResponse is a generic host API response.
type HostAPIResponse struct {
	Result json.RawMessage
	Error  string
}

// Call handles all host API calls from plugins.
func (s *HostAPIRPCServer) Call(req HostAPIRequest, resp *HostAPIResponse) error {
	ctx := context.Background()
	// Set caller plugin in context for better error messages
	if req.CallerPlugin != "" {
		ctx = context.WithValue(ctx, plugin.PluginCallerKey, req.CallerPlugin)
	}
	result, err := dispatchHostCall(ctx, s.Host, req.Method, req.Args)
	if err != nil {
		resp.Error = err.Error()
		return nil
	}
	resp.Result = result
	return nil
}

// dispatchHostCall routes the call to the appropriate HostAPI method.
func dispatchHostCall(ctx context.Context, host plugin.HostAPI, method string, args json.RawMessage) (json.RawMessage, error) {
	switch method {
	case "db_query":
		var req struct {
			Query string `json:"query"`
			Args  []any  `json:"args"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		rows, err := host.DBQuery(ctx, req.Query, req.Args...)
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
		affected, err := host.DBExec(ctx, req.Query, req.Args...)
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
		val, found, err := host.CacheGet(ctx, req.Key)
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
		err := host.CacheSet(ctx, req.Key, req.Value, req.TTL)
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
		status, body, err := host.HTTPRequest(ctx, req.Method, req.URL, req.Headers, req.Body)
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
		err := host.SendEmail(ctx, req.To, req.Subject, req.Body, req.HTML)
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
		val, err := host.ConfigGet(ctx, req.Key)
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
		val := host.Translate(ctx, req.Key, req.Args...)
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
		return host.CallPlugin(ctx, req.Plugin, req.Function, req.Args)

	default:
		return nil, &UnknownMethodError{Method: method}
	}
}

// UnknownMethodError is returned when a plugin calls an unknown host method.
type UnknownMethodError struct {
	Method string
}

func (e *UnknownMethodError) Error() string {
	return "unknown host API method: " + e.Method
}

// HostAPIRPCClient is the client plugins use to call the host.
// This runs on the plugin side.
type HostAPIRPCClient struct {
	client *rpc.Client
}

// NewHostAPIRPCClient creates a new host API client.
func NewHostAPIRPCClient(client *rpc.Client) *HostAPIRPCClient {
	return &HostAPIRPCClient{client: client}
}

// Call makes a host API call.
func (c *HostAPIRPCClient) Call(method string, args any) (json.RawMessage, error) {
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}

	req := HostAPIRequest{Method: method, Args: argsJSON}
	var resp HostAPIResponse

	if err := c.client.Call("HostAPI.Call", req, &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, &HostError{Message: resp.Error}
	}
	return resp.Result, nil
}

// HostError represents an error from the host.
type HostError struct {
	Message string
}

func (e *HostError) Error() string {
	return e.Message
}

// Convenience methods for common operations

func (c *HostAPIRPCClient) DBQuery(query string, args ...any) ([]map[string]any, error) {
	result, err := c.Call("db_query", map[string]any{"query": query, "args": args})
	if err != nil {
		return nil, err
	}
	var rows []map[string]any
	json.Unmarshal(result, &rows)
	return rows, nil
}

func (c *HostAPIRPCClient) DBExec(query string, args ...any) (int64, error) {
	result, err := c.Call("db_exec", map[string]any{"query": query, "args": args})
	if err != nil {
		return 0, err
	}
	var resp struct {
		Affected int64 `json:"affected"`
	}
	json.Unmarshal(result, &resp)
	return resp.Affected, nil
}

func (c *HostAPIRPCClient) CallPlugin(pluginName, fn string, args json.RawMessage) (json.RawMessage, error) {
	return c.Call("plugin_call", map[string]any{
		"plugin":   pluginName,
		"function": fn,
		"args":     args,
	})
}
