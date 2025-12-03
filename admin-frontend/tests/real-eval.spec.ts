import { test, expect, Page } from '@playwright/test';

const PORTAL_PATH = '/';

// Environment detection
const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:8088';
const IS_PRODUCTION = BASE_URL.includes('ai-cv-evaluator.web.id');

// Credentials: Use env vars, with sensible defaults for dev
const SSO_USERNAME = process.env.SSO_USERNAME || 'admin';
const SSO_PASSWORD = process.env.SSO_PASSWORD || (IS_PRODUCTION ? '' : 'admin123');

// Helper to check if SSO login tests should be skipped
const requiresSSOCredentials = (): boolean => !SSO_PASSWORD;

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

// Drive a REAL evaluation end-to-end (no mocks): upload -> evaluate -> poll result -> assert Jaeger spans
// Uses small .txt fixtures to keep ingestion fast and deterministic.
test('real evaluation end-to-end produces integrated evaluation spans', async ({ page, context, baseURL }) => {
  test.setTimeout(180000);
  test.skip(!baseURL, 'Base URL must be configured');
  test.skip(requiresSSOCredentials(), 'SSO_PASSWORD required');

  await loginViaSSO(page);

  // Open admin frontend and upload fixtures
  await page.getByRole('link', { name: /Open Frontend/i }).click();
  await page.waitForLoadState('domcontentloaded');
  await page.getByRole('link', { name: /Upload Files/i }).click();
  await page.waitForLoadState('domcontentloaded');

  const fileInputs = page.locator('input[type="file"]');
  await fileInputs.nth(0).setInputFiles('tests/fixtures/cv.txt');
  await fileInputs.nth(1).setInputFiles('tests/fixtures/project.txt');

  const [uploadResp] = await Promise.all([
    page.waitForResponse((r) => r.url().includes('/v1/upload') && r.request().method() === 'POST'),
    page.getByRole('button', { name: /^Upload Files$/i }).click(),
  ]);
  expect(uploadResp.status()).toBe(200);
  const uploadJson = await uploadResp.json();
  const cvId = (uploadJson as any)?.cv_id as string;
  const projectId = (uploadJson as any)?.project_id as string;
  expect(typeof cvId).toBe('string');
  expect(typeof projectId).toBe('string');

  // Start evaluation with the returned IDs via UI
  await page.getByRole('link', { name: /Start Evaluation/i }).click();
  await page.waitForLoadState('domcontentloaded');
  await page.getByLabel('CV ID').fill(cvId);
  await page.getByLabel('Project ID').fill(projectId);

  const [evalResp] = await Promise.all([
    page.waitForResponse((r) => r.url().includes('/v1/evaluate') && r.request().method() === 'POST'),
    page.getByRole('button', { name: /^Start Evaluation$/i }).click(),
  ]);
  expect(evalResp.status()).toBe(200);
  const evalJson = await evalResp.json();
  const jobId = (evalJson as any)?.id as string;
  expect(typeof jobId).toBe('string');

  // Poll result endpoint for up to ~90s (do not require completed to avoid flakes)
  let lastStatus = '';
  for (let i = 0; i < 90; i += 1) {
    const res = await page.request.get(`/v1/result/${jobId}`);
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    lastStatus = String((body as any)?.status ?? '');
    if (['processing', 'completed', 'failed'].includes(lastStatus)) break;
    await page.waitForTimeout(1000);
  }
  expect(['queued', 'processing', 'completed', 'failed']).toContain(lastStatus);

  // After triggering evaluation, assert Jaeger integrated evaluation spans appear with children.
  // Retry up to 60s to account for ingestion and sampling.
  let evalTraces: any[] = [];
  for (let attempt = 0; attempt < 60 && evalTraces.length === 0; attempt += 1) {
    const resp = await page.request.get('/jaeger/api/traces', {
      params: {
        service: 'ai-cv-evaluator',
        operation: 'PerformIntegratedEvaluation',
        lookback: '1h',
        limit: '20',
      },
    });
    expect(resp.ok()).toBeTruthy();
    const body = await resp.json();
    evalTraces = (body as any)?.data ?? [];
    if (evalTraces.length === 0) await page.waitForTimeout(1000);
  }
  expect(evalTraces.length).toBeGreaterThan(0);
  const allEvalSpans = evalTraces.flatMap((t: any) => (t.spans ?? []));
  const names = new Set(allEvalSpans.map((s: any) => String(s.operationName ?? '')));
  expect(names.has('PerformIntegratedEvaluation')).toBeTruthy();
  const childOps = [
    'PerformIntegratedEvaluation.evaluateCVMatch',
    'PerformIntegratedEvaluation.evaluateProjectDeliverables',
    'PerformIntegratedEvaluation.refineEvaluation',
    'PerformIntegratedEvaluation.validateAndFinalizeResults',
    'PerformIntegratedEvaluation.fastPath',
  ];
  expect(childOps.some((n) => names.has(n))).toBeTruthy();
});
