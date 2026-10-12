import { defineConfig, devices } from '@playwright/test';

// End-to-end smoke test of the web app against a running server with data:
//   BC_E2E_URL=http://localhost:8080 npm run test:e2e
// Uses Playwright's Chromium (npx playwright install chromium), or an installed browser with
// BC_E2E_CHANNEL=chrome|msedge.
export default defineConfig({
  testDir: 'e2e',
  timeout: 60_000,
  expect: { timeout: 15_000 },
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['github'], ['list']] : 'list',
  use: {
    baseURL: process.env.BC_E2E_URL ?? 'http://localhost:8080',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'], channel: process.env.BC_E2E_CHANNEL || undefined } }],
});
