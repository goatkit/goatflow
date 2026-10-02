// Walks through the GoatFlow TypeScript SDK against a live server:
//
//   GOATFLOW_URL=https://goatflow.example.com GOATFLOW_TOKEN=gf_... bun examples/basic-usage.ts
//
// Set GOATFLOW_QUEUE_ID to also create a ticket in that queue, add a note to
// it and close it again.
import { GoatflowClient, isNotFoundError } from '../src/index.js';

const baseURL = process.env.GOATFLOW_URL;
const token = process.env.GOATFLOW_TOKEN;
if (!baseURL || !token) {
  throw new Error('set GOATFLOW_URL and GOATFLOW_TOKEN (an API token, gf_...)');
}
const client = GoatflowClient.withApiKey(baseURL, token);

const health = await client.health();
console.log(`GoatFlow ${health.version} is ${health.status}`);

const me = await client.users.me();
console.log(`Authenticated as ${me.login} (${me.first_name} ${me.last_name})`);

const tickets = await client.tickets.list({ per_page: 5, status: 'open' });
console.log(`${tickets.pagination.total} open tickets, showing ${tickets.tickets.length}:`);
for (const t of tickets.tickets) {
  console.log(`  #${t.ticket_number} ${t.title} [${t.queue_name}, ${t.state_name}, ${t.priority_name}]`);
}

const queues = await client.queues.list();
console.log(`${queues.length} queues readable`);

try {
  await client.tickets.get(999999999);
} catch (error) {
  if (!isNotFoundError(error)) throw error;
  console.log(`Ticket 999999999 does not exist (${error.statusCode} ${error.message})`);
}

const queueId = Number(process.env.GOATFLOW_QUEUE_ID ?? 0);
if (queueId > 0) {
  const created = await client.tickets.create({
    title: 'SDK example ticket',
    queue_id: queueId,
    body: 'Created by the GoatFlow TypeScript SDK example.',
  });
  console.log(`Created ticket #${created.tn} (id ${created.id})`);

  const note = await client.articles.create(created.id, {
    subject: 'Internal note',
    body: 'Added by the SDK example.',
    article_type: 'note-internal',
  });
  console.log(`Added article ${note.id} (${note.article_type})`);

  // State ids differ between installations; resolve the state by name.
  const states = await client.http.get<Array<{ id: number; name: string }>>('/api/v1/states');
  const closed = states.find((s) => s.name === 'closed successful');
  if (!closed) throw new Error('no "closed successful" ticket state');
  const updated = await client.tickets.update(created.id, { state_id: closed.id });
  console.log(`Ticket ${updated.id} now in state ${updated.state_id}`);
}
