import { test, expect } from '@playwright/test';

import { PORTAL_PATH } from '../helpers/env.ts';
import { apiRequestWithRetry } from '../helpers/api.ts';
import { performApiLogin } from '../helpers/authelia.ts';
import { gotoWithRetry } from '../helpers/navigation.ts';
import { isSSOLoginUrl } from '../helpers/sso.ts';

export const registerPortalBackendApiLinksTests = (): void => {
  test('portal Backend API links work after SSO login', async ({ page, baseURL: _baseURL }) => {
    // Use API Login Bypass for robustness in CI
    await performApiLogin(page);
    await gotoWithRetry(page, PORTAL_PATH);

    // Return to the portal and verify the Backend API links are present and correct.
    await gotoWithRetry(page, PORTAL_PATH);
    expect(!isSSOLoginUrl(page.url())).toBeTruthy();

    const openApiLink = page.getByRole('link', { name: /Open API/i });
    await expect(openApiLink).toBeVisible();
    await expect(openApiLink).toHaveAttribute('href', '/openapi.yaml');

    const openapiResp = await apiRequestWithRetry(page, 'get', '/openapi.yaml');
    expect(openapiResp.ok()).toBeTruthy();
    const openapiContentType = openapiResp.headers()['content-type'] ?? '';
    expect(openapiContentType).toContain('application/yaml');
    const openapiBody = await openapiResp.text();
    expect(openapiBody).toContain('openapi:');

    const healthLink = page.getByRole('link', { name: /Health/i });
    await expect(healthLink).toBeVisible();
    await expect(healthLink).toHaveAttribute('href', '/healthz');

    const healthResp = await page.request.get('/healthz');
    expect(healthResp.ok()).toBeTruthy();
  });
};
