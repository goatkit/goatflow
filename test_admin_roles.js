const { chromium } = require('playwright');
const fs = require('fs');
const path = require('path');

async function testAdminRoles() {
  const browser = await chromium.launch({ 
    headless: false,
    slowMo: 1000 // Add delay to see what's happening
  });
  const context = await browser.newContext();
  const page = await context.newPage();

  // Set up console and error logging
  const consoleMessages = [];
  const errors = [];
  const networkErrors = [];

  page.on('console', msg => {
    const message = `[${msg.type()}] ${msg.text()}`;
    console.log('Console:', message);
    consoleMessages.push(message);
  });

  page.on('pageerror', error => {
    const errorMsg = error.toString();
    console.log('Page Error:', errorMsg);
    errors.push(errorMsg);
  });

  page.on('response', response => {
    if (response.status() >= 400) {
      const errorMsg = `${response.status()} ${response.url()}`;
      console.log('Network Error:', errorMsg);
      networkErrors.push(errorMsg);
    }
  });

  try {
    console.log('Navigating to Admin Roles page...');
    await page.goto('http://localhost:8080/admin/roles', { waitUntil: 'networkidle' });

    // Take initial screenshot
    await page.screenshot({ path: '/tmp/admin_roles_initial.png', fullPage: true });
    console.log('Initial screenshot saved to /tmp/admin_roles_initial.png');

    // Wait for page to load completely
    await page.waitForTimeout(2000);

    // Check if we're redirected to login
    const currentUrl = page.url();
    console.log('Current URL:', currentUrl);

    if (currentUrl.includes('login')) {
      console.log('Redirected to login page - this is expected behavior');
      await page.screenshot({ path: '/tmp/admin_roles_login_redirect.png', fullPage: true });
      console.log('Login redirect screenshot saved');
      
      // Try to proceed anyway to see what happens
      await page.goto('http://localhost:8080/admin/roles', { waitUntil: 'networkidle' });
      await page.waitForTimeout(2000);
    }

    // Look for membership buttons or any interactive elements
    const membershipButtons = await page.locator('button, a').filter({ hasText: /member|Member|MEMBER/ }).all();
    console.log(`Found ${membershipButtons.length} potential membership buttons`);

    // Look for any buttons that might be membership-related
    const allButtons = await page.locator('button').all();
    console.log(`Found ${allButtons.length} total buttons on page`);

    // Take screenshot of current state
    await page.screenshot({ path: '/tmp/admin_roles_loaded.png', fullPage: true });
    console.log('Page loaded screenshot saved to /tmp/admin_roles_loaded.png');

    // Try to find and click membership buttons
    if (membershipButtons.length > 0) {
      console.log('Clicking first membership button...');
      await membershipButtons[0].click();
      await page.waitForTimeout(2000);
      
      await page.screenshot({ path: '/tmp/admin_roles_after_click.png', fullPage: true });
      console.log('After click screenshot saved');
    } else {
      // If no specific membership buttons, try clicking any button that looks interactive
      const interactiveButtons = await page.locator('button:not([disabled])').all();
      if (interactiveButtons.length > 0) {
        console.log('No membership buttons found, trying first available button...');
        await interactiveButtons[0].click();
        await page.waitForTimeout(2000);
        
        await page.screenshot({ path: '/tmp/admin_roles_button_click.png', fullPage: true });
        console.log('Button click screenshot saved');
      }
    }

    // Check for any modals or popups that might have opened
    const modals = await page.locator('[role="dialog"], .modal, .popup').all();
    if (modals.length > 0) {
      console.log(`Found ${modals.length} modal(s) or popup(s)`);
      await page.screenshot({ path: '/tmp/admin_roles_modal.png', fullPage: true });
    }

    // Get page content to analyze
    const pageContent = await page.content();
    const hasErrorContent = pageContent.includes('error') || pageContent.includes('Error') || pageContent.includes('404') || pageContent.includes('500');
    
    if (hasErrorContent) {
      console.log('Page contains error indicators');
      await page.screenshot({ path: '/tmp/admin_roles_errors.png', fullPage: true });
    }

    // Final wait to catch any delayed errors
    await page.waitForTimeout(3000);

  } catch (error) {
    console.log('Test Error:', error.message);
    await page.screenshot({ path: '/tmp/admin_roles_test_error.png', fullPage: true });
  }

  // Create summary report
  const report = {
    timestamp: new Date().toISOString(),
    url: page.url(),
    consoleMessages,
    errors,
    networkErrors,
    summary: {
      totalConsoleMessages: consoleMessages.length,
      totalErrors: errors.length,
      totalNetworkErrors: networkErrors.length,
      hasErrors: errors.length > 0 || networkErrors.length > 0
    }
  };

  // Save report
  fs.writeFileSync('/tmp/admin_roles_test_report.json', JSON.stringify(report, null, 2));
  console.log('\n=== TEST SUMMARY ===');
  console.log(`Console Messages: ${consoleMessages.length}`);
  console.log(`Page Errors: ${errors.length}`);
  console.log(`Network Errors: ${networkErrors.length}`);
  console.log('Full report saved to /tmp/admin_roles_test_report.json');

  await browser.close();
  return report;
}

testAdminRoles().catch(console.error);