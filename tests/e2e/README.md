# E2E Testing with Playwright

This directory contains end-to-end tests for GoatFlow using Playwright with Go bindings.

## Overview

Our E2E tests provide comprehensive UI testing capabilities that allow us to:
- Verify UI elements are correctly populated
- Test user workflows end-to-end
- Catch regressions before they reach production
- Debug UI issues with screenshots and videos

## Architecture

```
tests/e2e/
├── config/        # Test configuration
├── helpers/       # Test utilities and helpers
├── playwright/    # Playwright test runner
├── auth_test.go   # Authentication tests
├── api_test.go    # API tests
├── queues_test.go # Queue management tests
├── smoke_test.go  # Basic smoke test
└── ...            # 20+ more test files (admin, customer, groups, 2FA, ...)
```

## Running Tests

### Quick Start

Both suites run in the `goatflow-playwright-go` container against the **test stack**
(never the dev backend). Credentials come from `.env` (`TEST_USERNAME` / `TEST_PASSWORD`).

```bash
make test-stack-up                                   # build + start backend-test, customer-fe-test
make test-e2e-playwright-go                          # tests/e2e/playwright
make test-e2e-go                                     # tests/e2e (all tests)
make test-e2e-go TEST='Groups|Queues'                # go test -run pattern
make test-e2e TEST='Login'                           # same as test-e2e-go TEST=...
make test-e2e-playwright-go ARGS='-run TestAdminGroupsUI'
```

### Defaults and overrides (Makefile)

- `BASE_URL`: `http://backend-test:8080` on the compose network `goatflow_goatflow-network`
  (customer portal `http://customer-fe-test:8080`). The `.env` `BASE_URL` (dev backend) is ignored;
  pass `BASE_URL=...` on the make command line to override. A localhost URL such as
  `BASE_URL=http://localhost:8082` switches to the host network and the published customer-fe-test
  port (`TEST_CUSTOMER_FE_PORT`).
- `PLAYWRIGHT_NETWORK`: force a docker network (e.g. `host`).
- `CUSTOMER_PORTAL_URL`: override the customer portal URL.
- `E2E_TIMEOUT`: `go test -timeout` for the whole run (default `30m`; go's default 10m is too short).
- `E2E_TMPFS_SIZE`: size of the container's `/tmp` tmpfs (default `4g`), which is `TMPDIR` for
  `go test` and Chromium. Playwright runs Chromium with `--disable-dev-shm-usage`, so the browser's
  shared memory lives there; keep it off the bind-mounted repository, whose free space a page
  otherwise exhausts within a few navigations (`net::ERR_INSUFFICIENT_RESOURCES` / "Page crashed").
- `HEADLESS` (default true), `SLOW_MO`, `SCREENSHOTS` (default true), `VIDEOS` (default false).

The image does not contain the source tree: the repository is bind-mounted at `/workspace`, so
source changes never rebuild the image. Tests use `-count=1` (results are never cached).

## Writing Tests

### Basic Test Structure

```go
func TestFeature(t *testing.T) {
    // Setup browser
    browser := helpers.NewBrowserHelper(t)
    err := browser.Setup()
    require.NoError(t, err)
    defer browser.TearDown()

    // Login if needed
    auth := helpers.NewAuthHelper(browser)
    err = auth.LoginAsAdmin()
    require.NoError(t, err)

    // Test your feature
    t.Run("Subtest", func(t *testing.T) {
        err := browser.NavigateTo("/page")
        require.NoError(t, err)
        
        // Find elements and interact
        button := browser.Page.Locator("button#submit")
        err = button.Click()
        require.NoError(t, err)
        
        // Assert results
        result := browser.Page.Locator(".result")
        text, _ := result.TextContent()
        assert.Contains(t, text, "Success")
    })
}
```

### Best Practices

1. **Use data attributes for testing**: Add `data-testid` attributes to elements for reliable selection
2. **Wait for HTMX**: Use `browser.WaitForHTMX()` after actions that trigger HTMX requests
3. **Clean up test data**: Delete any test data created during tests
4. **Use subtests**: Organize related tests using `t.Run()`
5. **Capture screenshots**: On failure, screenshots are automatically captured

## Debugging Failed Tests

### View Screenshots
Screenshots are saved to `test-results/screenshots/` when tests fail.

### Run with Visible Browser
```bash
make test-e2e-playwright-debug
```

### Enable Slow Motion
```bash
SLOW_MO=500 make test-e2e-playwright-debug
```

### View Videos
```bash
VIDEOS=true make test-e2e
```
Videos are saved to `test-results/videos/`

## Container Setup

The tests run in a Docker container with:
- Playwright browsers (Chromium, Firefox, WebKit)
- Go 1.25
- All necessary dependencies

To rebuild the container:
```bash
make playwright-build
```

## CI/CD Integration

The E2E tests can be integrated into CI/CD pipelines:

```yaml
# GitHub Actions example
- name: Run E2E Tests
  run: |
    make up
    make test-e2e
  env:
    HEADLESS: true
    SCREENSHOTS: true
```

## Troubleshooting

### Tests fail with "browser not found"
Run `make playwright-build` to build the container with browsers.

### Tests timeout
Increase the timeout in `config/config.go` or check if the backend is running.

### Can't see what's happening
Run `make test-e2e-playwright-debug` to see the browser or check screenshots in `test-results/`.

## Benefits

With this E2E testing setup, we can now:
1. **See exactly what users see** - No more guessing if the UI works
2. **Catch bugs early** - Tests run on every PR
3. **Debug visually** - Screenshots and videos show exactly what went wrong
4. **Test complex workflows** - Multi-step operations are fully tested
5. **Ensure consistency** - Same tests run locally and in CI