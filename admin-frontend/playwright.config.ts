import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  // Increase timeout for production tests
  timeout: 60000,
  // Retry failed tests to reduce flakiness
  retries: process.env.CI ? 2 : 0,
  // Run tests serially to avoid SSO session conflicts
  fullyParallel: false,
  workers: 1,
  use: {
    // Base URL for dev-nginx portal (make dev-full)
    baseURL: process.env.E2E_BASE_URL || 'http://localhost:8088',
    trace: 'on-first-retry',
    // Increase action timeout
    actionTimeout: 15000,
    // Increase navigation timeout
    navigationTimeout: 30000,
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
