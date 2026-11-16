import { readFileSync } from 'node:fs'
import { fileURLToPath, URL } from 'node:url'
import type { Plugin, Rolldown } from 'vite'
import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { VitePWA } from 'vite-plugin-pwa'
import { vitestDefaults } from '../shared/test/web/vitest.js'

const api = 'http://127.0.0.1:8130'

// The views a signed-out visitor can open: the routes router/index.ts marks `public: true`. Each
// such route names its view just before its meta, and a mark that can't be matched to a view
// stops the build rather than leaving a public page out of the service worker's precache.
function publicViews(): string[] {
  const source = readFileSync(
    fileURLToPath(new URL('./src/router/index.ts', import.meta.url)),
    'utf8',
  )
  const views = [
    ...source.matchAll(/import\('@\/(views\/\w+\.vue)'\)[^}]*\bpublic: true\b/g),
  ].flatMap((match) => (match[1] ? [match[1]] : []))
  const marked = source.match(/\bpublic: true\b/g)?.length ?? 0
  if (views.length === 0 || views.length !== marked) {
    throw new Error(
      `router/index.ts marks ${String(marked)} routes public, but ${String(views.length)} name a view ` +
        'the build can find; publicViews in vite.config.ts reads them',
    )
  }
  return views
}

// The built files a signed-out visitor's pages load, and the rest under assets/, which only a
// signed-in page loads. Filled while the bundle is written; the service worker's precache
// takes the first as its entries, and the check after it proves none of the rest got in.
const publicOutput = new Set<string>()
const signedInOutput = new Set<string>()
const publicEntries: { url: string; revision: null }[] = []

// Walks the bundle's graph from the entry and the public views. Every other view, and the
// shell that frames them, is a barrier: the entry's router reaches every view, so a walk that
// passed through them would reach everything and prove nothing. A chunk's stylesheet and the
// files it imports go with it.
function markPublicOutput(): Plugin {
  const views = publicViews()
  return {
    name: 'home-library:mark-public-output',
    apply: 'build',
    generateBundle(_options, bundle) {
      const chunks = new Map<string, Rolldown.OutputChunk>()
      for (const [file, output] of Object.entries(bundle)) {
        if (output.type === 'chunk') {
          chunks.set(file, output)
        }
      }
      const source = (file: string) =>
        (chunks.get(file)?.facadeModuleId ?? '').replaceAll('\\', '/')
      const isPublicView = (file: string) =>
        views.some((view) => source(file).endsWith(`/src/${view}`))
      const isPage = (file: string) =>
        /\/src\/(?:views\/\w+|components\/AppShell)\.vue$/.test(source(file))
      const isBarrier = (file: string) => isPage(file) && !isPublicView(file)

      const reached = new Set<string>()
      const queue = [...chunks.keys()].filter(
        (file) => chunks.get(file)?.isEntry || isPublicView(file),
      )
      for (let file = queue.pop(); file !== undefined; file = queue.pop()) {
        const chunk = chunks.get(file)
        if (!chunk || reached.has(file) || isBarrier(file)) {
          continue
        }
        reached.add(file)
        queue.push(...chunk.imports, ...chunk.dynamicImports)
      }

      publicOutput.clear()
      signedInOutput.clear()
      for (const file of reached) {
        publicOutput.add(file)
        const meta = chunks.get(file)?.viteMetadata
        for (const sidecar of [...(meta?.importedCss ?? []), ...(meta?.importedAssets ?? [])]) {
          publicOutput.add(sidecar)
        }
      }
      for (const file of Object.keys(bundle)) {
        if (file.startsWith('assets/') && !publicOutput.has(file)) {
          signedInOutput.add(file)
        }
      }
      // Named for their content, so the name alone says which version each one is.
      publicEntries.splice(
        0,
        publicEntries.length,
        ...[...publicOutput].map((url) => ({ url, revision: null })),
      )
    },
  }
}

// Reads the service worker the build wrote and fails the build when its precache holds a file
// only a signed-in page loads, or leaves out one a signed-out page needs (as Workbox does,
// quietly, with a file over its size limit). It runs after the plugin that writes sw.js, so a
// missing sw.js is a failure, not a build without one.
function checkPrecache(): Plugin {
  let outDir = 'dist'
  return {
    name: 'home-library:check-precache',
    apply: 'build',
    enforce: 'post',
    configResolved(config) {
      outDir = config.build.outDir
    },
    closeBundle: {
      sequential: true,
      order: 'post',
      handler() {
        const sw = fileURLToPath(new URL(`./${outDir}/sw.js`, import.meta.url))
        let source: string
        try {
          source = readFileSync(sw, 'utf8')
        } catch {
          this.error(`expected the service worker at ${sw}, found none`)
        }
        const precached = new Set(
          [...source.matchAll(/url:"([^"]+)"/g)].flatMap((match) => (match[1] ? [match[1]] : [])),
        )
        const list = (files: string[]) =>
          files.slice(0, 3).join(', ') +
          (files.length > 3 ? `, and ${String(files.length - 3)} more` : '')
        const leaked = [...precached].filter(
          (file) => file.startsWith('assets/') && !publicOutput.has(file),
        )
        if (leaked.length > 0) {
          this.error(
            `the service worker's precache holds ${String(leaked.length)} file(s) only a ` +
              `signed-in page loads, which every visitor would download: ${list(leaked)}`,
          )
        }
        const missing = ['index.html', ...publicOutput].filter((file) => !precached.has(file))
        if (missing.length > 0) {
          this.error(
            `the service worker's precache leaves out ${String(missing.length)} file(s) the ` +
              `signed-out pages need, so the app can't open offline: ${list(missing)}`,
          )
        }
        if (signedInOutput.size === 0) {
          this.error(
            'the walk in markPublicOutput found no file only a signed-in page loads; it no ' +
              'longer tells the two apart',
          )
        }
      },
    },
  }
}

export default defineConfig({
  plugins: [
    vue(),
    tailwindcss(),
    // Before VitePWA: it fills publicEntries while the bundle is written, and VitePWA reads
    // them after, when it writes the service worker.
    markPublicOutput(),
    // The service worker, written only by a build (main.ts registers it there). Its versioned
    // precache holds the shell and what the signed-out pages load, so the installed app opens
    // without a connection; a deploy changes the precache, and the new worker takes over at
    // once (autoUpdate). Pages ask the network first and fall back to the precached shell only
    // when there is none, so each answer's nonce stays its own. The built files a signed-in
    // page has loaded are kept as they come: named for their content, a copy is never stale.
    // The API, covers, documents (under the API), the calendar feed, share links and the
    // health check never pass through it.
    VitePWA({
      registerType: 'autoUpdate',
      injectRegister: null,
      manifest: false,
      workbox: {
        // The files copied from public/ are what any visitor's browser asks for; the built
        // files are the walk's, filled in by markPublicOutput before this array is read.
        globPatterns: ['index.html', 'manifest.webmanifest', 'icons/*'],
        additionalManifestEntries: publicEntries,
        // A page is the network's answer, not the precached shell's, while there is a network.
        navigateFallback: null,
        directoryIndex: null,
        // Each function is written into sw.js as its source, so it names nothing outside itself.
        runtimeCaching: [
          {
            // A page of the app; the server's own addresses are left to the browser.
            urlPattern: ({ request, url }) =>
              request.mode === 'navigate' &&
              !/^\/(?:api|media|calendar|s|healthz)(?:\/|$)/.test(url.pathname),
            handler: 'NetworkOnly',
            options: { precacheFallback: { fallbackURL: 'index.html' } },
          },
          {
            // A build has about a hundred files, so two builds' worth are kept: a deploy's new
            // files push out the old ones least lately used.
            urlPattern: ({ sameOrigin, url }) => sameOrigin && url.pathname.startsWith('/assets/'),
            handler: 'CacheFirst',
            options: {
              cacheName: 'hl-assets',
              cacheableResponse: { statuses: [200] },
              expiration: { maxEntries: 200, purgeOnQuotaError: true },
            },
          },
        ],
      },
    }),
    checkPrecache(),
  ],
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
      '/media': api,
      '/s': api,
      '/calendar': api,
      '/healthz': api,
    },
  },
  // The shared test settings, jsdom among them. `npm run test:coverage` fails below the
  // floor: a little under what the specs reach. Raise it as specs land; never lower it.
  test: vitestDefaults({
    include: ['src/__tests__/**/*.spec.ts'],
    floor: { statements: 10, branches: 9.75, functions: 6.25, lines: 10 },
  }),
})
