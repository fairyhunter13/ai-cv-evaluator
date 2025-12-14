import { test, expect } from '@playwright/test';

import { PORTAL_PATH } from '../helpers/env.ts';
import { apiRequestWithRetry } from '../helpers/api.ts';
import { performApiLogin } from '../helpers/authelia.ts';
import { gotoWithRetry } from '../helpers/navigation.ts';
import { isSSOLoginUrl } from '../helpers/sso.ts';

export const registerBackendApiAndHealthTests = (): void => {
  test('backend API and health reachable via portal after SSO login', async ({ page, baseURL: _baseURL }) => {
    // Use API Login Bypass for robustness in CI
    await performApiLogin(page);
    await gotoWithRetry(page, PORTAL_PATH);

    // Open API should route to the backend /v1/ root and not bounce to SSO.
    await gotoWithRetry(page, PORTAL_PATH);
    await page.getByRole('link', { name: /Open API/i }).click();
    await page.waitForLoadState('domcontentloaded');
    // We should not be bounced back into the SSO flow when opening the API.
    expect(isSSOLoginUrl(page.url())).toBeFalsy();

    // Fetch the OpenAPI document via the authenticated backend and validate
    // its structure. This avoids depending on any nginx redirect behaviour
    // while still ensuring the spec is actually served.
    const openapiResp = await apiRequestWithRetry(page, 'get', '/openapi.yaml');
    expect(openapiResp.status()).toBe(200);
    const openapiBody = await openapiResp.text();
    expect(openapiBody ?? '').toContain('openapi: 3.0.3');
    expect(openapiBody ?? '').toContain('AI CV Evaluator API');
    // Key documented paths should be present in the OpenAPI document so that
    // clients can discover the public and admin APIs.
    expect(openapiBody ?? '').toContain('/v1/upload:');
    expect(openapiBody ?? '').toContain('/v1/evaluate:');
    expect(openapiBody ?? '').toContain('/v1/result/{id}:');
    expect(openapiBody ?? '').toContain('/admin/api/stats:');
    expect(openapiBody ?? '').toContain('/admin/api/jobs:');
    expect(openapiBody ?? '').toContain('/admin/api/jobs/{id}:');

    // A clearly missing backend path should still return 404 (backend 404 semantics).
    const missingResp = await page.request.get('/v1/__nonexistent');
    expect(missingResp.status()).toBe(404);

    // Health link should return the JSON health payload from /healthz.
    await gotoWithRetry(page, PORTAL_PATH);
    await page.getByRole('link', { name: /Health/i }).click();
    await page.waitForLoadState('domcontentloaded');
    const healthBody = await page.textContent('body');
    expect(healthBody ?? '').toContain('"status":"healthy"');
    expect(healthBody ?? '').toContain('"checks"');
    const healthJson = JSON.parse(healthBody ?? '{}') as any;
    expect(healthJson.status).toBe('healthy');
    expect(typeof healthJson.timestamp).toBe('string');
    expect(healthJson.version).toBe('1.0.0');
    expect(Array.isArray(healthJson.checks)).toBeTruthy();
    const healthNames = (healthJson.checks as any[]).map((c) => c.name).sort();
    expect(healthNames).toEqual(
      expect.arrayContaining(['database', 'qdrant', 'tika', 'application', 'system']),
    );
    const unhealthyHealth = (healthJson.checks as any[]).filter((c) => c.ok === false);
    expect(unhealthyHealth.length).toBe(0);

    // Readyz endpoint should report all backing services as ready.
    const readyResp = await page.request.get('/readyz');
    expect(readyResp.status()).toBe(200);
    const readyJson = (await readyResp.json()) as any;
    expect(Array.isArray(readyJson.checks)).toBeTruthy();
    const readyNames = (readyJson.checks as any[]).map((c) => c.name).sort();
    expect(readyNames).toEqual(expect.arrayContaining(['db', 'qdrant', 'tika']));
    const notReady = (readyJson.checks as any[]).filter((c) => c.ok === false);
    expect(notReady.length).toBe(0);
  });
};
