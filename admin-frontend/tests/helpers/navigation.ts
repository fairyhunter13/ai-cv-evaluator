import type { Page } from '@playwright/test';

import { assertNotAutheliaOneTimePasswordUrl } from './sso.ts';

export const gotoWithRetry = async (page: Page, path: string): Promise<void> => {
  const maxAttempts = 10;
  const retryDelayMs = 3000;

  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    try {
      await page.goto(path, { waitUntil: 'domcontentloaded', timeout: 30000 });
      assertNotAutheliaOneTimePasswordUrl(page.url());
      return;
    } catch (err) {
      const message = String(err);
      const transientErrors = [
        'net::ERR_CONNECTION_REFUSED',
        'net::ERR_SOCKET_NOT_CONNECTED',
        'net::ERR_CONNECTION_RESET',
        'net::ERR_EMPTY_RESPONSE',
      ];

      // Only retry on transient connection-refused errors; propagate
      // everything else immediately so we still fail fast on real issues.
      if (!transientErrors.some((pattern) => message.includes(pattern)) || attempt === maxAttempts) {
        throw err;
      }

      // Wait before retrying to give the server time to become available.
      await page.waitForTimeout(retryDelayMs);
    }
  }
};
