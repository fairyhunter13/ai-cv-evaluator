import { test, expect } from '@playwright/test';

import { PORTAL_PATH } from '../helpers/env.ts';
import { ensureAutheliaUp, performApiLogin } from '../helpers/authelia.ts';
import { gotoWithRetry } from '../helpers/navigation.ts';
import {
  completeProfileUpdateIfNeeded,
  handleAutheliaConsent,
  isSSOLoginUrl,
  waitForNotSSOLoginUrl,
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

    await performApiLogin(page);
    await gotoWithRetry(page, PORTAL_PATH);

    await handleAutheliaConsent(page);
    await completeProfileUpdateIfNeeded(page);

    // Wait until we have returned from the SSO flow (no longer on oauth2-proxy
    // or OIDC provider URLs) so that the oauth2-proxy session cookie is set.
    await waitForNotSSOLoginUrl(page, isSSOLoginUrl, 30000);
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
