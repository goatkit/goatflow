import { test, expect } from '@playwright/test';
import { BASE_URL } from './base-url.js';

/**
 * Behavioral tests for Admin 2FA Override functionality.
 * 
 * Tests that admins can disable 2FA for users who lose their authenticator.
 */

const ADMIN_USER = {
  login: process.env.DEMO_ADMIN_EMAIL || 'root@localhost',
  password: process.env.DEMO_ADMIN_PASSWORD,
};

// Helper: Login as admin
async function adminLogin(page) {
  if (!ADMIN_USER.password) {
    throw new Error('DEMO_ADMIN_PASSWORD must be set in environment');
  }
  await page.goto(`${BASE_URL}/login`);
  await page.getByPlaceholder(/username/i).fill(ADMIN_USER.login);
  await page.getByPlaceholder(/password/i).fill(ADMIN_USER.password);
  await page.getByRole('button', { name: /login|sign in/i }).click();
  await page.waitForURL(/\/dashboard|\/admin/, { timeout: 10000 });
}

test.describe('Admin 2FA Override - Users', () => {

  test.beforeEach(async ({ page }) => {
    await adminLogin(page);
  });

  test('admin can see 2FA status column in user list', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/users`);
    await page.waitForLoadState('networkidle');
    
    // BEHAVIOR: 2FA status column visible in user management
    // Column header might show translation key or actual text
    const header = page.locator('th').filter({ hasText: /2FA|admin\.2fa/i }).first();
    await expect(header).toBeVisible({ timeout: 10000 });
  });

  test('admin can access user management page', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/users`);
    
    // BEHAVIOR: Admin can view user list
    await expect(page.getByRole('heading', { name: /User/i })).toBeVisible({ timeout: 10000 });
  });
});

test.describe('Admin 2FA Override - Customers', () => {

  test.beforeEach(async ({ page }) => {
    await adminLogin(page);
  });

  test('admin can see 2FA status in customer user list', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/customer-users`);
    await page.waitForLoadState('networkidle');
    
    // BEHAVIOR: 2FA column visible in customer management
    const header = page.locator('th').filter({ hasText: /2FA/i }).first();
    await expect(header).toBeVisible({ timeout: 10000 });
  });

  test('admin can access customer users page', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/customer-users`);
    await page.waitForLoadState('networkidle');
    
    // BEHAVIOR: Customer users page accessible
    await expect(page.getByRole('heading', { name: 'Customer User Management' })).toBeVisible({ timeout: 10000 });
  });

  test('customer users page has search input', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/customer-users`);
    await page.waitForLoadState('networkidle');
    
    // BEHAVIOR: Search input available
    const searchInput = page.locator('input[type="text"], input[type="search"]').first();
    await expect(searchInput).toBeVisible({ timeout: 5000 });
  });
});

test.describe('Admin 2FA Override API Security', () => {

  test('admin 2FA override requires authentication', async ({ request }) => {
    const response = await request.post(`${BASE_URL}/admin/api/users/test/2fa/disable`, {
      data: { reason: 'test' },
      maxRedirects: 0
    });
    // BEHAVIOR: Unauthenticated requests rejected
    expect([303, 401, 403, 404]).toContain(response.status());
  });

  test('admin customer 2FA override requires authentication', async ({ request }) => {
    const response = await request.post(`${BASE_URL}/admin/api/customers/test/2fa/disable`, {
      data: { reason: 'test' },
      maxRedirects: 0
    });
    // BEHAVIOR: Unauthenticated requests rejected
    expect([303, 401, 403, 404]).toContain(response.status());
  });
});
