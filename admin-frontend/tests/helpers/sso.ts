import type { Page } from '@playwright/test';

export const isSSOLoginUrl = (input: string | URL): boolean => {
  const url = typeof input === 'string' ? input : input.toString();
  return url.includes('/oauth2/') || url.includes('/realms/aicv') || url.includes(':9091') || url.includes('/api/oidc/authorization') || url.includes('/login/oauth/authorize');
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
