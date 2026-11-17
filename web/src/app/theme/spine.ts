import { channels, contrast } from './contrast'

// A book's spine on the Stacks: its cover's main colour, or its category's cloth for a book
// with no cover, made lighter or darker by as little as it takes for its lettering, in near
// black or white, to reach AAA's 7:1. A spine is the book's own, the same in either scheme.

/** The lettering on a spine: the darkest paper, and white. */
export const spineInk = { dark: '#12110f', light: '#ffffff' } as const

/** The cloth of a book with no cover, by its category: each reads in white at 7:1 as it is. */
export const categoryCloth: Record<string, string> = {
  fiction: '#852630',
  nonfiction: '#1f3a5f',
  textbook: '#2f5d3a',
  manga: '#8a3b12',
  light_novel: '#4b2e6b',
  graphic_novel: '#145e63',
  uncategorized: '#46423c',
}

/** The colours a spine is drawn in: its cloth, and the ink its title and author are in. */
export interface SpineColors {
  back: string
  ink: string
}

// A little over 7:1, so that rounding a mixed colour to whole channels never takes it under.
const aaa = 7.05

/**
 * A spine's colours from its cover's main colour (#rrggbb, or empty for none) and its category.
 * Of near black and white lettering, the one the cloth needs the smaller change for: the cloth
 * mixed towards white under dark ink, or towards black under white, until they reach 7:1.
 */
export function spineColors(main: string, category: string): SpineColors {
  const cloth = /^#[0-9a-f]{6}$/i.test(main)
    ? main.toLowerCase()
    : (categoryCloth[category] ?? categoryCloth.uncategorized ?? spineInk.dark)
  const lighter = towards(cloth, spineInk.light, spineInk.dark)
  const darker = towards(cloth, '#000000', spineInk.light)
  return lighter.by < darker.by ||
    (lighter.by === darker.by && contrast(cloth, spineInk.dark) > contrast(cloth, spineInk.light))
    ? { back: lighter.color, ink: spineInk.dark }
    : { back: darker.color, ink: spineInk.light }
}

// How far cloth must be mixed towards target, from 0 to 1, for ink on it to reach AAA, and the
// colour it makes. Mixing towards white only lightens and towards black only darkens, so the
// least that does it is found by halving.
function towards(cloth: string, target: string, ink: string): { by: number; color: string } {
  if (contrast(cloth, ink) >= aaa) {
    return { by: 0, color: cloth }
  }
  let [lo, hi] = [0, 1]
  for (let i = 0; i < 20; i++) {
    const mid = (lo + hi) / 2
    if (contrast(mix(cloth, target, mid), ink) >= aaa) {
      hi = mid
    } else {
      lo = mid
    }
  }
  return { by: hi, color: mix(cloth, target, hi) }
}

// a moved part of the way towards b, by the channels of each.
function mix(a: string, b: string, by: number): string {
  const [ca, cb] = [channels(a), channels(b)]
  return `#${ca
    .map((c, i) =>
      Math.round(c + ((cb[i] ?? c) - c) * by)
        .toString(16)
        .padStart(2, '0'),
    )
    .join('')}`
}
