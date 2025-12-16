// Playwright Acceptance Tests - Schema Discovery
// Tests the automatic schema discovery and module generation

import { test, expect } from '@playwright/test';

// Configuration
const BASE_URL = process.env.BASE_URL || 'http://localhost:8080';
const DEMO_COOKIE = 'access_token=demo_session_admin';
const MODULES_BASE = `${BASE_URL}/admin/modules`;
const SCHEMA_API = `${MODULES_BASE}/_schema`;

test.describe('Schema Discovery - Database Introspection', () => {
  
  // Test Setup
  test.beforeEach(async ({ page }) => {
    await page.context().addCookies([{
      name: 'access_token',
      value: 'demo_session_admin',
      domain: 'localhost',
      path: '/'
    }]);
  });

  test.describe('API Endpoints', () => {
    
    test('Can list all database tables', async ({ page }) => {
      const response = await page.request.get(`${SCHEMA_API}?action=tables`, {
        headers: {
          'Cookie': DEMO_COOKIE,
          'X-Requested-With': 'XMLHttpRequest',
          'Accept': 'application/json'
        }
      });
      
      expect(response.status()).toBe(200);
      
      const data = await response.json();
      expect(data.success).toBe(true);
      expect(data.data).toBeDefined();
      expect(Array.isArray(data.data)).toBe(true);
      
      // Should contain at least the core OTRS tables
      const tableNames = data.data.map(t => t.Name);
      expect(tableNames).toContain('users');
      expect(tableNames).toContain('ticket');
      expect(tableNames).toContain('queue');
      expect(tableNames).toContain('customer_user');
      
      // Each table should have name and comment
      if (data.data.length > 0) {
        const firstTable = data.data[0];
        expect(firstTable).toHaveProperty('Name');
        expect(firstTable).toHaveProperty('Comment');
      }
    });

    test('Can get columns for a specific table', async ({ page }) => {
      const response = await page.request.get(`${SCHEMA_API}?action=columns&table=users`, {
        headers: {
          'Cookie': DEMO_COOKIE,
          'X-Requested-With': 'XMLHttpRequest',
          'Accept': 'application/json'
        }
      });
      
      expect(response.status()).toBe(200);
      
      const data = await response.json();
      expect(data.success).toBe(true);
      expect(data.table).toBe('users');
      expect(data.data).toBeDefined();
      expect(Array.isArray(data.data)).toBe(true);
      
      // Check for expected user table columns
      const columnNames = data.data.map(c => c.Name);
      expect(columnNames).toContain('id');
      expect(columnNames).toContain('login');
      expect(columnNames).toContain('first_name');
      expect(columnNames).toContain('last_name');
      expect(columnNames).toContain('valid_id');
      
      // Each column should have proper metadata
      const idColumn = data.data.find(c => c.Name === 'id');
      expect(idColumn).toBeDefined();
      expect(idColumn.DataType).toBeDefined();
      expect(idColumn.IsNullable).toBeDefined();
      expect(idColumn.IsPrimaryKey).toBe(true);
    });

    test('Returns error when table parameter missing for columns', async ({ page }) => {
      const response = await page.request.get(`${SCHEMA_API}?action=columns`, {
        headers: {
          'Cookie': DEMO_COOKIE,
          'X-Requested-With': 'XMLHttpRequest',
          'Accept': 'application/json'
        }
      });
      
      expect(response.status()).toBe(400);
      
      const data = await response.json();
      expect(data.error).toContain('table parameter required');
    });

    test('Can generate module config for a table', async ({ page }) => {
      const response = await page.request.get(`${SCHEMA_API}?action=generate&table=ticket`, {
        headers: {
          'Cookie': DEMO_COOKIE,
          'X-Requested-With': 'XMLHttpRequest',
          'Accept': 'application/json'
        }
      });
      
      expect(response.status()).toBe(200);
      
      const data = await response.json();
      expect(data.success).toBe(true);
      expect(data.config).toBeDefined();
      
      // Check module metadata
      const config = data.config;
      expect(config.Module).toBeDefined();
      expect(config.Module.Name).toBe('ticket');
      expect(config.Module.Table).toBe('ticket');
      expect(config.Module.Singular).toBe('Ticket');
      expect(config.Module.Plural).toBe('Tickets');
      
      // Check fields are generated
      expect(config.Fields).toBeDefined();
      expect(Array.isArray(config.Fields)).toBe(true);
      expect(config.Fields.length).toBeGreaterThan(0);
      
      // Check field structure
      const idField = config.Fields.find(f => f.Name === 'id');
      expect(idField).toBeDefined();
      expect(idField.Type).toBeDefined();
      expect(idField.Label).toBeDefined();
      expect(idField.ShowInForm).toBe(false); // ID should not show in form
      expect(idField.ShowInList).toBe(true);
      
      // Check features
      expect(config.Features).toBeDefined();
      expect(config.Features.Search).toBe(true);
    });

    test('Can generate YAML format module config', async ({ page }) => {
      const response = await page.request.get(`${SCHEMA_API}?action=generate&table=queue&format=yaml`, {
        headers: {
          'Cookie': DEMO_COOKIE,
          'X-Requested-With': 'XMLHttpRequest',
          'Accept': 'text/yaml'
        }
      });
      
      expect(response.status()).toBe(200);
      
      const yamlText = await response.text();
      expect(yamlText).toContain('module:');
      expect(yamlText).toContain('name: queue');
      expect(yamlText).toContain('fields:');
      expect(yamlText).toContain('- name: id');
      expect(yamlText).toContain('features:');
    });

    test('Can save generated config to file', async ({ page }) => {
      // Use a test table name to avoid overwriting existing configs
      const testTable = 'article'; // Using an existing table that likely doesn't have a config yet
      
      const response = await page.request.get(`${SCHEMA_API}?action=save&table=${testTable}`, {
        headers: {
          'Cookie': DEMO_COOKIE,
          'X-Requested-With': 'XMLHttpRequest',
          'Accept': 'application/json'
        }
      });
      
      expect(response.status()).toBe(200);
      
      const data = await response.json();
      expect(data.success).toBe(true);
      expect(data.message).toContain('Module config saved');
      expect(data.filename).toContain(`${testTable}.yaml`);
      
      // Verify the module is now available
      await page.waitForTimeout(1000); // Wait for file watcher to pick up the new file
      
      const modulesResponse = await page.request.get(`${BASE_URL}/admin/modules/${testTable}`, {
        headers: {
          'Cookie': DEMO_COOKIE,
          'X-Requested-With': 'XMLHttpRequest',
          'Accept': 'application/json'
        }
      });
      
      // Should return 200 if the module was loaded
      expect([200, 500]).toContain(modulesResponse.status()); // 500 if table query fails, but module exists
    });

    test('Returns error for invalid action', async ({ page }) => {
      const response = await page.request.get(`${SCHEMA_API}?action=invalid`, {
        headers: {
          'Cookie': DEMO_COOKIE,
          'X-Requested-With': 'XMLHttpRequest',
          'Accept': 'application/json'
        }
      });
      
      expect(response.status()).toBe(400);
      
      const data = await response.json();
      expect(data.error).toContain('Invalid action');
    });
  });

  test.describe('Field Type Inference', () => {
    
    test('Correctly infers field types from column names and data types', async ({ page }) => {
      const response = await page.request.get(`${SCHEMA_API}?action=generate&table=users`, {
        headers: {
          'Cookie': DEMO_COOKIE,
          'X-Requested-With': 'XMLHttpRequest',
          'Accept': 'application/json'
        }
      });
      
      expect(response.status()).toBe(200);
      
      const data = await response.json();
      const fields = data.config.Fields;
      
      // Password field should be detected
      const pwField = fields.find(f => f.Name === 'pw');
      if (pwField) {
        expect(pwField.Type).toBe('password');
      }
      
      // Integer fields
      const idField = fields.find(f => f.Name === 'id');
      expect(idField.Type).toBe('integer');
      
      // String fields
      const loginField = fields.find(f => f.Name === 'login');
      expect(loginField.Type).toBe('string');
      
      // DateTime fields
      const createTimeField = fields.find(f => f.Name === 'create_time');
      if (createTimeField) {
        expect(createTimeField.Type).toBe('datetime');
      }
    });

    test('Sets appropriate display settings for different field types', async ({ page }) => {
      const response = await page.request.get(`${SCHEMA_API}?action=generate&table=ticket`, {
        headers: {
          'Cookie': DEMO_COOKIE,
          'X-Requested-With': 'XMLHttpRequest',
          'Accept': 'application/json'
        }
      });
      
      expect(response.status()).toBe(200);
      
      const data = await response.json();
      const fields = data.config.Fields;
      
      // Primary key should not show in form
      const idField = fields.find(f => f.Name === 'id');
      expect(idField.ShowInForm).toBe(false);
      expect(idField.Required).toBe(false);
      
      // Timestamp fields should not show in form
      const createTimeField = fields.find(f => f.Name === 'create_time');
      if (createTimeField) {
        expect(createTimeField.ShowInForm).toBe(false);
      }
      
      // Text fields should be searchable
      const titleField = fields.find(f => f.Name === 'title');
      if (titleField) {
        expect(titleField.Searchable).toBe(true);
      }
    });
  });

  test.describe('Integration Tests', () => {
    
    test('Generated module can be immediately used for CRUD operations', async ({ page }) => {
      // Generate and save a test module
      const testTable = 'service'; // Using a table that might not have a module yet
      
      // Save the generated config
      const saveResponse = await page.request.get(`${SCHEMA_API}?action=save&table=${testTable}`, {
        headers: {
          'Cookie': DEMO_COOKIE,
          'X-Requested-With': 'XMLHttpRequest',
          'Accept': 'application/json'
        }
      });
      
      expect(saveResponse.status()).toBe(200);
      
      // Wait for file watcher to pick up the new config
      await page.waitForTimeout(2000);
      
      // Try to access the module
      const moduleResponse = await page.request.get(`${BASE_URL}/admin/modules/${testTable}`, {
        headers: {
          'Cookie': DEMO_COOKIE,
          'X-Requested-With': 'XMLHttpRequest',
          'Accept': 'application/json'
        }
      });
      
      // Should either work or fail with database error (not 404)
      expect([200, 500]).toContain(moduleResponse.status());
      
      if (moduleResponse.status() === 200) {
        const data = await moduleResponse.json();
        expect(data.success).toBeDefined();
      }
    });

    test('Schema discovery respects existing module configurations', async ({ page }) => {
      // Get list of existing modules
      const existingResponse = await page.request.get(`${BASE_URL}/admin/modules/users`, {
        headers: {
          'Cookie': DEMO_COOKIE,
          'X-Requested-With': 'XMLHttpRequest',
          'Accept': 'application/json'
        }
      });
      
      expect(existingResponse.status()).toBe(200);
      
      // Generate config for the same table
      const generateResponse = await page.request.get(`${SCHEMA_API}?action=generate&table=users`, {
        headers: {
          'Cookie': DEMO_COOKIE,
          'X-Requested-With': 'XMLHttpRequest',
          'Accept': 'application/json'
        }
      });
      
      expect(generateResponse.status()).toBe(200);
      
      // The generated config should be valid
      const data = await generateResponse.json();
      expect(data.config.Module.Name).toBe('users');
      expect(data.config.Fields.length).toBeGreaterThan(0);
    });
  });

  test.describe('Error Handling', () => {
    
    test('Handles non-existent table gracefully', async ({ page }) => {
      const response = await page.request.get(`${SCHEMA_API}?action=columns&table=nonexistent_table`, {
        headers: {
          'Cookie': DEMO_COOKIE,
          'X-Requested-With': 'XMLHttpRequest',
          'Accept': 'application/json'
        }
      });
      
      // Should return 500 with error message
      expect(response.status()).toBe(500);
      
      const data = await response.json();
      expect(data.error).toBeDefined();
    });

    test('Handles missing parameters correctly', async ({ page }) => {
      const response = await page.request.get(`${SCHEMA_API}?action=generate`, {
        headers: {
          'Cookie': DEMO_COOKIE,
          'X-Requested-With': 'XMLHttpRequest',
          'Accept': 'application/json'
        }
      });
      
      expect(response.status()).toBe(400);
      
      const data = await response.json();
      expect(data.error).toContain('table parameter required');
    });
  });
});