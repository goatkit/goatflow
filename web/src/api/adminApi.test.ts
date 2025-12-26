/**
 * Unit tests for adminApi module.
 * 
 * These tests verify that:
 * 1. All API calls include required headers (Accept: application/json)
 * 2. POST/PUT calls include Content-Type: application/json
 * 3. HTML responses trigger proper error messages
 * 4. Auth redirects are handled correctly
 * 
 * This prevents the bug where Go tests pass but browser fails because
 * fetch calls are missing required headers.
 */

import {
  adminFetch,
  handleFetchResponse,
  JSON_REQUEST_HEADERS,
  JSON_BODY_HEADERS,
  rolesApi,
} from './adminApi';

// Mock fetch globally
const mockFetch = jest.fn();
global.fetch = mockFetch;

describe('adminApi', () => {
  beforeEach(() => {
    mockFetch.mockClear();
  });

  describe('JSON_REQUEST_HEADERS', () => {
    it('includes Accept: application/json', () => {
      expect(JSON_REQUEST_HEADERS['Accept']).toBe('application/json');
    });

    it('includes X-Requested-With: XMLHttpRequest', () => {
      expect(JSON_REQUEST_HEADERS['X-Requested-With']).toBe('XMLHttpRequest');
    });
  });

  describe('JSON_BODY_HEADERS', () => {
    it('includes all JSON_REQUEST_HEADERS', () => {
      expect(JSON_BODY_HEADERS['Accept']).toBe('application/json');
      expect(JSON_BODY_HEADERS['X-Requested-With']).toBe('XMLHttpRequest');
    });

    it('includes Content-Type: application/json', () => {
      expect(JSON_BODY_HEADERS['Content-Type']).toBe('application/json');
    });
  });

  describe('adminFetch', () => {
    it('sends Accept: application/json header on GET requests', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({ success: true, data: [] }),
      });

      await adminFetch('/admin/roles');

      expect(mockFetch).toHaveBeenCalledWith('/admin/roles', expect.objectContaining({
        headers: expect.objectContaining({
          'Accept': 'application/json',
        }),
      }));
    });

    it('sends Content-Type header on POST requests with body', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({ success: true, data: { id: 1 } }),
      });

      await adminFetch('/admin/roles', {
        method: 'POST',
        body: { name: 'Test Role' },
      });

      expect(mockFetch).toHaveBeenCalledWith('/admin/roles', expect.objectContaining({
        headers: expect.objectContaining({
          'Accept': 'application/json',
          'Content-Type': 'application/json',
        }),
        body: JSON.stringify({ name: 'Test Role' }),
      }));
    });

    it('throws error when response is HTML instead of JSON', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        headers: new Headers({ 'content-type': 'text/html' }),
      });

      await expect(adminFetch('/admin/roles')).rejects.toThrow(
        'Expected JSON response but got text/html'
      );
    });

    it('throws error with helpful message about Accept header', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        headers: new Headers({ 'content-type': 'text/html; charset=utf-8' }),
      });

      await expect(adminFetch('/admin/roles/1/users')).rejects.toThrow(
        /Accept header was not sent/
      );
    });

    it('returns error response for non-OK status', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 404,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({ success: false, error: 'Role not found' }),
      });

      const result = await adminFetch('/admin/roles/999');
      
      expect(result.success).toBe(false);
      expect(result.error).toBe('Role not found');
    });
  });

  describe('handleFetchResponse', () => {
    it('returns response when content-type is JSON', async () => {
      const mockResponse = {
        ok: true,
        redirected: false,
        headers: new Headers({ 'content-type': 'application/json' }),
      } as Response;

      const result = await handleFetchResponse(mockResponse);
      expect(result).toBe(mockResponse);
    });

    it('throws when content-type is HTML', async () => {
      const mockResponse = {
        ok: true,
        redirected: false,
        headers: new Headers({ 'content-type': 'text/html' }),
      } as Response;

      await expect(handleFetchResponse(mockResponse)).rejects.toThrow(
        /Expected JSON but got text\/html/
      );
    });

    it('redirects to login on auth failure', async () => {
      const originalLocation = window.location;
      delete (window as any).location;
      window.location = { href: '' } as Location;

      const mockResponse = {
        ok: true,
        redirected: true,
        url: 'http://localhost/login?redirect=/admin/roles',
        headers: new Headers({ 'content-type': 'text/html' }),
      } as Response;

      await expect(handleFetchResponse(mockResponse)).rejects.toThrow('Session expired');
      expect(window.location.href).toBe('/login');

      window.location = originalLocation;
    });
  });

  describe('rolesApi', () => {
    const mockJsonResponse = (data: unknown) => ({
      ok: true,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => ({ success: true, data }),
    });

    describe('getUsers', () => {
      it('sends Accept: application/json header', async () => {
        mockFetch.mockResolvedValueOnce(mockJsonResponse([]));

        await rolesApi.getUsers(1);

        // This is THE test that would have caught the bug
        expect(mockFetch).toHaveBeenCalledWith(
          '/admin/roles/1/users',
          expect.objectContaining({
            headers: expect.objectContaining({
              'Accept': 'application/json',
            }),
          })
        );
      });
    });

    describe('create', () => {
      it('sends correct headers and body', async () => {
        mockFetch.mockResolvedValueOnce(mockJsonResponse({ id: 1, name: 'Test' }));

        await rolesApi.create({ name: 'Test', comments: '', valid_id: 1 });

        expect(mockFetch).toHaveBeenCalledWith(
          '/admin/roles',
          expect.objectContaining({
            method: 'POST',
            headers: expect.objectContaining({
              'Accept': 'application/json',
              'Content-Type': 'application/json',
            }),
            body: expect.stringContaining('"name":"Test"'),
          })
        );
      });

      it('uses correct field names (comments not description, valid_id not is_active)', async () => {
        mockFetch.mockResolvedValueOnce(mockJsonResponse({ id: 1 }));

        await rolesApi.create({ name: 'Test', comments: 'A comment', valid_id: 1 });

        const callBody = JSON.parse(mockFetch.mock.calls[0][1].body);
        
        // These are the CORRECT field names per Go handler
        expect(callBody).toHaveProperty('comments');
        expect(callBody).toHaveProperty('valid_id');
        
        // These are the WRONG field names that caused bugs
        expect(callBody).not.toHaveProperty('description');
        expect(callBody).not.toHaveProperty('is_active');
      });
    });

    describe('update', () => {
      it('uses PUT method with correct path', async () => {
        mockFetch.mockResolvedValueOnce(mockJsonResponse({ id: 5 }));

        await rolesApi.update(5, { name: 'Updated' });

        expect(mockFetch).toHaveBeenCalledWith(
          '/admin/roles/5',
          expect.objectContaining({ method: 'PUT' })
        );
      });
    });

    describe('delete', () => {
      it('uses DELETE method with correct path', async () => {
        mockFetch.mockResolvedValueOnce(mockJsonResponse(null));

        await rolesApi.delete(5);

        expect(mockFetch).toHaveBeenCalledWith(
          '/admin/roles/5',
          expect.objectContaining({ method: 'DELETE' })
        );
      });
    });

    describe('addUser', () => {
      it('sends user_id in body', async () => {
        mockFetch.mockResolvedValueOnce(mockJsonResponse(null));

        await rolesApi.addUser(1, 42);

        const callBody = JSON.parse(mockFetch.mock.calls[0][1].body);
        expect(callBody.user_id).toBe(42);
      });
    });

    describe('removeUser', () => {
      it('uses DELETE with correct URL structure', async () => {
        mockFetch.mockResolvedValueOnce(mockJsonResponse(null));

        await rolesApi.removeUser(1, 42);

        expect(mockFetch).toHaveBeenCalledWith(
          '/admin/roles/1/users/42',
          expect.objectContaining({ method: 'DELETE' })
        );
      });
    });
  });
});
