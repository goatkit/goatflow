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
├── config/       # Test configuration
├── helpers/      # Test utilities and helpers
├── fixtures/     # Test data and setup
├── auth_test.go  # Authentication tests
├── queues_test.go # Queue management tests
└── smoke_test.go # Basic smoke test
```

## Running Tests

### Quick Start

```bash
# Run all E2E tests (headless)
make test-e2e

# Run with visible browser for debugging
make test-e2e-debug

# Run in watch mode for development
make test-e2e-watch

# View test results
make test-e2e-report
```

### Environment Variables

- `BASE_URL`: Target URL for tests (default: http://localhost:8080)
- `HEADLESS`: Run browser in headless mode (default: true)
- `SLOW_MO`: Slow down actions by X milliseconds (useful for debugging)
- `SCREENSHOTS`: Capture screenshots on failure (default: true)
- `VIDEOS`: Record videos of test runs (default: false)
- `DEMO_ADMIN_EMAIL`: Admin email for login tests
- `DEMO_ADMIN_PASSWORD`: Admin password for login tests

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
make test-e2e-debug
```

### Enable Slow Motion
```bash
SLOW_MO=500 make test-e2e-debug
```

### View Videos
```bash
VIDEOS=true make test-e2e
```
Videos are saved to `test-results/videos/`

## Container Setup

The tests run in a Docker container with:
- Playwright browsers (Chromium, Firefox, WebKit)
- Go 1.22
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
Run `make test-e2e-debug` to see the browser or check screenshots in `test-results/`.

## Benefits

With this E2E testing setup, we can now:
1. **See exactly what users see** - No more guessing if the UI works
2. **Catch bugs early** - Tests run on every PR
3. **Debug visually** - Screenshots and videos show exactly what went wrong
4. **Test complex workflows** - Multi-step operations are fully tested
5. **Ensure consistency** - Same tests run locally and in CI