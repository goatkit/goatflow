# Developer Guide

GoatFlow is container-first and primarily server-rendered (HTMX). This guide is the entry point for developers.

## Quick start

- [docs/getting-started/quickstart.md](../getting-started/quickstart.md)

## Key concepts

- **Container-first dev**: use `make up`/`make down`/`make test` (don’t run `go`, `mysql`, `npm`, `curl` directly on the host)
- **HTMX-first UI**: most UI is server-rendered HTML; keep JavaScript minimal

## Helpful references

| Topic | Document |
|-------|----------|
| Operating manual (build, test, deploy commands) | [docs/development/AGENT_GUIDE.md](../development/AGENT_GUIDE.md) |
| Testing | [docs/development/TESTING.md](../development/TESTING.md) |
| Database and migrations | [docs/development/DATABASE.md](../development/DATABASE.md) |
| Writing SQL that works on MySQL and PostgreSQL | [docs/development/DATABASE_ACCESS_PATTERNS.md](../development/DATABASE_ACCESS_PATTERNS.md) |
| Configuration | [docs/development/CONFIGURATION.md](../development/CONFIGURATION.md) |
| Platform vs product packages | [docs/development/PLATFORM_BOUNDARY.md](../development/PLATFORM_BOUNDARY.md) |
| Architecture | [ARCHITECTURE.md](../ARCHITECTURE.md) |
| Contributing | [CONTRIBUTING.md](../../CONTRIBUTING.md) |
| Roadmap and status | [ROADMAP.md](../../ROADMAP.md) |
