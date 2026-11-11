// Finds a book in a photo (outline.ts). It runs in a worker because OpenCV's bindings build
// functions at run time, which the pages' policy forbids and this script's own policy allows
// (web/embed.go); it also keeps the work off the page's thread.
import cvModule from '@techstark/opencv-js'
import { findOutline, type CV } from './outline'

// opencv-js 5 exports a promise of the module. Its types describe the module itself, so the
// type check cannot tell; the end-to-end test of a cover photo can. A runtime that cannot
// start rejects the promise, saying why, and the unhandled rejection below tells every
// request waiting.
const opencv = Promise.resolve(cvModule as unknown as Promise<CV>)

interface Request {
  id: number
  image: ImageData
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
  for (const id of waiting) {
    scope.postMessage({ id, points: null, error: unable })
  }
  waiting.clear()
})

scope.addEventListener('message', (e) => {
  const { id, image } = e.data
  if (unable) {
    scope.postMessage({ id, points: null, error: unable })
    return
  }
  waiting.add(id)
  opencv
    .then((cv) => scope.postMessage({ id, points: findOutline(cv, image) }))
    .catch((err: unknown) => scope.postMessage({ id, points: null, error: String(err) }))
    .finally(() => waiting.delete(id))
})
