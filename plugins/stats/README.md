# Stats Plugin

Ticket statistics and analytics plugin for GoatFlow.

## Features

- **Overview Dashboard Widget** - Total, open, pending, closed ticket counts
- **By Status Widget** - Ticket counts grouped by status
- **API Endpoints** - JSON data for custom dashboards

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/plugins/stats/overview` | Ticket counts overview |
| GET | `/api/plugins/stats/by-status` | Counts grouped by status |
| GET | `/api/plugins/stats/by-queue` | Counts grouped by queue |
| GET | `/api/plugins/stats/recent-activity` | Recent ticket changes |
| GET | `/api/plugins/stats/by-priority` | Counts grouped by priority |
| GET | `/api/plugins/stats/by-type` | Counts grouped by ticket type |
| GET | `/api/plugins/stats/by-owner` | Counts by owner/agent |
| GET | `/api/plugins/stats/timeline` | Daily ticket creation counts |
| GET | `/api/plugins/stats/sla-compliance` | SLA compliance rates by queue (`?range=7d|30d|90d|all`) |
| GET | `/api/plugins/stats/time-tracking` | Time tracking analytics by agent and queue (`?range=7d|30d|90d|all`) |

All endpoints require authentication. A `weekly-report` scheduled job generates a weekly summary.

## Widgets

| ID | Location | Size | Description |
|----|----------|------|-------------|
| `stats_overview` | dashboard | medium | Overview cards |
| `stats_by_status` | dashboard | small | Status breakdown |
| `stats_chart` | dashboard | large | Ticket count chart |
| `stats_sla` | dashboard | medium | SLA compliance |
| `stats_time_tracking` | dashboard | medium | Time tracking |

## Building

```bash
./build.sh
```

## i18n

Supports English and German translations.
