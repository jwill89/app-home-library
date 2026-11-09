import type cvModule from '@techstark/opencv-js'
import type { Point } from './perspective'

// Finds a book in a photo with OpenCV: the largest four-sided outline among the photo's edges.
// detect.worker.ts loads OpenCV and runs this off the page's thread; it lives here, apart from
// the worker and with OpenCV's types alone, so outline.test.ts can run it on the real OpenCV.

export type CV = typeof cvModule

/** The corners of the largest four-sided outline in the photo, or null when none is clear. */
export function findOutline(cv: CV, image: ImageData): Point[] | null {
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
    for (const m of [src, gray, edges, kernel, contours, hierarchy]) {
      m.delete()
    }
  }
}
