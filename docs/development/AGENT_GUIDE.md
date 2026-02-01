# AGENT.md — Engineering Assistant Operating Manual

Status: Canonical. This document supersedes CLAUDE.md.

Purpose: Provide clear, enforceable rules and a practical workflow for engineering assistants working in the GOTRS codebase. Follow this document as the single source of truth for operating procedures, quality bars, and guardrails.

## Golden Rules
- **All operations in containers**: Go toolchain and database clients are not installed on host. Use `make toolbox-*` targets for Go operations and `make db-*` targets for database operations. Never attempt to run `go`, `mysql`, or `psql` commands directly on host.
- Containers-first: Run builds, tests, and tools in containers. Use Makefile targets; do not bypass with ad‑hoc docker/podman commands unless mirroring Makefile behavior.
- **CRITICAL - Container lifecycle**: NEVER use `make up` - it blocks the terminal forever. Always use:
  - `make up-d` - start containers in detached mode (backgrounds immediately)
  - `make restart` - rebuild and restart containers (for code changes)
  - `make down` - stop containers
  - `make logs` - view logs
- SQL portability: Always wrap SQL strings with `database.ConvertPlaceholders(...)`. Never use raw `$n` placeholders directly.
- Schema freeze: Do not modify existing OTRS tables and do not add new tables before v1.0. See [SCHEMA_FREEZE.md](../architecture/SCHEMA_FREEZE.md).
- Security first: Rootless containers, Alpine base, SELinux labels, secrets only via environment variables (generated via synthesize). Never hardcode secrets.
- No self-attribution: Do not add assistant/AI attribution to commits, code, or docs. Follow repository commit conventions.
- TDD discipline: Write tests where practicable, run them, verify passing before claiming completion.
- Professional UX: No browser dialogs; use branded toasts/modals. Ensure dark mode and accessibility standards.
- Templating policy: Use Pongo2 templates only; never use Go's `html/template`. Do not generate HTML in handlers for user-facing views; render via Pongo2 with base layout and proper context.
- Routing policy: Define all HTTP routes in `routes/*.yaml` (YAML router). Do not register routes directly in Go code.

## Required Workflow
1. Plan (if multi-step): Outline non-trivial tasks and confirm scope.
2. **Go operations**: Use toolbox container for all Go work:
   - Build check: `make toolbox-compile` to ensure the repo compiles
   - Module management: `make toolbox-exec ARGS="go mod tidy"`
   - Code generation/formatting: `make toolbox-exec ARGS="go generate ./..."`
3. **Database operations**: Use make targets for all database work:
   - Database shell: `make db-shell` (automatically detects driver and uses correct credentials)
   - Database queries: `echo "SELECT * FROM table;" | make db-shell`
   - Database migrations: `make db-migrate` or `make db-migrate-schema-only`
4. Service lifecycle:
   - `make restart`
   - Health check: `curl -sf http://localhost:8080/health`
   - Logs sanity: `make logs | tail -200` (ensure no panic/errors)
4. Tests:
   - Unit/integration: `make test`
   - If failures: fix locally and rerun until green
5. Browser verification (for UI):
   - Open target pages, check Console and Network tabs (no errors/500s)
   - Exercise full workflow (create/edit/delete, save/refresh)
6. Only then report status. Be explicit about what is tested vs. pending.

## Database Access Patterns
Always use the placeholder wrapper for cross‑database compatibility (PostgreSQL/MySQL):

```go
rows, err := db.Query(
    database.ConvertPlaceholders(`
        SELECT id, title FROM ticket WHERE queue_id = $1
    `),
    queueID,
)
```

Do not write raw queries with `$n` placeholders without the conversion wrapper. Centralize queries in repositories; avoid SQL in handlers.

### Dynamic SQL with QueryBuilder (REQUIRED)
For **dynamic WHERE clauses**, **variable column selection**, or **IN lists**, use the sqlx-based QueryBuilder (`internal/database/querybuilder.go`). This pattern is **mandatory** for security compliance (gosec G201/G202):

```go
// ❌ WRONG - triggers gosec G201 SQL injection warning
query := fmt.Sprintf("SELECT id, name FROM users WHERE %s = $%d", column, argCount)

// ✅ CORRECT - use QueryBuilder for dynamic SQL
qb, _ := database.GetQueryBuilder()
sb := qb.NewSelect("id", "name").From("users").Where("status = ?", status)
if orgID != 0 {
    sb.Where("org_id = ?", orgID)
}
query, args, _ := sb.ToSQL()
rows, err := db.Query(database.Rebind(query), args...)
```

**For IN clauses**, use `database.In()` to expand slices:
```go
query, args, _ := database.In("SELECT * FROM tickets WHERE id IN (?)", ids)
rows, err := db.Query(database.Rebind(query), args...)
```

**Key methods:**
- `NewSelect(columns...).From(table)` - start a SELECT query
- `.Where(condition, args...)` - add WHERE clause (can chain multiple)
- `.Join(joinClause)` - add JOIN
- `.OrderBy(clause)`, `.Limit(n)`, `.Offset(n)` - pagination
- `.ToSQL()` - returns query string and args slice
- `database.Rebind(query)` - converts `?` to `$1, $2...` for PostgreSQL
- `database.In(query, args...)` - expands slices for IN clauses

### Row Iteration with rows.Err() (REQUIRED)
After iterating over `sql.Rows` with `for rows.Next()`, you **must** check `rows.Err()`. Errors during iteration (network issues, encoding problems) are stored and only accessible via `rows.Err()`:

```go
// ❌ WRONG - iteration errors silently lost
for rows.Next() {
    rows.Scan(&item)
    results = append(results, item)
}
return results, nil

// ✅ CORRECT - check rows.Err() after loop
for rows.Next() {
    rows.Scan(&item)
    results = append(results, item)
}
if err := rows.Err(); err != nil {
    return nil, err
}
return results, nil
```

**Preferred: Use helper functions** from `internal/database/rows.go`:
```go
// CollectRows handles iteration and rows.Err() automatically
users, err := database.CollectRows(rows, func(r *sql.Rows) (*User, error) {
    var u User
    err := r.Scan(&u.ID, &u.Name)
    return &u, err
})

// CollectStrings/CollectInts for simple single-column queries
names, err := database.CollectStrings(rows)
ids, err := database.CollectInts(rows)
```

## MariaDB CRUD Patterns (CRITICAL)
**Unit tests mock the database - they will NOT catch these errors. Always follow these patterns.**

### INSERT - Use Exec + LastInsertId (NOT QueryRow + RETURNING)
```go
// ❌ WRONG - PostgreSQL RETURNING doesn't work in MariaDB
var id int
err = db.QueryRow(database.ConvertPlaceholders(`
    INSERT INTO table (col1, col2) VALUES ($1, $2) RETURNING id
`), val1, val2).Scan(&id)

// ✅ CORRECT - Use Exec + LastInsertId
result, err := db.Exec(database.ConvertPlaceholders(`
    INSERT INTO table (col1, col2) VALUES ($1, $2)
`), val1, val2)
id, _ := result.LastInsertId()
```

### INSERT - Include All NOT NULL Timestamp Columns
**Most OTRS tables have NOT NULL create_time, create_by, change_time, change_by columns:**
```go
// ❌ WRONG - Missing timestamp columns causes "Field doesn't have default value" error
INSERT INTO standard_attachment (name, content, valid_id)
VALUES ($1, $2, $3)

// ✅ CORRECT - Include all NOT NULL columns
INSERT INTO standard_attachment (name, content, valid_id, create_time, create_by, change_time, change_by)
VALUES ($1, $2, $3, NOW(), 1, NOW(), 1)
```

### Pre-Implementation Checklist (Before ANY INSERT)
1. Run `make db-query QUERY="DESCRIBE table_name"` to see all columns
2. Identify all NOT NULL columns without defaults
3. Include `create_time`, `create_by`, `change_time`, `change_by` if they exist
4. Use `db.Exec()` + `LastInsertId()`, NOT `QueryRow()` + `RETURNING`
5. Wrap ALL queries with `database.ConvertPlaceholders()`

## Service Health Verification (After Route/Handler/Config Changes)
- Build: `make toolbox-compile`
- Restart: `make restart`
- Health: `curl -sf http://localhost:8080/health`
- Logs: `make logs | grep -E "(panic|error)" | tail -5`

Common issues: duplicate route registration, unused imports, nil dereferences. Fix and re‑run the protocol before proceeding.

## Routing Configuration (YAML)
- Source: `routes/*.yaml` files are loaded at startup by the YAML router.
- Policy: Do not register routes in Go code; declare/modify them in YAML.
- Changes: Edit YAML, then run build/restart/health protocol above.
- Warnings: Duplicate path+method combinations cause startup panics.

## UI Quality Bar (Baseline)
- Search/filter where applicable, with clear/reset
- Sortable columns where appropriate
- Branded modals/dialogs (submit on Enter), no native browser dialogs
- Error handling with friendly messages and focus management
- Loading states and success feedback
- Dark mode parity and responsive layout
- Accessibility: keyboard navigation, ARIA labels
- State persistence: preserve search/filter state across operations

## Pongo2 Template Gotchas
- Template inheritance paths are relative to the templates root, not the file: use `layouts/base.pongo2`.
- Filters use colon syntax, e.g., `default:"-"`. There is no `|string` or `|json` filter.
- Compare like types (string vs string, int vs int). Convert in handler if needed.
- If the page renders but looks wrong, check logs for template errors and browser console for JS errors.

## Form Submission Pattern (Checkbox Matrix)
Prefer URL-encoded form payloads for checkboxes to ensure consistent server parsing:

```javascript
const params = new URLSearchParams();
for (const cb of document.querySelectorAll('input[type="checkbox"][name^="perm_"]')) {
  params.append(cb.name, cb.checked ? '1' : '0');
}
fetch(url, {
  method: 'PUT',
  headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
  body: params.toString()
});
```

Avoid `FormData` for checkbox matrices when the backend expects `application/x-www-form-urlencoded`.

## Navigation & Theming Requirements (Admin Pages)
- Always render via base layout with correct context:
  - Provide `User` and `ActivePage` to enable nav visibility/highlighting
- Match global styling and dark mode; avoid direct HTML generation in handlers for user-facing views
- Ensure clear navigation out of the page (breadcrumbs/back links)

## Commit & PR Guidance
- Conventional commits (`feat:`, `fix:`, `docs:`, etc.)
- Focus messages on the “why” and scope, not implementation detail
- Never include assistant/AI attribution in commits or PRs
- When introducing route or behavior changes, briefly note testing steps (build, restart, health, logs, UI path)

## Development Environment
**Go toolchain and database clients are NOT installed on host system.** All development operations must use containers:

### Go Operations (Toolbox Container)
- **Build/Compile**: `make toolbox-compile`
- **Module management**: `make toolbox-exec ARGS="go mod tidy"`
- **Code generation**: `make toolbox-exec ARGS="go generate ./..."`
- **Formatting**: `make toolbox-exec ARGS="goimports -w ."`
- **Linting**: `make toolbox-exec ARGS="golangci-lint run"`
- **Interactive shell**: `make toolbox-run` (for complex multi-step operations)

### Database Operations (Make Targets)
- **Database shell**: `make db-shell` (driver-aware, uses correct credentials automatically)
- **Run SQL queries**: `echo "SELECT * FROM users;" | make db-shell`
- **Single query execution**: `make db-query QUERY="SELECT COUNT(*) FROM tickets"`
- **Database migrations**: `make db-migrate` (full migration with test data)
- **Schema only migration**: `make db-migrate-schema-only` (schema + initial data, no test data)
- **Fix sequences**: `make db-fix-sequences` (PostgreSQL only, after data imports)

**Never run `go` commands directly on host** - they will fail with "command not found".

## Go Performance Anti-Patterns (AVOID)

### Slice Preallocation (REQUIRED when size is known)
When building a slice in a loop where the final size is known or estimable, **always preallocate**:

```go
// ❌ WRONG - causes multiple reallocations and GC pressure
var results []Item
for _, src := range items {
    results = append(results, transform(src))
}

// ✅ CORRECT - single allocation, no reallocations
results := make([]Item, 0, len(items))
for _, src := range items {
    results = append(results, transform(src))
}
```

**Why it matters**: Without preallocation, Go doubles the backing array each time capacity is exceeded. For 1000 items, this means ~10 allocations, 10 copy operations, and 10 arrays for GC to clean up. With preallocation: 1 allocation, 0 copies, 0 GC pressure.

**Impact**: 2-10x speedup in hot paths, noticeably snappier UI for list rendering, search results, and bulk operations.

**golangci-lint**: The `prealloc` linter catches these. Run `make toolbox-exec ARGS="golangci-lint run"` to find violations.

### String Concatenation in Loops (AVOID)
```go
// ❌ WRONG - O(n²) allocations
var result string
for _, s := range parts {
    result += s
}

// ✅ CORRECT - O(n) with single final allocation
var b strings.Builder
b.Grow(estimatedSize) // optional but helps
for _, s := range parts {
    b.WriteString(s)
}
result := b.String()
```

## Makefile Targets (Common)
- `make up` / `make up-d`: start services (foreground/background)
- `make down`: stop services
- `make restart`: restart backend service
- `make logs` / `make backend-logs`: view logs
- `make db-shell`: open database shell (MySQL/PostgreSQL, driver-aware with automatic credentials)
- `make test`: run tests in containers
- `make toolbox-compile`: compile all packages inside toolbox container
- `make toolbox-exec ARGS="go mod tidy"`: run Go module management commands in container (tidy, download, etc.)
- `make frontend-build`: build CSS and JavaScript assets (Tailwind + esbuild)
- `make tiptap-build`: build only Tiptap JavaScript bundle for debugging
- `make frontend-clean-build`: clean and rebuild all frontend assets
- `make frontend-dev`: start frontend development server with hot reload

**DANGER**: Never use `docker compose down -v` - the `-v` flag removes ALL volumes including the dev database. Profile flags (`--profile testdb`) do NOT reliably isolate volume removal. If you need to reset the test database, use `make test-db-reset` or manually remove only test volumes.

**Note**: `css-deps` uses `npm-check-updates` which may upgrade Tailwind CSS to v4, causing build failures. Pin Tailwind to `^3.4.0` in package.json and avoid `npm-check-updates` for frontend dependencies.

### Container-First Enforcement Helpers
To keep drift from reintroducing host `go` usage low:

- Macro: `TOOLBOX_GO` (defined in `Makefile`) wraps commands: `$(MAKE) toolbox-exec ARGS=`. Use it only in simple targets; avoid nesting it inside already long `podman run` invocations.
- Verification: `make verify-container-first` runs `scripts/tools/check-container-go.sh` and fails if raw host `go` or `golangci-lint` lines are detected (tab-prefixed) in the `Makefile`.
- Acceptable exceptions: Inside a single explicit `gotrs-toolbox:latest` container run block (already containerized), direct `go build/test` is fine—do not wrap again.
- Add new Go-related targets by default via `toolbox-exec` pattern; if performance requires a single large container run, keep all `go` invocations inside that one block.

Checklist before committing new Go targets:
1. No plain `\tgo test` or `\tgo build` lines unless inside an existing `podman/docker run gotrs-toolbox` block.
2. `make verify-container-first` returns green.
3. For multi-step script-like flows prefer a dedicated script invoked via `toolbox-exec` instead of many Makefile inline commands.
4. CI runs `Container-First Guard` workflow on PRs/push to block violations automatically.

## Troubleshooting Checklist
- **Go command fails**: Go is not installed on host. Use `make toolbox-exec ARGS="go <command>"` instead
- **Database connection fails**: Database clients not installed on host. Use `make db-shell` for interactive access or pipe SQL to it
- **Wrong database credentials**: Never hardcode credentials. Use `make db-shell` which gets credentials from environment/Makefile variables
- Build fails: run `make toolbox-compile` and read the first error; fix from top to bottom
- Service panic: `make logs | tail -200`; search for duplicate routes or nil dereferences
- UI mismatch after save: verify network request payload and response; refresh view state after save
- SQL errors on MySQL: confirm `database.ConvertPlaceholders` usage and SQL syntax portability
- Missing assets: verify static route points to `./static` not `./web/static`

## Caching (Go & Tooling)
Persistent named volumes are used for Go build cache, module cache, and golangci-lint cache to speed up iterative development.

Targets:
- `make go-cache-info` / `make go-cache-clean`
- `make lint-cache-info` / `make lint-cache-clean`
- `make cache-clean-all` (aggregate purge; leaves node_modules intact)

Environment paths (inside toolbox):
- Build cache: `/workspace/.cache/go-build`
- Module cache: `/workspace/.cache/go-mod`
- Lint cache: `/workspace/.cache/golangci-lint`

Do not manually delete these inside containers; prefer the Make targets to keep workflow consistent.

### Cache Ownership & Guard Policy
- Canonical cache roots live under `/workspace/.cache/*` (build, mod, golangci-lint).
- The `cache_guard` runs automatically on major targets; it warns if any entries are owned by a UID/GID other than the invoking developer (UID 1000 inside containers by convention).
- Use `make cache-audit` to list ownership anomalies (foreign UID/GID first, then full listing).
- Use `make toolbox-fix-cache` to conditionally normalize ownership (only runs chown/chmod when mismatches exist).
- Avoid running ad hoc root containers that write into bind-mounted cache directories; if unavoidable, run `make toolbox-fix-cache` afterward.
- Do not rely on `chmod 777`; we now prefer `775` after normalization for least‑permissive collaborative access.
- If a workflow needs to bypass the guard (e.g. diagnosing container image layers), set `CACHE_GUARD_DISABLE=1` when invoking the Make target (e.g. `CACHE_GUARD_DISABLE=1 make toolbox-test`).

## Testing Controls (Prevent Recurring Issues)

### Route Registry Pattern (MANDATORY for Admin Modules)
**Problem**: Tests define their own routes that diverge from production, causing 404s in browser.

**Solution**: Use centralized route definitions in `internal/api/test_router_registry.go`:

```go
// In test_router_registry.go:
func GetAdminRolesRoutes() []AdminRouteDefinition {
    return []AdminRouteDefinition{
        {"GET", "/roles", handleAdminRoles},
        {"POST", "/roles", handleAdminRoleCreate},
        // ... all routes
    }
}

// In test file:
func setupRoleTestRouter() *gin.Engine {
    return SetupTestRouterWithRoutes(GetAdminRolesRoutes())
}

// In htmx_routes.go:
RegisterAdminRoutes(adminRoutes, GetAdminRolesRoutes())
```

**Enforcement**: Never manually register routes in test setup functions. Always use `SetupTestRouterWithRoutes()` with the module's `Get*Routes()` function.

### JavaScript API Module Pattern (MANDATORY for JSON Endpoints)
**Problem**: Handler checks `Accept: application/json` header to decide JSON vs HTML response. Go test sends header correctly, but inline JS fetch() in templates doesn't. Result: passes in Go test, fails in browser with "<!DOCTYPE... is not valid JSON".

**Root Cause**: Go unit tests cannot verify JavaScript behavior. Inline JS in templates is untestable.

**Solution**: Use the `adminApi` module in `web/src/api/adminApi.ts`:

```typescript
// adminApi.ts enforces required headers automatically
import { rolesApi } from '@/api/adminApi';

// BAD - inline fetch missing headers (untestable)
const response = await fetch(`/admin/roles/${roleId}/users`);

// GOOD - use API module (tested via make test-frontend)
const result = await rolesApi.getUsers(roleId);
```

**JS Unit Tests** in `web/src/api/adminApi.test.ts`:
- Verify all API calls include `Accept: application/json`
- Verify POST/PUT include `Content-Type: application/json`  
- Verify correct field names (`comments` not `description`, `valid_id` not `is_active`)
- Run via: `make test-frontend`

**Endpoint Contracts** in `test_router_registry.go` document expected headers per endpoint for reference.

### JSON Field Contract (MANDATORY)
**Problem**: JavaScript sends different field names than Go handler expects (e.g., `description` vs `comments`).

**Solution**: Define a contract struct comment in the handler and reference it in templates:

```go
// handleAdminRoleCreate creates a new role
// JSON Contract: { name: string (required), comments: string, valid_id: int }
func handleAdminRoleCreate(c *gin.Context) {
    var input struct {
        Name     string `json:"name" binding:"required"`
        Comments string `json:"comments"`
        ValidID  int    `json:"valid_id"`
    }
```

**Enforcement**: When creating/modifying JS fetch calls, verify field names match handler's JSON tags exactly.

### Pre-Module Checklist
Before starting any new admin module:

1. [ ] Create `Get<Module>Routes()` function in handler file
2. [ ] Create `Get<Module>Contracts()` function in test_router_registry.go
3. [ ] Register routes in `htmx_routes.go` via `RegisterAdminRoutes()`
4. [ ] Test setup uses `SetupTestRouterWithRoutes(Get<Module>Routes())`
5. [ ] Contract tests pass: `go test -run TestContracts`
6. [ ] Document JSON contract in handler comments
7. [ ] Template JS field names match handler JSON tags
8. [ ] Template JS fetch calls include headers per contract
9. [ ] Run browser test after unit tests pass (not just "tests pass")

### E2E Verification (Non-Negotiable)
Unit tests CANNOT catch:
- Route registration mismatches (404 in browser)
- JS/Go field name mismatches (JSON parse errors)
- Missing Accept headers (HTML returned instead of JSON)
- Template rendering issues

**After all unit tests pass**, you MUST:
1. `make restart`
2. Open the page in browser
3. Exercise the full workflow (create/edit/delete)
4. Check browser Console for errors
5. Check Network tab for failed requests

Only then report "feature complete".

### Template Testing (MANDATORY for Forms)
**Problem**: Templates with HTMX attributes (`hx-post`, `hx-put`) or form actions can have path mismatches that unit tests don't catch (e.g., using `/api/dynamic-fields` when route is `/admin/api/dynamic-fields`).

**Solution**: Template tests in `internal/template/` validate HTMX attributes and form actions:

```go
// Test framework in internal/template/pongo2_test.go
helper := NewTemplateTestHelper(t)
html, err := helper.RenderTemplate("pages/admin/my_form.pongo2", ctx)
asserter := NewHTMLAsserter(t, html)

// For create forms
asserter.HasHTMXPost("/admin/api/my-resource")
asserter.HasNoHTMXPut()

// For edit forms  
asserter.HasHTMXPut("/admin/api/my-resource/42")
asserter.HasNoHTMXPost()
```

**Test Files**:
- `internal/template/pongo2_test.go` - Test helper and HTML asserter utilities
- `internal/template/all_templates_test.go` - Comprehensive template tests + coverage scanner
- `internal/template/dynamic_fields_template_test.go` - Example module-specific tests

**Coverage Scanner**: `TestTemplateCoverage` in `all_templates_test.go` scans all templates and fails if any template with `hx-post`, `hx-put`, `hx-delete`, or `method="POST"` is not in the `testedTemplates` map.

**When Adding New Templates with Forms**:
1. Add template path to `testedTemplates` map in `all_templates_test.go`
2. Create test function that renders template and asserts correct HTMX/action attributes
3. Run `make test-templates` to verify

**Test Execution Order**: Template tests run **first** in `make test` for fail-fast behavior:
```
1. Template tests (internal/template/...) - ~30ms
2. Core packages
3. Integration tests
4. E2E Playwright tests
```

**Quick Validation**: `make test-templates` runs only template tests.


## Dynamic Fields System

### Architecture
Dynamic fields allow administrators to add custom fields to tickets, articles, and customer records without schema changes.

- **Field storage**: `dynamic_field` table with YAML config column
- **Screen visibility**: `dynamic_field_screen_config` table controls which fields appear on which screens
- **Cross-package wiring**: `DynamicFieldLoader` interface in `internal/routing/handlers.go` avoids import cycles; set in `internal/api/dynamic_field_init.go`

### Screen Configuration (IMPORTANT)
Fields **only appear on forms** if they have a screen config entry:
- Query uses `INNER JOIN` on `dynamic_field_screen_config`
- No config entry = field not displayed on that screen
- Config values: `0`=disabled, `1`=enabled, `2`=required

### Admin Workflow
1. Create field: `/admin/dynamic-fields` → "New Dynamic Field"
2. Enable for screens: `/admin/dynamic-fields/{id}/screens` → check boxes for target screens
3. Field now appears on those screens

### Screen Keys (OTRS Compatible)
| Screen Key | Where It Appears |
|------------|------------------|
| `AgentTicketPhone` | `/tickets/new` (agent new ticket) |
| `AgentTicketEmail` | Email ticket creation |
| `AgentTicketZoom` | Ticket detail view |
| `AgentTicketNote` | Add note form |
| `AgentTicketClose` | Close ticket dialog |
| `AgentTicketPending` | Set pending dialog |
| `CustomerTicketMessage` | Customer reply form |

### Template Integration
Include the partial in ticket forms:
```django
{% include "partials/dynamic_fields.pongo2" with DynamicFields=DynamicFields %}
```

Handler must load fields via `GetFieldsForScreenWithConfig(screenKey, objectType)` or use the `dynamicFieldLoader` callback.

### Troubleshooting
- **Fields not appearing**: Check `/admin/dynamic-fields/{id}/screens` - field must be enabled for the target screen
- **Field appears but no label**: Missing `Label` in dynamic_field record
- **DB error**: Ensure migration 000004 has run (`dynamic_field_screen_config` table exists)

## Legal & Compliance
- GOTRS-CE is an original implementation; maintain compatibility without copying upstream code
- Keep all secrets in environment variables; generate via project tooling; do not commit

## This Document vs CLAUDE.md
This document replaces CLAUDE.md as the authoritative operating guide for engineering assistants. CLAUDE.md remains for historical context only.

## ENTITY SELECTION MODAL UX BLUEPRINT - MANDATORY FOR ALL DIALOGS (Jan 12, 2026)

**This is the gold standard for entity selection modals (add users to role, assign agents to queue, etc.)**

Reference implementation: `templates/pages/admin/roles.pongo2` - roleUsersModal

### Modal Structure

```
+----------------------------------------------------------+
| [Icon] Modal Title                              [X Close] |
| Optional description/context text                         |
+----------------------------------------------------------+
| CURRENT MEMBERS                                           |
| [Filter members...] (local filter, instant)               |
| +------------------------------------------------------+ |
| | Member 1                              [Remove]       | |
| | Member 2                              [Remove]       | |
| +------------------------------------------------------+ |
+----------------------------------------------------------+
| ADD NEW MEMBERS                                           |
| [Search...] (API search, debounced)    [Spinner] [Enter] |
| +------------------------------------------------------+ |
| | Search Result 1                       [+ Add]        | |
| | Search Result 2                       [+ Add]        | |
| +------------------------------------------------------+ |
+----------------------------------------------------------+
| [Undo Toast - appears on remove, 5 second timeout]       |
+----------------------------------------------------------+
```

### API Design Pattern

```go
// Search endpoint - scalable, never returns all records
GET /admin/{entity}/:id/{members}/search?q={query}

// Requirements:
// - Minimum 2 characters required
// - Maximum 20 results returned
// - Excludes already-assigned members
// - Searches multiple fields (name, email, login, etc.)
// - Returns JSON: [{id, display_name, detail_info}, ...]
```

### JavaScript Patterns

```javascript
// 1. DEBOUNCED SEARCH (300ms delay)
let searchTimeout;
input.addEventListener('input', function() {
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => performSearch(this.value), 300);
});

// 2. LOCAL MEMBER CACHE (for filtering and undo)
let currentMembers = []; // Populated on modal open
function filterMembers(query) {
    // Filter cached members client-side - instant response
}

// 3. OPTIMISTIC UI UPDATES
async function addMember(id) {
    // 1. Add to UI immediately
    appendMemberToList(member);
    // 2. Clear from search results
    removeFromSearchResults(id);
    // 3. KEEP search query (don't clear input)
    // 4. Call API in background
    const response = await fetch(...);
    if (!response.ok) {
        // 5. Rollback on failure
        removeMemberFromList(id);
        showError('Failed to add');
    }
}

// 4. UNDO PATTERN FOR DESTRUCTIVE ACTIONS
async function removeMember(id) {
    const member = getMemberData(id);
    // 1. Hide from UI immediately (don't delete)
    hideMemberRow(id);
    // 2. Show undo toast
    showUndoToast(member, () => {
        // Undo callback - restore UI
        showMemberRow(id);
    });
    // 3. Set delayed actual deletion
    undoTimeout = setTimeout(async () => {
        await fetch(`DELETE /api/.../${id}`);
        actuallyRemoveFromDOM(id);
    }, 5000);
}

// 5. KEYBOARD NAVIGATION
document.addEventListener('keydown', (e) => {
    if (!modalIsOpen) return;
    if (e.key === 'Escape') closeModal();
    if (e.key === 'Enter' && searchHasResults()) {
        e.preventDefault();
        addFirstSearchResult();
    }
});
```

### CSS/Visual Patterns

```css
/* Add button - green on hover */
.add-btn:hover { @apply bg-green-100 text-green-700; }

/* Remove button - red on hover */
.remove-btn:hover { @apply bg-red-100 text-red-700; }

/* Row animations */
.member-row {
    transition: all 0.2s ease-out;
}
.member-row.removing {
    opacity: 0;
    transform: translateX(-10px);
}
.member-row.adding {
    animation: slideIn 0.2s ease-out;
}

/* Undo toast - fixed bottom */
.undo-toast {
    @apply fixed bottom-4 right-4 bg-gray-800 text-white 
           px-4 py-3 rounded-lg shadow-lg flex items-center gap-3;
}
```

### UX Requirements Checklist

1. **Header**: Icon + Title + X close button (top-right)
2. **Member Filter**: Local filtering of cached members (instant)
3. **Search Input**: 
   - Minimum 2 characters
   - 300ms debounce
   - Loading spinner while searching
   - "Press Enter to add first result" hint
4. **Search Results**: Max 20 results, excludes existing members
5. **Add Action**:
   - Optimistic UI (instant feedback)
   - KEEP search query after adding
   - Green hover state on button
6. **Remove Action**:
   - Undo toast with 5-second window
   - Delayed actual deletion
   - Red hover state on button
7. **Keyboard**: Escape to close, Enter to add first result
8. **Animations**: Slide in/out on add/remove
9. **Empty States**: Show helpful messages when no members/results
10. **Error Handling**: Rollback UI on API failure, show toast

### NEVER DO THIS

- Load ALL available entities into the DOM (use search API)
- Clear search input after adding (user may want to add more)
- Delete immediately without undo option
- Use browser confirm() dialogs
- Block UI during API calls (use optimistic updates)
- Forget keyboard navigation
- Skip loading indicators during search

**Every entity selection modal in the product MUST follow this pattern.**

---

## TESTING INFRASTRUCTURE - MEMORIZE THIS (Jan 11, 2026)

**We have a FULL test stack with a dedicated database.**

### Test Database Setup
- Dedicated test database container available
- Tests run WITH a real database, not mocks
- Seed stage populates baseline data before tests
- After each test, database resets to baseline for next test
- Run tests with: `make test`

### How to Write Tests
1. Use the real database connection - DO NOT mock the database
2. Seed data is available - use it
3. Database resets between tests - each test starts clean
4. Integration tests should use the actual DB, not be skipped

### Makefile Targets for Testing
- `make test` - brings up test stack and runs all tests
- `make toolbox-test` - runs tests in toolbox container with DB access
- `make db-shell-test` - access the database directly

### NEVER DO THIS
- Don't write tests that skip because "no DB connection"
- Don't mock database calls when real DB is available. Spoiler: REAL test db is available.
- Don't claim low coverage is acceptable because "DB required"
- Don't use `// +build integration` tags to skip DB tests

**The test database EXISTS. Use it.**

---

## YAML ROUTING - SINGLE SOURCE OF TRUTH (Jan 27, 2026)

**There is ONE YAML route loader. Tests use the same router as production.**

### The Single Router

All YAML route loading goes through `internal/routing/loader.go`:

```go
// For production (main.go)
routing.LoadYAMLRoutesFromGlobalMap(router, routesPath)

// For tests and dev scenarios
routing.LoadYAMLRoutesForTesting(router)
```

Both use the SAME middleware registration in `RegisterExistingHandlers()`.

### NO Test Auth Bypass

**Tests MUST authenticate the same way production does.**

There is NO test auth bypass. The auth middleware:
1. Checks for JWT token in cookie or Authorization header
2. Validates the token
3. Returns 401 Unauthorized if missing/invalid

Tests that need authenticated endpoints must:
1. Call the login endpoint to get a token
2. Include the token in subsequent requests

```go
// Example: Get a token for tests
func getTestToken(t *testing.T, router *gin.Engine) string {
    resp := httptest.NewRecorder()
    body := `{"login":"test@example.com","password":"testpass"}`
    req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    router.ServeHTTP(resp, req)

    var result map[string]interface{}
    json.Unmarshal(resp.Body.Bytes(), &result)
    return result["access_token"].(string)
}

// Use the token
req.Header.Set("Authorization", "Bearer " + token)
```

### Why No Bypass?

1. **Tests verify real auth** - If auth is broken, tests fail
2. **No security risk** - No bypass code that could leak to production
3. **Prevents "tests pass, production fails"** - Same code path everywhere

### The Incident (Jan 27, 2026)

We had TWO separate YAML loaders:
- `internal/routing/loader.go` - Used by production
- `internal/api/yaml_router_loader.go` - Used by tests (with auth bypass)

Result: Tests passed but production returned 401 because the loaders handled middleware differently.

**Fix**: Consolidated to single loader, removed all auth bypass code.

### NEVER DO THIS

- Don't create a separate route loader for tests
- Don't add "test mode" auth bypass
- Don't inject fake user context in tests
- Don't use `APP_ENV=test` to skip authentication
- Don't check `gin.Mode() == gin.TestMode` to bypass auth

### Files

- `internal/routing/loader.go` - THE router (production + tests)
- `internal/routing/handlers.go` - Middleware registration (auth, admin, etc.)
- `internal/api/yaml_router_loader.go` - ONLY for manifest generation tooling

**One router. Real auth. No exceptions.**

---

## RUNNING GO TESTS - MANDATORY METHOD (Jan 22, 2026)

**ALWAYS use these Makefile targets to run Go tests:**

```bash
# Run tests for a specific package (optionally filtered by test name)
make toolbox-test-pkg PKG=./internal/api TEST=^TestLogin

# Run tests scoped to explicit test files
make toolbox-test-files FILES='path/to/a_test.go'

# Run a single Go test by name
make toolbox-test-run TEST=TestName
```

### NEVER DO THIS
- Don't use `make toolbox` with heredoc to run tests
- Don't use `docker exec` to run go test directly
- Don't run `go test` on the host machine

**Always use the Makefile targets for running tests. No exceptions.**

---

## DATABASE QUERIES - MANDATORY METHOD (Jan 22, 2026)

**ALWAYS use this method for ALL database queries:**

```bash
echo "SELECT * FROM table_name;" | make db-shell
```

### Examples
```bash
# List tables
echo "show tables;" | make db-shell

# Query customer users
echo "SELECT login, first_name, last_name FROM customer_user LIMIT 10;" | make db-shell

# Check specific record
echo "SELECT * FROM users WHERE id = 1;" | make db-shell
```

### NEVER DO THIS
- Don't use `docker exec` with mysql/mariadb client directly
- Don't use `make toolbox` with heredoc for queries
- Don't try to connect to the database any other way
- Don't guess or make up alternative methods

**This is the ONLY way to query the database. No exceptions.**

---

## DATABASE WRAPPER PATTERNS - ALWAYS USE THESE (Jan 11, 2026)

**Use `database.ConvertPlaceholders()` for all SQL queries. This allows future sqlx migration.**

### The Correct Pattern
```go
import "github.com/gotrs-io/gotrs-ce/internal/database"

// Write SQL with ? placeholders, convert before execution
query := database.ConvertPlaceholders(`
    SELECT id, name FROM users WHERE id = ? AND valid_id = ?
`)
row := db.QueryRowContext(ctx, query, userID, 1)

// For INSERT with RETURNING (handles MySQL vs PostgreSQL)
query := database.ConvertPlaceholders(`
    INSERT INTO users (name, email) VALUES (?, ?) RETURNING id
`)
query, useLastInsert := database.ConvertReturning(query)
if useLastInsert {
    result, err := db.ExecContext(ctx, query, name, email)
    id, _ = result.LastInsertId()
} else {
    err = db.QueryRowContext(ctx, query, name, email).Scan(&id)
}
```

### For Complex Operations Use GetAdapter()
```go
// GetAdapter() is for complex cases like InsertWithReturning
adapter := database.GetAdapter()
id, err := adapter.InsertWithReturning(db, query, args...)
```

### Test Code Uses Same Patterns
```go
func TestSomething(t *testing.T) {
    if err := database.InitTestDB(); err != nil {
        t.Skip("Database not available")
    }
    defer database.CloseTestDB()

    db, err := database.GetDB()
    require.NoError(t, err)

    // Use ConvertPlaceholders for queries
    query := database.ConvertPlaceholders(`SELECT id FROM users WHERE id = ?`)
    row := db.QueryRowContext(ctx, query, 1)
}
```

### Why This Pattern
- `ConvertPlaceholders()` handles MySQL vs PostgreSQL placeholder differences
- Designed so sqlx can be swapped in later
- `ConvertReturning()` handles RETURNING clause differences
- `GetAdapter()` for complex operations like InsertWithReturning

---

## ADDING NEW THEMES - THEME PACKAGE STRUCTURE (Jan 25, 2026)

Themes are self-contained packages in `static/themes/builtin/`. Each theme has its own directory with all assets.

### Theme Package Structure

```
static/themes/builtin/{theme-name}/
├── theme.yaml          # Theme metadata (name, description, features)
├── theme.css           # Main stylesheet with CSS variables
├── fonts/              # Theme-specific fonts (optional)
│   ├── fonts.css       # @font-face declarations with relative paths
│   └── {font-name}/    # Font files (woff2, ttf)
└── images/             # Theme-specific images (optional)
```

### Step 1: Create Theme Directory and theme.yaml

Create `static/themes/builtin/{theme-name}/theme.yaml`:

```yaml
name: Your Theme
id: your-theme-name
description: Short description of your theme
version: 1.0.0
author: Your Name
license: MIT

preview:
  gradient: "linear-gradient(135deg, #COLOR1, #COLOR2)"

modes:
  dark: true
  light: true
  default: dark

assets:
  fonts:
    enabled: true       # or false if using system fonts
    css: fonts/fonts.css
    license: SIL-OFL-1.1

features:
  glowEffects: false
  gridBackground: false
  animations: true
  bevels3d: false
  terminalMode: false

compatibility:
  minVersion: "1.0.0"
```

### Step 2: Create theme.css

Create `static/themes/builtin/{theme-name}/theme.css`:

```css
/* Dark mode (default) */
:root, :root.dark, .dark {
  --gk-theme-name: 'your-theme-name';
  --gk-theme-mode: 'dark';
  --gk-primary: #COLOR;
  --gk-bg-base: #COLOR;
  /* ... see existing themes for full list */
}

/* Light mode */
:root.light, .light {
  --gk-theme-mode: 'light';
  /* Override colors for light backgrounds */
}
```

Reference: `static/themes/builtin/synthwave/theme.css`

### Step 3: Add Fonts (if custom fonts needed)

1. **Download WOFF2 files** to `static/themes/builtin/{theme-name}/fonts/{font-name}/`
2. **Create fonts.css** with RELATIVE paths:

```css
@font-face {
  font-family: 'YourFont';
  font-style: normal;
  font-weight: 400;
  font-display: swap;
  src: url('your-font/your-font-latin.woff2') format('woff2');
}
```

3. **Update THIRD_PARTY_NOTICES.md** with font license info

### Step 4: Register in ThemeManager

Edit `static/js/theme-manager.js`:

```javascript
const AVAILABLE_THEMES = ['synthwave', 'gotrs-classic', 'seventies-vibes', 'nineties-vibe', 'your-new-theme'];
const BUILTIN_THEMES = ['synthwave', 'gotrs-classic', 'seventies-vibes', 'nineties-vibe', 'your-new-theme'];

const THEME_METADATA = {
  'your-new-theme': {
    name: 'Your Theme',
    nameKey: 'theme.your_theme',
    description: 'Theme description',
    descriptionKey: 'theme.your_theme_desc',
    gradient: 'linear-gradient(135deg, #COLOR1, #COLOR2)',
    hasFonts: true  // or false if using system fonts
  }
};
```

### Step 5: Add i18n Translations

Add to ALL 15 language files in `internal/i18n/translations/*.json`:

```json
"theme": {
    "your_theme": "Theme Name",
    "your_theme_desc": "Short description"
}
```

Languages: en, de, es, fr, pt, pl, ru, zh, ja, ar, he, fa, ur, uk, tlh

### Quick Reference: Required CSS Variables

| Category | Variables |
|----------|-----------|
| Primary | `--gk-primary`, `--gk-primary-hover`, `--gk-primary-active`, `--gk-primary-subtle` |
| Secondary | `--gk-secondary`, `--gk-secondary-hover`, `--gk-secondary-subtle` |
| Backgrounds | `--gk-bg-base`, `--gk-bg-surface`, `--gk-bg-elevated`, `--gk-bg-overlay` |
| Text | `--gk-text-primary`, `--gk-text-secondary`, `--gk-text-muted`, `--gk-text-inverse` |
| Borders | `--gk-border-default`, `--gk-border-strong` |
| Status | `--gk-success`, `--gk-warning`, `--gk-error`, `--gk-info` (+ `-subtle` variants) |
| Effects | `--gk-glow-primary`, `--gk-shadow-sm/md/lg/xl`, `--gk-focus-ring` |

### Files Created/Modified When Adding a Theme

1. `static/themes/builtin/{name}/theme.yaml` - Theme metadata (NEW)
2. `static/themes/builtin/{name}/theme.css` - Theme CSS (NEW)
3. `static/themes/builtin/{name}/fonts/` - Fonts directory (NEW, if custom fonts)
4. `static/js/theme-manager.js` - Add to AVAILABLE_THEMES, BUILTIN_THEMES, THEME_METADATA
5. `internal/i18n/translations/*.json` - Add translations (15 files)
6. `THIRD_PARTY_NOTICES.md` - Add font attribution (if custom fonts)

**Backend auto-discovers themes** from `static/themes/builtin/` directories.
**Template selectors read from `ThemeManager.THEME_METADATA`** - no template changes needed.
