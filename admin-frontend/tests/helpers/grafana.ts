import { expect, type Page } from '@playwright/test';

// Validate AI Metrics dashboard panels via Grafana API.
export const validateAiMetricsDashboard = async (page: Page): Promise<void> => {
  const aiResp = await page.request.get('/grafana/api/dashboards/uid/ai-metrics');
  expect(aiResp.ok()).toBeTruthy();
  const aiBody = await aiResp.json();
  const aiDash: any = (aiBody as any).dashboard ?? aiBody;
  const aiPanels: any[] = aiDash.panels ?? [];

  // Validate new summary stat panels
  const totalAiRequestsPanel = aiPanels.find((p) => p.title === 'Total AI Requests');
  expect(totalAiRequestsPanel).toBeTruthy();
  expect(totalAiRequestsPanel?.type ?? '').toBe('stat');
  expect(String(totalAiRequestsPanel?.targets?.[0]?.expr ?? '')).toContain('ai_requests_total');

  const medianLatencyPanel = aiPanels.find((p) => p.title === 'Median AI Latency (p50)');
  expect(medianLatencyPanel).toBeTruthy();
  expect(String(medianLatencyPanel?.targets?.[0]?.expr ?? '')).toContain('histogram_quantile(0.50');

  const p95LatencyPanel = aiPanels.find((p) => p.title === '95th Percentile Latency');
  expect(p95LatencyPanel).toBeTruthy();
  expect(String(p95LatencyPanel?.targets?.[0]?.expr ?? '')).toContain('histogram_quantile(0.95');

  const totalTokensPanel = aiPanels.find((p) => p.title === 'Total Tokens Used');
  expect(totalTokensPanel).toBeTruthy();
  expect(String(totalTokensPanel?.targets?.[0]?.expr ?? '')).toContain('ai_tokens_total');

  // Validate existing panels
  const ratePanel = aiPanels.find((p) => p.title === 'AI Request Rate');
  expect(String(ratePanel?.targets?.[0]?.expr ?? '')).toContain('sum(rate(ai_requests_total');
  expect(ratePanel?.fieldConfig?.defaults?.unit ?? '').toBe('ops');
  const pctlPanel = aiPanels.find((p) => p.title === 'AI Request Latency Percentiles');
  const pctlExprs = (pctlPanel?.targets ?? []).map((t: any) => String(t.expr ?? ''));
  expect(pctlExprs.some((e: string) => e.includes('histogram_quantile(0.95'))).toBeTruthy();
  expect(pctlExprs.some((e: string) => e.includes('histogram_quantile(0.99'))).toBeTruthy();
  expect(pctlPanel?.fieldConfig?.defaults?.unit ?? '').toBe('s');
  // Macro hygiene and templating
  const aiTpl: any[] = aiDash.templating?.list ?? [];
  expect(aiTpl.length).toBe(0);
  for (const p of aiPanels) {
    const dfl: any[] = (p.fieldConfig?.defaults?.links as any[]) ?? [];
    for (const l of dfl) expect(String(l?.url ?? '')).not.toContain('__data.fields');
    const ovs: any[] = (p.fieldConfig?.overrides as any[]) ?? [];
    for (const o of ovs) {
      const props: any[] = (o.properties as any[]) ?? [];
      for (const pr of props) {
        if (pr.id === 'links') {
          const arr: any[] = (pr.value as any[]) ?? [];
          for (const lk of arr) expect(String(lk?.url ?? '')).not.toContain('__data.fields');
        }
      }
    }
  }
  // Default AI Metrics time range: last 6 hours.
  expect(aiDash.time?.from ?? '').toBe('now-6h');
  expect(aiDash.time?.to ?? '').toBe('now');
};

// Validate HTTP Metrics dashboard panels via Grafana API.
export const validateHttpMetricsDashboard = async (page: Page): Promise<void> => {
  const httpResp = await page.request.get('/grafana/api/dashboards/uid/http-metrics');
  expect(httpResp.ok()).toBeTruthy();
  const httpBody = await httpResp.json();
  const httpDash: any = (httpBody as any).dashboard ?? httpBody;
  const httpPanels: any[] = httpDash.panels ?? [];

  // Validate Error Rate by Route panel
  const errorRateByRoutePanel = httpPanels.find((p) => p.title === 'Error Rate Over Time by Route');
  expect(errorRateByRoutePanel).toBeTruthy();
  expect(errorRateByRoutePanel?.type ?? '').toBe('timeseries');
  const errorRateExpr = String(errorRateByRoutePanel?.targets?.[0]?.expr ?? '');
  expect(errorRateExpr).toContain('by (route)');
  expect(errorRateExpr).toContain('http_requests_total');

  // Validate Top Error Routes table panel
  const topErrorRoutesPanel = httpPanels.find((p) => p.title === 'Top Error Routes');
  expect(topErrorRoutesPanel).toBeTruthy();
  expect(topErrorRoutesPanel?.type ?? '').toBe('table');
  const topErrorExpr = String(topErrorRoutesPanel?.targets?.[0]?.expr ?? '');
  expect(topErrorExpr).toContain('topk');
  expect(topErrorExpr).toContain('by (route, status)');

  // Validate table has transformations
  const transformations = topErrorRoutesPanel?.transformations ?? [];
  expect(transformations.length).toBeGreaterThan(0);
};

// Validate Job Queue Metrics dashboard panels via Grafana API.
export const validateJobQueueMetricsDashboard = async (page: Page): Promise<void> => {
  const jqResp = await page.request.get('/grafana/api/dashboards/uid/job-queue-metrics');
  expect(jqResp.ok()).toBeTruthy();
  const jqBody = await jqResp.json();
  const jqDash: any = (jqBody as any).dashboard ?? jqBody;
  const jqPanels: any[] = jqDash.panels ?? [];
  const processing = jqPanels.find((p) => p.title === 'Jobs Currently Processing');
  expect(String(processing?.targets?.[0]?.expr ?? '')).toContain('sum(jobs_processing)');
  expect(processing?.fieldConfig?.defaults?.unit ?? '').toBe('short');
  const throughput = jqPanels.find((p) => p.title === 'Job Throughput');
  const thrExprs = (throughput?.targets ?? []).map((t: any) => String(t.expr ?? ''));
  expect(thrExprs.some((e: string) => e.includes('jobs_enqueued_total'))).toBeTruthy();
  expect(thrExprs.some((e: string) => e.includes('jobs_completed_total'))).toBeTruthy();
  expect(thrExprs.some((e: string) => e.includes('jobs_failed_total'))).toBeTruthy();
  const successRate = jqPanels.find((p) => p.title === 'Job Success Rate');
  expect(successRate?.fieldConfig?.defaults?.unit ?? '').toBe('percentunit');
  // Macro hygiene and templating
  const jqTpl: any[] = jqDash.templating?.list ?? [];
  expect(jqTpl.length).toBe(0);
  for (const p of jqPanels) {
    const dfl: any[] = (p.fieldConfig?.defaults?.links as any[]) ?? [];
    for (const l of dfl) expect(String(l?.url ?? '')).not.toContain('__data.fields');
    const ovs: any[] = (p.fieldConfig?.overrides as any[]) ?? [];
    for (const o of ovs) {
      const props: any[] = (o.properties as any[]) ?? [];
      for (const pr of props) {
        if (pr.id === 'links') {
          const arr: any[] = (pr.value as any[]) ?? [];
          for (const lk of arr) expect(String(lk?.url ?? '')).not.toContain('__data.fields');
        }
      }
    }
  }
  // Default Job Queue Metrics time range: last 6 hours.
  expect(jqDash.time?.from ?? '').toBe('now-6h');
  expect(jqDash.time?.to ?? '').toBe('now');
};
