import { test, expect } from '@playwright/test';
import { BASE_URL } from './test-helpers';

test('TC_007 - Strategy module source viewer can be closed cleanly', async ({ page }) => {
  await page.goto(BASE_URL);
  await page.getByRole('link', { name: 'Strategy Modules Manage' }).click();
  await expect(page).toHaveURL(`${BASE_URL}/strategies/modules`);

  await page
    .getByRole('row', { name: /grid\s+grid/i })
    .getByRole('button', { name: 'View source' })
    .click();

  const modal = page.getByRole('dialog', { name: /Source: grid\.js/i });
  await expect(modal).toBeVisible();

  await modal.getByRole('button', { name: 'Close' }).click();
  await expect(modal).toBeHidden();
});
