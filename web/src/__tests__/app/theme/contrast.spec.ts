import { describe, expect, it } from 'vitest'
import { contrast } from '@/app/theme/contrast'
import { schemes, type Scheme } from '@/app/theme/preset'

// WCAG 2.2 AAA, computed from the scheme colors the preset is built from. A pair the pages
// put together is listed here; a new pair is added in the same change that uses it.

type Key = keyof Scheme
const TEXT = 7 // 1.4.6, body text
const EDGE = 3 // 1.4.11, control boundaries, focus rings, states, meaningful icons

// Text and the surfaces it is set on.
const text: [Key, Key[]][] = [
  ['ink', ['page', 'panel', 'sunken', 'raised', 'highlight']],
  ['inkMuted', ['page', 'panel', 'sunken', 'raised', 'highlight']],
  ['accent', ['page', 'panel', 'raised']],
  ['danger', ['page', 'panel', 'raised']],
  ['success', ['page', 'panel', 'raised']],
  ['highlightInk', ['highlight']],
  ['accentContrast', ['accent', 'accentHover']],
  ['woodInk', ['wood']],
  ['dangerContrast', ['danger', 'dangerHover']],
]

// Edges and states and what they sit against.
const edges: [Key, Key[]][] = [
  ['outline', ['page', 'panel', 'sunken']],
  ['accent', ['page', 'panel', 'raised', 'shelfBack']], // the focus ring; a chosen segment beside the others
  // A switch says on or off by where its handle sits, so the handle is what must stand out
  // from the track, off and on; the two track colors need not differ from each other by 3:1.
  ['panel', ['outline', 'accent']],
]

describe.each(['light', 'dark'] as const)('%s scheme', (name) => {
  const s = schemes[name]
  for (const [fg, bgs] of text) {
    for (const bg of bgs) {
      it(`${fg} on ${bg} is at least ${TEXT}:1`, () => {
        expect(contrast(s[fg], s[bg])).toBeGreaterThanOrEqual(TEXT)
      })
    }
  }
  for (const [fg, bgs] of edges) {
    for (const bg of bgs) {
      it(`${fg} against ${bg} is at least ${EDGE}:1`, () => {
        expect(contrast(s[fg], s[bg])).toBeGreaterThanOrEqual(EDGE)
      })
    }
  }
})
