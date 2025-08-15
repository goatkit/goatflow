/**
 * Pact Consumer Contract Test for Tickets API
 * This defines what the frontend expects from the backend ticket endpoints
 */

import { Pact } from '@pact-foundation/pact';
import { Matchers } from '@pact-foundation/pact';
const { like, eachLike, term } = Matchers;
import { resolve } from 'path';

// Mock API client for testing
class TicketsApiClient {
  private baseUrl: string;
  private authToken: string;

  constructor(baseUrl: string, authToken: string) {
    this.baseUrl = baseUrl;
    this.authToken = authToken;
  }

  async getTickets(params?: {
    page?: number;
    per_page?: number;
    queue_id?: number;
    state_id?: number;
  }): Promise<{
    tickets: Array<any>;
    total: number;
    page: number;
    per_page: number;
    total_pages: number;
  }> {
    const queryParams = new URLSearchParams();
    if (params) {
      Object.entries(params).forEach(([key, value]) => {
        if (value !== undefined) {
          queryParams.append(key, value.toString());
        }
      });
    }
    
    const url = `${this.baseUrl}/api/v1/tickets${queryParams.toString() ? '?' + queryParams.toString() : ''}`;
    const response = await fetch(url, {
      headers: {
        'Authorization': `Bearer ${this.authToken}`,
      },
    });
    
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
    return response.json();
  }

  async getTicket(id: number): Promise<any> {
    const response = await fetch(`${this.baseUrl}/api/v1/tickets/${id}`, {
      headers: {
        'Authorization': `Bearer ${this.authToken}`,
      },
    });
    
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
    return response.json();
  }

  async createTicket(data: {
    title: string;
    queue_id: number;
    priority_id: number;
    body: string;
    subject?: string;
  }): Promise<any> {
    const response = await fetch(`${this.baseUrl}/api/v1/tickets`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${this.authToken}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(data),
    });
    
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
    return response.json();
  }

  async updateTicket(id: number, data: Partial<{
    title: string;
    queue_id: number;
    priority_id: number;
    state_id: number;
  }>): Promise<any> {
    const response = await fetch(`${this.baseUrl}/api/v1/tickets/${id}`, {
      method: 'PUT',
      headers: {
        'Authorization': `Bearer ${this.authToken}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(data),
    });
    
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
    return response.json();
  }

  async deleteTicket(id: number): Promise<void> {
    const response = await fetch(`${this.baseUrl}/api/v1/tickets/${id}`, {
      method: 'DELETE',
      headers: {
        'Authorization': `Bearer ${this.authToken}`,
      },
    });
    
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
  }

  async addArticle(ticketId: number, data: {
    body: string;
    subject?: string;
    is_visible_for_customer?: number;
  }): Promise<any> {
    const response = await fetch(`${this.baseUrl}/api/v1/tickets/${ticketId}/articles`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${this.authToken}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(data),
    });
    
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
    return response.json();
  }
}

describe('Tickets API Contract Tests', () => {
  // Configure Pact
  const provider = new Pact({
    consumer: 'gotrs-frontend',
    provider: 'gotrs-backend',
    port: 1235,
    log: resolve(process.cwd(), 'logs', 'pact-tickets.log'),
    dir: resolve(process.cwd(), 'pacts'),
    logLevel: 'info',
  });

  let client: TicketsApiClient;
  const validAuthToken = 'valid-jwt-token';

  beforeAll(async () => {
    await provider.setup();
    client = new TicketsApiClient('http://localhost:1235', validAuthToken);
  });

  afterAll(async () => {
    await provider.finalize();
  });

  afterEach(async () => {
    await provider.verify();
  });

  describe('GET /api/v1/tickets', () => {
    it('should return a list of tickets', async () => {
      const expectedResponse = {
        tickets: eachLike({
          id: like(1),
          tn: term({
            generate: '20240101-000001',
            matcher: '\\d{8}-\\d{6}'
          }),
          title: like('Example Ticket'),
          queue_id: like(1),
          ticket_state_id: like(1),
          ticket_priority_id: like(3),
          create_time: like('2024-01-01T10:00:00Z'),
          change_time: like('2024-01-01T10:00:00Z'),
        }),
        total: like(50),
        page: like(1),
        per_page: like(25),
        total_pages: like(2),
      };

      await provider.addInteraction({
        state: 'tickets exist',
        uponReceiving: 'a request for ticket list',
        withRequest: {
          method: 'GET',
          path: '/api/v1/tickets',
          headers: {
            'Authorization': `Bearer ${validAuthToken}`,
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

      const result = await client.getTickets();

      expect(result).toHaveProperty('tickets');
      expect(result).toHaveProperty('total');
      expect(result).toHaveProperty('page');
      expect(result).toHaveProperty('per_page');
      expect(result).toHaveProperty('total_pages');
      expect(Array.isArray(result.tickets)).toBe(true);
    });

    it('should filter tickets by queue', async () => {
      const expectedResponse = {
        tickets: eachLike({
          id: like(1),
          tn: like('20240101-000001'),
          title: like('Filtered Ticket'),
          queue_id: 2,
          ticket_state_id: like(1),
          ticket_priority_id: like(3),
        }),
        total: like(10),
        page: 1,
        per_page: 25,
        total_pages: 1,
      };

      await provider.addInteraction({
        state: 'tickets exist in queue 2',
        uponReceiving: 'a request for tickets filtered by queue',
        withRequest: {
          method: 'GET',
          path: '/api/v1/tickets',
          query: {
            queue_id: '2',
          },
          headers: {
            'Authorization': `Bearer ${validAuthToken}`,
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

      const result = await client.getTickets({ queue_id: 2 });

      expect(result.tickets).toBeDefined();
      expect(result.tickets.length).toBeGreaterThan(0);
    });

    it('should handle unauthorized requests', async () => {
      await provider.addInteraction({
        state: 'user is not authenticated',
        uponReceiving: 'a request without valid auth token',
        withRequest: {
          method: 'GET',
          path: '/api/v1/tickets',
          headers: {
            'Authorization': 'Bearer invalid-token',
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

      const unauthorizedClient = new TicketsApiClient('http://localhost:1235', 'invalid-token');
      await expect(unauthorizedClient.getTickets()).rejects.toThrow('HTTP 401');
    });
  });

  describe('GET /api/v1/tickets/:id', () => {
    it('should return a single ticket', async () => {
      const expectedResponse = {
        id: 1,
        tn: '20240101-000001',
        title: 'Test Ticket',
        queue_id: 1,
        ticket_state_id: 1,
        ticket_priority_id: 3,
        customer_id: 5,
        create_time: '2024-01-01T10:00:00Z',
        change_time: '2024-01-01T10:00:00Z',
        queue: {
          id: 1,
          name: 'General Support',
        },
        state: {
          id: 1,
          name: 'New',
          type_id: 1,
        },
        priority: {
          id: 3,
          name: 'Normal',
        },
      };

      await provider.addInteraction({
        state: 'ticket with id 1 exists',
        uponReceiving: 'a request for a specific ticket',
        withRequest: {
          method: 'GET',
          path: '/api/v1/tickets/1',
          headers: {
            'Authorization': `Bearer ${validAuthToken}`,
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

      const result = await client.getTicket(1);

      expect(result.id).toBe(1);
      expect(result.tn).toBe('20240101-000001');
      expect(result.title).toBe('Test Ticket');
      expect(result.queue).toBeDefined();
      expect(result.state).toBeDefined();
      expect(result.priority).toBeDefined();
    });

    it('should handle ticket not found', async () => {
      await provider.addInteraction({
        state: 'ticket with id 999 does not exist',
        uponReceiving: 'a request for a non-existent ticket',
        withRequest: {
          method: 'GET',
          path: '/api/v1/tickets/999',
          headers: {
            'Authorization': `Bearer ${validAuthToken}`,
          },
        },
        willRespondWith: {
          status: 404,
          headers: {
            'Content-Type': 'application/json',
          },
          body: {
            error: 'Ticket not found',
          },
        },
      });

      await expect(client.getTicket(999)).rejects.toThrow('HTTP 404');
    });
  });

  describe('POST /api/v1/tickets', () => {
    it('should create a new ticket', async () => {
      const requestBody = {
        title: 'New Support Request',
        queue_id: 1,
        priority_id: 3,
        body: 'I need help with my account',
        subject: 'Account Issue',
      };

      const expectedResponse = {
        id: like(2),
        tn: term({
          generate: '20240101-000002',
          matcher: '\\d{8}-\\d{6}'
        }),
        title: 'New Support Request',
        queue_id: 1,
        ticket_state_id: 1, // New tickets start as "New"
        ticket_priority_id: 3,
        create_time: like('2024-01-01T11:00:00Z'),
        change_time: like('2024-01-01T11:00:00Z'),
      };

      await provider.addInteraction({
        state: 'user is authenticated',
        uponReceiving: 'a request to create a ticket',
        withRequest: {
          method: 'POST',
          path: '/api/v1/tickets',
          headers: {
            'Authorization': `Bearer ${validAuthToken}`,
            'Content-Type': 'application/json',
          },
          body: requestBody,
        },
        willRespondWith: {
          status: 201,
          headers: {
            'Content-Type': 'application/json',
          },
          body: expectedResponse,
        },
      });

      const result = await client.createTicket(requestBody);

      expect(result.id).toBeDefined();
      expect(result.tn).toMatch(/\d{8}-\d{6}/);
      expect(result.title).toBe('New Support Request');
      expect(result.ticket_state_id).toBe(1);
    });

    it('should handle validation errors', async () => {
      const invalidRequest = {
        // Missing required fields
        queue_id: 1,
      };

      await provider.addInteraction({
        state: 'user is authenticated',
        uponReceiving: 'an invalid ticket creation request',
        withRequest: {
          method: 'POST',
          path: '/api/v1/tickets',
          headers: {
            'Authorization': `Bearer ${validAuthToken}`,
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
              title: 'Title is required',
              priority_id: 'Priority is required',
              body: 'Body is required',
            },
          },
        },
      });

      await expect(client.createTicket(invalidRequest as any)).rejects.toThrow('HTTP 400');
    });
  });

  describe('PUT /api/v1/tickets/:id', () => {
    it('should update a ticket', async () => {
      const updateData = {
        title: 'Updated Title',
        state_id: 2, // Change to "Open"
      };

      const expectedResponse = {
        id: 1,
        tn: '20240101-000001',
        title: 'Updated Title',
        queue_id: 1,
        ticket_state_id: 2,
        ticket_priority_id: 3,
        change_time: like('2024-01-01T12:00:00Z'),
      };

      await provider.addInteraction({
        state: 'ticket with id 1 exists',
        uponReceiving: 'a request to update a ticket',
        withRequest: {
          method: 'PUT',
          path: '/api/v1/tickets/1',
          headers: {
            'Authorization': `Bearer ${validAuthToken}`,
            'Content-Type': 'application/json',
          },
          body: updateData,
        },
        willRespondWith: {
          status: 200,
          headers: {
            'Content-Type': 'application/json',
          },
          body: expectedResponse,
        },
      });

      const result = await client.updateTicket(1, updateData);

      expect(result.title).toBe('Updated Title');
      expect(result.ticket_state_id).toBe(2);
    });
  });

  describe('DELETE /api/v1/tickets/:id', () => {
    it('should delete a ticket', async () => {
      await provider.addInteraction({
        state: 'ticket with id 1 exists and user has permission',
        uponReceiving: 'a request to delete a ticket',
        withRequest: {
          method: 'DELETE',
          path: '/api/v1/tickets/1',
          headers: {
            'Authorization': `Bearer ${validAuthToken}`,
          },
        },
        willRespondWith: {
          status: 204,
        },
      });

      await expect(client.deleteTicket(1)).resolves.toBeUndefined();
    });

    it('should handle forbidden deletion', async () => {
      await provider.addInteraction({
        state: 'user does not have permission to delete ticket',
        uponReceiving: 'a request to delete a ticket without permission',
        withRequest: {
          method: 'DELETE',
          path: '/api/v1/tickets/1',
          headers: {
            'Authorization': `Bearer ${validAuthToken}`,
          },
        },
        willRespondWith: {
          status: 403,
          headers: {
            'Content-Type': 'application/json',
          },
          body: {
            error: 'Forbidden: You do not have permission to delete this ticket',
          },
        },
      });

      await expect(client.deleteTicket(1)).rejects.toThrow('HTTP 403');
    });
  });

  describe('POST /api/v1/tickets/:id/articles', () => {
    it('should add an article to a ticket', async () => {
      const articleData = {
        body: 'This is a follow-up message',
        subject: 'Re: Test Ticket',
        is_visible_for_customer: 1,
      };

      const expectedResponse = {
        id: like(1),
        ticket_id: 1,
        body: 'This is a follow-up message',
        subject: 'Re: Test Ticket',
        is_visible_for_customer: 1,
        article_type_id: 7, // Note internal
        sender_type_id: 1, // Agent
        create_time: like('2024-01-01T13:00:00Z'),
      };

      await provider.addInteraction({
        state: 'ticket with id 1 exists',
        uponReceiving: 'a request to add an article to a ticket',
        withRequest: {
          method: 'POST',
          path: '/api/v1/tickets/1/articles',
          headers: {
            'Authorization': `Bearer ${validAuthToken}`,
            'Content-Type': 'application/json',
          },
          body: articleData,
        },
        willRespondWith: {
          status: 201,
          headers: {
            'Content-Type': 'application/json',
          },
          body: expectedResponse,
        },
      });

      const result = await client.addArticle(1, articleData);

      expect(result.id).toBeDefined();
      expect(result.ticket_id).toBe(1);
      expect(result.body).toBe('This is a follow-up message');
      expect(result.subject).toBe('Re: Test Ticket');
    });
  });
});

// Export for use in other tests
export { TicketsApiClient };