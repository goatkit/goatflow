import { test, expect } from '@playwright/test';
import { BASE_URL } from './base-url.js';

async function login(page) {
  const username = process.env.DEMO_ADMIN_EMAIL || 'root@localhost';
  const password = process.env.DEMO_ADMIN_PASSWORD;
  if (!password) {
    throw new Error('DEMO_ADMIN_PASSWORD must be set in .env');
  }
  const response = await page.request.post(`${BASE_URL}/api/auth/login`, {
    data: { username, password },
    headers: { 'Content-Type': 'application/json' },
  });

  if (!response.ok()) {
    throw new Error(`login failed: ${response.status()}`);
  }

  const payload = await response.json();
  const token = payload?.access_token;
  if (!token) throw new Error('login response missing access_token');

  await page.context().addCookies([{
    name: 'access_token',
    value: token,
    url: `${BASE_URL}/`,
    httpOnly: false,
    secure: false,
    sameSite: 'Lax',
  }]);
}

test.describe('Dynamic Fields JS Validation', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('required text field shows error when empty', async ({ page }) => {
    // Go to ticket create page (has dynamic fields)
    await page.goto(`${BASE_URL}/tickets/new`);
    
    // Look for a required dynamic field (marked with *)
    const requiredField = page.locator('input[name^="DynamicField_"][required]').first();
    
    // Skip test if no required dynamic fields configured
    if (await requiredField.count() === 0) {
      test.skip();
      return;
    }

    // Clear and blur to trigger validation
    await requiredField.fill('');
    await requiredField.blur();

    // Check for error message
    const errorMsg = page.locator('p.text-red-600, p.text-red-400').filter({ hasText: 'required' });
    await expect(errorMsg).toBeVisible({ timeout: 2000 });
  });

  test('required dropdown shows error when not selected', async ({ page }) => {
    await page.goto(`${BASE_URL}/tickets/new`);
    
    const requiredSelect = page.locator('select[name^="DynamicField_"][required]').first();
    
    if (await requiredSelect.count() === 0) {
      test.skip();
      return;
    }

    // Select empty option and blur
    await requiredSelect.selectOption('');
    await requiredSelect.blur();

    const errorMsg = page.locator('p.text-red-600, p.text-red-400').filter({ hasText: 'required' });
    await expect(errorMsg).toBeVisible({ timeout: 2000 });
  });

  test('valid input clears error message', async ({ page }) => {
    await page.goto(`${BASE_URL}/tickets/new`);
    
    const requiredField = page.locator('input[name^="DynamicField_"][required]').first();
    
    if (await requiredField.count() === 0) {
      test.skip();
      return;
    }

    // Trigger error first
    await requiredField.fill('');
    await requiredField.blur();

    // Now fill with valid value
    await requiredField.fill('test value');
    await requiredField.blur();

    // Error should be hidden
    const fieldName = await requiredField.getAttribute('name');
    const errorMsg = page.locator(`p[x-show="errors['${fieldName}']"]`);
    await expect(errorMsg).toBeHidden({ timeout: 2000 });
  });

  test('maxlength attribute is applied to text fields', async ({ page }) => {
    await page.goto(`${BASE_URL}/tickets/new`);
    
    const textFieldWithMax = page.locator('input[name^="DynamicField_"][maxlength]').first();
    
    if (await textFieldWithMax.count() === 0) {
      test.skip();
      return;
    }

    const maxLength = await textFieldWithMax.getAttribute('maxlength');
    expect(parseInt(maxLength)).toBeGreaterThan(0);
  });

  test('date field with restriction has min/max attribute', async ({ page }) => {
    await page.goto(`${BASE_URL}/tickets/new`);
    
    // Check for date fields with min or max attributes
    const dateWithRestriction = page.locator('input[type="date"][name^="DynamicField_"][min], input[type="date"][name^="DynamicField_"][max]').first();
    
    if (await dateWithRestriction.count() === 0) {
      test.skip();
      return;
    }

    const minAttr = await dateWithRestriction.getAttribute('min');
    const maxAttr = await dateWithRestriction.getAttribute('max');
    
    // Should have at least one restriction
    expect(minAttr || maxAttr).toBeTruthy();
  });
});
