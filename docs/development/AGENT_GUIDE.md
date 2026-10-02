# AGENT_GUIDE.md — Engineering Assistant Operating Manual

Status: Canonical.

Purpose: Clear rules and a practical workflow for engineering assistants working in the GoatFlow codebase. Follow this document for operating procedures, quality bars, and guardrails.

Related docs:
- [DATABASE_ACCESS_PATTERNS.md](DATABASE_ACCESS_PATTERNS.md) — full SQL portability rules
- [TESTING.md](TESTING.md) — test suite details

## Golden Rules
- **All operations in containers**: The Go toolchain and database clients are not installed on the host. Use `make toolbox-*` targets for Go work and `make db-*` targets for database work. Never run `go`, `mysql`, or `psql` directly on the host.
- **Containers first**: Run builds, tests, and tools in containers. Use Makefile targets. Do not bypass them with ad hoc docker/podman commands unless you mirror what the Makefile does.
- **CRITICAL - Container lifecycle**: NEVER use `make up` — it runs in the foreground and blocks the terminal. Always use:
  - `make up-d` - start containers in detached mode (returns at once)
  - `make restart` - runs `make down` then `make up-d` (rebuilds images)
  - `make down` - stop containers
  - `make logs` - view logs (database, valkey, backend)
- **SQL portability**: Write SQL with `?` placeholders and wrap it with `database.ConvertPlaceholders(...)`. Never write `$1`-style placeholders: `ConvertPlaceholders` panics on them.
- **Supported databases**: MySQL/MariaDB and PostgreSQL. Oracle and SQL Server return `ErrDatabaseNotImplemented`.
- **Security first**: Rootless containers, Alpine runtime image, SELinux-friendly mounts. Secrets only via environment variables (generate them with `make synthesize`). Never hardcode secrets.
- **No self-attribution**: Do not add assistant/AI attribution to commits, code, or docs. Follow repository commit conventions.
- **TDD discipline**: Write tests that test behaviour where practicable, run them, and see them pass before claiming completion.
- **Professional UX with multi-theme support**: No browser dialogs; use branded toasts/modals. Every theme must work in dark and light mode and meet accessibility standards.
- **Templating policy**: Use Pongo2 templates only; never use Go's `html/template`. Do not generate HTML in handlers for user-facing views; render via Pongo2 with the base layout and proper context.
- **Routing policy**: Define all HTTP routes in `routes/*.yaml` (YAML router). Do not register routes directly in Go code.
- **Full i18n support for 15 languages** must be kept in every code change or addition. `make check-i18n` flags hardcoded UI text.
- **Always write DRY code**: Do Not Repeat Yourself. Use or refactor existing code to be more flexible instead.
- **Commit discipline**: Interactive mode = stage only, ask before committing. CI mode = commit and push automatically. See [Commit Discipline](#commit-discipline).

## Commit Discipline

Commit behaviour depends on the execution context:

### Interactive Mode (Working with Human Reviewer)
When working directly with a human (chat sessions, pair programming):
- **Stage changes only** — do not commit or push without explicit permission
- Human batches and squashes commits for clean history
- Ask before running `git commit` or `git push`
- Reason: Humans prefer to review, squash, and craft meaningful commit messages

### CI/Automated Mode (GitHub Actions, scheduled tasks)
When running in CI pipelines or automated workflows:
- **Commit and push automatically** — no human approval needed
- Use conventional commit format (`feat:`, `fix:`, `docs:`, etc.)
- Include `[automated]` or `[ci]` tag if helpful for filtering
- Reason: CI is unattended; blocking on approval defeats the purpose

### How to Detect Context
- **Interactive**: Direct chat session, human messages in conversation
- **CI/Automated**: `CI=true` environment variable, GitHub Actions context, cron-triggered tasks

### Git Operations Reference
```bash
# Stage only (interactive mode default)
git add -A

# Commit (CI mode, or after human approval)
git commit -m "feat(module): description"

# Push (CI mode, or after human approval)
git push origin HEAD
```

## Required Workflow
1. Plan (if multi-step): Outline non-trivial tasks and confirm scope.
2. **Go operations**: Use the toolbox container for all Go work:
   - Build check: `make toolbox-compile` (runs `go build ./...`)
   - Module management: `make toolbox-exec ARGS="go mod tidy"`
   - Code generation: `make toolbox-exec ARGS="go generate ./..."`
3. **Database operations**: Use make targets for all database work:
   - Database shell: `make db-shell` (picks the client from `DB_DRIVER` and uses the credentials from `.env`)
   - Database queries: `echo "SELECT * FROM ticket LIMIT 5;" | make db-shell`
   - Database migrations: `make db-migrate`
4. Service lifecycle:
   - `make restart`
   - Health check: `curl -sf http://localhost:8081/health` (the host port is `BACKEND_PORT` in `.env`; `.env.example` sets 8081)
   - Logs sanity: `make logs | tail -200` (make sure there is no panic or error)
5. Tests:
   - Full suite: `make test`
   - If failures: fix locally and rerun until green
6. Browser verification (for UI):
   - Open target pages, check Console and Network tabs (no errors/500s)
   - Exercise the full workflow (create/edit/delete, save/refresh)
7. Only then report status. Be explicit about what is tested and what is pending.

## Database Access Patterns
Full rules: [DATABASE_ACCESS_PATTERNS.md](DATABASE_ACCESS_PATTERNS.md). The short version:

- Package: `internal/platform/database` (import `github.com/goatkit/goatflow/internal/platform/database`).
- Write SQL in MySQL dialect with `?` placeholders.
- Wrap every query with `database.ConvertPlaceholders(...)`. On PostgreSQL it turns `?` into `$1, $2, ...` and rewrites MySQL-only functions.
- `ConvertPlaceholders` **panics** on `$N` placeholders and on stacked (`;`-separated) statements.
- Table names must exist in `migrations/mysql` and `migrations/postgres`.
- Keep SQL in repositories; avoid SQL in handlers.

```go
rows, err := db.Query(
    database.ConvertPlaceholders(`
        SELECT id, title FROM ticket WHERE queue_id = ?
    `),
    queueID,
)
```

| Need | Use |
|------|-----|
| Any query | `database.ConvertPlaceholders(sql)` |
| Query that also uses PostgreSQL `::` casts | `database.ConvertQuery(sql)` |
| Upsert (`ON DUPLICATE KEY UPDATE`, `REPLACE INTO`) | `database.ConvertUpsert(sql, conflictCols...)` |
| Insert that needs the new id | `database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders("INSERT ... RETURNING id"), args...)` (`InsertWithReturningTx` for transactions) |

**Lint**: `make lint-platform` runs `cmd/gk-lint`. The pre-commit hook in `.githooks/pre-commit` runs it too. It fails on SQL that skips the conversion layer, on `LastInsertId()`, and on MySQL-only or PostgreSQL-only SQL. Reviewed exceptions carry a `// sql-converted: <reason>` comment.

### Dynamic SQL with QueryBuilder (REQUIRED)
For **dynamic WHERE clauses**, **variable column selection**, or **IN lists**, use the sqlx-based QueryBuilder (`internal/platform/database/querybuilder.go`). Do not build SQL with `fmt.Sprintf`; gosec flags it (G201/G202).

```go
// WRONG - string-built SQL (gosec G201)
query := fmt.Sprintf("SELECT id, login FROM users WHERE %s = ?", column)

// RIGHT - use the QueryBuilder for dynamic SQL
qb, err := database.GetQueryBuilder()
if err != nil {
    return err
}
sb := qb.NewSelect("id", "login").From("users").Where("valid_id = ?", validID)
if search != "" {
    sb.Where("login LIKE ?", "%"+search+"%")
}
query, args, err := sb.ToSQL()
if err != nil {
    return err
}
rows, err := qb.Query(query, args...)
```

**For IN clauses**, use `qb.In()` to expand slices:
```go
query, args, err := qb.In("SELECT id, title FROM ticket WHERE id IN (?)", ids)
rows, err := qb.Query(query, args...)
```

**Key methods** (all on the QueryBuilder `qb` or the SelectBuilder `sb`):
- `qb.NewSelect(columns...).From(table)` - start a SELECT query
- `sb.Where(condition, args...)` - add a WHERE condition (chain several; joined with AND)
- `sb.LeftJoin(joinClause)` - add a join
- `sb.OrderBy(columns...)`, `sb.Limit(n)`, `sb.Offset(n)` - ordering and pagination
- `sb.ToSQL()` - returns the converted query string and args slice
- `qb.Query`, `qb.QueryRow`, `qb.Exec` - run a query; they convert `?` queries and pass already-converted ones through
- `qb.In(query, args...)` - expands slices for IN clauses

### Row Iteration with rows.Err() (REQUIRED)
After iterating over `sql.Rows` with `for rows.Next()`, you **must** check `rows.Err()`. Errors during iteration (network issues, encoding problems) are stored and only reachable via `rows.Err()`:

```go
// WRONG - iteration errors silently lost
for rows.Next() {
    rows.Scan(&item)
    results = append(results, item)
}
return results, nil

// RIGHT - check rows.Err() after the loop
for rows.Next() {
    rows.Scan(&item)
    results = append(results, item)
}
if err := rows.Err(); err != nil {
    return nil, err
}
return results, nil
```

**Preferred: use the helpers** in `internal/platform/database/rows.go`:
```go
// CollectRows handles iteration and rows.Err() for you
users, err := database.CollectRows(rows, func(r *sql.Rows) (*User, error) {
    var u User
    err := r.Scan(&u.ID, &u.Login)
    return &u, err
})

// CollectStrings for simple single-column string queries
names, err := database.CollectStrings(rows)
```

## Cross-Database CRUD Patterns (CRITICAL)
**Unit tests that mock the database will NOT catch these errors. Always follow these patterns.**

### INSERT - Use InsertWithReturning (NOT LastInsertId, NOT raw RETURNING)
`LastInsertId()` does not work on PostgreSQL. A raw `RETURNING` clause does not work the same way on MySQL. `InsertWithReturning` handles both: on PostgreSQL it runs the `RETURNING` query; on MySQL it strips `RETURNING` and uses the last insert id.

```go
// WRONG - LastInsertId fails on PostgreSQL (gk-lint: sql-last-insert-id)
result, err := db.Exec(database.ConvertPlaceholders(`
    INSERT INTO standard_attachment (name, content, valid_id) VALUES (?, ?, ?)
`), name, content, validID)
id, _ := result.LastInsertId()

// RIGHT - InsertWithReturning
id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
    INSERT INTO standard_attachment
        (name, content, valid_id, create_time, create_by, change_time, change_by)
    VALUES (?, ?, ?, NOW(), ?, NOW(), ?)
    RETURNING id
`), name, content, validID, userID, userID)
```

### INSERT - Include All NOT NULL Timestamp Columns
**Most OTRS-style tables have NOT NULL `create_time`, `create_by`, `change_time`, `change_by` columns.** Leaving them out causes a "Field doesn't have a default value" error on MariaDB.

```sql
-- WRONG - missing timestamp columns
INSERT INTO standard_attachment (name, content, valid_id)
VALUES (?, ?, ?)

-- RIGHT - include all NOT NULL columns
INSERT INTO standard_attachment (name, content, valid_id, create_time, create_by, change_time, change_by)
VALUES (?, ?, ?, NOW(), ?, NOW(), ?)
```

### Pre-Implementation Checklist (Before ANY INSERT)
1. See all columns: `make db-query QUERY="DESCRIBE table_name"` (MariaDB) or `make db-query QUERY="\d table_name"` (PostgreSQL)
2. Find all NOT NULL columns without defaults
3. Include `create_time`, `create_by`, `change_time`, `change_by` if they exist
4. Use `database.GetAdapter().InsertWithReturning(...)` when you need the new id; never `LastInsertId()`
5. Wrap ALL queries with `database.ConvertPlaceholders()`

## Service Health Verification (After Route/Handler/Config Changes)
- Build: `make toolbox-compile`
- Restart: `make restart`
- Health: `curl -sf http://localhost:8081/health` (port = `BACKEND_PORT`)
- Logs: `make logs | grep -E "(panic|error)" | tail -5`

Common issues: duplicate route registration, unused imports, nil dereferences. Fix and re-run the steps above before going on.

## Routing Configuration (YAML)
- Source: `routes/*.yaml` files are loaded at startup by the YAML router (`internal/platform/routing/loader.go`).
- Policy: Do not register routes in Go code; declare or change them in YAML.
- Check: `make validate-routes` runs `scripts/validate_routes.sh`, which fails on hardcoded routes. `make test` runs the same check first.
- Changes: Edit YAML, then run the build/restart/health steps above.
- Warnings: Duplicate path+method combinations cause startup panics.

## UI Quality Bar (Baseline)
- Search/filter where applicable, with clear/reset
- Sortable columns where appropriate
- Branded modals/dialogs (submit on Enter), no native browser dialogs
- Error handling with friendly messages and focus management
- Loading states and success feedback
- Dark mode parity and responsive layout
- Accessibility: keyboard navigation, ARIA labels
- State persistence: keep search/filter state across operations

## Pongo2 Template Gotchas
- Template inheritance paths are relative to the templates root, not the file: use `layouts/base.pongo2`.
- Filters use colon syntax, e.g. `default:"-"`. There is no `|string` or `|json` filter.
- Compare like types (string vs string, int vs int). Convert in the handler if needed.
- If the page renders but looks wrong, check logs for template errors and the browser console for JS errors.

## Form Submission Pattern (Checkbox Matrix)
Prefer URL-encoded form payloads for checkboxes so the server parses them the same way every time:

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
- Always render via the base layout (`templates/layouts/base.pongo2`) with the right context:
  - Provide `User` and `ActivePage` so the nav shows the right items and highlights the current page
- Match global styling and dark mode; avoid direct HTML generation in handlers for user-facing views
- Give a clear way out of the page (breadcrumbs/back links)

## Commit & PR Guidance
- Conventional commits (`feat:`, `fix:`, `docs:`, etc.)
- Focus messages on the "why" and scope, not implementation detail
- Never include assistant/AI attribution in commits or PRs
- When you change routes or behaviour, briefly note testing steps (build, restart, health, logs, UI path)

## Development Environment
**The Go toolchain and database clients are NOT installed on the host.** All development work must use containers:

### Go Operations (Toolbox Container)
| Task | Command |
|------|---------|
| Build/compile | `make toolbox-compile` |
| Module management | `make toolbox-exec ARGS="go mod tidy"` or `make toolbox-mod-tidy` |
| Code generation | `make toolbox-exec ARGS="go generate ./..."` |
| Formatting | `make toolbox-exec ARGS="goimports -w ."` or `make toolbox-gofmt` |
| Linting | `make toolbox-lint` (golangci-lint) or `make lint` (all linters) |
| SQL portability + platform boundary lint | `make lint-platform` |
| Any other command | `make toolbox-exec ARGS="<command>"` |

### Database Operations (Make Targets)
| Task | Command |
|------|---------|
| Database shell | `make db-shell` (client chosen by `DB_DRIVER`; credentials from `.env`) |
| Run SQL from stdin | `echo "SELECT * FROM users;" \| make db-shell` |
| Single query | `make db-query QUERY="SELECT COUNT(*) FROM ticket"` |
| Apply all migrations | `make db-migrate` (runs `./migrate ... up` in the backend container) |
| Recreate the dev database from migrations | `make db-init` (drops all dev data) |
| Fix sequences | `make db-fix-sequences` (PostgreSQL only, after data imports; `make db-migrate` runs it for you) |
| Test database shell | `make db-shell-test` |

Migrations live in `migrations/mysql` and `migrations/postgres`. Both sets hold the same 29 versions, `000001` to `000029`.

**Never run `go` commands directly on the host** - they fail with "command not found".

## Go Performance Anti-Patterns (AVOID)

### Slice Preallocation (REQUIRED when size is known)
When building a slice in a loop where the final size is known or can be estimated, **always preallocate**:

```go
// WRONG - causes repeated reallocations and GC pressure
var results []Item
for _, src := range items {
    results = append(results, transform(src))
}

// RIGHT - single allocation, no reallocations
results := make([]Item, 0, len(items))
for _, src := range items {
    results = append(results, transform(src))
}
```

**Why it matters**: Without preallocation, Go grows the backing array each time capacity runs out. Each growth is a new allocation plus a copy, and the old arrays become garbage. With preallocation there is one allocation and no copies.

**golangci-lint**: The `prealloc` linter catches these. Run `make toolbox-lint` to find violations.

### String Concatenation in Loops (AVOID)
```go
// WRONG - O(n²) allocations
var result string
for _, s := range parts {
    result += s
}

// RIGHT - O(n) with single final allocation
var b strings.Builder
b.Grow(estimatedSize) // optional but helps
for _, s := range parts {
    b.WriteString(s)
}
result := b.String()
```

## Makefile Targets (Common)
| Target | What it does |
|--------|--------------|
| `make up` / `make up-d` | Start services in the foreground / background |
| `make down` | Stop services |
| `make restart` | `make down` then `make up-d` |
| `make logs` / `make backend-logs` | View logs (all core services / backend only) |
| `make db-shell` | Open a database shell (MariaDB or PostgreSQL, from `DB_DRIVER`) |
| `make test` | Full test suite in containers (see [Testing Infrastructure](#testing-infrastructure)) |
| `make toolbox-compile` | Compile all packages inside the toolbox container |
| `make toolbox-exec ARGS="..."` | Run any command in the toolbox container |
| `make frontend-build` | Build CSS (Tailwind) and JavaScript bundles (`css-build` + `js-build`) |
| `make css-build` / `make js-build` | Build only CSS / only JavaScript (`js-build` builds `static/js/tiptap.min.js`) |
| `make css-watch` | Rebuild CSS on change |
| `make frontend-clean-cache` | Clear frontend build caches |
| `make css-deps` | `bun install` in the toolbox container |
| `make bun-updates` | Upgrade all frontend dependencies with `npm-check-updates` |

**DANGER**: Never use `docker compose down -v` - the `-v` flag removes ALL volumes, including the dev database. Profile flags (`--profile testdb`) do NOT reliably isolate volume removal. To stop the test database, use `make test-db-down`.

**Note**: `make bun-updates` runs `npm-check-updates -u`, which can upgrade Tailwind CSS to v4 and break the build. Tailwind is pinned to `~3.4.17` in `package.json`; check that pin after running it.

### Container-First Enforcement Helpers
To stop host `go` usage creeping back in:

- Macro: `TOOLBOX_GO` (defined in `Makefile`) expands to `$(MAKE) toolbox-exec ARGS=`. Use it only in simple targets; do not nest it inside already long `podman run` / `docker run` invocations.
- Verification: `make verify-container-first` runs `scripts/tools/check-container-go.sh`. It fails if the `Makefile` has tab-prefixed raw `go` or `golangci-lint` lines (`build`, `test`, `run`, `vet`, `mod`, `list`).
- Acceptable exceptions: Inside a single explicit `$(TOOLBOX_IMAGE)` (`ghcr.io/goatkit/goatflow/toolbox:latest`) container run block, direct `go build/test` inside `bash -lc '...'` is fine - do not wrap it again.
- Add new Go-related targets via the `toolbox-exec` pattern by default. If performance needs a single large container run, keep all `go` calls inside that one block.

Checklist before committing new Go targets:
1. No plain `\tgo test` or `\tgo build` lines unless inside an existing toolbox container run block.
2. `make verify-container-first` passes.
3. For multi-step, script-like flows, prefer a dedicated script run via `toolbox-exec` over many inline Makefile commands.
4. CI runs the `Container-First Guard` workflow (`.github/workflows/container-first.yml`) on pull requests to `main`/`dev` and pushes to `main`.

## Troubleshooting Checklist
- **Go command fails**: Go is not installed on the host. Use `make toolbox-exec ARGS="go <command>"` instead
- **Database connection fails**: Database clients are not installed on the host. Use `make db-shell` for interactive access or pipe SQL to it
- **Wrong database credentials**: Never hardcode credentials. `make db-shell` reads them from `.env` / Makefile variables
- Build fails: run `make toolbox-compile` and read the first error; fix from top to bottom
- Service panic: `make logs | tail -200`; look for duplicate routes or nil dereferences
- UI mismatch after save: check the network request payload and response; refresh view state after save
- SQL errors: confirm `database.ConvertPlaceholders` usage and portable SQL; run `make lint-platform`
- Missing assets: static files are served from `./static` by `HandleStaticFiles` (`routes/static.yaml`, path `/static/*filepath`)

## Caching (Go & Tooling)
The toolbox container keeps its caches in workspace-local directories, so they survive between runs.

| Cache | Host directory | Path inside toolbox |
|-------|----------------|---------------------|
| Go build cache (`GOCACHE`) | `.go-build/` | `/workspace/.go-build` |
| Go module cache (`GOMODCACHE`) | `.gomodcache/` | `/workspace/.gomodcache` |
| golangci-lint cache (`GOLANGCI_LINT_CACHE`) | `.golangci-lint/` | `/workspace/.golangci-lint` |

Targets:
- `make cache-prune` - removes the old named cache volumes (`goatflow_cache`, `goatflow_go_build_cache`, `goatflow_go_mod_cache`, `goatflow_golangci_cache`)
- `make toolbox-exec` creates these directories and sets their permissions before each run

Avoid running ad hoc root containers that write into these directories; files owned by root break later non-root toolbox runs.

## Testing Controls (Prevent Recurring Issues)

### Route Registry Pattern (Admin Handler Tests)
**Problem**: Tests that define their own routes drift from production, causing 404s in the browser.

**Solution**: Use the shared route definitions in `internal/api/test_router_registry.go`:

```go
// In test_router_registry.go:
func GetAdminRolesRoutes() []AdminRouteDefinition {
    return []AdminRouteDefinition{
        {"GET", "/roles", handleAdminRoles},
        {"POST", "/roles", handleAdminRoleCreate},
        // ... all routes
    }
}

// In the test file:
func setupRoleTestRouter() *gin.Engine {
    return SetupTestRouterWithRoutes(GetAdminRolesRoutes())
}
```

Production routes still come from `routes/*.yaml`. Keep each `Get<Module>Routes()` list in step with the YAML file. Today the registry has `GetAdminRolesRoutes()` and `GetAdminDynamicFieldsRoutes()`.

**Rule**: Do not hand-register routes in test setup functions. Use `SetupTestRouterWithRoutes()` with the module's `Get*Routes()` function, or load the real YAML routes with `routing.LoadYAMLRoutesForTesting(router)`.

### JSON vs HTML Responses (Headers)
**Problem**: Some handlers decide between JSON and HTML from request headers. A Go test sends the header; an inline `fetch()` in a template may not. Result: the test passes, but the browser fails with "<!DOCTYPE... is not valid JSON".

**Rule**: Every `fetch()` that expects JSON must send the headers the endpoint needs. For admin roles they are listed in `GetAdminRolesContracts()` in `internal/api/test_router_registry.go` (for example `Accept: application/json` and `X-Requested-With: XMLHttpRequest`). Note: `/api/...` paths always get JSON.

### JSON Field Contract
**Problem**: JavaScript sends different field names than the Go handler expects (for example `description` vs `comments`).

**Rule**: When creating or changing JS fetch calls, check that the field names match the handler's `json:"..."` struct tags exactly.

### Pre-Module Checklist
Before starting any new admin module:

1. [ ] Add routes to the right `routes/*.yaml` file
2. [ ] If tests need a route list, add `Get<Module>Routes()` to `test_router_registry.go` and use `SetupTestRouterWithRoutes(Get<Module>Routes())`
3. [ ] If handlers pick JSON or HTML by header, add `Get<Module>Contracts()` to `test_router_registry.go`
4. [ ] Template JS field names match the handler JSON tags
5. [ ] Template JS fetch calls send the headers in the contract
6. [ ] Add any new page template to `AllPageTemplates` (see [Template Testing](#template-testing-mandatory-for-forms))
7. [ ] Run a browser test after unit tests pass (not just "tests pass")

### E2E Verification (Non-Negotiable)
Unit tests CANNOT catch:
- Route registration mismatches (404 in browser)
- JS/Go field name mismatches (JSON parse errors)
- Missing Accept headers (HTML returned instead of JSON)
- Template rendering issues

**After all unit tests pass**, you MUST:
1. `make restart`
2. Open the page in a browser
3. Exercise the full workflow (create/edit/delete)
4. Check the browser Console for errors
5. Check the Network tab for failed requests

Only then report "feature complete".

### Template Testing (MANDATORY for Forms)
**Problem**: Templates with HTMX attributes (`hx-post`, `hx-put`) or form actions can have path mismatches that unit tests don't catch (for example `/api/dynamic-fields` when the route is `/admin/api/dynamic-fields`).

**Solution**: Template tests in `internal/platform/template/` check HTMX attributes and form actions:

```go
// Helpers live in internal/platform/template/pongo2_test.go
helper := NewTemplateTestHelper(t)
html, err := helper.RenderTemplate("pages/admin/my_form.pongo2", ctx)
require.NoError(t, err)
asserter := NewHTMLAsserter(t, html)

// For create forms
asserter.HasHTMXPost("/admin/api/my-resource")
asserter.HasNoHTMXPut()

// For edit forms
asserter.HasHTMXPut("/admin/api/my-resource/42")
asserter.HasNoHTMXPost()
```

**Test files** (all in `internal/platform/template/`):
| File | Contents |
|------|----------|
| `pongo2_test.go` | Test helper and HTML asserter |
| `all_templates_test.go` | Form action / HTMX path tests |
| `template_coverage_test.go` | Render tests for every page template, plus the coverage check |
| `dynamic_fields_template_test.go` | Example module-specific tests |

**Coverage check**: `TestAllPageTemplatesHaveCoverage` in `template_coverage_test.go` walks `templates/pages/` and fails if any page template is missing from the `AllPageTemplates` map (or if the map lists a template that no longer exists).

**When adding a new page template**:
1. Add its path to `AllPageTemplates` in `template_coverage_test.go`, and render it in the matching `TestAll*TemplatesRender` test
2. If it has a form, add a test that asserts the right HTMX/action attributes
3. Run `make test-templates` to check

**Quick validation**: `make test-templates` runs only `./internal/platform/template/...`. In `make test`, template tests run first inside the unit-test step.

## Dynamic Fields System

### Architecture
Dynamic fields let administrators add custom fields to tickets, articles, customer users, and customer companies without schema changes.

- **Field storage**: `dynamic_field` table (field config stored as YAML)
- **Screen visibility**: `dynamic_field_screen_config` table (migration `000004`) controls which fields appear on which screens

### Screen Configuration (IMPORTANT)
Fields **only appear on forms** if they have a screen config entry:
- `GetFieldsForScreenWithConfig(screenKey, objectType)` uses an `INNER JOIN` on `dynamic_field_screen_config`
- No config entry = field not shown on that screen
- Config values: `0` = disabled, `1` = enabled, `2` = required

### Admin Workflow
1. Create field: `/admin/dynamic-fields` → "New Dynamic Field" (`/admin/dynamic-fields/new`)
2. Enable for screens: `/admin/dynamic-fields/screens` (the page saves via `/admin/api/dynamic-fields/:id/screens`)
3. The field now appears on those screens

### Screen Keys (OTRS Compatible)
The full list is `GetScreenDefinitions()` in `internal/api/dynamic_field_types.go`.

| Screen Key | Screen |
|------------|--------|
| `AgentTicketPhone` | New phone ticket (`/ticket/new/phone`) |
| `AgentTicketEmail` | New email ticket (`/ticket/new/email`) |
| `AgentTicketZoom` | Ticket detail view (display only) |
| `AgentTicketNote` | Add note |
| `AgentTicketClose` | Close ticket |
| `AgentTicketMove` | Move ticket |
| `AgentTicketOwner` | Change owner |
| `AgentTicketPriority` | Change priority |
| `CustomerTicketMessage` | Customer new ticket |
| `CustomerTicketZoom` | Customer ticket view (display only) |
| `AgentArticleZoom` | Article view (display only) |
| `AgentArticleNote` | Agent note article |
| `AgentArticleClose` | Close note article |
| `AgentArticleReply` | Agent reply article |
| `CustomerArticleReply` | Customer reply article |

### Template Integration
Include the partial in ticket forms:
```django
{% include "partials/dynamic_fields.pongo2" with DynamicFields=DynamicFields %}
```

The handler must load the fields with `GetFieldsForScreenWithConfig(screenKey, objectType)`.

### Troubleshooting
- **Fields not appearing**: Check `/admin/dynamic-fields/screens` - the field must be enabled for the target screen
- **Field appears but no label**: Missing `label` in the `dynamic_field` row
- **DB error**: Make sure migration `000004` has run (`dynamic_field_screen_config` table exists)

## Legal & Compliance
- GoatFlow-CE is an original implementation; keep compatibility without copying upstream code
- Keep all secrets in environment variables; generate them with `make synthesize`; never commit them

## ENTITY SELECTION MODAL UX BLUEPRINT - MANDATORY FOR ALL DIALOGS

**This is the standard for entity selection modals (add users to role, assign agents to queue, etc.)**

Reference implementation: `roleUsersModal` in `templates/pages/admin/roles.pongo2`, built on the shared component `static/js/entity-selector.js`. Reuse that component; do not write a new one.

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
// Example: GET /admin/roles/:id/users/search?q={query}
GET /admin/{entity}/:id/{members}/search?q={query}

// Requirements (as in handleAdminRoleUsersSearch):
// - Minimum 2 characters
// - Maximum 20 results
// - Excludes already-assigned members
// - Searches several fields (login, first name, last name, full name)
// - Returns JSON: {"success": true, "users": [...]}
```

### JavaScript Patterns

`static/js/entity-selector.js` defaults: `minChars: 2`, `debounceMs: 300`, `maxResults: 20`, `undoTimeoutMs: 5000`.

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
10. **Error Handling**: Roll back the UI on API failure, show toast

### NEVER DO THIS

- Load ALL available entities into the DOM (use the search API)
- Clear the search input after adding (the user may want to add more)
- Delete immediately without an undo option
- Use browser `confirm()` dialogs
- Block the UI during API calls (use optimistic updates)
- Forget keyboard navigation
- Skip loading indicators during search

**Every entity selection modal in the product MUST follow this pattern.**

---

## TESTING INFRASTRUCTURE

**We have a full test stack with a dedicated test database.** Details: [TESTING.md](TESTING.md).

### Test Database
- Separate test database containers: `mariadb-test` or `postgres-test` (from `docker-compose.testdb.yml`). `TEST_DB_DRIVER` picks which one.
- Tests run against a real database, not mocks.
- `make test-stack-up` starts the test stack and runs `make test-setup-admin`, which sets up the test admin (`TEST_USERNAME`, default `root@localhost`, with password `TEST_PASSWORD`).
- In `internal/api`, `TestMain` resets the test database to its baseline once before the package runs. Tests that change data should call `WithCleanDB(t)` (resets at start and end) or `t.Cleanup(ResetTestDB)`.

### What `make test` Runs
1. `check-i18n`, `check-deps`, `plugin-build-wasm`
2. `scripts/test-runner.sh`:
   - static route check (`scripts/validate_routes.sh`)
   - test stack start (`make test-stack-up`)
   - unit tests (`make test-unit`, via `scripts/unit-test-phases.sh`; template tests run first)
   - Playwright E2E tests
   - log analysis

### How to Write Tests
1. Use the real database connection - DO NOT mock the database
2. Seed data is available - use it
3. Reset the data you change (`WithCleanDB(t)`)
4. Integration tests should use the actual DB, not be skipped

### Makefile Targets for Testing
| Target | What it does |
|--------|--------------|
| `make test` | Full suite (see above) |
| `make test-unit` | Unit tests only (starts the test stack first) |
| `make test-templates` | Template tests only |
| `make toolbox-test` | Core tests: `./cmd/goats`, `./internal/platform/i18n`, a focused set of `./internal/api` tests, `./internal/service`, `./internal/services/escalation` |
| `make db-shell-test` | Shell on the test database |
| `make test-db-up` / `make test-db-down` | Start / stop the test database |

### NEVER DO THIS
- Don't write tests that skip because "no DB connection"
- Don't mock database calls when the real DB is available. It is.
- Don't claim low coverage is fine because "DB required"
- Don't use build tags to skip DB tests

**The test database EXISTS. Use it.**

---

## YAML ROUTING - SINGLE SOURCE OF TRUTH

**There is ONE YAML route loader. Tests use the same loader as production.**

### The Single Router

All YAML route loading goes through `internal/platform/routing/loader.go`:

```go
// Production: cmd/goats/main.go calls api.MountDynamicEngine, which calls
routing.LoadYAMLRoutes(engine, routesDir, resolver)

// Tests and dev scenarios
routing.LoadYAMLRoutesForTesting(router)
```

Both register middleware (auth, admin, etc.) through `RegisterExistingHandlers()` in `internal/platform/routing/handlers.go`.

### NO Test Auth Bypass

**Tests MUST authenticate the same way production does.**

There is NO test auth bypass. The test-only bypasses (`GOATFLOW_DISABLE_TEST_AUTH_BYPASS`, the `X-Test-Mode` header, `DEMO_LOGIN_*`, `TEST_AUTH_*`) were removed in 0.10.0. The auth middleware:
1. Checks for a JWT in the cookie or the `Authorization` header
2. Validates the token
3. Returns 401 Unauthorized if it is missing or invalid

Tests that need authenticated endpoints must:
1. Call the login endpoint to get a token
2. Send the token in later requests

```go
// Example: get a token for tests (POST /api/v1/auth/login, handler HandleLoginAPI)
func getTestToken(t *testing.T, router *gin.Engine) string {
    resp := httptest.NewRecorder()
    body := fmt.Sprintf(`{"login":%q,"password":%q}`,
        os.Getenv("TEST_USERNAME"), os.Getenv("TEST_PASSWORD"))
    req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    router.ServeHTTP(resp, req)

    var result map[string]interface{}
    require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
    return result["access_token"].(string)
}

// Use the token
req.Header.Set("Authorization", "Bearer " + token)
```

### Why No Bypass?

1. **Tests check real auth** - If auth is broken, tests fail
2. **No security risk** - No bypass code that could leak to production
3. **Prevents "tests pass, production fails"** - Same code path everywhere

### The Incident

We once had TWO separate YAML loaders:
- `internal/platform/routing/loader.go` - used by production
- `internal/api/yaml_router_loader.go` - used by tests (with auth bypass)

Result: tests passed but production returned 401, because the loaders handled middleware differently.

**Fix**: Consolidated to a single loader and removed all auth bypass code.

### NEVER DO THIS

- Don't create a separate route loader for tests
- Don't add "test mode" auth bypass
- Don't inject fake user context in tests
- Don't use `APP_ENV=test` to skip authentication
- Don't check `gin.Mode() == gin.TestMode` to bypass auth

### Files

- `internal/platform/routing/loader.go` - THE route loader (production + tests)
- `internal/platform/routing/handlers.go` - Middleware registration (auth, admin, etc.)
- `internal/api/yaml_router_loader.go` - ONLY for route manifest generation (`cmd/routes-manifest`) and route docs for the MCP handler

**One router. Real auth. No exceptions.**

---

## RUNNING GO TESTS - MANDATORY METHOD

**ALWAYS use these Makefile targets to run Go tests:**

```bash
# Run tests for one package (optionally filtered by test name)
make toolbox-test-pkg PKG=./internal/api TEST=^TestLogin

# Run tests from explicit test files (optionally filtered with TEST=)
make toolbox-test-files FILES='path/to/a_test.go'

# Run tests matching a name across all packages
make toolbox-test-run TEST=TestName
```

### NEVER DO THIS
- Don't use `docker exec` to run `go test` directly
- Don't run `go test` on the host machine

**Always use the Makefile targets for running tests. No exceptions.**

---

## DATABASE QUERIES - MANDATORY METHOD

**ALWAYS use this method for ad hoc database queries:**

```bash
echo "SELECT * FROM table_name;" | make db-shell
```

`make db-query QUERY="..."` also works for a single statement.

### Examples
```bash
# List tables (MariaDB)
echo "show tables;" | make db-shell

# Query customer users
echo "SELECT login, first_name, last_name FROM customer_user LIMIT 10;" | make db-shell

# Check a specific record
echo "SELECT * FROM users WHERE id = 1;" | make db-shell

# Test database
echo "SELECT COUNT(*) FROM ticket;" | make db-shell-test
```

### NEVER DO THIS
- Don't use `docker exec` with the mariadb/psql client directly
- Don't try to connect to the database any other way
- Don't guess or make up alternative methods

**This is the ONLY way to query the database. No exceptions.**

---

## DATABASE WRAPPER PATTERNS - ALWAYS USE THESE

**Use `database.ConvertPlaceholders()` for all SQL queries.** See [DATABASE_ACCESS_PATTERNS.md](DATABASE_ACCESS_PATTERNS.md) for the full rules.

### The Correct Pattern
```go
import "github.com/goatkit/goatflow/internal/platform/database"

// Write SQL with ? placeholders, convert before execution
query := database.ConvertPlaceholders(`
    SELECT id, login FROM users WHERE id = ? AND valid_id = ?
`)
row := db.QueryRowContext(ctx, query, userID, 1)

// INSERT that needs the new id (works on MySQL and PostgreSQL)
id, err := database.GetAdapter().InsertWithReturning(db, database.ConvertPlaceholders(`
    INSERT INTO standard_attachment
        (name, content, valid_id, create_time, create_by, change_time, change_by)
    VALUES (?, ?, ?, NOW(), ?, NOW(), ?)
    RETURNING id
`), name, content, validID, userID, userID)
```

### Test Code Uses Same Patterns
```go
func TestSomething(t *testing.T) {
    require.NoError(t, database.InitTestDB())

    db, err := database.GetDB()
    require.NoError(t, err)

    // Use ConvertPlaceholders for queries
    query := database.ConvertPlaceholders(`SELECT id FROM users WHERE id = ?`)
    row := db.QueryRowContext(ctx, query, 1)
}
```

### Why This Pattern
- `ConvertPlaceholders()` handles MySQL vs PostgreSQL placeholder and function differences
- `GetAdapter().InsertWithReturning()` handles the `RETURNING` vs last-insert-id difference
- `make lint-platform` (`cmd/gk-lint`) enforces both

---

## ADDING NEW THEMES - THEME PACKAGE STRUCTURE

Themes are self-contained packages in `static/themes/builtin/`. Each theme has its own directory with all its assets.

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

3. **Update THIRD_PARTY_NOTICES.md** with the font license info

### Step 4: Register in ThemeManager

Edit `static/js/theme-manager.js`:

```javascript
const AVAILABLE_THEMES = ['synthwave', 'goatflow-classic', 'seventies-vibes', 'nineties-vibe', 'your-new-theme'];
const BUILTIN_THEMES = ['synthwave', 'goatflow-classic', 'seventies-vibes', 'nineties-vibe', 'your-new-theme'];

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

Add to ALL 15 language files in `internal/platform/i18n/translations/*.json`:

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
5. `internal/platform/i18n/translations/*.json` - Add translations (15 files)
6. `THIRD_PARTY_NOTICES.md` - Add font attribution (if custom fonts)

**Backend auto-discovers themes**: any directory in `static/themes/builtin/` with a `theme.css` is listed.
**Template selectors read from `ThemeManager.THEME_METADATA`** - no template changes needed.
