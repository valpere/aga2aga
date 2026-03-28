import { Page } from '@playwright/test';

export async function login(page: Page, user = 'admin', pass?: string) {
  const password = pass || process.env.ADMIN_PASSWORD || 'changeme';
  await page.goto('/login');
  await page.locator('#username').fill(user);
  await page.locator('#password').fill(password);
  await page.locator('button[type="submit"]').click();
  await page.waitForURL('/');
}
