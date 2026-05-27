import { test, expect } from '@playwright/test';
import { BASE_URL } from './test-helpers';

test('TC_009 - Register module template inserts metadata skeleton', async ({ page }) => {
  await page.goto(BASE_URL);
  await page.getByRole('link', { name: 'Strategy Modules Manage' }).click();
  await expect(page).toHaveURL(`${BASE_URL}/strategies/modules`);

  await page.getByRole('button', { name: 'Register module' }).click();
  const modal = page.getByRole('dialog', { name: 'Register strategy module' });
  await expect(modal).toBeVisible();

  await modal.getByRole('button', { name: 'Insert template' }).click();
  await expect(modal.locator('textarea').first()).toContainText('module.exports = {');

  await modal.getByRole('button', { name: 'Close' }).click();
  await expect(modal).toBeHidden();
});
