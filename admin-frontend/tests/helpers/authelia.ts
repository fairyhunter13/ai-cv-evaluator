import type { Page } from '@playwright/test';

import { AUTHELIA_URL, IS_PRODUCTION, SSO_PASSWORD, SSO_USERNAME } from './env.ts';

const AUTHELIA_LOGIN_MIN_INTERVAL_MS = 2000;
const AUTHELIA_LOGIN_MAX_BACKOFF_MS = 30000;

let nextAutheliaLoginAllowedAtMs = 0;
let consecutiveAutheliaLoginFailures = 0;

export const waitForAutheliaLoginRateLimit = async (page: Page): Promise<void> => {
  const now = Date.now();
  const waitMs = Math.max(0, nextAutheliaLoginAllowedAtMs - now);
  if (waitMs > 0) {
    await page.waitForTimeout(waitMs);
  }
};

export const recordAutheliaLoginAttempt = (success: boolean): void => {
  const now = Date.now();

  if (success) {
    consecutiveAutheliaLoginFailures = 0;
    nextAutheliaLoginAllowedAtMs = now + AUTHELIA_LOGIN_MIN_INTERVAL_MS;
    return;
  }

  consecutiveAutheliaLoginFailures += 1;
  const exp = Math.min(Math.max(consecutiveAutheliaLoginFailures - 1, 0), 6);
  const backoffMs = Math.min(
    AUTHELIA_LOGIN_MAX_BACKOFF_MS,
    AUTHELIA_LOGIN_MIN_INTERVAL_MS * Math.pow(2, exp),
  );
  nextAutheliaLoginAllowedAtMs = now + Math.max(AUTHELIA_LOGIN_MIN_INTERVAL_MS, backoffMs);
};

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

  if (!SSO_PASSWORD) {
    throw new Error('SSO_PASSWORD environment variable is required for SSO login tests');
  }

  const maxAttempts = 3;
  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    await waitForAutheliaLoginRateLimit(page);

    let loginResp;
    try {
      loginResp = await page.request.post(`${AUTHELIA_URL}/api/firstfactor`, {
        data: { username: SSO_USERNAME, password: SSO_PASSWORD },
        headers: { 'Content-Type': 'application/json' },
      });
    } catch (error) {
      recordAutheliaLoginAttempt(false);
      if (attempt === maxAttempts) {
        throw error;
      }
      continue;
    }

    if (!loginResp.ok()) {
      const status = loginResp.status();
      recordAutheliaLoginAttempt(false);

      if (attempt < maxAttempts && [429, 502, 503, 504].includes(status)) {
        continue;
      }

      throw new Error(`API Login failed with status ${status}`);
    }

    const headers = loginResp.headers();
    const setCookie = headers['set-cookie'];
    if (!setCookie) {
      recordAutheliaLoginAttempt(false);
      throw new Error('API Login did not return a set-cookie header');
    }

    const sessionMatch = setCookie.match(/authelia_session=([^;]+)/);
    if (!sessionMatch) {
      recordAutheliaLoginAttempt(false);
      throw new Error('API Login did not return an authelia_session cookie');
    }

    const cookie: any = {
      name: 'authelia_session',
      value: sessionMatch[1],
      httpOnly: true,
      secure: IS_PRODUCTION,
      sameSite: 'Lax',
    };

    // Playwright req: Either url OR (domain + path)
    // Use URL to establish a Host-Only cookie for Authelia's domain
    cookie.url = AUTHELIA_URL;

    await page.context().addCookies([cookie]);
    console.log(`[AutheliaDebug] Injected session cookie: ${JSON.stringify(cookie)}`);
    recordAutheliaLoginAttempt(true);
    return;
  }

  throw new Error('API Login failed after retries');
};
