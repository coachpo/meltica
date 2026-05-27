import { test, expect } from '@playwright/test';
import { BASE_URL } from './test-helpers';

test('TC_003 - Instances modal can be cancelled', async ({ page }) => {
  await page.goto(BASE_URL);
  await page.getByRole('link', { name: 'View instances' }).click();
  await expect(page).toHaveURL(`${BASE_URL}/instances`);

  const modal = page.getByRole('dialog', { name: 'Create Strategy Instance' });
  await page.getByRole('button', { name: 'Create Instance' }).click();
  await expect(modal).toBeVisible();

  await modal.getByRole('button', { name: 'Cancel' }).click();
  await expect(modal).toBeHidden();
});
