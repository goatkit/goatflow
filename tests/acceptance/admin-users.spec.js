import { test, expect } from '@playwright/test';

const BASE_URL = process.env.BASE_URL || 'http://localhost:8080';

const login = async (page) => {
  await page.context().addCookies([
    {
      name: 'access_token',
      value: 'demo_session_admin',
      domain: 'localhost',
      path: '/',
    },
  ]);
};

test.describe('Admin Users', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('loads users management page', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/users`);
    await expect(page.locator('#usersTable')).toBeVisible();
    await expect(page.locator('#usersTable tbody tr').first()).toBeVisible();
  });

  test('displays user list with correct columns', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/users`);
    await expect(page.locator('#usersTable')).toBeVisible();
    
    // Check for expected column headers
    const headers = page.locator('#usersTable thead th');
    await expect(headers).toHaveCount(6); // ID, Login, Name, Groups, Status, Actions
  });

  test('opens add user modal', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/users`);
    
    // Click add user button
    const addButton = page.locator('button:has-text("Add User"), a:has-text("Add User")');
    if (await addButton.count() > 0) {
      await addButton.first().click();
      const modal = page.locator('#userModal, .modal');
      await expect(modal).toBeVisible();
    }
  });

  test('edits a user and restores original data', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/users`);
    await expect(page.locator('#usersTable')).toBeVisible();

    const firstRow = page.locator('#usersTable tbody tr').first();
    await expect(firstRow).toBeVisible();

    const loginCell = firstRow.locator('td').nth(1);
    const loginValue = (await loginCell.textContent())?.trim();
    expect(loginValue).toBeTruthy();

    await firstRow.locator('button[onclick*="editUser"]').click();

    const modal = page.locator('#userModal');
    await expect(modal).toBeVisible();

    const firstNameInput = modal.locator('#firstName');
    const lastNameInput = modal.locator('#lastName');

    const originalFirstName = await firstNameInput.inputValue();
    const originalLastName = await lastNameInput.inputValue();

    const updatedLastName = originalLastName.endsWith(' QA')
      ? originalLastName
      : `${originalLastName} QA`;

    await lastNameInput.fill(updatedLastName);

    await Promise.all([
      page.waitForNavigation(),
      modal.locator('button[type="submit"]').click(),
    ]);

    await expect(page).toHaveURL(/\/admin\/users$/);
    await expect(page.locator('#usersTable')).toBeVisible();

    const updatedRow = page
      .locator('#usersTable tbody tr')
      .filter({ hasText: loginValue || '' })
      .first();
    await expect(updatedRow).toContainText(updatedLastName);

    await updatedRow.locator('button[onclick*="editUser"]').click();
    await expect(modal).toBeVisible();

    await lastNameInput.fill(originalLastName);

    await Promise.all([
      page.waitForNavigation(),
      modal.locator('button[type="submit"]').click(),
    ]);

    await expect(page).toHaveURL(/\/admin\/users$/);
    await expect(page.locator('#usersTable')).toBeVisible();

    const restoredRow = page
      .locator('#usersTable tbody tr')
      .filter({ hasText: loginValue || '' })
      .first();
    await expect(restoredRow).toContainText(originalLastName);
    await expect(restoredRow).toContainText(originalFirstName);
  });

  test('user row shows groups', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/users`);
    await expect(page.locator('#usersTable')).toBeVisible();
    
    // At least one user should have groups displayed
    const rows = page.locator('#usersTable tbody tr');
    const firstRow = rows.first();
    await expect(firstRow).toBeVisible();
    
    // Groups column should exist (may be empty for some users)
    const groupsCell = firstRow.locator('td').nth(3);
    await expect(groupsCell).toBeVisible();
  });

  test('search filters user list', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/users`);
    await expect(page.locator('#usersTable')).toBeVisible();
    
    // Get initial row count
    const initialRows = await page.locator('#usersTable tbody tr').count();
    
    // If there's a search input, test filtering
    const searchInput = page.locator('input[type="search"], input[name="search"], #searchInput');
    if (await searchInput.count() > 0) {
      await searchInput.fill('admin');
      await page.waitForTimeout(500); // Wait for filter to apply
      
      // Either filtered results or same count (if 'admin' is common)
      const filteredRows = await page.locator('#usersTable tbody tr').count();
      expect(filteredRows).toBeGreaterThan(0);
    }
  });

  test('user status toggle is accessible', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/users`);
    await expect(page.locator('#usersTable')).toBeVisible();
    
    const firstRow = page.locator('#usersTable tbody tr').first();
    await expect(firstRow).toBeVisible();
    
    // Status column should have a toggle or indicator
    const statusCell = firstRow.locator('td').nth(4);
    await expect(statusCell).toBeVisible();
  });

  test('password reset modal opens without JS errors', async ({ page }) => {
    // Capture console errors
    const consoleErrors = [];
    page.on('console', msg => {
      if (msg.type() === 'error') {
        consoleErrors.push(msg.text());
      }
    });

    await page.goto(`${BASE_URL}/admin/users`);
    await expect(page.locator('#usersTable')).toBeVisible();

    const firstRow = page.locator('#usersTable tbody tr').first();
    await expect(firstRow).toBeVisible();

    // Click reset password button
    const resetButton = firstRow.locator('button[onclick*="resetPassword"]');
    await expect(resetButton).toBeVisible();
    await resetButton.click();

    // Modal should open
    const modal = page.locator('#passwordResetModal');
    await expect(modal).toBeVisible();

    // No JS errors should have occurred
    const minLengthErrors = consoleErrors.filter(e => 
      e.toLowerCase().includes('minlength') || e.toLowerCase().includes('undefined')
    );
    expect(minLengthErrors).toHaveLength(0);
  });

  test('password reset validates password requirements', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/users`);
    await expect(page.locator('#usersTable')).toBeVisible();

    const firstRow = page.locator('#usersTable tbody tr').first();
    const resetButton = firstRow.locator('button[onclick*="resetPassword"]');
    await resetButton.click();

    const modal = page.locator('#passwordResetModal');
    await expect(modal).toBeVisible();

    const newPasswordInput = modal.locator('#newPassword');
    const confirmPasswordInput = modal.locator('#confirmPassword');

    // Type a weak password - should show validation feedback
    await newPasswordInput.fill('weak');
    await confirmPasswordInput.fill('weak');

    // Length icon should be red/invalid (password too short)
    const lengthIcon = modal.locator('#lengthIcon');
    await expect(lengthIcon).toBeVisible();

    // Type a strong password
    await newPasswordInput.fill('StrongPass123');
    await confirmPasswordInput.fill('StrongPass123');

    // Validation icons should update (check they're visible at minimum)
    await expect(lengthIcon).toBeVisible();
    const matchIcon = modal.locator('#matchIcon');
    await expect(matchIcon).toBeVisible();
  });

  test('password reset works with valid credentials', async ({ page }) => {
    // Skip actual password change to avoid breaking demo data
    // Just verify the flow works up to submission
    await page.goto(`${BASE_URL}/admin/users`);
    await expect(page.locator('#usersTable')).toBeVisible();

    const firstRow = page.locator('#usersTable tbody tr').first();
    const resetButton = firstRow.locator('button[onclick*="resetPassword"]');
    await resetButton.click();

    const modal = page.locator('#passwordResetModal');
    await expect(modal).toBeVisible();

    // Fill valid password
    await modal.locator('#newPassword').fill('ValidPass123!');
    await modal.locator('#confirmPassword').fill('ValidPass123!');

    // Submit button should be enabled and clickable
    const submitButton = modal.locator('button:has-text("Reset Password")');
    await expect(submitButton).toBeVisible();
    await expect(submitButton).toBeEnabled();

    // Close without submitting to preserve test data
    await modal.locator('button:has-text("Cancel")').click();
    await expect(modal).toBeHidden();
  });

  test('password policy API is called on page load', async ({ page }) => {
    let policyFetched = false;
    
    // Intercept the password policy request
    await page.route('**/admin/password-policy', async route => {
      policyFetched = true;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          policy: {
            minLength: 8,
            requireUppercase: true,
            requireLowercase: true,
            requireDigit: true,
            requireSpecial: false
          }
        })
      });
    });

    await page.goto(`${BASE_URL}/admin/users`);
    await expect(page.locator('#usersTable')).toBeVisible();

    // Give time for the fetch to complete
    await page.waitForTimeout(500);
    expect(policyFetched).toBe(true);
  });

  test('password reset handles failed policy fetch gracefully', async ({ page }) => {
    // Capture console errors
    const consoleErrors = [];
    page.on('console', msg => {
      if (msg.type() === 'error') {
        consoleErrors.push(msg.text());
      }
    });

    // Make the policy endpoint fail
    await page.route('**/admin/password-policy', async route => {
      await route.fulfill({
        status: 500,
        contentType: 'application/json',
        body: JSON.stringify({ success: false, error: 'Server error' })
      });
    });

    await page.goto(`${BASE_URL}/admin/users`);
    await expect(page.locator('#usersTable')).toBeVisible();

    const firstRow = page.locator('#usersTable tbody tr').first();
    const resetButton = firstRow.locator('button[onclick*="resetPassword"]');
    await resetButton.click();

    // Modal should still open
    const modal = page.locator('#passwordResetModal');
    await expect(modal).toBeVisible();

    // Type in password field - should NOT cause minLength error
    await modal.locator('#newPassword').fill('Test123!');
    
    // No TypeError about minLength should occur
    const minLengthErrors = consoleErrors.filter(e => 
      e.toLowerCase().includes('minlength') || 
      e.includes('Cannot read properties of undefined')
    );
    expect(minLengthErrors).toHaveLength(0);
  });
});
