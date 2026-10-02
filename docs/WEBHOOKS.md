# Outbound Webhooks

GoatFlow can send ticket and article events to other systems as HTTP POST requests with a JSON body.
Each delivery can be signed with an HMAC-SHA256 signature, is retried on failure, and is kept in a
delivery log you can inspect and resend. This page describes GoatFlow 0.10.0.

## Requirements

### A runner process must be running

Events are found and delivered only by the background runner, not by the web app.
The runner is the same `goats` binary started with `-mode runner`:

```bash
./goats -mode runner
```

The runner registers a task named `webhook-dispatch` that runs every 10 seconds
(cron `*/10 * * * * *`). Without a runner, webhooks can be configured and the **Send test** button
works, but no ticket or article event is ever sent.

| Deployment | Runner | `GOATFLOW_SECURE_KEY` given to the runner |
|---|---|---|
| `docker-compose.yml` (repo root, development) | Service `runner`, command `./goats -mode runner` | Yes, required `${GOATFLOW_SECURE_KEY:?...}` (shared with the backend) |
| `deploy/docker-compose.yml` | Service `runner`, image `ghcr.io/goatkit/goatflow-runner`, command `./goats -mode runner` | Yes, required `${GOATFLOW_SECURE_KEY:?...}` (shared with app and customer-fe) |
| TrueNAS app (`docs/truenas-app/goatflow`) | Container `goatflow-runner`, command `./goats -mode runner` | Yes, from the `secure_key` setting (64 hex characters) |
| Helm chart (`charts/goatflow`) | Deployment `<fullname>-runner` (`runner.*` values) | Yes, from the `<fullname>-app` Secret (key `secure-key`), shared with the backend |

You may run more than one runner. Each event is queued once and each delivery attempt is claimed by
one runner only (see [How events are found](#how-events-are-found)).

### `GOATFLOW_SECURE_KEY` must be set, and be the same everywhere

Signing secrets and custom header values are stored encrypted (AES-256-GCM) with the key in
`GOATFLOW_SECURE_KEY`. The web app encrypts them when you save them. The runner decrypts them for
every delivery.

| Rule | Detail |
|---|---|
| Format | Exactly 64 hex characters (32 bytes). Example: `openssl rand -hex 32` |
| Same value | The app and every runner must use the same value. |
| Not set or empty | Each process makes its own random key at start and logs `generated an ephemeral secure settings key ... set GOATFLOW_SECURE_KEY`. The key itself is never logged. The runner then cannot read secrets and header values saved by the app, and they become unreadable after any restart. |
| Wrong format | Saving a webhook with a secret or custom headers fails with HTTP 500. The log shows `secure config key: invalid GOATFLOW_SECURE_KEY ...` or `GOATFLOW_SECURE_KEY must be 32 bytes (64 hex chars)`. |
| Changed later | Every stored secret and header value becomes unreadable. Enter them again (see [Troubleshooting](#troubleshooting)). |

Both compose files refuse to start until `GOATFLOW_SECURE_KEY` is set in `.env`, and give the
same value to every GoatFlow container. The Helm chart generates the key once and keeps it on
upgrade.

Webhooks without a secret and without custom headers do not use the key.

### Operator settings

These are environment variables of the app **and** every runner (set the same value on both).
They are not in the admin UI on purpose: an admin account must not be able to change them.

| Variable | Default | Meaning |
|---|---|---|
| `GOATFLOW_WEBHOOK_ALLOW_PRIVATE_TARGETS` | unset (deny) | `true` lets webhooks reach loopback, private, link-local and other internal addresses. See [Internal and private addresses](#internal-and-private-addresses). |
| `GOATFLOW_WEBHOOK_DELIVERY_RETENTION_DAYS` | `30` | Delivered and failed deliveries older than this many days are deleted by the runner (checked at most once an hour). `0` keeps them forever. Only the runner reads it. |

The compose files do not pass these variables: add them to the `environment` of the backend and
runner services when you need a value other than the default. With the Helm chart, use
`backend.extraEnv` and `runner.extraEnv`.

## Manage webhooks in the admin UI

Go to **Admin -> Webhooks** (`/admin/webhooks`). The page is for admins only.
It uses the [admin API](#admin-api) below.

| Action | What it does |
|---|---|
| **Add webhook** | Create a webhook: name, payload URL, events, signing secret, custom headers, retries, timeout, active. |
| Edit | Change any field. Leave the secret empty to keep the current secret. Stored custom headers show only their name and a hint; leave the value empty to keep it, or type a new value. |
| Remove the secret | Tick "Remove the secret (deliveries will not be signed)" when editing. |
| **Activate** / **Deactivate** | Switch the webhook on or off. Inactive webhooks receive no events. |
| Delete | Delete the webhook and its whole delivery log. |
| **Send test** | Send a `webhook.test` event now and show the result. |
| **Deliveries** | Show the 50 newest deliveries: time, event, status, HTTP status, attempts, duration. |
| Delivery details | Show the payload, the response, the error, created / delivered / next attempt times. |
| **Redeliver** | Send the same payload again now, as a new delivery. |

The list shows each webhook's URL, events, active state and last delivery.

The signing secret and the custom header values are write-only. After you save them, GoatFlow
shows only a hint: the last 4 characters of the secret, for example `••••••••abcd`, and of header
values that are at least 12 characters long (shorter header values show `••••••••` only).

### Fields and limits

| Field | Default | Rule |
|---|---|---|
| Name | none | Required. Up to 200 characters. Must be unique. |
| Payload URL | none | Required. Up to 2000 characters. Absolute `http://` or `https://` URL with a host. The host must not be an internal IP address or `localhost` unless internal targets are allowed (see [Internal and private addresses](#internal-and-private-addresses)). |
| Events | none | At least one. Must be a name from [Events](#events). Duplicates are removed. |
| Signing secret | none (unsigned) | Optional. 16 to 512 characters. |
| Custom headers | none | Up to 20. Name must be a valid HTTP header name of up to 256 characters and may appear only once (names are compared ignoring case). Value up to 1024 characters, no control characters (line breaks are rejected). These names are reserved and rejected: `Content-Type`, `Content-Length`, `Host`, `User-Agent`, `X-Webhook-Signature`, `X-Webhook-Event`, `X-Webhook-Delivery` (any case). |
| Retries | 3 | 0 to 10. Extra attempts after a failed first attempt. |
| Timeout (seconds) | 10 | 1 to 60. Per attempt. |
| Active | on | Inactive webhooks receive no events. |

Notes:

- Redirects are not followed. A 3xx response counts as a failure. Point the URL at the final endpoint.
- Custom header values are encrypted like the signing secret. They are never shown again or
  returned by the API.

### Internal and private addresses

By default a webhook cannot reach internal addresses, so a webhook URL cannot be used to make the
server call internal services (for example the cloud metadata endpoint `169.254.169.254`, a
database port on `127.0.0.1`, or hosts on the office LAN). Blocked ranges:

| Range | Addresses |
|---|---|
| Loopback | `127.0.0.0/8`, `::1`, the names `localhost` and `*.localhost` |
| Private | `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `fc00::/7` (includes `fd00:ec2::254`) |
| Link-local | `169.254.0.0/16` (includes the `169.254.169.254` metadata address), `fe80::/10` |
| Unspecified and "this network" | `0.0.0.0/8`, `::` |
| Multicast | `224.0.0.0/4`, `ff00::/8` |
| Shared and reserved | `100.64.0.0/10` (carrier-grade NAT, also used by cloud metadata services), `240.0.0.0/4` (includes `255.255.255.255`) |

IPv4 addresses written as IPv6 (`::ffff:127.0.0.1`) are checked as IPv4.

The check runs at two points:

| When | What is checked | Result |
|---|---|---|
| Create and update | The URL host, when it is an IP address, `localhost` or `*.localhost`. Other host names need DNS and are checked at delivery. | `400`: `webhook host <host> is a loopback, private, link-local or other internal address (set GOATFLOW_WEBHOOK_ALLOW_PRIVATE_TARGETS=true to allow internal targets)` |
| Every delivery attempt (runner, **Send test**, **Redeliver**) | GoatFlow resolves the host itself and checks **every** address in the answer. If any one is internal, no connection is made. Otherwise it connects to one of exactly those addresses, so a DNS answer that changes between check and connect (DNS rebinding) cannot redirect the request. | The delivery fails at once, without retries: `webhook host <host> resolves to <address>, a loopback, private, link-local or other internal address (...)` |

Because GoatFlow must see the address it connects to, deliveries do not use `HTTP_PROXY` or
`HTTPS_PROXY`.

To send webhooks to internal services (for example on-premises systems on your LAN), set
`GOATFLOW_WEBHOOK_ALLOW_PRIVATE_TARGETS=true` on the app and every runner and restart them. This
allows all of the ranges above for every webhook. Only do this when every admin may reach every
internal address the server can reach.

## Events

| Event | When it fires | Source table |
|---|---|---|
| `ticket.created` | A ticket row is added (any channel: agent, customer, email, API). | `ticket` |
| `ticket.updated` | History type `TitleUpdate`, `CustomerUpdate`, `TypeUpdate`, `ServiceUpdate`, `SLAUpdate`, `ResponsibleUpdate`, `Lock`, `Unlock`, `SetPendingTime`, `TicketDynamicFieldUpdate` or `ArchiveFlagUpdate`. | `ticket_history` |
| `ticket.state_changed` | History type `StateUpdate` to a state whose state type is not `closed`. | `ticket_history` |
| `ticket.closed` | History type `StateUpdate` to a state whose state type is `closed`. | `ticket_history` |
| `ticket.queue_moved` | History type `Move`. | `ticket_history` |
| `ticket.assigned` | History type `OwnerUpdate` (owner changed). | `ticket_history` |
| `ticket.priority_changed` | History type `PriorityUpdate`. | `ticket_history` |
| `ticket.merged` | History type `Merged`. | `ticket_history` |
| `ticket.escalated` | History type `EscalationResponseTimeStart`, `EscalationUpdateTimeStart` or `EscalationSolutionTimeStart`. | `ticket_history` |
| `article.created` | An article row is added (note, reply, email, phone call, web request). | `article` |

Other history types (for example `AddNote`, notifications, time accounting) do not produce events.
Articles are published from the `article` table instead.

`webhook.test` is sent only by **Send test**. You cannot subscribe to it.

`GET /api/v1/webhooks/events` returns this list with a description for each event.

### How events are found

GoatFlow does not hook into each handler. The runner reads new rows from the `ticket`, `article`
and `ticket_history` tables by id. So every write path produces events: UI, API, email import,
escalation checks and external tools that write OTRS-style rows.

| Step | Detail |
|---|---|
| Cursor | Table `gk_webhook_event_cursor` holds the last processed id per source (`ticket`, `article`, `ticket_history`). |
| First start | When a source has no cursor yet, the cursor is set to the current highest id. Existing tickets, articles and history are **not** sent. Only later changes become events. |
| Poll | Every 10 seconds. Up to 200 rows per source per batch. |
| Settle delay | A row is published only after it was already visible on the previous poll. Events are normally sent 10 to 20 seconds after the change. After the runner starts, the first poll publishes nothing. |
| Exactly once | The cursor move and the queued deliveries are written in one transaction. The cursor update only succeeds if the cursor still has the value the runner read. If another runner moved it first, the transaction is rolled back. |
| Runner down | The cursor is stored in the database. When the runner comes back, it continues from the cursor. No events are lost. |
| Ids go down | If the highest id is lower than the cursor (rows deleted, restore, sequence reset), the cursor is lowered to that id. |

Events are queued only for webhooks that are **active and subscribed at the moment the event is
published**. The cursor moves on even when no webhook wants an event. Activating a webhook or adding
an event later does not replay earlier changes.

## Payload

Every request body is one JSON object (the envelope):

| Field | Type | Meaning |
|---|---|---|
| `event` | string | Event name, for example `ticket.created`. |
| `occurred_at` | string (RFC 3339, UTC) | `create_time` of the source row. For `webhook.test`: the time of the test. |
| `data` | object | Event data, see below. |

The body is compact JSON (no spaces or line breaks). Examples below are formatted for reading.

### `data` by event

| Event | `data` fields |
|---|---|
| `ticket.created` | `ticket` |
| `article.created` | `ticket`, `article` |
| All other `ticket.*` events | `ticket`, `change` |
| `webhook.test` | `webhook_id`, `name`, `message` (`"Test delivery from GoatFlow"`) |

`ticket` and `article` are read **when the event is published**, not when the change happened.
If the ticket changed again in between, you get the newer values. Use `change` for what happened.

`data.ticket`:

| Field | Meaning |
|---|---|
| `id` | Ticket id |
| `ticket_number` | Ticket number (`ticket.tn`) |
| `title` | Title |
| `queue_id`, `queue` | Queue id and name |
| `state_id`, `state`, `state_type` | State id, state name, state type name |
| `priority_id`, `priority` | Priority id and name |
| `owner_id` | Owner user id |
| `responsible_user_id` | Responsible user id, or `null` |
| `customer_id`, `customer_user_id` | Customer company and customer user, or `null` |
| `created_at`, `updated_at` | Ticket create and change time (UTC) |

If the ticket was deleted before publishing, `ticket` is `{"id": <id>, "deleted": true}`.

`data.article` (metadata only, no body; fetch the body through the REST API, see
[docs/api/README.md](api/README.md)):

| Field | Meaning |
|---|---|
| `id` | Article id |
| `ticket_id` | Ticket id |
| `sender_type` | Sender type name, for example `agent` or `customer` |
| `is_visible_for_customer` | `true` or `false` |
| `from`, `subject` | From and subject from `article_data_mime`, or `null` |
| `created_by` | User id that created the article |
| `created_at` | Article create time (UTC) |

If the article was deleted before publishing, `article` is `{"id": <id>, "deleted": true}`.

`data.change` (history events only):

| Field | Meaning |
|---|---|
| `history_id` | `ticket_history.id` |
| `history_type` | History type name, for example `StateUpdate` |
| `message` | `ticket_history.name` text |
| `user_id` | User id that made the change |

### Example: `ticket.created`

```json
{
  "event": "ticket.created",
  "occurred_at": "2026-10-02T09:15:04Z",
  "data": {
    "ticket": {
      "created_at": "2026-10-02T09:15:04Z",
      "customer_id": "acme",
      "customer_user_id": "jane@example.com",
      "id": 42,
      "owner_id": 1,
      "priority": "3 normal",
      "priority_id": 3,
      "queue": "Raw",
      "queue_id": 2,
      "responsible_user_id": null,
      "state": "new",
      "state_id": 1,
      "state_type": "new",
      "ticket_number": "2026100210000012",
      "title": "Printer is offline",
      "updated_at": "2026-10-02T09:15:04Z"
    }
  }
}
```

## HTTP request

| Item | Value |
|---|---|
| Method | `POST` |
| `Content-Type` | `application/json` |
| `User-Agent` | `GoatFlow-Webhook/1.0` |
| `X-Webhook-Event` | Event name, for example `ticket.created` |
| `X-Webhook-Delivery` | Delivery id (`gk_webhook_delivery.id`). Same on every retry of one delivery. A Redeliver gets a new id. |
| `X-Webhook-Signature` | `sha256=<hex>`. Only sent when the webhook has a secret. |
| Custom headers | Added to every request. They cannot replace the headers above. |
| Timeout | The webhook's timeout (default 10 s, 1 to 60 s) covers the whole attempt. On timeout the error is `no response within <N> seconds`. |
| Redirects | Not followed. |
| Target address | Internal addresses are refused unless allowed, see [Internal and private addresses](#internal-and-private-addresses). No HTTP proxy is used. |

Each runner sends to up to 8 webhooks at the same time. The deliveries of one webhook are sent one
at a time, oldest due first, so a slow or unreachable endpoint delays only its own deliveries.
The `webhook-dispatch` run is limited to 5 minutes; deliveries still due when it ends stay
`pending` and are sent by the next run.

## Signature

When a webhook has a secret, every request has:

```
X-Webhook-Signature: sha256=<lower-case hex HMAC-SHA256>
```

- Key: the webhook's signing secret.
- Signed input: the raw request body bytes, exactly as received. Nothing else (no timestamp, no
  headers).
- Without a secret, the header is not sent.

Verify the signature on the raw body **before** you parse the JSON. Use a constant-time compare.

Go:

```go
import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

func validSignature(secret string, body []byte, header string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(header))
}

// In the handler:
//   body, _ := io.ReadAll(r.Body)
//   ok := validSignature(secret, body, r.Header.Get("X-Webhook-Signature"))
```

Python:

```python
import hashlib
import hmac

def valid_signature(secret: str, body: bytes, header: str) -> bool:
    expected = "sha256=" + hmac.new(secret.encode(), body, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, header or "")

# In the handler: valid_signature(secret, request.get_data(), request.headers.get("X-Webhook-Signature"))
```

The signature does not include a timestamp. To ignore duplicates, store the
`X-Webhook-Delivery` ids you have already processed.

## Delivery, retries and the log

### Success

Any `2xx` response means delivered. Everything else is a failure: other status codes (including
`3xx`), connection errors and timeouts.

### Retries

A failed attempt is retried while the number of attempts so far is not more than the webhook's
retry count. With the default of 3 retries a delivery gets up to 4 attempts.

Wait before the next attempt = 30 seconds × 2^(failed attempts − 1), at most 1 hour:

| After failed attempt | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 and later |
|---|---|---|---|---|---|---|---|---|
| Wait | 30 s | 1 min | 2 min | 4 min | 8 min | 16 min | 32 min | 1 h |

The runner checks for due deliveries every 10 seconds, so a retry can start up to 10 seconds late.

A delivery fails at once, without an HTTP attempt and without retries, when:

- the webhook is inactive when the delivery is due (error `webhook is inactive`),
- the secret or the custom header values cannot be decrypted (error starts with
  `cannot decrypt webhook secret` or `cannot decrypt webhook header values`), or
- the host is or resolves to an internal address and internal targets are not allowed (error
  starts with `webhook host`).

**Send test** and **Redeliver** make one attempt, right away, from the web app process (not the
runner). They are not retried. They also work for inactive webhooks.

If a runner stops in the middle of a send, the delivery stays in `delivering`. After 10 minutes
another runner takes it over and sends it again, so your endpoint can receive the same
`X-Webhook-Delivery` twice.

### Delivery status

| Status | Meaning |
|---|---|
| `pending` | Waiting for its first attempt or for a retry (`next_attempt_at` is set). |
| `delivering` | A runner or the web app is sending it now. A runner claims a delivery just before sending it. |
| `delivered` | Got a `2xx` response. |
| `failed` | Last attempt failed and no retries are left, or failed without an attempt (see above). |

### What is recorded

Each delivery is one row in `gk_webhook_delivery`. Each attempt updates that row:

| Field (API name) | Meaning |
|---|---|
| `attempts` | Number of HTTP attempts so far |
| `status_code` | HTTP status of the last attempt, or `null` if there was no response |
| `response` | First 4096 bytes of the last response body |
| `error` | Error of the last attempt, up to 1000 bytes, for example `endpoint returned HTTP 500` |
| `duration_ms` | Duration of the last attempt |
| `next_attempt_at` | When the next retry is due (only while `pending`) |
| `delivered_at` | When it was delivered |
| `payload` | The exact body that is sent |

### Retention

The runner deletes `delivered` and `failed` deliveries whose `created_at` is older than
`GOATFLOW_WEBHOOK_DELIVERY_RETENTION_DAYS` (default 30 days), at most once an hour, and logs
`webhook-dispatch: deleted N deliveries older than D days`. `pending` and `delivering` deliveries
are never deleted. `0` turns the clean-up off. Deleting a webhook still deletes its whole log.

## Admin API

All routes are in `routes/api-webhooks.yaml` under `/api/v1` with middleware `unified_auth` and
`admin`.

Who can call them:

- An admin with a browser session or a JWT (`Authorization: Bearer <jwt>`).
- An agent API token (`Authorization: Bearer gf_...`) whose owner is an admin **and** whose scopes
  include `*` or `admin:*`, or that has no scopes. Other tokens get `403 {"error": "Admin access required"}`.

Responses use `{"success": true, "data": ...}` or `{"success": false, "error": "<message>"}`.

| Method | Path | What it does |
|---|---|---|
| `GET` | `/api/v1/webhooks` | List webhooks, sorted by name. Optional `?active=true` or `?active=false`. |
| `POST` | `/api/v1/webhooks` | Create a webhook. Returns `201`. |
| `GET` | `/api/v1/webhooks/events` | List subscribable events: `[{"event", "description"}]`. |
| `GET` | `/api/v1/webhooks/:id` | Get one webhook. |
| `PUT` | `/api/v1/webhooks/:id` | Partial update. Only fields you send change. |
| `DELETE` | `/api/v1/webhooks/:id` | Delete the webhook and its delivery log. |
| `POST` | `/api/v1/webhooks/:id/test` | Send a `webhook.test` event now. Returns the delivery. |
| `GET` | `/api/v1/webhooks/:id/deliveries` | Newest deliveries first, without `payload` and `response`. `?limit=` 1 to 200, default 50. |
| `GET` | `/api/v1/webhook-deliveries/:id` | One delivery, with `payload` and `response`. |
| `POST` | `/api/v1/webhook-deliveries/:id/redeliver` | Send a delivery's payload again now as a new delivery. Returns the new delivery. |

`/test` and `/redeliver` return `200` even when the attempt failed. Check `data.success` and
`data.status`.

### Request body (create and update)

| Field | Type | Notes |
|---|---|---|
| `name` | string | Required on create. |
| `url` | string | Required on create. |
| `events` | array of strings | Required on create. On update, replaces the whole list. |
| `secret` | string | Optional. On update: leave it out to keep the secret, send `""` to remove it. |
| `headers` | object (name -> value) | Optional. Values are write-only. On update, the object replaces all custom headers: a `null` value keeps the stored value of that name, names left out are removed, `{}` removes all. Leave `headers` out to keep them all. A `null` for a name that has no stored value is a `400`. |
| `retry_count` | integer | Default 3. |
| `timeout_seconds` | integer | Default 10. |
| `is_active` | boolean | Default `true`. |

Rules are the same as in [Fields and limits](#fields-and-limits).

Example:

```bash
curl -X POST https://goatflow.example.com/api/v1/webhooks \
  -H "Authorization: Bearer gf_..." \
  -H "Content-Type: application/json" \
  -d '{
    "name": "CRM",
    "url": "https://crm.example.com/hooks/goatflow",
    "events": ["ticket.created", "ticket.closed"],
    "secret": "a-long-random-secret-value",
    "headers": {"X-Source": "goatflow"}
  }'
```

### Webhook object

`id`, `name`, `url`, `events`, `header_hints`, `has_secret`, `secret_hint` (only when a secret is
set), `retry_count`, `timeout_seconds`, `is_active`, `created_at`, `created_by`, `updated_at`,
`updated_by`. `header_hints` maps each custom header name to a masked hint of its value, for
example `{"X-Api-Key": "••••••••abcd"}` (`{}` without headers). The secret and the header values
are never returned.

### Delivery object

`id`, `webhook_id`, `event`, `payload`, `status`, `success` (`true` when `delivered`), `attempts`,
`status_code`, `response`, `error`, `duration_ms`, `next_attempt_at`, `delivered_at`,
`created_at`, `updated_at`.

### Errors

| Status | When |
|---|---|
| `400` | Validation failed (the message says which rule, including an internal URL host), invalid JSON, bad id, bad `active` or `limit`. |
| `401` | Create or update without a signed-in user (`authentication required`). |
| `403` | Not an admin. |
| `404` | Webhook or delivery not found. |
| `409` | `a webhook with this name already exists` |
| `500` | Database error, or the secret or header values could not be encrypted or decrypted (check `GOATFLOW_SECURE_KEY`). |
| `503` | `database unavailable` |

## Troubleshooting

| Problem | Cause | What to do |
|---|---|---|
| Test works, but no ticket events arrive | No runner is running. Test deliveries come from the web app; events come only from the runner. | Start a runner (`./goats -mode runner`; the Helm chart's `runner.enabled`). Its log shows the `webhook-dispatch` task registering and, when it sends, `webhook-dispatch: queued N deliveries, attempted M`. |
| No deliveries for one webhook | Webhook is inactive, or the event is not ticked. | Activate it and tick the events. Changes made before that are not replayed. |
| Old tickets were never sent | By design. On first start the cursor starts at the newest row. | None. Only changes after the runner's first poll are sent. |
| Events arrive 10 to 20 seconds late | By design (10 s poll plus one poll settle delay). | None. |
| Deliveries `failed` with `cannot decrypt webhook secret (is GOATFLOW_SECURE_KEY the same for every GoatFlow process?)` or `cannot decrypt webhook header values (...)` | `GOATFLOW_SECURE_KEY` is not set, differs between app and runner, or was changed after the secret or headers were saved. | Set the same 64-hex-character key on the app and every runner and restart them. Then edit the webhook and enter the secret and header values again. Use **Redeliver** for the failed deliveries. |
| Saving a webhook with a secret or custom headers returns HTTP 500 | `GOATFLOW_SECURE_KEY` is not 64 hex characters. | Generate one with `openssl rand -hex 32`. |
| Saving fails with `webhook host ... is a loopback, private, link-local or other internal address`, or deliveries fail with `webhook host ... resolves to ...` | The URL points at an internal address, and internal targets are not allowed. | Use a public address, or set `GOATFLOW_WEBHOOK_ALLOW_PRIVATE_TARGETS=true` on the app and every runner (see [Internal and private addresses](#internal-and-private-addresses)). |
| Receiver reports signature mismatch | The receiver signs parsed or re-encoded JSON, uses a different secret, or the receiver's framework changed the body. | Sign the raw body bytes. Compare with the whole header value including `sha256=`. If unsure of the secret, set a new one and update the receiver. |
| Deliveries `failed` with `endpoint returned HTTP 3xx` | Redirects are not followed. | Use the final URL (for example `https://` instead of `http://`). |
| Deliveries fail with `no response within N seconds` | Endpoint is slower than the timeout. | Raise the timeout (max 60) or answer with `2xx` before doing slow work. |
| Same delivery received twice | A retry after your endpoint timed out or returned non-2xx, or a runner stopped mid-send. | Ignore repeated `X-Webhook-Delivery` ids. |

## Database tables

Created by migration `000028_webhooks`; `000030_webhook_header_encryption` replaced the plain-text
`headers` column with encrypted header values (MySQL/MariaDB and PostgreSQL). Header values saved
by pre-release builds before `000030` are dropped and must be entered again.

| Table | Content |
|---|---|
| `gk_webhook` | Webhook settings. `secret_encrypted` holds the encrypted secret, `secret_hint` the last 4 characters, `headers_encrypted` the encrypted JSON object of custom header values, `header_hints` the JSON object of their hints, `valid_id` 1 = active, 2 = inactive. |
| `gk_webhook_delivery` | One row per delivery: payload, status, attempts, last response. Deleted with its webhook, and by [retention](#retention). |
| `gk_webhook_event_cursor` | Last processed id per event source (`ticket`, `article`, `ticket_history`). |

Count failed deliveries for one webhook:

```sql
SELECT COUNT(*) FROM gk_webhook_delivery WHERE webhook_id = ? AND status = 'failed';
```
