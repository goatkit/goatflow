# GoatFlow TypeScript SDK

The official TypeScript/JavaScript SDK for the GoatFlow ticketing system API.

## Installation

```bash
npm install @goatflow/sdk
# or
yarn add @goatflow/sdk
# or
pnpm add @goatflow/sdk
```

## Quick Start

```typescript
import { GoatflowClient } from '@goatflow/sdk';

// Create client with API key
const client = GoatflowClient.withApiKey('https://your-goatflow-instance.com', 'your-api-key');

// List tickets
const tickets = await client.tickets.list({
  page_size: 10,
  status: ['open'],
});

console.log(`Found ${tickets.total_count} tickets`);
```

## Authentication

### API Key (Recommended for server-to-server)

```typescript
const client = GoatflowClient.withApiKey('https://goatflow.example.com', 'your-api-key');
```

### JWT Token

```typescript
const expiresAt = new Date(Date.now() + 24 * 60 * 60 * 1000); // 24 hours
const client = GoatflowClient.withJWT(
  'https://goatflow.example.com',
  'jwt-token',
  'refresh-token',
  expiresAt
);
```

### OAuth2

```typescript
const client = GoatflowClient.withOAuth2(
  'https://goatflow.example.com',
  'access-token',
  'refresh-token',
  expiresAt
);
```

### Login Flow

```typescript
// Create client without auth
const client = new GoatflowClient({ baseURL: 'https://goatflow.example.com' });

// Login with credentials
await client.login('user@example.com', 'password');

// Now the client is authenticated and tokens will be managed automatically
const profile = await client.auth.getProfile();
```

### Custom Configuration

```typescript
const client = new GoatflowClient({
  baseURL: 'https://goatflow.example.com',
  auth: {
    type: 'api-key',
    apiKey: 'your-api-key',
  },
  timeout: 30000,
  retries: 3,
  debug: true,
  userAgent: 'my-app/1.0.0',
});
```

## Features

### Ticket Management

```typescript
// Create ticket
const ticket = await client.tickets.create({
  title: 'New Issue',
  description: 'Something is broken',
  priority: 'high',
  queue_id: 1,
  customer_id: 123,
  tags: ['bug', 'urgent'],
});

// Get ticket
const ticket = await client.tickets.get(ticketId);

// Update ticket
const updatedTicket = await client.tickets.update(ticketId, {
  status: 'in-progress',
  priority: 'urgent',
});

// Search tickets
const results = await client.tickets.search('error', {
  priority: ['high', 'urgent'],
  page_size: 20,
});

// Close ticket
const closedTicket = await client.tickets.close(ticketId, 'Issue resolved');

// Assign ticket
const assignedTicket = await client.tickets.assign(ticketId, userId);
```

### Messages and Attachments

```typescript
// Add message
const message = await client.tickets.addMessage(ticketId, {
  content: 'This is a response',
  is_internal: false,
});

// Get messages
const messages = await client.tickets.getMessages(ticketId);

// Upload attachment (Node.js)
const fileBuffer = fs.readFileSync('document.pdf');
const attachment = await client.tickets.uploadAttachment(ticketId, fileBuffer, 'document.pdf');

// Upload attachment (Browser)
const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement;
const file = fileInput.files[0];
const attachment = await client.tickets.uploadAttachment(ticketId, file);

// Download attachment
const data = await client.tickets.downloadAttachment(ticketId, attachmentId);
```

### User Management

```typescript
// List users
const users = await client.users.list();

// Create user
const user = await client.users.create({
  email: 'user@example.com',
  first_name: 'John',
  last_name: 'Doe',
  role: 'agent',
  password: 'secure-password',
});

// Get current user profile
const profile = await client.auth.getProfile();

// Update profile
const updatedProfile = await client.auth.updateProfile({
  first_name: 'Jane',
  title: 'Senior Support Agent',
});
```

### Dashboard & Analytics

```typescript
// Get dashboard statistics
const stats = await client.dashboard.getStats();
console.log({
  totalTickets: stats.total_tickets,
  openTickets: stats.open_tickets,
  myTickets: stats.my_tickets,
  ticketsByStatus: stats.tickets_by_status,
});

// Get my assigned tickets
const myTickets = await client.dashboard.getMyTickets();

// Get recent tickets
const recentTickets = await client.dashboard.getRecentTickets();
```

### Real-time Events

```typescript
// Connect to real-time events
await client.events.connect();

// Listen for ticket events
const ticketSubscription = client.events.onTicket((event) => {
  console.log(`Ticket ${event.type}:`, event.data.ticket_number);
});

// Listen for message events
const messageSubscription = client.events.onMessage((event) => {
  console.log(`New message on ticket ${event.data.ticket_id}`);
});

// Listen for all events
const allEventsSubscription = client.events.onAny((event) => {
  console.log(`Event: ${event.type}`, event.data);
});

// Handle connection events
const connectionSubscription = client.events.onConnection({
  connected: () => console.log('Connected to real-time events'),
  disconnected: (event) => console.log('Disconnected:', event.reason),
  error: (error) => console.error('WebSocket error:', error),
});

// Cleanup
ticketSubscription.unsubscribe();
messageSubscription.unsubscribe();
allEventsSubscription.unsubscribe();
connectionSubscription.unsubscribe();

// Disconnect
client.events.disconnect();
```

### LDAP Integration

```typescript
// Sync users from LDAP
const result = await client.ldap.syncUsers();
console.log(`Synced ${result.users_created} new users`);

// Get LDAP users
const ldapUsers = await client.ldap.getUsers();

// Test LDAP connection
await client.ldap.testConnection();

// Get sync status
const status = await client.ldap.getSyncStatus();
```

### Webhooks

```typescript
// Create webhook
const webhook = await client.webhooks.create({
  name: 'My Webhook',
  url: 'https://example.com/webhook',
  events: ['ticket.created', 'ticket.updated'],
  secret: 'webhook-secret',
});

// Test webhook
await client.webhooks.test(webhook.id);

// Get webhook deliveries
const deliveries = await client.webhooks.getDeliveries(webhook.id);
```

### Internal Notes

```typescript
// Create note
const note = await client.notes.createNote(ticketId, {
  content: 'Internal investigation notes',
  category: 'Investigation',
  is_important: true,
  tags: ['investigation', 'priority'],
});

// Get note templates
const templates = await client.notes.getTemplates();

// Create note from template
const template = templates.find(t => t.name === 'Investigation Started');
const noteFromTemplate = await client.notes.createNote(ticketId, {
  content: template.content.replace('{{time}}', new Date().toISOString()),
  category: template.category,
  is_important: template.is_important,
});
```

## Error Handling

The SDK provides structured error handling with specific error types:

```typescript
import {
  isNotFoundError,
  isUnauthorizedError,
  isForbiddenError,
  isRateLimitError,
  isValidationError,
  isNetworkError,
  isTimeoutError,
} from '@goatflow/sdk';

try {
  const ticket = await client.tickets.get(ticketId);
} catch (error) {
  if (isNotFoundError(error)) {
    console.log('Ticket not found');
  } else if (isUnauthorizedError(error)) {
    console.log('Authentication failed');
  } else if (isForbiddenError(error)) {
    console.log('Permission denied');
  } else if (isRateLimitError(error)) {
    console.log('Rate limit exceeded');
  } else if (isValidationError(error)) {
    console.log('Validation error:', error.field, error.message);
  } else if (isNetworkError(error)) {
    console.log('Network error:', error.operation, error.url);
  } else if (isTimeoutError(error)) {
    console.log('Request timeout:', error.timeout);
  } else {
    console.log('Unknown error:', error);
  }
}
```

## Pagination

Most list operations support pagination:

```typescript
// Basic pagination
const tickets = await client.tickets.list({
  page: 1,
  page_size: 50,
});

console.log(`Page ${tickets.page} of ${tickets.total_pages}`);
console.log(`Total tickets: ${tickets.total_count}`);

// Iterate through all pages
let page = 1;
let allTickets = [];

while (true) {
  const response = await client.tickets.list({
    page,
    page_size: 100,
  });
  
  allTickets.push(...response.tickets);
  
  if (page >= response.total_pages) {
    break;
  }
  
  page++;
}

console.log(`Loaded ${allTickets.length} tickets total`);
```

## TypeScript Support

The SDK is written in TypeScript and provides full type safety:

```typescript
import { Ticket, TicketCreateRequest, TicketListOptions } from '@goatflow/sdk';

// Type-safe ticket creation
const ticketData: TicketCreateRequest = {
  title: 'New Issue',
  description: 'Description here',
  priority: 'high', // TypeScript will enforce valid values
  queue_id: 1,
  customer_id: 123,
};

// Type-safe response handling
const ticket: Ticket = await client.tickets.create(ticketData);

// Type-safe filtering options
const options: TicketListOptions = {
  status: ['open', 'in-progress'], // TypeScript autocomplete
  priority: ['high', 'urgent'],
  page_size: 25,
};
```

## Browser Support

The SDK works in both Node.js and browser environments:

### Browser Usage

```html
<script type="module">
  import { GoatflowClient } from 'https://unpkg.com/@goatflow/sdk@latest/dist/index.esm.js';
  
  const client = GoatflowClient.withApiKey('https://your-goatflow.com', 'api-key');
  const tickets = await client.tickets.list();
  console.log(tickets);
</script>
```

### Node.js Usage

```javascript
// CommonJS
const { GoatflowClient } = require('@goatflow/sdk');

// ES Modules
import { GoatflowClient } from '@goatflow/sdk';
```

## Rate Limiting

The SDK automatically handles rate limiting with exponential backoff:

```typescript
// Configure retry behavior
const client = new GoatflowClient({
  baseURL: 'https://goatflow.example.com',
  auth: { type: 'api-key', apiKey: 'your-key' },
  retries: 5, // Retry up to 5 times
  timeout: 30000, // 30 second timeout
});
```

## Concurrent Operations

The SDK is designed for concurrent use:

```typescript
// Perform multiple operations in parallel
const [stats, tickets, users] = await Promise.all([
  client.dashboard.getStats(),
  client.tickets.list({ page_size: 10 }),
  client.users.list(),
]);

// Create multiple tickets concurrently
const ticketPromises = Array.from({ length: 5 }, (_, i) =>
  client.tickets.create({
    title: `Ticket ${i + 1}`,
    description: `Description ${i + 1}`,
    queue_id: 1,
    customer_id: 1,
  })
);

const createdTickets = await Promise.all(ticketPromises);
console.log(`Created ${createdTickets.length} tickets`);
```

## Testing

```bash
# Install dependencies
npm install

# Run tests
npm test

# Run tests with coverage
npm run test:coverage

# Run tests in watch mode
npm run test:watch
```

For integration tests:

```bash
export GOATFLOW_BASE_URL="https://your-test-instance.com"
export GOATFLOW_API_KEY="your-test-api-key"
npm run test:integration
```

## Examples

See the `examples/` directory for complete working examples:

- `basic-usage.ts` - Basic CRUD operations
- `authentication.ts` - Different authentication methods
- `real-time-events.ts` - WebSocket event handling
- `file-upload.ts` - File attachment handling
- `error-handling.ts` - Comprehensive error handling
- `advanced-features.ts` - LDAP, webhooks, and more

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/new-feature`
3. Add tests for new functionality
4. Ensure all tests pass: `npm test`
5. Build the project: `npm run build`
6. Submit a pull request

## Development

```bash
# Install dependencies
npm install

# Start development mode with hot reload
npm run dev

# Type checking
npm run typecheck

# Linting
npm run lint
npm run lint:fix

# Build for production
npm run build

# Generate documentation
npm run docs
```

## License

MIT License - see LICENSE file for details.

## Support

- Documentation: https://docs.goatflow.io/sdk/typescript
- Issues: https://github.com/goatkit/goatflow/issues
- Discussions: https://github.com/goatkit/goatflow/discussions