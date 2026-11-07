import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { vitestDefaults } from '../shared/test/web/vitest.js'

const api = 'http://127.0.0.1:8130'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  // The Go server replaces this placeholder with a fresh nonce on every index.html response.
  html: { cspNonce: '__CSP_NONCE__' },
  // The proxy keeps the browser's Host header, which the server checks against HL_ORIGINS
  // and makes the passkey's relying party from.
  server: {
    proxy: {
      '/api': api,
      '/auth': api,
      '/media': api,
      '/s': api,
      '/calendar': api,
      '/healthz': api,
    },
  },
  // The shared test settings (shared-tooling's test/web). No test needs a DOM yet, so the
  // suite runs on Node; one that does says `// @vitest-environment jsdom` at its top.
  // `npm run test:coverage` fails below the floor: a little under what the tests reached
  // when it was set (2026-11-07: 7.9% of statements, 10.2% of branches, 4.9% of functions,
  // 8.2% of lines). Raise it as tests land; never lower it.
  test: vitestDefaults({
    environment: 'node',
    include: ['src/**/*.test.ts'],
    floor: { statements: 7.5, branches: 9.5, functions: 4.5, lines: 7.5 },
  }),
})
