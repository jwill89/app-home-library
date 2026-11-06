/// <reference types="vitest/config" />
import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

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
  test: { environment: 'happy-dom' },
})
