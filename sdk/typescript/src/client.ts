import axios, { AxiosInstance, AxiosRequestConfig, AxiosResponse } from 'axios';
import {
  ClientConfig,
  AuthConfig,
  GoatflowError,
  NetworkError,
  TimeoutError,
  APIResponse,
  ErrorResponse,
} from './types';

/**
 * HTTP client for making requests to the GoatFlow API
 */
export class HttpClient {
  private axios: AxiosInstance;
  private auth?: AuthConfig;

  constructor(config: ClientConfig) {
    this.auth = config.auth;

    this.axios = axios.create({
      baseURL: config.baseURL,
      timeout: config.timeout || 30000,
      headers: {
        'Content-Type': 'application/json',
        'User-Agent': config.userAgent || 'goatflow-ts-sdk/1.0.0',
      },
    });

    // Request interceptor for authentication
    this.axios.interceptors.request.use(
      async (requestConfig) => {
        if (this.auth) {
          await this.setAuthHeaders(requestConfig);
        }
        return requestConfig;
      },
      (error) => Promise.reject(error)
    );

    // Response interceptor for error handling
    this.axios.interceptors.response.use(
      (response) => response,
      (error) => {
        throw this.handleError(error);
      }
    );

    // Set up retries if configured
    if (config.retries && config.retries > 0) {
      this.setupRetries(config.retries);
    }
  }

  /**
   * Set authentication headers on the request
   */
  private async setAuthHeaders(config: AxiosRequestConfig): Promise<void> {
    if (!this.auth) return;

    // Check if token is expired and refresh if needed
    if (this.auth.type === 'jwt' || this.auth.type === 'oauth2') {
      if (this.auth.expiresAt && new Date() >= this.auth.expiresAt) {
        await this.refreshToken();
      }
    }

    // Set appropriate auth header
    switch (this.auth.type) {
      case 'api-key':
        if (this.auth.apiKey) {
          config.headers = {
            ...config.headers,
            'X-API-Key': this.auth.apiKey,
          };
        }
        break;
      case 'jwt':
      case 'oauth2':
        if (this.auth.token) {
          config.headers = {
            ...config.headers,
            Authorization: `Bearer ${this.auth.token}`,
          };
        }
        break;
    }
  }

  /**
   * Refresh the authentication token
   */
  private async refreshToken(): Promise<void> {
    if (!this.auth || !this.auth.refreshFunction || !this.auth.refreshToken) {
      throw new GoatflowError('Cannot refresh token: no refresh function or refresh token available');
    }

    try {
      const result = await this.auth.refreshFunction(this.auth.refreshToken);
      this.auth.token = result.accessToken;
      this.auth.refreshToken = result.refreshToken;
      this.auth.expiresAt = result.expiresAt;
    } catch (error) {
      throw new GoatflowError('Failed to refresh token', undefined, 'TOKEN_REFRESH_FAILED', 
        error instanceof Error ? error.message : 'Unknown error');
    }
  }

  /**
   * Handle HTTP errors and convert them to GoatflowError instances
   */
  private handleError(error: any): Error {
    if (error.code === 'ECONNABORTED' || error.code === 'ETIMEDOUT') {
      return new TimeoutError('request', error.config?.timeout || 0);
    }

    if (error.code === 'ECONNREFUSED' || error.code === 'ENOTFOUND') {
      return new NetworkError('request', error.config?.url || '', error.message);
    }

    if (error.response) {
      const statusCode = error.response.status;
      let message = 'Unknown error';
      let code = '';
      let details = '';

      // Try to parse error response
      if (error.response.data) {
        const data = error.response.data;
        if (typeof data === 'object') {
          message = data.message || data.error || message;
          code = data.code || '';
          details = data.details || '';
        } else if (typeof data === 'string') {
          message = data;
        }
      }

      return new GoatflowError(message, statusCode, code, details);
    }

    if (error.request) {
      return new NetworkError('request', error.config?.url || '', 'No response received');
    }

    return new GoatflowError(error.message || 'Unknown error');
  }

  /**
   * Set up retry logic
   */
  private setupRetries(retries: number): void {
    // Implement exponential backoff retry logic
    const retryDelay = (retryNumber: number) => {
      return Math.min(1000 * Math.pow(2, retryNumber), 30000);
    };

    this.axios.interceptors.response.use(
      (response) => response,
      async (error) => {
        const config = error.config;
        
        if (!config || config.__retryCount >= retries) {
          return Promise.reject(error);
        }

        // Only retry on specific status codes or network errors
        const shouldRetry = 
          !error.response || 
          error.response.status >= 500 || 
          error.response.status === 429 ||
          error.code === 'ECONNABORTED' ||
          error.code === 'ETIMEDOUT';

        if (!shouldRetry) {
          return Promise.reject(error);
        }

        config.__retryCount = config.__retryCount || 0;
        config.__retryCount++;

        const delay = retryDelay(config.__retryCount);
        await new Promise(resolve => setTimeout(resolve, delay));

        return this.axios(config);
      }
    );
  }

  /**
   * Update authentication configuration
   */
  public setAuth(auth: AuthConfig): void {
    this.auth = auth;
  }

  /**
   * Make a GET request
   */
  public async get<T = any>(url: string, config?: AxiosRequestConfig): Promise<T> {
    const response = await this.axios.get<APIResponse<T>>(url, config);
    return this.extractData(response);
  }

  /**
   * Make a POST request
   */
  public async post<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<T> {
    const response = await this.axios.post<APIResponse<T>>(url, data, config);
    return this.extractData(response);
  }

  /**
   * Make a PUT request
   */
  public async put<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<T> {
    const response = await this.axios.put<APIResponse<T>>(url, data, config);
    return this.extractData(response);
  }

  /**
   * Make a DELETE request
   */
  public async delete<T = any>(url: string, config?: AxiosRequestConfig): Promise<T> {
    const response = await this.axios.delete<APIResponse<T>>(url, config);
    return this.extractData(response);
  }

  /**
   * Make a PATCH request
   */
  public async patch<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<T> {
    const response = await this.axios.patch<APIResponse<T>>(url, data, config);
    return this.extractData(response);
  }

  /**
   * Extract data from API response
   */
  private extractData<T>(response: AxiosResponse<APIResponse<T>>): T {
    const data = response.data;
    
    // Handle standard API response format
    if (typeof data === 'object' && 'success' in data) {
      if (!data.success) {
        throw new GoatflowError(
          data.error || 'API request failed',
          response.status,
          'API_ERROR',
          data.message
        );
      }
      return data.data as T;
    }

    // Return raw data if not in standard format
    return data as T;
  }

  /**
   * Get the underlying axios instance
   */
  public getAxios(): AxiosInstance {
    return this.axios;
  }
}