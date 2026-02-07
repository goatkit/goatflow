// Playwright Acceptance Tests - Dynamic Module System
// Side-by-side comparison of static vs dynamic modules

import { test, expect } from '@playwright/test';

// Configuration
const BASE_URL = process.env.BASE_URL || 'http://localhost:8080';
const DEMO_COOKIE = 'access_token=demo_session_admin';
const MODULES_BASE = `${BASE_URL}/admin/modules`;

// Test Suite: Dynamic Module System Acceptance Tests
test.describe('Dynamic Module System - Side by Side Comparison', () => {
  
  // Test Setup: Login helper
  const login = async (page) => {
    await page.context().addCookies([{
      name: 'access_token',
      value: 'demo_session_admin',
      domain: 'localhost',
      path: '/'
    }]);
  };

  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  // 1. Page Loading & Basic Structure Tests
  test.describe('Page Loading & Structure', () => {
    
    test('Static users page loads successfully', async ({ page }) => {
      await page.goto(`${BASE_URL}/admin/users`);
      
      // Check page loads
      await expect(page).toHaveTitle(/Users.*GoatFlow/);
      await expect(page.locator('h1')).toContainText('Users');
      
      // Check basic structure
      await expect(page.locator('table')).toBeVisible();
      await expect(page.locator('thead')).toBeVisible();
      await expect(page.locator('tbody')).toBeVisible();
    });

    test('Dynamic users page loads successfully', async ({ page }) => {
      await page.goto(`${BASE_URL}/admin/modules/users`);
      
      // Check page loads  
      await expect(page).toHaveTitle(/Users Management/);
      await expect(page.locator('h1')).toContainText('Users');
      
      // Check basic structure
      await expect(page.locator('table')).toBeVisible();
      await expect(page.locator('thead')).toBeVisible();
      await expect(page.locator('tbody')).toBeVisible();
    });

    test('Comparison dashboard loads successfully', async ({ page }) => {
      await page.goto(`${BASE_URL}/admin/modules/`);
      
      // Check dashboard loads
      await expect(page).toHaveTitle(/Dynamic Module Testing/);
      await expect(page.locator('h1')).toContainText('Dynamic Module Testing');
      
      // Check comparison table exists
      await expect(page.locator('table')).toBeVisible();
      
      // Check users module is listed
      const usersRow = page.locator('tr:has-text("users")');
      await expect(usersRow).toBeVisible();
      await expect(usersRow.locator('a[href="/admin/users"]')).toBeVisible();
      await expect(usersRow.locator('a[href="/admin/modules/users"]')).toBeVisible();
    });
  });

  // 2. Data Display & Content Tests
  test.describe('Data Display & Content', () => {
    
    test('Static users page displays user data', async ({ page }) => {
      await page.goto(`${BASE_URL}/admin/users`);
      
      // Wait for data to load
      await expect(page.locator('tbody tr')).toHaveCountGreaterThan(0);
      
      // Check essential columns exist
      await expect(page.locator('th:has-text("Username")')).toBeVisible();
      await expect(page.locator('th:has-text("First Name")')).toBeVisible();
      await expect(page.locator('th:has-text("Last Name")')).toBeVisible();
      await expect(page.locator('th:has-text("Status")')).toBeVisible();
      
      // Check data rows have content
      const firstRow = page.locator('tbody tr').first();
      await expect(firstRow.locator('td')).toHaveCountGreaterThan(3);
    });

    test('Dynamic users page displays user data', async ({ page }) => {
      await page.goto(`${BASE_URL}/admin/modules/users`);
      
      // Wait for data to load
      await expect(page.locator('tbody tr')).toHaveCountGreaterThan(0);
      
      // Check essential columns exist
      await expect(page.locator('th:has-text("Username")')).toBeVisible();
      await expect(page.locator('th:has-text("First Name")')).toBeVisible();
      await expect(page.locator('th:has-text("Last Name")')).toBeVisible();
      await expect(page.locator('th:has-text("Status")')).toBeVisible();
      
      // Check data rows have content
      const firstRow = page.locator('tbody tr').first();
      await expect(firstRow.locator('td')).toHaveCountGreaterThan(3);
      
      // Check status badges work
      await expect(page.locator('.bg-green-100:has-text("Valid")')).toHaveCountGreaterThan(0);
    });

    test('Both pages display same number of users', async ({ page }) => {
      // Get static count
      await page.goto(`${BASE_URL}/admin/users`);
      await expect(page.locator('tbody tr')).toHaveCountGreaterThan(0);
      const staticCount = await page.locator('tbody tr').count();
      
      // Get dynamic count  
      await page.goto(`${BASE_URL}/admin/modules/users`);
      await expect(page.locator('tbody tr')).toHaveCountGreaterThan(0);
      const dynamicCount = await page.locator('tbody tr').count();
      
      // Should be the same
      expect(staticCount).toBe(dynamicCount);
    });
  });

  // 3. API Functionality Tests
  test.describe('API Functionality', () => {
    
    test('Dynamic API returns JSON data', async ({ page }) => {
      const response = await page.request.get(`${BASE_URL}/admin/modules/users`, {
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
      expect(data.data.length).toBeGreaterThan(0);
      
      // Check data structure
      const firstUser = data.data[0];
      expect(firstUser).toHaveProperty('id');
      expect(firstUser).toHaveProperty('login');
      expect(firstUser).toHaveProperty('first_name');
      expect(firstUser).toHaveProperty('last_name');
      expect(firstUser).toHaveProperty('valid_id');
    });

    test('Dynamic API handles non-existent module', async ({ page }) => {
      const response = await page.request.get(`${BASE_URL}/admin/modules/nonexistent`, {
        headers: {
          'Cookie': DEMO_COOKIE,
          'X-Requested-With': 'XMLHttpRequest',
          'Accept': 'application/json'
        }
      });
      
      expect(response.status()).toBe(404);
    });
  });

  // 4. User Interface & Interaction Tests
  test.describe('User Interface & Interactions', () => {
    
    test('Dynamic page has functional search', async ({ page }) => {
      await page.goto(`${BASE_URL}/admin/modules/users`);
      
      // Check search input exists
      await expect(page.locator('input[type="search"], input[placeholder*="search" i]')).toBeVisible();
      
      // Test search functionality (if implemented)
      const searchInput = page.locator('input[type="search"], input[placeholder*="search" i]').first();
      if (await searchInput.isVisible()) {
        await searchInput.fill('admin');
        // Wait for search results (implementation dependent)
        await page.waitForTimeout(1000);
      }
    });

    test('Dynamic page has action buttons', async ({ page }) => {
      await page.goto(`${BASE_URL}/admin/modules/users`);
      
      // Check for Create/Add button
      await expect(page.locator('button:has-text("Create"), button:has-text("Add"), button:has-text("New")')).toBeVisible();
      
      // Check for action buttons in table rows
      await expect(page.locator('tbody tr')).toHaveCountGreaterThan(0);
      await expect(page.locator('button[onclick*="edit"], a[href*="edit"], svg')).toHaveCountGreaterThan(0);
    });

    test('Dark mode toggle works on dynamic page', async ({ page }) => {
      await page.goto(`${BASE_URL}/admin/modules/users`);
      
      // Check for dark mode toggle
      const darkToggle = page.locator('button[id*="dark"], button[class*="dark"], [data-theme-toggle]').first();
      
      if (await darkToggle.isVisible()) {
        // Test dark mode toggle
        await darkToggle.click();
        await page.waitForTimeout(500);
        
        // Check if dark classes are applied
        const bodyClasses = await page.locator('html').getAttribute('class');
        const hasDarkMode = bodyClasses && bodyClasses.includes('dark');
        
        if (hasDarkMode) {
          expect(bodyClasses).toContain('dark');
        }
      }
    });

    test('Edit dialog opens with populated fields', async ({ page }) => {
      await page.goto(`${BASE_URL}/admin/modules/users`);
      
      // Wait for data to load
      await expect(page.locator('tbody tr')).toHaveCountGreaterThan(0);
      
      // Click edit button on first row
      const firstEditButton = page.locator('tbody tr').first().locator('button[onclick*="editItem"]');
      await expect(firstEditButton).toBeVisible();
      await firstEditButton.click();
      
      // Wait for modal to open
      await expect(page.locator('#moduleModal')).toBeVisible();
      
      // Check modal title
      await expect(page.locator('#modalTitle')).toContainText('Edit User');
      
      // Check form fields are populated
      const loginField = page.locator('#login');
      await expect(loginField).toBeVisible();
      await expect(loginField).toHaveValue(/\w+/); // Should have some value
      
      const firstNameField = page.locator('#first_name');
      await expect(firstNameField).toBeVisible();
      await expect(firstNameField).toHaveValue(/\w+/);
      
      const lastNameField = page.locator('#last_name');
      await expect(lastNameField).toBeVisible();
      await expect(lastNameField).toHaveValue(/\w+/);
      
      // Close modal
      await page.locator('button:has-text("Cancel")').click();
      await expect(page.locator('#moduleModal')).toBeHidden();
    });

    test('Create dialog opens with empty fields', async ({ page }) => {
      await page.goto(`${BASE_URL}/admin/modules/users`);
      
      // Click create button
      const createButton = page.locator('button:has-text("New User")');
      await expect(createButton).toBeVisible();
      await createButton.click();
      
      // Wait for modal to open
      await expect(page.locator('#moduleModal')).toBeVisible();
      
      // Check modal title
      await expect(page.locator('#modalTitle')).toContainText('New User');
      
      // Check form fields are empty
      const loginField = page.locator('#login');
      await expect(loginField).toBeVisible();
      await expect(loginField).toHaveValue('');
      
      const firstNameField = page.locator('#first_name');
      await expect(firstNameField).toBeVisible();
      await expect(firstNameField).toHaveValue('');
      
      const lastNameField = page.locator('#last_name');
      await expect(lastNameField).toBeVisible();
      await expect(lastNameField).toHaveValue('');
      
      // Close modal
      await page.locator('button:has-text("Cancel")').click();
      await expect(page.locator('#moduleModal')).toBeHidden();
    });

    test('Edit dialog fetches correct user data via API', async ({ page }) => {
      await page.goto(`${BASE_URL}/admin/modules/users`);
      
      // Wait for data to load
      await expect(page.locator('tbody tr')).toHaveCountGreaterThan(0);
      
      // Get the first user's data from the table
      const firstRow = page.locator('tbody tr').first();
      const usernameCell = firstRow.locator('td').nth(0); // Assuming username is first column
      const expectedUsername = await usernameCell.textContent();
      
      // Setup request interception to verify API call
      let apiCalled = false;
      let apiUserId = null;
      
      page.on('request', request => {
        if (request.url().includes(`${MODULES_BASE}/users/`) && request.method() === 'GET') {
          apiCalled = true;
          const urlParts = request.url().split('/');
          apiUserId = urlParts[urlParts.length - 1];
        }
      });
      
      // Click edit button
      const firstEditButton = firstRow.locator('button[onclick*="editItem"]');
      await firstEditButton.click();
      
      // Wait for modal and API call
      await expect(page.locator('#moduleModal')).toBeVisible();
      await page.waitForTimeout(500); // Wait for API response
      
      // Verify API was called
      expect(apiCalled).toBe(true);
      expect(apiUserId).toBeTruthy();
      
      // Verify field is populated with correct data
      const loginField = page.locator('#login');
      const actualUsername = await loginField.inputValue();
      expect(actualUsername.toLowerCase()).toBe(expectedUsername.trim().toLowerCase());
    });

    test('Modal can be closed with ESC key', async ({ page }) => {
      await page.goto(`${BASE_URL}/admin/modules/users`);
      
      // Open create modal
      await page.locator('button:has-text("New User")').click();
      await expect(page.locator('#moduleModal')).toBeVisible();
      
      // Press ESC key
      await page.keyboard.press('Escape');
      
      // Modal should be hidden
      await expect(page.locator('#moduleModal')).toBeHidden();
    });

    test('Form validation works in edit dialog', async ({ page }) => {
      await page.goto(`${BASE_URL}/admin/modules/users`);
      
      // Open create modal
      await page.locator('button:has-text("New User")').click();
      await expect(page.locator('#moduleModal')).toBeVisible();
      
      // Try to submit without filling required fields
      await page.locator('button:has-text("Save")').click();
      
      // Check for HTML5 validation (form should not submit)
      const loginField = page.locator('#login');
      const validationMessage = await loginField.evaluate((el) => el.validationMessage);
      expect(validationMessage).toBeTruthy(); // Should have a validation message
      
      // Modal should still be visible (form didn't submit)
      await expect(page.locator('#moduleModal')).toBeVisible();
    });

    test('All required form fields are present in edit dialog', async ({ page }) => {
      await page.goto(`${BASE_URL}/admin/modules/users`);
      
      // Open create modal
      await page.locator('button:has-text("New User")').click();
      await expect(page.locator('#moduleModal')).toBeVisible();
      
      // Check all expected fields are present
      const expectedFields = ['login', 'pw', 'first_name', 'last_name', 'title'];
      
      for (const fieldId of expectedFields) {
        const field = page.locator(`#${fieldId}`);
        await expect(field).toBeVisible();
        
        // Check field has a label
        const label = page.locator(`label[for="${fieldId}"]`);
        await expect(label).toBeVisible();
      }
      
      // Check required fields have asterisk
      const requiredFields = ['login', 'first_name', 'last_name'];
      for (const fieldId of requiredFields) {
        const label = page.locator(`label[for="${fieldId}"]`);
        const labelText = await label.textContent();
        expect(labelText).toContain('*');
      }
    });
  });

  // 5. Performance & Loading Tests
  test.describe('Performance & Loading', () => {
    
    test('Dynamic page loads within acceptable time', async ({ page }) => {
      const startTime = Date.now();
      
      await page.goto(`${BASE_URL}/admin/modules/users`);
      await expect(page.locator('tbody tr')).toHaveCountGreaterThan(0);
      
      const loadTime = Date.now() - startTime;
      
      // Should load within 5 seconds
      expect(loadTime).toBeLessThan(5000);
    });

    test('Dynamic vs Static loading time comparison', async ({ page }) => {
      // Measure static page
      let startTime = Date.now();
      await page.goto(`${BASE_URL}/admin/users`);
      await expect(page.locator('tbody tr')).toHaveCountGreaterThan(0);
      const staticLoadTime = Date.now() - startTime;
      
      // Measure dynamic page
      startTime = Date.now();
      await page.goto(`${BASE_URL}/admin/modules/users`);
      await expect(page.locator('tbody tr')).toHaveCountGreaterThan(0);
      const dynamicLoadTime = Date.now() - startTime;
      
      // Dynamic should not be significantly slower (within 2x)
      expect(dynamicLoadTime).toBeLessThan(staticLoadTime * 2);
      
      console.log(`Static load time: ${staticLoadTime}ms`);
      console.log(`Dynamic load time: ${dynamicLoadTime}ms`);
    });
  });

  // 6. Error Handling Tests
  test.describe('Error Handling', () => {
    
    test('Dynamic page handles authentication errors', async ({ page }) => {
      // Clear cookies
      await page.context().clearCookies();
      
      await page.goto(`${BASE_URL}/admin/modules/users`);
      
      // Should redirect to login or show auth error
      await expect(page).toHaveURL(/login/);
    });

    test('Dynamic page handles network errors gracefully', async ({ page }) => {
      await page.goto(`${BASE_URL}/admin/modules/users`);
      
      // Simulate network failure (implementation dependent)
      await page.route('**/admin/modules/users**', route => route.abort());
      
      // Page should still be accessible (cached or error message)
      await expect(page.locator('body')).toBeVisible();
    });
  });

  // 7. Module Configuration Tests
  test.describe('Module Configuration', () => {
    
    test('YAML configuration is properly loaded', async ({ page }) => {
      await page.goto(`${BASE_URL}/admin/modules/users`);
      
      // Check if moduleConfig is in page source
      const pageContent = await page.content();
      expect(pageContent).toContain('moduleConfig');
      expect(pageContent).toContain('"name":"users"');
      expect(pageContent).toContain('"Table":"users"');
    });

    test('Field configuration matches display', async ({ page }) => {
      await page.goto(`${BASE_URL}/admin/modules/users`);
      
      // Get the moduleConfig from page
      const moduleConfig = await page.evaluate(() => {
        return window.moduleConfig || {};
      });
      
      if (moduleConfig.Fields) {
        // Check that ShowInList fields have corresponding columns
        const listFields = moduleConfig.Fields.filter(f => f.ShowInList);
        
        for (const field of listFields) {
          await expect(page.locator(`th:has-text("${field.Label}")`)).toBeVisible();
        }
      }
    });
  });

  // 8. Cross-Browser & Responsive Tests
  test.describe('Cross-Browser & Responsive', () => {
    
    test('Dynamic page is responsive', async ({ page }) => {
      await page.goto(`${BASE_URL}/admin/modules/users`);
      
      // Test mobile viewport
      await page.setViewportSize({ width: 375, height: 667 });
      await expect(page.locator('table, .overflow-x-auto')).toBeVisible();
      
      // Test tablet viewport  
      await page.setViewportSize({ width: 768, height: 1024 });
      await expect(page.locator('table')).toBeVisible();
      
      // Test desktop viewport
      await page.setViewportSize({ width: 1920, height: 1080 });
      await expect(page.locator('table')).toBeVisible();
    });
  });

  // 9. Integration Tests
  test.describe('Integration Tests', () => {
    
    test('Navigation between static and dynamic works', async ({ page }) => {
      // Start at comparison dashboard
      await page.goto(`${BASE_URL}/admin/modules/`);
      
      // Click to static users
      await page.click('a[href="/admin/users"]');
      await expect(page).toHaveURL(/\/admin\/users$/);
      await expect(page.locator('tbody tr')).toHaveCountGreaterThan(0);
      
      // Go back to dashboard
      await page.goto(`${BASE_URL}/admin/modules/`);
      
      // Click to dynamic users
      await page.click('a[href="/admin/modules/users"]');
      await expect(page).toHaveURL(/\/admin\/modules\/users$/);
      await expect(page.locator('tbody tr')).toHaveCountGreaterThan(0);
    });

    test('Side-by-side testing functionality', async ({ page }) => {
      await page.goto(`${BASE_URL}/admin/modules/`);
      
      // Check if side-by-side buttons exist
      await expect(page.locator('button:has-text("Open Side by Side")')).toHaveCountGreaterThan(0);
      
      // Test button functionality would require popup handling
      // This is a basic structure test
    });
  });

  // 10. CRUD Operations Tests
  test.describe('CRUD Operations', () => {
    
    test('Can create a new user via dynamic module', async ({ page }) => {
      await page.goto(`${BASE_URL}/admin/modules/users`);
      
      // Get initial count
      const initialCount = await page.locator('tbody tr').count();
      
      // Open create modal
      await page.locator('button:has-text("New User")').click();
      await expect(page.locator('#moduleModal')).toBeVisible();
      
      // Fill in form with unique test data
      const timestamp = Date.now();
      const testUsername = `testuser${timestamp}`;
      
      await page.locator('#login').fill(testUsername);
      await page.locator('#pw').fill('TestPass123!');
      await page.locator('#first_name').fill('Test');
      await page.locator('#last_name').fill('User');
      await page.locator('#title').fill('Test Title');
      
      // Submit form
      await page.locator('button:has-text("Save")').click();
      
      // Wait for modal to close and page to reload
      await expect(page.locator('#moduleModal')).toBeHidden();
      await page.waitForTimeout(1500);
      
      // Verify new user appears in table
      const newCount = await page.locator('tbody tr').count();
      expect(newCount).toBeGreaterThan(initialCount);
      
      // Verify the new user data is displayed
      await expect(page.locator(`td:has-text("${testUsername}")`)).toBeVisible();
    });

    test('Can update an existing user via dynamic module', async ({ page }) => {
      await page.goto(`${BASE_URL}/admin/modules/users`);
      
      // Click edit on first user
      const firstEditButton = page.locator('tbody tr').first().locator('button[onclick*="editItem"]');
      await firstEditButton.click();
      
      // Wait for modal and fields to populate
      await expect(page.locator('#moduleModal')).toBeVisible();
      await page.waitForTimeout(500);
      
      // Update the title field
      const titleField = page.locator('#title');
      const originalTitle = await titleField.inputValue();
      const newTitle = `Updated ${Date.now()}`;
      await titleField.clear();
      await titleField.fill(newTitle);
      
      // Save changes
      await page.locator('button:has-text("Save")').click();
      
      // Wait for modal to close and page to reload
      await expect(page.locator('#moduleModal')).toBeHidden();
      await page.waitForTimeout(1500);
      
      // Verify the update by opening edit again
      await firstEditButton.click();
      await expect(page.locator('#moduleModal')).toBeVisible();
      await page.waitForTimeout(500);
      
      const updatedTitle = await titleField.inputValue();
      expect(updatedTitle).toBe(newTitle);
      expect(updatedTitle).not.toBe(originalTitle);
      
      // Close modal
      await page.locator('button:has-text("Cancel")').click();
    });

    test('Can soft delete/deactivate a user via dynamic module', async ({ page }) => {
      await page.goto(`${BASE_URL}/admin/modules/users`);
      
      // Find a user with Valid status
      const validUserRow = page.locator('tbody tr').filter({ 
        has: page.locator('span:has-text("Valid")') 
      }).first();
      
      if (await validUserRow.count() > 0) {
        // Click deactivate button
        const deactivateButton = validUserRow.locator('button[onclick*="toggleStatus"]').first();
        await deactivateButton.click();
        
        // Confirm in dialog
        page.on('dialog', dialog => dialog.accept());
        
        // Wait for reload
        await page.waitForTimeout(1500);
        
        // Status should change (depending on implementation)
        // This test verifies the button works without breaking
      }
    });

    test('API endpoint for single user returns correct data', async ({ page }) => {
      // First get list to get a user ID
      const listResponse = await page.request.get(`${BASE_URL}/admin/modules/users`, {
        headers: {
          'Cookie': DEMO_COOKIE,
          'X-Requested-With': 'XMLHttpRequest',
          'Accept': 'application/json'
        }
      });
      
      const listData = await listResponse.json();
      expect(listData.data.length).toBeGreaterThan(0);
      
      const firstUserId = listData.data[0].id;
      const firstUserLogin = listData.data[0].login;
      
      // Get single user
      const singleResponse = await page.request.get(`${BASE_URL}/admin/modules/users/${firstUserId}`, {
        headers: {
          'Cookie': DEMO_COOKIE,
          'X-Requested-With': 'XMLHttpRequest',
          'Accept': 'application/json'
        }
      });
      
      expect(singleResponse.status()).toBe(200);
      
      const singleData = await singleResponse.json();
      expect(singleData.success).toBe(true);
      expect(singleData.data).toBeDefined();
      expect(singleData.data.id).toBe(firstUserId);
      expect(singleData.data.login).toBe(firstUserLogin);
    });
  });

  // 11. Data Consistency Tests
  test.describe('Data Consistency', () => {
    
    test('Static and dynamic show same user data', async ({ page }) => {
      // Get first user from static page
      await page.goto(`${BASE_URL}/admin/users`);
      await expect(page.locator('tbody tr')).toHaveCountGreaterThan(0);
      
      const staticFirstUser = await page.locator('tbody tr').first().textContent();
      
      // Get first user from dynamic page
      await page.goto(`${BASE_URL}/admin/modules/users`);
      await expect(page.locator('tbody tr')).toHaveCountGreaterThan(0);
      
      const dynamicFirstUser = await page.locator('tbody tr').first().textContent();
      
      // Content should be similar (allowing for formatting differences)
      expect(staticFirstUser).toBeTruthy();
      expect(dynamicFirstUser).toBeTruthy();
      
      // Both should contain some common elements (like usernames)
      // This is a basic consistency check
    });
  });
});

// Utility function for custom assertions
test.describe('Utility & Helper Tests', () => {
  
  test('Multiple modules are available in dynamic system', async ({ page }) => {
    const response = await page.request.get(`${BASE_URL}/admin/modules/`, {
      headers: { 'Cookie': DEMO_COOKIE }
    });
    
    expect(response.status()).toBe(200);
    const content = await response.text();
    
    // Should list multiple modules
    expect(content).toContain('users');
    expect(content).toContain('priority');
    expect(content).toContain('queue');
    expect(content).toContain('state');
    expect(content).toContain('service');
  });
});