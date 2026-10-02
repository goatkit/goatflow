# GoatFlow Features

Status of each feature in GoatFlow 0.10.0.

Legend:
- ✅ Works
- ⚠️ Partly works (the note says what is missing)
- ⏳ In progress outside the code (for example, a store submission)
- ❌ Not available

## Core Features

### Ticket Management
- ✅ Create, read, update, delete tickets
- ✅ Ticket numbering (Increment, Date, DateChecksum, Random generators)
- ✅ Priority levels (OTRS set: 1 very low, 2 low, 3 normal, 4 high, 5 very high)
- ✅ Ticket states (OTRS set: new, open, pending reminder, pending auto close+/-, closed successful, closed unsuccessful, merged, removed)
- ✅ Queue assignment
- ✅ Agent assignment (owner and responsible)
- ✅ Customer association
- ✅ Ticket history (`ticket_history`, one entry per changed field)
- ✅ Internal notes (not visible to customers)
- ✅ Email notifications (customers get emails on ticket create, reply and note; Admin -> Ticket Notifications rules send OTRS-style notifications on ticket, article and escalation events, see [admin guide](admin-guide/README.md#ticket-notifications))

### User Management
- ✅ Login for agents and customers
- ✅ Customer self-registration with email confirmation (`features.registration`, off by default)
- ✅ Role-based access control (Admin, Agent, Customer); admin rights come only from admin-group membership
- ✅ User profiles
- ✅ User preferences (language, theme, session timeout, reminder notifications)
- ✅ Password reset by email for agents and customers (`features.lost_password`, on by default; links use `BASE_URL`)
- ✅ Session management
- ✅ Queue permissions through groups and roles

### Communication
- ✅ Email integration (SMTP sending; IMAP and POP3 mail accounts in Admin -> Mail Accounts)
- ✅ Email-to-ticket conversion
- ✅ Reply by email
- ❌ CC/BCC (the article API stores a Cc value, but mail is sent to one recipient)
- ✅ HTML email support

### Basic UI
- ✅ Agent dashboard
- ✅ Customer portal (tickets, company pages, profile, 2FA)
- ✅ Ticket list view
- ✅ Ticket detail view
- ✅ Search
- ✅ Responsive design

## Standard Features

### Enhanced Ticket Management
- ✅ Ticket templates (Admin -> Templates, import/export)
- ✅ Canned responses (create, edit, share, import/export, statistics)
- ✅ Ticket merging (single and bulk)
- ❌ Ticket splitting (not available)
- ⚠️ Ticket linking (existing links are shown; links cannot be created or removed in the UI)
- ✅ Bulk operations (status, priority, queue, assign, lock, merge)
- ✅ Custom fields (dynamic fields, including web service fields)
- ✅ File attachments (stored in the database or an OTRS-compatible filesystem tree; PDF and image thumbnails)
- ✅ Ticket locking
- ❌ Watch/Follow tickets
- ❌ Ticket tags
- ✅ Time tracking (`time_accounting`)

### Search & Filters
- ✅ Search across tickets, articles and customers, limited to readable queues. Default: database (every word must match as a substring, `LIKE`). Optional Zinc or Elasticsearch full-text index kept in sync by the runner (see [ZINC_SEARCH.md](ZINC_SEARCH.md))
- ✅ Ticket filters
- ✅ Saved searches
- ❌ Search templates
- ❌ Quick filters
- ❌ Search history

### SLA Management
- ✅ SLA definitions (Admin -> SLA)
- ✅ OTRS escalation index: first response, update and solution times are computed from the ticket's SLA (else its queue) in working time and stored in `ticket.escalation_*`. The scheduler job `escalation-index` (every 15 seconds) rebuilds every ticket changed since its last run, and all tickets after an SLA, queue or calendar change
- ✅ Escalation shown in the ticket view (due time, overdue) and in the agent ticket list (Escalated badge and filter)
- ✅ Escalation events: the `escalation-check` job (every minute) writes `Escalation*TimeStart` / `Escalation*TimeNotifyBefore` history events (repeat interval `OTRSEscalationEvents::DecayTime`, default 1440 minutes); a change that ends a started escalation writes `Escalation*TimeStop`. Webhooks and ticket notifications use these events
- ⚠️ Business hours (OTRS `TimeWorkingHours` and `TimeWorkingHours::Calendar1`..`9` settings with `TimeZone::CalendarN`; default calendar in `app.timezone`; no admin page)
- ⚠️ Holiday calendars (read from `TimeVacationDays` / `TimeVacationDaysOneTime` settings; no admin page)
- ❌ SLA reporting
- ❌ Breach notifications

### Workflow Automation
- ✅ GenericAgent execution engine (Admin -> Generic Agent, runs every minute)
- ✅ Time-based triggers (GenericAgent schedules)
- ✅ Event-based triggers (GenericAgent conditions)
- ✅ Automated actions (GenericAgent actions)
- ✅ Conditional logic (GenericAgent conditions)
- ❌ Workflow templates
- ❌ Round-robin assignment
- ❌ Load balancing

### Reporting & Analytics
- ✅ Dashboard widgets (statistics plugin, drag and resize with gridstack.js)
- ✅ Reports page (Admin -> Reports): ticket totals, created-vs-closed trend (7 days, 30 days, 12 months), open/backlog per queue, agent activity, top customers. See [REPORTS.md](REPORTS.md)
- ❌ Custom report builder
- ⚠️ Export (CSV and JSON from Admin -> Reports; no Excel or PDF)
- ❌ Scheduled reports
- ❌ Report sharing

### Customer Management
- ✅ Customer companies (Admin -> Customer Companies: users, tickets, services and portal settings per company; customers see their own company at `/customer/company`)
- ❌ Customer hierarchies
- ✅ Customer users (create, edit, import, export, bulk actions)
- ❌ Customer history
- ❌ Customer notes
- ❌ VIP customer flags

### Knowledge Base
The knowledge base is not part of core. It comes from the **goat-kb** plugin. When the plugin is installed it adds the customer knowledge base at `/customer/kb`.

## Advanced Features

### Multi-Channel Support
- ⚠️ Web forms (agent email and phone ticket forms, customer portal form; no public embeddable form)
- ✅ API integration (REST API and outbound webhooks)
- ❌ Chat integration
- ❌ Social media
- ❌ Phone integration (VoIP)
- ❌ SMS
- ❌ WhatsApp Business

### Authentication
- ✅ Single sign-on (Admin -> Identity Providers)
- ✅ SAML 2.0
- ✅ OpenID Connect (PKCE, JWKS verification, auto-provisioning)
- ✅ OAuth 2.0 login with Google and GitHub (GoatFlow is the client, not an OAuth server)
- ✅ LDAP/Active Directory agent login (bind + search, StartTLS/LDAPS with certificate verification, optional account creation, name/email sync and admin-group mapping; see [LDAP.md](LDAP.md))
- ✅ Multi-factor authentication: TOTP, passkeys/security keys (WebAuthn), recovery codes, admin override; users list and remove their own passkeys
- ✅ Passkey login (without a password)
- ✅ Refresh tokens (`POST /api/v1/auth/refresh`)
- ✅ API tokens (scopes, expiry and a per-token hourly rate limit)

### Collaboration
- ❌ Team inbox
- ❌ Collision detection
- ⚠️ Real-time updates (Server-Sent Events for plugin events; no WebSocket)
- ❌ Agent chat
- ❌ Screen sharing
- ❌ Co-browsing
- ❌ Presence indicators

### Process Management
- ❌ Visual workflow designer
- ❌ BPMN 2.0 support
- ❌ Process templates
- ❌ Approval workflows
- ❌ Parallel processes
- ❌ Process versioning
- ❌ Process analytics

### Asset Management
- ❌ Configuration items (CMDB)
- ❌ Asset relationships
- ❌ Asset lifecycle
- ❌ Software license management
- ❌ Hardware inventory
- ❌ Warranty tracking
- ❌ Depreciation calculation

### Project Management
- ❌ Project tickets
- ❌ Gantt charts
- ❌ Resource allocation
- ❌ Milestone tracking
- ❌ Budget management
- ❌ Project templates

### Migration & Storage
- ✅ OTRS 6 / Znuny 6.x import with `goatflow-migrate` (from a dump file or a live MySQL/MariaDB or PostgreSQL database; one transaction; keeps OTRS ids). See [MIGRATION.md](MIGRATION.md)
- ✅ OTRS-compatible attachment storage: database (`db`) or ArticleStorageFS tree (`fs`), with the `goatflow-storage` command to move between them. See [ARTICLE_STORAGE.md](ARTICLE_STORAGE.md)
- ✅ MySQL/MariaDB and PostgreSQL (Oracle and SQL Server are not implemented)

## Enterprise Features

### ITSM Suite
- ❌ Incident Management
- ❌ Problem Management
- ❌ Change Management
- ❌ Release Management
- ⚠️ Service Catalog (services can be defined and assigned to customer users; no catalog page)
- ⚠️ Service Level Management (SLA definitions only; see SLA Management)
- ❌ Capacity Management
- ❌ Availability Management

### Advanced Security
- ❌ Field-level encryption (only plugin secure settings and webhook signing secrets are encrypted)
- ❌ Data loss prevention (DLP)
- ⚠️ Audit logging (ticket history; admin changes record the acting admin; admin 2FA overrides are logged)
- ❌ Session recording
- ❌ Compliance reporting (GDPR, HIPAA)
- ❌ Security incident response

### High Availability
See [HIGH_AVAILABILITY.md](HIGH_AVAILABILITY.md).
- ❌ Active-active clustering (Valkey is a shared cache only; scheduled jobs are coordinated through a database lock so each tick runs on one replica)
- ❌ Database replication (use a managed database)
- ❌ Failover mechanisms
- ❌ Disaster recovery
- ❌ Backup automation
- ❌ Point-in-time recovery
- ❌ Geographic distribution

### Multi-Tenancy
- ⚠️ Organisations: members, per-organisation settings, plugin access, captive plugin and identity providers (`/api/v1/organisations`). Plugin calls run in the caller's organisation: plugin secrets and files are kept per organisation, and plugin queries on organisation-owned tables (`gk_org_plugin_access`, `gk_user_organisation`, `sysconfig_org`, `gk_identity_provider_org`) are scoped to it. Core tickets, queues and customers are not separated by organisation, and plugin-owned tables are not scoped by the platform. Separate departments with queue/group permissions; give each client its own instance (see [Admin guide](admin-guide/README.md#separating-departments-and-clients))
- ❌ Resource quotas
- ❌ Billing integration
- ❌ White-labeling
- ❌ Custom domains

### Advanced Integrations
- ❌ ERP systems (SAP, Oracle)
- ❌ CRM systems (Salesforce, HubSpot)
- ❌ DevOps tools (Jira, GitLab, Jenkins)
- ✅ Prometheus metrics (`/metrics`, optional separate `METRICS_PORT` listener)
- ❌ Communication platforms (Slack, Teams, Discord)
- ❌ Payment gateways
- ❌ Shipping providers
- ❌ Cloud storage (S3, Azure Blob, GCS)

## AI/ML Features

### Intelligent Automation
- ❌ Smart ticket categorization
- ❌ Auto-tagging
- ❌ Priority prediction
- ❌ Agent recommendation
- ❌ Response time prediction
- ❌ Sentiment analysis
- ❌ Language detection in tickets
- ❌ Translation services

### Predictive Analytics
- ❌ Ticket volume forecasting
- ❌ Resource planning
- ❌ Customer churn prediction
- ❌ Issue trend analysis
- ❌ Performance prediction
- ❌ Anomaly detection
- ❌ Root cause analysis

### AI Assistant
- ✅ MCP server for AI assistants (`/api/mcp`, runs with the caller's permissions)
- ❌ Suggested responses
- ❌ Answer recommendations
- ❌ Knowledge base suggestions
- ❌ Similar ticket detection
- ❌ Chatbot integration
- ❌ Voice assistant

## Platform Features

### Developer Tools
- ✅ REST API v1 (OpenAPI 3.0 spec in `api/openapi.yaml`, Swagger UI at `/swagger/`)
- ✅ Webhooks (Admin -> Webhooks: ten ticket and article events, HMAC-SHA256 signature, retries, delivery log and redeliver). See [WEBHOOKS.md](WEBHOOKS.md)
- ✅ SDKs (Go, Python, TypeScript in `sdk/`)
- ✅ CLI tools (`gk plugin init`, `gk install/update/search/build/sign`, `goatflow-migrate`, `goatflow-storage`)
- ✅ API documentation (`make api-docs` regenerates it from `routes/*.yaml`)
- ✅ Markdown rendering API (`POST /api/v1/markdown/render`)
- ❌ WebSocket API (real-time uses Server-Sent Events)
- ❌ Postman collections

### Extension Framework
- ✅ Plugin architecture (WASM via wazero + gRPC via go-plugin)
- ✅ Plugin marketplace (Admin -> Marketplace, `gk install/update/search`)
- ✅ Theme system (4 built-in themes, dark/light modes)
- ✅ Custom widgets (plugin widgets, filtered by permissions)
- ✅ Widget drag/resize (gridstack.js, 12-column grid, per-user layout)
- ✅ Plugin navigation control (hide built-in nav items, custom landing page, Profile menu location)
- ⚠️ Hooks (plugins can register scheduled jobs and deletion cascades; no ticket-event hooks)
- ⚠️ Plugin events (plugins publish events to browsers over SSE, with an optional EventAuthorizer; plugins cannot subscribe to platform events)
- ⚠️ Sandboxed execution (WASM memory and call-time limits; gRPC plugins get mount and PID namespaces when GoatFlow runs as root outside a container; no CPU or memory limits)
- ✅ Hot reload (on unless `GOATFLOW_PLUGIN_HOT_RELOAD=false`; the container image sets it to `false`)

### Monitoring & Observability
See [OBSERVABILITY.md](OBSERVABILITY.md).
- ✅ Health checks (`/health` pings the database and returns 503 when it is down; `/health/detailed` for admins)
- ✅ Prometheus metrics
- ✅ Structured logging (`LOG_FORMAT=json|text`, `LOG_LEVEL`, `LOG_OUTPUT`)
- ✅ Graceful shutdown with connection draining (`DRAIN_TIMEOUT`)
- ❌ Tracing (OpenTelemetry)
- ❌ Error tracking
- ❌ Usage analytics

### Deployment Options
- ✅ Docker / Podman Compose
- ⚠️ Kubernetes Helm chart (K8s 1.25+; in 0.10.0 some variables must be added with `backend.extraEnv`, see [HIGH_AVAILABILITY.md](HIGH_AVAILABILITY.md))
- ⏳ TrueNAS SCALE app catalog (package ready in `docs/truenas-app/`, PR in submission)
- ✅ Auto-scaling (Helm HPA with CPU/memory targets)
- ❌ Ansible playbooks
- ❌ Cloud marketplace (AWS, Azure, GCP)
- ❌ One-click installers

## Mobile Features

### Mobile Apps (Native)
- ❌ iOS app
- ❌ Android app

### Progressive Web App (PWA)
- ✅ Install to home screen (`/manifest.json`)
- ✅ Offline page (service worker `/sw.js`)
- ✅ Web push notifications (VAPID)
- ❌ Background sync
- ✅ App-like (standalone) display
- ✅ Responsive design

## Accessibility Features

### WCAG 2.1
- ⚠️ Screen reader support (ARIA labels present, no full audit)
- ⚠️ Keyboard navigation (basic support, no full audit)
- ❌ High contrast mode
- ❌ Font size adjustment
- ❌ Color blind modes
- ✅ Focus indicators
- ✅ ARIA labels
- ✅ Skip-to-content link

## Localization

### Multi-Language Support
- ✅ Interface translation (15 languages)
- ✅ Right-to-left (RTL) support (Arabic, Hebrew, Persian, Urdu)
- ✅ Date/time localization
- ✅ Number formatting
- ✅ Currency formatting
- ❌ Per-user time zones
- ❌ Custom translations
- ✅ Language detection (`?lang=`, cookie, user preference, browser `Accept-Language`)
- ✅ User language preference

### Supported Languages
- ✅ English (en) - Base language
- ✅ Arabic (ar) - RTL, Arabic-Indic numerals
- ✅ German (de)
- ✅ Spanish (es)
- ✅ French (fr)
- ✅ Japanese (ja)
- ✅ Polish (pl)
- ✅ Portuguese (pt)
- ✅ Russian (ru)
- ✅ Ukrainian (uk)
- ✅ Urdu (ur) - RTL
- ✅ Hebrew (he) - RTL
- ✅ Chinese (zh)
- ✅ Persian (fa) - RTL, Persian numerals
- ✅ Klingon (tlh)

## Performance Features

### Optimization
- ✅ Database indexes (OTRS schema plus GoatFlow tables)
- ✅ Caching (Valkey)
- ❌ CDN support
- ❌ Lazy loading
- ✅ Image processing (libvips: WebP, AVIF, HEIC)
- ❌ HTTP compression

### Scalability
- ✅ Horizontal scaling (Helm HPA)
- ✅ Vertical scaling (resource limits)
- ❌ Database sharding
- ❌ Read replicas
- ✅ Connection pooling
- ✅ Rate limiting (login, second-factor codes, passkey login, password re-checks in profile/2FA settings, password reset and sign-up, API tokens, public plugin pages, plugin webhooks)
- ❌ Circuit breakers

## Comparison Matrix (GoatFlow 0.10.0)

| Feature Category | GoatFlow | OTRS | Zendesk | ServiceNow |
|-----------------|-------|------|---------|------------|
| Core Ticketing | ✅ | ✅ | ✅ | ✅ |
| Email Integration | ✅ | ✅ | ✅ | ✅ |
| Knowledge Base | ⚠️ (goat-kb plugin) | ✅ | ✅ | ✅ |
| SLA Management | ⚠️ | ✅ | ✅ | ✅ |
| Workflow Automation | ⚠️ | ✅ | ✅ | ✅ |
| Plugin Platform | ✅ | ❌ | ⚠️ | ✅ |
| Theme Engine | ✅ | ❌ | ⚠️ | ⚠️ |
| Dark Mode | ✅ | ❌ | ⚠️ | ⚠️ |
| API Access | ✅ | ⚠️ | ✅ | ✅ |
| API Documentation | ✅ | ⚠️ | ✅ | ✅ |
| MFA/2FA | ✅ | ⚠️ | ✅ | ✅ |
| Multi-Channel | ⚠️ | ⚠️ | ✅ | ✅ |
| ITSM Suite | ❌ | ✅ | ❌ | ✅ |
| AI/ML Features | ⚠️ | ❌ | ✅ | ✅ |
| Multi-Tenancy | ⚠️ | ❌ | ✅ | ✅ |
| High Availability | ❌ | ⚠️ | ✅ | ✅ |
| Source Code Access | ✅ | ✅ | ❌ | ❌ |
| Self-Hosted | ✅ | ✅ | ❌ | ✅ |
| Cloud Native | ✅ | ❌ | ✅ | ✅ |
| Air-Gapped Deploy | ✅ | ⚠️ | ❌ | ⚠️ |
| Modern UI | ✅ | ❌ | ✅ | ✅ |
| Localization | ✅ | ✅ | ✅ | ✅ |
