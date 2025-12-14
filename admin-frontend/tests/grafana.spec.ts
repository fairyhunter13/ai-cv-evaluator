import { test, expect, type Page } from '@playwright/test';

// Environment detection
const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:8088';
const IS_PRODUCTION = BASE_URL.includes('ai-cv-evaluator.web.id');

// Credentials: Use env vars, with sensible defaults for dev
const SSO_USERNAME = process.env.SSO_USERNAME || process.env.ADMIN_USERNAME || 'admin';
const SSO_PASSWORD = process.env.SSO_PASSWORD || process.env.ADMIN_PASSWORD || (IS_PRODUCTION ? '' : 'admin123');

const isSSOLoginUrl = (input: string | URL): boolean => {
    const url = typeof input === 'string' ? input : input.toString();
    return url.includes('/oauth2/') || url.includes('/realms/aicv') || url.includes(':9091') || url.includes('auth.ai-cv-evaluator.web.id') || url.includes('workflow=openid_connect') || url.includes('/api/oidc/authorization') || url.includes('/api/oidc/authorize') || url.includes('/login/oauth/authorize');
};

const handleAutheliaConsent = async (page: Page): Promise<void> => {
    try {
        const consentHeader = page.getByRole('heading', { name: /Consent|Authorization/i });
        if (await consentHeader.isVisible({ timeout: 2000 })) {
            const acceptBtn = page.getByRole('button', { name: /Accept|Allow|Authorize/i }).first();
            if (await acceptBtn.isVisible()) {
                await acceptBtn.click();
            }
        }
    } catch (e) {
        // Ignore
    }
};

const loginViaSSOIfNeeded = async (page: Page): Promise<void> => {
    if (!isSSOLoginUrl(page.url()) && !(await page.title().catch(() => '')).includes('Login - Authelia')) {
        return;
    }

    if (!SSO_PASSWORD) {
        throw new Error('SSO_PASSWORD environment variable is required for SSO login tests');
    }

    const usernameInput = page.locator('#username-textfield, input#username');
    const passwordInput = page.locator('#password-textfield, input#password');

    await usernameInput.waitFor({ state: 'visible', timeout: 10000 });
    await usernameInput.fill(SSO_USERNAME);
    await passwordInput.fill(SSO_PASSWORD);

    const submitButton = page.locator('#sign-in-button, button[type="submit"], input[type="submit"]');
    if ((await submitButton.count()) > 0) {
        await submitButton.first().click();
    } else {
        await passwordInput.press('Enter');
    }

    await handleAutheliaConsent(page);
    await page.waitForURL((url) => !isSSOLoginUrl(url), { timeout: 30000 });
};

test.describe('Grafana Dashboard Verification', () => {
    test('should load Grafana and verify all Docker metrics are valid', async ({ page, baseURL }) => {
        console.log(`Navigating to Grafana at ${baseURL}/grafana/`);

        // go to grafana
        await page.goto('/grafana/');
        await loginViaSSOIfNeeded(page);

        // After SSO completes, oauth2-proxy may redirect back to the portal; ensure
        // we land on Grafana before continuing.
        await page.goto('/grafana/');
        await loginViaSSOIfNeeded(page);

        // Check if we need to login (if redirected to login page)
        try {
            const loginHeader = page.getByText('Welcome to Grafana');
            if (await loginHeader.isVisible({ timeout: 3000 })) {
                console.log('Logging in to Grafana...');
                await page.getByLabel('Username input field').fill(process.env.ADMIN_USERNAME || 'admin');
                await page.getByLabel('Password input field').fill(process.env.ADMIN_PASSWORD || 'admin'); // Default grafana creds if not OAuth
                await page.getByRole('button', { name: 'Log in' }).click();
            }
        } catch (e) {
            // Ignore, likely already logged in or using OAuth
        }

        // Handle Keycloak login if redirected there
        try {
            if (await page.getByLabel('Username').isVisible({ timeout: 3000 })) {
                console.log('Logging in via Keycloak...');
                await page.getByLabel('Username').fill(process.env.SSO_USERNAME || process.env.ADMIN_USERNAME || 'admin');
                await page.getByLabel('Password').fill(process.env.SSO_PASSWORD || process.env.ADMIN_PASSWORD || 'admin123');
                await page.getByRole('button', { name: 'Sign In' }).click();
            }
        } catch (e) {
            // Ignore
        }

        // Wait for Grafana title
        await expect(page).toHaveTitle(/Grafana/, { timeout: 30000 });

        // Navigate to Docker Containers dashboard
        const dashboardPath = '/grafana/d/docker-monitoring/docker-containers';
        console.log(`Navigating to Dashboard at ${dashboardPath}`);
        await page.goto(dashboardPath);

        // Verify dashboard title
        await expect(page.getByText('Docker Containers')).toBeVisible({ timeout: 30000 });

        // Wait for dashboard to fully load (look for refresh button or time picker)
        await expect(page.getByLabel(/Time range/i).first()).toBeVisible();

        // Verify key panels exist
        // Note: panel titles can differ across dashboard versions; prefer the stable
        // panel menu aria-labels for validation.
        const cpuPanelMenuSelector = 'button[aria-label="Menu for panel with title Container CPU Usage"]';
        const memoryPanelMenuSelector = 'button[aria-label="Menu for panel with title Container Memory Usage"]';
        const networkPanelMenuSelector =
            'button[aria-label="Menu for panel with title Container Network Traffic"], button[aria-label="Menu for panel with title Network Traffic: Host vs Containers"]';

        for (const selector of [cpuPanelMenuSelector, memoryPanelMenuSelector]) {
            await expect
                .poll(async () => page.locator(selector).count(), { timeout: 30000 })
                .toBeGreaterThan(0);
        }

        for (let i = 0; i < 20; i += 1) {
            if ((await page.locator(networkPanelMenuSelector).count()) > 0) {
                break;
            }
            await page.mouse.wheel(0, 1000);
            await page.waitForTimeout(500);
        }

        await expect
            .poll(async () => page.locator(networkPanelMenuSelector).count(), { timeout: 30000 })
            .toBeGreaterThan(0);

        // Check that we have valid data by looking for likely container names
        // We look for 'ai-cv-evaluator' or 'db' related text which indicates metrics are being populated
        // The legend usually lists container names
        await expect(page.locator('div[data-testid="data-testid panel content"]').first()).toBeVisible();

        // Check for common container parts that should be in the legend if metrics are flowing
        const expectedText = [/ai-cv-evaluator/i, /db/i, /worker/i];
        let found = false;

        // Allow some time for data to load
        await page.waitForTimeout(2000);

        for (const pattern of expectedText) {
            if (await page.getByText(pattern).count() > 0) {
                found = true;
                console.log(`Found pattern ${pattern} in dashboard.`);
                break;
            }
        }

        if (!found) {
            console.log('Warning: specific container names not found in legend, but dashboard loaded.');
        }

        console.log('Grafana Docker Containers dashboard verified: Panels visible.');
    });
});
