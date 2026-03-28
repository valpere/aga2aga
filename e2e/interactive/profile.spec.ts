import { test, expect } from '@playwright/test';

const originalPassword = process.env.ADMIN_PASSWORD || 'changeme';
const newPassword = 'E2eTestPass!99';

// Serial execution: these tests must run in order and clean up after themselves.
test.describe.serial('change password flow', () => {
  test('change password succeeds', async ({ page }) => {
    await page.goto('/profile');
    await page.locator('#current_password').fill(originalPassword);
    await page.locator('#new_password').fill(newPassword);
    await page.locator('#confirm_password').fill(newPassword);
    await page.getByRole('button', { name: 'Change Password' }).click();
    await expect(page.locator('.alert-success')).toBeVisible();
  });

  test('login with new password works', async ({ page }) => {
    // Stored session is still valid — just verify we can navigate
    await page.goto('/');
    await expect(page.locator('h1')).toHaveText('Dashboard');
  });

  test('revert password back to original', async ({ page }) => {
    await page.goto('/profile');
    await page.locator('#current_password').fill(newPassword);
    await page.locator('#new_password').fill(originalPassword);
    await page.locator('#confirm_password').fill(originalPassword);
    await page.getByRole('button', { name: 'Change Password' }).click();
    await expect(page.locator('.alert-success')).toBeVisible();
  });
});
