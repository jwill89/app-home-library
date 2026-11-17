import { describe, expect, it } from 'vitest'
import { contrast } from '@/app/theme/contrast'
import { categoryCloth, spineColors, spineInk } from '@/app/theme/spine'
import { categories } from '@/constants'

// A colour a cover might be, written as the server sends it, from its channels: colours are
// written as such only in the theme.
const colour = (r: number, g: number, b: number) =>
  `#${[r, g, b].map((c) => c.toString(16).padStart(2, '0')).join('')}`
const [black, white, navy, paper, grey] = [
  colour(0, 0, 0),
  colour(255, 255, 255),
  colour(20, 30, 90),
  colour(249, 249, 249),
  colour(128, 128, 128),
]

describe('a spine', () => {
  it('reads at AAA whatever colour its cover is', () => {
    const covers = [grey, colour(119, 119, 119), colour(255, 0, 0), colour(0, 255, 0)]
    // And a spread of every hue and shade.
    for (let r = 0; r < 256; r += 51) {
      for (let g = 0; g < 256; g += 51) {
        for (let b = 0; b < 256; b += 51) {
          covers.push(colour(r, g, b))
        }
      }
    }
    for (const main of covers) {
      const { back, ink } = spineColors(main, 'fiction')
      expect(contrast(back, ink), `${main} became ${back}`).toBeGreaterThanOrEqual(7)
    }
  })

  it('keeps a cover colour that reads as it is, and changes one that does not by little', () => {
    expect(spineColors(navy.toUpperCase(), 'fiction')).toEqual({ back: navy, ink: spineInk.light })
    expect(spineColors(paper, 'fiction')).toEqual({ back: paper, ink: spineInk.dark })
    // A middling grey is darkened under white, or lightened under black, whichever is less.
    const { back, ink } = spineColors(grey, 'fiction')
    expect(back).not.toBe(grey)
    expect(contrast(back, ink)).toBeLessThan(7.5)
  })

  it('takes its category’s cloth without a cover, each reading in white as it is', () => {
    for (const c of categories) {
      const cloth = categoryCloth[c.value]
      expect(cloth, c.value).toBeDefined()
      expect(spineColors('', c.value)).toEqual({ back: cloth, ink: spineInk.light })
    }
    expect(spineColors('not a colour', 'no such kind').back).toBe(categoryCloth.uncategorized)
  })
})

describe('contrast', () => {
  it('is WCAG’s ratio, the lighter over the darker', () => {
    expect(contrast(black, white)).toBeCloseTo(21)
    expect(contrast(white, black)).toBeCloseTo(21)
    expect(contrast(grey, grey)).toBe(1)
  })
})
