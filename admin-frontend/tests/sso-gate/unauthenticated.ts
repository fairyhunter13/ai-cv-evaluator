import { test, expect } from '@playwright/test';

import { PROTECTED_PATHS, DEV_ONLY_SERVICES, IS_DEV } from '../helpers/env.ts';
import { gotoWithRetry } from '../helpers/navigation.ts';
import { isSSOLoginUrl } from '../helpers/sso.ts';

export const registerUnauthenticatedAccessTests = (): void => {
  // Unauthenticated users should always be driven into the SSO flow
  // when trying to hit any protected path directly.
  // Filter out dev-only services (like Redpanda Console) in production
  const pathsToTest = PROTECTED_PATHS.filter(
    (path) => IS_DEV || !DEV_ONLY_SERVICES.includes(path)
  );

  for (const path of pathsToTest) {
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
