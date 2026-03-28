import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './interactive',
  fullyParallel: false,
  workers: 1,
  timeout: 30_000,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    baseURL: process.env.ADMIN_URL || 'http://localhost:8087',
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'setup',
      testMatch: /.*\.setup\.ts/,
    },
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        storageState: '.auth/state.json',
      },
      dependencies: ['setup'],
    },
  ],
});
