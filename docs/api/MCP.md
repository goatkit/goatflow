# GoatFlow MCP Server

GoatFlow includes a Model Context Protocol (MCP) server that enables AI assistants to interact with the ticketing system programmatically.

## Overview

The MCP server dynamically generates tools from the REST API surface. Every `/api/v1/` endpoint is automatically available as an MCP tool, with the same RBAC enforcement as the REST API. Plugin endpoints are also exposed automatically.

Capabilities include:
- Full CRUD on tickets, queues, users, organisations, custom fields, and more
- Search across tickets
- Dashboard statistics and reporting
- Execute read-only SQL queries (admin only)
- All plugin APIs (auto-discovered from enabled plugins)

## Architecture: API Bridge

The MCP server acts as a thin adapter over the existing REST API. At startup, it reads YAML route definitions and the OpenAPI spec to generate MCP tools dynamically. Tool execution invokes the real Gin handler with the user's auth context, so RBAC is enforced by the same middleware stack.

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐     ┌──────────────┐
│  AI Assistant   │────>│   MCP Server    │────>│   API Bridge    │────>│  Gin Handler │
│                 │     │  (JSON-RPC 2.0) │     │  (synthetic ctx)│     │  + Middleware │
└─────────────────┘     └─────────────────┘     └─────────────────┘     └──────────────┘
        │                       │                       │
        │                       ▼                       ▼
        │               ┌──────────────┐        ┌──────────────┐
        │               │ Token → user │        │ RBAC enforced│
        └──────────────>│ + role       │        │ per endpoint │
                        └──────────────┘        └──────────────┘
```

### Dynamic Tool Generation

Tools are generated automatically at startup from two sources:

1. **REST API routes** (`routes/api-v1.yaml`) — each route becomes an MCP tool with input schema derived from the OpenAPI spec
2. **Plugin routes** — each enabled plugin's routes become MCP tools, namespaced by plugin name

No manual tool registration is needed. Adding a new API endpoint or plugin route automatically makes it available via MCP.

### Permission Model

All tools inherit the RBAC of the underlying API endpoint:

| Endpoint Type | Permission Source | Notes |
|---------------|------------------|-------|
| Ticket endpoints | Queue RBAC middleware | `ticket_access_ro`, `ticket_access_rw`, etc. |
| Admin endpoints | Admin middleware | Requires admin group membership |
| Plugin endpoints | Plugin middleware | `auth`, `admin`, `group:<name>` |
| Public endpoints | None | Health, info |

The MCP server resolves the API token owner's actual role (Admin/Agent) from the database, ensuring admin middleware works correctly regardless of how the token was issued.

### Plugin Tools

Enabled plugins are automatically exposed as MCP tools:

- **Route-based tools**: auto-generated from plugin `RouteSpec` declarations, named `{plugin}_{handler}`
- **Declared tools**: plugins can optionally declare `MCPTools` in their `GKRegistration` with full JSON Schema input schemas — these override route-based tools on name collision
- **Refresh on change**: tool list is rebuilt when plugins are enabled, disabled, or uploaded

## Endpoints

### HTTP POST (original)

```
POST /api/mcp
Authorization: Bearer <api-token>
```

Single-request JSON-RPC 2.0 endpoint. One request per HTTP call, stateless.

### Streamable HTTP / SSE (recommended)

```
POST /api/mcp/sse       # Client→server JSON-RPC (creates session on initialize)
GET  /api/mcp/sse       # Server→client SSE notification stream
DELETE /api/mcp/sse     # Session termination
```

MCP 2025-03-26 Streamable HTTP transport. Supports session state via `Mcp-Session-Id` header, server-initiated notifications (e.g. `tools/list_changed`), and heartbeat keepalive.

Both JWT and API tokens are supported on the SSE endpoint (`unified_auth` middleware).

## Authentication

Requires a valid API token or JWT with Bearer authentication:

```bash
curl -X POST https://your-goatflow-instance/api/mcp/sse \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

## Protocol

The MCP server supports two protocol versions:

| Version | Transport | Notes |
|---------|-----------|-------|
| `2024-11-05` | `POST /api/mcp` | Original, stateless |
| `2025-03-26` | `POST/GET/DELETE /api/mcp/sse` | Streamable HTTP with sessions |

Protocol version is negotiated during the `initialize` handshake — the server responds with the client's requested version if supported.

### Initialize

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-03-26",
    "clientInfo": {
      "name": "your-client",
      "version": "1.0"
    }
  }
}
```

### List Tools

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/list"
}
```

### Call a Tool

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "list_tickets",
    "arguments": {
      "limit": 10
    }
  }
}
```

## Tool Naming Convention

Tools are named based on HTTP method and path:

| Method | Path | Tool Name |
|--------|------|-----------|
| GET | /tickets | `list_tickets` |
| POST | /tickets | `create_ticket` |
| GET | /tickets/:id | `get_ticket` |
| PUT | /tickets/:id | `update_ticket` |
| DELETE | /tickets/:id | `delete_ticket` |
| GET | /tickets/:id/articles | `list_ticket_articles` |
| POST | /tickets/:id/articles | `create_ticket_article` |
| GET | /queues/:id/stats | `list_queue_stats` |
| POST | /admin/sql | `create_admin_sql` |

Plugin tools are prefixed with the plugin name: `myplugin_run_task`, `goatkit_llm_chat`, etc.

## Example Tools

### list_tickets

List tickets with optional filters. Results scoped to queues the authenticated user can access.

```json
{
  "jsonrpc": "2.0", "id": 1,
  "method": "tools/call",
  "params": {
    "name": "list_tickets",
    "arguments": { "status": "open", "limit": 5 }
  }
}
```

### get_ticket

Get detailed information about a specific ticket.

```json
{
  "jsonrpc": "2.0", "id": 1,
  "method": "tools/call",
  "params": {
    "name": "get_ticket",
    "arguments": { "id": 12345 }
  }
}
```

### create_admin_sql

Execute a read-only SQL query. Requires admin group membership. Allowlisted statements: SELECT, DESCRIBE, EXPLAIN, SHOW TABLES, SHOW COLUMNS.

```json
{
  "jsonrpc": "2.0", "id": 1,
  "method": "tools/call",
  "params": {
    "name": "create_admin_sql",
    "arguments": {
      "query": "SELECT COUNT(*) as count FROM ticket WHERE queue_id = ?",
      "args": [5]
    }
  }
}
```

## Route-Level MCP Control

Routes can control their MCP visibility via optional YAML fields:

```yaml
- path: /tickets
  method: GET
  handler: HandleListTicketsAPI
  description: "List tickets"
  mcp_description: "List tickets with optional filters. Returns ticket ID, number, title, state, queue, priority, and owner."
  # mcp: false  # uncomment to exclude from MCP
```

- `mcp: false` — exclude a route from MCP tool generation
- `mcp_description` — override the tool description with an LLM-friendly version

## Claude Code Configuration

To use GoatFlow's MCP server with Claude Code, add to `.mcp.json`:

```json
{
  "mcpServers": {
    "goatflow": {
      "type": "http",
      "url": "http://localhost:8080/api/mcp/sse",
      "headers": {
        "Authorization": "Bearer gf_your_api_token_here"
      }
    }
  }
}
```

> **Note:** Use `"type": "http"` (Streamable HTTP), not `"type": "sse"` (deprecated legacy format).

## Plugin MCP Tools

Plugins can declare rich MCP tool definitions in their `GKRegistration`:

```go
GKRegistration{
    Name: "my-plugin",
    MCPTools: []MCPToolSpec{
        {
            Name:        "do_action",
            Description: "Perform a specific action with detailed parameters",
            Handler:     "DoAction",
            InputSchema: map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "target": map[string]any{
                        "type": "string",
                        "description": "Target entity",
                    },
                },
                "required": []any{"target"},
            },
        },
    },
}
```

Declared tools provide richer schemas than auto-generated route tools and take priority on name collision.

## Error Handling

Errors are returned in JSON-RPC 2.0 format:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32601,
    "message": "Method not found: unknown/method"
  }
}
```

Tool execution errors are returned as successful responses with `isError: true`:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [{ "type": "text", "text": "Error (403): Admin access required" }],
    "isError": true
  }
}
```

## Creating an API Token

API tokens can be created via:

1. **Agent UI:** Settings -> API Tokens
2. **REST API:** `POST /api/v1/tokens`

Tokens use the format `gf_<prefix>_<random>` and are stored using bcrypt hashing.

## Rate Limiting

MCP requests are subject to the same rate limiting as other API endpoints. The default rate limit is 1000 requests per token per hour.

## SSE Session Management

SSE sessions expire after 30 minutes of inactivity. A background cleanup goroutine removes expired sessions. Heartbeat events (`:keepalive`) are sent every 30 seconds to prevent proxy timeouts.

Sessions are identified by the `Mcp-Session-Id` header, returned on the `initialize` response. The session is bound to the authenticated user — session user mismatch returns 403.
