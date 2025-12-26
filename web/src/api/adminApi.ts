/**
 * Admin API client with enforced headers.
 * 
 * This module ensures all fetch calls to admin endpoints include the
 * required headers for JSON responses. Without proper headers, handlers
 * may return HTML instead of JSON, causing parse errors.
 * 
 * ALWAYS use these functions instead of raw fetch() in admin templates.
 */

export interface ApiResponse<T> {
  success: boolean;
  data?: T;
  error?: string;
}

export interface FetchOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE';
  body?: unknown;
  headers?: Record<string, string>;
}

/**
 * Headers required for JSON API requests.
 * Handlers check these to decide JSON vs HTML response.
 */
export const JSON_REQUEST_HEADERS = {
  'Accept': 'application/json',
  'X-Requested-With': 'XMLHttpRequest',
} as const;

/**
 * Headers required for JSON POST/PUT requests with body.
 */
export const JSON_BODY_HEADERS = {
  ...JSON_REQUEST_HEADERS,
  'Content-Type': 'application/json',
} as const;

/**
 * Fetch wrapper that enforces required headers for admin API calls.
 * Automatically handles JSON parsing and error responses.
 * 
 * @param url - API endpoint URL
 * @param options - Fetch options (method, body, headers)
 * @returns Parsed JSON response
 * @throws Error if response is not JSON or request fails
 */
export async function adminFetch<T>(url: string, options: FetchOptions = {}): Promise<ApiResponse<T>> {
  const { method = 'GET', body, headers = {} } = options;

  const requestHeaders: Record<string, string> = {
    ...(body ? JSON_BODY_HEADERS : JSON_REQUEST_HEADERS),
    ...headers,
  };

  const response = await fetch(url, {
    method,
    headers: requestHeaders,
    body: body ? JSON.stringify(body) : undefined,
    credentials: 'same-origin',
  });

  // Check content-type before parsing
  const contentType = response.headers.get('content-type') || '';
  if (!contentType.includes('application/json')) {
    // This is the bug we're preventing - got HTML instead of JSON
    throw new Error(
      `Expected JSON response but got ${contentType}. ` +
      `This usually means the Accept header was not sent. URL: ${url}`
    );
  }

  const data = await response.json();

  if (!response.ok) {
    return {
      success: false,
      error: data.error || `HTTP ${response.status}`,
    };
  }

  return data;
}

/**
 * Handle fetch response with automatic auth redirect.
 * Use this to wrap existing fetch calls that don't use adminFetch.
 * 
 * @param response - Fetch Response object
 * @returns The response if valid JSON, throws otherwise
 */
export async function handleFetchResponse(response: Response): Promise<Response> {
  // Check for auth redirect (session expired)
  if (response.redirected && response.url.includes('/login')) {
    window.location.href = '/login';
    throw new Error('Session expired');
  }

  // Check for HTML response when JSON expected
  const contentType = response.headers.get('content-type') || '';
  if (!contentType.includes('application/json')) {
    throw new Error(
      `Expected JSON but got ${contentType}. Check that your fetch includes Accept: application/json header.`
    );
  }

  return response;
}

// Admin Roles API

export interface Role {
  id: number;
  name: string;
  comments: string;
  valid_id: number;
}

export interface RoleUser {
  user_id: number;
  login: string;
  first_name: string;
  last_name: string;
}

export const rolesApi = {
  list: () => adminFetch<Role[]>('/admin/roles'),
  
  get: (id: number) => adminFetch<Role>(`/admin/roles/${id}`),
  
  create: (role: Omit<Role, 'id'>) => adminFetch<Role>('/admin/roles', {
    method: 'POST',
    body: role,
  }),
  
  update: (id: number, role: Partial<Role>) => adminFetch<Role>(`/admin/roles/${id}`, {
    method: 'PUT',
    body: role,
  }),
  
  delete: (id: number) => adminFetch<void>(`/admin/roles/${id}`, {
    method: 'DELETE',
  }),
  
  getUsers: (roleId: number) => adminFetch<RoleUser[]>(`/admin/roles/${roleId}/users`),
  
  addUser: (roleId: number, userId: number) => adminFetch<void>(`/admin/roles/${roleId}/users`, {
    method: 'POST',
    body: { user_id: userId },
  }),
  
  removeUser: (roleId: number, userId: number) => adminFetch<void>(`/admin/roles/${roleId}/users/${userId}`, {
    method: 'DELETE',
  }),
};
