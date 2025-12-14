import type { Page } from '@playwright/test';

import { apiRequestWithRetry } from './api.ts';

export const clearMailpitMessages = async (page: Page): Promise<void> => {
  try {
    const listResp = await apiRequestWithRetry(page, 'get', '/mailpit/api/v1/messages');
    if (!listResp || listResp.status() !== 200) {
      return;
    }
    const body = (await listResp.json()) as any;
    const messages = body.messages ?? [];
    if (messages.length === 0) {
      return;
    }
    const ids = messages.map((m: any) => m.ID);
    await page.request.delete('/mailpit/api/v1/messages', {
      data: { ids },
    });
  } catch {
  }
};
