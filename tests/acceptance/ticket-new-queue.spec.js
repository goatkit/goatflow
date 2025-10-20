import { test, expect } from '@playwright/test';
import { BASE_URL, BASE_HOST } from './base-url.js';

async function login(page) {
  const username = process.env.DEMO_ADMIN_EMAIL || 'root@localhost';
  const password = process.env.DEMO_ADMIN_PASSWORD || 'admin123';
  const response = await page.request.post(`${BASE_URL}/api/auth/login`, {
    data: {
      username,
      password,
    },
    headers: {
      'Content-Type': 'application/json',
    },
  });

  if (!response.ok()) {
    throw new Error(`login failed for ${username}: ${response.status()} ${response.statusText()}`);
  }

  const payload = await response.json();
  const token = payload && payload.access_token;

  if (!token) {
    throw new Error('login response missing access_token');
  }

  await page.context().addCookies([
    {
      name: 'access_token',
      value: token,
      url: `${BASE_URL}/`,
      httpOnly: false,
      secure: false,
      sameSite: 'Lax',
    },
  ]);
}

test.describe('Agent ticket form queue preference', () => {
  test.beforeEach(async ({ page }) => {
    page.on('console', (msg) => {
      // eslint-disable-next-line no-console
      console.log('browser console', msg.type(), msg.text());
    });
    await login(page);
  });

  test('selecting customer user applies preferred queue', async ({ page }) => {
    await page.goto(`${BASE_URL}/tickets/new`);

    await page.waitForFunction(() => {
      const seed = window.GoatKitSeeds && window.GoatKitSeeds['customer-user'];
      return Array.isArray(seed) && seed.length > 0;
    });

    const target = await page.evaluate(() => {
      const entries = (window.GoatKitSeeds && window.GoatKitSeeds['customer-user']) || [];
      return entries.find((item) => item && item.preferredQueueId);
    });

    expect(target, 'expected at least one customer user with a preferred queue in seed data').toBeTruthy();

    const queueSelect = page.locator('#queue_id');
    const expectedQueueValue = String(target.preferredQueueId);
    await expect(queueSelect.locator(`option[value="${expectedQueueValue}"]`)).toHaveCount(1);

    const autocompleteInput = page.locator('#customer_user_input');
    await autocompleteInput.click();
    await autocompleteInput.fill(target.login.slice(0, 3));

    const option = page.locator(`[role="option"][data-value="${target.login}"]`);
    await option.waitFor({ state: 'visible' });
    await option.click();

    await expect(queueSelect).toHaveValue(expectedQueueValue);

    const infoPanel = page.locator('#customer-info-panel');
    await expect(infoPanel).toBeVisible();
    if (target.preferredQueueName) {
      await expect(infoPanel).toContainText(target.preferredQueueName);
    }
  });
});
