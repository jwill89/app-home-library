import type { BookRow } from '@/api/client'

// The library list as this device last had it, for the owner when there is no connection: the
// Library page still lists the books, and Scan still answers "do I own this?". It is kept in
// the browser's cache storage, on the owner's own device only, and replaced each time the
// list loads online.

const CACHE = 'hl-offline'
const KEY = '/offline/books'

interface Saved {
  savedAt: string
  rows: BookRow[]
}

/** Keeps the list for when the device is offline, where the browser allows. */
export async function saveCatalog(rows: BookRow[]): Promise<void> {
  if (!('caches' in globalThis)) return
  try {
    const cache = await caches.open(CACHE)
    const saved: Saved = { savedAt: new Date().toISOString(), rows }
    await cache.put(
      KEY,
      new Response(JSON.stringify(saved), { headers: { 'Content-Type': 'application/json' } }),
    )
  } catch {
    // Not kept: offline, the list is not there.
  }
}

/** The list last kept, and when, or nothing. */
export async function savedCatalog(): Promise<Saved | null> {
  if (!('caches' in globalThis)) return null
  try {
    const hit = await caches.match(KEY, { cacheName: CACHE })
    return hit ? ((await hit.json()) as Saved) : null
  } catch {
    return null
  }
}

/** Forgets the list, as when the owner signs out of this device. */
export async function forgetCatalog(): Promise<void> {
  if (!('caches' in globalThis)) return
  try {
    await caches.delete(CACHE)
  } catch {
    // Nothing kept to forget.
  }
}
