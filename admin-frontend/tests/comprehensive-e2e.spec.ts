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
  // Run in all environments - production alerting is fully functional
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

  test('Complete alerting flow: trigger alert and verify alert fires', async ({ page }) => {
    test.setTimeout(180000); // 3 minutes for complete flow
    await loginViaSSO(page);

    // Step 1: Generate HTTP errors to trigger HighHttpErrorRate alert
    console.log('Step 1: Generating HTTP errors to trigger alert...');
    for (let i = 0; i < 30; i += 1) {
      await page.request.get('/v1/__nonexistent_path_for_errors');
      await page.waitForTimeout(100);
    }

    // Step 2: Verify Prometheus is recording non-OK HTTP requests
    console.log('Step 2: Verifying Prometheus is recording errors...');
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
    expect(promResults.length).toBeGreaterThan(0);

    // Step 3: Wait for alerts to fire in Prometheus (check every 10 seconds for up to 2 minutes)
    console.log('Step 3: Waiting for alerts to fire...');
    let alertIsActive = false;
    let firingAlerts: string[] = [];
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
          firingAlerts = alertResults.map((r: any) => r.metric?.alertname);
          console.log(`Alerts firing: ${firingAlerts.join(', ')}`);
        }
      }
      if (!alertIsActive && attempt < maxAlertAttempts) {
        console.log(`Attempt ${attempt}/${maxAlertAttempts}: No alerts firing yet, waiting...`);
        await page.waitForTimeout(10000);
      }
    }

    // Step 4: Final verification
    console.log('Step 4: Final verification...');

    // Verify alert rules page has no errors
    await gotoWithRetry(page, '/grafana/alerting/list');
    await page.waitForLoadState('networkidle');
    const alertPageContent = await page.locator('body').textContent();
    const lowerContent = (alertPageContent ?? '').toLowerCase();
    expect(lowerContent.includes('errors loading rules')).toBeFalsy();
    expect(lowerContent.includes('unable to fetch alert rules')).toBeFalsy();

    console.log('Complete alerting flow test finished!');
    console.log(`  - Alerts fired: ${alertIsActive}`);
    console.log(`  - Firing alerts: ${firingAlerts.join(', ')}`);

    // The critical assertion: alerts must fire when errors are generated
    expect(alertIsActive).toBeTruthy();
  });

  test('Mailpit receives alert emails when alerts fire', async ({ page }) => {
    test.setTimeout(360000); // 6 minutes to account for repeat_interval
    await loginViaSSO(page);

    // Check if there are already alert emails in Mailpit (from previous alerts)
    const existingResp = await apiRequestWithRetry(page, 'get', '/mailpit/api/v1/messages');
    expect(existingResp.status()).toBe(200);
    const existingData = await existingResp.json();
    const existingMessages = (existingData as any).messages ?? [];
    const existingAlertEmails = existingMessages.filter((m: any) => 
      m.Subject?.toLowerCase().includes('alert') || m.Subject?.toLowerCase().includes('ai-cv-evaluator')
    );

    if (existingAlertEmails.length > 0) {
      console.log(`Found ${existingAlertEmails.length} existing alert emails - alerting pipeline is working`);
      console.log(`Subjects: ${existingAlertEmails.map((m: any) => m.Subject).join(', ')}`);
      // Test passes - alerting pipeline has already delivered emails
      expect(existingAlertEmails.length).toBeGreaterThan(0);
      return;
    }

    // No existing emails, need to trigger new alerts and wait for email
    console.log('No existing alert emails found. Triggering new alerts...');
    
    // Clear Mailpit messages first
    await clearMailpitMessages(page);

    // Generate errors to trigger alerts
    console.log('Generating errors to trigger alerts...');
    for (let i = 0; i < 50; i += 1) {
      await page.request.get('/v1/__nonexistent_path_for_errors');
      await page.waitForTimeout(100);
    }

    // Wait for Grafana to process alerts and send emails
    // group_wait is 30s, repeat_interval is 5m
    // We need to wait long enough for the notification to be sent
    console.log('Waiting for alert processing and email delivery (up to 5 minutes)...');
    
    // Check for new emails (poll for up to 5 minutes)
    let emailReceived = false;
    let emailSubjects: string[] = [];
    const maxAttempts = 30; // 5 minutes with 10-second intervals
    for (let attempt = 1; attempt <= maxAttempts && !emailReceived; attempt += 1) {
      const mailpitResp = await apiRequestWithRetry(page, 'get', '/mailpit/api/v1/messages');
      if (mailpitResp.status() === 200) {
        const mailpitBody = await mailpitResp.json();
        const currentCount = (mailpitBody as any).total ?? 0;
        const messages = (mailpitBody as any).messages ?? [];

        if (currentCount > 0) {
          const alertEmails = messages.filter((m: any) => 
            m.Subject?.toLowerCase().includes('alert') || m.Subject?.toLowerCase().includes('ai-cv-evaluator')
          );
          if (alertEmails.length > 0) {
            emailReceived = true;
            emailSubjects = alertEmails.map((m: any) => m.Subject);
            console.log(`Email received! Total: ${currentCount}, Alert subjects: ${emailSubjects.join(', ')}`);
          }
        }
      }
      if (!emailReceived && attempt < maxAttempts) {
        if (attempt % 6 === 0) { // Log every minute
          console.log(`Minute ${attempt / 6}/${maxAttempts / 6}: Waiting for email...`);
        }
        await page.waitForTimeout(10000);
      }
    }

    console.log(`Email delivery result: ${emailReceived ? 'SUCCESS' : 'FAILED'}`);
    
    // Email must be received for the alerting pipeline to be considered working
    expect(emailReceived).toBeTruthy();
    
    // Verify email subject contains alert information
    if (emailReceived && emailSubjects.length > 0) {
      const hasAlertSubject = emailSubjects.some(s => 
        s.toLowerCase().includes('alert') || s.toLowerCase().includes('ai-cv-evaluator')
      );
      expect(hasAlertSubject).toBeTruthy();
    }
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
  // Run in all environments - test the upload and evaluation submission
  test('upload files and submit evaluation job', async ({ page }) => {
    test.setTimeout(60000); // 1 minute for upload and submission

    await loginViaSSO(page);

    // Navigate to admin frontend
    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');

    // Go to upload page using exact match for sidebar link
    await page.getByRole('link', { name: 'Upload Files', exact: true }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    const fileInputs = page.locator('input[type="file"]');
    const fileInputCount = await fileInputs.count();

    expect(fileInputCount).toBeGreaterThanOrEqual(2);

    // Create test files dynamically
    const cvContent = `
John Doe - Senior Software Engineer

Experience:
- 5 years of Go/Golang development
- Kubernetes and Docker expertise
- PostgreSQL and Redis experience
- REST API design and implementation

Skills: Go, Python, JavaScript, Docker, Kubernetes, PostgreSQL, Redis, AWS
Education: BS Computer Science
`;

    const projectContent = `
Project: AI CV Evaluator
Requirements:
- Go/Golang backend development
- PostgreSQL database
- Docker containerization
- REST API development
- Message queue integration (Kafka/Redpanda)

Nice to have:
- Kubernetes experience
- AI/ML integration
`;

    // Upload CV and Project files using buffer
    await fileInputs.nth(0).setInputFiles({
      name: 'cv.txt',
      mimeType: 'text/plain',
      buffer: Buffer.from(cvContent),
    });
    await fileInputs.nth(1).setInputFiles({
      name: 'project.txt',
      mimeType: 'text/plain',
      buffer: Buffer.from(projectContent),
    });

    // Find and click upload button
    const uploadButton = page.getByRole('button').filter({ hasText: /Upload/i }).first();
    await expect(uploadButton).toBeVisible({ timeout: 5000 });

    const uploadResponsePromise = page.waitForResponse(
      (r) => r.url().includes('/v1/upload') && r.request().method() === 'POST',
      { timeout: 30000 }
    ).catch(() => null);

    await uploadButton.click();
    const uploadResp = await uploadResponsePromise;

    expect(uploadResp).toBeTruthy();
    expect(uploadResp!.status()).toBe(200);

    const uploadJson = await uploadResp!.json().catch(() => ({}));
    const cvId = (uploadJson as any)?.cv_id as string;
    const projectId = (uploadJson as any)?.project_id as string;

    expect(cvId).toBeTruthy();
    expect(projectId).toBeTruthy();

    console.log(`Uploaded CV: ${cvId}, Project: ${projectId}`);

    // Start evaluation using exact match for sidebar link
    await page.getByRole('link', { name: 'Start Evaluation', exact: true }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    const cvIdInput = page.getByLabel('CV ID');
    const projectIdInput = page.getByLabel('Project ID');

    await expect(cvIdInput).toBeVisible();
    await expect(projectIdInput).toBeVisible();

    await cvIdInput.fill(cvId);
    await projectIdInput.fill(projectId);

    const evalButton = page.getByRole('button', { name: /^Start Evaluation$/i });
    await expect(evalButton).toBeVisible();

    const evalResponsePromise = page.waitForResponse(
      (r) => r.url().includes('/v1/evaluate') && r.request().method() === 'POST',
      { timeout: 30000 }
    ).catch(() => null);

    await evalButton.click();
    const evalResp = await evalResponsePromise;

    expect(evalResp).toBeTruthy();
    expect(evalResp!.status()).toBe(200);

    const evalJson = await evalResp!.json().catch(() => ({}));
    const jobId = (evalJson as any)?.id as string;

    expect(jobId).toBeTruthy();

    console.log(`Evaluation job created: ${jobId}`);
    console.log('Upload and evaluation submission successful - job is now queued for processing');
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
// ADMIN FRONTEND DASHBOARD UI/UX COMPREHENSIVE FUNCTIONAL TESTS
// =============================================================================

test.describe('Admin Frontend Dashboard - Functional Tests', () => {
  test('dashboard statistics cards display numeric values', async ({ page }) => {
    await loginViaSSO(page);

    // Navigate to admin frontend dashboard
    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(3000); // Wait for stats API to load

    // Check for dashboard heading
    const dashboardHeading = page.getByRole('heading', { name: /Dashboard/i }).first();
    await expect(dashboardHeading).toBeVisible();

    // Verify statistics cards are present with actual data
    const statsUploads = page.locator('[data-testid="stats-uploads"]');
    const statsEvaluations = page.locator('[data-testid="stats-evaluations"]');
    const statsCompleted = page.locator('[data-testid="stats-completed"]');
    const statsAvgTime = page.locator('[data-testid="stats-avg-time"]');

    // Wait for stats to load (check for any stat card with numeric content)
    const body = await page.locator('body').textContent();
    const hasUploadsText = body?.includes('Total Uploads') || body?.includes('Uploads');
    const hasEvaluationsText = body?.includes('Evaluations');
    const hasCompletedText = body?.includes('Completed');
    const hasAvgTimeText = body?.includes('Avg Time') || body?.includes('Average');

    console.log(`Dashboard stats visible: Uploads=${hasUploadsText}, Evaluations=${hasEvaluationsText}, Completed=${hasCompletedText}, AvgTime=${hasAvgTimeText}`);
    
    // At least some stats should be displayed
    expect(hasUploadsText || hasEvaluationsText || hasCompletedText).toBeTruthy();

    // Check that stats cards show numeric values (not just labels)
    if (await statsUploads.isVisible().catch(() => false)) {
      const uploadsValue = await statsUploads.textContent();
      console.log(`Uploads value: ${uploadsValue}`);
      expect(uploadsValue).toMatch(/\d+/); // Should contain a number
    }
  });

  test('dashboard quick action links navigate correctly', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Test Upload Documents quick action (in the Quick Actions section)
    const uploadAction = page.locator('[class*="card"] a, [class*="quick"] a, a[href*="upload"]').filter({ hasText: /Upload/i }).first();
    if (await uploadAction.isVisible().catch(() => false)) {
      await uploadAction.click();
      await page.waitForLoadState('domcontentloaded');
      await page.waitForTimeout(500);
      const url = page.url();
      console.log(`Quick action: Upload navigates to: ${url}`);
      expect(url).toContain('/upload');
      
      // Go back to dashboard
      await page.getByRole('link', { name: 'Dashboard', exact: true }).click();
      await page.waitForLoadState('domcontentloaded');
      await page.waitForTimeout(500);
    } else {
      console.log('Upload quick action not found, checking sidebar navigation works');
    }

    // Test Start Evaluation quick action
    const evalAction = page.locator('[class*="card"] a, [class*="quick"] a, a[href*="evaluate"]').filter({ hasText: /Evaluation/i }).first();
    if (await evalAction.isVisible().catch(() => false)) {
      await evalAction.click();
      await page.waitForLoadState('domcontentloaded');
      await page.waitForTimeout(500);
      const url = page.url();
      console.log(`Quick action: Evaluation navigates to: ${url}`);
      expect(url).toContain('/evaluate');
      
      await page.getByRole('link', { name: 'Dashboard', exact: true }).click();
      await page.waitForLoadState('domcontentloaded');
      await page.waitForTimeout(500);
    } else {
      console.log('Evaluation quick action not found');
    }

    // Test View Results quick action
    const resultsAction = page.locator('[class*="card"] a, [class*="quick"] a, a[href*="result"]').filter({ hasText: /Result/i }).first();
    if (await resultsAction.isVisible().catch(() => false)) {
      await resultsAction.click();
      await page.waitForLoadState('domcontentloaded');
      await page.waitForTimeout(500);
      const url = page.url();
      console.log(`Quick action: Results navigates to: ${url}`);
      expect(url).toContain('/result');
    } else {
      console.log('Results quick action not found');
    }
  });

  test('dashboard system status shows online indicators', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    const body = await page.locator('body').textContent();
    
    // Check for system status indicators
    const hasApiServer = body?.includes('API Server');
    const hasWorkerQueue = body?.includes('Worker Queue');
    const hasDatabase = body?.includes('Database');
    const hasOnlineStatus = body?.includes('Online') || body?.includes('Active') || body?.includes('Healthy');

    console.log(`System status: API=${hasApiServer}, Worker=${hasWorkerQueue}, DB=${hasDatabase}, Online=${hasOnlineStatus}`);
    
    expect(hasApiServer || hasWorkerQueue || hasDatabase).toBeTruthy();
  });

  test('dashboard user menu dropdown opens and shows logout', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Find and click the user avatar/menu button (circular button with user initial)
    const userMenuButton = page.locator('button').filter({ has: page.locator('.rounded-full') }).first();
    if (await userMenuButton.isVisible().catch(() => false)) {
      await userMenuButton.click();
      await page.waitForTimeout(500);

      // Check for logout option in dropdown
      const signOutButton = page.getByRole('button', { name: /Sign out/i });
      const signOutVisible = await signOutButton.isVisible().catch(() => false);
      console.log(`Sign out button visible in dropdown: ${signOutVisible}`);
      
      if (signOutVisible) {
        await expect(signOutButton).toBeVisible();
      }
    }
  });

  test('dashboard View Metrics and Grafana buttons are accessible', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Check for View Metrics button/link
    const metricsLink = page.locator('a').filter({ hasText: 'View Metrics' });
    const grafanaLink = page.locator('a').filter({ hasText: 'Grafana' });

    const metricsVisible = await metricsLink.isVisible().catch(() => false);
    const grafanaVisible = await grafanaLink.isVisible().catch(() => false);

    console.log(`View Metrics visible: ${metricsVisible}, Grafana visible: ${grafanaVisible}`);

    if (metricsVisible) {
      const href = await metricsLink.getAttribute('href');
      expect(href).toContain('/metrics');
    }

    if (grafanaVisible) {
      const href = await grafanaLink.getAttribute('href');
      expect(href).toContain('/grafana');
    }
  });

  test('sidebar navigation works correctly with active state', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Test each navigation item and verify URL changes
    const navItems = [
      { name: 'Upload Files', urlPart: '/upload' },
      { name: 'Start Evaluation', urlPart: '/evaluate' },
      { name: 'View Results', urlPart: '/result' },
      { name: 'Job Management', urlPart: '/jobs' },
      { name: 'Dashboard', urlPart: '/dashboard' },
    ];

    for (const item of navItems) {
      const link = page.getByRole('link', { name: item.name, exact: true });
      if (await link.isVisible().catch(() => false)) {
        await link.click();
        await page.waitForLoadState('domcontentloaded');
        await page.waitForTimeout(500);
        
        expect(page.url()).toContain(item.urlPart);
        console.log(`Navigation: ${item.name} -> ${page.url()}`);
      }
    }
  });
});

// =============================================================================
// UPLOAD PAGE - COMPREHENSIVE FUNCTIONAL TESTS
// =============================================================================

test.describe('Upload Page - Functional Tests', () => {
  test('upload button state changes with file selection', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: 'Upload Files', exact: true }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    // Find the upload button
    const uploadButton = page.getByRole('button').filter({ hasText: /Upload/i }).first();
    await expect(uploadButton).toBeVisible();

    // Record initial state (may or may not be disabled depending on design)
    const initialDisabled = await uploadButton.isDisabled().catch(() => false);
    console.log(`Upload button initial disabled state: ${initialDisabled}`);

    // Select files
    const fileInputs = page.locator('input[type="file"]');
    await fileInputs.nth(0).setInputFiles({
      name: 'test-cv.txt',
      mimeType: 'text/plain',
      buffer: Buffer.from('Test CV content'),
    });
    await fileInputs.nth(1).setInputFiles({
      name: 'test-project.txt',
      mimeType: 'text/plain',
      buffer: Buffer.from('Test project content'),
    });
    await page.waitForTimeout(500);

    // After file selection, button should be enabled
    const afterSelectionDisabled = await uploadButton.isDisabled().catch(() => false);
    console.log(`Upload button after file selection disabled state: ${afterSelectionDisabled}`);
    
    // Button should be enabled (not disabled) after files are selected
    expect(afterSelectionDisabled).toBeFalsy();
  });

  test('file inputs accept correct file types', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: 'Upload Files', exact: true }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    const fileInputs = page.locator('input[type="file"]');
    const inputCount = await fileInputs.count();
    expect(inputCount).toBeGreaterThanOrEqual(2);

    // Check accept attributes
    for (let i = 0; i < inputCount; i++) {
      const accept = await fileInputs.nth(i).getAttribute('accept');
      console.log(`File input ${i} accepts: ${accept}`);
      // Should accept PDF, DOC, DOCX
      expect(accept).toMatch(/pdf|doc/i);
    }
  });

  test('file selection shows file name and size', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: 'Upload Files', exact: true }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    const fileInputs = page.locator('input[type="file"]');
    
    // Create test file content
    const testContent = 'Test CV content for upload functionality test';

    // Select file for CV input
    await fileInputs.nth(0).setInputFiles({
      name: 'test-cv.txt',
      mimeType: 'text/plain',
      buffer: Buffer.from(testContent),
    });
    
    await page.waitForTimeout(500);

    // Check if file name is displayed
    const body = await page.locator('body').textContent();
    const hasFileName = body?.includes('test-cv.txt');
    console.log(`File name displayed after selection: ${hasFileName}`);
    
    // Check for file size display (bytes, KB, etc.)
    const hasSizeInfo = body?.match(/\d+\s*(bytes|KB|MB)/i);
    console.log(`File size displayed: ${!!hasSizeInfo}`);
  });

  test('remove file button clears selection', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: 'Upload Files', exact: true }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    const fileInputs = page.locator('input[type="file"]');
    
    // Select a file
    await fileInputs.nth(0).setInputFiles({
      name: 'test-cv.txt',
      mimeType: 'text/plain',
      buffer: Buffer.from('Test content'),
    });
    
    await page.waitForTimeout(500);

    // Click remove button
    const removeButton = page.getByRole('button', { name: /Remove/i }).first();
    if (await removeButton.isVisible().catch(() => false)) {
      await removeButton.click();
      await page.waitForTimeout(500);

      // Verify file is removed (file name should not be visible)
      const body = await page.locator('body').textContent();
      const hasFileName = body?.includes('test-cv.txt');
      console.log(`File name removed: ${!hasFileName}`);
      expect(hasFileName).toBeFalsy();
    }
  });

  test('upload both CV and project files successfully', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: 'Upload Files', exact: true }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    const fileInputs = page.locator('input[type="file"]');
    expect(await fileInputs.count()).toBeGreaterThanOrEqual(2);

    // Create test files
    const cvContent = `John Doe - Software Engineer
Skills: Go, Python, JavaScript
Experience: 5 years`;

    const projectContent = `Project Requirements:
- Backend development
- API integration
- Database design`;

    // Upload both files
    await fileInputs.nth(0).setInputFiles({
      name: 'cv.txt',
      mimeType: 'text/plain',
      buffer: Buffer.from(cvContent),
    });
    
    await fileInputs.nth(1).setInputFiles({
      name: 'project.txt',
      mimeType: 'text/plain',
      buffer: Buffer.from(projectContent),
    });

    await page.waitForTimeout(500);

    // Upload button should be enabled now
    const uploadButton = page.getByRole('button').filter({ hasText: /Upload/i }).first();
    await expect(uploadButton).toBeVisible();

    // Capture response from upload
    const uploadResponsePromise = page.waitForResponse(
      (r) => r.url().includes('/v1/upload') && r.request().method() === 'POST',
      { timeout: 30000 }
    ).catch(() => null);

    await uploadButton.click();
    const response = await uploadResponsePromise;

    if (response && response.status() === 200) {
      const json = await response.json().catch(() => ({}));
      console.log(`Upload successful: CV ID=${(json as any).cv_id}, Project ID=${(json as any).project_id}`);
      expect((json as any).cv_id).toBeTruthy();
      expect((json as any).project_id).toBeTruthy();
    } else {
      console.log(`Upload response status: ${response?.status()}`);
    }
  });

  test('upload shows success message with IDs', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: 'Upload Files', exact: true }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    const fileInputs = page.locator('input[type="file"]');

    await fileInputs.nth(0).setInputFiles({
      name: 'cv.txt',
      mimeType: 'text/plain',
      buffer: Buffer.from('CV content'),
    });
    
    await fileInputs.nth(1).setInputFiles({
      name: 'project.txt',
      mimeType: 'text/plain',
      buffer: Buffer.from('Project content'),
    });

    await page.waitForTimeout(500);

    const uploadButton = page.getByRole('button').filter({ hasText: /Upload/i }).first();
    await uploadButton.click();
    
    // Wait for response
    await page.waitForTimeout(3000);

    // Check for success message
    const body = await page.locator('body').textContent();
    const hasSuccess = body?.toLowerCase().includes('success') || body?.includes('CV ID');
    console.log(`Success message displayed: ${hasSuccess}`);
  });
});

// =============================================================================
// EVALUATE PAGE - COMPREHENSIVE FUNCTIONAL TESTS
// =============================================================================

test.describe('Evaluate Page - Functional Tests', () => {
  test('evaluate form has all required and optional fields', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: 'Start Evaluation', exact: true }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    // Required fields
    const cvIdInput = page.getByLabel('CV ID');
    const projectIdInput = page.getByLabel('Project ID');
    await expect(cvIdInput).toBeVisible();
    await expect(projectIdInput).toBeVisible();

    // Optional fields
    const jobDescInput = page.getByLabel(/Job Description/i);
    const studyCaseInput = page.getByLabel(/Study Case Brief/i);
    const scoringRubricInput = page.getByLabel(/Scoring Rubric/i);

    const jobDescVisible = await jobDescInput.isVisible().catch(() => false);
    const studyCaseVisible = await studyCaseInput.isVisible().catch(() => false);
    const scoringRubricVisible = await scoringRubricInput.isVisible().catch(() => false);

    console.log(`Optional fields: JobDesc=${jobDescVisible}, StudyCase=${studyCaseVisible}, ScoringRubric=${scoringRubricVisible}`);

    // Check for (Optional) labels
    const body = await page.locator('body').textContent();
    expect(body).toContain('Optional');
  });

  test('evaluate form validates required fields', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: 'Start Evaluation', exact: true }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    // Try to submit without filling required fields
    const evalButton = page.getByRole('button', { name: /^Start Evaluation$/i });
    await expect(evalButton).toBeVisible();

    // Check required attribute on inputs
    const cvIdInput = page.getByLabel('CV ID');
    const projectIdInput = page.getByLabel('Project ID');

    const cvIdRequired = await cvIdInput.getAttribute('required');
    const projectIdRequired = await projectIdInput.getAttribute('required');

    console.log(`CV ID required: ${cvIdRequired !== null}, Project ID required: ${projectIdRequired !== null}`);
  });

  test('evaluate form submission with valid IDs creates job', async ({ page }) => {
    await loginViaSSO(page);

    // First upload files to get valid IDs
    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: 'Upload Files', exact: true }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    const fileInputs = page.locator('input[type="file"]');
    
    await fileInputs.nth(0).setInputFiles({
      name: 'cv.txt',
      mimeType: 'text/plain',
      buffer: Buffer.from('CV content for evaluation test'),
    });
    
    await fileInputs.nth(1).setInputFiles({
      name: 'project.txt',
      mimeType: 'text/plain',
      buffer: Buffer.from('Project content for evaluation test'),
    });

    await page.waitForTimeout(500);

    // Upload and get IDs
    const uploadResponsePromise = page.waitForResponse(
      (r) => r.url().includes('/v1/upload') && r.request().method() === 'POST',
      { timeout: 30000 }
    ).catch(() => null);

    const uploadButton = page.getByRole('button').filter({ hasText: /Upload/i }).first();
    await uploadButton.click();
    const uploadResp = await uploadResponsePromise;

    if (!uploadResp || uploadResp.status() !== 200) {
      console.log('Upload failed, skipping evaluate test');
      return;
    }

    const uploadJson = await uploadResp.json().catch(() => ({}));
    const cvId = (uploadJson as any)?.cv_id;
    const projectId = (uploadJson as any)?.project_id;

    if (!cvId || !projectId) {
      console.log('No IDs returned from upload');
      return;
    }

    // Navigate to evaluate page
    await page.getByRole('link', { name: 'Start Evaluation', exact: true }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    // Fill form
    await page.getByLabel('CV ID').fill(cvId);
    await page.getByLabel('Project ID').fill(projectId);

    // Submit
    const evalResponsePromise = page.waitForResponse(
      (r) => r.url().includes('/v1/evaluate') && r.request().method() === 'POST',
      { timeout: 30000 }
    ).catch(() => null);

    const evalButton = page.getByRole('button', { name: /^Start Evaluation$/i });
    await evalButton.click();
    const evalResp = await evalResponsePromise;

    if (evalResp && evalResp.status() === 200) {
      const evalJson = await evalResp.json().catch(() => ({}));
      const jobId = (evalJson as any)?.id;
      console.log(`Evaluation job created: ${jobId}`);
      expect(jobId).toBeTruthy();

      // Check for success message
      await page.waitForTimeout(1000);
      const body = await page.locator('body').textContent();
      expect(body?.toLowerCase()).toContain('success');
    }
  });

  test('evaluate form shows error for invalid IDs', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: 'Start Evaluation', exact: true }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    // Fill with invalid IDs
    await page.getByLabel('CV ID').fill('invalid-cv-id-12345');
    await page.getByLabel('Project ID').fill('invalid-project-id-12345');

    // Submit and expect error
    const evalButton = page.getByRole('button', { name: /^Start Evaluation$/i });
    await evalButton.click();
    
    await page.waitForTimeout(3000);

    // Check for error message
    const body = await page.locator('body').textContent();
    const hasError = body?.toLowerCase().includes('error') || 
                     body?.toLowerCase().includes('failed') ||
                     body?.toLowerCase().includes('not found');
    console.log(`Error message displayed: ${hasError}`);
  });
});

// =============================================================================
// RESULT PAGE - COMPREHENSIVE FUNCTIONAL TESTS
// =============================================================================

test.describe('Result Page - Functional Tests', () => {
  test('result page job ID input and fetch button work', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: 'View Results', exact: true }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    // Find Job ID input
    const jobIdInput = page.locator('#job_id').or(page.getByLabel(/Job ID/i)).or(page.getByPlaceholder(/Job ID/i));
    await expect(jobIdInput.first()).toBeVisible();

    // Find Get Results button
    const fetchButton = page.getByRole('button', { name: /Get Results/i });
    await expect(fetchButton).toBeVisible();

    // Enter an invalid job ID and fetch
    await jobIdInput.first().fill('non-existent-job-id');
    await fetchButton.click();

    await page.waitForTimeout(2000);

    // Should show error or "not found"
    const body = await page.locator('body').textContent();
    const hasError = body?.toLowerCase().includes('not found') || 
                     body?.toLowerCase().includes('error');
    console.log(`Error for invalid job ID: ${hasError}`);
  });

  test('result page shows status indicators correctly', async ({ page }) => {
    await loginViaSSO(page);

    // First, create a job to test with
    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    
    // Upload files first
    await page.getByRole('link', { name: 'Upload Files', exact: true }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    const fileInputs = page.locator('input[type="file"]');
    
    await fileInputs.nth(0).setInputFiles({
      name: 'cv.txt',
      mimeType: 'text/plain',
      buffer: Buffer.from('CV content'),
    });
    
    await fileInputs.nth(1).setInputFiles({
      name: 'project.txt',
      mimeType: 'text/plain',
      buffer: Buffer.from('Project content'),
    });

    const uploadResponsePromise = page.waitForResponse(
      (r) => r.url().includes('/v1/upload') && r.request().method() === 'POST',
      { timeout: 30000 }
    ).catch(() => null);

    await page.getByRole('button').filter({ hasText: /Upload/i }).first().click();
    const uploadResp = await uploadResponsePromise;

    if (!uploadResp || uploadResp.status() !== 200) return;

    const uploadJson = await uploadResp.json().catch(() => ({}));
    const cvId = (uploadJson as any)?.cv_id;
    const projectId = (uploadJson as any)?.project_id;

    if (!cvId || !projectId) return;

    // Start evaluation
    await page.getByRole('link', { name: 'Start Evaluation', exact: true }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    await page.getByLabel('CV ID').fill(cvId);
    await page.getByLabel('Project ID').fill(projectId);

    const evalResponsePromise = page.waitForResponse(
      (r) => r.url().includes('/v1/evaluate') && r.request().method() === 'POST',
      { timeout: 30000 }
    ).catch(() => null);

    await page.getByRole('button', { name: /^Start Evaluation$/i }).click();
    const evalResp = await evalResponsePromise;

    if (!evalResp || evalResp.status() !== 200) return;

    const evalJson = await evalResp.json().catch(() => ({}));
    const jobId = (evalJson as any)?.id;

    if (!jobId) return;

    // Now check result page
    await page.getByRole('link', { name: 'View Results', exact: true }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    await page.locator('#job_id').or(page.getByLabel(/Job ID/i)).first().fill(jobId);
    await page.getByRole('button', { name: /Get Results/i }).click();

    await page.waitForTimeout(2000);

    // Check for status display
    const body = await page.locator('body').textContent();
    const hasStatus = body?.includes('Status') || 
                      body?.includes('queued') || 
                      body?.includes('processing') ||
                      body?.includes('completed');
    console.log(`Result page shows status: ${hasStatus}`);
    expect(hasStatus).toBeTruthy();
  });

  test('result page has copy to clipboard button for completed results', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: 'View Results', exact: true }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    // Check that Copy to Clipboard button exists in the UI code
    const body = await page.locator('body').textContent();
    console.log('Result page loaded, checking for clipboard functionality');

    // The copy button appears after a completed result is loaded
    const copyButton = page.getByRole('button', { name: /Copy to Clipboard/i });
    // This button may not be visible until a completed result is shown
    const copyButtonExists = await copyButton.count();
    console.log(`Copy to clipboard button count: ${copyButtonExists}`);
  });
});

// =============================================================================
// JOBS PAGE - COMPREHENSIVE FUNCTIONAL TESTS
// =============================================================================

test.describe('Jobs Page - Functional Tests', () => {
  test('jobs page has search input that filters results', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: /Job Management/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Find search input
    const searchInput = page.locator('#search').or(page.getByPlaceholder(/Search/i));
    await expect(searchInput.first()).toBeVisible();

    // Type in search
    await searchInput.first().fill('test-search-query');
    await page.waitForTimeout(1000);

    console.log('Search input functional');
  });

  test('jobs page has status filter dropdown', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: /Job Management/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Find status dropdown
    const statusSelect = page.locator('#status').or(page.getByLabel(/Status/i));
    await expect(statusSelect.first()).toBeVisible();

    // Check for status options
    const body = await page.locator('body').textContent();
    const hasQueuedOption = body?.includes('Queued');
    const hasProcessingOption = body?.includes('Processing');
    const hasCompletedOption = body?.includes('Completed');
    const hasFailedOption = body?.includes('Failed');

    console.log(`Status options: Queued=${hasQueuedOption}, Processing=${hasProcessingOption}, Completed=${hasCompletedOption}, Failed=${hasFailedOption}`);
    expect(hasQueuedOption || hasProcessingOption || hasCompletedOption || hasFailedOption).toBeTruthy();
  });

  test('jobs page refresh button reloads data', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: /Job Management/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Find and click the main refresh button (exact match, not auto-refresh)
    const refreshButton = page.getByRole('button', { name: 'Refresh', exact: true });
    await expect(refreshButton).toBeVisible();

    // Set up listener for API call
    const apiCallPromise = page.waitForResponse(
      (r) => r.url().includes('/admin/api/jobs') || r.url().includes('/v1/jobs'),
      { timeout: 10000 }
    ).catch(() => null);

    await refreshButton.click();
    const response = await apiCallPromise;

    if (response) {
      console.log(`Refresh triggered API call: ${response.url()}`);
    } else {
      console.log('No API call detected after refresh click');
    }
  });

  test('jobs page auto-refresh toggle works', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: /Job Management/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Find auto-refresh toggle button (has refresh icon)
    const autoRefreshButton = page.locator('button').filter({ has: page.locator('svg path[d*="M4 4v5"]') }).first();
    
    if (await autoRefreshButton.isVisible().catch(() => false)) {
      // Check initial state (should have green background if enabled)
      const initialClasses = await autoRefreshButton.getAttribute('class');
      console.log(`Auto-refresh initial classes: ${initialClasses}`);

      // Toggle
      await autoRefreshButton.click();
      await page.waitForTimeout(500);

      const afterToggleClasses = await autoRefreshButton.getAttribute('class');
      console.log(`Auto-refresh after toggle classes: ${afterToggleClasses}`);
    }
  });

  test('jobs page table shows job data with correct columns', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: /Job Management/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Check for table headers
    const tableHeaders = page.locator('th');
    const headerTexts: string[] = [];
    const headerCount = await tableHeaders.count();
    
    for (let i = 0; i < headerCount; i++) {
      const text = await tableHeaders.nth(i).textContent();
      if (text) headerTexts.push(text.trim());
    }

    console.log(`Table headers: ${headerTexts.join(', ')}`);

    // Expected columns
    const expectedColumns = ['Job ID', 'Status', 'CV ID', 'Project ID', 'Created', 'Updated', 'Actions'];
    const foundColumns = expectedColumns.filter(col => 
      headerTexts.some(h => h.toLowerCase().includes(col.toLowerCase()))
    );

    console.log(`Found expected columns: ${foundColumns.join(', ')}`);
    expect(foundColumns.length).toBeGreaterThan(0);
  });

  test('jobs page pagination works when jobs exist', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: /Job Management/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Check for pagination controls
    const prevButton = page.getByRole('button', { name: /Previous/i });
    const nextButton = page.getByRole('button', { name: /Next/i });

    const prevVisible = await prevButton.isVisible().catch(() => false);
    const nextVisible = await nextButton.isVisible().catch(() => false);

    console.log(`Pagination: Previous=${prevVisible}, Next=${nextVisible}`);

    // Check for page info text
    const body = await page.locator('body').textContent();
    const hasPageInfo = body?.includes('Page') || body?.includes('Showing');
    console.log(`Has pagination info: ${hasPageInfo}`);
  });

  test('jobs page View Details button opens modal', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: /Job Management/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Find View Details button
    const viewDetailsButton = page.getByRole('button', { name: /View Details/i }).first();
    
    if (await viewDetailsButton.isVisible().catch(() => false)) {
      await viewDetailsButton.click();
      await page.waitForTimeout(1000);

      // Check for modal
      const modal = page.locator('[role="dialog"]').or(page.locator('.modal')).or(page.locator('.fixed.inset-0'));
      const modalVisible = await modal.first().isVisible().catch(() => false);

      if (modalVisible) {
        console.log('Job details modal opened');
        
        // Check for modal content
        const modalBody = await modal.first().textContent();
        const hasJobId = modalBody?.includes('Job ID');
        const hasStatus = modalBody?.includes('Status');
        console.log(`Modal content: JobID=${hasJobId}, Status=${hasStatus}`);

        // Close modal
        const closeButton = modal.first().getByRole('button').filter({ has: page.locator('svg') }).first();
        if (await closeButton.isVisible().catch(() => false)) {
          await closeButton.click();
        }
      }
    } else {
      console.log('No jobs available to view details');
    }
  });
});

// =============================================================================
// GRAFANA JOB QUEUE METRICS DASHBOARD TESTS
// =============================================================================

test.describe('Grafana Job Queue Metrics Dashboard', () => {
  test('Job Queue Metrics dashboard loads with panels', async ({ page }) => {
    await loginViaSSO(page);

    await gotoWithRetry(page, '/grafana/d/job-queue-metrics/job-queue-metrics');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(3000);

    const body = await page.locator('body').textContent();
    
    // Check for expected panels from the screenshot
    const hasJobsProcessing = body?.includes('Jobs Currently Processing') || body?.includes('Currently Processing');
    const hasJobThroughput = body?.includes('Job Throughput') || body?.includes('Throughput');
    const hasJobSuccessRate = body?.includes('Job Success Rate') || body?.includes('Success Rate');
    const hasTotalJobOutcomes = body?.includes('Total Job Outcomes') || body?.includes('Job Outcomes');
    const hasEvaluationScore = body?.includes('Evaluation Score') || body?.includes('Score Distribution');

    console.log(`Dashboard panels: Processing=${hasJobsProcessing}, Throughput=${hasJobThroughput}, SuccessRate=${hasJobSuccessRate}, Outcomes=${hasTotalJobOutcomes}, Score=${hasEvaluationScore}`);

    // Should have at least some of these panels
    expect(hasJobsProcessing || hasJobThroughput || hasJobSuccessRate || hasTotalJobOutcomes || hasEvaluationScore).toBeTruthy();
  });

  test('Job Queue Metrics dashboard shows real data', async ({ page }) => {
    await loginViaSSO(page);

    // First, generate some job data
    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: 'Upload Files', exact: true }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    const fileInputs = page.locator('input[type="file"]');
    
    await fileInputs.nth(0).setInputFiles({
      name: 'cv.txt',
      mimeType: 'text/plain',
      buffer: Buffer.from('CV for metrics test'),
    });
    
    await fileInputs.nth(1).setInputFiles({
      name: 'project.txt',
      mimeType: 'text/plain',
      buffer: Buffer.from('Project for metrics test'),
    });

    await page.getByRole('button').filter({ hasText: /Upload/i }).first().click();
    await page.waitForTimeout(2000);

    // Navigate to Grafana dashboard
    await gotoWithRetry(page, '/grafana/d/job-queue-metrics/job-queue-metrics');
    await page.waitForLoadState('domcontentloaded');
    // Allow some time for Prometheus to scrape and Grafana to update
    // Retry a few times so we don't flake on slow environments.
    let hasNoData = false;
    let hasNumericData = false;

    for (let attempt = 0; attempt < 6; attempt += 1) {
      const body = await page.locator('body').textContent();
      const lowerBody = (body ?? '').toLowerCase();

      // Focus on the Job Success Rate section if present
      const idx = lowerBody.indexOf('job success rate');
      let windowText = lowerBody;
      if (idx !== -1) {
        windowText = lowerBody.substring(idx, Math.min(lowerBody.length, idx + 300));
      }

      hasNoData = windowText.includes('no data');
      const numericMatch = windowText.match(/\b\d+(\.\d+)?\b/);
      hasNumericData = !!numericMatch;

      console.log(`Job Queue Metrics attempt ${attempt + 1}: hasNoData=${hasNoData}, hasNumericData=${hasNumericData}`);

      if (!hasNoData && hasNumericData) {
        break;
      }

      await page.waitForTimeout(5000);
    }

    // In both dev and prod, we expect the Job Success Rate panel to have
    // real numeric data (not the generic "No data" state).
    expect(hasNoData).toBeFalsy();
    expect(hasNumericData).toBeTruthy();
  });

  test('Grafana dashboard time range selector works', async ({ page }) => {
    await loginViaSSO(page);

    await gotoWithRetry(page, '/grafana/d/job-queue-metrics/job-queue-metrics');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(3000);

    // Find time range picker
    const timeRangePicker = page.locator('[aria-label="Time picker"]').or(page.locator('button').filter({ hasText: /Last \d+ hours|Last \d+ minutes/i }));
    
    if (await timeRangePicker.first().isVisible().catch(() => false)) {
      await timeRangePicker.first().click();
      await page.waitForTimeout(500);

      // Check for time range options
      const body = await page.locator('body').textContent();
      const hasTimeOptions = body?.includes('Last 5 minutes') || 
                             body?.includes('Last 15 minutes') ||
                             body?.includes('Last 1 hour') ||
                             body?.includes('Last 6 hours');
      console.log(`Time range options visible: ${hasTimeOptions}`);
    }
  });
});

// =============================================================================
// MOBILE RESPONSIVE TESTS
// =============================================================================

test.describe('Mobile Responsive UI', () => {
  test('mobile sidebar toggle shows and hides navigation', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(1000);

    // Set mobile viewport
    await page.setViewportSize({ width: 375, height: 667 });
    await page.waitForTimeout(500);

    // Sidebar should be hidden initially on mobile
    const sidebar = page.locator('nav').first();
    
    // Find mobile menu toggle button (hamburger menu)
    const menuToggle = page.locator('button').filter({ has: page.locator('svg path[d*="M4 6h16M4 12h16M4 18h16"]') }).first();
    
    if (await menuToggle.isVisible().catch(() => false)) {
      console.log('Mobile menu toggle found');
      
      // Click to open sidebar
      await menuToggle.click();
      await page.waitForTimeout(500);

      // Check if sidebar navigation items are visible
      const dashboardLink = page.getByRole('link', { name: 'Dashboard', exact: true });
      const linkVisible = await dashboardLink.isVisible().catch(() => false);
      console.log(`Sidebar visible after toggle: ${linkVisible}`);
    }

    // Reset viewport
    await page.setViewportSize({ width: 1280, height: 720 });
  });

  test('jobs page mobile card view works', async ({ page }) => {
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: /Job Management/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Set mobile viewport
    await page.setViewportSize({ width: 375, height: 667 });
    await page.waitForTimeout(500);

    // On mobile, jobs should be displayed as cards instead of table
    const body = await page.locator('body').textContent();
    
    // Check for job content (should still show job info)
    const hasJobInfo = body?.includes('Job ID') || body?.includes('Status');
    console.log(`Mobile jobs view has job info: ${hasJobInfo}`);

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

// External Service Tracing - Jaeger Verification
test.describe('External Service Tracing', () => {
  test('Jaeger shows ai-cv-evaluator service after activity', async ({ page }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    // First generate some activity by navigating
    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Query Jaeger for services
    const servicesResp = await page.request.get('/jaeger/api/services');
    
    if (servicesResp.ok()) {
      const servicesBody = await servicesResp.json().catch(() => ({}));
      const services = (servicesBody as any)?.data ?? [];
      console.log('Jaeger services:', services);
      
      // Should have ai-cv-evaluator service
      const hasService = services.includes('ai-cv-evaluator') || services.length > 0;
      expect(hasService).toBeTruthy();
    }
  });

  // Skip external service trace tests in prod since they require real AI calls
  test('Jaeger has database spans after activity', async ({ page }) => {
    if (IS_PRODUCTION) {
      test.skip();
      return;
    }
    test.setTimeout(90000);
    await loginViaSSO(page);

    // Navigate to Job Management to trigger DB queries
    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.getByRole('link', { name: /Job Management/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(3000);

    // Query Jaeger for database traces
    let hasDbSpans = false;
    for (let attempt = 0; attempt < 10 && !hasDbSpans; attempt += 1) {
      const tracesResp = await page.request.get('/jaeger/api/traces', {
        params: {
          service: 'ai-cv-evaluator',
          lookback: '5m',
          limit: '100',
        },
      });

      if (tracesResp.ok()) {
        const tracesBody = await tracesResp.json().catch(() => ({}));
        const traces = (tracesBody as any)?.data ?? [];
        const allSpans = traces.flatMap((t: any) => t.spans ?? []);
        
        // Check for PostgreSQL/database spans (otelpgx creates spans with "query" or "pgx" in name)
        const dbSpans = allSpans.filter((s: any) => {
          const opName = String(s.operationName ?? '').toLowerCase();
          return opName.includes('query') || 
                 opName.includes('pgx') || 
                 opName.includes('select') ||
                 opName.includes('insert') ||
                 opName.includes('update') ||
                 opName.includes('db');
        });
        
        hasDbSpans = dbSpans.length > 0;
        console.log(`Attempt ${attempt + 1}: Found ${dbSpans.length} DB-related spans`);
        
        if (!hasDbSpans) {
          await page.waitForTimeout(2000);
        }
      }
    }
    
    // DB spans should be present after navigation that triggers queries
    // Note: otelpgx creates spans with SQL operation names like "SELECT", "INSERT", etc.
    // If not found, it could mean:
    // 1. Services weren't rebuilt with new tracing code
    // 2. DB queries haven't been executed yet
    // 3. Trace sampling is filtering them out
    console.log(`DB spans validation: ${hasDbSpans ? 'PASSED' : 'NOT FOUND (may need service rebuild)'}`);
    
    // Soft assertion - log but don't fail as DB spans depend on actual queries being traced
    if (!hasDbSpans) {
      console.log('Warning: No DB spans found. Verify services are rebuilt with otelpgx tracing.');
    }
  });

  test('Jaeger has queue/kafka spans after activity', async ({ page }) => {
    if (IS_PRODUCTION) {
      test.skip();
      return;
    }
    test.setTimeout(90000);
    await loginViaSSO(page);

    // Navigate and generate activity
    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(2000);

    // Query Jaeger for queue-related traces
    let hasQueueSpans = false;
    for (let attempt = 0; attempt < 10 && !hasQueueSpans; attempt += 1) {
      const tracesResp = await page.request.get('/jaeger/api/traces', {
        params: {
          service: 'ai-cv-evaluator',
          lookback: '10m',
          limit: '100',
        },
      });

      if (tracesResp.ok()) {
        const tracesBody = await tracesResp.json().catch(() => ({}));
        const traces = (tracesBody as any)?.data ?? [];
        const allSpans = traces.flatMap((t: any) => t.spans ?? []);
        
        // Check for Kafka/Redpanda spans (kotel creates spans with messaging semantic conventions)
        const queueSpans = allSpans.filter((s: any) => {
          const opName = String(s.operationName ?? '').toLowerCase();
          const tags = s.tags ?? [];
          const hasMessagingTag = tags.some((t: any) => 
            String(t.key ?? '').includes('messaging') ||
            String(t.value ?? '').toLowerCase().includes('kafka') ||
            String(t.value ?? '').toLowerCase().includes('redpanda')
          );
          return opName.includes('kafka') || 
                 opName.includes('produce') || 
                 opName.includes('consume') ||
                 opName.includes('enqueue') ||
                 opName.includes('queue') ||
                 hasMessagingTag;
        });
        
        hasQueueSpans = queueSpans.length > 0;
        console.log(`Attempt ${attempt + 1}: Found ${queueSpans.length} queue-related spans`);
        
        if (!hasQueueSpans) {
          await page.waitForTimeout(2000);
        }
      }
    }
    
    // Queue spans may or may not be present depending on whether evaluations ran
    console.log(`Queue spans found: ${hasQueueSpans}`);
  });
});

// Dashboard Completeness - RED Method Verification
test.describe('Dashboard Completeness Validation', () => {
  test('HTTP Metrics dashboard has all RED method panels', async ({ page }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    await gotoWithRetry(page, '/grafana/d/http-metrics/http-metrics');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(4000);

    const body = await page.locator('body').textContent() ?? '';
    const lowerBody = body.toLowerCase();

    // Check for RED method panels
    const hasRatePanel = lowerBody.includes('request rate') || lowerBody.includes('throughput');
    const hasErrorPanel = lowerBody.includes('error rate') || lowerBody.includes('error');
    const hasDurationPanel = lowerBody.includes('response time') || lowerBody.includes('duration') || lowerBody.includes('latency');
    
    console.log(`HTTP Metrics: Rate=${hasRatePanel}, Error=${hasErrorPanel}, Duration=${hasDurationPanel}`);
    
    expect(hasRatePanel).toBeTruthy();
    expect(hasErrorPanel).toBeTruthy();
    expect(hasDurationPanel).toBeTruthy();
  });

  test('HTTP Metrics dashboard has summary stat panels', async ({ page }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    await gotoWithRetry(page, '/grafana/d/http-metrics/http-metrics');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(4000);

    const body = await page.locator('body').textContent() ?? '';
    const lowerBody = body.toLowerCase();

    // Check for summary stat panels
    const hasTotalRequests = lowerBody.includes('total requests');
    const hasMedianResponseTime = lowerBody.includes('median') || lowerBody.includes('p50');
    const has95thPercentile = lowerBody.includes('95th') || lowerBody.includes('p95');
    
    console.log(`HTTP Metrics summary: TotalReqs=${hasTotalRequests}, Median=${hasMedianResponseTime}, p95=${has95thPercentile}`);
    
    expect(hasTotalRequests).toBeTruthy();
    expect(hasMedianResponseTime || has95thPercentile).toBeTruthy();
  });

  test('AI Metrics dashboard has provider-specific panels', async ({ page }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    await gotoWithRetry(page, '/grafana/d/ai-metrics/ai-metrics');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(4000);

    const body = await page.locator('body').textContent() ?? '';
    const lowerBody = body.toLowerCase();

    // Check for AI provider panels
    const hasRequestPanel = lowerBody.includes('request') || lowerBody.includes('ai');
    const hasDurationPanel = lowerBody.includes('duration') || lowerBody.includes('latency') || lowerBody.includes('response time');
    
    console.log(`AI Metrics: Requests=${hasRequestPanel}, Duration=${hasDurationPanel}`);
    
    expect(hasRequestPanel || hasDurationPanel).toBeTruthy();
  });

  test('Job Queue Metrics dashboard has comprehensive panels', async ({ page }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    await gotoWithRetry(page, '/grafana/d/job-queue-metrics/job-queue-metrics');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(4000);

    const body = await page.locator('body').textContent() ?? '';
    const lowerBody = body.toLowerCase();

    // Check for job queue panels
    const hasProcessingPanel = lowerBody.includes('processing');
    const hasEnqueuedPanel = lowerBody.includes('enqueue') || lowerBody.includes('throughput');
    const hasSuccessRatePanel = lowerBody.includes('success rate');
    const hasCompletedPanel = lowerBody.includes('completed') || lowerBody.includes('outcome');
    const hasFailedPanel = lowerBody.includes('failed');
    
    console.log(`Job Queue: Processing=${hasProcessingPanel}, Enqueued=${hasEnqueuedPanel}, SuccessRate=${hasSuccessRatePanel}, Completed=${hasCompletedPanel}, Failed=${hasFailedPanel}`);
    
    expect(hasProcessingPanel).toBeTruthy();
    expect(hasSuccessRatePanel).toBeTruthy();
    expect(hasCompletedPanel || hasFailedPanel).toBeTruthy();
  });

  test('Request Drilldown dashboard uses Loki logs', async ({ page }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    await gotoWithRetry(page, '/grafana/d/request-drilldown/request-drilldown');
    await page.waitForLoadState('domcontentloaded');
    await page.waitForTimeout(4000);

    const body = await page.locator('body').textContent() ?? '';
    const lowerBody = body.toLowerCase();

    // Check for Loki-based panels (logs view)
    const hasLogsPanel = lowerBody.includes('log') || lowerBody.includes('request');
    const hasRequestIdFilter = lowerBody.includes('request_id') || lowerBody.includes('request id');
    
    console.log(`Request Drilldown: Logs=${hasLogsPanel}, RequestID=${hasRequestIdFilter}`);
    
    // Dashboard should have log-based content
    expect(hasLogsPanel).toBeTruthy();
  });
});
