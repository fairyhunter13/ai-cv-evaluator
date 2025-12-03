import { test, expect, Page } from '@playwright/test';

const PORTAL_PATH = '/';

// Environment detection
const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:8088';
const IS_PRODUCTION = BASE_URL.includes('ai-cv-evaluator.web.id');

// Credentials: Use env vars, with sensible defaults for dev
const SSO_USERNAME = process.env.SSO_USERNAME || 'admin';
const SSO_PASSWORD = process.env.SSO_PASSWORD || (IS_PRODUCTION ? '' : 'admin123');

// Helper to check if SSO login tests should be skipped

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

const loginViaSSO = async (page: Page): Promise<void> => {
  await page.goto(PORTAL_PATH, { waitUntil: 'domcontentloaded' });
  if (!isSSOLoginUrl(page.url())) return;
  const usernameInput = page.locator('input#username');
  const passwordInput = page.locator('input#password');
  if (await usernameInput.isVisible()) {
    if (!SSO_PASSWORD) {
      throw new Error('SSO_PASSWORD required for login');
    }
    await usernameInput.fill(SSO_USERNAME);
    await passwordInput.fill(SSO_PASSWORD);
    const submitButton = page.locator('button[type="submit"], input[type="submit"]');
    await submitButton.first().click();
  }
  await completeKeycloakProfileUpdate(page);
  await page.waitForURL((url) => !isSSOLoginUrl(url), { timeout: 15000 });
};

test('backend evaluate validation errors (missing required fields)', async ({ page, baseURL }) => {
  test.skip(!baseURL, 'Base URL must be configured');
  await loginViaSSO(page);
  const resp = await page.request.post('/v1/evaluate', {
    headers: { 'Accept': 'application/json' },
    data: { cv_id: '', project_id: '' },
  });
  expect(resp.status()).toBe(400);
  const body = await resp.json();
  const err = (body as any)?.error ?? {};
  expect(err.code).toBe('INVALID_ARGUMENT');
  const details = (err.details as any) ?? {};
  expect(typeof details).toBe('object');
  expect(details.cvid ?? details.cv_id).toBeDefined();
  expect(details.projectid ?? details.project_id).toBeDefined();
});

test('invalid file upload extension via frontend returns 415 and error envelope', async ({ page, baseURL }) => {
  test.setTimeout(60000);
  test.skip(!baseURL, 'Base URL must be configured');
  await loginViaSSO(page);
  await page.getByRole('link', { name: /Open Frontend/i }).click();
  await page.waitForLoadState('domcontentloaded');
  await page.getByRole('link', { name: /Upload Files/i }).click();
  await page.waitForLoadState('domcontentloaded');
  const fileInputs = page.locator('input[type="file"]');
  await fileInputs.nth(0).setInputFiles('tests/fixtures/evil.exe');
  await fileInputs.nth(1).setInputFiles('tests/fixtures/evil.exe');
  const [uploadResp] = await Promise.all([
    page.waitForResponse((r) => r.url().includes('/v1/upload')),
    page.getByRole('button', { name: /^Upload Files$/i }).click(),
  ]);
  expect(uploadResp.status()).toBe(415);
  const body = await uploadResp.json();
  const err = (body as any)?.error ?? {};
  expect(err.code).toBe('INVALID_ARGUMENT');
  expect(String(err.message ?? '').toLowerCase()).toContain('unsupported media type');
});

test('upload missing files returns 400 with field=cv detail', async ({ page, baseURL }) => {
  test.skip(!baseURL, 'Base URL must be configured');
  await loginViaSSO(page);
  const resp = await page.request.post('/v1/upload', {
    multipart: {
      note: 'no files attached',
    },
    headers: { Accept: 'application/json' },
  });
  expect(resp.status()).toBe(400);
  const body = await resp.json();
  const err = (body as any)?.error ?? {};
  expect(err.code).toBe('INVALID_ARGUMENT');
  const details = (err.details as any) ?? {};
  // server sets details: { field: "cv" }
  expect((details.field ?? '').toLowerCase()).toBe('cv');
});

test('evaluate invalid JSON returns 400 INVALID_ARGUMENT', async ({ page, baseURL }) => {
  test.skip(!baseURL, 'Base URL must be configured');
  await loginViaSSO(page);
  const resp = await page.request.post('/v1/evaluate', {
    headers: { 'Accept': 'application/json', 'Content-Type': 'application/json' },
    data: 'not-json',
  });
  expect(resp.status()).toBe(400);
  const body = await resp.json();
  const err = (body as any)?.error ?? {};
  expect(err.code).toBe('INVALID_ARGUMENT');
  expect(String(err.message ?? '').toLowerCase()).toContain('invalid json');
});

test('upload form disables submit until both CV and project files are selected', async ({ page, baseURL }) => {
  test.skip(!baseURL, 'Base URL must be configured');
  await loginViaSSO(page);
  await page.getByRole('link', { name: /Open Frontend/i }).click();
  await page.waitForLoadState('domcontentloaded');
  await page.getByRole('link', { name: /Upload Files/i }).click();
  await page.waitForLoadState('domcontentloaded');

  const uploadButton = page.getByRole('button', { name: /^Upload Files$/i });
  await expect(uploadButton).toBeDisabled();

  const fileInputs = page.locator('input[type="file"]');
  await fileInputs.nth(0).setInputFiles('tests/fixtures/cv.txt');
  await expect(uploadButton).toBeDisabled();

  await fileInputs.nth(1).setInputFiles('tests/fixtures/project.txt');
  await expect(uploadButton).toBeEnabled();
});

test('evaluate view surfaces backend validation error when fields are empty', async ({ page, baseURL }) => {
  test.skip(!baseURL, 'Base URL must be configured');
  await loginViaSSO(page);
  await page.getByRole('link', { name: /Open Frontend/i }).click();
  await page.waitForLoadState('domcontentloaded');
  await page.getByRole('link', { name: /Start Evaluation/i }).first().click();
  await page.waitForLoadState('domcontentloaded');

  const startButton = page.getByRole('button', { name: /^Start Evaluation$/i });
  await startButton.click();

  await expect(
    page.getByRole('heading', {
      name: /Invalid request\. Please check your input and try again\./i,
    }),
  ).toBeVisible({ timeout: 15000 });
});

test('result view shows client-side validation when Job ID is empty', async ({ page, baseURL }) => {
  test.skip(!baseURL, 'Base URL must be configured');
  await loginViaSSO(page);
  await page.getByRole('link', { name: /Open Frontend/i }).click();
  await page.waitForLoadState('domcontentloaded');
  await page.getByRole('link', { name: /View Results/i }).first().click();
  await page.waitForLoadState('domcontentloaded');

  const getResultsButton = page.getByRole('button', { name: /Get Results/i });
  await getResultsButton.click();

  await expect(page.getByText(/Please enter a Job ID/i)).toBeVisible({ timeout: 10000 });
});
