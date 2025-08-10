/**
 * Pact Consumer Contract Test for Health API
 * This defines what the frontend expects from the backend health endpoint
 */

import { Pact, Interaction } from '@pact-foundation/pact';
import { resolve } from 'path';

// Mock API client for testing
class HealthApiClient {
  private baseUrl: string;

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl;
  }

  async getHealth(): Promise<{ status: string; service: string }> {
    const response = await fetch(`${this.baseUrl}/health`);
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
    return response.json();
  }

  async getApiStatus(): Promise<{ 
    success: boolean; 
    data: { message: string; version: string } 
  }> {
    const response = await fetch(`${this.baseUrl}/api/v1/status`);
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
    return response.json();
  }
}

describe('Health API Contract Tests', () => {
  // Configure Pact
  const provider = new Pact({
    consumer: 'gotrs-frontend',
    provider: 'gotrs-backend',
    port: 1234,
    log: resolve(process.cwd(), 'logs', 'pact.log'),
    dir: resolve(process.cwd(), 'pacts'),
    logLevel: 'info',
  });

  let client: HealthApiClient;

  beforeAll(async () => {
    await provider.setup();
    client = new HealthApiClient('http://localhost:1234');
  });

  afterAll(async () => {
    await provider.finalize();
  });

  afterEach(async () => {
    await provider.verify();
  });

  describe('GET /health', () => {
    it('should return healthy status', async () => {
      // Arrange - Define the contract expectation
      const expectedResponse = {
        status: 'healthy',
        service: 'gotrs-backend'
      };

      await provider.addInteraction({
        state: 'server is running normally',
        uponReceiving: 'a request for health status',
        withRequest: {
          method: 'GET',
          path: '/health',
        },
        willRespondWith: {
          status: 200,
          headers: {
            'Content-Type': 'application/json',
          },
          body: expectedResponse,
        },
      });

      // Act - Make the actual API call
      const result = await client.getHealth();

      // Assert - Verify the response matches expectations
      expect(result).toEqual(expectedResponse);
      expect(result.status).toBe('healthy');
      expect(result.service).toBe('gotrs-backend');
    });

    it('should handle server error responses', async () => {
      await provider.addInteraction({
        state: 'server is experiencing issues',
        uponReceiving: 'a request for health status when server is down',
        withRequest: {
          method: 'GET',
          path: '/health',
        },
        willRespondWith: {
          status: 500,
          headers: {
            'Content-Type': 'application/json',
          },
          body: {
            status: 'unhealthy',
            service: 'gotrs-backend',
            error: 'Internal server error'
          },
        },
      });

      // Expect the client to throw an error for 500 status
      await expect(client.getHealth()).rejects.toThrow('HTTP 500');
    });
  });

  describe('GET /api/v1/status', () => {
    it('should return API status and version', async () => {
      const expectedResponse = {
        success: true,
        data: {
          message: 'GOTRS API is running',
          version: '0.1.0'
        }
      };

      await provider.addInteraction({
        state: 'API is running normally',
        uponReceiving: 'a request for API status',
        withRequest: {
          method: 'GET',
          path: '/api/v1/status',
        },
        willRespondWith: {
          status: 200,
          headers: {
            'Content-Type': 'application/json',
          },
          body: expectedResponse,
        },
      });

      const result = await client.getApiStatus();

      expect(result).toEqual(expectedResponse);
      expect(result.success).toBe(true);
      expect(result.data.message).toBe('GOTRS API is running');
      expect(result.data.version).toBe('0.1.0');
    });
  });
});

// Export for use in other tests
export { HealthApiClient };