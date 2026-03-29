import { test, expect } from '@playwright/test';

const agentID = `e2e-agent-${Date.now()}`;

test.afterAll(async ({ browser }) => {
  const context = await browser.newContext({ storageState: '.auth/state.json' });
  const page = await context.newPage();
  await page.goto(`/agents/${agentID}`);
  await page.locator('form[action$="/revoke"] button').click();
  await expect(page.locator('.badge-revoked')).toBeVisible();
  await context.close();
});

test('register a new agent', async ({ page }) => {
  await page.goto('/agents/new');
  await page.locator('#agent_id').fill(agentID);
  await page.locator('#display_name').fill('E2E Test Agent');
  await page.getByRole('button', { name: 'Register' }).click();
  // Server redirects to /agents list after registration
  await expect(page).toHaveURL('/agents');
  await expect(page.locator('table tbody')).toContainText(agentID);
});

test('registered agent appears in agents list', async ({ page }) => {
  await page.goto('/agents');
  await expect(page.locator('table tbody')).toContainText(agentID);
});

test('suspend agent', async ({ page }) => {
  await page.goto(`/agents/${agentID}`);
  await page.locator('form[action$="/suspend"] button').click();
  await expect(page.locator('.badge-suspended')).toBeVisible();
});

test('reactivate agent', async ({ page }) => {
  await page.goto(`/agents/${agentID}`);
  await page.locator('form[action$="/activate"] button').click();
  await expect(page.locator('.badge-active')).toBeVisible();
});
