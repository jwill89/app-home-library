// Finds a book in a photo: the largest four-sided outline among the photo's edges. It runs in
// a worker because OpenCV's bindings build functions at run time, which the pages' policy
// forbids and this script's own policy allows (web/embed.go); it also keeps the work off the
// page's thread.
import cvModule from '@techstark/opencv-js'

type CV = typeof cvModule
interface Request {
  id: number
  image: ImageData
}
interface Point {
  x: number
  y: number
}

const scope = self as unknown as {
  postMessage(message: unknown): void
  addEventListener(type: 'message', listener: (e: MessageEvent<Request>) => void): void
  addEventListener(type: 'unhandledrejection', listener: (e: PromiseRejectionEvent) => void): void
}

// A browser that holds this worker to the page's policy forbids building functions, and the
// runtime then fails where nothing reports it. That is found out first, so every request is
// answered at once and the owner places the corners by hand.
let unable = ''
try {
  // eslint-disable-next-line @typescript-eslint/no-implied-eval -- the check for exactly what the runtime does
  new Function('return 0')()
} catch (e) {
  unable = `This browser does not let the cover detection run: ${String(e)}`
}

// Requests still waiting when the runtime fails in a way only an unhandled rejection reports.
const waiting = new Set<number>()
scope.addEventListener('unhandledrejection', (e) => {
  unable = `The cover detection stopped: ${String(e.reason)}`
  for (const id of waiting) scope.postMessage({ id, points: null, error: unable })
  waiting.clear()
})

// The module is a thenable whose `then` calls back once the runtime is up, already or later.
// The promise here settles with nothing, never with the module: a promise resolved with a
// thenable calls its `then` again, which resolves again, forever. A runtime that cannot
// start says why through onAbort, so the page is told at once.
let ready: Promise<void> | undefined
function opencv(): Promise<void> {
  ready ??= new Promise((resolve, reject) => {
    const m = cvModule as CV & {
      then?: (callback: () => void) => unknown
      onAbort?: (what: unknown) => void
    }
    m.onAbort = (what) => reject(new Error(`OpenCV did not start: ${String(what)}`))
    if (typeof m.then === 'function') m.then(() => resolve())
    else resolve()
  })
  return ready
}

function find(cv: CV, image: ImageData): Point[] | null {
  const area = image.width * image.height
  const src = cv.matFromImageData(image)
  const gray = new cv.Mat()
  const edges = new cv.Mat()
  const kernel = cv.getStructuringElement(cv.MORPH_RECT, new cv.Size(5, 5))
  const contours = new cv.MatVector()
  const hierarchy = new cv.Mat()
  try {
    cv.cvtColor(src, gray, cv.COLOR_RGBA2GRAY)
    cv.GaussianBlur(gray, gray, new cv.Size(5, 5), 0)
    cv.Canny(gray, edges, 40, 120)
    // Close the small gaps a cover's edge has where it meets something of the same shade.
    cv.dilate(edges, edges, kernel)
    cv.findContours(edges, contours, hierarchy, cv.RETR_EXTERNAL, cv.CHAIN_APPROX_SIMPLE)
    let best: Point[] | null = null
    let bestArea = 0
    for (let i = 0; i < contours.size(); i++) {
      const c = contours.get(i)
      const a = cv.contourArea(c)
      // A cover fills a good part of a photo of it, and is not the photo's own frame.
      if (a < area * 0.15 || a > area * 0.98 || a <= bestArea) {
        c.delete()
        continue
      }
      const approx = new cv.Mat()
      cv.approxPolyDP(c, approx, 0.02 * cv.arcLength(c, true), true)
      let points: Point[]
      if (approx.rows === 4 && cv.isContourConvex(approx)) {
        const d = approx.data32S
        points = [0, 1, 2, 3].map((k) => ({ x: d[k * 2] ?? 0, y: d[k * 2 + 1] ?? 0 }))
      } else {
        // Rounded or worn corners: the tightest box around the outline.
        points = cv.RotatedRect.points(cv.minAreaRect(c)).map((p) => ({ x: p.x, y: p.y }))
      }
      best = points
      bestArea = a
      approx.delete()
      c.delete()
    }
    return best
  } finally {
    for (const m of [src, gray, edges, kernel, contours, hierarchy]) m.delete()
  }
}

scope.addEventListener('message', (e) => {
  const { id, image } = e.data
  if (unable) {
    scope.postMessage({ id, points: null, error: unable })
    return
  }
  waiting.add(id)
  opencv()
    .then(() => scope.postMessage({ id, points: find(cvModule, image) }))
    .catch((err: unknown) => scope.postMessage({ id, points: null, error: String(err) }))
    .finally(() => waiting.delete(id))
})
