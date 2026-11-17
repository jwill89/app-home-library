import type { Area, BookRow } from '@/types'
import type { stackArrangements } from '@/constants'
import { categories } from '@/constants'
import { byVolume, callNumberText } from '@/app/format'

// The Stacks: the library's books standing as spines along bookcases on the screen, in an order
// the owner chooses. A spine is drawn to the book's height and thickness, at one scale for both.

export type Arrangement = (typeof stackArrangements)[number]['value']

/** CSS pixels a millimetre: a 23 cm paperback stands about 290 pixels tall. */
export const pxPerMm = 1.25
/** The narrowest a spine is drawn, the target size: a thinner book is drawn this thick. */
export const spineMin = 44
/** The space between two spines on a shelf. */
export const spineGap = 2

export const spineWidth = (b: Pick<BookRow, 'thicknessMm'>) =>
  Math.max(spineMin, Math.round(b.thicknessMm * pxPerMm))
export const spineHeight = (b: Pick<BookRow, 'heightMm'>) => Math.round(b.heightMm * pxPerMm)

/** One bookcase of the Stacks: its name, and its books in the order they stand. */
export interface Bookcase {
  key: string
  title: string
  books: BookRow[]
}

const words = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' })

/**
 * Shelf order: the order the server's shelf key gives, compared character by character as it
 * is compared there, not by the reader's language.
 */
export const byShelf = (a: BookRow, b: BookRow) =>
  a.shelfKey < b.shelfKey ? -1 : a.shelfKey > b.shelfKey ? 1 : a.id - b.id

/** A book's first author's surname, as a spine shows it: the name it is filed under. */
export function surname(b: Pick<BookRow, 'authorSort' | 'authors'>): string {
  const filed = b.authorSort.split(',')[0]?.trim()
  return filed || (b.authors.split(', ')[0] ?? '')
}

// By the first author as filed, a book with none after those with one; an author's series
// together in volume order; then by title.
function byAuthor(a: BookRow, b: BookRow): number {
  return (
    Number(!a.authorSort) - Number(!b.authorSort) ||
    words.compare(a.authorSort, b.authorSort) ||
    words.compare(a.series, b.series) ||
    byVolume(a.seriesNumber, b.seriesNumber) ||
    words.compare(a.title, b.title)
  )
}

const byTitle = (a: BookRow, b: BookRow) =>
  words.compare(a.title, b.title) || words.compare(a.authors, b.authors)

/**
 * The bookcases the books stand in, arranged as chosen: a bookcase a category, in the
 * categories' own order; a bookcase an area, in the order books try them, with the books no
 * area takes after; or one run of every book. Within a category, and the books in no area, in
 * shelf order. An empty bookcase is left out.
 */
export function arrange(
  books: readonly BookRow[],
  by: Arrangement,
  areas: readonly Area[] = [],
): Bookcase[] {
  const every = (sort: (a: BookRow, b: BookRow) => number) => [
    { key: 'all', title: 'Every book', books: [...books].sort(sort) },
  ]
  switch (by) {
    case 'call-number':
      return every(byShelf).filter((c) => c.books.length)
    case 'title':
      return every(byTitle).filter((c) => c.books.length)
    case 'author':
      return every(byAuthor).filter((c) => c.books.length)
    case 'area': {
      const byId = new Map(books.map((b) => [b.id, b]))
      const placed = new Set<number>()
      const cases = areas.map((a) => {
        const own = (a.bookIds ?? []).flatMap((id) => byId.get(id) ?? [])
        own.forEach((b) => placed.add(b.id))
        return { key: `area-${String(a.id)}`, title: a.name, books: own }
      })
      const rest = books.filter((b) => !placed.has(b.id)).sort(byShelf)
      return [...cases, { key: 'no-area', title: 'In no area', books: rest }].filter(
        (c) => c.books.length,
      )
    }
    default:
      return categories
        .map((c) => ({
          key: c.value,
          title: c.plural,
          books: books.filter((b) => b.category === c.value).sort(byShelf),
        }))
        .filter((c) => c.books.length)
  }
}

/**
 * Things along shelves of a width, in order: each shelf takes as many as fit with the gap
 * between them, and the next goes on the next shelf. A thing wider than a shelf has one of its
 * own.
 */
export function shelvesOf<T>(
  items: readonly T[],
  width: number,
  widthOf: (t: T) => number,
  gap = spineGap,
): T[][] {
  const shelves: T[][] = []
  let shelf: T[] = []
  let used = 0
  for (const item of items) {
    const w = widthOf(item)
    if (shelf.length && used + gap + w > width) {
      shelves.push(shelf)
      shelf = []
      used = 0
    }
    used += (shelf.length ? gap : 0) + w
    shelf.push(item)
  }
  if (shelf.length) {
    shelves.push(shelf)
  }
  return shelves
}

// How a book is named at the end of a shelf, as the arrangement orders it: by call number in
// shelf order, by surname by author, and by title otherwise or where it has neither.
function shelfName(b: BookRow, by: Arrangement): string {
  const call = callNumberText((b.callNumber ?? []).slice(0, 2))
  if ((by === 'category' || by === 'call-number') && call) {
    return call
  }
  if (by === 'author' && surname(b)) {
    return surname(b)
  }
  return b.title
}

/** What a shelf holds, as its edge says it: its first book to its last, or the one. */
export function shelfRange(shelf: readonly BookRow[], by: Arrangement): string {
  const [first, last] = [shelf[0], shelf.at(-1)]
  if (!first || !last) {
    return ''
  }
  const [from, to] = [shelfName(first, by), shelfName(last, by)]
  return from === to ? from : `${from} to ${to}`
}
