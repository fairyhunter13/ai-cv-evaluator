import type { Page } from '@playwright/test';

// Retry an API request until it returns a valid 2xx response (handles 502/503 during startup).
export const apiRequestWithRetry = async (
  page: Page,
  method: 'get' | 'post' | 'put' | 'delete',
  url: string,
  options?: { data?: any },
): Promise<any> => {
  const maxAttempts = 10;
  const retryDelayMs = 3000;

  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    const resp = await page.request[method](url, options);
    const status = resp.status();
    const contentType = resp.headers()['content-type'] ?? '';

    // Success: 2xx with JSON or YAML content
    const isValidContent =
      contentType.includes('application/json') ||
      contentType.includes('application/yaml') ||
      contentType.includes('text/yaml') ||
      contentType.includes('text/plain');
    if (status >= 200 && status < 300 && isValidContent) {
      return resp;
    }

    // Retry on 502/503/504 (service unavailable during startup).
    if ([502, 503, 504].includes(status) && attempt < maxAttempts) {
      await page.waitForTimeout(retryDelayMs);
      continue;
    }

    // Retry if we got HTML instead of expected content (SSO redirect or error page).
    if (contentType.includes('text/html') && attempt < maxAttempts) {
      await page.waitForTimeout(retryDelayMs);
      continue;
    }

    return resp; // Return the response even if not ideal for assertion.
  }
};
