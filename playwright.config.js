// Playwright Configuration for Dynamic Module Testing
import { defineConfig, devices } from "@playwright/test";
import path from "path";
import { BASE_URL } from "./tests/acceptance/base-url.js";

const resultsDir = process.env.PLAYWRIGHT_RESULTS_DIR || "test-results";
const outputDir = process.env.PLAYWRIGHT_OUTPUT_DIR || path.join(resultsDir, "artifacts");
const htmlReportDir = process.env.PLAYWRIGHT_HTML_REPORT_DIR || "generated/playwright-report";

/**
 * @see https://playwright.dev/docs/test-configuration
 */
const skipWebServer = !!process.env.PLAYWRIGHT_SKIP_WEBSERVER;

export default defineConfig({
  testDir: "./tests/acceptance",
  /* Run tests in files in parallel */
  fullyParallel: true,
  /* Fail the build on CI if you accidentally left test.only in the source code. */
  forbidOnly: !!process.env.CI,
  /* Retry on CI only */
  retries: process.env.CI ? 2 : 0,
  /* Opt out of parallel tests on CI. */
  workers: process.env.CI ? 1 : undefined,
  /* Reporter to use. See https://playwright.dev/docs/test-reporters */
  reporter: [
    ["html", { outputFolder: htmlReportDir }],
    ["junit", { outputFile: path.join(resultsDir, "junit.xml") }],
    ["json", { outputFile: path.join(resultsDir, "results.json") }],
  ],
  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    /* Base URL to use in actions like `await page.goto('/')`. */
    baseURL: BASE_URL,

    /* Collect trace when retrying the failed test. See https://playwright.dev/docs/trace-viewer */
    trace: "on-first-retry",

    /* Take screenshot on failure */
    screenshot: "only-on-failure",

    /* Record video on failure */
    video: "retain-on-failure",

    /* Global timeout for actions */
    actionTimeout: 10000,
  },

  /* Configure projects for major browsers */
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },

    {
      name: "firefox",
      use: { ...devices["Desktop Firefox"] },
    },

    {
      name: "webkit",
      use: { ...devices["Desktop Safari"] },
    },

    /* Test against mobile viewports. */
    {
      name: "Mobile Chrome",
      use: { ...devices["Pixel 5"] },
    },
    {
      name: "Mobile Safari",
      use: { ...devices["iPhone 12"] },
    },

    /* Test against branded browsers. */
    // {
    //   name: 'Microsoft Edge',
    //   use: { ...devices['Desktop Edge'], channel: 'msedge' },
    // },
    // {
    //   name: 'Google Chrome',
    //   use: { ...devices['Desktop Chrome'], channel: 'chrome' },
    // },
  ],

  /* Run your local dev server before starting the tests */
  webServer: skipWebServer
    ? undefined
    : [
        {
          command: "make restart",
          port: 8080,
          reuseExistingServer: !process.env.CI,
          timeout: 30000,
        },
      ],

  /* Global test timeout */
  timeout: 30000,

  /* Expect timeout for assertions */
  expect: {
    timeout: 5000,
  },

  /* Output directories */
  outputDir,

  /* Test metadata */
  metadata: {
    "test-suite": "Dynamic Module System Acceptance Tests",
    version: "1.0.0",
    description:
      "Side-by-side comparison of static vs dynamic module implementations",
  },
});
