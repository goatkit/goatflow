import { HttpClient } from './client.js';
import {
  ArticlesService,
  AuthService,
  QueuesService,
  SearchService,
  StatisticsService,
  TicketsService,
  UsersService,
  WebhooksService,
} from './services/index.js';
import { AuthConfig, ClientConfig, GoatflowError, Health, TokenPair } from './types.js';

/** GoatFlow REST API client. */
export class GoatflowClient {
  readonly http: HttpClient;

  readonly tickets: TicketsService;
  readonly articles: ArticlesService;
  readonly users: UsersService;
  readonly queues: QueuesService;
  readonly statistics: StatisticsService;
  readonly search: SearchService;
  readonly webhooks: WebhooksService;
  readonly auth: AuthService;

  constructor(config: ClientConfig) {
    this.http = new HttpClient(config);
    this.tickets = new TicketsService(this.http);
    this.articles = new ArticlesService(this.http);
    this.users = new UsersService(this.http);
    this.queues = new QueuesService(this.http);
    this.statistics = new StatisticsService(this.http);
    this.search = new SearchService(this.http);
    this.webhooks = new WebhooksService(this.http);
    this.auth = new AuthService(this.http);
  }

  /** Client authenticating with a GoatFlow API token (gf_...). */
  static withApiKey(baseURL: string, apiKey: string, options: Partial<ClientConfig> = {}): GoatflowClient {
    return new GoatflowClient({ ...options, baseURL, auth: { type: 'api-key', apiKey } });
  }

  /**
   * Client authenticating with a JWT access token from POST /api/v1/auth/login.
   * With refreshToken and expiresAt, the token pair is renewed through
   * POST /api/v1/auth/refresh shortly before expiry.
   */
  static withJWT(
    baseURL: string,
    token: string,
    refreshToken?: string,
    expiresAt?: Date,
    options: Partial<ClientConfig> = {}
  ): GoatflowClient {
    const client = new GoatflowClient({ ...options, baseURL });
    client.useJWT(token, refreshToken, expiresAt);
    return client;
  }

  setAuth(auth: AuthConfig | undefined): void {
    this.http.setAuth(auth);
  }

  /** Authenticate with a JWT pair, renewed through POST /api/v1/auth/refresh. */
  useJWT(token: string, refreshToken?: string, expiresAt?: Date): void {
    this.setAuth({
      type: 'jwt',
      token,
      refreshToken,
      expiresAt,
      refreshFunction: async (current) => {
        const pair = await this.auth.refresh(current);
        return {
          accessToken: pair.access_token,
          refreshToken: pair.refresh_token,
          expiresAt: new Date(Date.now() + pair.expires_in * 1000),
        };
      },
    });
  }

  /** GET /health. An unhealthy server answers 503, thrown as GoatflowError. */
  async health(): Promise<Health> {
    return this.http.get<Health>('/health');
  }

  /** True when the server reports itself healthy. */
  async ping(): Promise<boolean> {
    try {
      await this.health();
      return true;
    } catch (error) {
      if (error instanceof GoatflowError) return false;
      throw error;
    }
  }

  /**
   * Logs in and switches the client to the returned access token, renewed
   * automatically through POST /api/v1/auth/refresh.
   */
  async login(login: string, password: string): Promise<TokenPair> {
    const pair = await this.auth.login(login, password);
    this.useJWT(pair.access_token, pair.refresh_token, new Date(Date.now() + pair.expires_in * 1000));
    return pair;
  }
}

export * from './types.js';
export * from './client.js';
export * from './services/index.js';

export default GoatflowClient;

export function isGoatflowError(error: unknown): error is GoatflowError {
  return error instanceof GoatflowError;
}

export function isNotFoundError(error: unknown): error is GoatflowError {
  return error instanceof GoatflowError && error.statusCode === 404;
}

export function isUnauthorizedError(error: unknown): error is GoatflowError {
  return error instanceof GoatflowError && error.statusCode === 401;
}

export function isForbiddenError(error: unknown): error is GoatflowError {
  return error instanceof GoatflowError && error.statusCode === 403;
}

export function isRateLimitError(error: unknown): error is GoatflowError {
  return error instanceof GoatflowError && error.statusCode === 429;
}
