# AGENT.md — Engineering Assistant Operating Manual

Status: Canonical. This document supersedes CLAUDE.md.

Purpose: Provide clear, enforceable rules and a practical workflow for engineering assistants working in the GOTRS codebase. Follow this document as the single source of truth for operating procedures, quality bars, and guardrails.

## Golden Rules
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
2. Build check: `make toolbox-compile` to ensure the repo compiles.
3. Service lifecycle:
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

## Makefile Targets (Common)
- `make up` / `make up-d`: start services (foreground/background)
- `make down`: stop services
- `make restart`: restart backend service
- `make logs` / `make backend-logs`: view logs
- `make db-shell`: open PostgreSQL shell
- `make test`: run tests in containers
- `make toolbox-compile`: compile all packages inside toolbox container

## Troubleshooting Checklist
- Build fails: run `make toolbox-compile` and read the first error; fix from top to bottom
- Service panic: `make logs | tail -200`; search for duplicate routes or nil dereferences
- UI mismatch after save: verify network request payload and response; refresh view state after save
- SQL errors on MySQL: confirm `database.ConvertPlaceholders` usage and SQL syntax portability
- Missing assets: verify static route points to `./static` not `./web/static`

## Legal & Compliance
- GOTRS-CE is an original implementation; maintain compatibility without copying upstream code
- Keep all secrets in environment variables; generate via project tooling; do not commit

## This Document vs CLAUDE.md
This document replaces CLAUDE.md as the authoritative operating guide for engineering assistants. CLAUDE.md remains for historical context only.
