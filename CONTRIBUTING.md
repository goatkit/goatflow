# Contributing to GoatFlow

Engineering assistants: See [docs/development/AGENT_GUIDE.md](docs/development/AGENT_GUIDE.md) for the canonical operating manual and workflow.

## Contributing

Contributions to GoatFlow are welcome.

- **Code of Conduct**: [.github/CODE_OF_CONDUCT.md](.github/CODE_OF_CONDUCT.md)
- **Development guide**: [docs/development/AGENT_GUIDE.md](docs/development/AGENT_GUIDE.md) - the canonical operating manual: container-first workflow, build, test, and deploy commands
- **Pull requests**: [.github/PULL_REQUEST_TEMPLATE.md](.github/PULL_REQUEST_TEMPLATE.md)
- **Reporting issues**: [.github/ISSUE_TEMPLATE/](.github/ISSUE_TEMPLATE/)
- **Testing**: [docs/development/TESTING.md](docs/development/TESTING.md). CI runs `make lint-platform` and `make test`, all in containers.
- **Contributor License Agreement**: [CLA.md](CLA.md)
- **Legal information**: [LEGAL.md](LEGAL.md)

## Quick Start

1. Fork the repository.
2. Turn on the pre-commit hook once: `make setup-hooks`. It scans for secrets, blocks binary
   files and checks SQL portability (`gk-lint`).
3. Create a feature branch.
4. Make your changes.
5. Run `make lint-platform` and `make test`. For database changes, also run the Go tests on
   PostgreSQL (see [TESTING.md](docs/development/TESTING.md#running-tests-on-postgresql)).
6. Submit a pull request.

## Contact

- GitHub Issues: [Report bugs or request features](https://github.com/goatkit/goatflow/issues)
- GitHub Discussions: [Ask questions and share ideas](https://github.com/goatkit/goatflow/discussions)
- Email: hello@goatflow.io

## Critical Standards

- Databases: code must work on MySQL/MariaDB and PostgreSQL. Write SQL with `?` placeholders and
  pass every SQL string through `database.ConvertPlaceholders` (or another `database.Convert*`
  function). Never write `$1`. See
  [docs/development/DATABASE_ACCESS_PATTERNS.md](docs/development/DATABASE_ACCESS_PATTERNS.md).
- **Dynamic SQL**: use `database.QueryBuilder` for any dynamic WHERE/column construction (mandatory for gosec compliance).
- No ORM: use `database/sql` with small repositories.
- Keep SQL in repositories, not handlers.
- Schema: do not change OTRS tables. Add migrations to both `migrations/mysql` and
  `migrations/postgres` with the same version number. See
  [docs/development/DATABASE.md](docs/development/DATABASE.md).
- Templating: Pongo2 only. Do not use Go's `html/template`. Render user-facing views via Pongo2 with `layouts/base.pongo2` and proper context (`User`, `ActivePage`).
- Routing: All routes defined in YAML under `routes/*.yaml` using the YAML router. Do not register routes directly in Go code.
- Tests: add/update tests for any DB-affecting change; run `make test`.
- Platform boundary: run `make lint-platform` before merging changes under `internal/platform/`; platform packages must not import product packages.

## Go Performance Standards

### Preallocate Slices (Required)
When building a slice in a loop where the size is known:

```go
// ❌ Wrong - causes multiple reallocations
var results []Item
for _, src := range items {
    results = append(results, transform(src))
}

// ✅ Correct - single allocation
results := make([]Item, 0, len(items))
for _, src := range items {
    results = append(results, transform(src))
}
```

### Use strings.Builder for Concatenation
```go
// ❌ Wrong - O(n²) allocations
var result string
for _, s := range parts {
    result += s
}

// ✅ Correct - O(n)
var b strings.Builder
for _, s := range parts {
    b.WriteString(s)
}
result := b.String()
```

Run `make toolbox-exec ARGS="golangci-lint run"` to catch these with the `prealloc` linter.

