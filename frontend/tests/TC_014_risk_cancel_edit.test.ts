import { test, expect } from '@playwright/test';
import { BASE_URL } from './test-helpers';

test('TC_014 - Cancelling risk edits restores original values', async ({ page }) => {
  await page.goto(BASE_URL);
  await page.getByRole('link', { name: 'Risk Limits Configure risk' }).click();
  await expect(page).toHaveURL(`${BASE_URL}/risk`);

  await page.getByRole('button', { name: 'Edit Limits' }).click();
  const positionSizeInput = page.getByRole('textbox', { name: 'Max Position Size' });
  await positionSizeInput.fill('123');

  await page.getByRole('button', { name: 'Cancel' }).click();
  await expect(page.getByRole('button', { name: 'Edit Limits' })).toBeVisible();

  await page.getByRole('button', { name: 'Edit Limits' }).click();
  await expect(page.getByRole('textbox', { name: 'Max Position Size' })).toHaveValue('250');
  await page.getByRole('button', { name: 'Cancel' }).click();
});
