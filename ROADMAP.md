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

### 0.7.0 - May 2026

**Statistics & Reporting**
- Dashboard widgets with ticket metrics
- Built-in report templates (tickets by queue, agent, SLA compliance)
- Chart visualizations (line, bar, pie)
- CSV/Excel export
- Scheduled report delivery via email

**REST API v2**
- OpenAPI 3.0 specification
- Versioned endpoints (`/api/v2/`)
- Improved error responses
- Rate limiting
- API key authentication option

**Quality**
- 65% test coverage target
- Performance benchmarks established

---

### 0.8.0 - September 2026

**FAQ / Knowledge Base**
- Public and internal article categories
- Rich text articles with attachments
- Search with relevance ranking
- Article ratings and feedback
- Link articles to tickets
- Customer portal FAQ integration

**Calendar & Appointments**
- Agent calendar view
- Ticket-linked appointments
- Recurring events
- Calendar sharing between agents
- iCal export/subscription

**Self-Service Authentication**
- Password recovery UI for customers (forgot password flow)
- Password recovery UI for agents (forgot password flow)
- Customer sign-up/registration UI
- Email verification for new accounts
- CAPTCHA integration option

**Enhancements**
- Mobile-responsive improvements
- Keyboard navigation accessibility (WCAG 2.1 AA)

**Quality**
- 70% test coverage target

---

### 0.9.0 - January 2027

**Process Management**
- Visual process designer
- Multi-step ticket workflows
- Conditional transitions
- Custom activity dialogs
- Process ticket templates
- SLA integration with process steps

**Theme Engine Enhancements**
- ZIP package upload and extraction
- Admin theme management module
- Sound event support (notifications, alerts)
- Theme marketplace preparation

**Production Preparation**
- Prometheus metrics endpoint
- Structured JSON logging
- Health check endpoints (liveness, readiness)
- Graceful shutdown handling

**Quality**
- 75% test coverage target
- Load testing harness

---

### 1.0.0 - April 2027

**Production Release**

*Feature Complete*
- All OTRS core modules operational
- Process Management GA
- FAQ/Knowledge Base GA
- Statistics & Reporting GA
- Calendar & Appointments GA

*Security*
- Third-party security audit
- Dependency vulnerability scanning
- Security hardening guide

*Performance*
- 1000+ concurrent users verified
- Sub-100ms response times (p95)
- Database query optimization
- Caching layer (Redis optional)

*Documentation*
- Administrator guide
- API reference (OpenAPI)
- Deployment guides (Docker, Kubernetes, bare metal)
- Migration guide from OTRS 6.x

*Quality*
- 80% test coverage
- Playwright E2E test suite
- Chaos engineering tests

---

## 📊 Version Summary

| Version | Date | Status | Theme |
|---------|------|--------|-------|
| 1.0.0 | Apr 2027 | 🔮 Future | Production Release |
| 0.9.0 | Jan 2027 | 🔮 Future | Process Management |
| 0.8.0 | Sep 2026 | 🔮 Future | FAQ & Calendar |
| 0.7.0 | May 2026 | 🔮 Future | Statistics & API v2 |
| 0.6.2 | Jan 2026 | ✅ Released | Add themes engine |
| 0.6.1 | Jan 2026 | ✅ Released | Automation & ACLs |
| 0.5.1 | Jan 2026 | ✅ Released | Polish & Portability |
| 0.5.0 | Jan 2026 | ✅ Released | MVP Release |
| 0.4.0 | Oct 2025 | ✅ Released | Filters & Typeahead |
| 0.3.0 | Sep 2025 | ✅ Released | Rich Text & Dark Mode |
| 0.2.0 | Sep 2025 | ✅ Released | YAML Routing |
| 0.1.0 | Aug 2025 | ✅ Released | Foundation |

---

## Get Involved

Want to influence the roadmap? [Join our Discord](https://discord.gg/gotrs) or open a [GitHub Discussion](https://github.com/gotrs-io/gotrs-ce/discussions).
