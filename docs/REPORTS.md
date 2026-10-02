# Reports & Analytics

GoatFlow 0.10.0 has a ticket reports page at Admin -> Reports & Analytics (`/admin/reports`). It shows
ticket totals, a created-vs-closed trend, per-queue counts, agent activity and top customers, and
offers CSV and JSON downloads. All figures come from the `/api/v1/statistics/*` API, which you can
also call directly.

## Opening the page

| Item | Value |
|------|-------|
| URL | `/admin/reports` |
| Menu | Admin dashboard (`/admin`) -> "Reports & Analytics" card |
| Who can open it | Admins only. The `/admin` route group uses the `auth` and `admin` middleware. |

The page itself holds no data. When it loads, the browser calls the statistics API with the
logged-in session. If those calls return 401 or 403 the page shows "You need read access to at
least one queue to see reports." Any other failure shows "Some report data could not be loaded."

All dates are UTC.

## What the page shows

The page has two selectors:

| Selector | Options | Default | Affects |
|----------|---------|---------|---------|
| Period | Last 24 hours, Last 7 days, Last 30 days | Last 7 days | Agent activity and the export links |
| Trend | Daily, last 7 days; Daily, last 30 days; Monthly, last 12 months | Daily, last 30 days | Created vs. closed chart |

Sections:

| Section | Shows | Time range | Source |
|---------|-------|------------|--------|
| Overview | Total tickets, Open, Pending, Closed | All time | `GET /api/v1/statistics/dashboard` (`overview`) |
| Created vs. closed | Bar chart of created and closed tickets per day or month, plus Created, Closed and Closure rate totals | Trend selector | `GET /api/v1/statistics/trends` |
| Queues | Per queue: Total tickets, Open, Backlog | All time | `GET /api/v1/statistics/queues` |
| Agent activity | Per agent: Assigned, Closed, Articles. Agents with all three at 0 are hidden. | Period selector | `GET /api/v1/statistics/agents` |
| Top customers | Top 10 customers: Customer, Email, Total tickets, Open, Last ticket. Also Customers, Active (30 days), New this month. | All time | `GET /api/v1/statistics/customers?top=10` |
| Export | Summary (CSV), Tickets (CSV), Summary (JSON) download links | Period selector | `GET /api/v1/statistics/export` |

## How the numbers are counted

### Open, pending and closed

Each ticket is put in a bucket by the name of its state type (`ticket_state_type.name`):

| Bucket | State types |
|--------|-------------|
| Open | `new`, `open` |
| Pending | any name starting with `pending` (`pending reminder`, `pending auto`) |
| Closed | `closed` |

Tickets in other state types (for example `removed` or `merged`) count in "Total tickets" but in
no bucket.

### Close time

GoatFlow does not store a separate close time. For a ticket that is now in a `closed` state type,
its last change time (`ticket.change_time`) is used as the close time. If a closed ticket is
changed later, it moves to the later date.

### Backlog

Backlog is the number of open tickets (state type `new` or `open`) created more than 24 hours ago.
Pending tickets are not in the backlog.

### Trend buckets

- Daily: one bucket per UTC day, from `days - 1` days ago up to and including today.
- Monthly: one bucket per UTC calendar month, from `months - 1` months ago up to and including the
  current month.
- A ticket counts as "created" in the bucket of its create time, and as "closed" in the bucket of
  its close time (see above).
- "Open" is the number of tickets still unresolved at the end of the bucket (for today: now):
  tickets created by then that were not yet closed, merged or removed. Tickets created before the
  window that are still unresolved count too. A ticket now in a `closed`, `merged`, `removed` or
  other non-open, non-pending state type counts as resolved from its last change time; pending
  tickets count as unresolved.
- Closure rate = closed / created x 100 over the whole window. It is 0 when nothing was created.

### Agent activity

For each valid agent (`users.valid_id = 1`), within the selected period (see
[Permission scoping](#permission-scoping) for which agents are listed):

| Column | Counts |
|--------|--------|
| Assigned | Tickets created in the period where the agent is the responsible agent (`ticket.responsible_user_id`) |
| Closed | Tickets where the agent is the responsible agent, now in a `closed` state type, closed in the period |
| Articles | Articles the agent created (`article.create_by`) in the period |

The agent name is the agent's login.

### Customers

- A customer is the value of `ticket.customer_user_id`. Tickets without one are skipped.
- Email comes from `customer_user` where `login` matches. It is empty for customers with no
  `customer_user` row.
- Open counts tickets in `new` or `open` state types.
- Last ticket is the create time of the customer's newest ticket.
- Active (30 days): customers whose newest ticket was created in the last 30 days.
- New this month: customers whose first ticket was created on or after the 1st of the current UTC
  month.
- Customers are sorted by ticket count, then by newest ticket.

## Permission scoping

Every statistics endpoint counts only tickets the caller may read.

| Caller | Tickets counted |
|--------|-----------------|
| Member of the `admin` group | All tickets in valid queues |
| Other agent | Tickets in valid queues whose group the agent holds `ro` or `rw` on, directly (`group_user`) or through a role (`role_user` -> `group_role`) |
| Agent with no such queue | Request refused with 403 "You do not have access to any queues" |
| Customer (customer JWT or customer API token) | Request refused with 403 |

The check uses the queue access service (`QueueAccessService.IsAdmin` and
`GetAccessibleQueueIDs` with permission `ro`), the same rules as the `queue_ro` route middleware.
Tickets in invalid queues are counted for nobody, so admin totals always equal the sum of the
per-queue lists (which list valid queues only). Because `/admin/reports` is admin-only, everyone
who opens the page sees all tickets in valid queues. Non-admin agents only get scoped figures when
they call the API directly.

The agent list in `/statistics/agents` depends on the caller. Admins get every valid agent,
including agents with all counts at 0. Other agents get only the agents with at least one assigned
ticket, closed ticket or article on tickets in the queues they can read: those agents are already
visible on those tickets, while the full agent roster is not exposed.

## Exports

The Export section links to `GET /api/v1/statistics/export` with the selected period:

| Button | Query |
|--------|-------|
| Summary (CSV) | `type=summary&format=csv&period=<period>` |
| Tickets (CSV) | `type=tickets&format=csv&period=<period>` |
| Summary (JSON) | `type=summary&format=json&period=<period>` |

`<period>` is `24h`, `7d` or `30d`. The period is a rolling window ending now.

The file is sent as a download named `statistics_<YYYYMMDD_HHMMSS>.csv` or
`statistics_<YYYYMMDD_HHMMSS>.json` (UTC time of the export). CSV is sent as `text/csv`, JSON as
`application/json`.

### Summary export

Counts the tickets created in the period, by their current bucket.

CSV columns: `Metric`, `Value`. Rows: `total_tickets`, `open_tickets`, `closed_tickets`,
`pending_tickets`.

JSON:

```json
{
  "export_date": "2026-10-02T09:00:00Z",
  "period": "7d",
  "type": "summary",
  "summary": {
    "total_tickets": 0,
    "open_tickets": 0,
    "closed_tickets": 0,
    "pending_tickets": 0
  }
}
```

### Ticket list export

Lists the tickets created in the period, newest first.

CSV columns: `Ticket Number`, `Title`, `Queue`, `State`, `Priority`, `Customer`, `Created`.

With `format=json` the file is a JSON array of objects with the fields `ticket_number`, `title`,
`queue`, `state`, `priority`, `customer`, `created`. "State" is the state name, not the bucket.
"Created" is UTC in the form `2006-01-02 15:04:05`.

## API reference

All routes are `GET`, under `/api/v1`, in the `api-v1-protected` route group.

**Auth.** The group uses `unified_auth` and `agent`; each route adds `queue_ro`. You can send:

- an agent JWT as `Authorization: Bearer <token>`,
- an agent API token (`gf_...`) in the `Authorization` header,
- or the `auth_token` / `access_token` cookie of a browser session.

No API token scope is checked on these routes. See [API documentation](api/README.md) for getting
tokens.

### Routes

| Path | Query parameters | Returns |
|------|------------------|---------|
| `/statistics/dashboard` | none | Counts by bucket, queue and priority; 10 newest tickets |
| `/statistics/trends` | `period` = `daily` (default) or `monthly`; `days` 1-366, default 7 (daily only); `months` 1-24, default 3 (monthly only) | Created, closed and open per day or month |
| `/statistics/agents` | `period` = `24h`, `7d` (default) or `30d` | Per-agent activity (which agents: see [Permission scoping](#permission-scoping)) |
| `/statistics/queues` | none | Per-queue total, open and backlog |
| `/statistics/analytics` | `type` = `hourly` (default) or `day_of_week`; `days` 1-366, default 30 | Created/closed by UTC hour of day or weekday |
| `/statistics/customers` | `top` 1-100, default 10 | Top customers and customer totals |
| `/statistics/export` | `format` = `json` (default) or `csv`; `type` = `summary` (default) or `tickets`; `period` = `24h`, `7d` (default) or `30d` | File download (see [Exports](#exports)) |
| `/ticket-states/statistics` | none | Ticket count per ticket state |

### Response fields

**`/statistics/dashboard`** (all time)

| Field | Content |
|-------|---------|
| `overview` | `total_tickets`, `open_tickets`, `closed_tickets`, `pending_tickets` |
| `by_queue[]` | `queue_id`, `queue_name`, `count` (all tickets in the queue). Valid queues only, largest first. |
| `by_priority[]` | `priority_id`, `priority_name`, `count`. Valid priorities only, by priority ID. |
| `recent_activity[]` | The 10 newest tickets: `type` (always `created`), `ticket_id`, `ticket_tn`, `timestamp` |

**`/statistics/trends`**

| Field | Content |
|-------|---------|
| `period` | `daily` or `monthly` |
| `days` or `months` | The window size (key matches the period) |
| `trends[]` | `date` (`YYYY-MM-DD` daily, `YYYY-MM` monthly), `created`, `closed`, `open` |
| `summary` | `total_created`, `total_closed`, `average_per_day`, `closure_rate` (percent) |

`trends[].open` is the number of tickets still unresolved at the end of the bucket (see
[Trend buckets](#trend-buckets)). `average_per_day` is `total_created` divided by the number of
days in the window, also for `monthly`.

**`/statistics/agents`**

| Field | Content |
|-------|---------|
| `period` | `24h`, `7d` or `30d` |
| `agents[]` | `agent_id`, `agent_name`, `tickets_assigned`, `tickets_closed`, `articles_created`. Sorted by closed, then assigned, then name. |
| `top_performers[]` | Up to 3 agents with at least one closed ticket: `agent_id`, `agent_name`, `metric` (always `tickets_closed`), `value` |

**`/statistics/queues`** (all time)

| Field | Content |
|-------|---------|
| `queues[]` | `queue_id`, `queue_name`, `total_tickets`, `open_tickets`, `backlog`. Valid queues only, sorted by total, then name. |
| `totals` | `all_queues` (number of queues), `total_tickets`, `total_open` |

**`/statistics/analytics`**

Counts tickets created in the last `days` days and tickets closed in that window.

| Field | Content |
|-------|---------|
| `type` | `hourly` or `day_of_week` |
| `days` | The window size |
| `data[]` | `hourly`: 24 rows of `hour` (0-23), `created`, `closed`. `day_of_week`: 7 rows of `day` (`Monday` to `Sunday`), `created`, `closed`. |
| `peak_hours` | `hourly` only: hours with the most created tickets (empty when none) |
| `busiest_days` | `day_of_week` only: day names with the most created tickets (empty when none) |

**`/statistics/customers`** (all time)

| Field | Content |
|-------|---------|
| `top_customers[]` | `customer_id`, `customer_email`, `ticket_count`, `open_tickets`, `last_activity` (RFC 3339) |
| `customer_metrics` | `total_customers`, `active_customers`, `new_customers_this_month`, `avg_tickets_per_customer` |

**`/ticket-states/statistics`** (all time)

| Field | Content |
|-------|---------|
| `statistics[]` | `state_id`, `state_name`, `type_id`, `ticket_count`. Valid states only, by state ID. |
| `total_tickets` | Sum of `ticket_count` |

### Errors

Errors are JSON with an `error` field. The statistics handlers put a plain message in it. The
401 from the auth middleware uses an object with `code` and `message`.

| Status | When |
|--------|------|
| 400 | A query parameter has a value not listed above, for example `period must be one of 24h, 7d, 30d` or `days must be an integer between 1 and 366` |
| 401 | No valid token or session |
| 403 | Customer caller, or an agent with read access to no queue |
| 500 | Database or permission lookup failed (`Failed to load statistics`, `Failed to check permissions`). Failed queries return 500, never zeros. |

### Example

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "https://goatflow.example.com/api/v1/statistics/trends?period=monthly&months=12"

curl -H "Authorization: Bearer $TOKEN" -OJ \
  "https://goatflow.example.com/api/v1/statistics/export?type=tickets&format=csv&period=30d"
```

## Limits

- The page and API offer only the fixed reports above. There is no custom report builder, no
  saved reports and no scheduled reports: the only routes are the read-only `GET` routes listed
  here.
- Periods are fixed: `24h`, `7d` or `30d` for agents and exports; trend and analytics windows are
  set with `days` / `months` within the ranges above.
- Close times are the ticket's last change time (see [Close time](#close-time)).
- All bucketing is in UTC.
