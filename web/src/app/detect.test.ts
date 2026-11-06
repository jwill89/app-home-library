import { describe, expect, it } from 'vitest'
import { orderCorners } from './detect'

describe('orderCorners', () => {
  it('puts any four corners in order, clockwise from the top left', () => {
    const tl = { x: 120, y: 80 }
    const tr = { x: 410, y: 60 }
    const br = { x: 460, y: 520 }
    const bl = { x: 70, y: 540 }
    expect(orderCorners([br, tl, bl, tr])).toEqual([tl, tr, br, bl])
    expect(orderCorners([bl, br, tr, tl])).toEqual([tl, tr, br, bl])
  })
})
