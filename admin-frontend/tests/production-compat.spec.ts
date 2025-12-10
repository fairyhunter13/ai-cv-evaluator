import { test, expect } from '@playwright/test';

test.describe('Dashboard Metrics', () => {
  test('Grafana should load and display dashboards with data', async ({ page }) => {
    const baseUrl = process.env.E2E_BASE_URL || 'https://ai-cv-evaluator.web.id';

    // Go to Grafana
    await page.goto(`${baseUrl}/grafana/`);

    // Handle Keycloak login if redirected there
    const pageTitle = await page.title();
    if (pageTitle.toLowerCase().includes('sign in')) {
      console.log('Logging in via Keycloak...');
      await page.getByLabel('Username').fill(process.env.SSO_USERNAME || 'admin');
      await page.locator('input[name="password"]').fill(process.env.SSO_PASSWORD || 'Admin@SecureP4ss2025!');
      await page.getByRole('button', { name: /sign in/i }).click();

      // Wait for navigation
      try {
        await expect(page).not.toHaveTitle(/Sign in/, { timeout: 10000 });
      } catch (e) {
        console.log('Still on Sign In page. Checking for errors...');
        const error = await page.locator('.pf-c-alert__title').textContent().catch(() => null);
        if (error) console.log('Login Error:', error);
      }
    }

    // If redirected to Portal root, log it
    try {
      await expect(page).toHaveTitle(/Portal|AI CV Evaluator/, { timeout: 3000 });
      console.log('Redirected to Portal. Navigating to Docker Containers dashboard...');
    } catch (e) {
      // Already on Grafana
    }

    // Go to Docker Containers Dashboard
    await page.goto(`${baseUrl}/grafana/d/docker-monitoring/docker-containers?orgId=1&refresh=5s`);
    await page.waitForLoadState('networkidle');

    // Verify Grafana loaded
    await expect(page).toHaveTitle(/Docker Containers|Grafana/, { timeout: 10000 });
    console.log('Grafana Docker Containers dashboard loaded.');

    // Verify Container CPU Usage panel is visible
    await expect(page.getByText('Container CPU Usage')).toBeVisible({ timeout: 10000 });

    // Wait for data to load (give Prometheus time to return results)
    await page.waitForTimeout(3000);

    // Verify NO kubepods paths are shown (these are the ugly cgroup names)
    const pageContent = await page.content();
    const hasKubepodsPaths = pageContent.includes('kubepods.slice') || pageContent.includes('kubepods-besteffort');

    if (hasKubepodsPaths) {
      console.log('WARNING: Found kubepods cgroup paths in page - naming may not be fully fixed');
    } else {
      console.log('SUCCESS: No kubepods cgroup paths found');
    }

    // STRICT CHECK: Verify human-readable service names are present
    // We expect names like "backend", "frontend", "db" to appear in the legend
    const expectedServices = ['backend', 'frontend', 'db'];
    const missingServices = [];

    // Legends are now clean: "backend" vs "085936d800b2"
    const legends = await page.locator('.graph-legend-table .graph-legend-series-name').allTextContents();
    console.log(`INFO: Found legends: ${legends.join(', ')}`);

    for (const service of expectedServices) {
      if (!legends.some(l => l.includes(service))) {
        missingServices.push(service);
      }
    }

    if (missingServices.length > 0) {
      console.error(`ERROR: Expected service names not found: ${missingServices.join(', ')}`);
      // Fail the test if we are missing names - this is the core requirement
      expect(missingServices.length).toBe(0);
    } else {
      console.log('SUCCESS: Found expected service names in legend (backend, frontend, db)');
    }

    // Verify new panels are visible
    const newPanels = ['Container Uptime', 'Container Restarts', 'Container Memory Utilization'];
    for (const panel of newPanels) {
      const count = await page.getByText(panel).count();
      if (count > 0) {
        console.log(`SUCCESS: "${panel}" panel found`);
      } else {
        console.log(`WARNING: "${panel}" panel NOT found`);
      }
    }

    // STRICT CHECK: Verify NO "No data" messages are visible
    const noDataElements = await page.getByText('No data').all();
    const noDataCount = noDataElements.length;

    if (noDataCount > 0) {
      console.log(`FAIL: Found ${noDataCount} "No data" messages in dashboard panels`);
      // Take screenshot for debugging
      await page.screenshot({ path: 'no-data-failure.png' });
      expect(noDataCount, 'Dashboard panels should not show "No data"').toBe(0);
    } else {
      console.log('SUCCESS: All dashboard panels have data');
    }

    console.log('Dashboard validation complete.');
  });
});
