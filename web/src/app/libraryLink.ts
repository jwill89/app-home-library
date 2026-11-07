import { ref, watch } from 'vue'
import { parseName } from './cite'
import { recall, remember } from './remember'

// The owner's library: a search of its databases for a document, opened in the browser, where
// the owner signs in with their own card. The app never signs in to a database itself.

const key = 'hl.librarySearch'

/** The library's search as the owner pasted it in Settings, kept on this device. */
export const librarySearch = ref(recall(key))
watch(librarySearch, (v) => remember(key, v.trim()))

/**
 * The link that searches the library for words, from the search the owner pasted: an EBSCO
 * research platform link (research.ebsco.com/c/…), whose results page takes the words as q,
 * or any https link with {query} where the words go. Empty when it is neither.
 */
export function librarySearchLink(base: string, words: string): string {
  let u: URL
  try {
    u = new URL(base.trim())
  } catch {
    return ''
  }
  if (u.protocol !== 'https:') {
    return ''
  }
  const ebsco = u.hostname === 'research.ebsco.com' && /^\/c\/([a-z0-9]+)/i.exec(u.pathname)
  if (ebsco) {
    return `https://research.ebsco.com/c/${ebsco[1]}/search/results?q=${encodeURIComponent(words)}`
  }
  const raw = base.trim()
  return raw.includes('{query}') ? raw.replace('{query}', encodeURIComponent(words)) : ''
}

/**
 * The words a library search finds a work by: its title before any subtitle or note, as a
 * phrase, and its first author's surname, as the server tells two editions of one work apart.
 * A DOI alone finds nothing in EBSCO's databases.
 */
export function workWords(title: string, authors: readonly string[] = []): string {
  const main = (title.split(/[:;,([.]/)[0] ?? '').replace(/["“”]/g, '').trim()
  const first = parseName(authors[0] ?? '')
  // A body is searched by its whole name; a person by the surname alone, past a suffix.
  const surname = first.whole ?? first.family.split(/[\s,]+/)[0] ?? ''
  return [main ? `"${main}"` : '', surname].filter(Boolean).join(' ')
}
