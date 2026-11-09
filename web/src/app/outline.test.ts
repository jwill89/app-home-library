import { createRequire } from 'node:module'
import { beforeAll, describe, expect, it } from 'vitest'
import { orderCorners } from './detect'
import { findOutline, type CV } from './outline'
import type { Point } from './perspective'

// The real OpenCV, so an upgrade that drops or changes what the detection calls fails here,
// where the type check cannot always see it. It is loaded through Node's require: the package
// is CommonJS exporting a promise (opencv-js 5), which Vitest's import wraps in a namespace that
// breaks when awaited. How the worker loads it in a browser, the end-to-end test of a cover
// photo checks (catalog.e2e.ts).
let cv: CV
beforeAll(async () => {
  const load = createRequire(import.meta.url)
  cv = await (load('@techstark/opencv-js') as Promise<CV>)
}, 30_000)

/** A width by height photo, the cover light where `cover` says and the table dark elsewhere. */
function photo(width: number, height: number, cover: (x: number, y: number) => boolean): ImageData {
  const data = new Uint8ClampedArray(width * height * 4)
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      const shade = cover(x, y) ? 210 : 35
      data.set([shade, shade, shade, 255], (y * width + x) * 4)
    }
  }
  return { width, height, data, colorSpace: 'srgb' }
}

// A cover from (80, 50) to (320, 250) in a 400 by 300 photo.
const onCover = (x: number, y: number) => x >= 80 && x < 320 && y >= 50 && y < 250
const corners: Point[] = [
  { x: 80, y: 50 },
  { x: 320, y: 50 },
  { x: 320, y: 250 },
  { x: 80, y: 250 },
]

/** The outline found has the cover's corners, give or take what the blur and dilation add. */
function expectCorners(found: Point[] | null) {
  expect(found).toHaveLength(4)
  orderCorners(found ?? []).forEach((p, i) => {
    expect(Math.abs(p.x - (corners[i]?.x ?? NaN))).toBeLessThanOrEqual(4)
    expect(Math.abs(p.y - (corners[i]?.y ?? NaN))).toBeLessThanOrEqual(4)
  })
}

describe('findOutline', () => {
  it('finds the four corners of a cover on a table', () => {
    expectCorners(findOutline(cv, photo(400, 300, onCover)))
  })

  it('boxes a cover with a worn corner by its outline', () => {
    // The top left corner gone, so the outline has five sides, not four.
    const worn = (x: number, y: number) => onCover(x, y) && x - 80 + (y - 50) >= 60
    expectCorners(findOutline(cv, photo(400, 300, worn)))
  })

  it('finds nothing in a photo with no cover in it', () => {
    expect(
      findOutline(
        cv,
        photo(400, 300, () => false),
      ),
    ).toBeNull()
  })
})
