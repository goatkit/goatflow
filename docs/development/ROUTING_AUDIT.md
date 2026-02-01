# Routing Audit & YAML Enforcement

Status: Active

## Manifest & Drift Governance (Added)
We generate `runtime/routes-manifest.json` from YAML definitions. A baseline `runtime/routes-manifest.baseline.json` is auto-created if missing. Drift detection now classifies:
* Added routes (method|path newly present)
* Removed routes
* Changed attributes per stable key (handler, redirectTo, status, middleware set, websocket flag)

Script: `scripts/check_routes_manifest.sh` (invoked by governance targets) prints structured sections. Update baseline after intentional changes.

Machine-friendly diff: `cmd/routes-diff` outputs JSON object `{added, removed, changed}` for CI or tooling.

Runtime directory is mounted as a named volume (`gotrs_runtime`) to ensure writes from toolbox and backend containers are consistent without permission fallbacks.

## Single Source of Truth
All business and API routes must be defined in YAML route files under `routes/`.
Code (`htmx_routes.go`) may only register:
* `/health`
* Static asset handlers (`/static/*`, favicon)
* Error / fallback handlers
* Bootstrapping middleware

## Why
* Eliminates drift between code + YAML
* Enables multi-document YAML loading (already implemented)
* Simplifies auditing + visualization (`generated/api-map/api-map.*`)
* Makes versioning (`/api/v1/...`) consistent

## Enforcement Mechanisms
1. **Pre-build hook** (`make build`):
   * `generate-route-map` – static scan of templates & JS → `generated/api-map/api-map.json|dot|mmd`
   * `validate-routes` – compares generated route manifest against baseline (`generated/routes-manifest.baseline.json`).
2. **Route Manifest Baseline**: `generated/routes-manifest.baseline.json`
   * Machine-readable JSON of all YAML-defined routes.
   * Update with `make routes-baseline-update` after intentional changes.
3. **Failure Condition**: Route drift (added/removed/changed routes vs baseline) fails the build unless baseline is updated.

## Migration Status: ✅ Complete
All business routes have been migrated to YAML. The migration workflow below is preserved for reference.

### Historical Migration Workflow
1. Identify a hard-coded route in `htmx_routes.go`.
2. Create / update appropriate YAML file in `routes/` with method, path, handler name.
3. Remove the code registration.
4. Run `make build`.

## Visual Map Generation
Artifacts generated each build:
* `generated/api-map/api-map.json` – canonical machine-friendly reference.
* `generated/api-map/api-map.dot` / `generated/api-map/api-map.svg` – Graphviz graph (if graphviz available).
* `generated/api-map/api-map.mmd` – Mermaid graph for docs.

## Acceptable Code Routes
If a non-business route must remain in code (e.g. temporary diagnostics), prefix it with `/dev/` and ensure it is protected; avoid adding it to baseline unless absolutely required.

## Adding New API Features
1. Define route in YAML.
2. Implement handler function referenced by `handler:` field.
3. Add tests (HTMX / API as appropriate).
4. Run `make build` (should pass without modifying baseline).

## Common Failure Scenarios
| Symptom | Cause | Resolution |
|---------|-------|------------|
| Build fails: new static routes detected | Added code route | Move to YAML or intentionally append to baseline (last resort) |
| API returns 404 though handler exists | Multi-doc YAML second document not loaded (pre-fix) or path mismatch | Confirm file delim `---` and method/path spelling |
| Route map empty | No `/api/` references in templates/JS or grep pattern too strict | Adjust `scripts/api_map.sh` regex |

## Scripts Overview
* `scripts/api_map.sh` – scans `web/templates` + `static/js` for `/api/` references; builds graphs.
* `scripts/validate_routes.sh` – audits `htmx_routes.go` for code-defined routes vs baseline.

## Future Enhancements (Optional)
* Live usage overlay (middleware + weight in DOT).
* Unused YAML route detector (YAML-defined but never referenced in templates/JS nor hit in logs).
* CI badge summarizing used vs total endpoints.

## Owner
Routing policy owned by platform / architecture maintainers. Changes require updating this document.
