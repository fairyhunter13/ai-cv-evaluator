import type { Page } from '@playwright/test';

import { AUTHELIA_URL, IS_PRODUCTION, SSO_PASSWORD, SSO_USERNAME } from './env.ts';

export const ensureAutheliaUp = async (page: Page): Promise<void> => {
  console.log(`[AutheliaDebug] Waiting for Authelia to be healthy at ${AUTHELIA_URL}...`);
  // Increase timeout to 60s (30 * 2s) for slower CI runners
  for (let i = 0; i < 30; i++) {
    try {
      const resp = await page.request.get(`${AUTHELIA_URL}/api/health`);
      if (resp.ok()) {
        const json = await resp.json();
        if (json.status === 'OK') {
          console.log('[AutheliaDebug] Authelia is healthy.');
          return;
        }
      }
    } catch (_e) {
      // ignore connection errors
    }
    await page.waitForTimeout(2000);
  }
  throw new Error(`Authelia failed to become healthy at ${AUTHELIA_URL} within 60s`);
};

/**
 * Performs API Login to Authelia and injects the session cookie.
 * This bypasses the flaky UI login flow for robustness in CI.
 */
export const performApiLogin = async (page: Page): Promise<void> => {
  await ensureAutheliaUp(page);

  const loginResp = await page.request.post(`${AUTHELIA_URL}/api/firstfactor`, {
    data: { username: SSO_USERNAME, password: SSO_PASSWORD },
    headers: { 'Content-Type': 'application/json' }
  });

  if (loginResp.ok()) {
    const headers = loginResp.headers();
    const setCookie = headers['set-cookie'];
    if (setCookie) {
      const sessionMatch = setCookie.match(/authelia_session=([^;]+)/);
      if (sessionMatch) {
        const cookie: any = {
          name: 'authelia_session',
          value: sessionMatch[1],
          httpOnly: true,
          secure: IS_PRODUCTION,
          sameSite: 'Lax'
        };

        // Playwright req: Either url OR (domain + path)
        if (IS_PRODUCTION) {
          cookie.domain = 'ai-cv-evaluator.web.id';
          cookie.path = '/';
        } else {
          // For localhost, use URL to establish Host-Only cookie
          cookie.url = AUTHELIA_URL;
        }

        await page.context().addCookies([cookie]);
        console.log(`[AutheliaDebug] Injected session cookie: ${JSON.stringify(cookie)}`);
      }
    }
  } else {
    throw new Error(`API Login failed with status ${loginResp.status()}`);
  }
};
