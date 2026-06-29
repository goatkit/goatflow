# GoatKit Platform / Product Decoupling — Implementation Plan

> Version: 1.4
> Date: 2026-06-29
> Status: Phase 1-5 complete, Phase 6 next
> Related: `docs/ARCHITECTURE.md`, `docs/PLUGIN_PLATFORM.md`, `docs/design/LLM_INTEGRATION.md`

## 1. Goal

Establish a clear platform/product boundary inside the goatflow repository by:

1. Breaking the 6 coupling points that entangle platform code with product code.
2. Reorganizing platform packages under `internal/platform/`.
3. Enforcing the boundary with a custom linter.
4. Keeping one Go module (`github.com/goatkit/goatflow`); defer the actual `goatkit`
   module split until a second consumer exists.

The deliverable is architectural clarity, not a new module. The FAQ/KB plugin (and
future plugins) benefit from a clean plugin runtime that doesn't transitively depend
on ticketing code.

## 2. Context and findings

The architecture docs (`docs/ARCHITECTURE.md:296-305`, `docs/VISION.md:138-163`)
describe a three-tier structure: GoatKit Core (platform) → Modules/Plugins → Products
(GoatFlow). In reality:

- The `goatkit/` repo at `~/git/goatkit/goatkit/` is an **empty git repo** with zero
  commits. There is no `go.mod`, no `pkg/`, nothing.
- `goatflow/go.mod` has **zero `replace` directives** — it does not import any
  `github.com/goatkit/goatkit` package.
- Every piece of "GoatKit platform" code (plugin manager, HostAPI, sandbox, search,
  custom fields, signing, loader) lives under `goatflow/internal/`, which is **not
  importable by any other Go module** by the language spec.
- The only externally-importable surface is `goatflow/pkg/plugin/` (the Plugin
  interface + types), which is enough to write an external *plugin*, but not enough
  to build a second host product on GoatKit.
- `goat-tables` (`github.com/gotrs-io/goat-tables`, sitting in the same parent
  directory) is a fully independent RackTables clone using chi (not gin), with zero
  goatkit imports. It is **not** an existing consumer of GoatKit.

The migration baseline (`migrations/postgres/000001_schema_alignment.up.sql`,
1651 lines, 127 `CREATE TABLE` statements) creates both platform tables (`users`,
`groups`, `roles`, `sessions`, `dynamic_field`, `mail_account`, `sysconfig_*`,
`system_*`, `valid`, `virtual_fs`, `oauth2_token`, etc.) and product tables
(`ticket`, `article`, `queue`, `sla`, `pm_*`, `calendar*`, `service_*`,
`customer_*`, etc.) in one atomic snapshot. Splitting it would require FK dependency
reordering and a reconciliation strategy for already-deployed databases (golang-migrate
silently skips lower-versioned migrations on DBs already at version >= 1).

**Conclusion:** the migration baseline stays unified in goatflow as a historical
artifact. The extraction is an internal reorganization, not a module split. The value
is in the refactoring (breaking coupling, creating clear package boundaries), not in
the `goatkit` module ceremony.

## 3. Decisions locked

| Decision | Choice | Rationale |
|---|---|---|
| Deliverable | Internal reorganization, no Go module split | Migration baseline makes module split artificially costly; no second consumer exists; value is in the refactoring |
| Package structure | `internal/platform/` prefix for platform packages + custom linter | Structural boundary, harder to accidentally violate |
| Migration baseline | Stays unified in goatflow (`000001` creates all 127 tables) | Historical artifact; splitting requires FK reordering + existing-DB reconciliation |
| Product placement | Product packages stay at `internal/` root (no prefix) | They're the majority; platform is the exception |
| FAQ/KB plugin | Proceeds after Phase 1 (`scheduler.go` decoupling) | Phase 1 makes the plugin runtime cleanly platform-only |
| Timeline | Soft target — quality over calendar; span 2-3 releases | 20-30 days of focused work; don't rush |

## 4. Platform package inventory

### 4.1 Already clean (no product imports — direct moves)

`apierrors`, `auth`, `cache`, `config`, `convert`, `customfields`, `data`,
`deletion`, `httpcookie`, `i18n`, `ldap`, `lookups`, `marketplace`, `mcp`,
`middleware`, `notifications`, `oauth2`, `organisation`, `pluginui`, `push`,
`runner`, `search`, `secureconfig`, `sysconfig`, `swconfig`, `template`, `utils`,
`version`, `webhook`, `yamlmgmt`, `zinc`

### 4.2 Clean sub-trees (move with parent)

`plugin/{grpc,loader,wasm,signing,packaging}`, `database/{drivers,schema}`,
`email/inbound/connector`, `services/{registry,database,k8s,user}`,
`components/lambda`, `platform/schema`

### 4.3 Need refactoring before move (the 6 coupling points)

| Package | Coupling | Phase |
|---|---|---|
| `plugin/` | `scheduler.go` imports `models` + `services/scheduler` | Phase 1 |
| `database/` | `connection.go` imports `services/adapter` | Phase 2 |
| `models/` | Mixed platform/product types in one package | Phase 3 |
| `routing/` | `handlers.go` imports `internal/models`; test imports `internal/api` | Phase 4 |
| `api/` | Flat ~250-file dir, no platform/product structural boundary | Phase 5 |
| `service/` | Mixed platform/product services in one package | Phase 5 |

### 4.4 Stay as PRODUCT (no move)

`repository`, `repository/memory`, `ticketnumber`, `ticketutil`, `tickets`,
`history`, `core`, `constants` (article_types), `email/inbound/{postmaster,filters,adapter}`,
`email/integration`, `selfservice`, `storage`, `runner/tasks`,
`services/{escalation,genericagent,acl,ticket,ticketattributerelations}`,
`service/genericinterface`, `service/ticket_number`,
`components/{dashboard,dynamic,handlers}`, `webhooks`, `mailaccountmeta`,
`mailqueue`, `api/{graphql,shared,v1}`, `api/` (product handler files)

## 5. Phases

### Phase 1 — Break `scheduler.go` coupling

#### Problem

`internal/plugin/scheduler.go` (82 lines) imports `internal/models` and
`internal/services/scheduler`. `services/scheduler` transitively imports
`repository` (11,682 lines), `services/escalation`, `services/genericagent`,
`email/inbound/*`, `ticketutil`, `ticketnumber` — approximately 16,000 lines of
product code dragged into the plugin runtime's transitive closure.

This is the single highest-leverage coupling to break: the smallest file (82 lines),
the biggest transitive unblock.

#### Fix

Define a platform-level scheduler interface in the plugin package. GoatFlow registers
its ticket-aware scheduler as the implementation.

#### Steps

1. ✅ Create `internal/plugin/scheduler_iface.go` with a `PlatformScheduler` interface
   and a platform `ScheduledJob` type (plain struct, no product concerns — only the
   fields the plugin bridge needs: `Slug`, `Handler`, `Name`, `Schedule`,
   `TimeoutSeconds`).
2. ✅ Rewrite `scheduler.go` to use the interface instead of `*scheduler.Service` and
   `*models.ScheduledJob`. The `RegisterPluginJobs` function takes a
   `PlatformScheduler` instead of `*scheduler.Service`.
3. ✅ Create `internal/services/scheduler/plugin_adapter.go` — implements
   `plugin.PlatformScheduler` by translating platform jobs into `models.ScheduledJob`
   and delegating to `scheduler.Service`.
4. ✅ Wire in `cmd/goats/main.go` — pass the adapter to `plugin.RegisterPluginJobs`.

#### Risks and mitigations

| # | Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|---|
| 1.1 | `models.ScheduledJob` is used in 30+ sites across `services/scheduler` (handlers.go, options.go, service.go) and `cmd/goats/scheduler_jobs.go` — moving the type breaks all of them | High | High | **Do not move the type.** Define a *new* platform `ScheduledJob` in `internal/plugin/scheduler_iface.go` with only the fields the plugin bridge needs. The adapter translates between platform and product types. The product `models.ScheduledJob` stays where it is. |
| 1.2 | `scheduler_test.go` exists and tests the current coupling directly | Medium | Low | Read the test before refactoring; rewrite to test the interface contract, not the concrete `*scheduler.Service` binding. Verify the test still exercises the same behavior. |
| 1.3 | `RegisterPluginJobs` is called from `cmd/goats/main.go` with a concrete `*scheduler.Service` — changing the signature breaks the call site | Certain | Low | Update the call site in the same commit. It is one line in `main.go`. |
| 1.4 | Plugin job handlers currently capture `mgr.Call(ctx, pName, jHandler, nil)` in a closure — the closure captures loop variables | Low (already handled) | Low | The existing code already captures `pName`, `jHandler`, `jID` as loop-local vars (`scheduler.go:34-36`). Preserve this pattern in the rewrite. |
| 1.5 | The adapter introduces a layer of indirection that could mask scheduler errors | Low | Medium | The adapter must propagate errors verbatim — no swallowing. Add a test that verifies a plugin job failure surfaces through the adapter. |

#### Verification checks

| # | Check | Command | Pass criteria |
|---|---|---|---|
| V1.1 | Plugin package has no product imports | `grep -rn 'goatflow/internal/models\|goatflow/internal/services/scheduler\|goatflow/internal/repository\|goatflow/internal/ticketutil\|goatflow/internal/ticketnumber' internal/platform/plugin/*.go` | Zero matches (excluding `_test.go` if tests legitimately test the adapter) |
| V1.2 | Plugin package compiles in isolation | `go build ./internal/platform/plugin/...` | Clean compile, no errors |
| V1.3 | Scheduler tests pass | `go test ./internal/platform/plugin/... ./internal/services/scheduler/...` | All pass |
| V1.4 | Full build compiles | `go build ./...` | Clean |
| V1.5 | Plugin jobs still register at boot | `go test -run TestRegisterPluginJobs ./internal/platform/plugin/...` | Jobs register and execute via the adapter |
| V1.6 | No regression in scheduler behavior | `go test ./internal/services/scheduler/...` | All existing scheduler tests pass (auto-close, email poll, escalation check, etc.) |
| V1.7 | `go vet` clean | `go vet ./internal/platform/plugin/... ./internal/services/scheduler/...` | No warnings |
| V1.8 | Transitive import closure of `internal/platform/plugin` contains no product packages | `go list -deps ./internal/platform/plugin/ \| grep -E 'goatflow/internal/(repository\|ticketnumber\|ticketutil\|history\|services/escalation\|services/genericagent\|email/inbound)'` | Zero matches |

#### Estimated effort: 1-2 days

---

### Phase 2 — Invert `database` to `services/adapter` dependency

#### Problem

`internal/database/connection.go:11` and `internal/database/testing.go:10` import
`internal/services/adapter`. `services/adapter` imports `services/database` +
`services/registry`. The DB package (platform) depends on a "services" layer —
inverted layering. Additionally, `services/adapter` is imported by 4 `internal/api`
files (`exports.go`, `admin_users_handlers.go`, `admin_crud_handlers.go`,
`exports_customer.go`) and `cmd/goats/main.go:59` — so the inversion must account
for all 6 import sites, not just `database/`.

#### Fix

Invert: `database/` owns connection lifecycle; `services/adapter` depends on it. Then
move `services/{adapter,database,registry}` to `internal/platform/services/`.

#### Steps

1. ✅ Audit what `database/` actually calls on `services/adapter` — likely
   `adapter.GetDB()` at `connection.go:41`. Move that logic into `database/` directly.
2. ✅ Audit what the 4 `internal/api` files and `main.go` call on `services/adapter` —
   likely `adapter.InitializeServiceRegistry()` and service lookups. These callers
   stay on the `services/adapter` API; only the internal dependency direction flips.
3. ✅ Move `services/adapter`, `services/database`, `services/registry` to
   `internal/platform/services/`.
4. ✅ Update all 6 import sites.
5. ✅ `database/` now imports only `internal/platform/services/registry`
   (platform to platform, allowed).

#### Risks and mitigations

| # | Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|---|
| 2.1 | `adapter.GetDB()` may use the service registry to resolve which DB connection to return (multi-DB routing) — moving this to `database/` could duplicate registry logic | Medium | High | **Audit before refactoring.** Read `services/adapter/*.go` fully. If `GetDB()` delegates to the registry, the registry must move with (or before) the inversion. Do not split `GetDB()` from the registry — move them together to `internal/platform/`. |
| 2.2 | The 4 `internal/api` files importing `services/adapter` may use it for more than just DB access (e.g., other service lookups) | Medium | Medium | Grep each of the 4 files for `adapter.` calls. Classify each call as DB-related (stays) or service-lookup (may need the registry directly). Update imports accordingly. |
| 2.3 | Circular import: `database/` to `platform/services/registry` to `database/` | Low | High | `services/registry` (`internal/platform/services/registry`) currently has **zero internal imports** (confirmed clean). It will not import `database/`. No cycle. |
| 2.4 | `cmd/goats/main.go:59` calls `adapter.InitializeServiceRegistry()` — if the init order changes, boot could fail | Medium | High | The init call stays in `main.go`; only the import path changes (`services/adapter` to `platform/services/adapter`). No logic change. Verify boot sequence with a smoke test. |
| 2.5 | `internal/database/testing.go` is used by tests across the codebase — changing its imports could break many test suites | Medium | Medium | Update `testing.go` in the same commit as the connection.go change. Run `go test ./...` to catch breakages. |
| 2.6 | MySQL and Postgres drivers in `database/drivers/` may have implicit dependencies on the adapter for driver selection | Low | Medium | Verify `database/drivers/{postgres,mysql,sqlite}/*.go` import only `internal/database` (confirmed clean in investigation). No adapter dependency. |

#### Verification checks

| # | Check | Command | Pass criteria |
|---|---|---|---|
| V2.1 | `database/` does not import old product `services/adapter` | `grep -rn 'goatflow/internal/services/adapter' internal/database/` | Zero matches (uses `internal/platform/services/...`) |
| V2.2 | `database/` imports only platform packages | `grep -rn 'goatflow/internal/' internal/database/*.go \| grep -v 'platform/\|database/'` | Zero matches (only self-imports and platform imports) |
| V2.3 | Full build compiles | `go build ./...` | Clean |
| V2.4 | Boot sequence works | `go test -run TestBootSequence ./cmd/goats/...` (or manual smoke test) | Services initialize, DB connects, migrations run |
| V2.5 | All tests pass | `go test ./...` | All pass |
| V2.6 | `go vet` clean | `go vet ./internal/database/... ./internal/platform/services/...` | No warnings |
| V2.7 | No circular imports | `go list -deps ./internal/database/ \| grep goatflow/internal \| sort -u` | No product packages in the dependency list |
| V2.8 | All 6 import sites use `internal/platform/services/adapter` | `grep -rn 'goatflow/internal/platform/services/adapter' internal/ cmd/` | 6 matches (all import sites updated to platform path) |

#### Estimated effort: 2-3 days

---

### Phase 3 — Split `internal/models/`

#### Problem

`internal/models/` is one 36-file, 5937-line package mixing platform types (`User`,
`Group`, `Role`, `Session`, `APIToken`, `LDAPConfiguration`, `SearchRequest`,
`EmailAccount`) with product types (`Ticket`, `SLA`, `Queue`, `Incident`, `Change`,
`ACL`, `CannedResponse`, `KnowledgeArticle`). It is imported by **158 non-test
files** — the largest blast radius of any phase.

#### Fix

Split into `internal/platform/models/` (8 moved whole + 2 split) and keep product files in
`internal/models/`.

#### Platform type outcomes

| File | Types | Outcome |
|---|---|---|
| `api_token.go` | `APITokenUserType`, etc. | ✅ Moved whole |
| `db_role.go` | `DBRole` | ✅ Moved whole |
| `email.go` | `EmailAccount` | ❌ Stayed product — `Queue *Queue` field creates platform→product coupling |
| `group.go` | `Group` | ✅ Moved whole |
| `ldap.go` | `LDAPConfiguration` | ✅ Moved whole |
| `lookups.go` | `LookupItem` | ✅ Split — `QueueInfo`, `TicketFormData` stayed product |
| `role.go` | `Role` | ✅ Moved whole |
| `scope_registry.go` | `ScopeDefinition` | ✅ Moved whole |
| `search.go` | `SearchRequest`, `SearchResult`, `SearchHit`, `Facet`, `IndexStats` | ✅ Split — ticket-specific search types stayed product |
| `session.go` | `Session` | ✅ Moved whole |
| `user.go` | `User` | ✅ Moved whole |

#### Product types to stay

`acl.go`, `business_hours.go`, `canned_response.go`, `change.go`, `cmdb.go`,
`escalation.go`, `generic_agent_job.go`, `incident.go`, `internal_note.go`,
`knowledge.go`, `problem.go`, `service_catalog.go`, `sla.go`,
`system_maintenance.go`, `ticket*.go` (7 files), `time_accounting.go`,
`webservice.go`, `workflow.go`

#### Steps

1. ✅ Create `internal/platform/models/` directory.
2. ✅ `git mv` 8 clean platform files (`api_token`, `db_role`, `group`, `ldap`, `role`,
   `scope_registry`, `session`, `user`).
3. ✅ Change package name in moved files (keep as `models` — Go allows same package name
   at different import paths).
4. ✅ Split mixed files — extract `LookupItem` from `lookups.go`; extract `SearchRequest`,
   `SearchResult`, `SearchHit`, `Facet`, `IndexStats` from `search.go`; keep product
   types in their original files.
5. ✅ Add `type_aliases.go` in `internal/models/` with type aliases, const aliases,
   var aliases, and func forwarding for all moved types.
6. ✅ Refactor `internal/zinc/` to remove product import from production code (use JSON
   roundtrip for generic document handling).

#### Risks and mitigations

| # | Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|---|
| 3.1 | **158 import sites is a large mechanical change — high risk of merge conflicts** if anyone else is working on the codebase | High | Medium | Do this in a dedicated commit, off-peak. Communicate timing to all contributors. Prefer aliases first, then migrate import paths in later focused passes. Run `go build` after each batch of import updates to catch errors early. |
| 3.2 | **Files using both platform and product types need two imports with the same package name** — Go disallows two imports with the same name unless one is aliased | Certain | High | Use import aliases: `import ( platformmodels "goatflow/internal/platform/models"; models "goatflow/internal/models" )`. Convention: platform models use `platformmodels` where product `models` is also needed. Document this convention. |
| 3.3 | **Type identity breakage** — if any code does `assert.(models.User)` or `interface{} to models.User` type assertions across the boundary, splitting the type breaks the assertion | Medium | High | Use type aliases (`type User = platformmodels.User`) in `internal/models/` for cross-boundary types. Aliases preserve type identity — `models.User` and `platformmodels.User` are the same type. Add aliases for all moved and split platform types. |
| 3.4 | **JSON serialization compatibility** — if API responses serialize `models.User` and the type moves, the JSON structure could change if field tags differ | Low | High | `git mv` preserves the file contents exactly — field tags do not change. The type alias ensures the same struct is serialized. Verify with API contract tests. |
| 3.5 | **Database scanning** — `sqlx` scans into `models.User` by field name; if the type moves, scanning still works (same struct, different import path) | Low | Low | No change — `sqlx` scans by struct field name, not import path. Verify with repository tests. |
| 3.6 | **The `internal/email/inbound/adapter` package imports `internal/models`** — it may use platform types (email config) or product types | Medium | Medium | Audit `email/inbound/adapter/*.go` during this phase. If it uses `EmailAccount` (platform), update to `platform/models`. If it uses ticket types, leave as `models`. |
| 3.7 | **`scheduler.go` (already refactored in Phase 1) may still reference `models.ScheduledJob`** — verify Phase 1 removed this dependency | Low | Medium | Phase 1 should have eliminated this. Verify with V1.1 check before starting Phase 3. |
| 3.8 | **Go version compatibility** — type aliases are Go 1.9+; goatflow uses Go 1.25.10, so no issue | None | None | N/A |

#### Verification checks

| # | Check | Command | Pass criteria |
|---|---|---|---|
| V3.1 | `internal/platform/models/` imports no product packages | `grep -rn 'goatflow/internal/' internal/platform/models/*.go \| grep -v 'platform/'` | Zero matches |
| V3.2 | Full build compiles | `go build ./...` | Clean — this is the critical check for the 114-file import update |
| V3.3 | All tests pass | `go test ./...` | All pass — catches type identity and scanning issues |
| V3.4 | Type aliases preserve identity | `go test -run TestTypeAlias ./internal/models/...` (write a small test asserting `models.User` == `platformmodels.User`) | Type assertion across the alias works |
| V3.5 | API contract tests pass | `go test -run TestAPI ./internal/api/...` (or Playwright E2E) | JSON responses unchanged — same field names, same structure |
| V3.6 | `go vet` clean | `go vet ./...` | No warnings — catches shadowed imports, unused aliases |
| V3.7 | No circular imports | `go list -deps ./internal/platform/models/` | Does not contain `goatflow/internal/models` |
| V3.8 | Import count verified | `grep -rln 'goatflow/internal/models"' --include="*.go" internal/ cmd/ \| wc -l` | Significantly reduced (only files using product types remain) |
| V3.9 | golangci-lint clean | `golangci-lint run ./internal/platform/models/... ./internal/models/...` | No new warnings |

#### Execution notes

- **8 files moved whole** (git mv, no changes): api_token, db_role, group, ldap, role, scope_registry, session, user
- **`email.go` stayed product** — `EmailAccount.Queue *Queue` field would create platform→product dependency
- **`lookups.go` split** — `LookupItem` to platform; `QueueInfo`, `TicketFormData` stay product
- **`search.go` split** — `SearchRequest`, `SearchResult`, `SearchHit`, `Facet`, `IndexStats` to platform; everything else stays product
- **`zinc` mock refactored** — production code now imports only platform models; uses JSON roundtrip for generic doc handling. Test files still import product models (test-only)
- **158 importing files** — no import path changes needed due to type aliases

#### Estimated effort: 3-4 days (actual: ~1 day)

---

### Phase 4 — Decouple `internal/routing/` from `internal/api/` and `internal/models/`

#### Problem

`internal/routing/handlers.go:17` imports `internal/models`. The test file
`yaml_handler_wiring_test.go:10` imports `internal/api` (test-only, but still a
coupling). The router engine (platform) should not import product models or the
product API package.

#### Fix

The router accepts handler registrations via a `HandlerResolver` interface. Product
code registers handlers by name; the router looks them up.

#### Steps

1. ✅ Define `HandlerResolver` interface in `internal/routing/`.
2. ✅ Remove `internal/models` import from `handlers.go` — `models.User` became
   `platformmodels.User` to preserve identity through Phase 3 aliases.
3. ✅ Create `internal/api/handler_registry.go` implementation via
   `RoutingHandlerResolver`.
4. ✅ Wire the API-backed resolver into YAML route loading (`dynamic_router.go`,
   `routes_simplified.go`; `cmd/goats/main.go` now documents the resolver path).
5. ✅ Update `yaml_handler_wiring_test.go` to use a mock resolver instead of importing
   `internal/api`; add API-side coverage for real YAML handler resolution.

#### Risks and mitigations

| # | Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|---|
| 4.1 | `handlers.go` may use `models.*` types in function signatures that are called from product code — changing the signature breaks callers | Medium | High | Audit `handlers.go:17` and all `models.` references in `routing/`. If types are used in signatures, either (a) move the type to platform models (if it is a platform type), (b) use `interface{}`/`any` at the boundary, or (c) define a router-internal type that the resolver maps. |
| 4.2 | The `HandlerResolver` lookup-by-name introduces a runtime failure mode that does not exist with direct handler references — a typo in a route name causes a 500 at request time, not a compile error | Medium | Medium | Add a startup validation pass: after all routes are registered, verify every route's handler name resolves. Fail fast at boot if any handler is missing. Log all unresolved handlers. |
| 4.3 | Route handler registration order matters — if the router is initialized before handlers are registered, route lookups fail | Low | High | Wire the resolver before route compilation. `cmd/goats/main.go` already controls init order — ensure handler registration happens before `router.Compile()`. Add a test. |
| 4.4 | Performance: the resolver adds a map lookup per request | Low | Low | `map[string]http.HandlerFunc` lookup is O(1), nanoseconds. Negligible vs. the HTTP handler itself. |
| 4.5 | The test file `yaml_handler_wiring_test.go` imports `internal/api` to test wiring — removing this import could reduce test coverage | Medium | Low | Create a mock handler registry in the test that registers a few dummy handlers by name. The test still verifies wiring logic without importing the product API. |
| 4.6 | Some route YAML files may reference handlers by a different name than the Go function name — the resolver must use the YAML-declared name, not the Go symbol | Low | Medium | The resolver maps YAML route names to `http.HandlerFunc`. The mapping is explicit in `handler_registry.go`. Audit all 31 route YAML files to ensure every handler name has a corresponding entry. |

#### Verification checks

| # | Check | Command | Pass criteria |
|---|---|---|---|
| V4.1 | Routing has no `internal/api` / `internal/models` imports | `grep -rn 'goatflow/internal/api\b\|goatflow/internal/models\b' internal/routing/` | ✅ Zero matches (including test files) |
| V4.2 | Routing/API compile and targeted tests pass | `make toolbox-exec ARGS="go test ./internal/routing/... ./internal/api/..."` | ✅ Clean |
| V4.3 | All YAML handlers resolve through product registry | `make toolbox-exec ARGS="go test ./internal/api -run TestAllYAMLHandlersResolveThroughAPIRegistry"` | ✅ Zero unresolved |
| V4.4 | Full test suite passes | `make toolbox-exec ARGS="go test ./..."` | ✅ All pass |
| V4.5 | HTTP integration smoke coverage | `make toolbox-exec ARGS="go test ./internal/api/... -run TestYAMLRoutesBasicAvailability"` | ✅ Routes registered |
| V4.6 | `go vet` clean | `make toolbox-exec ARGS="go vet ./internal/routing/..."` | ✅ No warnings |
| V4.7 | No circular imports | `make toolbox-exec ARGS="go list -deps ./internal/routing"` | ✅ No cycle errors; transitive product deps via `middleware`/`shared` remain for later phases |
| V4.8 | Routing package decoupled from product API tests | `grep -rn 'goatflow/internal/api\b' internal/routing/` | ✅ Zero matches |

#### Execution notes

- `internal/routing.HandlerResolver` now covers handler, middleware, and feature-flag lookup.
- `routing.RouteLoader` and `FileRouteStore` consume the resolver interface instead of a concrete registry.
- `internal/api.RoutingHandlerResolver` populates a routing registry from API handler registration and is wired into YAML route loading.
- `internal/routing/handlers.go` imports `internal/platform/models` for `User`; this preserves `shared` template/user context type assertions because Phase 3 aliases keep `models.User` and `platformmodels.User` identical.
- `internal/routing/yaml_handler_wiring_test.go` no longer imports `internal/api`; `internal/api/yaml_handler_wiring_test.go` owns product-registry coverage for real YAML handler names.

#### Estimated effort: 3-5 days (actual: ~1 day)

---

### Phase 5 — Reorganize `internal/api/` and `internal/service/`

#### Problem

`internal/api/` is a flat ~250-file directory with no platform/product structural
boundary. `internal/service/` (23 files, 7515 lines) mixes platform services
(api_token, auth, user) with product services (ticket, sla, escalation).
`internal/api` is imported by 4 non-test files; `internal/service` is imported by 40.

#### Fix

Move platform-flavored files to `internal/platform/api/` and
`internal/platform/service/`. Product files stay.

#### Platform API files moved

`auth_api.go`, `auth_handler.go`, `user_*.go`, `organisation_handlers.go`,
`deletion_handlers.go`, `custom_fields_api_handlers.go`, and `i18n_handlers.go`
(plus matching platform tests where the tests only covered moved handlers).

#### Product API files kept after triage

`api_token_*`, `auth_handlers.go`, `auth_htmx_handlers.go`, `auth_customer.go`,
`push_*`, `webauthn_*`, `totp_*`, `mcp_*`, `plugin_*`, `plugin_ui_admin_*`,
`org_plugin_access_handlers.go`, `lookup_*`, and product handlers (`ticket_*`,
`queue_*`, `article_*`, `sla_*`, `priority_*`, `state_*`, `type_*`,
`service_*`, `canned_response_*`, `admin_*`, `customer_*`, `agent_*`,
`graphql/`, `shared/`, `v1/`) stay in `internal/api/`.

#### Platform service files moved

`auth_service.go`, `totp_service.go`, `webauthn_service.go`, and
`user_preferences.go` (plus matching tests). `api_token_service.go` stayed in
product service because it still imports the mixed repository layer and uses
product token scopes.

#### Product service files to stay

Files for: `ticket`, `sla`, `escalation`, `generic_agent`, etc.

#### Steps

1. [x] File-by-file triage of `internal/api/`.
2. [x] `git mv` safe platform files to `internal/platform/api/`.
3. [x] File-by-file triage of `internal/service/`.
4. [x] `git mv` safe platform service files to `internal/platform/service/`.
5. [x] Update importers to use `platformapi` / `platformservice` aliases where
   product and platform packages are both needed.
6. [x] Keep product-coupled handlers and services in place instead of forcing a
   move that would reintroduce platform -> product imports.

#### Risks and mitigations

| # | Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|---|
| 5.1 | **Misclassification** — a file appears platform but actually imports product types, or vice versa | Medium | High | For each file, run `grep 'goatflow/internal/\(repository\|ticketnumber\|ticketutil\|history\|models\)' <file>` before moving. If it imports product packages, it is product. Document the classification in `PLATFORM_PACKAGES.md` for review. |
| 5.2 | **Shared helpers in `internal/api/shared/`** — some helpers are used by both platform and product handlers | High | Medium | Audit `shared/*.go` (1456 lines, 4 files). Split into `internal/platform/api/shared/` (generic response helpers) and keep article-creation helpers in `internal/api/shared/`. If a single file has both concerns, split the file. |
| 5.3 | **Package name collision** — `internal/platform/api` and `internal/api` both named `api` | Certain | Low | Use import aliases where both are imported in the same file: `platformapi "goatflow/internal/platform/api"`. This is the same pattern as Phase 3. |
| 5.4 | **40 files import `internal/service`** — large mechanical change with merge conflict risk | High | Medium | Same mitigation as Phase 3: dedicated commit, off-peak, scripted import updates, `go build` after each batch. |
| 5.5 | **Some API handlers may be registered in route YAML files by package-qualified name** — moving a handler to `platform/api` could break route resolution | Low | High | Route YAML references handlers by *name* (string), not by Go package path (Phase 4's resolver handles this). Verify by cross-referencing the handler registry with route YAML files. |
| 5.6 | **`internal/api/exports.go` and `exports_customer.go` import `services/adapter`** (from Phase 2 finding) — if these files are platform, they need updated imports | Medium | Low | These files are likely product (exports of customer/ticket data). Verify during triage. If platform, update the import to `platform/services/adapter`. |
| 5.7 | **Test files in `internal/api/` reference both platform and product handlers** — splitting the package could break tests that test cross-boundary interactions | Medium | Medium | Tests stay with the code they test. Platform handler tests move to `internal/platform/api/`. Cross-boundary integration tests stay in `internal/api/` or move to a top-level `tests/integration/` directory. |

#### Verification checks

| # | Check | Command | Result |
|---|---|---|---|
| V5.1 | `internal/platform/api/` and `internal/platform/service/` import no product packages directly | `grep 'github.com/goatkit/goatflow/internal/(api|models|service|repository|ticketnumber|ticketutil|history|zinc)' internal/platform/{api,service}` | Zero matches |
| V5.2 | Targeted split packages compile and test | `make toolbox-exec ARGS="go test ./internal/platform/api ./internal/platform/service ./internal/api ./internal/middleware ./cmd/goats"` | Pass |
| V5.3 | API contract tests pass | `make toolbox-exec ARGS="go test -run TestAPI ./internal/api/... ./internal/platform/api/..."` | Pass |
| V5.4 | Full build compiles | `make toolbox-exec ARGS="go build ./..."` | Pass |
| V5.5 | Full test suite passes | `make toolbox-exec ARGS="go test ./..."` | Pass |
| V5.6 | `go vet` clean | `make toolbox-exec ARGS="go vet ./..."` | Pass |
| V5.7 | Transitive platform dependency cleanup remains for Phase 6/7 | `make toolbox-exec ARGS="go list -deps ./internal/platform/api ./internal/platform/service"` | Shows transitive root packages via `auth`, `middleware`, `customfields`, etc.; direct imports are clean |

#### Estimated effort: 5-7 days (actual: ~1 day)

---

### Phase 6 — Move all remaining platform packages to `internal/platform/`

#### Problem

After Phases 1-5, approximately 25-30 already-clean platform packages still live at
`internal/` root. They need to move to `internal/platform/` for structural
consistency.

#### Steps

1. `git mv` each clean platform package to `internal/platform/<name>/`.
2. Update all importers across the codebase.
3. Keep `pkg/plugin/` at top level (public SDK, not internal).

#### Target directory structure

```
internal/platform/
├── api/              (from Phase 5)
├── auth/
├── cache/
├── config/
├── customfields/
├── database/         (from Phase 2)
├── deletion/
├── httpcookie/
├── i18n/
├── ldap/
├── lookups/
├── marketplace/
├── mcp/
├── middleware/
├── models/           (from Phase 3)
├── notifications/
├── oauth2/
├── organisation/
├── plugin/           (from Phase 1, minus scheduler.go coupling)
│   ├── grpc/
│   ├── loader/
│   ├── wasm/
│   ├── signing/
│   └── packaging/
├── pluginui/
├── push/
├── routing/          (from Phase 4)
├── runner/
├── search/
├── secureconfig/
├── service/          (from Phase 5)
├── services/         (from Phase 2: adapter, database, registry, k8s, user)
├── storage/          (if audited as platform — see risk 6.3)
├── sysconfig/
├── template/
├── utils/
├── webhook/
├── yamlmgmt/
└── zinc/
```

#### Risks and mitigations

| # | Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|---|
| 6.1 | **Large-scale `git mv` + import update is the most merge-conflict-prone step** — every package move changes import paths for all importers | High | Medium | Do moves in batched commits (one per package or small group). Run `go build` after each batch. Communicate timing. Consider a scripted approach: `go fmt -s` + `goimports` after path updates. |
| 6.2 | **Some "clean" packages may have hidden product imports not caught by the investigation** | Low | Medium | Run `grep -rn 'goatflow/internal/\(repository\|ticketnumber\|ticketutil\|history\|core\|tickets\|selfservice\|storage\|mailqueue\|mailaccountmeta\|webhooks\|email/inbound\|runner/tasks\|services/escalation\|services/genericagent\|services/acl\|services/ticket\|service/genericinterface\|service/ticket_number\|api/graphql\|api/v1\|api/shared\|components/dashboard\|components/dynamic\|components/handlers\|models"' internal/<package>/` for each package before moving. |
| 6.3 | **`internal/storage` (1436 lines) is classified as MIXED** — "article attachment storage" is product-flavored but storage infra is platform | Medium | Medium | Audit `storage/*.go` before moving. If it references `article` or `ticket` types, it is product. If it is generic file storage (S3, local FS), it is platform. Split if needed. |
| 6.4 | **`internal/components/lambda` (386 lines) uses goja (JS execution)** — verify it does not execute product-specific JS | Low | Low | Audit `lambda/*.go`. If it is a generic JS runtime, it is platform. If scripts reference ticket/queue types, those are product concerns loaded at runtime, not compile-time. |
| 6.5 | **`internal/email/inbound/connector` is platform (IMAP/POP3) but `email/inbound/postmaster` and `email/inbound/filters` are product** — the `email/inbound/` parent dir is mixed | Medium | Medium | Move only `email/inbound/connector` to `internal/platform/email/inbound/connector/`. Leave `postmaster`, `filters`, `adapter` in `internal/email/inbound/` (product). The parent `internal/email/` directory stays as product. |
| 6.6 | **`internal/notifications` (743 lines) is platform but may reference product notification types** | Low | Medium | Audit `notifications/*.go`. The investigation found it imports only `config`, `database`, `utils` — all platform. Verify with grep before moving. |
| 6.7 | **The `cmd/goats/main.go` imports ~20 internal packages** — every package move requires updating `main.go` | Certain | Low | Update `main.go` import paths in the same commit as each package move. `main.go` is the wiring point — it will reference both platform and product packages (that is correct; the host wires both). |

#### Verification checks

| # | Check | Command | Pass criteria |
|---|---|---|---|
| V6.1 | Every package under `internal/platform/` imports no product packages | `grep -rn 'goatflow/internal/\(repository\|ticketnumber\|ticketutil\|history\|core\|tickets\|selfservice\|storage\|mailqueue\|mailaccountmeta\|webhooks\|email/inbound/postmaster\|email/inbound/filters\|email/inbound/adapter\|runner/tasks\|services/escalation\|services/genericagent\|services/acl\|services/ticket\|services/ticketattributerelations\|service/genericinterface\|service/ticket_number\|api/graphql\|api/v1\|api/shared\|components/dashboard\|components/dynamic\|components/handlers\)' internal/platform/` | Zero matches |
| V6.2 | Full build compiles | `go build ./...` | Clean |
| V6.3 | All tests pass | `go test ./...` | All pass |
| V6.4 | `go vet` clean | `go vet ./...` | No warnings |
| V6.5 | golangci-lint clean | `golangci-lint run ./...` | No new warnings |
| V6.6 | Binary still starts and serves requests | `go build -o /tmp/goats ./cmd/goats && /tmp/goats -mode server &` then `curl localhost:8080/health` | Server starts, health check returns 200 |
| V6.7 | No product package under `internal/` (non-platform) imports another product package circularly due to the reorganization | `go list -deps ./internal/... \| sort -u \| head` | No circular dependency errors from `go list` |
| V6.8 | Plugin system still loads plugins | `go test -run TestPlugin ./internal/platform/plugin/...` | Plugins load, init, call, shutdown |

#### Estimated effort: 3-5 days

---

### Phase 7 — Enforce boundary with linter

#### Problem

Once the boundary exists structurally, entropy will re-entangle platform and product
without enforcement.

#### Steps

1. Write `cmd/gk-lint/main.go` — scans all Go files under `internal/platform/` and
   asserts none import a product package.
2. Product package list: any `internal/` package NOT under `internal/platform/` is
   product. The linter discovers product packages dynamically by scanning the
   directory tree (no hardcoded list).
3. Add `Makefile` target `lint-platform`.
4. Add to CI (`golangci-lint` already runs at `Makefile:1178`; add `gk-lint` as a
   separate required step).
5. Write `docs/development/PLATFORM_BOUNDARY.md` — explain the boundary, the linter,
   and how to add new packages to the correct side.

#### Risks and mitigations

| # | Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|---|
| 7.1 | **The linter has false positives** — a platform package legitimately needs a product type (e.g., platform models aliasing product types) | Medium | Medium | Maintain an allowlist in the linter config for known exceptions (e.g., type aliases in `internal/models/` that reference `internal/platform/models/`). Document each exception with a reason. |
| 7.2 | **The linter is too slow for pre-commit use** | Low | Low | The scan is `grep`-equivalent — scan ~180 files for import patterns. Sub-second. If needed, cache results. |
| 7.3 | **Contributors do not know about the linter and bypass it** | Medium | Medium | Add to CI as a **required** check (fails the PR). Document in `PLATFORM_BOUNDARY.md` and in `CONTRIBUTING.md` (if it exists). Add a `make lint` target that includes `lint-platform`. |
| 7.4 | **The product package list is hardcoded and goes stale as new packages are added** | High | Low | **Do not hardcode.** Use the convention: any `internal/` package NOT under `internal/platform/` is product. The linter discovers product packages dynamically by scanning the directory tree. New product packages are automatically covered. |
| 7.5 | **The linter does not catch transitive imports** — platform package A imports platform package B, which imports product package C | Medium | High | Use `go list -deps ./internal/platform/...` to get the transitive dependency closure, then check for product packages in the closure. This catches indirect imports. Supplement with the direct-import grep for fast feedback. |

#### Verification checks

| # | Check | Command | Pass criteria |
|---|---|---|---|
| V7.1 | Linter passes on clean codebase | `go run ./cmd/gk-lint/` (or `make lint-platform`) | Exit code 0, no violations |
| V7.2 | Linter catches deliberate violation | Temporarily add `import "goatflow/internal/repository"` to a file in `internal/platform/`, run linter, revert | Linter reports the violation with file path and import |
| V7.3 | Linter catches transitive violations | `go list -deps ./internal/platform/... \| grep -E 'goatflow/internal/(repository\|ticketnumber\|...)'` | Zero matches (integrated into the linter) |
| V7.4 | CI integration works | Run the CI pipeline (or simulate: `make lint`) | `lint-platform` passes as part of `make lint` |
| V7.5 | Linter is fast | `time go run ./cmd/gk-lint/` | < 2 seconds |
| V7.6 | Allowlist mechanism works | Add a legitimate exception to the allowlist, run linter | Exception is honored, no false positive |

#### Estimated effort: 1-2 days

---

### Phase 8 — Documentation and cleanup

#### Steps

1. Update `docs/ARCHITECTURE.md` — reflect `internal/platform/` structure; update the
   three-tier diagram to show the actual package layout.
2. Fix `docs/development/DATABASE.md:99` — false "Knowledge Base: Uses existing
   `faq_*` tables" claim. Replace with: "Knowledge Base: Planned `gk_kb_*` tables
   (future); Znuny `faq_*` import via `goatflow-migrate`."
3. Update `docs/PLUGIN_PLATFORM.md` — document the clean plugin runtime boundary.
4. Delete dead code:
   - `internal/models/knowledge.go` (261 lines, GORM-tagged, no backing tables)
   - Remove `KnowledgeArticles` field from `internal/models/service_catalog.go:100`
   - Remove TODO stub handlers from `internal/api/customer_routes.go:1561-1577`
5. Write `docs/development/PLATFORM_PACKAGES.md` — canonical list of platform vs.
   product packages, maintained alongside the linter.

#### Risks and mitigations

| # | Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|---|
| 8.1 | **Deleting `knowledge.go` breaks `service_catalog.go:100`** which references `KnowledgeArticle` | Certain | Low | Remove the `KnowledgeArticles` field from `service_catalog.go:100` in the same commit. Verify `service_catalog.go` compiles. The field is cosmetic (no queries use it — confirmed in investigation). |
| 8.2 | **Deleting TODO stub handlers in `customer_routes.go:1561-1577` breaks route registration** — the routes may be referenced in YAML | Medium | Low | Check `routes/customer.yaml:181-198` for the KB routes. If they reference the stub handlers, remove or comment out the route entries too. The FAQ plugin will register its own routes. |
| 8.3 | **Docs go stale quickly after being written** | High | Low | The linter (Phase 7) enforces the boundary mechanically. `PLATFORM_PACKAGES.md` is the human-readable companion — note in the file that the linter is the source of truth and the doc is maintained alongside it. |

#### Verification checks

| # | Check | Command | Pass criteria |
|---|---|---|---|
| V8.1 | Dead code removed | `ls internal/models/knowledge.go` (should fail); `grep -r "KnowledgeArticle" internal/` | File does not exist; zero references to `KnowledgeArticle` |
| V8.2 | `DATABASE.md:99` fixed | `grep -n "faq_\* tables" docs/development/DATABASE.md` | Either removed or corrected to reflect reality |
| V8.3 | Build still clean after deletions | `go build ./...` | Clean |
| V8.4 | Tests still pass after deletions | `go test ./...` | All pass |
| V8.5 | Docs reflect new structure | Manual review of `ARCHITECTURE.md`, `PLUGIN_PLATFORM.md`, `PLATFORM_PACKAGES.md`, `PLATFORM_BOUNDARY.md` | Diagrams and package lists match actual `internal/platform/` layout |

#### Estimated effort: 1-2 days

## 6. Cross-phase risks

| # | Risk | Affects | Mitigation |
|---|---|---|---|
| C1 | **Merge conflicts during multi-phase refactoring** — if other development continues during the refactoring, import path changes conflict with feature branches | All phases | Schedule refactoring in a dedicated branch. Merge frequently (after each phase). Communicate to all contributors. Consider a "refactoring freeze" where only import-path changes are merged. |
| C2 | **Test coverage gaps hide breakage** — if tests do not exercise the coupling points, breakage goes undetected until production | All phases | Run `go test ./...` after every phase. Add integration tests for the specific coupling points (scheduler job registration, DB connection, route resolution, API contracts). E2E tests (`make e2e`) are the safety net. |
| C3 | **golangci-lint configuration conflicts** — moving packages may trigger new linting warnings (import order, package naming) | Phases 3, 5, 6 | Run `golangci-lint run ./...` after each phase. Fix warnings in the same commit. |
| C4 | **The 114-file `models` import update (Phase 3) is the single highest-risk mechanical change** — a single missed import causes a compile error that blocks all other work | Phase 3 | Do Phase 3 in a dedicated commit. Use `goimports -local` to automate import grouping. Run `go build ./...` after every batch of 20 files. Have a rollback plan (`git reset`) if the change gets stuck. |
| C5 | **Phase ordering dependency** — Phase 3 depends on Phase 1 (scheduler type moved); Phase 4 depends on Phase 3 (models split); Phase 5 depends on Phase 4 (routing decoupled) | All phases | Follow the phase order strictly. Do not parallelize dependent phases. Phases 1 and 2 can run in parallel (independent). |
| C6 | **Performance regression from interface indirection** | Phases 1, 4 | The scheduler interface (Phase 1) and handler resolver (Phase 4) are called at **registration time** (startup), not per-request. Zero per-request overhead. Verify with a load test (`make load-test` or k6 at `tests/load/k6/goatflow_smoke.js`) before and after. |
| C7 | **Existing deployments are unaffected** — this is a code-only refactoring, no migration changes | All phases | No migration files are added, modified, or reordered. The `000001` baseline stays. `schema_migrations` table is untouched. Existing databases continue to work without any DB changes. Verify by running the full test suite against a persistent test database. |

## 7. Sequencing

```
Phase 1 (scheduler.go)         — 1-2 days  ┐
Phase 2 (database inversion)   — 2-3 days  ┤  ← can parallel with Phase 1
                                          ↓
Phase 3 (models split)         — 3-4 days  ┤  ← depends on Phase 1
                                          ↓
Phase 4 (routing decoupling)   — 3-5 days  ┤  ← depends on Phase 3
                                          ↓
Phase 5 (api/service reorg)    — 5-7 days  ┤  ← depends on Phase 4
                                          ↓
Phase 6 (move platform pkgs)   — 3-5 days  ┤  ← depends on Phases 1-5
                                          ↓
Phase 7 (linter)               — 1-2 days  ┤  ← depends on Phase 6
                                          ↓
Phase 8 (docs + cleanup)       — 1-2 days  ┘  ← depends on Phase 7

Total: ~20-30 days
```

### Recommended release mapping

| Release | Phases | Outcome |
|---|---|---|
| v0.9.0 | 1, 2, 3 | Plugin runtime decoupled from product; DB layering fixed; models split. **FAQ/KB plugin unblocked.** |
| v0.9.5 | 4, 5 | Router decoupled; API/service packages split. Structural boundary visible. |
| v0.10.0 | 6, 7, 8 | All platform packages under `internal/platform/`; linter enforces boundary; docs updated. **Extraction complete (internal).** |

## 8. What this enables

After Phase 1 alone:
- **FAQ/KB plugin can proceed** — the plugin runtime no longer transitively depends
  on ticketing code.
- Plugin runtime is a clean platform component.

After Phases 1-6:
- Clear `internal/platform/` vs `internal/` (product) structural boundary.
- Every platform package can be moved to a `goatkit` module with `git mv` + new
  `go.mod` + `replace` directive (mechanical, when a second consumer appears).
- The codebase is navigable — a developer looking for "where is auth handled" knows
  it is `internal/platform/auth/`, not buried in a flat `internal/api/auth_*.go`.

After Phase 7:
- The boundary is enforced — future contributors cannot accidentally re-entangle
  platform and product.
- CI catches violations before merge.

After Phase 8:
- Documentation reflects reality.
- Dead code (`knowledge.go`, TODO stubs) is cleaned up.
- The false `DATABASE.md:99` claim is fixed.

## 9. Future: the actual `goatkit` module split

This plan deliberately stops short of creating the `github.com/goatkit/goatkit`
module. The module split is deferred until:

1. A second consumer of GoatKit exists (e.g., goat-tables migrating to GoatKit, or a
   new product built on the platform), **or**
2. The internal reorganization is far enough along that the split is mechanical.

When that time comes, the split is:
1. `git mv internal/platform/*` to a new `goatkit/` repo/module.
2. New `go.mod` with module path `github.com/goatkit/goatkit`.
3. `goatflow/go.mod` gets `require github.com/goatkit/goatkit` + a `replace` directive
   during the transition.
4. The migration baseline stays unified in goatflow (historical artifact). New
   platform migrations going forward are authored in goatkit via `go:embed`.
5. `pkg/plugin/` moves to `goatkit/pkg/plugin/` (it is already clean — zero
   `internal/` imports).

The hard work is done by this plan. The module split is ceremony.