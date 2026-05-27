import { test, expect } from '@playwright/test';
import { BASE_URL } from './test-helpers';

test('TC_012 - Binance adapter schema exposes duration defaults', async ({ page }) => {
  await page.goto(BASE_URL);
  await page.getByRole('link', { name: 'Adapters View exchange' }).click();
  await expect(page).toHaveURL(`${BASE_URL}/adapters`);

  await page.getByRole('button', { name: 'View schema' }).first().click();

  const modal = page.getByRole('dialog', { name: 'Binance Spot' });
  await expect(modal).toBeVisible();
  await expect(modal.getByRole('row', { name: /snapshot_depth/i })).toContainText('1000');
  await expect(modal.getByRole('row', { name: /recv_window/i })).toContainText('5s');

  await modal.getByRole('button', { name: 'Close' }).click();
  await expect(modal).toBeHidden();
});
