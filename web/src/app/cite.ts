import { listed } from './format'

// Citations in the three styles a school asks for, from a book or a document. A citation is a
// list of pieces, each italic or not, so the page can show it and the clipboard can carry it
// with its italics.

export type CiteStyle = 'apa' | 'mla' | 'chicago'

export const citeStyles: { value: CiteStyle; label: string }[] = [
  { value: 'apa', label: 'APA' },
  { value: 'mla', label: 'MLA' },
  { value: 'chicago', label: 'Chicago' },
]

export interface CiteWork {
  kind: 'book' | 'article' | 'chapter' | 'document'
  authors: string[]
  /** A chapter's book's editors. */
  editors?: string[]
  title: string
  container?: string
  volume?: string
  issue?: string
  pages?: string
  published?: string
  publisher?: string
  place?: string
  edition?: string
  doi?: string
  url?: string
}

interface Piece {
  text: string
  italic?: boolean
}

interface Name {
  family: string
  given: string
  /** A body (a group, a department) cited as written, never turned around. */
  whole?: string
}

const suffixes = new Set(['jr.', 'jr', 'sr.', 'sr', 'ii', 'iii', 'iv'])
const bodies =
  /\b(group|association|society|committee|department|university|institute|council|board|office|press)\b/i

/** A name as written, "Ana María Ruiz" or "Ruiz, Ana María", taken apart. */
export function parseName(raw: string): Name {
  const name = raw.trim()
  if (bodies.test(name)) return { family: name, given: '', whole: name }
  if (name.includes(',')) {
    const [family = '', ...rest] = name.split(',')
    const given = rest.join(',').trim()
    if (!suffixes.has(given.toLowerCase())) return { family: family.trim(), given }
  }
  const words = name.split(/\s+/).filter(Boolean)
  let last = words.pop() ?? ''
  if (suffixes.has(last.toLowerCase()) && words.length > 1) last = `${words.pop()} ${last}`
  return { family: last, given: words.join(' ') }
}

const initials = (given: string) =>
  given
    .split(/[\s-]+/)
    .filter(Boolean)
    .map((g) => `${g[0]?.toUpperCase()}.`)
    .join(' ')

const inverted = (n: Name) => n.whole ?? (n.given ? `${n.family}, ${n.given}` : n.family)
const natural = (n: Name) => n.whole ?? [n.given, n.family].filter(Boolean).join(' ')
const apaName = (n: Name) => n.whole ?? (n.given ? `${n.family}, ${initials(n.given)}` : n.family)

function apaAuthors(names: Name[]): string {
  const list = names.map(apaName)
  if (list.length > 20) return `${list.slice(0, 19).join(', ')}, … ${list[list.length - 1]}`
  if (list.length <= 1) return list.join('')
  return `${list.slice(0, -1).join(', ')}, & ${list[list.length - 1]}`
}

function mlaAuthors(names: Name[]): string {
  const [first, second] = names
  if (!first) return ''
  if (names.length >= 3) return `${inverted(first)}, et al.`
  return second ? `${inverted(first)}, and ${natural(second)}` : inverted(first)
}

function chicagoAuthors(names: Name[]): string {
  const [first, ...rest] = names
  if (!first) return ''
  if (names.length > 10)
    return `${[first, ...rest.slice(0, 6)].map((n, i) => (i ? natural(n) : inverted(n))).join(', ')}, et al.`
  const parts = [inverted(first), ...rest.map(natural)]
  if (parts.length === 1) return parts.join('')
  return `${parts.slice(0, -1).join(', ')}, and ${parts[parts.length - 1]}`
}

/** APA's editors, initials first: "H. Price & R. Corry (Eds.)". */
const apaEditors = (eds: Name[]) =>
  `${listed(
    eds.map((n) => n.whole ?? [initials(n.given), n.family].filter(Boolean).join(' ')),
    '&',
  )} (${eds.length === 1 ? 'Ed.' : 'Eds.'})`

/** MLA's and Chicago's "edited by Huw Price and Richard Corry"; MLA cuts three or more short. */
const editedBy = (eds: Name[], short: boolean) =>
  `edited by ${short && eds.length >= 3 ? `${natural(eds[0])} et al.` : listed(eds.map(natural))}`

const ended = (s: string) => /[.?!]$/.test(s.trim())
/** Ends a sentence once: no full stop after a question mark or one already there. */
const stop = (s: string) => (ended(s) ? s.trim() : `${s.trim()}.`)
const year = (w: CiteWork) => w.published?.slice(0, 4) ?? ''
const range = (p?: string) => (p ? p.replace(/\s*[-–]+\s*/g, '–') : '')
const link = (w: CiteWork) => (w.doi ? `https://doi.org/${w.doi}` : (w.url ?? ''))
const edition = (e?: string) => {
  const t = (e ?? '').trim()
  if (!t || /^(1st|first)\b/i.test(t)) return ''
  return /\bed\.$/i.test(t) ? t : `${t.replace(/\s*edition$/i, '')} ed.`
}

type Part = string | Piece | false | undefined | null | 0

/** Pieces from parts, italic where asked, with the empties dropped. */
function pieces(...parts: (Part | Part[])[]): Piece[] {
  const out: Piece[] = []
  for (const p of parts.flat()) {
    if (!p) continue
    const piece = typeof p === 'string' ? { text: p } : p
    if (piece.text) out.push(piece)
  }
  return out
}

/** A title and the full stop after it: outside the italics, and not after a question mark. */
const titled = (text: string, italic: boolean): Part[] =>
  italic ? [{ text, italic: true }, !ended(text) && '.'] : [stop(text)]

function apa(w: CiteWork, names: Name[], eds: Name[]): Piece[] {
  const date = `(${year(w) || 'n.d.'}).`
  const italic = w.kind === 'book' || w.kind === 'document'
  const ed = edition(w.edition)
  const title =
    italic && ed ? [{ text: w.title, italic: true }, ` (${ed}).`] : titled(w.title, italic)
  // With no author the title leads, and the date follows it.
  const lead = names.length
    ? [stop(apaAuthors(names)), ' ', date, ' ', ...title]
    : [...title, ' ', date]
  const end = link(w) && ` ${link(w)}`
  if (w.kind === 'article') {
    return pieces(
      lead,
      ' ',
      { text: w.container ?? '', italic: true },
      w.volume && [', ', { text: w.volume, italic: true }, w.issue && `(${w.issue})`],
      w.pages && `, ${range(w.pages)}`,
      '.',
      end,
    )
  }
  if (w.kind === 'chapter') {
    return pieces(
      lead,
      ' In ',
      eds.length && `${apaEditors(eds)}, `,
      { text: w.container ?? '', italic: true },
      w.pages && ` (pp. ${range(w.pages)})`,
      '.',
      w.publisher && ` ${stop(w.publisher)}`,
      end,
    )
  }
  return pieces(lead, w.publisher && ` ${stop(w.publisher)}`, end)
}

function mla(w: CiteWork, names: Name[], eds: Name[]): Piece[] {
  const who = names.length && `${stop(mlaAuthors(names))} `
  const end = w.doi ? ` https://doi.org/${w.doi}.` : w.url ? ` ${w.url}.` : ''
  if (w.kind === 'article' || w.kind === 'chapter') {
    const tail = [
      w.kind === 'chapter' && eds.length && editedBy(eds, true),
      w.volume && `vol. ${w.volume}`,
      w.issue && `no. ${w.issue}`,
      w.kind === 'chapter' && w.publisher,
      year(w),
      w.pages && `pp. ${range(w.pages)}`,
    ].filter(Boolean)
    return pieces(
      who,
      `“${stop(w.title)}” `,
      { text: w.container ?? '', italic: true },
      tail.length ? `, ${stop(tail.join(', '))}` : '.',
      end,
    )
  }
  const tail = [edition(w.edition), w.publisher, year(w)].filter(Boolean)
  return pieces(who, titled(w.title, true), tail.length && ` ${tail.join(', ')}.`, end)
}

function chicago(w: CiteWork, names: Name[], eds: Name[]): Piece[] {
  const who = names.length && `${stop(chicagoAuthors(names))} `
  const press = w.place && w.publisher ? `${w.place}: ${w.publisher}` : w.publisher
  const imprint = [press, year(w)].filter(Boolean).join(', ')
  const end = link(w) && ` ${link(w)}.`
  if (w.kind === 'article') {
    return pieces(
      who,
      `“${stop(w.title)}” `,
      { text: w.container ?? '', italic: true },
      w.volume && ` ${w.volume}`,
      w.issue && `, no. ${w.issue}`,
      year(w) && ` (${year(w)})`,
      w.pages ? `: ${range(w.pages)}.` : '.',
      end,
    )
  }
  if (w.kind === 'chapter') {
    return pieces(
      who,
      `“${stop(w.title)}” In `,
      { text: w.container ?? '', italic: true },
      eds.length && `, ${editedBy(eds, false)}`,
      w.pages && `, ${range(w.pages)}`,
      '.',
      imprint && ` ${stop(imprint)}`,
      end,
    )
  }
  const ed = edition(w.edition)
  return pieces(who, titled(w.title, true), ed && ` ${ed}`, imprint && ` ${stop(imprint)}`, end)
}

/** The citation of a work in a style. */
export function cite(w: CiteWork, style: CiteStyle): Piece[] {
  const names = w.authors.filter((a) => a.trim()).map(parseName)
  const eds = w.kind === 'chapter' ? (w.editors ?? []).filter((e) => e.trim()).map(parseName) : []
  const out =
    style === 'apa'
      ? apa(w, names, eds)
      : style === 'mla'
        ? mla(w, names, eds)
        : chicago(w, names, eds)
  return out.map((p) => ({ ...p, text: p.text.replace(/\s{2,}/g, ' ') }))
}

export const plainText = (ps: Piece[]) => ps.map((p) => p.text).join('')

const escape = (s: string) =>
  s.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c] ?? c)

/** The citation as HTML for the clipboard, so a word processor keeps the italics. */
export const html = (ps: Piece[]) =>
  ps.map((p) => (p.italic ? `<i>${escape(p.text)}</i>` : escape(p.text))).join('')
