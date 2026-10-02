# Routes: YAML Only

All HTTP routes are declared in YAML files under `routes/`. Go code does not register routes
with a literal path.

## Rules

- Add a route to the right file in `routes/` (for example `routes/admin.yaml`,
  `routes/api-v1-global.yaml`). Give it `path`, `method` and `handler` (or `handlers:` with one
  handler per method).
- The handler name must be registered in `internal/api/handler_registry.go`.
- Files can hold several YAML documents separated by `---`. All of them are loaded
  (`routing.ParseYAMLDocuments` in `internal/platform/routing/parse.go`).
- Every route needs the right access middleware. `TestRouteAuthorizationMatrix`
  (`internal/api/route_authz_test.go`) fails for a new route that anonymous users or the wrong
  kind of user can reach. See [TESTING.md](TESTING.md).

## Checks

| Check | Command | Runs in |
|-------|---------|---------|
| No literal routes in `internal/api/htmx_routes.go` (`.GET("/...")` etc.) | `make validate-routes` (`scripts/validate_routes.sh`) | `make build` (pre-build) and the first step of `make test` |
| Route manifest drift against a local baseline | `make routes-verify` | Run by hand |

### Route manifest

The manifest is a JSON list of every YAML route. The `generated/` folder is gitignored, so the
manifest and its baseline exist only on your machine.

| Command | What it does |
|---------|--------------|
| `make routes-generate` | Writes `generated/routes-manifest.json` (`cmd/routes-manifest`). |
| `make routes-verify` | Generates the manifest if missing, then runs `scripts/check_routes_manifest.sh`. The first run copies the manifest to `generated/routes-manifest.baseline.json`. Later runs print added, removed and changed routes. |
| `make routes-baseline-update` | Copies the current manifest over the baseline. |

`go run ./cmd/routes-diff` prints the same difference as JSON (`added`, `removed`, `changed`).

## Generated files

`make build` runs `pre-build`, which is:

| Target | Output |
|--------|--------|
| `generate-route-map` | `scripts/api_map.sh` scans `templates/` and `static/js/` for `/api/` calls and writes `generated/api-map/api-map.json`, `.dot`, `.mmd`, and `.svg` when Graphviz is installed. |
| `generate-route-docs` | `cmd/route-docs` writes `docs/api/api.md`, `docs/api/openapi.json` (route map) and `docs/api/index.html` from the enabled groups in `routes/`, with the same absolute paths the server registers. |
| `validate-routes` | See [Checks](#checks). |

`make api-docs` writes the same three files to `generated-docs/` from a `Dockerfile.route-tools` container.
`make openapi-generate` rebuilds the Swagger files in `docs/api/` (`swagger.json`, `swagger.yaml`, `docs.go`) from
the swag annotations; `TestSwaggerAnnotationsMatchRoutes` fails when an `@Router` names a path the server does not
route. The REST API contract is the hand-written `api/openapi.yaml`, checked by `TestOpenAPISpecMatchesRoutes` and
`make openapi-lint`.

## Common problems

| Symptom | Cause | Fix |
|---------|-------|-----|
| `make validate-routes` fails | A route was added in `htmx_routes.go` | Move it to a file in `routes/`. |
| A route answers 404 although the handler exists | Path or method spelled wrong in YAML, or the handler name is not in the handler registry | Check the YAML entry and `handler_registry.go`. |
| `TestRouteAuthorizationMatrix` fails | The new route has no or the wrong access middleware | Add the middleware in the YAML group, or add a public route to `authzPublicRoutes`. |
