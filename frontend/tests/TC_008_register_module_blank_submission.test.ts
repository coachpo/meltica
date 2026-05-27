import { test, expect } from '@playwright/test';
import { BASE_URL } from './test-helpers';

test('TC_008 - Register module rejects empty source payload', async ({ page }) => {
  await page.goto(BASE_URL);
  await page.getByRole('link', { name: 'Strategy Modules Manage' }).click();
  await expect(page).toHaveURL(`${BASE_URL}/strategies/modules`);

  await page.getByRole('button', { name: 'Register module' }).click();
  const modal = page.getByRole('dialog', { name: 'Register strategy module' });
  await expect(modal).toBeVisible();

  await modal.getByRole('button', { name: 'Register & refresh' }).click();
  await expect(modal.getByText('Strategy source code cannot be empty')).toBeVisible();

  await modal.getByRole('button', { name: 'Cancel' }).click();
  await expect(modal).toBeHidden();
});
