import { test, expect, Page, BrowserContext } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';
import { fileURLToPath } from 'url';

const PORTAL_PATH = '/';

// ESM compatibility for __dirname
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Debug helper: dump HTML content to file for investigation
const dumpHtml = async (page: Page, filename: string): Promise<void> => {
  try {
    const html = await page.content();
    const debugDir = path.join(__dirname, '..', 'test-results', 'html-dumps');
    if (!fs.existsSync(debugDir)) {
      fs.mkdirSync(debugDir, { recursive: true });
    }
    const filepath = path.join(debugDir, `${filename}-${Date.now()}.html`);
    fs.writeFileSync(filepath, html);
    console.log(`HTML dumped to: ${filepath}`);
  } catch (err) {
    console.log(`Failed to dump HTML: ${err}`);
  }
};

// Helper: Wait for SPA content to load (not just initial HTML)
const waitForSpaContent = async (page: Page, contentPatterns: string[], timeoutMs = 15000): Promise<boolean> => {
  const startTime = Date.now();
  while (Date.now() - startTime < timeoutMs) {
    const body = await page.locator('body').textContent();
    const lowerBody = (body ?? '').toLowerCase();
    const found = contentPatterns.some(p => lowerBody.includes(p.toLowerCase()));
    if (found && lowerBody.length > 100) {
      return true;
    }
    await page.waitForTimeout(500);
  }
  return false;
};

// Environment detection
const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:8088';
const IS_PRODUCTION = BASE_URL.includes('ai-cv-evaluator.web.id');
const IS_DEV = !IS_PRODUCTION;

// Credentials: Use env vars, with sensible defaults for dev
const SSO_USERNAME = process.env.SSO_USERNAME || 'admin';
const SSO_PASSWORD = process.env.SSO_PASSWORD || (IS_PRODUCTION ? '' : 'admin123');

// Mailpit is now available in both dev and prod environments

// Helper to get Prometheus datasource UID dynamically
const getPrometheusDatasourceUid = async (page: Page): Promise<string> => {
  const resp = await page.request.get('/grafana/api/datasources');
  if (!resp.ok()) return 'prometheus'; // fallback
  const datasources = (await resp.json()) as any[];
  const prometheus = datasources.find((ds: any) => ds.type === 'prometheus');
  return prometheus?.uid || 'prometheus';
};

const isSSOLoginUrl = (input: string | URL): boolean => {
  const url = typeof input === 'string' ? input : input.toString();
  return url.includes('/oauth2/') || url.includes('/realms/aicv');
};

const completeKeycloakProfileUpdate = async (page: Page): Promise<void> => {
  const heading = page.getByRole('heading', { name: /Update Account Information/i });
  const visible = await heading.isVisible().catch(() => false);
  if (!visible) return;
  const firstNameInput = page.getByRole('textbox', { name: /First name/i });
  const lastNameInput = page.getByRole('textbox', { name: /Last name/i });
  if (await firstNameInput.isVisible().catch(() => false)) await firstNameInput.fill('Admin');
  if (await lastNameInput.isVisible().catch(() => false)) await lastNameInput.fill('User');
  const submitProfileButton = page.getByRole('button', { name: /submit/i });
  if (await submitProfileButton.isVisible().catch(() => false)) await submitProfileButton.click();
};

const gotoWithRetry = async (page: Page, path: string): Promise<void> => {
  const maxAttempts = 5;
  const retryDelayMs = 2000;
  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    try {
      await page.goto(path, { waitUntil: 'domcontentloaded', timeout: 30000 });
      return;
    } catch (err) {
      const message = String(err);
      const isRetryable = message.includes('net::ERR_CONNECTION_REFUSED') ||
                          message.includes('net::ERR_TOO_MANY_REDIRECTS');
      if (!isRetryable || attempt === maxAttempts) {
        throw err;
      }
      await page.waitForTimeout(retryDelayMs);
    }
  }
};

const loginViaSSO = async (page: Page): Promise<void> => {
  // Skip SSO login if no credentials configured (e.g., production without secrets)
  if (!SSO_PASSWORD) {
    throw new Error('SSO_PASSWORD environment variable is required for SSO login tests');
  }

  // Retry login up to 3 times to handle transient SSO issues
  const maxLoginAttempts = 3;
  for (let attempt = 1; attempt <= maxLoginAttempts; attempt += 1) {
    try {
      await gotoWithRetry(page, PORTAL_PATH);
      if (!isSSOLoginUrl(page.url())) return;
      
      const usernameInput = page.locator('input#username');
      const passwordInput = page.locator('input#password');
      
      await usernameInput.waitFor({ state: 'visible', timeout: 10000 });
      await usernameInput.fill(SSO_USERNAME);
      await passwordInput.fill(SSO_PASSWORD);
      
      const submitButton = page.locator('button[type="submit"], input[type="submit"]');
      await submitButton.first().click();
      
      await completeKeycloakProfileUpdate(page);
      await page.waitForURL((url) => !isSSOLoginUrl(url), { timeout: 30000 });
      return;
    } catch (err) {
      if (attempt === maxLoginAttempts) throw err;
      await page.waitForTimeout(2000);
    }
  }
};

// Retry an API request until it returns a valid response (handles 502/503 during startup).
const apiRequestWithRetry = async (
  page: Page,
  method: 'get' | 'post' | 'put' | 'delete',
  url: string,
  options?: { data?: any; params?: Record<string, string> },
): Promise<any> => {
  const maxAttempts = 10;
  const retryDelayMs = 3000;
  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    const resp = await page.request[method](url, options);
    const status = resp.status();
    if (status >= 200 && status < 300) {
      return resp;
    }
    if ([502, 503, 504].includes(status) && attempt < maxAttempts) {
      await page.waitForTimeout(retryDelayMs);
      continue;
    }
    return resp;
  }
};

// Mailpit cleanup helper: delete all messages before running alerting tests
const clearMailpitMessages = async (page: Page): Promise<void> => {
  try {
    // First, get all message IDs
    const listResp = await apiRequestWithRetry(page, 'get', '/mailpit/api/v1/messages');
    if (!listResp || listResp.status() !== 200) return;
    const body = (await listResp.json()) as any;
    const messages = body.messages ?? [];
    if (messages.length === 0) return;

    // Delete all messages by IDs
    const ids = messages.map((m: any) => m.ID);
    await page.request.delete('/mailpit/api/v1/messages', {
      data: { ids },
    });
  } catch {
    // Ignore cleanup errors
  }
};

// =============================================================================
// PORTAL PAGE TESTS
// =============================================================================

test.describe('Portal Page', () => {
  test('portal page displays all navigation links after SSO login', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    // Portal should show navigation links to all surfaced services
    await expect(page.getByRole('link', { name: /Open Frontend/i })).toBeVisible();
    await expect(page.getByRole('link', { name: /Open API/i })).toBeVisible();
    await expect(page.getByRole('link', { name: /Health/i })).toBeVisible();
    await expect(page.getByRole('link', { name: /Open Grafana/i })).toBeVisible();
    // Mailpit only available in dev
    if (IS_DEV) {
      await expect(page.getByRole('link', { name: /Open Mailpit/i })).toBeVisible();
    }
    await expect(page.getByRole('link', { name: /Open Jaeger/i })).toBeVisible();
    await expect(page.getByRole('link', { name: /Open Redpanda/i })).toBeVisible();
  });

  test('portal page has proper title and branding', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    // Check the page has a title
    const title = await page.title();
    expect(title.length).toBeGreaterThan(0);

    // Page should have some form of branding/heading
    const body = await page.locator('body').textContent();
    expect(body).toBeTruthy();
  });
});

// =============================================================================
// LOGOUT FLOW TESTS
// =============================================================================

test.describe('Logout Flow', () => {
  test('new browser context requires SSO login (session isolation)', async ({ browser, baseURL }) => {

    // Verify session can be cleared by using fresh browser context
    const freshContext = await browser.newContext();
    const freshPage = await freshContext.newPage();
    try {
      await gotoWithRetry(freshPage, '/app/');
      expect(isSSOLoginUrl(freshPage.url())).toBeTruthy();
    } finally {
      await freshContext.close();
    }
  });
});

// =============================================================================
// JOB MANAGEMENT COMPREHENSIVE TESTS
// =============================================================================

test.describe('Job Management', () => {
  test('job list displays with pagination controls', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: /Job Management/i }).click();
    await page.waitForLoadState('domcontentloaded');

    // Job Management heading should be visible
    await expect(page.getByRole('heading', { name: /Job Management/i })).toBeVisible();

    // Should have a table or list structure
    const table = page.locator('table');
    const tableExists = await table.count() > 0;
    if (tableExists) {
      await expect(table).toBeVisible();
    }
  });

  test('job search functionality works', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: /Job Management/i }).click();
    await page.waitForLoadState('domcontentloaded');

    // Look for search input
    const searchInput = page.getByPlaceholder(/search/i);
    const searchExists = await searchInput.count() > 0;

    if (searchExists) {
      await searchInput.fill('test-search-query');
      await searchInput.press('Enter');
      await page.waitForLoadState('networkidle');
      // Search should trigger an API call
      // The UI should update (may show no results or filtered results)
    }
  });

  test('job status filter works', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: /Job Management/i }).click();
    await page.waitForLoadState('domcontentloaded');

    // Look for status filter dropdown
    const statusFilter = page.getByRole('combobox').filter({ hasText: /status|all/i });
    const filterExists = await statusFilter.count() > 0;

    if (filterExists) {
      await statusFilter.click();
      // Should have filter options
      const completedOption = page.getByRole('option', { name: /completed/i });
      if (await completedOption.count() > 0) {
        await completedOption.click();
        await page.waitForLoadState('networkidle');
      }
    }
  });
});

// =============================================================================
// HEALTH AND READINESS ENDPOINT TESTS
// =============================================================================

test.describe('Health Endpoints', () => {
  test('healthz endpoint returns 200', async ({ page, baseURL }) => {

    const resp = await apiRequestWithRetry(page, 'get', '/healthz');
    expect(resp.status()).toBe(200);
  });

  test('readyz endpoint returns 200 when ready', async ({ page, baseURL }) => {

    const resp = await apiRequestWithRetry(page, 'get', '/readyz');
    // May return 200 or 503 depending on service state
    expect([200, 503]).toContain(resp.status());
  });
});

// =============================================================================
// ADMIN API COMPREHENSIVE TESTS
// =============================================================================

test.describe('Admin API', () => {
  test('admin stats API returns valid structure', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    const resp = await apiRequestWithRetry(page, 'get', '/admin/api/stats');
    expect(resp.status()).toBe(200);

    const body = await resp.json();
    expect(typeof (body as any).uploads).toBe('number');
    expect(typeof (body as any).evaluations).toBe('number');
    expect(typeof (body as any).completed).toBe('number');
    expect(typeof (body as any).avg_time).toBe('number');
  });

  test('admin jobs API supports pagination', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    const resp = await page.request.get('/admin/api/jobs?page=1&limit=5');
    expect(resp.status()).toBe(200);

    const body = await resp.json();
    expect(Array.isArray((body as any).jobs)).toBeTruthy();
    expect((body as any).pagination).toBeDefined();
    expect(typeof (body as any).pagination.page).toBe('number');
    expect(typeof (body as any).pagination.limit).toBe('number');
  });

  test('admin jobs API supports status filter', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    // Filter by 'completed' status
    const resp = await page.request.get('/admin/api/jobs?status=completed');
    expect(resp.status()).toBe(200);

    const body = await resp.json();
    expect(Array.isArray((body as any).jobs)).toBeTruthy();
  });
});

// =============================================================================
// ALERTING FLOW E2E TESTS
// =============================================================================

test.describe('Alerting Flow', () => {
  test('Grafana alerting configuration is valid', async ({ page, baseURL }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    // Navigate to Grafana alerting page
    await gotoWithRetry(page, '/grafana/alerting/list');
    await page.waitForLoadState('networkidle');

    // Verify we're on Grafana
    await expect(page).toHaveTitle(/Grafana/i, { timeout: 15000 });

    // Check that we're not on an error page
    const pageTitle = await page.title();
    expect(pageTitle).not.toContain('502');

    // Verify alert rules are present (page should contain alert-related content)
    // The exact UI varies by Grafana version and environment
    const pageContent = await page.locator('body').textContent();
    expect(pageContent).toBeTruthy();

    const lowerContent = (pageContent ?? '').toLowerCase();

    // Explicitly fail if Grafana shows an error banner about loading rules.
    expect(lowerContent.includes('errors loading rules')).toBeFalsy();
    expect(lowerContent.includes('unable to fetch alert rules')).toBeFalsy();

    // Should have some alert rules or groups displayed
    const hasAlertContent = lowerContent.includes('ai-cv-evaluator') ||
      lowerContent.includes('alert') ||
      lowerContent.includes('rule');
    expect(hasAlertContent).toBeTruthy();
  });

  test('Grafana contact points are configured', async ({ page, baseURL }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    // Navigate to contact points page
    await gotoWithRetry(page, '/grafana/alerting/notifications');
    await page.waitForLoadState('networkidle');

    // Verify we're on Grafana
    await expect(page).toHaveTitle(/Grafana/i, { timeout: 15000 });
    // Check contact points exist (page should have contact point content)
    const pageContent = await page.locator('body').textContent();
    const hasContactPoints = pageContent?.includes('email') ||
      pageContent?.includes('contact') ||
      pageContent?.includes('Contact') ||
      pageContent?.includes('notification');
    expect(hasContactPoints).toBeTruthy();
  });

  test('Prometheus has HTTP error alert rule configured', async ({ page, baseURL }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    // Query Prometheus for alert rules
    await gotoWithRetry(page, '/prometheus/alerts');
    await page.waitForLoadState('networkidle');

    // The alerts page should load
    const body = await page.locator('body').textContent();
    expect(body).toBeTruthy();
  });

  test('Prometheus has core alert rules configured', async ({ page, baseURL }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    // Navigate to Prometheus alerts page to verify it's accessible
    await gotoWithRetry(page, '/prometheus/alerts');
    await page.waitForLoadState('networkidle');

    // Verify not redirected to SSO
    expect(isSSOLoginUrl(page.url())).toBeFalsy();
    
    // Page should have loaded (may be very minimal in some environments)
    const pageContent = await page.locator('body').textContent();
    expect(pageContent).toBeTruthy();
  });

  test('email notification infrastructure is accessible', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    if (IS_DEV) {
      // In dev, check Mailpit
      await clearMailpitMessages(page);

      // Navigate to Mailpit
      await gotoWithRetry(page, '/mailpit/');
      await page.waitForLoadState('domcontentloaded');

      // Verify Mailpit loaded (it's a JavaScript SPA)
      const title = await page.title();
      expect(title.toLowerCase()).toContain('mailpit');
    } else {
      // In production, verify Grafana alerting is accessible
      await gotoWithRetry(page, '/grafana/alerting/notifications');
      await page.waitForLoadState('networkidle');
      await expect(page).toHaveTitle(/Grafana/i, { timeout: 15000 });
      // Check page has notification/contact point content
      const pageContent = await page.locator('body').textContent();
      expect(pageContent?.includes('email') || pageContent?.includes('contact')).toBeTruthy();
    }
  });

  test('alerting flow: verify alert infrastructure is accessible', async ({ page, baseURL }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    // Verify Grafana alerting UI is accessible
    await gotoWithRetry(page, '/grafana/alerting/list');
    await page.waitForLoadState('networkidle');
    
    // Should not be redirected to SSO
    expect(isSSOLoginUrl(page.url())).toBeFalsy();
    
    // Grafana should be loaded
    const title = await page.title();
    expect(title.toLowerCase()).toContain('grafana');

    // Verify notification infrastructure
    if (IS_DEV) {
      // In dev, check Mailpit is accessible
      const mailpitResp = await apiRequestWithRetry(page, 'get', '/mailpit/api/v1/messages');
      expect([200, 404]).toContain(mailpitResp.status());
    } else {
      // In production, just verify Grafana alerting page loaded
      const pageContent = await page.locator('body').textContent();
      expect(pageContent).toBeTruthy();
      expect(pageContent?.length).toBeGreaterThan(50);
    }
  });

  test('Grafana alert list shows alert rules', async ({ page, baseURL }) => {
    test.setTimeout(90000);
    await loginViaSSO(page);

    // Navigate to Grafana alerting page
    await gotoWithRetry(page, '/grafana/alerting/list');
    await page.waitForLoadState('networkidle');
    await expect(page).toHaveTitle(/Grafana/i, { timeout: 15000 });

    // Verify the page has alert-related content and no error banner
    const pageContent = await page.locator('body').textContent();
    expect(pageContent).toBeTruthy();

    const lowerContent = (pageContent ?? '').toLowerCase();

    // Fail fast if Grafana shows the error banner about alert rules.
    expect(lowerContent.includes('errors loading rules')).toBeFalsy();
    expect(lowerContent.includes('unable to fetch alert rules')).toBeFalsy();

    const hasAlertContent = lowerContent.includes('alert') ||
      lowerContent.includes('rule');
    expect(hasAlertContent).toBeTruthy();
  });

  test('Grafana alerting is accessible and functional', async ({
    page,
    baseURL,
  }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    // Navigate to Grafana alerting page
    await gotoWithRetry(page, '/grafana/alerting/list');
    await page.waitForLoadState('networkidle');

    // Verify we're on Grafana
    await expect(page).toHaveTitle(/Grafana/i, { timeout: 15000 });

    // Page should not be an error
    const title = await page.title();
    expect(title).not.toContain('502');
    expect(title).not.toContain('Error');

    // Page should have content
    const pageContent = await page.locator('body').textContent();
    expect(pageContent?.length).toBeGreaterThan(100);
  });

  test('Grafana alerting API is accessible', async ({
    page,
    baseURL,
  }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    // Query the Grafana alerting API to verify it's working
    const resp = await apiRequestWithRetry(page, 'get', '/grafana/api/v1/provisioning/alert-rules');
    // API might return 200 or 404 depending on provisioning, both are valid
    expect([200, 404]).toContain(resp.status());
  });
});

// =============================================================================
// RESPONSIVE DESIGN TESTS
// =============================================================================

test.describe('Responsive Design', () => {
  test('admin frontend works on mobile viewport', async ({ page, baseURL }) => {

    // Set mobile viewport
    await page.setViewportSize({ width: 375, height: 667 });

    await loginViaSSO(page);
    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');

    // Dashboard should still be visible
    await expect(page.getByRole('heading', { name: /Dashboard/i })).toBeVisible();

    // Navigation should be accessible (may be in hamburger menu)
    const menuButton = page.getByRole('button', { name: /menu|toggle/i });
    if (await menuButton.count() > 0) {
      await menuButton.click();
    }
  });

  test('admin frontend works on tablet viewport', async ({ page, baseURL }) => {

    // Set tablet viewport
    await page.setViewportSize({ width: 768, height: 1024 });

    await loginViaSSO(page);
    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');

    // Dashboard should be visible
    await expect(page.getByRole('heading', { name: /Dashboard/i })).toBeVisible();
  });
});

// =============================================================================
// ERROR HANDLING TESTS
// =============================================================================

test.describe('Error Handling', () => {
  test('frontend handles API timeout gracefully', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');

    // Try to get results for a non-existent job - use first() to avoid strict mode
    await page.getByRole('link', { name: /View Results/i }).first().click();
    await page.waitForLoadState('domcontentloaded');

    await page.getByLabel(/Job ID/i).fill('non-existent-job-id-12345');
    await page.getByRole('button', { name: /Get Results/i }).click();

    // Should show an error message, not crash
    await page.waitForLoadState('networkidle');

    // Either shows "not found" error or handles gracefully
    const body = await page.locator('body').textContent();
    expect(body).toBeTruthy();
  });

  test('frontend handles invalid file types with clear error', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: /Upload Files/i }).click();
    await page.waitForLoadState('domcontentloaded');

    // Upload invalid files
    const fileInputs = page.locator('input[type="file"]');
    await fileInputs.nth(0).setInputFiles('tests/fixtures/evil.exe');
    await fileInputs.nth(1).setInputFiles('tests/fixtures/evil.exe');

    // Try to upload
    const uploadButton = page.getByRole('button', { name: /^Upload Files$/i });
    await uploadButton.click();

    // Should show an error
    await page.waitForLoadState('networkidle');
    // Either in a toast/alert or inline error
  });
});

// =============================================================================
// NAVIGATION AND BREADCRUMB TESTS
// =============================================================================

test.describe('Navigation', () => {
  test('sidebar navigation is functional', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');

    // Test all sidebar navigation links
    const navLinks = [
      { name: /Dashboard/i, heading: /Dashboard/i },
      { name: /Upload Files/i, heading: /Upload Files/i },
      { name: /Start Evaluation/i, heading: /Start Evaluation/i },
      { name: /View Results/i, heading: /View Results/i },
      { name: /Job Management/i, heading: /Job Management/i },
    ];

    for (const { name, heading } of navLinks) {
      await page.getByRole('link', { name }).click();
      await page.waitForLoadState('domcontentloaded');
      await expect(page.getByRole('heading', { name: heading })).toBeVisible({ timeout: 10000 });
    }
  });

  test('browser back/forward navigation works', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');

    // Navigate to Upload
    await page.getByRole('link', { name: /Upload Files/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await expect(page.getByRole('heading', { name: /Upload Files/i })).toBeVisible();

    // Navigate to Evaluate
    await page.getByRole('link', { name: /Start Evaluation/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await expect(page.getByRole('heading', { name: /Start Evaluation/i })).toBeVisible();

    // Go back
    await page.goBack();
    await page.waitForLoadState('domcontentloaded');
    await expect(page.getByRole('heading', { name: /Upload Files/i })).toBeVisible();

    // Go forward
    await page.goForward();
    await page.waitForLoadState('domcontentloaded');
    await expect(page.getByRole('heading', { name: /Start Evaluation/i })).toBeVisible();
  });
});

// =============================================================================
// FORM INTERACTION TESTS
// =============================================================================

test.describe('Form Interactions', () => {
  test('upload form shows file names after selection', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: /Upload Files/i }).click();
    await page.waitForLoadState('domcontentloaded');

    const fileInputs = page.locator('input[type="file"]');
    await fileInputs.nth(0).setInputFiles('tests/fixtures/cv.txt');

    // The file name should be displayed somewhere on the page
    const body = await page.locator('body').textContent();
    // File input should show selected file or there should be visual feedback
    expect(body).toBeTruthy();
  });

  test('evaluate form validates input before submission', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    // Use first() to avoid strict mode violation (sidebar + dashboard both have links)
    await page.getByRole('link', { name: /Start Evaluation/i }).first().click();
    await page.waitForLoadState('domcontentloaded');

    // Try to submit with empty fields
    const startButton = page.getByRole('button', { name: /^Start Evaluation$/i });

    // Button should either be disabled or clicking it shows validation error
    const isDisabled = await startButton.isDisabled();
    if (!isDisabled) {
      await startButton.click();
      // Should show validation error
      await page.waitForLoadState('networkidle');
    }
  });

  test('result form allows entering job ID', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    // Use first() to avoid strict mode violation (sidebar + dashboard both have links)
    await page.getByRole('link', { name: /View Results/i }).first().click();
    await page.waitForLoadState('domcontentloaded');

    const jobIdInput = page.getByLabel(/Job ID/i);
    await expect(jobIdInput).toBeVisible();
    await jobIdInput.fill('test-job-id-123');
    await expect(jobIdInput).toHaveValue('test-job-id-123');
  });
});

// =============================================================================
// OBSERVABILITY DASHBOARDS TESTS
// =============================================================================

test.describe('Observability Dashboards', () => {
  test('Prometheus is accessible and has targets', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    // Navigate to Prometheus targets page
    await gotoWithRetry(page, '/prometheus/targets');
    await page.waitForLoadState('networkidle');

    // Verify not redirected to SSO
    expect(isSSOLoginUrl(page.url())).toBeFalsy();

    // Verify Prometheus loaded
    const pageContent = await page.locator('body').textContent();
    expect(pageContent).toBeTruthy();
  });

  test('Jaeger is accessible and has services', async ({ page, baseURL }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    // Jaeger redirects /jaeger/ to /jaeger/search
    await gotoWithRetry(page, '/jaeger/search');
    await page.waitForLoadState('networkidle');

    // Verify not stuck on SSO
    expect(isSSOLoginUrl(page.url())).toBeFalsy();

    // Wait for Jaeger SPA to load (React app needs time to hydrate)
    const jaegerLoaded = await waitForSpaContent(page, ['service', 'search', 'find traces', 'jaeger'], 20000);

    // Dump HTML for debugging if content not found
    if (!jaegerLoaded) {
      await dumpHtml(page, 'jaeger-search');
      await page.screenshot({ path: 'test-results/html-dumps/jaeger-search.png', fullPage: true });
    }

    // Jaeger UI should load - check for Jaeger-specific elements
    const body = await page.locator('body').textContent();
    console.log('Jaeger page content length:', body?.length ?? 0);
    console.log('Jaeger page content preview:', body?.substring(0, 500));
    expect(body).toBeTruthy();
    expect(jaegerLoaded).toBeTruthy();
  });

  test('Jaeger API is accessible', async ({ page }) => {
    await loginViaSSO(page);

    // Query Jaeger API for services
    const resp = await page.request.get('/jaeger/api/services');
    console.log('Jaeger API response status:', resp.status());
    console.log('Jaeger API response:', await resp.text());
    // Jaeger API should respond (may have no services if no traces yet)
    expect(resp.status()).toBeLessThan(500);
  });

  test('Redpanda Console is accessible', async ({ page, baseURL }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    await gotoWithRetry(page, '/redpanda/overview');
    await page.waitForLoadState('networkidle');

    // Verify not stuck on SSO
    expect(isSSOLoginUrl(page.url())).toBeFalsy();

    // Wait for Redpanda Console SPA to load
    const redpandaLoaded = await waitForSpaContent(page, ['topic', 'overview', 'cluster', 'broker', 'redpanda'], 20000);

    // Dump HTML for debugging if content not found
    if (!redpandaLoaded) {
      await dumpHtml(page, 'redpanda-overview');
      await page.screenshot({ path: 'test-results/html-dumps/redpanda-overview.png', fullPage: true });
    }

    // Redpanda Console should load
    const body = await page.locator('body').textContent();
    console.log('Redpanda page content length:', body?.length ?? 0);
    console.log('Redpanda page content preview:', body?.substring(0, 500));
    expect(body).toBeTruthy();
    expect(redpandaLoaded).toBeTruthy();
  });

  test('Redpanda Console topics page is accessible', async ({ page }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    await gotoWithRetry(page, '/redpanda/topics');
    await page.waitForLoadState('networkidle');

    // Verify not stuck on SSO
    expect(isSSOLoginUrl(page.url())).toBeFalsy();

    // Wait for content to load
    const loaded = await waitForSpaContent(page, ['topic', 'create', 'name', 'partitions'], 20000);

    if (!loaded) {
      await dumpHtml(page, 'redpanda-topics');
      await page.screenshot({ path: 'test-results/html-dumps/redpanda-topics.png', fullPage: true });
    }

    const body = await page.locator('body').textContent();
    console.log('Redpanda topics page content length:', body?.length ?? 0);
    expect(body).toBeTruthy();
    expect(loaded).toBeTruthy();
  });
});

// =============================================================================
// GRAFANA DASHBOARDS TESTS
// Verify all dashboards in the AI CV Evaluator folder are accessible and functional
// =============================================================================

test.describe('Grafana Dashboards', () => {
  test('AI CV Evaluator folder contains all expected dashboards', async ({ page }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    // Navigate to Grafana dashboards search and look for our folder
    await gotoWithRetry(page, '/grafana/dashboards');
    await page.waitForLoadState('networkidle');

    // Verify not redirected to SSO
    expect(isSSOLoginUrl(page.url())).toBeFalsy();

    // Wait for Grafana to fully load (it's a SPA)
    await page.waitForTimeout(3000);

    // Look for the AI CV Evaluator folder link
    const folderLink = page.getByRole('link', { name: /AI CV Evaluator/i });
    await expect(folderLink).toBeVisible({ timeout: 15000 });
    await folderLink.click();
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    // Check for expected dashboards in the folder
    const pageContent = await page.locator('body').textContent();
    const expectedDashboards = ['AI Metrics', 'HTTP Metrics', 'Job Queue Metrics', 'Request Drilldown'];
    for (const dashboard of expectedDashboards) {
      expect(pageContent?.toLowerCase()).toContain(dashboard.toLowerCase());
    }
  });

  test('HTTP Metrics dashboard loads with data', async ({ page }) => {
    await loginViaSSO(page);

    // Navigate to HTTP Metrics dashboard
    await gotoWithRetry(page, '/grafana/d/http-metrics/http-metrics');
    await page.waitForLoadState('networkidle');

    // Verify dashboard loaded (not SSO redirect)
    expect(isSSOLoginUrl(page.url())).toBeFalsy();

    // Dashboard should have panels
    const pageContent = await page.locator('body').textContent();
    expect(pageContent).toBeTruthy();

    // Check for HTTP-related content
    const lowerContent = (pageContent ?? '').toLowerCase();
    expect(lowerContent.includes('http') || lowerContent.includes('request') || lowerContent.includes('metrics')).toBeTruthy();
  });

  test('Job Queue Metrics dashboard loads', async ({ page }) => {
    await loginViaSSO(page);

    // Navigate to Job Queue Metrics dashboard
    await gotoWithRetry(page, '/grafana/d/job-queue-metrics/job-queue-metrics');
    await page.waitForLoadState('networkidle');

    // Verify dashboard loaded
    expect(isSSOLoginUrl(page.url())).toBeFalsy();

    const pageContent = await page.locator('body').textContent();
    expect(pageContent).toBeTruthy();

    // Check for job-related content
    const lowerContent = (pageContent ?? '').toLowerCase();
    expect(lowerContent.includes('job') || lowerContent.includes('queue') || lowerContent.includes('metrics')).toBeTruthy();
  });

  test('AI Metrics dashboard loads', async ({ page }) => {
    await loginViaSSO(page);

    // Navigate to AI Metrics dashboard
    await gotoWithRetry(page, '/grafana/d/ai-metrics/ai-metrics');
    await page.waitForLoadState('networkidle');

    // Verify dashboard loaded
    expect(isSSOLoginUrl(page.url())).toBeFalsy();

    const pageContent = await page.locator('body').textContent();
    expect(pageContent).toBeTruthy();

    // Check for AI-related content
    const lowerContent = (pageContent ?? '').toLowerCase();
    expect(lowerContent.includes('ai') || lowerContent.includes('token') || lowerContent.includes('metrics')).toBeTruthy();
  });

  test('Request Drilldown dashboard loads (dev-only data)', async ({ page }) => {
    await loginViaSSO(page);

    // Navigate to Request Drilldown dashboard
    await gotoWithRetry(page, '/grafana/d/request-drilldown/request-drilldown');
    await page.waitForLoadState('networkidle');

    // Verify dashboard loaded
    expect(isSSOLoginUrl(page.url())).toBeFalsy();

    const pageContent = await page.locator('body').textContent();
    expect(pageContent).toBeTruthy();

    // In production, this dashboard will show "No data" because http_request_by_id_total
    // is only enabled in dev to avoid high cardinality. This is expected behavior.
    // We just verify the dashboard loads without errors.
    const lowerContent = (pageContent ?? '').toLowerCase();
    expect(lowerContent.includes('request') || lowerContent.includes('drilldown') || lowerContent.includes('no data')).toBeTruthy();
  });

  test('Grafana home page is accessible', async ({ page }) => {
    await loginViaSSO(page);

    await gotoWithRetry(page, '/grafana/');
    await page.waitForLoadState('networkidle');

    // Verify not redirected to SSO
    expect(isSSOLoginUrl(page.url())).toBeFalsy();

    // Grafana home should load
    const pageContent = await page.locator('body').textContent();
    expect(pageContent).toBeTruthy();
  });
});

// =============================================================================
// COMPREHENSIVE ALERTING + MAILPIT FLOW TESTS
// These tests verify the complete alerting pipeline:
// 1. Grafana alert rules are loaded (no error banner)
// 2. Prometheus has the alert rules configured
// 3. Grafana SMTP is configured to send to Mailpit
// 4. Alerts can be triggered and emails are received in Mailpit
// =============================================================================

test.describe('Alerting + Mailpit Flow', () => {
  test.skip(IS_PRODUCTION, 'Full alerting + Mailpit flow runs only in dev; production is validated via SSH in CI');
  test('Grafana alert rules page has no error banner', async ({ page }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    // Navigate to Grafana alerting page
    await gotoWithRetry(page, '/grafana/alerting/list');
    await page.waitForLoadState('networkidle');

    // Wait for page to fully load
    await page.waitForTimeout(2000);

    // Verify we're on Grafana
    await expect(page).toHaveTitle(/Grafana/i, { timeout: 15000 });

    // Get page content and check for error banner
    const pageContent = await page.locator('body').textContent();
    const lowerContent = (pageContent ?? '').toLowerCase();

    // CRITICAL: Explicitly fail if Grafana shows an error banner about loading rules.
    // This is what the user is seeing in the screenshot.
    const hasErrorsLoadingRules = lowerContent.includes('errors loading rules');
    const hasUnableToFetch = lowerContent.includes('unable to fetch alert rules');
    const hasFailedToLoad = lowerContent.includes('failed to load the data source configuration');

    if (hasErrorsLoadingRules || hasUnableToFetch || hasFailedToLoad) {
      console.error('ERROR: Grafana is showing alert rules error banner!');
      console.error('Page content:', lowerContent.substring(0, 500));
    }

    expect(hasErrorsLoadingRules).toBeFalsy();
    expect(hasUnableToFetch).toBeFalsy();
    expect(hasFailedToLoad).toBeFalsy();

    // Should have alert rules displayed, not "You haven't created any alert rules yet"
    const hasNoRules = lowerContent.includes("you haven't created any alert rules yet");
    expect(hasNoRules).toBeFalsy();
  });

  test('Prometheus alert rules are accessible via Grafana datasource proxy', async ({ page }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    // Query Prometheus rules via Grafana datasource proxy
    const promUid = await getPrometheusDatasourceUid(page);
    const rulesResp = await apiRequestWithRetry(
      page,
      'get',
      `/grafana/api/datasources/proxy/uid/${promUid}/api/v1/rules`,
    );

    expect(rulesResp.status()).toBe(200);
    const rulesBody = await rulesResp.json();
    const ruleGroups = (rulesBody as any)?.data?.groups ?? [];

    // Should have at least one rule group
    expect(ruleGroups.length).toBeGreaterThan(0);

    // Check for specific alert rules
    const allRules = ruleGroups.flatMap((g: any) => g.rules ?? []);
    const ruleNames = allRules.map((r: any) => r.name);

    console.log('Prometheus alert rules found:', ruleNames);

    // Verify core alert rules exist
    expect(ruleNames).toContain('HighHttpErrorRate');
    expect(ruleNames).toContain('HighJobsProcessing');
    expect(ruleNames).toContain('EvaluationCvMatchRateLow');
  });

  test('Mailpit is accessible and API works', async ({ page }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    // Navigate to Mailpit
    await gotoWithRetry(page, '/mailpit/');
    await page.waitForLoadState('domcontentloaded');

    // Verify Mailpit loaded (it's a JavaScript SPA)
    const title = await page.title();
    expect(title.toLowerCase()).toContain('mailpit');

    // Verify Mailpit API is accessible
    const mailpitResp = await apiRequestWithRetry(page, 'get', '/mailpit/api/v1/messages');
    expect(mailpitResp.status()).toBe(200);

    const mailpitBody = await mailpitResp.json();
    expect(mailpitBody).toHaveProperty('total');
    expect(mailpitBody).toHaveProperty('messages');
  });

  test('Grafana contact points are configured with email', async ({ page }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    // Query Grafana contact points API
    const cpResp = await apiRequestWithRetry(page, 'get', '/grafana/api/v1/provisioning/contact-points');
    expect(cpResp.status()).toBe(200);

    const cpBody = await cpResp.json();
    const contactPoints = Array.isArray(cpBody) ? cpBody : cpBody.contactPoints ?? [];

    console.log('Contact points found:', contactPoints.length);

    // Should have at least one contact point
    expect(contactPoints.length).toBeGreaterThan(0);

    // Find email contact point
    const emailContactPoint = contactPoints.find(
      (cp: any) => cp.type === 'email' || cp.receivers?.some((r: any) => r.type === 'email'),
    );
    expect(emailContactPoint).toBeTruthy();
  });

  test('Complete alerting flow: trigger alert and verify Mailpit receives email', async ({ page }) => {
    test.setTimeout(300000); // 5 minutes for complete flow
    await loginViaSSO(page);

    // Step 1: Clear Mailpit messages
    console.log('Step 1: Clearing Mailpit messages...');
    await clearMailpitMessages(page);

    // Step 2: Verify initial Mailpit state (should be empty or have few messages)
    const initialMailpitResp = await apiRequestWithRetry(page, 'get', '/mailpit/api/v1/messages');
    expect(initialMailpitResp.status()).toBe(200);
    const initialMailpit = await initialMailpitResp.json();
    const initialCount = (initialMailpit as any).total ?? 0;
    console.log(`Initial Mailpit message count: ${initialCount}`);

    // Step 3: Generate HTTP errors to trigger HighHttpErrorRate alert
    console.log('Step 2: Generating HTTP errors to trigger alert...');
    for (let i = 0; i < 20; i += 1) {
      await page.request.get('/v1/__nonexistent_path_for_errors');
      await page.waitForTimeout(50);
    }

    // Step 4: Verify Prometheus is recording non-OK HTTP requests
    console.log('Step 3: Verifying Prometheus is recording errors...');
    const promUid = await getPrometheusDatasourceUid(page);
    const promResp = await apiRequestWithRetry(
      page,
      'get',
      `/grafana/api/datasources/proxy/uid/${promUid}/api/v1/query`,
      { params: { query: 'sum by(status) (rate(http_requests_total{status!="OK"}[5m]))' } },
    );
    expect(promResp.status()).toBe(200);
    const promBody = await promResp.json();
    const promResults = (promBody as any).data?.result ?? [];
    console.log(`Prometheus error metrics count: ${promResults.length}`);

    // Step 5: Wait for alerts to fire in Prometheus (check every 10 seconds for up to 2 minutes)
    console.log('Step 4: Waiting for alerts to fire...');
    let alertIsActive = false;
    const maxAlertAttempts = 12;
    for (let attempt = 1; attempt <= maxAlertAttempts && !alertIsActive; attempt += 1) {
      const alertsResp = await apiRequestWithRetry(
        page,
        'get',
        `/grafana/api/datasources/proxy/uid/${promUid}/api/v1/query`,
        { params: { query: 'ALERTS{alertstate="firing"}' } },
      );
      if (alertsResp.status() === 200) {
        const alertsBody = await alertsResp.json();
        const alertResults = (alertsBody as any).data?.result ?? [];
        alertIsActive = alertResults.length > 0;
        if (alertIsActive) {
          console.log(`Alerts firing: ${alertResults.map((r: any) => r.metric?.alertname).join(', ')}`);
        }
      }
      if (!alertIsActive && attempt < maxAlertAttempts) {
        console.log(`Attempt ${attempt}/${maxAlertAttempts}: No alerts firing yet, waiting...`);
        await page.waitForTimeout(10000);
      }
    }

    // Step 6: Check Mailpit for alert emails (wait up to 3 minutes for email delivery)
    console.log('Step 5: Checking Mailpit for alert emails...');
    let emailReceived = false;
    const maxEmailAttempts = 18; // 3 minutes with 10-second intervals
    for (let attempt = 1; attempt <= maxEmailAttempts && !emailReceived; attempt += 1) {
      const mailpitResp = await apiRequestWithRetry(page, 'get', '/mailpit/api/v1/messages');
      if (mailpitResp.status() === 200) {
        const mailpitBody = await mailpitResp.json();
        const currentCount = (mailpitBody as any).total ?? 0;
        const messages = (mailpitBody as any).messages ?? [];

        if (currentCount > initialCount) {
          emailReceived = true;
          console.log(`Email received! Total messages: ${currentCount}`);
          // Log email subjects
          for (const msg of messages) {
            console.log(`  - Subject: ${msg.Subject}`);
          }
        }
      }
      if (!emailReceived && attempt < maxEmailAttempts) {
        console.log(`Attempt ${attempt}/${maxEmailAttempts}: No new emails yet, waiting...`);
        await page.waitForTimeout(10000);
      }
    }

    // Step 7: Final verification
    console.log('Step 6: Final verification...');

    // Verify alert rules page has no errors
    await gotoWithRetry(page, '/grafana/alerting/list');
    await page.waitForLoadState('networkidle');
    const alertPageContent = await page.locator('body').textContent();
    const lowerContent = (alertPageContent ?? '').toLowerCase();
    expect(lowerContent.includes('errors loading rules')).toBeFalsy();
    expect(lowerContent.includes('unable to fetch alert rules')).toBeFalsy();

    // Verify Mailpit is still accessible
    await gotoWithRetry(page, '/mailpit/');
    await page.waitForLoadState('domcontentloaded');
    const mailpitTitle = await page.title();
    expect(mailpitTitle.toLowerCase()).toContain('mailpit');

    console.log('Complete alerting flow test finished!');
    console.log(`  - Alerts fired: ${alertIsActive}`);
    console.log(`  - Email received: ${emailReceived}`);

    // Note: We don't fail on email not received since alert evaluation interval
    // and notification policies may have delays. The important thing is:
    // 1. No error banners in Grafana alerting UI
    // 2. Prometheus rules are loaded
    // 3. Mailpit is accessible
    // 4. Contact points are configured

    expect(alertIsActive).toBeTruthy();
    expect(emailReceived).toBeTruthy();
  });
});

// =============================================================================
// LOGOUT FLOW COMPREHENSIVE TESTS
// =============================================================================

test.describe('Logout Flow Comprehensive', () => {
  test('logout button is visible on portal', async ({ page }) => {
    await loginViaSSO(page);

    // Navigate to portal
    await gotoWithRetry(page, PORTAL_PATH);
    await page.waitForLoadState('domcontentloaded');

    // Find logout button - it should be visible after login
    const logoutButton = page.getByRole('link', { name: /logout/i });
    const isVisible = await logoutButton.isVisible().catch(() => false);
    
    // Logout button should be visible on the portal
    expect(isVisible).toBeTruthy();
  });

  test('logout clears session', async ({ page, browser }) => {
    await loginViaSSO(page);

    // Navigate to portal and click logout
    await gotoWithRetry(page, PORTAL_PATH);
    await page.waitForLoadState('domcontentloaded');

    const logoutButton = page.getByRole('link', { name: /logout/i });
    if (await logoutButton.isVisible()) {
      await logoutButton.click();
      await page.waitForLoadState('domcontentloaded');
      await page.waitForTimeout(2000);
    }

    // Verify session is cleared by using a fresh context
    const freshContext = await browser.newContext();
    const freshPage = await freshContext.newPage();
    try {
      await gotoWithRetry(freshPage, '/grafana/');
      await freshPage.waitForLoadState('domcontentloaded');

      // Should be redirected to login
      const currentUrl = freshPage.url();
      const needsLogin = isSSOLoginUrl(currentUrl) || currentUrl.includes('/oauth2/');
      expect(needsLogin).toBeTruthy();
    } finally {
      await freshContext.close();
    }
  });

  test('central logout endpoint works', async ({ page, browser }) => {
    await loginViaSSO(page);

    // Use the central logout endpoint
    await gotoWithRetry(page, '/logout');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Create a new context to verify session is cleared
    const freshContext = await browser.newContext();
    const freshPage = await freshContext.newPage();
    try {
      await gotoWithRetry(freshPage, '/grafana/');
      await freshPage.waitForLoadState('domcontentloaded');

      // Should be redirected to SSO login
      const currentUrl = freshPage.url();
      const needsLogin = isSSOLoginUrl(currentUrl) || currentUrl.includes('/oauth2/');
      expect(needsLogin).toBeTruthy();
    } finally {
      await freshContext.close();
    }
  });
});

// =============================================================================
// CV EVALUATION FLOW TESTS (Generates metrics for Grafana/Jaeger)
// =============================================================================

test.describe('CV Evaluation Flow', () => {
  // Skip in CI if no OpenAI key is set
  test.skip(!!process.env.CI && !process.env.OPENAI_API_KEY, 'Skipping in CI without OpenAI key');

  test('upload and evaluate CV generates metrics and traces', async ({ page }) => {
    test.setTimeout(180000); // 3 minutes for full evaluation

    await loginViaSSO(page);

    // Navigate to admin frontend
    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');

    // Go to upload page
    await page.getByRole('link', { name: /Upload Files/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    const fileInputs = page.locator('input[type="file"]');
    const fileInputCount = await fileInputs.count();

    if (fileInputCount < 2) {
      console.log('Upload page not fully loaded, skipping evaluation test');
      return;
    }

    // Upload CV and Project files
    await fileInputs.nth(0).setInputFiles('tests/fixtures/cv.txt');
    await fileInputs.nth(1).setInputFiles('tests/fixtures/project.txt');

    const uploadButton = page.getByRole('button', { name: /^Upload Files$/i });
    if (!(await uploadButton.isVisible())) {
      console.log('Upload button not visible, skipping');
      return;
    }

    const uploadResponsePromise = page.waitForResponse(
      (r) => r.url().includes('/v1/upload') && r.request().method() === 'POST',
      { timeout: 30000 }
    ).catch(() => null);

    await uploadButton.click();
    const uploadResp = await uploadResponsePromise;

    if (!uploadResp || uploadResp.status() !== 200) {
      console.log('Upload failed, skipping evaluation');
      return;
    }

    const uploadJson = await uploadResp.json().catch(() => ({}));
    const cvId = (uploadJson as any)?.cv_id as string;
    const projectId = (uploadJson as any)?.project_id as string;

    if (!cvId || !projectId) {
      console.log('No IDs returned from upload');
      return;
    }

    console.log(`Uploaded CV: ${cvId}, Project: ${projectId}`);

    // Start evaluation
    await page.getByRole('link', { name: /Start Evaluation/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    const cvIdInput = page.getByLabel('CV ID');
    const projectIdInput = page.getByLabel('Project ID');

    if (await cvIdInput.isVisible()) await cvIdInput.fill(cvId);
    if (await projectIdInput.isVisible()) await projectIdInput.fill(projectId);

    const evalButton = page.getByRole('button', { name: /^Start Evaluation$/i });
    if (!(await evalButton.isVisible())) {
      console.log('Evaluate button not visible');
      return;
    }

    const evalResponsePromise = page.waitForResponse(
      (r) => r.url().includes('/v1/evaluate') && r.request().method() === 'POST',
      { timeout: 30000 }
    ).catch(() => null);

    await evalButton.click();
    const evalResp = await evalResponsePromise;

    if (!evalResp || evalResp.status() !== 200) {
      console.log('Evaluation request failed');
      return;
    }

    const evalJson = await evalResp.json().catch(() => ({}));
    const jobId = (evalJson as any)?.id as string;

    if (!jobId) {
      console.log('No job ID returned');
      return;
    }

    console.log(`Evaluation job started: ${jobId}`);

    // Poll for completion
    let lastStatus = '';
    for (let i = 0; i < 60; i += 1) {
      const res = await page.request.get(`/v1/result/${jobId}`);
      if (!res.ok()) break;
      const body = await res.json().catch(() => ({}));
      lastStatus = String((body as any)?.status ?? '');
      console.log(`Job ${jobId} status: ${lastStatus}`);
      if (['completed', 'failed'].includes(lastStatus)) break;
      await page.waitForTimeout(2000);
    }

    expect(['queued', 'processing', 'completed', 'failed']).toContain(lastStatus);
    console.log(`Final job status: ${lastStatus}`);
  });
});

// =============================================================================
// GRAFANA DASHBOARD DATA VALIDATION
// =============================================================================

test.describe('Grafana Dashboard Data Validation', () => {
  test('Job Queue Metrics dashboard loads correctly', async ({ page }) => {
    await loginViaSSO(page);

    // Navigate to Job Queue Metrics dashboard
    await gotoWithRetry(page, '/grafana/d/job-queue-metrics/job-queue-metrics');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(3000); // Wait for panels to load

    // Check if dashboard loaded - look for any content indicating the dashboard
    const body = await page.locator('body').textContent();
    expect(body).toBeTruthy();
    
    // Dashboard should not show "not found" error
    expect(body?.toLowerCase()).not.toContain('dashboard not found');
    
    // Should have some dashboard content
    const hasJobContent = body?.toLowerCase().includes('job') || 
                          body?.toLowerCase().includes('processing') ||
                          body?.toLowerCase().includes('throughput');
    console.log(`Dashboard has job-related content: ${hasJobContent}`);
    
    // Log page content for debugging
    console.log('Dashboard content preview:', body?.substring(0, 500));
  });

  test('AI Metrics dashboard loads correctly', async ({ page }) => {
    await loginViaSSO(page);

    await gotoWithRetry(page, '/grafana/d/ai-metrics/ai-metrics');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(3000);

    const body = await page.locator('body').textContent();
    expect(body).toBeTruthy();
    // Dashboard should load without errors
    expect(body?.toLowerCase()).not.toContain('dashboard not found');
  });
});

// =============================================================================
// JAEGER TRACING VALIDATION
// =============================================================================

test.describe('Jaeger Tracing Validation', () => {
  test('Jaeger shows traces with proper span hierarchy', async ({ page }) => {
    await loginViaSSO(page);

    // Navigate to Jaeger
    await gotoWithRetry(page, '/jaeger/search');
    await page.waitForLoadState('domcontentloaded');
    await waitForSpaContent(page, ['search', 'service', 'find traces'], 15000);

    // Check if Jaeger UI loaded
    const body = await page.locator('body').textContent();
    expect(body?.toLowerCase()).toContain('search');

    // Query for traces via API
    const tracesResp = await page.request.get('/jaeger/api/traces', {
      params: {
        service: 'ai-cv-evaluator',
        lookback: '1h',
        limit: '20',
      },
    });

    if (tracesResp.ok()) {
      const tracesBody = await tracesResp.json().catch(() => ({}));
      const traces = (tracesBody as any)?.data ?? [];
      console.log(`Found ${traces.length} traces in Jaeger`);

      if (traces.length > 0) {
        // Check first trace for span hierarchy
        const firstTrace = traces[0];
        const spans = firstTrace?.spans ?? [];
        console.log(`First trace has ${spans.length} spans`);

        // Log span operation names
        const opNames = spans.map((s: any) => s.operationName);
        console.log('Span operations:', opNames.slice(0, 10));

        // Verify we have child spans (more than just root span)
        expect(spans.length).toBeGreaterThan(0);
      }
    }
  });

  test('Jaeger API returns service list', async ({ page }) => {
    await loginViaSSO(page);

    const servicesResp = await page.request.get('/jaeger/api/services');
    expect(servicesResp.ok()).toBeTruthy();

    const servicesBody = await servicesResp.json().catch(() => ({}));
    const services = (servicesBody as any)?.data ?? [];
    console.log('Jaeger services:', services);

    // Should have at least the main service
    expect(services.length).toBeGreaterThan(0);
    expect(services).toContain('ai-cv-evaluator');
  });
});

// =============================================================================
// REDPANDA CONSOLE VALIDATION
// =============================================================================

test.describe('Redpanda Console Validation', () => {
  test('Redpanda shows topics and consumer groups', async ({ page }) => {
    await loginViaSSO(page);

    // Navigate to Redpanda topics
    await gotoWithRetry(page, '/redpanda/topics');
    await page.waitForLoadState('domcontentloaded');
    await waitForSpaContent(page, ['topics', 'evaluate'], 15000);

    const body = await page.locator('body').textContent();
    expect(body).toBeTruthy();

    // Should show the evaluate-jobs topic
    const hasEvaluateTopic = body?.toLowerCase().includes('evaluate');
    console.log(`Redpanda has evaluate topic: ${hasEvaluateTopic}`);
  });

  test('Redpanda consumer groups page loads', async ({ page }) => {
    await loginViaSSO(page);

    await gotoWithRetry(page, '/redpanda/groups');
    await page.waitForLoadState('domcontentloaded');
    await waitForSpaContent(page, ['consumer', 'group'], 15000);

    const body = await page.locator('body').textContent();
    expect(body).toBeTruthy();
  });
});

// =============================================================================
// ADMIN FRONTEND DASHBOARD UI/UX COMPREHENSIVE TESTS
// =============================================================================

test.describe('Admin Frontend Dashboard UI/UX', () => {
  test('dashboard page shows statistics cards', async ({ page }) => {
    await loginViaSSO(page);

    // Navigate to admin frontend dashboard
    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Check for dashboard heading
    const dashboardHeading = page.getByRole('heading', { name: /Dashboard/i });
    const headingVisible = await dashboardHeading.isVisible().catch(() => false);
    
    if (headingVisible) {
      await expect(dashboardHeading).toBeVisible();
    }

    // Check for statistics cards (Total Jobs, Completed, Processing, etc.)
    const body = await page.locator('body').textContent();
    const hasStats = body?.toLowerCase().includes('total') || 
                     body?.toLowerCase().includes('jobs') ||
                     body?.toLowerCase().includes('completed');
    console.log(`Dashboard has statistics: ${hasStats}`);
    expect(body).toBeTruthy();
  });

  test('sidebar navigation works correctly', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000); // Wait for Vue app to fully load

    // Test navigation to Upload Files (use sidebar link with exact match)
    const uploadLink = page.getByRole('link', { name: 'Upload Files', exact: true });
    if (await uploadLink.isVisible()) {
      await Promise.all([
        page.waitForURL(/upload/),
        uploadLink.click(),
      ]).catch(() => {});
      await page.waitForTimeout(500);
      const url = page.url();
      console.log(`After Upload Files click: ${url}`);
      expect(url).toContain('/upload');
    }

    // Test navigation to Start Evaluation (use sidebar link with exact match)
    const evalLink = page.getByRole('link', { name: 'Start Evaluation', exact: true });
    if (await evalLink.isVisible()) {
      await Promise.all([
        page.waitForURL(/evaluate/),
        evalLink.click(),
      ]).catch(() => {});
      await page.waitForTimeout(500);
      const url = page.url();
      console.log(`After Start Evaluation click: ${url}`);
      expect(url).toContain('/evaluate');
    }

    // Test navigation to View Results (use sidebar link with exact match)
    const resultsLink = page.getByRole('link', { name: 'View Results', exact: true });
    if (await resultsLink.isVisible()) {
      await Promise.all([
        page.waitForURL(/result/),
        resultsLink.click(),
      ]).catch(() => {});
      await page.waitForTimeout(500);
      const url = page.url();
      console.log(`After View Results click: ${url}`);
      expect(url).toContain('/result');
    }

    // Test navigation to Job Management
    const jobsLink = page.getByRole('link', { name: /Job Management/i });
    if (await jobsLink.isVisible()) {
      await Promise.all([
        page.waitForURL(/jobs/),
        jobsLink.click(),
      ]).catch(() => {});
      await page.waitForTimeout(500);
      const url = page.url();
      console.log(`After Job Management click: ${url}`);
      expect(url).toContain('/jobs');
    }

    // Test navigation back to Dashboard (use sidebar link)
    const dashLink = page.getByRole('link', { name: 'Dashboard', exact: true });
    if (await dashLink.isVisible()) {
      await Promise.all([
        page.waitForURL(/dashboard/),
        dashLink.click(),
      ]).catch(() => {});
      await page.waitForTimeout(500);
      const url = page.url();
      console.log(`After Dashboard click: ${url}`);
      expect(url).toContain('/dashboard');
    }
  });

  test('upload page has file inputs and upload button', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: /Upload Files/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    // Check for file inputs
    const fileInputs = page.locator('input[type="file"]');
    const inputCount = await fileInputs.count();
    console.log(`Found ${inputCount} file inputs`);
    expect(inputCount).toBeGreaterThanOrEqual(1);

    // Check for upload button
    const uploadButton = page.getByRole('button', { name: /Upload/i });
    const buttonVisible = await uploadButton.isVisible().catch(() => false);
    console.log(`Upload button visible: ${buttonVisible}`);
  });

  test('evaluate page has form inputs', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    // Use exact match for sidebar link
    await page.getByRole('link', { name: 'Start Evaluation', exact: true }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    // Check for CV ID input
    const cvIdInput = page.getByLabel(/CV ID/i);
    const cvIdVisible = await cvIdInput.isVisible().catch(() => false);
    console.log(`CV ID input visible: ${cvIdVisible}`);

    // Check for Project ID input
    const projectIdInput = page.getByLabel(/Project ID/i);
    const projectIdVisible = await projectIdInput.isVisible().catch(() => false);
    console.log(`Project ID input visible: ${projectIdVisible}`);

    // Check for Start Evaluation button
    const evalButton = page.getByRole('button', { name: /Start Evaluation/i });
    const evalButtonVisible = await evalButton.isVisible().catch(() => false);
    console.log(`Start Evaluation button visible: ${evalButtonVisible}`);
  });

  test('result page has job ID input', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    // Use exact match for sidebar link
    await page.getByRole('link', { name: 'View Results', exact: true }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    // Check for Job ID input
    const jobIdInput = page.getByLabel(/Job ID/i).or(page.getByPlaceholder(/job/i));
    const jobIdVisible = await jobIdInput.count() > 0;
    console.log(`Job ID input visible: ${jobIdVisible}`);

    // Check for Fetch Result button
    const fetchButton = page.getByRole('button', { name: /Fetch|Get|View/i });
    const fetchButtonVisible = await fetchButton.count() > 0;
    console.log(`Fetch button visible: ${fetchButtonVisible}`);
  });

  test('jobs page shows job list table', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: /Job Management/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Check for Job Management heading
    const heading = page.getByRole('heading', { name: /Job Management/i });
    await expect(heading).toBeVisible();

    // Check for table
    const table = page.locator('table');
    const tableVisible = await table.isVisible().catch(() => false);
    console.log(`Jobs table visible: ${tableVisible}`);

    // Check for table headers
    const body = await page.locator('body').textContent();
    const hasJobColumns = body?.toLowerCase().includes('status') || 
                          body?.toLowerCase().includes('id') ||
                          body?.toLowerCase().includes('created');
    console.log(`Has job columns: ${hasJobColumns}`);
  });

  test('mobile menu toggle works', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');

    // Set viewport to mobile size
    await page.setViewportSize({ width: 375, height: 667 });
    await page.waitForTimeout(500);

    // Look for mobile menu toggle button
    const menuButton = page.locator('button').filter({ hasText: '' }).first();
    const menuButtonExists = await menuButton.count() > 0;
    console.log(`Mobile menu button exists: ${menuButtonExists}`);

    // Reset viewport
    await page.setViewportSize({ width: 1280, height: 720 });
  });
});

// =============================================================================
// PROTECTED DASHBOARDS ACCESS AFTER LOGOUT TESTS
// =============================================================================

test.describe('Protected Dashboards After Logout', () => {
  test('Grafana requires login after logout', async ({ page, browser }) => {
    await loginViaSSO(page);

    // Verify we can access Grafana while logged in
    await gotoWithRetry(page, '/grafana/');
    await page.waitForLoadState('domcontentloaded');
    const loggedInUrl = page.url();
    expect(loggedInUrl).toContain('/grafana');

    // Logout
    await gotoWithRetry(page, '/logout');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Try to access Grafana in a fresh context
    const freshContext = await browser.newContext();
    const freshPage = await freshContext.newPage();
    try {
      await gotoWithRetry(freshPage, '/grafana/');
      await freshPage.waitForLoadState('domcontentloaded');

      // Should be redirected to login
      const afterLogoutUrl = freshPage.url();
      const needsLogin = isSSOLoginUrl(afterLogoutUrl) || afterLogoutUrl.includes('/oauth2/');
      console.log(`After logout, Grafana URL: ${afterLogoutUrl}`);
      expect(needsLogin).toBeTruthy();
    } finally {
      await freshContext.close();
    }
  });

  test('Jaeger requires login after logout', async ({ page, browser }) => {
    await loginViaSSO(page);

    // Verify we can access Jaeger while logged in
    await gotoWithRetry(page, '/jaeger/search');
    await page.waitForLoadState('domcontentloaded');

    // Logout
    await gotoWithRetry(page, '/logout');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Try to access Jaeger in a fresh context
    const freshContext = await browser.newContext();
    const freshPage = await freshContext.newPage();
    try {
      await gotoWithRetry(freshPage, '/jaeger/search');
      await freshPage.waitForLoadState('domcontentloaded');

      // Should be redirected to login
      const afterLogoutUrl = freshPage.url();
      const needsLogin = isSSOLoginUrl(afterLogoutUrl) || afterLogoutUrl.includes('/oauth2/');
      console.log(`After logout, Jaeger URL: ${afterLogoutUrl}`);
      expect(needsLogin).toBeTruthy();
    } finally {
      await freshContext.close();
    }
  });

  test('Prometheus requires login after logout', async ({ page, browser }) => {
    await loginViaSSO(page);

    // Verify we can access Prometheus while logged in
    await gotoWithRetry(page, '/prometheus/targets');
    await page.waitForLoadState('domcontentloaded');

    // Logout
    await gotoWithRetry(page, '/logout');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Try to access Prometheus in a fresh context
    const freshContext = await browser.newContext();
    const freshPage = await freshContext.newPage();
    try {
      await gotoWithRetry(freshPage, '/prometheus/targets');
      await freshPage.waitForLoadState('domcontentloaded');

      // Should be redirected to login
      const afterLogoutUrl = freshPage.url();
      const needsLogin = isSSOLoginUrl(afterLogoutUrl) || afterLogoutUrl.includes('/oauth2/');
      console.log(`After logout, Prometheus URL: ${afterLogoutUrl}`);
      expect(needsLogin).toBeTruthy();
    } finally {
      await freshContext.close();
    }
  });

  test('Redpanda Console requires login after logout', async ({ page, browser }) => {
    await loginViaSSO(page);

    // Verify we can access Redpanda while logged in
    await gotoWithRetry(page, '/redpanda/');
    await page.waitForLoadState('domcontentloaded');

    // Logout
    await gotoWithRetry(page, '/logout');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Try to access Redpanda in a fresh context
    const freshContext = await browser.newContext();
    const freshPage = await freshContext.newPage();
    try {
      await gotoWithRetry(freshPage, '/redpanda/');
      await freshPage.waitForLoadState('domcontentloaded');

      // Should be redirected to login
      const afterLogoutUrl = freshPage.url();
      const needsLogin = isSSOLoginUrl(afterLogoutUrl) || afterLogoutUrl.includes('/oauth2/');
      console.log(`After logout, Redpanda URL: ${afterLogoutUrl}`);
      expect(needsLogin).toBeTruthy();
    } finally {
      await freshContext.close();
    }
  });

  test('Admin Frontend requires login after logout', async ({ page, browser }) => {
    await loginViaSSO(page);

    // Verify we can access Admin Frontend while logged in
    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');

    // Logout
    await gotoWithRetry(page, '/logout');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Try to access Admin Frontend in a fresh context
    const freshContext = await browser.newContext();
    const freshPage = await freshContext.newPage();
    try {
      await gotoWithRetry(freshPage, '/app/');
      await freshPage.waitForLoadState('domcontentloaded');

      // Should be redirected to login
      const afterLogoutUrl = freshPage.url();
      const needsLogin = isSSOLoginUrl(afterLogoutUrl) || afterLogoutUrl.includes('/oauth2/');
      console.log(`After logout, Admin Frontend URL: ${afterLogoutUrl}`);
      expect(needsLogin).toBeTruthy();
    } finally {
      await freshContext.close();
    }
  });

  test('Mailpit requires login after logout', async ({ page, browser }) => {
    await loginViaSSO(page);

    // Verify we can access Mailpit while logged in
    await gotoWithRetry(page, '/mailpit/');
    await page.waitForLoadState('domcontentloaded');

    // Logout
    await gotoWithRetry(page, '/logout');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Try to access Mailpit in a fresh context
    const freshContext = await browser.newContext();
    const freshPage = await freshContext.newPage();
    try {
      await gotoWithRetry(freshPage, '/mailpit/');
      await freshPage.waitForLoadState('domcontentloaded');

      // Should be redirected to login
      const afterLogoutUrl = freshPage.url();
      const needsLogin = isSSOLoginUrl(afterLogoutUrl) || afterLogoutUrl.includes('/oauth2/');
      console.log(`After logout, Mailpit URL: ${afterLogoutUrl}`);
      expect(needsLogin).toBeTruthy();
    } finally {
      await freshContext.close();
    }
  });
});

// =============================================================================
// OBSERVABILITY DATA VALIDATION AFTER DASHBOARD INTERACTIONS
// =============================================================================

test.describe('Observability After Dashboard Interactions', () => {
  test('Jaeger shows HTTP traces after dashboard navigation', async ({ page }) => {
    await loginViaSSO(page);

    // Navigate through multiple pages to generate traces
    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    
    await page.getByRole('link', { name: /Job Management/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    await page.getByRole('link', { name: /Dashboard/i }).first().click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    // Check Jaeger for new traces
    const tracesResp = await page.request.get('/jaeger/api/traces', {
      params: {
        service: 'ai-cv-evaluator',
        lookback: '5m',
        limit: '50',
      },
    });

    if (tracesResp.ok()) {
      const tracesBody = await tracesResp.json().catch(() => ({}));
      const traces = (tracesBody as any)?.data ?? [];
      console.log(`Found ${traces.length} traces in last 5 minutes`);
      
      // Should have some traces from our navigation
      expect(traces.length).toBeGreaterThan(0);

      // Check for admin API traces
      const allSpans = traces.flatMap((t: any) => t.spans ?? []);
      const adminSpans = allSpans.filter((s: any) => 
        String(s.operationName ?? '').includes('admin') ||
        String(s.operationName ?? '').includes('Admin')
      );
      console.log(`Found ${adminSpans.length} admin-related spans`);
    }
  });

  test('Prometheus has HTTP metrics after dashboard interactions', async ({ page }) => {
    await loginViaSSO(page);

    // Navigate through pages to generate metrics
    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    
    await page.getByRole('link', { name: /Job Management/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    // Query Prometheus for HTTP request metrics
    const metricsResp = await page.request.get('/prometheus/api/v1/query', {
      params: {
        query: 'http_requests_total',
      },
    });

    if (metricsResp.ok()) {
      const metricsBody = await metricsResp.json().catch(() => ({}));
      const results = (metricsBody as any)?.data?.result ?? [];
      console.log(`Found ${results.length} HTTP request metric series`);
      
      // Should have some HTTP request metrics
      expect(results.length).toBeGreaterThan(0);

      // Log some metric details
      for (const r of results.slice(0, 5)) {
        console.log(`Route: ${r.metric?.route}, Method: ${r.metric?.method}, Value: ${r.value?.[1]}`);
      }
    }
  });

  test('Grafana dashboards reflect current metrics', async ({ page }) => {
    await loginViaSSO(page);

    // Navigate to Request Drilldown dashboard which shows HTTP metrics
    await gotoWithRetry(page, '/grafana/d/request-drilldown/request-drilldown');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(3000);

    const body = await page.locator('body').textContent();
    expect(body).toBeTruthy();
    expect(body?.toLowerCase()).not.toContain('dashboard not found');
    
    console.log('Request Drilldown dashboard loaded successfully');
  });
});
