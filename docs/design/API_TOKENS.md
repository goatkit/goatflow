# API Tokens (Personal Access Tokens)

## Overview

Enable users (customers AND agents) to create API tokens for programmatic access. Tokens inherit user permissions but can be scoped down. Supports AI/MCP integrations, automation scripts, and third-party tools.

## User Stories

- **Customer**: "I want to connect my AI assistant to check my support tickets"
- **Agent**: "I want to automate bulk ticket updates via script"
- **Admin**: "I want CI/CD to deploy and check system health"

## Database Schema

```sql
-- API tokens for programmatic access
CREATE TABLE user_api_tokens (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id INT NOT NULL,                    -- FK to users table
    user_type ENUM('agent', 'customer') NOT NULL DEFAULT 'agent',
    
    -- Token identification
    name VARCHAR(100) NOT NULL,              -- User-friendly label
    prefix VARCHAR(8) NOT NULL,              -- First 8 chars (for UI identification)
    token_hash VARCHAR(255) NOT NULL,        -- bcrypt hash of full token
    
    -- Permissions (NULL = inherit all user permissions)
    scopes JSON,                             -- e.g., ["tickets:read", "tickets:write"]
    
    -- Lifecycle
    expires_at TIMESTAMP NULL,               -- NULL = never expires
    last_used_at TIMESTAMP NULL,
    last_used_ip VARCHAR(45),
    
    -- Rate limiting
    rate_limit INT DEFAULT 1000,             -- Requests per hour (generous default)
    
    -- Audit
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by INT,
    revoked_at TIMESTAMP NULL,               -- Soft delete
    revoked_by INT,
    
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    INDEX idx_prefix (prefix),
    INDEX idx_user (user_id, user_type),
    INDEX idx_active (revoked_at, expires_at)
);

-- Rate limit tracking (in Valkey/Redis, not SQL)
-- Key: api_token_rate:{token_id}
-- Value: request count
-- TTL: 1 hour (rolling window)
```

## Token Format

```
gf_<prefix>_<random>

Example: gf_EXAMPLE_FAKE_TOKEN_DO_NOT_USE_1234567890ab
         ├─────────┤ └──────────────────────────────────┘
         Prefix (8)   Random (32+ chars)
         (shown in UI) (never shown again after creation)
```

- Prefix `gf_` = GoatFlow token (easy to identify in logs)
- First 8 chars stored as `prefix` for UI display
- Full token shown ONCE at creation, then only hash stored

## Scopes

Hierarchical scope system:

| Scope | Description |
|-------|-------------|
| `*` | Full access (user's complete RBAC) |
| `tickets:read` | View tickets (own queue for customers) |
| `tickets:write` | Create/update tickets |
| `tickets:delete` | Delete tickets (if RBAC allows) |
| `articles:read` | Read ticket articles |
| `articles:write` | Add articles/replies |
| `users:read` | View user info (self for customers) |
| `admin:*` | Admin operations (agents only) |
| `queues:read` | View queue info |

**Scope enforcement:**
- Requested scope must be ≤ user's RBAC permissions
- Customer tokens auto-filtered to own tickets regardless of scope

## API Endpoints

### Agent/Admin Token Management

```
GET    /api/v1/tokens              # List my tokens
POST   /api/v1/tokens              # Create token
GET    /api/v1/tokens/:id          # Get token info (no secret)
DELETE /api/v1/tokens/:id          # Revoke token
```

### Customer Token Management

```
GET    /customer/api/v1/tokens     # List my tokens
POST   /customer/api/v1/tokens     # Create token
DELETE /customer/api/v1/tokens/:id # Revoke token
```

### Admin: Manage All Tokens

```
GET    /api/v1/admin/tokens                    # List all tokens
GET    /api/v1/admin/tokens/user/:user_id      # List user's tokens
DELETE /api/v1/admin/tokens/:id                # Revoke any token
```

## Request/Response Examples

### Create Token

**Request:**
```json
POST /api/v1/tokens
{
  "name": "My AI Assistant",
  "scopes": ["tickets:read", "tickets:write"],
  "expires_in": "90d"  // or "30d", "1y", "never"
}
```

**Response (ONLY time full token is shown):**
```json
{
  "id": 42,
  "name": "My AI Assistant",
  "prefix": "a1b2c3d4",
  "token": "gf_EXAMPLE_FAKE_TOKEN_DO_NOT_USE_1234567890ab",
  "scopes": ["tickets:read", "tickets:write"],
  "expires_at": "2026-05-04T09:18:00Z",
  "created_at": "2026-02-04T09:18:00Z",
  "⚠️ warning": "Save this token now. It won't be shown again."
}
```

### List Tokens

```json
GET /api/v1/tokens
{
  "tokens": [
    {
      "id": 42,
      "name": "My AI Assistant",
      "prefix": "a1b2c3d4",
      "scopes": ["tickets:read", "tickets:write"],
      "expires_at": "2026-05-04T09:18:00Z",
      "last_used_at": "2026-02-04T08:30:00Z",
      "created_at": "2026-02-04T09:18:00Z"
    }
  ]
}
```

## Authentication Flow

```
┌─────────────────────────────────────────────────────────┐
│  Request with: Authorization: Bearer gf_EXAMPLE_xxx    │
└─────────────────────────┬───────────────────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────┐
│  1. Extract prefix (a1b2c3d4)                           │
│  2. Look up tokens with that prefix                     │
│  3. bcrypt verify full token against hash               │
│  4. Check: not revoked, not expired                     │
│  5. Check: rate limit (Valkey counter)                  │
│  6. Load user + RBAC permissions                        │
│  7. Filter permissions by token scopes                  │
│  8. Attach to request context                           │
└─────────────────────────┬───────────────────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────┐
│  Request proceeds with scoped user context              │
└─────────────────────────────────────────────────────────┘
```

## Rate Limiting

**Default limits (generous):**
- Standard token: 1,000 requests/hour
- Admin can adjust per-token

**Implementation:**
```go
// Valkey key: api_token_rate:{token_id}
// INCR + EXPIRE 3600
count := valkey.Incr(ctx, fmt.Sprintf("api_token_rate:%d", token.ID))
if count == 1 {
    valkey.Expire(ctx, key, time.Hour)
}
if count > token.RateLimit {
    return 429 Too Many Requests
}
```

**Response headers:**
```
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 847
X-RateLimit-Reset: 1707041880
```

## UI Mockups

### Agent Profile → API Tokens Tab

```
┌─────────────────────────────────────────────────────────┐
│ API Tokens                              [+ New Token]   │
├─────────────────────────────────────────────────────────┤
│ ┌─────────────────────────────────────────────────────┐ │
│ │ 🔑 My AI Assistant                                  │ │
│ │    gf_a1b2c3d4_••••••••                            │ │
│ │    Scopes: tickets:read, tickets:write              │ │
│ │    Expires: May 4, 2026 · Last used: 2 hours ago    │ │
│ │                                        [Revoke]     │ │
│ └─────────────────────────────────────────────────────┘ │
│ ┌─────────────────────────────────────────────────────┐ │
│ │ 🔑 CI/CD Pipeline                                   │ │
│ │    gf_x9y8z7w6_••••••••                            │ │
│ │    Scopes: * (full access)                          │ │
│ │    Expires: Never · Last used: Yesterday            │ │
│ │                                        [Revoke]     │ │
│ └─────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

### Create Token Modal

```
┌─────────────────────────────────────────────────────────┐
│ Create API Token                                    [X] │
├─────────────────────────────────────────────────────────┤
│ Name: [My AI Assistant_________________]                │
│                                                         │
│ Expiration: [90 days ▼]                                │
│   ○ 30 days  ○ 90 days  ○ 1 year  ○ Never              │
│                                                         │
│ Scopes:                                                 │
│   ☑ tickets:read    Read tickets                        │
│   ☑ tickets:write   Create/update tickets               │
│   ☐ articles:write  Add replies                         │
│   ☐ admin:*         Admin operations                    │
│                                                         │
│                              [Cancel]  [Create Token]   │
└─────────────────────────────────────────────────────────┘
```

## Security Considerations

1. **Token storage**: Only bcrypt hash stored, never plaintext
2. **One-time display**: Full token shown once at creation
3. **Prefix identification**: 8-char prefix for UI without exposing token
4. **Scope limitation**: Can't exceed user's RBAC
5. **Customer isolation**: Customer tokens always filtered to own data
6. **Audit trail**: Created/revoked timestamps and actors
7. **Rate limiting**: Prevents abuse
8. **IP logging**: Last used IP for security review

## Implementation Order

1. [ ] Database migration (schema)
2. [ ] Token model + repository
3. [ ] Token generation/verification service
4. [ ] Auth middleware extension (detect `gf_` prefix)
5. [ ] Rate limiting middleware
6. [ ] API endpoints (agent)
7. [ ] API endpoints (customer)
8. [ ] Admin management endpoints
9. [ ] Agent UI (profile → tokens tab)
10. [ ] Customer UI (portal → tokens)
11. [ ] Scope enforcement in existing handlers

## MCP Integration Example

Once tokens exist, users can configure MCP servers:

```json
{
  "mcpServers": {
    "goatflow": {
      "command": "npx",
      "args": ["goatflow-mcp"],
      "env": {
        "GOATFLOW_URL": "https://support.example.com",
        "GOATFLOW_TOKEN": "gf_EXAMPLE_xxx..."
      }
    }
  }
}
```

Then ask their AI: "Show me my open tickets" or "Create a ticket about X"

---

*Design: 2026-02-04*
