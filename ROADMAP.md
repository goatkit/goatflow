# GOTRS Roadmap

Current status, past releases, and future plans for GOTRS.

## 🚀 Current Status

**Version**: 0.6.3 (January 2026) - Stability & Theming

GOTRS is a modern, open-source ticketing system built with Go and HTMX, designed as an OTRS-compatible replacement.

### What Works
- Agent Interface: Full ticket management with bulk actions and multi-theme UI (4 themes)
- Customer Portal: Complete self-service with profile management, password changes
- Email Integration: POP3/IMAP + RFC-compliant threading + auto-responses
- Database: MySQL/MariaDB and PostgreSQL with cross-database compatibility
- Automation: GenericAgent, ACLs, SLA escalations, ticket attribute relations
- Integration: GenericInterface with REST/SOAP transports, webservice dynamic fields
- Security: Group-based queue permissions, session management, auth middleware
- i18n: 15 languages including RTL support (ar, he, fa, ur)
- Deployment: Docker Compose and Kubernetes Helm chart with multi-arch support
- Admin Modules: 30+ admin interfaces including ticket attribute relations, dynamic fields, templates

---

## 📜 Past Releases

### [0.6.3] - January 31, 2026

**Stability & Testing**

- Multi-arch Playwright E2E tests (amd64, arm64)
- Type conversion package (`internal/convert`) breaking circular dependencies
- Single YAML route loader consolidation
- Customer user lookup by login or email
- Handler registry dual registration fixes
- 90s theme button contrast improvements
- Test database setup with OTRS-compatible permissions
- UI test fixes (navigation, accessibility, error pages)

### [0.6.2] - January 25, 2026

**Theming & UX**

- Multi-theme system: Synthwave (default), GOTRS Classic, Seventies Vibes, Nineties Vibe
- Vendored fonts for offline/air-gapped deployments
- Ticket detail page refactoring (17 modular partials)
- Bulk ticket actions (assign, merge, priority, queue, status)
- Language selector partial component
- Customer password change functionality
- Ticket list pagination
- Customer profile page with preferences
- Admin ticket attribute relations (CSV/Excel import)
- Separate cookie names for agent/customer sessions

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

### Unreleased (In Progress)

**Bug Fixes & Improvements**
- Handler registry dual registration resolution
- 90s theme button contrast in dark mode
- Customer user lookup by login or email
- Queue access improvements in tests

### 0.7.0 - May 2026

**Statistics & Reporting**
- Dashboard widgets with ticket metrics
- Built-in report templates (tickets by queue, agent, SLA compliance)
- Chart visualizations (line, bar, pie)
- CSV/Excel export
- Scheduled report delivery via email
- Time tracking reports and analytics

**REST API v2**
- OpenAPI 3.0 specification
- Versioned endpoints (`/api/v2/`)
- Improved error responses with structured messages
- Rate limiting per endpoint/user
- API key authentication option
- Webhook subscriptions for events

**Mobile Optimization**
- Responsive mobile layouts for all pages
- Touch-optimized controls
- Mobile ticket creation flow
- Push notifications (PWA)

**Quality**
- 70% test coverage target
- Performance benchmarks established
- API documentation site

---

### 0.8.0 - September 2026

**FAQ / Knowledge Base**
- Public and internal article categories with permissions
- Rich text articles with attachments and images
- Search with relevance ranking and filters
- Article ratings, feedback, and usage analytics
- Link articles to tickets for quick reference
- Customer portal FAQ integration with search
- Article approval workflow

**Calendar & Appointments**
- Agent calendar view (day/week/month)
- Ticket-linked appointments with reminders
- Recurring events (daily, weekly, monthly)
- Calendar sharing between agents and teams
- iCal export/subscription
- Integration with ticket escalations
- Resource scheduling (meeting rooms, equipment)

**Self-Service Authentication**
- Password recovery UI for customers (email-based reset)
- Password recovery UI for agents (admin-initiated reset)
- Customer sign-up/registration UI with approval workflow
- Email verification for new accounts
- CAPTCHA integration (reCAPTCHA v3, hCaptcha)
- Two-factor authentication (TOTP)

**Enhancements**
- Mobile-responsive improvements across all pages
- Keyboard navigation accessibility (WCAG 2.1 AA compliance)
- Drag-and-drop file uploads
- Real-time collaborative ticket editing indicators

**Quality**
- 75% test coverage target
- Accessibility audit and fixes

---

### 0.9.0 - January 2027

**Process Management**
- Visual process designer with drag-and-drop
- Multi-step ticket workflows with validation
- Conditional transitions based on ticket data
- Custom activity dialogs with dynamic forms
- Process ticket templates with pre-filled data
- SLA integration with process steps and deadlines
- Process analytics and bottleneck identification

**Theme Engine Enhancements**
- ZIP package upload and extraction for custom themes
- Admin theme management module (enable/disable/preview)
- Sound event support (notifications, alerts, ticket actions)
- Theme marketplace integration
- Custom CSS injection per theme
- Theme versioning and updates

**Plugin System**
- Plugin architecture with hooks and filters
- Plugin marketplace and discovery
- Safe plugin sandboxing
- Hot plugin reload without restart
- Plugin development SDK and docs

**Production Preparation**
- Prometheus metrics endpoint with custom metrics
- Structured JSON logging (configurable levels)
- Health check endpoints (liveness, readiness, startup)
- Graceful shutdown handling with connection draining
- Distributed tracing (OpenTelemetry)
- Circuit breakers for external dependencies

**Quality**
- 80% test coverage target
- Load testing harness (Gatling/k6)
- Chaos engineering tests

---

### 1.0.0 - April 2027

**Production Release**

*Feature Complete*
- All OTRS core modules operational
- Process Management GA with production workflows
- FAQ/Knowledge Base GA with full-text search
- Statistics & Reporting GA with custom dashboards
- Calendar & Appointments GA with team scheduling

*Security*
- Third-party security audit completed
- Automated dependency vulnerability scanning (Dependabot, Snyk)
- Security hardening guide and best practices
- OWASP Top 10 compliance verification
- Rate limiting and DDoS protection
- Security response policy and CVE process

*Performance*
- 1000+ concurrent users verified under load
- Sub-100ms response times (p95) for all endpoints
- Database query optimization with indexes
- Caching layer (Redis/Valkey) with configurable TTLs
- Connection pooling tuning
- CDN integration for static assets

*Documentation*
- Administrator guide with best practices
- API reference (OpenAPI 3.0) with interactive docs
- Deployment guides (Docker, Kubernetes, cloud providers)
- Migration guide from OTRS 6.x with automation scripts
- Troubleshooting guide with common issues
- Video tutorials and screencasts

*Quality*
- 85% test coverage (unit + integration)
- Comprehensive Playwright E2E test suite
- Chaos engineering tests for resilience
- Performance regression testing in CI
- Automated smoke tests on production deployments

*Enterprise Features*
- High availability setup documentation
- Backup and disaster recovery procedures
- Multi-tenancy support
- LDAP/AD/SAML/OAuth integration guides
- Audit logging and compliance reporting

---

## 📊 Version Summary

| Version | Date | Status | Theme |
|---------|------|--------|-------|
| 1.0.0 | Apr 2027 | 🔮 Future | Production Release |
| 0.9.0 | Jan 2027 | 🔮 Future | Process Management & Plugins |
| 0.8.0 | Sep 2026 | 🔮 Future | FAQ & Calendar |
| 0.7.0 | May 2026 | 🔮 Future | Statistics & API v2 |
| 0.6.3 | Jan 2026 | ✅ Released | Stability & Testing |
| 0.6.2 | Jan 2026 | ✅ Released | Multi-Theme System |
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
