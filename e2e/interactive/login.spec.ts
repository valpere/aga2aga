import { test, expect } from '@playwright/test';
import { login } from '../helpers/auth';

// Use empty storageState to override the project-level auth cookie.
test.use({ storageState: { cookies: [], origins: [] } });

test('login with valid credentials redirects to dashboard', async ({ page }) => {
  await login(page);
  await expect(page).toHaveURL('/');
  await expect(page.locator('h1')).toHaveText('Dashboard');
});

test('login with bad password shows error', async ({ page }) => {
  await page.goto('/login');
  await page.locator('#username').fill('admin');
  await page.locator('#password').fill('wrong-password');
  await page.locator('button[type="submit"]').click();
  await expect(page.locator('.alert-error')).toBeVisible();
  await expect(page).toHaveURL('/login');
});

test('logout redirects to login', async ({ page }) => {
  await login(page);
  await page.locator('form[action="/logout"] button').click();
  await expect(page).toHaveURL('/login');
});

test('unauthenticated request redirects to login', async ({ page }) => {
  await page.goto('/agents');
  await expect(page).toHaveURL('/login');
});
