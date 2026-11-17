import type { CiteStyle } from '@/app/cite'
import type { Unit } from '@/app/format'
import type {
  AreaInput,
  Book,
  BookInput,
  Contributor,
  CopyInput,
  CourseInput,
  DocumentFile,
  DocumentInput,
  LogLevel,
  QueuedLabel,
  Scan,
  ShelvingRule,
} from '@/types'

// The app's fixed lists of choices, each a value the server or the device keeps and the words
// shown for it. Each list's values are typed by the field it is for, so a value the server
// does not take is a type error. The words for one value come from app/format.ts.

/**
 * The kinds of book the library sorts by: one each, set, or worked out from its details (a
 * textbook, from a copy of it on a course).
 */
export const categories = [
  { value: 'fiction', label: 'Fiction', plural: 'Fiction' },
  { value: 'nonfiction', label: 'Nonfiction', plural: 'Nonfiction' },
  { value: 'textbook', label: 'Textbook', plural: 'Textbooks' },
  { value: 'manga', label: 'Manga', plural: 'Manga' },
  { value: 'light_novel', label: 'Light novel', plural: 'Light novels' },
  { value: 'graphic_novel', label: 'Graphic novel', plural: 'Graphic novels' },
  { value: 'uncategorized', label: 'Uncategorized', plural: 'Uncategorized' },
] as const satisfies readonly { value: Book['category']; label: string; plural: string }[]

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

export const scanStatuses: { value: Scan['status']; label: string }[] = [
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

export const labelKinds: { value: QueuedLabel['kind']; label: string; short: string }[] = [
  { value: 'barcode', label: 'Barcode labels', short: 'barcode' },
  { value: 'spine', label: 'Spine labels', short: 'spine' },
]

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

export const textStatuses: { value: DocumentFile['textStatus']; label: string }[] = [
  { value: 'pending', label: 'Waiting to be read' },
  { value: 'done', label: 'Text read' },
  { value: 'empty', label: 'No text found' },
  { value: 'failed', label: 'Could not be read' },
  { value: 'owner', label: 'Text typed by you' },
]

type AreaKey = NonNullable<AreaInput['order']>[number]

/** What an area can order its books by, in words, as its dialog offers them. */
export const areaKeys = [
  { value: 'subject', label: 'Subject (Dewey number)' },
  { value: 'author', label: 'Author' },
  { value: 'series', label: 'Series and volume' },
  { value: 'title', label: 'Title' },
  { value: 'published', label: 'Date published' },
  { value: 'height', label: 'Height' },
] as const satisfies readonly { value: AreaKey; label: string }[]

export const citeStyles: { value: CiteStyle; label: string }[] = [
  { value: 'apa', label: 'APA' },
  { value: 'mla', label: 'MLA' },
  { value: 'chicago', label: 'Chicago' },
]

export const schemes = [
  { value: 'device', label: 'Match this device' },
  { value: 'light', label: 'Light' },
  { value: 'dark', label: 'Dark' },
] as const

export const unitOptions: { value: Unit; label: string }[] = [
  { value: 'in', label: 'Inches' },
  { value: 'cm', label: 'Centimetres' },
]

/** The four levels, from the most the log can hold to the least. */
export const logLevels: { value: LogLevel; label: string }[] = [
  { value: 'debug', label: 'Everything (debug)' },
  { value: 'info', label: 'What it does (info)' },
  { value: 'warn', label: 'Problems (warn)' },
  { value: 'error', label: 'Failures only (error)' },
]
