package grpc

import (
	"context"
	"encoding/json"
	"net/rpc"

	"github.com/goatkit/goatflow/internal/platform/plugin"
)

// HostAPIRPCServer exposes HostAPI to plugins via RPC.
// This runs on the host side and handles plugin callbacks.
type HostAPIRPCServer struct {
	Host       plugin.HostAPI
	CallerName string // Authenticated plugin name (set by host, not trusted from plugin)
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

	// SECURITY: Always use the authenticated caller name, not what the plugin claims
	// This prevents plugins from impersonating other plugins in host API calls
	authenticatedCaller := s.CallerName
	if authenticatedCaller == "" {
		authenticatedCaller = req.CallerPlugin // Fallback for compatibility, but log warning
		// TODO: In production, this should be an error
	}

	// Set authenticated caller plugin in context for error messages and access control
	if authenticatedCaller != "" {
		ctx = context.WithValue(ctx, plugin.PluginCallerKey, authenticatedCaller)
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

	case "publish_event":
		var req struct {
			Channel   string `json:"channel"`
			EventType string `json:"event_type"`
			Data      string `json:"data"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		if err := host.PublishEvent(ctx, req.Channel, req.EventType, req.Data); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"status": "ok"})

	case "create_article_attachment":
		var req struct {
			ArticleID   int64  `json:"article_id"`
			CreatedBy   int64  `json:"created_by"`
			Filename    string `json:"filename"`
			ContentType string `json:"content_type"`
			Content     []byte `json:"content"` // base64 over JSON
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		id, err := host.CreateArticleAttachment(ctx, req.ArticleID, req.CreatedBy, req.Filename, req.ContentType, req.Content)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]int64{"id": id})

	case "list_article_attachments":
		var req struct {
			ArticleID int64 `json:"article_id"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		atts, err := host.ListArticleAttachments(ctx, req.ArticleID)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"attachments": atts})

	case "delete_article_attachment":
		var req struct {
			ArticleID    int64 `json:"article_id"`
			AttachmentID int64 `json:"attachment_id"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		if err := host.DeleteArticleAttachment(ctx, req.ArticleID, req.AttachmentID); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"status": "ok"})

	case "render_markdown_to_pdf":
		var req struct {
			Markdown string                  `json:"markdown"`
			Options  plugin.PdfRenderOptions `json:"options"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		pdf, err := host.RenderMarkdownToPdf(ctx, req.Markdown, req.Options)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"pdf": pdf})

	case "create_article":
		var req struct {
			TicketID          int64  `json:"ticket_id"`
			CreatedBy         int64  `json:"created_by"`
			Subject           string `json:"subject"`
			Body              string `json:"body"`
			VisibleToCustomer bool   `json:"visible_to_customer"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		id, err := host.CreateArticle(ctx, req.TicketID, req.CreatedBy, req.Subject, req.Body, req.VisibleToCustomer)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]int64{"id": id})

	case "change_ticket_status":
		var req struct {
			TicketID  int64 `json:"ticket_id"`
			StateID   int64 `json:"state_id"`
			UserID    int64 `json:"user_id"`
			UntilTime int64 `json:"until_time"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		if err := host.ChangeTicketStatus(ctx, req.TicketID, req.StateID, req.UserID, req.UntilTime); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"status": "ok"})

	case "list_ticket_states":
		states, err := host.ListTicketStates(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"states": states})

	case "entity_soft_delete":
		var req struct {
			EntityType string `json:"entity_type"`
			EntityID   int64  `json:"entity_id"`
			Reason     string `json:"reason"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		if err := host.EntitySoftDelete(ctx, req.EntityType, req.EntityID, req.Reason); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"status": "ok"})

	case "entity_restore":
		var req struct {
			EntityType string `json:"entity_type"`
			EntityID   int64  `json:"entity_id"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		if err := host.EntityRestore(ctx, req.EntityType, req.EntityID); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"status": "ok"})

	case "entity_hard_delete":
		var req struct {
			EntityType string `json:"entity_type"`
			EntityID   int64  `json:"entity_id"`
			Reason     string `json:"reason"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		if err := host.EntityHardDelete(ctx, req.EntityType, req.EntityID, req.Reason); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"status": "ok"})

	case "recycle_bin_list":
		var req struct {
			EntityType string `json:"entity_type"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		result, err := host.RecycleBinList(ctx, req.EntityType)
		if err != nil {
			return nil, err
		}
		return result, nil

	case "secure_config_get":
		var req struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		val, err := host.SecureConfigGet(ctx, req.Key)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"value": val})

	case "secure_config_set":
		var req struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		if err := host.SecureConfigSet(ctx, req.Key, req.Value); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"status": "ok"})

	case "org_id":
		id := host.OrgID(ctx)
		return json.Marshal(id)

	case "custom_fields_get":
		var req struct {
			EntityType string   `json:"entity_type"`
			ObjectID   int64    `json:"object_id"`
			Fields     []string `json:"fields"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		result, err := host.CustomFieldsGet(ctx, req.EntityType, req.ObjectID, req.Fields)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)

	case "custom_fields_set":
		var req struct {
			EntityType string         `json:"entity_type"`
			ObjectID   int64          `json:"object_id"`
			Values     map[string]any `json:"values"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		if err := host.CustomFieldsSet(ctx, req.EntityType, req.ObjectID, req.Values); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"status": "ok"})

	case "custom_fields_query":
		var req struct {
			EntityType string                     `json:"entity_type"`
			Filters    []plugin.CustomFieldFilter `json:"filters"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		ids, err := host.CustomFieldsQuery(ctx, req.EntityType, req.Filters)
		if err != nil {
			return nil, err
		}
		return json.Marshal(ids)

	case "store_file":
		var req struct {
			Key      string            `json:"key"`
			Data     []byte            `json:"data"` // base64-encoded by JSON
			Metadata map[string]string `json:"metadata"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		if err := host.StoreFile(ctx, req.Key, req.Data, req.Metadata); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"status": "ok"})

	case "get_file":
		var req struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		data, metadata, err := host.GetFile(ctx, req.Key)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"data": data, "metadata": metadata})

	case "delete_file":
		var req struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		if err := host.DeleteFile(ctx, req.Key); err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"status": "ok"})

	case "list_files":
		var req struct {
			Prefix string `json:"prefix"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		files, err := host.ListFiles(ctx, req.Prefix)
		if err != nil {
			return nil, err
		}
		return json.Marshal(files)

	case "generate_thumbnail":
		var req struct {
			Data        []byte `json:"data"`
			ContentType string `json:"content_type"`
			MaxWidth    int    `json:"max_width"`
			MaxHeight   int    `json:"max_height"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		// Default to 200×200 if not specified
		if req.MaxWidth <= 0 {
			req.MaxWidth = 200
		}
		if req.MaxHeight <= 0 {
			req.MaxHeight = 200
		}
		thumbData, thumbCT, err := host.GenerateThumbnail(ctx, req.Data, req.ContentType, req.MaxWidth, req.MaxHeight)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{
			"thumb_data":         thumbData,
			"thumb_content_type": thumbCT,
		})

	case "log":
		var req struct {
			Level   string         `json:"level"`
			Message string         `json:"message"`
			Fields  map[string]any `json:"fields"`
		}
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}
		host.Log(ctx, req.Level, req.Message, req.Fields)
		return json.Marshal(map[string]bool{"ok": true})

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
