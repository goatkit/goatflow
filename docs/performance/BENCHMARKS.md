# Performance Benchmarks and Load Testing

GoatFlow keeps two lightweight performance harnesses in-repo:

- Go microbenchmarks for hot-path packages and utilities.
- A k6 smoke load-test profile for deployed HTTP behavior.

Generated benchmark and load-test artifacts are written under `generated/`, which is intentionally ignored by git.

## Go Benchmarks

Run the curated baseline suite:

```sh
make bench
```

By default this runs existing benchmarks across routing, middleware, API shell setup, config, models, HTML sanitizing, and LDAP config helpers. The output is written to `generated/benchmarks/go-<timestamp>.txt`.

Useful knobs:

```sh
make bench BENCH_COUNT=5 BENCH_TIME=2s
make bench BENCH_OUT=generated/benchmarks/baseline.txt
make bench BENCH_REGEX='BenchmarkRouting|BenchmarkTemplateLoading'
make bench BENCH_PACKAGES='./internal/api ./internal/platform/routing'
```

Compare two captures with benchstat:

```sh
make bench BENCH_OUT=generated/benchmarks/base.txt
# make the change you want to measure
make bench BENCH_OUT=generated/benchmarks/candidate.txt
make bench-compare BASE=generated/benchmarks/base.txt CANDIDATE=generated/benchmarks/candidate.txt
```

When adding new benchmarks, keep them deterministic and fast enough for the default suite. Prefer benchmarks that exercise a stable boundary: routing setup, permission checks, serialization, parser logic, template rendering, cache access, or service helpers. Benchmarks requiring external services should stay opt-in, like the LDAP integration benchmark.

## k6 Load Tests

Run the default smoke profile against the containerized test stack:

```sh
make load-test
```

The target starts the test stack, waits for `/health`, then runs `tests/load/k6/goatflow_smoke.js` with k6. The default profile exercises public, same-origin routes:

- `/health`
- `/manifest.json`
- `/sw.js`
- `/login`
- `/customer/login`

Useful knobs:

```sh
make load-test LOAD_TEST_VUS=25 LOAD_TEST_DURATION=3m
make load-test LOAD_TEST_BASE_URL=http://localhost:8080 LOAD_TEST_PREPARE_STACK=0
make load-test LOAD_TEST_SUMMARY=generated/load-tests/local.json
make load-test LOAD_TEST_ENDPOINTS='/health,/login,/customer/login'
```

For weighted endpoint profiles, pass JSON:

```sh
make load-test LOAD_TEST_ENDPOINTS='[{"name":"health","path":"/health","weight":5},{"name":"login","path":"/login","weight":2}]'
```

Default k6 thresholds are intentionally conservative for the smoke profile:

- HTTP failure rate below 1%.
- p95 request duration below 750 ms.
- p99 request duration below 1500 ms.

Override them with:

```sh
make load-test K6_MAX_ERROR_RATE=0.02 K6_P95_LIMIT=1000 K6_P99_LIMIT=2500
```
