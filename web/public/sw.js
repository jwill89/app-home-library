// The service worker. It keeps the app's shell and the scripts and styles it has loaded, so the
// installed app opens without a connection, and the owner's Scan page can answer "do I own
// this?" from the list the app keeps. Pages ask the network first and are kept as they come;
// the API, covers, and documents never pass through here.
const SHELL = 'hl-shell'
const FILES = 'hl-files'

self.addEventListener('install', () => self.skipWaiting())
self.addEventListener('activate', (event) => event.waitUntil(self.clients.claim()))

self.addEventListener('fetch', (event) => {
  const request = event.request
  if (request.method !== 'GET') return
  const url = new URL(request.url)
  if (url.origin !== self.location.origin) return
  if (request.mode === 'navigate') {
    event.respondWith(page(request))
  } else if (url.pathname.startsWith('/assets/') || url.pathname.startsWith('/icons/')) {
    event.respondWith(file(request))
  }
})

// A page from the network, kept as the shell; without a network, the shell last kept. The
// nonce differs on every answer, so a new build is told by the shell without it, and the old
// build's files are let go.
async function page(request) {
  try {
    const response = await fetch(request)
    const type = response.headers.get('Content-Type') || ''
    if (response.ok && type.startsWith('text/html')) {
      const cache = await caches.open(SHELL)
      const before = await cache.match('/')
      const bare = (text) => text.replace(/nonce="[^"]*"/g, '')
      if (before && bare(await before.text()) !== bare(await response.clone().text())) {
        await caches.delete(FILES)
      }
      await cache.put('/', response.clone())
    }
    return response
  } catch {
    return (await caches.match('/', { cacheName: SHELL })) || Response.error()
  }
}

// A built file: named for its content, so the one kept is the one wanted.
async function file(request) {
  const cache = await caches.open(FILES)
  const kept = await cache.match(request)
  if (kept) return kept
  const response = await fetch(request)
  if (response.ok) await cache.put(request, response.clone())
  return response
}
