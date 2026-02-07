# Ticket Number Generators

This document provides an authoritative, task‑focused reference for selecting and operating ticket number generators in GoatFlow.

## Overview
The generator is chosen via the configuration setting `Ticket::NumberGenerator`. It is resolved at service startup and wired into the `TicketRepository` so all ticket creation paths use a single, consistent source of truth.

Current implementations:
- Increment
- Date
- DateChecksum
- Random

All generators respect the system's `SystemID` (where applicable) and use the shared counter store to maintain global ordering semantics where meaningful.

## Selection & Switching
1. Edit `config/Config.yaml`, locate the setting with name `Ticket::NumberGenerator`.
2. Set its `value` field to one of: `Increment`, `Date`, `DateChecksum`, `Random`.
3. (Optional) Adjust `SystemID` setting if you need a distinct multi‑instance prefix (Random always prepends it).
4. Restart the service (`make restart`).
5. Verify with `GET /admin/debug/ticket-number` (JSON shows `generator` and `date_based`).

If an unknown value is provided, the system logs a warning and falls back to `DateChecksum`.

## Generator Matrix
| Name | Example | Determinism | Human Sortable | Encodes Date | Collision Probability | Notes |
|------|---------|-------------|----------------|--------------|-----------------------|-------|
| Increment | 1000001234 | Strictly monotonic | Yes | No | None (DB enforced) | Fastest to index; ideal for analytics |
| Date | 20251005-1234 | Monotonic within day | Partially (date groups) | Yes | None (per-day counter) | Resets midnight server TZ |
| DateChecksum | 20251005-AB12 | Monotonic within day | Partially | Yes | None | Adds short checksum; light obfuscation |
| Random | 10 0012345678 (SystemID + 10 digits) | Non‑sequential | No | No | Extremely low (retry guarded) | Harder to guess volume; privacy‑leaning |

(Whitespace in Random example only for readability; real value is concatenated.)

## Operational Characteristics
### Increment
- Implementation: Atomic DB counter increment (UPsert) then format.
- Pros: Dense, fast, ideal for external references and sorting.
- Cons: Reveals ticket volume growth directly.

### Date
- Daily counter resets; tn includes date stamp.
- Pros: Quick mental grouping by day; easy partitioning.
- Cons: Gapless only per day; cross-day ordering requires date parse.

### DateChecksum
- Same as Date plus checksum block for integrity / light tamper detection.
- Pros: Slightly harder to fabricate; recognizable pattern.
- Cons: Slight overhead in generation & length.

### Random
- SystemID prefix + 10 random digits from PRNG seeded at startup.
- Maintains a counter increment internally purely for uniformity (not used in output) and metrics alignment.
- Pros: Obscures volume and timing; collisions extremely unlikely; retry loop (up to 5) handles rare unique constraint conflicts.
- Cons: No natural ordering by creation time; must use DB timestamp fields for sorting.

## Collision Handling (Random)
On unique constraint violation of `ticket.tn` the repository automatically retries generation (max 5 attempts). Each collision logs a warning with attempt count. Persistent failure after retries surfaces the DB error upstream.

## Debug & Introspection
- Current generator: `GET /admin/debug/ticket-number`
- Config sources listing: `GET /admin/debug/config-sources` (shows effective `Ticket::NumberGenerator` and origin).

## Migration Guidance
| Scenario | Recommendation |
|----------|---------------|
| You need strict sequential IDs for audits | Use Increment |
| You want date grouping for operations dashboards | Use Date or DateChecksum |
| You require mild tamper resistance in manual entry | Use DateChecksum |
| You need to obscure ticket volumes from customers | Use Random |

## Changing Generators Mid-Flight
Changing does not rewrite historical TNs. Mixed formats will coexist. Ensure downstream integrations treat TN as opaque string.

Checklist before switching production:
- Update any downstream regex validators to accept new format.
- Communicate to support staff about changed appearance.
- Confirm monitoring dashboards parse TNs generically (or are updated).

## Format Reference (Regex)
| Generator | Regex (simplified) |
|-----------|--------------------|
| Increment | `^[0-9]{6,}$` |
| Date | `^[0-9]{8}-[0-9]{3,}$` |
| DateChecksum | `^[0-9]{8}-[A-Z0-9]{4}$` (example form) |
| Random | `^[0-9]{2}[0-9]{10}$` (SystemID 2 digits + 10 digits) |

(Actual Increment length depends on counter growth; adapt `{6,}` accordingly.)

## Failure Modes & Logs
| Condition | Log Pattern | Action |
|-----------|-------------|--------|
| Unknown generator name | `Unknown Ticket::NumberGenerator` | Falls back to DateChecksum |
| Random collision | `Random TN collision` | Automatic retry |
| Generator not initialized (startup gap) | `ticket number generator not initialized` | Investigate wiring / config load |

## Best Practices
- Treat ticket number as opaque externally; avoid parsing except for Date/DateChecksum analytics.
- Index `ticket.tn` (unique) and rely on secondary indexes for time-based queries (`create_time`).
- When exporting to external BI tools, include both TN and creation timestamp.

## Future Enhancements (Planned)
- Config‑driven Random length variation.
- Metrics: collision count, generation latency histogram.
- Admin UI toggle & preview for each generator.

## Quick Verification Commands (Container Workflow)
After editing `Config.yaml`:
```bash
make restart
curl -s localhost:8080/admin/debug/ticket-number | jq
```

If not using `jq`, just inspect raw JSON output.
