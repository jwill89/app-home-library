import { describe, expect, it } from 'vitest'
import type { BookRow } from '@/api/client'
import { matching } from './library'

const row = (over: Partial<BookRow>): BookRow => ({
  id: 1,
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
  cover: { id: '', widths: [] },
  wantedOn: '',
  copies: 1,
  barcode: '',
  addedAt: '',
  subjects: [],
  genres: '',
  credits: '',
  ...over,
})

describe('library search', () => {
  it('finds a book by its subjects, genres, translators, and class', () => {
    const odyssey = row({
      title: 'The Odyssey',
      authors: 'Homer',
      credits: 'Emily Wilson',
      subjects: ['Epic poetry, Greek', 'Odysseus (Greek mythology) -- Poetry'],
      genres: 'Classics',
      lcc: 'PA4025.A5',
    })
    for (const q of ['epic greek', 'mythology', 'wilson', 'classics', 'pa4025', 'homer odyssey']) {
      expect(matching(q)(odyssey), q).toBe(true)
    }
    expect(matching('romance')(odyssey)).toBe(false)
  })
})
