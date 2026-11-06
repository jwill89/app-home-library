import { describe, expect, it } from 'vitest'
import { isbn13 } from './isbn'

describe('isbn13', () => {
  it('reads ISBN-10s and ISBN-13s as the server does', () => {
    expect(isbn13('0425268284')).toBe('9780425268285')
    expect(isbn13('978-0-425-26828-5')).toBe('9780425268285')
    expect(isbn13('080442957X')).toBe('9780804429573')
  })

  it('refuses what does not check out, or is not an ISBN', () => {
    expect(isbn13('0425268285')).toBe('')
    expect(isbn13('9780425268286')).toBe('')
    expect(isbn13('070330757855')).toBe('')
    expect(isbn13('HL00042')).toBe('')
  })
})
