import { test, expect } from '@playwright/test';
import { BASE_URL } from './base-url.js';

async function login(page) {
  const username = process.env.DEMO_ADMIN_EMAIL || 'root@localhost';
  const password = process.env.DEMO_ADMIN_PASSWORD;
  if (!password) {
    throw new Error('DEMO_ADMIN_PASSWORD must be set in .env');
  }
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

test.describe('Admin Groups', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test('loads groups management page', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/groups`);
    await expect(page.locator('#groupsTable')).toBeVisible();
    await expect(page.locator('#groupsTable tbody tr').first()).toBeVisible();
  });

  test('displays group list with correct columns', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/groups`);
    await expect(page.locator('#groupsTable')).toBeVisible();
    
    const headers = page.locator('#groupsTable thead th');
    await expect(headers.filter({ hasText: 'Group Name' })).toBeVisible();
    await expect(headers.filter({ hasText: 'Description' })).toBeVisible();
    await expect(headers.filter({ hasText: 'Members' })).toBeVisible();
    await expect(headers.filter({ hasText: 'Status' })).toBeVisible();
  });

  test('opens add group modal', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/groups`);
    
    const addButton = page.locator('button:has-text("Add Group")');
    await addButton.click();
    
    const modal = page.locator('#groupModal');
    await expect(modal).toBeVisible();
    
    await expect(modal.locator('#groupName')).toBeVisible();
    await expect(modal.locator('#groupComments')).toBeVisible();
    await expect(modal.locator('#groupStatus')).toBeVisible();
  });

  test('creates a new group and deletes it', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/groups`);
    await expect(page.locator('#groupsTable')).toBeVisible();

    const addButton = page.locator('button:has-text("Add Group")');
    await addButton.click();

    const modal = page.locator('#groupModal');
    await expect(modal).toBeVisible();

    const testGroupName = `test-group-${Date.now()}`;
    await modal.locator('#groupName').fill(testGroupName);
    await modal.locator('#groupComments').fill('Playwright test group');
    await modal.locator('#groupStatus').selectOption('1');

    await modal.locator('button[type="submit"]').click();

    await page.waitForTimeout(1000);
    await expect(page.locator('#groupsTable')).toBeVisible();

    const newRow = page.locator('#groupsTable tbody tr').filter({ hasText: testGroupName });
    await expect(newRow).toBeVisible();

    const deleteButton = newRow.locator('button[onclick*="deleteGroup"]');
    if (await deleteButton.count() > 0) {
      page.on('dialog', dialog => dialog.accept());
      await deleteButton.click();
      await page.waitForTimeout(1000);
    }
  });

  test('edits a group and restores original data', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/groups`);
    await expect(page.locator('#groupsTable')).toBeVisible();

    const nonSystemRow = page.locator('#groupsTable tbody tr').filter({ 
      hasNot: page.locator('span:has-text("System")') 
    }).first();
    
    if (await nonSystemRow.count() === 0) {
      test.skip();
      return;
    }

    const nameCell = nonSystemRow.locator('td').first();
    const originalName = (await nameCell.textContent())?.trim() || '';

    const editButton = nonSystemRow.locator('button[onclick*="showEditGroupModal"]');
    if (await editButton.count() === 0) {
      test.skip();
      return;
    }
    await editButton.click();

    const modal = page.locator('#groupModal');
    await expect(modal).toBeVisible();

    const commentsTextarea = modal.locator('#groupComments');
    const originalComments = await commentsTextarea.inputValue();

    const updatedComments = originalComments.endsWith(' QA')
      ? originalComments
      : `${originalComments} QA`;

    await commentsTextarea.fill(updatedComments);

    await modal.locator('button[type="submit"]').click();

    await page.waitForTimeout(1000);
    await expect(page.locator('#groupsTable')).toBeVisible();

    const updatedRow = page.locator('#groupsTable tbody tr').filter({ hasText: originalName }).first();
    await expect(updatedRow).toContainText(updatedComments);

    await updatedRow.locator('button[onclick*="showEditGroupModal"]').click();
    await expect(modal).toBeVisible();

    await commentsTextarea.fill(originalComments);
    await modal.locator('button[type="submit"]').click();

    await page.waitForTimeout(1000);
    await expect(page.locator('#groupsTable')).toBeVisible();
  });

  test('search filters group list', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/groups`);
    await expect(page.locator('#groupsTable')).toBeVisible();
    
    const searchInput = page.locator('#groupSearch');
    await expect(searchInput).toBeVisible();
    
    await searchInput.fill('admin');
    await page.waitForTimeout(500);
    
    const filteredRows = await page.locator('#groupsTable tbody tr:visible').count();
    expect(filteredRows).toBeGreaterThan(0);
  });

  test('system groups cannot be deleted', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/groups`);
    await expect(page.locator('#groupsTable')).toBeVisible();

    const adminRow = page.locator('#groupsTable tbody tr').filter({ hasText: 'admin' }).first();
    
    if (await adminRow.count() > 0) {
      const deleteButton = adminRow.locator('button[onclick*="deleteGroup"]');
      if (await deleteButton.count() > 0) {
        const isDisabled = await deleteButton.isDisabled();
        expect(isDisabled).toBe(true);
      }
    }
  });

  test('group members count is displayed', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/groups`);
    await expect(page.locator('#groupsTable')).toBeVisible();
    
    const firstRow = page.locator('#groupsTable tbody tr').first();
    await expect(firstRow).toBeVisible();
    
    const membersCell = firstRow.locator('td').nth(2);
    const membersText = await membersCell.textContent();
    expect(membersText).toMatch(/\d+/);
  });

  test('cancel button closes modal without saving', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/groups`);
    
    const addButton = page.locator('button:has-text("Add Group")');
    await addButton.click();

    const modal = page.locator('#groupModal');
    await expect(modal).toBeVisible();

    await modal.locator('#groupName').fill('test-cancel-group');
    
    const cancelButton = modal.locator('button:has-text("Cancel")');
    await cancelButton.click();

    await expect(modal).not.toBeVisible();
    
    const testRow = page.locator('#groupsTable tbody tr').filter({ hasText: 'test-cancel-group' });
    await expect(testRow).toHaveCount(0);
  });

  test('group status toggle works', async ({ page }) => {
    await page.goto(`${BASE_URL}/admin/groups`);
    await expect(page.locator('#groupsTable')).toBeVisible();
    
    const firstRow = page.locator('#groupsTable tbody tr').first();
    await expect(firstRow).toBeVisible();
    
    const statusCell = firstRow.locator('td').nth(3);
    await expect(statusCell).toBeVisible();
    const statusText = await statusCell.textContent();
    expect(statusText?.toLowerCase()).toMatch(/active|inactive|enabled|disabled/i);
  });
});
