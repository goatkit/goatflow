# AGENT.md — Engineering Assistant Operating Manual

Status: Canonical. This document supersedes CLAUDE.md.

Purpose: Provide clear, enforceable rules and a practical workflow for engineering assistants working in the GOTRS codebase. Follow this document as the single source of truth for operating procedures, quality bars, and guardrails.

## Golden Rules
- **All operations in containers**: Go toolchain and database clients are not installed on host. Use `make toolbox-*` targets for Go operations and `make db-*` targets for database operations. Never attempt to run `go`, `mysql`, or `psql` commands directly on host.
- Containers-first: Run builds, tests, and tools in containers. Use Makefile targets; do not bypass with ad‑hoc docker/podman commands unless mirroring Makefile behavior.
- Makefile as entrypoint: Prefer `make up`, `make down`, `make logs`, `make restart`, `make test`, `make toolbox-compile`.
- SQL portability: Always wrap SQL strings with `database.ConvertPlaceholders(...)`. Never use raw `$n` placeholders directly.
- Schema freeze: Do not modify existing OTRS tables and do not add new tables before v1.0. See `SCHEMA_FREEZE.md`.
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
