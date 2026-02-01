# GOTRS Testing Documentation

## Overview

This document describes the testing strategy and implementation for the GOTRS ticketing system. We've implemented comprehensive unit tests for all core functionality to ensure code quality and reliability.

## Test Coverage

We have created unit tests for the following components:

### 1. Main Server (`cmd/server/main_test.go`)
- ✅ Health endpoint validation
- ✅ API status endpoint testing  
- ✅ Gin mode configuration (production/development)
- ✅ Port configuration via environment variables
- ✅ 404 handling for non-existent routes
- ✅ Method not allowed scenarios
- ✅ Performance benchmarks for endpoints

### 2. Configuration System (`internal/config/config_test.go`)
- ✅ Configuration struct validation
- ✅ Database DSN generation
- ✅ Valkey address formatting
- ✅ Server address configuration
- ✅ Environment detection (production/development)
- ✅ Auth configuration (JWT, session, password policies)
- ✅ Email configuration (SMTP, queue settings)
- ✅ Storage configuration (local, S3, attachments)
- ✅ Ticket configuration (ID format, SLA, notifications)
- ✅ Feature flags testing
- ✅ Integration configurations (Slack, Teams, webhooks)
- ✅ Configuration loading from files
- ✅ Thread-safe configuration access
- ✅ Concurrent configuration operations
- ✅ Performance benchmarks

### 3. OpenAPI Middleware (`internal/middleware/openapi_test.go`)
- ✅ OpenAPI spec validation
- ✅ Response validation against schema
- ✅ JSON schema validation (objects, arrays, primitives)
- ✅ Request/response capture
- ✅ Error handling for invalid specs
- ✅ Support for different response codes
- ✅ Nested object validation
- ✅ Array item validation
- ✅ Concurrent request handling
- ✅ Large response handling
- ✅ Empty response scenarios
- ✅ Performance benchmarks

## Running Tests

### Prerequisites

The project supports multiple container runtimes. The Makefile automatically detects and uses the best available option:
- Docker with `docker compose` plugin (v2) 
- `docker-compose` standalone (v1)
- Podman with `podman compose` plugin
- `podman-compose` standalone

### Running Tests in Containers (Recommended)

```bash
# Check which compose command will be used
make debug-env

# Start the development environment
make up

# Run all tests (auto-detects compose command)
make test

# Run tests with coverage report
make test-coverage

# Generate HTML coverage report
make test-coverage-html
```

### Running Tests Locally

If you have Go installed locally:

```bash
# Run the test script
./scripts/run_tests.sh

# Generate HTML coverage report
./scripts/run_tests.sh --html
```

### Using Go Commands Directly

```bash
# Run all tests
go test -v ./...

# Run with race detection
go test -v -race ./...

# Run with coverage
go test -v -race -coverprofile=coverage.out -covermode=atomic ./...

# View coverage report
go tool cover -func=coverage.out

# Generate HTML coverage report
go tool cover -html=coverage.out -o coverage.html
```

### Auth Bypass Controls

The regression harness disables the legacy test auth bypass to match production behavior. Set `GOTRS_DISABLE_TEST_AUTH_BYPASS=1` when running focused suites (for example `make toolbox-test-pkg`) so admin routes require real authentication. Compose-based targets (`make test`, `make test-comprehensive`) export this flag by default.

## Test Organization

Tests are organized following Go conventions:
- Test files are placed alongside the code they test
- Test files are named with `_test.go` suffix
- Test functions start with `Test` prefix
- Benchmark functions start with `Benchmark` prefix

## Test Patterns Used

### Table-Driven Tests
Used extensively for testing multiple scenarios with different inputs:
```go
testCases := []struct {
    name     string
    input    string
    expected string
}{
    {"case1", "input1", "expected1"},
    {"case2", "input2", "expected2"},
}
```

### Test Fixtures
Temporary directories and files are created for testing file operations:
```go
tmpDir := t.TempDir()
configFile := filepath.Join(tmpDir, "test-config.yaml")
```

### Mocking
HTTP responses are mocked using `httptest`:
```go
w := httptest.NewRecorder()
req, _ := http.NewRequest("GET", "/health", nil)
router.ServeHTTP(w, req)
```

### Concurrent Testing
Tests verify thread-safety and concurrent operations:
```go
var wg sync.WaitGroup
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        // concurrent operations
    }()
}
wg.Wait()
```

## Coverage Goals

- **Target Coverage**: 70% minimum
- **Critical Path Coverage**: 90% for authentication, configuration, and core business logic
- **Integration Coverage**: End-to-end tests for main user workflows

## Continuous Integration

Tests should be run on every:
- Pull request
- Commit to main branch
- Pre-release validation

## Test Dependencies

The following packages are used for testing:
- `github.com/stretchr/testify` - Assertions and test utilities
- `net/http/httptest` - HTTP testing utilities
- Standard library `testing` package

## Adding New Tests

When adding new functionality:
1. Write tests first (TDD approach recommended)
2. Ensure tests cover both happy path and error cases
3. Add benchmarks for performance-critical code
4. Update this document with new test coverage

## Performance Testing

Benchmarks are included for:
- HTTP endpoint response times
- Configuration access patterns
- JSON validation operations
- Concurrent request handling

Run benchmarks with:
```bash
go test -bench=. -benchmem ./...
```

## Test Maintenance

- Review and update tests when modifying existing functionality
- Remove obsolete tests when features are deprecated
- Keep test data minimal and focused
- Ensure tests are deterministic and don't depend on external services

## Known Issues

- Tests require testify package to be installed
- Some tests may need adjustment when running outside containers
- Database-dependent tests will be added once database layer is implemented

## Future Improvements

- [ ] Add integration tests for database operations
- [ ] Add end-to-end tests for complete workflows
- [ ] Add mutation testing for better coverage quality
- [ ] Add contract testing for API consumers
- [ ] Add load testing for performance validation
- [ ] Add security testing for authentication/authorization

## Contributing

When contributing tests:
1. Follow existing test patterns
2. Ensure tests are clear and well-documented
3. Avoid test interdependencies
4. Clean up test resources properly
5. Run the full test suite before submitting PR