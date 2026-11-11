import { defineConfig } from '@playwright/test'
import { playwrightDefaults } from '../shared/test/e2e/playwright.js'

// End-to-end tests in a real browser against the real server (see e2e/server.ts), on the
// shared Playwright settings: one worker in order (the tests share one library and some
// depend on what another added), a trace kept on failure, and the browser already
// installed (Edge locally on Windows, the runner's Chrome in CI).
export default defineConfig({
  ...playwrightDefaults,
  testMatch: '*.e2e.ts',
  globalSetup: './e2e/server.ts',
  timeout: 30_000,
  use: {
    ...playwrightDefaults.use,
    baseURL: 'http://localhost:8140',
    storageState: 'e2e/.state.json',
    viewport: { width: 1280, height: 900 },
  },
})
