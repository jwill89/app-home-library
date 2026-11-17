import { describe, expect, it } from 'vitest'
import type { Area, BookRow } from '@/types'
import {
  arrange,
  byShelf,
  shelfRange,
  shelvesOf,
  spineHeight,
  spineMin,
  spineWidth,
  surname,
} from '@/app/stacks'

let next = 1
const book = (over: Partial<BookRow>): BookRow => ({
  id: next++,
  workId: 1,
  title: '',
  subtitle: '',
  authors: '',
  series: '',
  seriesId: 0,
  hadAs: 0,
  category: 'fiction',
  seriesNumber: '',
  isbn13: '',
  format: '',
  ddc: '',
  lcc: '',
  published: '',
  publisher: '',
  readStatus: 'unread',
  sections: '',
  cover: { id: '', widths: [], color: '' },
  wantedOn: '',
  copies: 1,
  barcode: '',
  addedAt: '',
  subjects: [],
  genres: '',
  credits: '',
  callNumber: [],
  shelfKey: '',
  authorSort: '',
  heightMm: 210,
  thicknessMm: 25,
  sizeEstimated: false,
  ...over,
})

const titles = (books: readonly BookRow[] | undefined) => (books ?? []).map((b) => b.title)

describe('a spine', () => {
  it('is drawn to the book, at one scale, never narrower than the target size', () => {
    expect(spineHeight(book({ heightMm: 232 }))).toBe(290)
    expect(spineWidth(book({ thicknessMm: 12 }))).toBe(spineMin)
    expect(spineWidth(book({ thicknessMm: 50 }))).toBe(63)
  })

  it('names its author by the surname the book is filed under', () => {
    expect(surname(book({ authorSort: 'Austen, Jane', authors: 'Jane Austen' }))).toBe('Austen')
    expect(surname(book({ authorSort: '', authors: 'Homer, Emily Wilson' }))).toBe('Homer')
    expect(surname(book({}))).toBe('')
  })
})

describe('arranging the stacks', () => {
  const raven = "Raven's Mark"
  const emma = book({ title: 'Emma', shelfKey: 'b', authorSort: 'Austen, Jane' })
  const blink = book({
    title: 'Blink',
    category: 'nonfiction',
    shelfKey: 'a',
    authorSort: 'Gladwell, Malcolm',
  })
  const crowfall = book({
    title: 'Crowfall',
    shelfKey: 'c',
    authorSort: 'McDonald, Ed',
    series: raven,
    seriesNumber: '10',
  })
  const blackwing = book({
    title: 'Blackwing',
    shelfKey: 'c0',
    authorSort: 'McDonald, Ed',
    series: raven,
    seriesNumber: '2',
  })
  const beowulf = book({ title: 'Beowulf', shelfKey: 'a0' })
  const all = [crowfall, emma, blink, blackwing, beowulf]

  it('stands a bookcase for each category, in their own order, each in shelf order', () => {
    const cases = arrange(all, 'category')
    expect(cases.map((c) => c.title)).toEqual(['Fiction', 'Nonfiction'])
    expect(titles(cases[0]?.books)).toEqual(['Beowulf', 'Emma', 'Crowfall', 'Blackwing'])
  })

  it('stands every book in one run by call number, title, or author', () => {
    expect(titles(arrange(all, 'call-number')[0]?.books)).toEqual([
      'Blink',
      'Beowulf',
      'Emma',
      'Crowfall',
      'Blackwing',
    ])
    expect(titles(arrange(all, 'title')[0]?.books)).toEqual([
      'Beowulf',
      'Blackwing',
      'Blink',
      'Crowfall',
      'Emma',
    ])
    // An author's series in volume order, 2 before 10; a book with no author after the rest.
    expect(titles(arrange(all, 'author')[0]?.books)).toEqual([
      'Emma',
      'Blink',
      'Blackwing',
      'Crowfall',
      'Beowulf',
    ])
    expect(arrange([], 'title')).toEqual([])
  })

  it('stands a bookcase for each area in its order, and the books in no area after', () => {
    const area = (id: number, name: string, bookIds: number[]) => ({ id, name, bookIds }) as Area
    const cases = arrange(all, 'area', [
      area(1, 'Raven', [crowfall.id, blackwing.id, 9999]),
      area(2, 'Empty', []),
    ])
    expect(cases.map((c) => [c.title, titles(c.books)])).toEqual([
      ['Raven', ['Crowfall', 'Blackwing']],
      ['In no area', ['Blink', 'Beowulf', 'Emma']],
    ])
  })

  it('orders by the shelf key as text, character by character', () => {
    expect(byShelf(book({ shelfKey: 'B' }), book({ shelfKey: 'a' }))).toBeLessThan(0)
    expect(byShelf(book({ shelfKey: 'b' }), book({ shelfKey: 'a' }))).toBeGreaterThan(0)
    const [x, y] = [book({ shelfKey: 'same' }), book({ shelfKey: 'same' })]
    expect(byShelf(x, y)).toBeLessThan(0)
  })
})

describe('shelves', () => {
  it('take as many as fit with the gap between, and the rest go on the next', () => {
    const wide = (w: number) => w
    expect(shelvesOf([40, 40, 40, 40], 124, wide, 2)).toEqual([[40, 40, 40], [40]])
    expect(shelvesOf([200, 40], 100, wide, 2)).toEqual([[200], [40]])
    expect(shelvesOf([], 100, wide)).toEqual([])
  })

  it('say what they hold, as the arrangement names a book', () => {
    const first = book({
      title: 'Emma',
      callNumber: ['823.7', 'AUS', 'v.1'],
      authorSort: 'Austen, Jane',
    })
    const last = book({
      title: 'Wuthering Heights',
      callNumber: ['823.8', 'BRO'],
      authorSort: 'Brontë, Emily',
    })
    expect(shelfRange([first, last], 'call-number')).toBe('823.7 AUS to 823.8 BRO')
    expect(shelfRange([first, last], 'author')).toBe('Austen to Brontë')
    expect(shelfRange([first, last], 'title')).toBe('Emma to Wuthering Heights')
    expect(shelfRange([first], 'category')).toBe('823.7 AUS')
    expect(shelfRange([book({ title: 'Untitled' })], 'call-number')).toBe('Untitled')
    expect(shelfRange([], 'title')).toBe('')
  })
})
