# GoatFlow MCP Server

GoatFlow includes a Model Context Protocol (MCP) server that enables AI assistants to interact with the ticketing system programmatically.

## Overview

The MCP server provides a JSON-RPC 2.0 interface for AI assistants to:
- Query tickets, queues, and users
- Search across tickets
- Get dashboard statistics
- Execute read-only SQL queries (admin only)

## Architecture: Multi-User Proxy

The MCP server operates as a **multi-user proxy**. Each request is authenticated by an API token, and the token owner's permissions apply to all operations:

```
┌─────────────────┐     ┌─────────────────┐      ┌──────────────────┐
│  AI Assistant   │────>│   MCP Server    │─────>│  Underlying APIs │
│                 │     │  (proxy layer)  │      │  (RBAC enforced) │
└─────────────────┘     └─────────────────┘      └──────────────────┘
        │                       │
        │                       ▼
        │               ┌──────────────────┐
        │               │  Token identifies│
        └──────────────>│  user + perms    │
                        └──────────────────┘
```

### Permission Model

| Tool Type | Permission Source | Notes |
|-----------|------------------|-------|
| `list_tickets`, `get_ticket`, etc. | Underlying API RBAC | Same permissions as REST API |
| `list_queues`, `list_users` | Underlying API RBAC | Queue access filtered by user's groups |
| `execute_sql` | **Admin group only** | Bypasses API layer, requires admin |

Most MCP tools delegate to the same handlers that power the REST API. This means:
- Agents see only tickets in queues they have `ro` or `rw` access to
- Customers (if using customer tokens) see only their own tickets
- No additional permission configuration needed — existing RBAC applies

### Admin-Only Tools

The `execute_sql` tool bypasses the normal API permission layer and runs queries directly against the database. To prevent privilege escalation, it requires the token owner to be a member of the **admin group**.

```go
// Token must belong to a user in the "admin" group
if !permissionService.IsInGroup(userID, "admin") {
    return error("execute_sql requires admin group membership")
}
```

## Endpoint

```
POST /api/mcp
```

## Authentication

Requires a valid API token with Bearer authentication:

```bash
curl -X POST https://your-goatflow-instance/api/mcp \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

## Protocol

The MCP server implements the Model Context Protocol (version 2024-11-05) using JSON-RPC 2.0.

### Initialize

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
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

## Available Tools

### list_tickets

List tickets with optional filters.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `queue_id` | integer | Filter by queue ID |
| `state_id` | integer | Filter by state ID (1=new, 2=open, 3=pending, 4=closed) |
| `owner_id` | integer | Filter by owner user ID |
| `customer_id` | string | Filter by customer ID |
| `limit` | integer | Maximum tickets to return (default 20, max 100) |
| `offset` | integer | Offset for pagination (default 0) |

**Example:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "list_tickets",
    "arguments": {
      "state_id": 1,
      "limit": 5
    }
  }
}
```

### get_ticket

Get detailed information about a specific ticket.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `ticket_id` | integer | The ticket ID to retrieve |
| `ticket_number` | string | The ticket number (TN) to retrieve |
| `include_articles` | boolean | Include ticket articles/messages (default true) |

Either `ticket_id` or `ticket_number` is required.

**Example:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "get_ticket",
    "arguments": {
      "ticket_id": 12345,
      "include_articles": true
    }
  }
}
```

### search_tickets

Full-text search across tickets.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `query` | string | Search query string (required) |
| `limit` | integer | Maximum results (default 20) |

**Example:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "search_tickets",
    "arguments": {
      "query": "network outage",
      "limit": 10
    }
  }
}
```

### list_queues

List all queues the authenticated user has access to.

**Arguments:** None

**Example:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "list_queues",
    "arguments": {}
  }
}
```

### list_users

List agent users in the system.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `valid` | boolean | Filter by valid/active users only (default true) |
| `limit` | integer | Maximum users to return (default 50) |

**Example:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "list_users",
    "arguments": {
      "limit": 10
    }
  }
}
```

### get_statistics

Get dashboard statistics including ticket counts by state and queue.

**Arguments:** None

**Example:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "get_statistics",
    "arguments": {}
  }
}
```

**Returns:**
```json
{
  "tickets_by_state": {
    "new": 5,
    "open": 120,
    "pending reminder": 30,
    "closed successful": 500
  },
  "tickets_by_queue": {
    "Support": 400,
    "Sales": 150
  },
  "total_tickets": 655,
  "total_users": 10
}
```

### execute_sql

Execute a read-only SQL query. **SELECT queries only** — for debugging and development.

⚠️ **Requires admin group membership.** This tool bypasses the normal API permission layer, so it's restricted to administrators only. Non-admin tokens will receive an error.

**Arguments:**
| Name | Type | Description |
|------|------|-------------|
| `query` | string | SQL SELECT query to execute (required) |
| `args` | array | Query arguments for `?` placeholders |

**Example:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "execute_sql",
    "arguments": {
      "query": "SELECT COUNT(*) as count FROM ticket WHERE queue_id = ?",
      "args": [5]
    }
  }
}
```

**Security Notes:**
- Only SELECT queries are allowed (INSERT, UPDATE, DELETE, DDL rejected)
- Only tokens belonging to admin group members can use this tool
- Non-admin tokens receive: `"execute_sql requires admin group membership"`

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
    "content": [
      {
        "type": "text",
        "text": "Error: ticket not found"
      }
    ],
    "isError": true
  }
}
```

## Creating an API Token

API tokens can be created via:

1. **Agent UI:** Settings → API Tokens
2. **REST API:** `POST /api/v1/tokens`

Tokens use the format `gf_<prefix>_<random>` and are stored using bcrypt hashing.

## Rate Limiting

MCP requests are subject to the same rate limiting as other API endpoints. The default rate limit is 1000 requests per token per hour.
