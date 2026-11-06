import { describe, expect, it } from 'vitest'
import { librarySearchLink, workWords } from './libraryLink'

describe('librarySearchLink', () => {
  it('turns an EBSCO search page into its results for the words', () => {
    expect(librarySearchLink('https://research.ebsco.com/c/xun5sx/search', '"Mind" Ryle')).toBe(
      'https://research.ebsco.com/c/xun5sx/search/results?q=%22Mind%22%20Ryle',
    )
    expect(
      librarySearchLink('https://research.ebsco.com/c/xun5sx/search/results?q=old&db=a9h', 'new'),
    ).toBe('https://research.ebsco.com/c/xun5sx/search/results?q=new')
  })

  it('fills {query} in any other https search', () => {
    expect(librarySearchLink('https://catalog.example.org/find?q={query}', 'a b')).toBe(
      'https://catalog.example.org/find?q=a%20b',
    )
  })

  it('gives nothing for a link it cannot search with', () => {
    expect(librarySearchLink('', 'x')).toBe('')
    expect(librarySearchLink('not a link', 'x')).toBe('')
    expect(librarySearchLink('http://catalog.example.org/find?q={query}', 'x')).toBe('')
    expect(librarySearchLink('https://catalog.example.org/find', 'x')).toBe('')
  })
})

describe('workWords', () => {
  it('searches the main title as a phrase, with the first author', () => {
    expect(workWords('Theory Choice: A Study', ['Thomas S. Kuhn', 'Ian Hacking'])).toBe(
      '"Theory Choice" Kuhn',
    )
    expect(workWords('On “Quotes”')).toBe('"On Quotes"')
    // The surname a name is filed under, however the name is written.
    expect(workWords('Why We Can’t Wait', ['Martin Luther King Jr.'])).toBe(
      '"Why We Can’t Wait" King',
    )
    expect(workWords('A Treatise of Human Nature (Penguin)', ['Hume, David'])).toBe(
      '"A Treatise of Human Nature" Hume',
    )
  })
})
