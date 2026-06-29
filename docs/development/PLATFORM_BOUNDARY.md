# Platform Boundary

GoatFlow keeps platform infrastructure under `internal/platform/`. Product code stays under `internal/` outside that prefix.

The boundary rule is mechanical:

- A platform package may import standard-library packages, third-party packages, `github.com/goatkit/goatflow/pkg/...`, and `github.com/goatkit/goatflow/internal/platform/...`.
- A platform package must not import `github.com/goatkit/goatflow/internal/...` packages outside `internal/platform/`.
- Product packages may import platform packages. The host wiring in `cmd/goats` may import both sides.
- `pkg/plugin/` is the public plugin SDK, not an internal platform package.

## Source of truth

`cmd/gk-lint` enforces the boundary. It uses two checks:

1. Direct import scan: parses Go files under `internal/platform/` and reports actionable `file:line` diagnostics for product imports.
2. Transitive dependency scan: runs `go list -deps ./internal/platform/...` and fails if the compiler dependency graph contains any `github.com/goatkit/goatflow/internal/...` package outside `internal/platform/`.

Tests under `internal/platform/` are held to the same direct-import rule as production files.

The transitive scan is authoritative. It catches hidden coupling such as `platform/a -> platform/b -> product/c`, even when no file in `platform/a` directly imports product code.

Run it with:

```sh
make lint-platform
```

`make lint` also runs `lint-platform` before the broader Go/YAML/OpenAPI/Helm lint suite. CI runs `make lint-platform` as a required step before tests.

## Adding packages

Use the import direction to choose the side:

| Package kind | Location | Rule |
|---|---|---|
| Reusable host infrastructure | `internal/platform/<name>` | Must not depend on product packages |
| Ticketing, queues, articles, customers, SLAs, escalations | `internal/<name>` | May depend on platform packages |
| Public plugin API | `pkg/plugin` | Externally importable SDK surface |
| Product host wiring | `cmd/goats` | May connect platform and product implementations |

When a package looks generic but needs a product type, keep it product or inject a narrow platform interface implemented by product code. Do not move a package to `internal/platform/` and then allow it to import product code.

## Exceptions

Exceptions are allowed only when the platform boundary would otherwise be impossible to preserve. Add them to `allowedProductImports` in `cmd/gk-lint/main.go` with a reason and keep the exception as narrow as possible.

Current exception list: none.

Every exception is technical debt. Prefer one of these before adding one:

1. Move the shared type to `internal/platform/`.
2. Keep the package on the product side.
3. Define a small interface in platform code and wire the product implementation from `cmd/goats`.
4. Split a mixed package into platform and product files/packages.

## Review checklist

Before merging platform changes:

1. `make lint-platform` passes.
2. New imports under `internal/platform/` point only to `internal/platform/...`, `pkg/...`, standard library, or third-party packages.
3. Any new product wiring lives in product packages or `cmd/goats`, not in platform packages.
4. If a package moved sides, update imports and run the targeted package tests.
