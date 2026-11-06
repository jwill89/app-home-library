import { describe, expect, it } from 'vitest'
import { cite, html, parseName, plainText, type CiteStyle, type CiteWork } from './cite'

// Italic pieces are written between asterisks, so a test reads as the citation would look.
const shown = (w: CiteWork, s: CiteStyle) =>
  cite(w, s)
    .map((p) => (p.italic ? `*${p.text}*` : p.text))
    .join('')

const article: CiteWork = {
  kind: 'article',
  authors: ['Ana María Ruiz', "Ben O'Connor"],
  title: 'Teaching Calculus for Understanding',
  container: 'The Journal of Higher Education',
  volume: '70',
  issue: '6',
  pages: '623-649',
  published: '1999-11',
  doi: '10.1080/00221546.1999.11780773',
}

const book: CiteWork = {
  kind: 'book',
  authors: ['Ed McDonald'],
  title: 'Crowfall',
  publisher: 'Ace',
  place: 'New York',
  published: '2019-07-02',
  edition: 'First edition',
}

const chapter: CiteWork = {
  kind: 'chapter',
  authors: ['John D. Norton'],
  editors: ['Huw Price', 'Richard Corry'],
  title: 'Causation as Folk Science',
  container: 'Causation, Physics, and the Constitution of Reality',
  pages: '11-44',
  published: '2007',
  publisher: 'Oxford University Press',
}

describe('cite', () => {
  it('takes names apart as they are written', () => {
    expect(parseName('Ana María Ruiz')).toEqual({ family: 'Ruiz', given: 'Ana María' })
    expect(parseName('Ruiz, Ana María')).toEqual({ family: 'Ruiz', given: 'Ana María' })
    expect(parseName('Martin Luther King Jr.')).toEqual({
      family: 'King Jr.',
      given: 'Martin Luther',
    })
    expect(parseName('The Calculus Study Group').whole).toBe('The Calculus Study Group')
  })

  it("cites a chapter with its book's editors", () => {
    expect(shown(chapter, 'apa')).toBe(
      'Norton, J. D. (2007). Causation as Folk Science. In H. Price & R. Corry (Eds.), *Causation, Physics, and the Constitution of Reality* (pp. 11–44). Oxford University Press.',
    )
    expect(shown(chapter, 'mla')).toBe(
      'Norton, John D. “Causation as Folk Science.” *Causation, Physics, and the Constitution of Reality*, edited by Huw Price and Richard Corry, Oxford University Press, 2007, pp. 11–44.',
    )
    expect(shown(chapter, 'chicago')).toBe(
      'Norton, John D. “Causation as Folk Science.” In *Causation, Physics, and the Constitution of Reality*, edited by Huw Price and Richard Corry, 11–44. Oxford University Press, 2007.',
    )
    // One editor is "Ed."; MLA cuts three short; an article's editors are never cited.
    const one = { ...chapter, editors: ['Huw Price'] }
    expect(plainText(cite(one, 'apa'))).toContain('In H. Price (Ed.), ')
    const three = { ...chapter, editors: ['Huw Price', 'Richard Corry', 'Ann Other'] }
    expect(plainText(cite(three, 'mla'))).toContain('edited by Huw Price et al., Oxford')
    expect(plainText(cite(three, 'chicago'))).toContain(
      'edited by Huw Price, Richard Corry, and Ann Other, 11–44.',
    )
    expect(plainText(cite({ ...article, editors: ['Ed Itor'] }, 'chicago'))).not.toContain('edited')
  })

  it('cites an article in APA, MLA, and Chicago', () => {
    expect(shown(article, 'apa')).toBe(
      "Ruiz, A. M., & O'Connor, B. (1999). Teaching Calculus for Understanding. *The Journal of Higher Education*, *70*(6), 623–649. https://doi.org/10.1080/00221546.1999.11780773",
    )
    expect(shown(article, 'mla')).toBe(
      "Ruiz, Ana María, and Ben O'Connor. “Teaching Calculus for Understanding.” *The Journal of Higher Education*, vol. 70, no. 6, 1999, pp. 623–649. https://doi.org/10.1080/00221546.1999.11780773.",
    )
    expect(shown(article, 'chicago')).toBe(
      "Ruiz, Ana María, and Ben O'Connor. “Teaching Calculus for Understanding.” *The Journal of Higher Education* 70, no. 6 (1999): 623–649. https://doi.org/10.1080/00221546.1999.11780773.",
    )
  })

  it('cites a book, leaving out a first edition', () => {
    expect(shown(book, 'apa')).toBe('McDonald, E. (2019). *Crowfall*. Ace.')
    expect(shown(book, 'mla')).toBe('McDonald, Ed. *Crowfall*. Ace, 2019.')
    expect(shown(book, 'chicago')).toBe('McDonald, Ed. *Crowfall*. New York: Ace, 2019.')
    const second = { ...book, edition: '2nd edition' }
    expect(shown(second, 'apa')).toBe('McDonald, E. (2019). *Crowfall* (2nd ed.). Ace.')
    expect(shown(second, 'chicago')).toBe('McDonald, Ed. *Crowfall*. 2nd ed. New York: Ace, 2019.')
  })

  it('leads with the title when there is no author, and says n.d. with no date', () => {
    const handout: CiteWork = { kind: 'document', authors: [], title: 'Calculus III syllabus' }
    expect(shown(handout, 'apa')).toBe('*Calculus III syllabus*. (n.d.).')
    expect(shown(handout, 'mla')).toBe('*Calculus III syllabus*.')
    expect(shown({ ...handout, title: 'Why limits?' }, 'chicago')).toBe('*Why limits?*')
  })

  it('shortens long author lists as each style does', () => {
    const many = { ...article, authors: ['A One', 'B Two', 'C Three'] }
    expect(plainText(cite(many, 'apa'))).toMatch(/^One, A., Two, B., & Three, C. \(1999\)/)
    expect(plainText(cite(many, 'mla'))).toMatch(/^One, A, et al\. /)
    expect(plainText(cite(many, 'chicago'))).toMatch(/^One, A, B Two, and C Three\. /)
  })

  it('carries italics to the clipboard as HTML, escaped', () => {
    expect(html([{ text: 'A & B ' }, { text: '<Title>', italic: true }])).toBe(
      'A &amp; B <i>&lt;Title&gt;</i>',
    )
  })
})
