# GoatFlow TypeScript SDK

TypeScript/JavaScript client for the GoatFlow REST API (`/api/v1`). ESM, no runtime dependencies; uses `fetch` (Node 18+, Bun, browsers).

## Installation

```bash
npm install @goatflow/sdk
```

## Quick start

```typescript
import { GoatflowClient } from '@goatflow/sdk';

const gf = GoatflowClient.withApiKey('https://goatflow.example.com', 'gf_...');

const tickets = await gf.tickets.list({ status: 'open', per_page: 20 });
console.log(`${tickets.pagination.total} open tickets`);
for (const t of tickets.tickets) {
  console.log(`#${t.ticket_number} ${t.title} (${t.state_name})`);
}
```

A runnable walkthrough is in [`examples/basic-usage.ts`](examples/basic-usage.ts):

```bash
GOATFLOW_URL=https://goatflow.example.com GOATFLOW_TOKEN=gf_... bun examples/basic-usage.ts
```

## Authentication

Both kinds of credential are sent as `Authorization: Bearer <token>`.

```typescript
// API token (gf_...), from the API Tokens settings page or POST /api/v1/tokens
const gf = GoatflowClient.withApiKey(baseURL, 'gf_...');

// JWT: login() switches the client to the returned access token
const gf = new GoatflowClient({ baseURL });
const pair = await gf.login('agent@example.com', 'password');

// or resume from a stored pair
const gf = GoatflowClient.withJWT(baseURL, accessToken, refreshToken, expiresAt);

// explicit refresh: new access token and a rotated refresh token
const next = await gf.auth.refresh(pair.refresh_token);
```

A client from `login()`, `withJWT()` or `useJWT()` renews its token pair through `POST /api/v1/auth/refresh` when the access token is within a minute of expiry; concurrent requests share one refresh. A rejected refresh token (401) fails the request. For your own renewal logic pass a `refreshFunction` via `setAuth({ type: 'jwt', ... })`.

## Configuration

```typescript
const gf = new GoatflowClient({
  baseURL: 'https://goatflow.example.com',
  auth: { type: 'api-key', apiKey: 'gf_...' },
  timeout: 30000, // ms, default
  userAgent: 'my-app/1.0',
  fetch: customFetch, // optional, defaults to globalThis.fetch
});
```

## Services

### Tickets

```typescript
const list = await gf.tickets.list({
  queue_id: 3,
  status: 'open',
  sort: 'updated',
  order: 'desc',
  include: ['article_count', 'last_article'],
});
const ticket = await gf.tickets.get(123);

const created = await gf.tickets.create({ title: 'Printer on fire', queue_id: 3, body: 'Smoke everywhere', priority_id: 4 });
// State ids differ between installations; resolve them by name.
const states = await gf.http.get<Array<{ id: number; name: string }>>('/api/v1/states');
const closed = states.find((s) => s.name === 'closed successful')!;
const updated = await gf.tickets.update(created.id, { state_id: closed.id });
const reopened = await gf.tickets.reopen(created.id, 'Customer replied');
await gf.tickets.delete(created.id);
```

### Articles

```typescript
const { articles, total } = await gf.articles.list(ticketId, true); // with attachment lists
const note = await gf.articles.create(ticketId, {
  subject: 'Call back',
  body: 'Customer asked for a call',
  article_type: 'note-internal',
});
await gf.articles.update(ticketId, note.id, { body: 'Corrected text' });
await gf.articles.delete(ticketId, note.id);
```

### Users, queues, statistics, search

```typescript
const me = await gf.users.me();
const agents = await gf.users.list({ search: 'smith', valid: '1' });
const agent = await gf.users.get(5);

const queues = await gf.queues.list({ include_stats: true });
const queue = await gf.queues.get(3);

const stats = await gf.statistics.dashboard();

const results = await gf.search.query({ query: 'printer', types: ['ticket'] });
```

### Webhooks

Admin only. Deliveries carry `X-Webhook-Event`, `X-Webhook-Delivery` and, with a
secret, `X-Webhook-Signature: sha256=<hex HMAC-SHA256 of the raw body>`.

```typescript
// Create webhook (signed)
const webhook = await gf.webhooks.create({
  name: 'My Webhook',
  url: 'https://example.com/webhook',
  events: ['ticket.created', 'ticket.closed', 'article.created'],
  secret: 'a-signing-secret-of-16+-chars',
});

// Send a webhook.test event now
const delivery = await gf.webhooks.test(webhook.id);

// Delivery log, one delivery with payload/response, and redelivery
const deliveries = await gf.webhooks.getDeliveries(webhook.id);
const detail = await gf.webhooks.getDelivery(deliveries[0].id);
const again = await gf.webhooks.redeliver(detail.id);
```

### Other endpoints

`gf.http` applies the same envelope and error handling to any path:

```typescript
const priorities = await gf.http.get<Array<{ id: number; name: string }>>('/api/v1/priorities');
```

## Errors

Error responses (HTTP status outside 2xx, or `{"success": false}`) throw `GoatflowError` with `statusCode`, `message` and, when the API sent one, `code` (e.g. `core:invalid_token`). Requests without an HTTP response throw `NetworkError` or `TimeoutError` (both extend `GoatflowError`).

```typescript
import { GoatflowError, isNotFoundError } from '@goatflow/sdk';

try {
  await gf.tickets.get(123);
} catch (error) {
  if (isNotFoundError(error)) {
    // 404
  } else if (error instanceof GoatflowError) {
    console.error(error.statusCode, error.message, error.code);
  }
}
```

## Development

```bash
npm install --ignore-scripts
npm run typecheck   # tsc --noEmit (src, examples, tests)
npm test            # bun test, against the response bodies in ../testdata
npm run build       # ESM + .d.ts into dist/
```
