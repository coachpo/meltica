import { test, expect } from '@playwright/test';
import { BASE_URL } from './test-helpers';

test('TC_006 - Copying strategy source surfaces clipboard error toast', async ({ page }) => {
  await page.goto(BASE_URL);
  await page.getByRole('link', { name: 'Strategy Modules Manage' }).click();
  await expect(page).toHaveURL(`${BASE_URL}/strategies/modules`);

  await page
    .getByRole('row', { name: /grid\s+grid/i })
    .getByRole('button', { name: 'View source' })
    .click();

  const modal = page.getByRole('dialog', { name: /Source: grid\.js/i });
  await expect(modal).toBeVisible();

  await modal.getByRole('button', { name: 'Copy source' }).click();
  await expect(page.getByText('Copy failed')).toBeVisible();
  await expect(page.getByText('Clipboard API unavailable', { exact: false })).toBeVisible();

  await modal.getByRole('button', { name: 'Close' }).click();
  await expect(modal).toBeHidden();
});
