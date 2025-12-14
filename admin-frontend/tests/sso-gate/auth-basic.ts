import { test, expect } from '@playwright/test';

import { AUTHELIA_URL, PORTAL_PATH } from '../helpers/env.ts';
import { ensureAutheliaUp, performApiLogin } from '../helpers/authelia.ts';
import { gotoWithRetry } from '../helpers/navigation.ts';
import { isSSOLoginUrl } from '../helpers/sso.ts';

export const registerAuthBasicTests = (): void => {
  test('invalid credentials show error', async ({ page, baseURL: _baseURL }) => {
    await ensureAutheliaUp(page);
    await gotoWithRetry(page, PORTAL_PATH);

    // API Login with bad creds
    const loginResp = await page.request.post(`${AUTHELIA_URL}/api/firstfactor`, {
      data: { username: 'admin', password: 'wrongpassword' },
      headers: { 'Content-Type': 'application/json' }
    });

    expect(loginResp.status()).toBe(401);
  });

  test('clearing cookies forces re-authentication', async ({ page }) => {
    await performApiLogin(page);
    await gotoWithRetry(page, PORTAL_PATH);

    expect(!isSSOLoginUrl(page.url())).toBeTruthy();

    await page.context().clearCookies();

    await gotoWithRetry(page, '/app/');
    expect(isSSOLoginUrl(page.url())).toBeTruthy();
  });

  test('oauth2 endpoints tolerate burst traffic', async ({ page }) => {
    const requestOauth2StartStatus = async (): Promise<number> => {
      const maxAttempts = 2;
      for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
        try {
          const resp = await page.request.get('/oauth2/start', {
            maxRedirects: 0,
            failOnStatusCode: false,
          });
          return resp.status();
        } catch {
          if (attempt === maxAttempts) return 0;
          await page.waitForTimeout(200);
        }
      }
      return 0;
    };

    let initialStatus = 0;
    for (let attempt = 1; attempt <= 10; attempt += 1) {
      initialStatus = await requestOauth2StartStatus();
      if ([302, 429].includes(initialStatus)) {
        break;
      }
      await page.waitForTimeout(1000);
    }

    expect([302, 429]).toContain(initialStatus);

    const burst = 40;
    const codes: number[] = [];
    for (let i = 0; i < burst; i += 1) {
      codes.push(await requestOauth2StartStatus());
    }
    expect(codes.every((c) => c === 302 || c === 429)).toBeTruthy();
  });
};
