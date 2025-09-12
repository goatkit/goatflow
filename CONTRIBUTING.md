# Contributing to GOTRS

Engineering assistants: See [AGENT.md](AGENT.md) for the canonical operating manual and workflow.

## Coming Soon

Thank you for your interest in contributing to GOTRS! 

This document will contain:
- Code of Conduct
- Development setup instructions
- Coding standards and style guide
- Pull request process
- Issue reporting guidelines
- Testing requirements
- Documentation standards
- CLA (Contributor License Agreement) information

For now, please check:
- [README.md](README.md) for project overview
- [ROADMAP.md](ROADMAP.md) for development priorities
- [docs/development/MVP.md](docs/development/MVP.md) for current development focus

## Quick Start

While we prepare comprehensive contribution guidelines, here are the basics:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Ensure tests pass
5. Submit a pull request

## Contact

- GitHub Issues: [Report bugs or request features]
- Discord: [Join our community]
- Email: contribute@gotrs.io

---

*This document is under development. Check back soon for complete contribution guidelines.*

## Temporary Critical Standards

- Database access: use `database.ConvertPlaceholders` for every SQL string (no exceptions).
- No ORM: use `database/sql` with small repositories.
- Keep SQL in repositories, not handlers.
- Templating: Pongo2 only. Do not use Go's `html/template`. Render user-facing views via Pongo2 with `layouts/base.pongo2` and proper context (`User`, `ActivePage`).
- Routing: All routes defined in YAML under `routes/*.yaml` using the YAML router. Do not register routes directly in Go code.
- Tests: add/update tests for any DB-affecting change; run `make test`.

