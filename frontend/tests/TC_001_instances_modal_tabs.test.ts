import { test, expect } from '@playwright/test';
import { BASE_URL } from './test-helpers';

test('TC_001 - Instances modal toggles JSON spec and Guided form', async ({ page }) => {
  await page.goto(BASE_URL);
  await page.getByRole('link', { name: 'View instances' }).click();
  await expect(page).toHaveURL(`${BASE_URL}/instances`);

  const modal = page.getByRole('dialog', { name: 'Create Strategy Instance' });
  await page.getByRole('button', { name: 'Create Instance' }).click();
  await expect(modal).toBeVisible();

  await modal.getByRole('tab', { name: 'JSON spec' }).click();
  await expect(modal.getByText('Instance specification')).toBeVisible();

  await modal.getByRole('tab', { name: 'Guided form' }).click();
  await expect(modal.getByLabel('Instance ID')).toBeVisible();

  await modal.getByRole('button', { name: 'Cancel' }).click();
  await expect(modal).toBeHidden();
});
