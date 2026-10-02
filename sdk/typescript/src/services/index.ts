import { HttpClient } from '../client.js';
import {
  CreatedUser,
  CurrentUser,
  DashboardStatistics,
  GoatflowError,
  TokenPair,
  Queue,
  QueueListOptions,
  SearchQuery,
  SearchResults,
  User,
  UserCreateRequest,
  UserList,
  UserListOptions,
  UserUpdateRequest,
  Webhook,
  WebhookDelivery,
  WebhookRequest,
} from '../types.js';

export { TicketsService, ArticlesService } from './tickets.js';

/** /api/v1/users (agents) */
export class UsersService {
  constructor(private readonly client: HttpClient) {}

  /** One page of agents. */
  async list(options: UserListOptions = {}): Promise<UserList> {
    const { data, pagination } = await this.client.request<User[]>('GET', '/api/v1/users', { query: { ...options } });
    if (!pagination) {
      throw new GoatflowError('User list response has no pagination');
    }
    return { users: data, pagination };
  }

  /** One agent, including email and preferences. */
  async get(id: number): Promise<User> {
    return this.client.get<User>(`/api/v1/users/${id}`);
  }

  /** The authenticated agent. */
  async me(): Promise<CurrentUser> {
    return this.client.get<CurrentUser>('/api/v1/users/me');
  }

  async create(data: UserCreateRequest): Promise<CreatedUser> {
    return this.client.post<CreatedUser>('/api/v1/users', data);
  }

  /** Changes the given fields. The API answers only with the id; use get() to read the result. */
  async update(id: number, data: UserUpdateRequest): Promise<void> {
    await this.client.put(`/api/v1/users/${id}`, data);
  }

  async delete(id: number): Promise<void> {
    await this.client.delete(`/api/v1/users/${id}`);
  }
}

/** /api/v1/queues */
export class QueuesService {
  constructor(private readonly client: HttpClient) {}

  /** Queues the caller can read. */
  async list(options: QueueListOptions = {}): Promise<Queue[]> {
    return this.client.get<Queue[]>('/api/v1/queues', { ...options });
  }

  async get(id: number): Promise<Queue> {
    return this.client.get<Queue>(`/api/v1/queues/${id}`);
  }
}

/** /api/v1/statistics */
export class StatisticsService {
  constructor(private readonly client: HttpClient) {}

  /** Ticket counts overall, per queue and per priority, and the newest tickets. */
  async dashboard(): Promise<DashboardStatistics> {
    return this.client.get<DashboardStatistics>('/api/v1/statistics/dashboard');
  }
}

/** POST /api/v1/search */
export class SearchService {
  constructor(private readonly client: HttpClient) {}

  async query(query: SearchQuery): Promise<SearchResults> {
    return this.client.post<SearchResults>('/api/v1/search', query);
  }
}

/** /api/v1/webhooks (admin only) */
export class WebhooksService {
  constructor(private readonly client: HttpClient) {}

  async list(): Promise<Webhook[]> {
    return this.client.get<Webhook[]>('/api/v1/webhooks');
  }

  async get(id: number): Promise<Webhook> {
    return this.client.get<Webhook>(`/api/v1/webhooks/${id}`);
  }

  async create(data: WebhookRequest): Promise<Webhook> {
    return this.client.post<Webhook>('/api/v1/webhooks', data);
  }

  async update(id: number, data: WebhookRequest): Promise<Webhook> {
    return this.client.put<Webhook>(`/api/v1/webhooks/${id}`, data);
  }

  async delete(id: number): Promise<void> {
    await this.client.delete(`/api/v1/webhooks/${id}`);
  }

  /** Sends a webhook.test event now and returns the recorded delivery. */
  async test(id: number): Promise<WebhookDelivery> {
    return this.client.post<WebhookDelivery>(`/api/v1/webhooks/${id}/test`);
  }

  /** Newest deliveries first, without payload and response bodies. */
  async getDeliveries(id: number, limit?: number): Promise<WebhookDelivery[]> {
    return this.client.get<WebhookDelivery[]>(`/api/v1/webhooks/${id}/deliveries`, { limit });
  }

  /** One delivery including payload and response body. */
  async getDelivery(deliveryId: number): Promise<WebhookDelivery> {
    return this.client.get<WebhookDelivery>(`/api/v1/webhook-deliveries/${deliveryId}`);
  }

  /** Sends a delivery's payload again and returns the new delivery. */
  async redeliver(deliveryId: number): Promise<WebhookDelivery> {
    return this.client.post<WebhookDelivery>(`/api/v1/webhook-deliveries/${deliveryId}/redeliver`);
  }
}

/** POST /api/v1/auth/login and /api/v1/auth/refresh, sent without the client's credentials. */
export class AuthService {
  constructor(private readonly client: HttpClient) {}

  /** Exchanges an agent's login and password for a token pair. */
  async login(login: string, password: string): Promise<TokenPair> {
    const { data } = await this.client.request<TokenPair>('POST', '/api/v1/auth/login', {
      body: { login, password },
      authenticate: false,
    });
    return data;
  }

  /**
   * Exchanges a refresh token for a new access token and a new (rotated)
   * refresh token. A rejected refresh token throws a 401 GoatflowError.
   */
  async refresh(refreshToken: string): Promise<TokenPair> {
    const { data } = await this.client.request<TokenPair>('POST', '/api/v1/auth/refresh', {
      body: { refresh_token: refreshToken },
      authenticate: false,
    });
    return data;
  }
}
