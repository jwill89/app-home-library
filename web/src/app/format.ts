import {
  acquiredAs,
  areaKeys,
  categories,
  conditions,
  documentKinds,
  formats,
  goneKinds,
  labelKinds,
  levels,
  logLevels,
  readStatuses,
  roles,
  scanStatuses,
  shelvings,
  terms,
  textStatuses,
} from '@/constants'
import type { Area, Book, BookInput, Copy, Course, Cover, DocumentInput, ShelfPlace } from '@/types'

// The one place the catalog's stored words become the words shown: a value's words come from
// its list in constants/index.ts, and everything else said about a record is made here.

export const categoryLabel = (v: string) => categories.find((c) => c.value === v)?.label ?? ''

/** A sheet of labels by its kind: Barcode labels, Spine labels. */
export const labelKindLabel = (v: string) => labelIn(labelKinds, v)

/** The labels a copy waits on, in a sentence: Barcode and spine labels wait to print. */
export function labelsWaiting(kinds: string[]): string {
  const words = kinds.map((k) => labelKinds.find((x) => x.value === k)?.short ?? k)
  const sentence = `${words.join(' and ')} ${words.length === 1 ? 'label waits' : 'labels wait'} to print.`
  return sentence.charAt(0).toUpperCase() + sentence.slice(1)
}

export const goneKindLabel = (v: string) => labelIn(goneKinds, v)

/** How and when a copy left the library: Donated on Nov 5, 2026: to the school library. */
export function goneLine(c: { goneAs: string; goneOn: string; goneNote: string }): string {
  const line = `${goneKindLabel(c.goneAs)} on ${calendarDate(c.goneOn)}`
  return c.goneNote ? `${line}: ${c.goneNote}` : line
}

/**
 * A series number's volumes, each span its first and last: one number ("3", "2.5", "v.3"), or
 * what an omnibus holds, a range ("1-3", "1 to 3"), a list ("1, 2 & 3", "4+5"), or both ("1-3,
 * 5"). Null for a number that is none of these ("Prequel"). The server reads them alike.
 */
export function volumeSpans(n: string): [number, number][] | null {
  const s = n.trim().replace(/^v\./, '').trim()
  if (!s) {
    return null
  }
  const spans: [number, number][] = []
  for (const part of s.split(/\s*(?:,|&|\+|\band\b)\s*/)) {
    const m = /^(\d+(?:\.\d+)?)(?:\s*(?:-|–|—|\bto\b)\s*(\d+(?:\.\d+)?))?$/.exec(part)
    if (!m) {
      return null
    }
    const first = Number(m[1])
    const last = m[2] ? Number(m[2]) : first
    if (last < first) {
      return null
    }
    spans.push([first, last])
  }
  return spans
}

/**
 * Two series numbers in the series' order: by their first volume, then their last, so an
 * omnibus's 1-3 comes after 1 and before 2; a number that is none of them after the rest.
 */
export function byVolume(a: string, b: string): number {
  const ends = (n: string): [number, number] | null => {
    const spans = volumeSpans(n)
    return spans && [Math.min(...spans.map((s) => s[0])), Math.max(...spans.map((s) => s[1]))]
  }
  const x = ends(a)
  const y = ends(b)
  if (x && y) {
    return x[0] - y[0] || x[1] - y[1]
  }
  return Number(!x) - Number(!y)
}

/**
 * Volume numbers as they are read: runs of whole numbers joined, an omnibus's among them (1–5,
 * 7, 9–12), and any other number (2.5) on its own, in order.
 */
export function numberRanges(numbers: string[] | null | undefined): string {
  const out: string[] = []
  let start = 0
  let end = 0
  const close = () => {
    if (start) {
      out.push(start === end ? String(start) : `${start}–${end}`)
    }
    start = 0
  }
  for (const v of numbers ?? []) {
    const spans = volumeSpans(v)
    if (
      !spans?.every(
        ([first, last]) => Number.isInteger(first) && Number.isInteger(last) && first >= 1,
      )
    ) {
      close()
      out.push(v)
      continue
    }
    for (const [first, last] of spans) {
      if (start && first <= end + 1) {
        end = Math.max(end, last)
      } else {
        close()
        start = first
        end = last
      }
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

/**
 * The choices of a filter by detail: any detail, then each detail some book has in its list
 * (lacks, or differs in), with how many books, the most first.
 */
export function detailChoices<F extends string>(
  lists: readonly (readonly F[] | null | undefined)[],
): { value: F | 'any'; label: string }[] {
  const books = new Map<F, number>()
  for (const list of lists) {
    for (const f of list ?? []) {
      books.set(f, (books.get(f) ?? 0) + 1)
    }
  }
  return [
    { value: 'any', label: 'Any detail' },
    ...[...books]
      .sort(([a, n], [b, m]) => m - n || detailLabel(a).localeCompare(detailLabel(b)))
      .map(([f, n]) => ({ value: f, label: `${detailLabel(f)} (${n})` })),
  ]
}

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

/** A file's size as a person reads it: 340 KB, 2.4 MB. */
export function fileSize(bytes: number): string {
  if (bytes < 1024 * 1024) {
    return `${Math.max(1, Math.round(bytes / 1024))} KB`
  }
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function labelIn(list: { value: string; label: string }[], value: string): string {
  return list.find((x) => x.value === value)?.label ?? value
}

export const levelLabel = (v: string) => (v === 'unknown' ? '' : labelIn(levels, v))
export const shelvingLabel = (v: string) => labelIn(shelvings, v)
export const documentKindLabel = (v: string) => labelIn(documentKinds, v)

/** The server's log level as the Settings choice names it: Problems (warn). */
export const logLevelLabel = (v: string) => labelIn(logLevels, v)

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
  course: 'a course a copy is on',
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
  if (!source) {
    return ''
  }
  return sourceNames[source] ?? source
}

export type Unit = 'in' | 'cm'

// Whole, half, and quarter inches, or whole centimetres: a tape measure's marks, as the owner
// reads them.
const markSteps = { in: [1, 0.5, 0.25], cm: [1] } as const

/**
 * Millimetres in the owner's unit: a tape measure's mark where one comes back to the same
 * millimetres (279 mm is 11 in, not 10.98; 19 mm is 0.75 in), and otherwise three significant
 * figures (30 mm is 1.18 in, 212 mm 8.35 in, 279 mm 27.9 cm), or more where three would not come
 * back to the same millimetres (1524 mm is 152.4 cm). What is shown is what a field holds, so a
 * field never changes a length by showing it.
 */
export function inUnit(mm: number, unit: Unit): number {
  const exact = unit === 'in' ? mm / 25.4 : mm / 10
  for (const step of markSteps[unit]) {
    const v = Math.round(exact / step) * step
    if (fromUnit(v, unit) === mm) {
      return v
    }
  }
  let v = exact
  for (let figures = 3; figures <= 6; figures++) {
    v = Number(exact.toPrecision(figures))
    if (fromUnit(v, unit) === mm) {
      break
    }
  }
  return v
}

/** A measurement in the owner's unit back to whole millimetres. */
export function fromUnit(value: number, unit: Unit): number {
  return Math.round(unit === 'in' ? value * 25.4 : value * 10)
}

/** One length in the owner's unit: 1.2 in, or nothing when it is not known. */
export const lengthIn = (mm: number, unit: Unit) => (mm ? `${inUnit(mm, unit)} ${unit}` : '')

/**
 * Where a copy stands in the layout in use: its bookcase, shelf, and area, and with `neighbours`
 * the books either side of it there.
 */
export function placeLine(p: ShelfPlace, neighbours = false): string {
  if (!p.inUse) {
    return 'No layout is in use: suggest one on the Shelves page, and put it to use.'
  }
  if (!p.placed) {
    return 'The layout in use has no place for it: no area takes it, or its area has no room.'
  }
  const where = `${p.bookcase}, shelf ${p.position} from the top${p.area ? ` (${p.area})` : ''}`
  const between = !neighbours
    ? ''
    : p.after && p.before
      ? `, after ${p.after} and before ${p.before}`
      : p.after
        ? `, at the end, after ${p.after}`
        : p.before
          ? `, at the start, before ${p.before}`
          : ''
  return `On ${where}${between}.`
}

/** An order key in words, its first letter small to stand mid-sentence: subject (Dewey number). */
export function areaKeyText(k: string, first = true): string {
  const label = areaKeys.find((o) => o.value === k)?.label ?? k
  return first ? label : label.charAt(0).toLowerCase() + label.slice(1)
}

/**
 * Shelf numbers in words, runs gathered: shelf 5; shelves 1 and 2; shelves 1 to 4 and 6. The
 * numbers come in order, as a bookcase lists its shelves from the top.
 */
export function shelfNumbers(positions: readonly number[]): string {
  const runs: string[] = []
  let start = positions[0] ?? 0
  for (let i = 1; i <= positions.length; i++) {
    const prev = positions[i - 1] ?? 0
    if (positions[i] !== prev + 1) {
      // Two side by side read as two; three or more as a run.
      runs.push(
        ...(start === prev
          ? [`${start}`]
          : prev === start + 1
            ? [`${start}`, `${prev}`]
            : [`${start} to ${prev}`]),
      )
      start = positions[i] ?? 0
    }
  }
  return `${positions.length === 1 ? 'shelf' : 'shelves'} ${listed(runs)}`
}

/** An area's order in words: Subject (Dewey number), then author, then title. */
export function areaOrderText(order: readonly string[] | null | undefined): string {
  return (order ?? []).map((k, i) => areaKeyText(k, i === 0)).join(', then ')
}

/** What an area takes, in words, or that it takes whatever no area before it does. */
export function areaRulesText(a: Area, unit: Unit): string {
  const parts: string[] = []
  const kinds = (a.categories ?? []).map((c) => categories.find((k) => k.value === c)?.plural ?? c)
  if (kinds.length) {
    parts.push(listed(kinds, 'or'))
  }
  if (a.deweyFrom && a.deweyTo) {
    parts.push(`Dewey ${a.deweyFrom} to ${a.deweyTo}`)
  } else if (a.deweyFrom) {
    parts.push(`Dewey ${a.deweyFrom} and up`)
  } else if (a.deweyTo) {
    parts.push(`Dewey up to ${a.deweyTo}`)
  }
  if (a.subject) {
    parts.push(`subject headings with “${a.subject}”`)
  }
  if (a.authors?.length) {
    parts.push(`by ${listed(a.authors, 'or')}`)
  }
  if (a.series?.length) {
    parts.push(`in ${listed(a.series, 'or')}`)
  }
  if (a.minHeightMm && a.maxHeightMm) {
    parts.push(`${lengthIn(a.minHeightMm, unit)} to ${lengthIn(a.maxHeightMm, unit)} tall`)
  } else if (a.minHeightMm) {
    parts.push(`at least ${lengthIn(a.minHeightMm, unit)} tall`)
  } else if (a.maxHeightMm) {
    parts.push(`no taller than ${lengthIn(a.maxHeightMm, unit)}`)
  }
  if (!parts.length) {
    return 'Every book no area before it takes'
  }
  const text = parts.join('; ')
  return text.charAt(0).toUpperCase() + text.slice(1)
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
  if (!s.braceHeightMm || !s.braceWidthMm) {
    return size
  }
  return `${size}, braces ${inUnit(s.braceHeightMm, unit)} × ${inUnit(s.braceWidthMm, unit)} ${unit}`
}

/** Cents as dollars, or empty when the price is not known (-1). */
export function money(cents: number): string {
  if (cents < 0) {
    return ''
  }
  return (cents / 100).toLocaleString(undefined, { style: 'currency', currency: 'USD' })
}

const monthYear = new Intl.DateTimeFormat(undefined, { year: 'numeric', month: 'long' })
const fullDate = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' })

/** A date as precisely as it is known: 2014, June 2014, or Jun 2, 2014. */
export function calendarDate(value: string): string {
  const [y, m, d] = value.split('-').map(Number)
  if (!y) {
    return ''
  }
  if (!m) {
    return String(y)
  }
  if (!d) {
    return monthYear.format(new Date(y, m - 1, 1))
  }
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
  if (!due || due >= on) {
    return 0
  }
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
  if (list.length <= 1) {
    return list.join('')
  }
  if (list.length === 2) {
    return `${list[0]} ${and} ${list[1]}`
  }
  return `${list.slice(0, -1).join(', ')}, ${and} ${list[list.length - 1]}`
}

/** A number of things: "1 book", "3 books"; `many` where the plural is not one more s. */
export const count = (n: number, one: string, many = `${one}s`) => `${n} ${n === 1 ? one : many}`
