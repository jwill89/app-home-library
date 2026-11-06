import type { RouteLocationRaw, Router } from 'vue-router'

/** A page above this one, in the trail over its title. */
export interface Crumb {
  label: string
  to: RouteLocationRaw
}

// Leaving a page that is a step on the way (an edit, a record just removed) so that Back does not
// walk through it again. Vue Router keeps the address of the page before in history.state.back;
// it is null when this page is the first the app showed.

function before(): string | null {
  const back: unknown = (window.history.state as { back?: unknown } | null)?.back
  return typeof back === 'string' ? back : null
}

const pathOf = (address: string) => address.split(/[?#]/)[0]

/**
 * The one way to leave an edit for the record it edits: back to the record when it is the page
 * before, so the edit leaves no step behind; otherwise the record in the edit's place.
 */
export function returnTo(router: Router, path: string) {
  const back = before()
  return back !== null && pathOf(back) === path ? router.back() : router.replace(path)
}

/**
 * The one way to leave a page there is nothing more to do on (a record removed, a new one given
 * up): back to the page before, unless that is a page of the same record, or `fallback` in its place.
 */
export function goBack(router: Router, fallback: string, leaving?: string) {
  const back = before()
  const path = back === null ? '' : pathOf(back)
  const stale = leaving !== undefined && (path === leaving || path.startsWith(`${leaving}/`))
  return back !== null && !stale ? router.back() : router.replace(fallback)
}
