import { defineConfig } from '@playwright/test'

// End-to-end tests in a real browser against the real server (see e2e/server.ts). Locally they
// drive the Edge already on the machine; on CI, the runner's Chrome. One worker: the tests
// share one library and some depend on what another added.
export default defineConfig({
  testDir: 'e2e',
  testMatch: '*.e2e.ts',
  globalSetup: './e2e/server.ts',
  workers: 1,
  fullyParallel: false,
  reporter: [['list']],
  timeout: 30_000,
  use: {
    baseURL: 'http://localhost:8140',
    channel: process.env.CI ? 'chrome' : 'msedge',
    storageState: 'e2e/.state.json',
    viewport: { width: 1280, height: 900 },
  },
})
