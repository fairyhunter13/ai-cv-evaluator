import { test, expect } from '@playwright/test';

import { PROTECTED_PATHS } from '../helpers/env.ts';
import { gotoWithRetry } from '../helpers/navigation.ts';
import { isSSOLoginUrl } from '../helpers/sso.ts';

export const registerUnauthenticatedAccessTests = (): void => {
  // Unauthenticated users should always be driven into the SSO flow
  // when trying to hit any protected path directly.
  for (const path of PROTECTED_PATHS) {
    test(`unauthenticated access to ${path} is redirected to SSO`, async ({ page, baseURL: _baseURL }) => {
      await gotoWithRetry(page, path);
      const finalUrl = page.url();

      expect(
        isSSOLoginUrl(finalUrl),
        `Expected unauthenticated navigation to ${path} to end on SSO login, got ${finalUrl}`,
      ).toBeTruthy();
    });
  }
};
