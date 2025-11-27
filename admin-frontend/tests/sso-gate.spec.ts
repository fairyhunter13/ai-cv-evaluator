import { test, expect } from '@playwright/test';

const PORTAL_PATH = '/';
const PROTECTED_PATHS = ['/app/', '/grafana/', '/prometheus/', '/jaeger/', '/redpanda/', '/admin/'];

const isSSOLoginUrl = (url: string): boolean => {
  return url.includes('/oauth2/') || url.includes('/realms/aicv');
};

// Unauthenticated users should always be driven into the SSO flow
// when trying to hit any protected path directly.
for (const path of PROTECTED_PATHS) {
  test(`unauthenticated access to ${path} is redirected to SSO`, async ({ page, baseURL }) => {
    test.skip(!baseURL, 'Base URL must be configured');

    await page.goto(path, { waitUntil: 'domcontentloaded' });
    const finalUrl = page.url();

    expect(
      isSSOLoginUrl(finalUrl),
      `Expected unauthenticated navigation to ${path} to end on SSO login, got ${finalUrl}`,
    ).toBeTruthy();
  });
}

// Happy path: log in via SSO once, land on the portal, then access dashboards
// without seeing the login page again.
test('single sign-on via portal allows access to dashboards', async ({ page, baseURL }) => {
  test.skip(!baseURL, 'Base URL must be configured');

  // Start at portal; unauthenticated users should be redirected to SSO login
  await page.goto(PORTAL_PATH, { waitUntil: 'domcontentloaded' });

  // We expect to be on Keycloak login page
  await expect(page).toHaveURL(/(oauth2|realms\/aicv)/);

  // Try default dev credentials from realm-aicv.dev.json
  const usernameInput = page.locator('input#username');
  const passwordInput = page.locator('input#password');

  if (await usernameInput.isVisible()) {
    await usernameInput.fill('admin');
    await passwordInput.fill('admin123');

    // Keycloak 25 uses a submit button with name or text containing "Sign in"
    const submitButton = page.locator('button[type="submit"], input[type="submit"]');
    await submitButton.first().click();
  }

  // After successful login, we should eventually land on the portal root
  await page.waitForURL(/\/$/);

  // Now navigate to a couple of dashboards and assert we do not get bounced back to SSO
  for (const path of ['/app/', '/grafana/', '/prometheus/']) {
    await page.goto(path, { waitUntil: 'domcontentloaded' });
    const url = page.url();
    expect(
      !isSSOLoginUrl(url),
      `Expected authenticated navigation to ${path} to stay on service, got ${url}`,
    ).toBeTruthy();
  }

  // Logout should invalidate SSO session
  await page.goto('/logout');
  await page.waitForLoadState('domcontentloaded');

  // After logout, accessing a protected path should again send us to SSO login
  await page.goto('/app/', { waitUntil: 'domcontentloaded' });
  expect(isSSOLoginUrl(page.url())).toBeTruthy();
});
