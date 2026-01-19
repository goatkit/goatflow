# GOTRS Roadmap

Current status, past releases, and future plans for GOTRS.

## 🚀 Current Status

**Version**: 0.6.1 (January 2026) - Automation & Access Control

GOTRS is a modern, open-source ticketing system built with Go and HTMX, designed as an OTRS-compatible replacement.

### What Works
- Agent Interface: Full ticket management with bulk actions
- Customer Portal: Login, submission, replies, closure
- Email Integration: POP3/IMAP + RFC-compliant threading + auto-responses
- Database: MySQL/MariaDB and PostgreSQL
- Automation: GenericAgent, ACLs, SLA escalations
- Integration: GenericInterface with REST/SOAP transports
- Security: Group-based queue permissions, session management
- i18n: 15 languages including RTL support
- Deployment: Docker Compose and Kubernetes Helm chart

---

## 📜 Past Releases

### [0.6.1] - January 17, 2026

**Automation & Access Control**

- GenericAgent execution engine for scheduled ticket processing
- ACL evaluation engine for dynamic form filtering
- GenericInterface framework (REST + SOAP transports)
- Group-based queue permission enforcement
- 15 languages with RTL support (ar, he, fa, ur)
- 12 new admin modules (sessions, maintenance, postmaster filters, etc.)
- Webservice dynamic field types (dropdown, multiselect)

### [0.5.1] - January 9, 2026

**Polish & Portability**

- PostgreSQL support alongside MySQL/MariaDB
- Enhanced customer portal
- Improved test coverage

### [0.5.0] - January 3, 2026

**MVP Release** - Core ticketing system complete

- Templates, Roles, Dynamic Fields, Services modules
- Customer portal with full i18n (EN, DE, ES, FR, AR)
- Email threading (Message-ID, In-Reply-To, References)
- 1000+ unit tests, Codecov integration

### [0.4.0] - October 20, 2025

- Preferred queue auto-selection
- Playwright acceptance harness
- Ticket filters (not_closed option)
- GoatKit typeahead/autocomplete modules

### [0.3.0] - September 23, 2025

- Queue detail view with statistics
- Tiptap rich text editor
- Dark mode + Tailwind palette
- Unicode support

### [0.2.0] - September 3, 2025

- DB-less fallbacks for testing
- Toolbox targets
- YAML API routing
- Auth provider registry

### [0.1.0] - August 17, 2025

- OTRS-compatible schema (116 tables)
- Docker/Podman containerization
- JWT authentication, RBAC
- LDAP integration, i18n

---

## 🔮 Future Roadmap

### 0.7.0 - June 2026

**Enhancements**
- Reporting and analytics
- Workflow automation (Process Management)
- Knowledge base
- REST API v2

### 1.0.0 - Q4 2026

**Production Release**
- All core OTRS features
- 1000+ concurrent users
- Production documentation
- Enterprise integrations
- 80% test coverage

---

## 📊 Version Summary

| Version | Date | Status |
|---------|------|--------|
| 1.0.0 | Q4 2026 | 🔮 Future |
| 0.9.0 | Jun 2026 | 🔮 Future |
| 0.8.0 | Jun 2026 | 🔮 Future |
| 0.7.0 | Jun 2026 | 🔮 Future |
| 0.6.1 | Jan 2026 | ✅ Released (Current) |
| 0.5.1 | Jan 2026 | ✅ Released |
| 0.5.0 | Jan 2026 | ✅ Released (MVP) |
| 0.4.0 | Oct 2025 | ✅ Released |
| 0.3.0 | Sep 2025 | ✅ Released |
| 0.2.0 | Sep 2025 | ✅ Released |
| 0.1.0 | Aug 2025 | ✅ Released |

---

## Get Involved

Want to influence the roadmap? [Join our Discord](https://discord.gg/gotrs) or open a [GitHub Discussion](https://github.com/gotrs-io/gotrs-ce/discussions).
