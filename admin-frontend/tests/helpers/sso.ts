import type { Page } from '@playwright/test';

export const isSSOLoginUrl = (input: string | URL): boolean => {
  const url = typeof input === 'string' ? input : input.toString();
  return url.includes('/oauth2/') ||
    url.includes('/realms/aicv') ||
    url.includes(':9091') ||
    url.includes('/api/oidc/authorization') ||
    url.includes('/api/oidc/authorize') ||
    url.includes('/login/oauth/authorize') ||
    url.includes('auth.ai-cv-evaluator.web.id') ||
    url.includes('workflow=openid_connect');
};

export const isAutheliaOneTimePasswordUrl = (input: string | URL): boolean => {
  const url = typeof input === 'string' ? input : input.toString();
  return url.includes('/2fa/one-time-password');
};

export const assertNotAutheliaOneTimePasswordUrl = (input: string | URL): void => {
  if (!isAutheliaOneTimePasswordUrl(input)) {
    return;
  }

  throw new Error(
    'Authelia requires two-factor authentication (one-time password). This Playwright suite only supports username/password and intentionally does not bypass 2FA. Disable 2FA for the test user or adjust access control policy.',
  );
};

export const waitForNotSSOLoginUrl = async (
  page: Page,
  isLoginUrl: (input: string | URL) => boolean,
  timeoutMs = 30000,
): Promise<void> => {
  const start = Date.now();

  while (Date.now() - start < timeoutMs) {
    const url = page.url();
    assertNotAutheliaOneTimePasswordUrl(url);
    if (!isLoginUrl(url)) {
      return;
    }
    await page.waitForTimeout(250);
  }

  const finalUrl = page.url();
  assertNotAutheliaOneTimePasswordUrl(finalUrl);
  throw new Error(`Timed out waiting for SSO login flow to complete; still on ${finalUrl}`);
};

export const handleAutheliaConsent = async (page: Page): Promise<void> => {
  // Authelia v4.37/v4.38 consent page handling
  try {
    const consentHeader = page.getByRole('heading', { name: /Consent|Authorization/i });
    if (await consentHeader.isVisible({ timeout: 2000 })) {
      const acceptBtn = page.getByRole('button', { name: /Accept|Allow|Authorize/i }).first();
      if (await acceptBtn.isVisible()) {
        await acceptBtn.click();
      }
    }
  } catch (_error) {
    // Ignore error
  }
};

export const completeKeycloakProfileUpdate = async (page: Page): Promise<void> => {
  const heading = page.getByRole('heading', { name: /Update Account Information/i });
  const visible = await heading.isVisible().catch(() => false);
  if (!visible) {
    return;
  }

  const firstNameInput = page.getByRole('textbox', { name: /First name/i });
  const lastNameInput = page.getByRole('textbox', { name: /Last name/i });

  if (await firstNameInput.isVisible().catch(() => false)) {
    await firstNameInput.fill('Admin');
  }
  if (await lastNameInput.isVisible().catch(() => false)) {
    await lastNameInput.fill('User');
  }

  const submitProfileButton = page.getByRole('button', { name: /submit/i });
  if (await submitProfileButton.isVisible().catch(() => false)) {
    await submitProfileButton.click();
  }
};
