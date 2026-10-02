# Stats Plugin

Ticket statistics and analytics plugin for GoatFlow (TinyGo WASM). Its widgets make up the agent
dashboard.

## Behaviour

- **Queue access** - admins see every queue. Other agents see only tickets in valid queues whose
  group they hold `rw` on, directly (`group_user`) or through a role (`role_user` -> `group_role`).
  A call without an identified user sees nothing. API routes, widgets and the weekly report apply
  the same rule (the report runs as admin).
- **Time windows** - "today", "last 30 days" and `?range=` windows are computed from the server's
  local time (host function `time_now`) and bound as query parameters, so MySQL and PostgreSQL
  return the same numbers regardless of the database time zone.
- **Errors** - a failed query never shows as a number: widgets render the "widget unavailable"
  state and API routes answer HTTP 500 `{"error":"query_failed"}`. An unknown `range` answers
  HTTP 400 `{"error":"invalid_range"}`.

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/plugins/stats/overview` | Total/open/pending/closed counts (`?range=7d\|30d\|90d\|365d\|all`) |
| GET | `/api/plugins/stats/by-status` | Counts grouped by state (`?range=`) |
| GET | `/api/plugins/stats/by-queue` | Top 10 queues by ticket count (`?range=`) |
| GET | `/api/plugins/stats/by-priority` | Counts grouped by priority (`?range=`) |
| GET | `/api/plugins/stats/by-type` | Counts grouped by ticket type (`?range=`) |
| GET | `/api/plugins/stats/by-owner` | Top 10 owners by ticket count (`?range=`) |
| GET | `/api/plugins/stats/recent-activity` | Recently changed tickets (`?limit=1..50`, default 10) |
| GET | `/api/plugins/stats/timeline` | Tickets created per local day (`?range=7d\|30d\|90d`, default 30d) |
| GET | `/api/plugins/stats/sla-compliance` | Per queue: open tickets with a running SLA and how many are past their escalation time (`?range=`) |
| GET | `/api/plugins/stats/time-tracking` | Accounted time by agent and queue (`?range=`) |

SLA figures cover open tickets only: closing a ticket clears its escalation times, so closed
tickets carry no SLA outcome. Tickets without an SLA are not counted.

All endpoints require authentication. The `weekly-report` job (Mondays 08:00) emails the last
7 days' figures to valid `rw` members of the `admin` group that have an email address; it needs
the `email` permission granted in the plugin's resource policy.

## Widgets

| ID | Location | Size | Description |
|----|----------|------|-------------|
| `stats_overview` | dashboard | medium | Total, open, closed, new today, pending, overdue |
| `stats_by_status` | dashboard | small | Top 5 states |
| `stats_chart` | dashboard | large | Tickets created per day, last 30 days |
| `stats_sla` | dashboard | medium | Open tickets within SLA, per queue |
| `stats_time_tracking` | dashboard | medium | Hours accounted in the last 30 days |

## Building

```bash
./build.sh
```

`stats.wasm` is committed; rebuild it after changing `main.go`.
