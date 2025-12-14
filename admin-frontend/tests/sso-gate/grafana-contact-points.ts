import { test, expect } from '@playwright/test';

import { PORTAL_PATH } from '../helpers/env.ts';
import { performApiLogin } from '../helpers/authelia.ts';
import { gotoWithRetry } from '../helpers/navigation.ts';
import { isSSOLoginUrl } from '../helpers/sso.ts';

export const registerGrafanaContactPointsTests = (): void => {
  test('grafana contact points are accessible', async ({ page, baseURL: _baseURL }) => {
    // Use API Login Bypass for robustness in CI
    await performApiLogin(page);
    await gotoWithRetry(page, PORTAL_PATH);

    // Navigate to Grafana alerting notifications page
    await gotoWithRetry(page, '/grafana/alerting/notifications');
    await page.waitForLoadState('networkidle');

    // Verify we're on Grafana and not redirected to SSO
    expect(isSSOLoginUrl(page.url())).toBeFalsy();
    await expect(page).toHaveTitle(/Grafana/i, { timeout: 15000 });

    // Page should have contact point content
    const pageContent = await page.locator('body').textContent();
    expect(pageContent?.length).toBeGreaterThan(100);
  });
};
