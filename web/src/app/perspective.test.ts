import { describe, expect, it } from 'vitest'
import { outputSize, project, projection, warp, type Quad } from './perspective'

const rect = (w: number, h: number): Quad => [
  { x: 0, y: 0 },
  { x: w, y: 0 },
  { x: w, y: h },
  { x: 0, y: h },
]

describe('perspective', () => {
  it('maps the unit square onto a rectangle unchanged', () => {
    const m = projection(rect(200, 300))
    expect(project(m, 0.5, 0.5)).toEqual({ x: 100, y: 150 })
    expect(project(m, 1, 1)).toEqual({ x: 200, y: 300 })
  })

  it('sends each corner of the square to its corner of a tilted shape', () => {
    // A cover photographed from below and to the left: wider at the bottom, leaning.
    const quad: Quad = [
      { x: 120, y: 80 },
      { x: 410, y: 60 },
      { x: 460, y: 520 },
      { x: 70, y: 540 },
    ]
    const m = projection(quad)
    const corners = [project(m, 0, 0), project(m, 1, 0), project(m, 1, 1), project(m, 0, 1)]
    corners.forEach((p, i) => {
      expect(p.x).toBeCloseTo(quad[i].x, 6)
      expect(p.y).toBeCloseTo(quad[i].y, 6)
    })
  })

  it('sizes the result from the shape and keeps it under the limit', () => {
    expect(outputSize(rect(400, 600), 1600)).toEqual({ width: 400, height: 600 })
    expect(outputSize(rect(3000, 4000), 1600)).toEqual({ width: 1200, height: 1600 })
  })

  it('leaves a photo whose corners are its own edges as it was', () => {
    const data = new Uint8ClampedArray(4 * 4 * 4).map((_, i) => (i * 37) % 256)
    const out = warp({ width: 4, height: 4, data }, rect(4, 4), 4, 4)
    expect([...out.data]).toEqual([...data])
  })
})
