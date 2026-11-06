import type {
  Book,
  BookInput,
  Contributor,
  Copy,
  Course,
  Cover,
  CopyInput,
  CourseInput,
  DocumentFile,
  DocumentInput,
  QueuedLabel,
  Scan,
  ShelfPlace,
  ShelvingRule,
} from '@/api/client'

// The one place the catalog's stored words become the words shown. Each list's values are typed
// by the field it is for, so a value the server does not take is a type error.

/** The kinds of book the library sorts by: one each, set or worked out from its details. */
export const categories = [
  { value: 'fiction', label: 'Fiction', plural: 'Fiction' },
  { value: 'nonfiction', label: 'Nonfiction', plural: 'Nonfiction' },
  { value: 'manga', label: 'Manga', plural: 'Manga' },
  { value: 'light_novel', label: 'Light novel', plural: 'Light novels' },
  { value: 'graphic_novel', label: 'Graphic novel', plural: 'Graphic novels' },
  { value: 'uncategorized', label: 'Uncategorized', plural: 'Uncategorized' },
] as const satisfies readonly { value: Book['category']; label: string; plural: string }[]

export const categoryLabel = (v: string) => categories.find((c) => c.value === v)?.label ?? ''

export const formats: { value: BookInput['format']; label: string }[] = [
  { value: '', label: 'Not known' },
  { value: 'hardcover', label: 'Hardcover' },
  { value: 'paperback', label: 'Paperback' },
  { value: 'trade_paperback', label: 'Trade paperback' },
  { value: 'mass_market', label: 'Mass market paperback' },
  { value: 'library_binding', label: 'Library binding' },
  { value: 'spiral', label: 'Spiral-bound' },
  { value: 'board_book', label: 'Board book' },
  { value: 'leather', label: 'Leather-bound' },
  { value: 'other', label: 'Other' },
]

export const readStatuses: { value: BookInput['readStatus']; label: string }[] = [
  { value: 'unread', label: 'Unread' },
  { value: 'reading', label: 'Reading' },
  { value: 'read', label: 'Read' },
]

export const conditions: { value: CopyInput['condition']; label: string }[] = [
  { value: 'unknown', label: 'Not recorded' },
  { value: 'fine', label: 'Fine' },
  { value: 'very_good', label: 'Very good' },
  { value: 'good', label: 'Good' },
  { value: 'fair', label: 'Fair' },
  { value: 'poor', label: 'Poor' },
]

export const acquiredAs: { value: CopyInput['acquiredAs']; label: string }[] = [
  { value: 'unknown', label: 'Not recorded' },
  { value: 'new', label: 'New' },
  { value: 'used', label: 'Used' },
]

export const dustJackets: { value: CopyInput['dustJacket']; label: string }[] = [
  { value: 'unknown', label: 'Not recorded' },
  { value: 'yes', label: 'Has one' },
  { value: 'no', label: 'None' },
]

export const roles: { value: Contributor['role']; label: string }[] = [
  { value: 'author', label: 'Author' },
  { value: 'editor', label: 'Editor' },
  { value: 'translator', label: 'Translator' },
  { value: 'illustrator', label: 'Illustrator' },
  { value: 'narrator', label: 'Narrator' },
  { value: 'contributor', label: 'Contributor' },
]

const scanStatuses: { value: Scan['status']; label: string }[] = [
  { value: 'pending', label: 'Looking it up…' },
  { value: 'found', label: 'Not in your library yet' },
  { value: 'adding', label: 'Adding…' },
  { value: 'added', label: 'Added' },
  { value: 'owned', label: 'Already in your library' },
  { value: 'not_found', label: 'No source knows this ISBN' },
  { value: 'failed', label: 'The sources could not be reached' },
]

export const levels: { value: CourseInput['level']; label: string }[] = [
  { value: 'unknown', label: 'Not recorded' },
  { value: 'high_school', label: 'High school' },
  { value: 'undergraduate', label: 'Undergraduate' },
  { value: 'graduate', label: 'Graduate' },
  { value: 'professional', label: 'Professional development' },
  { value: 'other', label: 'Other' },
]

export const terms: { value: CourseInput['term']; label: string }[] = [
  { value: 'unknown', label: 'No season' },
  { value: 'winter', label: 'Winter' },
  { value: 'spring', label: 'Spring' },
  { value: 'summer', label: 'Summer' },
  { value: 'fall', label: 'Fall' },
]

export const shelvings: { value: ShelvingRule['shelving']; label: string }[] = [
  { value: 'dewey', label: 'By Dewey number' },
  { value: 'author', label: 'By author' },
  { value: 'series', label: 'By series' },
]

const labelKinds: { value: QueuedLabel['kind']; label: string; short: string }[] = [
  { value: 'barcode', label: 'Barcode labels', short: 'barcode' },
  { value: 'spine', label: 'Spine labels', short: 'spine' },
]

/** A sheet of labels by its kind: Barcode labels, Spine labels. */
export const labelKindLabel = (v: string) => labelIn(labelKinds, v)

/** The labels a copy waits on, in a sentence: Barcode and spine labels wait to print. */
export function labelsWaiting(kinds: string[]): string {
  const words = kinds.map((k) => labelKinds.find((x) => x.value === k)?.short ?? k)
  const sentence = `${words.join(' and ')} ${words.length === 1 ? 'label waits' : 'labels wait'} to print.`
  return sentence.charAt(0).toUpperCase() + sentence.slice(1)
}

export const goneKinds: {
  value: 'lost' | 'sold' | 'donated' | 'given' | 'discarded'
  label: string
}[] = [
  { value: 'lost', label: 'Lost' },
  { value: 'sold', label: 'Sold' },
  { value: 'donated', label: 'Donated' },
  { value: 'given', label: 'Given away' },
  { value: 'discarded', label: 'Discarded' },
]

export const goneKindLabel = (v: string) => labelIn(goneKinds, v)

/** How and when a copy left the library: Donated on Nov 5, 2026: to the school library. */
export function goneLine(c: { goneAs: string; goneOn: string; goneNote: string }): string {
  const line = `${goneKindLabel(c.goneAs)} on ${calendarDate(c.goneOn)}`
  return c.goneNote ? `${line}: ${c.goneNote}` : line
}

/**
 * Volume numbers as they are read: runs of whole numbers joined (1–5, 7, 9–12), and any other
 * number (2.5) on its own, in order.
 */
export function numberRanges(numbers: string[] | null | undefined): string {
  const out: string[] = []
  let start = 0
  let end = 0
  const close = () => {
    if (start) out.push(start === end ? String(start) : `${start}–${end}`)
    start = 0
  }
  for (const v of numbers ?? []) {
    const n = Number(v)
    if (!Number.isInteger(n) || n < 1) {
      close()
      out.push(v)
    } else if (start && n === end + 1) {
      end = n
    } else {
      close()
      start = end = n
    }
  }
  close()
  return out.join(', ')
}

const detailLabels: Record<string, string> = {
  cover: 'Cover',
  isbn13: 'ISBN-13',
  isbn10: 'ISBN-10',
  title: 'Title',
  subtitle: 'Subtitle',
  contributors: 'People',
  series: 'Series',
  seriesNumber: 'Number in the series',
  category: 'Category',
  lccn: 'LCCN',
  publisher: 'Publisher',
  publishPlace: 'Place published',
  published: 'Date published',
  editionStatement: 'Edition',
  format: 'Format',
  pages: 'Pages',
  heightMm: 'Height',
  widthMm: 'Width',
  thicknessMm: 'Thickness',
  language: 'Language',
  genres: 'Genres',
  ddc: 'Dewey number',
  lcc: 'Library of Congress class',
  description: 'Description',
}

/** A detail of a book, by its field's name, in words. */
export const detailLabel = (field: string) => detailLabels[field] ?? field

/** A detail of a book as text, as its page shows it: a book saved, or one being edited. */
export function detailText(b: Book | BookInput, field: string, unit: Unit): string {
  switch (field) {
    case 'contributors':
      return (b.contributors ?? [])
        .map((c) =>
          c.role === 'author' ? c.name : `${c.name} (${roleLabel(c.role).toLowerCase()})`,
        )
        .join(', ')
    case 'format':
      return b.format ? formatLabel(b.format) : ''
    case 'category':
      return categoryLabel(b.category)
    case 'published':
      return calendarDate(b.published)
    case 'pages':
      return b.pages ? String(b.pages) : ''
    case 'heightMm':
    case 'widthMm':
    case 'thicknessMm':
      return lengthIn(b[field], unit)
    case 'genres':
      return (b.genres ?? []).join(', ')
  }
  const v = (b as Record<string, unknown>)[field]
  return typeof v === 'string' ? v : ''
}

/** A book's place in its series: "The Broken Empire #2". */
export const seriesLine = (series: string, number: string) =>
  number ? `${series} #${number}` : series

/** A course as a choice names it: its name, then its term. */
export const courseLabel = (c: Course) => [courseName(c), termLabel(c)].filter(Boolean).join(' · ')

/** What the place a paper appeared in is called, for its kind. */
export const containerLabel = (kind: DocumentInput['kind']) =>
  kind === 'article' || kind === 'review' ? 'Journal' : kind === 'chapter' ? 'Book' : 'Appeared in'

/** A copy as a choice names it: its barcode, its section, and who has it. */
export const copyLabel = (c: Copy) =>
  [c.barcode, c.section, c.loan ? `lent to ${c.loan.borrower}` : ''].filter(Boolean).join(' · ')

/** A call number on one line, as it reads across the spine label's lines. */
export const callNumberText = (lines: string[] | null) => (lines ?? []).join(' ')

export const documentKinds: { value: DocumentInput['kind']; label: string }[] = [
  { value: 'article', label: 'Article' },
  { value: 'chapter', label: 'Chapter' },
  { value: 'review', label: 'Book review' },
  { value: 'handout', label: 'Handout' },
  { value: 'syllabus', label: 'Syllabus' },
  { value: 'notes', label: 'Notes' },
  { value: 'exam', label: 'Exam' },
  { value: 'other', label: 'Other' },
]

const textStatuses: { value: DocumentFile['textStatus']; label: string }[] = [
  { value: 'pending', label: 'Waiting to be read' },
  { value: 'done', label: 'Text read' },
  { value: 'empty', label: 'No text found' },
  { value: 'failed', label: 'Could not be read' },
  { value: 'owner', label: 'Text typed by you' },
]

/** A file's size as a person reads it: 340 KB, 2.4 MB. */
export function fileSize(bytes: number): string {
  if (bytes < 1024 * 1024) return `${Math.max(1, Math.round(bytes / 1024))} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function labelIn(list: { value: string; label: string }[], value: string): string {
  return list.find((x) => x.value === value)?.label ?? value
}

export const levelLabel = (v: string) => (v === 'unknown' ? '' : labelIn(levels, v))
export const shelvingLabel = (v: string) => labelIn(shelvings, v)
export const documentKindLabel = (v: string) => labelIn(documentKinds, v)

const paperLabels: Record<string, string> = {
  kind: 'Kind',
  title: 'Title',
  authors: 'Authors',
  editors: 'Editors',
  volume: 'Volume',
  issue: 'Issue',
  pages: 'Pages',
  published: 'Date',
  publisher: 'Publisher',
  url: 'Link',
  doi: 'DOI',
  abstract: 'Abstract',
  notes: 'Notes',
  subjects: 'Subjects',
}

/** A detail of a paper, by its field's name, in words: where it appeared named for its kind. */
export const paperDetailLabel = (field: string, kind: DocumentInput['kind']) =>
  field === 'container' ? containerLabel(kind) : (paperLabels[field] ?? field)

/** A detail of a paper as text, as its page shows it. */
export function paperDetailText(d: DocumentInput, field: string): string {
  switch (field) {
    case 'kind':
      return documentKindLabel(d.kind)
    case 'authors':
      return listed(d.authors ?? [])
    case 'editors':
      return listed(d.editors ?? [])
    case 'subjects':
      return (d.subjects ?? []).join(', ')
    case 'published':
      return calendarDate(d.published)
  }
  const v = (d as Record<string, unknown>)[field]
  return typeof v === 'string' ? v : ''
}
export const textStatusLabel = (v: string) => labelIn(textStatuses, v)

interface CourseWords {
  department: string
  number: string
  title: string
  term: string
  year: number
}

/** A course as a transcript writes it: MATH 241 Calculus III. */
export function courseName(c: CourseWords): string {
  return [c.department, c.number, c.title].filter(Boolean).join(' ')
}

/** When a course was taken: Fall 2004, or 2004, or nothing. */
export function termLabel(c: CourseWords): string {
  const season = c.term === 'unknown' ? '' : labelIn(terms, c.term)
  return [season, c.year ? String(c.year) : ''].filter(Boolean).join(' ')
}

export const formatLabel = (v: string) => labelIn(formats, v)
export const readStatusLabel = (v: string) => labelIn(readStatuses, v)
export const conditionLabel = (v: string) => labelIn(conditions, v)
export const acquiredLabel = (v: string) => labelIn(acquiredAs, v)
export const roleLabel = (v: string) => labelIn(roles, v)
export const scanStatusLabel = (v: string) => labelIn(scanStatuses, v)

const sourceNames: Record<string, string> = {
  loc: 'Library of Congress',
  'loc-related': 'Library of Congress, from another edition',
  openlibrary: 'Open Library',
  google: 'Google Books',
  hardcover: 'Hardcover',
  hathitrust: 'HathiTrust',
  inferred: 'its class number, subjects, and publisher',
  series: 'the rest of the series',
  bookshelf: 'the Bookshelf import',
  endnote: 'EndNote',
  goodreads: 'Goodreads',
  librarything: 'LibraryThing',
  'bookshelf-cut': 'the Bookshelf import, cut off',
  owner: 'you',
  merged: 'several sources',
  web: 'a link',
}

/** Where a field's value came from, in words. */
export function sourceLabel(source: string | undefined): string {
  if (!source) return ''
  return sourceNames[source] ?? source
}

export type Unit = 'in' | 'cm'

// Whole, half, and quarter inches first: a tape measure's marks, as the owner reads them.
const inchSteps = [1, 0.5, 0.25, 0.1, 0.01]

/**
 * Millimetres in the owner's unit, as the plainest number that comes back to the same
 * millimetres: 279 mm is 11 in, not 10.98; 32 mm is 1.25 in; 279 mm is 27.9 cm. Read to a
 * tenth (212 mm is 8.3 in); a field asks for hundredths, so what was typed comes back.
 */
export function inUnit(mm: number, unit: Unit, places: 1 | 2 = 1): number {
  const exact = unit === 'in' ? mm / 25.4 : mm / 10
  const scale = 10 ** places
  for (const step of (unit === 'in' ? inchSteps : [1, 0.1]).filter((s) => s * scale >= 1)) {
    const v = Math.round(Math.round(exact / step) * step * 100) / 100
    if (fromUnit(v, unit) === mm) return v
  }
  return Math.round(exact * scale) / scale
}

/** A measurement in the owner's unit back to whole millimetres. */
export function fromUnit(value: number, unit: Unit): number {
  return Math.round(unit === 'in' ? value * 25.4 : value * 10)
}

/** One length in the owner's unit: 1.2 in, or nothing when it is not known. */
export const lengthIn = (mm: number, unit: Unit) => (mm ? `${inUnit(mm, unit)} ${unit}` : '')

/**
 * Where a copy stands, or would in the proposed layout: its bookcase and shelf, and with
 * `neighbours` the books either side of it there.
 */
export function placeLine(p: ShelfPlace, neighbours = false): string {
  if (!p.placed) return 'No shelf has room for it. Add a shelf, or plan the shelves fuller.'
  const where = `${p.bookcase}, shelf ${p.position} from the top${p.oversize ? ' (oversize)' : ''}`
  const between = !neighbours
    ? ''
    : p.after && p.before
      ? `, after ${p.after} and before ${p.before}`
      : p.after
        ? `, at the end, after ${p.after}`
        : p.before
          ? `, at the start, before ${p.before}`
          : ''
  return p.inUse
    ? `On ${where}${between}.`
    : `Would go on ${where}${between}, in the proposed layout.`
}

/** A shelf's size as it is read: 31.5 × 11 × 10 in, and its corner braces if it has them. */
export function shelfSize(
  s: {
    widthMm: number
    heightMm: number
    depthMm: number
    braceHeightMm?: number
    braceWidthMm?: number
  },
  unit: Unit,
): string {
  const size = `${inUnit(s.widthMm, unit)} × ${inUnit(s.heightMm, unit)} × ${inUnit(s.depthMm, unit)} ${unit}`
  if (!s.braceHeightMm || !s.braceWidthMm) return size
  return `${size}, braces ${inUnit(s.braceHeightMm, unit)} × ${inUnit(s.braceWidthMm, unit)} ${unit}`
}

/** Cents as dollars, or empty when the price is not known (-1). */
export function money(cents: number): string {
  if (cents < 0) return ''
  return (cents / 100).toLocaleString(undefined, { style: 'currency', currency: 'USD' })
}

const monthYear = new Intl.DateTimeFormat(undefined, { year: 'numeric', month: 'long' })
const fullDate = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' })

/** A date as precisely as it is known: 2014, June 2014, or Jun 2, 2014. */
export function calendarDate(value: string): string {
  const [y, m, d] = value.split('-').map(Number)
  if (!y) return ''
  if (!m) return String(y)
  if (!d) return monthYear.format(new Date(y, m - 1, 1))
  return fullDate.format(new Date(y, m - 1, d))
}

/** A time stamp from the server as the device writes a date; empty is never. */
export function day(stamp: string): string {
  return stamp ? fullDate.format(new Date(stamp)) : 'Never'
}

/** Today on the device's clock as YYYY-MM-DD: the owner's day, not the server's or UTC's. */
export function today(now = new Date()): string {
  return [now.getFullYear(), now.getMonth() + 1, now.getDate()]
    .map((n, i) => String(n).padStart(i ? 2 : 4, '0'))
    .join('-')
}

/** The day `days` after a YYYY-MM-DD day. */
export function addDays(date: string, days: number): string {
  const [y = 0, m = 1, d = 1] = date.split('-').map(Number)
  return today(new Date(y, m - 1, d + days))
}

/** How many days past `due` a copy still out is on `on`; 0 when it is not late. */
export function daysLate(due: string, on: string): number {
  if (!due || due >= on) return 0
  const [a, b] = [due, on].map((s) => {
    const [y = 0, m = 1, d = 1] = s.split('-').map(Number)
    return Date.UTC(y, m - 1, d)
  })
  return Math.round(((b ?? 0) - (a ?? 0)) / 86_400_000)
}

/** The moment something good through `date` (YYYY-MM-DD, on the device's clock) ends. */
export function endOf(date: string): string {
  const [y = 0, m = 1, d = 1] = date.split('-').map(Number)
  return new Date(y, m - 1, d + 1).toISOString()
}

/** The last day before `stamp`, which is endOf's day back again. */
export function lastDay(stamp: string): string {
  return fullDate.format(new Date(new Date(stamp).getTime() - 1))
}

/** The address of a stored cover's rendition: the smallest at least `width` wide, else the master. */
export function coverURL(cover: Cover, width: number): string {
  const fit = (cover.widths ?? []).find((w) => w >= width)
  return `/media/${cover.id}/${fit ? `w${fit}` : 'master'}.jpg`
}

/** A srcset over a stored cover's renditions. */
export function coverSrcset(cover: Cover): string {
  return [...(cover.widths ?? []).map((w) => `/media/${cover.id}/w${w}.jpg ${w}w`)].join(', ')
}

/** The names in a role, joined for reading. */
export function names(people: { name: string; role: string }[] | null, role: string): string {
  return listed((people ?? []).filter((p) => p.role === role).map((p) => p.name))
}

/** "A", "A and B", "A, B, and C"; APA's citations write the last "and" as "&". */
export function listed(list: readonly string[], and = 'and'): string {
  if (list.length <= 1) return list.join('')
  if (list.length === 2) return `${list[0]} ${and} ${list[1]}`
  return `${list.slice(0, -1).join(', ')}, ${and} ${list[list.length - 1]}`
}

/** A number of things: "1 book", "3 books"; `many` where the plural is not one more s. */
export const count = (n: number, one: string, many = `${one}s`) => `${n} ${n === 1 ? one : many}`
