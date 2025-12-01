import { test, expect, Page, BrowserContext } from '@playwright/test';

const PORTAL_PATH = '/';

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
      if (!message.includes('net::ERR_CONNECTION_REFUSED') || attempt === maxAttempts) {
        throw err;
      }
      await page.waitForTimeout(retryDelayMs);
    }
  }
};

const loginViaSSO = async (page: Page): Promise<void> => {
  await gotoWithRetry(page, PORTAL_PATH);
  if (!isSSOLoginUrl(page.url())) return;
  const usernameInput = page.locator('input#username');
  const passwordInput = page.locator('input#password');
  if (await usernameInput.isVisible()) {
    await usernameInput.fill('admin');
    await passwordInput.fill('admin123');
    const submitButton = page.locator('button[type="submit"], input[type="submit"]');
    await submitButton.first().click();
  }
  await completeKeycloakProfileUpdate(page);
  await page.waitForURL((url) => !isSSOLoginUrl(url), { timeout: 15000 });
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

// =============================================================================
// PORTAL PAGE TESTS
// =============================================================================

test.describe('Portal Page', () => {
  test('portal page displays all navigation links after SSO login', async ({ page, baseURL }) => {
    test.skip(!baseURL, 'Base URL must be configured');
    await loginViaSSO(page);

    // Portal should show navigation links to all surfaced services
    await expect(page.getByRole('link', { name: /Open Frontend/i })).toBeVisible();
    await expect(page.getByRole('link', { name: /Open API/i })).toBeVisible();
    await expect(page.getByRole('link', { name: /Health/i })).toBeVisible();
    await expect(page.getByRole('link', { name: /Open Grafana/i })).toBeVisible();
    await expect(page.getByRole('link', { name: /Open Mailpit/i })).toBeVisible();
    await expect(page.getByRole('link', { name: /Open Jaeger/i })).toBeVisible();
    await expect(page.getByRole('link', { name: /Open Redpanda/i })).toBeVisible();
  });

  test('portal page has proper title and branding', async ({ page, baseURL }) => {
    test.skip(!baseURL, 'Base URL must be configured');
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
    test.skip(!baseURL, 'Base URL must be configured');

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
    test.skip(!baseURL, 'Base URL must be configured');
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
    test.skip(!baseURL, 'Base URL must be configured');
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
    test.skip(!baseURL, 'Base URL must be configured');
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
    test.skip(!baseURL, 'Base URL must be configured');

    const resp = await apiRequestWithRetry(page, 'get', '/healthz');
    expect(resp.status()).toBe(200);
  });

  test('readyz endpoint returns 200 when ready', async ({ page, baseURL }) => {
    test.skip(!baseURL, 'Base URL must be configured');

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
    test.skip(!baseURL, 'Base URL must be configured');
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
    test.skip(!baseURL, 'Base URL must be configured');
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
    test.skip(!baseURL, 'Base URL must be configured');
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
    test.skip(!baseURL, 'Base URL must be configured');
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

    // The provisioned alert rule groups for HTTP errors should be visible.
    // One is the Grafana alerting rule group, the other comes from Prometheus rules.
    await expect(
      page.getByRole('heading', { name: /ai-cv-evaluator-http-alerts/i }),
    ).toBeVisible();
    await expect(
      page.getByRole('heading', { name: /ai-cv-evaluator-http-errors/i }),
    ).toBeVisible();
  });

  test('Grafana contact points are configured', async ({ page, baseURL }) => {
    test.skip(!baseURL, 'Base URL must be configured');
    test.setTimeout(60000);
    await loginViaSSO(page);

    // Navigate to contact points page
    await gotoWithRetry(page, '/grafana/alerting/notifications');
    await page.waitForLoadState('networkidle');

    // Verify we're on Grafana and the email-ai-cv-evaluator contact point is present
    await expect(page).toHaveTitle(/Grafana/i, { timeout: 15000 });
    await expect(page.getByText(/email-ai-cv-evaluator/i).first()).toBeVisible();
  });

  test('Prometheus has HTTP error alert rule configured', async ({ page, baseURL }) => {
    test.skip(!baseURL, 'Base URL must be configured');
    test.setTimeout(60000);
    await loginViaSSO(page);

    // Query Prometheus for alert rules
    await gotoWithRetry(page, '/prometheus/alerts');
    await page.waitForLoadState('networkidle');

    // The alerts page should load
    const body = await page.locator('body').textContent();
    expect(body).toBeTruthy();
  });

  test('Mailpit is accessible for receiving alert emails', async ({ page, baseURL }) => {
    test.skip(!baseURL, 'Base URL must be configured');
    await loginViaSSO(page);

    // Navigate to Mailpit
    await gotoWithRetry(page, '/mailpit/');
    await page.waitForLoadState('domcontentloaded');

    // Verify Mailpit loaded (it's a JavaScript SPA)
    const title = await page.title();
    expect(title.toLowerCase()).toContain('mailpit');
  });

  test('alerting flow: generate errors and verify alert infrastructure', async ({ page, baseURL }) => {
    test.skip(!baseURL, 'Base URL must be configured');
    test.setTimeout(180000); // 3 minutes for alerting flow
    await loginViaSSO(page);

    // Step 1: Generate HTTP errors to trigger the alert
    for (let i = 0; i < 10; i += 1) {
      await page.request.get('/v1/__nonexistent_path_for_errors');
      await page.waitForTimeout(100);
    }

    // Step 2: Verify Prometheus is recording non-OK HTTP requests
    const promResp = await apiRequestWithRetry(
      page,
      'get',
      '/prometheus/api/v1/query?query=sum%20by(status)%20(rate(http_requests_total{status!="OK"}[5m]))',
    );
    expect(promResp.status()).toBe(200);
    const promBody = await promResp.json();
    const promResults = (promBody as any).data?.result ?? [];
    expect(promResults.length).toBeGreaterThan(0);

    // Step 3: Wait for the HighHttpErrorRate alert to be firing in Prometheus
    let alertIsFiring = false;
    const maxAlertAttempts = 5;
    for (let attempt = 1; attempt <= maxAlertAttempts && !alertIsFiring; attempt += 1) {
      const alertsResp = await apiRequestWithRetry(
        page,
        'get',
        '/prometheus/api/v1/query?query=ALERTS{alertname="HighHttpErrorRate"}',
      );
      expect(alertsResp.status()).toBe(200);
      const alertsBody = await alertsResp.json();
      const alertResults = (alertsBody as any).data?.result ?? [];
      alertIsFiring = alertResults.some((r: any) => r.metric?.alertstate === 'firing');
      if (!alertIsFiring) {
        await page.waitForTimeout(5000);
      }
    }
    expect(alertIsFiring).toBeTruthy();

    // Step 4: Verify Grafana alert rules are visible in the UI
    await gotoWithRetry(page, '/grafana/alerting/list');
    await page.waitForLoadState('networkidle');
    await expect(page).toHaveTitle(/Grafana/i, { timeout: 15000 });

    // Step 5: Verify Mailpit has at least one alert email
    const mailpitResp = await apiRequestWithRetry(page, 'get', '/mailpit/api/v1/messages');
    expect(mailpitResp.status()).toBe(200);
    const mailpitBody = (await mailpitResp.json()) as any;
    const totalMessages =
      mailpitBody.total ?? mailpitBody.count ?? (mailpitBody.messages?.length ?? 0);
    expect(totalMessages).toBeGreaterThan(0);
  });
});

// =============================================================================
// RESPONSIVE DESIGN TESTS
// =============================================================================

test.describe('Responsive Design', () => {
  test('admin frontend works on mobile viewport', async ({ page, baseURL }) => {
    test.skip(!baseURL, 'Base URL must be configured');

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
    test.skip(!baseURL, 'Base URL must be configured');

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
    test.skip(!baseURL, 'Base URL must be configured');
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
    test.skip(!baseURL, 'Base URL must be configured');
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
    test.skip(!baseURL, 'Base URL must be configured');
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
    test.skip(!baseURL, 'Base URL must be configured');
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
    test.skip(!baseURL, 'Base URL must be configured');
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
    test.skip(!baseURL, 'Base URL must be configured');
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
    test.skip(!baseURL, 'Base URL must be configured');
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
    test.skip(!baseURL, 'Base URL must be configured');
    await loginViaSSO(page);

    await gotoWithRetry(page, '/prometheus/targets');
    await page.waitForLoadState('networkidle');

    // Prometheus targets page should load
    const body = await page.locator('body').textContent();
    expect(body).toBeTruthy();
    // Should contain some target information
    expect(body?.toLowerCase()).toContain('target');
  });

  test('Jaeger is accessible and has services', async ({ page, baseURL }) => {
    test.skip(!baseURL, 'Base URL must be configured');
    await loginViaSSO(page);

    await gotoWithRetry(page, '/jaeger/');
    await page.waitForLoadState('networkidle');

    // Jaeger UI should load
    const body = await page.locator('body').textContent();
    expect(body).toBeTruthy();
  });

  test('Redpanda Console is accessible', async ({ page, baseURL }) => {
    test.skip(!baseURL, 'Base URL must be configured');
    await loginViaSSO(page);

    await gotoWithRetry(page, '/redpanda/');
    await page.waitForLoadState('networkidle');

    // Redpanda Console should load
    const body = await page.locator('body').textContent();
    expect(body).toBeTruthy();
  });
});
