// Request and response shapes of the GoatFlow REST API (/api/v1). Field sets
// mirror what the handlers send; where two endpoints describe the same
// resource differently (ticket list rows vs. a single ticket) they get
// separate types. Timestamps are RFC 3339 strings.

/** The "pagination" object paginated list endpoints send next to "data". */
export interface Pagination {
  page: number;
  per_page: number;
  total: number;
  total_pages: number;
  has_next: boolean;
  has_prev: boolean;
}

export interface GroupRef {
  id: number;
  name: string;
}

// Tickets

/** One row of GET /api/v1/tickets. */
export interface TicketSummary {
  id: number;
  tn: string;
  ticket_number: string;
  title: string;
  queue_id: number;
  queue_name: string;
  state_id: number;
  state_name: string;
  priority_id: number;
  priority_name: string;
  customer_user_id: string;
  customer_id: string;
  /** Owner. */
  user_id: number;
  responsible_user_id: number | null;
  created_at: string;
  updated_at: string;
  /** Only with include: ['article_count']. */
  article_count?: number;
  /** Only with include: ['last_article']; null when the ticket has no article. */
  last_article?: { subject: string; created_at: string } | null;
}

export interface TicketListOptions {
  page?: number;
  /** 1-100, server default 20. */
  per_page?: number;
  /** "open", "closed", "pending" or an exact state name. */
  status?: string;
  queue_id?: number;
  priority_id?: number;
  customer_user_id?: string;
  /** Responsible agent. */
  assigned_user_id?: number;
  search?: string;
  sort?: 'created' | 'updated' | 'priority' | 'tn' | 'title';
  order?: 'asc' | 'desc';
  include?: Array<'article_count' | 'last_article'>;
}

export interface TicketList {
  tickets: TicketSummary[];
  pagination: Pagination;
}

/** GET /api/v1/tickets/:id. */
export interface Ticket {
  id: number;
  ticket_number: string;
  title: string;
  state_id: number;
  state: string;
  priority_id: number;
  priority: string;
  queue_id: number;
  queue: string;
  type_id?: number;
  customer_id?: string;
  customer_user_id?: string;
  owner_user_id: number;
  responsible_user_id?: number;
  article_count: number;
  create_time: string;
  change_time: string;
}

/** Body of POST /api/v1/tickets; body becomes the first article. */
export interface TicketCreateRequest {
  title: string;
  queue_id: number;
  body?: string;
  priority_id?: number;
  state_id?: number;
  type_id?: number;
  customer_email?: string;
  customer_id?: string;
  customer_user_id?: string;
}

/** Data of POST /api/v1/tickets. */
export interface CreatedTicket {
  id: number;
  tn: string;
  title: string;
  queue_id: number;
  ticket_state_id: number;
  ticket_priority_id: number;
}

/** Body of PUT /api/v1/tickets/:id; omitted fields are unchanged. */
export interface TicketUpdateRequest {
  title?: string;
  queue_id?: number;
  type_id?: number;
  state_id?: number;
  priority_id?: number;
  customer_user_id?: string;
  customer_id?: string;
  /** Owner. */
  user_id?: number;
  responsible_user_id?: number;
  ticket_lock_id?: number;
}

/** Data of PUT /api/v1/tickets/:id: the ticket row after the update. */
export interface TicketRecord {
  id: number;
  tn: string;
  title: string;
  queue_id: number;
  type_id: number;
  state_id: number;
  priority_id: number;
  user_id: number;
  responsible_user_id: number | null;
  ticket_lock_id: number;
  customer_user_id: string;
  customer_id: string;
  create_time: string;
  create_by: number;
  change_time: string;
  change_by: number;
}

/** Response of POST /api/v1/tickets/:id/reopen. */
export interface ReopenResult {
  id: number;
  state_id: number;
  state: string;
  reason: string;
  reopened_at: string;
}

// Articles

export interface ArticleAttachment {
  id: number;
  filename: string;
  content_type: string;
  size: number;
  disposition: string;
}

export interface Article {
  id: number;
  ticket_id: number;
  article_sender_type_id: number;
  sender_type?: string;
  communication_channel_id: number;
  is_visible_for_customer: boolean;
  /** e.g. "email-external", "note-internal", "phone". */
  article_type: string;
  from?: string;
  to?: string;
  cc?: string;
  subject: string;
  body: string;
  content_type: string;
  message_id?: string;
  create_time: string;
  create_by: number;
  change_time?: string;
  change_by?: number;
  /** Only when requested with includeAttachments. */
  attachments?: ArticleAttachment[];
}

/** GET /api/v1/tickets/:id/articles, newest first. */
export interface ArticleList {
  articles: Article[];
  total: number;
}

/** Body of POST /api/v1/tickets/:id/articles; body is required. */
export interface ArticleCreateRequest {
  subject?: string;
  body: string;
  content_type?: string;
  /** e.g. "note-internal" (alias "note"), "note-external", "email-external" (alias "email"), "phone". */
  article_type?: string;
  sender_type?: 'agent' | 'customer' | 'system';
  is_visible_for_customer?: boolean;
  from?: string;
  to?: string;
  cc?: string;
  time_unit?: number;
}

/** Body of PUT /api/v1/tickets/:id/articles/:aid; at least one field. */
export interface ArticleUpdateRequest {
  subject?: string;
  body?: string;
}

export interface ArticleUpdate {
  id: number;
  ticket_id: number;
  subject: string;
  body: string;
}

// Users

/** Group membership with permission keys (ro, move_into, create, note, owner, priority, rw). */
export interface UserGroup {
  id: number;
  name: string;
  permissions: string[];
}

/** An agent from GET /api/v1/users or /api/v1/users/:id. */
export interface User {
  id: number;
  login: string;
  first_name?: string;
  last_name?: string;
  valid_id: number;
  valid: boolean;
  create_time?: string;
  change_time?: string;
  groups: UserGroup[];
  /** Only from GET /api/v1/users/:id. */
  email?: string;
  /** Only from GET /api/v1/users/:id. */
  preferences?: Record<string, string>;
}

export interface UserListOptions {
  page?: number;
  per_page?: number;
  search?: string;
  /** "1" valid only, "2" invalid only. */
  valid?: '1' | '2';
  group_id?: number;
}

export interface UserList {
  users: User[];
  pagination: Pagination;
}

/** GET /api/v1/users/me. */
export interface CurrentUser {
  id: number;
  login: string;
  email: string;
  first_name: string;
  last_name: string;
  active: boolean;
  groups: GroupRef[];
}

/** Body of POST /api/v1/users; password needs 8+ characters, valid_id defaults to 1. */
export interface UserCreateRequest {
  login: string;
  email: string;
  password: string;
  first_name?: string;
  last_name?: string;
  valid_id?: number;
  /** Group IDs to add the user to. */
  groups?: number[];
}

/** Data of POST /api/v1/users. */
export interface CreatedUser {
  id: number;
  login: string;
  email: string;
  first_name?: string;
  last_name?: string;
  valid_id: number;
  valid: boolean;
  groups: number[];
  created_at: string;
}

/** Body of PUT /api/v1/users/:id; omitted fields are unchanged. */
export interface UserUpdateRequest {
  email?: string;
  first_name?: string;
  last_name?: string;
  password?: string;
  valid_id?: number;
}

// Queues

/**
 * A ticket queue. valid, comment, create_time, change_time and the ticket
 * counts are only sent by GET /api/v1/queues; comments, salutation_id and
 * signature_id only by GET /api/v1/queues/:id.
 */
export interface Queue {
  id: number;
  name: string;
  valid_id: number;
  valid?: boolean;
  group_id?: number;
  group_name?: string;
  groups: GroupRef[];
  system_address_id?: number;
  salutation_id?: number;
  signature_id?: number;
  unlock_timeout?: number;
  follow_up_id?: number;
  follow_up_lock?: number;
  comment?: string;
  comments?: string;
  create_time?: string;
  change_time?: string;
  /** Only with include_stats. */
  ticket_count?: number;
  open_tickets?: number;
  closed_tickets?: number;
  pending_tickets?: number;
}

export interface QueueListOptions {
  valid?: '1' | '2';
  include_stats?: boolean;
}

// Statistics

/** GET /api/v1/statistics/dashboard, limited to the caller's readable queues. */
export interface DashboardStatistics {
  overview: {
    total_tickets: number;
    open_tickets: number;
    closed_tickets: number;
    pending_tickets: number;
  };
  by_queue: Array<{ queue_id: number; queue_name: string; count: number }>;
  by_priority: Array<{ priority_id: number; priority_name: string; count: number }>;
  /** The ten newest tickets. */
  recent_activity: Array<{ type: string; ticket_id: number; ticket_tn: string; timestamp: string }>;
}

// Search

/** Body of POST /api/v1/search. */
export interface SearchQuery {
  query: string;
  /** Defaults to ticket, article and customer. */
  types?: string[];
  filters?: Record<string, string>;
  offset?: number;
  /** Default 20, max 100. */
  limit?: number;
  sort_by?: string;
  sort_order?: 'asc' | 'desc';
  highlight?: boolean;
  facets?: string[];
}

export interface SearchHit {
  id: string;
  type: string;
  score: number;
  title: string;
  content: string;
  highlights?: Record<string, string[]>;
  metadata: Record<string, unknown>;
}

/** Response of POST /api/v1/search; warning is set when the search backend is unavailable. */
export interface SearchResults {
  query?: string;
  total_hits: number;
  took_ms: number;
  hits: SearchHit[];
  facets?: Record<string, Array<{ value: string; count: number }>>;
  suggestions?: string[];
  warning?: string;
}

// Webhooks

export interface Webhook {
  id: number;
  name: string;
  url: string;
  events: string[];
  /** Header name -> masked hint; header values are write-only. */
  header_hints: Record<string, string>;
  has_secret: boolean;
  secret_hint?: string;
  retry_count: number;
  timeout_seconds: number;
  is_active: boolean;
  created_at: string;
  created_by: number;
  updated_at: string;
  updated_by: number;
}

export interface WebhookRequest {
  name?: string;
  url?: string;
  events?: string[];
  /** 16-512 characters; empty string removes the secret. */
  secret?: string;
  /** Replaces the custom headers; on update a null value keeps the stored value. */
  headers?: Record<string, string | null>;
  retry_count?: number;
  timeout_seconds?: number;
  is_active?: boolean;
}

export interface WebhookDelivery {
  id: number;
  webhook_id: number;
  event: string;
  status: 'pending' | 'delivering' | 'delivered' | 'failed';
  success: boolean;
  attempts: number;
  status_code: number | null;
  error?: string;
  duration_ms: number | null;
  next_attempt_at?: string;
  delivered_at?: string;
  /** Only on getDelivery. */
  payload?: string;
  /** Only on getDelivery; first 4 KiB of the response body. */
  response?: string;
  created_at: string;
  updated_at: string;
}

// Auth

/**
 * Response of POST /api/v1/auth/login and POST /api/v1/auth/refresh. Every
 * refresh rotates the refresh token.
 */
export interface TokenPair {
  success: true;
  user: {
    id: number;
    login: string;
    email: string;
    first_name: string;
    last_name: string;
    role: string;
  };
  access_token: string;
  refresh_token: string;
  token_type: string;
  /** Access token lifetime in seconds. */
  expires_in: number;
  /** Refresh token lifetime in seconds. */
  refresh_expires_in: number;
}

/** GET /health. */
export interface Health {
  status: string;
  components: Record<string, string>;
  version: string;
}

// Client configuration

export interface ClientConfig {
  /** Server root, e.g. "https://goatflow.example.com". */
  baseURL: string;
  auth?: AuthConfig;
  /** Per-request timeout in milliseconds; default 30000. */
  timeout?: number;
  userAgent?: string;
  /** fetch implementation; defaults to the global fetch (Node 18+, browsers). */
  fetch?: typeof fetch;
}

/**
 * Credentials, sent as "Authorization: Bearer <token>".
 * - api-key: a GoatFlow API token (gf_...).
 * - jwt: an access token from POST /api/v1/auth/login. When expiresAt is less
 *   than a minute ahead, refreshFunction is called with refreshToken first.
 *   GoatflowClient.login() and withJWT() set one backed by
 *   POST /api/v1/auth/refresh.
 */
export type AuthConfig =
  | { type: 'api-key'; apiKey: string }
  | {
      type: 'jwt';
      token: string;
      refreshToken?: string;
      expiresAt?: Date;
      refreshFunction?: (refreshToken: string) => Promise<{
        accessToken: string;
        refreshToken: string;
        expiresAt: Date;
      }>;
    };

// Errors

/**
 * A non-successful API response: HTTP status outside 2xx, or a 2xx response
 * whose envelope says {"success": false}. code is set when the API sent one
 * (e.g. "core:invalid_token"); body holds the raw body when it was not JSON.
 */
export class GoatflowError extends Error {
  constructor(
    message: string,
    public readonly statusCode?: number,
    public readonly code?: string,
    public readonly body?: string
  ) {
    super(message);
    this.name = 'GoatflowError';
  }
}

/** The request produced no HTTP response. */
export class NetworkError extends GoatflowError {
  constructor(
    public readonly method: string,
    public readonly url: string,
    cause: string
  ) {
    super(`Network error during ${method} ${url}: ${cause}`);
    this.name = 'NetworkError';
  }
}

export class TimeoutError extends GoatflowError {
  constructor(
    public readonly method: string,
    public readonly url: string,
    public readonly timeout: number
  ) {
    super(`${method} ${url} timed out after ${timeout}ms`);
    this.name = 'TimeoutError';
  }
}
