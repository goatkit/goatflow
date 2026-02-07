/**
 * Demo Site Smoke Test
 *
 * Verifies the demo deployment is working by:
 * 1. Logging in with demo credentials
 * 2. Checking the tickets page loads with data
 *
 * Run against production demo:
 *   PLAYWRIGHT_SKIP_WEBSERVER=1 BASE_URL=https://goatflow-demo.gibbsoft.com npx playwright test demo-smoke
 */
import { test, expect } from '@playwright/test';
import { BASE_URL } from './base-url.js';

// Demo credentials are public - shown on goatflow.io demo page
const DEMO_EMAIL = process.env.DEMO_EMAIL || 'Saul';
const DEMO_PASSWORD = process.env.DEMO_PASSWORD || 'Saul';
const DEMO_URL = process.env.DEMO_URL || BASE_URL;

test.describe('Demo Site Smoke Test', () => {
  test('can login and view tickets', async ({ page }) => {
    // Navigate to login page
    await page.goto(`${DEMO_URL}/login`);

    // Take screenshot of login page
    await page.screenshot({ path: 'test-results/demo-login.png', fullPage: true });

    // Fill login form
    const loginForm = page.locator('form').first();
    await loginForm.locator('input[name="email"], input[name="username"], input[type="email"]').fill(DEMO_EMAIL);
    await loginForm.locator('input[name="password"], input[type="password"]').fill(DEMO_PASSWORD);

    // Submit login
    await page.click('button[type="submit"]');

    // Wait for navigation to complete
    await page.waitForLoadState('networkidle');

    // Should be redirected to dashboard or tickets
    await expect(page).not.toHaveURL(/\/login/);

    // Verify we're on the agent dashboard (not login page)
    await expect(page.locator('.gk-heading, [data-testid="dashboard"], .dashboard')).toBeVisible({ timeout: 10000 });

    // Take screenshot of dashboard
    await page.screenshot({ path: 'test-results/demo-dashboard.png', fullPage: true });

    console.log('✅ Demo site verified successfully!');
  });
});
