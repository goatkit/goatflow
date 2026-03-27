# Organisations & Multi-Tenancy (GoatKit PaaS Core)

## Overview

Core `gk_organisation` entity with user membership, data isolation, and per-org settings. Provides the organisational backbone that plugins (and GoatFlow itself) use to scope data, permissions, and configuration.

The existing `customer_company` table remains for OTRS-compatible customer management. `gk_organisation` is the new GoatKit-native entity that supports hierarchy, user membership (agents AND customers), and platform-wide data scoping.

## User Stories

- **Admin**: "I want to create organisations and assign users to them so data is isolated between clients"
- **Agent**: "I want to switch between organisations to manage tickets for different clients"
- **Plugin author**: "I want my plugin data automatically scoped to the user's organisation without writing scoping logic"
- **Customer**: "I only see my organisation's data — tickets, devices, invoices — nothing from other orgs"
- **Admin**: "I want different configuration per organisation — different SLA rules, different branding, different feature flags"

## Design Principles

1. **Additive, not replacing** — `customer_company` stays; `gk_organisation` is a new entity that can optionally link to it
2. **Agents AND customers** — both user types can belong to organisations (unlike `customer_company` which is customer-only)
3. **HostAPI enforcement** — the platform scopes HostAPI queries by org automatically; plugins don't filter manually
4. **Settings inheritance** — org settings override system defaults; user preferences override org settings
5. **Backward compatible** — existing single-org deployments work without creating any organisations

## Database Schema

```sql
-- Organisations
CREATE TABLE gk_organisation (
    id          BIGINT PRIMARY KEY AUTO_INCREMENT,
    name        VARCHAR(200) NOT NULL,
    slug        VARCHAR(100) NOT NULL,              -- URL-safe identifier
    parent_id   BIGINT DEFAULT NULL,                -- for org hierarchy (optional)
    status      VARCHAR(20) NOT NULL DEFAULT 'active',  -- active, suspended, archived

    -- Optional link to legacy customer_company
    customer_company_id VARCHAR(150) DEFAULT NULL,

    valid_id    SMALLINT NOT NULL DEFAULT 1,
    create_time DATETIME NOT NULL,
    create_by   INT NOT NULL,
    change_time DATETIME NOT NULL,
    change_by   INT NOT NULL,

    UNIQUE KEY uk_slug (slug),
    KEY idx_parent (parent_id),
    KEY idx_status (status),
    KEY idx_customer_company (customer_company_id),
    CONSTRAINT fk_org_parent FOREIGN KEY (parent_id) REFERENCES gk_organisation(id),
    CONSTRAINT fk_org_create_by FOREIGN KEY (create_by) REFERENCES users(id),
    CONSTRAINT fk_org_change_by FOREIGN KEY (change_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- User ↔ Organisation membership (agents and customers)
CREATE TABLE gk_user_organisation (
    id          BIGINT PRIMARY KEY AUTO_INCREMENT,
    org_id      BIGINT NOT NULL,
    user_id     INT DEFAULT NULL,                   -- agent user (from users table)
    customer_login VARCHAR(200) DEFAULT NULL,        -- customer user (from customer_user)
    role        VARCHAR(50) NOT NULL DEFAULT 'member', -- member, admin, owner
    is_default  BOOLEAN NOT NULL DEFAULT FALSE,      -- user's default org
    create_time DATETIME NOT NULL,
    create_by   INT NOT NULL,

    KEY idx_org (org_id),
    KEY idx_user (user_id),
    KEY idx_customer (customer_login),
    UNIQUE KEY uk_org_user (org_id, user_id),
    UNIQUE KEY uk_org_customer (org_id, customer_login),
    CONSTRAINT fk_uo_org FOREIGN KEY (org_id) REFERENCES gk_organisation(id) ON DELETE CASCADE,
    CONSTRAINT fk_uo_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_uo_create_by FOREIGN KEY (create_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

### Per-Org Settings (sysconfig_org)

Per-org settings extend the existing sysconfig system rather than introducing a separate config mechanism. A new `sysconfig_org` table stores org-level overrides using the same key names as `sysconfig_default`:

```sql
-- Per-org sysconfig overrides
CREATE TABLE sysconfig_org (
    id              BIGINT PRIMARY KEY AUTO_INCREMENT,
    org_id          BIGINT NOT NULL,
    name            VARCHAR(250) NOT NULL,           -- same key as sysconfig_default
    effective_value LONGBLOB NOT NULL,
    is_valid        SMALLINT NOT NULL DEFAULT 1,
    create_time     DATETIME NOT NULL,
    create_by       INT NOT NULL,
    change_time     DATETIME NOT NULL,
    change_by       INT NOT NULL,

    UNIQUE KEY uk_org_name (org_id, name),
    CONSTRAINT fk_sco_org FOREIGN KEY (org_id) REFERENCES gk_organisation(id) ON DELETE CASCADE,
    CONSTRAINT fk_sco_create_by FOREIGN KEY (create_by) REFERENCES users(id),
    CONSTRAINT fk_sco_change_by FOREIGN KEY (change_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

### Config Resolution Cascade

`ConfigGet(key)` resolves through four tiers, highest priority first:

```
1. User preference    (user_preferences table)      ← per-user
2. Org override       (sysconfig_org table)          ← per-org
3. System override    (sysconfig_modified table)     ← system-wide admin change
4. System default     (sysconfig_default table)      ← shipped default
```

Example:
```
ConfigGet("max_ticket_attachments")
  user_preferences:    (not set)
  sysconfig_org:       org 42 → "10"     ← returns this
  sysconfig_modified:  "5"
  sysconfig_default:   "3"
  Result: "10"
```

Plugins call `ConfigGet` as normal — the platform resolves the org scope from the session context automatically. No new API needed. The existing sysconfig admin UI extends to show per-org overrides.

### What Can Be Overridden Per Org

Any existing sysconfig key can be overridden per org. Common examples:

| Key | Description | Default |
|-----|-------------|---------|
| `Branding::AppName` | Application name in header | GoatFlow |
| `Branding::Logo` | Logo URL | (default logo) |
| `Branding::PrimaryColor` | Brand colour | (theme default) |
| `Security::TwoFactorRequired` | Enforce 2FA | false |
| `Customer::SelfRegistration` | Allow customer sign-up | false |
| `Ticket::MaxAttachments` | Max file attachments | 5 |
| `SLA::DefaultResponseHours` | Default first response SLA | 8 |
| `SLA::DefaultResolutionHours` | Default resolution SLA | 48 |

## User Membership

### Membership Model

Users (both agents and customers) belong to organisations via `gk_user_organisation`:

- **One user, multiple orgs** — an agent managing several client organisations
- **One default org** — `is_default=true` determines which org is active after login
- **Org roles** — `member` (standard access), `admin` (manage org settings/users), `owner` (full control)

### Org Context

After login, the user's active org is stored in the session. All subsequent requests are scoped to that org. The org context is set via:

1. `is_default=true` membership → auto-selected at login
2. Org switcher UI → user changes active org mid-session
3. Subdomain → `acme.goatflow.example.com` resolves to org via slug

### Backward Compatibility

If no organisations exist in the database, the system operates in "single-org" mode:
- All HostAPI queries return unscoped data (no `org_id` filter)
- Org switcher is hidden
- Per-org settings fall through to system defaults
- Everything works exactly as it does today

## HostAPI Enforcement

All HostAPI database methods automatically filter by the caller's active org:

```go
// Before (no org scoping):
host.DBQuery(ctx, "SELECT * FROM ticket WHERE queue_id = ?", queueID)

// After (platform adds org filter transparently):
// The sandbox injects: AND org_id = ? (from session context)
// Plugin code doesn't change.
```

### Implementation

The `SandboxedHostAPI` (or a new `OrgScopedHostAPI` wrapper) intercepts `DBQuery`/`DBExec` calls and:

1. Parses the SQL to find the primary table
2. If the table has an `org_id` column, appends `AND org_id = ?` to WHERE clause
3. For INSERT, sets `org_id` to the active org automatically
4. Tables without `org_id` pass through unmodified

This is opt-in per table — only tables that have been migrated to include `org_id` get scoped.

### Org-Aware Tables

Tables that gain an `org_id` column:

| Table | Scoping Behaviour |
|-------|-------------------|
| `ticket` | Tickets visible only within the org |
| `queue` | Queues can be org-specific or global |
| `customer_user` | Customers scoped to their org |
| `gk_custom_field_value` | Custom field values scoped per org |
| `gk_plugin_ui` | Plugin UIs can be org-specific |

### HostAPI Addition

```go
type HostAPI interface {
    // ... existing methods ...

    // OrgID returns the active organisation ID for the current request.
    // Returns 0 if no organisation context (single-org mode).
    OrgID(ctx context.Context) int64
}
```

## Org Switching UI

For users in multiple organisations, a switcher appears in the top navigation bar:

```
┌──────────────────────────────────────────────────┐
│ [Logo] Dashboard  Tickets  [Acme Corp ▼]  [User] │
│                            ├──────────────┤      │
│                            │ ✓ Acme Corp  │      │
│                            │   Beta Ltd   │      │
│                            │   Gamma Inc  │      │
│                            └──────────────┘      │
└──────────────────────────────────────────────────┘
```

Switching orgs:
1. POST `/api/v1/session/org` with `{"org_id": 42}`
2. Server updates the session's active org
3. Page reloads with new org context
4. All data now scoped to the selected org

## Per-Org Settings

Per-org settings use the `sysconfig_org` table (defined above in the schema section). Sysconfig is GoatFlow's database-stored configuration system (`sysconfig_default` + `sysconfig_modified` tables) — distinct from KernelConfig which is YAML/env-based and loaded at boot. Only sysconfig keys (runtime-changeable, DB-stored) can be overridden per org.

The resolution cascade has four tiers:

```
User Preference  →  Org Override  →  System Override  →  System Default
   (highest)        (sysconfig_org)  (sysconfig_modified) (sysconfig_default)
```

Example: system default language is English, org "Acme" sets German via `sysconfig_org`, user "Alice" sets French in her preferences — Alice sees French. But user "Bob" (no preference) in the same org sees German.

The existing `sysconfig.Manager.Get(name)` method is extended to accept an org context. Plugins using `ConfigGet` automatically get org-scoped values without code changes.

### Admin UI

Org admins access org settings via **Admin → Organisations → [Org Name] → Settings**. The UI shows all overridable sysconfig keys with the current effective value and whether it's inherited or overridden:

```
┌──────────────────────────────────────────────────┐
│ Acme Corp — Settings                             │
├──────────────────────────────────────────────────┤
│ Branding                                         │
│   App Name:     [Acme Support_________] override │
│   Logo:         [Upload]                inherited │
│   Primary Color:[#2563eb___]            override │
│                                                  │
│ Features                                         │
│   Self-Registration:    [x] Enabled     override │
│   2FA Required:         [ ] Disabled   inherited │
│   Max Attachments:      [10___]         override │
│                                                  │
│ SLA Defaults                                     │
│   Response Time (hrs):  [4____]         override │
│   Resolution Time (hrs):[24___]        inherited │
│                                                  │
│              [Reset to Defaults] [Save Settings] │
└──────────────────────────────────────────────────┘
```

"Reset to Defaults" removes the org override, falling through to the system value.

## Security Considerations

1. **Org isolation is server-side** — scoping happens in the HostAPI, not in plugin code; plugins can't bypass it
2. **Cross-org access** — users can only access orgs they're members of; the session stores the active org
3. **Org admin != system admin** — org admins manage their org's settings and users, not the platform
4. **Slug uniqueness** — slugs are unique and URL-safe for subdomain routing
5. **Cascade delete** — deleting an org removes memberships but NOT the data (tickets, custom fields); data requires separate cleanup

## Implementation Order

1. [ ] Design spec (this document)
2. [ ] Database migration — `gk_organisation` + `gk_user_organisation` + `sysconfig_org` tables
3. [ ] Go models — `Organisation`, `UserOrganisation`, org constants
4. [ ] Repository layer — CRUD for organisations + memberships
5. [ ] Org context middleware — resolve active org from session/subdomain
6. [ ] HostAPI `OrgID()` method
7. [ ] Org switching API endpoint — POST `/api/v1/session/org`
8. [ ] Org switcher UI component — dropdown in navigation
9. [ ] Per-org settings — extend sysconfig.Manager with org-scoped resolution via `sysconfig_org`
10. [ ] Admin UI — organisation CRUD, membership management, settings
11. [ ] HostAPI query scoping — auto-inject org_id filters (opt-in per table)
12. [ ] i18n — translations for all 15 languages
13. [ ] Tests — unit, integration, E2E

---

*Design: 2026-03-26*
