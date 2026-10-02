import { AuthConfig, ClientConfig, GoatflowError, NetworkError, Pagination, TimeoutError } from './types.js';

export type QueryValue = string | number | boolean | undefined | null | ReadonlyArray<string | number>;

export interface RequestOptions {
  query?: Record<string, QueryValue>;
  body?: unknown;
  /** Send the client's credentials (default true). Login and refresh send none. */
  authenticate?: boolean;
}

export interface DecodedResponse<T> {
  data: T;
  /** Set when a paginated list endpoint sent one next to "data". */
  pagination?: Pagination;
}

/**
 * Turns an API response into its payload or throws GoatflowError.
 *
 * GoatFlow wraps most responses in {"success": bool, "data": ..., "error": ...}
 * (paginated lists add "pagination"). Some endpoints answer
 * {"success": true, ...fields} without "data", and some send a bare object;
 * both are returned whole. Errors are {"error": "message"} or
 * {"error": {"code": "...", "message": "..."}}, with or without "success".
 */
export function decodeResponse<T>(status: number, statusText: string, text: string): DecodedResponse<T> {
  const trimmed = text.trim();
  let parsed: unknown;
  let isJSON = false;
  if (trimmed !== '') {
    try {
      parsed = JSON.parse(trimmed);
      isJSON = true;
    } catch {
      // Not JSON; handled below.
    }
  }
  const fields =
    isJSON && typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed) ? (parsed as Envelope) : undefined;
  const success = typeof fields?.success === 'boolean' ? fields.success : undefined;

  if (status < 200 || status > 299 || success === false) {
    throw apiError(status, statusText, trimmed, fields, isJSON);
  }
  if (trimmed === '') {
    return { data: undefined as T };
  }
  if (!isJSON) {
    throw new GoatflowError(`Response is not JSON (HTTP ${status})`, status, undefined, trimmed);
  }
  const pagination =
    typeof fields?.pagination === 'object' && fields.pagination !== null ? (fields.pagination as Pagination) : undefined;
  if (fields && success !== undefined && 'data' in fields) {
    return { data: fields.data as T, pagination };
  }
  return { data: parsed as T, pagination };
}

/** Top-level keys of a GoatFlow JSON response; values are unchecked. */
interface Envelope {
  success?: unknown;
  data?: unknown;
  error?: unknown;
  message?: unknown;
  pagination?: unknown;
}

function apiError(status: number, statusText: string, body: string, fields: Envelope | undefined, isJSON: boolean): GoatflowError {
  let message = '';
  let code: string | undefined;
  const error = fields?.error;
  if (typeof error === 'string') {
    message = error;
  } else if (typeof error === 'object' && error !== null) {
    const structured = error as { code?: unknown; message?: unknown };
    if (typeof structured.message === 'string') message = structured.message;
    if (typeof structured.code === 'string') code = structured.code;
  }
  if (!message && typeof fields?.message === 'string') {
    message = fields.message;
  }
  if (!message && (status < 200 || status > 299)) {
    message = statusText || `HTTP ${status}`;
  }
  return new GoatflowError(message || 'request failed', status, code, isJSON ? undefined : body);
}

/**
 * HTTP transport for the GoatFlow API (fetch-based; Node 18+ or browsers).
 */
export class HttpClient {
  private readonly baseURL: string;
  private readonly timeout: number;
  private readonly userAgent: string;
  private readonly fetchImpl: typeof fetch;
  private auth?: AuthConfig;
  /** In-flight token refresh shared by concurrent requests. */
  private refreshing?: Promise<void>;

  constructor(config: ClientConfig) {
    this.baseURL = config.baseURL.replace(/\/+$/, '');
    this.timeout = config.timeout ?? 30000;
    this.userAgent = config.userAgent ?? 'goatflow-ts-sdk/1.0.0';
    this.fetchImpl = config.fetch ?? globalThis.fetch.bind(globalThis);
    this.auth = config.auth;
  }

  setAuth(auth: AuthConfig | undefined): void {
    this.auth = auth;
    this.refreshing = undefined;
  }

  async get<T>(path: string, query?: Record<string, QueryValue>): Promise<T> {
    return (await this.request<T>('GET', path, { query })).data;
  }

  async post<T>(path: string, body?: unknown): Promise<T> {
    return (await this.request<T>('POST', path, { body })).data;
  }

  async put<T>(path: string, body?: unknown): Promise<T> {
    return (await this.request<T>('PUT', path, { body })).data;
  }

  async delete<T = void>(path: string): Promise<T> {
    return (await this.request<T>('DELETE', path)).data;
  }

  /** Sends a request and returns the decoded payload plus pagination. */
  async request<T>(method: string, path: string, options: RequestOptions = {}): Promise<DecodedResponse<T>> {
    const url = this.baseURL + path + buildQuery(options.query);
    const headers: Record<string, string> = {
      Accept: 'application/json',
      'User-Agent': this.userAgent,
    };
    if (options.authenticate !== false) {
      const authorization = await this.authorizationHeader();
      if (authorization) headers.Authorization = authorization;
    }
    let body: string | undefined;
    if (options.body !== undefined) {
      headers['Content-Type'] = 'application/json';
      body = JSON.stringify(options.body);
    }

    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), this.timeout);
    let response: Response;
    let text: string;
    try {
      // Unauthenticated calls may be redirected to the login page; surface the
      // redirect instead of following it to HTML.
      response = await this.fetchImpl(url, { method, headers, body, signal: controller.signal, redirect: 'manual' });
      text = await response.text();
    } catch (error) {
      if (controller.signal.aborted) throw new TimeoutError(method, url, this.timeout);
      throw new NetworkError(method, url, error instanceof Error ? error.message : String(error));
    } finally {
      clearTimeout(timer);
    }
    if (response.type === 'opaqueredirect') {
      throw new GoatflowError('Redirected (not authenticated?)', response.status || undefined);
    }
    return decodeResponse<T>(response.status, response.statusText, text);
  }

  private async authorizationHeader(): Promise<string | undefined> {
    const auth = this.auth;
    if (!auth) return undefined;
    if (auth.type === 'api-key') return `Bearer ${auth.apiKey}`;
    if (auth.expiresAt && auth.expiresAt.getTime() - 60_000 <= Date.now()) {
      this.refreshing ??= this.refresh(auth).finally(() => {
        this.refreshing = undefined;
      });
      await this.refreshing;
    }
    return `Bearer ${auth.token}`;
  }

  private async refresh(auth: Extract<AuthConfig, { type: 'jwt' }>): Promise<void> {
    const expired = auth.expiresAt?.toISOString();
    if (!auth.refreshFunction) {
      throw new GoatflowError(`Access token expired at ${expired} and no refreshFunction is configured`);
    }
    if (!auth.refreshToken) {
      throw new GoatflowError(`Access token expired at ${expired} and no refresh token is available`);
    }
    const refreshed = await auth.refreshFunction(auth.refreshToken);
    auth.token = refreshed.accessToken;
    auth.refreshToken = refreshed.refreshToken;
    auth.expiresAt = refreshed.expiresAt;
  }
}

function buildQuery(query: Record<string, QueryValue> | undefined): string {
  if (!query) return '';
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === null || value === '' || value === false) continue;
    if (Array.isArray(value)) {
      if (value.length > 0) params.set(key, value.join(','));
    } else {
      params.set(key, String(value));
    }
  }
  const encoded = params.toString();
  return encoded ? `?${encoded}` : '';
}
