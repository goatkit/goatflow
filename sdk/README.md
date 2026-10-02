# GoatFlow SDKs

Client libraries for the GoatFlow REST API (`/api/v1`).

| SDK | Path | Package | Runtime |
|-----|------|---------|---------|
| Go | [`go/`](go/) | `github.com/goatkit/goatflow/sdk/go` | Go 1.23+ |
| TypeScript/JavaScript | [`typescript/`](typescript/) | `@goatflow/sdk` (ESM) | Node 18+, Bun, browsers (`fetch`) |
| Python | [`python/`](python/) | `goatflow-sdk` | Python 3.8+, asyncio (httpx, Pydantic v2) |

## Quick start

```go
gf := client.NewClientWithAPIKey("https://goatflow.example.com", "gf_...")
tickets, err := gf.Tickets.List(ctx, &types.TicketListOptions{Status: "open"})
```

```typescript
const gf = GoatflowClient.withApiKey('https://goatflow.example.com', 'gf_...');
const tickets = await gf.tickets.list({ status: 'open' });
```

```python
async with GoatflowClient.with_api_key("https://goatflow.example.com", "gf_...") as gf:
    tickets = await gf.tickets.list(status="open")
```

## Authentication

Every SDK sends `Authorization: Bearer <token>` with one of:

- **API token** (`gf_...`), created on the API Tokens settings page (`/settings/tokens`) or via `POST /api/v1/tokens`. Recommended for integrations.
- **JWT access token** from `POST /api/v1/auth/login` (`login` + `password`). Clients created by login (or from a stored token pair) renew it through `POST /api/v1/auth/refresh` before it expires; each refresh rotates the refresh token.

## Responses and errors

Most endpoints answer `{"success": true, "data": ...}` (paginated lists add `"pagination"`); a few answer `{"success": true, ...fields}` or a bare object. The SDKs unwrap all three and return the payload. Errors (`{"error": "message"}` or `{"error": {"code", "message"}}`, and `{"success": false}` even with HTTP 200) become a typed error carrying the HTTP status, the message and the code when present. A redirect (e.g. to the login page) is reported as an error instead of being followed.

## Coverage

| Area | Endpoints |
|------|-----------|
| Tickets | `GET/POST /tickets`, `GET/PUT/DELETE /tickets/:id`, `POST /tickets/:id/reopen` |
| Articles | `GET/POST /tickets/:id/articles`, `GET/PUT/DELETE /tickets/:id/articles/:article_id` |
| Users (agents) | `GET/POST /users`, `GET /users/me`, `GET/PUT/DELETE /users/:id` |
| Queues | `GET /queues`, `GET /queues/:id` |
| Statistics | `GET /statistics/dashboard` |
| Search | `POST /search` |
| Webhooks (admin) | `GET/POST /webhooks`, `GET/PUT/DELETE /webhooks/:id`, `POST /webhooks/:id/test`, `GET /webhooks/:id/deliveries`, `GET /webhooks/deliveries/:id`, `POST /webhooks/deliveries/:id/redeliver` |
| Auth | `POST /auth/login`, `POST /auth/refresh` |
| Health | `GET /health` (outside `/api/v1`) |

Other `/api/v1` endpoints (lookups, custom fields, organisations, tokens, ...) can be called through each client's generic request methods, which apply the same envelope and error handling.

## Testing

All three test suites run against the same response bodies in [`testdata/`](testdata/): captured from a running server for read endpoints, the handler's JSON literal for endpoints that change data.

```bash
make test-sdk-go                                   # go vet + go test + build examples (toolbox)
cd sdk/typescript && npm install --ignore-scripts && npm run typecheck && bun test
cd sdk/python && python3 -m pytest                 # needs httpx, pydantic, pytest
```

## License

SDK packages are released under the MIT License; the GoatFlow platform itself is licensed under Apache-2.0.
