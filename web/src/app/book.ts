import type { Book, BookInput, CopyInput, Proposal, Suggestions } from '@/api/client'
import type { CiteWork } from './cite'
import type { Crumb } from './navigation'

// The form's shapes: a blank book, a book from a lookup, a saved book back into the form,
// and the sources rewritten for what the owner typed.

export function emptyBook(): BookInput {
  return {
    title: '',
    subtitle: '',
    contributors: [{ name: '', role: 'author' }],
    series: '',
    seriesNumber: '',
    category: '',
    readStatus: 'unread',
    currentPage: 0,
    startedOn: '',
    finishedOn: '',
    rating: 0,
    notes: '',
    genres: [],
    isbn: '',
    lccn: '',
    publisher: '',
    publishPlace: '',
    published: '',
    editionStatement: '',
    format: '',
    pages: 0,
    heightMm: 0,
    widthMm: 0,
    thicknessMm: 0,
    language: '',
    ddc: '',
    lcc: '',
    subjects: [],
    description: '',
    sources: {},
  }
}

export function emptyCopy(section = ''): CopyInput {
  return {
    section,
    acquiredAs: 'unknown',
    acquiredOn: '',
    acquiredFrom: '',
    priceCents: -1,
    condition: 'unknown',
    dustJacket: 'unknown',
    signed: false,
    location: '',
    notes: '',
  }
}

export function bookFromProposal(p: Proposal): BookInput {
  return {
    ...emptyBook(),
    title: p.title,
    subtitle: p.subtitle,
    contributors: p.contributors?.length ? p.contributors : [{ name: '', role: 'author' }],
    series: p.series,
    seriesNumber: p.seriesNumber,
    isbn: p.isbn13,
    lccn: p.lccn,
    publisher: p.publisher,
    publishPlace: p.publishPlace,
    published: p.published,
    editionStatement: p.editionStatement,
    format: p.format,
    pages: p.pages,
    heightMm: p.heightMm,
    widthMm: p.widthMm,
    thicknessMm: p.thicknessMm,
    language: p.language,
    ddc: p.ddc,
    lcc: p.lcc,
    subjects: p.subjects ?? [],
    description: p.description,
    sources: { ...p.sources },
  }
}

/** The pages above a book's: the library, or the wishlist for one wanted, and its series. */
export function bookTrail(b: { series: string; seriesId: number; wantedOn?: string }): Crumb[] {
  if (b.wantedOn) {
    return [{ label: 'Wishlist', to: '/wishlist' }]
  }
  const trail: Crumb[] = [{ label: 'Library', to: '/' }]
  if (b.seriesId) {
    trail.push({ label: b.series, to: `/series/${b.seriesId}` })
  }
  return trail
}

export function bookToInput(b: Book): BookInput {
  return {
    title: b.title,
    subtitle: b.subtitle,
    contributors: b.contributors?.length ? b.contributors.map((c) => ({ ...c })) : [],
    series: b.series,
    seriesNumber: b.seriesNumber,
    category: b.category,
    readStatus: b.readStatus as BookInput['readStatus'],
    currentPage: b.currentPage,
    startedOn: b.startedOn,
    finishedOn: b.finishedOn,
    rating: b.rating,
    notes: b.notes,
    genres: [...(b.genres ?? [])],
    isbn: b.isbn13,
    lccn: b.lccn,
    publisher: b.publisher,
    publishPlace: b.publishPlace,
    published: b.published,
    editionStatement: b.editionStatement,
    format: b.format,
    pages: b.pages,
    heightMm: b.heightMm,
    widthMm: b.widthMm,
    thicknessMm: b.thicknessMm,
    language: b.language,
    ddc: b.ddc,
    lcc: b.lcc,
    subjects: [...(b.subjects ?? [])],
    description: b.description,
    sources: { ...b.sources },
  }
}

// The fields a lookup can fill, by their names in BookInput, as the server names them.
type LookupField = NonNullable<Suggestions['fields']>[number]
const lookupFields = [
  'title',
  'subtitle',
  'contributors',
  'series',
  'seriesNumber',
  'lccn',
  'publisher',
  'publishPlace',
  'published',
  'editionStatement',
  'format',
  'pages',
  'heightMm',
  'widthMm',
  'thicknessMm',
  'language',
  'ddc',
  'lcc',
  'description',
] as const satisfies readonly LookupField[]
// A field the server adds and the list above lacks is a type error here.
type Listed<T extends true> = T
export type EveryLookupFieldListed = Listed<
  [Exclude<LookupField, (typeof lookupFields)[number]>] extends [never] ? true : false
>

// The fields a source is recorded for: those, and the two a lookup only adds to.
const tracked = [...lookupFields, 'category', 'subjects'] as const

/**
 * The input ready to send: blank contributors dropped, and every field that differs from
 * where it started marked as the owner's, so no later lookup takes it for a blank to fill.
 */
export function prepare(start: BookInput, edited: BookInput): BookInput {
  const out: BookInput = {
    ...edited,
    contributors: (edited.contributors ?? []).filter((c) => c.name.trim() !== ''),
    sources: { ...edited.sources },
  }
  const sources = out.sources
  for (const field of tracked) {
    if (JSON.stringify(start[field]) !== JSON.stringify(edited[field])) {
      sources[field] = 'owner'
    }
  }
  out.sources = sources
  return out
}

/** The work a book's citation is made from. */
export function bookCite(b: Book): CiteWork {
  const people = b.contributors ?? []
  return {
    kind: 'book',
    authors: people.filter((c) => c.role === 'author').map((c) => c.name),
    title: b.subtitle ? `${b.title}: ${b.subtitle}` : b.title,
    publisher: b.publisher,
    place: b.publishPlace,
    published: b.published,
    edition: b.editionStatement,
  }
}
