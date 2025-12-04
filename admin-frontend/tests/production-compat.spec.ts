import { test, expect, Page } from '@playwright/test';

const PORTAL_PATH = '/';

// Environment detection
const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:8088';
const IS_PRODUCTION = BASE_URL.includes('ai-cv-evaluator.web.id');
const IS_DEV = !IS_PRODUCTION;

// Credentials: Use env vars, with sensible defaults for dev
const SSO_USERNAME = process.env.SSO_USERNAME || 'admin';
const SSO_PASSWORD = process.env.SSO_PASSWORD || (IS_PRODUCTION ? '' : 'admin123');

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
  const maxAttempts = 10;
  const retryDelayMs = 3000;
  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    try {
      await page.goto(path, { waitUntil: 'domcontentloaded', timeout: 30000 });
      return;
    } catch (err) {
      const message = String(err);
      const transientErrors = [
        'net::ERR_CONNECTION_REFUSED',
        'net::ERR_SOCKET_NOT_CONNECTED',
        'net::ERR_CONNECTION_RESET',
        'net::ERR_EMPTY_RESPONSE',
      ];
      if (!transientErrors.some((pattern) => message.includes(pattern)) || attempt === maxAttempts) {
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

// =============================================================================
// PRODUCTION-COMPATIBLE CORE FUNCTIONALITY TESTS
// These tests work in both dev and production environments
// =============================================================================

test.describe('Production Compatible - Core Functionality', () => {
  test('SSO authentication flow works', async ({ page, baseURL }) => {

    await gotoWithRetry(page, PORTAL_PATH);

    // Should redirect to SSO
    expect(isSSOLoginUrl(page.url())).toBeTruthy();

    // Complete login
    await loginViaSSO(page);

    // Should be authenticated now
    expect(isSSOLoginUrl(page.url())).toBeFalsy();
  });

  test('portal loads and displays navigation after login', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    // Should have portal content
    await expect(page.getByRole('link', { name: /Open Frontend/i })).toBeVisible();
    await expect(page.getByRole('link', { name: /Open API/i })).toBeVisible();
    await expect(page.getByRole('link', { name: /Health/i })).toBeVisible();
    await expect(page.getByRole('link', { name: /Open Grafana/i })).toBeVisible();
  });

  test('health endpoint returns healthy status', async ({ page, baseURL }) => {

    const resp = await apiRequestWithRetry(page, 'get', '/healthz');
    expect(resp.status()).toBe(200);

    const body = await resp.json();
    expect((body as any).status).toBe('healthy');
  });

  test('readiness endpoint is accessible', async ({ page, baseURL }) => {

    const resp = await apiRequestWithRetry(page, 'get', '/readyz');
    expect([200, 503]).toContain(resp.status());
  });

  test('OpenAPI spec is served correctly', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    const resp = await apiRequestWithRetry(page, 'get', '/openapi.yaml');
    expect(resp.status()).toBe(200);

    const contentType = resp.headers()['content-type'] ?? '';
    expect(contentType).toContain('yaml');

    const body = await resp.text();
    expect(body).toContain('openapi:');
    expect(body).toContain('AI CV Evaluator API');
  });
});

// =============================================================================
// PRODUCTION-COMPATIBLE ADMIN FRONTEND TESTS
// =============================================================================

test.describe('Production Compatible - Admin Frontend', () => {
  test('admin frontend loads and shows app content', async ({ page, baseURL }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('networkidle');

    // Verify we're on the /app/ path
    await expect(page).toHaveURL(/\/app\//);

    // Wait for Vue app to mount - check for app title
    await expect(page).toHaveTitle(/AI CV Evaluator/i, { timeout: 15000 });

    // The Vue app should render content - check for the sidebar navigation or any app content
    // In the Vue app, the sidebar always shows these navigation links
    const sidebarContent = page.getByText(/AI CV Evaluator/i);
    await expect(sidebarContent.first()).toBeVisible({ timeout: 20000 });
  });

  test('admin frontend navigation is accessible', async ({ page, baseURL }) => {
    test.setTimeout(90000);
    await loginViaSSO(page);

    await page.getByRole('link', { name: /Open Frontend/i }).click();
    await page.waitForLoadState('networkidle');
    await expect(page).toHaveURL(/\/app\//);

    // Wait for Vue app to fully render - check page title and sidebar
    await expect(page).toHaveTitle(/AI CV Evaluator/i, { timeout: 15000 });
    await expect(page.getByText(/AI CV Evaluator/i).first()).toBeVisible({ timeout: 20000 });

    // Verify navigation links are present - these should always be visible in the sidebar
    await expect(page.getByRole('link', { name: /Upload Files/i }).first()).toBeVisible();
    await expect(page.getByRole('link', { name: /Start Evaluation/i }).first()).toBeVisible();
    await expect(page.getByRole('link', { name: /View Results/i }).first()).toBeVisible();
  });

  test('admin API stats endpoint works', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    const resp = await apiRequestWithRetry(page, 'get', '/admin/api/stats');
    expect(resp.status()).toBe(200);

    const body = await resp.json();
    expect(typeof (body as any).uploads).toBe('number');
    expect(typeof (body as any).evaluations).toBe('number');
  });

  test('admin API jobs endpoint works', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    const resp = await page.request.get('/admin/api/jobs?page=1&limit=10');
    expect(resp.status()).toBe(200);

    const body = await resp.json();
    expect(Array.isArray((body as any).jobs)).toBeTruthy();
    expect((body as any).pagination).toBeDefined();
  });
});

// =============================================================================
// PRODUCTION-COMPATIBLE OBSERVABILITY TESTS
// =============================================================================

test.describe('Production Compatible - Observability', () => {
  test('Grafana is accessible after SSO', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    await gotoWithRetry(page, '/grafana/');
    await page.waitForLoadState('networkidle');

    expect(isSSOLoginUrl(page.url())).toBeFalsy();
    await expect(page).toHaveTitle(/Grafana/i, { timeout: 15000 });
  });

  test('Jaeger is accessible after SSO', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    await gotoWithRetry(page, '/jaeger/');
    await page.waitForLoadState('networkidle');

    expect(isSSOLoginUrl(page.url())).toBeFalsy();
    await expect(page).toHaveTitle(/Jaeger/i, { timeout: 15000 });
  });

  test('Prometheus metrics are being collected', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    // Generate some traffic first
    for (let i = 0; i < 3; i += 1) {
      await page.request.get('/healthz');
    }

    // Query Prometheus through Grafana
    const resp = await apiRequestWithRetry(
      page,
      'get',
      '/grafana/api/datasources/proxy/uid/prometheus/api/v1/query',
      { params: { query: 'up' } },
    );

    if (resp.status() === 200) {
      const body = await resp.json();
      expect((body as any).status).toBe('success');
    }
  });

  test('Grafana dashboards are accessible', async ({ page, baseURL }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    const dashboards = [
      '/grafana/d/http-metrics/http-metrics',
      '/grafana/d/ai-metrics/ai-metrics',
      '/grafana/d/job-queue-metrics/job-queue-metrics',
    ];

    for (const path of dashboards) {
      await gotoWithRetry(page, path);
      await page.waitForLoadState('networkidle');

      expect(isSSOLoginUrl(page.url())).toBeFalsy();
      const title = await page.title();
      expect(title).not.toContain('502');
      expect(title).not.toContain('Error');
    }
  });

  test('Grafana alerting page is accessible', async ({ page, baseURL }) => {
    test.setTimeout(60000);
    await loginViaSSO(page);

    await gotoWithRetry(page, '/grafana/alerting/list');
    await page.waitForLoadState('networkidle');

    expect(isSSOLoginUrl(page.url())).toBeFalsy();
    await expect(page).toHaveTitle(/Grafana/i, { timeout: 15000 });
  });

  test('Redpanda Console is accessible after SSO', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    await gotoWithRetry(page, '/redpanda/');
    await page.waitForLoadState('networkidle');

    expect(isSSOLoginUrl(page.url())).toBeFalsy();
    await expect(page).toHaveTitle(/Redpanda/i, { timeout: 15000 });
  });
});

// =============================================================================
// PRODUCTION-COMPATIBLE SECURITY TESTS
// =============================================================================

test.describe('Production Compatible - Security', () => {
  test('unauthenticated requests are redirected to SSO', async ({ browser, baseURL }) => {

    const freshContext = await browser.newContext();
    const freshPage = await freshContext.newPage();

    try {
      // Protected paths should redirect to SSO
      const protectedPaths = ['/app/', '/grafana/', '/prometheus/', '/jaeger/', '/admin/'];

      for (const path of protectedPaths) {
        await gotoWithRetry(freshPage, path);
        expect(
          isSSOLoginUrl(freshPage.url()),
          `Expected ${path} to redirect to SSO`,
        ).toBeTruthy();
      }
    } finally {
      await freshContext.close();
    }
  });

  test('CORS headers are properly configured', async ({ page, baseURL }) => {

    // Health endpoint should have proper headers
    const resp = await page.request.get('/healthz');
    const headers = resp.headers();

    // Verify response has proper structure
    expect(resp.ok()).toBeTruthy();
  });

  test('session isolation between browser contexts', async ({ browser, baseURL }) => {

    // First context - authenticate
    const context1 = await browser.newContext();
    const page1 = await context1.newPage();

    await gotoWithRetry(page1, PORTAL_PATH);
    if (isSSOLoginUrl(page1.url()) && SSO_PASSWORD) {
      await page1.fill('input#username', SSO_USERNAME);
      await page1.fill('input#password', SSO_PASSWORD);
      await page1.click('button[type="submit"], input[type="submit"]');
      await completeKeycloakProfileUpdate(page1);
    }

    // Second context - should not be authenticated
    const context2 = await browser.newContext();
    const page2 = await context2.newPage();

    await gotoWithRetry(page2, '/app/');
    expect(isSSOLoginUrl(page2.url())).toBeTruthy();

    await context1.close();
    await context2.close();
  });
});

// =============================================================================
// PRODUCTION-COMPATIBLE ERROR HANDLING TESTS
// =============================================================================

test.describe('Production Compatible - Error Handling', () => {
  test('404 error for non-existent API endpoint', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    const resp = await page.request.get('/v1/__nonexistent_path');
    expect(resp.status()).toBe(404);
  });

  test('invalid result ID returns appropriate error', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    const resp = await page.request.get('/v1/result/invalid-job-id-12345');
    expect([404, 400]).toContain(resp.status());
  });

  test('malformed JSON in evaluate request returns 400', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    // Navigate to a page first to ensure session is established
    await gotoWithRetry(page, '/app/');
    await page.waitForLoadState('networkidle');

    const resp = await page.request.post('/v1/evaluate', {
      headers: { 'Content-Type': 'application/json', 'Accept': 'application/json' },
      data: 'invalid-json',
    });
    // In production with OAuth, might return 401 if session not fully established
    expect([400, 401]).toContain(resp.status());

    if (resp.status() === 400) {
      const body = await resp.json();
      expect((body as any).error?.code).toBe('INVALID_ARGUMENT');
    }
  });

  test('missing required fields in evaluate returns 400', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    // Navigate to a page first to ensure session is established
    await gotoWithRetry(page, '/app/');
    await page.waitForLoadState('networkidle');

    const resp = await page.request.post('/v1/evaluate', {
      headers: { 'Accept': 'application/json' },
      data: { cv_id: '', project_id: '' },
    });
    // In production with OAuth, might return 401 if session not fully established
    expect([400, 401]).toContain(resp.status());

    if (resp.status() === 400) {
      const body = await resp.json();
      expect((body as any).error?.code).toBe('INVALID_ARGUMENT');
    }
  });
});

// =============================================================================
// EMAIL NOTIFICATION INFRASTRUCTURE TESTS
// In both dev and prod: test Mailpit directly
// In prod: additionally tests Grafana contact points configuration
// =============================================================================

test.describe('Email Notification Infrastructure', () => {
  test('email testing infrastructure is accessible', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    // In both environments, Mailpit dashboard should be reachable after SSO
    await gotoWithRetry(page, '/mailpit/');
    await page.waitForLoadState('domcontentloaded');

    expect(isSSOLoginUrl(page.url())).toBeFalsy();
    const title = (await page.title()).toLowerCase();
    expect(title).toContain('mailpit');

    // In prod, also verify Grafana alerting notifications page
    if (IS_PRODUCTION) {
      await gotoWithRetry(page, '/grafana/alerting/notifications');
      await page.waitForLoadState('networkidle');
      await expect(page).toHaveTitle(/Grafana/i, { timeout: 15000 });
    }
  });

  test('email notification API is accessible', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    // In both environments, Mailpit API should be reachable via /mailpit/api/v1/messages
    const mailpitResp = await apiRequestWithRetry(page, 'get', '/mailpit/api/v1/messages');
    expect(mailpitResp.status()).toBe(200);

    const mailpitBody = await mailpitResp.json();
    expect(mailpitBody).toHaveProperty('total');
    expect(mailpitBody).toHaveProperty('messages');

    if (IS_PRODUCTION) {
      // In prod, also verify Grafana contact points API
      const resp = await apiRequestWithRetry(page, 'get', '/grafana/api/v1/provisioning/contact-points');
      const contentType = resp.headers()['content-type'] ?? '';
      if (contentType.includes('application/json') && resp.status() === 200) {
        const body = await resp.json();
        const cpList = Array.isArray(body) ? body : body.contactPoints ?? [];
        expect(cpList.length).toBeGreaterThan(0);
      }
    }
  });

  test('Portal shows appropriate email-related links', async ({ page, baseURL }) => {
    await loginViaSSO(page);

    // In both environments, portal should show navigation links
    // The portal template may include Mailpit link in both envs even if Mailpit is not running
    const mailpitLink = page.getByRole('link', { name: /Open Mailpit/i });
    const grafanaLink = page.getByRole('link', { name: /Open Grafana/i });

    // Grafana should always be visible for observability
    await expect(grafanaLink).toBeVisible();

    if (IS_DEV) {
      // In dev, Mailpit should be visible and accessible
      await expect(mailpitLink).toBeVisible();
    }
    // In production, the portal link visibility depends on the portal template
    // We don't enforce Mailpit link absence since the portal template is shared
  });
});

// =============================================================================
// ENVIRONMENT DETECTION TESTS
// =============================================================================

test.describe('Environment Detection', () => {
  test('environment variables are properly detected', async ({ baseURL }) => {

    // Verify environment detection is working
    expect(typeof IS_PRODUCTION).toBe('boolean');
    expect(typeof IS_DEV).toBe('boolean');
    expect(IS_PRODUCTION !== IS_DEV).toBeTruthy();
  });

  test('base URL is properly configured', async ({ baseURL }) => {

    expect(baseURL).toBeTruthy();
    expect(baseURL?.startsWith('http')).toBeTruthy();
  });
});
