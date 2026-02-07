import { test, expect, request as playwrightRequest } from '@playwright/test';
import { BASE_URL, BASE_HOST } from './base-url.js';

const adminCookie = {
  name: 'access_token',
  value: 'demo_session_admin',
  domain: BASE_HOST,
  path: '/',
};

const defaultPortal = {
  enabled: true,
  loginRequired: true,
  title: 'Customer Portal',
  footer: 'Powered by GoatFlow',
  landing: '/customer/tickets',
};

const setToggle = async (checkbox, target) => {
  const parentLabel = checkbox.locator('..');
  const current = await checkbox.isChecked();
  if (current !== target) {
    await parentLabel.click();
  }
  await expect(checkbox).toBeChecked({ checked: target });
};

test.describe('Admin Portal Settings', () => {
  test.beforeEach(async ({ page }) => {
    await page.context().addCookies([adminCookie]);
  });

  test('updates and restores global portal settings via UI', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/customer/portal/settings`);

    const enabled = page.locator('input[name="enabled"][type="checkbox"]');
    const loginRequired = page.locator('input[name="login_required"][type="checkbox"]');
    const title = page.locator('#title');
    const footer = page.locator('#footer_text');
    const landing = page.locator('#landing_page');
    const save = page.locator('button:has-text("Save settings")');

    const original = {
      enabled: await enabled.isChecked(),
      loginRequired: await loginRequired.isChecked(),
      title: (await title.inputValue()).trim(),
      footer: (await footer.inputValue()).trim(),
      landing: (await landing.inputValue()).trim(),
    };

    await setToggle(enabled, !original.enabled);
    await setToggle(loginRequired, !original.loginRequired);
    await title.fill(`Playwright Portal ${Date.now()}`);
    await footer.fill('Portal footer via Playwright');
    await landing.fill(`/customer/tickets/playwright-${Date.now()}`);

    await Promise.all([page.waitForLoadState('networkidle'), save.click()]);
    await page.reload();

    await expect(enabled).toBeChecked({ checked: !original.enabled });
    await expect(loginRequired).toBeChecked({ checked: !original.loginRequired });
    await expect(title).toHaveValue(/Playwright Portal/);
    await expect(footer).toHaveValue('Portal footer via Playwright');

    await setToggle(enabled, original.enabled);
    await setToggle(loginRequired, original.loginRequired);
    await title.fill(original.title);
    await footer.fill(original.footer);
    await landing.fill(original.landing);
    await Promise.all([page.waitForLoadState('networkidle'), save.click()]);
    await page.reload();

    await expect(title).toHaveValue(original.title);
    await expect(footer).toHaveValue(original.footer);
    await expect(landing).toHaveValue(original.landing);
    if (original.enabled) {
      await expect(enabled).toBeChecked();
    } else {
      await expect(enabled).not.toBeChecked();
    }
    if (original.loginRequired) {
      await expect(loginRequired).toBeChecked();
    } else {
      await expect(loginRequired).not.toBeChecked();
    }
  });

  test('updates and resets company portal overrides via API', async () => {
    const api = await playwrightRequest.newContext({
      baseURL: BASE_URL,
      extraHTTPHeaders: {
        Cookie: `access_token=${adminCookie.value}; Path=/; Domain=${adminCookie.domain}`,
      },
    });

    const customerID = `PLAYPORTAL${Date.now()}`;
    const companyForm = new URLSearchParams({
      customer_id: customerID,
      name: `Playwright ${customerID}`,
    });

    const createResp = await api.post(`/admin/customer/companies`, {
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      data: companyForm.toString(),
    });
    expect(createResp.ok()).toBeTruthy();

    const form = new URLSearchParams({
      enabled: '0',
      login_required: '0',
      title: `Company Portal ${customerID}`,
      footer_text: `Footer ${customerID}`,
      landing_page: `/customer/tickets/${customerID.toLowerCase()}`,
    });

    let resp = await api.post(`/admin/customer/companies/${customerID}/portal-settings`, {
      headers: { 'Content-Type': 'application/x-www-form-urlencoded', Accept: 'application/json' },
      data: form.toString(),
    });
    expect(resp.ok()).toBeTruthy();
    let body = await resp.json();
    if (typeof body.success !== 'undefined') {
      expect(body.success).toBe(true);
    }

    resp = await api.get(`/admin/customer/companies/${customerID}/portal-settings`);
    expect(resp.ok()).toBeTruthy();
    body = await resp.json();
    expect(body.settings.title).toContain(customerID);
    expect(body.settings.enabled).toBe(false);

    const resetForm = new URLSearchParams({
      enabled: defaultPortal.enabled ? '1' : '0',
      login_required: defaultPortal.loginRequired ? '1' : '0',
      title: defaultPortal.title,
      footer_text: defaultPortal.footer,
      landing_page: defaultPortal.landing,
    });

    resp = await api.post(`/admin/customer/companies/${customerID}/portal-settings`, {
      headers: { 'Content-Type': 'application/x-www-form-urlencoded', Accept: 'application/json' },
      data: resetForm.toString(),
    });
    expect(resp.ok()).toBeTruthy();

    resp = await api.get(`/admin/customer/companies/${customerID}/portal-settings`);
    expect(resp.ok()).toBeTruthy();
    body = await resp.json();
    expect(body.settings.title).toBe(defaultPortal.title);
    expect(body.settings.footerText || body.settings.footer_text || body.settings.footer).toContain('Powered by GoatFlow');
    expect(body.settings.enabled).toBe(defaultPortal.enabled);
    expect(body.settings.loginRequired).toBe(defaultPortal.loginRequired);
  });
});
