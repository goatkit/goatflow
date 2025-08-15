/**
 * Pact Consumer Contract Test for Authentication API
 * This defines what the frontend expects from the backend auth endpoints
 */

import { Pact } from '@pact-foundation/pact';
import { Matchers } from '@pact-foundation/pact';
const { like, term } = Matchers;
import { resolve } from 'path';

// Mock API client for testing
class AuthApiClient {
  private baseUrl: string;

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl;
  }

  async login(email: string, password: string): Promise<{
    token: string;
    refresh_token: string;
    user: {
      id: number;
      email: string;
      role: string;
      first_name: string;
      last_name: string;
    };
  }> {
    const response = await fetch(`${this.baseUrl}/api/v1/auth/login`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ email, password }),
    });
    
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
    return response.json();
  }

  async refresh(refreshToken: string): Promise<{
    token: string;
    refresh_token: string;
  }> {
    const response = await fetch(`${this.baseUrl}/api/v1/auth/refresh`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });
    
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
    return response.json();
  }

  async logout(token: string): Promise<void> {
    const response = await fetch(`${this.baseUrl}/api/v1/auth/logout`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${token}`,
      },
    });
    
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
  }

  async getMe(token: string): Promise<{
    id: number;
    email: string;
    role: string;
    first_name: string;
    last_name: string;
    tenant_id: number;
  }> {
    const response = await fetch(`${this.baseUrl}/api/v1/auth/me`, {
      method: 'GET',
      headers: {
        'Authorization': `Bearer ${token}`,
      },
    });
    
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
    return response.json();
  }

  async changePassword(
    token: string,
    oldPassword: string,
    newPassword: string
  ): Promise<{ message: string }> {
    const response = await fetch(`${this.baseUrl}/api/v1/auth/change-password`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        old_password: oldPassword,
        new_password: newPassword,
      }),
    });
    
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
    return response.json();
  }
}

describe('Authentication API Contract Tests', () => {
  // Configure Pact
  const provider = new Pact({
    consumer: 'gotrs-frontend',
    provider: 'gotrs-backend',
    port: 1236,
    log: resolve(process.cwd(), 'logs', 'pact-auth.log'),
    dir: resolve(process.cwd(), 'pacts'),
    logLevel: 'info',
  });

  let client: AuthApiClient;

  beforeAll(async () => {
    await provider.setup();
    client = new AuthApiClient('http://localhost:1236');
  });

  afterAll(async () => {
    await provider.finalize();
  });

  afterEach(async () => {
    await provider.verify();
  });

  describe('POST /api/v1/auth/login', () => {
    it('should successfully login with valid credentials', async () => {
      const loginRequest = {
        email: 'admin@example.com',
        password: 'correctpassword',
      };

      const expectedResponse = {
        token: term({
          generate: 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ',
          matcher: '^[A-Za-z0-9-_]+\\.[A-Za-z0-9-_]+\\.[A-Za-z0-9-_]+$'
        }),
        refresh_token: term({
          generate: 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ',
          matcher: '^[A-Za-z0-9-_]+\\.[A-Za-z0-9-_]+\\.[A-Za-z0-9-_]+$'
        }),
        user: {
          id: like(1),
          email: 'admin@example.com',
          role: 'Admin',
          first_name: like('John'),
          last_name: like('Doe'),
        },
      };

      await provider.addInteraction({
        state: 'user with email admin@example.com exists with correct password',
        uponReceiving: 'a login request with valid credentials',
        withRequest: {
          method: 'POST',
          path: '/api/v1/auth/login',
          headers: {
            'Content-Type': 'application/json',
          },
          body: loginRequest,
        },
        willRespondWith: {
          status: 200,
          headers: {
            'Content-Type': 'application/json',
          },
          body: expectedResponse,
        },
      });

      const result = await client.login('admin@example.com', 'correctpassword');

      expect(result.token).toBeDefined();
      expect(result.refresh_token).toBeDefined();
      expect(result.user.email).toBe('admin@example.com');
      expect(result.user.role).toBe('Admin');
    });

    it('should fail login with invalid credentials', async () => {
      const loginRequest = {
        email: 'admin@example.com',
        password: 'wrongpassword',
      };

      await provider.addInteraction({
        state: 'user with email admin@example.com exists',
        uponReceiving: 'a login request with invalid password',
        withRequest: {
          method: 'POST',
          path: '/api/v1/auth/login',
          headers: {
            'Content-Type': 'application/json',
          },
          body: loginRequest,
        },
        willRespondWith: {
          status: 401,
          headers: {
            'Content-Type': 'application/json',
          },
          body: {
            error: 'Invalid email or password',
          },
        },
      });

      await expect(client.login('admin@example.com', 'wrongpassword')).rejects.toThrow('HTTP 401');
    });

    it('should handle account lockout', async () => {
      const loginRequest = {
        email: 'locked@example.com',
        password: 'anypassword',
      };

      await provider.addInteraction({
        state: 'user account is locked due to too many failed attempts',
        uponReceiving: 'a login request for a locked account',
        withRequest: {
          method: 'POST',
          path: '/api/v1/auth/login',
          headers: {
            'Content-Type': 'application/json',
          },
          body: loginRequest,
        },
        willRespondWith: {
          status: 423,
          headers: {
            'Content-Type': 'application/json',
          },
          body: {
            error: 'Account is locked. Please contact support.',
          },
        },
      });

      await expect(client.login('locked@example.com', 'anypassword')).rejects.toThrow('HTTP 423');
    });

    it('should handle validation errors', async () => {
      const invalidRequest = {
        email: 'notanemail',
        password: '',
      };

      await provider.addInteraction({
        state: 'no preconditions',
        uponReceiving: 'a login request with invalid email format',
        withRequest: {
          method: 'POST',
          path: '/api/v1/auth/login',
          headers: {
            'Content-Type': 'application/json',
          },
          body: invalidRequest,
        },
        willRespondWith: {
          status: 400,
          headers: {
            'Content-Type': 'application/json',
          },
          body: {
            error: 'Validation failed',
            details: {
              email: 'Invalid email format',
              password: 'Password is required',
            },
          },
        },
      });

      await expect(client.login('notanemail', '')).rejects.toThrow('HTTP 400');
    });
  });

  describe('POST /api/v1/auth/refresh', () => {
    it('should refresh tokens with valid refresh token', async () => {
      const validRefreshToken = 'valid.refresh.token';
      
      const expectedResponse = {
        token: term({
          generate: 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.newtoken',
          matcher: '^[A-Za-z0-9-_]+\\.[A-Za-z0-9-_]+\\.[A-Za-z0-9-_]+$'
        }),
        refresh_token: term({
          generate: 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.newrefresh',
          matcher: '^[A-Za-z0-9-_]+\\.[A-Za-z0-9-_]+\\.[A-Za-z0-9-_]+$'
        }),
      };

      await provider.addInteraction({
        state: 'valid refresh token exists',
        uponReceiving: 'a token refresh request',
        withRequest: {
          method: 'POST',
          path: '/api/v1/auth/refresh',
          headers: {
            'Content-Type': 'application/json',
          },
          body: {
            refresh_token: validRefreshToken,
          },
        },
        willRespondWith: {
          status: 200,
          headers: {
            'Content-Type': 'application/json',
          },
          body: expectedResponse,
        },
      });

      const result = await client.refresh(validRefreshToken);

      expect(result.token).toBeDefined();
      expect(result.refresh_token).toBeDefined();
      expect(result.token).toMatch(/^[A-Za-z0-9-_]+\.[A-Za-z0-9-_]+\.[A-Za-z0-9-_]+$/);
    });

    it('should reject invalid refresh token', async () => {
      await provider.addInteraction({
        state: 'no preconditions',
        uponReceiving: 'a token refresh request with invalid token',
        withRequest: {
          method: 'POST',
          path: '/api/v1/auth/refresh',
          headers: {
            'Content-Type': 'application/json',
          },
          body: {
            refresh_token: 'invalid.refresh.token',
          },
        },
        willRespondWith: {
          status: 401,
          headers: {
            'Content-Type': 'application/json',
          },
          body: {
            error: 'Invalid refresh token',
          },
        },
      });

      await expect(client.refresh('invalid.refresh.token')).rejects.toThrow('HTTP 401');
    });
  });

  describe('POST /api/v1/auth/logout', () => {
    it('should successfully logout', async () => {
      const validToken = 'valid.jwt.token';

      await provider.addInteraction({
        state: 'user is logged in',
        uponReceiving: 'a logout request',
        withRequest: {
          method: 'POST',
          path: '/api/v1/auth/logout',
          headers: {
            'Authorization': `Bearer ${validToken}`,
          },
        },
        willRespondWith: {
          status: 200,
          headers: {
            'Content-Type': 'application/json',
          },
          body: {
            message: 'Logged out successfully',
          },
        },
      });

      await expect(client.logout(validToken)).resolves.toBeUndefined();
    });
  });

  describe('GET /api/v1/auth/me', () => {
    it('should return current user information', async () => {
      const validToken = 'valid.jwt.token';
      
      const expectedResponse = {
        id: like(1),
        email: like('admin@example.com'),
        role: like('Admin'),
        first_name: like('John'),
        last_name: like('Doe'),
        tenant_id: like(1),
      };

      await provider.addInteraction({
        state: 'user is authenticated',
        uponReceiving: 'a request for current user info',
        withRequest: {
          method: 'GET',
          path: '/api/v1/auth/me',
          headers: {
            'Authorization': `Bearer ${validToken}`,
          },
        },
        willRespondWith: {
          status: 200,
          headers: {
            'Content-Type': 'application/json',
          },
          body: expectedResponse,
        },
      });

      const result = await client.getMe(validToken);

      expect(result.id).toBeDefined();
      expect(result.email).toBeDefined();
      expect(result.role).toBeDefined();
      expect(result.tenant_id).toBeDefined();
    });

    it('should reject unauthorized requests', async () => {
      await provider.addInteraction({
        state: 'no authentication',
        uponReceiving: 'a request without auth token',
        withRequest: {
          method: 'GET',
          path: '/api/v1/auth/me',
          headers: {
            'Authorization': 'Bearer invalid.token',
          },
        },
        willRespondWith: {
          status: 401,
          headers: {
            'Content-Type': 'application/json',
          },
          body: {
            error: 'Unauthorized',
          },
        },
      });

      await expect(client.getMe('invalid.token')).rejects.toThrow('HTTP 401');
    });
  });

  describe('POST /api/v1/auth/change-password', () => {
    it('should successfully change password', async () => {
      const validToken = 'valid.jwt.token';
      const passwordRequest = {
        old_password: 'currentPassword123',
        new_password: 'newSecurePassword456',
      };

      await provider.addInteraction({
        state: 'user is authenticated with password currentPassword123',
        uponReceiving: 'a password change request',
        withRequest: {
          method: 'POST',
          path: '/api/v1/auth/change-password',
          headers: {
            'Authorization': `Bearer ${validToken}`,
            'Content-Type': 'application/json',
          },
          body: passwordRequest,
        },
        willRespondWith: {
          status: 200,
          headers: {
            'Content-Type': 'application/json',
          },
          body: {
            message: 'Password changed successfully',
          },
        },
      });

      const result = await client.changePassword(validToken, 'currentPassword123', 'newSecurePassword456');

      expect(result.message).toBe('Password changed successfully');
    });

    it('should reject incorrect old password', async () => {
      const validToken = 'valid.jwt.token';
      const passwordRequest = {
        old_password: 'wrongPassword',
        new_password: 'newSecurePassword456',
      };

      await provider.addInteraction({
        state: 'user is authenticated',
        uponReceiving: 'a password change request with wrong old password',
        withRequest: {
          method: 'POST',
          path: '/api/v1/auth/change-password',
          headers: {
            'Authorization': `Bearer ${validToken}`,
            'Content-Type': 'application/json',
          },
          body: passwordRequest,
        },
        willRespondWith: {
          status: 400,
          headers: {
            'Content-Type': 'application/json',
          },
          body: {
            error: 'Invalid old password',
          },
        },
      });

      await expect(
        client.changePassword(validToken, 'wrongPassword', 'newSecurePassword456')
      ).rejects.toThrow('HTTP 400');
    });

    it('should validate password strength', async () => {
      const validToken = 'valid.jwt.token';
      const passwordRequest = {
        old_password: 'currentPassword123',
        new_password: '123', // Too weak
      };

      await provider.addInteraction({
        state: 'user is authenticated',
        uponReceiving: 'a password change request with weak new password',
        withRequest: {
          method: 'POST',
          path: '/api/v1/auth/change-password',
          headers: {
            'Authorization': `Bearer ${validToken}`,
            'Content-Type': 'application/json',
          },
          body: passwordRequest,
        },
        willRespondWith: {
          status: 400,
          headers: {
            'Content-Type': 'application/json',
          },
          body: {
            error: 'Password does not meet strength requirements',
            details: {
              min_length: 8,
              requirements: ['Must contain uppercase', 'Must contain lowercase', 'Must contain number'],
            },
          },
        },
      });

      await expect(
        client.changePassword(validToken, 'currentPassword123', '123')
      ).rejects.toThrow('HTTP 400');
    });
  });
});

// Export for use in other tests
export { AuthApiClient };