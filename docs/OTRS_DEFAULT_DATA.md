# OTRS Default Data Reference

This document lists all default data that is included in GoTRS migrations to match OTRS standard installation.

## Migration Files Overview

1. **000002_otrs_initial_data.up.sql** - Core ticket system data
2. **000020_lookup_defaults.up.sql** - Salutations, signatures, system addresses
3. **000021_otrs_core_defaults.up.sql** - Communication channels, auto-responses, services
4. **000022_otrs_config_defaults.up.sql** - Templates, calendars, system settings
5. **000023_otrs_final_defaults.up.sql** - Link types, notifications, admin user

## Default Data by Category

### Ticket System (000002)

**Ticket State Types (7)**
- new, open, closed, pending reminder, pending auto, removed, merged

**Ticket States (9)**
- new, open, closed successful, closed unsuccessful, pending reminder
- pending auto close+, pending auto close-, removed, merged

**Ticket Priorities (5)**
- 1 very low, 2 low, 3 normal, 4 high, 5 very high

**Ticket Types (5)**
- Unclassified, Incident, Service Request, Problem, Change Request

**Ticket Lock Types (3)**
- unlock, lock, tmp_lock

**Article Sender Types (3)**
- agent, system, customer

**Ticket History Types (48)**
- Complete set of history tracking types for all ticket operations

**Default Queues (4)**
- Postmaster (default for incoming), Raw (unprocessed), Junk (spam), Misc

### Communication & Responses (000020, 000021)

**Salutations (4)**
- system standard salutation (en)
- Formal (Mr/Ms)
- Informal
- No salutation

**Signatures (4)**
- system standard signature (en)
- Support Team
- Technical Support
- No signature

**System Email Addresses (6)**
- otrs@localhost, support@example.com, noreply@example.com
- helpdesk@example.com, sales@example.com, billing@example.com

**Communication Channels (5)**
- Email, Phone, Internal, Chat, SMS

**Auto Response Types (5)**
- auto reply, auto reject, auto follow up, auto reply/new ticket, auto remove

**Auto Responses (2)**
- Default reply for new tickets
- Default reject for closed ticket follow-ups

### Services & SLAs (000021)

**Services (5)**
- IT Support, IT Support::Hardware, IT Support::Software
- IT Support::Network, (plus any existing)

**SLAs (10)**
- Standard, Premium, Gold, Critical (plus any existing)

### User Management (000021, 000023)

**Roles (5)**
- Agent, Admin, Customer (plus any existing)

**Groups (11+)**
- admin, stats, users (plus queue-specific groups)

**Default Admin User**
- root@localhost (created disabled; set a password via `make reset-password` or `make synthesize`)

### Templates & Configuration (000022)

**Standard Templates (3)**
- empty answer, Thank you for your email, Your request has been closed

**Web Service Config (1)**
- Example Web Service (empty config)

**Calendar (1)**
- Default calendar with blue color

**Generic Agent Jobs (1)**
- close-tickets-after-7-days (example auto-close job)

**System Data Settings (6+)**
- SystemID, FQDN, Organization, AdminEmail
- NotificationSenderName, NotificationSenderEmail

### Relationships & Events (000023)

**Link Types (7)**
- Normal, ParentChild, DependsOn, RelevantTo
- AlternativeTo, ConnectedTo, DuplicateOf

**Notification Events (16)**
- Complete set of ticket and article notification triggers
- Includes state changes, owner updates, escalations, etc.

## Tables Without Defaults

These tables are populated dynamically during system use:
- article_flag (created when articles are viewed/marked)
- customer_company (no default customers)
- customer_user (no default customer users)
- ticket (no default tickets)
- article (no default articles)

## OTRS Compatibility Notes

- All table structures match OTRS v6 schema exactly
- Column names, types, and constraints are identical to OTRS
- Default data values match OTRS standard installation
- Some tables use different patterns (e.g., pm_entity_sync has no create_by/change_by)
- article_flag uses composite primary key, not an ID sequence

## Usage

To apply all defaults on a fresh installation:

```bash
# Apply migrations in order
for i in {1..23}; do
    migration=$(printf "%06d" $i)
    if [ -f migrations/${migration}_*.up.sql ]; then
        cat migrations/${migration}_*.up.sql | psql -h localhost -U gotrs_user -d gotrs
    fi
done
```

To verify default data:

```sql
-- Check counts
SELECT 'users' as entity, COUNT(*) FROM users
UNION ALL SELECT 'groups', COUNT(*) FROM groups
UNION ALL SELECT 'queues', COUNT(*) FROM queue
UNION ALL SELECT 'services', COUNT(*) FROM service
UNION ALL SELECT 'slas', COUNT(*) FROM sla
UNION ALL SELECT 'communication_channels', COUNT(*) FROM communication_channel
ORDER BY 1;
```