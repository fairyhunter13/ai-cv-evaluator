import { test, expect } from '@playwright/test';

import { AUTHELIA_URL, PORTAL_PATH } from '../helpers/env.ts';
import { performApiLogin } from '../helpers/authelia.ts';
import { gotoWithRetry } from '../helpers/navigation.ts';
import { isSSOLoginUrl } from '../helpers/sso.ts';

export const registerLogoutTests = (): void => {
  test.skip('logout clears session', async ({ page, baseURL }) => {
    // Rely on previous login state or login again? Playwright tests are isolated primarily.
    // Need to login first.
    await performApiLogin(page);
    await gotoWithRetry(page, PORTAL_PATH);
    await expect(page).toHaveTitle(/Portal/i);

    // Perform Logout via OAuth2 Proxy to clear proxy cookies AND redirect to Authelia logout
    // We assume oauth2-proxy is mounted at /oauth2/
    await page.goto(`${baseURL}/oauth2/sign_out?rd=${encodeURIComponent(AUTHELIA_URL + '/logout')}`);

    // Verify redirect to Login (Authelia)
    await expect(page).toHaveURL(/.*9091.*/);

    // Verify accessing portal again redirects to SSO
    await page.goto(PORTAL_PATH);
    expect(isSSOLoginUrl(page.url())).toBeTruthy();
  });

  test.skip('logout flow redirects to login page', async ({ page, baseURL: _baseURL }) => {
    // Navigate to Portal
    await performApiLogin(page);

    await gotoWithRetry(page, PORTAL_PATH);

    console.log(`URL after Login Bypass: ${page.url()}`);
    console.log(`Title after Login Bypass: ${await page.title()}`);

    // Trigger Logout via Portal UI (to utilize the ?rd=/logout chaining)
    const logoutButton = page.locator('a[href*="/oauth2/sign_out"]');
    await logoutButton.waitFor({ state: 'visible', timeout: 5000 });
    await logoutButton.click();

    // Verify we are redirected back to the Login Page OR the Sign Out confirmation (depending on proxy config)
    await page.waitForTimeout(5000);
    const url = page.url();
    // We accept: Authelia Login (9091), /logout, or /oauth2/sign_out (proxy confirmation)
    expect(isSSOLoginUrl(url) || url.includes('/logout') || url.includes('/sign_out')).toBeTruthy();
  });
};
