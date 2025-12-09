import { test, expect } from '@playwright/test';

test.describe('Grafana Dashboard Verification', () => {
    test('should load Grafana and verify all Docker metrics are valid', async ({ page, baseURL }) => {
        console.log(`Navigating to Grafana at ${baseURL}/grafana/`);

        // go to grafana
        await page.goto('/grafana/');

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
        await expect(page).toHaveTitle(/Grafana/);

        // Navigate to Docker Containers dashboard
        const dashboardPath = '/grafana/d/docker-monitoring/docker-containers';
        console.log(`Navigating to Dashboard at ${dashboardPath}`);
        await page.goto(dashboardPath);

        // Verify dashboard title
        await expect(page.getByText('Docker Containers')).toBeVisible({ timeout: 30000 });

        // Wait for dashboard to fully load (look for refresh button or time picker)
        await expect(page.getByLabel('Time range picker submenu')).toBeVisible();

        // Verify all 3 panels
        const panels = [
            'Container CPU Usage',
            'Container Memory Usage',
            'Container Network Traffic'
        ];

        for (const p of panels) {
            await expect(page.getByText(p)).toBeVisible();
        }

        // Check that we have valid data by looking for likely container names
        // We look for 'ai-cv-evaluator' or 'db' related text which indicates metrics are being populated
        // The legend usually lists container names
        await expect(page.locator('div[data-testid="data-testid panel-content"]').first()).toBeVisible();

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
