# Host API Reference

The Host API is how plugins talk to GoatFlow. The Go interface is `HostAPI` in `pkg/plugin/plugin.go`.

Both runtimes support every method with the same host function names and the same JSON arguments and results. One dispatcher (`DispatchHostCall` in `internal/platform/plugin/hostcall.go`) serves both:

- **gRPC plugins** call the host through `pkg/plugin/grpcutil.HostAPIClient`, which sends the host function name and JSON arguments over RPC.
- **WASM plugins** call the `gk.host_call` host function with the same name and JSON arguments. An unknown name returns no result (the host logs `unknown host API method`).

Every plugin call reaches the host through the plugin's **SandboxedHostAPI**, which checks permissions and applies rate limits. See [Sandboxing & Permissions](#sandboxing--permissions).

## Runtime support

| Method | Host function name | gRPC | WASM | Permission check |
|--------|-------------------|------|------|------------------|
| `DBQuery` | `db_query` | yes | yes | `db` read (tables written by the statement need `db` write) |
| `DBExec` | `db_exec` | yes | yes | `db` write |
| `CacheGet` | `cache_get` | yes | yes | `cache` read |
| `CacheSet` | `cache_set` | yes | yes | `cache` write |
| `CacheDelete` | `cache_delete` | yes | yes | `cache` write |
| `HTTPRequest` | `http_request` | yes | yes | `http` |
| `SendEmail` | `send_email` | yes | yes | `email` |
| `Log` | `log` (WASM also: separate `gk.log` export) | yes | yes | none |
| `ConfigGet` | `config_get` | yes | yes | `config` read |
| `Translate` | `translate` | yes | yes | none |
| `CallPlugin` | `plugin_call` | yes | yes | `plugin_call` |
| `PublishEvent` | `publish_event` | yes | yes | none (own plugin's channels only) |
| `EntitySoftDelete` | `entity_soft_delete` | yes | yes | `entity` write |
| `EntityRestore` | `entity_restore` | yes | yes | `entity` write |
| `EntityHardDelete` | `entity_hard_delete` | yes | yes | `entity` `hard_delete` (admin policy only) |
| `RecycleBinList` | `recycle_bin_list` | yes | yes | `entity` read |
| `SecureConfigGet` | `secure_config_get` | yes | yes | none (own plugin's keys only) |
| `SecureConfigSet` | `secure_config_set` | yes | yes | none (own plugin's keys only) |
| `OrgID` | `org_id` | yes | yes | none |
| `CustomFieldsGet` | `custom_fields_get` | yes | yes | DB rate limit |
| `CustomFieldsSet` | `custom_fields_set` | yes | yes | DB rate limit (own plugin's fields only) |
| `CustomFieldsQuery` | `custom_fields_query` | yes | yes | DB rate limit |
| `StoreFile` | `store_file` | yes | yes | `file` write (own plugin's files only) |
| `GetFile` | `get_file` | yes | yes | `file` read (own plugin's files only) |
| `DeleteFile` | `delete_file` | yes | yes | `file` write (own plugin's files only) |
| `ListFiles` | `list_files` | yes | yes | `file` read (own plugin's files only) |
| `GenerateThumbnail` | `generate_thumbnail` | yes | yes | none |
| `CreateArticleAttachment` | `create_article_attachment` | yes | yes | `article` write |
| `ListArticleAttachments` | `list_article_attachments` | yes | yes | `article` read |
| `DeleteArticleAttachment` | `delete_article_attachment` | yes | yes | `article` write |
| `CreateArticle` | `create_article` | yes | yes | `article` write |
| `ChangeTicketStatus` | `change_ticket_status` | yes | yes | `ticket` write |
| `ListTicketStates` | `list_ticket_states` | yes | yes | `ticket` read |
| `ListTicketViews` | `list_ticket_views` | yes | yes | `ticket` read |
| `RenderMarkdownToPdf` | `render_markdown_to_pdf` | yes | yes | none |
| (no Go method) | `time_now` | yes | yes | none |

Notes:

- **time_now:** returns `{"now": "<RFC 3339 time>"}` in the server's local time zone, offset included. Meant for WASM plugins: TinyGo has no time zone data, and ticket times are stored in server-local time.
- "none" in the last column means the sandbox adds no permission check. The host still records the calling plugin.

## Call context and acting user

Every plugin call the host makes for a request (API routes, admin `call`, widgets, plugin UI pages and their nav badges, setup tasks, MCP tools, event authorizers) carries the host's call envelope in its arguments: `_user_id`, `_user_login`, `_user_email`, `_customer_login`, `_user_role`, `_is_admin`, `_org_id` / `org_id` and `_lang`. The host builds these from the authenticated session and drops any client-supplied value under the same names first, so a request body cannot name another user, role, organisation or language.

While the call runs, the host puts three envelope values into the call context:

- `_user_id` is the **acting user**: `EntitySoftDelete`, `EntityRestore`, `EntityHardDelete` and `SecureConfigSet` are recorded as that agent (recycle bin, deletion log, `change_by`).
- `_org_id` is the call's **organisation**: `OrgID` returns it, secure config and plugin files are kept per organisation, and the sandbox scopes queries on organisation-owned tables to it (see [Organisation](#organisation)). A plugin that sets `SkipOrgInjection` gets no `_org_id` and runs without an organisation.
- `_lang` is the call's **language** for `Translate`.

When a call has no acting agent, the host records the action as the system user (`users.id` 1, `plugin.SystemUserID`). That is the case for scheduled jobs, plugin `Init` and migrations, calls made for a customer (`_customer_login` set or role `Customer`) and gRPC callbacks made without the call context. Scheduled jobs, `Init` and migrations also run without an organisation (they serve every organisation). Plugin-to-plugin calls run in the calling plugin's call context; the host never reads envelope keys from arguments one plugin passes to another.

Methods that take the user as an argument (`CreateArticle`, `CreateArticleAttachment`, `ChangeTicketStatus`) record the user you pass. Pass the call's `_user_id`.

- **WASM plugins** always run their host calls in the call context.
- **gRPC plugins** get it by implementing `grpcutil.GKPluginWithContext`. The host then calls `CallWithContext(ctx, fn, args)` instead of `Call`. Pass that `ctx` (or a context derived from it) to the HostAPI methods: the host then runs them with the call's acting user, organisation, language and deadline. A HostAPI call made with another context (for example `context.Background()`) has no call context (system user, no organisation, `en`), and one made after the plugin call returned is refused.

## Database

### DBQuery

Execute a read-only SQL query.

```go
DBQuery(ctx context.Context, query string, args ...any) ([]map[string]any, error)
```

**Parameters:**
- `query` — SQL SELECT statement with `?` placeholders
- `args` — Values for placeholders

**Returns:**
- Array of row maps (column name → value). `[]byte` values are converted to strings.
- Error if query fails or permission denied

**SQL Portability:**
Queries are automatically processed through `ConvertPlaceholders()` for MySQL/PostgreSQL compatibility. Always use `?` placeholders — never use PostgreSQL-style `$1`.

**Multi-Database:**
Prefix query with `@dbname:` to target a named database (e.g., `@analytics:SELECT...`). Default database is used if no prefix.

**Example:**
```go
rows, err := host.DBQuery(ctx,
    "SELECT id, title, state_id FROM tickets WHERE queue_id = ? AND state_id IN (?, ?)",
    5, 1, 4)
// rows = [{"id": 1, "title": "Issue", "state_id": 1}, ...]
```

**Permission required:** `db` with `read` (or `readwrite`) access

**Organisation scoping:** see [Organisation](#organisation). `DBExec` is scoped the same way.

---

### DBExec

Execute a write SQL statement (INSERT, UPDATE, DELETE).

```go
DBExec(ctx context.Context, query string, args ...any) (int64, error)
```

**Parameters:**
- `query` — SQL statement with `?` placeholders
- `args` — Values for placeholders

**Returns:**
- `int64` — Number of rows affected
- Error if execution fails or permission denied

**DDL Protection:** Plugins without `write` access are blocked from executing DDL statements (DROP, ALTER, TRUNCATE, CREATE, GRANT, REVOKE).

**Table Whitelisting:** If the plugin's `db` permission has a scope (e.g. `["tickets", "queue"]`), table names are extracted from the query and validated against the allowlist. Queries touching tables outside the scope are rejected with an error.

**Example:**
```go
affected, err := host.DBExec(ctx,
    "UPDATE tickets SET priority_id = ? WHERE id = ?",
    3, 123)
// affected = 1
```

**Permission required:** `db` with `write` (or `readwrite`) access

---

## Cache

Fast in-memory caching via Redis/Valkey with TTL.

**Important:** Cache keys are **automatically namespaced** per plugin. When you set key `"stats"`, it's stored as `"plugin:my-plugin:stats"`. This prevents cross-plugin key collisions — you don't need to prefix keys yourself.

### CacheGet

Retrieve a cached value.

```go
CacheGet(ctx context.Context, key string) ([]byte, bool, error)
```

**Returns:**
- `[]byte` — Value if found
- `bool` — `true` if found and not expired
- `error` — Error if permission denied

**Example:**
```go
if data, found, err := host.CacheGet(ctx, "dashboard_stats"); found {
    return data, nil
}
```

**Permission required:** `cache` with `read` (or `readwrite`) access

---

### CacheSet

Store a value with expiration.

```go
CacheSet(ctx context.Context, key string, value []byte, ttlSeconds int) error
```

**Example:**
```go
stats, _ := json.Marshal(computeStats())
host.CacheSet(ctx, "dashboard_stats", stats, 300) // 5 minutes
```

**Permission required:** `cache` with `write` (or `readwrite`) access

---

### CacheDelete

Remove a cached value.

```go
CacheDelete(ctx context.Context, key string) error
```

**Permission required:** `cache` with `write` (or `readwrite`) access

---

## HTTP

### HTTPRequest

Make an outbound HTTP request.

```go
HTTPRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) (int, []byte, error)
```

**Parameters:**
- `method` — HTTP method (GET, POST, PUT, DELETE, etc.)
- `url` — Full URL including protocol
- `headers` — Request headers (may be nil)
- `body` — Request body (nil for GET)

**Returns:**
- `int` — HTTP status code
- `[]byte` — Response body
- `error` — Error if request fails or permission denied

**Example:**
```go
statusCode, body, err := host.HTTPRequest(ctx, "POST", "https://api.example.com/webhook",
    map[string]string{
        "Content-Type":  "application/json",
        "Authorization": "Bearer secret",
    },
    []byte(`{"event": "ticket.created"}`))
```

**Permission required:** `http`

**URL Filtering:** If the plugin's policy specifies HTTP scope patterns (e.g., `["*.example.com"]`), only matching URLs are allowed. Wildcard `*.example.com` matches any subdomain.

**Notes:**
- Default timeout: 30 seconds (enforced by host)

---

## Email

### SendEmail

Send an email using the host's configured email provider.

```go
SendEmail(ctx context.Context, to, subject, body string, html bool) error
```

**Parameters:**
- `to` — Recipient email address
- `subject` — Email subject
- `body` — Email body (plain text or HTML)
- `html` — `true` for HTML body

**Example:**
```go
err := host.SendEmail(ctx, "user@example.com", "Report Ready",
    "<h1>Your report is ready</h1>", true)
```

**Permission required:** `email`

**Domain Scoping:** If the plugin's `email` permission has a scope (e.g. `["@example.com", "@mycompany.com"]`), only recipients at those domains are allowed. Domain patterns match the email suffix — `@example.com` allows `user@example.com`, `admin@example.com`, etc.

**Rate Limiting:** Email sending is rate-limited to 10 emails per minute per plugin (sliding window), independent of the general rate limits.

---

## File Storage

### GenerateThumbnail

Generate an image thumbnail server-side. The plugin does not need libvips — the host handles all image processing.

```go
GenerateThumbnail(ctx context.Context, data []byte, contentType string, maxWidth, maxHeight int) (thumbData []byte, thumbContentType string, err error)
```

**Parameters:**
- `data` — Raw image bytes
- `contentType` — MIME type (jpeg, png, gif, webp, avif, heic, tiff)
- `maxWidth` — Max bounding-box width (aspect ratio preserved)
- `maxHeight` — Max bounding-box height (aspect ratio preserved)

**Returns:**
- `thumbData` — Thumbnail bytes
- `thumbContentType` — Output format (`"image/jpeg"` or `"image/png"`)
- `error` — Error if generation fails or service not configured

**Example:**
```go
thumb, ct, err := host.GenerateThumbnail(ctx, imgBytes, "image/png", 300, 300)
if err != nil {
    // Service not configured — fall back to the original / CSS-scaled thumbnail.
    return imgBytes, "image/png", nil
}
```

**Notes:**
- Uses libvips Lanczos3 downscale for high-quality resampling (intended behaviour; govips v2.x migration pending in `internal/service/thumbnail_service.go`).
- Non-image content types return a placeholder icon based on the content type.
- Returns the error `"thumbnail service not configured"` when the host has no `ThumbnailGenerator` wired (e.g. a build without the `vips-dev` C headers); the plugin should fall back to serving the original or a CSS-scaled thumbnail. The KB plugin decodes originals via blob URLs in this mode.
- Pair with `StoreFile` / `GetFile` to persist the generated thumbnail and reference it by key.

**Permission required:** none — always allowed (the sandbox stamps the caller identity but applies no permission gate, like `Log`/`Translate`).

---

## Logging

### Log

Write a structured log entry. **Always allowed** — no permission check.

```go
Log(ctx context.Context, level string, message string, fields map[string]any)
```

**Levels:** `debug`, `info`, `warn`, `error`

The plugin name is automatically added to the `fields` map. Logs are written to both the structured logger and the in-memory log buffer (visible in Admin → Plugins → View Logs).

**Example:**
```go
host.Log(ctx, "info", "Processing webhook", map[string]any{
    "webhook_id": "wh_123",
    "event_type": "ticket.created",
})
```

---

## Configuration

### ConfigGet

Read host configuration values.

```go
ConfigGet(ctx context.Context, key string) (string, error)
```

**Available Keys:**
- `app.name` — Application name
- `app.timezone` — Timezone
- `app.env` — Environment (development, production, etc.)

**Permission required:** `config` with `read` access

**Sensitive Key Blocking:** The following key patterns are blocked by default and return an error, even with `config` permission: `database.*`, `db.*`, `mysql.*`, `postgres.*`, `smtp.*`, `mail.*`, `secret`, `password`, `credential`, `token`, `key`, `private`, `auth`, `session`, `cookie`, `ldap.*`, `oauth.*`, `saml.*`, `aws.*`, `gcp.*`, `azure.*`, `cloud.*`. If the `config` permission has a scope, only keys matching the scope patterns are allowed (overrides the default blacklist).

---

## i18n

### Translate

Translate a key to the current locale. **Always allowed** — no permission check.

```go
Translate(ctx context.Context, key string, args ...any) string
```

The host reads the language from the call context (`PluginLanguageKey`, set from the call's `_lang` for every request-driven call: API routes, widgets, plugin UI pages, setup tasks, MCP tools) and falls back to `en`. WASM plugins always have the call context; gRPC plugins have it when they pass the `ctx` from `CallWithContext` (see [Call context and acting user](#call-context-and-acting-user)), otherwise the host uses `en`.

Every plugin call from a route or UI page also carries the user's language in `args["_lang"]` (from `?lang=`, cookie, user preference or Accept-Language). Plugins that ship their own translations can look strings up with `_lang`. Declaring the maps in `I18nSpec` still registers them with the host catalogue. See [AUTHOR_GUIDE.md](AUTHOR_GUIDE.md).

**Example:**
```go
title := host.Translate(ctx, "my_plugin.widget_title")
```

---

## Plugin Interop

### CallPlugin

Call a function in another plugin.

```go
CallPlugin(ctx context.Context, pluginName string, fn string, args json.RawMessage) (json.RawMessage, error)
```

**Example:**
```go
args, _ := json.Marshal(map[string]any{"period": "day"})
result, err := host.CallPlugin(ctx, "stats", "get_ticket_stats", args)
```

**Permission required:** `plugin_call`

**Scope:** If the plugin's policy specifies a `plugin_call` scope (e.g., `["stats"]`), only listed plugins can be called. Without scope, all plugins are callable.

**Call Depth Limit:** Plugin-to-plugin call chains are limited to a maximum depth of 10. This prevents infinite recursion when Plugin A calls Plugin B which calls Plugin A. The depth is tracked via context and incremented on each cross-plugin call. Exceeding the limit returns: `plugin call depth exceeded (max 10): pluginA -> pluginB`

**Notes:**
- Target plugin must be loaded and enabled
- Lazy loading is attempted if the target isn't loaded yet
- Caller plugin name is tracked for better error messages and stamped by the host (plugins can't impersonate each other)


---

## Events (SSE)

### PublishEvent

Send an event to browsers that are listening on one of your plugin's channels.

```go
PublishEvent(ctx context.Context, channel string, eventType string, data string) error
```

- `channel` - your channel name, for example `"status"`.
- `eventType` - the SSE event name, for example `"device-table"`.
- `data` - the payload, usually an HTML fragment.

Browsers subscribe at `GET /api/v1/plugins/<plugin>/events/<channel>` (agents only). Returns `SSE broker not available` when the host has no broker.

By default every logged-in agent can subscribe to every channel of your plugin. To limit this, set `EventAuthorizer` in your `GKRegistration` to the name of a plugin function. The host calls it with `{"channel": "<channel>"}` plus the caller fields (`_user_id`, `_user_role`, `_is_admin`, `_org_id`, ...). Return `{"allow": true}` to allow. Any other answer is a `403`; an error is a `502`. Details: [AUTHOR_GUIDE.md](AUTHOR_GUIDE.md).

---

## Entity Deletion

Entity types with a delete handler (`internal/platform/deletion/types.go`): `ticket`, `contact`, `agent`, `customer_group`.

| Method | What it does |
|--------|--------------|
| `EntitySoftDelete(ctx, entityType, entityID, reason) error` | Move to the recycle bin and anonymise personal data |
| `EntityRestore(ctx, entityType, entityID) error` | Restore from the recycle bin |
| `EntityHardDelete(ctx, entityType, entityID, reason) error` | Remove the entity and its linked data for good |
| `RecycleBinList(ctx, entityType) (json.RawMessage, error)` | List soft-deleted entities as JSON |

The host records these actions as the call's acting agent, or as the system user when the call has none (see [Call context and acting user](#call-context-and-acting-user)).

Permissions: `RecycleBinList` needs `entity` read; `EntitySoftDelete` and `EntityRestore` need `entity` write; `EntityHardDelete` needs `entity` with access `hard_delete`. `readwrite` does not include `hard_delete`, and the host never grants `hard_delete` from a plugin's own declarations: an admin must store a policy that grants it. The `entity` scope lists the entity types the plugin may touch (for example `["ticket"]`); no scope means all types.

---

## Secure Config

Encrypted key/value storage for secrets such as API keys.

| Method | What it does |
|--------|--------------|
| `SecureConfigGet(ctx, key) (string, error)` | Read and decrypt a value. Error if the key is not set |
| `SecureConfigSet(ctx, key, value) error` | Encrypt and store a value |

- Keys belong to the calling plugin and to the current organisation. A plugin cannot read another plugin's keys.
- Values are encrypted with the key in `GOATFLOW_SECURE_KEY` (hex).

---

## Organisation

### OrgID

```go
OrgID(ctx context.Context) int64
```

Returns the call's organisation (the caller's active organisation, from the call envelope's `_org_id`), or `0` when there is none: single-organisation mode, a caller with no active organisation, a scheduled job, or a plugin with `SkipOrgInjection`.

### What is scoped to the organisation

When the call has an organisation, the host keeps these apart per organisation:

- **Secure config** keys (`SecureConfigGet` / `SecureConfigSet`).
- **Plugin files** (`<plugin>/org-<id>/<key>`).
- **Queries on organisation-owned tables.** `DBQuery` and `DBExec` on `gk_org_plugin_access`, `gk_user_organisation`, `sysconfig_org` and `gk_identity_provider_org` get `AND org_id = ?` added (`WHERE org_id = ?` when there is no `WHERE`). Statements that already mention `org_id`, `INSERT`s and DDL are left alone.

Nothing else is scoped. Core tables such as `ticket`, `queue`, `article` and `customer_user` have no `org_id` column, so plugin queries on them return every organisation's rows. Plugin-owned tables are not scoped either: a plugin that keeps data per organisation stores `org_id` itself and filters on the call's `_org_id` / `OrgID`.

---

## Custom Fields

Read and write custom field values on GoatKit entities. Field names are prefixed with your plugin name by the host; you use the short name.

| Method | What it does |
|--------|--------------|
| `CustomFieldsGet(ctx, entityType, objectID, fields []string) (map[string]any, error)` | Get values. `nil` fields means all fields |
| `CustomFieldsSet(ctx, entityType, objectID, values map[string]any) error` | Set values. Types and rules are checked first |
| `CustomFieldsQuery(ctx, entityType, filters []CustomFieldFilter) ([]int64, error)` | Find object IDs by field values |

`CustomFieldsSet` accepts a `plugin.FieldOp` as a value for an atomic change:

| `Op` | Effect |
|------|--------|
| `increment` | Add `Value` to a number. Optional `Floor` / `Ceiling`; the update is rejected if it would pass them |
| `append` | Add `Value` to a `multi_select` field |
| `remove` | Remove `Value` from a `multi_select` field |
| `cas` | Set to `Value` only if the current value equals `Expect` |
| `toggle` | Flip a boolean |

`CustomFieldFilter` fields: `Field`, `Operator` (`eq`, `neq`, `gt`, `lt`, `gte`, `lte`, `like`, `in`, `between`, `near`), `Value`, `Value2` (upper bound for `between`, radius in km for `near`).

These calls count against the plugin's DB query rate limit.

---

## Plugin Files

Files are stored under the plugin's own namespace (`<plugin>/<key>`, or `<plugin>/org-<id>/<key>` inside an organisation). `StoreFile` and `DeleteFile` need `file` write; `GetFile` and `ListFiles` need `file` read.

| Method | What it does |
|--------|--------------|
| `StoreFile(ctx, key, data, metadata) error` | Store a file. `metadata` is optional |
| `GetFile(ctx, key) ([]byte, map[string]string, error)` | Read a file and its metadata |
| `DeleteFile(ctx, key) error` | Delete a file |
| `ListFiles(ctx, prefix) ([]FileInfo, error)` | List files under a prefix, for example `"backups/"` |

- `FileInfo` fields: `Key`, `Size`, `ContentType`, `Metadata`, `ModifiedAt`.
- Storage limit: 500 MB per plugin by default. An admin can change it with `MaxFileStorageBytes` in the plugin's resource policy.
- By default files are stored on disk under `$STORAGE_PATH/plugins` (`STORAGE_PATH` defaults to `/app/storage`).

---

## Articles and Attachments

| Method | What it does |
|--------|--------------|
| `CreateArticle(ctx, ticketID, createdBy, subject, body, visibleToCustomer) (int64, error)` | Add an article (internal channel, agent sender) to a ticket. Returns the article ID. Use this instead of writing article rows with SQL |
| `CreateArticleAttachment(ctx, articleID, createdBy, filename, contentType, content) (int64, error)` | Attach a file to an article. Returns the attachment ID |
| `ListArticleAttachments(ctx, articleID) ([]ArticleAttachment, error)` | List attachments (metadata only) |
| `DeleteArticleAttachment(ctx, articleID, attachmentID) error` | Remove one attachment |

- `createdBy` is a `users.id`.
- Attachments can be at most `storage.attachments.max_size` bytes (default 10 MB).
- `ArticleAttachment` fields: `ID`, `ArticleID`, `Filename`, `ContentType`, `Size`, `URL`. Use the `URL` the host gives you; add `/view` for the viewer or `/thumbnail` for a preview. Do not build the URL yourself.
- `CreateArticle`, `CreateArticleAttachment` and `DeleteArticleAttachment` need `article` write; `ListArticleAttachments` needs `article` read.

---

## Ticket States and Views

| Method | What it does |
|--------|--------------|
| `ChangeTicketStatus(ctx, ticketID, stateID, userID, untilTime) error` | Change a ticket's state. Pending states need `untilTime > 0` (Unix seconds); other states clear the pending time |
| `ListTicketStates(ctx) ([]TicketStateInfo, error)` | All valid states, ordered by ID. Fields: `ID`, `Name`, `Color`, `TypeID`, `TypeName` |
| `ListTicketViews(ctx) ([]TicketViewInfo, error)` | Ticket views declared by enabled plugin UIs. Fields: `PluginName`, `UIID`, `Label`, `URL` (contains `{ticket_id}`) |

`ChangeTicketStatus` needs `ticket` write; `ListTicketStates` and `ListTicketViews` need `ticket` read.

---

## PDF Rendering

### RenderMarkdownToPdf

Turns Markdown into PDF bytes using the headless Chromium sidecar (Browserless).

```go
RenderMarkdownToPdf(ctx context.Context, markdown string, options PdfRenderOptions) ([]byte, error)
```

`PdfRenderOptions`:

| Field | Meaning |
|-------|---------|
| `PageSize` | `"A4"` (default) or `"Letter"` |
| `MarginMM` | Page margin in mm (default 15) |
| `Title` | Shown in the PDF header when set |
| `BrandName` | Name shown in the running header |
| `BrandColor` | `#RRGGBB` colour for headings and table headers. Other values are ignored |
| `BrandLogoURL` | Logo for the header as an inline `data:image/png`, `jpeg`, `gif` or `webp` base64 URI. Any other value (including `https` URLs) is ignored |

The Markdown is sanitized (bluemonday UGC policy) before printing, and the renderer never fetches anything remote: images print only from inline `data:` URIs (png, jpeg, gif, webp). Any other image source, whether `https`, `http`, protocol-relative or relative, is replaced by an empty image. To print a remote image, fetch it with `HTTPRequest` (your `http` grant applies) and embed the bytes as a `data:` URI. Links stay links.

The sidecar address comes from `BROWSERLESS_URL` (default `http://127.0.0.1:3000`) and `BROWSERLESS_TOKEN`.

---

## Sandboxing & Permissions

Every plugin receives a `SandboxedHostAPI` that wraps the real HostAPI with enforcement. The Manager hands it to the plugin's `Init`; for gRPC plugins every host callback from the plugin process goes through it (callbacks before `Init` are refused).

### Permission Enforcement

Each HostAPI call checks the plugin's `ResourcePolicy` before executing. If the permission isn't granted, the call returns an error like:

```
plugin "my-plugin": database write access not granted
```

### Rate Limiting

Three sliding-window rate limiters (configurable per plugin via policy):

| Limiter | Default | Scope |
|---------|---------|-------|
| DB queries/min | 600 | DBQuery + DBExec |
| HTTP requests/min | 60 | HTTPRequest |
| Calls/sec | 100 | All Call() invocations |

Exceeding a limit returns an error:
```
plugin "my-plugin": DB query rate limit exceeded
```

### Resource Accounting

All operations are counted via atomic counters:
- `DBQueries`, `DBExecs`, `CacheOps`, `HTTPRequests`, `Calls`, `Errors`
- `LastCallAt` timestamp

Admins can view these stats via the plugin management API.

### Permissions in a policy

A permission is `{type, access, scope}` (see `Permission` in `pkg/plugin/plugin.go`). Access is `read`, `write` or `readwrite`. Several entries of the same type add up: each entry grants its access on its own scope, so a plugin can have `db` readwrite on `["gk_coach_*"]` and `db` read on `["users"]`. No scope means no restriction for that type and access.

For `db`, the sandbox reads the tables out of the SQL (FROM/JOIN/USING lists, INSERT/UPDATE/DELETE targets, DDL targets, quoted and schema-qualified names; CTE names and string literals are ignored). Tables the statement writes need a `db` write grant, also when sent through `DBQuery`; every other table needs a `db` read grant. `information_schema` is readable with any `db` read grant. Scope patterns match the whole name; `*` matches any run of characters.

For `http`, a scope entry is an exact host, `*.example.com` (the domain and its subdomains) or `*` (any host).

### Effective policy

| Situation | Permissions the plugin gets |
|-----------|-----------------------------|
| An admin stored a policy (`Plugin::<name>::Policy` in sysconfig, `Manager.SetPolicy`) | Exactly that policy |
| No stored policy, the plugin declares permissions in `GKRegister` → `Resources.Permissions` | Exactly the declared permissions, except `entity` `hard_delete` |
| No stored policy, nothing declared | `DefaultResourcePolicy`: `db` read, `cache` readwrite, `file` readwrite |

Without a stored policy the declarations are read again on every plugin (re)load, so a new plugin version's declarations take effect when it is deployed. Rate limits and timeouts come from the stored policy, or from `DefaultResourcePolicy` (256 MB memory, 30s call timeout, 100 calls/sec, 600 DB queries/min, 60 HTTP requests/min).

### Policy Status

| Status | Effect |
|--------|--------|
| `pending_review` | No admin decision yet (the status of policies built from declarations or defaults) |
| `approved` | Admin-granted permissions |
| `restricted` | Limited by admin |
| `blocked` | All permission-checked HostAPI calls denied |

---

## Error Handling

All Host API functions can return errors. Always check:

```go
result, err := host.DBQuery(ctx, query, args...)
if err != nil {
    host.Log(ctx, "error", "Query failed", map[string]any{"error": err.Error()})
    return nil, fmt.Errorf("database error: %w", err)
}
```

### Common Errors

| Error | Cause |
|-------|-------|
| `permission denied` / `access not granted` | Missing permission in policy |
| `rate limit exceeded` | Too many calls in the time window |
| `DDL statements not permitted` | Write SQL without write permission |
| `HTTP access to "..." not permitted` | URL not in allowed scope |
| `not permitted to call plugin "..."` | Target not in plugin_call scope |
| `plugin "..." is disabled` | Target plugin disabled by admin |
| `access to table "..." not permitted` | Table not in DB permission scope |
| `plugin call depth exceeded (max 10)` | Too many nested plugin-to-plugin calls |
| `email recipient "..." not allowed` | Recipient domain not in email scope |
| `email rate limit exceeded` | More than 10 emails/minute |
| `config access not granted` / blocked key | Key matches sensitive pattern or not in scope |

---

## Context

The `context.Context` passed to handlers carries:

- Request timeout/deadline
- Language preference (via `PluginLanguageKey`, from the call's `_lang`)
- Acting user (`plugin.ActingUserID(ctx)`, from the call's `_user_id`)
- Organisation (`OrgID(ctx)`, from the call's `_org_id`)
- Caller plugin name (via `PluginCallerKey`, for plugin-to-plugin calls)
