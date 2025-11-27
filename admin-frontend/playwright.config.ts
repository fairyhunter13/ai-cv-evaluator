import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  use: {
    // Base URL for dev-nginx portal (make dev-full)
    baseURL: process.env.E2E_BASE_URL || 'http://localhost:8088',
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
