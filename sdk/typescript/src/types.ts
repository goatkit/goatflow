/**
 * TypeScript types for the GoatFlow API
 */

export interface Ticket {
  id: number;
  ticket_number: string;
  title: string;
  description: string;
  status: string;
  priority: string;
  type: string;
  queue_id: number;
  customer_id: number;
  assigned_to?: number;
  created_at: string;
  updated_at: string;
  closed_at?: string;
  tags?: string[];
  custom_fields?: Record<string, any>;
  customer?: User;
  assigned_user?: User;
  queue?: Queue;
  messages?: TicketMessage[];
  attachments?: Attachment[];
}

export interface TicketMessage {
  id: number;
  ticket_id: number;
  content: string;
  message_type: string;
  is_internal: boolean;
  author_id: number;
  created_at: string;
  updated_at: string;
  author?: User;
  attachments?: Attachment[];
  custom_fields?: Record<string, any>;
}

export interface User {
  id: number;
  email: string;
  first_name: string;
  last_name: string;
  login: string;
  title: string;
  role: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
  last_login_at: string;
}

export interface Queue {
  id: number;
  name: string;
  description: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface Attachment {
  id: number;
  filename: string;
  content_type: string;
  size: number;
  ticket_id: number;
  message_id?: number;
  uploaded_by: number;
  created_at: string;
}

export interface Group {
  id: number;
  name: string;
  description: string;
  type: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface DashboardStats {
  total_tickets: number;
  open_tickets: number;
  closed_tickets: number;
  pending_tickets: number;
  overdue_tickets: number;
  unassigned_tickets: number;
  my_tickets: number;
  tickets_by_status: Record<string, number>;
  tickets_by_priority: Record<string, number>;
  tickets_by_queue: Record<string, number>;
}

export interface SearchResult {
  total_count: number;
  page: number;
  page_size: number;
  tickets: Ticket[];
}

export interface InternalNote {
  id: number;
  ticket_id: number;
  content: string;
  category: string;
  is_important: boolean;
  is_pinned: boolean;
  tags: string[];
  author_id: number;
  author_name: string;
  author_email: string;
  created_at: string;
  updated_at: string;
  edited_at: string;
  edited_by: number;
}

export interface NoteTemplate {
  id: number;
  name: string;
  content: string;
  category: string;
  tags: string[];
  is_important: boolean;
  created_by: number;
  created_at: string;
  updated_at: string;
}

export interface LDAPUser {
  dn: string;
  username: string;
  email: string;
  first_name: string;
  last_name: string;
  display_name: string;
  phone: string;
  department: string;
  title: string;
  manager: string;
  groups: string[];
  attributes: Record<string, string>;
  object_guid: string;
  object_sid: string;
  last_login: string;
  is_active: boolean;
}

export interface LDAPSyncResult {
  users_found: number;
  users_created: number;
  users_updated: number;
  users_disabled: number;
  groups_found: number;
  groups_created: number;
  groups_updated: number;
  errors: string[];
  start_time: string;
  end_time: string;
  duration: string;
  dry_run: boolean;
}

export interface Webhook {
  id: number;
  name: string;
  url: string;
  events: string[];
  secret?: string;
  is_active: boolean;
  retry_count: number;
  timeout: number;
  headers?: Record<string, string>;
  created_at: string;
  updated_at: string;
  last_fired_at?: string;
}

export interface WebhookDelivery {
  id: number;
  webhook_id: number;
  event: string;
  payload: string;
  status_code: number;
  response: string;
  success: boolean;
  attempt: number;
  delivered_at: string;
}

// Request types
export interface TicketCreateRequest {
  title: string;
  description: string;
  priority?: string;
  type?: string;
  queue_id?: number;
  customer_id?: number;
  assigned_to?: number;
  tags?: string[];
  custom_fields?: Record<string, any>;
}

export interface TicketUpdateRequest {
  title?: string;
  description?: string;
  status?: string;
  priority?: string;
  type?: string;
  queue_id?: number;
  assigned_to?: number;
  tags?: string[];
  custom_fields?: Record<string, any>;
}

export interface TicketListOptions {
  page?: number;
  page_size?: number;
  status?: string[];
  priority?: string[];
  queue_id?: number[];
  assigned_to?: number;
  customer_id?: number;
  search?: string;
  tags?: string[];
  created_after?: string;
  created_before?: string;
  sort_by?: string;
  sort_order?: string;
}

export interface TicketListResponse {
  tickets: Ticket[];
  total_count: number;
  page: number;
  page_size: number;
  total_pages: number;
}

export interface MessageCreateRequest {
  content: string;
  message_type?: string;
  is_internal?: boolean;
  custom_fields?: Record<string, any>;
}

export interface UserCreateRequest {
  email: string;
  first_name: string;
  last_name: string;
  login: string;
  title?: string;
  role?: string;
  password: string;
}

export interface UserUpdateRequest {
  email?: string;
  first_name?: string;
  last_name?: string;
  title?: string;
  role?: string;
  is_active?: boolean;
}

export interface AuthLoginRequest {
  email: string;
  password: string;
}

export interface AuthLoginResponse {
  token: string;
  refresh_token: string;
  expires_at: string;
  user: User;
}

export interface APIResponse<T = any> {
  success: boolean;
  data?: T;
  error?: string;
  message?: string;
}

export interface ErrorResponse {
  error: string;
  message: string;
  code: number;
}

// Configuration types
export interface ClientConfig {
  baseURL: string;
  auth?: AuthConfig;
  timeout?: number;
  retries?: number;
  debug?: boolean;
  userAgent?: string;
}

export interface AuthConfig {
  type: 'api-key' | 'jwt' | 'oauth2';
  apiKey?: string;
  token?: string;
  refreshToken?: string;
  expiresAt?: Date;
  refreshFunction?: (refreshToken: string) => Promise<{
    accessToken: string;
    refreshToken: string;
    expiresAt: Date;
  }>;
}

// Event types for WebSocket
export interface WebSocketEvent {
  type: string;
  data: any;
  timestamp: string;
}

export interface TicketEvent extends WebSocketEvent {
  type: 'ticket.created' | 'ticket.updated' | 'ticket.closed' | 'ticket.assigned';
  data: Ticket;
}

export interface MessageEvent extends WebSocketEvent {
  type: 'message.created' | 'message.updated';
  data: TicketMessage;
}

// Error types
export class GoatflowError extends Error {
  public statusCode?: number;
  public code?: string;
  public details?: string;

  constructor(message: string, statusCode?: number, code?: string, details?: string) {
    super(message);
    this.name = 'GoatflowError';
    this.statusCode = statusCode;
    this.code = code;
    this.details = details;
  }
}

export class ValidationError extends GoatflowError {
  public field: string;
  public value?: any;

  constructor(field: string, message: string, value?: any) {
    super(`Validation error for field '${field}': ${message}`);
    this.name = 'ValidationError';
    this.field = field;
    this.value = value;
  }
}

export class NetworkError extends GoatflowError {
  public operation: string;
  public url: string;

  constructor(operation: string, url: string, message: string) {
    super(`Network error during ${operation} to ${url}: ${message}`);
    this.name = 'NetworkError';
    this.operation = operation;
    this.url = url;
  }
}

export class TimeoutError extends GoatflowError {
  public timeout: number;

  constructor(operation: string, timeout: number) {
    super(`Timeout error during ${operation} after ${timeout}ms`);
    this.name = 'TimeoutError';
    this.timeout = timeout;
  }
}