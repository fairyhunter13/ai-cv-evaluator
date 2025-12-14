import { test, expect } from '@playwright/test';

import {
  AUTHELIA_URL,
  IS_PRODUCTION,
  PORTAL_PATH,
  SSO_PASSWORD,
  SSO_USERNAME,
} from '../helpers/env.ts';
import { ensureAutheliaUp } from '../helpers/authelia.ts';
import { gotoWithRetry } from '../helpers/navigation.ts';
import {
  completeKeycloakProfileUpdate,
  handleAutheliaConsent,
  isSSOLoginUrl,
} from '../helpers/sso.ts';

export const registerPortalLoginTests = (): void => {
  // Happy path: log in via SSO once, land on the portal, then access dashboards
  // without seeing the login page again.
  test('single sign-on via portal allows access to dashboards', async ({ page, baseURL: _baseURL }) => {

    // Ensure Authelia is up before doing anything to avoid race conditions in CI
    await ensureAutheliaUp(page);

    // Start at portal; unauthenticated users should be redirected to SSO login
    await gotoWithRetry(page, PORTAL_PATH);

    expect(isSSOLoginUrl(page.url())).toBeTruthy();

    const usernameInput = page.locator('input#username');
    const passwordInput = page.locator('input#password');

    // Attempt API login to bypass potential UI flakiness (Authelia v4.37)
    const loginResp = await page.request.post(`${AUTHELIA_URL}/api/firstfactor`, {
      data: { username: SSO_USERNAME, password: SSO_PASSWORD },
      headers: { 'Content-Type': 'application/json' }
    });

    if (loginResp.ok()) {
      // EXTRACT COOKIE and force it to be non-secure for localhost HTTP testing
      const headers = loginResp.headers();
      const setCookie = headers['set-cookie'];

      if (setCookie) {
        const sessionMatch = setCookie.match(/authelia_session=([^;]+)/);
        if (sessionMatch) {
          const sessionValue = sessionMatch[1];
          await page.context().addCookies([{
            name: 'authelia_session',
            value: sessionValue,
            domain: IS_PRODUCTION ? 'ai-cv-evaluator.web.id' : 'localhost',
            path: '/',
            httpOnly: true,
            secure: IS_PRODUCTION,
            sameSite: 'Lax'
          }]);
        }
      }

      await page.goto(PORTAL_PATH);
    } else {
      if (await usernameInput.isVisible()) {
        await usernameInput.fill(SSO_USERNAME);
        await passwordInput.fill(SSO_PASSWORD);
        await passwordInput.press('Enter');
      }
      await page.waitForTimeout(2000);
    }

    await handleAutheliaConsent(page);
    await completeKeycloakProfileUpdate(page);

    // Wait until we have returned from the SSO flow (no longer on oauth2-proxy
    // or Keycloak realm URLs) so that the oauth2-proxy session cookie is set.
    await page.waitForURL((url) => !isSSOLoginUrl(url), { timeout: 30000 });
    // After successful login (and any required profile update), navigate directly
    // to representative dashboards and assert we do not get bounced back to SSO.
    for (const path of ['/app/', '/prometheus/']) {
      await gotoWithRetry(page, path);
      const url = page.url();
      expect(
        !isSSOLoginUrl(url),
        `Expected authenticated navigation to ${path} to stay on service, got ${url}`,
      ).toBeTruthy();
    }
  });
};
