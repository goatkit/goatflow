# Testing GoatFlow

All tests run in containers. Do not run `go test` on the host. The Makefile targets below start
the right containers and pass the right environment.

## Quick reference

| Task | Command |
|------|---------|
| Everything CI runs | `make lint-platform` then `make test` |
| Unit tests only (starts the test stack first) | `make test-unit` |
| Unit tests, with Go's test cache | `make test-fast` |
| One package (start the test DB first if it needs one) | `make toolbox-test-pkg PKG=./internal/api` |
| One test in one package | `make toolbox-test-pkg PKG=./internal/api TEST=TestRouteAuthorizationMatrix` |
| Template tests | `make test-templates` |
| Integration-tagged tests | `make toolbox-test-integration` (all packages) or `INT_PKGS="./internal/repository"` |
| LDAP integration | `make test-db-up` then `make test-ldap-integration` |
| OIDC integration | `make test-oidc-integration` |
| Browser E2E | `make test-stack-up` then `make test-e2e-go` / `make test-e2e-playwright-go` |
| Go SDK | `make test-sdk-go` |
| API contract tests | `make test-contracts` |
| Coverage | `make test-coverage` |
| Benchmarks | `make bench` (all) or `make bench BENCH_PACKAGES="./internal/api"` |
| k6 load smoke test | `make load-test` |
| SQL portability and platform boundary lint | `make lint-platform` |
| All linters | `make lint` |

## `make test`

`make test` is the full run. CI (`.github/workflows/test.yml`) runs it after
`make lint-platform`. It runs these steps in order:

1. `check-i18n`: `scripts/check-hardcoded-text.sh --strict` looks for UI text that is not
   translated.
2. `check-deps`: audits frontend dependencies (`bun audit` or `npm audit`). Findings do not fail
   the run.
3. `plugin-build-wasm`: builds every WASM plugin under `plugins/`.
4. `test-comprehensive`, which runs `scripts/test-runner.sh`:
   1. `scripts/validate_routes.sh` checks that no routes are hard-coded in Go. Routes belong in
      `routes/*.yaml`.
   2. Checks for Docker (with buildx) or Podman.
   3. `make test-stack-up` starts the test stack.
   4. `make test-unit` runs the unit tests.
   5. `make test-e2e-playwright-go` runs the browser tests in `tests/e2e/playwright`.
   6. Scans the backend container log. Any HTTP 500 fails the run.

Logs go to `/tmp/goatflow-test-<date>_<time>/` (`test-comprehensive.log`, `unit-tests.log`,
`e2e-tests.log`, `container-backend.log`).

## The test stack

`make test-stack-up` removes old test app containers, then starts:

| Container | What it is |
|-----------|------------|
| `mariadb-test` | MariaDB test database (`docker-compose.testdb.yml`, profile `testdb`) |
| `valkey-test` | Valkey cache for tests |
| `smtp4dev` | Email sandbox |
| `backend-test`, `runner-test`, `customer-fe-test` | GoatFlow app containers built from your working tree (`docker-compose.test.yaml`) |

It then waits for `backend-test` to answer `/health` and runs `scripts/setup-test-admin.sh`, which
sets the test admin password.

| Port on the host | Variable | Default |
|------------------|----------|---------|
| Test backend | `TEST_BACKEND_PORT` | 18081 |
| Test customer portal | `TEST_CUSTOMER_FE_PORT` | 18082 |
| MariaDB test DB | `TEST_DB_MYSQL_PORT` | set in `.env` (3308 in `.env.example`) |
| PostgreSQL test DB | `TEST_DB_POSTGRES_PORT` | set in `.env` (5433 in `.env.example`) |

Other commands:

| Command | What it does |
|---------|--------------|
| `make test-status` | Shows the test containers. |
| `make test-logs` | Follows the test container logs. |
| `make test-down` | Stops the test stack. |
| `make test-db-up` / `make test-db-down` | Starts or stops only the test database for `TEST_DB_DRIVER`. |

The test stack never touches the dev stack (`make up`).

## Test databases

Both test databases are defined in `docker-compose.testdb.yml`. Their data lives on tmpfs, so
every container start begins empty. At start-up `docker/mariadb/testdb/10-apply-migrations.sh`
and `docker/postgres/testdb/10-apply-migrations.sh` apply every `*.up.sql` file in
`migrations/mysql` or `migrations/postgres`, in version order. Seed files and
`60-set-admin-password.sh` run after that.

`TEST_DB_DRIVER` picks the database (`mysql` or `postgres`). The default in `.env.example` is
`mysql`. The Makefile maps it to `TEST_DB_MYSQL_*` or `TEST_DB_POSTGRES_*` and passes the
result to the test container as both `DB_*` and `TEST_DB_*`.

### Running tests on PostgreSQL

The test app containers in `docker-compose.test.yaml` always use MariaDB. To run Go tests against
PostgreSQL, start the PostgreSQL test DB and pass `TEST_DB_DRIVER=postgres`:

```bash
make test-db-up TEST_DB_DRIVER=postgres
make toolbox-test-pkg PKG=./internal/api TEST_DB_DRIVER=postgres
make test-unit TEST_DB_DRIVER=postgres
```

Run the Go suite on both databases before you merge a database change. See
[DATABASE_ACCESS_PATTERNS.md](DATABASE_ACCESS_PATTERNS.md) for how to write SQL that works on
both.

### Tests that need the database

Some tests (for example in `cmd/goatflow-migrate` and `cmd/goatflow-storage`) skip unless
`GOATFLOW_TEST_DB_READY=1`. Set it in `.env` (`.env.example` sets it). The make test targets
pass it through, and `make test-ldap-integration` always sets it.

`database.InitTestDB()` returns no connection when `APP_ENV=test` and neither `TEST_DB_HOST`,
`TEST_DB_NAME` nor `DATABASE_URL` is set. Tests that call it must then skip.

## Unit tests

`make test-unit` runs `scripts/unit-test-phases.sh` in the toolbox container on the host network:

1. Template tests: `./internal/platform/template/...`.
2. Packages whose tests do not use the shared test DB, in parallel.
3. Packages whose tests use the shared test DB (`database.GetDB`, `InitTestDB`, `SetDB`,
   `ResetDB`, `CloseTestDB`), one package at a time. These tests reset shared rows, so they
   must not overlap.

`tests/e2e`, `tests/integration`, `internal/email/integration` and the template package are left
out of steps 2 and 3. Tests run with `APP_ENV=test`. All three steps always run; the script
fails if any step failed.

`make test-unit` uses `-count=1`. `make test-fast` is the same without `-count=1`, so Go skips
packages that have not changed.

## Build tags

| Tag | Where | How to run |
|-----|-------|------------|
| `integration` | `internal/api`, `internal/repository`, `internal/platform/auth`, `internal/service/...`, `internal/services/...`, `internal/ticketnumber`, `internal/email/inbound/filters`, `tests/integration`, `tests/api` | `make toolbox-test-integration INT_PKGS="<packages>"`, `make test-ldap-integration`, `make test-oidc-integration` |
| `e2e` | `tests/e2e`, `tests/e2e/playwright` | `make test-e2e-go`, `make test-e2e-playwright-go` |

Plain `go test ./...` (and so `make test-unit`) does not build files with these tags.

### Integration targets

| Target | What it runs | Needs |
|--------|--------------|-------|
| `make toolbox-test-integration [INT_PKGS="..."]` | `go test -tags=integration -p 1`. Without `INT_PKGS` it runs every package that has `//go:build integration` test files. It starts the test stack first and mounts the Docker socket (the LDAP and OIDC tests start testcontainers). | Test stack, Docker |
| `make test-ldap-integration` | `^TestLDAP` tests in `./internal/api`, against an OpenLDAP container started by testcontainers. Logs in through `/api/auth/login`. | Docker socket, `make test-db-up` |
| `make test-oidc-integration` | `-tags=integration` tests in `./internal/platform/auth/...`, against a Keycloak container started by testcontainers. | Docker socket |
| `make test-integration` | `scripts/integration-test.sh`: curl checks against the test backend. | Test stack (started for you) |

## Browser E2E tests

See [tests/e2e/README.md](../../tests/e2e/README.md). In short:

```bash
make test-stack-up
make test-e2e-playwright-go                # tests/e2e/playwright
make test-e2e-go                           # tests/e2e
make test-e2e-go TEST='Groups|Queues'      # go test -run pattern
```

## Other test targets

| Target | What it runs |
|--------|--------------|
| `make test-templates` | `go test ./internal/platform/template/...` |
| `make test-contracts` | `go test ./internal/testing/contracts/...` |
| `make test-sdk-go` | `go vet`, `go test` and `go build` in `sdk/go` (a separate Go module) |
| `make test-coverage` | `scripts/run_coverage.sh`: `go test -race -coverprofile=generated/coverage.out`. It leaves out `internal/api` and `tests/`. |
| `make test-coverage-html` | HTML coverage report |
| `make bench` | `scripts/perf/run_benchmarks.sh`. By default it runs every benchmark in every package that has one. Narrow it with `BENCH_PACKAGES` and `BENCH_REGEX`; tune with `BENCH_COUNT`, `BENCH_TIME`. Output goes to `generated/benchmarks/`. |
| `make bench-compare BASE=... CANDIDATE=...` | Compares two benchmark result files. |
| `make load-test` | k6 smoke profile (`tests/load/k6/goatflow_smoke.js`) against the test backend. Output goes to `generated/load-tests/`. |

## Linting

| Target | What it checks |
|--------|----------------|
| `make lint-platform` | `go run ./cmd/gk-lint/`: SQL portability (every query goes through the conversion layer and names tables that exist on both drivers) and the platform/product import boundary. CI runs it before `make test`. |
| `make toolbox-lint` | `golangci-lint run ./...` |
| `make yaml-lint`, `make openapi-lint`, `make helm-lint` | YAML, OpenAPI spec, Helm chart |
| `make lint` | All of the above |
| `make gosec` | `gosec ./...` in the toolbox (version pinned by `GOSEC_VERSION` in `Dockerfile.toolbox`, flags in `GOSEC_FLAGS`). Test files are not scanned. CI runs the same version and flags (`make gosec-host`) and fails on any finding. Mark a false positive on the flagged line with `// #nosec G<rule> -- <why it is safe>`. |

The gk-lint rules are listed in
[DATABASE_ACCESS_PATTERNS.md](DATABASE_ACCESS_PATTERNS.md#enforcement). The import boundary is
described in [PLATFORM_BOUNDARY.md](PLATFORM_BOUNDARY.md).

## Pre-commit hook

The hook lives in `.githooks/pre-commit`. Turn it on once after cloning:

```bash
make setup-hooks          # runs: git config core.hooksPath .githooks
```

On every commit it checks:

| Check | Fails the commit when |
|-------|-----------------------|
| Secret scan | `gitleaks protect --staged` finds a secret. Uses a gitleaks container when gitleaks is not installed. If neither is available, the check is skipped with a warning. |
| Binary files | A staged file is an executable, archive, package, database, dump, media or disk image, or git sees it as binary (images and fonts are allowed). Files over 10 MB give a warning only. |
| Attribution | The commit message mentions Claude, Anthropic, `Co-Authored-By: ... Claude` or "AI generated". |
| SQL guard | `scripts/tools/check-sql.sh --staged` finds a raw `$N` placeholder or a direct `.Rebind()` call. `ILIKE` gives a warning only. Add `// sql-ok` to the line for a false positive. |
| gk-lint | Go files are staged and `CGO_ENABLED=0 go run ./cmd/gk-lint/` fails. Skipped when Go is not installed on the host. CI still runs `make lint-platform`. |

## Writing tests

- Put tests next to the code, in `_test.go` files.
- Write SQL with `?` placeholders and pass it through `database.ConvertPlaceholders`, so the test
  works on both databases.
- sqlmock expectations must not hard-code `\?` or `$1`. See
  [DATABASE_ACCESS_PATTERNS.md](DATABASE_ACCESS_PATTERNS.md#sqlmock-tests).
- A test that needs the database must skip when no test database is configured.
- A test that changes shared rows in the test DB must restore them. DB packages run one at a
  time, but tests inside a package share the same rows.
- `TestRouteAuthorizationMatrix` (`internal/api/route_authz_test.go`) sends every production
  route a request as each kind of user. A new route without the right middleware fails it.
  Anonymous access needs an entry in `authzPublicRoutes`.
