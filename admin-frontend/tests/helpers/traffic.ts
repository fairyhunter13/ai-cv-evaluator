import type { Page } from '@playwright/test';

// Generate real backend traffic so that Prometheus and Loki have recent
// samples for http_request_by_id_total and request_id labels. This helps
// ensure Grafana dashboards such as HTTP Metrics and Request Drilldown have
// non-empty data during E2E runs.
export const generateBackendTraffic = async (page: Page): Promise<void> => {
  // Hit healthz (not SSO-gated) to generate basic metrics/logs.
  for (let i = 0; i < 3; i += 1) {
    await page.request.get('/healthz');
  }

  // After SSO login, /v1/result is SSO-gated but will return either 200/404;
  // in both cases the backend still logs and records metrics with request_id.
  for (let i = 0; i < 3; i += 1) {
    await page.request.get(`/v1/result/nonexistent-${i}`);
  }
};
