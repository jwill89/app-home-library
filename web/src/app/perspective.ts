// Straightening a photographed cover: the owner puts a corner on each of the book's corners,
// and the four-sided shape they mark is mapped onto a flat rectangle. A cover shot at an
// angle comes out square, the way a document scanner's page does.

export interface Point {
  x: number
  y: number
}

/** The four corners, clockwise from the top left. */
export type Quad = [Point, Point, Point, Point]

/** A projective map from the unit square (u, v in 0..1) to a quad: x = (a u + b v + c) / (g u + h v + 1). */
interface Projection {
  a: number
  b: number
  c: number
  d: number
  e: number
  f: number
  g: number
  h: number
}

// The closed form for the square-to-quadrilateral projective mapping, with (0,0), (1,0),
// (1,1), (0,1) going to the corners in order.
export function projection([p0, p1, p2, p3]: Quad): Projection {
  const sx = p0.x - p1.x + p2.x - p3.x
  const sy = p0.y - p1.y + p2.y - p3.y
  if (Math.abs(sx) < 1e-9 && Math.abs(sy) < 1e-9) {
    // A parallelogram: the map is affine.
    return {
      a: p1.x - p0.x,
      b: p3.x - p0.x,
      c: p0.x,
      d: p1.y - p0.y,
      e: p3.y - p0.y,
      f: p0.y,
      g: 0,
      h: 0,
    }
  }
  const dx1 = p1.x - p2.x
  const dx2 = p3.x - p2.x
  const dy1 = p1.y - p2.y
  const dy2 = p3.y - p2.y
  const det = dx1 * dy2 - dx2 * dy1
  const g = (sx * dy2 - dx2 * sy) / det
  const h = (dx1 * sy - sx * dy1) / det
  return {
    a: p1.x - p0.x + g * p1.x,
    b: p3.x - p0.x + h * p3.x,
    c: p0.x,
    d: p1.y - p0.y + g * p1.y,
    e: p3.y - p0.y + h * p3.y,
    f: p0.y,
    g,
    h,
  }
}

/** Where a point of the unit square lands in the photo. */
export function project(m: Projection, u: number, v: number): Point {
  const w = m.g * u + m.h * v + 1
  return { x: (m.a * u + m.b * v + m.c) / w, y: (m.d * u + m.e * v + m.f) / w }
}

const distance = (p: Point, q: Point) => Math.hypot(p.x - q.x, p.y - q.y)

/**
 * The size of the straightened cover: the average of the marked shape's opposite sides,
 * scaled down so neither side passes `longest`.
 */
export function outputSize(
  [p0, p1, p2, p3]: Quad,
  longest: number,
): { width: number; height: number } {
  const width = (distance(p0, p1) + distance(p3, p2)) / 2
  const height = (distance(p0, p3) + distance(p1, p2)) / 2
  const fit = Math.min(1, longest / Math.max(width, height))
  return {
    width: Math.max(1, Math.round(width * fit)),
    height: Math.max(1, Math.round(height * fit)),
  }
}

/** The pixels a photo has: ImageData's shape, without needing a browser to make one. */
interface Pixels {
  width: number
  height: number
  data: Uint8ClampedArray<ArrayBuffer>
}

/**
 * Maps the marked shape of the photo onto a width × height rectangle, each output pixel
 * sampled from the four photo pixels around where it lands.
 */
export function warp(src: Pixels, quad: Quad, width: number, height: number): Pixels {
  const m = projection(quad)
  const out = new Uint8ClampedArray(width * height * 4)
  const sw = src.width
  const sh = src.height
  const s = src.data
  for (let y = 0; y < height; y++) {
    const v = (y + 0.5) / height
    for (let x = 0; x < width; x++) {
      const p = project(m, (x + 0.5) / width, v)
      const fx = Math.min(Math.max(p.x - 0.5, 0), sw - 1)
      const fy = Math.min(Math.max(p.y - 0.5, 0), sh - 1)
      const x0 = Math.floor(fx)
      const y0 = Math.floor(fy)
      const x1 = Math.min(x0 + 1, sw - 1)
      const y1 = Math.min(y0 + 1, sh - 1)
      const tx = fx - x0
      const ty = fy - y0
      const i00 = (y0 * sw + x0) * 4
      const i10 = (y0 * sw + x1) * 4
      const i01 = (y1 * sw + x0) * 4
      const i11 = (y1 * sw + x1) * 4
      const o = (y * width + x) * 4
      for (let c = 0; c < 4; c++) {
        const top = (s[i00 + c] ?? 0) * (1 - tx) + (s[i10 + c] ?? 0) * tx
        const bottom = (s[i01 + c] ?? 0) * (1 - tx) + (s[i11 + c] ?? 0) * tx
        out[o + c] = top * (1 - ty) + bottom * ty
      }
    }
  }
  return { width, height, data: out }
}
