
import { test, expect } from '@playwright/test';

test.describe('Grafana Dashboard Verification', () => {
    test('should load Grafana and verify all Docker metrics are valid', async ({ page }) => {
        // Determine base URL from environment or default to local dev
        const baseURL = process.env.BASE_URL || 'http://localhost:8088';

        console.log(`Navigating to Grafana at ${baseURL}/grafana/`);

        // go to grafana
        await page.goto(`${baseURL}/grafana/`);

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
                await page.getByLabel('Username').fill(process.env.ADMIN_USERNAME || 'admin');
                await page.getByLabel('Password').fill(process.env.ADMIN_PASSWORD || 'admin123');
                await page.getByRole('button', { name: 'Sign In' }).click();
            }
        } catch (e) {
            // Ignore
        }

        // Wait for Grafana title
        await expect(page).toHaveTitle(/Grafana/);

        // Navigate to Docker Containers dashboard
        const dashboardUrl = `${baseURL}/grafana/d/docker-monitoring/docker-containers`;
        console.log(`Navigating to Dashboard at ${dashboardUrl}`);
        await page.goto(dashboardUrl);

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

        // Check that we have valid data by looking for known container names on the page
        // Using `ai-cv-evaluator-db-1` as a reliable indicator of data presence
        const expectedContainers = ['ai-cv-evaluator-db-1'];

        for (const container of expectedContainers) {
            // We expect to see this text somewhere on the dashboard (in a legend)
            // Using first() because it might appear multiple times (once per panel legend)
            await expect(page.getByText(container).first()).toBeVisible();
        }

        console.log('Grafana Docker Containers dashboard verified: Panels visible and data populated.');
    });
});
