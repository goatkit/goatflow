import { test, expect } from '@playwright/test';
import { BASE_URL } from './base-url.js';

/**
 * Behavioral tests for Agent 2FA functionality.
 * 
 * Test user: e2e.2fa.agent / AgentTest123! (defined in docker/mariadb/testdb/70-test-fixtures.sql)
 */

const TEST_AGENT = {
  login: process.env.E2E_AGENT_LOGIN || 'e2e.2fa.agent',
  password: process.env.E2E_AGENT_PASSWORD || 'AgentTest123!',
};

// Helper: Login as agent
async function agentLogin(page) {
  await page.goto(`${BASE_URL}/login`);
  await page.getByPlaceholder(/username/i).fill(TEST_AGENT.login);
  await page.getByPlaceholder(/password/i).fill(TEST_AGENT.password);
  await page.getByRole('button', { name: /login|sign in/i }).click();
  await page.waitForURL(/\/dashboard|\/agent/, { timeout: 10000 });
}

// Helper: Navigate to profile settings
async function goToSettings(page) {
  await page.waitForLoadState('networkidle');
  // Click user menu (button with initials "A2")
  const userMenu = page.getByRole('button', { name: 'A2' });
  await userMenu.click();
  await page.waitForTimeout(300);
  await page.getByRole('link', { name: /Settings|Profile/i }).click();
  await page.waitForLoadState('networkidle');
}

test.describe('Agent 2FA Behaviors', () => {

  test.beforeEach(async ({ page }) => {
    await agentLogin(page);
  });

  test('agent can see 2FA section in settings', async ({ page }) => {
    await goToSettings(page);
    
    // BEHAVIOR: 2FA section visible in agent settings
    await expect(page.getByText(/Two-Factor Authentication/i).first()).toBeVisible({ timeout: 10000 });
  });

  test('agent sees setup option when 2FA not configured', async ({ page }) => {
    await goToSettings(page);
    
    // BEHAVIOR: When 2FA disabled, show setup/enable option
    const setupOption = page.getByRole('button', { name: /Enable|Set up|Configure/i }).first();
    await expect(setupOption).toBeVisible({ timeout: 10000 });
  });

  test('agent profile page has settings link', async ({ page }) => {
    // Click user menu
    await page.getByRole('button', { name: 'A2' }).click();
    await page.waitForTimeout(300);
    
    // BEHAVIOR: User menu should have Settings/Profile link
    const settingsLink = page.getByRole('link', { name: /Settings|Profile/i });
    await expect(settingsLink).toBeVisible({ timeout: 3000 });
  });
});

test.describe('Agent 2FA API Security', () => {

  test('agent 2FA status requires authentication', async ({ request }) => {
    const response = await request.get(`${BASE_URL}/api/preferences/2fa/status`, {
      maxRedirects: 0
    });
    // BEHAVIOR: Unauthenticated = redirect to login
    expect([303, 401, 403]).toContain(response.status());
  });

  test('agent 2FA setup requires authentication', async ({ request }) => {
    const response = await request.post(`${BASE_URL}/api/preferences/2fa/setup`, {
      data: { password: 'test' },
      maxRedirects: 0
    });
    expect([303, 401, 403]).toContain(response.status());
  });
});
