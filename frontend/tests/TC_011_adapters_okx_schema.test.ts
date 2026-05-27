import { test, expect } from '@playwright/test';
import { BASE_URL } from './test-helpers';

test('TC_011 - OKX adapter schema lists required fields', async ({ page }) => {
  await page.goto(BASE_URL);
  await page.getByRole('link', { name: 'Adapters View exchange' }).click();
  await expect(page).toHaveURL(`${BASE_URL}/adapters`);

  await page.getByRole('button', { name: 'View schema' }).nth(1).click();

  const modal = page.getByRole('dialog', { name: 'OKX Spot' });
  await expect(modal).toBeVisible();
  await expect(modal.getByRole('row', { name: /api_key\s+string/i })).toBeVisible();
  await expect(modal.getByRole('row', { name: /instrument_refresh_interval/i })).toContainText('15m0s');

  await modal.getByRole('button', { name: 'Close' }).click();
  await expect(modal).toBeHidden();
});
