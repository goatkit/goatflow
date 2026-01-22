import { test, expect } from '@playwright/test';
import { BASE_URL, BASE_HOST } from './base-url.js';

/**
 * E2E test for customer profile initials display.
 *
 * Tests that the customer initials in the header user menu show TWO letters
 * (e.g., "ET" for E2E TestCustomer), not just one letter (e.g., "E").
 *
 * Test user: e2e.test.customer (E2E TestCustomer) - expected initials: "ET"
 * This user is created in docker/mariadb/testdb/70-test-fixtures.sql
 */

const TEST_CUSTOMER = {
  login: 'e2e.test.customer',
  password: 'TestPass123!',
  firstName: 'E2E',
  lastName: 'TestCustomer',
  expectedInitials: 'ET',
};

test.describe('Customer Profile Initials', () => {

  test('header shows two-letter initials (ET) not single letter (E)', async ({ page }) => {
    // Step 1: Go to customer login page
    await page.goto(`${BASE_URL}/customer/login`);

    // Step 2: Log in as E2E TestCustomer
    const loginInput = page.locator('input[name="login"], input[name="username"], #login, #username');
    const passwordInput = page.locator('input[name="password"], #password');
    const submitButton = page.locator('button[type="submit"], input[type="submit"]');

    await loginInput.first().fill(TEST_CUSTOMER.login);
    await passwordInput.first().fill(TEST_CUSTOMER.password);
    await submitButton.first().click();

    // Step 3: Wait for redirect to customer dashboard
    await page.waitForURL(/\/customer/, { timeout: 10000 });

    // Step 4: Find the initials in the header user menu
    // The initials are in a span inside a button with rounded-full class (avatar circle)
    const initialsElement = page.locator('button.rounded-full span, .rounded-full span').first();

    // Wait for the element to be visible
    await expect(initialsElement).toBeVisible({ timeout: 5000 });

    // Step 5: Get the initials text
    const initialsText = await initialsElement.textContent();
    const trimmedInitials = initialsText?.trim() || '';

    console.log(`Found initials: "${trimmedInitials}"`);

    // Step 6: Assert that initials are exactly "ET" (two letters)
    expect(trimmedInitials).toBe(TEST_CUSTOMER.expectedInitials);

    // Additional assertion: initials should be exactly 2 characters
    expect(trimmedInitials.length).toBe(2);

    // Verify it's not just the first letter
    expect(trimmedInitials).not.toBe('E');
  });

  test('profile page shows two-letter initials in avatar', async ({ page }) => {
    // Step 1: Go to customer login page
    await page.goto(`${BASE_URL}/customer/login`);

    // Step 2: Log in as E2E TestCustomer
    const loginInput = page.locator('input[name="login"], input[name="username"], #login, #username');
    const passwordInput = page.locator('input[name="password"], #password');
    const submitButton = page.locator('button[type="submit"], input[type="submit"]');

    await loginInput.first().fill(TEST_CUSTOMER.login);
    await passwordInput.first().fill(TEST_CUSTOMER.password);
    await submitButton.first().click();

    // Step 3: Wait for redirect to customer dashboard
    await page.waitForURL(/\/customer/, { timeout: 10000 });

    // Step 4: Click the user menu button to open dropdown
    const userMenuButton = page.locator('button.rounded-full').first();
    await expect(userMenuButton).toBeVisible({ timeout: 5000 });
    await userMenuButton.click();

    // Step 5: Click on Profile link in dropdown
    const profileLink = page.locator('a:has-text("Profile"), a[href*="profile"]').first();
    await profileLink.click();

    // Step 6: Wait for profile page to load
    await page.waitForURL(/\/customer\/profile/, { timeout: 10000 });

    // Step 7: Find the large initials avatar on the profile page
    // Profile page has a larger avatar with the initials in a rounded-full container
    const allSpans = page.locator('.rounded-full span');
    let initialsText = '';

    for (let i = 0; i < await allSpans.count(); i++) {
      const text = (await allSpans.nth(i).textContent())?.trim() || '';
      if (text.length === 2 && /^[A-Z]{2}$/.test(text)) {
        initialsText = text;
        break;
      }
    }

    console.log(`Found profile initials: "${initialsText}"`);

    // Assert initials are "ET"
    expect(initialsText).toBe(TEST_CUSTOMER.expectedInitials);
  });
});
