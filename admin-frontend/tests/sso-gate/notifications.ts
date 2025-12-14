import { test, expect } from '@playwright/test';

import { IS_DEV, PORTAL_PATH } from '../helpers/env.ts';
import { performApiLogin } from '../helpers/authelia.ts';
import { clearMailpitMessages } from '../helpers/mailpit.ts';
import { gotoWithRetry } from '../helpers/navigation.ts';
import { isSSOLoginUrl } from '../helpers/sso.ts';

export const registerNotificationAndEmailTests = (): void => {
  test('email notification dashboard reachable after SSO login', async ({ page, baseURL: _baseURL }) => {
    // Use API Login Bypass for robustness in CI
    await performApiLogin(page);
    await gotoWithRetry(page, PORTAL_PATH);

    if (IS_DEV) {
      // In dev, test Mailpit dashboard
      await clearMailpitMessages(page);

      await gotoWithRetry(page, PORTAL_PATH);
      await page.getByRole('link', { name: /Mailpit/i }).click();
      await page.waitForLoadState('domcontentloaded');
      expect(isSSOLoginUrl(page.url())).toBeFalsy();
      const mailpitTitle = (await page.title()).toLowerCase();
      expect(mailpitTitle).toContain('mailpit');
    } else {
      // In prod, test Grafana alerting notifications
      await gotoWithRetry(page, '/grafana/alerting/notifications');
      await page.waitForLoadState('networkidle');
      expect(isSSOLoginUrl(page.url())).toBeFalsy();
      await expect(page).toHaveTitle(/Grafana/i, { timeout: 15000 });
    }
  });

  test('email service requires SSO login', async ({ browser, baseURL: _baseURL }) => {
    // Use a fresh browser context without any cookies to simulate unauthenticated access.
    const freshContext = await browser.newContext();
    const freshPage = await freshContext.newPage();

    try {
      if (IS_DEV) {
        // In dev, test Mailpit SSO protection
        await gotoWithRetry(freshPage, '/mailpit/');
        expect(isSSOLoginUrl(freshPage.url())).toBeTruthy();

        // Direct API access should redirect to SSO
        const apiResp = await freshPage.request.get('/mailpit/api/v1/messages');
        const isRedirectedToSSO = isSSOLoginUrl(apiResp.url());
        const isNon200 = apiResp.status() !== 200;
        expect(isRedirectedToSSO || isNon200).toBeTruthy();
      } else {
        // In prod, test Grafana alerting SSO protection
        await gotoWithRetry(freshPage, '/grafana/alerting/notifications');
        expect(isSSOLoginUrl(freshPage.url())).toBeTruthy();
      }
    } finally {
      await freshContext.close();
    }
  });
};
