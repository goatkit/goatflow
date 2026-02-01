# Developer Guide

GOTRS is container-first and primarily server-rendered (HTMX). This guide is the entry point for developers.

## Quick start

- [docs/getting-started/quickstart.md](../getting-started/quickstart.md)

## Key concepts

- **Container-first dev**: use `make up`/`make down`/`make test` (don’t run `go`, `mysql`, `npm`, `curl` directly on the host)
- **HTMX-first UI**: most UI is server-rendered HTML; keep JavaScript minimal

## Helpful references

- Architecture: [ARCHITECTURE.md](../ARCHITECTURE.md)
- Contributing: [CONTRIBUTING.md](../../CONTRIBUTING.md)
- Demo notes: [docs/DEMO.md](../DEMO.md)
- MVP status: [docs/development/MVP.md](../development/MVP.md)
