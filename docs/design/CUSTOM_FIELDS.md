# Custom Fields (GoatKit PaaS Core)

## Overview

Universal custom fields on every core entity. Plugins declare fields at registration; GoatKit handles storage, validation, UI rendering, and querying. Eliminates the need for plugin extension tables.

Replaces the legacy OTRS `dynamic_field` system (ticket/article only) with a unified EAV store that covers all GoatKit entities.

## User Stories

- **Plugin author**: "I want to add a `device_imei` field to contacts without creating my own table"
- **Admin**: "I want to add a `department` dropdown to agents without writing code"
- **Plugin author**: "I want to store lat/lng coordinates on an organisation and query nearby ones"
- **Agent**: "I want to see all custom fields on a contact's detail page, grouped logically"
- **Admin**: "I want to migrate our existing ticket dynamic fields into the new unified system"

## Design Principles

1. **One table pair for everything** — no per-entity extension tables
2. **Plugins declare, platform owns** — plugins declare fields via `GKRegistration`; the platform creates, stores, indexes, renders, and enforces validation
3. **Admin parity** — admins can create the same field types as plugins, via the admin UI
4. **Backwards compatible** — legacy `dynamic_field` tables remain; migration is opt-in
5. **Query-friendly EAV** — denormalised typed columns for indexed queries without full-table scans

## Supported Entities

| Entity Key | Table | Description |
|------------|-------|-------------|
| `ticket` | `ticket` | Support tickets |
| `article` | `article` | Ticket articles |
| `contact` | `customer_user` | End-user contacts |
| `agent` | `users` | Agent/admin users |
| `group` | `groups` | Permission groups |
| `customer_group` | `customer_company` | Customer organisations (legacy) |
| `queue` | `queue` | Ticket queues |
| `organisation` | `gk_organisation` | GoatKit organisations (0.8.0) |

New entities can be added by registering an entity key → table mapping in the custom fields registry.

## Field Types

### Standard Types

| Type | Storage Column | Input Control | Indexable |
|------|---------------|---------------|-----------|
| `text` | `val_text` | `<input type="text">` | Yes |
| `textarea` | `val_text` | `<textarea>` | No |
| `integer` | `val_int` | `<input type="number">` | Yes |
| `decimal` | `val_decimal` | `<input type="number" step="0.01">` | Yes |
| `boolean` | `val_int` (0/1) | `<input type="checkbox">` | Yes |
| `date` | `val_date` | `<input type="date">` | Yes |
| `datetime` | `val_datetime` | `<input type="datetime-local">` | Yes |
| `select` | `val_text` | `<select>` | Yes |
| `multi_select` | `val_json` | multi-select checkboxes | No |
| `url` | `val_text` | `<input type="url">` | No |
| `email` | `val_text` | `<input type="email">` | Yes |
| `phone` | `val_text` | `<input type="tel">` | Yes |

### GIS Types

| Type | Storage Column | Input Control | Indexable |
|------|---------------|---------------|-----------|
| `point` | `val_json` `{"lat":..,"lng":..}` | Map pin picker | Yes (lat/lng extracted to `val_decimal`/`val_decimal2`) |
| `polygon` | `val_json` (GeoJSON) | Map polygon draw | No |
| `address` | `val_json` (structured) | Address form + auto-geocode | Yes (postcode in `val_text`) |

**Address JSON structure:**
```json
{
  "line1": "123 High Street",
  "line2": "Suite 4",
  "city": "London",
  "region": "Greater London",
  "postcode": "EC1A 1BB",
  "country": "GB",
  "lat": 51.5194,
  "lng": -0.0983
}
```

## Database Schema

```sql
-- Field definitions
CREATE TABLE gk_custom_field_def (
    id          BIGINT PRIMARY KEY AUTO_INCREMENT,

    -- Identity
    name        VARCHAR(200) NOT NULL,          -- machine name, e.g. "device_imei"
    label       VARCHAR(200) NOT NULL,          -- display label, e.g. "Device IMEI"
    entity_type VARCHAR(50) NOT NULL,           -- "contact", "agent", "queue", etc.
    field_type  VARCHAR(50) NOT NULL,           -- "text", "select", "point", etc.

    -- Ownership
    owner_type  ENUM('plugin','admin','legacy') NOT NULL DEFAULT 'admin',
    owner_name  VARCHAR(100) DEFAULT NULL,      -- plugin name if owner_type='plugin'
    migrated_from BIGINT DEFAULT NULL,          -- legacy dynamic_field.id (if owner_type='legacy')

    -- Display
    section     VARCHAR(100) DEFAULT 'custom',  -- UI grouping, e.g. "location", "billing"
    field_order INT NOT NULL DEFAULT 0,         -- sort within section
    description VARCHAR(500) DEFAULT NULL,      -- help text shown below input
    placeholder VARCHAR(200) DEFAULT NULL,      -- input placeholder text

    -- Validation & config
    required    BOOLEAN NOT NULL DEFAULT FALSE,
    config      JSON,                           -- type-specific config (see below)

    -- Lifecycle
    valid_id    SMALLINT NOT NULL DEFAULT 1,    -- 1=valid, 2=invalid
    create_time DATETIME NOT NULL,
    create_by   INT NOT NULL,
    change_time DATETIME NOT NULL,
    change_by   INT NOT NULL,

    UNIQUE KEY uk_entity_name (entity_type, name),
    INDEX idx_entity_type (entity_type, valid_id),
    INDEX idx_owner (owner_type, owner_name),
    INDEX idx_migrated (migrated_from),
    CONSTRAINT fk_cfdef_create_by FOREIGN KEY (create_by) REFERENCES users(id),
    CONSTRAINT fk_cfdef_change_by FOREIGN KEY (change_by) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Field values (EAV with denormalised typed columns)
CREATE TABLE gk_custom_field_value (
    id            BIGINT PRIMARY KEY AUTO_INCREMENT,
    field_id      BIGINT NOT NULL,                -- FK to gk_custom_field_def
    object_id     BIGINT NOT NULL,                -- PK of the entity row

    -- Denormalised typed columns (only one populated per row)
    val_text      VARCHAR(4000) DEFAULT NULL,     -- text, select, email, phone, url
    val_int       BIGINT DEFAULT NULL,            -- integer, boolean (0/1)
    val_decimal   DECIMAL(18,8) DEFAULT NULL,     -- decimal, point lat
    val_decimal2  DECIMAL(18,8) DEFAULT NULL,     -- point lng
    val_date      DATE DEFAULT NULL,              -- date
    val_datetime  DATETIME DEFAULT NULL,          -- datetime
    val_json      JSON DEFAULT NULL,              -- multi_select, point, polygon, address

    UNIQUE KEY uk_field_object (field_id, object_id),
    INDEX idx_object (object_id),
    INDEX idx_text (field_id, val_text(191)),
    INDEX idx_int (field_id, val_int),
    INDEX idx_decimal (field_id, val_decimal),
    INDEX idx_date (field_id, val_date),
    INDEX idx_datetime (field_id, val_datetime),
    CONSTRAINT fk_cfval_field FOREIGN KEY (field_id) REFERENCES gk_custom_field_def(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

### Column Mapping

Each field type maps to exactly one storage column (or column pair for `point`):

```
text, select, url, email, phone  → val_text
textarea                         → val_text (up to 4000 chars)
integer, boolean                 → val_int
decimal                          → val_decimal
date                             → val_date
datetime                         → val_datetime
multi_select, polygon, address   → val_json
point                            → val_json + val_decimal (lat) + val_decimal2 (lng)
```

The denormalised `val_decimal`/`val_decimal2` for `point` fields enables bounding-box queries without JSON extraction:

```sql
SELECT object_id FROM gk_custom_field_value
WHERE field_id = ?
  AND val_decimal BETWEEN ? AND ?    -- lat range
  AND val_decimal2 BETWEEN ? AND ?;  -- lng range
```

### Type-Specific Config (JSON)

The `config` column stores type-specific settings:

```json
// text
{"max_length": 200, "regex": "^[A-Z0-9]{15}$", "regex_error": "Must be 15 alphanumeric chars"}

// select
{"options": [{"value": "low", "label": "Low"}, {"value": "high", "label": "High"}], "allow_empty": true}

// multi_select
{"options": [...], "min_selected": 1, "max_selected": 5}

// integer / decimal
{"min": 0, "max": 9999, "step": 1}

// date / datetime
{"min_date": "2020-01-01", "max_date": "2030-12-31", "future_only": false}

// point
{"default_zoom": 13, "default_center": {"lat": 51.5074, "lng": -0.1278}}

// address
{"countries": ["GB", "US", "DE"], "geocode_provider": "nominatim"}

// url
{"allowed_schemes": ["https"]}
```

## Plugin Registration

Plugins declare custom fields in `GKRegistration`:

```go
type GKRegistration struct {
    // ... existing fields ...

    // Custom fields the plugin needs on GoatKit entities
    CustomFields []CustomFieldSpec `json:"custom_fields,omitempty"`
}

type CustomFieldSpec struct {
    Name        string          `json:"name"`         // machine name (auto-prefixed with plugin name)
    Label       string          `json:"label"`        // display label (can be i18n key)
    EntityType  string          `json:"entity_type"`  // "contact", "agent", "organisation", etc.
    FieldType   string          `json:"field_type"`   // "text", "select", "point", etc.
    Section     string          `json:"section,omitempty"`     // UI section grouping
    Order       int             `json:"order,omitempty"`       // sort within section
    Required    bool            `json:"required,omitempty"`
    Description string          `json:"description,omitempty"` // help text
    Placeholder string          `json:"placeholder,omitempty"`
    Config      json.RawMessage `json:"config,omitempty"`      // type-specific config
}
```

### Name Prefixing

Plugin-owned field names are auto-prefixed with the plugin name to prevent collisions:

```
Plugin "inventory" declares field "sku_code"
→ stored as name="inventory_sku_code" in gk_custom_field_def
→ plugin references it as "sku_code" (prefix stripped by HostAPI)
```

Admin-created fields have no prefix.

### Registration Flow

```
Plugin loads
  → GKRegister() returns CustomFields[]
  → Platform iterates CustomFields:
      → For each field:
          1. Prefix name with plugin name
          2. Check if field already exists (by entity_type + name)
          3. If missing: INSERT into gk_custom_field_def (owner_type='plugin', owner_name=plugin)
          4. If exists: UPDATE label, config, etc. (only if owner matches)
          5. If plugin removed a field: mark valid_id=2 (soft disable, preserve data)
```

### Example Registration

```go
func (p *InventoryPlugin) GKRegister() plugin.GKRegistration {
    return plugin.GKRegistration{
        Name:    "inventory",
        Version: "1.0.0",
        // ...
        CustomFields: []plugin.CustomFieldSpec{
            {
                Name:       "sku_code",
                Label:      "SKU Code",
                EntityType: "contact",
                FieldType:  "text",
                Section:    "inventory",
                Required:   true,
                Config:     json.RawMessage(`{"max_length":20,"regex":"^[A-Z0-9-]+$"}`),
            },
            {
                Name:       "warehouse_location",
                Label:      "Warehouse Location",
                EntityType: "organisation",
                FieldType:  "point",
                Section:    "location",
                Config:     json.RawMessage(`{"default_zoom":10}`),
            },
        },
    }
}
```

## HostAPI Methods

Three new methods on the `HostAPI` interface:

```go
type HostAPI interface {
    // ... existing methods ...

    // CustomFieldsGet retrieves custom field values for an entity.
    // Returns map of field_name → value (plugin prefix stripped).
    // fields=nil returns all fields; fields=["x","y"] returns only those.
    CustomFieldsGet(ctx context.Context, entityType string, objectID int64, fields []string) (map[string]any, error)

    // CustomFieldsSet stores custom field values for an entity.
    // values is field_name → value (plugin prefix stripped).
    // Validates types and constraints before storing.
    CustomFieldsSet(ctx context.Context, entityType string, objectID int64, values map[string]any) error

    // CustomFieldsQuery finds entities by custom field values.
    // Returns slice of matching object IDs.
    CustomFieldsQuery(ctx context.Context, entityType string, filters []CustomFieldFilter) ([]int64, error)
}

type CustomFieldFilter struct {
    Field    string `json:"field"`    // field name (plugin prefix stripped)
    Operator string `json:"operator"` // "eq", "neq", "gt", "lt", "gte", "lte", "like", "in", "between", "near"
    Value    any    `json:"value"`    // comparison value
    Value2   any    `json:"value2"`   // second value for "between" and "near" (radius in km)
}
```

### Sandbox Enforcement

The sandboxed HostAPI enforces:
- Plugins can only get/set fields they own (prefix match) or admin-created fields marked as plugin-accessible
- Field validation runs server-side before storage (type checks, regex, min/max, required)
- Rate limits apply (counted against `MaxDBQueriesPerMin`)
- `CustomFieldsQuery` with `near` operator is limited to bounding-box pre-filter + Haversine on ≤1000 candidates

### WASM / gRPC Wire Format

Custom field methods are exposed as JSON-RPC calls through the existing `Call` mechanism:

```json
// CustomFieldsGet
{"fn": "host.custom_fields_get", "args": {"entity_type": "contact", "object_id": 42, "fields": ["device_imei"]}}
→ {"device_imei": "123456789012345"}

// CustomFieldsSet
{"fn": "host.custom_fields_set", "args": {"entity_type": "contact", "object_id": 42, "values": {"device_imei": "123456789012345"}}}
→ {}

// CustomFieldsQuery
{"fn": "host.custom_fields_query", "args": {"entity_type": "organisation", "filters": [{"field": "location", "operator": "near", "value": {"lat": 51.5, "lng": -0.1}, "value2": 25}]}}
→ [10, 23, 47]
```

## Auto UI Rendering

Custom fields render automatically on entity detail/edit pages. No plugin template code required.

### Rendering Rules

1. Fields grouped by `section` — each section gets a collapsible `<fieldset>` with a legend
2. Within a section, fields ordered by `field_order`
3. Plugin sections appear after core fields, ordered alphabetically by section name
4. Admin sections appear after plugin sections
5. Invalid fields (`valid_id=2`) are hidden from forms but values display read-only if populated

### Template Integration

A new reusable partial replaces the existing `dynamic_fields.pongo2` pattern:

```
{% include "partials/custom_fields.pongo2" with entity_type="contact" object_id=contact.ID mode="edit" %}
```

**Modes:**
- `edit` — full input controls with validation
- `view` — read-only display with formatted values
- `inline` — compact single-line display (for table rows)

### GIS Field Rendering

- **point**: Leaflet.js map with draggable marker + lat/lng text inputs as fallback
- **polygon**: Leaflet.js map with draw controls + raw GeoJSON textarea as fallback
- **address**: Structured form fields (line1, line2, city, region, postcode, country dropdown) + "Lookup" button that geocodes via Nominatim and places a map pin

Map assets are loaded on-demand (not bundled into every page).

## Admin UI

### Admin → Custom Fields

```
┌─────────────────────────────────────────────────────────────┐
│ Custom Fields                                [+ New Field]  │
├─────────────────────────────────────────────────────────────┤
│ Entity: [All ▼]  Owner: [All ▼]  Status: [Active ▼]        │
├──────────────┬──────────┬──────────┬────────┬───────┬───────┤
│ Name         │ Entity   │ Type     │ Owner  │ Sect. │       │
├──────────────┼──────────┼──────────┼────────┼───────┼───────┤
│ department   │ agent    │ select   │ admin  │ org   │ [Edit]│
│ inventory_sku│ contact  │ text     │ invent.│ inv   │ [View]│
│ inventory_loc│ org      │ point    │ invent.│ loc   │ [View]│
└──────────────┴──────────┴──────────┴────────┴───────┴───────┘
```

- Plugin-owned fields are **read-only** in admin (view config, cannot edit)
- Admin-owned fields have full CRUD
- Deleting an admin field soft-deletes (`valid_id=2`), preserving data
- "Purge Data" action hard-deletes the field definition + all values (requires `admin.custom_fields.purge` permission)

### Create/Edit Field Form

```
┌─────────────────────────────────────────────────────────────┐
│ New Custom Field                                        [X] │
├─────────────────────────────────────────────────────────────┤
│ Entity:      [Contact ▼]                                    │
│ Name:        [emergency_contact________]                    │
│ Label:       [Emergency Contact________]                    │
│ Type:        [Phone ▼]                                      │
│ Section:     [personal_________________]                    │
│ Order:       [10___]                                        │
│ Required:    [ ]                                            │
│ Description: [Next of kin phone number_]                    │
│ Placeholder: [+44 7700 900000__________]                    │
│                                                             │
│ ── Type Config ──                                           │
│ (varies by type — regex for text, options for select, etc.) │
│                                                             │
│                              [Cancel]  [Save Field]         │
└─────────────────────────────────────────────────────────────┘
```

## Migration: Legacy Dynamic Fields

Legacy `dynamic_field` + `dynamic_field_value` tables are **never modified or deleted**. Migration is an automatic copy-on-startup that keeps both systems working side by side. This ensures safe downgrades — reverting to a pre-0.8.0 binary restores the original dynamic fields path with no data loss.

### Auto-Migration on Startup

Runs once per startup, after database migrations and before plugin loading:

```
Server starts
  → Run DB migrations (creates gk_custom_field_* tables if missing)
  → Run legacy field migration:
      1. SELECT all rows from dynamic_field
      2. For each legacy field:
          a. Map entity_type: Ticket→ticket, Article→article,
             CustomerUser→contact, CustomerCompany→customer_group
          b. Map field_type (see table below)
          c. Check gk_custom_field_def for matching (entity_type, name)
          d. If exists → skip (idempotent)
          e. If missing → INSERT into gk_custom_field_def:
               owner_type = 'legacy'
               owner_name = NULL
               migrated_from = legacy field ID
      3. For each newly created definition:
          a. Copy values: INSERT INTO gk_custom_field_value
             SELECT ... FROM dynamic_field_value WHERE field_id = ?
             (type-mapped to correct val_* column)
      4. Log summary: "Migrated N legacy dynamic fields (M values)"
  → Load plugins
  → Start HTTP server
```

### Key Properties

- **Copy, not move** — legacy tables are read-only during migration, never written to
- **Idempotent** — matched by `(entity_type, name)`, safe to run on every startup
- **owner_type='legacy'** — distinguishes migrated fields from plugin and admin fields
- **migrated_from** column — stores the legacy `dynamic_field.id` for traceability
- **Downgrade safe** — revert to an older binary and the legacy `dynamic_field` path still works with the original data intact
- **Fast** — most startups are a no-op SELECT that finds all fields already migrated

### Schema Addition

```sql
-- Add to gk_custom_field_def
ALTER TABLE gk_custom_field_def
    MODIFY owner_type ENUM('plugin','admin','legacy') NOT NULL DEFAULT 'admin',
    ADD COLUMN migrated_from BIGINT DEFAULT NULL,  -- legacy dynamic_field.id
    ADD INDEX idx_migrated (migrated_from);
```

### Type Mapping

| Legacy `field_type` | New `field_type` |
|---------------------|------------------|
| Text | text |
| TextArea | textarea |
| Checkbox | boolean |
| Dropdown | select |
| Multiselect | multi_select |
| Date | date |
| DateTime | datetime |
| WebserviceDropdown | select (options flattened) |
| WebserviceMultiselect | multi_select (options flattened) |

Legacy config YAML is converted to the new JSON config format during the copy.

### Legacy Rendering Path

The existing `dynamic_fields.pongo2` partial and `dynamic_field_repository.go` remain functional. Pages that currently render dynamic fields continue to work unchanged. The new `custom_fields.pongo2` partial renders fields from `gk_custom_field_def` — including legacy-migrated ones.

Over time, entity pages switch from the legacy partial to the new one. Once all pages are migrated, the legacy rendering path can be removed in a future release (but the legacy tables stay forever — they're someone else's problem to clean up).

## Searchable Fields & Indexing

Fields stored in `val_text`, `val_int`, `val_decimal`, `val_date`, `val_datetime` are indexed and queryable. The `val_json` column is not indexed (use extracted columns like `val_decimal`/`val_decimal2` for `point` queries).

### Query Examples

```sql
-- Find contacts where department = "Engineering"
SELECT v.object_id FROM gk_custom_field_value v
JOIN gk_custom_field_def d ON d.id = v.field_id
WHERE d.entity_type = 'agent' AND d.name = 'department'
  AND v.val_text = 'Engineering';

-- Find organisations within bounding box (pre-filter for "near" query)
SELECT v.object_id FROM gk_custom_field_value v
WHERE v.field_id = ?
  AND v.val_decimal BETWEEN ? AND ?
  AND v.val_decimal2 BETWEEN ? AND ?;
```

### Full-Text Search

`val_text` fields with `config.searchable: true` are included in the platform's full-text search index. This is opt-in per field to avoid index bloat.

## Validation

All validation runs server-side before storage:

| Check | Applies To | Error |
|-------|-----------|-------|
| Required | All types | "Field X is required" |
| Type coercion | All types | "Expected integer, got string" |
| `max_length` | text, textarea | "Must be ≤ N characters" |
| `regex` | text | Custom `regex_error` or "Invalid format" |
| `min` / `max` | integer, decimal | "Must be between X and Y" |
| `min_date` / `max_date` | date, datetime | "Date must be after X" |
| `options` membership | select, multi_select | "Invalid option: X" |
| `min_selected` / `max_selected` | multi_select | "Select between X and Y options" |
| Email format | email | "Invalid email address" |
| URL format | url | "Invalid URL" |
| Phone format | phone | "Invalid phone number" |
| GeoJSON validity | polygon | "Invalid GeoJSON polygon" |
| Country code | address | "Invalid country code" |

Validation errors are returned as structured errors using the existing `internal/apierrors` pattern.

## API Endpoints

### REST API v1

```
GET    /api/v1/custom-fields/definitions                    # List field definitions (filterable by entity_type)
GET    /api/v1/custom-fields/definitions/:id                # Get single definition
POST   /api/v1/custom-fields/definitions                    # Create definition (admin only)
PUT    /api/v1/custom-fields/definitions/:id                # Update definition (admin only)
DELETE /api/v1/custom-fields/definitions/:id                # Soft-delete definition (admin only)

GET    /api/v1/custom-fields/values/:entity_type/:id        # Get values for entity
PUT    /api/v1/custom-fields/values/:entity_type/:id        # Set values for entity
POST   /api/v1/custom-fields/query                          # Query entities by field values
```

### MCP Tools

```
custom_fields_get     — Get custom field values for an entity
custom_fields_set     — Set custom field values for an entity
custom_fields_query   — Query entities by custom field values
custom_fields_list    — List field definitions for an entity type
```

## Security Considerations

1. **Plugin isolation**: Plugins can only access their own prefixed fields + admin fields marked accessible
2. **SQL injection**: All queries use parameterised placeholders — no field names in user input are interpolated
3. **JSON validation**: `val_json` content is validated against expected schema before storage
4. **GeoJSON size limit**: Polygons capped at 100KB to prevent storage abuse
5. **Regex timeout**: User-provided regex patterns run with a 100ms deadline to prevent ReDoS
6. **Admin permissions**: Field CRUD requires `admin.custom_fields` group permission
7. **Data purge**: Hard delete requires `admin.custom_fields.purge` — separate from soft delete
8. **Rate limiting**: HostAPI custom field calls count against plugin rate limits

## Implementation Order

1. [ ] Design spec (this document)
2. [ ] Database migration — `gk_custom_field_def` + `gk_custom_field_value` tables
3. [ ] Go models — `CustomFieldDef`, `CustomFieldValue`, `CustomFieldSpec`
4. [ ] Repository layer — CRUD for definitions + values
5. [ ] Validation engine — type-specific validators
6. [ ] Legacy auto-migration — copy `dynamic_field` → `gk_custom_field_def` on startup
7. [ ] `CustomFieldSpec` in `GKRegistration` — plugin field declaration
8. [ ] Plugin manager integration — auto-create fields on plugin load
9. [ ] HostAPI methods — `CustomFieldsGet`, `CustomFieldsSet`, `CustomFieldsQuery`
10. [ ] Sandbox enforcement — prefix filtering, validation, rate limits
11. [ ] WASM + gRPC wire format — JSON-RPC host function registration
12. [ ] Auto UI rendering — `custom_fields.pongo2` partial (edit/view/inline modes)
13. [ ] Standard field types — text through phone
14. [ ] GIS field types — point, polygon, address (Leaflet integration)
15. [ ] Admin UI — list, create, edit, delete field definitions
16. [ ] REST API endpoints
17. [ ] MCP tool registration
18. [ ] Tests — unit (validation, storage), integration (HostAPI), E2E (admin UI, plugin fields)

---

*Design: 2026-03-26*
