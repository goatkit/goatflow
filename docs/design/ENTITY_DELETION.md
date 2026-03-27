# Entity Deletion (GoatKit PaaS Core)

## Overview

Two deletion patterns for all GoatKit entities: **soft delete** (move to recycle bin, anonymise PII, preserve business records) and **hard delete** (purge — physical removal of entity + all linked data). The recycle bin is the standard deletion pipeline.

Today, GoatFlow has inconsistent deletion: tickets use `archive_flag`, users/queues use `valid_id=2`, articles use hard DELETE, and there's no recycle bin, no GDPR anonymisation, and no plugin cascade. This feature unifies deletion across all entities.

## User Stories

- **Admin**: "I want to delete a customer and have all their PII anonymised while keeping ticket history intact"
- **Admin**: "I want to restore a ticket that was accidentally deleted, from the recycle bin"
- **Admin**: "I want to purge all data for a decommissioned project — tickets, custom fields, everything"
- **Plugin author**: "When a contact is deleted, I want my plugin's device records for that contact cleaned up too"
- **Compliance officer**: "I need a log proving that a deletion happened, without logging what was deleted"

## Design Principles

1. **Soft delete is the default** — all deletes go to recycle bin first; hard delete requires explicit permission
2. **Anonymise, don't erase** — soft delete replaces PII with `[DELETED]` so business records (ticket counts, SLA stats) stay accurate
3. **Plugin cascade** — plugins declare `CascadeSpec` to handle their own data when entities are deleted
4. **Tombstone logging** — the `gk_deletion_log` records THAT something was deleted (entity type, ID, when, by whom) but NOT what the data contained
5. **Backward compatible** — existing `valid_id` and `archive_flag` patterns continue to work; the deletion service wraps them

## Recycle Bin

### Flow

```
Delete request
  → Permission check (RBAC)
  → Soft delete: set deleted_at, anonymise PII if configured
  → Plugin cascade: notify plugins via CascadeSpec handlers
  → Tombstone log entry
  → Entity hidden from normal queries

Restore request
  → Permission check (admin only)
  → Clear deleted_at
  → De-anonymise NOT possible (PII is gone) — restore only brings back the record

Hard delete (purge)
  → Requires entity.hard_delete RBAC permission
  → Physical DELETE from database
  → Plugin cascade: hard delete plugin data
  → Tombstone log entry (permanent)
```

### Database Changes

Rather than adding a `deleted_at` column to every existing table (breaking change), the recycle bin is a **separate tracking table**:

```sql
-- Recycle bin: tracks soft-deleted entities
CREATE TABLE gk_recycle_bin (
    id          BIGINT PRIMARY KEY AUTO_INCREMENT,
    entity_type VARCHAR(50) NOT NULL,       -- ticket, contact, agent, queue, organisation, etc.
    entity_id   BIGINT NOT NULL,            -- PK of the deleted entity
    entity_name VARCHAR(255) DEFAULT NULL,  -- display name at time of deletion (for bin UI)
    deleted_by  INT NOT NULL,
    deleted_at  DATETIME NOT NULL,
    expires_at  DATETIME DEFAULT NULL,      -- auto-purge date (NULL = never)
    org_id      BIGINT DEFAULT NULL,

    KEY idx_entity (entity_type, entity_id),
    KEY idx_deleted_at (deleted_at),
    KEY idx_expires (expires_at),
    CONSTRAINT fk_rb_deleted_by FOREIGN KEY (deleted_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Tombstone log: immutable record that deletion happened
CREATE TABLE gk_deletion_log (
    id          BIGINT PRIMARY KEY AUTO_INCREMENT,
    entity_type VARCHAR(50) NOT NULL,
    entity_id   BIGINT NOT NULL,
    action      VARCHAR(20) NOT NULL,       -- soft_delete, restore, hard_delete
    deleted_by  INT NOT NULL,
    deleted_at  DATETIME NOT NULL,
    org_id      BIGINT DEFAULT NULL,
    reason      VARCHAR(500) DEFAULT NULL,

    KEY idx_entity (entity_type, entity_id),
    KEY idx_deleted_at (deleted_at),
    CONSTRAINT fk_dl_deleted_by FOREIGN KEY (deleted_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

### How Soft Delete Works Per Entity Type

The deletion service uses the existing soft-delete mechanism for each entity type:

| Entity | Soft Delete Method | Anonymisation |
|--------|-------------------|---------------|
| ticket | `archive_flag=1`, `ticket_state_id=2` | title → `[DELETED]`, customer_id → `[DELETED]` |
| contact (customer_user) | `valid_id=2` | first_name, last_name, email, phone → `[DELETED]` |
| agent (users) | `valid_id=2` | first_name, last_name → `[DELETED]` (login preserved for audit) |
| queue | `valid_id=2` | No PII to anonymise |
| organisation | `status='archived'` | No PII |
| customer_company | `valid_id=2` | name, street, city → `[DELETED]` |

Each entity type registers an `EntityDeleteHandler` that knows how to soft-delete and anonymise that type.

## PII Anonymisation

Anonymisation is **configurable per entity type and field**. When a soft delete triggers anonymisation:

1. PII fields are overwritten with `[DELETED]` in the database
2. The original values are NOT stored anywhere (irreversible)
3. Non-PII fields (IDs, timestamps, queue assignments) are preserved
4. Business metrics remain accurate (ticket counts, SLA calculations)

Anonymisation can be disabled per entity type in sysconfig for deployments that don't need GDPR compliance.

## Plugin Cascade

Plugins declare cascade handlers in `GKRegistration`:

```go
type GKRegistration struct {
    // ... existing fields ...

    // Cascade handlers for entity deletion.
    // Called when platform entities are soft/hard deleted.
    Cascades []CascadeSpec `json:"cascades,omitempty"`
}

type CascadeSpec struct {
    EntityType string `json:"entity_type"` // ticket, contact, agent, organisation, etc.
    OnSoftDelete string `json:"on_soft_delete,omitempty"` // plugin function for soft delete
    OnHardDelete string `json:"on_hard_delete,omitempty"` // plugin function for hard delete
}
```

Example: an inventory plugin that stores device assignments per contact:
```go
Cascades: []plugin.CascadeSpec{
    {
        EntityType:   "contact",
        OnSoftDelete: "cascade_contact_soft_delete",  // anonymise device owner
        OnHardDelete: "cascade_contact_hard_delete",  // delete device records
    },
},
```

The platform calls these handlers after performing its own deletion. If a cascade handler fails, the deletion still proceeds (logged as warning).

## HostAPI Methods

```go
type HostAPI interface {
    // ... existing methods ...

    // EntitySoftDelete soft-deletes an entity (moves to recycle bin, anonymises PII).
    EntitySoftDelete(ctx context.Context, entityType string, entityID int64, reason string) error

    // EntityRestore restores a soft-deleted entity from the recycle bin.
    EntityRestore(ctx context.Context, entityType string, entityID int64) error

    // EntityHardDelete permanently removes an entity and all linked data.
    EntityHardDelete(ctx context.Context, entityType string, entityID int64, reason string) error

    // RecycleBinList lists soft-deleted entities in the recycle bin.
    RecycleBinList(ctx context.Context, entityType string) ([]RecycleBinEntry, error)
}

type RecycleBinEntry struct {
    EntityType string    `json:"entity_type"`
    EntityID   int64     `json:"entity_id"`
    EntityName string    `json:"entity_name"`
    DeletedBy  int       `json:"deleted_by"`
    DeletedAt  time.Time `json:"deleted_at"`
    ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}
```

## RBAC

| Permission | Required For |
|-----------|-------------|
| (existing role-based) | Soft delete (any agent with write access) |
| `entity.hard_delete` | Hard delete / purge |
| `admin` | Restore from recycle bin |

## Auto-Purge

A scheduled job runs daily and permanently deletes entries from the recycle bin where `expires_at < NOW()`. The retention period is configurable per entity type via sysconfig:

| Sysconfig Key | Default | Description |
|---------------|---------|-------------|
| `Deletion::RetentionDays::ticket` | 90 | Days before auto-purge |
| `Deletion::RetentionDays::contact` | 30 | Days before auto-purge |
| `Deletion::RetentionDays::default` | 60 | Default for unspecified types |

## Implementation Order

1. [ ] Design spec (this document)
2. [ ] Database migration — `gk_recycle_bin` + `gk_deletion_log` tables
3. [ ] Go types — `RecycleBinEntry`, `DeletionLog`, `CascadeSpec`, entity delete handlers
4. [ ] Deletion service — soft delete, restore, hard delete with entity-type dispatch
5. [ ] PII anonymisation — per-entity-type field anonymiser
6. [ ] `CascadeSpec` in `GKRegistration` — plugin cascade declaration
7. [ ] Plugin manager cascade dispatch — call plugin handlers on entity deletion
8. [ ] Tombstone logging — immutable `gk_deletion_log` entries
9. [ ] HostAPI methods — `EntitySoftDelete`, `EntityRestore`, `EntityHardDelete`, `RecycleBinList`
10. [ ] Sandbox enforcement + WASM/gRPC wire format
11. [ ] Auto-purge scheduled job
12. [ ] RBAC — `entity.hard_delete` permission
13. [ ] Admin recycle bin UI
14. [ ] All mock HostAPIs updated
15. [ ] i18n — translations for all 15 languages
16. [ ] Tests — unit, integration, cascade, anonymisation

---

*Design: 2026-03-27*
