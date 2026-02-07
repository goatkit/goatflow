# Configuration System

This document explains how GoatFlow configuration is structured, how precedence works, and how to inspect and change settings (including ticket number generators).

## Layers & Precedence (Current)
1. default.yaml (static operational defaults shipped with the binary/container; contains no real secrets)
2. Config.yaml (SysConfig registry: settings array with name, group, description, default, value)
   - Each setting may define `default` and optionally a concrete `value` override.
3. (Planned) Dynamic overrides (DB persisted edits via UI or API)
4. (Planned) Environment / runtime ephemeral overrides

Effective value resolution today:
- If a setting has a non-nil `value`, that is used (empty string counts as intentional override)
- Else if it has `default`, that is used
- Else unresolved (treated as error when fetched)

## File Roles
- default.yaml: Not user edited at runtime. Provides stable fallback values the code expects. Credential fields are blank/placeholders—real secrets come from env or local override.
- Config.yaml: Canonical registry of configurable settings plus human metadata.

## Example Setting Shape (Config.yaml)
```yaml
settings:
  - name: Ticket::NumberGenerator
    group: Ticket
    description: Select ticket number generator implementation
    default: Date
    value: Increment
```

## Ticket Number Generators
For full operational guidance (selection matrix, regex formats, migration notes) see `docs/ticket_number_generators.md`.

Available generators (select via `Ticket::NumberGenerator`) quick reference:

| Name | Core Idea | When To Use |
|------|-----------|-------------|
| Increment | Global monotonic counter | Audits, sortable identifiers |
| Date | Date + daily counter | Operational grouping by day |
| DateChecksum | Date + checksum | Date grouping with light integrity check |
| Random | SystemID + 10 random digits | Obfuscate volume / privacy |

(Actual Increment visual depends on configured formatting; above is illustrative.)

## Switching Generators
1. Edit `Config.yaml` setting `Ticket::NumberGenerator` and set `value` to desired generator name.
2. Restart service (`make restart` in container workflow) so startup selects the new generator.
3. Verify via: `GET /admin/debug/ticket-number` (returns current generator and dateBased flag).

## Introspection (Planned Endpoint)
Endpoint: `GET /admin/debug/config-sources`
Will return JSON: each setting with its name, default, value, effective, and source (`value` or `default`).

## Collision Handling (Random)
Random generator composes SystemID + 10 random digits. Collisions are improbable but possible. A retry loop (up to 5 attempts on unique `tn` constraint violation) will be added alongside a metric counter.

## Duplicate Settings
At startup a scan will log a warning if duplicate setting names are detected in Config.yaml to prevent shadowing.

## Future Enhancements
- Persist runtime edits (UI) into versioned config history
- Env var injection (e.g. GOATFLOW__Ticket__NumberGenerator)
- Hot reload with audit log of changes

## Operational Verification
- List current generator: `GET /admin/debug/ticket-number`
- (Soon) List all settings with sources: `GET /admin/debug/config-sources`

## Failure Modes
- Missing setting fetch → error returned to caller
- Duplicate names → startup warning only (first wins)
- Invalid generator name → falls back to default defined in setting (currently Date if unresolved)

## Safety Notes
- Avoid editing default.yaml; changes are overwritten on upgrade and must stay secret-free.
- Keep Config.yaml under version control for auditable changes.
- Place developer-only tweaks (e.g. timezone) in `config/config.yaml` which is gitignored.

### Secrets & Local Overrides
Secrets must be injected via environment or `config/config.yaml` (gitignored). Example:

`config/config.yaml`:
```yaml
app:
  timezone: Europe/London
database:
  password: ${DEV_DB_PASSWORD}
auth:
  jwt:
    secret: ${DEV_JWT_SECRET}
```

Shell before starting:
```sh
export DEV_DB_PASSWORD=supersecret
export DEV_JWT_SECRET=change-this-local
```
