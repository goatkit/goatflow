import { afterEach, describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { GoatflowClient, GoatflowError, NetworkError, isNotFoundError, isUnauthorizedError } from '../src/index.js';

// Response bodies shared with the Go and Python SDK tests (sdk/testdata):
// captured from a running server for GET endpoints, the handler's JSON literal
// for endpoints that change data.
const fixture = (name: string): string => readFileSync(join(import.meta.dir, '..', '..', 'testdata', `${name}.json`), 'utf8');

interface Recorded {
  method: string;
  path: string;
  query: string;
  authorization: string | null;
  contentType: string | null;
  body: unknown;
}

let server: Bun.Server<undefined> | undefined;

afterEach(() => {
  server?.stop(true);
  server = undefined;
});

/** Starts a server answering every request with status and body; returns a client and the last request. */
function serve(status: number, body: string, headers: Record<string, string> = {}): { client: GoatflowClient; last: Recorded } {
  const last: Recorded = { method: '', path: '', query: '', authorization: null, contentType: null, body: undefined };
  server = Bun.serve({
    port: 0,
    async fetch(request) {
      const url = new URL(request.url);
      const text = await request.text();
      Object.assign(last, {
        method: request.method,
        path: url.pathname,
        query: url.search,
        authorization: request.headers.get('authorization'),
        contentType: request.headers.get('content-type'),
        body: text ? JSON.parse(text) : undefined,
      });
      return new Response(body === '' ? null : body, {
        status,
        headers: { 'Content-Type': 'application/json; charset=utf-8', ...headers },
      });
    },
  });
  return { client: GoatflowClient.withApiKey(`http://localhost:${server.port}`, 'gf_test_token'), last };
}

/** Starts a server answering by path ([status, body]); returns the log of every request. */
function serveRoutes(routes: Record<string, [number, string]>): Recorded[] {
  const log: Recorded[] = [];
  server = Bun.serve({
    port: 0,
    async fetch(request) {
      const url = new URL(request.url);
      const text = await request.text();
      log.push({
        method: request.method,
        path: url.pathname,
        query: url.search,
        authorization: request.headers.get('authorization'),
        contentType: request.headers.get('content-type'),
        body: text ? JSON.parse(text) : undefined,
      });
      const [status, body] = routes[url.pathname] ?? [404, '{"error":"no such route in test"}'];
      return new Response(body, { status, headers: { 'Content-Type': 'application/json; charset=utf-8' } });
    },
  });
  return log;
}

describe('envelope handling', () => {
  test('tickets.list unwraps data and pagination and sends filters', async () => {
    const { client, last } = serve(200, fixture('ticket_list'));

    const list = await client.tickets.list({ per_page: 2, status: 'open', queue_id: 37, include: ['article_count', 'last_article'] });

    expect(last.method).toBe('GET');
    expect(last.path).toBe('/api/v1/tickets');
    expect(last.query).toBe('?per_page=2&status=open&queue_id=37&include=article_count%2Clast_article');
    expect(last.authorization).toBe('Bearer gf_test_token');
    expect(list.tickets).toHaveLength(1);
    expect(list.tickets[0]).toMatchObject({ id: 37558, ticket_number: 'COACH-9664946', state_name: 'new', responsible_user_id: 14 });
    expect(list.pagination).toEqual({ page: 1, per_page: 2, total: 37445, total_pages: 18723, has_next: true, has_prev: false });
  });

  test('tickets.get unwraps data', async () => {
    const { client, last } = serve(200, fixture('ticket_get'));

    const ticket = await client.tickets.get(37558);

    expect(last.path).toBe('/api/v1/tickets/37558');
    expect(ticket).toMatchObject({ id: 37558, title: 'E2E identity check', state: 'new', queue: 'Coaching', article_count: 1 });
    expect(ticket.type_id).toBeUndefined();
  });

  test('error envelope becomes GoatflowError with status', async () => {
    const { client } = serve(404, fixture('ticket_not_found'));

    const error = await client.tickets.get(999999999).catch((e: unknown) => e);

    expect(error).toBeInstanceOf(GoatflowError);
    expect(error).toMatchObject({ statusCode: 404, message: 'Ticket not found', code: undefined });
    expect(isNotFoundError(error)).toBe(true);
  });

  test('structured auth error keeps code and message', async () => {
    const { client } = serve(401, fixture('invalid_token'));

    const error = await client.users.me().catch((e: unknown) => e);

    expect(error).toMatchObject({ statusCode: 401, code: 'core:invalid_token', message: 'Invalid or malformed token' });
    expect(isUnauthorizedError(error)).toBe(true);
  });

  test('success:false with HTTP 200 is an error', async () => {
    const { client } = serve(200, '{"success":false,"error":"Database unavailable"}');

    await expect(client.queues.get(1)).rejects.toMatchObject({ statusCode: 200, message: 'Database unavailable' });
  });

  test('redirect to the login page is not followed', async () => {
    const { client } = serve(303, '<a href="/login">See Other</a>.', { Location: '/login', 'Content-Type': 'text/html' });

    await expect(client.tickets.list()).rejects.toMatchObject({ statusCode: 303, body: '<a href="/login">See Other</a>.' });
  });

  test('bare article list is returned whole', async () => {
    const { client, last } = serve(200, fixture('article_list'));

    const list = await client.articles.list(37558, true);

    expect(last.path).toBe('/api/v1/tickets/37558/articles');
    expect(last.query).toBe('?include_attachments=true');
    expect(list.total).toBe(1);
    expect(list.articles[0]).toMatchObject({ id: 91, article_type: 'note-internal', is_visible_for_customer: false });
    expect(list.articles[0]?.attachments?.[0]).toMatchObject({ filename: 'log.txt', size: 120 });
  });

  test('reopen returns the top-level fields of a data-less success response', async () => {
    const { client, last } = serve(200, fixture('reopen'));

    const result = await client.tickets.reopen(37558, 'Customer replied');

    expect(last.path).toBe('/api/v1/tickets/37558/reopen');
    expect(last.body).toEqual({ reason: 'Customer replied' });
    expect(result).toMatchObject({ id: 37558, state_id: 4, state: 'open' });
  });

  test('delete accepts 204 No Content', async () => {
    const { client, last } = serve(204, '');

    await expect(client.tickets.delete(37558)).resolves.toBeUndefined();
    expect(last.method).toBe('DELETE');
  });
});

describe('endpoints', () => {
  test('tickets.create posts JSON and returns the created ticket', async () => {
    const { client, last } = serve(201, fixture('ticket_created'));

    const created = await client.tickets.create({ title: 'Printer on fire', queue_id: 37, body: 'Smoke everywhere' });

    expect(last.method).toBe('POST');
    expect(last.contentType).toBe('application/json');
    expect(last.body).toEqual({ title: 'Printer on fire', queue_id: 37, body: 'Smoke everywhere' });
    expect(created).toEqual({ id: 37559, tn: '2026100110000017', title: 'Printer on fire', queue_id: 37, ticket_state_id: 1, ticket_priority_id: 3 });
  });

  test('users.me and queues.get', async () => {
    let { client, last } = serve(200, fixture('user_me'));
    const me = await client.users.me();
    expect(last.path).toBe('/api/v1/users/me');
    expect(me).toMatchObject({ id: 1, login: 'root@localhost', active: true });
    expect(me.groups).toHaveLength(2);
    server?.stop(true);

    ({ client, last } = serve(200, fixture('queue_get')));
    const queue = await client.queues.get(37);
    expect(last.path).toBe('/api/v1/queues/37');
    expect(queue).toMatchObject({ name: 'Coaching', comments: 'Coaching engagement queue (GoatCoach)', signature_id: 1 });
  });

  test('statistics.dashboard', async () => {
    const { client, last } = serve(200, fixture('dashboard'));

    const stats = await client.statistics.dashboard();

    expect(last.path).toBe('/api/v1/statistics/dashboard');
    expect(stats.overview.total_tickets).toBe(12);
    expect(stats.by_queue[0]).toEqual({ queue_id: 37, queue_name: 'Coaching', count: 9 });
    expect(stats.recent_activity[0]?.ticket_tn).toBe('COACH-9664946');
  });

  test('search.query posts the query', async () => {
    const { client, last } = serve(200, fixture('search_unavailable'));

    const results = await client.search.query({ query: 'printer', limit: 5 });

    expect(last.method).toBe('POST');
    expect(last.path).toBe('/api/v1/search');
    expect(last.body).toEqual({ query: 'printer', limit: 5 });
    expect(results).toMatchObject({ total_hits: 0, hits: [], warning: 'search backend unavailable' });
  });

  test('login switches the client to the access token', async () => {
    const { client, last } = serve(200, fixture('login'));
    client.setAuth(undefined);

    const response = await client.login('root@localhost', 'secret');

    expect(last.authorization).toBeNull();
    expect(last.body).toEqual({ login: 'root@localhost', password: 'secret' });
    expect(response.user.role).toBe('Admin');
    await client.users.me().catch(() => undefined);
    expect(last.authorization).toBe('Bearer eyJhbGciOiJIUzI1NiJ9.e30.sig');
  });

  test('failed login is a 401 error', async () => {
    const { client } = serve(401, fixture('login_failed'));

    await expect(client.auth.login('root@localhost', 'wrong')).rejects.toMatchObject({ statusCode: 401, message: 'Invalid credentials' });
  });

  test('auth.refresh posts the refresh token without credentials', async () => {
    const { client, last } = serve(200, fixture('refresh'));

    const pair = await client.auth.refresh('refresh-1');

    expect(last.method).toBe('POST');
    expect(last.path).toBe('/api/v1/auth/refresh');
    expect(last.authorization).toBeNull();
    expect(last.body).toEqual({ refresh_token: 'refresh-1' });
    expect(pair).toMatchObject({
      access_token: 'eyJhbGciOiJIUzI1NiJ9.e30.sig2',
      refresh_token: 'eyJhbGciOiJIUzI1NiJ9.e30.ref2',
      expires_in: 86400,
      refresh_expires_in: 604800,
    });
  });

  test('rejected refresh token is a 401 error', async () => {
    const { client } = serve(401, fixture('refresh_rejected'));

    await expect(client.auth.refresh('stale')).rejects.toMatchObject({ statusCode: 401, message: 'Invalid or expired refresh token' });
  });

  test('expired JWT is renewed once through /api/v1/auth/refresh', async () => {
    const log = serveRoutes({
      '/api/v1/auth/refresh': [200, fixture('refresh')],
      '/api/v1/users/me': [200, fixture('user_me')],
    });
    const client = GoatflowClient.withJWT(`http://localhost:${server?.port}`, 'expired', 'refresh-1', new Date(Date.now() - 3600_000));

    await Promise.all([client.users.me(), client.users.me()]);
    await client.users.me();

    expect(log.map((r) => r.path)).toEqual(['/api/v1/auth/refresh', '/api/v1/users/me', '/api/v1/users/me', '/api/v1/users/me']);
    expect(log[0]).toMatchObject({ authorization: null, body: { refresh_token: 'refresh-1' } });
    for (const r of log.slice(1)) expect(r.authorization).toBe('Bearer eyJhbGciOiJIUzI1NiJ9.e30.sig2');
  });

  test('rejected refresh fails the request without sending it', async () => {
    const log = serveRoutes({ '/api/v1/auth/refresh': [401, fixture('refresh_rejected')] });
    const client = GoatflowClient.withJWT(`http://localhost:${server?.port}`, 'expired', 'refresh-1', new Date(Date.now() - 3600_000));

    await expect(client.users.me()).rejects.toMatchObject({ statusCode: 401 });
    expect(log.map((r) => r.path)).toEqual(['/api/v1/auth/refresh']);
  });

  test('unreachable server is a NetworkError', async () => {
    const { client } = serve(200, '{}');
    const port = server?.port;
    server?.stop(true);
    server = undefined;

    const error = await client.tickets.get(1).catch((e: unknown) => e);

    expect(error).toBeInstanceOf(NetworkError);
    expect(error).toMatchObject({ method: 'GET', url: `http://localhost:${port}/api/v1/tickets/1` });
  });
});
