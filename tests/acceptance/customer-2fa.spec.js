import { test, expect } from '@playwright/test';
import { BASE_URL } from './base-url.js';

/**
 * Behavioral tests for Customer 2FA functionality.
 * 
 * Test user: e2e.2fa.setup / Test2FA! (defined in docker/mariadb/testdb/70-test-fixtures.sql)
 */

const TEST_CUSTOMER = {
  login: process.env.E2E_CUSTOMER_LOGIN || 'e2e.2fa.setup',
  password: process.env.E2E_CUSTOMER_PASSWORD || 'Test2FA!',
};

// Helper: Login as customer
async function customerLogin(page) {
  await page.goto(`${BASE_URL}/customer/login`);
  await page.getByPlaceholder(/username|email/i).fill(TEST_CUSTOMER.login);
  await page.getByPlaceholder(/password/i).fill(TEST_CUSTOMER.password);
  await page.getByRole('button', { name: /login|sign in/i }).click();
  await page.waitForURL(/\/customer/, { timeout: 10000 });
}

// Helper: Navigate to profile
async function goToProfile(page) {
  await page.waitForLoadState('networkidle');
  await page.getByRole('button', { name: '2S' }).click();
  await page.waitForTimeout(300);
  await page.getByRole('link', { name: 'Profile' }).click();
  await page.waitForURL(/\/customer\/profile/, { timeout: 10000 });
  await page.waitForLoadState('networkidle');
}

// Helper: Clear 2FA for test user (reset state)
async function clear2FAState(page) {
  // Make API call to check and potentially reset state
  await page.evaluate(async () => {
    // This runs in browser context - just ensure we're starting fresh
  });
}

test.describe('Customer 2FA Behaviors', () => {

  test.beforeEach(async ({ page }) => {
    await customerLogin(page);
  });

  test('customer can see 2FA section on profile page', async ({ page }) => {
    await goToProfile(page);
    
    // BEHAVIOR: 2FA section should be visible with description
    await expect(page.getByRole('heading', { name: /Two-Factor Authentication/i })).toBeVisible();
    await expect(page.getByText(/authenticator app/i).first()).toBeVisible();
  });

  test('customer sees Enable button when 2FA is not configured', async ({ page }) => {
    await goToProfile(page);
    
    // BEHAVIOR: When 2FA disabled, show Enable button
    const enableBtn = page.getByRole('button', { name: /Enable 2FA/i });
    await expect(enableBtn).toBeVisible({ timeout: 10000 });
  });

  test('clicking Enable 2FA prompts for password first', async ({ page }) => {
    await goToProfile(page);
    
    await page.getByRole('button', { name: /Enable 2FA/i }).click();
    
    // BEHAVIOR: Must verify identity with password before showing QR
    await expect(page.getByText(/Enter your password/i)).toBeVisible({ timeout: 5000 });
    await expect(page.getByRole('textbox', { name: /password/i })).toBeVisible();
    await expect(page.getByRole('button', { name: /Continue/i })).toBeVisible();
  });

  test('wrong password shows error, does not show QR code', async ({ page }) => {
    await goToProfile(page);
    
    await page.getByRole('button', { name: /Enable 2FA/i }).click();
    await page.getByRole('textbox', { name: /password/i }).fill('WrongPassword123');
    await page.getByRole('button', { name: /Continue/i }).click();
    
    // BEHAVIOR: Wrong password = error message, no QR code
    await expect(page.getByText(/incorrect|invalid|wrong/i)).toBeVisible({ timeout: 5000 });
    await expect(page.locator('img[alt*="QR"]')).not.toBeVisible();
  });

  test('correct password reveals QR code and recovery codes', async ({ page }) => {
    await goToProfile(page);
    
    await page.getByRole('button', { name: /Enable 2FA/i }).click();
    await page.getByRole('textbox', { name: /password/i }).fill(TEST_CUSTOMER.password);
    await page.getByRole('button', { name: /Continue/i }).click();
    
    // BEHAVIOR: Correct password shows QR code for authenticator app
    await expect(page.getByRole('img', { name: 'QR Code' })).toBeVisible({ timeout: 10000 });
    
    // BEHAVIOR: Shows manual entry code (base32 secret)
    await expect(page.getByText(/manually/i)).toBeVisible();
    
    // BEHAVIOR: Shows recovery codes warning
    await expect(page.getByText(/Save Recovery Codes/i)).toBeVisible();
  });

  test('recovery codes have copy button', async ({ page }) => {
    await goToProfile(page);
    
    await page.getByRole('button', { name: /Enable 2FA/i }).click();
    await page.getByRole('textbox', { name: /password/i }).fill(TEST_CUSTOMER.password);
    await page.getByRole('button', { name: /Continue/i }).click();
    
    // Wait for QR code step
    await expect(page.getByRole('img', { name: 'QR Code' })).toBeVisible({ timeout: 10000 });
    
    // BEHAVIOR: Copy button exists for recovery codes
    const copyBtn = page.getByRole('button', { name: /Copy/i });
    await expect(copyBtn).toBeVisible();
    await expect(copyBtn).toBeEnabled();
  });

  test('cancel button closes setup without enabling 2FA', async ({ page }) => {
    await goToProfile(page);
    
    await page.getByRole('button', { name: /Enable 2FA/i }).click();
    await expect(page.getByText(/Enter your password/i)).toBeVisible();
    
    // BEHAVIOR: Cancel returns to profile without changes
    await page.getByRole('button', { name: /Cancel/i }).first().click();
    
    // Modal should close, Enable 2FA should still be visible
    await expect(page.getByText(/Enter your password to set up/i)).not.toBeVisible({ timeout: 3000 });
    await expect(page.getByRole('button', { name: /Enable 2FA/i })).toBeVisible();
  });

  test('setup requires verification code to complete', async ({ page }) => {
    await goToProfile(page);
    
    await page.getByRole('button', { name: /Enable 2FA/i }).click();
    await page.getByRole('textbox', { name: /password/i }).fill(TEST_CUSTOMER.password);
    await page.getByRole('button', { name: /Continue/i }).click();
    
    // Wait for QR code step
    await expect(page.getByRole('img', { name: 'QR Code' })).toBeVisible({ timeout: 10000 });
    
    // BEHAVIOR: Must enter valid TOTP code to complete setup
    await expect(page.getByText(/6-digit code/i)).toBeVisible();
    await expect(page.getByRole('button', { name: /Verify & Enable/i })).toBeVisible();
  });
});

test.describe('Customer 2FA Security', () => {

  test('2FA status API requires authentication', async ({ request }) => {
    const response = await request.get(`${BASE_URL}/customer/api/preferences/2fa/status`, {
      maxRedirects: 0  // Don't follow redirects
    });
    // BEHAVIOR: Unauthenticated requests get redirected to login
    expect(response.status()).toBe(303);
  });

  test('2FA setup API requires authentication', async ({ request }) => {
    const response = await request.post(`${BASE_URL}/customer/api/preferences/2fa/setup`, {
      data: { password: 'test' },
      maxRedirects: 0
    });
    // BEHAVIOR: Unauthenticated requests get redirected to login
    expect(response.status()).toBe(303);
  });

  test('status response never exposes TOTP secret', async ({ page }) => {
    await customerLogin(page);
    
    const response = await page.evaluate(async () => {
      const res = await fetch('/customer/api/preferences/2fa/status', { credentials: 'include' });
      return await res.text();
    });
    
    // BEHAVIOR: Secret should never be in status response
    expect(response).not.toMatch(/secret/i);
    expect(response).not.toMatch(/[A-Z2-7]{16,}/); // Base32 pattern
  });
});
