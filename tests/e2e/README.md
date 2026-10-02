# E2E Testing with Playwright

This directory holds the browser end-to-end tests for GoatFlow. They use
[playwright-go](https://github.com/playwright-community/playwright-go) and Chromium.
Every test file has the `e2e` build tag, so normal `go test ./...` runs skip them.

## Layout

```
tests/e2e/
├── config/        # config.go: reads BASE_URL, credentials, HEADLESS etc. from the environment
├── helpers/       # browser.go, auth.go, fixtures.go: browser setup, login, HTMX waits
├── playwright/    # second suite: 17 test files (admin pages, customer portal, ticket search, ...)
├── auth_test.go   # authentication
├── api_test.go    # API checks
├── queues_test.go # queue management
├── smoke_test.go  # basic smoke test
└── ...            # 14 more test files (admin groups, plugins, 2FA, customer tickets, ...)
```

## Running the tests

Both suites run in the `goatflow-playwright-go` container (built from
`Dockerfile.playwright-go`). They run against the **test stack**, never the dev backend.

```bash
make test-stack-up                                # build and start backend-test, runner-test, customer-fe-test
make test-e2e-playwright-go                       # tests/e2e/playwright
make test-e2e-go                                  # tests/e2e (all tests)
make test-e2e-go TEST='Groups|Queues'             # go test -run pattern
make test-e2e TEST='Login'                        # same as test-e2e-go; TEST is required
make test-e2e-playwright-go ARGS='-run TestAdminGroupsUI'
```

`make test` also runs `make test-e2e-playwright-go` after the unit tests.

Each run rebuilds the image first (`make e2e-image`). The image does not contain the source
tree. The repository is bind-mounted at `/workspace`, so source changes never need a rebuild.
Tests use `-count=1`, so results are never cached.

## Settings the Makefile passes to the container

| Setting | Default | What it does |
|---------|---------|--------------|
| `BASE_URL` | `http://backend-test:8080` | Backend under test. The `BASE_URL` in `.env` (dev backend) is ignored. Set it on the make command line to override. |
| `CUSTOMER_PORTAL_URL` | `http://customer-fe-test:8080` | Customer portal under test. Make command line only. |
| `PLAYWRIGHT_NETWORK` | `goatflow_goatflow-network` | Docker network for the container. |
| `HEADLESS` | `true` | `false` opens a visible browser. That needs a display, which the container does not have. |
| `TEST_USERNAME`, `TEST_PASSWORD` | from `.env` | Admin login. `DEMO_ADMIN_EMAIL` / `DEMO_ADMIN_PASSWORD` are used when these are empty. |
| `E2E_TIMEOUT` | `30m` | `go test -timeout` for the whole run. Go's default of 10m is too short. |
| `E2E_TMPFS_SIZE` | `4g` | Size of the container's `/tmp` tmpfs. See below. |
| `SLOW_MO` | unset (0) | Milliseconds Playwright waits after each browser action. |
| `SCREENSHOTS` | on | A screenshot is saved when a test fails. `SCREENSHOTS=false` turns this off. |
| `VIDEOS` | off | `VIDEOS=true` records a video of every browser test. |

`SLOW_MO`, `SCREENSHOTS` and `VIDEOS` are passed through from your environment or the make
command line, for example `make test-e2e-go TEST='Groups' VIDEOS=true SLOW_MO=250`.

A `localhost` or `127.0.0.1` `BASE_URL` switches the container to the host network. The customer
portal URL then becomes `http://localhost:$(TEST_CUSTOMER_FE_PORT)` (default 18082). Example:

```bash
make test-e2e-go BASE_URL=http://localhost:18081
```

Port 18081 is `TEST_BACKEND_PORT`, the host port `docker-compose.test.yaml` publishes for
backend-test.

### Why `/tmp` is a tmpfs

`TMPDIR` is `/tmp` inside the container, a tmpfs of `E2E_TMPFS_SIZE`. Playwright runs Chromium
with `--disable-dev-shm-usage`, so the browser keeps its shared memory there as files. `go test`
also builds its test binaries there. Keeping this off the bind-mounted repository stops pages
failing with `net::ERR_INSUFFICIENT_RESOURCES` or "Page crashed" when the repository disk is
nearly full.

## Test output

Paths are relative to the package directory, because `go test` runs each package in its own
directory:

| Suite | Screenshots on failure | Videos (`VIDEOS=true`) |
|-------|------------------------|--------|
| `tests/e2e` | `tests/e2e/test-results/screenshots/` | `tests/e2e/test-results/videos/` |
| `tests/e2e/playwright` | `tests/e2e/playwright/test-results/screenshots/` | `tests/e2e/playwright/test-results/videos/` |

`test-results/` is in `.gitignore`.

## Writing tests

### Basic test structure

```go
//go:build e2e

func TestFeature(t *testing.T) {
    // Setup browser. NewBrowserHelper fails the test if BASE_URL/login does not answer.
    browser := helpers.NewBrowserHelper(t)
    err := browser.Setup()
    require.NoError(t, err)
    defer browser.TearDown()

    // Login if needed
    auth := helpers.NewAuthHelper(browser)
    err = auth.LoginAsAdmin()
    require.NoError(t, err)

    t.Run("Subtest", func(t *testing.T) {
        err := browser.NavigateTo("/page")
        require.NoError(t, err)

        button := browser.Page.Locator("button#submit")
        err = button.Click()
        require.NoError(t, err)

        result := browser.Page.Locator(".result")
        text, _ := result.TextContent()
        assert.Contains(t, text, "Success")
    })
}
```

### Good practice

1. Add `data-testid` attributes to elements you need to select.
2. Call `browser.WaitForHTMX()` after actions that send an HTMX request.
3. Delete any test data your test creates.
4. Group related checks with `t.Run()`.

## CI

`.github/workflows/test.yml` runs `make lint-platform` and then `make test`. `make test` starts
the test stack and runs `make test-e2e-playwright-go` as one of its steps. To run the same E2E
steps by hand:

```bash
make test-stack-up
make test-e2e-playwright-go
make test-e2e-go
```

## Troubleshooting

| Problem | What to check |
|---------|---------------|
| `backend under test not reachable at .../login` | The test stack is not running. Run `make test-stack-up`, or set `BASE_URL`. |
| `TEST_PASSWORD (or DEMO_ADMIN_PASSWORD) must be set in .env` | Set `TEST_PASSWORD` in `.env`. |
| A single test times out | Each page action waits up to 30 seconds (`Timeout` in `config/config.go`). |
| The whole run is killed | Raise `E2E_TIMEOUT`, e.g. `make test-e2e-go E2E_TIMEOUT=60m`. |
| Pages crash after a few navigations | Raise `E2E_TMPFS_SIZE`, e.g. `E2E_TMPFS_SIZE=8g`. |
