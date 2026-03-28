import { test, expect } from '@playwright/test';

const ts = Date.now();
const opKeyName = `e2e-op-${ts}`;
const agentKeyName = `e2e-agent-key-${ts}`;

let agentID: string;

test.beforeAll(async ({ browser }) => {
  agentID = `e2e-key-agent-${ts}`;
  const context = await browser.newContext({ storageState: '.auth/state.json' });
  const page = await context.newPage();
  await page.goto('/agents/new');
  await page.locator('#agent_id').fill(agentID);
  await page.locator('#display_name').fill('E2E Key Agent');
  await page.getByRole('button', { name: 'Register' }).click();
  await context.close();
});

test('create an operator key shows the raw key once', async ({ page }) => {
  await page.goto('/api-keys');
  await page.locator('input[name="name"]').fill(opKeyName);
  // role=operator is default, no Agent ID needed
  await page.getByRole('button', { name: 'Create Key' }).click();
  await expect(page.locator('.alert-success')).toBeVisible();
  await expect(page.locator('.alert-success code')).not.toBeEmpty();
});

test('create an agent key shows role and agent ID in table', async ({ page }) => {
  await page.goto('/api-keys');
  await page.locator('input[name="name"]').fill(agentKeyName);
  await page.locator('#key-role').selectOption('agent');
  await expect(page.locator('#agent-id-field')).toBeVisible();
  await page.locator('input[name="agent_id"]').fill(agentID);
  await page.getByRole('button', { name: 'Create Key' }).click();
  await expect(page.locator('.alert-success')).toBeVisible();
  const row = page.locator('table tbody tr', { hasText: agentKeyName });
  await expect(row).toContainText('agent');
  await expect(row).toContainText(agentID);
});

test('revoke the operator key removes it from the list', async ({ page }) => {
  await page.goto('/api-keys');
  const row = page.locator('table tbody tr', { hasText: opKeyName });
  await row.locator('button', { hasText: 'Revoke' }).click();
  // Revoked keys are excluded from ListAPIKeys — the row should be gone.
  await expect(page.locator('table tbody tr', { hasText: opKeyName })).not.toBeVisible();
});
