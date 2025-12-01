import { test, expect, Page } from '@playwright/test';

const PORTAL_PATH = '/';
const MOCK_CV_ID = 'cv-test-1';
const MOCK_PROJECT_ID = 'project-test-1';
const MOCK_JOB_ID = 'job-test-1';

const isSSOLoginUrl = (input: string | URL): boolean => {
  const url = typeof input === 'string' ? input : input.toString();
  return url.includes('/oauth2/') || url.includes('/realms/aicv');
};

const completeKeycloakProfileUpdate = async (page: Page): Promise<void> => {
  const heading = page.getByRole('heading', { name: /Update Account Information/i });
  const visible = await heading.isVisible().catch(() => false);
  if (!visible) {
    return;
  }

  const firstNameInput = page.getByRole('textbox', { name: /First name/i });
  const lastNameInput = page.getByRole('textbox', { name: /Last name/i });

  if (await firstNameInput.isVisible().catch(() => false)) {
    await firstNameInput.fill('Admin');
  }
  if (await lastNameInput.isVisible().catch(() => false)) {
    await lastNameInput.fill('User');
  }

  const submitProfileButton = page.getByRole('button', { name: /submit/i });
  if (await submitProfileButton.isVisible().catch(() => false)) {
    await submitProfileButton.click();
  }
};

const loginViaSSO = async (page: Page): Promise<void> => {
  await page.goto(PORTAL_PATH, { waitUntil: 'domcontentloaded' });

  if (!isSSOLoginUrl(page.url())) {
    // Already authenticated.
    return;
  }

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

const mockEvaluationBackend = async (page: Page): Promise<void> => {
  await page.route('**/v1/upload', async (route) => {
    await route.fulfill({
      status: 200,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ cv_id: MOCK_CV_ID, project_id: MOCK_PROJECT_ID }),
    });
  });

  await page.route('**/v1/evaluate', async (route) => {
    await route.fulfill({
      status: 200,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: MOCK_JOB_ID }),
    });
  });

  await page.route(`**/v1/result/${MOCK_JOB_ID}`, async (route) => {
    await route.fulfill({
      status: 200,
      headers: {
        'Content-Type': 'application/json',
        ETag: '"test-etag"',
      },
      body: JSON.stringify({
        status: 'completed',
        result: {
          summary: 'Mock evaluation result',
        },
      }),
    });
  });

  await page.route('**/admin/api/jobs?**', async (route) => {
    await route.fulfill({
      status: 200,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        jobs: [
          {
            id: MOCK_JOB_ID,
            status: 'completed',
            cv_id: MOCK_CV_ID,
            project_id: MOCK_PROJECT_ID,
            created_at: '2024-01-01T00:00:00Z',
            updated_at: '2024-01-01T00:10:00Z',
          },
        ],
        pagination: {
          page: 1,
          limit: 10,
          total: 1,
        },
      }),
    });
  });

  await page.route(`**/admin/api/jobs/${MOCK_JOB_ID}`, async (route) => {
    await route.fulfill({
      status: 200,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        id: MOCK_JOB_ID,
        status: 'completed',
        cv_id: MOCK_CV_ID,
        project_id: MOCK_PROJECT_ID,
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:10:00Z',
        result: {
          cv_match_rate: 0.9,
          project_score: 8,
          cv_feedback: 'Looks good',
          project_feedback: 'Strong project',
          overall_summary: 'Highly recommended',
        },
      }),
    });
  });
};

// Comprehensive admin UI flow after SSO login:
// Portal -> Frontend Dashboard -> Upload -> Evaluate -> Results -> Jobs
// Verifies that key headings and form controls render correctly.

test('admin UI main flow after SSO login', async ({ page, baseURL }) => {
  test.skip(!baseURL, 'Base URL must be configured');

  await loginViaSSO(page);

  // From portal, open the admin frontend.
  await page.getByRole('link', { name: /Open Frontend/i }).click();
  await page.waitForLoadState('domcontentloaded');

  // Dashboard view
  await expect(page.getByRole('heading', { name: /Dashboard/i })).toBeVisible();
  await expect(page.getByText(/Welcome back, /i)).toBeVisible();

  // Navigate to Upload view via sidebar
  await page.getByRole('link', { name: /Upload Files/i }).click();
  await page.waitForLoadState('domcontentloaded');
  await expect(page.getByRole('heading', { name: /Upload Files/i })).toBeVisible();
  await expect(page.getByText(/Upload your CV and project files/i)).toBeVisible();

  // Navigate to Evaluate view
  await page.getByRole('link', { name: /Start Evaluation/i }).click();
  await page.waitForLoadState('domcontentloaded');
  await expect(page.getByRole('heading', { name: /Start Evaluation/i })).toBeVisible();
  await expect(page.getByLabel('CV ID')).toBeVisible();
  await expect(page.getByLabel('Project ID')).toBeVisible();

  // Navigate to Results view
  await page.getByRole('link', { name: /View Results/i }).click();
  await page.waitForLoadState('domcontentloaded');
  await expect(page.getByRole('heading', { name: /View Results/i })).toBeVisible();
  await expect(page.getByLabel(/Job ID/i)).toBeVisible();

  // Navigate to Jobs view
  await page.getByRole('link', { name: /Job Management/i }).click();
  await page.waitForLoadState('domcontentloaded');
  await expect(page.getByRole('heading', { name: /Job Management/i })).toBeVisible();
  await expect(page.getByText(/Manage and monitor evaluation jobs/i).first()).toBeVisible();
});

test('dashboard stats reflect admin API stats', async ({ page, baseURL }) => {
  test.skip(!baseURL, 'Base URL must be configured');

  await loginViaSSO(page);

  // From portal, open the admin frontend dashboard.
  await page.getByRole('link', { name: /Open Frontend/i }).click();
  await page.waitForLoadState('domcontentloaded');

  // Wait until stats have been loaded (not in loading or error state).
  await expect(page.getByTestId('stats-uploads')).toBeVisible();

  const apiResp = await page.request.get('/admin/api/stats');
  expect(apiResp.ok()).toBeTruthy();
  const apiBody = await apiResp.json();

  const uploadsText = await page.getByTestId('stats-uploads').innerText();
  const evaluationsText = await page.getByTestId('stats-evaluations').innerText();
  const completedText = await page.getByTestId('stats-completed').innerText();
  const avgTimeText = await page.getByTestId('stats-avg-time').innerText();

  const parseNumber = (value: string): number => {
    const cleaned = value.replace(/[^0-9.\-]/g, '');
    const n = Number(cleaned || '0');
    return Number.isFinite(n) ? n : 0;
  };

  expect(parseNumber(uploadsText)).toBe((apiBody as any).uploads ?? 0);
  expect(parseNumber(evaluationsText)).toBe((apiBody as any).evaluations ?? 0);
  expect(parseNumber(completedText)).toBe((apiBody as any).completed ?? 0);

  const uiAvg = parseNumber(avgTimeText);
  const apiAvg = (apiBody as any).avg_time ?? 0;
  expect(Math.abs(uiAvg - apiAvg)).toBeLessThanOrEqual(0.001);
});

test('admin evaluation flow with mocked backend', async ({ page, baseURL }) => {
  test.skip(!baseURL, 'Base URL must be configured');

  await mockEvaluationBackend(page);

  await loginViaSSO(page);

  // From portal, open the admin frontend.
  await page.getByRole('link', { name: /Open Frontend/i }).click();
  await page.waitForLoadState('domcontentloaded');

  // Upload documents using fixture files.
  await page.getByRole('link', { name: /Upload Files/i }).click();
  await page.waitForLoadState('domcontentloaded');

  const fileInputs = page.locator('input[type="file"]');
  await fileInputs.nth(0).setInputFiles('tests/fixtures/cv.txt');
  await fileInputs.nth(1).setInputFiles('tests/fixtures/project.txt');

  await page.getByRole('button', { name: /^Upload Files$/i }).click();
  await expect(page.getByText(/Files uploaded successfully!/i)).toBeVisible();

  // Start evaluation with the returned IDs.
  await page.getByRole('link', { name: /Start Evaluation/i }).click();
  await page.waitForLoadState('domcontentloaded');

  await page.getByLabel('CV ID').fill(MOCK_CV_ID);
  await page.getByLabel('Project ID').fill(MOCK_PROJECT_ID);
  await page.getByRole('button', { name: /^Start Evaluation$/i }).click();

  await expect(page.getByText(/Evaluation started successfully!/i)).toBeVisible();
  await expect(page.locator('code', { hasText: MOCK_JOB_ID })).toBeVisible();

  // View results for the started job.
  await page.getByRole('link', { name: /View Results/i }).click();
  await page.waitForLoadState('domcontentloaded');

  await page.getByLabel(/Job ID/i).fill(MOCK_JOB_ID);
  await page.getByRole('button', { name: /Get Results/i }).click();

  await expect(page.getByRole('heading', { name: /Status/i })).toBeVisible();
  await expect(page.getByRole('heading', { level: 3, name: /Evaluation Results/i })).toBeVisible();

  // Check that the job appears in the Jobs table.
  await page.getByRole('link', { name: /Job Management/i }).click();
  await page.waitForLoadState('domcontentloaded');
  await expect(page.getByText(/Manage and monitor evaluation jobs/i).first()).toBeVisible();
  await expect(page.getByText(MOCK_JOB_ID)).toBeVisible();
});
