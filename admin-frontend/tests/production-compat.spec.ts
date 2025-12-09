import { test, expect } from '@playwright/test';

test.describe('Dashboard Metrics', () => {
  test('Grafana should load and display dashboards', async ({ page }) => {
    // Go to Grafana
    await page.goto('https://dashboard.ai-cv-evaluator.web.id/grafana/');

    // Handle Keycloak login if redirected there
    const pageTitle = await page.title();
    if (pageTitle.includes('Sign in')) {
      console.log('Logging in via Keycloak...');
      await page.getByLabel('Username').fill(process.env.SSO_USERNAME || 'admin');
      await page.getByLabel('Password').fill(process.env.SSO_PASSWORD || 'admin123');
      await page.getByRole('button', { name: 'Sign In' }).click();
    }

    // Login if redirected (Auto-login might be enabled via OAuth2, but let's handle title check)
    await expect(page).toHaveTitle(/Grafana|Home/);

    // Go to Docker Containers Dashboard
    await page.goto('https://dashboard.ai-cv-evaluator.web.id/grafana/d/docker-monitoring/docker-containers?orgId=1&refresh=5s');

    // Check for "No data" message - we want to ensure it is NOT present eventually, 
    // but for now we just verify the page loads. 
    // Ideally, we wait for a specific panel title.
    await expect(page.getByText('Container CPU Usage')).toBeVisible();

    // Verify at least one graph is rendering canvas or legend
    // This selector is generic for Grafana panels
    await expect(page.locator('.panel-content')).toBeVisible();
  });
});
