import { test, expect } from '@playwright/test';

test('existing policies are visible', async ({ page }) => {
  await page.goto('/policies');
  await expect(page.locator('table tbody tr')).not.toHaveCount(0);
});

test('add a new policy', async ({ page }) => {
  await page.goto('/policies/new');
  await page.locator('#source_id').selectOption('*');
  await page.locator('#target_id').selectOption('*');
  await page.locator('#direction').selectOption('unidirectional');
  await page.locator('#action').selectOption('allow');
  await page.locator('#priority').fill('999');
  await page.getByRole('button', { name: 'Add Policy' }).click();
  await expect(page).toHaveURL('/policies');
  await expect(page.locator('table tbody')).toContainText('999');
});

test('edit the priority-999 policy to deny', async ({ page }) => {
  await page.goto('/policies');
  const row = page.locator('table tbody tr', { hasText: '999' });
  await row.locator('a', { hasText: 'Edit' }).click();
  await page.locator('#action').selectOption('deny');
  await page.getByRole('button', { name: 'Save Changes' }).click();
  await expect(page).toHaveURL('/policies');
  await expect(page.locator('table tbody tr', { hasText: '999' }).locator('.badge')).toContainText('deny');
});

test('delete the priority-999 policy', async ({ page }) => {
  await page.goto('/policies');
  const row = page.locator('table tbody tr', { hasText: '999' });
  await row.locator('button', { hasText: 'Delete' }).click();
  await expect(page).toHaveURL('/policies');
  await expect(page.locator('table tbody')).not.toContainText('999');
});
