import { describe, expect, it } from 'vitest'
import { emptyBook, prepare } from './book'

describe('prepare', () => {
  it('marks what the owner changed and drops blank people', () => {
    const start = {
      ...emptyBook(),
      title: 'Blood song',
      ddc: '823.92',
      sources: { title: 'loc', ddc: 'loc' },
    }
    const edited = {
      ...start,
      title: 'Blood Song',
      contributors: [
        { name: 'Anthony Ryan', role: 'author' as const },
        { name: '  ', role: 'author' as const },
      ],
    }
    const out = prepare(start, edited)
    expect(out.sources?.title).toBe('owner')
    expect(out.sources?.ddc).toBe('loc')
    expect(out.sources?.contributors).toBe('owner')
    expect(out.contributors).toHaveLength(1)
  })
})
