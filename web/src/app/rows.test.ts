import { describe, expect, it } from 'vitest'
import { refreshRows } from './rows'

interface Row {
  id: number
  title: string
}

describe('refreshRows', () => {
  it('adds, removes, and replaces only the rows the feed named', () => {
    const kept = { id: 1, title: 'Kept' }
    const same = { id: 2, title: 'Same' }
    const list: Row[] = [kept, same, { id: 3, title: 'Gone' }, { id: 4, title: 'Old' }]
    const answer = [
      { id: 2, title: 'Same' },
      { id: 4, title: 'New title' },
      { id: 5, title: 'Added' },
    ]
    expect(refreshRows(list, new Set([2, 3, 4, 5]), answer, (r) => r.id)).toBe(true)
    expect(list.map((r) => r.title)).toEqual(['Kept', 'Same', 'New title', 'Added'])
    // A row unchanged is the same object still: the table has nothing to draw again.
    expect(list[0]).toBe(kept)
    expect(list[1]).toBe(same)
  })

  it('says when nothing changed', () => {
    const list: Row[] = [{ id: 1, title: 'A' }]
    expect(refreshRows(list, new Set([1, 9]), [{ id: 1, title: 'A' }], (r) => r.id)).toBe(false)
  })
})
