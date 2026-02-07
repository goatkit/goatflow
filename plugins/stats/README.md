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

All endpoints require authentication.

## Widgets

| ID | Location | Size | Description |
|----|----------|------|-------------|
| `stats_overview` | dashboard | medium | Overview cards |
| `stats_by_status` | dashboard | small | Status breakdown |

## Building

```bash
./build.sh
```

## i18n

Supports English and German translations.
