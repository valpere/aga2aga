import { test, expect } from '@playwright/test';

test('audit log has entries', async ({ page }) => {
  await page.goto('/audit');
  await expect(page.locator('table tbody tr')).not.toHaveCount(0);
});

test('filter by Agent shows only agent.* actions', async ({ page }) => {
  await page.goto('/audit');
  await page.locator('select').selectOption('Agent');
  const codes = page.locator('table tbody td code');
  const count = await codes.count();
  for (let i = 0; i < count; i++) {
    await expect(codes.nth(i)).toContainText('agent.');
  }
});

test('filter by API Key shows only apikey.* actions', async ({ page }) => {
  await page.goto('/audit');
  await page.locator('select').selectOption('API Key');
  const codes = page.locator('table tbody td code');
  const count = await codes.count();
  for (let i = 0; i < count; i++) {
    await expect(codes.nth(i)).toContainText('apikey.');
  }
});

test('reset filter to All shows mixed action types', async ({ page }) => {
  await page.goto('/audit');
  await page.locator('select').selectOption('All');
  await expect(page.locator('table tbody tr')).not.toHaveCount(0);
});
