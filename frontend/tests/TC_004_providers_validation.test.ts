import { test, expect } from '@playwright/test';
import { BASE_URL } from './test-helpers';

test('TC_004 - Provider creation validates required name', async ({ page }) => {
  await page.goto(BASE_URL);
  await page.getByRole('link', { name: 'Provider console' }).click();
  await expect(page).toHaveURL(`${BASE_URL}/providers`);

  const modal = page.getByRole('dialog', { name: 'Create provider' });
  await page.getByRole('button', { name: 'Create provider' }).click();
  await expect(modal).toBeVisible();

  await modal.getByRole('button', { name: 'Create provider' }).click();
  await expect(modal.getByText('Provider name is required')).toBeVisible();

  await modal.getByRole('button', { name: 'Cancel' }).click();
  await expect(modal).toBeHidden();
});
