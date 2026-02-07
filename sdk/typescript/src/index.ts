import { HttpClient } from './client';
import { EventsClient } from './events';
import {
  TicketsService,
  UsersService,
  QueuesService,
  DashboardService,
  LDAPService,
  WebhooksService,
  NotesService,
  AuthService,
} from './services';
import { ClientConfig, AuthConfig } from './types';

/**
 * Main GoatFlow API client
 */
export class GoatflowClient {
  private httpClient: HttpClient;
  private eventsClient?: EventsClient;

  // Service instances
  public readonly tickets: TicketsService;
  public readonly users: UsersService;
  public readonly queues: QueuesService;
  public readonly dashboard: DashboardService;
  public readonly ldap: LDAPService;
  public readonly webhooks: WebhooksService;
  public readonly notes: NotesService;
  public readonly auth: AuthService;

  constructor(config: ClientConfig) {
    this.httpClient = new HttpClient(config);

    // Initialize services
    this.tickets = new TicketsService(this.httpClient);
    this.users = new UsersService(this.httpClient);
    this.queues = new QueuesService(this.httpClient);
    this.dashboard = new DashboardService(this.httpClient);
    this.ldap = new LDAPService(this.httpClient);
    this.webhooks = new WebhooksService(this.httpClient);
    this.notes = new NotesService(this.httpClient);
    this.auth = new AuthService(this.httpClient);
  }

  /**
   * Create a GoatFlow client with API key authentication
   */
  static withApiKey(baseURL: string, apiKey: string, options?: Partial<ClientConfig>): GoatflowClient {
    return new GoatflowClient({
      baseURL,
      auth: {
        type: 'api-key',
        apiKey,
      },
      ...options,
    });
  }

  /**
   * Create a GoatFlow client with JWT authentication
   */
  static withJWT(
    baseURL: string,
    token: string,
    refreshToken?: string,
    expiresAt?: Date,
    options?: Partial<ClientConfig>
  ): GoatflowClient {
    return new GoatflowClient({
      baseURL,
      auth: {
        type: 'jwt',
        token,
        refreshToken,
        expiresAt,
      },
      ...options,
    });
  }

  /**
   * Create a GoatFlow client with OAuth2 authentication
   */
  static withOAuth2(
    baseURL: string,
    accessToken: string,
    refreshToken?: string,
    expiresAt?: Date,
    options?: Partial<ClientConfig>
  ): GoatflowClient {
    return new GoatflowClient({
      baseURL,
      auth: {
        type: 'oauth2',
        token: accessToken,
        refreshToken,
        expiresAt,
      },
      ...options,
    });
  }

  /**
   * Set new authentication configuration
   */
  setAuth(auth: AuthConfig): void {
    this.httpClient.setAuth(auth);
    
    // Update events client auth if exists
    if (this.eventsClient && auth.type !== 'api-key') {
      const authHeader = auth.token ? `Bearer ${auth.token}` : '';
      this.eventsClient.setAuth(authHeader);
    }
  }

  /**
   * Get or create the events client for real-time updates
   */
  get events(): EventsClient {
    if (!this.eventsClient) {
      // Extract auth header based on current auth type
      let authHeader = '';
      // We'll need to access the auth config somehow - for now using a simple approach
      
      this.eventsClient = new EventsClient(this.httpClient.getAxios().defaults.baseURL || '', authHeader);
    }
    return this.eventsClient;
  }

  /**
   * Test the connection to the GoatFlow API
   */
  async ping(): Promise<boolean> {
    try {
      await this.httpClient.get('/api/v1/health');
      return true;
    } catch (error) {
      return false;
    }
  }

  /**
   * Get API information
   */
  async getApiInfo(): Promise<any> {
    return this.httpClient.get('/api/v1/info');
  }

  /**
   * Login with email and password and update the client's authentication
   */
  async login(email: string, password: string): Promise<void> {
    const response = await this.auth.login({ email, password });
    
    this.setAuth({
      type: 'jwt',
      token: response.token,
      refreshToken: response.refresh_token,
      expiresAt: new Date(response.expires_at),
      refreshFunction: async (refreshToken: string) => {
        const refreshResponse = await this.auth.refreshToken(refreshToken);
        return {
          accessToken: refreshResponse.token,
          refreshToken: refreshResponse.refresh_token,
          expiresAt: new Date(refreshResponse.expires_at),
        };
      },
    });
  }

  /**
   * Logout and clear authentication
   */
  async logout(): Promise<void> {
    try {
      await this.auth.logout();
    } finally {
      // Clear auth regardless of API call success
      this.setAuth({ type: 'api-key' }); // Reset to basic auth
      
      // Disconnect events client
      if (this.eventsClient) {
        this.eventsClient.disconnect();
        this.eventsClient = undefined;
      }
    }
  }
}

// Export everything for direct usage
export * from './types';
export * from './client';
export * from './events';
export * from './services';

// Default export for convenience
export default GoatflowClient;

// Helper functions for error checking
export function isGoatflowError(error: any): error is import('./types').GoatflowError {
  return error && error.name === 'GoatflowError';
}

export function isNotFoundError(error: any): boolean {
  return isGoatflowError(error) && error.statusCode === 404;
}

export function isUnauthorizedError(error: any): boolean {
  return isGoatflowError(error) && error.statusCode === 401;
}

export function isForbiddenError(error: any): boolean {
  return isGoatflowError(error) && error.statusCode === 403;
}

export function isRateLimitError(error: any): boolean {
  return isGoatflowError(error) && error.statusCode === 429;
}

export function isValidationError(error: any): error is import('./types').ValidationError {
  return error && error.name === 'ValidationError';
}

export function isNetworkError(error: any): error is import('./types').NetworkError {
  return error && error.name === 'NetworkError';
}

export function isTimeoutError(error: any): error is import('./types').TimeoutError {
  return error && error.name === 'TimeoutError';
}