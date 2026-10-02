# GoatFlow API Reference

## Overview

GoatFlow has three API layers:

1. **v1 API** (`/api/v1/*`) - JSON REST API for scripts and integrations.
2. **MCP server** (`/api/mcp`) - JSON-RPC for AI assistants. See [MCP.md](MCP.md).
3. **Internal API** (`/api/*`) - used by the web interface (HTMX). Not covered here.

This page lists the main endpoints. The full, generated list is in [api.md](api.md) (built from `routes/*.yaml` by `make api-docs`).

## Quick Start

### Base URL

```
http://localhost:8080
```

### Authentication

You can use a JWT from the login endpoint or an API token (`gf_...`, see [API tokens](#api-tokens)).

```bash
# Log in and get an access token and a refresh token
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"login":"admin","password":"your-password"}'

# Use the token
curl http://localhost:8080/api/v1/tickets \
  -H "Authorization: Bearer $TOKEN"
```

### Response format

List endpoints such as `GET /api/v1/tickets` answer like this:

```json
{
  "success": true,
  "data": [ ... ],
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

Errors:

```json
{
  "success": false,
  "error": "Error message description"
}
```

## API Documentation

- **OpenAPI YAML**: [openapi.yaml](openapi.yaml)
- **OpenAPI JSON**: [openapi.json](openapi.json)
- **Interactive docs**: `http://localhost:8080/swagger` (on by default; `GOATFLOW_SWAGGER_ENABLED`)
- **All routes**: [api.md](api.md) (generated)

## Who can call what

Each route group in `routes/*.yaml` names its middleware:

| Middleware | Meaning |
|------------|---------|
| none | Public |
| `unified_auth` | Any valid JWT or API token (agent or customer) |
| `agent` | Agents only |
| `admin` | Members of the `admin` group. API tokens also need the `admin:*` scope |
| `queue_ro`, `ticket_access_*`, ... | Queue or ticket permission checks |

## Core Endpoints

### Authentication

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/api/v1/auth/login` | public | Body `{"login": "...", "password": "..."}`. Returns an access token and a refresh token |
| POST | `/api/v1/auth/refresh` | public | Body `{"refresh_token": "..."}`. Returns a new access token and a new (rotated) refresh token |
| POST | `/api/auth/login` | public | Session (cookie) login, used by the web UI |
| POST | `/api/auth/logout` | session | End the session |

### Tickets

Customers (customer JWT or customer API token) can call the `GET` ticket and article routes. They only see their own tickets and customer-visible articles. All other ticket routes are for agents.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/tickets` | List tickets |
| POST | `/api/v1/tickets` | Create ticket |
| GET | `/api/v1/tickets/:id` | Get ticket |
| PUT | `/api/v1/tickets/:id` | Update ticket (also used to assign, move, or change state) |
| DELETE | `/api/v1/tickets/:id` | Delete ticket |
| POST | `/api/v1/tickets/:id/reopen` | Reopen ticket |
| POST | `/api/v1/tickets/:id/time` | Add time accounting |
| GET | `/api/v1/tickets/:id/internal-notes` | List internal notes |
| POST | `/api/v1/tickets/:id/internal-notes` | Add internal note |
| PUT | `/api/v1/tickets/:id/internal-notes/:note_id` | Edit internal note |
| DELETE | `/api/v1/tickets/:id/internal-notes/:note_id` | Delete internal note |

`GET /api/v1/tickets` query parameters:

| Parameter | Meaning |
|-----------|---------|
| `page`, `per_page` | Paging. `per_page` max is 100. `limit` is an alias for `per_page`; `offset` is turned into a page |
| `status` | `open`, `closed`, `pending`, or a state name |
| `queue_id`, `priority_id`, `customer_user_id`, `assigned_user_id` | Filters |
| `search` | Text search |
| `sort`, `order` | Sort field (default `created`) and `asc`/`desc` (default `desc`) |
| `include` | Comma-separated extra data |

### Articles

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/tickets/:id/articles` | List articles |
| POST | `/api/v1/tickets/:id/articles` | Add article |
| GET | `/api/v1/tickets/:id/articles/:article_id` | Get article |
| PUT | `/api/v1/tickets/:id/articles/:article_id` | Update article (needs `rw` on the ticket's queue) |
| DELETE | `/api/v1/tickets/:id/articles/:article_id` | Delete article (needs `rw` on the ticket's queue) |

### Queues

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/queues` | List queues |
| GET | `/api/v1/queues/:id` | Get queue |
| GET | `/api/v1/queues/:id/agents` | Agents with access to the queue |
| POST | `/api/v1/queues` | Create queue (admin) |
| PUT | `/api/v1/queues/:id` | Update queue (admin) |
| DELETE | `/api/v1/queues/:id` | Delete queue (admin) |
| GET | `/api/v1/queues/:id/stats` | Queue statistics (admin) |
| POST | `/api/v1/queues/:id/groups` | Give a group access to the queue (admin) |
| DELETE | `/api/v1/queues/:id/groups/:group_id` | Remove a group from the queue (admin) |

### Users and groups

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/users/me` | Current user |
| GET | `/api/v1/users` | List users |
| GET | `/api/v1/users/:id` | Get user |
| POST | `/api/v1/users` | Create user (admin) |
| PUT | `/api/v1/users/:id` | Update user (admin) |
| DELETE | `/api/v1/users/:id` | Delete user (admin) |
| GET | `/api/v1/groups` | List groups |

### Lookups

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/priorities`, `/api/v1/priorities/:id` | Priorities. `POST`, `PUT`, `DELETE` are admin only |
| GET | `/api/v1/types` | Ticket types |
| GET | `/api/v1/states` | Ticket states |
| GET | `/api/v1/services` | Services |
| GET, POST, PUT | `/api/v1/system-addresses` | System addresses (writes admin only) |
| GET, POST, PUT | `/api/v1/salutations` | Salutations (writes admin only) |
| GET, POST, PUT | `/api/v1/signatures` | Signatures (writes admin only) |
| POST | `/api/v1/markdown/render` | Render Markdown to safe HTML |

### Search

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/search` | Global search |
| GET | `/api/v1/search/tickets` | Search tickets |
| GET | `/api/v1/search/health` | Search backend health |
| POST | `/api/v1/search/reindex` | Rebuild the search index (admin) |
| GET | `/api/v1/search/saved` | List saved searches |
| POST | `/api/v1/search/saved` | Create saved search |
| GET | `/api/v1/search/saved/:name` | Get saved search |
| PUT | `/api/v1/search/saved/:name` | Update saved search |
| DELETE | `/api/v1/search/saved/:name` | Delete saved search |
| POST | `/api/v1/search/saved/:name/execute` | Run saved search |

### Statistics

All need read access to at least one queue (`queue_ro`). Results only cover queues the caller can read.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/statistics/dashboard` | Ticket counts, per-queue and per-priority counts, recent tickets |
| GET | `/api/v1/statistics/trends` | Ticket trends over time |
| GET | `/api/v1/statistics/agents` | Agent performance |
| GET | `/api/v1/statistics/queues` | Queue metrics |
| GET | `/api/v1/statistics/analytics` | Time-based analytics |
| GET | `/api/v1/statistics/customers` | Customer statistics |
| GET | `/api/v1/statistics/export` | Export statistics |
| GET | `/api/v1/ticket-states/statistics` | Tickets per state |

### Custom fields and organisations

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/custom-fields/definitions[/:id]` | Field definitions |
| POST, PUT, DELETE | `/api/v1/custom-fields/definitions[/:id]` | Manage definitions (admin) |
| GET, PUT | `/api/v1/custom-fields/values/:entity_type/:id` | Read or set values |
| POST | `/api/v1/custom-fields/query` | Find entities by field value |
| GET | `/api/v1/session/orgs` | Organisations of the current user |
| POST | `/api/v1/session/org` | Switch organisation |
| GET, POST, PUT, DELETE | `/api/v1/organisations[/:id]` | Manage organisations (admin) |
| GET, POST, DELETE | `/api/v1/organisations/:id/members[/:member_id]` | Members (admin) |
| GET, POST, DELETE | `/api/v1/organisations/:id/plugin-access[/:plugin]` | Plugin access per organisation (admin) |
| POST | `/api/v1/organisations/:id/captive-plugin` | Set captive plugin (admin) |
| GET, PUT, DELETE | `/api/v1/organisations/:id/config[/:name]` | Organisation config (admin) |

### Webhooks

Admin only (`routes/api-webhooks.yaml`). The same settings are in the UI at **Admin -> Webhooks** (`/admin/webhooks`).

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/webhooks` | List webhooks |
| POST | `/api/v1/webhooks` | Create webhook |
| GET | `/api/v1/webhooks/events` | List events you can subscribe to |
| GET | `/api/v1/webhooks/:id` | Get webhook |
| PUT | `/api/v1/webhooks/:id` | Update webhook |
| DELETE | `/api/v1/webhooks/:id` | Delete webhook |
| POST | `/api/v1/webhooks/:id/test` | Send a `webhook.test` delivery |
| GET | `/api/v1/webhooks/:id/deliveries` | Delivery log |
| GET | `/api/v1/webhooks/deliveries/:id` | One delivery |
| POST | `/api/v1/webhooks/deliveries/:id/redeliver` | Send a delivery again |

Create body: `name`, `url`, `events` (required); `secret`, `headers`, `retry_count`, `timeout_seconds`, `is_active` (optional).

| Setting | Rule |
|---------|------|
| `secret` | 16 to 512 characters. Stored encrypted with `GOATFLOW_SECURE_KEY`. Never returned; responses show `has_secret` and `secret_hint` |
| `retry_count` | 0 to 10, default 3 |
| `timeout_seconds` | 1 to 60, default 10 |
| `headers` | Extra headers. You cannot override the `X-Webhook-*` headers |

Events:

| Event | When |
|-------|------|
| `ticket.created` | A ticket was created (any channel) |
| `ticket.updated` | Title, customer, type, service, SLA, responsible, lock, pending time or a dynamic field changed |
| `ticket.state_changed` | State changed to a state that is not closed |
| `ticket.closed` | State changed to a closed state |
| `ticket.queue_moved` | Ticket moved to another queue |
| `ticket.assigned` | Owner changed |
| `ticket.priority_changed` | Priority changed |
| `ticket.merged` | Ticket was merged |
| `ticket.escalated` | A response, update or solution escalation started |
| `article.created` | An article was added to a ticket |

Each delivery is an HTTP `POST` with a JSON body:

```json
{ "event": "ticket.created", "occurred_at": "2026-10-01T12:00:00Z", "data": { ... } }
```

Headers on each delivery:

| Header | Value |
|--------|-------|
| `X-Webhook-Event` | Event name |
| `X-Webhook-Delivery` | Delivery ID |
| `X-Webhook-Signature` | `sha256=<hex>`: HMAC-SHA256 of the raw body, keyed by the secret. Only sent when a secret is set |
| `User-Agent` | `GoatFlow-Webhook/1.0` |

Events are found and delivered by the runner process (`goats -mode runner`). With no runner running, no webhooks are sent.

### API tokens

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/api/v1/tokens` | agent | List your tokens |
| POST | `/api/v1/tokens` | agent | Create a token. The token value is shown once |
| DELETE | `/api/v1/tokens/:id` | agent | Revoke your token |
| GET | `/api/v1/tokens/scopes` | agent | List scopes |
| GET | `/api/v1/admin/tokens` | admin | All tokens |
| DELETE | `/api/v1/admin/tokens/:id` | admin | Revoke any token |
| GET, POST | `/api/v1/admin/users/:userId/tokens` | admin | List or create tokens for an agent |
| DELETE | `/api/v1/admin/users/:userId/tokens/:tokenId` | admin | Revoke an agent's token |
| GET, POST | `/api/v1/admin/customer-users/:customerId/tokens` | admin | List or create tokens for a customer |
| DELETE | `/api/v1/admin/customer-users/:customerId/tokens/:tokenId` | admin | Revoke a customer's token |
| GET, POST | `/customer/api/v1/tokens` | customer | List or create your own tokens |
| DELETE | `/customer/api/v1/tokens/:id` | customer | Revoke your token |
| GET | `/customer/api/v1/tokens/scopes` | customer | List scopes |

Send a token as `Authorization: Bearer gf_...`. The `admin:*` scope is needed for admin routes, and only works when the token owner is an admin.

### Canned responses

Agents only. Prefix `/api/canned-responses` (`routes/api-canned-responses.yaml`).

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/canned-responses` | List (filters: `category`, `scope`, `search`, `tags`, `sort_by`, `sort_order`, `limit`) |
| POST | `/api/canned-responses` | Create |
| GET | `/api/canned-responses/:id` | Get |
| PUT | `/api/canned-responses/:id` | Update |
| DELETE | `/api/canned-responses/:id` | Delete |
| GET | `/api/canned-responses/popular` | Most used first |
| GET | `/api/canned-responses/categories` | Categories |
| GET | `/api/canned-responses/category/:category` | By category |
| GET | `/api/canned-responses/search?q=` | Search name or content |
| GET | `/api/canned-responses/user` | Responses for the current user |
| GET | `/api/canned-responses/statistics` | Usage of your responses |
| GET | `/api/canned-responses/export` | Export as JSON, or CSV with `format=csv` |
| POST | `/api/canned-responses/import` | Import personal responses from CSV (form field `file`) |
| POST | `/api/canned-responses/:id/use` | Fill in placeholders and count the use |
| POST | `/api/canned-responses/:id/share` | Change scope (`global` needs admin) |
| POST | `/api/canned-responses/:id/copy` | Copy into your personal scope |

### Self-service (password reset and sign-up)

These are public HTML form pages, not JSON APIs (`routes/selfservice.yaml`). They answer 404 while their feature switch is off.

| Method | Path | Feature switch | Description |
|--------|------|----------------|-------------|
| GET, POST | `/forgot-password` | `features.lost_password` | Agent asks for a reset link |
| GET, POST | `/reset-password` | `features.lost_password` | Agent sets a new password (token from email) |
| GET, POST | `/customer/forgot-password` | `features.lost_password` | Customer asks for a reset link |
| GET, POST | `/customer/reset-password` | `features.lost_password` | Customer sets a new password |
| GET, POST | `/customer/register` | `features.registration` | Customer sign-up; sends a confirmation email |
| GET, POST | `/customer/register/complete` | `features.registration` | Confirm email and choose a password |

Reset links last 1 hour. Sign-up links last 24 hours. Emails need `BASE_URL` to be set.

### Translations (i18n)

Public, registered in Go (`internal/platform/api/i18n_handlers.go`).

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/i18n/languages` | Supported languages |
| POST | `/api/v1/i18n/language` | Set language |
| GET | `/api/v1/i18n/translations[/:lang]` | Translations |
| POST | `/api/v1/i18n/translate` | Translate a key |
| GET | `/api/v1/i18n/stats` | Language statistics |
| GET | `/api/v1/i18n/coverage` | Coverage per language |
| GET | `/api/v1/i18n/missing/:lang` | Missing keys |
| GET | `/api/v1/i18n/export/:lang` | Export a language |
| GET | `/api/v1/i18n/validate/:lang` | Validate a language |

### Plugins

Registered in Go (`internal/api/plugin_handlers.go`).

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/api/v1/plugins` | agent | List plugins |
| GET | `/api/v1/plugins/health` | agent | Plugin health |
| GET | `/api/v1/plugins/widgets` | agent | Dashboard widgets |
| GET | `/api/v1/plugins/:name/widgets/:id` | agent | Render a widget |
| GET | `/api/v1/plugins/:name/events/:channel` | agent | Plugin event stream (SSE) |
| POST | `/api/v1/plugins/:name/call/:fn` | admin | Call a plugin function |
| POST | `/api/v1/plugins/:name/enable`, `/disable`, `/reset-crashloop` | admin | Lifecycle |
| DELETE | `/api/v1/plugins/:name` | admin | Uninstall |
| POST | `/api/v1/plugins/upload` | admin | Upload a plugin |
| GET, DELETE | `/api/v1/plugins/logs` | admin | Plugin logs |
| GET | `/api/v1/plugins/marketplace`, `/marketplace/search` | admin | Marketplace |
| POST | `/api/v1/plugins/marketplace/install` | admin | Install from marketplace |
| GET | `/api/v1/sse` | logged in | Plugin event stream |

### Admin

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/admin/sql` | Run a read-only SQL query |

Admin user management is under [Users and groups](#users-and-groups) and [API tokens](#api-tokens).

## MCP Server (AI Integration)

See [MCP.md](MCP.md) for full documentation.

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/mcp` | JSON-RPC 2.0 endpoint (needs a token) |
| GET | `/api/mcp` | Server info (public) |

- Each tool call runs with the caller's own permissions.
- Tools are made from every `/api/v1` endpoint and enabled plugin route.
- Admin tools (for example `create_admin_sql`) need admin group membership. API tokens also need the `admin:*` scope.

```bash
curl -X POST https://your-goatflow/api/mcp \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

## Health Checks

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/health` | public | Basic health check (use this for probes) |
| GET | `/healthz` | public | Liveness probe |
| GET | `/health/detailed` | admin | Build info, uptime, cache state |
| GET | `/metrics` | admin | Prometheus metrics |

To scrape metrics without a login, set `METRICS_ENABLED=true`. GoatFlow then serves Prometheus metrics on a separate port, `METRICS_PORT` (default `9090`).

## Rate Limiting

| What | Limit | Source |
|------|-------|--------|
| Login: `/api/v1/auth/login`, customer portal login, passkey login | 5 failed attempts in 5 minutes for the same IP and login name, then a growing wait (2 s up to 60 s) | `internal/platform/auth/login_ratelimit.go` |
| Forgot-password and sign-up forms | 10 posts per IP per hour; 3 emails per account or address per hour | `internal/selfservice/types.go` |
| Public plugin UIs | 60 requests per minute per IP by default (plugin can set `rate_limit`) | `internal/platform/pluginui/router.go` |

There is no general rate limit on the REST API.

## See Also

- [Developer Guide](../developer-guide/README.md)
- [Security Documentation](../SECURITY.md)
- [LDAP Integration](../LDAP.md)
- [Plugin Host API](../plugins/HOST_API.md)
