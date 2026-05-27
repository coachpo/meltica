import { test, expect } from '@playwright/test';
import { BASE_URL } from './test-helpers';

test('TC_013 - Risk limits require max position size before saving', async ({ page }) => {
  await page.goto(BASE_URL);
  await page.getByRole('link', { name: 'Risk Limits Configure risk' }).click();
  await expect(page).toHaveURL(`${BASE_URL}/risk`);

  const editButton = page.getByRole('button', { name: 'Edit Limits' });
  await editButton.click();

  const positionSizeInput = page.getByRole('textbox', { name: 'Max Position Size' });
  await positionSizeInput.fill('');
  await page.getByRole('button', { name: 'Save Changes' }).click();

  await expect(page.getByText('maxPositionSize required')).toBeVisible();
  await expect(page.getByText('Save failed')).toBeVisible();

  await page.getByRole('button', { name: 'Cancel' }).click();
  await expect(editButton).toBeVisible();
});
