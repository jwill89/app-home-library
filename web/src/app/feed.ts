import { onBeforeUnmount, onMounted } from 'vue'
import type { Change } from '@/api/client'
import { useSession } from './session'

// The change feed: the server tells the owner's open pages, as it happens, what changed
// anywhere (on another device, in the background lookup, in the text service, in an import),
// and each page fetches again what it shows. One stream a tab, open while anything listens.

type ChangeType = Change['type']

/**
 * What a listener is told: the records of its types that changed, gathered over a moment so a
 * burst is one fetch, and `all` when the stream was away too long to say, so everything it
 * shows is to be fetched again.
 */
export type Changed = (refs: ReadonlySet<number>, all: boolean) => unknown

interface Listener {
  types: readonly ChangeType[]
  changed: Changed
}

const gatherMs = 300
// The server pings every 25 seconds. Nearly three missed means the line is dead even though
// nothing said so, as a phone's is after its screen sleeps.
const quietMs = 70_000
const reopenMs = 5000
// The stream stays open a moment after the last page lets go of it, for the next page.
const lingerMs = 2000

const listeners = new Set<Listener>()
const gathered = new Map<Listener, Set<number>>()
let source: EventSource | null = null
let last = 0
let heardAt = 0
let flush: ReturnType<typeof setTimeout> | undefined
let quiet: ReturnType<typeof setTimeout> | undefined
let reopen: ReturnType<typeof setTimeout> | undefined
let linger: ReturnType<typeof setTimeout> | undefined

function heard() {
  heardAt = Date.now()
  clearTimeout(quiet)
  quiet = setTimeout(open, quietMs)
}

// Opened again, the stream goes on from the last change this tab had: none is missed.
function open() {
  clearTimeout(reopen)
  source?.close()
  source = new EventSource(last ? `/api/v1/changes?since=${last}` : '/api/v1/changes')
  heard()
  source.addEventListener('ping', heard)
  source.addEventListener('reset', () => {
    heard()
    for (const l of listeners) {
      void l.changed(new Set(), true)
    }
  })
  source.addEventListener('change', (e) => {
    heard()
    try {
      take(JSON.parse((e as MessageEvent<string>).data) as Change)
    } catch {
      // A frame that does not read is the server's to log; the next stands alone.
    }
  })
  source.onerror = () => {
    // The browser opens a dropped stream again by itself, but gives up for good when the
    // server answers with an error, as it does for a moment while it restarts.
    if (source?.readyState === EventSource.CLOSED) {
      reopen = setTimeout(open, reopenMs)
    }
  }
}

function take(c: Change) {
  last = Math.max(last, c.id)
  for (const l of listeners) {
    if (!l.types.includes(c.type)) {
      continue
    }
    const refs = gathered.get(l) ?? new Set<number>()
    refs.add(c.ref)
    gathered.set(l, refs)
  }
  flush ??= setTimeout(() => {
    flush = undefined
    for (const [l, refs] of gathered) {
      if (listeners.has(l)) {
        void l.changed(refs, false)
      }
    }
    gathered.clear()
  }, gatherMs)
}

function close() {
  clearTimeout(quiet)
  clearTimeout(reopen)
  source?.close()
  source = null
  document.removeEventListener('visibilitychange', onVisible)
}

// A tab back in view whose stream has been quiet may have lost it while hidden: it opens again
// now, not when the silence runs out.
function onVisible() {
  if (document.visibilityState !== 'visible') {
    return
  }
  if (source?.readyState === EventSource.CLOSED || Date.now() - heardAt > 30_000) {
    open()
  }
}

/**
 * Listens to the feed until the returned function is called: the way a store listens, for as
 * long as it lives. A visitor has no feed.
 */
export function listen(types: readonly ChangeType[], changed: Changed): () => void {
  if (!useSession().isOwner) {
    return () => undefined
  }
  const l = { types, changed }
  listeners.add(l)
  clearTimeout(linger)
  if (!source) {
    open()
    document.addEventListener('visibilitychange', onVisible)
  }
  return () => {
    listeners.delete(l)
    gathered.delete(l)
    if (!listeners.size) {
      linger = setTimeout(close, lingerMs)
    }
  }
}

/** Listens to the feed while a page is open: the way a page learns what changed. */
export function useFeed(types: readonly ChangeType[], changed: Changed) {
  let stop: (() => void) | undefined
  onMounted(() => (stop = listen(types, changed)))
  onBeforeUnmount(() => stop?.())
}
