# GoatFlow MVP Status & Delivery Schedule

This document tracks MVP completion and the delivery schedule.

## Source of truth

- Status checklist lives in: [ROADMAP.md](../../ROADMAP.md)
- Setup/usage docs: [docs/getting-started/quickstart.md](../getting-started/quickstart.md) and [README.md](../../README.md)

## MVP definition

MVP means: “a usable ticket system end-to-end” (agent workflow + core communications), running via containers.
## MVP Scope

### Core Requirements
- ✅ Basic ticket CRUD operations (completed October 28, 2025)
- ✅ User authentication and authorization (completed September 2025)
- ✅ Email integration (send/receive) (completed November 1, 2025 outbound; December 2025 inbound)
- ✅ Basic workflow (New → Open → Resolved → Closed) (completed October 28, 2025)
- ✅ Agent and customer portals (agent: ✅ completed Oct 2025; customer: target Jan 4, 2026)
- ✅ Basic search functionality (completed September 2025)
- ✅ Docker deployment (completed September 2025)

### Explicitly Excluded from MVP (Not MVP)
- ❌ Complex workflows
- ❌ Advanced reporting
- ❌ Multiple languages
- ❌ Plugin system
- ❌ Mobile apps
- ❌ AI/ML features
- ❌ ITSM modules
## MVP success criteria (reviewed December 20, 2025)

These items mirror the MVP checklist in `ROADMAP.md`. Notes here are purely clarifying.

- [x] Agents can create and manage tickets (✅ completed October 28, 2025)
- [ ] Customers can submit tickets via web form (target: January 4, 2026)
	- Remaining MVP work: customer-facing create ticket flow (UI + handler + persistence + permissions)
- [x] Basic ticket workflow (new → open → closed) (✅ completed October 28, 2025)
- [x] Comments/articles on tickets work (✅ completed October 28, 2025)
- [x] Email notifications sent on ticket events (✅ completed November 1, 2025)
	- Outbound: threaded notifications on create + public reply
	- Inbound: POP3/IMAP pipeline operational (December 2025)
- [x] Search tickets by number or title (✅ completed September 2025)
- [x] 5+ test tickets successfully processed (✅ completed November 2025)
- [ ] System stable for 48 hours without crashes (target: January 18, 2026)
	- Remaining MVP work: run a controlled 48h burn-in and fix any crashers
- [x] Basic documentation for setup and usage (✅ completed December 2025)

## Delivery schedule (baseline from December 20, 2025)

Re-aligned schedule assuming **1 developer**. Major agent-side features complete; customer portal is the primary remaining work.

- **Week 1 (Dec 22–Dec 28, 2025)**: Customer web submission scaffold + persistence
  - Customer user authentication and session management
  - Customer ticket creation form UI
  - Backend handler for customer ticket submission
- **Week 2 (Dec 29, 2025–Jan 4, 2026)**: Customer web submission complete + harden + docs
  - Customer ticket list view (own tickets only)
  - Customer reply to own tickets
  - Permission isolation (customers can't see other customers' tickets)
  - Documentation updates
- **Week 3 (Jan 5–Jan 11, 2026)**: Stability pass + first 24h burn-in
  - Load testing with concurrent users
  - Memory leak detection
  - First 24-hour stability test
  - Bug fixes from burn-in
- **Week 4 (Jan 12–Jan 18, 2026)**: Complete 48h burn-in + cut MVP release (v0.5.0)
  - Final 48-hour stability test
  - Performance verification
  - Cut release v0.5.0
  - Deployment documentation

The detailed checklist for each week lives in [ROADMAP.md](../../ROADMAP.md).

## Day-by-Day Development Schedule

### Day 1-3: Setup
- [x] Project initialized with Go modules and frontend setup (✅ completed August 2025)
- [x] Install backend dependencies (gin, jwt, database drivers) (✅ completed August 2025)
- [x] Frontend dependencies configured (✅ completed September 2025)

### Day 4-10: Backend Development
- [x] Database schema and migrations (✅ completed September 2025)
- [x] User authentication endpoints (✅ completed September 2025)
- [x] Ticket CRUD operations (✅ completed October 28, 2025)
- [x] Email service integration (✅ completed November 1, 2025 outbound; December 2025 inbound)
- [x] Basic validation and error handling (✅ completed October 2025)

### Day 11-20: Frontend Development
- [x] Authentication flow (✅ completed September 2025)
- [x] Agent dashboard (✅ completed October 2025)
- [x] Ticket management UI (✅ completed October 28, 2025)
- [ ] Customer portal (target: January 4, 2026)
- [x] Basic search and filters (✅ completed September 2025)

### Day 21-25: Integration
- [x] Connect frontend to backend (✅ completed October 2025)
- [x] Email notifications (✅ completed November 1, 2025)
- [x] File uploads (✅ completed November 2025)
- [ ] Real-time updates (deferred post-MVP; using page refresh for now)

### Day 26-30: Testing & Deployment
- [x] Unit tests for critical paths (✅ completed November 2025)
- [x] Integration tests (✅ completed November 2025)
- [x] Docker configuration (✅ completed September 2025)
- [x] Basic documentation (✅ completed December 2025)
- [ ] Demo data generation (target: January 2026)

## MVP Features Checklist

### Authentication
- [x] User registration (✅ completed September 2025)
- [x] Email/password login (✅ completed September 2025)
- [x] JWT tokens (✅ completed September 2025)
- [x] Role-based access (admin, agent, customer) (✅ completed September 2025)
- [x] Password reset via email (✅ completed November 2025)

### Tickets
- [x] Create ticket (✅ completed October 28, 2025)
- [x] View ticket list (✅ completed October 28, 2025)
- [x] View ticket details (✅ completed October 28, 2025)
- [x] Update ticket status (✅ completed October 28, 2025)
- [x] Add messages/comments (✅ completed October 28, 2025)
- [x] Assign to agent (✅ completed October 28, 2025)
- [x] Set priority (✅ completed October 28, 2025)
- [x] Basic search (✅ completed September 2025)

### Email
- [x] Send ticket notifications (✅ completed November 1, 2025)
- [x] Receive tickets via email (✅ completed December 2025)
- [x] Reply to ticket via email (✅ completed December 2025)

### UI/UX
- [x] Responsive design (✅ completed October 2025)
- [x] Agent dashboard (✅ completed October 2025)
- [ ] Customer portal (target: January 4, 2026)
- [x] Login/logout (✅ completed September 2025)
- [x] Basic theming (✅ completed October 2025)

### Deployment
- [x] Docker containers (✅ completed September 2025)
- [x] Docker Compose setup (✅ completed September 2025)
- [x] Environment configuration (✅ completed September 2025)
- [x] Basic health checks (✅ completed September 2025)

## MVP Success Criteria Status

### Functional
- [x] Can create and manage tickets (✅ completed October 28, 2025)
- [x] Email notifications work (✅ completed November 1, 2025)
- [x] Users can login and see appropriate data (✅ completed September 2025)
- [x] Basic workflow functions (✅ completed October 28, 2025)

### Performance
- [x] Page loads < 3 seconds (✅ verified November 2025)
- [x] API responses < 500ms (✅ verified November 2025)
- [ ] Supports 100 concurrent users (target: January 18, 2026 burn-in test)

### Quality
- [x] No critical bugs (✅ ongoing; last verified December 2025)
- [x] Core features have tests (✅ 65% coverage as of December 2025)
- [x] Documentation exists (✅ completed December 2025)
- [x] Deployable with Docker (✅ completed September 2025)

## Post-MVP Priorities

1. **Advanced Search**: Full-text search, saved filters
2. **Reporting**: Basic metrics and charts
3. **SLA Management**: Response time tracking
4. **Workflow Automation**: Triggers and actions
5. **API Documentation**: OpenAPI specification
6. **Performance Optimization**: Caching, query optimization
7. **Security Hardening**: Rate limiting, audit logs

## Quick validation

```bash
cp .env.development .env
make up
make test
```

Confirm:

- UI loads at `http://localhost`
- smtp4dev UI loads at `http://localhost:8025`

## Notes

- GoatFlow is HTMX-first / server-rendered.
- Local dev is container-first; prefer `make api-call` and other Makefile helpers over host tooling.