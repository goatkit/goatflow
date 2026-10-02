# GoatFlow Python SDK

Async Python client for the GoatFlow REST API (`/api/v1`), built on httpx and Pydantic v2.

## Installation

```bash
pip install goatflow-sdk
```

## Quick start

```python
import asyncio
from goatflow_sdk import GoatflowClient

async def main() -> None:
    async with GoatflowClient.with_api_key("https://goatflow.example.com", "gf_...") as gf:
        tickets = await gf.tickets.list(status="open", per_page=20)
        print(f"{tickets.pagination.total} open tickets")
        for t in tickets.tickets:
            print(f"#{t.ticket_number} {t.title} ({t.state_name})")

asyncio.run(main())
```

## Authentication

Both kinds of credential are sent as `Authorization: Bearer <token>`.

```python
# API token (gf_...), from the API Tokens settings page or POST /api/v1/tokens
gf = GoatflowClient.with_api_key(base_url, "gf_...")

# JWT: login() switches the client to the returned access token
gf = GoatflowClient(base_url)
pair = await gf.login("agent@example.com", "password")

# or resume from a stored pair
gf = GoatflowClient.with_jwt(base_url, access_token, refresh_token, expires_at)

# explicit refresh: new access token and a rotated refresh token
pair = await gf.auth.refresh(pair.refresh_token)
```

A client from `login()` or `with_jwt()` renews its token pair through `POST /api/v1/auth/refresh` when the access token is within a minute of expiry; concurrent requests share one refresh. A rejected refresh token raises `UnauthorizedError`. For your own renewal logic use `JWTAuth` with a `refresh_function` returning `(access_token, refresh_token, expires_at)`.

## Services

```python
from goatflow_sdk import ArticleCreateRequest, SearchQuery, TicketCreateRequest, TicketUpdateRequest

# Tickets
page = await gf.tickets.list(queue_id=3, status="open", include=["article_count"])
ticket = await gf.tickets.get(123)
created = await gf.tickets.create(TicketCreateRequest(title="Printer on fire", queue_id=3, body="Smoke everywhere"))
# State ids differ between installations; resolve them by name.
states = await gf.http.get("/api/v1/states")
closed_id = next(s["id"] for s in states if s["name"] == "closed successful")
updated = await gf.tickets.update(created.id, TicketUpdateRequest(state_id=closed_id))
reopened = await gf.tickets.reopen(created.id, "Customer replied")
await gf.tickets.delete(created.id)

# Articles
articles = await gf.articles.list(ticket.id, include_attachments=True)
note = await gf.articles.create(ticket.id, ArticleCreateRequest(body="Call back", article_type="note-internal"))

# Users, queues, statistics, search
me = await gf.users.me()
agents = await gf.users.list(search="smith", valid="1")
queues = await gf.queues.list(include_stats=True)
stats = await gf.statistics.dashboard()
results = await gf.search.query(SearchQuery(query="printer", types=["ticket"]))

# Webhooks (admin only)
from goatflow_sdk import WebhookRequest
hook = await gf.webhooks.create(WebhookRequest(name="My Webhook", url="https://example.com/hook",
                                               events=["ticket.created"], secret="a-signing-secret-of-16+-chars"))
delivery = await gf.webhooks.test(hook.id)

# Any other endpoint, with the same envelope and error handling
priorities = await gf.http.get("/api/v1/priorities")
```

## Errors

Error responses (HTTP status outside 2xx, or `{"success": false}`) raise `GoatflowError` subclasses with `status_code`, `message` and, when the API sent one, `code`: `UnauthorizedError` (401), `ForbiddenError` (403), `NotFoundError` (404), `RateLimitError` (429), `ServerError` (5xx). `NetworkError`/`TimeoutError` mean no HTTP response; `ResponseShapeError` means a 2xx body did not match the model.

## Development

```bash
python3 -m pytest    # from sdk/python; uses the response bodies in ../testdata
```
