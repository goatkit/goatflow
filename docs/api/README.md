# GoatFlow API Reference

## Overview

GoatFlow provides three API layers:

1. **v1 API** (`/api/v1/*`) - RESTful JSON API for programmatic access
2. **MCP Server** (`/api/mcp`) - AI assistant integration via JSON-RPC
3. **Internal API** (`/api/*`) - Used by the HTMX web interface

## Quick Start

### Base URL
```
http://localhost:8080
```

### Authentication

```bash
# Login and get JWT token
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}'

# Use token in subsequent requests
curl -X GET http://localhost:8080/api/v1/tickets \
  -H "Authorization: Bearer $JWT_TOKEN"
```

### Response Format

All API responses follow this structure:

```json
{
  "success": true,
  "data": { ... },
  "pagination": {
    "page": 1,
    "per_page": 20,
    "total": 100,
    "total_pages": 5,
    "has_next": true,
    "has_prev": false
  }
}
```

### Error Format

```json
{
  "success": false,
  "error": "Error message description"
}
```

## API Documentation

### OpenAPI/Swagger Specification

- **YAML**: [openapi.yaml](openapi.yaml)
- **JSON**: [openapi.json](openapi.json)
- **Interactive Docs**: `http://localhost:8080/api/docs` (when running)

### Route Documentation

- **Generated Routes**: [api.md](api.md) - Auto-generated from YAML route definitions

## Core Endpoints

### Authentication
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/auth/login` | Authenticate and get JWT tokens |
| POST | `/api/auth/login` | Session-based login |
| POST | `/api/auth/logout` | End session |
| POST | `/api/auth/refresh` | Refresh token |

### Tickets
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/tickets` | List tickets |
| POST | `/api/v1/tickets` | Create ticket |
| GET | `/api/v1/tickets/:id` | Get ticket |
| PUT | `/api/v1/tickets/:id` | Update ticket |
| DELETE | `/api/v1/tickets/:id` | Delete ticket |
| POST | `/api/v1/tickets/:id/assign` | Assign ticket |
| POST | `/api/v1/tickets/:id/close` | Close ticket |
| POST | `/api/v1/tickets/:id/reopen` | Reopen ticket |

### Articles
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/tickets/:id/articles` | Get ticket articles |
| POST | `/api/v1/tickets/:id/articles` | Add article |
| GET | `/api/v1/tickets/:id/articles/:article_id` | Get specific article |

### Queues
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/queues` | List queues |
| POST | `/api/v1/queues` | Create queue (Admin) |
| GET | `/api/v1/queues/:id` | Get queue |
| PUT | `/api/v1/queues/:id` | Update queue (Admin) |
| DELETE | `/api/v1/queues/:id` | Delete queue (Admin) |
| GET | `/api/v1/queues/:id/tickets` | Get queue tickets |
| GET | `/api/v1/queues/:id/stats` | Get queue statistics |

### Users
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/users/me` | Get current user |
| PUT | `/api/v1/users/me` | Update current user |
| GET | `/api/v1/users/me/preferences` | Get preferences |
| PUT | `/api/v1/users/me/preferences` | Update preferences |
| POST | `/api/v1/users/me/password` | Change password |

### Search
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/search` | Global search |
| GET | `/api/v1/search/tickets` | Search tickets |
| GET | `/api/v1/search/users` | Search users |
| GET | `/api/v1/search/suggestions` | Search suggestions |
| GET | `/api/v1/search/saved` | Get saved searches |
| POST | `/api/v1/search/saved` | Create saved search |

### Dashboard
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/dashboard/stats` | Dashboard statistics |
| GET | `/api/v1/dashboard/activity` | Recent activity |
| GET | `/api/v1/dashboard/my-tickets` | My assigned tickets |
| GET | `/api/v1/dashboard/notifications` | Notifications |

### Admin (Requires Admin Role)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/admin/users` | List all users |
| POST | `/api/v1/admin/users` | Create user |
| GET | `/api/v1/admin/users/:id` | Get user |
| PUT | `/api/v1/admin/users/:id` | Update user |
| DELETE | `/api/v1/admin/users/:id` | Delete user |
| GET | `/api/v1/admin/system/config` | Get system config |
| PUT | `/api/v1/admin/system/config` | Update system config |
| GET | `/api/v1/admin/audit/logs` | Get audit logs |

### LDAP Integration (Admin Only)
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/admin/ldap/configure` | Configure LDAP |
| POST | `/api/v1/admin/ldap/test` | Test connection |
| GET | `/api/v1/admin/ldap/config` | Get configuration |
| POST | `/api/v1/admin/ldap/disable` | Disable LDAP |
| POST | `/api/v1/admin/ldap/authenticate` | LDAP authentication |
| GET | `/api/v1/admin/ldap/users/:username` | Get LDAP user |
| GET | `/api/v1/admin/ldap/groups` | List LDAP groups |
| POST | `/api/v1/admin/ldap/sync/users` | Sync users |
| GET | `/api/v1/admin/ldap/sync/status` | Sync status |

## MCP Server (AI Integration)

The MCP (Model Context Protocol) server enables AI assistants to interact with GoatFlow. See [MCP.md](MCP.md) for full documentation.

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/mcp` | JSON-RPC 2.0 endpoint |
| GET | `/api/mcp` | Server info and tool list |

**Key Features:**
- Multi-user proxy: token owner's RBAC permissions apply to all operations
- 10 tools: `list_tickets`, `get_ticket`, `search_tickets`, `list_queues`, `list_users`, `get_statistics`, `execute_sql`, etc.
- Admin-only tools: `execute_sql` requires admin group membership
- Bearer token authentication via API tokens

```bash
curl -X POST https://your-goatflow/api/mcp \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

## Health Checks

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Basic health check |
| GET | `/health/detailed` | Detailed health with components |
| GET | `/healthz` | Kubernetes liveness probe |
| GET | `/metrics` | Prometheus metrics |
| GET | `/api/v1/health` | API health check |
| GET | `/api/v1/status` | System status |

## Rate Limiting

Rate limits vary by endpoint:

| Endpoint Type | Limit |
|---------------|-------|
| Login attempts | 10/minute |
| Password reset | 3/hour |
| Ticket creation | 10/hour (customer) |
| General API | 100/minute |

## Pagination

List endpoints support pagination:

```
GET /api/v1/tickets?page=1&limit=20
```

Response includes pagination metadata:

```json
{
  "pagination": {
    "page": 1,
    "per_page": 20,
    "total": 157,
    "total_pages": 8,
    "has_next": true,
    "has_prev": false
  }
}
```

## Filtering

Many endpoints support filtering via query parameters:

```
GET /api/v1/tickets?status=open&priority=high&queue_id=5
```

## See Also

- [Developer Guide](../developer-guide/README.md)
- [Security Documentation](../SECURITY.md)
- [LDAP Integration](../LDAP.md)

---

*For the complete API specification, see [openapi.yaml](openapi.yaml)*
