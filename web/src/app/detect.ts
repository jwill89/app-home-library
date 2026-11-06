import type { Point, Quad } from './perspective'

// Automatic cover detection: the photo goes to a worker running OpenCV (detect.worker.ts),
// which answers with the corners of the book it finds, or nothing.

// Small enough to search quickly on a phone, large enough to find a book's edges.
const SEARCH = 640
// The first search loads OpenCV, several megabytes; a phone on a slow line gets this long.
const WAIT = 30000

/** The four corners in order, clockwise from the top left, whatever order they came in. */
export function orderCorners(points: Point[]): Quad {
  const bySum = [...points].sort((a, b) => a.x + a.y - (b.x + b.y))
  const byDiff = [...points].sort((a, b) => a.y - a.x - (b.y - b.x))
  const [tl, br] = [bySum[0], bySum[bySum.length - 1]]
  const [tr, bl] = [byDiff[0], byDiff[byDiff.length - 1]]
  return [tl, tr, br, bl].map((p) => ({ x: p?.x ?? 0, y: p?.y ?? 0 })) as Quad
}

let worker: Worker | undefined
let next = 0
const waiting = new Map<number, (points: Point[] | null) => void>()

function detector(): Worker {
  if (!worker) {
    worker = new Worker(new URL('./detect.worker.ts', import.meta.url))
    worker.addEventListener(
      'message',
      (e: MessageEvent<{ id: number; points: Point[] | null }>) => {
        waiting.get(e.data.id)?.(e.data.points)
        waiting.delete(e.data.id)
      },
    )
    // A worker that cannot start answers every question with nothing, and the owner places
    // the corners by hand as before.
    worker.addEventListener('error', () => {
      waiting.forEach((answer) => answer(null))
      waiting.clear()
    })
  }
  return worker
}

/** The corners of the cover in a photo, in the photo's own pixels, or null when none is clear. */
export async function detectCover(photo: HTMLCanvasElement): Promise<Quad | null> {
  const fit = Math.min(1, SEARCH / Math.max(photo.width, photo.height))
  const small = document.createElement('canvas')
  small.width = Math.max(1, Math.round(photo.width * fit))
  small.height = Math.max(1, Math.round(photo.height * fit))
  const ctx = small.getContext('2d')
  if (!ctx) return null
  ctx.drawImage(photo, 0, 0, small.width, small.height)
  const image = ctx.getImageData(0, 0, small.width, small.height)
  const id = ++next
  const points = await new Promise<Point[] | null>((resolve) => {
    waiting.set(id, resolve)
    setTimeout(() => {
      if (waiting.delete(id)) resolve(null)
    }, WAIT)
    detector().postMessage({ id, image })
  })
  if (points?.length !== 4) return null
  return orderCorners(points.map((p) => ({ x: p.x / fit, y: p.y / fit })))
}
